#!/bin/bash

set -e

CLUSTER_NAME="kind"

if ! command -v jq >/dev/null 2>&1; then
    echo "Error: jq is required but not installed. Please install jq."
    exit 1
fi

if [[ -f "tilt-settings.json" ]]; then
    KIND_CLUSTER_NAME=$(jq -r '.kindClusterName // empty' tilt-settings.json)
    if [[ -n "$KIND_CLUSTER_NAME" ]]; then
        CLUSTER_NAME="$KIND_CLUSTER_NAME"
    fi
fi

echo "Deleting kind cluster: $CLUSTER_NAME"

if ! kind get clusters | grep -q "^$CLUSTER_NAME$"; then
    echo "Kind cluster '$CLUSTER_NAME' does not exist"
    exit 0
fi

kind delete cluster --name "$CLUSTER_NAME"

echo "Kind cluster '$CLUSTER_NAME' deleted successfully"