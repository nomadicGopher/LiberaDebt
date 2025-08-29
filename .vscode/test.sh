#!/bin/bash


cd ..

rm -rf ./*.txt ./*.exe

go build

./LiberaDebt.exe -data="obligations_sample.xlsx" -income="2,000" -goal="Determine a specific prioritized strategy to payoff my loan(s) and credit card(s) as quickly & efficiently as possible without straining my monthly budget " #-model="deepseek-r1:1.5b"