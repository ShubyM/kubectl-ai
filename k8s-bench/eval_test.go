package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/k8s-bench/pkg/model"
)

func TestRunAgentHonorsContextTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group behavior differs on windows")
	}

	agentPath := buildHangingAgent(t, "#!/bin/sh\ntrap '' TERM\nwhile true; do\n  sleep 1\ndone\n")

	exec := newTestTaskExecution(t, agentPath)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := exec.runAgent(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}

	if time.Since(start) > 5*time.Second {
		t.Fatalf("runAgent should return shortly after deadline")
	}
}

func TestRunAgentKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group behavior differs on windows")
	}

	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "child-lived")

	script := "#!/bin/sh\ntrap '' TERM\n(\n  trap '' TERM\n  sleep 2\n  touch \"$1\"\n) &\nwait\n"
	agentPath := buildHangingAgent(t, script)

	exec := newTestTaskExecution(t, agentPath, marker)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := exec.runAgent(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}

	// Give any stray child process time to run if it survived cancellation.
	time.Sleep(3 * time.Second)

	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			t.Fatalf("marker file %s was created, child process survived cancellation", marker)
		}
		t.Fatalf("unexpected error stating marker file: %v", err)
	}
}

func buildHangingAgent(t *testing.T, script string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "agent.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write agent script: %v", err)
	}
	return path
}

func newTestTaskExecution(t *testing.T, agentPath string, args ...string) *TaskExecution {
	t.Helper()

	taskDir := t.TempDir()
	kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(kubeconfig, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}

	exec := &TaskExecution{
		AgentBin:      agentPath,
		agentArgs:     args,
		kubeConfig:    kubeconfig,
		result:        &model.TaskResult{},
		llmConfig:     model.LLMConfig{},
		log:           io.Discard,
		task:          &Task{},
		taskID:        "test",
		taskDir:       taskDir,
		taskOutputDir: filepath.Join(t.TempDir(), "output"),
	}

	return exec
}
