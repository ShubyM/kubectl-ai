package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/k8s-bench/pkg/model"
)

type jsonReport struct {
	GeneratedAt  time.Time                  `json:"generatedAt"`
	Totals       jsonSummary                `json:"totals"`
	Leaderboards map[string]jsonLeaderboard `json:"leaderboards"`
	Results      []jsonTaskResult           `json:"results"`
}

type jsonSummary struct {
	Runs       int     `json:"runs"`
	Succeeded  int     `json:"succeeded"`
	Failed     int     `json:"failed"`
	Errored    int     `json:"errored"`
	Percentage float64 `json:"percentage"`
}

type jsonLeaderboard struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Entries     []jsonLeaderboardEntry `json:"entries"`
}

type jsonLeaderboardEntry struct {
	Agent      string  `json:"agent"`
	Model      string  `json:"model"`
	Succeeded  int     `json:"succeeded"`
	Failed     int     `json:"failed"`
	Errored    int     `json:"errored"`
	Percentage float64 `json:"percentage"`
}

type jsonTaskResult struct {
	Task     string   `json:"task"`
	Agent    string   `json:"agent"`
	Model    string   `json:"model"`
	Category string   `json:"category"`
	Result   string   `json:"result"`
	Failures []string `json:"failures,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type leaderboardAccumulator struct {
	agent     string
	model     string
	succeeded int
	failed    int
	errored   int
}

func buildJSONReport(config AnalyzeConfig, results []model.TaskResult) jsonReport {
	summary := jsonSummary{
		Runs: len(results),
	}

	categoryAccumulators := map[string]map[string]*leaderboardAccumulator{
		"text":   {},
		"webdev": {},
	}

	taskResults := make([]jsonTaskResult, 0, len(results))

	for _, result := range results {
		outcome := classifyResult(result.Result)
		switch outcome {
		case "success":
			summary.Succeeded++
		case "fail":
			summary.Failed++
		default:
			summary.Errored++
		}

		categoryKey := determineCategory(config.IgnoreToolUseShim, result.LLMConfig.EnableToolUseShim)

		accumulatorMap, ok := categoryAccumulators[categoryKey]
		if !ok {
			accumulatorMap = map[string]*leaderboardAccumulator{}
			categoryAccumulators[categoryKey] = accumulatorMap
		}

		agentID := result.LLMConfig.ProviderID
		key := fmt.Sprintf("%s|%s", agentID, result.LLMConfig.ModelID)

		acc, exists := accumulatorMap[key]
		if !exists {
			acc = &leaderboardAccumulator{
				agent: agentID,
				model: result.LLMConfig.ModelID,
			}
			accumulatorMap[key] = acc
		}

		switch outcome {
		case "success":
			acc.succeeded++
		case "fail":
			acc.failed++
		default:
			acc.errored++
		}

		taskResults = append(taskResults, buildTaskResultJSON(config, result, categoryKey))
	}

	sort.Slice(taskResults, func(i, j int) bool {
		if taskResults[i].Task == taskResults[j].Task {
			if taskResults[i].Agent == taskResults[j].Agent {
				if taskResults[i].Model == taskResults[j].Model {
					return taskResults[i].Category < taskResults[j].Category
				}
				return taskResults[i].Model < taskResults[j].Model
			}
			return taskResults[i].Agent < taskResults[j].Agent
		}
		return taskResults[i].Task < taskResults[j].Task
	})

	textEntries := buildLeaderboardEntries(categoryAccumulators["text"])
	webdevEntries := buildLeaderboardEntries(categoryAccumulators["webdev"])

	textLeaderboard := jsonLeaderboard{
		Title:   "Text",
		Entries: textEntries,
	}
	webdevLeaderboard := jsonLeaderboard{
		Title:   "WebDev",
		Entries: webdevEntries,
	}

	if config.IgnoreToolUseShim {
		textLeaderboard.Description = "Aggregated across tool use shim settings"
		if len(webdevEntries) == 0 {
			webdevLeaderboard.Description = "Tool use shim results aggregated into Text leaderboard"
		}
	} else {
		textLeaderboard.Description = "Tool use shim disabled"
		webdevLeaderboard.Description = "Tool use shim enabled"
	}

	summary.Percentage = calculateSuccessRate(summary.Succeeded, summary.Runs)

	report := jsonReport{
		GeneratedAt:  time.Now().UTC(),
		Totals:       summary,
		Leaderboards: map[string]jsonLeaderboard{},
		Results:      taskResults,
	}

	report.Leaderboards["text"] = textLeaderboard
	report.Leaderboards["webdev"] = webdevLeaderboard

	return report
}

func buildLeaderboardEntries(accumulator map[string]*leaderboardAccumulator) []jsonLeaderboardEntry {
	entries := make([]jsonLeaderboardEntry, 0, len(accumulator))
	for _, acc := range accumulator {
		total := acc.succeeded + acc.failed + acc.errored
		entry := jsonLeaderboardEntry{
			Agent:      acc.agent,
			Model:      acc.model,
			Succeeded:  acc.succeeded,
			Failed:     acc.failed,
			Errored:    acc.errored,
			Percentage: calculateSuccessRate(acc.succeeded, total),
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Percentage == entries[j].Percentage {
			if entries[i].Succeeded == entries[j].Succeeded {
				if entries[i].Failed == entries[j].Failed {
					if entries[i].Agent == entries[j].Agent {
						return entries[i].Model < entries[j].Model
					}
					return entries[i].Agent < entries[j].Agent
				}
				return entries[i].Failed < entries[j].Failed
			}
			return entries[i].Succeeded > entries[j].Succeeded
		}
		return entries[i].Percentage > entries[j].Percentage
	})

	return entries
}

func buildTaskResultJSON(config AnalyzeConfig, result model.TaskResult, category string) jsonTaskResult {
	taskResult := jsonTaskResult{
		Task:     result.Task,
		Agent:    result.LLMConfig.ProviderID,
		Model:    result.LLMConfig.ModelID,
		Category: category,
		Result:   result.Result,
		Error:    result.Error,
	}

	if config.ShowFailures && len(result.Failures) > 0 {
		failures := make([]string, 0, len(result.Failures))
		for _, failure := range result.Failures {
			failures = append(failures, failure.Message)
		}
		taskResult.Failures = failures
	}

	return taskResult
}

func classifyResult(result string) string {
	normalized := strings.ToLower(result)
	switch {
	case strings.Contains(normalized, "success"):
		return "success"
	case strings.Contains(normalized, "fail"):
		return "fail"
	default:
		return "error"
	}
}

func determineCategory(ignoreToolUse bool, toolUseEnabled bool) string {
	if ignoreToolUse {
		return "text"
	}
	if toolUseEnabled {
		return "webdev"
	}
	return "text"
}
func calculateSuccessRate(success, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(success) / float64(total)
}
