.PHONY: build clean install-deps test

# Build for Windows from macOS
build:
	@echo "========================================"
	@echo "Building Windows Network Manager"
	@echo "Cross-compiling for Windows from macOS"
	@echo "========================================"
	@echo ""
	@echo "Installing dependencies..."
	@go mod tidy
	@echo ""
	@echo "Cross-compiling for Windows (amd64)..."
	@GOOS=windows GOARCH=amd64 go build -o WindowsNetworkManager.exe
	@echo ""
	@echo "Build successful! Created: WindowsNetworkManager.exe"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f WindowsNetworkManager.exe
	@echo "Done!"

# Install dependencies
install-deps:
	@echo "Installing Go dependencies..."
	@go mod tidy
	@go mod download
	@echo "Dependencies installed!"

# Run tests (if any)
test:
	@echo "Running tests..."
	@go test ./...

# Help
help:
	@echo "Available targets:"
	@echo "  make build        - Build Windows executable (cross-compile)"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make install-deps - Install Go dependencies"
	@echo "  make test         - Run tests"
	@echo "  make help         - Show this help message"
