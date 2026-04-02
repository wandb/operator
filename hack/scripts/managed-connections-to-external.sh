#!/usr/bin/env bash
set -euo pipefail

if ! command -v kubectl >/dev/null 2>&1; then
  echo "error: kubectl is required but not installed" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required but not installed" >&2
  exit 1
fi

APP_PREFIX="${1:-wandb}"
NAMESPACE="${2:-default}"
SCRIPT_NAME="$(basename "$0")"

log() {
  printf '[%s] %s\n' "${SCRIPT_NAME}" "$*"
}

secrets="$(kubectl get secrets -n "${NAMESPACE}" -o json |
  jq -r --arg prefix "${APP_PREFIX}" '
    .items[]
    | select(.metadata.name | test("^" + $prefix + "-.*-connection$"))
    | .metadata.name
  ')"

if [[ -z "${secrets}" ]]; then
  log "No secrets matching '${APP_PREFIX}-*-connection' found in namespace '${NAMESPACE}'"
  exit 0
fi

failed=()

while IFS= read -r secret_name; do
  [[ -z "${secret_name}" ]] && continue

  suffix="${secret_name#"${APP_PREFIX}-"}"
  new_name="external-${suffix}"

  log "Copying '${secret_name}' -> '${new_name}'"

  kubectl get secret "${secret_name}" -n "${NAMESPACE}" -o json |
    jq --arg name "${new_name}" '
      .metadata.name = $name
      | del(.metadata.uid, .metadata.resourceVersion, .metadata.creationTimestamp,
            .metadata.ownerReferences, .metadata.managedFields, .metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"])
    ' |
    kubectl apply -n "${NAMESPACE}" -f - || {
      log "Failed to create '${new_name}', skipping deletion of '${secret_name}'"
      failed+=("${secret_name}")
      continue
    }

  log "Deleting '${secret_name}'"
  kubectl delete secret "${secret_name}" -n "${NAMESPACE}"
done <<< "${secrets}"

if [[ "${#failed[@]}" -gt 0 ]]; then
  log "Completed with errors. Failed to migrate: ${failed[*]}"
  exit 1
fi

log "Done"
