#!/bin/bash

# Remove log & data test files
rm -f ./*.log

cd ..

go run . -income="\$1,234"