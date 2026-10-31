# Running retention acceptance

The implementation lives in `internal/controller/retention`; the v2 reconcile
path uses its scoped client and durable inventory. The DELETE webhook validates
both served API versions against fresh canonical state. See the
[contract and inventory](design/wandb_v2/resource_retention.md) for dependencies,
readoption, migration requirements, and the guarantee's limits.

All real-cluster tests live in the WESTest repository. Its
`docs/retention-testing.md` describes the twenty-two scenario cases and local runs.
The four representative PR checks cover paused foreground detach, orphan
purge through v1, mixed policies under background deletion, and managed-Gateway route cleanup.

`.github/workflows/retention.yaml` is both a PR workflow and a reusable full
acceptance gate. The existing nightly workflow calls its full matrix, and the
tagged release job waits for that matrix before publishing. The existing nightly
schedule remains disabled. Maintainers can dispatch either profile manually.

The gate compiles a WESTest runner with its embedded scenarios and runs it inside
the pinned published tool image. It therefore does not assume a new WESTest
container release already exists. The candidate operator is built from the
checkout for PR/release checks; nightly checks use the published chart produced
by the nightly build.

CI rollout prerequisites:

- Merge the companion WESTest branch first, or set `WESTEST_RETENTION_REF` to an
  available reviewed revision containing the scenarios. The default is `main`.
- The existing `WESTEST_APP_ID` / `WESTEST_APP_PRIVATE_KEY` app needs read access
  to `wandb/westest` and `wandb/watchtower` for source and pinned build assets.
- Use the existing eight-core runner pool. Fork PRs keep unit/envtest coverage;
  maintainers dispatch acceptance explicitly because private assets require the
  app credential.

Only the existing public-safe summary is uploaded. Inventory digests, cluster
state, logs and kubeconfigs stay in the private run artifacts. The source
checkout and runner executable mount read-only into the local Kind test runner;
cloud scenarios cannot use those mounts.

Before deleting an upgraded installation, wait for the new webhook endpoints,
the completed operator rollout, and `RetentionReady=True` for its generation.
Admission also checks current ownership, resource UIDs, Argo protection and the
spec fingerprint, so the condition alone cannot authorize deletion. Do not
remove the inventory ConfigMap or cleanup finalizer to bypass a migration block.

To readopt after a completed detach, create the replacement WandB CR with the
same infrastructure instance keys, resource names and namespaces, and set
`metadata.annotations["weightsandbiases.apps.wandb.com/adopt-from-uid"]` to the
previous WandB UID. Keep its inventory ConfigMap. The old CR must be gone;
resources must match the donor inventory's retained identities. A matching CR
name alone does not authorize adoption. Wait for retention and workload
readiness before directing traffic to the replacement installation.

## Development validation, 2026-10-30

The following WESTest Kind runs passed during implementation. Artifacts are in
WESTest's ignored `artifacts/<scenario>/<run-id>` directories; each scenario
completed its own cluster teardown.

| Scenario (prefix `local-kind-retention-`) | Run ID | Evidence |
| --- | --- | --- |
| `detach-foreground` | `20261030T194816Z-22583` | 100 resource identities survived active GC while reconciliation was paused; restart/readoption and seeded-data readback passed. This development run loaded the workload-ordering fix before its recovery assertions. |
| `argocd-purge-cascade-combined` | `20261030T201829Z-60024` | Resource-tree association and Application cascade; all 101 recorded identities satisfied purge/supplied-resource assertions. |
| `mixed-background` | `20261030T202950Z-85688` | 124 identities checked; custom-named Redis Sentinel in another namespace purged while default instances and supplied resources survived. |
| `argocd-detach-prune-combined` | `20261030T203052Z-89779` | Automated Git pruning completed with 101 resource identities satisfying detach/supplied-resource assertions. |
| `purge-orphan` | `20261030T204114Z-3437` | v1 DELETE with orphan propagation completed explicit purge; 100 recorded identities checked. |
| `upgrade` | `20261030T203957Z-1833` | Published `2.0.0-beta.9` to candidate; existing identities survived migration, followed by foreground detach, workload restart, readoption, and seeded-data readback. |
| `argocd-detach-cascade-annotation` | `20261030T205032Z-9866` | Annotation tracking, resource-tree association, and Argo Application cascade deletion preserved retained roots, configuration, credentials, and claims. |

These representative development runs do not replace the full twenty-two-case
release matrix. Unit/envtest suites and the WESTest runner suite pass. Workflow
helper tests and workflow syntax checks pass. Lint reports no new findings
against the branch base; the repository's five existing staticcheck findings
remain outside this change.

## Gateway follow-up validation

Four additional WESTest cases cover managed Gateway detach under foreground and
orphan propagation, and external Gateway detach/purge under foreground
propagation. The validator lists live infrastructure HTTPRoutes independently
of the inventory, checks their expected policy against the backend Services,
and checks Gateway removal or preservation. Unit tests additionally cover
legacy-route migration, explicit readoption, GKE HealthCheckPolicies,
cross-namespace routes, UID replacement rejection, and finalizer retries.

Operator unit/envtest and WESTest runner suites pass after this change. The full
operator linter reports the same five pre-existing findings, with no new
findings. CI matrix helper tests pass; the full release matrix now has 22 cases.

The initial managed-Gateway run `20261030T230659Z-97593` and external-Gateway
purge run `20261030T230701Z-97950` both created and attached the bucket HTTPRoute.
Before deletion assertions, the local OrbStack Docker engine stopped responding;
Kubernetes API requests timed out and Docker container listing and stop requests
also hung. These attempts do **not** establish real-cluster teardown results.
The new Gateway scenarios still require a completed run on a healthy engine.
