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

// gatekeeper-gen generates k8s-ai-bench tasks from the Gatekeeper library.
// It scrapes constraint templates and their examples to create compliance
// checking tasks where AI models must determine if a resource complies
// with a given policy.
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
)

// ConstraintTemplate represents a Gatekeeper constraint template
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

// Task represents a k8s-ai-bench task
type Task struct {
	Script     []ScriptStep `json:"script"`
	Difficulty string       `json:"difficulty"`
	Expect     []Expect     `json:"expect"`
}

type ScriptStep struct {
	Prompt string `json:"prompt"`
}

type Expect struct {
	Contains string `json:"contains"`
}

// PolicyInfo contains extracted policy information
type PolicyInfo struct {
	Name            string
	Category        string
	Description     string
	Title           string
	TemplateContent string
}

func main() {
	var (
		repoDir   string
		outputDir string
		skipClone bool
		dryRun    bool
	)

	flag.StringVar(&repoDir, "repo-dir", defaultRepoDir, "Directory to clone/use the Gatekeeper library")
	flag.StringVar(&outputDir, "output-dir", "./tasks", "Output directory for generated tasks")
	flag.BoolVar(&skipClone, "skip-clone", false, "Skip cloning the repository (use existing)")
	flag.BoolVar(&dryRun, "dry-run", false, "Print what would be generated without creating files")
	flag.Parse()

	if err := run(repoDir, outputDir, skipClone, dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(repoDir, outputDir string, skipClone, dryRun bool) error {
	// Step 1: Clone or update the Gatekeeper library
	if !skipClone {
		if err := cloneOrUpdateRepo(repoDir); err != nil {
			return fmt.Errorf("failed to clone/update repo: %w", err)
		}
	}

	// Step 2: Find all constraint templates
	libraryDir := filepath.Join(repoDir, "library")
	if _, err := os.Stat(libraryDir); os.IsNotExist(err) {
		return fmt.Errorf("library directory not found at %s", libraryDir)
	}

	// Walk through categories (general, pod-security-policy)
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
		// If pull fails, just warn and continue
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
		Name:            policyName,
		Category:        category,
		Description:     template.Metadata.Annotations.Description,
		Title:           template.Metadata.Annotations.Metadata,
		TemplateContent: string(templateData),
	}

	// If no description, try to extract from metadata or use a default
	if policyInfo.Description == "" {
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

	// Walk through sample directories
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
	taskName := fmt.Sprintf("gatekeeper-%s-%s-%s", sanitizeName(policyInfo.Name), sanitizeName(sampleName), compliance)

	// Generate natural language description
	description := generateNaturalLanguageDescription(policyInfo)

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
Then briefly explain your reasoning.`, description, "```yaml\n"+string(exampleData)+"```")

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
		fmt.Printf("Description: %s\n", description)
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

func generateNaturalLanguageDescription(info PolicyInfo) string {
	// Map of policy names to human-readable descriptions
	descriptions := map[string]string{
		"privileged-containers":        "Containers must NOT run in privileged mode. The securityContext.privileged field must be set to false or omitted.",
		"host-network-ports":           "Pods must NOT use host networking or bind to host ports. The hostNetwork field must be false and hostPort must not be specified.",
		"host-namespaces":              "Pods must NOT share the host's PID, IPC, or network namespaces. hostPID, hostIPC, and hostNetwork must all be false.",
		"host-filesystem":              "Pods must NOT mount sensitive host filesystem paths. hostPath volumes should be restricted or disallowed.",
		"read-only-root-filesystem":    "Containers must use a read-only root filesystem. The securityContext.readOnlyRootFilesystem must be set to true.",
		"allow-privilege-escalation":   "Containers must NOT allow privilege escalation. The securityContext.allowPrivilegeEscalation must be set to false.",
		"capabilities":                 "Container capabilities must be restricted. Containers should drop all capabilities and only add specific required ones.",
		"users":                        "Containers must run as a non-root user. The runAsNonRoot field must be true and runAsUser must be set to a non-zero value.",
		"seccomp":                      "Pods must have an appropriate Seccomp profile configured.",
		"apparmor":                     "Pods must have an appropriate AppArmor profile configured.",
		"selinux":                      "Pods must have appropriate SELinux options configured.",
		"proc-mount":                   "The /proc mount type must be Default, not Unmasked.",
		"volumes":                      "Only specific volume types are allowed. Pods should not use hostPath or other dangerous volume types.",
		"fsgroup":                      "Pods must configure fsGroup appropriately.",
		"flexvolume-drivers":           "FlexVolume drivers must be from an approved list.",
		"forbidden-sysctls":            "Pods must not set forbidden sysctls.",
		"httpsonly":                    "Ingress resources must use HTTPS. TLS must be configured and HTTP must be explicitly disabled.",
		"requiredlabels":               "Resources must have all required labels with values matching specified patterns.",
		"requiredannotations":          "Resources must have all required annotations.",
		"requiredprobes":               "Containers must define readiness and/or liveness probes.",
		"containerlimits":              "Containers must specify resource limits (CPU and memory).",
		"containerrequests":            "Containers must specify resource requests (CPU and memory).",
		"containerresources":           "Containers must specify both resource requests and limits.",
		"containerresourceratios":      "Container resource limits must be within a specified ratio of requests.",
		"allowedrepos":                 "Container images must come from allowed repositories.",
		"disallowedrepos":              "Container images must NOT come from disallowed repositories.",
		"disallowedtags":               "Container images must NOT use disallowed tags (like 'latest').",
		"imagedigests":                 "Container images must be specified by digest, not tag.",
		"replicalimits":                "Deployments/ReplicaSets must have replica counts within allowed ranges.",
		"block-nodeport-services":      "Services must NOT use NodePort type.",
		"block-loadbalancer-services":  "Services must NOT use LoadBalancer type.",
		"block-wildcard-ingress":       "Ingress resources must NOT use wildcard hosts.",
		"uniqueingresshost":            "Ingress hosts must be unique across the cluster.",
		"uniqueserviceselector":        "Service selectors must be unique.",
		"externalip":                   "Services must NOT use external IPs, or only approved ones.",
		"storageclass":                 "PersistentVolumeClaims must use allowed storage classes.",
		"poddisruptionbudget":          "PodDisruptionBudgets must be configured appropriately.",
		"horizontalpodautoscaler":      "HorizontalPodAutoscalers must have appropriate min/max replicas.",
		"disallowanonymous":            "RBAC bindings must NOT grant permissions to anonymous users or the system:unauthenticated group.",
		"block-endpoint-edit-default-role": "The default system roles must NOT be modified to allow endpoint editing.",
		"verifydeprecatedapi":          "Resources must NOT use deprecated API versions.",
		"noupdateserviceaccount":       "Pod service accounts must NOT be changed after creation.",
		"disallowinteractive":          "Pods must NOT have interactive TTY or stdin enabled.",
		"automount-serviceaccount-token": "Pods should explicitly configure service account token automounting.",
		"ephemeralstoragelimit":        "Containers must specify ephemeral storage limits.",
	}

	// Try to find a matching description
	if desc, ok := descriptions[info.Name]; ok {
		return desc
	}

	// Fall back to the template description if available
	if info.Description != "" {
		return info.Description
	}

	// Generate from name
	return generateDescriptionFromName(info.Name)
}

func generateDescriptionFromName(name string) string {
	// Convert kebab-case to readable format
	words := strings.Split(name, "-")
	for i, word := range words {
		words[i] = strings.Title(strings.ToLower(word))
	}
	return fmt.Sprintf("Resources must comply with the %s policy.", strings.Join(words, " "))
}

func formatTitle(name string) string {
	words := strings.Split(name, "-")
	for i, word := range words {
		words[i] = strings.Title(strings.ToLower(word))
	}
	return strings.Join(words, " ")
}

func sanitizeName(name string) string {
	// Remove or replace characters that might cause issues in directory names
	reg := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	name = reg.ReplaceAllString(name, "-")
	// Remove consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	name = reg.ReplaceAllString(name, "-")
	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")
	return strings.ToLower(name)
}
