# Argo Rollouts API Vendored Code

This directory contains vendored API types from the [Argo Rollouts](https://github.com/argoproj/argo-rollouts) project.

## Source

- **Repository**: https://github.com/argoproj/argo-rollouts
- **Version**: v1.8.3
- **Date Vendored**: 2026-01-26

## Reason for Vendoring

To have full control over the CRD API types and avoid unexpected breaking changes when the upstream operator updates.
This allows the W&B operator to manage Argo Rollouts while controlling when and how we adopt upstream changes.

## Changes Made

### Updated
- Update import in `argoproj.io.rollouts/v1alpha/register.go` to refer to `argoproj.io.rollouts/register.go` to avoid
upstream dependancy

### Deleted Files

- generated.pb.go
- generated.proto
- openai_generated.go