//go:build !darwin

package exec

import (
	"context"
	"fmt"
)

// SeatbeltExecutor executes commands locally using os/exec wrapped in sandbox-exec.
type SeatbeltExecutor struct {
}

// NewSeatbeltExecutor creates a new SeatbeltExecutor.
func NewSeatbeltExecutor() *SeatbeltExecutor {
	return &SeatbeltExecutor{}
}

// Execute executes the command locally wrapped in sandbox-exec.
func (e *SeatbeltExecutor) Execute(ctx context.Context, command string, env []string, workDir string) (*ExecResult, error) {
	return nil, fmt.Errorf("SeatbeltExecutor is only supported on macOS")
}
