# Development workflow

Run commands from the repository root with Go matching [go.mod](../../go.mod).
Use [setup](setup.md) for the local cluster and Tilt environment.

## What to run after a change

| Change | Required work |
| --- | --- |
| Controller/helper Go code | Tilt's `Operator-Build` watches `internal`, `pkg`, `api`, and `cmd`; inspect the rebuild and deployment |
| API types or kubebuilder markers | Run `make manifests generate sync-crd-embed`; ensure the chart installs the regenerated CRDs before testing admission/reconciliation |
| Mocked interface | Run `go generate ./...` and inspect the changed fakes |
| Helm templates or values | Run chart checks and inspect Tilt's Helm release update |
| Dependency API/CRDs | Follow the dependency's colocated vendoring notes and verify generated/embedded CRDs |
| Local W&B configuration | Edit `tilt-settings.star` or the `crFile` base; Tilt's scalar settings can override base fields |
| Kustomize configuration | Rebuild the affected Kustomize output; the default Tilt installation uses Helm, so it is not a Kustomize validation path |

## Code generation and CRD installation

```bash
make manifests generate sync-crd-embed
```

This updates the RBAC/webhook/CRD manifests, generated Go methods, and embedded
Operator CRDs used by the installer. Review the diff alongside the source change.
Do not hand-edit generated CRDs or deepcopy methods.

The current Tilt resources are `Operator-Codegen` and `Operator-Build`.
`Operator-Codegen` runs `make manifests generate` and has no API-file watch list;
a binary rebuild alone does not prove the installed schema has changed.
After regenerating, trigger the Operator Helm resource if needed:

```bash
tilt trigger Operator-Codegen
tilt trigger wandb-operator
```

The default Helm release uses the CRD installer hook. Check the hook Job and the
installed schema with `kubectl explain`; restart Tilt when its configuration or
dependencies need reevaluation. Do not apply raw CRD bases as a generic substitute
for the chart's webhook and certificate configuration.

## Validate and contribute

```bash
make lint
make test
```

Run the additional checks relevant to the change in [testing](testing.md), including
cluster scenarios for behavior envtest does not exercise. `make build` also runs
generation, formatting, vetting, and builds both manager and CRD installer.

Commit messages and PR titles follow Conventional Commits with an uppercase
subject, for example `docs: Separate user and developer guides`. The
[PR title workflow](../../.github/workflows/pr-title.yaml) enforces the title
format. Releases use the [tagged release procedure](releasing.md).
