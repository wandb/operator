# Managed Redis runtime security

Managed Redis, replication, Sentinel, and exporter containers use read-only
roots. The pinned Redis operator chart 0.22.2 enables its upstream
`GenerateConfigInInitContainer` feature. The controller generates `init-config`,
writes configuration to its `config` emptyDir mounted at `/etc/redis`, and
starts Redis/Sentinel directly. Our vendor CRs supply the restricted context
for that generated init container while leaving the separate custom init
container disabled.

The image entrypoints normally append authentication, protected-mode,
persistence, TLS, and Sentinel settings to `/etc/redis`. Making only the root
read-only silently prevents these writes: local Redis health probes pass but
application connections are rejected by protected mode. A writable directory
alone also lets Sentinel's append-based entrypoint repeat its monitor settings
on container restart. The upstream configuration-init path avoids both cases.

Keep this chart feature enabled when using the read-only renderer. Upstream
labels it Alpha in this pinned version, so release validation must cover the
supported authentication, TLS, replication and Sentinel configurations. No
controller/image version changes are required. Custom Redis images must support
the standard `redis-server`/`redis-sentinel` executables and generated config;
their image entrypoints are not executed by this upstream feature.

Existing `/tmp` mounts and data PVCs remain writable. Fixed identities are still
omitted from renderer output on OpenShift; actual SCC acceptance requires a
separate platform run. Runtime tests must use a different Pod as a Redis client,
verify effective persistence/auth settings, and exercise container and Pod
restarts as well as local health probes.

Sentinel also mounts an `emptyDir` at `/var/run`: the pinned upstream config
writes `/var/run/redis-sentinel.pid` on startup. The PID file is ephemeral and
is recreated on container/Pod restart; the root filesystem remains read-only.

Redis and replication PVC templates omit metadata labels because the pinned
upstream CRD rejects that field under strict validation and otherwise prunes
it. Reconciliation derives PVC names from the actual StatefulSet claim templates
and labels those PVCs, including retained ordinals after scale-down. This handles
the current upstream CR-name default as well as legacy/custom template names;
it does not assume a fixed `redis-data` prefix. Validate renderer output with strict API field
validation rather than disabling validation to apply it.
