#!/usr/bin/env bash
set -euo pipefail

usage() {
    echo "Usage: $0 <cloud-tf-dir>"
    echo ""
    echo "Check the Terraform state of a cloud cluster directory and report its lifecycle state."
    echo ""
    echo "States:"
    echo "  EMPTY    No tfstate or zero resources (exit 3)"
    echo "  PENDING  Resources exist but cluster is not yet created (exit 2)"
    echo "  READY    Cluster and nodes exist, no tainted resources (exit 0)"
    echo "  ERROR    Resources are tainted or state is unreadable (exit 1)"
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
TF_DIR_NAME="$(basename "$TF_DIR")"

for cmd in terraform jq; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: $cmd is required but not found in PATH"
        exit 1
    fi
done

STATEFILE="$TF_DIR/terraform.tfstate"
LOCKFILE="$TF_DIR/.terraform.tfstate.lock.info"

if [[ ! -f "$STATEFILE" ]]; then
    echo "EMPTY"
    echo "  No terraform.tfstate found."
    echo "  Run: cd $TF_DIR && terraform init && terraform apply"
    exit 3
fi

if [[ -f "$LOCKFILE" ]]; then
    LOCK_OP=$(jq -r '.Operation // "unknown"' "$LOCKFILE" 2>/dev/null)
    echo "PENDING"
    echo "  Terraform operation in progress ($LOCK_OP)."
    echo "  Wait for it to complete or check the running terraform process."
    exit 2
fi

RESOURCE_COUNT=$(jq '.resources | length' "$STATEFILE")

if [[ "$RESOURCE_COUNT" -eq 0 ]]; then
    echo "EMPTY"
    echo "  terraform.tfstate exists but contains zero resources."
    echo "  Run: cd $TF_DIR && terraform init && terraform apply"
    exit 3
fi

TAINTED=$(jq '[.resources[].instances[] | select(.status == "tainted")] | length' "$STATEFILE")
if [[ "$TAINTED" -gt 0 ]]; then
    echo "ERROR"
    echo "  $TAINTED tainted resource(s) found."
    echo "  Run: cd $TF_DIR && terraform plan"
    exit 1
fi

CLUSTER_TYPES="aws_eks_cluster azurerm_kubernetes_cluster google_container_cluster"
CLUSTER_FOUND=false
for ctype in $CLUSTER_TYPES; do
    count=$(jq --arg t "$ctype" '[.resources[] | select(.type == $t and (.instances | length) > 0)] | length' "$STATEFILE")
    if [[ "$count" -gt 0 ]]; then
        CLUSTER_FOUND=true
        break
    fi
done

if [[ "$CLUSTER_FOUND" == "false" ]]; then
    echo "PENDING"
    echo "  $RESOURCE_COUNT resource(s) in state but no cluster resource found."
    echo "  Terraform apply may still be running or was interrupted."
    echo "  Run: cd $TF_DIR && terraform apply"
    exit 2
fi

NODE_TYPES="aws_eks_node_group google_container_node_pool"
NODE_FOUND=false
for ntype in $NODE_TYPES; do
    count=$(jq --arg t "$ntype" '[.resources[] | select(.type == $t and (.instances | length) > 0)] | length' "$STATEFILE")
    if [[ "$count" -gt 0 ]]; then
        NODE_FOUND=true
        break
    fi
done
# AKS embeds nodes in the cluster resource (default_node_pool), so no separate resource
if [[ "$TF_DIR_NAME" == *"aks"* ]]; then
    NODE_FOUND=true
fi

if [[ "$NODE_FOUND" == "false" ]]; then
    echo "PENDING"
    echo "  Cluster exists but node group/pool not yet created."
    echo "  Terraform apply may still be running or was interrupted."
    echo "  Run: cd $TF_DIR && terraform apply"
    exit 2
fi

CLUSTER_NAME=$(cd "$TF_DIR" && terraform output -raw cluster_name 2>/dev/null || echo "")
echo "READY"
echo "  $RESOURCE_COUNT resources in state, cluster and nodes present."
if [[ -n "$CLUSTER_NAME" ]]; then
    echo "  Cluster name: $CLUSTER_NAME"
fi
exit 0
