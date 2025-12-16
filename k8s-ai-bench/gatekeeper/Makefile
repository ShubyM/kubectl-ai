# Gatekeeper Task Generator Makefile
#
# This Makefile provides commands for generating k8s-ai-bench tasks
# from the OPA Gatekeeper library.

.PHONY: all generate clean dry-run update help

# Default output directory for generated tasks
OUTPUT_DIR ?= ./tasks

# Gatekeeper library clone location
REPO_DIR ?= .gatekeeper-library

all: generate

# Generate tasks from the Gatekeeper library
generate:
	@echo "Generating Gatekeeper compliance tasks..."
	go run . --output-dir $(OUTPUT_DIR)
	@echo "Done! Tasks generated in $(OUTPUT_DIR)"

# Preview what would be generated without creating files
dry-run:
	@echo "Dry run - showing what would be generated..."
	go run . --output-dir $(OUTPUT_DIR) --dry-run

# Clean generated tasks and regenerate
regenerate: clean generate

# Remove all generated tasks
clean:
	@echo "Cleaning generated tasks..."
	go run . --output-dir $(OUTPUT_DIR) --clean --skip-clone
	@echo "Done!"

# Update the Gatekeeper library and regenerate tasks
update:
	@echo "Updating Gatekeeper library and regenerating tasks..."
	rm -rf $(REPO_DIR)
	go run . --output-dir $(OUTPUT_DIR) --clean
	@echo "Done!"

# Remove the cloned Gatekeeper library
clean-repo:
	@echo "Removing Gatekeeper library clone..."
	rm -rf $(REPO_DIR)

# Build the generator binary
build:
	go build -o gatekeeper-gen .

# Run tests
test:
	go test -v ./...

help:
	@echo "Gatekeeper Task Generator"
	@echo ""
	@echo "Usage:"
	@echo "  make generate    - Generate tasks from Gatekeeper library"
	@echo "  make dry-run     - Preview what would be generated"
	@echo "  make regenerate  - Clean and regenerate all tasks"
	@echo "  make clean       - Remove all generated tasks"
	@echo "  make update      - Update Gatekeeper library and regenerate"
	@echo "  make clean-repo  - Remove the cloned Gatekeeper library"
	@echo "  make build       - Build the generator binary"
	@echo ""
	@echo "Options:"
	@echo "  OUTPUT_DIR=path  - Set output directory (default: ./tasks)"
	@echo "  REPO_DIR=path    - Set Gatekeeper library location (default: .gatekeeper-library)"
