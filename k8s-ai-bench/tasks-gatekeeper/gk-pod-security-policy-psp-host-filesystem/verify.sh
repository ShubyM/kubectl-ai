#!/usr/bin/env bash
set -euo pipefail

# This script verifies that the model correctly identified policy violations.
# The model's output should identify the following resources as violations:

# - Pod/nginx-host-filesystem-2

# - Pod/nginx-host-filesystem-3


# Read the model's output from stdin or first argument
MODEL_OUTPUT="${1:-}"

if [ -z "$MODEL_OUTPUT" ]; then
    echo "Error: No model output provided"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXPECTED_VIOLATIONS="$SCRIPT_DIR/artifacts/expected_violations.txt"

# Check that all expected violations are mentioned in the output
MISSING=0
while IFS= read -r violation; do
    # Extract resource name from Kind/Name format
    RESOURCE_NAME=$(echo "$violation" | cut -d'/' -f2)
    if ! echo "$MODEL_OUTPUT" | grep -qi "$RESOURCE_NAME"; then
        echo "Missing violation: $violation"
        MISSING=$((MISSING + 1))
    fi
done < "$EXPECTED_VIOLATIONS"

if [ $MISSING -gt 0 ]; then
    echo "Failed: $MISSING expected violations not identified"
    exit 1
fi

# Check for false positives (allowed resources marked as violations)

if echo "$MODEL_OUTPUT" | grep -qi "nginx-host-filesystem.*violat"; then
    echo "False positive: Pod/nginx-host-filesystem was incorrectly marked as a violation"
    # This is a warning, not a failure
fi


echo "Verification passed: All expected violations were correctly identified"
exit 0
