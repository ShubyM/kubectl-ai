package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "scrape":
		runScrape(os.Args[2:])
	case "generate":
		runGenerate(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Gatekeeper Benchmark Tools

Usage:
  gatekeeper-bench <command> [options]

Commands:
  scrape     Fetch constraint examples from the OPA Gatekeeper library
  generate   Generate benchmark tasks from fetched constraints
  help       Show this help message

Examples:
  # Fetch all constraints from Gatekeeper library
  gatekeeper-bench scrape -output ./constraints

  # Generate predict-violation benchmarks
  gatekeeper-bench generate -input ./constraints/constraints.yaml -type predict-violation

  # Generate fix-violation benchmarks (harder)
  gatekeeper-bench generate -input ./constraints/constraints.yaml -type fix-violation -max 20

Run 'gatekeeper-bench <command> -h' for more information on a command.
`)
}

func runScrape(args []string) {
	fs := flag.NewFlagSet("scrape", flag.ExitOnError)
	outputDir := fs.String("output", "./constraints", "Output directory for fetched constraints")
	categories := fs.String("categories", "general,pod-security-policy", "Comma-separated list of categories to fetch")
	format := fs.String("format", "yaml", "Output format (yaml or json)")

	fs.Usage = func() {
		fmt.Println(`Scrape constraints from the OPA Gatekeeper library

Usage:
  gatekeeper-bench scrape [options]

Options:`)
		fs.PrintDefaults()
		fmt.Println(`
This command fetches constraint templates and examples from the
open-policy-agent/gatekeeper-library GitHub repository.

Categories available:
  - general: General-purpose policies (allowedrepos, requiredlabels, etc.)
  - pod-security-policy: Pod security policies (privileged, capabilities, etc.)
`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	scraper := &Scraper{
		OutputDir:  *outputDir,
		Categories: *categories,
		Format:     *format,
	}

	if err := scraper.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runGenerate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	inputFile := fs.String("input", "./constraints/constraints.yaml", "Input constraints file")
	outputDir := fs.String("output", "../tasks", "Output directory for benchmark tasks")
	benchType := fs.String("type", "predict-violation", "Benchmark type")
	maxTasks := fs.Int("max", 0, "Maximum number of tasks to generate (0 = unlimited)")

	fs.Usage = func() {
		fmt.Println(`Generate benchmark tasks from fetched constraints

Usage:
  gatekeeper-bench generate [options]

Options:`)
		fs.PrintDefaults()
		fmt.Println(`
Benchmark Types:
  predict-violation  - Test if model can predict whether a resource violates a constraint
  explain-violation  - Test if model can explain why a resource violates
  fix-violation      - Test if model can fix a violating resource
  audit-cluster      - Test if model can audit multiple resources

Examples:
  # Generate 20 predict-violation tasks
  gatekeeper-bench generate -type predict-violation -max 20

  # Generate all fix-violation tasks
  gatekeeper-bench generate -type fix-violation
`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	generator := &Generator{
		InputFile:     *inputFile,
		OutputDir:     *outputDir,
		BenchmarkType: BenchmarkType(*benchType),
		MaxTasks:      *maxTasks,
	}

	if err := generator.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
