.PHONY: build run clean test fmt vet lint install help

# Binary name
BINARY_NAME=goputioarr
VERSION=0.5.36

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build directory
BUILD_DIR=bin

# LDFLAGS for version injection
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd

# Run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME) run

# Run with a specific config
run-config: build
	./$(BUILD_DIR)/$(BINARY_NAME) run -c $(CONFIG)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install ./cmd

# Generate token
get-token: build
	./$(BUILD_DIR)/$(BINARY_NAME) get-token

# Generate config
generate-config: build
	./$(BUILD_DIR)/$(BINARY_NAME) generate-config

# Show version
version: build
	./$(BUILD_DIR)/$(BINARY_NAME) version

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .

# Docker run
docker-run:
	docker run -it --rm -p 9091:9091 -v $(PWD)/config:/config -v $(PWD)/downloads:/downloads $(BINARY_NAME):latest

# Help
help:
	@echo "Available targets:"
	@echo "  build           - Build the binary"
	@echo "  build-all       - Build for all platforms (linux, darwin, windows)"
	@echo "  build-linux     - Build for Linux (amd64 and arm64)"
	@echo "  build-darwin    - Build for macOS (amd64 and arm64)"
	@echo "  build-windows   - Build for Windows (amd64)"
	@echo "  run             - Build and run the proxy"
	@echo "  run-config      - Build and run with CONFIG=/path/to/config.toml"
	@echo "  clean           - Clean build artifacts"
	@echo "  test            - Run tests"
	@echo "  test-coverage   - Run tests with coverage report"
	@echo "  fmt             - Format code"
	@echo "  vet             - Run go vet"
	@echo "  deps            - Download dependencies"
	@echo "  tidy            - Tidy dependencies"
	@echo "  install         - Install the binary"
	@echo "  get-token       - Generate a Put.io API token"
	@echo "  generate-config - Generate a config file"
	@echo "  version         - Show version"
	@echo "  docker-build    - Build Docker image"
	@echo "  docker-run      - Run Docker container"
	@echo "  help            - Show this help message"
