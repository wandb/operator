# Releasing Operator v2

Production v2 releases are prepared through a reviewed pull request and
published from an annotated `v2.x.y` tag. A single workflow publishes the
operator image and Helm chart from the same commit.

## Production release

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

7. Create and push an annotated release tag at that commit:

   ```bash
   version=v2.0.0
   git tag -a "${version}" -m "Operator ${version}"
   git push origin "${version}"
   ```

8. Monitor the `Release v2` workflow. It publishes the versioned GAR image
   first, then the matching OCI Helm chart, and creates the GitHub Release only
   after both artifacts succeed.
9. Record the source commit, image digest, chart digest, and GitHub Release URL
   in the release record.

Production versions are immutable. Never move, delete, reuse, or overwrite a
`v2.x.y` tag or its `2.x.y` image/chart tags. If a release is incorrect or only
partially publishes, fix it with a new patch version. The production workflow
does not publish a `latest` tag.

## Development artifacts

The `Internal Image Publish` workflow accepts only tags in the form
`dev-<name>-<7-to-40-character-sha>`, for example
`dev-bucket-proxy-1106901`. It cannot publish production-style tags.

The `Internal Chart Publish` workflow accepts the prerelease version already
declared in `deploy/operator/Chart.yaml`, such as `2.0.0-rc.1`. It rejects
stable versions and refuses to overwrite an existing prerelease chart tag.
