#!/usr/bin/env bash

set -euo pipefail

NAMESPACE="wandb-ca-e2e"
NAME="wandb"
API_APP="api"
TIMEOUT="20m"
POLL_SECONDS=10

usage() {
  cat <<EOF
Usage: $(basename "$0") [--namespace <ns>] [--name <wandb-name>] [--api-app <application>] [--timeout <duration>]

Verifies the Tilt custom CA e2e path:
  - inline and user-provided CA ConfigMaps
  - generated MySQL/Redis connection URL CA parameters when configured
  - W&B workload pod template env, volumes, mounts, and checksum annotation
  - live workload pod CA files when a ready workload pod is available
  - recent workload logs for TLS trust failures
EOF
}

log() {
  printf '[custom-ca-e2e] %s\n' "$*"
}

fail() {
  printf '[custom-ca-e2e] ERROR: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

duration_seconds() {
  case "$1" in
    *m) echo $((${1%m} * 60)) ;;
    *s) echo "${1%s}" ;;
    *) echo "$1" ;;
  esac
}

wait_until() {
  local description="$1"
  shift
  local deadline
  deadline=$(($(date +%s) + $(duration_seconds "${TIMEOUT}")))

  log "Waiting for ${description}"
  until "$@"; do
    if [[ "$(date +%s)" -ge "${deadline}" ]]; then
      fail "timed out waiting for ${description}"
    fi
    sleep "${POLL_SECONDS}"
  done
}

secret_value() {
  local secret="$1"
  local key="$2"
  kubectl -n "${NAMESPACE}" get secret "${secret}" -o json |
    python3 -c 'import base64,json,sys; obj=json.load(sys.stdin); print(base64.b64decode(obj["data"][sys.argv[1]]).decode())' "${key}"
}

assert_url_param() {
  local url="$1"
  local key="$2"
  local expected="$3"
  python3 - "${url}" "${key}" "${expected}" <<'PY'
import sys
from urllib.parse import parse_qs, urlparse

url, key, expected = sys.argv[1:]
actual = parse_qs(urlparse(url).query).get(key, [""])[0]
if actual != expected:
    raise SystemExit(f"{key}={actual!r}, expected {expected!r} in {url}")
PY
}

json_has_env() {
  local name="$1"
  jq -e --arg name "${name}" '[.spec.containers[]?.env[]? | select(.name == $name)] | length > 0' >/dev/null
}

json_has_volume() {
  local name="$1"
  jq -e --arg name "${name}" '[.spec.volumes[]? | select(.name == $name)] | length > 0' >/dev/null
}

json_has_mount() {
  local name="$1"
  local path="$2"
  jq -e --arg name "${name}" --arg path "${path}" \
    '[.spec.containers[]?.volumeMounts[]? | select(.name == $name and .mountPath == $path)] | length > 0' >/dev/null
}

check_wandb_ready() {
  kubectl -n "${NAMESPACE}" get weightsandbiases.apps.wandb.com "${NAME}" -o json |
    jq -e '.status.ready == true' >/dev/null
}

check_application_ready() {
  kubectl -n "${NAMESPACE}" get application "${API_APP}" -o json |
    jq -e '.status.ready == true' >/dev/null
}

selected_migration_job_json() {
  kubectl -n "${NAMESPACE}" get jobs \
    -l "app.kubernetes.io/component=migration,app.kubernetes.io/instance=${NAME},app.kubernetes.io/managed-by=wandb-operator" \
    -o json |
    jq -er '
      [
        .items[]
        | select(.spec.template.metadata.annotations["weightsandbiases.apps.wandb.com/ca-certs-checksum"]? != null)
      ]
      | sort_by(.status.succeeded // 0)
      | reverse
      | .[0]
    '
}

check_migration_workload_exists() {
  selected_migration_job_json >/dev/null
}

select_workload_template() {
  if kubectl -n "${NAMESPACE}" get application "${API_APP}" >/dev/null 2>&1; then
    wait_until "Application ${NAMESPACE}/${API_APP} status.ready=true" check_application_ready
    WORKLOAD_KIND="Application"
    WORKLOAD_NAME="${API_APP}"
    WORKLOAD_POD_SELECTOR="app.kubernetes.io/name=${API_APP},app.kubernetes.io/instance=${NAME}"
    WORKLOAD_TEMPLATE_JSON="$(kubectl -n "${NAMESPACE}" get application "${API_APP}" -o json | jq -c '.spec.podTemplate')"
    return
  fi

  log "Application ${NAMESPACE}/${API_APP} not found; falling back to a custom-CA-injected W&B migration job"
  wait_until "custom-CA-injected W&B migration job" check_migration_workload_exists
  local job_json
  job_json="$(selected_migration_job_json)"
  WORKLOAD_KIND="Job"
  WORKLOAD_NAME="$(echo "${job_json}" | jq -r '.metadata.name')"
  WORKLOAD_POD_SELECTOR="job-name=${WORKLOAD_NAME}"
  WORKLOAD_TEMPLATE_JSON="$(echo "${job_json}" | jq -c '.spec.template')"
}

ready_workload_pod_name() {
  kubectl -n "${NAMESPACE}" get pods -l "${WORKLOAD_POD_SELECTOR}" -o json |
    jq -r '
      .items[]
      | select(.status.phase == "Running")
      | select(any(.status.containerStatuses[]?; .ready == true))
      | .metadata.name
    ' |
    head -n 1
}

check_inline_configmap() {
  kubectl -n "${NAMESPACE}" get configmap "${NAME}-ca-certs" -o json |
    jq -e '.data["customCA0.crt"] | contains("BEGIN CERTIFICATE")' >/dev/null
}

check_user_configmap() {
  kubectl -n "${NAMESPACE}" get configmap "${USER_CONFIGMAP}" -o json |
    jq -e '.data | to_entries | any(.value | contains("BEGIN CERTIFICATE"))' >/dev/null
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)
      NAMESPACE="${2:?missing namespace}"
      shift 2
      ;;
    --name)
      NAME="${2:?missing name}"
      shift 2
      ;;
    --api-app)
      API_APP="${2:?missing api app name}"
      shift 2
      ;;
    --timeout)
      TIMEOUT="${2:?missing timeout}"
      shift 2
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

need_cmd kubectl
need_cmd jq
need_cmd python3

wait_until "WeightsAndBiases ${NAMESPACE}/${NAME} status.ready=true" check_wandb_ready

wandb_json="$(kubectl -n "${NAMESPACE}" get weightsandbiases.apps.wandb.com "${NAME}" -o json)"
inline_ca_count="$(echo "${wandb_json}" | jq -r '(.spec.global.customCACerts // []) | length')"
USER_CONFIGMAP="$(echo "${wandb_json}" | jq -r '.spec.global.caCertsConfigMap // ""')"
mysql_ca_enabled="$(echo "${wandb_json}" | jq -r '(((.spec.mysql.externalMysql.sslCa.name // "") | length) > 0 and ((.spec.mysql.externalMysql.sslCa.key // "") | length) > 0)')"
redis_ca_enabled="$(echo "${wandb_json}" | jq -r '(((.spec.redis.externalRedis.sslCa.name // "") | length) > 0 and ((.spec.redis.externalRedis.sslCa.key // "") | length) > 0)')"

if [[ "${inline_ca_count}" == "0" && -z "${USER_CONFIGMAP}" ]]; then
  fail "WeightsAndBiases ${NAMESPACE}/${NAME} does not configure global custom CA material"
fi

if [[ "${inline_ca_count}" != "0" ]]; then
  wait_until "inline custom CA ConfigMap" check_inline_configmap
fi

if [[ -n "${USER_CONFIGMAP}" ]]; then
  wait_until "user custom CA ConfigMap ${USER_CONFIGMAP}" check_user_configmap
fi

if [[ "${mysql_ca_enabled}" == "true" ]]; then
  mysql_url="$(secret_value wandb-mysql-connection url)"
  assert_url_param "${mysql_url}" "tls" "custom"
  assert_url_param "${mysql_url}" "ssl-ca" "/etc/ssl/certs/mysql_ca.pem"
  log "MySQL connection URL includes expected CA parameters"
fi

if [[ "${redis_ca_enabled}" == "true" ]]; then
  redis_url="$(secret_value wandb-redis-connection url)"
  assert_url_param "${redis_url}" "tls" "true"
  assert_url_param "${redis_url}" "caCertPath" "/etc/ssl/certs/redis_ca.pem"
  log "Redis connection URL includes expected CA parameters"
fi

select_workload_template
workload_ref="${WORKLOAD_KIND} ${WORKLOAD_NAME}"

for env_name in SSL_CERT_FILE SSL_CERT_DIR REQUESTS_CA_BUNDLE; do
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_env "${env_name}" || fail "missing env ${env_name} on ${workload_ref}"
done
if [[ "${mysql_ca_enabled}" == "true" ]]; then
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_env MYSQL_CA_CERT_PATH || fail "missing env MYSQL_CA_CERT_PATH on ${workload_ref}"
fi

echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_volume wandb-ca-certs-root || fail "missing volume wandb-ca-certs-root on ${workload_ref}"
echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_mount wandb-ca-certs-root /usr/local/share/ca-certificates/ ||
  fail "missing root CA mount"

if [[ "${inline_ca_count}" != "0" ]]; then
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_volume wandb-ca-certs || fail "missing volume wandb-ca-certs on ${workload_ref}"
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_mount wandb-ca-certs /usr/local/share/ca-certificates/inline ||
    fail "missing inline CA mount"
fi
if [[ -n "${USER_CONFIGMAP}" ]]; then
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_volume wandb-ca-certs-user || fail "missing volume wandb-ca-certs-user on ${workload_ref}"
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_mount wandb-ca-certs-user /usr/local/share/ca-certificates/configmap ||
    fail "missing user CA ConfigMap mount"
fi
if [[ "${mysql_ca_enabled}" == "true" ]]; then
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_volume mysql-ca || fail "missing volume mysql-ca on ${workload_ref}"
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_mount mysql-ca /etc/ssl/certs/mysql_ca.pem ||
    fail "missing MySQL CA mount"
fi
if [[ "${redis_ca_enabled}" == "true" ]]; then
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_volume redis-ca || fail "missing volume redis-ca on ${workload_ref}"
  echo "${WORKLOAD_TEMPLATE_JSON}" | json_has_mount redis-ca /etc/ssl/certs/redis_ca.pem ||
    fail "missing Redis CA mount"
fi
echo "${WORKLOAD_TEMPLATE_JSON}" |
  jq -e '.metadata.annotations["weightsandbiases.apps.wandb.com/ca-certs-checksum"] | length > 0' >/dev/null ||
  fail "missing CA checksum annotation"
log "${workload_ref} pod template contains expected CA env, mounts, volumes, and checksum"

workload_pod="$(ready_workload_pod_name)"
if [[ -n "${workload_pod}" ]]; then
  pod_checks=("test -d /usr/local/share/ca-certificates/")
  if [[ "${inline_ca_count}" != "0" ]]; then
    pod_checks+=("test -d /usr/local/share/ca-certificates/inline")
  fi
  if [[ -n "${USER_CONFIGMAP}" ]]; then
    pod_checks+=("test -d /usr/local/share/ca-certificates/configmap")
  fi
  if [[ "${mysql_ca_enabled}" == "true" ]]; then
    pod_checks+=("test -s /etc/ssl/certs/mysql_ca.pem")
  fi
  if [[ "${redis_ca_enabled}" == "true" ]]; then
    pod_checks+=("test -s /etc/ssl/certs/redis_ca.pem")
  fi
  pod_check_cmd="$(printf ' && %s' "${pod_checks[@]}")"
  kubectl -n "${NAMESPACE}" exec "${workload_pod}" -- sh -c "${pod_check_cmd# && }" ||
    fail "live ${workload_ref} pod does not have expected CA files"
  log "Live ${workload_ref} pod has expected CA files"
else
  log "No ready pod found for ${workload_ref}; verified the workload pod template and skipped live filesystem checks"
fi

workload_logs="$(kubectl -n "${NAMESPACE}" logs -l "${WORKLOAD_POD_SELECTOR}" --all-containers --tail=500 --prefix=true 2>/dev/null || true)"
if echo "${workload_logs}" | grep -Eiq 'x509:|certificate signed by unknown authority|unknown authority|tls: failed to verify'; then
  echo "${workload_logs}" >&2
  fail "recent ${workload_ref} logs contain TLS trust failures"
fi

log "Custom CA e2e verification passed"
