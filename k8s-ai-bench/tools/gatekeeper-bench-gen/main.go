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

// gatekeeper-bench-gen generates compliance benchmarks from the OPA Gatekeeper library.
// It scrapes constraint templates and examples to create test cases that evaluate
// a model's ability to identify Kubernetes resource policy violations.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"sigs.k8s.io/yaml"
)

const (
	gatekeeperRepoURL = "https://github.com/open-policy-agent/gatekeeper-library.git"
	defaultOutputDir  = "tasks"
)

// ConstraintTemplate represents the structure of a Gatekeeper constraint template
type ConstraintTemplate struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name        string `yaml:"name"`
		Annotations struct {
			Description string `yaml:"description"`
			Metadata    string `yaml:"metadata"`
		} `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		CRD struct {
			Spec struct {
				Names struct {
					Kind string `yaml:"kind"`
				} `yaml:"names"`
				Validation struct {
					OpenAPIV3Schema struct {
						Type       string                            `yaml:"type"`
						Properties map[string]map[string]interface{} `yaml:"properties"`
					} `yaml:"openAPIV3Schema"`
				} `yaml:"validation"`
			} `yaml:"spec"`
		} `yaml:"crd"`
	} `yaml:"spec"`
}

// Constraint represents a Gatekeeper constraint instance
type Constraint struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Match struct {
			Kinds []struct {
				APIGroups []string `yaml:"apiGroups"`
				Kinds     []string `yaml:"kinds"`
			} `yaml:"kinds"`
			ExcludedNamespaces []string `yaml:"excludedNamespaces"`
			Namespaces         []string `yaml:"namespaces"`
		} `yaml:"match"`
		Parameters map[string]interface{} `yaml:"parameters"`
	} `yaml:"spec"`
}

// Sample represents a sample constraint with its examples
type Sample struct {
	Name         string
	ConstraintPath string
	Constraint   Constraint
	Allowed      []Resource
	Disallowed   []Resource
}

// Resource represents a Kubernetes resource from an example file
type Resource struct {
	Name     string
	Kind     string
	Content  string
	FileName string
}

// clusterScopedKinds are Kubernetes resources that are cluster-scoped (not namespaced)
var clusterScopedKinds = map[string]bool{
	"Namespace":                      true,
	"Node":                           true,
	"PersistentVolume":               true,
	"ClusterRole":                    true,
	"ClusterRoleBinding":             true,
	"CustomResourceDefinition":       true,
	"StorageClass":                   true,
	"PriorityClass":                  true,
	"VolumeAttachment":               true,
	"CSIDriver":                      true,
	"CSINode":                        true,
	"RuntimeClass":                   true,
	"PodSecurityPolicy":              true,
	"MutatingWebhookConfiguration":   true,
	"ValidatingWebhookConfiguration": true,
}

// BenchmarkTask represents a generated benchmark task
type BenchmarkTask struct {
	Name                string
	Category            string
	Description         string
	Message             string
	Parameters          map[string]interface{}
	MatchedKinds        []string
	AllowedResources    []Resource
	DisallowedResources []Resource
	HasClusterScoped    bool // true if any resource is cluster-scoped
}

// isClusterScoped returns true if the given kind is cluster-scoped
func isClusterScoped(kind string) bool {
	return clusterScopedKinds[kind]
}

// deduplicateResources renames resources with duplicate Kind/Name combinations
func deduplicateResources(resources []Resource) []Resource {
	seen := make(map[string]int)
	var result []Resource

	for _, res := range resources {
		key := fmt.Sprintf("%s/%s", res.Kind, res.Name)
		count := seen[key]
		seen[key] = count + 1

		if count > 0 {
			// Rename the resource to make it unique
			newName := fmt.Sprintf("%s-%d", res.Name, count+1)
			res = renameResource(res, newName)
		}
		result = append(result, res)
	}

	return result
}

// renameResource updates a resource's name in both the struct and YAML content
func renameResource(res Resource, newName string) Resource {
	// Update the YAML content to reflect the new name
	var obj map[string]interface{}
	if err := yaml.Unmarshal([]byte(res.Content), &obj); err != nil {
		return res
	}

	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return res
	}
	metadata["name"] = newName

	newContent, err := yaml.Marshal(obj)
	if err != nil {
		return res
	}

	return Resource{
		Name:     newName,
		Kind:     res.Kind,
		Content:  string(newContent),
		FileName: res.FileName,
	}
}

// hasClusterScopedResources checks if any resource in the list is cluster-scoped
func hasClusterScopedResources(resources []Resource) bool {
	for _, res := range resources {
		if isClusterScoped(res.Kind) {
			return true
		}
	}
	return false
}

// uniqueStrings returns a deduplicated slice of strings
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func main() {
	outputDir := flag.String("output", defaultOutputDir, "Output directory for generated benchmarks")
	repoDir := flag.String("repo", "", "Path to existing gatekeeper-library clone (will clone if not provided)")
	categoryFilter := flag.String("category", "", "Filter by category (general, pod-security-policy)")
	constraintFilter := flag.String("constraint", "", "Filter by constraint name (regex)")
	flag.Parse()

	// Clone or use existing repo
	libraryPath := *repoDir
	if libraryPath == "" {
		tempDir, err := os.MkdirTemp("", "gatekeeper-library-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create temp directory: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(tempDir)

		fmt.Println("Cloning gatekeeper-library...")
		cmd := exec.Command("git", "clone", "--depth", "1", gatekeeperRepoURL, tempDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to clone repository: %v\n", err)
			os.Exit(1)
		}
		libraryPath = tempDir
	}

	libraryDir := filepath.Join(libraryPath, "library")

	// Process categories
	categories := []string{"general", "pod-security-policy"}
	if *categoryFilter != "" {
		categories = []string{*categoryFilter}
	}

	var constraintRegex *regexp.Regexp
	if *constraintFilter != "" {
		var err error
		constraintRegex, err = regexp.Compile(*constraintFilter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid constraint filter regex: %v\n", err)
			os.Exit(1)
		}
	}

	totalGenerated := 0
	for _, category := range categories {
		categoryPath := filepath.Join(libraryDir, category)
		if _, err := os.Stat(categoryPath); os.IsNotExist(err) {
			fmt.Printf("Category %s not found, skipping\n", category)
			continue
		}

		entries, err := os.ReadDir(categoryPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read category %s: %v\n", category, err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			constraintName := entry.Name()
			if constraintRegex != nil && !constraintRegex.MatchString(constraintName) {
				continue
			}

			constraintPath := filepath.Join(categoryPath, constraintName)
			tasks, err := processConstraint(constraintPath, category, constraintName)
			if err != nil {
				fmt.Printf("Warning: Failed to process %s/%s: %v\n", category, constraintName, err)
				continue
			}

			for _, task := range tasks {
				if err := generateBenchmark(*outputDir, task); err != nil {
					fmt.Printf("Warning: Failed to generate benchmark for %s: %v\n", task.Name, err)
					continue
				}
				totalGenerated++
			}
		}
	}

	fmt.Printf("\nGenerated %d benchmarks in %s\n", totalGenerated, *outputDir)
}

func processConstraint(constraintPath, category, constraintName string) ([]BenchmarkTask, error) {
	// Read template.yaml
	templatePath := filepath.Join(constraintPath, "template.yaml")
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("reading template.yaml: %w", err)
	}

	var ct ConstraintTemplate
	if err := yaml.Unmarshal(templateData, &ct); err != nil {
		return nil, fmt.Errorf("parsing template.yaml: %w", err)
	}

	// Process samples
	samplesDir := filepath.Join(constraintPath, "samples")
	if _, err := os.Stat(samplesDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("no samples directory found")
	}

	sampleEntries, err := os.ReadDir(samplesDir)
	if err != nil {
		return nil, fmt.Errorf("reading samples directory: %w", err)
	}

	var tasks []BenchmarkTask
	for _, sampleEntry := range sampleEntries {
		if !sampleEntry.IsDir() {
			continue
		}

		samplePath := filepath.Join(samplesDir, sampleEntry.Name())
		sample, err := processSample(samplePath, sampleEntry.Name())
		if err != nil {
			fmt.Printf("  Warning: Failed to process sample %s: %v\n", sampleEntry.Name(), err)
			continue
		}

		// Skip samples with no disallowed resources (nothing to detect)
		if len(sample.Disallowed) == 0 {
			fmt.Printf("  Skipping sample %s: no disallowed resources\n", sampleEntry.Name())
			continue
		}

		// Deduplicate resources to avoid naming conflicts
		allResources := append(sample.Allowed, sample.Disallowed...)
		deduped := deduplicateResources(allResources)

		// Split back into allowed and disallowed
		allowedDeduped := deduped[:len(sample.Allowed)]
		disallowedDeduped := deduped[len(sample.Allowed):]

		// Check if we have cluster-scoped resources
		hasClusterScoped := hasClusterScopedResources(deduped)

		// Build task
		task := BenchmarkTask{
			Name:                fmt.Sprintf("gk-%s-%s", category, sampleEntry.Name()),
			Category:            category,
			Description:         ct.Metadata.Annotations.Description,
			Message:             getConstraintMessage(sample.Constraint),
			Parameters:          sample.Constraint.Spec.Parameters,
			MatchedKinds:        getMatchedKinds(sample.Constraint),
			AllowedResources:    allowedDeduped,
			DisallowedResources: disallowedDeduped,
			HasClusterScoped:    hasClusterScoped,
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func processSample(samplePath, sampleName string) (*Sample, error) {
	sample := &Sample{
		Name: sampleName,
	}

	// Read constraint.yaml
	constraintPath := filepath.Join(samplePath, "constraint.yaml")
	constraintData, err := os.ReadFile(constraintPath)
	if err != nil {
		return nil, fmt.Errorf("reading constraint.yaml: %w", err)
	}

	if err := yaml.Unmarshal(constraintData, &sample.Constraint); err != nil {
		return nil, fmt.Errorf("parsing constraint.yaml: %w", err)
	}
	sample.ConstraintPath = constraintPath

	// Read example files
	entries, err := os.ReadDir(samplePath)
	if err != nil {
		return nil, fmt.Errorf("reading sample directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		if !strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, ".yml") {
			continue
		}

		// Skip constraint.yaml
		if fileName == "constraint.yaml" {
			continue
		}

		filePath := filepath.Join(samplePath, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		resources := parseResources(content, fileName)

		if strings.Contains(fileName, "allowed") && !strings.Contains(fileName, "disallowed") {
			sample.Allowed = append(sample.Allowed, resources...)
		} else if strings.Contains(fileName, "disallowed") {
			sample.Disallowed = append(sample.Disallowed, resources...)
		}
	}

	return sample, nil
}

func parseResources(content []byte, fileName string) []Resource {
	var resources []Resource

	// Split by YAML document separator
	docs := strings.Split(string(content), "---")
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Parse to get kind and name
		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			continue
		}

		kind, _ := obj["kind"].(string)
		metadata, _ := obj["metadata"].(map[string]interface{})
		name, _ := metadata["name"].(string)

		if kind != "" && name != "" {
			resources = append(resources, Resource{
				Name:     name,
				Kind:     kind,
				Content:  doc,
				FileName: fileName,
			})
		}
	}

	return resources
}

func getConstraintMessage(c Constraint) string {
	if c.Spec.Parameters == nil {
		return ""
	}
	if msg, ok := c.Spec.Parameters["message"].(string); ok {
		return msg
	}
	return ""
}

func getMatchedKinds(c Constraint) []string {
	var kinds []string
	for _, match := range c.Spec.Match.Kinds {
		kinds = append(kinds, match.Kinds...)
	}
	return kinds
}

func generateBenchmark(outputDir string, task BenchmarkTask) error {
	taskDir := filepath.Join(outputDir, task.Name)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("creating task directory: %w", err)
	}

	artifactsDir := filepath.Join(taskDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return fmt.Errorf("creating artifacts directory: %w", err)
	}

	// Write resource files
	allResources := append(task.AllowedResources, task.DisallowedResources...)
	var resourcesContent strings.Builder
	for i, res := range allResources {
		if i > 0 {
			resourcesContent.WriteString("\n---\n")
		}
		resourcesContent.WriteString(res.Content)
	}

	if err := os.WriteFile(filepath.Join(artifactsDir, "resources.yaml"), []byte(resourcesContent.String()), 0644); err != nil {
		return fmt.Errorf("writing resources.yaml: %w", err)
	}

	// Write expected violations file (for verification reference)
	var violations []string
	for _, res := range task.DisallowedResources {
		violations = append(violations, fmt.Sprintf("%s/%s", res.Kind, res.Name))
	}
	violationsContent := strings.Join(violations, "\n")
	if err := os.WriteFile(filepath.Join(artifactsDir, "expected_violations.txt"), []byte(violationsContent), 0644); err != nil {
		return fmt.Errorf("writing expected_violations.txt: %w", err)
	}

	// Generate setup.sh
	if err := generateSetupScript(taskDir, task); err != nil {
		return fmt.Errorf("generating setup.sh: %w", err)
	}

	// Generate cleanup.sh
	if err := generateCleanupScript(taskDir, task); err != nil {
		return fmt.Errorf("generating cleanup.sh: %w", err)
	}

	// Generate verify.sh
	if err := generateVerifyScript(taskDir, task); err != nil {
		return fmt.Errorf("generating verify.sh: %w", err)
	}

	// Generate task.yaml
	if err := generateTaskYAML(taskDir, task); err != nil {
		return fmt.Errorf("generating task.yaml: %w", err)
	}

	fmt.Printf("Generated benchmark: %s\n", task.Name)
	return nil
}

// setupTemplateNamespaced deploys namespaced resources to a dedicated namespace
var setupTemplateNamespaced = template.Must(template.New("setup-ns").Parse(`#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="{{.Namespace}}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Cleanup existing namespace if present
kubectl delete namespace "$NAMESPACE" --ignore-not-found --wait=true 2>/dev/null || true

# Create namespace
kubectl create namespace "$NAMESPACE"

# Apply resources
kubectl apply -f "$SCRIPT_DIR/artifacts/resources.yaml" -n "$NAMESPACE"

# Wait for pods to be ready (if any)
kubectl wait --for=condition=Ready pods --all -n "$NAMESPACE" --timeout=60s 2>/dev/null || true

echo "Setup complete. Resources deployed to namespace $NAMESPACE"
`))

// setupTemplateClusterScoped deploys cluster-scoped resources directly
var setupTemplateClusterScoped = template.Must(template.New("setup-cluster").Parse(`#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Cleanup existing resources
{{range .CleanupCommands}}
{{.}}
{{end}}

# Apply resources
kubectl apply -f "$SCRIPT_DIR/artifacts/resources.yaml"

echo "Setup complete. Cluster-scoped resources deployed."
`))

func generateSetupScript(taskDir string, task BenchmarkTask) error {
	namespace := strings.ReplaceAll(task.Name, "_", "-")
	if len(namespace) > 63 {
		namespace = namespace[:63]
	}

	f, err := os.Create(filepath.Join(taskDir, "setup.sh"))
	if err != nil {
		return err
	}
	defer f.Close()

	if task.HasClusterScoped {
		// For cluster-scoped resources, generate cleanup commands
		var cleanupCommands []string
		allResources := append(task.AllowedResources, task.DisallowedResources...)
		for _, res := range allResources {
			cleanupCommands = append(cleanupCommands,
				fmt.Sprintf(`kubectl delete %s %s --ignore-not-found 2>/dev/null || true`, strings.ToLower(res.Kind), res.Name))
		}

		data := struct {
			CleanupCommands []string
		}{
			CleanupCommands: cleanupCommands,
		}

		if err := setupTemplateClusterScoped.Execute(f, data); err != nil {
			return err
		}
	} else {
		data := struct {
			Namespace string
		}{
			Namespace: namespace,
		}

		if err := setupTemplateNamespaced.Execute(f, data); err != nil {
			return err
		}
	}

	return os.Chmod(filepath.Join(taskDir, "setup.sh"), 0755)
}

// cleanupTemplateNamespaced removes the namespace and all resources
var cleanupTemplateNamespaced = template.Must(template.New("cleanup-ns").Parse(`#!/usr/bin/env bash
kubectl delete namespace "{{.Namespace}}" --ignore-not-found --wait=false
`))

// cleanupTemplateClusterScoped removes cluster-scoped resources
var cleanupTemplateClusterScoped = template.Must(template.New("cleanup-cluster").Parse(`#!/usr/bin/env bash
{{range .CleanupCommands}}
{{.}}
{{end}}
`))

func generateCleanupScript(taskDir string, task BenchmarkTask) error {
	namespace := strings.ReplaceAll(task.Name, "_", "-")
	if len(namespace) > 63 {
		namespace = namespace[:63]
	}

	f, err := os.Create(filepath.Join(taskDir, "cleanup.sh"))
	if err != nil {
		return err
	}
	defer f.Close()

	if task.HasClusterScoped {
		var cleanupCommands []string
		allResources := append(task.AllowedResources, task.DisallowedResources...)
		for _, res := range allResources {
			cleanupCommands = append(cleanupCommands,
				fmt.Sprintf(`kubectl delete %s %s --ignore-not-found 2>/dev/null || true`, strings.ToLower(res.Kind), res.Name))
		}

		data := struct {
			CleanupCommands []string
		}{
			CleanupCommands: cleanupCommands,
		}

		if err := cleanupTemplateClusterScoped.Execute(f, data); err != nil {
			return err
		}
	} else {
		data := struct {
			Namespace string
		}{
			Namespace: namespace,
		}

		if err := cleanupTemplateNamespaced.Execute(f, data); err != nil {
			return err
		}
	}

	return os.Chmod(filepath.Join(taskDir, "cleanup.sh"), 0755)
}

var verifyTemplate = template.Must(template.New("verify").Parse(`#!/usr/bin/env bash
set -euo pipefail

# This script verifies that the model correctly identified policy violations.
# The model's output should identify the following resources as violations:
{{range .Violations}}
# - {{.Kind}}/{{.Name}}
{{end}}

# Read the model's output from stdin or first argument
MODEL_OUTPUT="${1:-}"

if [ -z "$MODEL_OUTPUT" ]; then
    echo "Error: No model output provided"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXPECTED_VIOLATIONS="$SCRIPT_DIR/artifacts/expected_violations.txt"

# Check that all expected violations are mentioned in the output
MISSING=0
while IFS= read -r violation; do
    # Extract resource name from Kind/Name format
    RESOURCE_NAME=$(echo "$violation" | cut -d'/' -f2)
    if ! echo "$MODEL_OUTPUT" | grep -qi "$RESOURCE_NAME"; then
        echo "Missing violation: $violation"
        MISSING=$((MISSING + 1))
    fi
done < "$EXPECTED_VIOLATIONS"

if [ $MISSING -gt 0 ]; then
    echo "Failed: $MISSING expected violations not identified"
    exit 1
fi

# Check for false positives (allowed resources marked as violations)
{{range .Allowed}}
if echo "$MODEL_OUTPUT" | grep -qi "{{.Name}}.*violat"; then
    echo "False positive: {{.Kind}}/{{.Name}} was incorrectly marked as a violation"
    # This is a warning, not a failure
fi
{{end}}

echo "Verification passed: All expected violations were correctly identified"
exit 0
`))

func generateVerifyScript(taskDir string, task BenchmarkTask) error {
	data := struct {
		Violations []Resource
		Allowed    []Resource
	}{
		Violations: task.DisallowedResources,
		Allowed:    task.AllowedResources,
	}

	f, err := os.Create(filepath.Join(taskDir, "verify.sh"))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := verifyTemplate.Execute(f, data); err != nil {
		return err
	}

	return os.Chmod(filepath.Join(taskDir, "verify.sh"), 0755)
}

func generateTaskYAML(taskDir string, task BenchmarkTask) error {
	// Build the prompt
	prompt := buildPrompt(task)

	// Build expect patterns - look for violation mentions (deduplicated)
	var expectPatterns []map[string]string
	seenNames := make(map[string]bool)
	for _, res := range task.DisallowedResources {
		if seenNames[res.Name] {
			continue
		}
		seenNames[res.Name] = true
		// Use case-insensitive regex pattern
		expectPatterns = append(expectPatterns, map[string]string{
			"contains": fmt.Sprintf("(?i)%s", regexp.QuoteMeta(res.Name)),
		})
	}

	taskYAML := map[string]interface{}{
		"script": []map[string]string{
			{"prompt": prompt},
		},
		"setup":      "setup.sh",
		"cleanup":    "cleanup.sh",
		"difficulty": "medium",
		"expect":     expectPatterns,
	}

	data, err := yaml.Marshal(taskYAML)
	if err != nil {
		return fmt.Errorf("marshaling task.yaml: %w", err)
	}

	return os.WriteFile(filepath.Join(taskDir, "task.yaml"), data, 0644)
}

func buildPrompt(task BenchmarkTask) string {
	var sb strings.Builder

	sb.WriteString("You are reviewing Kubernetes resources for policy compliance.\n\n")

	// Add constraint description
	if task.Description != "" {
		sb.WriteString("## Policy Rule\n")
		sb.WriteString(task.Description)
		sb.WriteString("\n\n")
	}

	// Add custom message if present
	if task.Message != "" {
		sb.WriteString("## Policy Message\n")
		sb.WriteString(task.Message)
		sb.WriteString("\n\n")
	}

	// Add parameters if present
	if len(task.Parameters) > 0 {
		sb.WriteString("## Policy Parameters\n")
		paramsYAML, _ := yaml.Marshal(task.Parameters)
		sb.WriteString("```yaml\n")
		sb.WriteString(string(paramsYAML))
		sb.WriteString("```\n\n")
	}

	// Add resource kinds being evaluated
	if len(task.MatchedKinds) > 0 {
		sb.WriteString("## Applicable Resource Types\n")
		sb.WriteString("This policy applies to: ")
		sb.WriteString(strings.Join(task.MatchedKinds, ", "))
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Task\n")
	if task.HasClusterScoped {
		sb.WriteString("Inspect the cluster-scoped ")
		sb.WriteString(strings.Join(task.MatchedKinds, "/"))
		sb.WriteString(" resources and identify which ones violate the policy described above.\n\n")
	} else {
		namespace := strings.ReplaceAll(task.Name, "_", "-")
		if len(namespace) > 63 {
			namespace = namespace[:63]
		}
		sb.WriteString(fmt.Sprintf("Inspect the resources in the `%s` namespace and identify which ones violate the policy described above.\n\n", namespace))
	}

	sb.WriteString("List each violating resource in the format: `Kind/Name` with a brief explanation of why it violates the policy.\n")
	sb.WriteString("If a resource is compliant, you do not need to list it.\n")

	return sb.String()
}
