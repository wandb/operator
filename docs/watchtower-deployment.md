# Deploying Watchtower

[Watchtower](https://github.com/wandb/watchtower) is the cluster administration UI
that replaces the deprecated W&B console. This document covers how it is packaged
and installed alongside the operator.

## Packaging: one image, two entrypoints

Watchtower is **not** a separate image to pull, mirror and version. Its binary is
copied into the operator image at build time and sits next to `/manager`:

```dockerfile
ARG WATCHTOWER_IMAGE=us-docker.pkg.dev/wandb-production/public/wandb/watchtower
ARG WATCHTOWER_VERSION=0.11.0
...
FROM ${WATCHTOWER_IMAGE}:${WATCHTOWER_VERSION} AS watchtower
...
COPY --from=watchtower /watchtower .
```

The binary is lifted from the published Watchtower image rather than rebuilt from
Go source, because it embeds a Next.js static export — building it here would mean
carrying the Watchtower frontend, a Node toolchain and a circular module
dependency (Watchtower already depends on `github.com/wandb/operator`).

Pin a different release with:

```bash
make docker-build WATCHTOWER_VERSION=0.12.0
```

Selecting which of the two binaries runs is the container's `command`: the image
`ENTRYPOINT` stays `/manager`, and the Watchtower Deployment overrides it with
`/watchtower`.

## Installation: a separate release

`deploy/watchtower/` is its own chart and its own Helm release. It is **not** a
dependency of `deploy/operator` — installing Watchtower does not install the
operator, upgrading one does not touch the other, and deleting one leaves the
other running. The only tie between them is the image.

```bash
helm install watchtower oci://us-docker.pkg.dev/wandb-production/charts/watchtower \
  --version 2.0.0-beta.3 -n wandb --create-namespace
```

Both charts are published from this repo by the same release workflow, and the
version check there holds the watchtower chart version, its appVersion, and the
operator image tag equal to the release. So `--version 2.0.0-beta.3` deploys the
Watchtower binary from operator image `2.0.0-beta.3` with no second version to
track — which is also why `image.tag` can be left empty and defaults to the
chart's appVersion.

It creates:

| Resource | Purpose |
|----------|---------|
| `Deployment` | The operator image run as `/watchtower --port 8080` |
| `Service` | `NodePort`, publishing the Go HTTP server directly |
| `ServiceAccount` | With its token projected — Watchtower calls the Kubernetes API |
| `Role` + `RoleBinding` | Write access to the W&B CRs and to secrets |
| `Secret` | The generated admin password (see Authentication below) |

A minimal values file:

```yaml
service:
  nodePort: 32080       # omit to let Kubernetes allocate one
role:
  type: ClusterRole     # Role scopes Watchtower to its own namespace
```

`replicas` is deliberately fixed at 1 and not exposed: in-flight deploy jobs and
their SSE streams live in the serving pod's memory, so a reconnect landing on a
second pod would see no history.

## Routing: a node port, not an Ingress

Watchtower's own Go HTTP server is the public entry point. There is no Ingress
path on the W&B hostname and no reverse proxy in front of it — the `NodePort`
Service publishes the port on every node, and reaching it from the public
internet is a matter of opening that port in the node firewall or security group.
Nothing in this chart opens it.

Set `service.type` to `ClusterIP` for an internal-only install, or to
`LoadBalancer` to get a dedicated address instead.

### The base path is a build-time value

`basePath` defaults to `/watchtower` and cannot be changed on its own. Next.js
bakes `basePath` into every asset URL and router href at build time, and the
Watchtower binary refuses to start when its runtime base path disagrees with the
compiled-in one — so serving at the root requires a Watchtower image built with
`BASE_PATH=` empty, and `watchtower.basePath: ""` to match. The published image
is built with `/watchtower`.

The health probes carry the prefix for the same reason: `{basePath}/healthz` and
`{basePath}/ready`, which sit outside the auth gate (the kubelet sends no cookie)
but inside the base path.

## RBAC

The chart's `Role` grants exactly what Watchtower needs to manage an install:

- `apps.wandb.com` — `weightsandbiases`, `applications` and their `/status`
  subresources, full verbs
- core `secrets`, full verbs

`role.type` defaults to `ClusterRole`, so Watchtower can manage installs in any
namespace. Set it to `Role` to confine it to its own release namespace.

Cluster-scoped names are qualified with the namespace —
`<namespace>-<release>-watchtower` — because `ClusterRole` and
`ClusterRoleBinding` names are cluster-global. Without that, a second Watchtower
release in another namespace would adopt the first one's object and silently
overwrite its rules and subject list. Namespaced `Role`s keep the plain name.

Because the container runs with `readOnlyRootFilesystem: true`, the Deployment
mounts `emptyDir`s at `/home/watchtower` (its working directory, where the
air-gapped dependency bundle lands), `/helm` and `/tmp`.

## Authentication: a chart-generated admin password

Watchtower implements no OIDC and does not share the W&B app's session. That
earlier design only worked because Watchtower was served under the app's
hostname, so the browser sent the app's cookie along; published on its own origin
it never arrives. A single admin password gates the UI instead.

The chart generates it on first install into a Secret named
`<release>-watchtower-auth` and injects it as `WATCHTOWER_PASSWORD` via
`secretKeyRef` — never inlined in the pod spec, where anyone with
`kubectl get deployment` could read it. Retrieve it with:

```bash
kubectl get secret -n wandb wandb-watchtower-auth \
  -o jsonpath='{.data.password}' | base64 -d
```

The template reads any existing Secret via `lookup` before generating, so
`helm upgrade` preserves the password rather than silently rotating it and
locking the admin out. Two overrides: `auth.existingSecret` to manage the Secret
yourself (preferred for GitOps — a generated password is invisible until someone
reads it), or `auth.password` to pin a value, which lands in the Helm release
history and is best avoided.

On the wire: `POST <basePath>/login` checks the password in constant time and
sets an `HttpOnly`, `SameSite=Lax` session cookie scoped to the base path,
holding an expiry signed with an HMAC keyed on the password itself. There is no
server-side session store — Watchtower is a single replica that restarts freely —
and because the key is derived from the password, rotating the Secret invalidates
every outstanding session for free. Sessions last 12 hours. `Secure` is set only
when the request arrived over TLS, since the Service publishes plain HTTP and an
unconditionally-Secure cookie would never be sent back.

Unauthenticated `/api/v1/*` calls get a JSON 401 so the frontend can render
"session expired"; page loads redirect to the login form. `/healthz` and `/ready`
stay outside the gate — the kubelet holds no session.

`mode` defaults to `cluster`, which is what turns the gate on. Setting it to
`web` disables authentication entirely; only do that against a sandbox you do not
care about.

### Still worth doing

The password is a shared secret with no rate limiting on the login endpoint. A
32-character generated password is not guessable, but a user-chosen
`auth.password` might be — consider a lockout or backoff before this is exposed
broadly, and keep the node port firewalled to known source ranges regardless.
