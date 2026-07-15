#!/usr/bin/env bash
set -euo pipefail

if ! command -v kubectl >/dev/null 2>&1; then
    echo "error: kubectl is required but not installed" >&2
    exit 1
fi

current_context=$(kubectl config current-context 2>/dev/null || true)
if [[ -z "$current_context" ]]; then
    echo "error: unable to determine the current kubectl context" >&2
    exit 1
fi

mapfile -t crds < <(kubectl get crd -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')

if [[ ${#crds[@]} -eq 0 ]]; then
    echo "No CRDs found in context: $current_context"
    exit 0
fi

echo "Current context: $current_context"
echo ""

found_any=0
name_column_width=60

for crd in "${crds[@]}"; do
    [[ -z "$crd" ]] && continue

    group=$(kubectl get crd "$crd" -o jsonpath='{.spec.group}')
    plural=$(kubectl get crd "$crd" -o jsonpath='{.spec.names.plural}')
    kind=$(kubectl get crd "$crd" -o jsonpath='{.spec.names.kind}')
    scope=$(kubectl get crd "$crd" -o jsonpath='{.spec.scope}')
    version=$(kubectl get crd "$crd" -o jsonpath='{range .spec.versions[?(@.storage==true)]}{.name}{end}')

    if [[ -z "$version" ]]; then
        version=$(kubectl get crd "$crd" -o jsonpath='{.spec.version}')
    fi

    resource="${plural}.${group}"

    if [[ "$scope" == "Namespaced" ]]; then
        raw_output=$(
            kubectl get "$resource" -A \
                -o custom-columns='NAME:.metadata.name,NAMESPACE:.metadata.namespace' \
                --no-headers 2>/dev/null || true
        )
    else
        raw_output=$(
            kubectl get "$resource" \
                -o custom-columns='NAME:.metadata.name' \
                --no-headers 2>/dev/null || true
        )
    fi

    [[ -z "$raw_output" ]] && continue

    found_any=1
    printf '\033[1m%s\033[0m\n' "$crd ($kind, ${group}/${version})"
    printf '%-*s %s\n' "$name_column_width" "NAME" "SCOPE"
    printf '%-*s %s\n' "$name_column_width" "----" "-----"

    while IFS= read -r line; do
        [[ -z "$line" ]] && continue

        if [[ "$scope" == "Namespaced" ]]; then
            name=${line%%[[:space:]]*}
            namespace=${line##*[[:space:]]}
            printf '%-*s %s\n' "$name_column_width" "$name" "namespaced:$namespace"
        else
            printf '%-*s %s\n' "$name_column_width" "$line" "clustered"
        fi
    done <<< "$raw_output"

    echo ""
done

if [[ "$found_any" -eq 0 ]]; then
    echo "No custom resources found for any CRD in context: $current_context"
fi
