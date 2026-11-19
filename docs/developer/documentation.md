# Maintain Operator documentation

Root documentation and `docs/` have two reader paths:

- [User documentation](../user/README.md) explains published installation,
  configuration, observable behavior, and operations.
- [Developer documentation](README.md) explains local workflows, architecture,
  implementation contracts, testing, and releases.

Keep specialized guides colocated with code in their existing directories and
link to them where useful. Keep design rationale under [design records](../design/README.md).
The original [outline](../design/documentation-outline.md) records the intended
coverage and source assessment.

## Updating a guide

When a behavior changes, update the relevant task guide and examples, the
configuration reference, and any implementation guide describing the contract.
Use one canonical procedure and link to it instead of repeating installation
steps. Keep placeholders explicit and distinguish complete manifests from
fragments. User installation should work with published charts and local values
files, without requiring a checkout.

Validate examples against the current CRD schema and admission rules. Rendering
or schema checks do not prove a live deployment works; record the version and
scenario when an end-to-end procedure has been exercised. Check local links and
anchors whenever pages move. The old flat documentation pages have been removed;
link directly to the audience guides and update in-scope references when moving
new material from main.

Design records need an explicit status and links to the current implementation
or replacement guide. Historical examples are evidence of a decision, not
instructions for current deployments.

## Remaining coverage

This refactor establishes both guide sets, reuses the existing material, and
corrects examples against the current source. The following require further
evidence or operational validation:

| Area | Current coverage | Work still required |
| --- | --- | --- |
| Compatibility and sizing | Version roles, prerequisites, size profiles | Publish tested Operator/W&B/Kubernetes combinations and measured capacity guidance |
| First deployment | Complete managed Ingress example and verification steps | Exercise the sequence on supported cluster profiles and record results |
| External infrastructure | Named instances, canonical Secret/value form, MySQL/Redis/S3/ClickHouse examples | Test provider combinations, TLS variants, and cloud workload identity recipes, including GCS/Azure |
| Isolated environments | Registry, manifest, CA, proxy, and telemetry image/plugin configuration | Validate a complete offline mirror inventory and per-dependency image overrides for each release |
| OpenShift | Profile, log-agent opt-out, runtime constraints, and external-store example | Validate managed-service images and SCC admission, especially SeaweedFS non-root behavior; revalidate frontend constraints and produce a tested Route recipe |
| Workload security | CR precedence, manifest merge rules, and managed-service implementation records | Exercise supported images, writable paths, restarts, migration Jobs, and platform admission |
| Support bundles | Template configuration, launch/status checks, and permission model | Validate collection, redaction, object-store delivery, retrieval, and cleanup on supported clusters |
| CI integration | Current PR and nightly WESTest runbooks | Complete cloud cleanup validation and secure failed-teardown recovery before enabling unattended schedules |
| Routine upgrades | Separate Operator and W&B procedures, verification, recovery limits | Verify supported rollback and downgrade behavior per release and document the constraints |
| v1 migration | Preserved preflight, ownership caveats, cutover checks | Validate release-specific controller handoff, resource adoption, downtime, cleanup, and rollback sequence |
| Restore and recovery | Retention semantics and ownership checks | Exercise provider-specific backup/restore and detached-resource recovery |
| Reference | Main field index and installed-schema lookup | Decide whether to generate and publish a full versioned API/Helm reference |
| Application test data | Basic smoke tests and test selection | Recover or recreate the broader proposed data inventory and implement/verify a seed-and-readback workflow; the draft referenced in the original assessment is absent from this checkout |
| Historical architecture and plans | Status banners and current-guide links | Reconcile detailed historical statements only when preserving them as a current reference |
| Legacy Config API | Archived integration note | Determine whether the original console/configmap contract still has a supported consumer |

Prioritize tested onboarding, configuration examples, and migration/recovery
runbooks before expanding historical implementation notes.
