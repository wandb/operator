#!/bin/bash

set -e

WORKER_NODES="${1:-0}"

if ! [[ "$WORKER_NODES" =~ ^[0-9]+$ ]]; then
    echo "Error: WORKER_NODES must be a non-negative integer, got '$WORKER_NODES'"
    exit 1
fi

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

echo "Creating kind cluster: $CLUSTER_NAME"

if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
    echo "Kind cluster '$CLUSTER_NAME' already exists"
    kubectl config use-context "kind-$CLUSTER_NAME"
    exit 0
fi

KIND_CONFIG="kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4"

if [[ "$WORKER_NODES" -gt 0 ]]; then
    KIND_CONFIG+=$'\nnodes:\n- role: control-plane'
    for ((i = 0; i < WORKER_NODES; i++)); do
        KIND_CONFIG+=$'\n- role: worker'
    done
fi

echo "$KIND_CONFIG" | kind create cluster --name "$CLUSTER_NAME" --config=-

echo "Kind cluster '$CLUSTER_NAME' created successfully with $WORKER_NODES worker nodes"

echo "Installing Kubernetes Metrics Server..."
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch -n kube-system deployment metrics-server --type=json \
  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
