#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

usage() {
    echo "Usage: $0 <cloud-tf-dir> [options]"
    echo ""
    echo "Install the W&B operator infrastructure on a cloud cluster created by Terraform."
    echo "This installs cert-manager, CRDs, third-party operators, and the W&B operator controller."
    echo ""
    echo "Arguments:"
    echo "  <cloud-tf-dir>    Path to the terraform directory (e.g., hack/clusters/eks-tf)"
    echo ""
    echo "Options:"
    echo "  --skip-build      Skip building and pushing the operator image"
    echo "  --registry <url>  Override the container registry URL"
    echo "  --image <ref>     Use a pre-built operator image instead of building"
    exit 1
}

if [[ $# -lt 1 ]]; then
    usage
fi

TF_DIR="$1"; shift
SKIP_BUILD=false
REGISTRY=""
OPERATOR_IMAGE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --skip-build)     SKIP_BUILD=true; shift ;;
        --registry)       REGISTRY="$2"; shift 2 ;;
        --image)          OPERATOR_IMAGE="$2"; shift 2 ;;
        *)                usage ;;
    esac
done

if [[ ! -d "$TF_DIR" ]]; then
    echo "ERROR: $TF_DIR does not exist"
    exit 1
fi

TF_DIR="$(cd "$TF_DIR" && pwd)"

log() { echo "==> $*"; }

retry() {
    local max_attempts="${RETRY_MAX:-3}"
    local delay="${RETRY_DELAY:-3}"
    for attempt in $(seq 1 "$max_attempts"); do
        if "$@"; then return 0; fi
        if [[ "$attempt" -eq "$max_attempts" ]]; then return 1; fi
        log "  Transient failure (attempt $attempt/$max_attempts), retrying in ${delay}s..."
        sleep "$delay"
    done
}

for cmd in kubectl helm terraform jq; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: $cmd is required but not found in PATH"
        exit 1
    fi
done

# ---------------------------------------------------------------------------
# Step 1: Validate cluster is READY and configure kubectl context
# ---------------------------------------------------------------------------
log "Step 1: Validating cluster state"
CLUSTER_STATE=$("$SCRIPT_DIR/cluster_state.sh" "$TF_DIR" | head -1)
if [[ "$CLUSTER_STATE" != "READY" ]]; then
    echo "ERROR: Cluster is not READY (state: $CLUSTER_STATE)"
    "$SCRIPT_DIR/cluster_state.sh" "$TF_DIR" || true
    exit 1
fi

PRECHECKED_CTX=$(cd "$TF_DIR" && terraform output -raw cluster_name 2>/dev/null || echo "")
if [[ -n "$PRECHECKED_CTX" ]] && kubectl config get-contexts "$PRECHECKED_CTX" &>/dev/null; then
    log "Step 1: kubectl context '$PRECHECKED_CTX' already configured, skipping kubeconfig fetch"
else
    log "Step 1: Configuring kubectl context"
    "$SCRIPT_DIR/cluster_kubeconfig.sh" "$TF_DIR"
fi

# ---------------------------------------------------------------------------
# Read terraform outputs
# ---------------------------------------------------------------------------
log "Reading terraform outputs"
cd "$TF_DIR"

CLUSTER_NAME=$(terraform output -raw cluster_name 2>/dev/null || echo "")

CTX="$CLUSTER_NAME"
kctl()  { kubectl --context "$CTX" "$@"; }
hctl()  { helm --kube-context "$CTX" "$@"; }

if [[ -z "$REGISTRY" ]]; then
    REGISTRY=$(terraform output -raw registry_url 2>/dev/null || echo "")
    if [[ -n "$REGISTRY" ]]; then
        # ACR/GKE registry_url is a registry base — append image name
        REGISTRY="$REGISTRY/operator"
    else
        # ECR outputs host + repo separately — already a complete image base
        ECR_HOST=$(terraform output -raw ecr_registry_host 2>/dev/null || echo "")
        ECR_REPO=$(terraform output -raw ecr_repo_name 2>/dev/null || echo "")
        if [[ -n "$ECR_HOST" && -n "$ECR_REPO" ]]; then
            REGISTRY="$ECR_HOST/$ECR_REPO"
        fi
    fi
fi

REGISTRY_LOGIN_CMD=$(terraform output -raw registry_login_command 2>/dev/null || echo "")
if [[ -z "$REGISTRY_LOGIN_CMD" ]]; then
    REGISTRY_LOGIN_CMD=$(terraform output -raw ecr_login_command 2>/dev/null || echo "")
fi

cd "$REPO_ROOT"

# ---------------------------------------------------------------------------
# Step 2: cert-manager
# ---------------------------------------------------------------------------
if ! kctl get namespace cert-manager &>/dev/null; then
    log "Step 2: Installing cert-manager"
    retry kctl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.17.2/cert-manager.yaml
    log "Step 2: Waiting for cert-manager to be ready"
    kctl wait --for=condition=available --timeout=180s -n cert-manager \
        deploy/cert-manager \
        deploy/cert-manager-webhook \
        deploy/cert-manager-cainjector
else
    log "Step 2: cert-manager already installed, skipping"
fi

# ---------------------------------------------------------------------------
# Step 3: Pre-apply CRDs with server-side apply
# ---------------------------------------------------------------------------
log "Step 3: Pre-applying CRDs from chart dependencies"

for chart in "$REPO_ROOT"/deploy/operator/charts/*.tgz; do
    chartname=$(basename "$chart" .tgz)
    tmpdir="/tmp/bootstrap-crds/$CTX/$chartname"
    mkdir -p "$tmpdir"
    tar -xzf "$chart" -C "$tmpdir" --strip-components=1 2>/dev/null || true

    for crddir in "$tmpdir/crds" "$tmpdir/files/crds"; do
        if [[ -d "$crddir" ]] && ls "$crddir"/*.yaml &>/dev/null; then
            log "  Applying CRDs from $chartname"
            kctl apply --server-side --force-conflicts --field-manager=helm -f "$crddir/" 2>&1 | grep -v "^$" || true
        fi
    done
done

log "  Applying W&B operator CRDs"
retry kctl apply --server-side --force-conflicts --field-manager=helm \
    -f "$REPO_ROOT/deploy/operator/crds/apps.wandb.com_applications.yaml" \
    -f "$REPO_ROOT/deploy/operator/crds/apps.wandb.com_weightsandbiases.yaml"

log "Step 3: Labeling CRDs for Helm ownership"
for crd in $(kctl get crds -o name | grep -E 'minio|mysql|redis|kafka|strimzi|clickhouse|victoria|grafana|altinity|wandb'); do
    kctl annotate "$crd" meta.helm.sh/release-name=third-party-operators meta.helm.sh/release-namespace=wandb-operator --overwrite 2>/dev/null || true
    kctl label "$crd" app.kubernetes.io/managed-by=Helm --overwrite 2>/dev/null || true
done

# ---------------------------------------------------------------------------
# Step 4: Third-party operators
# ---------------------------------------------------------------------------
log "Step 4: Installing third-party operators"
if ! ls "$REPO_ROOT"/deploy/operator/charts/*.tgz &>/dev/null; then
    helm dependency update "$REPO_ROOT/deploy/operator" 2>&1 | tail -3
fi

# Recover from a stuck Helm release left by a previous failed attempt
HELM_STATUS=$(hctl status third-party-operators -n wandb-operator -o json 2>/dev/null | jq -r '.info.status // empty' || echo "")
if [[ "$HELM_STATUS" == "pending-upgrade" || "$HELM_STATUS" == "pending-install" || "$HELM_STATUS" == "pending-rollback" ]]; then
    log "Step 4: Recovering stuck Helm release (status=$HELM_STATUS)"
    hctl rollback third-party-operators -n wandb-operator 2>/dev/null || true
fi

retry hctl upgrade --install \
    --set=wandb-operator.enabled=false \
    --set=telemetry.enabled=false \
    --set=altinity-clickhouse-operator.crdHook.enabled=false \
    --create-namespace \
    --namespace wandb-operator \
    --skip-crds \
    --no-hooks \
    third-party-operators "$REPO_ROOT/deploy/operator"

log "Step 4: Waiting for third-party operator deployments"
kctl wait --for=condition=available --timeout=300s -n wandb-operator \
    deploy -l app.kubernetes.io/managed-by=Helm 2>/dev/null || true

# ---------------------------------------------------------------------------
# Step 5: Operator manifests (RBAC, webhooks, certs, namespace)
# ---------------------------------------------------------------------------
log "Step 5: Generating operator manifests"
cd "$REPO_ROOT"
LOCKFILE="/tmp/bootstrap-manifests.lock"
if ( set -o noclobber; echo $$ > "$LOCKFILE" ) 2>/dev/null; then
    make manifests generate 2>&1 | tail -3
    rm -f "$LOCKFILE"
else
    log "Step 5: Waiting for another instance to finish generating manifests"
    while [[ -f "$LOCKFILE" ]]; do sleep 1; done
fi

log "Step 5: Applying operator manifests (namespace, RBAC, webhooks, certs)"
retry bash -c "kubectl kustomize config/default | kubectl --context '$CTX' apply --server-side --force-conflicts -f -"

log "Step 5: Waiting for W&B CRDs to be established"
kctl wait --for=condition=established --timeout=120s \
    crd/applications.apps.wandb.com \
    crd/weightsandbiases.apps.wandb.com

# ---------------------------------------------------------------------------
# Step 6: Build and deploy operator controller
# ---------------------------------------------------------------------------
if [[ "$SKIP_BUILD" == "false" && -z "$OPERATOR_IMAGE" ]]; then
    BUILD_LOCK="/tmp/bootstrap-build.lock"

    # Serialize go build — identical output for all clouds
    if ( set -o noclobber; echo $$ > "$BUILD_LOCK" ) 2>/dev/null; then
        log "Step 6: Building operator binary"
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o tilt_bin/manager cmd/main.go
        rm -f "$BUILD_LOCK"
    else
        log "Step 6: Waiting for another instance to finish building"
        while [[ -f "$BUILD_LOCK" ]]; do sleep 1; done
    fi

    if [[ -n "$REGISTRY" ]]; then
        if [[ -n "$REGISTRY_LOGIN_CMD" ]]; then
            log "Step 6: Authenticating to container registry"
            eval "$REGISTRY_LOGIN_CMD"
        fi

        OPERATOR_IMAGE="$REGISTRY:bootstrap-$(date +%s)"
        log "Step 6: Building and pushing operator image: $OPERATOR_IMAGE"
        docker build --platform=linux/amd64 -t "$OPERATOR_IMAGE" -f - . <<'DOCKERFILE'
FROM alpine:3.21
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 65532 nonroot && \
    mkdir -p /helm/.cache/helm /helm/.config/helm /helm/.local/share/helm && \
    chown -R nonroot:nonroot /helm
ADD tilt_bin/manager /manager
ADD hack/testing-manifests/server-manifest /server-manifest
ENV HELM_CACHE_HOME=/helm/.cache/helm
ENV HELM_CONFIG_HOME=/helm/.config/helm
ENV HELM_DATA_HOME=/helm/.local/share/helm
USER 65532
ENTRYPOINT ["/manager", "--log-format=pretty", "--telemetry-enabled=true"]
DOCKERFILE

        # Serialize pushes — concurrent pushes of shared layers crash the Docker daemon
        PUSH_LOCK="/tmp/bootstrap-push.lock"
        while ! ( set -o noclobber; echo $$ > "$PUSH_LOCK" ) 2>/dev/null; do
            log "Step 6: Waiting for another push to finish"
            sleep 5
        done
        RETRY_MAX=3 RETRY_DELAY=5 retry docker push "$OPERATOR_IMAGE"
        rm -f "$PUSH_LOCK"
    else
        echo "WARNING: No registry configured. Set --registry or create_ecr/create_registry in terraform."
        echo "         Falling back to published image."
        SKIP_BUILD=true
    fi
fi

PUBLISHED_IMAGE="us-docker.pkg.dev/wandb-production/public/wandb/operator:internal-testing.3"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-$PUBLISHED_IMAGE}"

log "Step 6: Patching operator deployment with image: $OPERATOR_IMAGE"
kctl set image -n operator-system \
    deploy/operator-controller-manager \
    manager="$OPERATOR_IMAGE" 2>/dev/null || true

log "Step 6: Waiting for operator controller to be ready"
kctl wait --for=condition=available --timeout=180s -n operator-system \
    deploy/operator-controller-manager 2>/dev/null || \
    echo "WARNING: Operator controller not ready yet — it may need the image to be set"

# ---------------------------------------------------------------------------
# Step 7: Wait for webhook readiness
# ---------------------------------------------------------------------------
log "Step 7: Waiting for webhook CA bundle injection"
for i in $(seq 1 60); do
    ca=$(kctl get mutatingwebhookconfiguration operator-mutating-webhook-configuration \
        -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null || echo "")
    if [[ -n "$ca" ]]; then
        log "Step 7: Webhook is ready"
        break
    fi
    if [[ "$i" -eq 60 ]]; then
        echo "WARNING: Webhook CA bundle not injected after 120s"
    fi
    sleep 2
done

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
log "Setup complete!"
echo ""
echo "  Cluster:    $CLUSTER_NAME"
echo "  Context:    $CTX"
echo "  Operator:   $OPERATOR_IMAGE"
echo ""
echo "  Next: $SCRIPT_DIR/cluster_deploy.sh $TF_DIR [--overlay ...]"
echo ""
