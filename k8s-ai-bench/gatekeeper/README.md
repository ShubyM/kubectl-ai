# Gatekeeper Policy Compliance Tasks

This directory contains infrastructure for generating k8s-ai-bench tasks from the
[OPA Gatekeeper Library](https://github.com/open-policy-agent/gatekeeper-library).

## Overview

These tasks test whether AI models can correctly identify policy violations in
Kubernetes resources - the same compliance checking that Gatekeeper performs.

**Task Flow:**
1. **Setup**: Install Gatekeeper, apply constraint, deploy resource with violation
2. **Prompt**: Ask the LLM to identify which resources violate the policy
3. **Verify**: Check LLM mentions the violating resource AND Gatekeeper agrees

## Quick Start

```bash
# Generate all tasks
make generate

# Preview what would be generated
make dry-run

# Update Gatekeeper library and regenerate
make update
```

## Example Task

**Task:** `gatekeeper-check-privileged-containers-psp-privileged-container`

**Prompt:**
```
Check the resources in namespace 'gatekeeper-psp-privileged-container' for policy compliance.

**Policy Requirement:**
Containers must NOT run in privileged mode. The securityContext.privileged field
must be set to false or omitted.

List any resources that are violating this policy. For each violation, state the
resource name and what specifically violates the policy.

If there are no violations, state that all resources are compliant.
```

**What the LLM must do:**
- Query the namespace for resources
- Identify that `nginx-privileged-disallowed` has `privileged: true`
- Report the violation

**Verification:**
- `expect` checks the LLM output contains the resource name
- `verify.sh` confirms Gatekeeper also detected this violation

## Generated Task Structure

```
gatekeeper-check-<policy>-<sample>/
├── task.yaml           # Prompt + expect for resource name
├── setup.sh           # Install Gatekeeper, apply constraint, deploy bad resource
├── verify.sh          # Confirm Gatekeeper detected the same violation
├── cleanup.sh         # Remove test resources
└── artifacts/
    ├── template.yaml   # ConstraintTemplate
    ├── constraint.yaml # The constraint
    └── resource.yaml   # Resource with violation
```

## Policy Categories

### General Policies (~20 tasks)
- `containerlimits` - Missing resource limits
- `requiredlabels` - Missing required labels
- `httpsonly` - Ingress without HTTPS
- `block-nodeport-services` - NodePort service type
- And more...

### Pod Security Policies (~25 tasks)
- `privileged-containers` - Privileged mode enabled
- `capabilities` - Dangerous capabilities added
- `host-namespaces` - Host namespace sharing
- `read-only-root-filesystem` - Writable root filesystem
- And more...

## Running the Benchmark

```bash
# Run just Gatekeeper compliance tasks
cd ..
./k8s-ai-bench run --task-pattern "gatekeeper-check-*"

# Run a specific task
./k8s-ai-bench run --task-pattern "gatekeeper-check-privileged"
```

## Requirements

- Kubernetes cluster (kind, minikube, etc.)
- `kubectl` configured
- Internet access to install Gatekeeper

## Architecture

```
gatekeeper/
├── main.go              # Generator
├── types.go             # Data structures
├── policies.go          # Human-readable policy descriptions
├── Makefile             # Build commands
├── scripts/
│   └── ensure-gatekeeper.sh   # Idempotent Gatekeeper installer
└── tasks/               # Generated tasks (gitignored)
```
