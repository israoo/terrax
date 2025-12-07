.PHONY: help install build run clean test fmt vet lint tidy dev release

# Variables
BINARY_NAME=terrax
MAIN_PACKAGE=.
BUILD_DIR=./build
DIST_DIR=./dist

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build flags
LDFLAGS=-ldflags "-s -w"
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

help: ## Show this help
	@echo ""
	@echo "═══════════════════════════════════════════════════════════════"
	@echo "  🌍 Terrax - Terra eXecutor"
	@echo "═══════════════════════════════════════════════════════════════"
	@echo ""
	@echo "📦 INSTALLATION & SETUP:"
	@grep -E '^(install|init|upgrade):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "🔨 BUILD & COMPILATION:"
	@grep -E '^(build|build-all|release):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "🚀 RUN & DEVELOPMENT:"
	@grep -E '^(run|dev):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "🧪 TESTING & COVERAGE:"
	@grep -E '^(test|test-coverage):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "🔍 CODE QUALITY:"
	@grep -E '^(fmt|vet|lint|check):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "🧹 MAINTENANCE:"
	@grep -E '^(clean|tidy):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "📋 INFORMATION:"
	@grep -E '^(info|help):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "═══════════════════════════════════════════════════════════════"
	@echo ""

build: ## Build the binary
	@echo "🔨 Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "✅ Binary built: ./$(BINARY_NAME)"

build-all: ## Build for multiple platforms
	@echo "🔨 Building for multiple platforms..."
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)
	@echo "✅ Binaries built in $(DIST_DIR)/"

dev: ## Run in development mode (with hot reload using air if available)
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "⚠️  'air' is not installed. Running normally..."; \
		echo "💡 Install air with: go install github.com/air-verse/air@latest"; \
		$(MAKE) run; \
	fi

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "✅ All checks passed"

clean: ## Clean generated files
	@echo "🧹 Cleaning files..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -f coverage.out
	@echo "✅ Files cleaned"

fmt: ## Format code
	@echo "🎨 Formatting code..."
	$(GOFMT) ./...
	@echo "✅ Code formatted"

info: ## Show project information
	@echo "📋 Project Information"
	@echo "  Name:      $(BINARY_NAME)"
	@echo "  Version:   $(VERSION)"
	@echo "  Go:        $(shell $(GOCMD) version)"
	@echo "  Module:    $(shell head -n1 go.mod | cut -d' ' -f2)"
	@echo "  Build:     $(BUILD_TIME)"

init: ## Initialize project (install + build)
	@echo "🚀 Initializing project..."
	$(MAKE) install
	$(MAKE) build
	@echo "✅ Project initialized"

install: ## Install dependencies
	@echo "📦 Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✅ Dependencies installed"

lint: ## Run golangci-lint
	@echo "🔍 Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "⚠️  golangci-lint is not installed"; \
		echo "💡 Install with: brew install golangci-lint"; \
	fi

release: clean test build-all ## Create release (clean + test + build-all)
	@echo "🎉 Release completed"
	@ls -lh $(DIST_DIR)/

run: build ## Build and run
	@echo "🚀 Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

test: ## Run tests
	@echo "🧪 Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "✅ Tests completed"

test-coverage: test ## Run tests and show coverage
	@echo "📊 Showing coverage..."
	$(GOCMD) tool cover -html=coverage.out

tidy: ## Clean and update dependencies
	@echo "🧹 Cleaning dependencies..."
	$(GOMOD) tidy
	@echo "✅ Dependencies updated"

upgrade: ## Upgrade all dependencies
	@echo "⬆️  Upgrading dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy
	@echo "✅ Dependencies upgraded"

vet: ## Run go vet
	@echo "🔍 Running go vet..."
	$(GOVET) ./...
	@echo "✅ go vet completed"
