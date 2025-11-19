package exec

import (
	"context"
	"fmt"
)

// Executor defines the interface for executing commands.
type Executor interface {
	// Execute runs a command and returns the result.
	Execute(ctx context.Context, command string, env []string, workDir string) (*ExecResult, error)
}

// ExecResult represents the result of a command execution.
type ExecResult struct {
	Command    string `json:"command,omitempty"`
	Error      string `json:"error,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	StreamType string `json:"stream_type,omitempty"`
}

func (e *ExecResult) String() string {
	return fmt.Sprintf("Command: %q\nError: %q\nStdout: %q\nStderr: %q\nExitCode: %d\nStreamType: %q}", e.Command, e.Error, e.Stdout, e.Stderr, e.ExitCode, e.StreamType)
}
