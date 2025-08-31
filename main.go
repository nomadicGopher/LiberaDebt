package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	ollama "github.com/ollama/ollama/api"
	"github.com/tealeg/xlsx"
)

type Obligations struct {
	Obligations []Obligation `json:"obligations"`
}

// Obligation is the columns associated with each row of data. Required vs Optional
// is controlled via logic found in getObligations().
type Obligation struct {
	Description      string  `json:"description"`                 // Required
	Type             string  `json:"type"`                        // Required
	RemainingBalance float64 `json:"remaining_balance,omitempty"` // Optional
	InterestRate     float64 `json:"interest_rate,omitempty"`     // Optional
	MonthlyPayment   float64 `json:"monthly_payment"`             // Required
}

const defaultGoal = "Maximize leftover monthly budget and create the fastest, most effective plan to pay off all loans and credit cards"

var guidelines = []string{
	"give concise, actionable steps with precise dollar amounts and briefly explain your reasoning.",
	"if no leisure/fun expense is listed, allocate up to 10 percent of monthly income if affordable and inform the user; otherwise, do not suggest changes to their leisure budget.",
	"monthly payments represent a minimum payment per month, only suggest additional payments for loans and credit cards",
	"ensure the payoff plan fully uses my monthly budget, allocating all remaining funds to loan or credit card payoff",
	"do not include formulas or require user calculations",
	"ignore principal contributions and interest rate types",
	"do not include monthly expenses or bills in your response; they are for context only",
	"ensure no loan or credit card payment is counted or allocated more than once in any transaction or calculation",
	"focus only on the most financially efficient strategy, not all possible user preferences",
}

func main() {
	dataPath := flag.String("data", "./obligations.xlsx", "Full-path to financial obligations spreadsheet.")
	income := flag.String("income", "", "User's monthly income (after taxes & deductions). Exclude $ and , characters.")
	goal := flag.String("goal", defaultGoal, "User's financial goal for AI to provide advice for accomplishing.")
	model := flag.String("model", "qwen3:8b", "What Large Language Model will be used via Ollama?")
	excludeThink := flag.Bool("excludeThink", true, "true to remove thinking content from the output file, false to keep it.")
	outDir := flag.String("outDir", "./", "Directory to write the output file to.")
	flag.Parse()

	incomeFlt, err := determineIncome(*income)
	checkErr(err)

	*goal, err = determineGoal(*goal, defaultGoal)
	checkErr(err)

	obligations, err := getObligations(*dataPath)
	checkErr(err)

	formattedObligations, err := formatObligations(obligations)
	checkErr(err)

	responseBuilder, err := promptOllama(incomeFlt, formattedObligations, *goal, *model)
	checkErr(err)

	err = writeOutFile(*outDir, *goal, *excludeThink, responseBuilder)
	checkErr(err)
}

// determineIncome checks the stdIn flags for an income. If none is found then the user is prompted to enter one.
// Then the value is stripped of special characters & assigned to a float to ensure it is valid.
func determineIncome(income string) (incomeFlt float64, _ error) {
	// Check if flag was passed at runtime. If so, no need to prompt the user.
	if income == "" {
		fmt.Print("What is your monthly income (after taxes & deductions)? ")

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			income = scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			return 0, fmt.Errorf("error reading income response: %v", err)
		}

		fmt.Println()
	}

	// Verify income is a valid dollar amount by convetting to Float64.
	replacer := strings.NewReplacer("$", " ", ",", "")
	income = replacer.Replace(income)

	var err error
	incomeFlt, err = strconv.ParseFloat(income, 64)
	if err != nil {
		return 0, fmt.Errorf("error formatting income: %v", err)
	}

	return incomeFlt, nil
}

// determineGoal checks the stdIn flags for a non-default goal.
// If it's still the default then the user is prompted for a new goal or to verify the default.
func determineGoal(goal, defaultGoal string) (string, error) {
	// Check if flag was passed at runtime, if so no need to prompt the user.
	if goal != defaultGoal {
		return goal, nil
	}

	// Prompt the user for their desired financial goal.
	fmt.Printf("Default Goal: %s\nWhat is your financial goal? (Press enter for the default): ", defaultGoal)

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		goal = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading goal response: %v", err)
	}

	fmt.Println()

	// User chose the default goal.
	if goal == "" {
		return defaultGoal, nil
	}

	return goal, nil
}

// getObligations fetches data from obligations.xlsx & reads them into memory for use in other functions.
func getObligations(dataPath string) (obligations []Obligation, _ error) {
	workBook, err := xlsx.OpenFile(dataPath)
	if err != nil {
		return nil, fmt.Errorf("error opening XLSX workbook: %v", err)
	}

	sheet := workBook.Sheets[0]

	if len(sheet.Rows) < 2 {
		return nil, fmt.Errorf("no obligations (rows of data) exist in XLSX sheet")
	}

	// isRowEmpty checks if all cells in a row are empty.
	isRowEmpty := func(row *xlsx.Row) bool {
		for _, cell := range row.Cells {
			if strings.TrimSpace(cell.Value) != "" {
				return false
			}
		}
		return true
	}

	for i, row := range sheet.Rows[1:] { // skip header row
		if isRowEmpty(row) {
			continue
		}

		xlsxRowNumber := i + 2

		description := strings.TrimSpace(row.Cells[0].Value)      // Required
		obligationType := strings.TrimSpace(row.Cells[1].Value)   // Required
		remainingBalance := strings.TrimSpace(row.Cells[2].Value) // Optional
		interestRate := strings.TrimSpace(row.Cells[3].Value)     // Optional
		monthlyPayment := strings.TrimSpace(row.Cells[4].Value)   // Required

		// Ensure that required fields are populated with more than ""
		if description == "" {
			return nil, fmt.Errorf("xlsx row %d, Description is required but is empty", xlsxRowNumber)
		}

		if obligationType == "" {
			return nil, fmt.Errorf("xlsx row %d, Type is required but is empty", xlsxRowNumber)
		}

		if monthlyPayment == "" {
			return nil, fmt.Errorf("xlsx row %d, Monthly Amount is required but is empty", xlsxRowNumber)
		}

		// Ensure input values convert to their appropriate types
		var (
			remainingBalanceFloat, interestRateFloat float64
			err                                      error
		)

		if remainingBalance != "" {
			remainingBalanceFloat, err = strconv.ParseFloat(remainingBalance, 64)
			if err != nil {
				return nil, fmt.Errorf("error formatting Remaining Balance from XLSX row %d: %v", xlsxRowNumber, err)
			}
		}

		if interestRate != "" {
			interestRateFloat, err = strconv.ParseFloat(interestRate, 64)
			if err != nil {
				return nil, fmt.Errorf("error formatting Interest Rate from XLSX row %d: %v", xlsxRowNumber, err)
			}
			// Convert decimal to percent if value is less than or equal to 1
			if interestRateFloat <= 1.0 {
				interestRateFloat = interestRateFloat * 100
			}
			// Round to 2 decimal places
			interestRateFloat = math.Round(interestRateFloat*100) / 100
		}

		monthlyPaymentFloat, err := strconv.ParseFloat(monthlyPayment, 64)
		if err != nil {
			return nil, fmt.Errorf("error formatting Monthly Payment (required) from XLSX row %d: %v", xlsxRowNumber, err)
		}

		// Required Fields
		obligation := Obligation{
			Description:    description,
			Type:           obligationType,
			MonthlyPayment: monthlyPaymentFloat,
		}

		// Optional Fields
		if remainingBalanceFloat != 0.00 {
			obligation.RemainingBalance = remainingBalanceFloat
		}

		if interestRateFloat != 0.00 {
			obligation.InterestRate = interestRateFloat
		}

		obligations = append(obligations, obligation)
	}

	return obligations, nil
}

// formatObligations concatenates xlsx.rows (obligations) into a single string which Ollama can understand.
func formatObligations(obligations []Obligation) (formattedObligations string, _ error) {
	for i, obligation := range obligations {
		formattedObligation, err := json.Marshal(obligation)
		if err != nil {
			return "", fmt.Errorf("error marshaling obligation XLSX row #%d: %v", i+2, err)
		}

		formattedObligations = formattedObligations + string(formattedObligation)
	}

	return formattedObligations, nil
}

// promptOllama sets up the connection with Ollama & generates a request/response to stdOut and a .txt file.
func promptOllama(incomeFlt float64, formattedObligations, goal, model string) (responseBuilder strings.Builder, _ error) {
	// Establish client & verify is running
	client, err := ollama.ClientFromEnvironment()
	if err != nil {
		return strings.Builder{}, fmt.Errorf("error creating an Ollama client: %v", err)
	}

	ctx := context.Background()

	err = client.Heartbeat(ctx)
	if err != nil {
		return strings.Builder{}, fmt.Errorf("error connecting to the Ollama server, ensure it's running elsewhere with $ ollama serve")
	}

	// Ensure model exists
	installed, err := client.List(ctx)
	if err != nil {
		return strings.Builder{}, fmt.Errorf("error fetching the list of Ollama models: %v", err)
	}

	modelExists := false
	for _, models := range installed.Models {
		if models.Name == model {
			modelExists = true
		}
	}
	if !modelExists {
		modelReq := &ollama.PullRequest{
			Model: model,
		}

		progressFunc := func(resp ollama.ProgressResponse) error {
			fmt.Printf("Progress: %v ( %v / %v )\n", resp.Status, resp.Completed, resp.Total)
			return nil
		}

		err = client.Pull(ctx, modelReq, progressFunc)
		if err != nil {
			return strings.Builder{}, fmt.Errorf("error installing AI model (if missing): %v", err)
		}
	}

	// Prepare to generate response with Ollama
	concatenatedGuidelines := strings.Join(guidelines, " | ")

	respReq := &ollama.GenerateRequest{
		Model: model,
		Prompt: fmt.Sprintf(
			"My financial obligtations are %s. My monthly income is $%.2f. %s. Follow these guidelines: %s.",
			formattedObligations, incomeFlt, goal, concatenatedGuidelines),
	}

	// fmt.Printf("%s\n\n", respReq.Prompt)

	respFunc := func(resp ollama.GenerateResponse) error {
		fmt.Print(resp.Response) // Stream to stdout as it arrives
		responseBuilder.WriteString(resp.Response)
		return nil
	}

	// Generate response with Ollama
	fmt.Printf("Communicating with AI...\n\n")
	err = client.Generate(ctx, respReq, respFunc)
	if err != nil {
		return strings.Builder{}, fmt.Errorf("error generating AI response: %v", err)
	}

	return responseBuilder, nil
}

// writeOutFile creates an output file and write goal and response
func writeOutFile(outDir, goal string, excludeThink bool, responseBuilder strings.Builder) error {
	now := time.Now()
	outFileName := fmt.Sprintf("Obligation Advice %d-%d %dh %dm %ds.md",
		now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	outFilePath := filepath.Join(outDir, outFileName)
	outFile, err := os.Create(outFilePath)
	if err != nil {
		return fmt.Errorf("could not create output file: %v", err)
	}
	defer outFile.Close()

	fmt.Fprintf(outFile, "**Goal**: `%s`\n\n", goal)
	output := responseBuilder.String()
	if excludeThink {
		// removeThinkTags removes all <think>...</think> blocks and any surrounding blank lines.
		removeThinkTags := func(s string) string {
			re := regexp.MustCompile(`(?s)<think>.*?</think>`)
			s = re.ReplaceAllString(s, "")
			return strings.TrimSpace(s)
		}

		output = removeThinkTags(output)
	}
	fmt.Fprint(outFile, output)

	if outDir == "./" {
		fullDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not get the current working directory: %v", err)
		}
		outFilePath = filepath.Join(fullDir, outFilePath)
	}

	fmt.Printf("\n\nOutput file written to: %s\n", outFilePath)

	return nil
}

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
