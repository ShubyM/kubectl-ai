#!/usr/bin/env bash
# Ensure Gatekeeper is installed in the cluster
# This script is idempotent - safe to run multiple times

set -e

GATEKEEPER_VERSION="${GATEKEEPER_VERSION:-v3.14.0}"
TIMEOUT="${TIMEOUT:-120s}"

# Check if Gatekeeper is already installed and running
if kubectl get deployment gatekeeper-controller-manager -n gatekeeper-system &>/dev/null; then
    if kubectl wait --for=condition=Available deployment/gatekeeper-controller-manager -n gatekeeper-system --timeout=10s &>/dev/null; then
        echo "Gatekeeper is already installed and running"
        exit 0
    fi
fi

echo "Installing Gatekeeper ${GATEKEEPER_VERSION}..."

# Install Gatekeeper
kubectl apply -f "https://raw.githubusercontent.com/open-policy-agent/gatekeeper/${GATEKEEPER_VERSION}/deploy/gatekeeper.yaml"

# Wait for Gatekeeper to be ready
echo "Waiting for Gatekeeper to be ready..."
kubectl wait --for=condition=Available deployment/gatekeeper-controller-manager -n gatekeeper-system --timeout="${TIMEOUT}"
kubectl wait --for=condition=Available deployment/gatekeeper-audit -n gatekeeper-system --timeout="${TIMEOUT}"

# Wait a bit for webhook to be fully ready
sleep 5

echo "Gatekeeper is ready"
