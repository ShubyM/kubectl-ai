# Agent Sandbox Integration

This document describes how to use the Kubernetes agent-sandbox with kubectl-ai for enhanced security and isolation.

## Overview

[agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox) is a Kubernetes subproject under SIG Apps that provides a declarative API for managing isolated, stateful, singleton workloads. It's specifically designed for AI agent runtimes and offers:

- **Sub-second latency**: Up to 90% improvement over cold starts using pre-warming pools
- **Enhanced security**: Support for gVisor and Kata Containers runtimes
- **Declarative management**: Native Kubernetes CRD-based API
- **Scalability**: Handles thousands of concurrent sandboxes

## Prerequisites

1. **Kubernetes cluster** with agent-sandbox installed:
   ```bash
   kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/v0.1.0/manifest.yaml
   ```

2. **RuntimeClass** (optional, for enhanced isolation):
   ```yaml
   apiVersion: node.k8s.io/v1
   kind: RuntimeClass
   metadata:
     name: gvisor
   handler: runsc
   ```

   Or for Kata Containers:
   ```yaml
   apiVersion: node.k8s.io/v1
   kind: RuntimeClass
   metadata:
     name: kata
   handler: kata
   ```

3. **RBAC permissions**: kubectl-ai needs permissions to create/delete Sandbox resources:
   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: kubectl-ai-agent-sandbox
   rules:
   - apiGroups: ["agents.x-k8s.io"]
     resources: ["sandboxes"]
     verbs: ["create", "get", "list", "delete", "watch"]
   - apiGroups: [""]
     resources: ["pods"]
     verbs: ["get", "list"]
   - apiGroups: [""]
     resources: ["configmaps"]
     verbs: ["create", "delete"]
   ```

## Usage

### Basic Usage

Use the `--sandbox=agent-sandbox` flag to enable agent-sandbox:

```bash
kubectl-ai --sandbox=agent-sandbox "list all pods in the default namespace"
```

### With Custom Image

Specify a custom container image:

```bash
kubectl-ai --sandbox=agent-sandbox \
  --sandbox-image=my-registry/kubectl:v1.28.0 \
  "show me cluster info"
```

### With Enhanced Isolation (gVisor)

Use gVisor for additional security:

```bash
kubectl-ai --sandbox=agent-sandbox \
  --runtime-class=gvisor \
  "analyze the cluster resources"
```

### With Kata Containers

Use Kata Containers for VM-level isolation:

```bash
kubectl-ai --sandbox=agent-sandbox \
  --runtime-class=kata \
  "check pod security policies"
```

### Configuration File

Add to your `~/.config/kubectl-ai/config.yaml`:

```yaml
sandbox: agent-sandbox
sandboxImage: bitnami/kubectl:latest
runtimeClass: gvisor  # optional
```

Then run without flags:

```bash
kubectl-ai "describe deployment nginx"
```

## Comparison with Standard k8s Sandbox

| Feature | k8s (Standard Pod) | agent-sandbox |
|---------|-------------------|---------------|
| API | Pod (core/v1) | Sandbox (agents.x-k8s.io/v1alpha1) |
| Startup Time | 2-5 seconds | < 1 second (with warm pool) |
| Isolation | Container runtime default | Configurable (gVisor, Kata) |
| Lifecycle | Manual pod management | Declarative Sandbox CRD |
| Scaling | Manual | Built-in warm pool support |
| Purpose | General workloads | AI agent runtimes |

## Architecture

When using `--sandbox=agent-sandbox`, kubectl-ai:

1. Creates a `Sandbox` Custom Resource in the `computer` namespace
2. The agent-sandbox controller creates and manages the underlying pod
3. kubectl-ai executes commands via `kubectl exec` into the sandbox pod
4. On session close, the Sandbox resource is deleted (controller cleans up the pod)

```
┌─────────────┐
│  kubectl-ai │
└──────┬──────┘
       │ creates
       v
┌─────────────────┐
│ Sandbox CRD     │ (agents.x-k8s.io/v1alpha1)
│ Name: kubectl-  │
│   ai-sandbox-   │
│   <uuid>        │
└────────┬────────┘
         │ reconciled by
         v
┌─────────────────┐
│ agent-sandbox   │
│ controller      │
└────────┬────────┘
         │ creates/manages
         v
┌─────────────────┐
│ Pod             │
│ Runtime: gVisor │
│ or Kata         │
└─────────────────┘
```

## Security Benefits

### gVisor Runtime
- **Application kernel**: Implements Linux system calls in userspace
- **Reduced attack surface**: Limits syscalls exposed to the host kernel
- **Performance**: Near-native performance for most workloads
- **Use case**: Balance between security and performance

### Kata Containers Runtime
- **VM isolation**: Each container runs in its own lightweight VM
- **Strong isolation**: Complete kernel separation
- **Hardware virtualization**: Uses KVM or similar
- **Use case**: Maximum security for untrusted workloads

## Troubleshooting

### Sandbox Not Ready
```bash
# Check sandbox status
kubectl get sandboxes -n computer

# Check sandbox controller logs
kubectl logs -n agent-sandbox-system deployment/agent-sandbox-controller

# Describe sandbox for conditions
kubectl describe sandbox kubectl-ai-sandbox-<uuid> -n computer
```

### RuntimeClass Not Found
```bash
# List available runtime classes
kubectl get runtimeclass

# Check node supports the runtime
kubectl describe node <node-name> | grep -i runtime
```

### Permission Denied
```bash
# Check RBAC permissions
kubectl auth can-i create sandboxes.agents.x-k8s.io -n computer

# View role bindings
kubectl get rolebindings -n computer
```

## Performance Optimization

### Using Warm Pools

agent-sandbox supports pre-warming pools for faster startup. Create a `SandboxWarmPool`:

```yaml
apiVersion: agents.x-k8s.io/v1alpha1
kind: SandboxWarmPool
metadata:
  name: kubectl-ai-pool
  namespace: computer
spec:
  template:
    spec:
      podTemplate:
        spec:
          containers:
          - name: main
            image: bitnami/kubectl:latest
          runtimeClassName: gvisor
  minSize: 3
  maxSize: 10
```

This maintains 3-10 pre-warmed sandboxes ready for instant allocation.

## References

- [agent-sandbox GitHub Repository](https://github.com/kubernetes-sigs/agent-sandbox)
- [agent-sandbox Documentation](https://agent-sandbox.sigs.k8s.io)
- [Google Blog: Agentic AI on Kubernetes](https://opensource.googleblog.com/2025/11/unleashing-autonomous-ai-agents-why-kubernetes-needs-a-new-standard-for-agent-execution.html)
- [gVisor Documentation](https://gvisor.dev/)
- [Kata Containers Documentation](https://katacontainers.io/)
