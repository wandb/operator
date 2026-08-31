#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: telemetry-crd-manager.sh --namespace NAMESPACE --release-name NAME --mode MODE --values FILE [--dry-run]

Prepare telemetry CRDs and enable telemetry.

Options:
  --namespace NAMESPACE   Operator release namespace
  --release-name NAME     Operator Helm release name
  --mode MODE             Telemetry mode: off, forward, or full
  --values FILE           Values for the telemetry-enabling upgrade
  --dry-run               Render both upgrades without applying them
  -h, --help              Show this help
EOF
}

fail() {
  echo "Error: $*" >&2
  exit 1
}

step() {
  echo "[$1/5] $2"
}

namespace=""
release_name=""
mode=""
values_file=""
dry_run=false

while (($# > 0)); do
  case "$1" in
    --namespace)
      (($# >= 2)) || fail "--namespace requires a value"
      namespace="$2"
      shift 2
      ;;
    --release-name)
      (($# >= 2)) || fail "--release-name requires a value"
      release_name="$2"
      shift 2
      ;;
    --mode)
      (($# >= 2)) || fail "--mode requires a value"
      mode="$2"
      shift 2
      ;;
    --values)
      (($# >= 2)) || fail "--values requires a value"
      values_file="$2"
      shift 2
      ;;
    --dry-run)
      dry_run=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

[[ -n "${namespace}" ]] || fail "--namespace is required"
[[ -n "${release_name}" ]] || fail "--release-name is required"

case "${mode}" in
  off)
    echo "Telemetry is off; no CRD preparation is needed."
    exit 0
    ;;
  forward)
    grafana_crds=false
    required_crds=(vmsingles.operator.victoriametrics.com)
    crd_pattern='operator\.victoriametrics\.com'
    ;;
  full)
    grafana_crds=true
    required_crds=(grafanas.grafana.integreatly.org vmsingles.operator.victoriametrics.com)
    crd_pattern='grafana\.integreatly\.org|operator\.victoriametrics\.com'
    ;;
  *)
    fail "--mode must be off, forward, or full"
    ;;
esac

[[ -n "${values_file}" ]] || fail "--values is required"
[[ -f "${values_file}" ]] || fail "values file does not exist: ${values_file}"

step 1 "Checking prerequisites"
for command in helm kubectl grep xargs; do
  command -v "${command}" >/dev/null 2>&1 || fail "${command} is required"
done
kubectl version --request-timeout=5s >/dev/null 2>&1 || fail "Kubernetes cluster is not reachable"
[[ -d ./deploy/operator ]] || fail "run this script from the operator repository root"

step 2 "Checking the Helm release and telemetry CRDs"
if ! releases="$(helm list -n "${namespace}" -q)"; then
  fail "unable to list Helm releases in ${namespace}"
fi
if ! grep -Fxq "${release_name}" <<<"${releases}"; then
  echo "Release ${release_name} does not exist; the fresh install will handle CRDs."
  exit 0
fi
if [[ "${dry_run}" == false ]] && kubectl get crd "${required_crds[@]}" >/dev/null 2>&1; then
  step 3 "Required telemetry CRDs are already installed"
  step 4 "Telemetry CRDs are established"
else
  step 3 "Running the CRD preparation upgrade"
  prepare_args=(
    upgrade "${release_name}" ./deploy/operator
    --namespace "${namespace}"
    --reuse-values
    --set telemetry.mode=off
    --set telemetry.crds.victoriaMetrics=true
    --set telemetry.crds.grafana="${grafana_crds}"
    --set victoria-metrics-operator.enabled=false
    --set grafana-operator.enabled=false
    --wait
    --timeout 10m
  )
  if [[ "${dry_run}" == true ]]; then
    prepare_args+=(--dry-run=client)
  fi
  helm "${prepare_args[@]}"

  step 4 "Waiting for telemetry CRDs"
  if [[ "${dry_run}" == false ]]; then
    kubectl get crd -o name \
      | grep -E "${crd_pattern}" \
      | xargs kubectl wait --for=condition=Established --timeout=2m
  else
    echo "Dry run; no CRDs were installed."
  fi
fi

step 5 "Enabling ${mode} telemetry"
enable_args=(
  upgrade "${release_name}" ./deploy/operator
  --namespace "${namespace}"
  --reuse-values
  --values "${values_file}"
  --wait
  --timeout 10m
)
if [[ "${dry_run}" == true ]]; then
  enable_args+=(--dry-run=client)
fi
helm "${enable_args[@]}"
