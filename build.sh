#!/bin/bash

echo "========================================"
echo "Building Windows Network Manager"
echo "Cross-compiling for Windows from macOS"
echo "========================================"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed!"
    echo "Please install Go from: https://golang.org/dl/"
    exit 1
fi

echo "Installing dependencies..."
go mod tidy
if [ $? -ne 0 ]; then
    echo "ERROR: Failed to install dependencies"
    exit 1
fi

echo ""
echo "Cross-compiling for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -o WindowsNetworkManager.exe

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================"
    echo "Build successful!"
    echo "========================================"
    echo ""
    echo "Created: WindowsNetworkManager.exe"
    echo ""
    echo "Next steps:"
    echo "1. Copy WindowsNetworkManager.exe to your Windows PC"
    echo "2. Download WinDivert and copy WinDivert.dll to the same directory"
    echo "3. Run on Windows as Administrator"
    echo ""
    echo "The web interface will be available at:"
    echo "  http://localhost:18080"
    echo ""
else
    echo ""
    echo "========================================"
    echo "Build failed!"
    echo "========================================"
    exit 1
fi
