# Releasing Operator v2

Stable and preview v2 releases are prepared through a reviewed pull request and
published from an annotated `v2.x.y` or `v2.x.y-<prerelease>` tag. A single
workflow publishes the operator image and Helm chart from the same commit, then
creates the matching GitHub Release.

## Tagged release

1. Open a release pull request against `main`.
2. Update `wandb.version` in `deploy/operator/values.yaml` to the intended W&B
   server release.
3. Set all three operator versions to the release number without a leading
   `v`:
   - `version` in `deploy/operator/Chart.yaml`
   - `appVersion` in `deploy/operator/Chart.yaml`
   - `wandb-operator.image.tag` in `deploy/operator/values.yaml`
4. Run the chart validation commands used by CI:

   ```bash
   helm dependency build deploy/operator
   ct lint --all --config deploy/ct.yaml
   helm lint --strict deploy/operator
   ```

5. Merge the pull request after all required checks and approvals pass.
6. Update the local branch and confirm it exactly matches the remote branch:

   ```bash
   git switch main
   git pull --ff-only origin main
   test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)"
   ```

7. Create and push an annotated release tag at that commit. Use a SemVer
   prerelease suffix such as `-beta.2` or `-rc.1` for previews:

   ```bash
   version=v2.0.0
   git tag -a "${version}" -m "Operator ${version}"
   git push origin "${version}"
   ```

8. Monitor the `Release v2` workflow. It publishes the versioned GAR image
   first, then the matching OCI Helm chart, and creates the GitHub Release only
   after both artifacts succeed. A tag with a prerelease suffix creates a
   GitHub prerelease.
9. Record the source commit, image digest, chart digest, and GitHub Release URL
   in the release record.

Tagged versions are immutable. Never move, delete, reuse, or overwrite a
`v2.x.y` or `v2.x.y-<prerelease>` tag or its matching image/chart tags. If a
release is incorrect or only partially publishes, fix it with a new version.
The release workflow does not publish a `latest` tag.

Every version offered to users must use the tagged release workflow so the OCI
chart version, operator image tag, and GitHub Release stay aligned. Operator v2
charts are published at
`oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator`; the
legacy `charts.wandb.ai` repository is not the Operator v2 release channel.

## Development artifacts

The `Internal Image Publish` workflow accepts only tags in the form
`dev-<name>-<7-to-40-character-sha>`, for example
`dev-bucket-proxy-1106901`. It cannot publish production-style tags.

The `Internal Chart Publish` workflow is for engineering-only chart validation.
It accepts only a development version already declared in
`deploy/operator/Chart.yaml`, such as `2.0.0-dev.1106901`. It rejects stable
versions and release prereleases such as `-beta.2` or `-rc.1`, and refuses to
overwrite an existing development chart tag. Do not offer these chart-only
builds to users; publish a tagged release instead.
