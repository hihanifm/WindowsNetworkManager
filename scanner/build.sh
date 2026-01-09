#!/bin/bash

echo "========================================"
echo "Building Windows Network Manager Scanner"
echo "========================================"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed!"
    echo "Please install Go from: https://golang.org/dl/"
    exit 1
fi

echo "Building scanner..."
go build -o wnm-scanner .

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================"
    echo "Build successful!"
    echo "========================================"
    echo ""
    echo "Created: wnm-scanner"
    echo ""
    echo "Usage:"
    echo "  ./wnm-scanner scan          - Scan network for instances"
    echo "  ./wnm-scanner scan -json    - Output in JSON format"
    echo "  ./wnm-scanner open <ip>    - Open instance in browser"
    echo ""
else
    echo ""
    echo "========================================"
    echo "Build failed!"
    echo "========================================"
    exit 1
fi
