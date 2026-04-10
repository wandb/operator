#!/usr/bin/env bash
set -euo pipefail

usage() {
    echo "Usage: $0 <cloud-tf-dir>"
    echo ""
    echo "Fetch kubeconfig for a Terraform-managed cluster and rename the context"
    echo "to match the cluster name."
    exit 1
}

if [[ $# -lt 1 ]]; then
    usage
fi

TF_DIR="$1"

if [[ ! -d "$TF_DIR" ]]; then
    echo "ERROR: $TF_DIR does not exist"
    exit 1
fi

TF_DIR="$(cd "$TF_DIR" && pwd)"

for cmd in terraform kubectl; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: $cmd is required but not found in PATH"
        exit 1
    fi
done

log() { echo "==> $*"; }

log "Reading terraform outputs from $TF_DIR"
CLUSTER_NAME=$(cd "$TF_DIR" && terraform output -raw cluster_name 2>/dev/null || echo "")
if [[ -z "$CLUSTER_NAME" ]]; then
    echo "ERROR: Could not read cluster_name from terraform output."
    echo "       Is the cluster created? Run: $(basename "$0" .sh | sed 's/k8scontext/cluster/') $TF_DIR"
    exit 1
fi

KUBECONFIG_CMD=$(cd "$TF_DIR" && terraform output -raw kubeconfig_command 2>/dev/null || echo "")
KUBE_CONTEXT=$(cd "$TF_DIR" && terraform output -raw kube_context_name 2>/dev/null || echo "")

if [[ -z "$KUBECONFIG_CMD" ]]; then
    echo "ERROR: No kubeconfig_command output from terraform."
    exit 1
fi

log "Fetching kubeconfig: $KUBECONFIG_CMD"
eval "$KUBECONFIG_CMD"

if [[ -n "$KUBE_CONTEXT" && "$KUBE_CONTEXT" != "$CLUSTER_NAME" ]]; then
    if kubectl config get-contexts "$KUBE_CONTEXT" &>/dev/null; then
        log "Renaming context: $KUBE_CONTEXT → $CLUSTER_NAME"
        if kubectl config get-contexts "$CLUSTER_NAME" &>/dev/null; then
            kubectl config delete-context "$CLUSTER_NAME" &>/dev/null || true
        fi
        kubectl config rename-context "$KUBE_CONTEXT" "$CLUSTER_NAME"
    fi
fi

log "Setting current context to $CLUSTER_NAME"
kubectl config use-context "$CLUSTER_NAME"

log "Verifying cluster access"
kubectl get nodes
