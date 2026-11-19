# Develop Operator

Start with the local development environment, then use the change workflow to
keep generated code, CRDs, and the running controller in sync.

| Task | Guide |
| --- | --- |
| Run Operator locally with Kind and Tilt | [Development setup](setup.md) |
| Understand components and reconciliation | [Architecture](architecture.md) |
| Change code, APIs, CRDs, or charts | [Development workflow](workflow.md) |
| Evolve APIs and preserve compatibility | [API development](api-development.md) |
| Work with manifests and integrations | [Reconciliation and integrations](reconciliation.md) |
| Define manifest Secrets and migration identities | [Secrets](server-manifest-secrets.md), [service accounts](server-manifest-service-accounts.md) |
| Change workload security contracts | [Workload security](workload-security.md) |
| Maintain collection templates and permissions | [Support-bundle integration](support-bundles.md) |
| Run WESTest in CI | [PR testing](pr-westest-testing.md), [nightly testing](nightly-westest.md) |
| Test locally and through CI | [Testing](testing.md) |
| Diagnose local build and reconciliation failures | [Debugging](debugging.md) |
| Work on telemetry | [Telemetry internals](telemetry.md) |
| Work on the administration UI integration | [Console integration](console.md) |
| Inspect upstream connection formats | [Connection internals](connection-internals.md) |
| Publish Operator releases | [v2 releases](releasing.md), [v1 maintenance releases](releasing-v1.md) |
| Maintain these guides and track missing coverage | [Documentation maintenance](documentation.md) |

Use the [user guides](../user/README.md) for deployment behavior and the
[design index](../design/README.md) for rationale and historical proposals.
Specialized documentation colocated with code remains in its original directory.
