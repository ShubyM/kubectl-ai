// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exec

import (
	"bytes"
	"context"
	"os/exec"
)

// SeatbeltExecutor executes commands locally using os/exec wrapped in sandbox-exec.
type SeatbeltExecutor struct {
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

	// This profile allows reading/writing to the working directory and /tmp,
	// but denies writing to other system locations by default (implicitly, though 'allow default' is permissive).

	profile := e.Profile
	if profile == "" {
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
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Error = exitError.Error()
		} else {
			return nil, err
		}
	}

	return result, nil
}
