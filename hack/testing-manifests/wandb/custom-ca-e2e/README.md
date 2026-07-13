# Custom CA Tilt E2E

This runbook exercises operator v2 custom CA parity with composable Tilt
settings. Tilt generates test CA material at render time, writes it through the
normal W&B CR and user ConfigMap inputs, and installs TLS-enabled external
MySQL/Redis only when those external services are selected.

## Full Parity Run

Use all external infra plus custom CA:

```python
SETTINGS = {
    "includeCR": True,
    "wandbNamespace": "wandb-ca-e2e",
    "useExternalMysql": True,
    "useExternalRedis": True,
    "useExternalObjectStore": True,
    "useCustomCA": True,
}
```

Then run:

```bash
tilt up
```

Wait for `Test-Infra`, `wandb-operator`, `Wandb`, and `Wandb-Endpoint`, then
run the verifier from a terminal:

```bash
./hack/scripts/verify-custom-ca-e2e.sh --namespace wandb-ca-e2e --name wandb
```

## Composable Variants

The settings are independent:

- `useExternalMysql=True` installs local MySQL from `test-infra` and points the
  W&B CR at the generated connection Secret.
- `useExternalRedis=True` does the same for Redis.
- `useExternalObjectStore=True` does the same for SeaweedFS/S3.
- `useCustomCA=True` generates global custom CA material. If external MySQL or
  Redis are also enabled, their test-infra services use TLS and the CR includes
  `sslCa` selectors for the generated CA Secrets.

## Clean Up

```bash
./hack/scripts/tilt-down-dev-clean.sh --namespace wandb-ca-e2e --name wandb
```
