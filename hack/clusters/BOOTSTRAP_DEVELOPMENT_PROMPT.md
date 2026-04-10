# Bootstrap Development Prompt

Operating instructions for implementing the bootstrap scripts described in
`BOOTSTRAP_PLAN.md`. This document is the driver — follow it when working on bootstrap
implementation.

The session runs with `--dangerously-skip-permissions`. I have direct access to run
scripts, kubectl, helm, docker, and cloud CLIs against live clusters. The user handles
`terraform apply` and `terraform destroy` only.

## Scripts

| Script | Purpose |
|--------|---------|
| `cluster_state.sh` | Check TF state: EMPTY/PENDING/READY/ERROR |
| `cluster_kubeconfig.sh` | Fetch kubeconfig, rename context to cluster_name |
| `cluster_setup.sh` | Install cert-manager, CRDs, operators, controller, webhooks |
| `cluster_deploy.sh` | Apply W&B CR, external secrets, telemetry |
| `cluster_status.sh` | Health checklist with context name and pass/fail |

## Constraints

**What I do autonomously:**
- Write, edit, and run shell scripts
- Run cluster scripts against live clusters and capture output
- Run `kubectl`, `helm`, `docker` commands directly
- Run cloud CLIs (`aws`, `az`, `gcloud`) for non-destructive operations
- Validate scripts (`bash -n`, `shellcheck`, `terraform fmt`)
- Diagnose failures from command output — read logs, describe pods, check events
- Update `BOOTSTRAP_PROGRESS.md` with results and iteration log entries
- Update `BOOTSTRAP_PLAN.md` when I discover something that changes the plan

**What requires the user:**
- `terraform apply` — prompt with the exact directory and any tfvars notes
- `terraform destroy` — prompt with the exact directory
- Cloud credential renewal if auth expires mid-session
- Strategic/tactical decisions (see "When to Prompt the User")

## Logging

Raw command output goes in `hack/clusters/tmp/` (gitignored). One log file per session
or per cloud, with timestamps on every entry.

**Log file format:** `tmp/<cloud>-<date>.log` (e.g., `tmp/eks-20260410.log`)

**BOOTSTRAP_PROGRESS.md** stays concise — iteration log tables reference log files
by name and timestamp for deep-diving into specific failures.

## Development Loop

### Phase 1: Utility scripts (cluster_state.sh, cluster_kubeconfig.sh)

1. Write both scripts.
2. Validate locally with `bash -n` and `shellcheck`.
3. Ask the user which clouds have TF applied. If none, prompt them to apply one.
4. Run `cluster_state.sh` against each applied cloud. Fix issues.
5. Run `cluster_kubeconfig.sh` against each READY cloud. Verify with `kubectl get nodes`.

### Phase 2: cluster_setup.sh — iterative, one cloud at a time

1. **Pick the first cloud.** Ask the user which has a READY cluster.
2. **Run the full script.** Capture output. It will fail somewhere.
3. **Diagnose from output.** The `==>` log lines identify which step failed.
4. **Fix the script and re-run.** Steps are idempotent — re-runs skip completed steps.
5. **After first cloud works**, move to the second. Third should be nearly free.

### Phase 3: cluster_deploy.sh — W&B CR and telemetry

1. Requires `cluster_setup.sh` to have completed (checks webhook readiness).
2. Iterate on CR construction — the base CR may need cloud-specific adjustments.
3. Test overlay combinations (size-small, external-objectstore, etc.).

### Phase 4: cluster_status.sh

Write early — use as a diagnostic tool during Phase 2 and 3 iteration.

### Phase 5: Cross-cloud validation

Run the full sequence on all three clouds:
`cluster_setup.sh` → `cluster_deploy.sh` → `cluster_status.sh`

## When to Prompt the User

**Prompt for terraform operations:**
- "Please run `cd hack/clusters/eks-tf && terraform apply`"
- "Please run `cd hack/clusters/gke-tf && terraform destroy`"

**Prompt for discoveries that change the plan:**
- A deployment step that fundamentally can't work on a specific cloud
- A missing TF output or resource that we didn't anticipate
- A step ordering dependency that invalidates the plan

**Prompt for tactical/strategic decisions:**
- Performance tradeoffs (e.g., a step takes too long)
- Image compatibility questions
- Cloud-specific IAM/networking changes needed in TF

**Do NOT prompt when:**
- I can diagnose and fix from the output I already have
- I'm editing scripts between runs
- A transient error resolves on retry

## When to Abort

**Stop and prompt immediately if:**
- Cloud credentials are expired or insufficient
- TF state is corrupted or resources are orphaned
- A fundamental assumption in `BOOTSTRAP_PLAN.md` is wrong
- The operator itself is broken in a way unrelated to bootstrap

**Do NOT abort for:**
- Transient cloud API errors (retry once)
- Helm timeouts (increase timeout and retry)
- CRD conflicts (server-side apply + force-conflicts handles this)
- Pods not ready yet (wait longer, check events)

## Progress Tracking

Two files are maintained:

**`BOOTSTRAP_PROGRESS.md`** (checked in) — concise summary tables and iteration log.

**`tmp/<cloud>-<date>.log`** (gitignored) — raw command output with timestamps.

A script is WORKING when it succeeds on one cloud. It's DONE when all three clouds pass.

## Efficiency Notes

- **Run long waits in background.** Use `run_in_background` for kubectl wait / helm upgrade.
- **Read errors carefully before changing code.** Don't shotgun changes.
- **Keep scripts idempotent.** Every step checks "is this already done?" before doing work.
- **Use cluster_status.sh as the definition of "done."**
- **One cloud at a time.** Don't context-switch between clouds mid-debug.
- **Scripts must be run sequentially** per cloud (kubeconfig writes can't be parallelized).
