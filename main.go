package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	ollama "github.com/ollama/ollama/api"
	"github.com/tealeg/xlsx"
)

type Obligations struct {
	Obligations []Obligation `json:"obligations"`
}

type Obligation struct {
	ID               int     `json:"id"`
	Description      string  `json:"description"`
	Type             string  `json:"type"`
	Institution      string  `json:"institution,omitempty"`
	RemainingBalance float64 `json:"remaining_balance,omitempty"`
	InterestRate     float64 `json:"interest_rate,omitempty"`
	MonthlyPayment   float64 `json:"monthly_payment"`
	DayOfMonth       int     `json:"day_of_month,omitempty"`
}

func main() {
	const defaultGoal = "Pay off debt as quickly and efficiently as possible while not straining my monthly budget."

	income := flag.String("income", "", "User's monthly income (after taxes & deductions).")
	goal := flag.String("goal", defaultGoal, "User's financial goal for AI to provide advice for accomplishing.")
	dataPath := flag.String("data", "./obligations.xlsx", "Full-path to financial obligations spreadsheet.")
	llm := flag.String("llm", "qwen3:0.6b", "What Large Language Model will be used via Ollama?")
	flag.Parse()

	incomeFlt, err := determineIncome(*income)
	checkErr(err)

	*goal, err = determineGoal(*goal, defaultGoal)
	checkErr(err)

	obligations, err := getObligations(*dataPath)
	checkErr(err)

	formattedObligations, err := formatObligations(obligations)
	checkErr(err)

	err = promptOllama(incomeFlt, formattedObligations, *goal, *llm)
	checkErr(err)

	os.Exit(0)
}

// determineIncome checks the stdIn flags for an income. If none is found then the user is prompted to enter one. Then the value is
// stripped of special characters and assigned to a float to ensure it is valid.
func determineIncome(income string) (incomeFlt float64, _ error) {
	// Check if flag was passed at runtime. If so, no need to prompt the user.
	if income == "" {
		fmt.Println("What is your monthly income (after taxes & deductions)?")

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			income = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			return 0, fmt.Errorf("error reading income response: %v", err)
		}
	}

	// Verify income is a valid dollar amount by convetting to Float64.
	replacer := strings.NewReplacer("$", "", ",", "")
	income = replacer.Replace(income)

	var err error
	incomeFlt, err = strconv.ParseFloat(income, 64)
	if err != nil {
		return 0, fmt.Errorf("error formatting income: %v", err)
	}

	return incomeFlt, nil
}

// determineGoal checks the stdIn flags for a non-default goal. If it's still the default then the user is prompted for a new goal or to verify the default.
func determineGoal(goal, defaultGoal string) (string, error) {
	// Check if flag was passed at runtime, if so no need to prompt the user.
	if goal != defaultGoal {
		return goal, nil
	}

	// Prompt the user for their desired financial goal.
	fmt.Println("What is your financial goal? (If you like the default option, then just press enter.)\nDefault: ", defaultGoal)

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		goal = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading goal response: %v", err)
	}

	// User chose the default goal.
	if goal == "" {
		return defaultGoal, nil
	}

	return goal, nil
}

func getObligations(dataPath string) (obligations []Obligation, _ error) {
	workBook, err := xlsx.OpenFile(dataPath)
	if err != nil {
		return nil, fmt.Errorf("error opening XLSX workbook: %v", err)
	}

	sheet := workBook.Sheets[0]

	if len(sheet.Rows) < 2 {
		return nil, fmt.Errorf("no obligations (data rows) exist in XLSX sheet")
	}

	for i := 2; i <= len(sheet.Rows); i++ { // skip header row // TODO: Test this
		remainingBalance, err := sheet.Rows[i].Cells[3].Float()
		if err != nil {
			return nil, fmt.Errorf("error formatting Remaining Balance from XLSX row %d: %v", i, err)
		}

		interestRate, err := sheet.Rows[i].Cells[4].Float()
		if err != nil {
			return nil, fmt.Errorf("error formatting Interest Rate from XLSX row %d: %v", i, err)
		}

		monthlyPayment, err := sheet.Rows[i].Cells[5].Float()
		if err != nil {
			return nil, fmt.Errorf("error formatting Monthly Payment from XLSX row %d: %v", i, err)
		}

		dayOfMonth, err := sheet.Rows[i].Cells[6].Int()
		if err != nil {
			return nil, fmt.Errorf("error formatting Day Of Month from XLSX row %d: %v", i, err)
		}

		obligation := Obligation{
			ID:               i - 1, // TODO: Validate this
			Description:      sheet.Rows[i].Cells[0].String(),
			Type:             sheet.Rows[i].Cells[1].String(),
			Institution:      sheet.Rows[i].Cells[2].String(),
			RemainingBalance: remainingBalance,
			InterestRate:     interestRate,
			MonthlyPayment:   monthlyPayment,
			DayOfMonth:       dayOfMonth,
		}

		obligations = append(obligations, obligation)
	}

	return obligations, nil
}

func formatObligations(obligations []Obligation) (formattedObligations string, _ error) {
	// TODO
	for i, obligation := range obligations {
		formattedObligation, err := json.Marshal(obligation)
		if err != nil {
			return "", fmt.Errorf("error marshaling obligation XLSX row #%d: %v", i+2, err) // TODO: Test i by omitting a required field
		}

		formattedObligations = formattedObligations + string(formattedObligation)
	}

	return formattedObligations, nil
}

func promptOllama(incomeFlt float64, formattedObligations, goal, llm string) error {
	client, err := ollama.ClientFromEnvironment()
	if err != nil {
		return fmt.Errorf("error establishing connection to AI: %v", err)
	}

	ctx := context.Background()

	// Ensure model exists in Ollama
	modelReq := &ollama.PullRequest{
		Model: llm,
	}

	progressFunc := func(resp ollama.ProgressResponse) error {
		fmt.Printf("Progress: %v ( %v / %v )\n", resp.Status, resp.Completed, resp.Total)
		return nil
	}

	err = client.Pull(ctx, modelReq, progressFunc)
	if err != nil {
		return fmt.Errorf("error installing AI model: %v", err)
	}

	fmt.Println("")

	// Generate response
	log.Fatal(formattedObligations) ///!

	const headers = "" // TODO in JSON format

	respReq := &ollama.GenerateRequest{
		Model: llm,
		Prompt: fmt.Sprintf(`I make $%.2f a month. As a list of JSON formatted objects (starting with header info), my 
financial obligations are: %s%s. My goal is: %s. How can I most efficiently accomplish my goal?`, incomeFlt, headers,
			formattedObligations, goal),
	}

	respFunc := func(resp ollama.GenerateResponse) error {
		fmt.Print(resp.Response)
		return nil
	}

	err = client.Generate(ctx, respReq, respFunc)
	if err != nil {
		return fmt.Errorf("error generating AI response: %v", err)
	}

	log.Println("Response complete.")
	return nil
}

func checkErr(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
