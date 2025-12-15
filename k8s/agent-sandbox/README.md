# agent-sandbox Integration for kubectl-ai

This directory contains Kubernetes manifests and configuration for running kubectl-ai with [agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox) support.

## What is agent-sandbox?

agent-sandbox is a Kubernetes SIG Apps project that provides a standardized API for managing isolated, stateful, singleton workloads. It's specifically designed for AI agent runtimes and offers:

- **Fast startup**: Sub-second latency with pre-warming pools (90% improvement over cold starts)
- **Enhanced security**: Support for gVisor and Kata Containers runtimes
- **Native Kubernetes integration**: CRD-based declarative API
- **Production-ready**: Scales to thousands of concurrent sandboxes

## Quick Start

### 1. Install agent-sandbox

```bash
kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/v0.1.0/manifest.yaml
```

### 2. Apply RBAC Configuration

```bash
kubectl apply -f rbac.yaml
```

### 3. (Optional) Install RuntimeClass

For enhanced security with gVisor:

```bash
# Install gVisor on your nodes first
# See: https://gvisor.dev/docs/user_guide/install/

kubectl apply -f runtime-classes.yaml
```

### 4. Deploy kubectl-ai

#### Option A: Web UI (Recommended for multi-user access)

```bash
# Deploy with Web UI
kubectl apply -f web-ui-deployment.yaml

# Port-forward to access
kubectl port-forward -n kubectl-ai service/kubectl-ai-web 8888:80

# Open browser
open http://localhost:8888
```

#### Option B: CLI (Direct command-line usage)

```bash
kubectl-ai --sandbox=agent-sandbox "list all pods"

# With gVisor isolation
kubectl-ai --sandbox=agent-sandbox --runtime-class=gvisor "describe nodes"
```

#### Option C: Production Web UI with Warm Pool

```bash
# Deploy everything including warm pool
kubectl apply -f web-ui-with-warm-pool.yaml

# Wait for warm pool to be ready
kubectl wait --for=condition=ready sandboxwarmpool/kubectl-ai-warm-pool -n computer --timeout=300s

# Port-forward
kubectl port-forward -n kubectl-ai service/kubectl-ai-web 8888:80

# Access at http://localhost:8888
```

## Files in this Directory

| File | Description |
|------|-------------|
| `rbac.yaml` | RBAC permissions for kubectl-ai to manage Sandbox resources |
| `runtime-classes.yaml` | RuntimeClass definitions for gVisor and Kata Containers |
| `example-deployment.yaml` | Example kubectl-ai with MCP server (for external API clients) |
| `web-ui-deployment.yaml` | **Web UI deployment with port-forward access** ⭐ |
| `web-ui-with-warm-pool.yaml` | **Production Web UI with warm pool** ⭐ |
| `warm-pool.yaml` | SandboxWarmPool for pre-warmed sandboxes |

## Architecture

```
┌──────────────┐
│  kubectl-ai  │
└──────┬───────┘
       │
       │ creates Sandbox CRD
       ↓
┌──────────────────────────┐
│ Sandbox                  │
│ (agents.x-k8s.io/v1alpha1)│
└──────┬───────────────────┘
       │
       │ managed by
       ↓
┌──────────────────┐
│ agent-sandbox    │
│ controller       │
└──────┬───────────┘
       │
       │ creates/manages
       ↓
┌──────────────────┐
│ Pod (with gVisor │
│ or Kata runtime) │
└──────────────────┘
```

## Comparison with Standard k8s Sandbox

| Feature | k8s (Pod) | agent-sandbox |
|---------|-----------|---------------|
| **Startup Time** | 2-5 seconds | < 1 second |
| **API** | Pod (core/v1) | Sandbox (agents.x-k8s.io/v1alpha1) |
| **Warm Pools** | Manual | Built-in |
| **Runtime Isolation** | Optional | Designed for gVisor/Kata |
| **Lifecycle** | StatefulSet/Deployment | Declarative Sandbox CRD |

## Performance Optimization

### Using Warm Pools

Pre-warm sandboxes for instant availability:

```bash
kubectl apply -f warm-pool.yaml
```

This maintains 3-10 ready sandboxes for immediate use.

### Monitoring

```bash
# Check warm pool status
kubectl get sandboxwarmpools -n computer

# View sandboxes
kubectl get sandboxes -n computer

# Check sandbox readiness
kubectl describe sandbox <name> -n computer
```

## Security Isolation Options

### gVisor (Recommended for most use cases)

- **Pros**: Near-native performance, good security
- **Cons**: Some syscalls may not be supported
- **Setup**: Requires gVisor runtime on nodes

```bash
kubectl-ai --sandbox=agent-sandbox --runtime-class=gvisor "your query"
```

### Kata Containers (Maximum isolation)

- **Pros**: VM-level isolation, complete kernel separation
- **Cons**: Higher resource overhead, slower startup
- **Setup**: Requires KVM and Kata runtime on nodes

```bash
kubectl-ai --sandbox=agent-sandbox --runtime-class=kata "your query"
```

### No RuntimeClass (Default)

- **Pros**: No additional setup required
- **Cons**: Standard container isolation only
- **Use**: Development and testing

```bash
kubectl-ai --sandbox=agent-sandbox "your query"
```

## Troubleshooting

### Sandbox Not Ready

```bash
# Check sandbox status
kubectl get sandboxes -n computer -w

# View controller logs
kubectl logs -n agent-sandbox-system deployment/agent-sandbox-controller

# Check pod events
kubectl describe sandbox <name> -n computer
```

### Permission Errors

```bash
# Verify RBAC
kubectl auth can-i create sandboxes.agents.x-k8s.io --as=system:serviceaccount:kubectl-ai:kubectl-ai -n computer

# Check role bindings
kubectl get clusterrolebindings | grep kubectl-ai
kubectl get rolebindings -n computer | grep kubectl-ai
```

### RuntimeClass Not Available

```bash
# List runtime classes
kubectl get runtimeclass

# Check node supports runtime
kubectl describe node <node-name> | grep -i runtime

# Verify gVisor installation
kubectl run test --rm -it --image=ubuntu --overrides='{"spec": {"runtimeClassName": "gvisor"}}' -- uname -a
```

## Configuration Examples

### In config.yaml

```yaml
# ~/.config/kubectl-ai/config.yaml
sandbox: agent-sandbox
runtimeClass: gvisor
sandboxImage: bitnami/kubectl:latest
```

### As Environment Variables

```bash
export KUBECTL_AI_SANDBOX=agent-sandbox
export KUBECTL_AI_RUNTIME_CLASS=gvisor
kubectl-ai "your query"
```

### In Kubernetes Deployment

See [example-deployment.yaml](example-deployment.yaml) for a complete example.

## Resources

- [agent-sandbox GitHub](https://github.com/kubernetes-sigs/agent-sandbox)
- [agent-sandbox Documentation](https://agent-sandbox.sigs.k8s.io)
- [kubectl-ai agent-sandbox Guide](../../docs/agent-sandbox.md)
- [gVisor Documentation](https://gvisor.dev/)
- [Kata Containers Documentation](https://katacontainers.io/)

## License

Same as kubectl-ai (Apache 2.0)
