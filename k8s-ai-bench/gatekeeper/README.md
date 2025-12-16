# Gatekeeper Policy Compliance Tasks

This directory contains infrastructure for generating k8s-ai-bench tasks from the
[OPA Gatekeeper Library](https://github.com/open-policy-agent/gatekeeper-library).

## Overview

The Gatekeeper library provides a collection of policy constraints for Kubernetes
clusters. Each constraint includes example resources that either comply with or
violate the policy.

This generator scrapes those examples and creates benchmark tasks that test whether
AI models can correctly determine if a Kubernetes resource complies with a given
policy requirement - **without showing them the actual Gatekeeper/Rego policy code**.

Instead, the policy is described in natural language, testing the model's understanding
of Kubernetes security best practices.

## Quick Start

```bash
# Generate all tasks
make generate

# Preview what would be generated
make dry-run

# Update Gatekeeper library and regenerate
make update
```

## Generated Task Format

Each generated task presents:

1. **Policy Requirement** - A natural language description of what the policy enforces
2. **Resource to Review** - A Kubernetes resource YAML
3. **Question** - Whether the resource complies with the policy

Example task prompt:
```
You are a Kubernetes security reviewer checking resources for policy compliance.

**Policy Requirement:**
Containers must NOT run in privileged mode. The securityContext.privileged
field must be set to false or omitted.

**Resource to Review:**
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx-privileged
spec:
  containers:
  - name: nginx
    image: nginx
    securityContext:
      privileged: true
```

Does this resource COMPLY with the policy requirement above?

First, answer with exactly "COMPLIANT" or "NON-COMPLIANT".
Then briefly explain your reasoning.
```

## Directory Structure

```
gatekeeper/
├── main.go          # Generator entry point and CLI
├── types.go         # Data structures
├── policies.go      # Policy name to description mapping
├── Makefile         # Build and generation commands
├── README.md        # This file
└── tasks/           # Generated tasks (gitignored)
    └── gatekeeper-*/
        └── task.yaml
```

## Policy Categories

Tasks are generated from two categories:

### General Policies
- `requiredlabels` - Enforce required labels
- `httpsonly` - Require HTTPS on Ingress
- `containerlimits` - Require resource limits
- `disallowedtags` - Block certain image tags
- `allowedrepos` - Restrict image repositories
- And many more...

### Pod Security Policies
- `privileged-containers` - Block privileged mode
- `capabilities` - Restrict Linux capabilities
- `host-namespaces` - Block host namespace sharing
- `read-only-root-filesystem` - Require read-only root
- `allow-privilege-escalation` - Block privilege escalation
- And many more...

## Adding New Policy Descriptions

To add or improve policy descriptions, edit `policies.go`:

```go
var PolicyDescriptions = map[string]string{
    "policy-name": "Clear description of what the policy requires...",
}
```

Good descriptions should:
- Clearly state what is required or prohibited
- Reference specific Kubernetes fields when relevant
- Be actionable and unambiguous

## Running the Benchmark

After generating tasks, run the benchmark with:

```bash
cd ..
./k8s-ai-bench run --task-pattern "gatekeeper-*"
```

Or run all tasks including the generated ones:

```bash
./k8s-ai-bench run
```

## Updating Tasks

When the Gatekeeper library is updated with new policies or examples:

```bash
make update
```

This will:
1. Re-clone the Gatekeeper library
2. Remove existing generated tasks
3. Generate fresh tasks from the updated library
