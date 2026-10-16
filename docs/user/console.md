# Administer W&B with Console

Console v2 is the administration UI served at your W&B hostname's `/console`
path. It is implemented by Watchtower and packaged in the Operator image, so it
does not require a separate chart or Helm release.

## Enable Console

Complete [installation](installation.md), including the public hostname and a
working `ingress` or `gateway` configuration. Add this field to your W&B resource:

```yaml
spec:
  adminConsoleEnabled: true
```

For an existing deployment named `wandb`:

```bash
kubectl patch weightsandbiases wandb -n wandb --type=merge \
  -p '{"spec":{"adminConsoleEnabled":true}}'
```

Console is disabled by default. Enabling it grants authenticated administrators
cluster-wide read access and access to modify the W&B resource. With an external
Ingress, add its `/console` route to your routing configuration.

## Verify and sign in

```bash
kubectl get pods -n wandb -l weightsandbiases.apps.wandb.com/component=watchtower
kubectl get weightsandbiases wandb -n wandb \
  -o jsonpath='{.status.watchtowerStatus}'
```

Open `https://<your-wandb-hostname>/console`. Sign in with your W&B administrator
session or the generated Console password. To retrieve the latter for a resource
named `wandb`:

```bash
kubectl get secret wandb-watchtower-auth -n wandb \
  -o jsonpath='{.data.password}' | base64 -d
```

Use the password at `/console/login`. It allows access when the W&B application
is unavailable. Console is brought up before infrastructure readiness, although
it still requires a usable server manifest, networking, and cluster credentials.
Console readiness does not make the W&B deployment ready.

## Rotate the generated password

The password is created once and retained across upgrades. Delete its Secret,
wait for Operator to recreate it, and then restart the Console Deployment so it
loads the new value:

```bash
kubectl delete secret wandb-watchtower-auth -n wandb
kubectl get secret wandb-watchtower-auth -n wandb
```

Repeat the read until the Secret exists, then run:

```bash
kubectl rollout restart deployment/wandb-watchtower -n wandb
kubectl rollout status deployment/wandb-watchtower -n wandb
```

Names are derived from the W&B resource; substitute the names for your deployment.
For very long CR names, inspect the generated resources rather than assuming the
unshortened name.

## Permissions and troubleshooting

| Symptom | Check |
| --- | --- |
| Console pod is absent | `spec.adminConsoleEnabled` and Operator logs |
| Status URL is empty | `spec.wandb.hostname` |
| `/console` returns 404 | Routing mode, route creation, and any external Ingress rules |
| W&B login is rejected | The W&B account must be an administrator |
| Pod never becomes ready | ServiceAccount, RBAC, cluster client initialization, and logs |
| Operator cannot derive the authentication service | The server manifest must declare the `/oidc` application route |

The generated permissions do not grant CRD discovery or pod port-forwarding;
features depending on those permissions are limited. Installing/upgrading the
Operator from inside Console also requires permissions beyond the default grant.
Secret-writing and database administration are separately controlled Operator
deployment policies, described in [Console integration](../developer/console.md).

Setting `adminConsoleEnabled: false` removes the Console resources, including its
password Secret. Re-enabling it generates a new password.
