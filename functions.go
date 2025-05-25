package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

func promptOllama(incomeFlt float64, goal string) {
	requestDetails := OllamaRequest{
		Url: "http://loalhost:11434/api/chat",
		Headers: Headers{
			ContentType: "application/json",
		},
		Data: Data{
			Model:  "",
			Prompt: "",
			Stream: true,
		},
	}

	log.Print(requestDetails) // TODO: Verify
}

func determineIncome(income string) (incomeFlt float64) {
	// Check if flag was passed at runtime, if so no need to prompt the user.
	if income == "" {
		scanner := bufio.NewScanner(os.Stdin)

		fmt.Println("What is your monthly income (after taxes & deductions)?")
		if scanner.Scan() {
			income = scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			log.Fatalln("Error reading income response: ", err)
		}
	}

	// Verify income is a valid dollar amount by convetting to Float64.
	func() {
		replacer := strings.NewReplacer("$", "", ",", "")
		income = replacer.Replace(income)

		var err error
		incomeFlt, err = strconv.ParseFloat(income, 64)
		if err != nil {
			log.Fatalln("Error formatting income: ", err)
		}
	}()

	return incomeFlt
}

func determineGoal(goal string) string {
	const defaultGoal = "Pay off debt as quickly and efficiently as possible while not straining my monthly budget."

	// Check if flag was passed at runtime, if so no need to prompt the user.
	if goal != defaultGoal {
		return goal
	}

	// Prompt the user for their desired financial goal.
	func() {
		scanner := bufio.NewScanner(os.Stdin)

		fmt.Printf("What is your financial goal? If you like the default value of \"%s\", then just press enter.\n", defaultGoal)
		if scanner.Scan() {
			goal = scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			log.Fatalln("Error reading goal response: ", err)
		}
	}()

	if goal == "" {
		return defaultGoal
	}

	return goal
}
