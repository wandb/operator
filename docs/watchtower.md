With wandb console being deprecated, users want a way to access a wandb deployed UI for managing their infrastructure. Today we have https://github.com/wandb/watchtower which can connect to any context and manage instance deploys there, but this access is too general and we want a wandb app for on-prem customers to manage their deploys the way they used to with console.

# How Console Was Set Up

In https://github.com/wandb/helm-charts/

- **In-cluster nginx** — `charts/operator-wandb/templates/nginx.yaml:33`

    ```
    location /console {  proxy_pass http://{{ .Release.Name }}-console:8082;}
    ```

- **Ingress path** — `charts/operator-wandb/templates/_ingress.tpl:190`

    ```yaml
    -pathType:Prefixpath:/consolebackend:service:name:{{$.Release.Name}}-consoleport:number:808
    ```


With the Deployment of console coming from https://github.com/wandb/deployments with the service named `wandb-console` (`wandb/console/values.yaml`) so that, with release name `wandb` the chart's `{{ .Release.Name }}-console` selector resolves to it.

In the console frontend itself, `next.config.ts` sets `basePath: "/console"` when `NODE_ENV === "production"`.

## Authentication

Console is/was not performing its own authorization or oidc validation. The OIDC login happens in the main W&B app on the same host. Console only asks Gorilla to validate the session cookie that the browser already carries. I believe if we piggyback watchtower off of the wandb app (which is the ask), we can use this same apporach that console was using.

The check is a plain HTTP sub-request (`~/console/src/server/api/routers/auth/util.ts:7`):

```tsx
exportconst APP_AUTH_CHECK_URL=`http://${env.AUTH_SERVICE}/oidc/auth`;
```

and (`:170`):

```tsx
exportconst isLoggedInWithApp= async(headers: Headers)=>{const res=await fetch(APP_AUTH_CHECK_URL,{ method:"GET", headers, signal});return res.ok;};
```

The incoming request headers — including `Cookie` — are forwarded verbatim. It is gated on every tRPC call through `protectedProcedure` (`~/console/src/server/api/trpc.ts:86`).

`AUTH_SERVICE` comes from the `wandb-console-configmap` rendered by the chart https://github.com/wandb/helm-charts/:  (`charts/operator-wandb/templates/console.yaml`):

```
AUTH_SERVICE: "{{ .Release.Name }}-api:8081"   # when global.localService.bypassAUTH_SERVICE: "{{ .Release.Name }}-app:8080"   # otherwise
```

Dedicated-cloud values set `global.localService.bypass: true` and `app.install: false` (`~/deployments/shared-tenant/charts/wandb/values/enabled-services.yaml`), so in practice it resolves to `wandb-api:8081`.

The endpoint on the other side is set up in Core `services/gorilla/api/handler/oidc.go:99` and does not require any changes for watchtower, just call `/oidc/auth` without `?admin=false`. The response also hands back the identity, which is useful for audit logging and for showing the current user in the UI.

Because of this design, watchtower will need to come from the same hostname as the wandb app but I’m pretty sure that’s what we want, so we’re good there. Please flag if that’s not the case.

Console also had a second, independent auth path intended for on-prem: a root password stored in the `{release}-password` Secret, verified server-side, issuing an ES512 JWT in a `wandb-console-auth` cookie (`util.ts`, `getPassword`/`generateToken`/ `decodeToken`). `isLoggedIn` accepted **either** path.

## Deployment

Console was deployed through ArgoCD/Control Plane which is also no longer the case.

// TODO  - Look into deployment through Orca?

Rollout policy for Console per `~/deployments/wandb/user-spec/README.md`: sandbox/QA deploy automatically on merge; production requires manual approval with a 2-hour gradual rollout.

# For Watchtower

Watchtower today is a single Go binary that embeds a Next.js static export (`backend/static/`, populated by `make frontend`) and drives clusters through the `wsm` library. Serving it from inside a cluster means four changes: a base path, an auth gate, a locked-down single-cluster mode, and a deployment pipeline.

```
browser ──► ingress / wandb-nginx ──► /watchtower ──► Service wandb-watchtower:8080                                                              │                                                              ├─ auth middleware ──► http://wandb-api:8081/oidc/auth                                                              └─ wsm/client-go ────► in-cluster Kubernetes API
```

---

## Base Path

**Frontend**

- `frontend/next.config.ts` — add `basePath` and `assetPrefix`, driven by an env var so dev and desktop builds stay at `/`. `trailingSlash: true` is already set.
- `frontend/lib/api.ts:46` — `const BASE = "/api/v1"` is the single entry point for every typed fetch wrapper (`request<T>()`).
- `frontend/lib/sse.ts:16` — `SSE_ORIGIN` needs to become base-path aware. SSE URLs are handed to the frontend by the backend as absolute paths (`/api/v1/deploy/{jobId}/stream`), so either the backend emits them prefixed or `useSSEStream` prefixes them. Pick one and keep it consistent; the backend is the better place since it already knows its own mount point.

**Backend**

- `backend/server/server.go:39-58` — wrap both the API router and the `http.FileServer` in `http.StripPrefix(basePath, …)`. Add a `BasePath` field to `server.Options` and a `-base-path` flag in `backend/main.go` (the desktop entrypoint keeps the default empty value).
- Add `GET /healthz` and `GET /ready`. **These do not exist today** — `backend/api/router.go` has no health endpoints. Console's probes hit `/console/api/healthz` and `/console/api/ready`; we need the equivalents for liveness/readiness/startup probes in the chart. `/healthz` should be a static 200 (process is up); `/ready` should additionally confirm the Kubernetes client initialized.

## Auth

middleware in `backend/api/router.go` mirroring `isLoggedInWithApp`:

1. Forward the incoming `Cookie` (and `Authorization`, for API-token callers) to `GET http://$AUTH_SERVICE/oidc/auth`, with a short timeout (2-3s)
2. `200` → allow, and stash `X-Wandb-User-Email` on the request context for logging.
3. `401/403` → for HTML navigations, redirect to the app login with `?redirect_to=/watchtower`; for `/api/v1/*` calls, return `401` so the frontend can surface a clean "session expired" state instead of rendering a broken page.

## RBAC

Use Console's `role:` block in `~/deployments/wandb/console/values.yaml` as the starting point.

Watchtower needs, on top of it: `apps.wandb.com` `weightsandbiases` at v2 with `get/list/watch` , `update/patch` , `apiextensions.k8s.io` `customresourcedefinitions` `get/list` (used to detect whether v2 is served), and `pods/portforward` for telemetry port-forward.

## **Packaging, Deployment and Routing**

The existing `Dockerfile` will not build in CI. It copies `go.work` / `go.work.sum` and relies on the workspace's `replace` directives pointing at `../operator` and `../wsm`, which do not exist in the build context 

Convert it to the vendored path the rest of the repo uses:

```docker
ENV GOWORK=offRUN go build-mod=vendor-ldflags"-X .../backend/version.Version=${VERSION}"-o/watchtower./backend/...
```

It should also `EXPOSE` the port the chart expects and default `--base-path` from env.

`release.yml` currently produces desktop artifacts only. Add a job that builds and pushes `wandb/watchtower:<tag>` 

Add `/watchtower` to the `operator-wandb` chart. A `location /watchtower` in `templates/nginx.yaml` and a path in `_ingress.tpl`, both gated on a `watchtower.install` value. Consistent with how `/console` works, and correct regardless of whether traffic enters via ingress or the internal nginx.

The chart route above applies to **v1 / `operator-wandb` only**. Under the v2
operator there is no in-cluster nginx and no chart in the request path: the
operator publishes `/watchtower` itself. See below.

# Operator-side implementation (v2)

Watchtower is deployed by `wandb/operator` as an **operator-owned component**. It
is not published in the server manifest, so it cannot ride the manifest-driven
`reconcileApplications` path the W&B applications use; instead the operator
synthesises its `Application` directly, the same way managed Kafka/etcd do.

## Config surface

```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
spec:
  watchtower:
    install: true                 # opt-in; defaults to false
    image:
      repository: us-docker.pkg.dev/wandb-production/public/wandb/watchtower
      tag: 0.11.0                 # digest wins over tag when both are set
    basePath: /watchtower         # published route and container base path
    authService: ""               # empty = derive from the server manifest
    resources: {}                 # defaults to 100m / 256Mi requests
    serviceAccount:
      create: true
      serviceAccountName: wandb-watchtower
```

`install` defaults to **false**: Watchtower grants cluster-wide read access to
anyone holding a W&B session, so turning it on is an explicit decision.

## What the operator creates

| Resource | Name | Notes |
|----------|------|-------|
| `Application` | `wandb-watchtower` | `Kind: Deployment`, `replicas: 1`, labelled `weightsandbiases.apps.wandb.com/component=watchtower` so manifest-driven pruning skips it |
| `Service` | `wandb-watchtower` | ClusterIP `8080`, derived from the Application by the application controller |
| `ServiceAccount` | `wandb-watchtower` | Token automounted — unlike the W&B app pods, Watchtower calls the Kubernetes API |
| `Role` / `RoleBinding` | `wandb-watchtower` | Namespaced reads: secrets, configmaps, jobs, ingresses |
| `ClusterRole` / `ClusterRoleBinding` | `<namespace>-<cr-name>-watchtower` | Cluster-wide reads plus `weightsandbiases` `update/patch` |
| Ingress path | `/watchtower` on the consolidated Ingress | Added in `reconcileConsolidatedIngress` |
| `HTTPRoute` | via `Application.spec.httpRouteTemplate` | Gateway API mode only, same hostnames as the app |

`replicas` is deliberately **not** configurable: in-flight deploy jobs and their
SSE streams live in the serving pod's memory, so a reconnect landing on a second
pod would see no history.

Reconciliation runs **before** the infra-readiness gate in
`ReconcileWandbManifest`, so Watchtower comes up even when the install it is
meant to diagnose is stuck.

Status lands in `status.watchtowerStatus` (`ready`, `url`, `image`,
`authService`). Watchtower never gates the CR's `Ready` condition.

## Container contract

The operator passes the deployment-specific bits as env vars, so nothing has to
be baked into the image:

| Env var | Value | Purpose |
|---------|-------|---------|
| `WATCHTOWER_MODE` | `cluster` | Locks the UI to its own cluster: no context selection, no teardown |
| `WATCHTOWER_BASE_PATH` | `spec.watchtower.basePath` | Backend `StripPrefix` mount and frontend `basePath`/`assetPrefix` |
| `WATCHTOWER_AUTH_SERVICE` | e.g. `api:8080` | Host:port for the `GET /oidc/auth` sub-request |
| `WATCHTOWER_WANDB_NAME` | CR name | Which install this Watchtower manages |
| `WATCHTOWER_NAMESPACE` | fieldRef | Namespace of that install |

`WATCHTOWER_MODE` already exists in the Watchtower repo
(`backend/api/status/handler.go:27`, which checks for `desktop`); the other four
are new and still need implementing there.

Probes are `GET {basePath}/healthz` (liveness) and `GET {basePath}/ready`
(readiness) on port 8080 — they go through the base path because the server
mounts every route, health included, behind it. **Neither endpoint exists in the
Watchtower repo yet**, so until Phase 1 lands there the pods will never go ready.

## Deriving `authService`

`spec.watchtower.authService` is normally left empty. The operator finds the
manifest application that owns the `/oidc` ingress path — `api` in current
manifests — and uses `<application name>:<ingress service port>`, since the
application controller names each Service after its Application. That keeps
working across manifest renames and port changes.

If no application declares `/oidc`, reconciliation **fails** rather than deploying
an unauthenticated Watchtower.

## Known RBAC gap

The apiserver refuses to grant permissions the operator does not itself hold, so
two rules from the list above are **missing** from the ClusterRole the operator
creates:

- `apiextensions.k8s.io/customresourcedefinitions` `get/list` — used to detect
  whether v2 is served
- `pods/portforward` — telemetry port-forward

Both need the operator's own ClusterRole widened first (kubebuilder markers in
`internal/controller/weightsandbiases_controller.go`, then `make manifests`).
Installing or upgrading the operator from inside the pod needs more again, and is
not granted today.

# Open Questions/Notes

- How to test a deploy for this? Watchtower is only operator v2 compatible, do we have dedicated instances on operator v2?
- How to deploy with Orca?
- Need to pin replicas: 1 so that reconnects to land in a pod that has never seen a run and has no history
- Probably want to remove some functionality from Watchtower for this deployment like context selecting and `teardown`
- The Watchtower service is going to need greater permissions to install/upgrade the operator from inside the pods

### Notes from Claude

## **Suggested phasing**

| **Phase** | **Scope** | **Rough size** |
| --- | --- | --- |
| 1 | `--base-path` (frontend + backend), `/healthz` + `/ready`, vendored `Dockerfile`, dev-only CORS | 2–3 days |
| 2 | `/oidc/auth` middleware, `AUTH_SERVICE` wiring, login redirect, verdict cache, fail-closed | 2–3 days |
| 3 | `Mode: "cluster"`, router allowlist, frontend gating, in-cluster config test coverage | 4–5 days |
| 4 | Role/ClusterRole, `wandb-base` chart, ArgoCD app, Ctrlplane deployment, releaser workflow, ingress routing | 4–5 days |

## **Reference index**

| **Thing** | **Location** |
| --- | --- |
| Console auth sub-request | `~/console/src/server/api/routers/auth/util.ts:7`, `:170` |
| Console auth gate | `~/console/src/server/api/trpc.ts:86` |
| Console base path | `~/console/next.config.ts` |
| Console health routes | `~/console/src/app/api/healthz/route.ts`, `.../ready/route.ts` |
| Gorilla sub-auth handler | `~/core/services/gorilla/api/handler/oidc.go:99`, `:590` |
| nginx `/console` route | `~/helm-charts/charts/operator-wandb/templates/nginx.yaml:33` |
| ingress `/console` path | `~/helm-charts/charts/operator-wandb/templates/_ingress.tpl:190` |
| `AUTH_SERVICE` ConfigMap | `~/helm-charts/charts/operator-wandb/templates/console.yaml` |
| Console chart + Argo app | `~/deployments/wandb/console/` |
| Ctrlplane deployment def | `~/deployments/systems-terraform/modules/wandb/deployments.tf:20` |
| Console releaser workflow | `~/deployments/.github/workflows/wandb-console-releaser.yaml` |
| Spec-ownership warning | `~/deployments/wandb/user-spec/README.md` |
| Dedicated-cloud service toggles | `~/deployments/shared-tenant/charts/wandb/values/enabled-services.yaml` |
| wsm in-cluster fallback | `vendor/github.com/wandb/wsm/pkg/kubectl/kubectl.go:100` |
| Watchtower router mounts | `backend/api/router.go` |
| Watchtower mode flag | `backend/types/api.go:117`, `backend/api/status/handler.go:27` |
| Watchtower manifest stub | `deploy/k8s/` |