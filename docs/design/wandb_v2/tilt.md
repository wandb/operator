# Tilt Resource Dependency Graph

Resources shown with their labels in parentheses. Telemetry resources are only
active when `installTelemetry: true` and Wandb is only active when
`installWandb: true` in `tilt-settings.json`.

```mermaid
graph TD
    %% ── Bootstrapping ──────────────────────────────────────────────
    cert_manager["cert-manager\n(ext)"]
    helm_dep_update["helm-dep-update\n(Helm-Repos)"]

    subgraph codegen["Code Generation"]
        manifests["manifests"]
        generate["generate"]
    end

    %% ── Third-Party Operators ──────────────────────────────────────
    third_party["third-party-operators\n(Third-Party-Operators)"]
    helm_dep_update --> third_party

    %% ── Operator CRDs & RBAC ───────────────────────────────────────
    app_crd["Application CRD\n(Operator-Resources)"]
    wandb_crd["Wandb CRD\n(Operator-Resources)"]
    rbac["RBAC\n(Operator-Resources)"]
    operator_certs["Operator-Certs\n(Operator-Resources)"]
    codegen --> app_crd
    codegen --> wandb_crd

    %% ── CRD Readiness Gate ─────────────────────────────────────────
    crds_ready["operator-crds-ready\n(Operator-Resources)"]
    app_crd   --> crds_ready
    wandb_crd --> crds_ready

    %% ── Operator Controller ────────────────────────────────────────
    watch_compile["Watch&Compile\n(Operator-Resources)"]
    controller["operator-controller-manager\n(Operator-Resources)"]
    codegen     --> watch_compile
    codegen     --> controller
    crds_ready  --> controller
    third_party --> controller

    %% ── Webhook Readiness ──────────────────────────────────────────
    webhook_ready["webhook-ready\n(Operator-Resources)"]
    controller --> webhook_ready

    %% ── Wandb CR (installWandb) ────────────────────────────────────
    wandb["Wandb\n(Operator-Resources)"]
    webhook_ready --> wandb

    %% ── Telemetry: CRD & Operator Gates ───────────────────────────
    vm_crds["vm-crds-ready\n(Telemetry)"]
    grafana_crds["grafana-crds-ready\n(Telemetry)"]
    vm_operator["vm-operator-ready\n(Telemetry)"]
    third_party --> vm_crds
    third_party --> grafana_crds
    vm_crds     --> vm_operator

    %% ── Victoria Metrics Stack ─────────────────────────────────────
    subgraph victoria_stack["Victoria-* Stack"]
        victoria_metrics["Victoria-Metrics\n(Telemetry)"]
        victoria_logs["Victoria-Logs\n(Telemetry)"]
        victoria_traces["Victoria-Traces\n(Telemetry)"]
    end
    vm_operator --> victoria_stack

    %% ── Telemetry Dependents ───────────────────────────────────────
    otel["OTEL-Connection-Secret\n(Telemetry)"]
    kube_metrics["Kubernetes-Metrics\n(Telemetry)"]
    op_metrics["Operator-Metrics\n(Telemetry)"]
    infra_metrics["Infrastructure-Metrics\n(Telemetry)"]
    victoria_stack   --> otel
    vm_crds          --> kube_metrics
    victoria_metrics --> kube_metrics
    vm_crds          --> op_metrics
    victoria_metrics --> op_metrics
    vm_crds          --> infra_metrics
    victoria_metrics --> infra_metrics

    %% ── Grafana Stack ──────────────────────────────────────────────
    grafana["Grafana\n(Telemetry)"]
    grafana_ds["Grafana-Datasources\n(Telemetry)"]
    grafana_crds   --> grafana
    grafana_crds   --> grafana_ds
    grafana        --> grafana_ds
    victoria_stack --> grafana_ds

    %% ── Styles ─────────────────────────────────────────────────────
    classDef bootstrap  fill:#f5f5f5,stroke:#999
    classDef operator   fill:#dbeafe,stroke:#3b82f6
    classDef thirdparty fill:#dcfce7,stroke:#16a34a
    classDef gate       fill:#fef9c3,stroke:#ca8a04
    classDef telemetry  fill:#fce7f3,stroke:#db2777
    classDef wandb      fill:#ede9fe,stroke:#7c3aed

    class manifests,generate,cert_manager,helm_dep_update bootstrap
    class app_crd,wandb_crd,rbac,operator_certs,watch_compile,controller,webhook_ready operator
    class third_party thirdparty
    class crds_ready,vm_crds,grafana_crds,vm_operator gate
    class victoria_metrics,victoria_logs,victoria_traces,otel,kube_metrics,op_metrics,infra_metrics,grafana,grafana_ds telemetry
    class wandb wandb
```
