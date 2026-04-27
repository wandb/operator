# OpenShift Local Development with CRC

This overlay configures the operator deployment for OpenShift's `restricted-v2` SCC. It is used automatically by Tilt when targeting a CRC (Red Hat OpenShift Local) cluster.

## Prerequisites

- [CRC](https://console.redhat.com/openshift/create/local) (free Red Hat account required)
- [oc CLI](https://formulae.brew.sh/formula/openshift-cli): `brew install openshift-cli`
- [Tilt](https://docs.tilt.dev/install.html): `brew install tilt-dev/tap/tilt`
- Docker (via OrbStack or Docker Desktop)
- At least 16GB RAM allocated to CRC (48GB+ host recommended)

## Setup

### 1. Install CRC

```bash
crc setup
```

This downloads a ~4GB VM image on first run and prompts for a pull secret (available from the CRC download page). The setup script handles configuration and starting CRC.

### 2. Run the setup script

```bash
./hack/scripts/setup_crc.sh
```

This script:
- Configures CRC memory (16GB) and disk size (80GB)
- Starts CRC if not already running
- Logs in as `kubeadmin`
- Exposes the internal image registry via a route
- Configures Docker to trust the CRC registry (handles OrbStack automatically)
- Creates `operator-system` and `wandb-operator` namespaces

### 3. Configure tilt-settings.star

```python
SETTINGS = {
    "allowedContexts": ["crc-admin"],
}
```

### 4. Start Tilt

```bash
kubectl config use-context crc-admin
tilt up
```

## How it works

When Tilt detects a CRC context, it automatically:

- Enables `openshiftSCC` mode
- Uses `config/openshift-dev` as the kustomize overlay
- Builds the operator image with a non-root Dockerfile (`USER 1001`, group-writable dirs)
- Pushes images to CRC's internal registry (`default-route-openshift-image-registry.apps-crc.testing`)

### What this overlay does

`openshift-security.yaml` patches the controller-manager deployment with a security context that satisfies the `restricted-v2` SCC:

- `runAsNonRoot: true`
- `seccompProfile: RuntimeDefault`
- `allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true`
- `capabilities.drop: [ALL]`

OpenShift assigns the UID from the namespace's `openshift.io/sa.scc.uid-range` annotation at admission time.

## Troubleshooting

### Docker push fails with TLS certificate error

The setup script handles this automatically for OrbStack. If using Docker Desktop, add the CRC registry to **Settings > Docker Engine > insecure-registries**:

```json
{
  "insecure-registries": ["default-route-openshift-image-registry.apps-crc.testing"]
}
```

### Pods stuck in Pending (insufficient memory)

The setup script configures 16GB RAM automatically. If CRC was already running when the script ran, the config won't take effect until restart:

```bash
crc stop
./hack/scripts/setup_crc.sh
```

### Docker login expired

CRC tokens expire. Re-run the setup script or manually:

```bash
oc login -u kubeadmin -p $(crc console --credentials | grep kubeadmin | sed 's/.*-p \([^ ]*\) .*/\1/') https://api.crc.testing:6443
oc whoami -t | docker login -u kubeadmin --password-stdin default-route-openshift-image-registry.apps-crc.testing
```
