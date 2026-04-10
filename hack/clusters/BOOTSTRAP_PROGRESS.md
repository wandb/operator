# Bootstrap Progress

Tracking document for the cloud bootstrap script development.
Raw command output is in `hack/clusters/tmp/` (gitignored).

## Scripts

| Script | Status | Notes |
|--------|--------|-------|
| `cluster_state.sh` | DONE | Tested all 3 clouds, detects TF lock for in-progress applies |
| `cluster_kubeconfig.sh` | DONE | Tested all 3 clouds, context rename works |
| `cluster_setup.sh` | DONE | Steps 1-7 pass on all 3 clouds (with --skip-build) |
| `cluster_deploy.sh` | DONE | Steps 8-10 pass on EKS/GKE (AKS skipped — registry issue) |
| `cluster_status.sh` | DONE | 13/13 pass on EKS/GKE, telemetry detection fixed |

## Cloud Deployments

| Cloud | TF Applied | Cluster State | Setup | Deploy | Status Check |
|-------|-----------|---------------|-------|--------|-------------|
| EKS (`eks-tf`) | YES | READY (29 resources) | PASS (--skip-build) | PASS | 13 PASS |
| AKS (`aks-tf`) | YES | READY (9 resources) | PASS (--skip-build) | NOT RUN (registry issue) | 9 PASS, 2 FAIL, 2 SKIP |
| GKE (`gke-tf`) | YES | READY (15 resources) | PASS (--skip-build) | PASS | 13 PASS |

## Iteration Log — 2026-04-10 (Session 2)

### Changes Made

- `cluster_status.sh`: Added TF state check via `cluster_state.sh` before kubectl checks
- `cluster_status.sh`: Fixed telemetry detection — use Helm release secret instead of deploy labels
- `cluster_state.sh`: Detect `.terraform.tfstate.lock.info` to report PENDING during active applies
- `cluster_setup.sh`: Added `retry` helper for transient TLS errors
- `cluster_setup.sh`: Added Helm stuck release recovery (pending-upgrade/install/rollback)
- `cluster_setup.sh`: Skip `cluster_kubeconfig.sh` if context already exists (parallel safety)
- `cluster_setup.sh`: Per-context temp dirs for CRD extraction (parallel safety)
- `cluster_setup.sh`: Lockfiles for `make manifests generate` and `go build` (parallel safety)
- `cluster_setup.sh`: Serialized docker push via lockfile (concurrent pushes crash Docker daemon)
- `cluster_setup.sh`: Fixed Dockerfile — `USER 65532` for `runAsNonRoot` security context
- `cluster_setup.sh`: Fixed `GOARCH=amd64` for cross-compilation from Apple Silicon
- `cluster_setup.sh`: Fixed ACR/GKE registry URL — append `/operator` image name for non-ECR registries

### EKS

| Time | Action | Result |
|------|--------|--------|
| 17:00 | terraform apply | PASS (29 resources) |
| 17:10 | cluster_setup.sh (parallel, build) | Multiple failures: TLS, docker push network, runAsNonRoot |
| 17:45 | cluster_setup.sh --skip-build | PASS |
| 17:46 | cluster_deploy.sh --overlay size-small | PASS |
| 17:47 | cluster_status.sh | 13 PASS |

### AKS

| Time | Action | Result |
|------|--------|--------|
| 17:01 | terraform apply | PASS (9 resources) |
| 17:10 | cluster_setup.sh (parallel, build) | FAIL: ACR registry_url missing repo path, docker push failures |
| 17:30 | cluster_setup.sh --skip-build | PASS |
| — | cluster_deploy.sh | NOT RUN (registry/objectstore issue) |

### GKE

| Time | Action | Result |
|------|--------|--------|
| 17:00 | terraform apply | PASS (15 resources) |
| 17:10 | cluster_setup.sh (parallel, build) | Multiple failures: TLS, GKE Artifact Registry path format, docker push |
| 17:40 | cluster_setup.sh --skip-build | PASS |
| 17:46 | cluster_deploy.sh --overlay size-small | PASS |
| 17:47 | cluster_status.sh | 13 PASS |

## TF Changes Made

- [x] GKE: Truncate SA prefix to 24 chars (`sa_prefix` local)
- [x] GKE: Lowercase tags for GCP resource labels (`gcp_labels` local)
- [x] AKS: Set explicit `service_cidr=10.1.0.0/16` and `dns_service_ip=10.1.0.10`
- [x] All: Changed timestamp format from `YYYYMMDDhhmm` to `YYMMDDhhmm` (2-digit year)
- [x] All: Changed `tfvars.example` deployment names from `wandb-operator-*` to `wandb-op-*`
- [ ] EKS: Consider adding `registry_url` output combining `ecr_registry_host`/`ecr_repo_name`

## Discovered Issues

1. **GKE: SA ID length limit** — GCP service accounts limited to 30 chars. Fixed by truncating SA prefix.
2. **GKE: Label case sensitivity** — GCP resource labels must be all lowercase. Fixed by lowercasing.
3. **AKS: Service CIDR overlap** — Default `10.0.0.0/16` overlaps node subnet. Fixed with explicit CIDR.
4. **All clouds: TLS "bad record MAC"** — Transient on fresh clusters. Fixed with `retry` helper wrapping all kubectl/helm calls.
5. **Parallel execution unsafe** — Concurrent kubeconfig writes corrupt the file. Fixed with `--context` pinning on all kubectl/helm calls. Scripts should still be run sequentially.
6. **bash `((var++))` with `set -e`** — Incrementing from 0 returns exit code 1. Fixed with `var=$((var + 1))`.
7. **W&B CR uses Tilt-specific fields** — `manifestRepository: "file:///server-manifest"` is a local path from the Tilt dev workflow. Needs a cloud-appropriate base or overlay.
8. **ACR registry_url is just a hostname** — No repo path, so `$REGISTRY:tag` gets treated as a Docker Hub push. Fixed by appending `/operator` for non-ECR registries.
9. **GKE Artifact Registry requires IMAGE segment** — `HOST/PROJECT/REPO:tag` invalid, needs `HOST/PROJECT/REPO/IMAGE:tag`. Same fix as #8.
10. **Concurrent docker pushes crash Docker daemon** — Same cached layers pushed simultaneously cause `use of closed network connection`. Fixed with push lockfile.
11. **Dockerfile runs as root** — Deployment security context requires `runAsNonRoot`. Fixed with `USER 65532`.
12. **Cross-compilation missing GOARCH** — `go build` on Apple Silicon produces arm64 binary. Fixed by adding `GOARCH=amd64`.
13. **Helm stuck in pending state** — TLS error during `helm upgrade` leaves release as `pending-upgrade`. Fixed with pre-upgrade status check and automatic rollback.

## Open

- [ ] Revisit W&B CR construction for cloud (remove Tilt-specific fields)
- [ ] AKS: Fix registry/objectstore terraform and test cluster_deploy.sh
