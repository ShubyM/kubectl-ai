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
	fullCommand := command

	if workDir != "" {
		fullCommand = fmt.Sprintf("cd %q && %s", workDir, fullCommand)
	}

	for _, envVar := range env {
		fullCommand = fmt.Sprintf("export %s; %s", envVar, fullCommand)
	}

	cmd := e.sandbox.CommandContext(ctx, fullCommand)
	output, err := cmd.CombinedOutput()

	result := &ExecResult{
		Command: command,
		Stdout:  string(output),
	}
	if err != nil {
		result.Error = err.Error()
		result.ExitCode = 1
	}

	return result, nil
}
