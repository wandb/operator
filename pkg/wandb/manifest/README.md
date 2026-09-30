# Server manifest contract

The server manifest is the contract between the W&B server release and the
operator. Its `manifestVersion` identifies the structure and reconciliation
semantics the artifact requires, independently of either product's release tag.

The operator currently supports **version 1 only**. An artifact with no version
declaration anywhere is permanently interpreted as version 1. That default will
not change when newer contracts are introduced.

```yaml
---
manifestVersion: 1
requiredOperatorVersion: ^2.0.0 # Deprecated, ignored compatibility metadata.
applications:
  # Version-1 application definitions.
```

`manifestVersion` is a positive decimal integer, not SemVer. Stable, beta, RC,
nightly, and branch builds use the contract their payload requires; prerelease
ordering is irrelevant. This field is unrelated to the Kubernetes CR API
version `apps.wandb.com/v2`.

## Version declarations

The version covers the complete artifact, including `sizing.yaml` and other
fragments. The producer writes it in the root `manifest.yaml`. Consumers resolve
all explicit declarations before selecting a decoder:

| Declaration | Result |
| --- | --- |
| Absent in every file | Version 1 |
| `manifestVersion: 1` | Supported version 1 |
| Another positive integer, such as `2` | Unsupported contract |
| Same explicit integer in several files | One artifact with that version |
| Explicit version in some files, absent in others | Undeclared fragments inherit that version |
| Conflicting declarations | Invalid metadata |
| Duplicate keys within a document, even with equal values | Invalid metadata |
| Null, an empty value, a string, boolean, float, sequence, or mapping | Invalid metadata |
| Zero, negative numbers, or integers above 2147483647 | Invalid metadata |

Explicit numbers use digits only, starting with `1` through `9`: `+1`, `01`,
`0x1`, and `1.0` are invalid. The key must be spelled `manifestVersion`, including
case. Aliased version keys or values and versions supplied by YAML merges are
invalid. Payload aliases remain supported. Each file must contain at most one
YAML document; multiple documents are rejected so metadata and payload cannot
refer to different documents.

An unsupported contract never falls back to the version-1 decoder. For example,
a version-2 root and an unversioned sizing fragment resolve to version 2 and are
rejected by this build before typed payload decoding.

The deprecated `requiredOperatorVersion` field remains accepted and ignored. It
does not override or supplement this check.

## Loading and support

Local single-file, local directory, and OCI loading use the same artifact
decoder in [version.go](version.go). It reads YAML metadata, resolves the
artifact version, checks the private decoder registry, and then decodes and
merges the payload. Version 1 retains the existing payload decoding and merge
rules, including acceptance of unknown payload fields.

`SupportedVersions()` returns a sorted copy of the registered versions, currently
`[1]`. There is no separate configurable support range or bypass. Registering a
new version promises both decoding and reconciliation support for its semantics.

Every successfully loaded `Manifest` has a normalized `ManifestVersion`.
`VersionExplicit()` reports whether the artifact declared it, for diagnostics.
Downstream reconciliation helpers consume manifests returned by these loaders
and assume compatibility has already been established.

Loaders return errors that can be inspected with `errors.As` through wrapping:

- `InvalidManifestVersionError`: invalid or conflicting metadata, with a source
  file and explanation.
- `UnsupportedManifestVersionError`: a valid but unsupported version, with the
  supported set.
- `ManifestDecodeError`: invalid YAML or a payload the selected decoder cannot
  decode.

Fetch and layer extraction failures remain errors. A partially read OCI artifact
cannot silently lose its metadata and default to version 1. Cached OCI artifacts
are validated on every load against the current binary's registry. The existing
repository-scoped cache treats tags as immutable: publish a new artifact
reference to correct metadata, rather than retagging a cached reference.

## Reconciliation and recovery

The W&B reconciler checks compatibility before legacy Secret/spec migrations,
infrastructure provisioning, application reconciliation, networking, or migration
Jobs. Unsupported or invalid metadata stops those operations for that W&B
instance. Other instances can still reconcile. Already accepted Application
specs and running Jobs remain under their existing controllers; this gate does
not cancel work started before the manifest selection changed.

Deletion and retention finalizers run without loading the manifest. Adding the
existing finalizer, updating status, and recording Events are permitted before
the compatibility gate passes.

The existing conditions array reports the result without new CRD fields:

| Result | `ManifestCompatible` | Reason |
| --- | --- | --- |
| Successfully loaded version 1 | `True` | `SupportedManifestVersion` |
| Valid but unsupported version | `False` | `UnsupportedManifestVersion` |
| Invalid metadata | `False` | `InvalidManifestVersion` |
| Credentials or artifact retrieval fail | `Unknown` | `ManifestUnavailable` |
| YAML or payload decoding fails | `Unknown` | `InvalidManifest` |

An unsuccessful check also sets `Ready=False` and `status.ready=false`. This
describes the requested configuration; existing workloads may still be running.
The condition carries the CR generation that was checked without advancing the
top-level `status.observedGeneration` used by application reconciliation.

Version errors retry after one minute and on CR changes. Retrieval, decoding,
and status-write errors retain normal controller error retries. Events are
emitted when the compatibility outcome changes, not on every retry. Success
records whether version 1 was explicit or defaulted and clears a prior
compatibility failure. It does not mark the deployment ready; ordinary readiness
checks still apply.

For an unsupported version, install an operator that supports that contract or
select a supported server artifact. Selecting an older artifact does not by
itself establish database downgrade safety.

## API conversion

The v1-to-v2 conversion path needs manifest application metadata to preserve
application overrides. It propagates compatibility errors rather than producing
a successful conversion with missing overrides. Unversioned and explicit
version-1 manifests work identically.

A v1 request can therefore fail conversion before a CR exists and return an API
error instead of a status condition. Existing v2 objects remain readable,
editable, and deletable through the v2 API while their artifact is unsupported.
Recovery of a historically stored v1 object that needs an unsupported manifest
for conversion can require a compatible operator. Any future removal of
version-1 support must address stored-version migration and conversion.

## Publishing and evolution

The core server-manifest template emits literal version 1 and validates the
rendered declaration during its image build. It is not a release build argument.
WSM image rewriting preserves explicit version metadata and leaves legacy
artifacts unversioned.

Routine image, configuration-value, and sizing changes that use existing
semantics keep version 1. A change needs a new revision when an older consumer
could ignore or misinterpret it and produce an incorrect deployment: examples
include new required fields, enum cases, defaults, or resource/migration behavior.
The producer declares the oldest contract sufficient for its requirements.
Published revisions are immutable, even when first used in prerelease builds.
Coordinate revision allocation across branches so one number has one meaning.

A future operator can support `[1, 2]` to provide an overlapping upgrade path.
Higher numbers never imply automatic support for lower ones. Unversioned
artifacts continue to resolve to 1 even if support for 1 is eventually removed.

Older operators ignore version metadata and cannot be made to enforce this gate
retroactively. Before publishing a newer contract, establish an upgrade path to
an enforcing operator. WSM also uses typed structures to enumerate images;
preserving metadata alone does not make it capable of mirroring future
contracts. New contracts require consumer support or explicit rejection before
partial image enumeration. Target-operator preflight in WSM remains future work.

Contract compatibility does not establish a product support window, require a
particular server release, or guarantee database upgrade/downgrade safety.
