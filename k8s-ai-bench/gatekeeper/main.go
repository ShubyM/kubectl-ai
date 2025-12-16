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

// gatekeeper-gen generates k8s-ai-bench tasks from the OPA Gatekeeper library.
//
// It scrapes constraint templates and their examples to create compliance
// checking tasks where AI models must determine if a Kubernetes resource
// complies with a given policy requirement.
//
// Usage:
//
//	go run . [flags]
//	make generate
//
// The generator clones the Gatekeeper library from GitHub, parses constraint
// templates for natural language descriptions, and generates task.yaml files
// for each example resource (both compliant and non-compliant).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	gatekeeperRepoURL = "https://github.com/open-policy-agent/gatekeeper-library.git"
	defaultRepoDir    = ".gatekeeper-library"
	defaultOutputDir  = "./tasks"
)

func main() {
	var (
		repoDir   string
		outputDir string
		skipClone bool
		dryRun    bool
		clean     bool
	)

	flag.StringVar(&repoDir, "repo-dir", defaultRepoDir, "Directory to clone/use the Gatekeeper library")
	flag.StringVar(&outputDir, "output-dir", defaultOutputDir, "Output directory for generated tasks")
	flag.BoolVar(&skipClone, "skip-clone", false, "Skip cloning the repository (use existing)")
	flag.BoolVar(&dryRun, "dry-run", false, "Print what would be generated without creating files")
	flag.BoolVar(&clean, "clean", false, "Remove all generated tasks before generating")
	flag.Parse()

	if err := run(repoDir, outputDir, skipClone, dryRun, clean); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(repoDir, outputDir string, skipClone, dryRun, clean bool) error {
	// Clean existing tasks if requested
	if clean && !dryRun {
		fmt.Printf("Cleaning existing tasks in %s...\n", outputDir)
		entries, err := os.ReadDir(outputDir)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("reading output dir: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "gatekeeper-") {
				taskDir := filepath.Join(outputDir, entry.Name())
				if err := os.RemoveAll(taskDir); err != nil {
					return fmt.Errorf("removing %s: %w", taskDir, err)
				}
			}
		}
	}

	// Clone or update the Gatekeeper library
	if !skipClone {
		if err := cloneOrUpdateRepo(repoDir); err != nil {
			return fmt.Errorf("failed to clone/update repo: %w", err)
		}
	}

	// Verify library directory exists
	libraryDir := filepath.Join(repoDir, "library")
	if _, err := os.Stat(libraryDir); os.IsNotExist(err) {
		return fmt.Errorf("library directory not found at %s", libraryDir)
	}

	// Process all policy categories
	categories := []string{"general", "pod-security-policy"}
	var totalGenerated int

	for _, category := range categories {
		categoryDir := filepath.Join(libraryDir, category)
		if _, err := os.Stat(categoryDir); os.IsNotExist(err) {
			fmt.Printf("Skipping category %s (not found)\n", category)
			continue
		}

		policies, err := os.ReadDir(categoryDir)
		if err != nil {
			return fmt.Errorf("reading category %s: %w", category, err)
		}

		for _, policy := range policies {
			if !policy.IsDir() {
				continue
			}

			policyDir := filepath.Join(categoryDir, policy.Name())
			generated, err := processPolicy(policyDir, category, policy.Name(), outputDir, dryRun)
			if err != nil {
				fmt.Printf("Warning: failed to process policy %s/%s: %v\n", category, policy.Name(), err)
				continue
			}
			totalGenerated += generated
		}
	}

	fmt.Printf("\nTotal tasks generated: %d\n", totalGenerated)
	return nil
}

func cloneOrUpdateRepo(repoDir string) error {
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		fmt.Printf("Cloning Gatekeeper library to %s...\n", repoDir)
		cmd := exec.Command("git", "clone", "--depth=1", gatekeeperRepoURL, repoDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	fmt.Printf("Updating Gatekeeper library in %s...\n", repoDir)
	cmd := exec.Command("git", "-C", repoDir, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to update repo (continuing with existing): %v\n", err)
	}
	return nil
}

func processPolicy(policyDir, category, policyName, outputDir string, dryRun bool) (int, error) {
	// Read the template.yaml to get the description
	templatePath := filepath.Join(policyDir, "template.yaml")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return 0, fmt.Errorf("reading template: %w", err)
	}

	var template ConstraintTemplate
	if err := yaml.Unmarshal(templateData, &template); err != nil {
		return 0, fmt.Errorf("parsing template: %w", err)
	}

	policyInfo := PolicyInfo{
		Name:        policyName,
		Category:    category,
		Description: template.Metadata.Annotations.Description,
		Title:       template.Metadata.Annotations.Metadata,
	}

	// Use our curated description if available, otherwise fall back to template
	if desc, ok := PolicyDescriptions[policyName]; ok {
		policyInfo.Description = desc
	} else if policyInfo.Description == "" {
		policyInfo.Description = generateDescriptionFromName(policyName)
	}

	if policyInfo.Title == "" {
		policyInfo.Title = formatTitle(policyName)
	}

	// Find samples directory
	samplesDir := filepath.Join(policyDir, "samples")
	if _, err := os.Stat(samplesDir); os.IsNotExist(err) {
		return 0, fmt.Errorf("no samples directory found")
	}

	samples, err := os.ReadDir(samplesDir)
	if err != nil {
		return 0, fmt.Errorf("reading samples: %w", err)
	}

	var generated int
	for _, sample := range samples {
		if !sample.IsDir() {
			continue
		}

		sampleDir := filepath.Join(samplesDir, sample.Name())

		// Process example_allowed.yaml
		allowedPath := filepath.Join(sampleDir, "example_allowed.yaml")
		if _, err := os.Stat(allowedPath); err == nil {
			if err := generateTask(policyInfo, allowedPath, true, outputDir, sample.Name(), dryRun); err != nil {
				fmt.Printf("Warning: failed to generate allowed task for %s: %v\n", sample.Name(), err)
			} else {
				generated++
			}
		}

		// Process example_disallowed.yaml
		disallowedPath := filepath.Join(sampleDir, "example_disallowed.yaml")
		if _, err := os.Stat(disallowedPath); err == nil {
			if err := generateTask(policyInfo, disallowedPath, false, outputDir, sample.Name(), dryRun); err != nil {
				fmt.Printf("Warning: failed to generate disallowed task for %s: %v\n", sample.Name(), err)
			} else {
				generated++
			}
		}
	}

	if generated > 0 {
		fmt.Printf("Generated %d tasks for %s/%s\n", generated, category, policyName)
	}
	return generated, nil
}

func generateTask(policyInfo PolicyInfo, examplePath string, isAllowed bool, outputDir, sampleName string, dryRun bool) error {
	// Read the example resource
	exampleData, err := os.ReadFile(examplePath)
	if err != nil {
		return fmt.Errorf("reading example: %w", err)
	}

	// Create task name
	compliance := "compliant"
	if !isAllowed {
		compliance = "noncompliant"
	}
	taskName := fmt.Sprintf("gatekeeper-%s-%s-%s",
		sanitizeName(policyInfo.Name),
		sanitizeName(sampleName),
		compliance)

	// Expected answer
	expectedAnswer := "COMPLIANT"
	if !isAllowed {
		expectedAnswer = "NON-COMPLIANT"
	}

	// Build the prompt
	prompt := fmt.Sprintf(`You are a Kubernetes security reviewer checking resources for policy compliance.

**Policy Requirement:**
%s

**Resource to Review:**
%s
Does this resource COMPLY with the policy requirement above?

First, answer with exactly "COMPLIANT" or "NON-COMPLIANT".
Then briefly explain your reasoning.`, policyInfo.Description, "```yaml\n"+string(exampleData)+"```")

	// Create the task
	task := Task{
		Script: []ScriptStep{
			{Prompt: prompt},
		},
		Difficulty: "easy",
		Expect: []Expect{
			{Contains: expectedAnswer},
		},
	}

	if dryRun {
		fmt.Printf("\n--- Would generate: %s ---\n", taskName)
		fmt.Printf("Expected: %s\n", expectedAnswer)
		fmt.Printf("Description: %s\n", policyInfo.Description[:min(100, len(policyInfo.Description))])
		return nil
	}

	// Create task directory
	taskDir := filepath.Join(outputDir, taskName)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("creating task directory: %w", err)
	}

	// Write task.yaml
	taskData, err := yaml.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshaling task: %w", err)
	}

	taskPath := filepath.Join(taskDir, "task.yaml")
	if err := os.WriteFile(taskPath, taskData, 0644); err != nil {
		return fmt.Errorf("writing task.yaml: %w", err)
	}

	return nil
}

func generateDescriptionFromName(name string) string {
	words := strings.Split(name, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return fmt.Sprintf("Resources must comply with the %s policy.", strings.Join(words, " "))
}

func formatTitle(name string) string {
	words := strings.Split(name, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

func sanitizeName(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	name = reg.ReplaceAllString(name, "-")
	reg = regexp.MustCompile(`-+`)
	name = reg.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return strings.ToLower(name)
}
