# Gatekeeper Policy Compliance Tasks

This directory contains infrastructure for generating k8s-ai-bench tasks from the
[OPA Gatekeeper Library](https://github.com/open-policy-agent/gatekeeper-library).

## Overview

These tasks test whether AI models can fix Kubernetes resources that violate
Gatekeeper policies. Unlike simple Q&A tasks, these run against a real cluster
with Gatekeeper installed in audit mode to verify the fix.

**Task Flow:**
1. **Setup**: Install Gatekeeper, apply constraint template + constraint, deploy non-compliant resource
2. **Prompt**: Ask the LLM to fix the policy violation
3. **Verify**: Gatekeeper audit confirms the resource is now compliant
4. **Cleanup**: Remove test resources

## Quick Start

```bash
# Generate all tasks
make generate

# Preview what would be generated
make dry-run

# Update Gatekeeper library and regenerate
make update
```

## Requirements

- A Kubernetes cluster (kind, minikube, etc.)
- `kubectl` configured to access the cluster
- Internet access to install Gatekeeper

## Generated Task Structure

Each task includes:

```
gatekeeper-fix-<policy>-<sample>/
├── task.yaml           # Task definition with prompt
├── setup.sh           # Installs Gatekeeper, applies constraint, deploys bad resource
├── verify.sh          # Checks Gatekeeper audit shows no violations
├── cleanup.sh         # Removes test resources
└── artifacts/
    ├── template.yaml   # Gatekeeper ConstraintTemplate
    ├── constraint.yaml # The constraint being tested
    └── resource.yaml   # The non-compliant resource to fix
```

## Example Task

**Task:** `gatekeeper-fix-privileged-containers-psp-privileged-container`

**Prompt:**
```
A Pod named 'nginx-privileged-disallowed' in namespace 'gatekeeper-psp-privileged-container'
is violating a Gatekeeper policy.

**Policy Requirement:**
Containers must NOT run in privileged mode. The securityContext.privileged field
must be set to false or omitted.

Please investigate the policy violation and fix the Pod to make it compliant.
You can use 'kubectl get constraint' to see the constraint and its violations.
```

**What the LLM must do:**
- Understand the policy requirement
- Inspect the Pod and find `privileged: true`
- Patch or recreate the Pod with `privileged: false`

**Verification:**
- Gatekeeper audit runs and checks for violations
- If the Pod is fixed, no violations are reported → SUCCESS

## How Verification Works

The verify script:
1. Triggers a Gatekeeper audit cycle
2. Queries the constraint's `.status.violations`
3. Checks if the specific resource is listed as a violation
4. Returns success only if the resource is NOT in the violations list

This means:
- The LLM must actually fix the resource, not just describe the fix
- Gatekeeper (not regex matching) determines correctness
- The same verification Gatekeeper uses in production

## Policy Categories

Tasks are generated from two categories:

### General Policies (~20 tasks)
- `containerlimits` - Require resource limits
- `requiredlabels` - Enforce required labels
- `httpsonly` - Require HTTPS on Ingress
- `disallowedtags` - Block certain image tags
- `block-nodeport-services` - Disallow NodePort
- And more...

### Pod Security Policies (~25 tasks)
- `privileged-containers` - Block privileged mode
- `capabilities` - Restrict Linux capabilities
- `host-namespaces` - Block host namespace sharing
- `read-only-root-filesystem` - Require read-only root
- `allow-privilege-escalation` - Block privilege escalation
- `seccomp` - Require Seccomp profiles
- And more...

## Running the Benchmark

After generating tasks:

```bash
# Run just Gatekeeper tasks
cd ..
./k8s-ai-bench run --task-pattern "gatekeeper-fix-*"

# Run a specific task
./k8s-ai-bench run --task-pattern "gatekeeper-fix-privileged-containers"
```

## Adding Custom Policy Descriptions

Edit `policies.go` to improve how policies are described to the LLM:

```go
var PolicyDescriptions = map[string]string{
    "privileged-containers": "Containers must NOT run in privileged mode...",
}
```

Good descriptions should:
- Clearly state what is required or prohibited
- Reference specific Kubernetes fields
- Be actionable without revealing the exact fix

## Updating Tasks

When the Gatekeeper library adds new policies:

```bash
make update
```

This will:
1. Re-clone the Gatekeeper library
2. Clean existing generated tasks
3. Generate fresh tasks

## Architecture

```
gatekeeper/
├── main.go              # Generator CLI
├── types.go             # Data structures
├── policies.go          # Human-readable policy descriptions
├── Makefile             # Build commands
├── scripts/
│   ├── ensure-gatekeeper.sh   # Idempotent Gatekeeper installer
│   └── check-violations.sh    # Violation checker utility
└── tasks/               # Generated tasks (gitignored)
```
