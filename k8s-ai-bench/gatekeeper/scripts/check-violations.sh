#!/usr/bin/env bash
# Check if a resource has Gatekeeper violations
# Usage: check-violations.sh <namespace> <kind> <name> [constraint-kind]
#
# Returns 0 if NO violations (compliant), 1 if violations exist

set -e

NAMESPACE="${1:?Namespace required}"
KIND="${2:?Resource kind required}"
NAME="${3:?Resource name required}"
CONSTRAINT_KIND="${4:-}"  # Optional: filter by specific constraint

# Trigger an audit cycle and wait for it
echo "Triggering Gatekeeper audit..."
kubectl annotate constraint --all audit.gatekeeper.sh/trigger=manual --overwrite 2>/dev/null || true
sleep 3

# Check for violations on the specific resource
echo "Checking for violations on ${KIND}/${NAME} in namespace ${NAMESPACE}..."

# Get all constraints and check their violations
VIOLATIONS_FOUND=0

if [[ -n "$CONSTRAINT_KIND" ]]; then
    CONSTRAINTS=$(kubectl get "$CONSTRAINT_KIND" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
    for constraint in $CONSTRAINTS; do
        VIOLATIONS=$(kubectl get "$CONSTRAINT_KIND" "$constraint" -o jsonpath='{.status.violations}' 2>/dev/null || echo "[]")
        if echo "$VIOLATIONS" | grep -q "\"name\":\"${NAME}\"" && echo "$VIOLATIONS" | grep -q "\"namespace\":\"${NAMESPACE}\""; then
            echo "VIOLATION FOUND: ${CONSTRAINT_KIND}/${constraint} reports violation for ${KIND}/${NAME}"
            VIOLATIONS_FOUND=1
        fi
    done
else
    # Check all constraint types
    CONSTRAINT_KINDS=$(kubectl get constrainttemplates -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
    for ck in $CONSTRAINT_KINDS; do
        # Convert template name to constraint kind (e.g., k8srequiredlabels -> K8sRequiredLabels)
        CONSTRAINTS=$(kubectl get "$ck" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
        for constraint in $CONSTRAINTS; do
            VIOLATIONS=$(kubectl get "$ck" "$constraint" -o jsonpath='{.status.violations}' 2>/dev/null || echo "[]")
            if echo "$VIOLATIONS" | grep -q "\"name\":\"${NAME}\""; then
                if [[ "$NAMESPACE" == "*" ]] || echo "$VIOLATIONS" | grep -q "\"namespace\":\"${NAMESPACE}\""; then
                    echo "VIOLATION FOUND: ${ck}/${constraint} reports violation for ${KIND}/${NAME}"
                    VIOLATIONS_FOUND=1
                fi
            fi
        done
    done
fi

if [[ "$VIOLATIONS_FOUND" -eq 0 ]]; then
    echo "No violations found - resource is compliant"
    exit 0
else
    echo "Violations found - resource is NOT compliant"
    exit 1
fi
