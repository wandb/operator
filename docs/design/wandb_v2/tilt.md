# Tilt Resource Dependency Graph

Resources shown with their labels in parentheses. The default path installs one
`wandb-operator` Helm release, Gateway API networking, an optional W&B CR, and
telemetry disabled.

Conditional resources:

- `Wandb`, `Wandb-Endpoint`, and `WandB-CA` only appear when `includeCR=True`.
- `WandB-Namespace` appears when Tilt needs a W&B namespace for the CR or telemetry resources.
- `gateway-api-crds` and `nginx-gateway-fabric` appear when `networkMode="gateway"`.
- `ingress-nginx-*` appears when `networkMode="ingress"`.
- `Telemetry-Endpoint-*` appears only when `observabilityMode="full"`.

Tilt generates the W&B CR through `go run ./hack/tilt/wandbcr`, then reads the
typed YAML back for the resource name, namespace, networking mode, and endpoint
hostname. The default CR omits `spec.wandb.manifestRepository` so the webhook
uses the published server manifest repository; `manifestSource="local"` mounts
`localManifestPath` into the operator image at `/server-manifest` and writes
`file:///server-manifest` into the generated CR.

```mermaid
graph TD
    %% Bootstrap and dependencies
    operator_codegen["Operator-Codegen\n(Wandb-Operator)"]
    operator_build["Operator-Build\n(Wandb-Operator)"]
    operator_chart_deps["Operator-Chart-Deps\n(Dependencies)"]
    wandb_crds_apply["WandB-CRDs-Apply\n(Dependencies)"]
    wandb_crds_ready["WandB-CRDs-Ready\n(Dependencies)"]
    cert_manager["cert-manager\n(Dependencies)"]
    gateway_api_crds["gateway-api-crds\n(Dependencies)"]
    nginx_gateway_fabric["nginx-gateway-fabric\n(Dependencies)"]
    ingress_nginx_repo["ingress-nginx-repo\n(Dependencies)"]
    ingress_nginx_controller["ingress-nginx-controller\n(Dependencies)"]
    wandb_namespace["WandB-Namespace\n(Dependencies)"]

    operator_codegen --> operator_build
    operator_codegen --> wandb_crds_apply
    wandb_crds_apply --> wandb_crds_ready
    gateway_api_crds --> nginx_gateway_fabric
    ingress_nginx_repo --> ingress_nginx_controller

    %% Operator install
    wandb_operator["wandb-operator\n(Wandb-Operator)"]
    operator_chart_deps --> wandb_operator
    operator_build --> wandb_operator
    wandb_crds_ready --> wandb_operator
    cert_manager --> wandb_operator
    nginx_gateway_fabric --> wandb_operator
    wandb_namespace --> wandb_operator

    %% Webhook and W&B CR
    operator_webhook_ready["Operator-Webhook-Ready\n(Wandb-Operator)"]
    wandb_ca["WandB-CA\n(Wandb-App)"]
    wandb["Wandb\n(Wandb-App)"]
    wandb_endpoint["Wandb-Endpoint\n(Wandb-App)"]
    dev_clean["Dev-Clean\n(Wandb-App)"]

    wandb_operator --> operator_webhook_ready
    cert_manager --> wandb_ca
    operator_webhook_ready --> wandb
    wandb_namespace --> wandb
    wandb_ca --> wandb
    nginx_gateway_fabric --> wandb
    ingress_nginx_controller --> wandb
    wandb --> wandb_endpoint
    nginx_gateway_fabric --> wandb_endpoint
    ingress_nginx_controller --> wandb_endpoint

    %% Telemetry endpoint port-forwards
    telemetry_grafana["Telemetry-Endpoint-Grafana\n(Telemetry)"]
    telemetry_metrics["Telemetry-Endpoint-VictoriaMetrics\n(Telemetry)"]
    telemetry_logs["Telemetry-Endpoint-VictoriaLogs\n(Telemetry)"]
    telemetry_traces["Telemetry-Endpoint-VictoriaTraces\n(Telemetry)"]

    wandb_operator --> telemetry_grafana
    wandb_operator --> telemetry_metrics
    wandb_operator --> telemetry_logs
    wandb_operator --> telemetry_traces

    classDef dependencies fill:#f5f5f5,stroke:#777
    classDef operator fill:#dbeafe,stroke:#2563eb
    classDef wandb fill:#ede9fe,stroke:#7c3aed
    classDef telemetry fill:#fce7f3,stroke:#db2777

    class operator_chart_deps,wandb_crds_apply,wandb_crds_ready,cert_manager,gateway_api_crds,nginx_gateway_fabric,ingress_nginx_repo,ingress_nginx_controller,wandb_namespace dependencies
    class operator_codegen,operator_build,wandb_operator,operator_webhook_ready operator
    class wandb_ca,wandb,wandb_endpoint,dev_clean wandb
    class telemetry_grafana,telemetry_metrics,telemetry_logs,telemetry_traces telemetry
```
