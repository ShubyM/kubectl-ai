## k8s-bench

`k8s-bench` is a benchmark for assessing the performance of LLM models for kubernetes related tasks.


### Usage

```sh
# build the k8s-bench binary
go build
```

#### Run Subcommand

The `run` subcommand executes the benchmark evaluations.

```sh
# Basic usage with mandatory output directory
./k8s-bench run --agent-bin <path/to/agent> --output-dir .build/k8sbench

# Run evaluation for tasks matching a pattern
./k8s-bench run --agent-bin <path/to/agent> --task-pattern scale --output-dir .build/k8sbench

# Pass agent specific arguments (repeatable) using Go templates
./k8s-bench run \
  --agent-bin <path/to/gemini-cli> \
  --agent-args '--kubeconfig={{.Kubeconfig}}' \
  --agent-args '--model=gemini-1.5-pro-latest' \
  --agent-args '--approval-policy=yolo' \
  --output-dir .build/k8sbench

# Run evaluation sequentially (one task at a time)
./k8s-bench run --agent-bin <path/to/agent> --tasks-dir ./tasks --output-dir .build/k8sbench --concurrency 1

# Automatically create a temporary kind cluster for isolation
./k8s-bench run --agent-bin <path/to/agent> --create-kind-cluster --output-dir .build/k8sbench
```

`--agent-args` values accept Go templates with access to `{{.AgentID}}`, `{{.Kubeconfig}}`, and `{{.TracePath}}` so you can forward Kubernetes context information or trace locations to the evaluated agent. The benchmark picks up `KUBECONFIG` from the environment by default (falling back to `~/.kube/config`).

#### Available flags for `run` subcommand:

| Flag | Description | Default | Required |
|------|-------------|---------|----------|
| `--agent-bin` | Path to the agent executable to benchmark | - | Yes |
| `--output-dir` | Directory to write results to | - | Yes |
| `--tasks-dir` | Directory containing evaluation tasks | ./tasks | No |
| `--task-pattern` | Pattern to filter tasks (e.g. `pod` or `redis`) | - | No |
| `--agent-args` | Additional arguments for the agent (repeatable, supports templates) | - | No |
| `--concurrency` | Number of tasks to run concurrently (0 = auto based on number of tasks, 1 = sequential) | 0 | No |
| `--create-kind-cluster` | Create a temporary kind cluster for the evaluation run | false | No |

#### Analyze Subcommand

The `analyze` subcommand processes results from previous runs:

```sh
# Analyze previous evaluation results and output in markdown format (default)
./k8s-bench analyze --input-dir .build/k8sbench

# Analyze previous evaluation results and output in JSON format
./k8s-bench analyze --input-dir .build/k8sbench --output-format json

# Save analysis results to a file
./k8s-bench analyze --input-dir .build/k8sbench --results-filepath ./results.md

# Analyze with all available options
./k8s-bench analyze \
  --input-dir .build/k8sbench \
  --output-format markdown \
  --ignore-tool-use-shim true \
  --results-filepath ./detailed-analysis.md
```

#### Available flags for `analyze` subcommand:

| Flag | Description | Default | Required |
|------|-------------|---------|----------|
| `--input-dir` | Directory containing evaluation results | - | Yes |
| `--output-format` | Output format (markdown or json) | markdown | No |
| `--ignore-tool-use-shim` | Ignore tool use shim in result grouping | true | No |
| `--results-filepath` | Optional file path to write results to | - | No |

Running the benchmark with the `run` subcommand will produce results as below:

```sh
Evaluation Results:
==================

Task: scale-deployment
  Agent: gemini-cli (gemini-1.5-pro)
    Args: --kubeconfig=/tmp/kubeconfig --model=gemini-1.5-pro
  Result: success

Task: scale-down-deployment
  Agent: gemini-cli (gemini-1.5-pro)
    Args: --kubeconfig=/tmp/kubeconfig --model=gemini-1.5-pro
  Result: success
```

The `analyze` subcommand will gather the results from previous runs and display them in a tabular format with emoji indicators for success (✅) and failure (❌).

### Contributions

We're open to contributions in k8s-bench, check out the [contributions guide.](contributing.md)
