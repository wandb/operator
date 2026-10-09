# Launch agent for local testing

The optional test deployment uses one real W&B user for human login, queue
management, and the Launch agent. It works with the single-user installation.
The first user receives the normal server administrator role during onboarding.
The operator's production Helm chart and CRDs do not enable this test setup.

## With Tilt

Install `python3` (3.10+), `uv`, `kubectl`, and Helm. Add these settings to your
gitignored `tilt-settings.star`:

```python
"wandbHostname": "http://wandb.localhost:8080",
"enableCoreDNSRewrite": True,
"launchAgentEnabled": True,
"bootstrapUsername": "wandb-dev",
"bootstrapEmail": "wandb-dev@example.test",
```

The hostname must resolve locally to the forwarded gateway/ingress and from pods
to its Kubernetes Service. Artifact storage URLs must also be reachable from
both environments. Tilt already configures CoreDNS rewrites for W&B and the
optional external SeaweedFS endpoint. `localhost` cannot be used by Launch pods.

```bash
tilt up --context kind-wandb-operator
```

Tilt enables `spec.wandb.enableGlobalAdminAPIKey` and runs
`WandB-User-Bootstrap` after `Wandb-Endpoint`, followed by
`Launch-Prepare` and the separate `Launch-Agent` Helm release. Launch implies
user bootstrap; `bootstrapUserEnabled=True` can also enable user setup alone.
Each bootstrap invocation waits up to 600 seconds for the W&B CR's `Ready=True`
condition with `observedGeneration` matching the current CR generation. The
helper performs this check for both Tilt and standalone invocations. A timeout
or Kubernetes error stops it before user creation or credential changes.
Trigger `Launch-Smoke-Test` in Tilt to enqueue a CPU job that records a metric
and uploads/downloads an artifact. The test waits for the finished W&B run and
checks its summary, so a running agent pod alone does not count as success.

The upstream chart is pinned to `0.13.12`. The agent and smoke workload use the
amd64/arm64 `0.30.0` image, pinned by digest in
[`values.yaml`](../hack/testing-manifests/launch-agent/values.yaml).
`uv` runs the helper with an isolated `wandb==0.30.0` SDK environment.
There is one agent, one concurrent job, no sweep schedulers, and no image builder.
The integration targets Kind/Kubernetes; the upstream fixed UID is not suitable
for OpenShift's default restricted SCC.

## Credentials and retries

The operator generates a random 40-character hex key in
`<wandbName>-global-admin-api-key` (`key`) and reuses it on subsequent runs. The
server manifest binds generation and `GLOBAL_ADMIN_API_KEY` in the API and
Gorilla migration job to the CR flag. Bootstrap reads the Secret selector from
`status.generatedSecrets["global-admin-api-key"]` and validates ownership.
Migrations install the key for the server's built-in hidden service identity,
which does not consume another licensed user seat. The key stays in the W&B
namespace; it is never printed or passed to Launch.

Upgrade the operator and its CRDs before selecting a manifest with
`featureBindings`. Older operators ignore these new fields, and
`requiredOperatorVersion` is not currently enforced. The local test fixture
`0.85.0-dpanzella-server-manifest-test.1` includes the required wiring; manifests
without a binding to this CR flag cannot enable operator-managed bootstrap.

Bootstrap generates and stores the login password **before** creating the user.
After a failed login, it uses the deployment's admin key to verify that there
are no real users, then authenticates `PUT /oidc/users` with that key. It signs
in as the new user, completes onboarding, and generates that user's API key.
No built-in login password is used. Reruns first validate the
saved key. If it is revoked or missing, they sign in with the saved password and
generate a replacement for the same user. They never reset an existing user's
password or create a second user to recover from an authentication failure.

After saving the real user's credentials, bootstrap disables the built-in
`local@wandb.com` account through `PUT /oidc/users`, which also removes its password
record. This works around server initialization leaving that account active when
the global admin key is set, causing browsers to show first-user setup. Cleanup
uses the global admin key when available, otherwise the real user's API key
(which needs server administrator access if the local account is still active).
Reruns also perform this cleanup, including when reusing saved credentials;
an absent or already-disabled local account needs no change. Cleanup errors fail
bootstrap, and retrying reuses the saved credentials.

The source Secret is `<wandbName>-dev-credentials` in the W&B namespace, with
`username`, `email`, `loginPassword`, `loginPasswordVerified`, `apiKey`, and
identity/server metadata. A password is marked verified only after signing in
as the configured user; pending signup passwords are saved for retries but are
not displayed as login credentials.
The Launch namespace receives `wandb-api-key-<launchRelease>`; its `password`
field contains the API key, not the human login password.

Local bootstrap prints the login URL, username, email, password, and API key.
Set `bootstrapShowCredentials=False` to suppress automatic display. Retrieve
them later through the manual `WandB-Show-Credentials` Tilt resource or:

```bash
python3 hack/scripts/wandb_dev.py show-credentials --context kind-wandb-operator
```

When `CI` is set to a truthy value, both commands suppress credential output.
Other diagnostics omit authentication response bodies and Secret manifests.

For an existing installation, configure the existing user's email/username and
supply `WANDB_DEV_PASSWORD` or `WANDB_DEV_API_KEY` through the process environment.
With an API key alone, the helper cannot recover the existing login password and
discards any saved password that has not been verified, including passwords in
older Secrets without verification metadata. Supply `WANDB_DEV_PASSWORD` along
with the API key to verify and save an existing login password. Previously
verified passwords are retained on API-key-only reruns.
An existing saved Secret pins the server and user;
configuration mismatches fail rather than silently adopting another identity.
If that Secret is lost, supply the existing password/key to resume. A lost
response while generating a key may leave an unused key on the server; a retry
can generate a replacement but still uses the same user. Run one bootstrap
process per instance at a time, and avoid concurrent manual signup: the empty
installation check and user creation are separate server requests.

## Standalone commands

These commands assume the default namespace, release, username, queue, and
project from the example values. They also require a reachable W&B server and
working pod DNS; Helm does not bootstrap the W&B server itself.

For a fresh server, enable the flag before deploying with a supporting manifest
(Tilt applies it automatically):

```yaml
spec:
  wandb:
    enableGlobalAdminAPIKey: true
```

Remove any legacy `GLOBAL_ADMIN_API_KEY` environment override; the operator
rejects it when the manifest manages this setting. The old Tilt
`prepare-admin-key` command and `--admin-secret` option are no longer used.
Existing user credentials can be reused with the flag disabled. Once deployed:

```bash
python3 hack/scripts/wandb_dev.py bootstrap --context kind-wandb-operator --show-credentials
uv run hack/scripts/wandb_launch.py prepare --context kind-wandb-operator --values-output /tmp/launch-values.yaml
helm upgrade --install wandb-launch-test \
  https://github.com/wandb/helm-charts/releases/download/launch-agent-0.13.12/launch-agent-0.13.12.tgz \
  --kube-context kind-wandb-operator --namespace wandb-launch-test \
  -f /tmp/launch-values.yaml --wait --timeout 10m
uv run hack/scripts/wandb_launch.py smoke --context kind-wandb-operator
```

`prepare --values-output` writes non-secret Helm values for the configured
identity and queue, including credential/CA checksums so an upgrade rolls the
agent after rotation. For non-default settings, keep the helper flags and Helm
release/namespace aligned. `prepare` reuses an identical queue and fails
on conflicting configuration; it does not overwrite an existing queue.

For private HTTPS endpoints, bootstrap accepts `--ca-file <PEM>` or
`--ca-secret <Secret containing ca.crt>`. Tilt automatically selects its generated
root CA, or accepts `bootstrapCAFile`. Public roots and the custom CA are combined
and copied into a ConfigMap used by both the agent and queued job pods. The
helpers use the same bundle for their SDK calls. TLS verification remains enabled.

## Lifecycle and validation

The test agent can create Jobs, Pods, and Secrets only in its dedicated namespace.
The upstream chart creates a ClusterRole but binds it through namespace-scoped
RoleBindings. Workload pods have no Kubernetes API token. Jobs have a 600-second
deadline and are retained for 600 seconds after finishing for diagnostics.

`Dev-Clean` uninstalls matching Launch releases and deletes their labeled test
namespaces before tearing down the server. It then removes the saved user
credentials along with the development data; the CR owns its generated admin
Secret. Disabling Launch does not delete the W&B user, queue, or project. Use `Dev-Clean` for a fresh environment; when
using an external database, account deletion/reset is the database owner's task.

Local checks:

```bash
python3 -m unittest discover -s hack/scripts/tests -v
helm lint --strict /path/to/launch-agent-0.13.12.tgz \
  -f hack/testing-manifests/launch-agent/values.yaml
```

Chart CI runs the helper tests and checks the pinned upstream chart checksum,
lint, rendering, external credential handling, and absence of ClusterRoleBindings.

## Manifest feature bindings and key lifecycle

The manifest defines feature names and their CR field bindings:

```yaml
featureBindings:
  globalAdminAPIKey:
    field: spec.wandb.enableGlobalAdminAPIKey

generatedSecrets:
  - name: global-admin-api-key
    type: hex
    length: 40
    features: [globalAdminAPIKey]

commonEnvvars:
  gorillaGlobalAdmin:
    - name: GLOBAL_ADMIN_API_KEY
      features: [globalAdminAPIKey]
      sources:
        - name: global-admin-api-key
          type: generatedSecret
```

Only the API application and Gorilla migration reference `gorillaGlobalAdmin`
in `commonEnvs`. Binding paths start with `spec.` and resolve raw JSON values:
`true` enables; false, absent, or null disables; other types are errors.
Matching entries in the CR's `spec.wandb.features` map are ignored for bound
features. Unbound features keep their existing manifest defaults and CR
overrides. Declaring the same name in manifest `features` and `featureBindings`
is an error. Item-level `features` keeps its existing OR semantics.

The flag defaults to false. When disabled initially, no admin Secret is created
and neither workload receives the variable. Disabling after use retains the
Secret, removes its active status selector, and reruns Gorilla migrations to
revoke the database credential. Re-enabling reuses the same Secret. Removing an
environment variable from API pods alone would not revoke the database key.

Migration status records a hash of the version, resolved pod configuration
(excluding security contexts), and consumed generated credentials. Security
profile changes apply the next time a migration runs; they do not restart
completed migrations. Other changed inputs rerun only affected jobs, including
at the same server version. Active jobs finish before replacement;
foreground deletion ensures their pods are gone before a new job starts. API
readiness waits for the corresponding rollout. A valid generated-key change
also triggers migration and API rollout; invalid keys or foreign Secrets are
rejected without rotation. Hex lengths must be positive and even, measured in
encoded characters. Existing password Secret behavior is retained.

The generated key is intentionally stable. The previous assessment prototype
created a non-hex Secret because the old generator ignored `type`. A deployment
containing that unused prototype Secret must remove or explicitly repair it
before enabling this setting; the operator will not silently replace it.
