package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/outrigdev/outrig"
)

func main() {
	outrig.Init("LiberaDebt", nil)
	defer outrig.AppDone()

	income := flag.String("income", "", "User's monthly income (after taxes & deductions).")
	goal := flag.String("goal", "Pay off debt as quickly and efficiently as possible while not straining my monthly budget.", "User's financial goal for AI to provide advice for accomplishing.")
	flag.Parse()

	incomeFlt := determineIncome(*income)
	*goal = determineGoal(*goal)

	// TODO: func promptOllama(){}

	fmt.Sprintf("income: %.2f | goal: %s\n", incomeFlt, *goal)

	os.Exit(0)
}
