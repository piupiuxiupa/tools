.PHONY: build test clean build-all build-linux build-linux-arm64 build-linux-amd64 build-darwin build-darwin-amd64 build-windows

# Variables
BINARY_NAME=doris-export
BINARY_PATH=bin/$(BINARY_NAME)
CMD_PATH=./cmd/doris-export

# Build for current architecture
build:
	@echo "Building $(BINARY_NAME) for current architecture..."
	@mkdir -p bin
	go build -o $(BINARY_PATH) $(CMD_PATH)
	@echo "Build complete: $(BINARY_PATH)"
	@file $(BINARY_PATH)

# Build for all common architectures
build-all: build-linux build-darwin build-windows
	@echo "All builds complete!"

# Linux ARM64 (适合 ARM 服务器，如 AWS Graviton)
build-linux-arm64:
	@echo "Building for Linux ARM64..."
	@mkdir -p bin
	GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)
	@echo "Build complete: bin/$(BINARY_NAME)-linux-arm64"

# Linux AMD64 (适合 x86_64 服务器)
build-linux-amd64:
	@echo "Building for Linux AMD64..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	@echo "Build complete: bin/$(BINARY_NAME)-linux-amd64"

# macOS AMD64 (适合 Intel Mac)
build-darwin-amd64:
	@echo "Building for macOS AMD64..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	@echo "Build complete: bin/$(BINARY_NAME)-darwin-amd64"

# Windows AMD64
build-windows:
	@echo "Building for Windows AMD64..."
	@mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)
	@echo "Build complete: bin/$(BINARY_NAME)-windows-amd64.exe"

# Run all tests
test:
	@echo "Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	go clean

# Run go mod tidy
tidy:
	@echo "Running go mod tidy..."
	go mod tidy

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Default target
all: build
