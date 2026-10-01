# Server manifest contract versioning

This design is implemented. The durable specification is the
[server manifest contract](../../../pkg/wandb/manifest/README.md).

The initial operator supports only version 1. Missing declarations permanently
mean version 1; unsupported or invalid declarations block reconciliation before
managed-resource writes. Core emits explicit version 1, and WSM preserves
metadata while rewriting images. The contract documentation covers loading,
status, conversion, recovery, and requirements for future revisions.
