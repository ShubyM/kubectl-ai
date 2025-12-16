.PHONY: build scrape generate clean all

# Build the gatekeeper-bench tool
build:
	GOWORK=off go build -o gatekeeper-bench .

# Scrape all constraints from Gatekeeper library
scrape: build
	./gatekeeper-bench scrape -output ./constraints -categories "general,pod-security-policy"

# Generate predict-violation benchmarks
generate-predict: build
	./gatekeeper-bench generate -input ./constraints/constraints.yaml -output ../tasks -type predict-violation

# Generate fix-violation benchmarks
generate-fix: build
	./gatekeeper-bench generate -input ./constraints/constraints.yaml -output ../tasks -type fix-violation

# Generate audit-cluster benchmarks
generate-audit: build
	./gatekeeper-bench generate -input ./constraints/constraints.yaml -output ../tasks -type audit-cluster

# Generate explain-violation benchmarks
generate-explain: build
	./gatekeeper-bench generate -input ./constraints/constraints.yaml -output ../tasks -type explain-violation

# Generate a limited set of each benchmark type for testing
generate-sample: build
	./gatekeeper-bench generate -input ./constraints/constraints.yaml -output ../tasks -type predict-violation -max 10
	./gatekeeper-bench generate -input ./constraints/constraints.yaml -output ../tasks -type fix-violation -max 5
	./gatekeeper-bench generate -input ./constraints/constraints.yaml -output ../tasks -type audit-cluster -max 5

# Generate all benchmark types
generate-all: generate-predict generate-fix generate-audit generate-explain

# Full workflow: scrape and generate
all: scrape generate-sample

# Clean generated files
clean:
	rm -f gatekeeper-bench
	rm -rf constraints/
	rm -rf ../tasks/gk-*

# Help
help:
	@echo "Gatekeeper Benchmark Generator"
	@echo ""
	@echo "Usage:"
	@echo "  make build          - Build the gatekeeper-bench tool"
	@echo "  make scrape         - Fetch constraints from Gatekeeper library"
	@echo "  make generate-predict - Generate predict-violation benchmarks"
	@echo "  make generate-fix   - Generate fix-violation benchmarks"
	@echo "  make generate-audit - Generate audit-cluster benchmarks"
	@echo "  make generate-all   - Generate all benchmark types"
	@echo "  make generate-sample - Generate a small sample set for testing"
	@echo "  make all            - Scrape and generate sample benchmarks"
	@echo "  make clean          - Remove generated files"
