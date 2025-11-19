package exec

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

const (
	defaultBashBin = "/bin/bash"
)

// LocalExecutor executes commands locally using os/exec.
type LocalExecutor struct{}

// NewLocalExecutor creates a new LocalExecutor.
func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{}
}

// Execute executes the command locally.
func (e *LocalExecutor) Execute(ctx context.Context, command string, env []string, workDir string) (*ExecResult, error) {
	isWatch := strings.Contains(command, " get ") && strings.Contains(command, " -w")
	isLogs := strings.Contains(command, " logs ") && strings.Contains(command, " -f")
	isAttach := strings.Contains(command, " attach ")

	var cmdCtx context.Context
	var cancel context.CancelFunc

	if isWatch || isLogs || isAttach {
		// Create a context with timeout for streaming commands
		cmdCtx, cancel = context.WithTimeout(ctx, 7*time.Second)
		defer cancel()
	} else {
		// Use the provided context directly
		cmdCtx = ctx
		cancel = func() {} // No-op cancel
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, os.Getenv("COMSPEC"), "/c", command)
	} else {
		cmd = exec.CommandContext(cmdCtx, lookupBashBin(), "-c", command)
	}
	cmd.Dir = workDir
	cmd.Env = env

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	result := &ExecResult{
		Command: command,
		Stdout:  stdoutBuf.String(),
		Stderr:  stderrBuf.String(),
	}

	if isWatch || isLogs || isAttach {
		if cmdCtx.Err() == context.DeadlineExceeded {
			// Timeout is expected for streaming commands
			result.StreamType = "timeout"
			result.Error = "Timeout reached after 7 seconds"
			return result, nil
		}
		// If it finished before timeout, determine stream type
		if isWatch {
			result.StreamType = "watch"
		} else if isLogs {
			result.StreamType = "logs"
		} else if isAttach {
			result.StreamType = "attach"
		}
	}

	if err != nil {
		// If it wasn't a timeout (or not a streaming command), it's a real error
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Error = exitError.Error()
			// Stderr is already captured in result.Stderr
		} else {
			return nil, err
		}
	}

	return result, nil
}

// Find the bash executable path using exec.LookPath.
func lookupBashBin() string {
	actualBashPath, err := exec.LookPath("bash")
	if err != nil {
		klog.Warningf("'bash' not found in PATH, defaulting to %s: %v", defaultBashBin, err)
		return defaultBashBin
	}
	return actualBashPath
}
