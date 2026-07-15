#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<EOF
Usage: $(basename "$0") [tilt-dev-clean args]

Runs the safe dev cleanup flow, then calls \`tilt down\`.

Examples:
  $(basename "$0")
  $(basename "$0") --namespace default --name wandb-dev-v2
  $(basename "$0") --dry-run
EOF
  exit 0
fi

"${SCRIPT_DIR}/tilt-dev-clean.sh" "$@"

if [[ " $* " == *" --dry-run "* ]]; then
  exit 0
fi

tilt down
