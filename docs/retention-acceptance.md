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

The gate runs a published WESTest container from the shared action's fixed image
repository. It uses the local operator checkout support from
[operator PR #438](https://github.com/wandb/operator/pull/438) for PR/release
checks; nightly checks use the published chart produced by the nightly build.
For retention upgrades, the baseline phase stays on its published chart while
the candidate phase can build the checkout. Operator CI does not build WESTest
or replace the executable inside its container.

CI rollout prerequisites:

- Merge operator PR #438 before the retention PR; the latter is stacked on it.
- Merge [WESTest PR #67](https://github.com/wandb/westest/pull/67), publish a
  WESTest container containing the scenarios, and set the operator repository
  variable `WESTEST_RETENTION_IMAGE_TAG` to that published tag. Manual and
  reusable invocations can override it with `westest_image_tag`. Setup fails
  clearly if no tag is configured; a missing scenario is also a hard failure.
  WESTest 0.5.2 predates the retention scenarios and cannot run this gate.
- The existing `WESTEST_APP_ID` / `WESTEST_APP_PRIVATE_KEY` app needs read access
  to `wandb/watchtower` for local operator image assets.
- Use the existing eight-core runner pool. Fork PRs keep unit/envtest coverage;
  draft and Dependabot PR runs also skip provisioning, matching PR WESTest's
  credential requirements. Maintainers can dispatch acceptance explicitly.

Only the existing public-safe summary is uploaded. Inventory digests, cluster
state, logs and kubeconfigs stay in the private run artifacts. The operator
checkout mounts read-only into the local Kind test runner through the
existing shared action; cloud scenarios cannot use checkout mode.

Before deleting an upgraded installation, wait for the new webhook endpoints,
the completed operator rollout, and `RetentionReady=True` for its generation.
Admission also checks current ownership, resource UIDs, Argo protection and the
spec/lifecycle fingerprint, so the condition alone cannot authorize deletion. Do not
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
purge run `20261030T230701Z-97950` both created and attached the bucket HTTPRoute,
but stalled before deletion assertions. Kubernetes API requests timed out and
Docker listing and stop requests hung; the underlying cause was not established.
On November 2, Docker responded normally without a restart. Both abandoned test
environments were removed before resuming scenarios sequentially; the unrelated
development cluster was left running.

The first resumed managed-Gateway run, `20261102T165821Z-78620`, caught a real
teardown bug: the route disappeared, but NGINX continually recreated children of
the foreground-deleting Gateway. Managed Gateways now enter the explicit
inventory, lose their WandB owner before admission, and use UID-checked
background deletion after routes. Their children remain under ordinary Gateway
GC. A lifecycle revision in the readiness fingerprint prevents older inventories
from authorizing deletion before this migration completes.

| Scenario (prefix `local-kind-retention-`) | Run ID | Evidence |
| --- | --- | --- |
| `gateway-detach-foreground` | `20261102T172302Z-236` | Passed with operator `05b2922b`; all 102 recorded identities checked, managed Gateway/routes removed and detached backends preserved. The test cluster was then explicitly destroyed. |
| `gateway-detach-orphan` | `20261102T173230Z-16182` | Passed with operator `54cdb6bf`; 102 identities checked after orphan deletion, including explicit Gateway/route cleanup and detached backend preservation. Automatic cluster teardown passed. |
| `external-gateway-detach-foreground` | `20261102T174108Z-26306` | Passed with operator `54cdb6bf`; all 102 identities checked, including preserved customer Gateway, infrastructure routes, and detached backends. Automatic cluster teardown passed. |
| `external-gateway-purge-foreground` | `20261102T174849Z-31142` | Passed with operator `54cdb6bf`; all 102 identities checked, infrastructure routes/backends purged while the customer Gateway and supplied resources survived. Automatic cluster teardown passed. |

The failed November 2 environment was also destroyed. The readiness-revision
guard was added in `54cdb6bf` and is covered by a legacy-inventory admission and
migration regression test; it does not change fresh-installation teardown.
All four Gateway scenarios completed on November 2 without restarting OrbStack.
Operator unit/envtest and WESTest runner tests passed; lint still reports only
the five baseline findings and zero new findings against `aa068410`.
