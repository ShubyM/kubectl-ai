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
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/gollm"
	pkgexec "github.com/GoogleCloudPlatform/kubectl-ai/pkg/exec"
	"k8s.io/klog/v2"
)

func init() {
	RegisterTool(&BashTool{})
}

const (
	defaultBashBin = "/bin/bash"
)

// Find the bash executable path using exec.LookPath.
// On some systems (like NixOS), executables might not be in standard locations like /bin/bash.
func lookupBashBin() string {
	actualBashPath, err := osexec.LookPath("bash")
	if err != nil {
		klog.Warningf("'bash' not found in PATH, defaulting to %s: %v", defaultBashBin, err)
		return defaultBashBin
	}
	return actualBashPath
}

// expandShellVar expands shell variables and syntax using bash
func expandShellVar(value string) (string, error) {
	if strings.Contains(value, "~") {
		if len(value) >= 2 && value[0] == '~' && os.IsPathSeparator(value[1]) {
			if runtime.GOOS == "windows" {
				value = filepath.Join(os.Getenv("USERPROFILE"), value[2:])
			} else {
				value = filepath.Join(os.Getenv("HOME"), value[2:])
			}
		}
	}
	return os.ExpandEnv(value), nil
}

type BashTool struct{}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Description() string {
	return "Executes a bash command. Use this tool only when you need to execute a shell command."
}

func (t *BashTool) FunctionDefinition() *gollm.FunctionDefinition {
	return &gollm.FunctionDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &gollm.Schema{
			Type: gollm.TypeObject,
			Properties: map[string]*gollm.Schema{
				"command": {
					Type:        gollm.TypeString,
					Description: `The bash command to execute.`,
				},
				"modifies_resource": {
					Type: gollm.TypeString,
					Description: `Whether the command modifies a kubernetes resource.
Possible values:
- "yes" if the command modifies a resource
- "no" if the command does not modify a resource
- "unknown" if the command's effect on the resource is unknown
`,
				},
			},
		},
	}
}

func (t *BashTool) Run(ctx context.Context, args map[string]any) (any, error) {
	command := args["command"].(string)

	// Interactive commands are now supported via the Executor interface
	// if strings.Contains(command, "kubectl edit") { ... }


	executor := executorFromContext(ctx)
	workDir, _ := ctx.Value(WorkDirKey).(string)
	kubeconfig, _ := ctx.Value(KubeconfigKey).(string)

	var env []string
	if kubeconfig != "" {
		expanded, err := expandShellVar(kubeconfig)
		if err != nil {
			return nil, err
		}
		env = append(env, "KUBECONFIG="+expanded)
	}

	return runCommandWithExecutor(ctx, executor, command, workDir, env, false)
}

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



func executorFromContext(ctx context.Context) pkgexec.Executor {
	if val := ctx.Value(ExecutorKey); val != nil {
		if executor, ok := val.(pkgexec.Executor); ok && executor != nil {
			return executor
		}
	}
	return pkgexec.NewLocal()
}

func shellCommandForExecutor(executor pkgexec.Executor, command string) (string, []string) {
	switch executor.(type) {
	case *pkgexec.Local:
		if runtime.GOOS == "windows" {
			return os.Getenv("COMSPEC"), []string{"/c", command}
		}
		return lookupBashBin(), []string{"-c", command}
	case *pkgexec.K8s:
		// Sandbox already wraps in /bin/sh -c, so pass command directly
		return command, nil
	default:
		return "/bin/sh", []string{"-c", command}
	}
}

func runCommandWithExecutor(ctx context.Context, executor pkgexec.Executor, command, workDir string, env []string, isInteractive bool) (*ExecResult, error) {
	if executor == nil {
		executor = pkgexec.NewLocal()
	}

	isWatch := strings.Contains(command, " get ") && (strings.Contains(command, " -w") || strings.Contains(command, " --watch"))
	isLogs := strings.Contains(command, " logs ") && (strings.Contains(command, " -f") || strings.Contains(command, " --follow"))
	isAttach := strings.Contains(command, " attach ")
	isStreaming := isWatch || isLogs || isAttach

	var cancel context.CancelFunc
	// Only timeout if NOT interactive and IS streaming (like logs -f without interaction)
	if isStreaming && !isInteractive {
		ctx, cancel = context.WithTimeout(ctx, 7*time.Second)
		defer cancel()
	}

	opts := []pkgexec.Option{
		pkgexec.WithDir(workDir),
		pkgexec.WithEnv(env),
	}

	if isInteractive {
		opts = append(opts, pkgexec.WithStreams(pkgexec.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			TTY:    true,
		}))
	}

	// Execute
	shell, args := shellCommandForExecutor(executor, command)
	result, err := executor.Run(ctx, shell, args, opts...)

	execResult := &ExecResult{
		Command: command,
	}

	if isWatch {
		execResult.StreamType = "watch"
	}
	if isLogs {
		execResult.StreamType = "logs"
	}
	if isAttach {
		execResult.StreamType = "attach"
	}

	if result != nil {
		execResult.Stdout = result.Stdout
		execResult.Stderr = result.Stderr
		execResult.ExitCode = result.ExitCode
	}

	if ctx.Err() == context.DeadlineExceeded {
		if execResult.Stderr != "" {
			execResult.Stderr += "\n"
		}
		execResult.Stderr += "(Command stopped after 7s timeout)"
		return execResult, nil
	}

	if err != nil {
		execResult.Error = err.Error()
	}

	return execResult, nil
}

func (t *BashTool) IsInteractive(args map[string]any) (bool, error) {
	// Delegate to Kubectl tool for interactive check since we only support kubectl interactive commands for now
	kubectlTool := &Kubectl{}
	return kubectlTool.IsInteractive(args)
}

// CheckModifiesResource determines if the command modifies kubernetes resources
// This is used for permission checks before command execution
// Returns "yes", "no", or "unknown"
func (t *BashTool) CheckModifiesResource(args map[string]any) string {
	command, ok := args["command"].(string)
	if !ok {
		return "unknown"
	}

	if strings.Contains(command, "kubectl") {
		return kubectlModifiesResource(command)
	}

	return "unknown"
}
