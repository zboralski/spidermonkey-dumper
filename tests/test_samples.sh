#!/bin/bash

# Test script for dedicated test samples
set -e

echo "Testing dedicated test samples in samples/testdata/"

samples=(
    "simple.jsc"
    "functions.jsc" 
    "minimal.jsc"
    "nested.jsc"
    "constants.jsc"
)

for sample in "${samples[@]}"; do
    echo "Testing $sample..."
    ./dumper "samples/testdata/$sample" >/dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "✓ $sample - OK"
    else
        echo "✗ $sample - FAILED"
        exit 1
    fi
done

echo "All test samples passed!"