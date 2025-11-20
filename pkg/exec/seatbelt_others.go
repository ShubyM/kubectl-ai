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
