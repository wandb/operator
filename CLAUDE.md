# Claude Code Style Guidelines

This file contains coding style guidelines and preferences for this project when working with Claude Code.

## General Code Style

### Comments
- **Do NOT add inline comments that simply restate what the code does**
- Only add comments when they explain WHY something is done, not WHAT is being done
- Avoid comments like `# Create the kind cluster` before `kind create cluster --name "$CLUSTER_NAME"`
- Good comments explain business logic, non-obvious behavior, or important context

### Examples

**Bad:**
```bash
# Check if cluster exists
if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
```

**Good:**
```bash
if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
```

**Acceptable (when context is needed):**
```bash
# Use empty string fallback — grep returns nothing if the key is absent
KIND_CLUSTER_NAME=$(grep '"kindClusterName"' tilt-settings.star | sed 's/.*: *"\(.*\)".*/\1/' | head -1)
```

## Testing and Linting

- Always run `make lint` and `make test` before considering tasks complete
- Generate coverage reports with `make test` (produces `coverage.html`)

## Dependencies

- Scripts should explicitly check for required dependencies (like `jq`) and fail with clear error messages if missing
- Do not provide fallbacks for missing dependencies unless specifically requested