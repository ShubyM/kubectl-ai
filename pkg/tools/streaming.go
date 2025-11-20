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

package tools

import (
	"context"
	"strings"
	"time"

	pkgexec "github.com/GoogleCloudPlatform/kubectl-ai/pkg/exec"
)

// ExecuteWithStreamingHandling executes a command using the provided executor,
// handling streaming commands (watch, logs -f, attach) by applying a timeout
// and capturing partial output.
func ExecuteWithStreamingHandling(ctx context.Context, command string, env []string, workDir string, executor pkgexec.Executor) (*pkgexec.ExecResult, error) {
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

	result, err := executor.Execute(cmdCtx, command, env, workDir)

	// If executor returns nil result on error (it shouldn't, but let's be safe), create one
	if result == nil {
		result = &pkgexec.ExecResult{Command: command}
	}

	if isWatch || isLogs || isAttach {
		if cmdCtx.Err() == context.DeadlineExceeded {
			// Timeout is expected for streaming commands
			result.StreamType = "timeout"
			result.Error = "Timeout reached after 7 seconds"
			// Clear the error if it was just the timeout
			err = nil
			// Determine stream type
			if isWatch {
				result.StreamType = "watch"
			} else if isLogs {
				result.StreamType = "logs"
			} else if isAttach {
				result.StreamType = "attach"
			}
			return result, nil
		}
	}

	return result, err
}
