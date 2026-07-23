#!/usr/bin/env bash
#
# verify-fresh-deploy.sh
#
# End-to-end verification for operator code changes: destroy and redeploy the
# local kind cluster FROM SCRATCH, then run `wandb verify` against the fresh
# W&B instance. Every code sub-PR in the v2 migration bugfix work is expected
# to pass this harness (see the plan's "Verification requirement" section).
#
# The script:
#   1. Destroys the existing kind cluster (hack/scripts/teardown_kind.sh)
#   2. Recreates it (hack/scripts/setup_kind.sh)
#   3. Runs an optional --pre-deploy hook (seed secrets, install cert-manager,
#      apply a custom/v1 CR, etc.) BEFORE the operator reconciles
#   4. Deploys the operator + CR headlessly via `tilt ci`
#   5. Optionally applies a custom CR (--cr) and waits for it to go Available
#   6. Runs an optional --post-deploy hook
#   7. Prompts for a W&B API key (only if WANDB_API_KEY is unset), logs in, and
#      runs `wandb verify`
#
# It is intentionally interactive: `wandb verify` needs an account on the
# freshly deployed instance, so the API key is prompted for when necessary.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# --- defaults -----------------------------------------------------------------
HOST=""
CR_FILE=""
PRE_DEPLOY=""
POST_DEPLOY=""
NAMESPACE="wandb"
WAIT_TIMEOUT="1200s"
SKIP_VERIFY="false"

usage() {
    cat <<'EOF'
Usage: hack/scripts/verify-fresh-deploy.sh [options]

Destroys and recreates the local kind cluster, deploys the operator via Tilt,
and runs `wandb verify` against the fresh instance.

Options:
  --host URL           W&B base URL to verify against.
                       Defaults to `wandbHostname` from tilt-settings.star,
                       falling back to http://localhost:8080.
  --cr PATH            Apply this CR (server-side) after Tilt is up and wait for
                       it to become Available. Use for v1/custom CR fixtures.
  --pre-deploy SCRIPT  Executable run after the cluster is created but BEFORE
                       Tilt deploys (e.g. seed a Secret, install cert-manager).
  --post-deploy SCRIPT Executable run after the deployment is healthy but BEFORE
                       `wandb verify`.
  --namespace NS       Namespace the W&B app is deployed into. Default: wandb.
  --timeout DURATION   kubectl wait timeout for readiness. Default: 1200s.
  --skip-verify        Deploy only; skip the `wandb verify` step.
  -h, --help           Show this help.

Environment:
  WANDB_API_KEY   If set, used non-interactively. If unset, you are prompted.

Examples:
  # Clean redeploy + verify
  hack/scripts/verify-fresh-deploy.sh

  # Issue 3: convert a v1 CR with an external ClickHouse, then verify
  hack/scripts/verify-fresh-deploy.sh --cr hack/testing-manifests/wandb/wandb-external-clickhouse-v1.yaml

  # Issue 4: seed a binary weave-worker-auth secret before deploy, then verify
  hack/scripts/verify-fresh-deploy.sh --pre-deploy hack/scripts/seed-binary-weave-auth.sh
EOF
}

# --- arg parsing --------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --host) HOST="$2"; shift 2 ;;
        --cr) CR_FILE="$2"; shift 2 ;;
        --pre-deploy) PRE_DEPLOY="$2"; shift 2 ;;
        --post-deploy) POST_DEPLOY="$2"; shift 2 ;;
        --namespace) NAMESPACE="$2"; shift 2 ;;
        --timeout) WAIT_TIMEOUT="$2"; shift 2 ;;
        --skip-verify) SKIP_VERIFY="true"; shift ;;
        -h|--help) usage; exit 0 ;;
        *) echo "Error: unknown argument '$1'" >&2; usage >&2; exit 1 ;;
    esac
done

# --- dependency checks --------------------------------------------------------
require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "Error: required command '$1' not found in PATH." >&2
        echo "       Install it before running this script (see README.md)." >&2
        exit 1
    fi
}

require_cmd kind
require_cmd kubectl
require_cmd tilt
if [[ "$SKIP_VERIFY" != "true" ]]; then
    require_cmd wandb
fi

# --- resolve host from tilt-settings.star (mirrors setup_kind.sh) -------------
read_star_setting() {
    local key="$1"
    local file="${REPO_ROOT}/tilt-settings.star"
    if [[ -f "$file" ]]; then
        grep "\"$key\"" "$file" | sed 's/.*: *"\(.*\)".*/\1/' | head -1
    fi
}

if [[ -z "$HOST" ]]; then
    HOST="$(read_star_setting "wandbHostname")"
fi
if [[ -z "$HOST" ]]; then
    HOST="http://localhost:8080"
fi

cd "$REPO_ROOT"

echo "==> Destroying existing kind cluster (from scratch)"
./hack/scripts/teardown_kind.sh

echo "==> Recreating kind cluster"
./hack/scripts/setup_kind.sh

if [[ -n "$PRE_DEPLOY" ]]; then
    echo "==> Running pre-deploy hook: $PRE_DEPLOY"
    bash "$PRE_DEPLOY"
fi

echo "==> Deploying operator + CR via 'tilt ci' (this blocks until healthy)"
tilt ci

if [[ -n "$CR_FILE" ]]; then
    echo "==> Applying custom CR: $CR_FILE"
    kubectl apply --server-side --force-conflicts -f "$CR_FILE"
    echo "==> Waiting for WeightsAndBiases to become Available"
    # Best-effort: the CR exposes an Available condition once reconciled.
    kubectl wait --for=condition=Available weightsandbiases --all \
        -n "$NAMESPACE" --timeout="$WAIT_TIMEOUT" || {
        echo "Warning: 'Available' condition not reported; falling back to pod readiness." >&2
        kubectl wait --for=condition=Ready pods --all -n "$NAMESPACE" --timeout="$WAIT_TIMEOUT"
    }
fi

if [[ -n "$POST_DEPLOY" ]]; then
    echo "==> Running post-deploy hook: $POST_DEPLOY"
    bash "$POST_DEPLOY"
fi

if [[ "$SKIP_VERIFY" == "true" ]]; then
    echo "==> --skip-verify set; deployment is up at ${HOST}. Done."
    exit 0
fi

echo "==> Verifying W&B at ${HOST}"
export WANDB_BASE_URL="$HOST"

if [[ -z "${WANDB_API_KEY:-}" ]]; then
    # The key comes from signing up on the freshly deployed instance at $HOST.
    read -rsp "Enter W&B API key for ${WANDB_BASE_URL}: " WANDB_API_KEY
    echo
    export WANDB_API_KEY
fi

if [[ -z "${WANDB_API_KEY:-}" ]]; then
    echo "Error: no API key provided; cannot run 'wandb verify'." >&2
    exit 1
fi

wandb login --relogin --host "$WANDB_BASE_URL" "$WANDB_API_KEY"
wandb verify

echo "==> wandb verify passed against ${HOST}"
