package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	ollama "github.com/ollama/ollama/api"
)

type FinancialInfo struct {
	Obligations []Obligation `json:"obligations"`
}

type Obligation struct {
	Description      string  `json:"description"`
	Type             string  `json:"type,omitempty"`
	Institution      string  `json:"institution,omitempty"`
	RemainingBalance float64 `json:"remaining_balance"`
	InterestRate     float64 `json:"interest_rate"`
	MonthlyPayment   float64 `json:"monthly_payment"`
	DayOfMonth       uint8   `json:"day_of_month,omitempty"`
}

func main() {
	const defaultGoal = "Pay off debt as quickly and efficiently as possible while not straining my monthly budget."

	income := flag.String("income", "", "User's monthly income (after taxes & deductions).")
	goal := flag.String("goal", defaultGoal, "User's financial goal for AI to provide advice for accomplishing.")
	financesPath := flag.String("finances", "./finances.xlsx", "Full-path to financial spreadsheet.")
	flag.Parse()

	incomeFlt, err := determineIncome(*income)
	checkErr(err)

	*goal, err = determineGoal(*goal, defaultGoal)
	checkErr(err)

	financialInfo, err := getFinancialInfo(*financesPath)
	checkErr(err)

	err = promptOllama(incomeFlt, financialInfo, *goal)
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
	fmt.Println("What is your financial goal? (If you like the default option, then ust press enter.)\nDefault: ", defaultGoal)

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

func getFinancialInfo(financesPath string) (financialInfo string, _ error) {
	// TODO: Use github.com/tealeg/xlsx to extract sheet info into FinancialInfo and then unmarshall it into a string for ollama to read.
	return financialInfo, nil
}

func promptOllama(incomeFlt float64, financialInfo, goal string) error {
	client, err := ollama.ClientFromEnvironment()
	if err != nil {
		return fmt.Errorf("error establishing connection to AI: %v", err)
	}

	ctx := context.Background()

	// Ensure model exists in Ollama
	const model = "qwen3:0.6b"

	modelReq := &ollama.PullRequest{
		Model: model,
	}

	progressFunc := func(resp ollama.ProgressResponse) error {
		fmt.Printf("Progress: status=%v, total=%v, completed=%v\n", resp.Status, resp.Total, resp.Completed)
		return nil
	}

	err = client.Pull(ctx, modelReq, progressFunc)
	if err != nil {
		return fmt.Errorf("error installing AI model: %v", err)
	}

	// Generate response
	respReq := &ollama.GenerateRequest{
		Model:  model,
		Prompt: fmt.Sprintf("I make $%.2f a month. My financial info is: %s. My goal is: %s. How can i most efficiently accomplish this?", incomeFlt, financialInfo, goal),
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
