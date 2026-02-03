# Go parameters
BINARY_NAME=gdocker
BINARY_UNIX=$(BINARY_NAME)_unix
INSTALL_PATH=/usr/local/bin

# Build variables
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build clean install uninstall run test help

all: build ## Build the binary

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "Build complete: $(BINARY_NAME)"

install: build ## Build and install to system path
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo install -m 755 $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete! You can now run 'gdocker' from anywhere."

uninstall: ## Remove from system path
	@echo "Uninstalling $(BINARY_NAME) from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstall complete."

clean: ## Remove build artifacts
	@echo "Cleaning..."
	@go clean
	@rm -f $(BINARY_NAME)
	@rm -f $(BINARY_UNIX)
	@echo "Clean complete."

run: build ## Build and run
	./$(BINARY_NAME)

test: ## Run tests
	go test -v ./...

fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy dependencies
	go mod tidy

check: fmt vet ## Run format and vet

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# Cross compilation targets
build-linux: ## Build for Linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_UNIX) .

build-all: ## Build for all platforms
	@echo "Building for multiple platforms..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)_darwin_amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)_darwin_arm64 .
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)_linux_amd64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)_windows_amd64.exe .
	@echo "Cross-compilation complete!"
