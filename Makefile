.PHONY: build clean install-deps test release

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

# Run tests (sched has tests; root package is Windows-only and breaks go test on non-Windows hosts)
test:
	@echo "Running tests..."
	@go test ./sched/...

# Build release package (EXE + DLL + ZIP)
release:
	@./build_release.sh $(VERSION)

# Help
help:
	@echo "Available targets:"
	@echo "  make build        - Build Windows executable (cross-compile)"
	@echo "  make release      - Build release package with EXE, DLL, and ZIP (VERSION=2.0.0)"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make install-deps - Install Go dependencies"
	@echo "  make test         - Run tests"
	@echo "  make help         - Show this help message"
