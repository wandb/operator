# Server-manifest workload security profiles

Server manifests can opt each application and migration into security settings
supported by its images. These fields belong to the server manifest, not the
WeightsAndBiases CR. There is no server-version threshold or automatic activation.

```yaml
applications:
  api:
    name: api
    image:
      repository: example/api
      tag: nonroot-compatible
    securityProfile:
      runAsNonRoot: true
      readOnlyRootFilesystem: true
migrations:
  database:
    image:
      repository: example/migrate
      tag: nonroot-compatible
    securityProfile:
      runAsNonRoot: true
```

Both settings are optional booleans. Omitted settings remain unset; explicit
`false` is preserved. An omitted or empty profile retains legacy behavior.
`runAsNonRoot` applies to the pod; `readOnlyRootFilesystem` applies to every
application container, sidecar, and manifest init container. Migrations have
independent profiles applied when their Jobs are created. Existing/completed
migration Jobs are not restarted when only the profile changes.

Application pods retain RuntimeDefault seccomp, and application/init containers
retain disabled privilege escalation, dropped ALL capabilities, and RuntimeDefault
seccomp. Unprofiled migration Jobs retain their existing security contexts; a
migration with either field explicitly set also receives that baseline hardening.
No numeric UID, GID, or filesystem group is supplied by this profile.

## Fragment merging and rollback

Manifest fragments load in lexicographic filename order. For an application,
later explicitly supplied profile fields override earlier ones, including `false`.
Omitted fields and `{}` preserve values from earlier fragments; they do not clear
a prior opt-in. Sizing-only fragments preserve the profile.

Migration definitions retain the existing whole-entry replacement behavior: a
later fragment with the same migration key replaces the entire definition,
including image, arguments, and profile. Supply the complete migration definition
when overriding it; profile-only migration overlays are not supported.

To remove application settings, remove them from all contributing fragments.
Reconciliation clears those settings from the generated Application pod template.
Use explicit `false` when an overriding fragment must disable a setting inherited
from another fragment.

## Compatibility and activation

Only opt in after all of the application's images (including init containers and
sidecars) support the requested settings. Non-root execution needs a compatible
image user or platform-assigned identity; the profile does not select one.
Read-only root execution needs explicit writable mounts for every runtime write
path. Image tags alone do not prove compatibility.

Core owns activation in its server-manifest source and must select an operator
release supporting this contract. This operator change does not activate shipping
manifests, modify dependency security profiles, or establish image/OpenShift
runtime acceptance. Those require separate rendered and running-workload evidence.
