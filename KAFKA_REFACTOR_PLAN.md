# Kafka Implementation Refactor Analysis

## Executive Summary

The current Kafka implementation uses an **old pattern** with separate dev/HA handlers and inline configuration. This analysis details the gaps between the current Kafka implementation and the new Redis pattern, enabling a structured refactor to align Kafka with the modernized architecture.

---

## 1. Current Kafka Implementation

### File Locations

| File | Lines | Purpose |
|------|-------|---------|
| `internal/controller/wandb_v2/kafkaOp.go` | 826 | Main dev Kafka handler + backup logic |
| `internal/controller/wandb_v2/kafkaHaOp.go` | 192 | HA-specific handler |
| `internal/vendored/strimzi-kafka/v1beta2/kafka_types.go` | 200+ | Strimzi Kafka CRD types |
| `internal/canary/kafka.go` | 40 | Health check test using Sarama |
| `weightsandbiases_v2_controller.go` | 144-151 | Controller integration (conditional routing) |

### API Specification

```go
// From api/v2/weightsandbiases_types.go
type WBKafkaSpec struct {
    Enabled     bool              // Enable/disable Kafka
    StorageSize string            // Default: "10Gi" (no validation)
    Replicas    int32             // Dev: 1, HA: 3 (hardcoded in handlers)
    Backup      WBKafkaBackupSpec // Backup configuration
    Namespace   string            // Target namespace (defaults to wandb NS)
}

type WBKafkaBackupSpec struct {
    Enabled        bool           // Only filesystem support
    StorageName    string
    StorageType    WBBackupStorageType
    Filesystem     *WBBackupFilesystemSpec
    TimeoutSeconds int
}

type WBKafkaStatus struct {
    Ready          bool
    State          WBStateType
    Details        []WBStatusDetail
    LastReconciled metav1.Time
    BackupStatus   WBBackupStatus
}
```

### Strimzi Resources Created

| Resource | Name | Dev Replicas | HA Replicas | Notes |
|----------|------|--------------|-------------|-------|
| Kafka CR | `wandb-kafka` | 0 (KRaft) | 0 (KRaft) | Replicas managed by NodePool |
| KafkaNodePool | `wandb-kafka-pool` | 1 | 3 | Broker+Controller combined |
| Secret | `wandb-kafka-connection` | 1 | 1 | Contains KAFKA_BOOTSTRAP_SERVERS |

### Kafka Configuration Matrix

#### Dev Deployment
- **Replicas**: 1
- **Storage**: "10Gi" (default, overridable)
- **Replication Factor**: 1
- **Min ISR**: 1
- **Listener**: plain (9092)

#### HA Deployment
- **Replicas**: 3 (hardcoded)
- **Storage**: "10Gi" (default, overridable)
- **Replication Factor**: 3
- **Min ISR**: 2
- **Listener**: plain (9092)

#### Both Deployments
- **Kafka Version**: "4.1.0" (hardcoded)
- **Metadata Version**: "4.1-IV0"
- **Listeners**: plain (9092), tls (9093)
- **Storage Type**: JBOD with single volume
- **DeleteClaim**: true

---

## 2. Current Implementation Pattern Analysis

### High-Level Flow

```
handleKafka() OR handleKafkaHA()
├── getActualKafka()               # Fetch Kafka CR, NodePool, Secret
├── maybeHandleDeletion()          # Handle finalizer & backup on deletion
├── getDesiredKafka()              # Build desired state (inline config)
├── computeKafkaReconcileDrift()   # Compare actual vs desired
└── reconciliation.Execute()       # Apply changes (Create/Delete/StatusUpdate)
```

### Wrapper Types

```go
type wandbKafkaWrapper struct {
    kafkaInstalled    bool
    nodePoolInstalled bool
    kafkaObj          *strimziv1beta2.Kafka
    nodePoolObj       *strimziv1beta2.KafkaNodePool
    secretInstalled   bool
    secret            *corev1.Secret
}

type wandbKafkaDoReconcile interface {
    Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState
}
```

### Action Types

- `wandbNodePoolCreate` / `wandbNodePoolDelete` - NodePool lifecycle
- `wandbKafkaCreate` / `wandbKafkaDelete` - Kafka CR lifecycle
- `wandbKafkaConnInfoCreate` / `wandbKafkaConnInfoDelete` - Connection secret
- `wandbKafkaStatusUpdate` - Status tracking
- `KafkaBackupExecutor` - Backup on deletion (placeholder implementation)

### Key Code Sections

| Function | Lines | Purpose |
|----------|-------|---------|
| `getActualKafka()` | 154-204 | Fetch 3 resources (Kafka, NodePool, Secret) |
| `getDesiredKafka()` | 206-346 | Build Strimzi CRs with hardcoded config |
| `computeKafkaReconcileDrift()` | 348-409 | 6-way comparison logic |
| `maybeHandleDeletion()` | 539-631 | Finalizer + backup handling |
| `handleKafkaBackup()` | 633-711 | Placeholder backup executor |
| `EnsureKafkaBackup()` | 736-760 | Returns hardcoded success |

---

## 3. Redis Pattern (Target Reference)

### Model Layer Structure

```
internal/model/
├── redis.go                         # RedisConfig type
├── defaults/
│   └── redis.go                     # Kafka(profile WBSize) function
├── config.go                        # InfraConfigBuilder
└── status.go                        # Status extraction logic
```

### Controller Layer

```
internal/controller/
├── infra/redis/
│   ├── interfaces.go                # Redis interface definitions
│   └── opstree/
│       ├── actual.go                # Fetch actual state
│       ├── desired.go               # Build desired state
│       ├── create.go                # Creation logic
│       ├── update.go                # Update logic
│       ├── delete.go                # Deletion logic
│       └── values.go                # Utility functions
└── wandb_v2/
    ├── redis.go                     # reconcileRedis()
    ├── weightsandbiases_v2_controller.go (updated)
```

### Controller Integration Pattern

```go
// Old Kafka Pattern
if wandb.Spec.Size == apiv2.WBSizeDev {
    ctrlState = r.handleKafka(ctx, wandb, req)
} else {
    ctrlState = r.handleKafkaHA(ctx, wandb, req)
}

// New Redis Pattern (target for Kafka)
infraConfig := model.BuildInfraConfig().
    AddRedisSpec(&(wandb.Spec.Redis), wandb.Spec.Size)

result := r.reconcileRedis(ctx, infraConfig, wandb)
```

### Key Abstractions

1. **InfraConfigBuilder**: Builder pattern for infrastructure config
2. **RedisConfig**: Typed configuration struct
3. **defaults.Redis()**: Profile-based sizing
4. **ActualRedis interface**: Implementation-agnostic actual state
5. **model.Results**: Unified result aggregation
6. **Status extraction**: `ExtractRedisStatus()` method

---

## 4. Critical Differences

| Aspect | Current Kafka | New Redis Pattern |
|--------|---------------|------------------|
| **Defaults** | Hardcoded in handler | `defaults.Kafka()` function |
| **Config Model** | None | `model.KafkaConfig` struct |
| **Dev/HA Routing** | Separate handlers | Unified, config-driven |
| **State Fetch** | Inline in handler | `infra/kafka/opstree/actual.go` |
| **Desired State** | Inline in handler | `infra/kafka/opstree/desired.go` |
| **Status Update** | Action types + inline | `model.ExtractKafkaStatus()` |
| **Resource Constraints** | None defined | Defined in defaults |
| **Backup Handling** | Finalizer + executor | Infrastructure layer |
| **Error Handling** | Inline errors | `model.KafkaError` types |

---

## 5. Current Implementation Issues

### Missing Components

- [ ] No Kafka defaults model (`internal/model/defaults/kafka.go`)
- [ ] No Kafka config type (`internal/model/kafka.go`)
- [ ] No infrastructure abstraction layer
- [ ] No resource constraints (CPU/memory)
- [ ] No centralized Kafka error codes
- [ ] Backup executor is placeholder only

### Design Issues

- Hardcoded config in `getDesiredKafka()` function
- Dev/HA split duplicates business logic
- Status updates embedded in action types
- Finalizer logic couples backup to deletion
- No model-layer validation or defaults

### Maintainability Issues

- 826 lines in single file difficult to test
- Configuration scattered across handler
- Backup logic unreachable from tests
- Status inference not reusable
- New features require handler changes

---

## 6. Refactor Roadmap

### Phase 1: Model Layer (Days 1-2)

1. **Create Kafka defaults** (`internal/model/defaults/kafka.go`)
   ```go
   func Kafka(profile v2.WBSize) (v2.WBKafkaSpec, error)
   ```

2. **Create Kafka config** (`internal/model/kafka.go`)
   ```go
   type KafkaConfig struct {
       Enabled     bool
       Namespace   string
       StorageSize resource.Quantity
       Version     string
       Replicas    int32
       // ... more fields
   }
   ```

3. **Update InfraConfig** (`internal/model/config.go`)
   ```go
   func (i *InfraConfigBuilder) AddKafkaSpec(
       actual *apiv2.WBKafkaSpec, 
       size apiv2.WBSize,
   ) *InfraConfigBuilder
   
   func (i *InfraConfigBuilder) GetKafkaConfig() (KafkaConfig, error)
   ```

### Phase 2: Infrastructure Layer (Days 2-4)

1. **Create interfaces** (`internal/controller/infra/kafka/interfaces.go`)
2. **Implement actual state** (`opstree/actual.go`)
3. **Implement desired state** (`opstree/desired.go`)
4. **Implement CRUD** (`opstree/{create,update,delete}.go`)
5. **Utilities** (`opstree/values.go`)

### Phase 3: Status & Errors (Day 4)

1. **Define Kafka error codes** (in `internal/model/kafka.go`)
2. **Implement status extraction** (add to `model/status.go`)
3. **Create Kafka status codes** (model layer)

### Phase 4: Controller Updates (Day 5)

1. **Create `reconcileKafka()`** (`internal/controller/wandb_v2/kafka.go`)
2. **Update main controller** (`weightsandbiases_v2_controller.go`)
3. **Remove old handlers** (deprecate `kafkaOp.go`, `kafkaHaOp.go`)

### Phase 5: Testing & Validation (Day 5-6)

1. **Unit tests** for Kafka model layer
2. **Integration tests** for infrastructure layer
3. **E2E tests** for controller
4. **Backup logic** completion (if needed)

---

## 7. File Structure After Refactor

```
internal/
├── model/
│   ├── kafka.go                     # NEW: KafkaConfig, KafkaError, KafkaStatus
│   ├── defaults/
│   │   └── kafka.go                 # NEW: Kafka(profile) function
│   ├── config.go                    # UPDATED: AddKafkaSpec, GetKafkaConfig
│   └── status.go                    # UPDATED: ExtractKafkaStatus()
├── controller/
│   ├── infra/
│   │   └── kafka/                   # NEW directory
│   │       ├── interfaces.go        # NEW: Kafka interface defs
│   │       └── opstree/             # NEW: implementation
│   │           ├── actual.go        # NEW
│   │           ├── desired.go       # NEW
│   │           ├── create.go        # NEW
│   │           ├── update.go        # NEW
│   │           ├── delete.go        # NEW (if needed)
│   │           └── values.go        # NEW
│   └── wandb_v2/
│       ├── kafka.go                 # NEW: reconcileKafka()
│       ├── kafkaOp.go               # DELETE/DEPRECATE
│       ├── kafkaHaOp.go             # DELETE/DEPRECATE
│       └── weightsandbiases_v2_controller.go # UPDATED
```

---

## 8. Implementation Checklist

### Defaults & Config Model
- [ ] Create `defaults/kafka.go` with sizing logic
- [ ] Define `KafkaConfig` struct
- [ ] Update `InfraConfigBuilder` with Kafka support
- [ ] Add validation for storage/replicas
- [ ] Add defaults for version/listeners

### Infrastructure Layer
- [ ] Create `infra/kafka/interfaces.go`
- [ ] Implement `Initialize()` for state fetch
- [ ] Implement desired state builder
- [ ] Implement create/update/delete operations
- [ ] Handle NodePool + Kafka CR coordination
- [ ] Implement connection secret management

### Status & Errors
- [ ] Define Kafka error codes (DeploymentConflict, etc.)
- [ ] Define Kafka status codes (Created, Ready, etc.)
- [ ] Implement `ExtractKafkaStatus()` function
- [ ] Handle Ready state inference

### Controller
- [ ] Create `reconcileKafka()` function
- [ ] Update main controller routing
- [ ] Remove old handler calls
- [ ] Integrate with model layer
- [ ] Validate status updates

### Testing
- [ ] Unit tests for defaults
- [ ] Unit tests for config merging
- [ ] Integration tests for opstree layer
- [ ] E2E tests for full reconciliation
- [ ] Backup logic tests (if applicable)

### Documentation
- [ ] Update CLAUDE.md with Kafka patterns
- [ ] Document Kafka sizing rationale
- [ ] Add code examples to defaults
- [ ] Document error handling flow

---

## 9. Key Code Examples

### Current State (kafkaOp.go)
```go
func getDesiredKafka(...) (wandbKafkaWrapper, error) {
    // ... 140 lines of hardcoded config ...
    replicas := wandb.Spec.Kafka.Replicas
    if replicas == 0 {
        replicas = 1
    }
    kafka := &strimziv1beta2.Kafka{
        // ... manually constructed ...
        Config: map[string]string{
            "offsets.topic.replication.factor": "1",
            // ...
        },
    }
    // ... more manual construction ...
}
```

### Target State (after refactor)
```go
// defaults/kafka.go
func Kafka(profile v2.WBSize) (v2.WBKafkaSpec, error) {
    var spec v2.WBKafkaSpec
    switch profile {
    case v2.WBSizeDev:
        spec = v2.WBKafkaSpec{
            Replicas:    1,
            StorageSize: DevStorageRequest,
            // ...
        }
    case v2.WBSizeSmall:
        spec = v2.WBKafkaSpec{
            Replicas:    3,
            StorageSize: SmallStorageRequest,
            // ...
        }
    }
    return spec, nil
}

// opstree/desired.go
func (a *opstreeKafka) desiredKafka(...) *strimziv1beta2.Kafka {
    return constructKafka(a.config)
}

// wandb_v2/kafka.go
func (r *WeightsAndBiasesV2Reconciler) reconcileKafka(...) *model.Results {
    infraConfig := model.BuildInfraConfig().
        AddKafkaSpec(&(wandb.Spec.Kafka), wandb.Spec.Size)
    
    kafkaConfig, err := infraConfig.GetKafkaConfig()
    // ...
    
    actual, err := opstree.Initialize(ctx, r.Client, kafkaConfig, ...)
    result := actual.Upsert(ctx, kafkaConfig)
    
    wandb.Status.KafkaStatus = result.ExtractKafkaStatus(ctx)
    return result
}
```

---

## 10. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| **Breaking Changes** | Medium | Comprehensive tests before deploying |
| **State Migration** | Low | Kubernetes manages CRs independently |
| **Backup Compatibility** | High | Implement backup logic carefully |
| **Performance** | Low | Same operations, better organization |
| **User Experience** | Low | Status updates work identically |

---

## 11. Success Criteria

1. All Kafka functionality preserved and tested
2. Dev/HA routing unified via config
3. Consistent error/status handling with Redis
4. No duplication of business logic
5. Easily extensible for new profiles
6. Reduced cyclomatic complexity in handlers
7. Better testability of components
8. Alignment with operator architecture

---

## References

- Current Redis implementation: `internal/controller/wandb_v2/redis.go`
- Model layer: `internal/model/{config,redis,defaults/redis}.go`
- Infrastructure layer: `internal/controller/infra/redis/`
- API types: `api/v2/weightsandbiases_types.go`

