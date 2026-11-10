#!/bin/bash

set -e

usage() {
    cat <<'USAGE'
Usage: setup_kind.sh [WORKER_NODES] [--kubernetes-version VERSION]

Create a kind cluster, or reuse an existing cluster with a matching version.
WORKER_NODES defaults to 0. Without VERSION, kind chooses its default image.
VERSION accepts 1.35, 1.35.2, or v1.35.2; a minor version selects patch 0
when creating a cluster. The corresponding kindest/node image must exist.

Examples:
  setup_kind.sh --kubernetes-version 1.35
  setup_kind.sh 2 --kubernetes-version 1.36
  setup_kind.sh --kubernetes-version v1.37.0 1
USAGE
}

WORKER_NODES="0"
WORKER_NODES_SET=false
KUBERNETES_VERSION=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --kubernetes-version)
            if [[ $# -lt 2 || -z "$2" ]]; then
                echo "Error: --kubernetes-version requires a version" >&2
                exit 1
            fi
            KUBERNETES_VERSION="$2"
            shift 2
            ;;
        --kubernetes-version=*)
            KUBERNETES_VERSION="${1#*=}"
            if [[ -z "$KUBERNETES_VERSION" ]]; then
                echo "Error: --kubernetes-version requires a version" >&2
                exit 1
            fi
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            echo "Error: unknown option '$1'" >&2
            usage >&2
            exit 1
            ;;
        *)
            if [[ "$WORKER_NODES_SET" == true ]]; then
                echo "Error: unexpected argument '$1'" >&2
                usage >&2
                exit 1
            fi
            WORKER_NODES="$1"
            WORKER_NODES_SET=true
            shift
            ;;
    esac
done

if [[ -n "$KUBERNETES_VERSION" ]]; then
    if ! [[ "$KUBERNETES_VERSION" =~ ^v?[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
        echo "Error: Kubernetes version must be MAJOR.MINOR or MAJOR.MINOR.PATCH, got '$KUBERNETES_VERSION'" >&2
        exit 1
    fi
    KUBERNETES_VERSION="${KUBERNETES_VERSION#v}"
fi

if ! [[ "$WORKER_NODES" =~ ^[0-9]+$ ]]; then
    echo "Error: WORKER_NODES must be a non-negative integer, got '$WORKER_NODES'"
    exit 1
fi

for tool in kind kubectl; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "Error: required tool '$tool' is not installed" >&2
        exit 1
    fi
done

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
    if [[ -n "$KUBERNETES_VERSION" ]]; then
        NODE_VERSIONS=$(kubectl --context "kind-$CLUSTER_NAME" get nodes \
            -o jsonpath='{range .items[*]}{.status.nodeInfo.kubeletVersion}{"\n"}{end}')
        if [[ -z "$NODE_VERSIONS" ]]; then
            echo "Error: could not determine existing cluster's Kubernetes version" >&2
            exit 1
        fi
        while IFS= read -r node_version; do
            node_version="${node_version#v}"
            node_version="${node_version%%+*}"
            if [[ "$node_version" != "$KUBERNETES_VERSION" &&
                  ( "$KUBERNETES_VERSION" == *.*.* || "$node_version" != "$KUBERNETES_VERSION".* ) ]]; then
                echo "Error: cluster '$CLUSTER_NAME' runs Kubernetes $node_version; requested $KUBERNETES_VERSION" >&2
                echo "Delete the existing cluster or select another kindClusterName in tilt-settings.star." >&2
                exit 1
            fi
        done <<< "$NODE_VERSIONS"
    fi
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

KIND_CREATE_ARGS=(create cluster --name "$CLUSTER_NAME" --config=-)
if [[ -n "$KUBERNETES_VERSION" ]]; then
    NODE_VERSION="$KUBERNETES_VERSION"
    if [[ "$NODE_VERSION" != *.*.* ]]; then
        NODE_VERSION+=".0"
    fi
    KIND_CREATE_ARGS+=(--image "kindest/node:v$NODE_VERSION")
    echo "Using Kubernetes $NODE_VERSION (kindest/node:v$NODE_VERSION)"
fi

echo "$KIND_CONFIG" | kind "${KIND_CREATE_ARGS[@]}"

echo "Kind cluster '$CLUSTER_NAME' created successfully with $WORKER_NODES worker nodes"

echo "Installing Kubernetes Metrics Server..."
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch -n kube-system deployment metrics-server --type=json \
  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
