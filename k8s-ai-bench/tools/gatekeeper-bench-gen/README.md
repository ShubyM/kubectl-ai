# Gatekeeper Benchmark Generator

This tool generates compliance benchmarks from the [OPA Gatekeeper Library](https://github.com/open-policy-agent/gatekeeper-library) to test model abilities in Kubernetes policy compliance.

## How It Works

The generator:
1. Clones the gatekeeper-library repository
2. Parses constraint templates to extract policy descriptions
3. Processes sample resources (allowed and disallowed examples)
4. Generates benchmark tasks that deploy resources to a cluster

## Benchmark Design

Each generated benchmark:
- **setup.sh**: Deploys example resources to the cluster (namespace or cluster-scoped)
- **task.yaml**: Prompts the model to inspect the cluster and identify policy violations
- **verify.sh**: Validates the model correctly identified violating resources
- **cleanup.sh**: Removes deployed resources
- **artifacts/**: Contains resource definitions and expected violations

The model must use `kubectl` to inspect the deployed resources and determine which ones violate the policy.

## Usage

### Build

```bash
cd k8s-ai-bench/tools/gatekeeper-bench-gen
GOWORK=off go build -o gatekeeper-bench-gen .
```

### Generate Benchmarks

```bash
# Generate all benchmarks to default location
./gatekeeper-bench-gen -output ../../tasks-gatekeeper

# Generate benchmarks for a specific category
./gatekeeper-bench-gen -output ../../tasks-gatekeeper -category general
./gatekeeper-bench-gen -output ../../tasks-gatekeeper -category pod-security-policy

# Generate benchmarks matching a pattern
./gatekeeper-bench-gen -output ../../tasks-gatekeeper -constraint "privileged"

# Use an existing clone of gatekeeper-library
./gatekeeper-bench-gen -output ../../tasks-gatekeeper -repo /path/to/gatekeeper-library
```

### Run Benchmarks

```bash
# From k8s-ai-bench directory
TEST_ARGS="--task-pattern gk-" make run-evals
```

## Categories

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

## Example Generated Task

```yaml
script:
- prompt: |
    You are reviewing Kubernetes resources for policy compliance.

    ## Policy Rule
    Controls the ability of any container to enable privileged mode.

    ## Policy Parameters
    exemptImages:
    - safeimages.com/*

    ## Applicable Resource Types
    This policy applies to: Pod

    ## Task
    Inspect the resources in the `gk-pod-security-policy-psp-privileged-container`
    namespace and identify which ones violate the policy described above.

    List each violating resource in the format: `Kind/Name` with a brief explanation.
setup: setup.sh
cleanup: cleanup.sh
difficulty: medium
expect:
- contains: (?i)nginx-privileged-disallowed
```

## Notes

- Generated tasks are not committed to git (see `.gitignore`)
- No Gatekeeper installation is required - only the example resources are deployed
- The model inspects real cluster resources via kubectl
