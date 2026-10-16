# Operator documentation outline

Status: approved outline; the audience guides are now established.
See [documentation maintenance](../developer/documentation.md) for current coverage
and remaining validation. The source assessment below records the starting point
reviewed on 2026-10-01.

This outline separates documentation for people deploying and operating W&B from
documentation for people developing Operator. It maps existing material to each
section and identifies the writing and verification needed before that material
can become a current guide.

## Scope and entry points

The refactor covers root-level documentation and everything under `docs/`.
Documentation colocated with code elsewhere stays in place; the main guides can
link to it when relevant. The refactor now provides forwarding pages at the former top-level doc paths.

Entry points established by the refactor:

| Location | Purpose |
| --- | --- |
| Root `README.md` | Brief product overview and links to the user and developer guides. |
| `docs/user/README.md` | Deploy and operate W&B using published Operator releases. |
| `docs/developer/README.md` | Set up, understand, change, test, and release Operator. |
| `docs/design/` | Developer design records, with explicit proposal, implemented, or superseded status. |
| Root `DEVELOPMENT.md` | Short pointer to the developer guide after its content is consolidated. |

Keep [CLAUDE.md](../../CLAUDE.md) as agent guidance and
[CHANGELOG.md](../../CHANGELOG.md) as release history. Agent guidance should link
to the relevant developer procedures. Keep changelog generation under the release
tooling and link to release history from the user guide. The changelog currently
covers the v1 release line; clarify the v2 release-note location in that guide.

The sections below are topic outlines, not a requirement to create one file per
heading. “Existing” identifies source material, not verified completeness.
Design proposals must be checked against the implementation before their examples
or behavior are promoted into user instructions. Gaps describe missing coverage
in the documentation in scope, not necessarily missing product features.

## User documentation

Audience: administrators deploying Operator and W&B, configuring their
environment, and keeping the deployment running. The main installation path
should work from published releases without requiring a source checkout.

### Understand and plan a deployment

- What Operator manages, managed versus external infrastructure, and the roles of
  the Helm chart, `WeightsAndBiases` resource, and W&B server manifest.
- Operator version versus W&B version, supported combinations, cluster
  prerequisites, permissions, storage, capacity, hostname, and licensing needs.

**Existing:** [README overview](../user/planning.md), prerequisites in
[migration](../migrating-v1-to-v2.md) and [Console setup](../console-v2-setup.md).

**Work:** Consolidate the concepts and prerequisites. Add verified compatibility
and capacity guidance; explain what must exist before installation begins.

### Install Operator and deploy W&B

- Install prerequisites and a pinned Operator release, prepare namespaces and
  configuration, and apply a complete W&B resource.
- Verify infrastructure, application readiness, hostname access, and initial
  login. Include an OpenShift deployment path.

**Existing:** [README installation](../user/installation.md),
[OpenShift deployment](../openshift.md), and installation steps in
[Console setup](../console-v2-setup.md).

**Work:** Assemble a complete first deployment guide. Validate examples and
readiness checks together; feature guides should link to this installation
procedure instead of repeating it.

### Configure W&B

- Choose size and version, supply a license, and configure supported application
  settings such as authentication and notifications.
- Use literal values and Secret references, distinguish Helm settings from CR
  settings, and understand how configuration changes take effect.

**Existing:** README examples and the implemented design for
[value or secret fields](wandb_v2/secret_or_value_connection_fields.md).

**Work:** Write the configuration guide and extract current examples from the
design record. [Config API](../config-api.md) describes console/configmap
integration and cannot serve as the CR configuration reference.

### Configure backing infrastructure

- Configure managed or external MySQL, Redis, Kafka, object storage, and
  ClickHouse, including supported combinations and instance selection.
- Supply connection settings and credentials, configure TLS, and verify
  connectivity. Explain storage endpoint requirements for browser access.

**Existing:** [Infrastructure connection settings](../infra-connection-settings.md),
object-store examples in [OpenShift](../openshift.md), connection checks in
[migration](../migrating-v1-to-v2.md), and the
[value or secret design](wandb_v2/secret_or_value_connection_fields.md).

**Work:** Turn the connector reference into CR-oriented instructions with examples
for each service. Verify current field shapes, defaults, and instance keys;
keep application parser internals in developer documentation.

### Expose W&B through networking and TLS

- Configure managed or external Ingress and Gateway API resources, DNS, TLS
  termination, and certificates.
- Explain resource ownership, networking mode changes, controller-specific
  requirements, and how to verify routes.

**Existing:** [Networking](../networking.md),
[README Gateway TLS example](../user/networking.md#implementation-specific-tls-options), and
[Ingress and Gateway design](../plan-ingress-gateway-api.md).

**Work:** Retain useful Ingress guidance and write the general Gateway setup
guide. Extract and validate design examples before use; cover prerequisites and
verification alongside controller-specific recipes.

### Deploy with private registries and restricted connectivity

- Mirror the required release artifacts, configure registry locations and
  credentials, and distinguish image pulls from server-manifest retrieval.
- Configure supported proxy settings and custom CAs for Operator and W&B
  workloads, including how to verify access.

**Existing:** [README custom CAs](../user/restricted-environments.md#trust-a-custom-registry-ca)
and [private registry design](wandb_v2/private_registry_manifests.md).

**Work:** Write an end-to-end deployment procedure from verified implementation
behavior. The CA fragment and registry design do not form a complete installation
guide; proxy guidance is also missing from the main user docs.

### Administer and monitor the deployment

- Enable Console, sign in, understand permissions, and rotate its credential.
- Enable or disable telemetry, use Grafana, forward telemetry externally, and
  verify that collection and forwarding work.

**Existing:** [Console setup](../console-v2-setup.md),
[Watchtower deployment](../watchtower-deployment.md), and
[monitoring](../monitoring.md).

**Work:** Keep Console and telemetry as distinct guides within this section.
Reconcile conflicting Console configuration examples and terminology. Move Tilt,
image packaging, manifest authoring, and reconciliation details to developer docs.

### Upgrade Operator and W&B

- Upgrade Operator and W&B as distinct operations, with version selection,
  preparation, readiness checks, and supported recovery limitations.
- Migrate from Operator v1 to v2, preserve existing infrastructure, verify
  application data, and determine when legacy resources can be cleaned up.

**Existing:** [Migration guide](../migrating-v1-to-v2.md), an Operator upgrade
command in [Console setup](../console-v2-setup.md), and
[release history](../../CHANGELOG.md).

**Work:** Complete the migration guide's pre-upgrade TODO. Write routine Operator
and W&B upgrade procedures and document rollback constraints after verifying
supported behavior. Maintainer release instructions are not deployment upgrade
instructions.

### Manage retention and recovery

- Explain ownership, deletion, detach behavior, persistent data, and the
  consequences of removing a W&B resource or the Operator release.
- Identify backup responsibilities, provider-specific restore prerequisites,
  and supported recovery or reattachment procedures.

**Existing:** Retention examples in [README](../../README.md) and backup and
ownership cautions in [migration](../migrating-v1-to-v2.md).

**Work:** Write lifecycle and recovery guidance. Verify which actions are
supported and document their limits. Local Tilt cleanup belongs in the developer
guide and must not be reused as a production uninstall procedure.

### Troubleshoot a deployment

- Follow a diagnostic sequence through installation, CRDs and webhooks,
  infrastructure, migration Jobs, applications, and public access.
- Interpret status and events, collect relevant logs, and diagnose common
  storage, registry, TLS, networking, Console, and telemetry failures.

**Existing:** Troubleshooting and verification fragments in
[migration](../migrating-v1-to-v2.md), [Console setup](../console-v2-setup.md),
[OpenShift](../openshift.md), and [monitoring](../monitoring.md).

**Work:** Create a shared diagnostic guide and link specialized failure cases
from their feature guides. Define useful evidence to collect for support.

### Configuration and compatibility reference

- Document supported Helm values and `WeightsAndBiases` fields, defaults,
  validation, Secret/value forms, and status meanings.
- Provide minimal complete examples and link version-specific compatibility
  information and release notes.

**Existing:** Scattered examples, [connection settings](../infra-connection-settings.md),
and the [value or secret design](wandb_v2/secret_or_value_connection_fields.md).

**Work:** Build a reference checked against the current API and chart. Decide how
to keep it synchronized with releases. Avoid maintaining separate field
definitions in every task guide.

## Developer documentation

Audience: contributors changing Operator code, charts, APIs, tests, or releases.
Development docs can link to the user guide for deployment behavior and to
colocated documentation for specialized workflows.

### Set up the development environment

- Install tools, create a local cluster, configure Tilt, and run a first local
  deployment using published or local server manifests.
- Configure local infrastructure and telemetry, use the OpenShift development
  path when needed, and distinguish routine cleanup from a full reset.

**Existing:** [README development and testing](../../README.md#development) and
[Tilt dependency guide](wandb_v2/tilt.md).

**Work:** Consolidate onboarding and validate current defaults and commands.
Link to specialized colocated guides without relocating them.

### Understand the architecture

- Describe the manager, controllers, CRDs, admission and conversion webhooks,
  CRD installer, charts, and server manifest.
- Explain reconciliation, dependency readiness, resource ownership, status,
  and managed versus external infrastructure.

**Existing:** [CLAUDE architecture map](../../CLAUDE.md),
[infrastructure reconciliation](wandb_v2/infra_reconciliation.md),
[struct usage mapping](wandb_v2/struct_usage_mapping.md), and
[struct usage graph](wandb_v2/struct_usage_graph.md).

**Work:** Write a current architecture overview. Verify historical paths,
component choices, and diagrams before reuse; extract durable guidance from the
agent instructions.

### Make and validate a change

- Explain what to run when Go code, API types, CRDs, charts, or fixtures change.
- Document code generation, embedded CRD synchronization, local rebuilds,
  linting, formatting, and contribution conventions.

**Existing:** [Development flow](../../DEVELOPMENT.md) and workflow rules in
[CLAUDE.md](../../CLAUDE.md).

**Work:** Replace stale Tilt resource names and paths with the current workflow.
Separate working instructions from suggested Tilt improvements and maintain a
single authoritative change-to-command checklist.

### Evolve APIs and preserve compatibility

- Add or change CR fields, defaulting, validation, and conversion behavior.
- Explain v1/v2 compatibility, Secret/value handling, legacy overrides, and
  the tests needed to preserve migration and round-trip behavior.

**Existing:** [Value or secret design](wandb_v2/secret_or_value_connection_fields.md),
[legacy overrides](wandb_v2/legacy_overrides.md),
[legacy env mapping proposal](wandb_v2/legacy_env_var_mapping.md), and
[its implementation plan](wandb_v2/legacy_env_var_mapping_plan.md).

**Work:** Extract verified contracts into a contributor guide. Preserve proposals
and alternatives as design history with accurate status labels.

### Work with server manifests and application reconciliation

- Explain manifest provenance, resolution, caching, local fixtures, and the
  boundary between this repository and upstream manifest generation.
- Trace manifests through infrastructure requirements, generated secrets,
  migrations, application rendering, and cleanup.

**Existing:** Server-manifest explanation in [CLAUDE.md](../../CLAUDE.md), local
manifest setup in [README](../../README.md), and
[private registry design](wandb_v2/private_registry_manifests.md).

**Work:** Write the contributor guide around the current code path. Distinguish
editing local test fixtures from publishing upstream W&B manifests.

### Extend infrastructure and integrations

- Explain the contracts for managed and external infrastructure, readiness,
  connection material, naming, retention, and resource ownership.
- Document networking, telemetry, and Console integration points and the
  corresponding chart and RBAC changes.

**Existing:** [Infrastructure reconciliation](wandb_v2/infra_reconciliation.md),
[connection internals](../infra-connection-settings.md),
[networking plan](../plan-ingress-gateway-api.md), [monitoring](../monitoring.md),
[Watchtower deployment](../watchtower-deployment.md),
[Watchtower research](../watchtower.md), and [Config API](../config-api.md).

**Work:** Separate current implementation contracts from historical research.
Verify whether the configmap/console API note still describes an active contract.
Keep user configuration examples in the user guides and link to them.

### Test changes

- Explain unit and envtest coverage, chart checks, fake generation, and the
  choice of local integration or WESTest scenarios for a change.
- Cover managed/external infrastructure, networking, migration, custom CAs,
  and application data verification; explain test artifact inspection.

**Existing:** Testing sections in [README](../../README.md) and
[DEVELOPMENT.md](../../DEVELOPMENT.md),
[nightly WESTest plan and runbook](../nightly-westest-testing.md), and the
application test data inventory (the earlier working draft is no longer present;
see the [coverage tracker](../developer/documentation.md#remaining-coverage)).

**Work:** Build a current testing guide and separate implemented CI procedures
from rollout proposals. The application data inventory is a proposed coverage
contract in the working tree, not evidence of an implemented generator.

### Debug local development and reconciliation

- Debug controllers and webhooks, inspect generated resources, trace a reconcile
  through status and logs, and distinguish stale CRDs from code failures.
- Diagnose Tilt build and deployment issues and reset local state safely.

**Existing:** [Development troubleshooting](../../DEVELOPMENT.md),
[README cleanup](../developer/setup.md#cleaning-up-tilt), and
[Tilt dependencies](wandb_v2/tilt.md).

**Work:** Update obsolete commands and add a reproducible debugging workflow.
Link to user troubleshooting when the failure is in the deployment rather than
the developer toolchain.

### Build and release Operator

- Build images and charts, package Console, and manage dependency updates.
- Validate and publish development, preview, and stable releases; distinguish
  v2 release procedures from v1 maintenance.

**Existing:** [v2 releasing](../releasing.md), [v1 releasing](../releasing-v1.md),
packaging details in [Watchtower deployment](../watchtower-deployment.md), and
release conventions in [CLAUDE.md](../../CLAUDE.md).

**Work:** Use the current release workflows as the authority. Resolve the
semantic-release description in CLAUDE against the tagged release guide, and
verify Console packaging details before reuse.

### Maintain documentation and design records

- Index current architecture references separately from proposals and historical
  implementation plans.
- Record design status, affected version or implementation reference, and
  replacement links when a design is superseded.
- Define which user and developer pages to update when behavior changes.

**Existing:** The design records linked above and this outline.

**Work:** Add the two audience indexes and a design index. Review status labels,
preserve useful rationale, and give every current procedure one canonical home.

## Suggested order for filling the outlines

1. Assemble user prerequisites and first deployment alongside developer
   onboarding and the change workflow. These establish both entry paths.
2. Write the CR configuration reference and external infrastructure examples;
   use them to validate examples in networking, Console, and OpenShift guides.
3. Complete Gateway, private registry, proxy, and telemetry guidance by extracting
   verified behavior from mixed documents and design records.
4. Fill upgrade, retention, recovery, and troubleshooting gaps. Complete the
   existing migration TODO before treating that guide as ready.
5. Consolidate developer architecture, compatibility, testing, and release
   guidance; label historical designs and replace old entry points with links.

For each section, move or adapt existing material first, validate it against the
current implementation, and then write the missing steps. Check examples and
local links before making the new guide the canonical entry point.
