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

package main

// ConstraintTemplate represents a Gatekeeper constraint template YAML structure
type ConstraintTemplate struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name        string `json:"name"`
		Annotations struct {
			Description string `json:"description"`
			Metadata    string `json:"metadata.gatekeeper.sh/title"`
		} `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		CRD struct {
			Spec struct {
				Names struct {
					Kind string `json:"kind"`
				} `json:"names"`
			} `json:"spec"`
		} `json:"crd"`
	} `json:"spec"`
}

// Task represents a k8s-ai-bench task definition
type Task struct {
	Script     []ScriptStep `json:"script"`
	Difficulty string       `json:"difficulty"`
	Expect     []Expect     `json:"expect"`
}

// ScriptStep represents a prompt step in a task
type ScriptStep struct {
	Prompt string `json:"prompt"`
}

// Expect represents an expected output pattern
type Expect struct {
	Contains string `json:"contains"`
}

// PolicyInfo contains extracted policy information for task generation
type PolicyInfo struct {
	Name        string
	Category    string
	Description string
	Title       string
}
