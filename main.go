package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	Institution      string  `json:"institution,omitempty"`       // Optional
	RemainingBalance float64 `json:"remaining_balance,omitempty"` // Optional
	InterestRate     float64 `json:"interest_rate,omitempty"`     // Optional
	MonthlyPayment   float64 `json:"monthly_payment"`             // Required
	DayOfMonth       int     `json:"day_of_month,omitempty"`      // Optional
}

func main() {
	const defaultGoal = "Determine a specific prioritized strategy to payoff my loan(s) and credit card(s) as quickly & efficiently as possible without straining my monthly budget"

	dataPath := flag.String("data", "./obligations.xlsx", "Full-path to financial obligations spreadsheet.")
	income := flag.String("income", "", "User's monthly income (after taxes & deductions). Exclude $ and , characters.")
	goal := flag.String("goal", defaultGoal, "User's financial goal for AI to provide advice for accomplishing.")
	model := flag.String("model", "deepseek-r1:1.5b", "What Large Language Model will be used via Ollama?")
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

	for i := 1; i <= len(sheet.Rows); i++ { // skip header row
		xlsxRowNumber := i + 1

		// Ensure that required fields are populated with more than ""
		if sheet.Rows[i].Cells[0].Value == "" &&
			(sheet.Rows[i].Cells[1].Value != "" || len(sheet.Rows[i].Cells) > 2) {
			return nil, fmt.Errorf("xlsx row %d, Description is required but is empty", xlsxRowNumber)
		} else if sheet.Rows[i].Cells[0].Value == "" {
			break // End of data despite number of rows in sheet since Description is required.
		}

		if sheet.Rows[i].Cells[1].Value == "" {
			return nil, fmt.Errorf("xlsx row %d, Type is required but is empty", xlsxRowNumber)
		}

		if sheet.Rows[i].Cells[5].Value == "" {
			return nil, fmt.Errorf("xlsx row %d, Monthly Amount is required but is empty", xlsxRowNumber)
		}

		// Ensure input values convert to their appropriate types
		var (
			institution                    string = sheet.Rows[i].Cells[2].String()
			remainingBalance, interestRate float64
			dayOfMonth                     int
			err                            error
		)

		if sheet.Rows[i].Cells[3].Value != "" {
			remainingBalance, err = sheet.Rows[i].Cells[3].Float()
			if err != nil {
				return nil, fmt.Errorf("error formatting Remaining Balance from XLSX row %d: %v", xlsxRowNumber, err)
			}
		}

		if sheet.Rows[i].Cells[4].Value != "" {
			interestRate, err = sheet.Rows[i].Cells[4].Float()
			if err != nil {
				return nil, fmt.Errorf("error formatting Interest Rate from XLSX row %d: %v", xlsxRowNumber, err)
			}
		}

		monthlyPayment, err := sheet.Rows[i].Cells[5].Float()
		if err != nil {
			return nil, fmt.Errorf("error formatting Monthly Payment (required) from XLSX row %d: %v", xlsxRowNumber, err)
		}

		if sheet.Rows[i].Cells[4].Value != "" {
			dayOfMonth, err = sheet.Rows[i].Cells[6].Int()
			if err != nil {
				return nil, fmt.Errorf("error formatting Day Of Month from XLSX row %d: %v", xlsxRowNumber, err)
			}
		}

		// Required Fields
		obligation := Obligation{
			Description:    sheet.Rows[i].Cells[0].String(),
			Type:           sheet.Rows[i].Cells[1].String(),
			MonthlyPayment: monthlyPayment,
		}

		// Optional Fields
		if institution != "" {
			obligation.Institution = institution
		}

		if remainingBalance != 0.00 {
			obligation.RemainingBalance = remainingBalance
		}

		if interestRate != 0.00 {
			obligation.InterestRate = interestRate
		}

		if dayOfMonth != 0 {
			obligation.DayOfMonth = dayOfMonth
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
	respReq := &ollama.GenerateRequest{
		Model:  model,
		Prompt: fmt.Sprintf(`My financial obligtations are %s. I make $%.2f a month. Help me accomplish my goal to %s. `, formattedObligations, incomeFlt, goal),
	}

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
			re := regexp.MustCompile(`(?m)(\s*\n)?<think>[\s\S]*?</think>(\s*\n)?`)
			return re.ReplaceAllString(s, "")
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
