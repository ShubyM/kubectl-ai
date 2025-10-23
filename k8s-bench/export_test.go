package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"os"

	"github.com/GoogleCloudPlatform/kubectl-ai/k8s-bench/pkg/model"
	"sigs.k8s.io/yaml"
)

func TestBuildRunSummaryCalculatesStatistics(t *testing.T) {
	results := []model.TaskResult{
		{
			Task:   "Create Deployment",
			Result: "success",
			LLMConfig: model.LLMConfig{
				ProviderID: "google",
				ModelID:    "Gemini 1.5 Pro",
			},
		},
		{
			Task:   "Expose Service",
			Result: "fail",
			Failures: []model.Failure{
				{Message: "invalid port"},
			},
			LLMConfig: model.LLMConfig{
				ProviderID: "google",
				ModelID:    "Gemini 1.5 Pro",
			},
		},
		{
			Task:   "Scale Deployment",
			Result: "error",
			Error:  "cluster timeout",
			LLMConfig: model.LLMConfig{
				ProviderID: "google",
				ModelID:    "Gemini 1.5 Pro",
			},
		},
	}

	summary := buildRunSummary("agent-alpha", "Gemini 1.5 Pro", "google", results)

	if summary.NumSuccess != 1 {
		t.Fatalf("expected 1 success, got %d", summary.NumSuccess)
	}
	if summary.NumFailed != 1 {
		t.Fatalf("expected 1 failure, got %d", summary.NumFailed)
	}
	if summary.NumError != 1 {
		t.Fatalf("expected 1 error, got %d", summary.NumError)
	}
	if summary.Total != 3 {
		t.Fatalf("expected total 3, got %d", summary.Total)
	}
	if summary.Percentage != 33.33 {
		t.Fatalf("expected percentage 33.33, got %f", summary.Percentage)
	}

	expectedOrder := []string{"Create Deployment", "Expose Service", "Scale Deployment"}
	if len(summary.Tasks) != len(expectedOrder) {
		t.Fatalf("expected %d tasks, got %d", len(expectedOrder), len(summary.Tasks))
	}
	for i, task := range summary.Tasks {
		if task.Name != expectedOrder[i] {
			t.Fatalf("expected task %q at index %d, got %q", expectedOrder[i], i, task.Name)
		}
	}
}

func TestAggregateResultsProducesStableJSON(t *testing.T) {
	tmp := t.TempDir()

	fixtures := []struct {
		agent  string
		task   string
		result model.TaskResult
	}{
		{
			agent: "agent-alpha",
			task:  "create-deployment",
			result: model.TaskResult{
				Task:   "Create Deployment",
				Result: "success",
				LLMConfig: model.LLMConfig{
					ID:         "agent-alpha",
					ProviderID: "google",
					ModelID:    "Gemini 1.5 Pro",
				},
			},
		},
		{
			agent: "agent-alpha",
			task:  "expose-service",
			result: model.TaskResult{
				Task:   "Expose Service",
				Result: "fail",
				Failures: []model.Failure{
					{Message: "invalid port"},
				},
				LLMConfig: model.LLMConfig{
					ID:         "agent-alpha",
					ProviderID: "google",
					ModelID:    "Gemini 1.5 Pro",
				},
			},
		},
		{
			agent: "agent-alpha",
			task:  "scale-deployment",
			result: model.TaskResult{
				Task:   "Scale Deployment",
				Result: "error",
				Error:  "cluster timeout",
				LLMConfig: model.LLMConfig{
					ID:         "agent-alpha",
					ProviderID: "google",
					ModelID:    "Gemini 1.5 Pro",
				},
			},
		},
		{
			agent: "agent-alpha",
			task:  "build-ingress",
			result: model.TaskResult{
				Task:   "Build Ingress",
				Result: "success",
				LLMConfig: model.LLMConfig{
					ID:         "agent-alpha",
					ProviderID: "google",
					ModelID:    "Gemini Ultra",
				},
			},
		},
		{
			agent: "agent-alpha",
			task:  "cleanup",
			result: model.TaskResult{
				Task:   "Cleanup",
				Result: "success",
				LLMConfig: model.LLMConfig{
					ID:         "agent-alpha",
					ProviderID: "google",
					ModelID:    "Gemini Ultra",
				},
			},
		},
		{
			agent: "agent-beta",
			task:  "create-deployment",
			result: model.TaskResult{
				Task:   "Create Deployment",
				Result: "success",
				LLMConfig: model.LLMConfig{
					ID:         "agent-beta",
					ProviderID: "openai",
					ModelID:    "GPT-4o",
				},
			},
		},
	}

	for idx, fx := range fixtures {
		dir := filepath.Join(tmp, fx.agent, fx.task, "run", "latest")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating directory: %v", err)
		}
		data, err := yaml.Marshal(fx.result)
		if err != nil {
			t.Fatalf("marshalling yaml: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "results.yaml"), data, 0o644); err != nil {
			t.Fatalf("writing results file %d: %v", idx, err)
		}
	}

	summaries, err := aggregateResults(tmp)
	if err != nil {
		t.Fatalf("aggregateResults returned error: %v", err)
	}

	payload, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		t.Fatalf("marshal summaries: %v", err)
	}

	expected := `[
  {
    "id": "agent-alpha-google-gemini-1-5-pro",
    "agent": "agent-alpha",
    "model": "Gemini 1.5 Pro",
    "model_provider": "google",
    "total": 3,
    "num_success": 1,
    "num_failed": 1,
    "num_error": 1,
    "percentage": 33.33,
    "tasks": [
      {
        "name": "Create Deployment",
        "provider": "google",
        "model": "Gemini 1.5 Pro",
        "result": "success"
      },
      {
        "name": "Expose Service",
        "provider": "google",
        "model": "Gemini 1.5 Pro",
        "result": "fail",
        "failures": [
          {
            "message": "invalid port"
          }
        ]
      },
      {
        "name": "Scale Deployment",
        "provider": "google",
        "model": "Gemini 1.5 Pro",
        "result": "error",
        "error": "cluster timeout"
      }
    ]
  },
  {
    "id": "agent-alpha-google-gemini-ultra",
    "agent": "agent-alpha",
    "model": "Gemini Ultra",
    "model_provider": "google",
    "total": 2,
    "num_success": 2,
    "num_failed": 0,
    "num_error": 0,
    "percentage": 100,
    "tasks": [
      {
        "name": "Build Ingress",
        "provider": "google",
        "model": "Gemini Ultra",
        "result": "success"
      },
      {
        "name": "Cleanup",
        "provider": "google",
        "model": "Gemini Ultra",
        "result": "success"
      }
    ]
  },
  {
    "id": "agent-beta-openai-gpt-4o",
    "agent": "agent-beta",
    "model": "GPT-4o",
    "model_provider": "openai",
    "total": 1,
    "num_success": 1,
    "num_failed": 0,
    "num_error": 0,
    "percentage": 100,
    "tasks": [
      {
        "name": "Create Deployment",
        "provider": "openai",
        "model": "GPT-4o",
        "result": "success"
      }
    ]
  }
]`

	if string(payload) != expected {
		t.Fatalf("unexpected JSON summary.\nwant:\n%s\n\ngot:\n%s", expected, string(payload))
	}
}
