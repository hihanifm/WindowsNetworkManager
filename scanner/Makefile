.PHONY: build install clean

BINARY_NAME=wnm-scanner

build:
	@echo "Building Mac scanner..."
	@go build -o $(BINARY_NAME) .
	@echo "Build complete: $(BINARY_NAME)"

install: build
	@echo "Installing to /usr/local/bin..."
	@sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "Installed! You can now run 'wnm-scanner' from anywhere."

clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@echo "Done!"

test:
	@echo "Running tests..."
	@go test ./...

help:
	@echo "Available targets:"
	@echo "  make build    - Build the scanner binary"
	@echo "  make install  - Build and install to /usr/local/bin"
	@echo "  make clean    - Remove build artifacts"
	@echo "  make test     - Run tests"
	@echo "  make help     - Show this help"
