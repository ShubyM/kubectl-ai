#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

set -x

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd ${REPO_ROOT}

if [[ -z "${OUTPUT_DIR:-}" ]]; then
    OUTPUT_DIR="${REPO_ROOT}/.build/k8s-ai-bench"
    mkdir -p "${OUTPUT_DIR}"
fi
echo "Writing results to ${OUTPUT_DIR}"

BINDIR="${REPO_ROOT}/.build/bin"
mkdir -p "${BINDIR}"

# Build kubectl-ai from current checkout
cd "${REPO_ROOT}"
go build -o "${BINDIR}/kubectl-ai" ./cmd

# Build k8s-ai-bench from current checkout
cd "${REPO_ROOT}/k8s-ai-bench"
go build -o "${BINDIR}/k8s-ai-bench" .

"${BINDIR}/k8s-ai-bench" run --agent-bin "${BINDIR}/kubectl-ai" --kubeconfig "${KUBECONFIG:-~/.kube/config}" --output-dir "${OUTPUT_DIR}" ${TEST_ARGS:-}