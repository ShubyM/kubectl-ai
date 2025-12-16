# Gatekeeper Constraint Benchmarks

This directory contains tools for generating AI benchmarks based on [OPA Gatekeeper](https://github.com/open-policy-agent/gatekeeper) constraint policies.

## Overview

The Gatekeeper benchmark suite tests whether AI models can:
1. **Predict violations** - Determine if a Kubernetes resource violates a constraint
2. **Explain violations** - Explain why a resource violates a policy
3. **Fix violations** - Modify a resource to make it compliant
4. **Audit clusters** - Analyze multiple resources and identify violations

## How It Works

1. **Scrape**: Fetches constraint templates and examples from the [gatekeeper-library](https://github.com/open-policy-agent/gatekeeper-library) repository
2. **Generate**: Creates benchmark tasks using the fetched constraints and examples
3. **Run**: Benchmarks are run using the k8s-ai-bench framework with Gatekeeper installed in the cluster

## Prerequisites

- Go 1.22+
- kubectl with access to a Kubernetes cluster
- Gatekeeper installed in the cluster (or the setup scripts will install it)

## Quick Start

```bash
# Build the tool
make build

# Scrape constraints from Gatekeeper library
make scrape

# Generate a sample set of benchmarks
make generate-sample

# Or run the full workflow
make all
```

## Commands

### Scrape

Fetches constraint templates and examples from the OPA Gatekeeper library:

```bash
./gatekeeper-bench scrape -output ./constraints -categories "general,pod-security-policy"
```

Options:
- `-output`: Output directory for fetched constraints (default: `./constraints`)
- `-categories`: Comma-separated list of categories to fetch (default: `general,pod-security-policy`)
- `-format`: Output format, `yaml` or `json` (default: `yaml`)

### Generate

Creates benchmark tasks from the fetched constraints:

```bash
./gatekeeper-bench generate -input ./constraints/constraints.yaml -type predict-violation -max 20
```

Options:
- `-input`: Path to the constraints file (default: `./constraints/constraints.yaml`)
- `-output`: Output directory for generated tasks (default: `../tasks`)
- `-type`: Benchmark type to generate
- `-max`: Maximum number of tasks to generate (0 = unlimited)

#### Benchmark Types

| Type | Difficulty | Description |
|------|------------|-------------|
| `predict-violation` | Medium | Model predicts if a resource passes or fails a constraint |
| `explain-violation` | Medium | Model explains why a resource violates a constraint |
| `fix-violation` | Hard | Model fixes a violating resource to make it compliant |
| `audit-cluster` | Hard | Model audits multiple resources and identifies violations |

## Generated Task Structure

Each generated benchmark task follows this structure:

```
gk-predict-allowedrepos-abc123/
├── task.yaml           # Task definition
├── prompt.md           # Prompt for the AI model
├── setup.sh            # Setup script (installs Gatekeeper, applies constraint)
├── verify.sh           # Verification script (checks Gatekeeper audit)
├── cleanup.sh          # Cleanup script
└── artifacts/
    ├── template.yaml   # Gatekeeper constraint template
    ├── constraint.yaml # Constraint configuration
    ├── test-resource.yaml  # Resource to test
    └── metadata.json   # Benchmark metadata
```

## Verification

Benchmarks are verified using Gatekeeper's audit functionality:

1. Resources are deployed to a test namespace
2. Gatekeeper audits the resources against the constraint
3. The verify script checks the audit results
4. For `predict-violation` tasks, the model's answer is compared to actual violations
5. For `fix-violation` tasks, the script verifies no violations remain

## Example Usage

### Running a Single Benchmark

```bash
# From k8s-ai-bench directory
./k8s-ai-bench run --task-pattern gk-predict --agent-bin ./kubectl-ai --output-dir .build
```

### Running All Gatekeeper Benchmarks

```bash
./k8s-ai-bench run --task-pattern "gk-" --agent-bin ./kubectl-ai --output-dir .build
```

## Constraint Categories

### General Policies
- `allowedrepos` - Restrict container images to allowed repositories
- `requiredlabels` - Require specific labels on resources
- `containerlimits` - Enforce container resource limits
- `httpsonly` - Require HTTPS for ingress
- And many more...

### Pod Security Policies
- `privileged-containers` - Block privileged containers
- `host-namespaces` - Block host namespace access
- `capabilities` - Restrict container capabilities
- `read-only-root-filesystem` - Require read-only root filesystem
- And many more...

## Sources

- [OPA Gatekeeper](https://github.com/open-policy-agent/gatekeeper)
- [Gatekeeper Library](https://github.com/open-policy-agent/gatekeeper-library)
- [Gatekeeper Library Documentation](https://open-policy-agent.github.io/gatekeeper-library/website/)
