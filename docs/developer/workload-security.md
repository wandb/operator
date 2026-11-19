# Develop workload security profiles

The [user guide](../user/workload-security.md) defines the CR override paths,
precedence, and rollout behavior. `WorkloadSecurityProfile` is shared between the
CR and server manifest. Preserve the distinction between omitted fields and
explicit `false` through defaulting, conversion, merging, and rendering.

## Manifest defaults and merging

The current Operator defaults leave both runtime flags unset. A versioned server
manifest can opt compatible applications and migrations into stricter settings:

```yaml
applications:
  api:
    # Other application fields omitted.
    securityProfile:
      runAsNonRoot: true
      readOnlyRootFilesystem: true
migrations:
  gorilla:
    # Other migration fields omitted.
    securityProfile:
      runAsNonRoot: true
      readOnlyRootFilesystem: true
```

Application fragments merge in filename order: later explicit fields win, while
omitted fields, null, and empty profiles inherit. Sizing fragments preserve the
profile. Migration fragments replace the whole entry, so an override must include
its complete image, arguments, and profile. CR migration overrides change only
the profile and do not replace manifest image or arguments.

Once supported server images can use stricter settings by default, update
`defaultWorkloadSecurityProfile` and establish the supported image/version floor.
That change must preserve the CR override precedence. Manifest activation must
require an Operator version supporting the contract.

## Implementation and verification

Start with [the shared API type](../../api/v2/security_profile.go),
[profile resolution](../../internal/controller/reconciler/security_profile.go),
[application rendering](../../internal/controller/reconciler/pods.go), and
[manifest merging](../../pkg/wandb/manifest/manifest.go).

Test inheritance, explicit false, fragment ordering, conversion, application
sidecars/init containers, and new versus existing migration Jobs. Validate the
actual images and writable mounts before changing release defaults. Keep the
user guide's platform limitations aligned with runtime evidence.

## Managed infrastructure

Managed infrastructure uses component-specific renderers rather than the W&B
application profile. The implementation records describe upstream contracts:

- [MOCO](../design/wandb_v2/moco_runtime_security.md): read-only roots for MySQL,
  generated sidecars, and both upstream init containers.
- [Redis](../design/wandb_v2/redis_runtime_security.md): generated configuration
  init containers, writable runtime paths, restart validation, and PVC cleanup.
- [SeaweedFS](../design/wandb_v2/seaweedfs_runtime_security.md): read-only roots,
  the curated upstream API, and the unresolved non-root image requirement.

ClickHouse and Keeper also use read-only roots in their managed renderers.
Treat renderer tests, image runtime tests, and OpenShift admission as separate
checks. The [original profile design](../design/wandb_v2/workload_security_profiles.md)
preserves the implementation rationale.
