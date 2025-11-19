package exec

import (
	"bytes"
	"context"
	"os/exec"
)

// SeatbeltExecutor executes commands locally using os/exec wrapped in sandbox-exec.
type SeatbeltExecutor struct {
	// Profile is the seatbelt profile to use.
	// If empty, a default profile will be used.
	Profile string
}

// NewSeatbeltExecutor creates a new SeatbeltExecutor.
func NewSeatbeltExecutor() *SeatbeltExecutor {
	return &SeatbeltExecutor{}
}

// Execute executes the command locally wrapped in sandbox-exec.
func (e *SeatbeltExecutor) Execute(ctx context.Context, command string, env []string, workDir string) (*ExecResult, error) {
	// Use the provided context directly
	cmdCtx := ctx

	// Default profile if none specified
	// This profile allows reading/writing to the working directory and /tmp,
	// but denies writing to other system locations by default (implicitly, though 'allow default' is permissive).
	// We use a simple profile for now that allows everything but logs it, or just 'allow default'.
	// Ideally, we should generate a strict profile.
	profile := e.Profile
	if profile == "" {
		// Generate a default profile
		profile = `(version 1)
(allow default)
(allow file-write*
    (literal "/dev/null")
    (literal "/dev/zero")
    (literal "/dev/dtracehelper")
    (literal "/dev/tty")
    (subpath "/tmp")
    (subpath "/private/tmp")
    (subpath "` + workDir + `")
)
(allow network*)
(allow process-exec*)
(allow sysctl-read)
`
	}

	// Wrap command with sandbox-exec
	// We use bash to execute the command string within the sandbox
	// Use -p to pass the profile directly
	sandboxArgs := []string{"-p", profile, "/bin/bash", "-c", command}
	cmd := exec.CommandContext(cmdCtx, "sandbox-exec", sandboxArgs...)
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
