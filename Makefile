BINARY := gwtui
BUILD_DIR := bin
INSTALL_DIR := $(HOME)/bin

.PHONY: build install clean

build: ## Build the gwt binary
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd

install: build ## Install gwt to ~/bin
	cp $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
