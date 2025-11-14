# MySQL Infrastructure Refactor Plan

## Session Context & Resume Instructions

**Document Purpose**: This document outlines the refactor plan for MySQL (currently called "Database") infrastructure to match the new Redis/Kafka pattern.

**Current State**: ✅ **Planning phase COMPLETE** - All checkpoints approved. Ready to begin implementation. NO code has been written yet.

**Key Context**:
1. ✅ Redis has been successfully refactored to new layered pattern
2. ✅ Kafka has been successfully refactored to new layered pattern
3. MySQL currently uses old pattern with size-based branching (`handlePerconaMysql` vs `handlePerconaMysqlHA`)
4. **Naming**: Will rename "database" → "mysql" throughout codebase for clarity
5. Uses Percona XtraDB Cluster operator
6. Learnings from Redis and Kafka will inform this refactor

**Important Notes**:
- Old files (mysqlOp.go, mysqlHaOp.go) should be **KEPT**, not deleted
- Controller code should **COMMENT OUT** old calls, not delete them
- This matches the Redis/Kafka refactor pattern

**Files Currently Exist**:
- ✅ `internal/controller/wandb_v2/redis.go` - NEW pattern (reference)
- ✅ `internal/controller/wandb_v2/kafka.go` - NEW pattern (reference)
- ✅ `internal/controller/wandb_v2/mysqlOp.go` - OLD pattern (will be kept)
- ✅ `internal/controller/wandb_v2/mysqlHaOp.go` - OLD pattern (will be kept)

**To Resume**:
1. Review checkpoints (Config, Defaults, Status, Functionality Loss, Implementation Order)
2. Provide feedback/confirmation on each checkpoint
3. Make any necessary adjustments to the plan
4. Once all checkpoints approved, begin implementation

---

## Overview

This document outlines the refactor plan to align **MySQL** infrastructure management with the new pattern established by Redis and Kafka.

### General Size-to-Topology Mapping

**IMPORTANT**: This mapping applies across ALL infrastructure components:

- **`dev` size** = **Standalone mode** (non-HA)
  - Single replica
  - No replication/redundancy
  - Suitable for development/testing only

- **`small` size** = **HA mode** (High Availability)
  - Multiple replicas (typically 3)
  - Replication enabled
  - Production-ready with fault tolerance

- **`large` size** = **HA mode** (High Availability, more resources)
  - Same topology as `small`
  - Differs primarily in resource allocation and storage

### Status
- ✅ **Redis**: Refactored (new pattern)
- ✅ **Kafka**: Refactored (new pattern)
- 🔄 **MySQL**: CURRENT FOCUS - Planning phase
- ⏳ **Minio**: Pending
- ⏳ **ClickHouse**: Pending

### Goals
1. Establish unified infrastructure reconciliation pattern
2. Rename "database" to "mysql" for clarity
3. Improve testability and maintainability
4. Create clear separation of concerns
5. Reduce code duplication

---

## Section 1: Understanding the Current MySQL Implementation

### 1.1 Current Naming (To Be Changed)

**API Types:**
- `WBDatabaseSpec` → Will rename to `WBMySQLSpec`
- `WBDatabaseStatus` → Will rename to `WBMySQLStatus`
- `WBDatabaseType` → Will rename to `WBMySQLType` (or remove if only Percona)
- Field: `wandb.Spec.Database` → `wandb.Spec.MySQL`

**Model/Controller:**
- `infraName = "database"` → `infraName = "mysql"`
- `handlePerconaMysql()` → Will use `reconcileMySQL()`
- `handlePerconaMysqlHA()` → Will use `reconcileMySQL()`

### 1.2 File Structure (OLD PATTERN)

```
internal/controller/wandb_v2/
├── mysqlOp.go          - Dev size handler
└── mysqlHaOp.go        - HA size handler
```

### 1.3 Current Flow (OLD PATTERN)

```
weightsandbiases_v2_controller.go:
if wandb.Spec.Size == apiv2.WBSizeDev {
    ctrlState = r.handlePerconaMysql(ctx, wandb, req)
} else {
    ctrlState = r.handlePerconaMysqlHA(ctx, wandb, req)
}
```

Both handlers follow similar pattern:
1. Check if Database enabled
2. Get actual Percona XtraDB Cluster CR
3. Handle deletion if finalizer present
4. Get desired Percona configuration
5. Compute drift between actual and desired
6. Execute reconciliation command
7. Return CtrlState (Continue/Error)

### 1.4 Percona XtraDB Cluster Resources

```yaml
# PerconaXtraDBCluster CR
apiVersion: pxc.percona.com/v1
kind: PerconaXtraDBCluster
metadata:
  name: wandb-mysql
spec:
  crVersion: "..."
  pxc:
    size: 1  # Dev: 1, HA: 3
    image: "percona/percona-xtradb-cluster:8.0.x"
    volumeSpec:
      persistentVolumeClaim:
        resources:
          requests:
            storage: "10Gi"
```

### 1.5 Key Observations

**What MySQL has that Kafka doesn't:**
- Backup configuration (uses backup operator)
- More complex PerconaXtraDBCluster spec
- Multiple sub-components (PXC, HAProxy, PMM)

**Similarities to Kafka:**
- Size-based branching (dev vs HA)
- Similar wrapper pattern
- Uses vendor operator (Percona)

---

## Section 2: Current Implementation Analysis

### 2.1 Key Differences: Dev vs HA

| Aspect | Dev (Standalone) | Small (HA) |
|--------|------------------|------------|
| **PXC Replicas** | 1 | 3 |
| **PXC Image** | `perconalab/...operator:main-pxc8.0` | `percona/percona-xtradb-cluster:8.0` |
| **ProxySQL** | **DISABLED** (not present) | **ENABLED** (3 replicas) |
| **HAProxy** | Disabled explicitly | Not used (ProxySQL instead) |
| **Connection** | Direct: `wandb-mysql-pxc.{ns}.svc` | Via ProxySQL: `wandb-mysql-proxysql.{ns}.svc` |
| **Unsafe Flags** | PXCSize: true, ProxySize: true | PXCSize: false, ProxySize: false |
| **Storage Default** | 20Gi | 20Gi |
| **TLS** | Disabled | Enabled |
| **LogCollector** | Enabled | Not present in HA |

### 2.2 Percona XtraDB Cluster Components

**Single CR manages multiple components:**
- **PXC**: The actual MySQL database pods
- **ProxySQL**: Connection pooling/load balancing (HA only)
- **HAProxy**: Alternative proxy (not used, explicitly disabled)
- **PMM**: Monitoring (not configured in current impl)
- **Backup**: Optional backup component

**Connection Secret:**
- Created when cluster is ready
- Name: `wandb-mysql-connection`
- Contains: `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`
- Password source: `{clustername}-secrets` (created by Percona operator)

---

## Section 3: Questions & Decisions

### ✅ CHECKPOINT 1: Naming Strategy - APPROVED (2025-11-14)

**DECISIONS:**

**1.1 API Renaming: ✅ FULL RENAME**
- `WBDatabaseSpec` → `WBMySQLSpec`
- `WBDatabaseStatus` → `WBMySQLStatus`
- Field: `spec.database` → `spec.mysql`
- Internal: `infraName = "database"` → `infraName = "mysql"`

**1.2 Type Enum: ✅ REMOVE WBDatabaseType**
- Only supporting Percona, no need for type selection
- Simplifies API and code

---

### ✅ CHECKPOINT 2: Config Structure - APPROVED (2025-11-14)

**DECISIONS:**

**2.1 Backup: ✅ DEFER ENTIRELY (Option A)**
- Omit backup from initial refactor
- Accept no backup functionality (like Kafka)
- Can be added in future iteration

**2.2 Connection Secret: ✅ DO NOT CREATE**
- Put connection info in `Status.Connection` only (like Redis/Kafka)
- No separate connection secret creation
- Simpler, more consistent with other infra

**2.3 ProxySQL Configuration: ✅ INCLUDE IN CONFIG**
- ProxySQLEnabled (dev: false, small: true)
- ProxySQLReplicas (small: 3)

**2.4 Images: ✅ AS CONSTANTS**
- Store in values.go
- Set in defaults based on size
- Dev vs Production images differ

**2.5 Optional Components:**
- PMM: ✅ Omit entirely
- HAProxy: ✅ Keep disabled (using ProxySQL)
- LogCollector: ✅ Hardcode based on size (dev: true, small: false)

**APPROVED CONFIG STRUCTURE:**

```go
type MySQLConfig struct {
    Enabled     bool
    Namespace   string
    StorageSize string  // Use CoalesceQuantity
    Replicas    int32   // Dev: 1, Small: 3
    Resources   v1.ResourceRequirements

    // Percona XtraDB specific
    PXCImage            string  // Dev vs Prod differ
    ProxySQLEnabled     bool    // Dev: false, Small: true
    ProxySQLReplicas    int32   // Small: 3
    ProxySQLImage       string
    TLSEnabled          bool    // Dev: false, Small: true
    LogCollectorEnabled bool    // Dev: true, Small: false
    LogCollectorImage   string

    // Unsafe flags (dev only)
    AllowUnsafePXCSize   bool
    AllowUnsafeProxySize bool
}
```

---

### ✅ CHECKPOINT 3: Size-Based Defaults - APPROVED (2025-11-14)

**DECISIONS:**

**Dev (Standalone):**
- Storage: ✅ `1Gi` (reduced from 20Gi)
- Replicas: ✅ `1`
- Resources: ✅ **None** (as a rule)
- TLS: ✅ `false`
- ProxySQL: ✅ `false`
- LogCollector: ✅ `true`
- Unsafe flags: ✅ `true`
- Image: ✅ `perconalab/percona-xtradb-cluster-operator:main-pxc8.0`

**Small (HA):**
- Storage: ✅ `10Gi` (reduced from 20Gi)
- Replicas: ✅ `3`
- Resources: ✅ CPU `500m`/`1000m`, Memory `1Gi`/`2Gi` (same as Kafka)
- TLS: ✅ `true`
- ProxySQL: ✅ `true` (3 replicas)
- LogCollector: ✅ `false`
- Unsafe flags: ✅ `false`
- Image: ✅ `percona/percona-xtradb-cluster:8.0`

**Constants in values.go:**
```go
DevPXCImage         = "perconalab/percona-xtradb-cluster-operator:main-pxc8.0"
SmallPXCImage       = "percona/percona-xtradb-cluster:8.0"
ProxySQLImage       = "percona/proxysql2:2.7.3"
LogCollectorImage   = "perconalab/percona-xtradb-cluster-operator:main-logcollector"
CRVersion           = "1.18.0"
```

---

### ✅ CHECKPOINT 4: Status & Connection - APPROVED (2025-11-14)

**DECISIONS:**

**4.1 Connection Endpoints: ✅ SIZE-DEPENDENT**
- Dev: `wandb-mysql-pxc.{namespace}.svc.cluster.local:3306` (direct to PXC)
- Small: `wandb-mysql-proxysql.{namespace}.svc.cluster.local:3306` (via ProxySQL)

**4.2 Connection in Status: ✅ YES (No separate secret)**
```go
type WBMySQLConnection struct {
    MySQLHost string `json:"MYSQL_HOST,omitempty"`
    MySQLPort string `json:"MYSQL_PORT,omitempty"`
}
```

**4.3 Ready Condition: ✅ status.ready == status.size**
- Use Percona's built-in ready check

**4.4 Credentials: ✅ NOT IN STATUS**
- Don't expose MYSQL_USER or MYSQL_PASSWORD
- Only expose host and port

**APPROVED STATUS STRUCTURE:**
```go
type WBMySQLStatus struct {
    Ready          bool              `json:"ready"`
    State          WBStateType       `json:"state,omitempty"`
    Details        []WBStatusDetail  `json:"details,omitempty"`
    LastReconciled metav1.Time       `json:"lastReconciled,omitempty"`
    Connection     WBMySQLConnection `json:"connection,omitempty"`
    BackupStatus   WBBackupStatus    `json:"backupStatus,omitempty"`  // Keep field, leave empty
}
```

---

### ✅ CHECKPOINT 5: Functionality Loss - APPROVED (2025-11-14)

**DECISIONS - All losses acceptable:**

| Feature | Current | After Refactor | Decision |
|---------|---------|----------------|----------|
| **Backup** | Supported | ❌ Omitted | ✅ DEFER |
| **Connection Secret** | Created | ❌ Not created | ✅ DEFER |
| **Drift detection** | Explicit | Minimal | ✅ ACCEPT |
| **Update logic** | Compares | Minimal | ✅ ACCEPT |
| **ProxySQL** | Configured | ✅ Preserved | ✅ KEEP |
| **TLS** | Size-dependent | ✅ Preserved | ✅ KEEP |

**Summary:**
- ❌ Backup: Deferred entirely
- ❌ Connection secret: Not created (connection in status only)
- ✅ ProxySQL: Fully preserved in config
- ✅ TLS: Fully preserved (size-dependent)
- ⚠️ Drift/Update: Minimal (like Kafka)

---

### ✅ CHECKPOINT 6: Implementation Order - APPROVED (2025-11-14)

**APPROVED PHASES:**

**Phase 0: API Renaming & Types** (~30 minutes)
1. Rename `WBDatabaseSpec` → `WBMySQLSpec`
2. Rename `WBDatabaseStatus` → `WBMySQLStatus`
3. Remove `WBDatabaseType` enum
4. Rename field `spec.database` → `spec.mysql` in WeightsAndBiases
5. Add `WBMySQLConnection` struct
6. Add `WBMySQLConfig` struct with Resources
7. Run `make generate`

**Phase 1: Model Layer** (~2-3 hours)
1. Create `internal/model/mysql.go`
   - MySQLConfig struct (with ProxySQL, TLS, LogCollector fields)
   - Status/error types
   - ExtractMySQLStatus() method
2. Create `internal/model/defaults/mysql.go`
   - Constants (storage, resources, images)
   - DefaultMySQLDev() - 1Gi, no resources
   - DefaultMySQLSmall() - 10Gi, with resources
3. Create `internal/model/merge/v2/mysql.go`
   - Merge logic (Config, StorageSize, Namespace)
4. Modify `internal/model/config.go`
   - AddMySQLSpec(), GetMySQLConfig()
5. Modify `internal/model/common.go`
   - Rename `Database` → `MySQL` in infraName constant

**Phase 2: Infrastructure Layer** (~3-4 hours)
1. Create `internal/controller/infra/mysql/interfaces.go`
   - ActualMySQL interface
2. Create `internal/controller/infra/mysql/percona/values.go`
   - Images, CRVersion, component names, port
3. Create `internal/controller/infra/mysql/percona/desired.go`
   - buildDesiredPXC() - handles dev vs HA (ProxySQL, TLS, LogCollector)
4. Create `internal/controller/infra/mysql/percona/create.go`
   - createPXC() with owner reference
5. Create `internal/controller/infra/mysql/percona/update.go`
   - updatePXC() - extract connection info (size-dependent endpoint)
6. Create `internal/controller/infra/mysql/percona/actual.go`
   - Initialize(), Upsert(), Delete()

**Phase 3: Reconciliation Layer** (~1 hour)
1. Create `internal/controller/wandb_v2/mysql.go`
   - reconcileMySQL() matching Redis/Kafka pattern

**Phase 4: Controller Integration** (~30 minutes)
1. Modify `weightsandbiases_v2_controller.go`
   - Add infraConfig.AddMySQLSpec()
   - Comment out handlePerconaMysql/handlePerconaMysqlHA calls
   - Add reconcileMySQL() call
   - **DO NOT delete mysqlOp.go or mysqlHaOp.go** (keep for reference)

**Phase 5: Testing** (~1 hour)
- Automated: compilation, linting
- Manual: dev/small deployments, deletion, connection in status
- Verify ProxySQL in HA, direct connection in dev

**Total: ~7-9 hours**

**Implementation approach:**
- Sequential phases (0 → 1 → 2 → 3 → 4 → 5)
- **Wait for user confirmation after each phase**
- Old code preserved, not deleted
- Focus on Dev and Small sizes only

---

## ✅ ALL CHECKPOINTS APPROVED - READY FOR IMPLEMENTATION

**Option A: Full Rename (Recommended)**
- Rename `WBDatabaseSpec` → `WBMySQLSpec`
- Rename `WBDatabaseStatus` → `WBMySQLStatus`
- Rename `WBDatabaseType` → `WBMySQLType`
- Field: `database` → `mysql` in WeightsAndBiases spec
- **Pros**: Clear, consistent, future-proof
- **Cons**: Breaking API change (requires migration)

**Option B: Keep API, Rename Internals**
- Keep `WBDatabaseSpec` in API for compatibility
- Use "MySQL" in internal model/controller layers only
- **Pros**: No breaking changes
- **Cons**: Inconsistent naming

**Question**: Which option do you prefer?

**Question 1.2: WBDatabaseType Enum**

Currently:
```go
type WBDatabaseType string
const (
    WBDatabaseTypePercona WBDatabaseType = "percona"
)
```

Since we only support Percona and likely won't add other database types:

**Option A**: Remove the Type enum entirely, assume Percona
**Option B**: Keep it for future extensibility

**Question**: Remove or keep the Type field?

---

### CHECKPOINT 2: Config Model Structure

Based on Kafka pattern and current Percona implementation:

**Proposed MySQLConfig:**

```go
type MySQLConfig struct {
    Enabled     bool
    Namespace   string
    StorageSize string  // e.g., "10Gi", use CoalesceQuantity for merging
    Replicas    int32   // Dev: 1, Small: 3
    Resources   v1.ResourceRequirements

    // MySQL/Percona specific
    Version     string  // Percona image version

    // Backup configuration (if preserving)
    Backup      MySQLBackupConfig
}

type MySQLBackupConfig struct {
    Enabled        bool
    StorageName    string
    StorageType    string  // filesystem, s3, etc.
    // ... other backup fields
}
```

**Questions:**

2.1. **Resources**: Should we have separate resources for PXC nodes vs HAProxy vs PMM?
   - **Recommendation**: Start with single Resources for PXC nodes, can split later

2.2. **Version**: Make Percona version configurable or constant?
   - **Recommendation**: Keep as constant in values.go initially (like Kafka)

2.3. **Backup**: Preserve backup functionality or defer like Kafka?
   - Current MySQL implementation has backup support
   - **Question**: Keep backup in initial refactor or defer to later?

2.4. **HAProxy**: Percona clusters use HAProxy for load balancing
   - **Question**: Expose HAProxy config or use defaults?

2.5. **PMM (Percona Monitoring)**: Optional monitoring component
   - **Question**: Include PMM config or omit entirely?

---

### CHECKPOINT 3: Size-Based Defaults

Following the dev/small pattern:

**Proposed Defaults:**

```go
// Dev Size: Standalone MySQL
func DefaultMySQLDev() *apiv2.WBMySQLSpec {
    return &apiv2.WBMySQLSpec{
        Enabled:     true,
        StorageSize: "??Gi",  // Question 3.1
        Namespace:   defaults.DefaultNamespace,
        // NO Resources field - dev has no limits (as a rule)
    }
}

// Small Size: HA MySQL with 3 replicas
func DefaultMySQLSmall() *apiv2.WBMySQLSpec {
    return &apiv2.WBMySQLSpec{
        Enabled:     true,
        StorageSize: "??Gi",  // Question 3.2
        Namespace:   defaults.DefaultNamespace,
        Config: &apiv2.WBMySQLConfig{
            Resources: v1.ResourceRequirements{
                Requests: v1.ResourceList{
                    v1.ResourceCPU:    "??",  // Question 3.3
                    v1.ResourceMemory: "??",  // Question 3.4
                },
                Limits: v1.ResourceList{
                    v1.ResourceCPU:    "??",  // Question 3.5
                    v1.ResourceMemory: "??",  // Question 3.6
                },
            },
        },
    }
}
```

**Questions:**

3.1. **Dev Storage**: What size? (Kafka uses 1Gi)
3.2. **Small Storage**: What size? (Kafka uses 5Gi)
3.3. **Small CPU Request**: What value? (Kafka uses 500m)
3.4. **Small Memory Request**: What value? (Kafka uses 1Gi)
3.5. **Small CPU Limit**: What value? (Kafka uses 1000m)
3.6. **Small Memory Limit**: What value? (Kafka uses 2Gi)

**Recommendation**: MySQL typically needs more resources than Kafka
- Dev: 2-5Gi storage, no limits
- Small: 10-20Gi storage, 1 CPU, 2Gi memory?

**Question**: What are appropriate defaults for your use case?

---

### CHECKPOINT 4: Status Handling

**Proposed Status Structure:**

```go
type WBMySQLStatus struct {
    Ready          bool              `json:"ready"`
    State          WBStateType       `json:"state,omitempty"`
    Details        []WBStatusDetail  `json:"details,omitempty"`
    LastReconciled metav1.Time       `json:"lastReconciled,omitempty"`
    Connection     WBMySQLConnection `json:"connection,omitempty"`
    BackupStatus   WBBackupStatus    `json:"backupStatus,omitempty"`  // If keeping backup
}

type WBMySQLConnection struct {
    MySQLHost string `json:"MYSQL_HOST,omitempty"`
    MySQLPort string `json:"MYSQL_PORT,omitempty"`
    MySQLUser string `json:"MYSQL_USER,omitempty"`  // If exposing credentials
}
```

**Questions:**

4.1. **Connection format**: What should we expose?
   - Host: `wandb-mysql-haproxy.{namespace}.svc.cluster.local` (HA) or `wandb-mysql-pxc.{namespace}.svc.cluster.local` (standalone)?
   - Port: `3306` (standard MySQL)
   - **Question**: Expose through HAProxy or direct to PXC?

4.2. **Ready condition**: When is MySQL considered ready?
   - Percona has: `status.ready == status.size`
   - **Recommendation**: All PXC replicas must be ready

4.3. **Credentials**: Expose in status or keep in secrets?
   - **Recommendation**: Keep in secrets, don't expose in status

4.4. **BackupStatus**: Keep or defer?
   - **Question**: Preserve backup status reporting?

---

### CHECKPOINT 5: Functionality Loss/Preservation

Comparison with Kafka refactor:

| Feature | Current | Kafka Approach | MySQL Approach | Decision Needed |
|---------|---------|----------------|----------------|-----------------|
| **Backup on deletion** | Supported | Deferred | ??? | Keep or defer? |
| **Drift detection** | Explicit state machine | Minimal | ??? | Minimal ok? |
| **Update logic** | Compares actual vs desired | Minimal | ??? | Minimal ok? |
| **HAProxy config** | Configurable | N/A | ??? | Expose or defaults? |
| **PMM monitoring** | Optional | N/A | ??? | Include or omit? |
| **Multi-component** | PXC + HAProxy + PMM | Single CR + NodePool | ??? | How to handle? |

**Key Questions:**

5.1. **Backup**: MySQL currently supports backup. Options:
   - A) Defer backup entirely (like Kafka)
   - B) Keep backup configuration in model but don't implement operator logic
   - C) Fully implement backup from the start
   - **Question**: Which approach?

5.2. **HAProxy**: Used for HA MySQL connection pooling
   - Currently gets its own configuration
   - **Question**: Essential or can we use Percona defaults?

5.3. **PMM (Percona Monitoring and Management)**:
   - Optional monitoring component
   - **Question**: Include in refactor or omit entirely?

5.4. **Multi-CR Structure**: Percona uses PerconaXtraDBCluster CR which manages multiple components
   - Simpler than Kafka (single CR vs Kafka + NodePool)
   - But CR itself is more complex
   - **Question**: Any concerns about the single-CR approach?

---

### CHECKPOINT 6: Implementation Order

**Proposed Phases:**

**Phase 0: API Renaming** (~30 minutes)
1. Rename `WBDatabaseSpec` → `WBMySQLSpec`
2. Rename `WBDatabaseStatus` → `WBMySQLStatus`
3. Rename field `database` → `mysql` in WeightsAndBiases
4. Add `WBMySQLConnection` struct
5. Add `WBMySQLConfig` struct (if needed)
6. Run `make generate`

**Phase 1: Model Layer** (~2-3 hours)
1. Create `internal/model/mysql.go`
2. Create `internal/model/defaults/mysql.go`
3. Create `internal/model/merge/v2/mysql.go`
4. Modify `internal/model/config.go` (AddMySQLSpec, GetMySQLConfig)
5. Update `internal/model/common.go` (rename Database → MySQL)

**Phase 2: Infrastructure Layer** (~3-4 hours)
1. Create `internal/controller/infra/mysql/interfaces.go`
2. Create `internal/controller/infra/mysql/percona/values.go`
3. Create `internal/controller/infra/mysql/percona/desired.go`
4. Create `internal/controller/infra/mysql/percona/create.go`
5. Create `internal/controller/infra/mysql/percona/update.go`
6. Create `internal/controller/infra/mysql/percona/actual.go`

**Phase 3: Reconciliation Layer** (~1 hour)
1. Create `internal/controller/wandb_v2/mysql.go`

**Phase 4: Controller Integration** (~30 minutes)
1. Modify `weightsandbiases_v2_controller.go`
   - Update infraConfig builder
   - Comment out old handlePerconaMysql calls
   - Add new reconcileMySQL() call

**Phase 5: Testing** (~1 hour)
- Automated: code compilation, linting
- Manual: dev deployment, small deployment, deletion, status

**Total estimated time: ~7-9 hours**

---

## Section 3: Open Questions Summary

**Critical Decisions Needed Before Implementation:**

1. **Naming**: Full API rename or internal only?
2. **Type Enum**: Remove WBDatabaseType or keep?
3. **Backup**: Preserve, defer, or partial implementation?
4. **Storage Sizes**: What defaults for dev/small?
5. **Resource Limits**: What CPU/memory for dev/small?
6. **HAProxy**: Essential config or use defaults?
7. **PMM**: Include or omit?
8. **Connection**: Expose via HAProxy or direct to PXC?

**Once these are decided, we can proceed with implementation.**

---

## Next Steps

1. Review and answer questions in Checkpoints 1-6
2. Confirm naming strategy
3. Confirm config structure
4. Confirm defaults
5. Confirm functionality scope (backup, HAProxy, PMM)
6. Once all approved, begin Phase 0

---

**Last Updated**: 2025-11-14
**Status**: Code analysis complete, awaiting user decisions on checkpoints

---

## CURRENT SESSION STATE (Before Context Compaction)

### Analysis Complete - Key Findings:

**From mysqlOp.go (Dev):**
- PXC Size: 1, Image: `perconalab/percona-xtradb-cluster-operator:main-pxc8.0`
- Storage default: 20Gi
- ProxySQL: NOT present (direct PXC connection)
- HAProxy: Explicitly disabled
- TLS: Disabled
- LogCollector: Enabled
- Unsafe flags: PXCSize=true, ProxySize=true (allows size=1)
- Connection: `wandb-mysql-pxc.{ns}.svc.cluster.local:3306`

**From mysqlHaOp.go (Small/HA):**
- PXC Size: 3, Image: `percona/percona-xtradb-cluster:8.0`
- Storage default: 20Gi
- ProxySQL: **ENABLED** (3 replicas, image: `percona/proxysql2:2.7.3`)
- HAProxy: Not used
- TLS: Enabled
- LogCollector: Not present
- Unsafe flags: PXCSize=false, ProxySize=false
- Connection: `wandb-mysql-proxysql.{ns}.svc.cluster.local:3306`

**Both:**
- Create `wandb-mysql-connection` secret with MYSQL_HOST/PORT/USER/PASSWORD
- Password from `{clustername}-secrets` (Percona-managed)
- Backup supported (PXCScheduledBackup with filesystem storage)
- CRVersion: "1.18.0"

### ✅ ALL USER DECISIONS CAPTURED:

**CHECKPOINT 1:**
- ✅ Full API rename (Database → MySQL)
- ✅ Remove WBDatabaseType enum

**CHECKPOINT 2:**
- ✅ Defer backup entirely (Option A)
- ✅ NO connection secret creation (connection info in status only)
- ✅ Include ProxySQL in config
- ✅ Images as constants in values.go
- ✅ Omit PMM, keep HAProxy disabled

**CHECKPOINT 3:**
- ✅ Dev: 1Gi storage, no resources
- ✅ Small: 10Gi storage, CPU 500m/1000m, Memory 1Gi/2Gi

**CHECKPOINT 4:**
- ✅ Size-dependent connection endpoints (dev=PXC, small=ProxySQL)
- ✅ Ready when status.ready == status.size
- ✅ No credentials in status

**CHECKPOINT 5:**
- ✅ All losses acceptable (backup, connection secret, minimal drift/update)

**CHECKPOINT 6:**
- ✅ Implementation phases 0-5 approved
- ✅ Phase-by-phase confirmation required

### USER CLARIFICATIONS PROVIDED:

1. ✅ Use ProxySQL (not HAProxy)
2. ✅ Use existing CR definitions from mysqlOp.go/mysqlHaOp.go as foundation
3. ✅ Small should have 3 replicas
4. ✅ Connection info goes in Status.Connection (NOT separate secret)
5. ✅ Storage: 1Gi dev, 10Gi small
6. ✅ Resources: Same as Kafka (500m/1000m CPU, 1Gi/2Gi mem)

### READY TO BEGIN IMPLEMENTATION

**Next action**: Start Phase 0 (API renaming)
