#!/bin/bash
rm -f ./Obligation*.md

go run ../. -data="obligations_sample.xlsx" -income="2000" -goal="determine a strategy to payoff loan(s) and credit card(s) efficiently over time " -model="qwen3:0.6b" #-excludeThink=false -outDir=""