#!/bin/bash

cd ..

rm -rf ./*.txt ./*.exe

go build

./LiberaDebt.exe -income="2000" -data="obligations_sample.xlsx" -goal="Determine a specific prioritized strategy to payoff my loan(s) and credit card(s) as quickly & efficiently as possible without straining my monthly budget " #-model="deepseek-r1:1.5b"