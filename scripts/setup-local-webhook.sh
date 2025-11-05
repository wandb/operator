#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CERT_DIR="${PROJECT_ROOT}/.local-webhook-certs"
CONFIG_DIR="${PROJECT_ROOT}/config/local-dev"

WEBHOOK_PORT="${WEBHOOK_PORT:-9443}"

if [[ -f "${CERT_DIR}/tls.crt" ]] && [[ -f "${CERT_DIR}/tls.key" ]] && [[ -f "${CERT_DIR}/ca.crt" ]] && [[ -f "${CONFIG_DIR}/webhook_local_patch.yaml" ]]; then
    echo "✅ Local webhook setup already complete"
    echo ""
    echo "Existing certificates found in: ${CERT_DIR}"
    echo "Existing webhook config found in: ${CONFIG_DIR}/webhook_local_patch.yaml"
    echo ""
    echo "To regenerate certificates, delete the directory and re-run:"
    echo "  rm -rf ${CERT_DIR} ${CONFIG_DIR}/webhook_local_patch.yaml"
    echo "  make setup-local-webhook"
    echo ""
    echo "To run the controller with webhook support:"
    echo "  make run-local-webhook"
    exit 0
fi

get_kind_cluster_name() {
    local CLUSTER_NAME="kind"

    if [[ -f "${PROJECT_ROOT}/tilt-settings.json" ]] && command -v jq >/dev/null 2>&1; then
        local KIND_CLUSTER_NAME=$(jq -r '.kindClusterName // empty' "${PROJECT_ROOT}/tilt-settings.json")
        if [[ -n "$KIND_CLUSTER_NAME" ]]; then
            CLUSTER_NAME="$KIND_CLUSTER_NAME"
        fi
    fi

    echo "$CLUSTER_NAME"
}

detect_webhook_host() {
    if [[ -n "${WEBHOOK_HOST}" ]]; then
        echo "Using manually set WEBHOOK_HOST: ${WEBHOOK_HOST}"
        return
    fi

    echo "Auto-detecting webhook host..."

    local OS="$(uname -s)"

    if [[ "$OS" == "Darwin" ]]; then
        if pgrep -f "OrbStack" >/dev/null 2>&1 || [[ -d "/Applications/OrbStack.app" ]]; then
            echo "  Detected OrbStack on macOS"
            WEBHOOK_HOST="host.docker.internal"
            echo "  Using host.docker.internal (OrbStack supports this)"
        else
            echo "  Detected Docker Desktop on macOS"
            WEBHOOK_HOST="host.docker.internal"
            echo "  Using host.docker.internal"
        fi
    elif [[ "$OS" == "Linux" ]]; then
        local GATEWAY_IP=$(docker network inspect bridge 2>/dev/null | grep -m1 Gateway | awk '{print $2}' | tr -d '",')
        if [[ -n "$GATEWAY_IP" ]]; then
            WEBHOOK_HOST="$GATEWAY_IP"
            echo "  Using Docker bridge gateway: ${WEBHOOK_HOST}"
        else
            echo "  Warning: Could not detect Docker gateway IP"
            WEBHOOK_HOST="172.17.0.1"
            echo "  Using default: ${WEBHOOK_HOST}"
        fi
    else
        echo "  Unsupported OS: $OS"
        WEBHOOK_HOST="host.docker.internal"
        echo "  Using default: ${WEBHOOK_HOST}"
    fi
}

detect_webhook_host

echo ""
echo "Setting up local webhook development environment..."
echo "Webhook will be accessible at: https://${WEBHOOK_HOST}:${WEBHOOK_PORT}"

mkdir -p "${CERT_DIR}"

echo "Generating self-signed certificates..."

cat > "${CERT_DIR}/csr.conf" <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
prompt = no

[req_distinguished_name]
CN = ${WEBHOOK_HOST}

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${WEBHOOK_HOST}
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

openssl genrsa -out "${CERT_DIR}/ca.key" 2048

openssl req -x509 -new -nodes -key "${CERT_DIR}/ca.key" \
    -subj "/CN=Local Webhook CA" \
    -days 3650 \
    -out "${CERT_DIR}/ca.crt"

openssl genrsa -out "${CERT_DIR}/tls.key" 2048

openssl req -new -key "${CERT_DIR}/tls.key" \
    -out "${CERT_DIR}/tls.csr" \
    -config "${CERT_DIR}/csr.conf"

openssl x509 -req -in "${CERT_DIR}/tls.csr" \
    -CA "${CERT_DIR}/ca.crt" \
    -CAkey "${CERT_DIR}/ca.key" \
    -CAcreateserial \
    -out "${CERT_DIR}/tls.crt" \
    -days 365 \
    -extensions v3_req \
    -extfile "${CERT_DIR}/csr.conf"

echo "Certificates generated in: ${CERT_DIR}"

CA_BUNDLE=$(cat "${CERT_DIR}/ca.crt" | base64 | tr -d '\n')

echo "Updating webhook configuration with CA bundle..."

cat > "${CONFIG_DIR}/webhook_local_patch.yaml" <<EOF
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: validating-webhook-configuration
webhooks:
- name: vweightsandbiases.wandb.com
  clientConfig:
    \$patch: replace
    url: https://${WEBHOOK_HOST}:${WEBHOOK_PORT}/validate-apps-wandb-com-v2-weightsandbiases
    caBundle: ${CA_BUNDLE}
EOF

echo ""
echo "✅ Setup complete!"
echo ""
echo "To run the controller locally with webhook support:"
echo ""
echo "  1. Apply the CRDs and webhook configuration to your cluster:"
echo "     kubectl apply -k config/local-dev"
echo ""
echo "  2. Run the controller with webhook certificates:"
echo "     go run ./cmd/controller/main.go \\"
echo "       --webhook-cert-path=${CERT_DIR} \\"
echo "       --webhook-cert-name=tls.crt \\"
echo "       --webhook-cert-key=tls.key \\"
echo "       --v2-webhook=true"
echo ""
echo "     Or use the Makefile target:"
echo "     make run-local-webhook"
echo ""
echo "Certificate files:"
echo "  CA:   ${CERT_DIR}/ca.crt"
echo "  Cert: ${CERT_DIR}/tls.crt"
echo "  Key:  ${CERT_DIR}/tls.key"
echo ""
echo "Webhook configured for: ${WEBHOOK_HOST}:${WEBHOOK_PORT}"
echo ""
echo "Note: If auto-detection failed, manually set WEBHOOK_HOST and re-run:"
echo "  export WEBHOOK_HOST=<your-host-ip>"
echo "  ./scripts/setup-local-webhook.sh"