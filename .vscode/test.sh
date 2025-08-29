#!/bin/bash
rm -f ./Obligation*.md

go run ../. -data="obligations_sample.xlsx" -income="2000" -goal="determine a strategy to payoff my loan(s) and credit card(s) as quickly & efficiently as possible " #-model="" -excludeThink=false -outDir=""