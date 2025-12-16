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
// Tasks deploy actual resources to a cluster with Gatekeeper installed,
// and use Gatekeeper's audit mode to verify compliance.
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
	// Read the template.yaml
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
		Name:         policyName,
		Category:     category,
		Description:  template.Metadata.Annotations.Description,
		Title:        template.Metadata.Annotations.Metadata,
		TemplateYAML: string(templateData),
	}

	// Use our curated description if available
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

		// Read constraint.yaml
		constraintPath := filepath.Join(sampleDir, "constraint.yaml")
		constraintData, err := os.ReadFile(constraintPath)
		if err != nil {
			fmt.Printf("Warning: no constraint.yaml in %s: %v\n", sampleDir, err)
			continue
		}
		policyInfo.ConstraintYAML = string(constraintData)

		// Parse constraint to get the kind
		var constraint map[string]interface{}
		if err := yaml.Unmarshal(constraintData, &constraint); err == nil {
			if kind, ok := constraint["kind"].(string); ok {
				policyInfo.ConstraintKind = kind
			}
		}

		// Process example_disallowed.yaml - this becomes a "fix" task
		disallowedPath := filepath.Join(sampleDir, "example_disallowed.yaml")
		if _, err := os.Stat(disallowedPath); err == nil {
			disallowedData, err := os.ReadFile(disallowedPath)
			if err == nil {
				if err := generateFixTask(policyInfo, disallowedData, outputDir, sample.Name(), dryRun); err != nil {
					fmt.Printf("Warning: failed to generate fix task for %s: %v\n", sample.Name(), err)
				} else {
					generated++
				}
			}
		}
	}

	if generated > 0 {
		fmt.Printf("Generated %d tasks for %s/%s\n", generated, category, policyName)
	}
	return generated, nil
}

func generateFixTask(policyInfo PolicyInfo, resourceYAML []byte, outputDir, sampleName string, dryRun bool) error {
	// Parse the resource to extract metadata
	var resource map[string]interface{}
	if err := yaml.Unmarshal(resourceYAML, &resource); err != nil {
		return fmt.Errorf("parsing resource: %w", err)
	}

	// Extract resource info
	kind := "Pod"
	name := "unknown"
	namespace := "default"

	if k, ok := resource["kind"].(string); ok {
		kind = k
	}
	if meta, ok := resource["metadata"].(map[string]interface{}); ok {
		if n, ok := meta["name"].(string); ok {
			name = n
		}
		if ns, ok := meta["namespace"].(string); ok {
			namespace = ns
		}
	}

	// Use a consistent namespace for the task
	taskNamespace := fmt.Sprintf("gatekeeper-%s", sanitizeName(sampleName))

	// Create task name
	taskName := fmt.Sprintf("gatekeeper-fix-%s-%s", sanitizeName(policyInfo.Name), sanitizeName(sampleName))

	if dryRun {
		fmt.Printf("\n--- Would generate: %s ---\n", taskName)
		fmt.Printf("Policy: %s\n", policyInfo.Name)
		fmt.Printf("Resource: %s/%s in namespace %s\n", kind, name, taskNamespace)
		return nil
	}

	// Create task directory
	taskDir := filepath.Join(outputDir, taskName)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("creating task directory: %w", err)
	}

	// Update resource namespace
	resourceYAMLStr := string(resourceYAML)
	if namespace != taskNamespace {
		// Simple replacement - might need more sophisticated handling
		if strings.Contains(resourceYAMLStr, "namespace:") {
			resourceYAMLStr = regexp.MustCompile(`namespace:\s*\S+`).ReplaceAllString(resourceYAMLStr, "namespace: "+taskNamespace)
		} else {
			// Add namespace to metadata
			resourceYAMLStr = strings.Replace(resourceYAMLStr, "metadata:", "metadata:\n  namespace: "+taskNamespace, 1)
		}
	}

	// Write artifacts
	artifactsDir := filepath.Join(taskDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return fmt.Errorf("creating artifacts directory: %w", err)
	}

	if err := os.WriteFile(filepath.Join(artifactsDir, "template.yaml"), []byte(policyInfo.TemplateYAML), 0644); err != nil {
		return fmt.Errorf("writing template.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "constraint.yaml"), []byte(policyInfo.ConstraintYAML), 0644); err != nil {
		return fmt.Errorf("writing constraint.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "resource.yaml"), []byte(resourceYAMLStr), 0644); err != nil {
		return fmt.Errorf("writing resource.yaml: %w", err)
	}

	// Generate setup.sh
	setupScript := generateSetupScript(taskNamespace, kind, name)
	if err := os.WriteFile(filepath.Join(taskDir, "setup.sh"), []byte(setupScript), 0755); err != nil {
		return fmt.Errorf("writing setup.sh: %w", err)
	}

	// Generate verify.sh
	verifyScript := generateVerifyScript(taskNamespace, kind, name, policyInfo.ConstraintKind)
	if err := os.WriteFile(filepath.Join(taskDir, "verify.sh"), []byte(verifyScript), 0755); err != nil {
		return fmt.Errorf("writing verify.sh: %w", err)
	}

	// Generate cleanup.sh
	cleanupScript := generateCleanupScript(taskNamespace, policyInfo.ConstraintKind)
	if err := os.WriteFile(filepath.Join(taskDir, "cleanup.sh"), []byte(cleanupScript), 0755); err != nil {
		return fmt.Errorf("writing cleanup.sh: %w", err)
	}

	// Generate task.yaml
	prompt := fmt.Sprintf(`A %s named '%s' in namespace '%s' is violating a Gatekeeper policy.

**Policy Requirement:**
%s

Please investigate the policy violation and fix the %s to make it compliant.
You can use 'kubectl get constraint' to see the constraint and its violations.`,
		kind, name, taskNamespace, policyInfo.Description, kind)

	task := Task{
		Script: []ScriptStep{
			{Prompt: prompt},
		},
		Setup:      "setup.sh",
		Verifier:   "verify.sh",
		Cleanup:    "cleanup.sh",
		Difficulty: "medium",
	}

	taskData, err := yaml.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshaling task: %w", err)
	}

	if err := os.WriteFile(filepath.Join(taskDir, "task.yaml"), taskData, 0644); err != nil {
		return fmt.Errorf("writing task.yaml: %w", err)
	}

	return nil
}

func generateSetupScript(namespace, kind, name string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="%s"

# Ensure Gatekeeper is installed
echo "Ensuring Gatekeeper is installed..."
"${SCRIPT_DIR}/../scripts/ensure-gatekeeper.sh"

# Create namespace
kubectl delete namespace "$NAMESPACE" --ignore-not-found --wait=true 2>/dev/null || true
kubectl create namespace "$NAMESPACE"

# Apply constraint template
echo "Applying constraint template..."
kubectl apply -f "${SCRIPT_DIR}/artifacts/template.yaml"

# Wait for template to be ready
sleep 2

# Apply constraint
echo "Applying constraint..."
kubectl apply -f "${SCRIPT_DIR}/artifacts/constraint.yaml"

# Wait for constraint to be ready
sleep 2

# Apply the non-compliant resource
echo "Applying non-compliant resource..."
kubectl apply -f "${SCRIPT_DIR}/artifacts/resource.yaml" 2>/dev/null || true

# Wait for audit to detect the violation
echo "Waiting for Gatekeeper to audit the resource..."
sleep 5

# Trigger audit
kubectl annotate constraint --all audit.gatekeeper.sh/trigger=manual --overwrite 2>/dev/null || true
sleep 3

echo "Setup complete. Resource %s/%s should have policy violations."
`, namespace, kind, name)
}

func generateVerifyScript(namespace, kind, name, constraintKind string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -e

NAMESPACE="%s"
KIND="%s"
NAME="%s"
CONSTRAINT_KIND="%s"
TIMEOUT=60

echo "Verifying that %s/%s is now compliant..."

# Check the resource exists
if ! kubectl get "$KIND" "$NAME" -n "$NAMESPACE" &>/dev/null; then
    echo "FAIL: Resource $KIND/$NAME not found in namespace $NAMESPACE"
    exit 1
fi

# Trigger Gatekeeper audit
echo "Triggering Gatekeeper audit..."
kubectl annotate constraint --all audit.gatekeeper.sh/trigger=manual --overwrite 2>/dev/null || true
sleep 5

# Check for violations
echo "Checking for policy violations..."

# Get violations from the constraint
VIOLATIONS=$(kubectl get "$CONSTRAINT_KIND" -o jsonpath='{range .items[*]}{.status.violations}{"\n"}{end}' 2>/dev/null || echo "")

# Check if our resource is in the violations
if echo "$VIOLATIONS" | grep -q "\"name\":\"$NAME\"" && echo "$VIOLATIONS" | grep -q "\"namespace\":\"$NAMESPACE\""; then
    echo "FAIL: Resource $KIND/$NAME still has policy violations"
    echo "Violations found:"
    kubectl get "$CONSTRAINT_KIND" -o jsonpath='{range .items[*]}{.status.violations}{"\n"}{end}' | grep "$NAME" || true
    exit 1
fi

echo "SUCCESS: Resource $KIND/$NAME is compliant with the policy"
exit 0
`, namespace, kind, name, constraintKind, kind, name)
}

func generateCleanupScript(namespace, constraintKind string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
# Cleanup script - remove test resources but keep Gatekeeper installed

NAMESPACE="%s"
CONSTRAINT_KIND="%s"

echo "Cleaning up test resources..."

# Delete the namespace (removes all resources in it)
kubectl delete namespace "$NAMESPACE" --ignore-not-found --wait=false

# Delete the constraint
kubectl delete "$CONSTRAINT_KIND" --all --ignore-not-found 2>/dev/null || true

# Note: We keep the constraint template and Gatekeeper installed
# as other tasks may use them

echo "Cleanup complete"
`, namespace, constraintKind)
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
