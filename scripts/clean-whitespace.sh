#!/bin/bash

# Clean trailing whitespace from all Go files
echo "Cleaning trailing whitespace from Go files..."

# Find all .go files and remove trailing whitespace
find . -name "*.go" -type f -exec sed -i '' 's/[[:space:]]*$//' {} \;

# Clean other source files
find . -name "*.yaml" -o -name "*.yml" -o -name "*.json" -o -name "*.md" -o -name "*.txt" | xargs sed -i '' 's/[[:space:]]*$//'

echo "Whitespace cleanup completed."