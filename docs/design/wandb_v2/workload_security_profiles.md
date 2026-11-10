# Workload security profiles

Configure workload security through the WeightsAndBiases CR. Server manifests
can supply release-specific defaults while compatible image versions roll out.
Both use the same optional `runAsNonRoot` and `readOnlyRootFilesystem` fields.

## CR overrides

Settings resolve per field, with the last explicit value winning:

1. Operator defaults.
2. The application's or migration's server-manifest `securityProfile`.
3. `spec.wandb.securityProfile` in the CR.
4. `spec.wandb.applications.<name>.securityProfile` or
   `spec.wandb.migrations.<name>.securityProfile` in the CR.

Omitted fields inherit. Explicit `false` overrides `true`. Empty profiles inherit
all fields. Removing a CR override restores the underlying manifest or operator
value; it does not necessarily disable the setting.

```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
spec:
  wandb:
    # Other installation settings omitted.
    securityProfile:
      runAsNonRoot: true
      readOnlyRootFilesystem: true
    applications:
      weave:
        securityProfile:
          readOnlyRootFilesystem: false
    migrations:
      weave-trace:
        securityProfile:
          readOnlyRootFilesystem: false
```

Names match manifest application names and migration keys. Unknown names do not
create workloads. The shared profile applies to manifest applications and
migrations; managed dependencies and Watchtower have separate security settings.

`runAsNonRoot` applies to the Pod. `readOnlyRootFilesystem` applies to every
application container, sidecar, and manifest init container. Migration settings
apply when a Job is created; changes do not modify or restart existing/completed
Jobs. Application profile changes reconcile into the generated pod template.

Application Pods retain RuntimeDefault seccomp, and application/init containers
retain disabled privilege escalation, dropped ALL capabilities and RuntimeDefault
seccomp. Migration Jobs receive that baseline when either resolved profile field
is set, including explicit `false`. Neither runtime flag disables that baseline.
The profile supplies no numeric UID, GID or filesystem group.

## Shipping defaults

Operator defaults currently leave both runtime flags unset. Core can opt compatible
workloads in through its versioned server manifest:

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

Once the supported server images can use these settings by default, enable them
in `defaultWorkloadSecurityProfile` in the operator and remove redundant profiles
from new server manifests. The CR override paths and precedence stay unchanged;
customers will not need explicit manifest profiles to receive the defaults.
That later release must establish the supported image/version floor. This change
does not introduce a version threshold or turn those defaults on prematurely.

Enable flags only when all affected images support them. Non-root execution needs
a compatible image user or platform-assigned identity. Read-only roots need
explicit writable mounts for runtime paths. Manifest activation must require an
operator release supporting the contract. Image, upgrade and OpenShift acceptance
remain separate runtime validation.

## Manifest fragments

Application fragments merge in filename order: later explicit fields win, while
omitted fields, null and empty profiles inherit. Sizing fragments preserve the
profile. Migration fragments replace the whole entry, including its image,
arguments and profile; supply the complete definition when overriding a migration
in another manifest fragment. CR migration overrides change only the profile and
do not replace the manifest's image or arguments.
