.PHONY: build build-cli build-bot clean run-cli run-bot test help

# Binary names
CLI_BINARY=bin/netblocks-cli
BOT_BINARY=bin/netblocks-bot

# Build flags
LDFLAGS=-ldflags "-s -w"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: build-cli build-bot ## Build all binaries

build-cli: ## Build CLI binary
	@echo "Building CLI..."
	@mkdir -p bin
	@go build $(LDFLAGS) -o $(CLI_BINARY) ./cmd/cli
	@echo "Built: $(CLI_BINARY)"

build-bot: ## Build Telegram bot binary
	@echo "Building Telegram bot..."
	@mkdir -p bin
	@go build $(LDFLAGS) -o $(BOT_BINARY) ./cmd/telegram-bot
	@echo "Built: $(BOT_BINARY)"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@echo "Cleaned"

run-cli: build-cli ## Build and run CLI
	@./$(CLI_BINARY)

run-bot: build-bot ## Build and run Telegram bot
	@./$(BOT_BINARY)

test: ## Run tests
	@go test -v ./...

fmt: ## Format code
	@go fmt ./...

vet: ## Run go vet
	@go vet ./...

lint: fmt vet ## Run linters

install-deps: ## Install dependencies
	@go mod download
	@go mod tidy

