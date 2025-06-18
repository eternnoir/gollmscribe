# Binary name
BINARY_NAME=gollmscribe
VERSION?=0.2.0

# Build directory
BUILD_DIR=build

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Main package path
MAIN_PACKAGE=./cmd/gollmscribe

# Build flags
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"

# Default target
.PHONY: all
all: clean build-all

# Create build directory
build-dir:
	mkdir -p $(BUILD_DIR)

# Build for all platforms
.PHONY: build-all
build-all: build-dir darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64 windows-arm64 freebsd-amd64

# macOS builds
.PHONY: darwin-amd64
darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)

.PHONY: darwin-arm64
darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

# Linux builds
.PHONY: linux-amd64
linux-amd64:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

.PHONY: linux-arm64
linux-arm64:
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)

# Windows builds
.PHONY: windows-amd64
windows-amd64:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

.PHONY: windows-arm64
windows-arm64:
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(MAIN_PACKAGE)

# FreeBSD build
.PHONY: freebsd-amd64
freebsd-amd64:
	GOOS=freebsd GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-freebsd-amd64 $(MAIN_PACKAGE)

# Build for current platform
.PHONY: build
build: build-dir
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

# Run the application
.PHONY: run
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Test
.PHONY: test
test:
	$(GOTEST) -v ./...

# Test with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -v -cover -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) ./...

# Vet code
.PHONY: vet
vet:
	$(GOVET) ./...

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run --timeout=5m

# Check code quality (format, vet, lint)
.PHONY: check
check: fmt vet lint
	@echo "All code quality checks passed!"

# Download dependencies
.PHONY: deps
deps:
	$(GOMOD) download

# Tidy dependencies
.PHONY: tidy
tidy:
	$(GOMOD) tidy

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install to GOPATH/bin
.PHONY: install
install:
	$(GOCMD) install $(LDFLAGS) $(MAIN_PACKAGE)

# Create release archives
.PHONY: release
release: build-all
	cd $(BUILD_DIR) && \
	for file in *; do \
		if [[ "$$file" == *.exe ]]; then \
			zip "$${file%.exe}.zip" "$$file"; \
		else \
			tar czf "$$file.tar.gz" "$$file"; \
		fi \
	done

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make all          - Clean and build for all platforms"
	@echo "  make build        - Build for current platform"
	@echo "  make build-all    - Build for all platforms"
	@echo "  make darwin-amd64 - Build for macOS (Intel)"
	@echo "  make darwin-arm64 - Build for macOS (Apple Silicon)"
	@echo "  make linux-amd64  - Build for Linux (x64)"
	@echo "  make linux-arm64  - Build for Linux (ARM64)"
	@echo "  make windows-amd64- Build for Windows (x64)"
	@echo "  make windows-arm64- Build for Windows (ARM64)"
	@echo "  make freebsd-amd64- Build for FreeBSD (x64)"
	@echo "  make test         - Run tests"
	@echo "  make test-coverage- Run tests with coverage"
	@echo "  make fmt          - Format code"
	@echo "  make vet          - Vet code"
	@echo "  make lint         - Run linter (requires golangci-lint)"
	@echo "  make check        - Run all code quality checks (fmt, vet, lint)"
	@echo "  make deps         - Download dependencies"
	@echo "  make tidy         - Tidy dependencies"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make install      - Install to GOPATH/bin"
	@echo "  make release      - Create release archives"
	@echo "  make help         - Show this help message"