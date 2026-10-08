#!/usr/bin/env bash

set -euo pipefail

SCRIPT_NAME="$(basename "$0")"
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-wandb-operators}"
LEGACY_OPERATOR_NAMESPACE="wandb-operator"
WAIT_TIMEOUT="10m"
DRY_RUN="false"
APP_NAMESPACE=""
APP_NAME=""
KUBE_CONTEXT=""

kubectl() {
  if [[ -n "${KUBE_CONTEXT}" ]]; then
    command kubectl --context "${KUBE_CONTEXT}" "$@"
  else
    command kubectl "$@"
  fi
}

helm() {
  if [[ -n "${KUBE_CONTEXT}" ]]; then
    command helm --kube-context "${KUBE_CONTEXT}" "$@"
  else
    command helm "$@"
  fi
}

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [--namespace <ns>] [--name <wandb-name>] [--dry-run]

Safely cleans up dev W&B installs before a fresh Tilt rebuild:
1. Stops test Launch agents/jobs, then deletes the W&B CR while the operator is running
2. Waits for finalizer-driven cleanup to complete
3. Uninstalls Tilt-managed Helm releases
4. Deletes dev-only PVCs and generated secrets for the app

By default, it targets all WeightsAndBiases resources labeled:
  app.kubernetes.io/managed-by=tilt

Options:
  --context <context> Select the Kubernetes context for kubectl and Helm
  --namespace <ns>   Limit cleanup to a single namespace
  --name <name>      Limit cleanup to a single WeightsAndBiases resource
  --dry-run          Print actions without executing them
  -h, --help         Show this help text
EOF
}

log() {
  printf '[%s] %s\n' "${SCRIPT_NAME}" "$*"
}

run() {
  if [[ "${DRY_RUN}" == "true" ]]; then
    printf '[dry-run] %s\n' "$*"
    return 0
  fi

  "$@"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --context)
      KUBE_CONTEXT="${2:?missing context value}"
      shift 2
      ;;
    --namespace)
      APP_NAMESPACE="${2:?missing namespace value}"
      shift 2
      ;;
    --name)
      APP_NAME="${2:?missing name value}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -n "${APP_NAME}" && -z "${APP_NAMESPACE}" ]]; then
  echo "--name requires --namespace" >&2
  exit 1
fi

discover_apps() {
  if [[ -n "${APP_NAME}" ]]; then
    kubectl get weightsandbiases.apps.wandb.com "${APP_NAME}" -n "${APP_NAMESPACE}" -o json |
      jq -r '[.metadata.namespace, .metadata.name] | @tsv'
    return
  fi

  kubectl get weightsandbiases.apps.wandb.com -A -o json |
    jq -r '
      .items[]
      | select(.metadata.labels["app.kubernetes.io/managed-by"] == "tilt")
      | [.metadata.namespace, .metadata.name]
      | @tsv
    '
}

delete_app_and_wait() {
  local namespace="$1"
  local name="$2"

  log "Deleting WeightsAndBiases ${namespace}/${name}"
  run kubectl delete weightsandbiases.apps.wandb.com "${name}" -n "${namespace}" --ignore-not-found

  if [[ "${DRY_RUN}" == "true" ]]; then
    printf '[dry-run] kubectl wait --for=delete weightsandbiases.apps.wandb.com/%s -n %s --timeout=%s\n' "${name}" "${namespace}" "${WAIT_TIMEOUT}"
    return 0
  fi

  if ! kubectl wait --for=delete "weightsandbiases.apps.wandb.com/${name}" -n "${namespace}" --timeout="${WAIT_TIMEOUT}"; then
    log "Timed out waiting for ${namespace}/${name} to finish finalizer cleanup"
    kubectl get weightsandbiases.apps.wandb.com "${name}" -n "${namespace}" -o yaml || true
    exit 1
  fi
}

delete_launch_for_app() {
  local namespace="$1"
  local name="$2"
  local launch_namespace release_name
  while IFS=$'\t' read -r launch_namespace release_name; do
    [[ -z "${launch_namespace}" ]] && continue
    if [[ -n "${release_name}" ]]; then
      uninstall_release "${launch_namespace}" "${release_name}"
    fi
    log "Deleting test Launch namespace ${launch_namespace}"
    run kubectl delete namespace "${launch_namespace}" --ignore-not-found --timeout="${WAIT_TIMEOUT}"
  done < <(kubectl get namespaces \
    -l "app.kubernetes.io/managed-by=wandb-dev-bootstrap,wandb.ai/test-instance=${name},wandb.ai/test-namespace=${namespace}" \
    -o json | jq -r '.items[] | [.metadata.name, (.metadata.annotations["wandb.ai/launch-release"] // "")] | @tsv')
}

delete_state_for_app() {
  local namespace="$1"
  local name="$2"

  log "Deleting dev PVCs for ${namespace}/${name}"
  run kubectl delete pvc -n "${namespace}" -l "weightsandbiases.apps.wandb.com/name=${name}" --ignore-not-found

  log "Deleting labeled dev secrets for ${namespace}/${name}"
  run kubectl delete secret -n "${namespace}" -l "weightsandbiases.apps.wandb.com/name=${name}" --ignore-not-found

  local prefixed_secrets
  prefixed_secrets="$(kubectl get secret -n "${namespace}" -o json |
    jq -r --arg prefix "${name}" '
      .items[]
      | select(.metadata.name | startswith($prefix + "-"))
      | .metadata.name
    ')"

  if [[ -n "${prefixed_secrets}" ]]; then
    log "Deleting prefixed dev secrets for ${namespace}/${name}"
    while IFS= read -r secret_name; do
      [[ -z "${secret_name}" ]] && continue
      run kubectl delete secret -n "${namespace}" "${secret_name}" --ignore-not-found
    done <<< "${prefixed_secrets}"
  fi

  run kubectl delete secret -n "${namespace}" wandb-otel-connection --ignore-not-found
  run kubectl delete secret -n "${namespace}" \
    -l "app.kubernetes.io/managed-by=wandb-dev-bootstrap,wandb.ai/test-instance=${name},wandb.ai/test-namespace=${namespace}" --ignore-not-found
}

uninstall_release() {
  local namespace="$1"
  local release_name="$2"

  if helm status "${release_name}" --namespace "${namespace}" >/dev/null 2>&1; then
    log "Uninstalling Helm release ${namespace}/${release_name}"
    run helm uninstall "${release_name}" --namespace "${namespace}"
  else
    log "Helm release ${namespace}/${release_name} is already absent"
  fi
}

apps=()
while IFS= read -r app; do
  [[ -z "${app}" ]] && continue
  apps+=("${app}")
done < <(discover_apps)

if [[ "${#apps[@]}" -eq 0 ]]; then
  log "No Tilt-managed WeightsAndBiases resources found"
else
  log "Found ${#apps[@]} Tilt-managed WeightsAndBiases resource(s)"
fi

for app in "${apps[@]}"; do
  namespace="${app%%$'\t'*}"
  name="${app##*$'\t'}"
  delete_launch_for_app "${namespace}" "${name}"
  delete_app_and_wait "${namespace}" "${name}"
done

uninstall_release "${OPERATOR_NAMESPACE}" "wandb-operator"
uninstall_release "${LEGACY_OPERATOR_NAMESPACE}" "telemetry-stack"
uninstall_release "${LEGACY_OPERATOR_NAMESPACE}" "third-party-operators"

for app in "${apps[@]}"; do
  namespace="${app%%$'\t'*}"
  name="${app##*$'\t'}"
  delete_state_for_app "${namespace}" "${name}"
done

log "Dev cleanup complete"
