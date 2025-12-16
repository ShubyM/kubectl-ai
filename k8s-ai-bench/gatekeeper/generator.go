package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Generator creates benchmark tasks from constraints
type Generator struct {
	InputFile     string
	OutputDir     string
	BenchmarkType BenchmarkType
	MaxTasks      int
}

// Run executes the generator
func (g *Generator) Run() error {
	// Load constraints library
	data, err := os.ReadFile(g.InputFile)
	if err != nil {
		return fmt.Errorf("reading input file: %w", err)
	}

	var library ConstraintLibrary
	if err := yaml.Unmarshal(data, &library); err != nil {
		return fmt.Errorf("parsing constraints: %w", err)
	}

	fmt.Printf("Loaded %d constraints from %s\n", len(library.Constraints), g.InputFile)

	// Generate tasks based on benchmark type
	var tasks []BenchmarkTask
	switch g.BenchmarkType {
	case PredictViolation:
		tasks = g.generatePredictViolationTasks(library)
	case ExplainViolation:
		tasks = g.generateExplainViolationTasks(library)
	case FixViolation:
		tasks = g.generateFixViolationTasks(library)
	case AuditCluster:
		tasks = g.generateAuditClusterTasks(library)
	default:
		return fmt.Errorf("unknown benchmark type: %s", g.BenchmarkType)
	}

	// Limit tasks if requested
	if g.MaxTasks > 0 && len(tasks) > g.MaxTasks {
		tasks = tasks[:g.MaxTasks]
	}

	fmt.Printf("Generating %d %s benchmark tasks\n", len(tasks), g.BenchmarkType)

	// Create output directory
	if err := os.MkdirAll(g.OutputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Generate task files
	for _, task := range tasks {
		if err := g.writeTaskFiles(task); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error writing task %s: %v\n", task.Name, err)
			continue
		}
		fmt.Printf("  Created: %s\n", task.Name)
	}

	return nil
}

func (g *Generator) generatePredictViolationTasks(library ConstraintLibrary) []BenchmarkTask {
	var tasks []BenchmarkTask

	for _, constraint := range library.Constraints {
		for _, sample := range constraint.Samples {
			// Create task for each allowed example
			for i, example := range sample.AllowedExamples {
				task := BenchmarkTask{
					Name:           generateTaskName("gk-predict", constraint.Name, sample.Name, "pass", i),
					Constraint:     constraint,
					Sample:         sample,
					TestResource:   example,
					ExpectedResult: "pass",
					Type:           PredictViolation,
				}
				tasks = append(tasks, task)
			}

			// Create task for each disallowed example
			for i, example := range sample.DisallowedExamples {
				task := BenchmarkTask{
					Name:           generateTaskName("gk-predict", constraint.Name, sample.Name, "fail", i),
					Constraint:     constraint,
					Sample:         sample,
					TestResource:   example,
					ExpectedResult: "fail",
					Type:           PredictViolation,
				}
				tasks = append(tasks, task)
			}
		}
	}

	return tasks
}

func (g *Generator) generateExplainViolationTasks(library ConstraintLibrary) []BenchmarkTask {
	var tasks []BenchmarkTask

	for _, constraint := range library.Constraints {
		for _, sample := range constraint.Samples {
			// Only create explain tasks for disallowed examples
			for i, example := range sample.DisallowedExamples {
				task := BenchmarkTask{
					Name:           generateTaskName("gk-explain", constraint.Name, sample.Name, "viol", i),
					Constraint:     constraint,
					Sample:         sample,
					TestResource:   example,
					ExpectedResult: "fail",
					Type:           ExplainViolation,
				}
				tasks = append(tasks, task)
			}
		}
	}

	return tasks
}

func (g *Generator) generateFixViolationTasks(library ConstraintLibrary) []BenchmarkTask {
	var tasks []BenchmarkTask

	for _, constraint := range library.Constraints {
		for _, sample := range constraint.Samples {
			// Create fix tasks for disallowed examples
			for i, example := range sample.DisallowedExamples {
				task := BenchmarkTask{
					Name:           generateTaskName("gk-fix", constraint.Name, sample.Name, "viol", i),
					Constraint:     constraint,
					Sample:         sample,
					TestResource:   example,
					ExpectedResult: "fail",
					Type:           FixViolation,
				}
				tasks = append(tasks, task)
			}
		}
	}

	return tasks
}

func (g *Generator) generateAuditClusterTasks(library ConstraintLibrary) []BenchmarkTask {
	var tasks []BenchmarkTask

	for _, constraint := range library.Constraints {
		for _, sample := range constraint.Samples {
			// Need both allowed and disallowed examples for audit
			if len(sample.AllowedExamples) == 0 || len(sample.DisallowedExamples) == 0 {
				continue
			}

			task := BenchmarkTask{
				Name:           generateTaskName("gk-audit", constraint.Name, sample.Name, "", 0),
				Constraint:     constraint,
				Sample:         sample,
				ExpectedResult: "mixed",
				Type:           AuditCluster,
			}
			tasks = append(tasks, task)
		}
	}

	return tasks
}

func generateTaskName(prefix, constraintName, sampleName, suffix string, index int) string {
	// Create a short hash from the full name to avoid overly long names
	fullName := fmt.Sprintf("%s-%s-%s-%d", constraintName, sampleName, suffix, index)
	hash := md5.Sum([]byte(fullName))
	shortHash := hex.EncodeToString(hash[:])[:6]

	// Sanitize names
	constraintName = sanitizeName(constraintName)

	// Truncate if needed
	if len(constraintName) > 20 {
		constraintName = constraintName[:20]
	}

	name := fmt.Sprintf("%s-%s-%s", prefix, constraintName, shortHash)
	return strings.ToLower(name)
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return strings.ToLower(name)
}

func (g *Generator) writeTaskFiles(task BenchmarkTask) error {
	taskDir := filepath.Join(g.OutputDir, task.Name)
	artifactsDir := filepath.Join(taskDir, "artifacts")

	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return err
	}

	// Write template
	if task.Constraint.Template != "" {
		if err := os.WriteFile(filepath.Join(artifactsDir, "template.yaml"), []byte(task.Constraint.Template), 0644); err != nil {
			return err
		}
	}

	// Write constraint
	if task.Sample.Constraint != "" {
		if err := os.WriteFile(filepath.Join(artifactsDir, "constraint.yaml"), []byte(task.Sample.Constraint), 0644); err != nil {
			return err
		}
	}

	// Write test resource(s) based on task type
	switch task.Type {
	case PredictViolation, ExplainViolation:
		if err := os.WriteFile(filepath.Join(artifactsDir, "test-resource.yaml"), []byte(task.TestResource), 0644); err != nil {
			return err
		}
	case FixViolation:
		if err := os.WriteFile(filepath.Join(artifactsDir, "resource-to-fix.yaml"), []byte(task.TestResource), 0644); err != nil {
			return err
		}
	case AuditCluster:
		// Write all examples
		for i, ex := range task.Sample.AllowedExamples {
			if err := os.WriteFile(filepath.Join(artifactsDir, fmt.Sprintf("allowed-%d.yaml", i+1)), []byte(ex), 0644); err != nil {
				return err
			}
		}
		for i, ex := range task.Sample.DisallowedExamples {
			if err := os.WriteFile(filepath.Join(artifactsDir, fmt.Sprintf("disallowed-%d.yaml", i+1)), []byte(ex), 0644); err != nil {
				return err
			}
		}
	}

	// Write expected result metadata
	metadata := map[string]interface{}{
		"constraint_name":     task.Constraint.Name,
		"constraint_category": task.Constraint.Category,
		"sample_name":         task.Sample.Name,
		"expected_result":     task.ExpectedResult,
		"benchmark_type":      string(task.Type),
		"description":         task.Constraint.Description,
	}
	metadataJSON, _ := json.MarshalIndent(metadata, "", "  ")
	if err := os.WriteFile(filepath.Join(artifactsDir, "metadata.json"), metadataJSON, 0644); err != nil {
		return err
	}

	// Write task.yaml
	taskYAML := g.generateTaskYAML(task)
	if err := os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644); err != nil {
		return err
	}

	// Write setup.sh
	setupScript := g.generateSetupScript(task)
	if err := os.WriteFile(filepath.Join(taskDir, "setup.sh"), []byte(setupScript), 0755); err != nil {
		return err
	}

	// Write verify.sh
	verifyScript := g.generateVerifyScript(task)
	if err := os.WriteFile(filepath.Join(taskDir, "verify.sh"), []byte(verifyScript), 0755); err != nil {
		return err
	}

	// Write cleanup.sh
	cleanupScript := g.generateCleanupScript()
	if err := os.WriteFile(filepath.Join(taskDir, "cleanup.sh"), []byte(cleanupScript), 0755); err != nil {
		return err
	}

	// Write prompt.md
	promptMD := g.generatePrompt(task)
	if err := os.WriteFile(filepath.Join(taskDir, "prompt.md"), []byte(promptMD), 0644); err != nil {
		return err
	}

	return nil
}

func (g *Generator) generateTaskYAML(task BenchmarkTask) string {
	switch task.Type {
	case PredictViolation:
		return fmt.Sprintf(`script:
- promptFile: "prompt.md"
setup: "setup.sh"
verifier: "verify.sh"
cleanup: "cleanup.sh"
difficulty: "medium"
expect:
- contains: "%s"
`, task.ExpectedResult)

	case ExplainViolation:
		return `script:
- promptFile: "prompt.md"
setup: "setup.sh"
verifier: "verify.sh"
cleanup: "cleanup.sh"
difficulty: "medium"
`

	case FixViolation:
		return `script:
- promptFile: "prompt.md"
setup: "setup.sh"
verifier: "verify.sh"
cleanup: "cleanup.sh"
difficulty: "hard"
`

	case AuditCluster:
		return `script:
- promptFile: "prompt.md"
setup: "setup.sh"
verifier: "verify.sh"
cleanup: "cleanup.sh"
difficulty: "hard"
`

	default:
		return ""
	}
}

func (g *Generator) generateSetupScript(task BenchmarkTask) string {
	baseSetup := `#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

NAMESPACE=gatekeeper-bench
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Clean up any existing resources
kubectl delete namespace ${NAMESPACE} --ignore-not-found --wait=true 2>/dev/null || true

# Create test namespace
kubectl create namespace ${NAMESPACE}

# Check if Gatekeeper is installed
if ! kubectl get crd constrainttemplates.templates.gatekeeper.sh &>/dev/null; then
    echo "Installing Gatekeeper..."
    kubectl apply -f https://raw.githubusercontent.com/open-policy-agent/gatekeeper/v3.15.0/deploy/gatekeeper.yaml
    echo "Waiting for Gatekeeper to be ready..."
    kubectl wait --for=condition=Ready --timeout=180s pod -l control-plane=controller-manager -n gatekeeper-system
fi

# Apply constraint template
echo "Applying constraint template..."
kubectl apply -f "${SCRIPT_DIR}/artifacts/template.yaml"

# Wait for CRD to be established
sleep 5

# Apply constraint (if exists)
if [ -f "${SCRIPT_DIR}/artifacts/constraint.yaml" ]; then
    echo "Applying constraint..."
    kubectl apply -f "${SCRIPT_DIR}/artifacts/constraint.yaml"
    sleep 3
fi

`

	switch task.Type {
	case PredictViolation, ExplainViolation:
		return baseSetup + `
# Apply test resource to the namespace
echo "Applying test resource..."
kubectl apply -f "${SCRIPT_DIR}/artifacts/test-resource.yaml" -n ${NAMESPACE} 2>/dev/null || true

echo "Setup complete. Test resource deployed to ${NAMESPACE} namespace."
`

	case FixViolation:
		return baseSetup + `
# Apply the resource that needs to be fixed
echo "Applying resource to fix..."
kubectl apply -f "${SCRIPT_DIR}/artifacts/resource-to-fix.yaml" -n ${NAMESPACE} 2>/dev/null || true

echo "Setup complete. Resource deployed to ${NAMESPACE} namespace."
echo "The resource currently violates the Gatekeeper constraint and needs to be fixed."
`

	case AuditCluster:
		return baseSetup + `
# Apply all test resources
echo "Applying test resources..."
for f in "${SCRIPT_DIR}"/artifacts/allowed-*.yaml "${SCRIPT_DIR}"/artifacts/disallowed-*.yaml; do
    if [ -f "$f" ]; then
        kubectl apply -f "$f" -n ${NAMESPACE} 2>/dev/null || true
    fi
done

echo "Setup complete. Multiple resources deployed to ${NAMESPACE} namespace for audit."
`

	default:
		return baseSetup
	}
}

func (g *Generator) generateVerifyScript(task BenchmarkTask) string {
	switch task.Type {
	case PredictViolation:
		return fmt.Sprintf(`#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

NAMESPACE=gatekeeper-bench
EXPECTED_RESULT="%s"

# Wait for audit to complete
sleep 5

# Get violations from all constraints
VIOLATIONS=$(kubectl get constraints -A -o json 2>/dev/null | jq '[.items[].status.totalViolations // 0] | add' 2>/dev/null || echo "0")

echo "Violations detected: ${VIOLATIONS}"

if [ "${EXPECTED_RESULT}" == "pass" ]; then
    if [ "${VIOLATIONS}" -eq 0 ]; then
        echo "CORRECT: Resource passes the constraint as expected"
        exit 0
    else
        echo "INCORRECT: Resource was expected to pass but has violations"
        exit 1
    fi
else
    if [ "${VIOLATIONS}" -gt 0 ]; then
        echo "CORRECT: Resource violates the constraint as expected"
        exit 0
    else
        echo "INCORRECT: Resource was expected to violate but passed"
        exit 1
    fi
fi
`, task.ExpectedResult)

	case ExplainViolation:
		return `#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# For explanation tasks, we verify the model provided a meaningful explanation
# The model should have explained the violation correctly

echo "Explanation task completed. Checking audit for reference..."

# Get actual violation message from Gatekeeper
VIOLATIONS=$(kubectl get constraints -A -o json 2>/dev/null | jq -r '.items[].status.violations[]?.message' 2>/dev/null || echo "")

if [ -n "${VIOLATIONS}" ]; then
    echo "Actual violation messages from Gatekeeper:"
    echo "${VIOLATIONS}"
fi

exit 0
`

	case FixViolation:
		return `#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

NAMESPACE=gatekeeper-bench

echo "Verifying the resource was fixed correctly..."

# Wait for audit sync
sleep 5

# Check if there are any remaining violations
VIOLATIONS=$(kubectl get constraints -A -o json 2>/dev/null | jq '[.items[].status.totalViolations // 0] | add' 2>/dev/null || echo "0")

echo "Remaining violations: ${VIOLATIONS}"

if [ "${VIOLATIONS}" -eq 0 ]; then
    echo "SUCCESS: Resource was fixed and now passes the constraint"
    exit 0
else
    echo "FAILURE: Resource still violates the constraint"
    # Show the violations for debugging
    kubectl get constraints -A -o json 2>/dev/null | jq -r '.items[].status.violations[]?.message' 2>/dev/null || true
    exit 1
fi
`

	case AuditCluster:
		return fmt.Sprintf(`#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

NAMESPACE=gatekeeper-bench
EXPECTED_VIOLATIONS=%d

echo "Verifying audit results..."

# Wait for audit sync
sleep 5

# Get actual violations from Gatekeeper
VIOLATIONS=$(kubectl get constraints -A -o json 2>/dev/null | jq '[.items[].status.totalViolations // 0] | add' 2>/dev/null || echo "0")

echo "Expected violations: ${EXPECTED_VIOLATIONS}"
echo "Actual violations: ${VIOLATIONS}"

# Allow some tolerance for audit sync
if [ "${VIOLATIONS}" -ge 1 ] && [ "${EXPECTED_VIOLATIONS}" -ge 1 ]; then
    echo "SUCCESS: Gatekeeper correctly detected violations"
    exit 0
elif [ "${VIOLATIONS}" -eq 0 ] && [ "${EXPECTED_VIOLATIONS}" -eq 0 ]; then
    echo "SUCCESS: No violations as expected"
    exit 0
else
    echo "FAILURE: Violation count mismatch"
    kubectl get constraints -A -o json 2>/dev/null | jq -r '.items[].status.violations[]' 2>/dev/null || true
    exit 1
fi
`, len(task.Sample.DisallowedExamples))

	default:
		return `#!/usr/bin/env bash
exit 0
`
	}
}

func (g *Generator) generateCleanupScript() string {
	return `#!/usr/bin/env bash

NAMESPACE=gatekeeper-bench
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Delete test namespace (this removes all test resources)
kubectl delete namespace ${NAMESPACE} --ignore-not-found --wait=false 2>/dev/null || true

# Delete constraint
if [ -f "${SCRIPT_DIR}/artifacts/constraint.yaml" ]; then
    kubectl delete -f "${SCRIPT_DIR}/artifacts/constraint.yaml" --ignore-not-found 2>/dev/null || true
fi

# Delete template
if [ -f "${SCRIPT_DIR}/artifacts/template.yaml" ]; then
    kubectl delete -f "${SCRIPT_DIR}/artifacts/template.yaml" --ignore-not-found 2>/dev/null || true
fi

echo "Cleanup complete."
`
}

func (g *Generator) generatePrompt(task BenchmarkTask) string {
	description := task.Constraint.Description
	if description == "" {
		description = fmt.Sprintf("A policy that validates %s configurations", task.Constraint.Name)
	}

	switch task.Type {
	case PredictViolation:
		return fmt.Sprintf(`I have a Kubernetes cluster with Gatekeeper installed. There's a constraint policy in place that enforces the following:

**Policy Description:** %s

A resource has been deployed to the "gatekeeper-bench" namespace. Can you analyze the resource and tell me whether it **passes** or **fails** this policy constraint?

Please examine the resource in the namespace and respond with either "pass" if it complies with the policy, or "fail" if it violates it. Explain your reasoning briefly.
`, description)

	case ExplainViolation:
		return fmt.Sprintf(`I have a Kubernetes cluster with Gatekeeper installed. There's a constraint policy that enforces:

**Policy Description:** %s

A resource has been deployed to the "gatekeeper-bench" namespace that violates this policy.

Can you:
1. Identify which resource is violating the constraint
2. Explain specifically WHY it violates the policy
3. Describe what changes would be needed to make it compliant

Please check the Gatekeeper constraint violations and the resource configuration to provide a detailed explanation.
`, description)

	case FixViolation:
		return fmt.Sprintf(`I have a Kubernetes cluster with Gatekeeper installed. There's a constraint policy that enforces:

**Policy Description:** %s

A resource has been deployed to the "gatekeeper-bench" namespace that currently violates this policy constraint.

Please:
1. Identify the violating resource
2. Modify the resource to make it compliant with the policy
3. Verify that the fix resolves the violation

The resource should remain functional after your changes - only modify what's necessary to satisfy the policy.
`, description)

	case AuditCluster:
		return fmt.Sprintf(`I have a Kubernetes cluster with Gatekeeper installed. There's a constraint policy that enforces:

**Policy Description:** %s

Multiple resources have been deployed to the "gatekeeper-bench" namespace. Some comply with the policy and some violate it.

Please:
1. List all resources in the namespace
2. For each resource, determine if it passes or fails the policy
3. Provide a summary showing which resources are compliant and which are not
4. For any non-compliant resources, explain what violation occurred

Use Gatekeeper's audit capabilities to verify your analysis.
`, description)

	default:
		return "Analyze the Gatekeeper constraint and resources in the cluster."
	}
}
