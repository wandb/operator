# Development Flow Guide

This guide explains what to execute when different things change during development to ensure changes are properly deployed.

## Understanding the Flow

The operator has several interconnected components:
1. **API Types** (`api/v1/`, `api/v2/`) - Go structs defining CRDs
2. **Generated CRDs** (`config/crd/bases/`) - Kubernetes YAML generated from Go types
3. **Kustomize Patches** (`config/crd/patches/`) - Modifications to generated CRDs
4. **Controller Code** (`controllers/`) - Reconciliation logic
5. **Manager Binary** (`tilt_bin/manager`) - Compiled operator

## Critical Issue: CRD Update Flow

**The Problem**: Tilt does NOT automatically regenerate CRDs when API types change. Here's why:

1. Line 76 of `Tiltfile` runs `local(manifests() + generate())` **once at startup**
2. Line 89 runs `local(manifests() + 'kustomize build config/default')` **once at startup**
3. Lines 109-110: The `Watch&Compile` local resource only watches for code changes and recompiles the binary
4. **CRDs are never regenerated when API types change during development**

## What to Execute When Things Change

### 1. API Types Changed (`api/v1/*_types.go` or `api/v2/*_types.go`)

**What needs to happen**:
- Regenerate Go deepcopy methods
- Regenerate CRD YAML files
- Rebuild and apply CRDs to cluster
- Recompile controller binary

**Manual Steps** (current workaround):
```bash
# Regenerate everything
make manifests generate

# Let Tilt detect the change and rebuild the binary
# OR manually trigger: tilt trigger Watch&Compile

# Apply updated CRDs manually
kubectl apply -f config/crd/bases/apps.wandb.com_weightsandbiases.yaml
kubectl apply -f config/crd/bases/apps.wandb.com_applications.yaml
```

**Better Approach** (restart Tilt):
```bash
# Stop Tilt (Ctrl+C)
# Restart Tilt
tilt up
```

### 2. Controller Logic Changed (`controllers/*.go`, `pkg/*.go`)

**What needs to happen**:
- Recompile controller binary
- Restart controller pod

**Steps**:
```bash
# Tilt automatically handles this via Watch&Compile local resource
# The live_update will sync the new binary and restart the container
# No manual action needed - just save the file
```

### 3. CRD Patches Changed (`config/crd/patches/*.yaml`)

**What needs to happen**:
- Rebuild kustomize output
- Apply updated CRDs to cluster

**Steps**:
```bash
# Stop Tilt (Ctrl+C)
# Restart Tilt to regenerate kustomize output
tilt up

# OR manually apply
make manifests
kubectl apply -f <(kustomize build config/crd)
```

### 4. Test CR Changed (`hack/testing-manifests/wandb/*.yaml`)

**What needs to happen**:
- Reapply the custom resource

**Steps**:
```bash
# Tilt automatically watches this file (via watch_settings)
# Just save the file and Tilt will reapply it
# No manual action needed
```

### 5. Kustomize Config Changed (`config/default/*`, `config/manager/*`)

**What needs to happen**:
- Rebuild kustomize output
- Reapply all resources

**Steps**:
```bash
# Stop Tilt (Ctrl+C)
# Restart Tilt
tilt up
```

## Recommended Tiltfile Improvements

The current Tiltfile has these limitations:

1. **CRDs not watched**: Changes to `api/` require manual `make manifests` or Tilt restart
2. **Kustomize not watched**: Changes to kustomize configs require Tilt restart
3. **One-time generation**: `local()` commands only run at Tilt startup

### Proposed Fix

Add a local resource to watch and regenerate CRDs:

```python
# Add this after line 110 in Tiltfile
local_resource('Watch&Regenerate-CRDs',
               manifests(),
               deps=['api'],
               ignore=['*/*/zz_generated.deepcopy.go'],
               auto_init=False,  # Don't run automatically, only on trigger
               trigger_mode=TRIGGER_MODE_MANUAL
)
```

Then manually trigger when API types change:
```bash
tilt trigger Watch&Regenerate-CRDs
```

### Alternative: Auto-regenerate CRDs

```python
# Replace line 76 with a local_resource
local_resource('Generate-CRDs',
               manifests() + generate(),
               deps=['api'],
               ignore=['*/*/zz_generated.deepcopy.go', 'config/crd/bases/*.yaml'],
               auto_init=True
)

# Note: This may cause Tilt to restart frequently during active API development
```

## Quick Reference

| What Changed | Automatic? | Manual Steps |
|-------------|-----------|--------------|
| `api/**/*_types.go` | ❌ No | `make manifests generate`, then restart Tilt or manually apply CRDs |
| `controllers/**/*.go` | ✅ Yes | Tilt auto-rebuilds and restarts |
| `pkg/**/*.go` | ✅ Yes | Tilt auto-rebuilds and restarts |
| `config/crd/patches/*.yaml` | ❌ No | Restart Tilt |
| `hack/testing-manifests/wandb/*.yaml` | ✅ Yes | Tilt auto-applies |
| `config/default/**/*.yaml` | ❌ No | Restart Tilt |

## Testing Changes

Always test before committing:

```bash
# Run linter
make lint

# Run unit tests
make test

# View coverage
open coverage.html

# Test in cluster (Tilt should be running)
# 1. Make changes
# 2. Wait for Tilt to rebuild (or trigger manually)
# 3. Check logs in Tilt UI
# 4. Verify CR status: kubectl get wandb -o yaml
```

## Common Issues

### Issue: "CRD changes not applied"
**Cause**: Tilt doesn't regenerate CRDs automatically
**Fix**: Run `make manifests generate` and restart Tilt, or manually apply CRDs

### Issue: "Controller crashes with 'field not found' error"
**Cause**: CRD not updated but controller code expects new fields
**Fix**: Ensure CRDs are regenerated and applied before controller restarts

### Issue: "kubectl shows old CRD version"
**Cause**: CRD not reapplied to cluster
**Fix**: `kubectl apply -f config/crd/bases/apps.wandb.com_*.yaml`

### Issue: "Changes to v2 API not taking effect"
**Cause**: Kustomize patch may be disabling v2 or `wandbCrName` setting wrong
**Fix**: Check `config/crd/patches/use_v2.yaml` and `tilt-settings.json`

## Full Clean Rebuild

When in doubt:

```bash
# Stop Tilt
# Ctrl+C in Tilt terminal

# Clean everything
make clean  # if this target exists
rm -rf bin/ tilt_bin/

# Regenerate
make manifests generate

# Rebuild and redeploy
tilt up
```