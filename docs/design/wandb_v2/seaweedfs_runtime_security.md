# Managed SeaweedFS runtime security

The renderer uses the supported `podSecurityContext` and
`containerSecurityContext` fields on all four components: master, volume, filer,
and S3. The pinned seaweedfs-operator chart 0.1.35 (controller 1.0.32) and its CRD
support these fields. The vendored Go API exposes the same fields; no controller
upgrade or post-reconcile Pod patch is needed.

Containers use a read-only root, disable privilege escalation, drop all Linux
capabilities, and use RuntimeDefault seccomp. Existing `/tmp` emptyDirs and data
PVCs remain writable. These settings apply through upstream reconciliation to
the actual component Pods.

Non-root identity activation remains blocked. The default SeaweedFS 4.35 image
declares root, and upstream overrides its entrypoint with component commands.
The operator leaves `runAsNonRoot`, `runAsUser`, `runAsGroup` and `fsGroup` unset.
A published image declaring a numeric non-root user and supporting arbitrary-UID
volume access is required before non-root enforcement. Do not add fixed IDs to
work around that image contract. This change does not establish restricted Pod
Security or OpenShift runtime acceptance.

The vendored API is a subset of upstream. Regenerate its deep copies with:

```sh
./bin/controller-gen object:headerFile=hack/boilerplate.go.txt paths=./pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1
```

Keep the CRD sourced from the pinned upstream chart; generating it from this
API subset would drop supported upstream fields and validation rules.

Upstream support: [seaweedfs-operator#251](https://github.com/seaweedfs/seaweedfs-operator/pull/251).
