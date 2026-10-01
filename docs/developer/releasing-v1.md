# Releasing Operator v1

Operator v1 releases are prepared through a reviewed pull request and published
from an annotated `v1.x.y` tag. The release workflow never writes to the `v1`
branch.

1. Open a release pull request against `v1` containing the reviewed changelog
   entry and all intended release changes.
2. Merge the pull request after its required checks and approvals pass.
3. Update the local branch and confirm it exactly matches the remote branch:

   ```bash
   git switch v1
   git pull --ff-only origin v1
   test "$(git rev-parse HEAD)" = "$(git rev-parse origin/v1)"
   ```

4. Create and push an annotated release tag at that commit:

   ```bash
   version=v1.22.1
   git tag -a "${version}" -m "Operator ${version}"
   git push origin "${version}"
   ```

5. Monitor the `Release v1` GitHub Actions workflow. When it succeeds, record
   the Docker Hub, Quay.io, and GitHub Release URLs and image digests in the
   release record.
6. Never move, delete, or reuse an exact `v1.x.y` release tag. If a published
   release is incorrect, fix it with a new patch version.

The workflow publishes `1.x.y`, `1.x`, `1`, and `latest` to
`docker.io/wandb/controller`. It publishes `1.x.y`, `1.x`, and `1` to
`quay.io/wandb_tools/wandb-k8s-operator`; Quay.io does not receive a `latest`
tag.
