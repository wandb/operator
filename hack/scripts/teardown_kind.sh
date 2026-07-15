#!/bin/bash

set -e

CLUSTER_NAME="kind"

read_star_setting() {
    local key="$1"
    local file="tilt-settings.star"
    if [[ -f "$file" ]]; then
        grep "\"$key\"" "$file" | sed 's/.*: *"\(.*\)".*/\1/' | head -1
    fi
}

KIND_CLUSTER_NAME=$(read_star_setting "kindClusterName")
if [[ -n "$KIND_CLUSTER_NAME" ]]; then
    CLUSTER_NAME="$KIND_CLUSTER_NAME"
fi

echo "Deleting kind cluster: $CLUSTER_NAME"

if ! kind get clusters | grep -q "^$CLUSTER_NAME$"; then
    echo "Kind cluster '$CLUSTER_NAME' does not exist"
    exit 0
fi

kind delete cluster --name "$CLUSTER_NAME"

echo "Kind cluster '$CLUSTER_NAME' deleted successfully"

if [[ -d "dist" ]]; then
    echo "Cleaning dist/ directory..."
    rm -rf dist
    echo "dist/ directory removed"
fi
