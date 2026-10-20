# Migration service accounts in server manifests

A migration can declare a dedicated Kubernetes identity, namespaced permissions,
and whether its API token is automatically mounted. Application identities are
unchanged by this first implementation.

```yaml
secrets:
  - name: wandb-internal-signer
    useExactName: true

migrations:
  internal-signer:
    serviceAccount:
      automountServiceAccountToken: true
      permissions:
        - resources: [secrets]
          resourceNames: [wandb-internal-signer]
          verbs: [get, update]
    image:
      repository: us-docker.pkg.dev/wandb-production/public/wandb/megabinary
      tag: <server-version>
    args: [secret-generation-job]
    env:
      - name: INTERNAL_SIGNER_K8S_SECRET_NAME
        value: wandb-internal-signer
      - name: INTERNAL_SIGNER_K8S_NAMESPACE
        valueFrom:
          fieldRef:
            fieldPath: metadata.namespace
```

For a CR named `wandb`, the declaration creates `wandb-migration-internal-signer`
as the ServiceAccount, Role, and RoleBinding in the CR's namespace. The Role contains
the declared Kubernetes policy rules; omitted `apiGroups` defaults to `[""]`
(the core API group). Rules must contain resources and verbs; `nonResourceURLs`
are not supported by namespaced Roles. Cluster-scoped access is not granted.
The operator must itself be authorized to grant the requested permissions;
this feature does not add `bind` or `escalate` grants to the operator.

The account name defaults to `<cr-name>-migration-<migration-name>`. Long derived
account names are shortened deterministically. Optional `name` supplies an exact
name with no CR prefix or shortening; it must be a valid Kubernetes ServiceAccount
name. The signer example uses the generated default. Two migrations cannot
declare the same account, and a dedicated account cannot reuse the shared
application account. Identity resources are CR-owned and labeled as migration
identities; the operator refuses
to adopt resources outside that identity set. Global image pull secrets are
propagated to the account.

Omitting `serviceAccount` preserves the shared migration account and its current
credential behavior. An empty declaration (`serviceAccount: {}`) creates a
dedicated account without Role permissions and defaults automounting to false.
Setting `automountServiceAccountToken: true` enables automatic credentials;
the chosen value is applied to both the account and Job PodSpec.

The signer uses automatic credentials because it is a short-lived Job that needs
standard in-cluster API access. Kubernetes supplies the pod-bound token, CA, and
namespace at `/var/run/secrets/kubernetes.io/serviceaccount/`; the operator does
not add explicit credential volumes. Token issuance and refresh do not require
`serviceaccounts/token` permission on the migration account. The Job process
exiting does not itself invalidate the token: deletion of its pod or token expiry
determines its remaining validity.

Top-level `secrets` declares the signer's empty Secret before migrations run.
Secret creation is independent of service accounts and migration permissions.
See [manifest Secrets](server-manifest-secrets.md) for optional value generation
and compatibility with legacy `generatedSecrets` declarations.

This is useful for the signer because Kubernetes RBAC cannot restrict Secret
`create` requests by resource name. The current signer can initialize an existing
empty Secret using `get/update`, so it needs no namespace-wide create permission.
The Secret name in its environment must match the declared permission and
Secret. This change does not implement key rotation or recovery after a
completed migration.

## Lifecycle and publication

Identity resources reconcile before migration Jobs and even after migration
completion. Adding this configuration does not rerun completed migrations for
the same server version. Existing Jobs retain their immutable PodSpec; new Jobs
use the declared identity. Allow an existing migration to complete before
changing its account or permissions.

Removing `permissions` from a retained account declaration removes its managed
Role and RoleBinding. Secrets are preserved. Removing the declaration or migration,
or changing its name, does not delete old identity resources: those remain
CR-owned until CR deletion. Revoke the old account's permissions before removing
or renaming it if immediate revocation is required.

Declared ownerless Secrets are adopted by the CR. On CR deletion they follow
the global retention policy: detach removes this CR's owner reference, while
purge leaves ownership for garbage collection. A replacement CR can adopt
ownerless retained Secrets without resetting signing material.

Published manifests are generated in `wandb/core/onprem/server-manifest`, outside
this repository. The local `0.86.0-daily.14` fixture demonstrates the declaration.
Upstream publication must require an operator version supporting these fields;
older operators ignore them and continue using the shared account.

Before publishing, validate fresh bootstrap with the intended server image,
initialization of an empty Secret, preservation of existing keys, upgrades and
rollback, and signed uploads/downloads. Shared application RBAC, anonymous OIDC
discovery, and operator/dependency RBAC reductions remain separate changes.
