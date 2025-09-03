#!/bin/bash
rm -f ./Obligation*.md

go run ../. -data="obligations_sample.xlsx" -income="2000" -goal="Provide a shortest-time payoff plan using any leftover budget for extra payments to loans and/or credit cards " -excludeThink=false -model="qwen3:0.6b" -outDir=""