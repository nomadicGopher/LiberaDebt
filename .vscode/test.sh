#!/bin/bash

rm -f ./*.log

cd ..

go run . -model="smollm2:135m" -data="obligations_sample.xlsx" -income="$1,234" -goal="Determine a specific strategy including priorities & amounts to payoff my obligations as quickly & efficiently as possible without straining my monthly budget. "
