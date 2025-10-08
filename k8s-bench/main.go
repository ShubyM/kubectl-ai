// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/k8s-bench/pkg/model"
	"sigs.k8s.io/yaml"
)

type Task struct {
	Setup      string `json:"setup,omitempty"`
	Verifier   string `json:"verifier,omitempty"`
	Cleanup    string `json:"cleanup,omitempty"`
	Difficulty string `json:"difficulty"`
	Disabled   bool   `json:"disabled,omitempty"`

	Expect []Expectation `json:"expect,omitempty"`

	Script []ScriptStep `json:"script,omitempty"`

	// Isolation can be set to automatically create an isolated cluster
	// TODO: support namespaces also
	Isolation IsolationMode `json:"isolation,omitempty"`
}

type IsolationMode string

const (
	// IsolationModeCluster will create a cluster for the task evaluation.
	IsolationModeCluster IsolationMode = "cluster"
)

type ScriptStep struct {
	Prompt     string `json:"prompt"`
	PromptFile string `json:"promptFile"`
}

// ResolvePrompt resolves the prompt from either inline or file source
func (s *ScriptStep) ResolvePrompt(baseDir string) (string, error) {
	// Fail if both prompt and promptFile are provided to avoid confusion
	if s.Prompt != "" && s.PromptFile != "" {
		return "", fmt.Errorf("both 'prompt' and 'promptFile' are specified in script step; only one should be provided")
	}

	// If promptFile is provided, read the file
	if s.PromptFile != "" {
		// If the path is relative, resolve it relative to the task directory
		promptPath := s.PromptFile
		if !filepath.IsAbs(promptPath) {
			promptPath = filepath.Join(baseDir, s.PromptFile)
		}

		content, err := os.ReadFile(promptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read prompt file %q: %w", promptPath, err)
		}

		return string(content), nil
	}

	// If prompt is provided, use it
	if s.Prompt != "" {
		return s.Prompt, nil
	}

	// If neither is provided, return an error
	return "", fmt.Errorf("neither 'prompt' nor 'promptFile' is specified in script step")
}

type Expectation struct {
	Contains string `json:"contains,omitempty"`
}

type EvalConfig struct {
	LLMConfigs        []model.LLMConfig
	KubeConfig        string
	TasksDir          string
	TaskPattern       string
	AgentBin          string
	AgentID           string
	AgentArgTemplates []AgentArgTemplate
	Concurrency       int
	CreateKindCluster bool

	OutputDir string
}

type AgentArgTemplate struct {
	Raw      string
	Template *template.Template
}

type AnalyzeConfig struct {
	InputDir          string
	OutputFormat      string
	IgnoreToolUseShim bool
	ShowFailures      bool
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Clean(os.ExpandEnv(path)), nil
}

func main() {
	// Print top-level usage if help is requested directly
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printUsage()
		return
	}

	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// Define custom usage text to show subcommands
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  run       Run evaluation benchmarks\n")
	fmt.Fprintf(os.Stderr, "  analyze   Analyze results from previous benchmark runs\n\n")
	fmt.Fprintf(os.Stderr, "Run '%s <command> --help' for more information on a command.\n", os.Args[0])
}

type Strings []string

func (f *Strings) String() string {
	return strings.Join(*f, ",")
}

func (f *Strings) Set(s string) error {
	*f = append(*f, s)
	return nil
}

func run(ctx context.Context) error {
	// No need to check for help flags here anymore

	// Default to "run" subcommand if no arguments provided
	subCommand := "run"
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		subCommand = os.Args[1]
		// Shift the arguments
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}

	switch subCommand {
	case "run":
		return runEvals(ctx)
	case "analyze":
		return runAnalyze()
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand: %s, valid options are 'run' or 'analyze'", subCommand)
	}
}

func runEvals(ctx context.Context) error {
	start := time.Now()
	config := EvalConfig{
		TasksDir: "./tasks",
	}

	// Set custom usage for 'run' subcommand
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s run [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Run K8s-bench evaluation benchmarks.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	// Derive kubeconfig from environment as a default; fall back to the standard path.
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if kubeconfigEnv != "" {
		paths := strings.Split(kubeconfigEnv, string(os.PathListSeparator))
		config.KubeConfig = paths[0]
	} else {
		config.KubeConfig = "~/.kube/config"
	}

	var agentArgTemplates Strings

	flag.StringVar(&config.TasksDir, "tasks-dir", config.TasksDir, "Directory containing evaluation tasks")
	flag.StringVar(&config.TaskPattern, "task-pattern", config.TaskPattern, "Pattern to filter tasks (e.g. 'pod' or 'redis')")
	flag.StringVar(&config.AgentBin, "agent-bin", config.AgentBin, "Path to the agent executable to benchmark")
	flag.IntVar(&config.Concurrency, "concurrency", 0, "Number of tasks to run concurrently (0 = auto, 1 = sequential)")
	flag.BoolVar(&config.CreateKindCluster, "create-kind-cluster", false, "Create a temporary kind cluster for the evaluation run")
	flag.StringVar(&config.OutputDir, "output-dir", config.OutputDir, "Directory to write results to")
	flag.Var(&agentArgTemplates, "agent-args", "Additional arguments for the agent (repeatable). Templates may reference: AgentID, Kubeconfig, TracePath.")
	flag.Parse()

	expandedKubeconfig, err := expandPath(config.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to expand kubeconfig path %q: %w", config.KubeConfig, err)
	}
	config.KubeConfig = expandedKubeconfig

	if config.AgentBin == "" {
		return fmt.Errorf("--agent-bin is required")
	}

	agentID := strings.TrimSuffix(filepath.Base(config.AgentBin), filepath.Ext(config.AgentBin))
	if agentID == "" {
		agentID = "agent"
	}
	config.AgentID = agentID
	config.LLMConfigs = append(config.LLMConfigs, model.LLMConfig{
		ID:      agentID,
		AgentID: agentID,
	})

	for _, tmplStr := range agentArgTemplates {
		tmpl, err := template.New("agent-arg").Option("missingkey=error").Parse(tmplStr)
		if err != nil {
			return fmt.Errorf("failed to parse agent arg template %q: %w", tmplStr, err)
		}
		config.AgentArgTemplates = append(config.AgentArgTemplates, AgentArgTemplate{Raw: tmplStr, Template: tmpl})
	}

	tasks, err := loadTasks(config)
	if err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}

	// If concurrency is set to auto (0), use the number of tasks
	if config.Concurrency == 0 {
		config.Concurrency = len(tasks)
		fmt.Printf("Auto-configuring concurrency to %d (number of tasks)\n", config.Concurrency)
	}

	if err := runEvaluation(ctx, config); err != nil {
		return fmt.Errorf("running evaluation: %w", err)
	}

	fmt.Printf("Total evaluation time: %s\n", time.Since(start))
	return nil
}

func runAnalyze() error {
	config := AnalyzeConfig{
		InputDir:     "",
		OutputFormat: "markdown",
	}

	// Set custom usage for 'analyze' subcommand
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s analyze --input-dir <directory> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Analyze results from previous K8s-bench runs.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	var resultsFilePath string
	flag.StringVar(&config.InputDir, "input-dir", config.InputDir, "Directory containing evaluation results (required)")
	flag.StringVar(&config.OutputFormat, "output-format", config.OutputFormat, "Output format (markdown or json)")
	flag.BoolVar(&config.IgnoreToolUseShim, "ignore-tool-use-shim", true, "Ignore tool use shim")
	flag.BoolVar(&config.ShowFailures, "show-failures", false, "Show failure details in markdown output")
	flag.StringVar(&resultsFilePath, "results-filepath", "", "Optional file path to write results to")
	flag.Parse()

	// Check if input-dir is provided
	if config.InputDir == "" {
		flag.Usage()
		return fmt.Errorf("--input-dir is required")
	}

	// Check if output format is valid
	if config.OutputFormat != "markdown" && config.OutputFormat != "json" {
		return fmt.Errorf("invalid output format: %s, valid options are 'markdown' or 'json'", config.OutputFormat)
	}

	// Check if input directory exists
	if _, err := os.Stat(config.InputDir); os.IsNotExist(err) {
		return fmt.Errorf("input directory does not exist: %s", config.InputDir)
	}

	allResults, err := collectResults(config.InputDir)
	if err != nil {
		return fmt.Errorf("collecting results: %w", err)
	}

	// Format and output results
	if config.OutputFormat == "markdown" {
		if err := printMarkdownResults(config, allResults, resultsFilePath); err != nil {
			return fmt.Errorf("printing markdown results: %w", err)
		}
	} else {
		if err := printJSONResults(allResults, resultsFilePath); err != nil {
			return fmt.Errorf("printing JSON results: %w", err)
		}
	}

	return nil
}

func collectResults(inputDir string) ([]model.TaskResult, error) {
	var allResults []model.TaskResult

	// Walk through the directory structure to find all results.yaml files
	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process results.yaml files
		if !info.IsDir() && info.Name() == "results.yaml" {
			// Read and parse the results file
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading file %s: %w", path, err)
			}

			var result model.TaskResult
			if err := yaml.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parsing yaml from %s: %w", path, err)
			}

			allResults = append(allResults, result)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return allResults, nil
}

func normalizedAgentID(agentID string) string {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return "unknown"
	}
	return agentID
}

func normalizedModelID(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return "gemini"
	}
	return modelID
}

func agentModelLabel(agentID, modelID string) string {
	return fmt.Sprintf("%s (%s)", normalizedAgentID(agentID), normalizedModelID(modelID))
}

func normalizedProviderID(providerID string) string {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return "unspecified"
	}
	return providerID
}

func printFailureDetails(buffer *strings.Builder, results []model.TaskResult, title string, showModel bool) {
	hasFailures := false
	for _, result := range results {
		if len(result.Failures) > 0 {
			if !hasFailures {
				buffer.WriteString(fmt.Sprintf("\n**%s Failure Details**\n\n", title))
				hasFailures = true
			}
			agentLabel := agentModelLabel(result.LLMConfig.AgentID, result.LLMConfig.ModelID)
			shimLabel := "tool-use disabled"
			if result.LLMConfig.EnableToolUseShim {
				shimLabel = "tool-use enabled"
			}
			buffer.WriteString(fmt.Sprintf("**Task: %s (%s, %s)**\n", result.Task, agentLabel, shimLabel))
			for _, failure := range result.Failures {
				buffer.WriteString(fmt.Sprintf("```\n%s\n```\n", failure.Message))
			}
			buffer.WriteString("\n")
		}
	}
}

func printMarkdownResults(config AnalyzeConfig, results []model.TaskResult, resultsFilePath string) error {
	// Create a buffer to hold the output
	var buffer strings.Builder

	buffer.WriteString("# K8s-bench Evaluation Results\n\n")

	type summaryCounts struct {
		success int
		fail    int
	}

	totalCount := len(results)
	overallSuccessCount := 0
	overallFailCount := 0

	agentModelSet := make(map[string]map[string]struct{})
	summaryByAgent := make(map[string]map[string]*summaryCounts)
	toolUseSummaryByAgent := make(map[string]map[string]map[string]*summaryCounts)
	toolUseKeysSet := make(map[string]struct{})
	resultsByAgentModel := make(map[string]map[string][]model.TaskResult)
	resultsByToolUseShim := make(map[string][]model.TaskResult)

	for _, result := range results {
		agentID := normalizedAgentID(result.LLMConfig.AgentID)
		modelID := normalizedModelID(result.LLMConfig.ModelID)

		if _, ok := agentModelSet[agentID]; !ok {
			agentModelSet[agentID] = make(map[string]struct{})
		}
		agentModelSet[agentID][modelID] = struct{}{}

		if _, ok := summaryByAgent[agentID]; !ok {
			summaryByAgent[agentID] = make(map[string]*summaryCounts)
		}
		if _, ok := summaryByAgent[agentID][modelID]; !ok {
			summaryByAgent[agentID][modelID] = &summaryCounts{}
		}

		shimKey := "shim_disabled"
		if result.LLMConfig.EnableToolUseShim {
			shimKey = "shim_enabled"
		}
		toolUseKeysSet[shimKey] = struct{}{}
		resultsByToolUseShim[shimKey] = append(resultsByToolUseShim[shimKey], result)

		if _, ok := toolUseSummaryByAgent[agentID]; !ok {
			toolUseSummaryByAgent[agentID] = make(map[string]map[string]*summaryCounts)
		}
		if _, ok := toolUseSummaryByAgent[agentID][modelID]; !ok {
			toolUseSummaryByAgent[agentID][modelID] = make(map[string]*summaryCounts)
		}
		if _, ok := toolUseSummaryByAgent[agentID][modelID][shimKey]; !ok {
			toolUseSummaryByAgent[agentID][modelID][shimKey] = &summaryCounts{}
		}

		if strings.Contains(strings.ToLower(result.Result), "success") {
			summaryByAgent[agentID][modelID].success++
			toolUseSummaryByAgent[agentID][modelID][shimKey].success++
			overallSuccessCount++
		} else {
			summaryByAgent[agentID][modelID].fail++
			toolUseSummaryByAgent[agentID][modelID][shimKey].fail++
			overallFailCount++
		}

		if _, ok := resultsByAgentModel[agentID]; !ok {
			resultsByAgentModel[agentID] = make(map[string][]model.TaskResult)
		}
		resultsByAgentModel[agentID][modelID] = append(resultsByAgentModel[agentID][modelID], result)
	}

	agents := make([]string, 0, len(agentModelSet))
	for agentID := range agentModelSet {
		agents = append(agents, agentID)
	}
	sort.Strings(agents)

	toolUseKeys := make([]string, 0, len(toolUseKeysSet))
	for key := range toolUseKeysSet {
		toolUseKeys = append(toolUseKeys, key)
	}
	sort.Strings(toolUseKeys)

	// --- Performance Summary ---
	buffer.WriteString("## Model Performance Summary\n\n")

	if config.IgnoreToolUseShim {
		buffer.WriteString("| Agent | Success | Fail |\n")
		buffer.WriteString("|-------|---------|------|\n")

		for _, agentID := range agents {
			modelSet := agentModelSet[agentID]
			models := make([]string, 0, len(modelSet))
			for modelID := range modelSet {
				models = append(models, modelID)
			}
			sort.Strings(models)

			for _, modelID := range models {
				counts := summaryByAgent[agentID][modelID]
				buffer.WriteString(fmt.Sprintf("| %s | %d | %d |\n", agentModelLabel(agentID, modelID), counts.success, counts.fail))
			}
		}

		buffer.WriteString(fmt.Sprintf("| **Total** | %d | %d |\n\n", overallSuccessCount, overallFailCount))
	} else {
		buffer.WriteString("| Agent |")
		for _, shimKey := range toolUseKeys {
			buffer.WriteString(fmt.Sprintf(" %s Success | %s Fail |", shimKey, shimKey))
		}
		buffer.WriteString("\n|-------|")
		for range toolUseKeys {
			buffer.WriteString("---------------|-----------|")
		}
		buffer.WriteString("\n")

		for _, agentID := range agents {
			modelSet := agentModelSet[agentID]
			models := make([]string, 0, len(modelSet))
			for modelID := range modelSet {
				models = append(models, modelID)
			}
			sort.Strings(models)

			for _, modelID := range models {
				buffer.WriteString(fmt.Sprintf("| %s |", agentModelLabel(agentID, modelID)))
				for _, shimKey := range toolUseKeys {
					counts := toolUseSummaryByAgent[agentID][modelID][shimKey]
					successCount := 0
					failCount := 0
					if counts != nil {
						successCount = counts.success
						failCount = counts.fail
					}
					buffer.WriteString(fmt.Sprintf(" %d | %d |", successCount, failCount))
				}
				buffer.WriteString("\n")
			}
		}

		buffer.WriteString("| **Total** |")
		for _, shimKey := range toolUseKeys {
			totalSuccess := 0
			totalFail := 0
			for _, agentID := range agents {
				for modelID := range agentModelSet[agentID] {
					counts := toolUseSummaryByAgent[agentID][modelID][shimKey]
					if counts != nil {
						totalSuccess += counts.success
						totalFail += counts.fail
					}
				}
			}
			buffer.WriteString(fmt.Sprintf(" %d | %d |", totalSuccess, totalFail))
		}
		buffer.WriteString("\n\n")
	}

	// --- Overall Summary ---
	buffer.WriteString("## Overall Summary\n\n")
	buffer.WriteString(fmt.Sprintf("- Total Runs: %d\n", totalCount))
	buffer.WriteString(fmt.Sprintf("- Overall Success: %d (%d%%)\n", overallSuccessCount, calculatePercentage(overallSuccessCount, totalCount)))
	buffer.WriteString(fmt.Sprintf("- Overall Fail: %d (%d%%)\n\n", overallFailCount, calculatePercentage(overallFailCount, totalCount)))

	// --- Detailed Results ---
	if config.IgnoreToolUseShim {
		for _, agentID := range agents {
			buffer.WriteString(fmt.Sprintf("## Agent: %s\n\n", normalizedAgentID(agentID)))
			modelSet := agentModelSet[agentID]
			models := make([]string, 0, len(modelSet))
			for modelID := range modelSet {
				models = append(models, modelID)
			}
			sort.Strings(models)

			for _, modelID := range models {
				configurationLabel := agentModelLabel(agentID, modelID)
				buffer.WriteString(fmt.Sprintf("### Configuration: %s\n\n", configurationLabel))
				buffer.WriteString("| Task | Provider | Tool Use Shim | Result |\n")
				buffer.WriteString("|------|----------|---------------|--------|\n")

				modelResults := append([]model.TaskResult(nil), resultsByAgentModel[agentID][modelID]...)
				sort.Slice(modelResults, func(i, j int) bool {
					if modelResults[i].Task != modelResults[j].Task {
						return modelResults[i].Task < modelResults[j].Task
					}
					return modelResults[i].LLMConfig.EnableToolUseShim && !modelResults[j].LLMConfig.EnableToolUseShim
				})

				modelSuccessCount := 0
				modelFailCount := 0
				modelTotalCount := len(modelResults)

				for _, result := range modelResults {
					resultEmoji := "❌"
					if strings.Contains(strings.ToLower(result.Result), "success") {
						resultEmoji = "✅"
						modelSuccessCount++
					} else {
						modelFailCount++
					}

					shimLabel := "disabled"
					if result.LLMConfig.EnableToolUseShim {
						shimLabel = "enabled"
					}

					buffer.WriteString(fmt.Sprintf("| %s | %s | %s | %s %s |\n",
						result.Task,
						normalizedProviderID(result.LLMConfig.ProviderID),
						shimLabel,
						resultEmoji, result.Result))
				}

				buffer.WriteString(fmt.Sprintf("\n**%s Summary**\n\n", configurationLabel))
				buffer.WriteString(fmt.Sprintf("- Total: %d\n", modelTotalCount))
				buffer.WriteString(fmt.Sprintf("- Success: %d (%d%%)\n", modelSuccessCount, calculatePercentage(modelSuccessCount, modelTotalCount)))
				buffer.WriteString(fmt.Sprintf("- Fail: %d (%d%%)\n\n", modelFailCount, calculatePercentage(modelFailCount, modelTotalCount)))
				if config.ShowFailures {
					printFailureDetails(&buffer, modelResults, configurationLabel, true)
				}
			}
		}
	} else {
		toolUseShimStrs := make([]string, 0, len(resultsByToolUseShim))
		for toolUseShimStr := range resultsByToolUseShim {
			toolUseShimStrs = append(toolUseShimStrs, toolUseShimStr)
		}
		sort.Strings(toolUseShimStrs)

		for _, toolUseShimStr := range toolUseShimStrs {
			toolUseShimStrResults := append([]model.TaskResult(nil), resultsByToolUseShim[toolUseShimStr]...)
			buffer.WriteString(fmt.Sprintf("## Tool Use: %s\n\n", toolUseShimStr))
			buffer.WriteString("| Task | Agent | Provider | Result |\n")
			buffer.WriteString("|------|-------|----------|--------|\n")

			successCount := 0
			failCount := 0
			totalCount := len(toolUseShimStrResults)

			sort.Slice(toolUseShimStrResults, func(i, j int) bool {
				agentI := agentModelLabel(toolUseShimStrResults[i].LLMConfig.AgentID, toolUseShimStrResults[i].LLMConfig.ModelID)
				agentJ := agentModelLabel(toolUseShimStrResults[j].LLMConfig.AgentID, toolUseShimStrResults[j].LLMConfig.ModelID)
				if agentI != agentJ {
					return agentI < agentJ
				}
				return toolUseShimStrResults[i].Task < toolUseShimStrResults[j].Task
			})

			for _, result := range toolUseShimStrResults {
				resultEmoji := "❌"
				if strings.Contains(strings.ToLower(result.Result), "success") {
					resultEmoji = "✅"
					successCount++
				} else {
					failCount++
				}

				agentLabel := agentModelLabel(result.LLMConfig.AgentID, result.LLMConfig.ModelID)
				buffer.WriteString(fmt.Sprintf("| %s | %s | %s | %s %s |\n",
					result.Task,
					agentLabel,
					normalizedProviderID(result.LLMConfig.ProviderID),
					resultEmoji, result.Result))
			}

			buffer.WriteString(fmt.Sprintf("\n**%s Summary**\n\n", toolUseShimStr))
			buffer.WriteString(fmt.Sprintf("- Total: %d\n", totalCount))
			buffer.WriteString(fmt.Sprintf("- Success: %d (%d%%)\n", successCount, calculatePercentage(successCount, totalCount)))
			buffer.WriteString(fmt.Sprintf("- Fail: %d (%d%%)\n\n", failCount, calculatePercentage(failCount, totalCount)))
			if config.ShowFailures {
				printFailureDetails(&buffer, toolUseShimStrResults, toolUseShimStr, true)
			}
		}
	}

	// --- Footer ---
	buffer.WriteString("---\n\n")
	buffer.WriteString(fmt.Sprintf("_Report generated on %s_\n", time.Now().Format("January 2, 2006 at 3:04 PM")))

	// Get the final output
	output := buffer.String()

	// Write to file if path is provided, otherwise print to stdout
	if resultsFilePath != "" {
		if err := os.WriteFile(resultsFilePath, []byte(output), 0644); err != nil {
			return fmt.Errorf("writing to file %q: %w", resultsFilePath, err)
		}
		fmt.Printf("Results written to %s\n", resultsFilePath)
	} else {
		// Print to stdout only if no file path is specified
		fmt.Print(output)
	}

	return nil
}

func calculatePercentage(part, total int) int {
	if total == 0 {
		return 0
	}
	return int((float64(part) / float64(total)) * 100)
}

func printJSONResults(results []model.TaskResult, resultsFilePath string) error {
	// Convert the results to JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling results to JSON: %w", err)
	}

	// Write to file if path is provided, otherwise print to stdout
	if resultsFilePath != "" {
		if err := os.WriteFile(resultsFilePath, jsonData, 0644); err != nil {
			return fmt.Errorf("writing to file %q: %w", resultsFilePath, err)
		}
		fmt.Printf("Results written to %s\n", resultsFilePath)
	} else {
		// Print to stdout only if no file path is specified
		fmt.Println(string(jsonData))
	}

	return nil
}
