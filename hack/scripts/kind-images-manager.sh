#!/usr/bin/env bash
set -euo pipefail

IMAGES_FILE=".k8s-images"

cmd_scrape() {
    kubectl get pods -A \
        -o jsonpath='{range .items[*]}{range .spec.containers[*]}{.image}{"\n"}{end}{range .spec.initContainers[*]}{.image}{"\n"}{end}{end}' \
        | sort -u \
        > "$IMAGES_FILE"
    echo "Wrote $(wc -l < "$IMAGES_FILE") images to $IMAGES_FILE"
}

cmd_load() {
    if [[ ! -f "$IMAGES_FILE" ]]; then
        echo "error: $IMAGES_FILE not found — run 'scrape' first" >&2
        exit 1
    fi

    local context
    context=$(kubectl config current-context)

    if [[ "$context" != kind-* ]]; then
        echo "error: current kube context '$context' does not start with 'kind-'" >&2
        exit 1
    fi

    local cluster="${context#kind-}"

    local failed=()

    echo "Loading images into kind cluster '$cluster'..."
    while IFS= read -r image; do
        [[ -z "$image" ]] && continue
        echo "  $image"
        if ! kind load docker-image "$image" --name "$cluster"; then
            echo "  warning: failed to load $image" >&2
            failed+=("$image")
        fi
    done < "$IMAGES_FILE"

    if [[ ${#failed[@]} -gt 0 ]]; then
        echo "" >&2
        echo "Failed to load ${#failed[@]} image(s):" >&2
        for image in "${failed[@]}"; do
            echo "  $image" >&2
        done
        exit 1
    fi

    echo "Done."
}

cmd_pull() {
    if [[ ! -f "$IMAGES_FILE" ]]; then
        echo "error: $IMAGES_FILE not found — run 'scrape' first" >&2
        exit 1
    fi

    local failed=()

    echo "Pulling images..."
    while IFS= read -r image; do
        [[ -z "$image" ]] && continue
        echo "  $image"
        if ! docker pull "$image"; then
            echo "  warning: failed to pull $image" >&2
            failed+=("$image")
        fi
    done < "$IMAGES_FILE"

    if [[ ${#failed[@]} -gt 0 ]]; then
        echo "" >&2
        echo "Failed to pull ${#failed[@]} image(s):" >&2
        for image in "${failed[@]}"; do
            echo "  $image" >&2
        done
        exit 1
    fi

    echo "Done."
}

case "${1:-}" in
    scrape)  cmd_scrape ;;
    pull)    cmd_pull ;;
    load)    cmd_load ;;
    *)
        echo "Usage: $(basename "$0") <scrape|pull|load>" >&2
        echo ""
        echo "Keep docker images cached on the host and loads them into Kind."
        echo "It uses the k8s current context, if named 'kind-*', as the Kind cluster."
        echo "This is helpful if you run into rate-limiting problem with a repository."
        echo ""
        echo "scrape: copies image names from kind cluster into .k8s-images file"
        echo "pull:   pulls images named .k8s-images file into host's docker"
        echo "load:   loads images named .k8s-images from host's docker into Kind cluster"
        exit 1
        ;;
esac
