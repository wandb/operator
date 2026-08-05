#!/usr/bin/env bash
set -euo pipefail

REPOSITORY="us-docker.pkg.dev/wandb-production/public/wandb/server-manifest"
SCRIPT_NAME="$(basename "$0")"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEST_ROOT="${REPO_ROOT}/hack/testing-manifests/server-manifest"

log() {
  printf '[%s] %s\n' "${SCRIPT_NAME}" "$*"
}

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} <tag>

Downloads the published server manifest OCI artifact for <tag> from
${REPOSITORY} and unpacks its manifest yaml files into
${DEST_ROOT}/<tag>, replacing any existing manifest already checked in
for that version.
EOF
}

if ! command -v oras >/dev/null 2>&1; then
  echo "error: oras is required but not installed (https://oras.land/docs/installation)" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required but not installed" >&2
  exit 1
fi

if [[ $# -ne 1 || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 1
fi

TAG="$1"
IMAGE_REF="${REPOSITORY}:${TAG}"
DEST_DIR="${DEST_ROOT}/${TAG}"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

log "Fetching manifest for ${IMAGE_REF}"
MANIFEST_JSON="$(oras manifest fetch "${IMAGE_REF}")"

# Multi-platform tags resolve to an image index; follow the first entry, same
# as pkg/wandb/manifest.processManifest does for the operator itself.
IS_INDEX="$(jq -r 'if .manifests then "true" else "false" end' <<<"${MANIFEST_JSON}")"
if [[ "${IS_INDEX}" == "true" ]]; then
  CHILD_DIGEST="$(jq -r '.manifests[0].digest' <<<"${MANIFEST_JSON}")"
  log "Tag resolved to an image index; following ${CHILD_DIGEST}"
  MANIFEST_JSON="$(oras manifest fetch "${REPOSITORY}@${CHILD_DIGEST}")"
fi

LAYER_DIGESTS="$(jq -r '.layers[]?.digest' <<<"${MANIFEST_JSON}")"
if [[ -z "${LAYER_DIGESTS}" ]]; then
  echo "error: no layers found in manifest for ${IMAGE_REF}" >&2
  exit 1
fi

EXTRACT_DIR="${WORKDIR}/extracted"
mkdir -p "${EXTRACT_DIR}"

while IFS= read -r digest; do
  [[ -z "${digest}" ]] && continue
  log "Extracting layer ${digest}"
  oras blob fetch "${REPOSITORY}@${digest}" --output - | tar -xf - -C "${EXTRACT_DIR}"
done <<<"${LAYER_DIGESTS}"

YAML_FILES=()
while IFS= read -r f; do
  [[ -z "${f}" ]] && continue
  YAML_FILES+=("${f}")
done < <(find "${EXTRACT_DIR}" -type f -name '*.yaml' | sort)
if [[ ${#YAML_FILES[@]} -eq 0 ]]; then
  echo "error: no .yaml files found in manifest layers for ${IMAGE_REF}" >&2
  exit 1
fi

rm -rf "${DEST_DIR}"
mkdir -p "${DEST_DIR}"

for f in "${YAML_FILES[@]}"; do
  name="$(basename "${f}")"
  if [[ -e "${DEST_DIR}/${name}" ]]; then
    log "warning: duplicate manifest file name '${name}' found across layers; last one wins"
  fi
  cp "${f}" "${DEST_DIR}/${name}"
done

log "Wrote ${#YAML_FILES[@]} manifest file(s) to ${DEST_DIR}"
