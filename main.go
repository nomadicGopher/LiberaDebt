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
	Description string  `json:"Description"`                       // Required
	Type        string  `json:"Type"`                              // Required
	Balance     float64 `json:"Total Remaining Balance,omitempty"` // Optional
	Interest    float64 `json:"Interest Rate %,omitempty"`         // Optional
	Payment     float64 `json:"Minimum Monthly Payment"`           // Required
}

func main() {
	const defaultGoal = "Provide a shortest-time payoff plan using any leftover budget for extra payments to loans and/or credit cards"

	dataPath := flag.String("data", "./obligations.xlsx", "Full-path to financial obligations spreadsheet.")
	income := flag.String("income", "", "User's monthly income (after taxes & deductions). Exclude $ and , characters.")
	goal := flag.String("goal", defaultGoal, "User's financial goal for Ollama to provide advice for accomplishing.")
	excludeThink := flag.Bool("excludeThink", true, "true to remove thinking content from the output file, false to keep it.")
	model := flag.String("model", "qwen3:8b", "What Large Language Model will be used via Ollama?")
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

	err = writeOutFile(*dataPath, *goal, *excludeThink, responseBuilder)
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

		description := strings.TrimSpace(row.Cells[0].Value)    // Required
		obligationType := strings.TrimSpace(row.Cells[1].Value) // Required
		balance := strings.TrimSpace(row.Cells[2].Value)        // Optional
		interest := strings.TrimSpace(row.Cells[3].Value)       // Optional
		payment := strings.TrimSpace(row.Cells[4].Value)        // Required

		// Ensure that required fields are populated with more than ""
		if description == "" {
			return nil, fmt.Errorf("xlsx row %d, Description is required but is empty", xlsxRowNumber)
		}

		if obligationType == "" {
			return nil, fmt.Errorf("xlsx row %d, Type is required but is empty", xlsxRowNumber)
		}

		if payment == "" {
			return nil, fmt.Errorf("xlsx row %d, Minimum Monthly Payment is required but is empty", xlsxRowNumber)
		}

		// Ensure input values convert to their appropriate types
		var (
			remainingBalanceFloat, interestRateFloat float64
			err                                      error
		)

		if balance != "" {
			remainingBalanceFloat, err = strconv.ParseFloat(balance, 64)
			if err != nil {
				return nil, fmt.Errorf("error formatting Total Remaining Balance from XLSX row %d: %v", xlsxRowNumber, err)
			}
		}

		if interest != "" {
			interestRateFloat, err = strconv.ParseFloat(interest, 64)
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

		monthlyPaymentFloat, err := strconv.ParseFloat(payment, 64)
		if err != nil {
			return nil, fmt.Errorf("error formatting Monthly Payment (required) from XLSX row %d: %v", xlsxRowNumber, err)
		}

		// Required Fields
		obligation := Obligation{
			Description: description,
			Type:        obligationType,
			Payment:     monthlyPaymentFloat,
		}

		// Optional Fields
		if remainingBalanceFloat != 0.00 {
			obligation.Balance = remainingBalanceFloat
		}

		if interestRateFloat != 0.00 {
			obligation.Interest = interestRateFloat
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
			return strings.Builder{}, fmt.Errorf("error installing %s in Ollama model (if missing): %v", model, err)
		}
	}

	// Prepare to generate response with Ollama
	respReq := &ollama.GenerateRequest{
		Model: model,
		Prompt: fmt.Sprintf(`You are a cost-efficient financial planner.
My monthly income is $%.2f.
My obligations are %s.
If no comparable leisure budget exists and at least 5 percent (x) of income remains, create a $x leisure expense.
%s.
If no money is leftover, let the user know and assume this plan is for when additional funds are available.
Provide concise, actionable short-term and long-term steps with exact dollar amounts.
Briefly explain your reasoning for each step.
Only suggest extra payments for loans and credit cards.
Do not consider user preferences or alternative scenarios; only provide the most efficient solution.
Do not enumerate or compare multiple strategies.
Do not respond with formulas or calculations for the user to perform.
Do not list monthly expenses or bills in your response; they are for context only.
Ignore the concepts of principal contributions as well as fixed vs variable interest rate types.
Ensure no loan or credit card payment is counted or allocated more than once in any transactions or calculations.`,
			incomeFlt, formattedObligations, goal),
	}

	fmt.Printf("%s\n\n", respReq.Prompt)

	respFunc := func(resp ollama.GenerateResponse) error {
		fmt.Print(resp.Response) // Stream to stdout as it arrives
		responseBuilder.WriteString(resp.Response)
		return nil
	}

	// Generate response with Ollama
	fmt.Printf("Communicating with Ollama...\n\n")
	startTime := time.Now()
	err = client.Generate(ctx, respReq, respFunc)
	if err != nil {
		return strings.Builder{}, fmt.Errorf("error generating Ollama response: %v", err)
	}
	endTime := time.Now()
	fmt.Printf("\n\nResponse generated in %v.\n", endTime.Sub(startTime))

	return responseBuilder, nil
}

// writeOutFile creates an output file and writes goal and response in the same directory as the data file
func writeOutFile(dataPath, goal string, excludeThink bool, responseBuilder strings.Builder) error {
	now := time.Now()
	outFileName := fmt.Sprintf("obligation_advice_%s.md",
		now.Format("2006-01-02_15-04-05"))
	dataDir := filepath.Dir(dataPath)
	outFilePath := filepath.Join(dataDir, outFileName)
	outFile, err := os.Create(outFilePath)
	if err != nil {
		return fmt.Errorf("could not create output file: %v", err)
	}
	defer outFile.Close()

	fmt.Fprintf(outFile, "**Goal**: `%s`\n\n", goal)
	output := responseBuilder.String()
	if excludeThink {
		// remove all <think>...</think> blocks and any surrounding blank lines.
		re := regexp.MustCompile(`(?s)\s*<think>.*?</think>\s*`)
		output = re.ReplaceAllString(output, "")
	} else {
		output = strings.ReplaceAll(output, "<think>", "---\n### Started thinking")
		output = strings.ReplaceAll(output, "</think>", "### Ended thinking\n---")
	}

	fmt.Fprint(outFile, output)

	// Ensure outFilePath is absolute so the user knows exactly where the file was created
	if !filepath.IsAbs(outFilePath) {
		absPath, err := filepath.Abs(outFilePath)
		if err == nil {
			outFilePath = absPath
		}
	}

	fmt.Printf("\nOutput file written to: %s\n", outFilePath)

	return nil
}

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
