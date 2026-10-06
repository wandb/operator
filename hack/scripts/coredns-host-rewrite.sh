#!/usr/bin/env bash

set -euo pipefail

if [[ $# -eq 0 || $(( $# % 2 )) -ne 0 ]]; then
  echo "Usage: $(basename "$0") <hostname> <target-service-fqdn> [<hostname> <target-service-fqdn> ...]" >&2
  exit 1
fi

marker_begin="# BEGIN wandb-tilt-host-rewrites"
marker_end="# END wandb-tilt-host-rewrites"
legacy_marker_begin="# BEGIN wandb-tilt-host-rewrite"
legacy_marker_end="# END wandb-tilt-host-rewrite"
rules=()

while [[ $# -gt 0 ]]; do
  hostname="$1"
  target="$2"
  shift 2

  if [[ ! "${hostname}" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]]; then
    echo "Invalid rewrite hostname: ${hostname}" >&2
    exit 1
  fi
  if [[ ! "${target}" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]]; then
    echo "Invalid rewrite target: ${target}" >&2
    exit 1
  fi

  rules+=("rewrite name exact ${hostname} ${target}")
done

rules_text=""
for rule in "${rules[@]}"; do
  if [[ -n "${rules_text}" ]]; then
    rules_text+="|"
  fi
  rules_text+="${rule}"
done

corefile="$(kubectl -n kube-system get configmap coredns -o jsonpath='{.data.Corefile}')"
updated_corefile="$(
  awk \
    -v marker_begin="${marker_begin}" \
    -v marker_end="${marker_end}" \
    -v legacy_marker_begin="${legacy_marker_begin}" \
    -v legacy_marker_end="${legacy_marker_end}" \
    -v rules="${rules_text}" '
    index($0, marker_begin) || index($0, legacy_marker_begin) { skipping = 1; next }
    index($0, marker_end) || index($0, legacy_marker_end) { skipping = 0; next }
    skipping { next }
    {
      print
      if (!inserted && $0 ~ /^[[:space:]]*\.:53[[:space:]]*\{[[:space:]]*$/) {
        match($0, /^[[:space:]]*/)
        indent = substr($0, RSTART, RLENGTH) "    "
        print indent marker_begin
        count = split(rules, rendered_rules, "|")
        for (i = 1; i <= count; i++) {
          if (rendered_rules[i] != "") {
            print indent rendered_rules[i]
          }
        }
        print indent marker_end
        inserted = 1
      }
    }
    END {
      if (!inserted) {
        print "Could not find the .:53 server block in the CoreDNS Corefile" > "/dev/stderr"
        exit 1
      }
    }
  ' <<<"${corefile}"
)"

if [[ "${corefile}" == "${updated_corefile}" ]]; then
  echo "CoreDNS host rewrites are already current"
  exit 0
fi

patch="$(jq -cn --arg corefile "${updated_corefile}" '{data: {Corefile: ($corefile + "\n")}}')"
kubectl -n kube-system patch configmap coredns --type=merge --patch "${patch}"
kubectl -n kube-system rollout restart deployment/coredns
kubectl -n kube-system rollout status deployment/coredns --timeout=120s

echo "CoreDNS host rewrites updated"
