#!/bin/bash

set -euo pipefail

command -v crc >/dev/null 2>&1 || { echo "Error: crc is required but not installed." >&2; echo "Download from: https://console.redhat.com/openshift/create/local" >&2; exit 1; }
command -v oc >/dev/null 2>&1 || { echo "Error: oc is required but not installed." >&2; echo "Install with: brew install openshift-cli" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Error: docker is required but not installed." >&2; exit 1; }

echo "Configuring CRC resources..."
crc config set memory 16384
crc config set disk-size 80

CRC_STATUS=$(crc status --output json 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('crcStatus','Unknown'))" 2>/dev/null || echo "Unknown")
if [[ "$CRC_STATUS" != "Running" ]]; then
    echo "Starting CRC..."
    crc start
fi

eval "$(crc oc-env)"

KUBEADMIN_PASSWORD=$(crc console --credentials 2>/dev/null | grep kubeadmin | sed "s/.*-p \([^ ]*\) .*/\1/" || true)
if [[ -z "$KUBEADMIN_PASSWORD" ]]; then
    echo "Error: could not retrieve kubeadmin password from CRC."
    exit 1
fi

echo "Logging in as kubeadmin..."
oc login -u kubeadmin -p "$KUBEADMIN_PASSWORD" https://api.crc.testing:6443

echo "Exposing the internal image registry..."
oc patch configs.imageregistry.operator.openshift.io/cluster \
    --patch '{"spec":{"defaultRoute":true}}' --type=merge

REGISTRY="default-route-openshift-image-registry.apps-crc.testing"
echo "Waiting for registry route..."
for i in $(seq 1 30); do
    if curl -sk "https://$REGISTRY/healthz" >/dev/null 2>&1; then
        break
    fi
    if [[ $i -eq 30 ]]; then
        echo "Error: registry route not ready after 30s."
        exit 1
    fi
    sleep 1
done

echo "Logging Docker into the CRC registry..."
docker login -u kubeadmin -p "$(oc whoami -t)" "$REGISTRY"

ORBSTACK_DOCKER_CONFIG="$HOME/.orbstack/config/docker.json"
if docker info 2>&1 | grep -q "Operating System: OrbStack"; then
    if ! grep -q "$REGISTRY" "$ORBSTACK_DOCKER_CONFIG" 2>/dev/null; then
        echo "Configuring OrbStack Docker to trust the CRC registry..."
        mkdir -p "$(dirname "$ORBSTACK_DOCKER_CONFIG")"
        python3 -c "
import json, pathlib, sys
p = pathlib.Path('$ORBSTACK_DOCKER_CONFIG')
cfg = json.loads(p.read_text()) if p.exists() else {}
regs = set(cfg.get('insecure-registries', []))
regs.add('$REGISTRY')
cfg['insecure-registries'] = sorted(regs)
p.write_text(json.dumps(cfg, indent=2) + '\n')
"
        echo "Restarting OrbStack Docker daemon..."
        orb restart docker 2>/dev/null || true
        sleep 3
        docker login -u kubeadmin -p "$(oc whoami -t)" "$REGISTRY"
    fi
fi

echo "Creating operator-system namespace..."
oc new-project operator-system 2>/dev/null || oc project operator-system 2>/dev/null || true

echo "Creating wandb-operator namespace..."
oc new-project wandb-operator 2>/dev/null || oc project wandb-operator 2>/dev/null || true

echo ""
echo "Done. CRC is ready for Tilt."
echo ""
echo "Configure tilt-settings.star:"
echo '  SETTINGS = {'
echo '      "allowedContexts": ["crc-admin"],'
echo '  }'
echo ""
echo "Then run: tilt up"
