package exec

import (
	"bytes"
	"context"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/sandbox"
)

// K8s runs commands inside a remote pod.
type K8s struct {
	sandbox *sandbox.KubeSandbox
}

var _ Executor = (*K8s)(nil)

func NewK8s(sb *sandbox.KubeSandbox) *K8s {
	return &K8s{sandbox: sb}
}

func (k *K8s) Run(ctx context.Context, command string, args []string, opts ...Option) (*Result, error) {
	// Currently Kubernetes execution does not support working directories or custom env,
	// but we still process options to avoid unused parameter errors.
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}

	_ = cfg

	cmd := k.sandbox.CommandContext(ctx, command, args...)

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
	cmd.TTY = cfg.StreamOptions.TTY

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		// TODO: Parse exit code from error if possible
		exitCode = 1
	}

	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, err
}

func (k *K8s) Close(ctx context.Context) error {
	return k.sandbox.Delete(ctx)
}
