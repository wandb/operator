#!/bin/bash

set -e

CLUSTER_NAME="kind"
DEV_PROFILE=1

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

if [[ -f "tilt-settings.json" ]]; then
    CRD_NAME=$(jq -r '.wandbCRD // empty' tilt-settings.json)
    if [[ -n "$CRD_NAME" && "$CRD_NAME" != *dev* ]]; then
        DEV_PROFILE=0
    fi
fi

echo "Creating kind cluster: $CLUSTER_NAME"

if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
    echo "Kind cluster '$CLUSTER_NAME' already exists"
    kubectl config use-context "kind-$CLUSTER_NAME"
    exit 0
fi

if [[ "$DEV_PROFILE" -eq 1 ]]; then
    cat <<EOF | kind create cluster --name "$CLUSTER_NAME" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
EOF
else
    cat <<EOF | kind create cluster --name "$CLUSTER_NAME" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
- role: worker
EOF
fi

echo "Kind cluster '$CLUSTER_NAME' created successfully with 3 worker nodes"

echo "Installing Kubernetes Metrics Server..."
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch -n kube-system deployment metrics-server --type=json \
  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
