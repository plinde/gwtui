BINARY := gwtui
BUILD_DIR := bin
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: all build install clean test test-v cover

all: build ## Build the default artifact

build: ## Build the gwt binary
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd

install: build ## Install gwt to ~/.local/bin
	cp $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
ifeq ($(shell uname -s),Darwin)
	codesign --force --sign - $(INSTALL_DIR)/$(BINARY)
endif

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

test: ## Run all tests
	go test ./...

test-v: ## Run tests with verbose output
	go test -v ./...

cover: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
