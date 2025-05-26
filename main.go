package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	_ "github.com/ollama/ollama"
)

func main() {
	const defaultGoal = "Pay off debt as quickly and efficiently as possible while not straining my monthly budget."

	income := flag.String("income", "", "User's monthly income (after taxes & deductions).")
	goal := flag.String("goal", defaultGoal, "User's financial goal for AI to provide advice for accomplishing.")
	flag.Parse()

	incomeFlt := determineIncome(*income)
	*goal = determineGoal(*goal, defaultGoal)

	promptOllama(incomeFlt, *goal)

	os.Exit(0)
}

// determineIncome checks the stdIn flags for an income. If none is found then the user is prompted to enter one. Then the value is
// stripped of special characters and assigned to a float to ensure it is valid.
func determineIncome(income string) (incomeFlt float64) {
	// Check if flag was passed at runtime. If so, no need to prompt the user.
	if income == "" {
		fmt.Println("What is your monthly income (after taxes & deductions)?")

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			income = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			log.Fatalln("Error reading income response: ", err)
		}
	}

	// Verify income is a valid dollar amount by convetting to Float64.
	replacer := strings.NewReplacer("$", "", ",", "")
	income = replacer.Replace(income)

	var err error
	incomeFlt, err = strconv.ParseFloat(income, 64)
	if err != nil {
		log.Fatalln("Error formatting income: ", err)
	}

	return incomeFlt
}

// determineGoal checks the stdIn flags for a non-default goal. If it's still the default then the user is prompted for a new goal
// or to verify the default.
func determineGoal(goal, defaultGoal string) string {
	// Check if flag was passed at runtime, if so no need to prompt the user.
	if goal != defaultGoal {
		return goal
	}

	// Prompt the user for their desired financial goal.
	fmt.Println("What is your financial goal? (If you like the default option, then ust press enter.)\nDefault: ", defaultGoal)

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		goal = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		log.Fatalln("Error reading goal response: ", err)
	}

	// User chose the default goal.
	if goal == "" {
		return defaultGoal
	}

	return goal
}

func promptOllama(incomeFlt float64, goal string) {
	// TODO
}
