# Release

## Description

In this document, we'll go over on how to properly create a release, and push it.

## Creating a Release

1. Create a branch off `v2` -- it's irrelevant of what the branch name is.
2. Think of the tag you'd like to update to, and we'll continue to use **tag** to reference this. For example, `2.0.0-alpha.3`.
3. In the [operator values](../deploy/operator/values.yaml), update the following:
   1. The version at the top under `wandb:` is pointing to the latest server release. This server release should be the latest that was cut specifically for On-Prem.
   2. The version under `wandb-operator:` needs to update the tag version to the one desired on point 2.
4. In the [operator chart](../deploy/operator/Chart.yaml), update the `version` and `appVersion` to the desired tag.
5. Ensure the [chart repos](..deploy/ct.yaml) match to that of the [chart](..deploy/operator/Chart.yaml). If there's missing ones, please proceed to add them, or update it.
6. Alas, run `ct lint --config deploy/ct.yaml` from root directory of this repository to ensure things will pass
7. Create a PR and get manager approval to merge and create the release

## Pushing the Release

Ensure the PR created has been merged onto `v2` branch.

1. Run the following GitHub Action: [Internal Image Publish](https://github.com/wandb/operator/actions/workflows/internal-image-publish.yaml) off the `v2` branch and use the desired tag that was used in part 1.
2. Run the following GitHub Action: [Internal Chart Publish](https://github.com/wandb/operator/actions/workflows/internal-chart-publish.yaml) off the `v2` branch
