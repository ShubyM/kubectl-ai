package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GoogleCloudPlatform/kubectl-ai/k8s-bench/pkg/model"
)

type RunSummary struct {
	ID            string          `json:"id"`
	Agent         string          `json:"agent"`
	Model         string          `json:"model"`
	ModelProvider string          `json:"model_provider"`
	Total         int             `json:"total"`
	NumSuccess    int             `json:"num_success"`
	NumFailed     int             `json:"num_failed"`
	NumError      int             `json:"num_error"`
	Percentage    float64         `json:"percentage"`
	Tasks         []TaskBreakdown `json:"tasks"`
}

type TaskBreakdown struct {
	Name     string          `json:"name"`
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	Result   string          `json:"result"`
	Failures []model.Failure `json:"failures,omitempty"`
	Error    string          `json:"error,omitempty"`
}

func aggregateResults(rootDir string) ([]RunSummary, error) {
	allResults, err := collectResults(rootDir)
	if err != nil {
		return nil, fmt.Errorf("collecting results from %q: %w", rootDir, err)
	}
	if len(allResults) == 0 {
		return nil, nil
	}

	type summaryKey struct {
		agent    string
		modelID  string
		provider string
	}

	groups := make(map[summaryKey][]model.TaskResult)
	var orderedKeys []summaryKey

	for _, result := range allResults {
		agent := normalizeAgent(result.LLMConfig.ID)
		provider := normalizeProvider(result.LLMConfig.ProviderID)
		modelID := normalizeModel(result.LLMConfig.ModelID)

		key := summaryKey{
			agent:    agent,
			modelID:  modelID,
			provider: provider,
		}

		if _, ok := groups[key]; !ok {
			orderedKeys = append(orderedKeys, key)
		}
		groups[key] = append(groups[key], result)
	}

	sort.Slice(orderedKeys, func(i, j int) bool {
		if orderedKeys[i].agent != orderedKeys[j].agent {
			return orderedKeys[i].agent < orderedKeys[j].agent
		}
		if orderedKeys[i].provider != orderedKeys[j].provider {
			return orderedKeys[i].provider < orderedKeys[j].provider
		}
		return orderedKeys[i].modelID < orderedKeys[j].modelID
	})

	var summaries []RunSummary
	for _, key := range orderedKeys {
		results := groups[key]
		if len(results) == 0 {
			continue
		}
		summaries = append(summaries, buildRunSummary(key.agent, key.modelID, key.provider, results))
	}

	return summaries, nil
}

func buildRunSummary(agent, modelID, provider string, results []model.TaskResult) RunSummary {
	var successCount, failCount, errorCount int
	tasks := make([]TaskBreakdown, 0, len(results))
	modelName := normalizeModel(modelID)
	providerName := normalizeProvider(provider)

	for _, result := range results {
		classification := classifyResult(result)
		switch classification {
		case "success":
			successCount++
		case "fail":
			failCount++
		default:
			errorCount++
		}
		taskProvider := normalizeProvider(result.LLMConfig.ProviderID)
		taskModel := normalizeModel(result.LLMConfig.ModelID)
		if taskProvider == "" {
			taskProvider = providerName
		}
		if taskModel == "" {
			taskModel = modelName
		}
		tasks = append(tasks, TaskBreakdown{
			Name:     result.Task,
			Provider: taskProvider,
			Model:    taskModel,
			Result:   result.Result,
			Failures: result.Failures,
			Error:    result.Error,
		})
	}

	sort.Slice(tasks, func(i, j int) bool {
		return strings.ToLower(tasks[i].Name) < strings.ToLower(tasks[j].Name)
	})

	total := len(results)
	percentage := 0.0
	if total > 0 {
		percentage = math.Round((float64(successCount)/float64(total))*10000) / 100
	}

	return RunSummary{
		ID:            makeSummaryID(agent, providerName, modelName),
		Agent:         normalizeAgent(agent),
		Model:         modelName,
		ModelProvider: providerName,
		Total:         total,
		NumSuccess:    successCount,
		NumFailed:     failCount,
		NumError:      errorCount,
		Percentage:    percentage,
		Tasks:         tasks,
	}
}

func classifyResult(result model.TaskResult) string {
	value := strings.ToLower(result.Result)
	switch {
	case strings.Contains(value, "success"):
		return "success"
	case strings.Contains(value, "fail"):
		return "fail"
	case strings.Contains(value, "error"):
		return "error"
	}

	if result.Error != "" {
		return "error"
	}
	if len(result.Failures) > 0 {
		return "fail"
	}
	return "error"
}

func makeSummaryID(agent, provider, modelID string) string {
	base := strings.ToLower(fmt.Sprintf("%s-%s-%s", agent, provider, modelID))
	var builder strings.Builder
	prevDash := false
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			builder.WriteRune('-')
			prevDash = true
		}
	}
	id := builder.String()
	id = strings.Trim(id, "-")
	if id == "" {
		return "run"
	}
	return id
}

func normalizeAgent(agent string) string {
	trimmed := strings.TrimSpace(agent)
	if trimmed == "" {
		return "unknown-agent"
	}
	return trimmed
}

func normalizeProvider(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return "custom"
	}
	return trimmed
}

func normalizeModel(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "custom-model"
	}
	return trimmed
}

func runExport() error {
	var inputRoot string
	var outputPath string

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s export --input-root <dir> --output <file>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Aggregate benchmark results into a single leaderboard JSON file.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&inputRoot, "input-root", "", "Root directory containing agent result folders (required)")
	flag.StringVar(&outputPath, "output", "", "Path to write the aggregated JSON output (required)")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		return err
	}

	if inputRoot == "" {
		flag.Usage()
		return fmt.Errorf("--input-root is required")
	}
	if outputPath == "" {
		flag.Usage()
		return fmt.Errorf("--output is required")
	}

	summaries, err := aggregateResults(inputRoot)
	if err != nil {
		return fmt.Errorf("aggregating results: %w", err)
	}

	data, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling summaries: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}

	return nil
}
