# Local Webhook Development

This directory contains configuration for running the operator with webhooks locally (outside the cluster) for development purposes.

## Overview

The standard webhook configuration expects the webhook server to run as a Service inside the cluster. For local development, this configuration patches the ValidatingWebhookConfiguration to use a URL pointing to your local machine instead.

## Prerequisites

- A Kubernetes cluster (kind, minikube, or similar)
- `kubectl` configured to access your cluster
- `openssl` for generating certificates
- The operator CRDs installed in your cluster

## Quick Start

### Option 1: Using Tilt (Recommended)

If you're using Tilt for development:

1. **Enable local webhook mode** in your `tilt-settings.json`:
   ```json
   {
     "localWebhookDev": true
   }
   ```

2. **Start Tilt**:
   ```bash
   tilt up
   ```

3. **In the Tilt UI, trigger the manual resource**: `Setup-Local-Webhook-Certs`

4. **Run the controller** (outside Tilt, in your terminal or IDE):
   ```bash
   make run-local-webhook
   ```

Tilt will:
- Apply CRDs, RBAC, and webhook configuration to your cluster
- Skip deploying the controller to the cluster
- Provide a manual button to setup webhook certificates
- Keep watching for code changes

### Option 2: Manual Setup

If you're not using Tilt:

1. **Generate certificates and configure webhook**:
   ```bash
   ./scripts/setup-local-webhook.sh
   ```

2. **Apply configuration to cluster**:
   ```bash
   kubectl apply -k config/local-dev
   ```

3. **Run the controller locally**:
   ```bash
   go run ./cmd/controller/main.go \
     --webhook-cert-path=.local-webhook-certs \
     --webhook-cert-name=tls.crt \
     --webhook-cert-key=tls.key \
     --v2-webhook=true
   ```

   Or use the Makefile target:
   ```bash
   make run-local-webhook
   ```

The webhook server will start on `https://host.docker.internal:9443` (or your configured host).

## How It Works

The `setup-local-webhook.sh` script automatically detects your environment:

1. **Checks for kind cluster**: Uses the same cluster name resolution as `setup_kind.sh` (reads from `tilt-settings.json` if available)
2. **Detects platform**: macOS, Linux, Docker Desktop, or OrbStack
3. **Configures appropriate host**: Uses the Docker bridge gateway for kind, or `host.docker.internal` for Docker Desktop/OrbStack
4. **Generates certificates**: Creates self-signed TLS certificates with the correct hostname
5. **Updates webhook config**: Patches the ValidatingWebhookConfiguration with the URL and CA bundle

### Most Common Usage

For most setups, just run:

```bash
./scripts/setup-local-webhook.sh
```

The script will detect your environment and configure everything automatically.

## Platform-Specific Configuration

### macOS (Docker Desktop or OrbStack)

Auto-detection works out of the box:

```bash
./scripts/setup-local-webhook.sh
```

The script detects:
- **OrbStack**: Uses `host.docker.internal` (natively supported)
- **Docker Desktop**: Uses `host.docker.internal`
- **kind cluster**: Automatically uses the Docker bridge gateway IP

### Linux

Auto-detection works for most setups:

```bash
./scripts/setup-local-webhook.sh
```

The script automatically:
- Detects if you're using kind and finds the Docker bridge gateway IP
- Falls back to detecting the default Docker bridge IP
- Uses sensible defaults (172.17.0.1) if auto-detection fails

**Manual override** (if auto-detection doesn't work):

```bash
# Find your Docker bridge IP
docker network inspect kind | grep Gateway
# Usually 172.18.0.1 or 172.17.0.1

# Set the webhook host before running setup
export WEBHOOK_HOST=172.18.0.1
./scripts/setup-local-webhook.sh
```

### Minikube

For minikube, use the host IP that the minikube VM can reach:

```bash
# Get the host IP (usually 192.168.x.x)
export WEBHOOK_HOST=$(minikube ssh -- ip route | grep default | awk '{print $3}')
./scripts/setup-local-webhook.sh
```

## Customizing the Port

By default, the webhook server runs on port 9443. To use a different port:

```bash
export WEBHOOK_PORT=8443
./scripts/setup-local-webhook.sh
```

Then run the controller with the matching port configuration.

## Disabling the Webhook

If you want to develop without the webhook:

```bash
go run ./cmd/controller/main.go --v2-webhook=false
```

Or set the environment variable:

```bash
export V2_WEBHOOK=false
go run ./cmd/controller/main.go
```

## Troubleshooting

### "Connection refused" errors

The Kubernetes API server cannot reach your local webhook. Check:
- The webhook host is correct for your platform
- Your firewall allows connections on port 9443
- The controller is running and listening on the correct port

### "x509: certificate is valid for X, not Y"

The certificate doesn't match the hostname. Regenerate certificates with the correct host:

```bash
export WEBHOOK_HOST=<your-correct-host>
./scripts/setup-local-webhook.sh
kubectl apply -k config/local-dev
```

### "context deadline exceeded"

The webhook took too long to respond. This usually means:
- The controller crashed or isn't running
- The webhook validation logic has an infinite loop or deadlock
- Network connectivity issues

Check the controller logs for errors.

## Files Generated

- `.local-webhook-certs/ca.crt` - CA certificate
- `.local-webhook-certs/ca.key` - CA private key
- `.local-webhook-certs/tls.crt` - Server certificate
- `.local-webhook-certs/tls.key` - Server private key
- `config/local-dev/webhook_local_patch.yaml` - Updated with CA bundle

## Cleaning Up

To remove the local development configuration:

```bash
kubectl delete -k config/local-dev
rm -rf .local-webhook-certs
```

## Differences from Production

This local development setup differs from production in these ways:

1. **URL instead of Service**: Uses a URL pointing to localhost instead of an in-cluster Service
2. **Self-signed certificates**: Uses locally generated certificates instead of cert-manager
3. **No cert-manager**: Certificate management is manual
4. **Direct access**: The Kubernetes API server connects directly to your machine

For production deployment, use the standard configuration with cert-manager or a proper certificate authority.