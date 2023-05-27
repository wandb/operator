#!/bin/bash

# Set the Helm repo URL
HELM_REPO_URL="https://wandb.github.io/helm-charts"

# Add the Helm repo to your Helm client
helm repo add downloadrepo "$HELM_REPO_URL"
helm repo update

# Get the list of chart names and versions
chart_list=$(helm search repo downloadrepo --output json | jq -r '.[].name + "===" + .[].version' || echo "")

# Download each chart in the list
if [ -n "$chart_list" ]; then
    mkdir -p charts
    IFS=$'\n'
    for chart_info in $chart_list; do
        full=$(echo "$chart_info" | cut -d'=' -f1)
        name=$(echo "$full" | cut -d'/' -f2)
        version=$(echo "$chart_info" | cut -d'=' -f4)
        echo "Downloading $full $version..."
        helm pull "$full" --version "$version"
    done
    unset IFS
    echo "All charts downloaded to the downloaded_charts directory."
else
    echo "No charts found in the repo."
fi