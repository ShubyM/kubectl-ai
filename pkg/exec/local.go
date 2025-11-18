package exec

import (
	"bytes"
	"context"
	"os"
	osExec "os/exec"
)

// Local runs commands on the host machine.
type Local struct{}

var _ Executor = (*Local)(nil)

func NewLocal() *Local {
	return &Local{}
}

func (l *Local) Run(ctx context.Context, command string, args []string, opts ...Option) (*Result, error) {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}

	cmd := osExec.CommandContext(ctx, command, args...)
	if cfg.Dir != "" {
		cmd.Dir = cfg.Dir
	}

	if len(cfg.Env) > 0 {
		cmd.Env = append(os.Environ(), cfg.Env...)
	}

	var stdout, stderr bytes.Buffer

	// If streams are provided, use them. Otherwise, capture to buffer for Result.
	if cfg.StreamOptions.Stdout != nil {
		cmd.Stdout = cfg.StreamOptions.Stdout
	} else {
		cmd.Stdout = &stdout
	}

	if cfg.StreamOptions.Stderr != nil {
		cmd.Stderr = cfg.StreamOptions.Stderr
	} else {
		cmd.Stderr = &stderr
	}

	if cfg.StreamOptions.Stdin != nil {
		cmd.Stdin = cfg.StreamOptions.Stdin
	}

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*osExec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, err
}
