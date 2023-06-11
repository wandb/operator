# Config API

## Creating First Config map

The operator is designed to handle multiple W&B deployments simultaneously,
which could cause confusion when configuring deployments via the UI. To address
this issue, the UI categorizes deployments by their spec.name.

Consequently, all API calls should include the spec name within the URL to
identify the corresponding configmap to create or update. Note that the state
configmaps follow the naming convention <spec.name>-config-<version>.

By providing the spec name, the operator can either create the initial configmap
or generate a new one and invoke the reconciliation function.

To enhance user experience, you can add a label named wandb.ai/console/default.
When set to true, the console will automatically use the specified spec,
eliminating the need for users to include the spec name in API calls. This makes
the API more user-friendly for developers working with multiple W&B deployments.
