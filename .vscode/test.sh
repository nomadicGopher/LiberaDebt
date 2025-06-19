#!/bin/bash

rm -f ./*.log

cd ..

go run . -model="smollm:135m" #-income="\$1,234" -goal="Pay off debt as quickly and efficiently as possible while not straining my monthly budget. "
