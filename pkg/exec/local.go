package exec

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"

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
	// Use the provided context directly
	cmdCtx := ctx

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
