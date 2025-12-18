# Gatekeeper Compliance Benchmarks

This directory contains benchmarks generated from the [OPA Gatekeeper Library](https://github.com/open-policy-agent/gatekeeper-library). These benchmarks test a model's ability to identify Kubernetes resource policy violations.

## Benchmark Structure

Each benchmark includes:
- `task.yaml` - Task definition with prompt and expected output patterns
- `setup.sh` - Minimal setup script (no cluster resources needed)
- `cleanup.sh` - Minimal cleanup script
- `artifacts/` - Directory containing:
  - `resources.yaml` - The Kubernetes resources being analyzed
  - `expected_violations.txt` - List of resources that violate the policy

## How These Benchmarks Work

Unlike other k8s-ai-bench tasks that test actions on a live cluster, these benchmarks test **YAML analysis capabilities**:

1. The prompt includes:
   - A policy description from the Gatekeeper constraint template
   - Optional policy parameters
   - The resource types the policy applies to
   - Complete YAML definitions of resources to analyze

2. The model must identify which resources violate the policy

3. The `expect` field validates that the model correctly identified violating resources

## Categories

Benchmarks are organized by Gatekeeper library category:

### General Policies (`gk-general-*`)
- Required labels/annotations
- Container resource limits and requests
- Image repository restrictions
- Ingress/networking policies
- Deprecated API detection

### Pod Security Policies (`gk-pod-security-policy-*`)
- Privileged containers
- Host namespaces/filesystem access
- Security contexts (SELinux, seccomp, capabilities)
- Volume restrictions

## Generating Benchmarks

The benchmarks are generated using the `gatekeeper-bench-gen` tool:

```bash
# From the k8s-ai-bench directory
cd tools/gatekeeper-bench-gen
GOWORK=off go build -o gatekeeper-bench-gen .

# Generate all benchmarks
./gatekeeper-bench-gen -output ../../tasks-gatekeeper

# Generate benchmarks for a specific category
./gatekeeper-bench-gen -output ../../tasks-gatekeeper -category general

# Generate benchmarks matching a pattern
./gatekeeper-bench-gen -output ../../tasks-gatekeeper -constraint "privileged"
```

## Running Benchmarks

```bash
# From k8s-ai-bench directory
TEST_ARGS="--task-pattern gk-" make run-evals
```

## Example Task

Here's an example of what a benchmark prompt looks like:

```
You are reviewing Kubernetes resources for policy compliance.

## Policy Rule
Controls the ability of any container to enable privileged mode.

## Policy Parameters
exemptImages:
- safeimages.com/*

## Applicable Resource Types
This policy applies to: Pod

## Resources to Review
[YAML resources are embedded here]

## Task
Review the resources above and identify which ones violate the policy.
List each violating resource in the format: `Kind/Name` with a brief explanation.
```

## Contributing

To add new benchmarks or improve existing ones:
1. Update the generator in `tools/gatekeeper-bench-gen/`
2. Regenerate benchmarks
3. Test with `make run-evals`
