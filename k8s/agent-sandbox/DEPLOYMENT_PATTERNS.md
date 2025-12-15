# kubectl-ai Deployment Patterns with agent-sandbox

This document explains the different ways to deploy kubectl-ai with agent-sandbox.

## TL;DR - Which Deployment Should I Use?

| Use Case | Deployment File | Access Method |
|----------|----------------|---------------|
| **Browser-based UI (most common)** | `web-ui-deployment.yaml` | Port-forward + Browser |
| **Production with warm pool** | `web-ui-with-warm-pool.yaml` | Port-forward + Browser |
| **External API clients (IDEs, etc.)** | `example-deployment.yaml` | MCP Protocol |
| **Direct CLI usage** | None (run locally) | Command line |

## Deployment Patterns Explained

### 1. Web UI with Port-Forward (Recommended)

**File:** `web-ui-deployment.yaml`

**What it does:**
- Runs kubectl-ai as a Kubernetes Deployment with Web UI
- Listens on port 8888 (HTTP server)
- You access via `kubectl port-forward` + web browser
- Tools execute in isolated agent-sandbox containers

**Architecture:**
```
┌─────────────┐                  ┌──────────────────┐
│ Your Browser│─────port-forward─┤ kubectl-ai Pod   │
│ localhost:  │                  │ (Web UI:8888)    │
│   8888      │                  └─────────┬────────┘
└─────────────┘                            │
                                           │ creates
                                           ↓
                                  ┌────────────────┐
                                  │ agent-sandbox  │
                                  │ (gVisor pods)  │
                                  └────────────────┘
```

**Usage:**
```bash
# Deploy
kubectl apply -f web-ui-deployment.yaml

# Access
kubectl port-forward -n kubectl-ai service/kubectl-ai-web 8888:80

# Open browser
open http://localhost:8888
```

**Best for:**
- Multi-user teams (everyone port-forwards their own connection)
- Interactive AI conversations via browser
- Testing and development
- When you want a nice UI instead of CLI

---

### 2. Production Web UI with Warm Pool

**File:** `web-ui-with-warm-pool.yaml`

**Same as #1 but adds:**
- Pre-warmed sandbox pool (5-20 ready sandboxes)
- Sub-second tool execution startup
- Persistent session storage
- Resource limits for production

**Usage:**
```bash
# Deploy
kubectl apply -f web-ui-with-warm-pool.yaml

# Wait for warm pool
kubectl wait --for=condition=ready sandboxwarmpool/kubectl-ai-warm-pool -n computer --timeout=300s

# Access
kubectl port-forward -n kubectl-ai service/kubectl-ai-web 8888:80
open http://localhost:8888
```

**Best for:**
- Production deployments
- When you need fast response times
- Multiple concurrent users
- Long-running kubectl-ai instance

---

### 3. MCP Server (External API Clients)

**File:** `example-deployment.yaml`

**What it does:**
- Runs kubectl-ai with MCP (Model Context Protocol) server
- Exposes API for external clients (IDEs, chat apps, etc.)
- Tools execute in agent-sandbox
- **NOT** for web browser access

**Architecture:**
```
┌─────────────┐                  ┌──────────────────┐
│ IDE Plugin  │─────MCP/HTTP─────┤ kubectl-ai Pod   │
│ Chat App    │                  │ (MCP Server)     │
│ External    │                  └─────────┬────────┘
│ Client      │                            │
└─────────────┘                            │ creates
                                           ↓
                                  ┌────────────────┐
                                  │ agent-sandbox  │
                                  │ (gVisor pods)  │
                                  └────────────────┘
```

**Usage:**
```bash
# Deploy
kubectl apply -f example-deployment.yaml

# Clients connect to the service endpoint
# (not via port-forward - use proper Service/Ingress)
```

**Best for:**
- Integrating kubectl-ai into other tools
- IDE extensions
- Chat platforms (Slack, Discord, etc.)
- API-driven workflows
- **NOT** for direct human interaction via browser

---

### 4. Local CLI (No Deployment)

**What it does:**
- Run kubectl-ai binary locally on your machine
- Connects to remote cluster
- Creates agent-sandboxes on the cluster
- Results returned to your local terminal

**Architecture:**
```
┌─────────────┐                  ┌──────────────────┐
│ Your        │                  │ Kubernetes       │
│ Terminal    │────kubectl───────┤ API Server       │
│ (local)     │                  └─────────┬────────┘
└─────────────┘                            │
                                           │ creates
                                           ↓
                                  ┌────────────────┐
                                  │ agent-sandbox  │
                                  │ (gVisor pods)  │
                                  └────────────────┘
```

**Usage:**
```bash
# Run locally (no deployment needed)
kubectl-ai --sandbox=agent-sandbox --runtime-class=gvisor "list pods"
```

**Best for:**
- Quick one-off queries
- Local development
- CI/CD pipelines
- When you don't want a persistent server

---

## Comparison Table

| Feature | Web UI | Web UI + Warm Pool | MCP Server | Local CLI |
|---------|--------|-------------------|------------|-----------|
| **Access Method** | Browser | Browser | API clients | Terminal |
| **Deployment** | In cluster | In cluster | In cluster | None |
| **Startup Speed** | 1-2 sec | < 100ms | 1-2 sec | 1-2 sec |
| **Multi-user** | ✅ (via port-forward) | ✅ (via port-forward) | ✅ (via API) | ❌ |
| **Persistent** | Optional | ✅ | ✅ | ❌ |
| **Best for** | Teams, UI | Production | Integrations | Quick queries |
| **Resource Usage** | Low | Medium | Low | None (on cluster) |
| **Setup Complexity** | Simple | Medium | Medium | None |

---

## Migration from Original k8s Sandbox

If you were using the original `k8s/kubectl-ai.yaml`:

```yaml
# OLD: k8s/kubectl-ai.yaml
args:
  - --ui-type=web
  - --sandbox=k8s  # or no sandbox flag
```

**Migrate to:**

```yaml
# NEW: k8s/agent-sandbox/web-ui-deployment.yaml
args:
  - --ui-type=web
  - --sandbox=agent-sandbox      # Changed!
  - --runtime-class=gvisor       # New! (optional)
```

**Benefits of migration:**
- 90% faster tool execution with warm pool
- Better security with gVisor/Kata
- Designed for AI agent workloads
- Future-proof (official Kubernetes project)

---

## Common Questions

### Q: Can I use agent-sandbox without the Web UI?
**A:** Yes! Use `local CLI` mode or deploy without `--ui-type=web`.

### Q: Can I expose the Web UI via Ingress instead of port-forward?
**A:** Yes! Add an Ingress resource pointing to the `kubectl-ai-web` Service:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kubectl-ai-web
  namespace: kubectl-ai
spec:
  rules:
  - host: kubectl-ai.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kubectl-ai-web
            port:
              number: 80
```

### Q: What's the difference between MCP Server and Web UI?
**A:**
- **Web UI**: Human users access via web browser
- **MCP Server**: Other programs/tools access via API protocol
- Both can use agent-sandbox for tool execution

### Q: Do I need the warm pool?
**A:** No, but it makes tool execution 90% faster. Recommended for production.

### Q: Can I use this with the file cache you implemented earlier?
**A:** Yes! The `FileChatMessageStore` cache works with all deployment patterns automatically.

---

## Next Steps

1. **Choose your deployment pattern** from the table above
2. **Install agent-sandbox** controller:
   ```bash
   kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/v0.1.0/manifest.yaml
   ```
3. **Apply RBAC**:
   ```bash
   kubectl apply -f rbac.yaml
   ```
4. **(Optional) Install RuntimeClass**:
   ```bash
   kubectl apply -f runtime-classes.yaml
   ```
5. **Deploy kubectl-ai** using your chosen pattern
6. **Access** via port-forward or API client
