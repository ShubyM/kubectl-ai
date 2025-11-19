package exec

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/sandbox"
)

// SandboxExecutor executes commands in a Kubernetes sandbox.
type SandboxExecutor struct {
	sandbox *sandbox.Sandbox
}

// NewSandboxExecutor creates a new SandboxExecutor.
func NewSandboxExecutor(s *sandbox.Sandbox) *SandboxExecutor {
	return &SandboxExecutor{sandbox: s}
}

// Execute executes the command in the sandbox.
func (e *SandboxExecutor) Execute(ctx context.Context, command string, env []string, workDir string) (*ExecResult, error) {
	// Construct the full command with env and workDir
	fullCommand := command

	// Handle workDir
	if workDir != "" {
		fullCommand = fmt.Sprintf("cd %q && %s", workDir, fullCommand)
	}

	// Handle env
	// Note: We prepend exports. Be careful with escaping if values contain special characters.
	// For now, we assume simple KEY=VALUE pairs.
	for _, envVar := range env {
		fullCommand = fmt.Sprintf("export %s; %s", envVar, fullCommand)
	}

	cmd := e.sandbox.CommandContext(ctx, fullCommand)
	output, err := cmd.CombinedOutput()

	result := &ExecResult{
		Command: command, // Return original command for clarity? Or fullCommand?
		Stdout:  string(output),
	}
	if err != nil {
		result.Error = err.Error()
		result.ExitCode = 1 // Sandbox doesn't return exit code easily yet
	}

	return result, nil
}
