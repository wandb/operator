# Configure workload security

W&B applications and migration Jobs accept two runtime settings:
`runAsNonRoot` and `readOnlyRootFilesystem`. Configure them in the W&B resource;
the selected server manifest can also supply defaults for compatible images.

## Set shared and per-workload profiles

Merge this fragment into your complete W&B resource, using application names and
migration keys from your server manifest:

```yaml
spec:
  wandb:
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

Settings resolve separately for each field, in this order:

1. Operator defaults, which currently leave both flags unset.
2. The application or migration's server-manifest profile.
3. The shared `spec.wandb.securityProfile`.
4. The named application or migration's CR profile.

The last explicit value wins. Omitted fields inherit; `false` overrides `true`.
An empty profile inherits both fields. Removing an override restores the
underlying value, which may still enable the setting. Unknown workload names do
not create workloads.

## Check image and platform requirements

`runAsNonRoot` applies to the Pod. `readOnlyRootFilesystem` applies to application
containers, sidecars, and manifest init containers. Enable these settings only
when every affected image supports them: non-root execution needs a compatible
image user or platform-assigned identity, and read-only roots need writable
mounts for runtime files. Profiles do not supply numeric user or group IDs.

Application Pods retain RuntimeDefault seccomp, and their application/init
containers retain disabled privilege escalation, dropped ALL capabilities, and
RuntimeDefault seccomp. Migration Jobs receive that baseline when either resolved
profile field is set, including explicit `false`. Turning a runtime flag off
does not disable that baseline.

These overrides apply to manifest applications and migrations. Managed databases,
object storage, Console, and telemetry have their own security settings. The
managed services' read-only roots do not establish non-root or OpenShift
compatibility for every image. See [OpenShift limitations](openshift.md).

## Apply and verify

Follow the [configuration change procedure](configuration.md#apply-and-verify-changes).
Application profile changes update generated pod templates. Migration profiles
apply when a Job is created; changing them does not modify or restart an existing
or completed Job.

Check Pod admission events, effective security contexts, and application logs.
Test startup, runtime writes, and restart behavior with the selected images.
Configuration and rendering checks alone do not establish platform acceptance.
