# Managed MOCO runtime security

The MySQL container and upstream-generated agent, slow-log, exporter, and both
init containers use read-only roots. Existing data, configuration, shared
binary, log, socket, and temporary volumes provide their writable paths.

MOCO generates two init containers: `copy-moco-init` copies its initialization
binary into a shared volume, and `moco-init` runs that binary to initialize
MySQL. Previously the renderer supplied a security override only for
`moco-init`; the copy container retained upstream defaults. Both names now
receive the same context through `podTemplate.overwriteContainers`.

The pinned chart 0.26.0 (controller 0.36.0) already supports the copy-container
override. Refresh the vendored MOCO CRD from that chart with `make
generate-vendored`: its stale enum otherwise rejects `copy-moco-init`. The Go
API dependency predates the named enum constant, so the renderer uses the
upstream copy-container name with the API's string-backed type. This requires
no chart, image, or controller upgrade.

The renderer preserves existing identity behavior and omits fixed IDs on
OpenShift. Renderer tests are not proof of SCC compatibility: validate the
actual upstream-generated Pods on OpenShift before claiming acceptance there.
MOCO controller and direct MySQL-initialization Job hardening are separate
workstreams.
