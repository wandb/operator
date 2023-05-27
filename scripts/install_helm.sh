#!/bin/bash

if ! command -v helm &>/dev/null; then
    echo "Helm not installed. Installing..."
    curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
    chmod 700 get_helm.sh
    ./get_helm.sh
    rm -f get_helm.sh
fi
