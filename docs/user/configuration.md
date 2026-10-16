# Configure W&B

Keep two configuration files: Helm values for the Operator installation, and a
`WeightsAndBiases` resource for each W&B deployment. Apply instance settings to
the CR, rather than editing generated Deployments that Operator will reconcile.
The [reference](reference.md) lists the main configuration surfaces.

## Version and size

```yaml
spec:
  size: small
  requireLimits: false
  wandb:
    hostname: https://wandb.example.com
    version: <wandb-version>
```

`size` accepts `dev`, `micro`, `small`, `medium`, `large`, `xlarge`, and `xxlarge`;
the default is `dev`. The selected server manifest supplies the size profile.
Set the size deliberately for your deployment and check resource requests and
storage before changing it. `requireLimits` controls whether generated workloads
also receive resource limits. See [upgrades](upgrades.md) before changing version.

## License

The current API accepts the license as the string `spec.wandb.license`:

```yaml
spec:
  wandb:
    license: <your-license>
```

This field does not use the value-or-secret envelope described below. Keep files
containing the license in your deployment's protected configuration workflow and
exclude them from public repositories and support attachments.

## Literal values and Secret references

External connections, OIDC settings, notification settings, and proxy URLs use
an object containing either `value` or `valueFrom.secretKeyRef`. Values are
strings, including ports and boolean connection settings:

```yaml
spec:
  mysql:
    default:
      externalMysql:
        host: {value: mysql.example.com}
        port: {value: "3306"}
        database: {value: wandb}
        username: {value: wandb}
        password:
          valueFrom:
            secretKeyRef:
              name: wandb-mysql
              key: password
```

Create the referenced Secret in the W&B resource's namespace before applying
the configuration. Use Secret references for credentials. Set only one source
per field. The older `{name, key}` form is accepted and normalized for compatibility;
use the envelope above for new configuration. A bare string such as
`host: mysql.example.com` is not this API shape.

## Authentication and notifications

OIDC settings live under `spec.wandb.oidc`: `clientId`, `clientSecret`,
`issuerUrl`, and `authMethod` use the envelope above; `sessionLength` is a string.
Match these settings to your W&B release and identity provider configuration.

Email is configured under `spec.wandb.notifications.email`. Choose a `sink` URL
or an `smtp` object, not both. SMTP uses `host`, `port`, `username`, and `password`.
Slack uses `spec.wandb.notifications.slack.clientId` and `clientSecret`.
These connection and credential fields also use value-or-secret objects.

## Application overrides

`spec.wandb.features` selects manifest feature flags. Use flags supported by the
selected W&B release. `spec.wandb.applications` is keyed by manifest application
name and can override autoscaling bounds:

```yaml
spec:
  wandb:
    applications:
      api:
        autoscaling:
          minReplicas: 2
          maxReplicas: 4
```

Use an application name present in your server manifest. Unknown names are
ignored with a log message. Minimum and maximum replicas must be at least one,
and the minimum cannot exceed the maximum. `legacyOverrides` carries migrated
v1 settings; prefer the current fields for new deployments.

## Apply and verify changes

Merge examples into your complete resource; the fragments above are not standalone
manifests. Review and apply the desired configuration:

```bash
kubectl apply --dry-run=server -f wandb.yaml
kubectl diff -f wandb.yaml
kubectl apply -f wandb.yaml
kubectl get weightsandbiases wandb -n wandb -o yaml
```

After reconciliation, check the `Ready` condition for the current generation and
verify the affected application behavior. Some changes replace or restart
workloads. Changing credentials in a Secret does not establish that every
already-running application has reloaded them; verify the affected workloads
and follow the service's rotation procedure.
