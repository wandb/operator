#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

usage() {
    echo "Usage: $0 <cloud-tf-dir> [options]"
    echo ""
    echo "Deploy the W&B CR and telemetry stack on a cluster that has been set up"
    echo "with cluster_setup.sh."
    echo ""
    echo "Arguments:"
    echo "  <cloud-tf-dir>    Path to the terraform directory (e.g., hack/clusters/eks-tf)"
    echo ""
    echo "Options:"
    echo "  --skip-wandb      Skip deploying the W&B CR"
    echo "  --skip-telemetry  Skip the telemetry stack"
    echo "  --overlay <name>  Add a kustomize overlay (repeatable). Available overlays:"
    echo "                      sizes: size-small, size-micro"
    echo "                      networking: networking-ingress-local, networking-gateway-local"
    echo "                      retention: purge-retention"
    echo "                      server: server-version-0.78.0"
    echo "                      external: external-mysql, external-redis, external-kafka,"
    echo "                                external-objectstore, external-clickhouse"
    echo "                      disable: disable-mysql, disable-redis, disable-kafka,"
    echo "                               disable-minio, disable-clickhouse"
    exit 1
}

if [[ $# -lt 1 ]]; then
    usage
fi

TF_DIR="$1"; shift
SKIP_WANDB=false
SKIP_TELEMETRY=false
OVERLAYS=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --skip-wandb)     SKIP_WANDB=true; shift ;;
        --skip-telemetry) SKIP_TELEMETRY=true; shift ;;
        --overlay)        OVERLAYS+=("$2"); shift 2 ;;
        *)                usage ;;
    esac
done

if [[ ! -d "$TF_DIR" ]]; then
    echo "ERROR: $TF_DIR does not exist"
    exit 1
fi

TF_DIR="$(cd "$TF_DIR" && pwd)"

log() { echo "==> $*"; }

for cmd in kubectl terraform jq; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: $cmd is required but not found in PATH"
        exit 1
    fi
done

# ---------------------------------------------------------------------------
# Resolve context and TF outputs
# ---------------------------------------------------------------------------
log "Reading terraform outputs"
cd "$TF_DIR"

CLUSTER_NAME=$(terraform output -raw cluster_name 2>/dev/null || echo "")
if [[ -z "$CLUSTER_NAME" ]]; then
    echo "ERROR: Could not read cluster_name from terraform output."
    exit 1
fi

CTX="$CLUSTER_NAME"
kctl()  { kubectl --context "$CTX" "$@"; }
hctl()  { helm --kube-context "$CTX" "$@"; }

if ! kubectl config get-contexts "$CTX" &>/dev/null; then
    echo "ERROR: kubectl context '$CTX' not found. Run cluster_kubeconfig.sh first."
    exit 1
fi

log "Using context: $CTX"

OBJECTSTORE_URL=$(terraform output -raw objectstore_url 2>/dev/null || echo "")
OBJECTSTORE_ENDPOINT=$(terraform output -raw objectstore_endpoint 2>/dev/null || echo "")
OBJECTSTORE_PORT=$(terraform output -raw objectstore_port 2>/dev/null || echo "")
OBJECTSTORE_BUCKET=$(terraform output -raw objectstore_bucket 2>/dev/null || echo "")
OBJECTSTORE_REGION=$(terraform output -raw objectstore_region 2>/dev/null || echo "")
OBJECTSTORE_ACCESS_KEY=$(terraform output -raw objectstore_access_key 2>/dev/null || echo "")
OBJECTSTORE_SECRET_KEY=$(terraform output -raw objectstore_secret_key 2>/dev/null || echo "")

cd "$REPO_ROOT"

# ---------------------------------------------------------------------------
# Verify prerequisites from cluster_setup.sh
# ---------------------------------------------------------------------------
log "Verifying operator is ready"
ca=$(kctl get mutatingwebhookconfiguration operator-mutating-webhook-configuration \
    -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null || echo "")
if [[ -z "$ca" ]]; then
    echo "ERROR: Operator webhook not ready. Run cluster_setup.sh first."
    exit 1
fi

# ---------------------------------------------------------------------------
# Create external object store secret (conditional)
# ---------------------------------------------------------------------------
if [[ -n "$OBJECTSTORE_URL" ]]; then
    log "Creating external-objectstore-connection secret"
    kctl create secret generic external-objectstore-connection \
        --namespace=default \
        --from-literal=url="$OBJECTSTORE_URL" \
        --from-literal=Host="$OBJECTSTORE_ENDPOINT" \
        --from-literal=Port="$OBJECTSTORE_PORT" \
        --from-literal=Bucket="$OBJECTSTORE_BUCKET" \
        --from-literal=Region="$OBJECTSTORE_REGION" \
        --from-literal=AccessKey="$OBJECTSTORE_ACCESS_KEY" \
        --from-literal=SecretKey="$OBJECTSTORE_SECRET_KEY" \
        --dry-run=client -o yaml | kctl apply -f -

    OVERLAY_ALREADY_ADDED=false
    for o in "${OVERLAYS[@]+"${OVERLAYS[@]}"}"; do
        if [[ "$o" == "external-objectstore" ]]; then
            OVERLAY_ALREADY_ADDED=true
            break
        fi
    done
    if [[ "$OVERLAY_ALREADY_ADDED" == "false" ]]; then
        log "Auto-adding external-objectstore overlay"
        OVERLAYS+=("external-objectstore")
    fi
else
    log "No object store configured in terraform, skipping secret"
fi

# ---------------------------------------------------------------------------
# W&B CR
# ---------------------------------------------------------------------------
if [[ "$SKIP_WANDB" == "false" ]]; then
    log "Building and applying W&B CR"
    GENERATED_DIR="$REPO_ROOT/hack/testing-manifests/wandb/.generated"
    OVERLAY_DIR="$REPO_ROOT/hack/testing-manifests/wandb/kustomize/overlays"
    mkdir -p "$GENERATED_DIR"

    KUSTOMIZATION="apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../kustomize/base
"

    if [[ ${#OVERLAYS[@]} -gt 0 ]]; then
        KUSTOMIZATION+="components:
"
        for overlay in "${OVERLAYS[@]}"; do
            if [[ ! -d "$OVERLAY_DIR/$overlay" ]]; then
                echo "ERROR: Unknown overlay '$overlay'. Available overlays:"
                ls "$OVERLAY_DIR/"
                exit 1
            fi
            KUSTOMIZATION+="  - ../kustomize/overlays/$overlay
"
            log "  Adding overlay: $overlay"
        done
    fi

    echo "$KUSTOMIZATION" > "$GENERATED_DIR/kustomization.yaml"
    kubectl kustomize "$GENERATED_DIR" > "$GENERATED_DIR/wandb-cr.yaml"

    log "Generated W&B CR:"
    grep -E '^\s+(size|mode|hostname|version):' "$GENERATED_DIR/wandb-cr.yaml" || true

    kctl apply -f "$GENERATED_DIR/wandb-cr.yaml"
    log "W&B CR applied"
fi

# ---------------------------------------------------------------------------
# Telemetry stack
# ---------------------------------------------------------------------------
if [[ "$SKIP_TELEMETRY" == "false" ]]; then
    log "Waiting for telemetry CRDs to be established"
    kctl wait --for=condition=established --timeout=120s \
        crd/vmsingles.operator.victoriametrics.com \
        crd/vmagents.operator.victoriametrics.com \
        crd/vlsingles.operator.victoriametrics.com \
        crd/vtsingles.operator.victoriametrics.com \
        crd/vmservicescrapes.operator.victoriametrics.com \
        crd/vmpodscrapes.operator.victoriametrics.com \
        crd/vmnodescrapes.operator.victoriametrics.com \
        crd/grafanas.grafana.integreatly.org \
        crd/grafanadatasources.grafana.integreatly.org 2>/dev/null || \
        echo "WARNING: Some telemetry CRDs not ready"

    log "Waiting for telemetry operator deployments"
    kctl wait --for=condition=available --timeout=180s -n wandb-operator \
        deploy/third-party-operators-victoria-metrics-operator \
        deploy/third-party-operators-grafana-operator 2>/dev/null || true

    log "Installing telemetry stack"
    hctl upgrade --install \
        --set=enabled=true \
        --set=namespace=default \
        --create-namespace \
        --namespace wandb-operator \
        telemetry-stack "$REPO_ROOT/deploy/telemetry"
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
log "Deploy complete!"
echo ""
echo "  Cluster:    $CLUSTER_NAME"
echo "  Context:    $CTX"
if [[ -n "$OBJECTSTORE_URL" ]]; then
    echo "  ObjectStore: configured (external)"
fi
echo ""
echo "  Check status: $SCRIPT_DIR/cluster_status.sh $TF_DIR"
echo "  kubectl --context $CTX get pods -A"
echo ""
