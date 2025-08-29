#!/bin/bash

rm -f ./Obligation Advice*.md

go run ../. -data="obligations_sample.xlsx" -income="2000" -goal="Determine a specific prioritized strategy to payoff my loan(s) and credit card(s) as quickly & efficiently as possible without straining my monthly budget " #-model="" -excludeThink=false -outDir=""