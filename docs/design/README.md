# Design records

These records preserve rationale and earlier implementation plans. Use the
[developer guides](../developer/README.md) for current workflows and architecture,
and the [user guides](../user/README.md) for deployment instructions. Historical
code snippets and package paths may differ from the current implementation.

| Record | Status and current destination |
| --- | --- |
| [Documentation outline](documentation-outline.md) | Original approved structure; see [coverage tracking](../developer/documentation.md) for implementation progress and remaining work |
| [Infrastructure reconciliation](wandb_v2/infra_reconciliation.md) | Historical architecture; replaced as a current overview by [architecture](../developer/architecture.md) |
| [Struct usage mapping](wandb_v2/struct_usage_mapping.md) and [graph](wandb_v2/struct_usage_graph.md) | Historical package/type snapshots; not maintained as a current API reference |
| [Tilt dependency graph](wandb_v2/tilt.md) | Dependency snapshot; use [setup](../developer/setup.md) and [workflow](../developer/workflow.md) for current commands |
| [Legacy overrides](wandb_v2/legacy_overrides.md) | Design history for implemented compatibility behavior; see [API development](../developer/api-development.md) |
| [Legacy environment mapping](wandb_v2/legacy_env_var_mapping.md) and [implementation plan](wandb_v2/legacy_env_var_mapping_plan.md) | Original proposal; related implementation now exists in `legacy_env_mapping.go`; verify details against current code |
| [Value or secret connection fields](wandb_v2/secret_or_value_connection_fields.md) | Implemented design record; current examples are in [configuration](../user/configuration.md) |
| [Private registry manifests](wandb_v2/private_registry_manifests.md) | Original design; authenticated manifest retrieval is implemented; see [restricted environments](../user/restricted-environments.md) |
| [Ingress and Gateway plan](networking-plan.md) | Historical implementation plan; use [networking](../user/networking.md) for current API values |
| [Watchtower research](watchtower-history.md) | Historical integration research; current packaging is in [Console integration](../developer/console.md) |
| [Nightly WESTest plan](nightly-westest-plan.md) | Historical rollout plan; manual workflow exists, schedule is disabled; see [testing](../developer/testing.md) |
| [Legacy Config API](legacy-config-api.md) | Historical console/configmap note; current consumer and applicability require review |

When revisiting a record, update its status and link to the current implementation
or successor. Preserve the original reasoning without presenting an old proposal
as a supported deployment procedure.
