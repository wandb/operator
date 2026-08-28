#!/usr/bin/env bash
set -euo pipefail

# Downloads the self-contained westest e2e binary from the wandb/westest GitHub
# releases and installs it to <dest>. Mirrors what wandb/westest/actions/run does
# (same asset naming + checksum verification), so the local binary matches CI.
#
# wandb/westest is private, so this relies on the GitHub CLI (`gh`) being
# installed and authenticated (run `gh auth login`, or set GH_TOKEN).

SCRIPT_NAME="$(basename "$0")"

WESTEST_REPO="${WESTEST_REPO:-wandb/westest}"
WESTEST_VERSION="${WESTEST_VERSION:-latest}"

log() {
  printf '[%s] %s\n' "${SCRIPT_NAME}" "$*"
}

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} <dest>

Downloads the westest binary from ${WESTEST_REPO} releases and installs it to
<dest> (default: ./bin/westest).

Environment:
  WESTEST_VERSION   Release tag to download, or "latest" (default: latest,
                    resolved including prereleases like the CI action).
  WESTEST_REPO      Source repo (default: wandb/westest).
  WESTEST_OS        Override OS       (default: from uname -s: linux|darwin).
  WESTEST_ARCH      Override arch     (default: from uname -m: amd64|arm64).
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

dest="${1:-./bin/westest}"

command -v gh >/dev/null 2>&1 || {
  log "error: the GitHub CLI (gh) is required and must be authenticated."
  log "       install it (https://cli.github.com/) and run 'gh auth login', or set GH_TOKEN."
  exit 1
}
command -v tar >/dev/null 2>&1 || { log "error: tar is required"; exit 1; }

os="${WESTEST_OS:-}"
if [[ -z "${os}" ]]; then
  case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) log "error: unsupported OS $(uname -s) (westest supports linux and darwin)"; exit 1 ;;
  esac
fi

arch="${WESTEST_ARCH:-}"
if [[ -z "${arch}" ]]; then
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) log "error: unsupported arch $(uname -m) (westest supports amd64 and arm64)"; exit 1 ;;
  esac
fi

tag="${WESTEST_VERSION}"
if [[ "${tag}" == "latest" ]]; then
  # gh's implicit "latest" excludes prereleases; resolve the newest published
  # release (prereleases included) explicitly, matching the CI action.
  tag="$(gh release list --repo "${WESTEST_REPO}" --limit 1 --json tagName --jq '.[0].tagName')"
  [[ -n "${tag}" ]] || { log "error: no releases found in ${WESTEST_REPO}"; exit 1; }
fi

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

log "downloading westest ${tag} (${os}/${arch}) from ${WESTEST_REPO}"
gh release download "${tag}" --repo "${WESTEST_REPO}" \
  --pattern "westest_*_${os}_${arch}.tar.gz" --pattern checksums.txt --dir "${tmp}"

archive="$(ls "${tmp}"/westest_*_"${os}"_"${arch}".tar.gz)"

if [[ -f "${tmp}/checksums.txt" ]]; then
  line="$(grep " $(basename "${archive}")\$" "${tmp}/checksums.txt" || true)"
  [[ -n "${line}" ]] || { log "error: no checksum entry for $(basename "${archive}")"; exit 1; }
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "${tmp}" && printf '%s\n' "${line}" | sha256sum -c -)
  else
    (cd "${tmp}" && printf '%s\n' "${line}" | shasum -a 256 -c -)
  fi
else
  log "warning: checksums.txt not published; skipping integrity check"
fi

tar -xzf "${archive}" -C "${tmp}"
mkdir -p "$(dirname "${dest}")"
install -m 0755 "${tmp}/westest" "${dest}"

log "installed westest ${tag} to ${dest}"
"${dest}" version || true
