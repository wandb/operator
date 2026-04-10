#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

usage() {
    echo "Usage: $0 <cloud-tf-dir>"
    echo ""
    echo "Check the health of a W&B operator deployment on a cloud cluster."
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

for cmd in kubectl terraform jq; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: $cmd is required but not found in PATH"
        exit 1
    fi
done

PASS=0
FAIL=0
SKIP=0
BLOCKED=false

pass() { echo "[PASS] $*"; PASS=$((PASS + 1)); BLOCKED=false; }
fail() { echo "[FAIL] $*"; FAIL=$((FAIL + 1)); BLOCKED=true; }
skip() { echo "[SKIP] $*"; SKIP=$((SKIP + 1)); }

# ---------------------------------------------------------------------------
# 0. Terraform state
# ---------------------------------------------------------------------------
CLUSTER_STATE=$("$SCRIPT_DIR/cluster_state.sh" "$TF_DIR" 2>/dev/null | head -1) || true
CLUSTER_NAME=$(cd "$TF_DIR" && terraform output -raw cluster_name 2>/dev/null || echo "")

if [[ "$CLUSTER_STATE" == "READY" && -n "$CLUSTER_NAME" ]]; then
    pass "Terraform state: READY ($CLUSTER_NAME)"
else
    fail "Terraform state: $CLUSTER_STATE"
    "$SCRIPT_DIR/cluster_state.sh" "$TF_DIR" 2>/dev/null | tail -n +2 | sed 's/^/       /' || true
    echo ""
    echo "Summary: $PASS passed, $FAIL failed, $SKIP skipped"
    exit 1
fi

OBJECTSTORE_URL=$(cd "$TF_DIR" && terraform output -raw objectstore_url 2>/dev/null || echo "")

# ---------------------------------------------------------------------------
# 0b. Kubectl context
# ---------------------------------------------------------------------------
if ! kubectl config get-contexts "$CLUSTER_NAME" &>/dev/null; then
    fail "Kubectl context: '$CLUSTER_NAME' not found. Run cluster_kubeconfig.sh first."
    echo ""
    echo "Summary: $PASS passed, $FAIL failed, $SKIP skipped"
    exit 1
fi

kubectl config use-context "$CLUSTER_NAME" &>/dev/null

# Pin all kubectl to this context
kctl() { kubectl --context "$CLUSTER_NAME" "$@"; }
echo ""

# ---------------------------------------------------------------------------
# 1. Cluster connectivity
# ---------------------------------------------------------------------------
NODES_READY=$(kctl get nodes --no-headers 2>/dev/null | grep -c " Ready" || echo "0")
NODES_TOTAL=$(kctl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "$NODES_READY" -gt 0 ]]; then
    pass "Cluster connectivity: $NODES_READY/$NODES_TOTAL nodes Ready"
else
    fail "Cluster connectivity: no Ready nodes ($NODES_TOTAL total)"
fi

# ---------------------------------------------------------------------------
# 2. cert-manager
# ---------------------------------------------------------------------------
if [[ "$BLOCKED" == "true" ]]; then
    skip "cert-manager (requires: cluster connectivity)"
else
    CM_AVAILABLE=$(kctl get deploy -n cert-manager --no-headers 2>/dev/null | grep -c "1/1" || echo "0")
    CM_TOTAL=$(kctl get deploy -n cert-manager --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$CM_TOTAL" -eq 0 ]]; then
        fail "cert-manager: not installed"
    elif [[ "$CM_AVAILABLE" -eq "$CM_TOTAL" ]]; then
        pass "cert-manager: $CM_AVAILABLE/$CM_TOTAL deployments available"
    else
        fail "cert-manager: $CM_AVAILABLE/$CM_TOTAL deployments available"
    fi
fi

# ---------------------------------------------------------------------------
# 3. W&B CRDs
# ---------------------------------------------------------------------------
if [[ "$BLOCKED" == "true" ]]; then
    skip "W&B CRDs (requires: cert-manager)"
else
    CRD_COUNT=0
    for crd in applications.apps.wandb.com weightsandbiases.apps.wandb.com; do
        if kctl get crd "$crd" &>/dev/null; then
            ESTABLISHED=$(kctl get crd "$crd" -o jsonpath='{.status.conditions[?(@.type=="Established")].status}' 2>/dev/null || echo "")
            if [[ "$ESTABLISHED" == "True" ]]; then
                CRD_COUNT=$((CRD_COUNT + 1))
            fi
        fi
    done
    if [[ "$CRD_COUNT" -eq 2 ]]; then
        pass "W&B CRDs: 2/2 established"
    else
        fail "W&B CRDs: $CRD_COUNT/2 established"
    fi
fi

# ---------------------------------------------------------------------------
# 4. Third-party operators
# ---------------------------------------------------------------------------
if [[ "$BLOCKED" == "true" ]]; then
    skip "Third-party operators (requires: W&B CRDs)"
else
    TP_DEPLOY_TOTAL=$(kctl get deploy -n wandb-operator --no-headers 2>/dev/null | wc -l | tr -d ' ')
    TP_DEPLOY_AVAILABLE=$(kctl get deploy -n wandb-operator --no-headers 2>/dev/null | awk '{split($2,a,"/"); if(a[1]==a[2]) count++} END{print count+0}')
    if [[ "$TP_DEPLOY_TOTAL" -eq 0 ]]; then
        fail "Third-party operators: no deployments in wandb-operator namespace"
    elif [[ "$TP_DEPLOY_AVAILABLE" -eq "$TP_DEPLOY_TOTAL" ]]; then
        pass "Third-party operators: $TP_DEPLOY_AVAILABLE/$TP_DEPLOY_TOTAL deployments available"
    else
        fail "Third-party operators: $TP_DEPLOY_AVAILABLE/$TP_DEPLOY_TOTAL deployments available"
    fi
fi

# ---------------------------------------------------------------------------
# 5. Operator namespace
# ---------------------------------------------------------------------------
if [[ "$BLOCKED" == "true" ]]; then
    skip "Operator namespace (requires: third-party operators)"
else
    if kctl get namespace operator-system &>/dev/null; then
        pass "Operator namespace: operator-system exists"
    else
        fail "Operator namespace: operator-system not found"
    fi
fi

# ---------------------------------------------------------------------------
# 6. Operator RBAC
# ---------------------------------------------------------------------------
if [[ "$BLOCKED" == "true" ]]; then
    skip "Operator RBAC (requires: operator namespace)"
else
    RBAC_EXPECTED=("operator-manager-role" "operator-manager-rolebinding" "operator-leader-election-role")
    RBAC_FOUND=0
    for r in "${RBAC_EXPECTED[@]}"; do
        if kctl get clusterrole "$r" &>/dev/null 2>&1 || kctl get clusterrolebinding "$r" &>/dev/null 2>&1 || kctl get role "$r" -n operator-system &>/dev/null 2>&1; then
            RBAC_FOUND=$((RBAC_FOUND + 1))
        fi
    done
    if [[ "$RBAC_FOUND" -eq "${#RBAC_EXPECTED[@]}" ]]; then
        pass "Operator RBAC: $RBAC_FOUND/${#RBAC_EXPECTED[@]} key roles present"
    else
        fail "Operator RBAC: $RBAC_FOUND/${#RBAC_EXPECTED[@]} key roles present"
    fi
fi

# ---------------------------------------------------------------------------
# 7. Operator controller
# ---------------------------------------------------------------------------
if [[ "$BLOCKED" == "true" ]]; then
    skip "Operator controller (requires: operator RBAC)"
else
    OC_STATUS=$(kctl get deploy operator-controller-manager -n operator-system -o jsonpath='{.status.availableReplicas}' 2>/dev/null || echo "0")
    if [[ "${OC_STATUS:-0}" -ge 1 ]]; then
        pass "Operator controller: available"
    else
        fail "Operator controller: not available"
    fi
fi

# ---------------------------------------------------------------------------
# 8. Webhooks
# ---------------------------------------------------------------------------
if [[ "$BLOCKED" == "true" ]]; then
    skip "Webhooks (requires: operator controller)"
else
    CA_BUNDLE=$(kctl get mutatingwebhookconfiguration operator-mutating-webhook-configuration \
        -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null || echo "")
    if [[ -n "$CA_BUNDLE" ]]; then
        pass "Webhooks: CA bundle injected"
    else
        fail "Webhooks: CA bundle not injected"
    fi
fi

# ---------------------------------------------------------------------------
# 9. W&B CR
# ---------------------------------------------------------------------------
if [[ "$BLOCKED" == "true" ]]; then
    skip "W&B CR (requires: webhooks)"
else
    WANDB_CR=$(kctl get weightsandbiases -A --no-headers 2>/dev/null | head -1 || echo "")
    if [[ -n "$WANDB_CR" ]]; then
        CR_NS=$(echo "$WANDB_CR" | awk '{print $1}')
        CR_NAME=$(echo "$WANDB_CR" | awk '{print $2}')
        CR_PHASE=$(kctl get weightsandbiases "$CR_NAME" -n "$CR_NS" -o jsonpath='{.status.phase}' 2>/dev/null || echo "unknown")
        pass "W&B CR: $CR_NAME in $CR_NS (phase=$CR_PHASE)"
    else
        fail "W&B CR: no WeightsAndBiases resource found"
    fi
fi

# ---------------------------------------------------------------------------
# 10. External secrets
# ---------------------------------------------------------------------------
if [[ -n "$OBJECTSTORE_URL" ]]; then
    if kctl get secret external-objectstore-connection -n default &>/dev/null; then
        pass "External secrets: external-objectstore-connection present"
    else
        fail "External secrets: external-objectstore-connection missing (object store configured in TF)"
    fi
else
    skip "External secrets: no object store configured in terraform"
fi

# ---------------------------------------------------------------------------
# 11. W&B pods
# ---------------------------------------------------------------------------
if [[ "$BLOCKED" == "true" ]]; then
    skip "W&B pods (requires: W&B CR)"
else
    WANDB_PODS_RUNNING=$(kctl get pods -n default --no-headers 2>/dev/null | grep -c "Running" || echo "0")
    WANDB_PODS_TOTAL=$(kctl get pods -n default --no-headers 2>/dev/null | grep -v "Completed" | wc -l | tr -d ' ')
    if [[ "$WANDB_PODS_TOTAL" -eq 0 ]]; then
        fail "W&B pods: no pods in default namespace"
    elif [[ "$WANDB_PODS_RUNNING" -eq "$WANDB_PODS_TOTAL" ]]; then
        pass "W&B pods: $WANDB_PODS_RUNNING/$WANDB_PODS_TOTAL running"
    else
        fail "W&B pods: $WANDB_PODS_RUNNING/$WANDB_PODS_TOTAL running"
        kctl get pods -n default --no-headers 2>/dev/null | grep -v "Running\|Completed" | awk '{print "       " $1 " " $3}' || true
    fi
fi

# ---------------------------------------------------------------------------
# 12. Telemetry
# ---------------------------------------------------------------------------
TEL_RELEASE=$(kctl get secret -n wandb-operator -l name=telemetry-stack,owner=helm --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "$TEL_RELEASE" -gt 0 ]]; then
    VM_PODS=$(kctl get pods -n default -l 'app.kubernetes.io/name in (vmsingle,vlsingle,vtsingle,vmagent)' --no-headers 2>/dev/null | wc -l | tr -d ' ')
    GRAFANA_PODS=$(kctl get pods -n default -l app=grafana --no-headers 2>/dev/null | wc -l | tr -d ' ')
    TOTAL_TEL=$((VM_PODS + GRAFANA_PODS))
    if [[ "$TOTAL_TEL" -gt 0 ]]; then
        pass "Telemetry: $TOTAL_TEL pods found (vm=$VM_PODS, grafana=$GRAFANA_PODS)"
    else
        fail "Telemetry: helm release exists but no pods found"
    fi
else
    skip "Telemetry: not installed"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "Summary: $PASS passed, $FAIL failed, $SKIP skipped"
if [[ "$FAIL" -gt 0 ]]; then
    exit 1
fi
exit 0
