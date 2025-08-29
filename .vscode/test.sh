#!/bin/bash


cd ..

rm -f ./*.txt

go build

./LiberaDebt.exe -model="deepseek-r1:1.5b" -data="obligations_sample.xlsx" -income="2,000" -goal="Determine a specific strategy including priorities & amounts to payoff my obligations as quickly & efficiently as possible without straining my monthly budget. "