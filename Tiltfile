# Local operator development.
#
# Tilt keeps the fast local controller loop while installing the operator
# through the same Helm chart path as a normal install.

GENERATED_DIR = "hack/testing-manifests/wandb/.generated"
GENERATED_WANDB_CR = GENERATED_DIR + "/tilt-wandb-cr.yaml"
GENERATED_OPERATOR_VALUES = GENERATED_DIR + "/tilt-operator-values.yaml"

GATEWAY_API_CRDS_URL = "https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.0/standard-install.yaml"
IMG = "controller:latest"

GROUP_DEPENDENCIES = "Dependencies"
GROUP_WANDB_APP = "Wandb-App"
GROUP_TELEMETRY = "Telemetry"
GROUP_WANDB_OPERATOR = "Wandb-Operator"

settings = {
    "allowedContexts": [
        "docker-desktop",
        "minikube",
        "kind-kind",
        "kind-wandb-operator",
        "orbstack",
        "crc-admin",
    ],

    # Operator install settings.
    "operatorNamespace": "wandb-operators",
    "openshiftSCC": False,

    # W&B CR settings.
    "includeCR": True,
    "crFile": "",
    "wandbName": "wandb",
    "wandbNamespace": "wandb",
    "wandbHostname": "http://localhost:8080",
    "wandbVersion": "0.80.0",
    "size": "dev",
    "retentionPolicy": "detach",
    "licenseFile": "",
    "manifestSource": "published",  # published or local
    "localManifestPath": "hack/testing-manifests/server-manifest",
    "networkMode": "gateway",  # gateway or ingress
    "gatewayClass": "nginx",
    "ingressClass": "nginx",
    "createCA": True,
    "issuerName": "",

    # off, full, forward.
    "observabilityMode": "off",

    "logFormat": "pretty",  # pretty, text, json
}

if os.path.exists("tilt-settings.json"):
    fail("tilt-settings.json is no longer supported. Migrate to tilt-settings.star (see tilt-settings.sample.star).")

if not os.path.exists("tilt-settings.star"):
    local("cp tilt-settings.sample.star tilt-settings.star")

load("./tilt-settings.star", "SETTINGS")
settings.update(SETTINGS)


def warn(message):
    print("WARNING: " + message)


def as_bool(value):
    if value == True:
        return True
    if value == False or value == None:
        return False
    return str(value).lower() in ["true", "yes", "1", "on"]


def bool_string(value):
    if value:
        return "true"
    return "false"


def normalize_observability_mode():
    mode = str(settings.get("observabilityMode", "off")).lower()
    if mode in ["off", "full", "forward"]:
        return mode

    fail("observabilityMode must be one of: off, full, forward")


def normalize_network_mode():
    mode = str(settings.get("networkMode", "gateway")).lower()
    if mode in ["gateway", "ingress"]:
        return mode

    fail("networkMode must be one of: gateway, ingress")


def normalize_manifest_source():
    source = str(settings.get("manifestSource", "published")).lower()
    if source in ["published", "local"]:
        return source

    fail("manifestSource must be one of: published, local")


def shell_quote(value):
    return "'" + str(value).replace("'", "'\"'\"'") + "'"


def write_file_cmd(path, contents):
    return "cat > %s <<'EOF'\n%s\nEOF" % (path, contents)


def k8s_yaml_object(obj):
    k8s_yaml(encode_yaml(obj))


def repo_path(path):
    path = str(path)
    if path.startswith("./"):
        return path
    return "./" + path


def validate_local_manifest_path(path):
    path = str(path)
    if path.startswith("/") or path.startswith("../") or "/../" in path or path == "..":
        fail("localManifestPath must be a repo-relative Docker build-context path.")
    if " " in path:
        fail("localManifestPath cannot contain spaces because it is used in a Dockerfile ADD instruction.")
    if not os.path.exists(path):
        fail("manifestSource='local' requires localManifestPath to exist: %s" % path)
    return path


def write_generated_yaml(path, obj):
    local("mkdir -p " + GENERATED_DIR)
    local(write_file_cmd(path, encode_yaml(obj)))
    return path


def helm_supports_take_ownership():
    return str(local("helm upgrade --help | grep -q -- '--take-ownership' && echo true || echo false")).strip() == "true"


def url_host(url):
    rest = str(url)
    if "://" in rest:
        rest = rest.split("://", 1)[1]
    host_port = rest.split("/", 1)[0]
    if "@" in host_port:
        host_port = host_port.split("@", 1)[1]
    parts = host_port.split(":")
    return parts[0]


def url_port(url):
    rest = str(url)
    if "://" in rest:
        rest = rest.split("://", 1)[1]
    host_port = rest.split("/", 1)[0]
    parts = host_port.split(":")
    if len(parts) > 1:
        return int(parts[len(parts) - 1])
    if str(url).startswith("https://"):
        return 443
    return 80


settings["networkMode"] = normalize_network_mode()
settings["observabilityMode"] = normalize_observability_mode()
settings["manifestSource"] = normalize_manifest_source()
settings["openshiftSCC"] = as_bool(settings.get("openshiftSCC"))
if settings["manifestSource"] == "local":
    settings["localManifestPath"] = validate_local_manifest_path(settings.get("localManifestPath"))

watch_settings(ignore=["**/.git", "**/*.out", GENERATED_DIR + "/**"])
update_settings(k8s_upsert_timeout_secs=300)

currentContext = k8s_context()
if currentContext in settings.get("allowedContexts"):
    print("Context is allowed")
else:
    fail("Selected context is not in allow list")

allow_k8s_contexts(settings.get("allowedContexts"))

IS_CRC = "crc" in currentContext or "api-crc-testing" in currentContext
if IS_CRC:
    settings["openshiftSCC"] = True
    default_registry(
        "default-route-openshift-image-registry.apps-crc.testing/%s" % settings.get("operatorNamespace"),
        host_from_cluster="image-registry.openshift-image-registry.svc:5000/%s" % settings.get("operatorNamespace"),
    )

os.putenv("PATH", "./bin:" + os.getenv("PATH"))

load("ext://restart_process", "docker_build_with_restart")
load("ext://helm_resource", "helm_repo", "helm_resource")

def operator_dockerfile():
    lines = [
        "FROM registry.access.redhat.com/ubi9/ubi",
        "",
        "ADD tilt_bin/manager /manager",
    ]

    if settings.get("manifestSource") == "local":
        lines.append("ADD %s /server-manifest" % settings.get("localManifestPath"))

    if settings.get("openshiftSCC"):
        lines += [
            "",
            "RUN mkdir -p /helm/.cache/helm /helm/.config/helm /helm/.local/share/helm && \\",
            "    chgrp -R 0 /helm && chmod -R g=u /helm",
        ]
    else:
        lines += [
            "",
            "RUN mkdir -p /helm/.cache/helm /helm/.config/helm /helm/.local/share/helm",
        ]

    lines += [
        "",
        "ENV HELM_CACHE_HOME=/helm/.cache/helm",
        "ENV HELM_CONFIG_HOME=/helm/.config/helm",
        "ENV HELM_DATA_HOME=/helm/.local/share/helm",
    ]

    if settings.get("openshiftSCC"):
        lines += [
            "",
            "USER 1001",
        ]

    lines.append("")

    return "\n".join(lines)


def binary():
    return "CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -o tilt_bin/manager cmd/main.go"


def managed_endpoint_resource(name, anchor_object, deps, local_port, remote_port, link_name, pod_selector, labels, local_host="localhost"):
    k8s_resource(
        new_name=name,
        objects=[anchor_object],
        discovery_strategy="selectors-only",
        extra_pod_selectors=[pod_selector],
        resource_deps=deps,
        port_forwards=[
            port_forward(local_port, remote_port, name=link_name, host=local_host),
        ],
        labels=labels,
    )


def endpoint_anchor(name):
    return {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
            "name": name,
            "namespace": "default",
        },
        "data": {
            "managed-by": "tilt",
        },
    }


def build_endpoint_anchors(names):
    for name in names:
        k8s_yaml_object(endpoint_anchor(name))


def build_wandb_namespace(namespace):
    k8s_yaml_object({
        "apiVersion": "v1",
        "kind": "Namespace",
        "metadata": {
            "name": namespace,
            "labels": {
                "app.kubernetes.io/managed-by": "tilt",
            },
        },
    })


def build_operator_values(telemetry_namespace):
    telemetry_enabled = settings.get("observabilityMode") != "off"
    grafana_enabled = settings.get("observabilityMode") == "full"

    values = {
        "wandb": {
            "install": False,
        },
        "wandb-operator": {
            "image": {
                "pullPolicy": "IfNotPresent",
            },
            "containers": {
                "operator": {
                    "command": [],
                },
            },
        },
        "victoria-metrics-operator": {
            "enabled": telemetry_enabled,
            "crds": {
                "plain": True,
            },
            "admissionWebhooks": {
                "enabled": False,
            },
        },
        "grafana-operator": {
            "enabled": grafana_enabled,
        },
        "telemetry": {
            "mode": settings.get("observabilityMode"),
            "namespace": telemetry_namespace,
        },
    }

    if settings.get("openshiftSCC"):
        values["wandb-operator"]["podSecurityContext"] = {
            "runAsNonRoot": True,
            "runAsUser": None,
            "runAsGroup": None,
            "fsGroup": None,
            "fsGroupChangePolicy": None,
            "seccompProfile": {
                "type": "RuntimeDefault",
            },
        }
        values["wandb-operator"]["containers"]["operator"]["env"] = {
            "KAFKA_FSGROUP": {
                "value": "0",
            },
        }
        values["wandb-operator"]["containers"]["operator"]["securityContext"] = {
            "allowPrivilegeEscalation": False,
            "readOnlyRootFilesystem": True,
            "capabilities": {
                "drop": ["ALL"],
            },
        }
        values["altinity-clickhouse-operator"] = {
            "crdHook": {
                "enabled": False,
            },
        }

    return write_generated_yaml(GENERATED_OPERATOR_VALUES, values)


def helper_flag(name, value):
    if value == None or value == "":
        return ""
    return " --%s %s" % (name, shell_quote(value))


def helper_bool_flag(name, value):
    return " --%s=%s" % (name, bool_string(as_bool(value)))


def build_wandb_cr():
    if settings.get("wandbCR"):
      return settings.get("wandbCR")
    else:
      cmd = "go run ./hack/tilt/wandbcr"
      cmd += helper_flag("out", GENERATED_WANDB_CR)
      cmd += helper_flag("cr-file", settings.get("crFile", ""))
      cmd += helper_flag("name", settings.get("wandbName"))
      cmd += helper_flag("namespace", settings.get("wandbNamespace"))
      cmd += helper_flag("hostname", settings.get("wandbHostname"))
      cmd += helper_flag("version", settings.get("wandbVersion"))
      cmd += helper_flag("size", settings.get("size"))
      cmd += helper_flag("retention-policy", settings.get("retentionPolicy"))
      cmd += helper_flag("license-file", settings.get("licenseFile", ""))
      cmd += helper_flag("manifest-source", settings.get("manifestSource"))
      cmd += helper_flag("observability-mode", settings.get("observabilityMode"))
      cmd += helper_flag("network-mode", settings.get("networkMode"))
      cmd += helper_flag("gateway-class", settings.get("gatewayClass"))
      cmd += helper_flag("ingress-class", settings.get("ingressClass"))
      cmd += helper_bool_flag("create-ca", settings.get("createCA"))
      cmd += helper_flag("issuer-name", settings.get("issuerName", ""))
      local(cmd)

      return GENERATED_WANDB_CR


def build_wandb_ca(name, namespace):
    root_cert_name = name + "-root-cert"
    selfsigned_issuer_name = name + "-selfsigned-issuer"
    ca_issuer_name = name + "-ca-issuer"

    k8s_yaml_object({
        "apiVersion": "cert-manager.io/v1",
        "kind": "Issuer",
        "metadata": {
            "name": selfsigned_issuer_name,
            "namespace": namespace,
        },
        "spec": {
            "selfSigned": {},
        },
    })
    k8s_yaml_object({
        "apiVersion": "cert-manager.io/v1",
        "kind": "Certificate",
        "metadata": {
            "name": root_cert_name,
            "namespace": namespace,
        },
        "spec": {
            "secretName": root_cert_name,
            "isCA": True,
            "commonName": "wandb-ca",
            "duration": "210240h",
            "issuerRef": {
                "name": selfsigned_issuer_name,
                "kind": "Issuer",
                "group": "cert-manager.io",
            },
        },
    })
    k8s_yaml_object({
        "apiVersion": "cert-manager.io/v1",
        "kind": "Issuer",
        "metadata": {
            "name": ca_issuer_name,
            "namespace": namespace,
        },
        "spec": {
            "ca": {
                "secretName": root_cert_name,
            },
        },
    })
    k8s_resource(
        new_name="WandB-CA",
        objects=[
            "%s:issuer:%s" % (selfsigned_issuer_name, namespace),
            "%s:certificate:%s" % (root_cert_name, namespace),
            "%s:issuer:%s" % (ca_issuer_name, namespace),
        ],
        resource_deps=["cert-manager"],
        labels=[GROUP_WANDB_APP],
    )


WANDB_CR = build_wandb_cr() if as_bool(settings.get("includeCR")) else ""
WANDB_CR_CONTENT = read_yaml(WANDB_CR) if as_bool(settings.get("includeCR")) else {}
WANDB_NAME = WANDB_CR_CONTENT.get("metadata", {}).get("name", settings.get("wandbName"))
WANDB_NAMESPACE = WANDB_CR_CONTENT.get("metadata", {}).get("namespace", settings.get("wandbNamespace"))
WANDB_HOSTNAME = WANDB_CR_CONTENT.get("spec", {}).get("wandb", {}).get("hostname", settings.get("wandbHostname"))
OPERATOR_VALUES = build_operator_values(WANDB_NAMESPACE)
LOCAL_NETWORKING_MODE = WANDB_CR_CONTENT.get("spec", {}).get("networking", {}).get("mode", settings.get("networkMode"))
CREATE_WANDB_NAMESPACE = as_bool(settings.get("includeCR")) or settings.get("observabilityMode") != "off"

endpoint_anchors = []
if as_bool(settings.get("includeCR")):
    if LOCAL_NETWORKING_MODE in ["gateway", "ingress"]:
        endpoint_anchors.append("wandb-endpoint-anchor")

if settings.get("observabilityMode") == "full":
    endpoint_anchors += [
        "telemetry-grafana-endpoint-anchor",
        "telemetry-victoria-metrics-endpoint-anchor",
        "telemetry-victoria-logs-endpoint-anchor",
        "telemetry-victoria-traces-endpoint-anchor",
    ]

if endpoint_anchors:
    build_endpoint_anchors(endpoint_anchors)

if CREATE_WANDB_NAMESPACE:
    build_wandb_namespace(WANDB_NAMESPACE)
    k8s_resource(
        new_name="WandB-Namespace",
        objects=["%s:namespace" % WANDB_NAMESPACE],
        labels=[GROUP_DEPENDENCIES],
    )

local_resource(
    "Operator-Codegen",
    "make manifests generate",
    labels=[GROUP_WANDB_OPERATOR],
)

local_resource(
    "Operator-Build",
    binary(),
    deps=["internal", "pkg", "api", "cmd"],
    resource_deps=["Operator-Codegen"],
    ignore=["*/*/zz_generated.deepcopy.go"],
    labels=[GROUP_WANDB_OPERATOR],
)

local_resource(
    "Operator-Chart-Deps",
    "helm dependency build ./deploy/operator --skip-refresh",
    deps=[
        "deploy/operator/Chart.yaml",
        "deploy/operator/Chart.lock",
        "deploy/telemetry/Chart.yaml",
        "deploy/telemetry/values.yaml",
        "deploy/telemetry/templates",
        "deploy/telemetry/dashboards",
    ],
    labels=[GROUP_DEPENDENCIES],
)

local_resource(
    "WandB-CRDs-Apply",
    "kubectl apply --server-side=true --force-conflicts --field-manager=helm " +
    "-f config/crd/bases/apps.wandb.com_applications.yaml " +
    "-f config/crd/bases/apps.wandb.com_weightsandbiases.yaml",
    resource_deps=["Operator-Codegen"],
    labels=[GROUP_DEPENDENCIES],
)

local_resource(
    "WandB-CRDs-Ready",
    "kubectl wait --for=condition=established --timeout=120s " +
    "crd/applications.apps.wandb.com " +
    "crd/weightsandbiases.apps.wandb.com",
    resource_deps=["WandB-CRDs-Apply"],
    labels=[GROUP_DEPENDENCIES],
)

cert_manager_flags = [
        "--create-namespace",
        "--version=v1.20.2",
        "--set=crds.enabled=true",
        "--set=startupapicheck.enabled=false",
]
cert_manager_deps = []

if LOCAL_NETWORKING_MODE == "gateway":
    local_resource(
        "gateway-api-crds",
        "kubectl apply -f " + GATEWAY_API_CRDS_URL,
        labels=[GROUP_DEPENDENCIES],
    )
    cert_manager_flags.append("--set=config.enableGatewayAPI=true")
    cert_manager_deps.append("gateway-api-crds")

helm_resource(
    "cert-manager",
    chart="oci://quay.io/jetstack/charts/cert-manager",
    namespace="cert-manager",
    flags=cert_manager_flags,
    resource_deps=cert_manager_deps,
    labels=[GROUP_DEPENDENCIES],
)

if LOCAL_NETWORKING_MODE == "gateway":
    nginx_gateway_flags = [
        "--create-namespace",
        "--version=2.5.1",
    ]
    if currentContext.startswith("kind-"):
        nginx_gateway_flags += [
            "--set=nginx.service.type=NodePort",
            "--set=nginx.service.nodePorts[0].port=31437",
            "--set=nginx.service.nodePorts[0].listenerPort=8080",
            "--set=nginx.service.nodePorts[1].port=30478",
            "--set=nginx.service.nodePorts[1].listenerPort=8443",
        ]

    helm_resource(
        "nginx-gateway-fabric",
        chart="oci://ghcr.io/nginx/charts/nginx-gateway-fabric",
        namespace="nginx-gateway",
        flags=nginx_gateway_flags,
        resource_deps=["gateway-api-crds"],
        labels=[GROUP_DEPENDENCIES],
    )

if LOCAL_NETWORKING_MODE == "ingress":
    helm_repo(
        "ingress-nginx",
        "https://kubernetes.github.io/ingress-nginx",
        resource_name="ingress-nginx-repo",
        labels=[GROUP_DEPENDENCIES],
    )
    helm_resource(
        "ingress-nginx-controller",
        chart="ingress-nginx/ingress-nginx",
        release_name="ingress-nginx",
        namespace="ingress-nginx",
        flags=[
            "--create-namespace",
            "--version=4.14.1",
            "--set-string=controller.ingressClass=%s" % settings.get("ingressClass"),
            "--set-string=controller.ingressClassResource.name=%s" % settings.get("ingressClass"),
            "--set-string=controller.service.type=ClusterIP",
        ],
        resource_deps=["ingress-nginx-repo"],
        labels=[GROUP_DEPENDENCIES],
    )

operator_deps = ["Operator-Chart-Deps", "Operator-Build", "WandB-CRDs-Ready"]
operator_deps.append("cert-manager")
if LOCAL_NETWORKING_MODE == "gateway":
    operator_deps.append("nginx-gateway-fabric")
if settings.get("observabilityMode") != "off":
    operator_deps.append("WandB-Namespace")

operator_flags = ["--create-namespace"]
if helm_supports_take_ownership():
    operator_flags.append("--take-ownership")
else:
    warn("helm does not support --take-ownership; legacy CRD ownership may require Dev-Clean before the operator release can install.")

operator_flags += [
    "-f",
    OPERATOR_VALUES,
]
operator_deps_files = [
    OPERATOR_VALUES,
    "deploy/operator/Chart.yaml",
    "deploy/operator/values.yaml",
]

if settings.get("openshiftSCC"):
    operator_flags += [
        "--values=./deploy/operator/profiles/openshift.yaml",
    ]
    operator_deps_files.append("deploy/operator/profiles/openshift.yaml")

helm_resource(
    "wandb-operator",
    chart="./deploy/operator",
    release_name="wandb-operator",
    namespace=settings.get("operatorNamespace"),
    flags=operator_flags,
    image_deps=[IMG],
    image_keys=[("wandb-operator.image.repository", "wandb-operator.image.tag")],
    deps=operator_deps_files,
    resource_deps=operator_deps,
    labels=[GROUP_WANDB_OPERATOR],
)

local_resource(
    "Operator-Webhook-Ready",
    cmd="kubectl wait --for=condition=available --timeout=300s -n %s deploy/wandb-operator && " % settings.get("operatorNamespace") +
    "until kubectl get mutatingwebhookconfiguration wandb-operator-mutating-webhook-configuration " +
    "-o jsonpath='{.webhooks[0].clientConfig.caBundle}' | grep -q .; " +
    "do echo 'Waiting for webhook CA bundle to be injected...'; sleep 2; done && echo 'Webhook is ready!'",
    resource_deps=["wandb-operator"],
    labels=[GROUP_WANDB_OPERATOR],
)

local_resource(
    "Dev-Clean",
    "./hack/scripts/tilt-dev-clean.sh",
    auto_init=False,
    labels=[GROUP_WANDB_APP],
)

if as_bool(settings.get("includeCR")):
    wandb_deps = ["Operator-Webhook-Ready", "WandB-Namespace"]
    if LOCAL_NETWORKING_MODE == "gateway":
        wandb_deps.append("nginx-gateway-fabric")
    if LOCAL_NETWORKING_MODE == "ingress":
        wandb_deps.append("ingress-nginx-controller")

    if str(WANDB_HOSTNAME).startswith("https://") and as_bool(settings.get("createCA")):
        build_wandb_ca(WANDB_NAME, WANDB_NAMESPACE)
        wandb_deps.append("WandB-CA")

    k8s_yaml(WANDB_CR)

    k8s_resource(
        new_name="Wandb",
        objects=["%s:weightsandbiases:%s" % (WANDB_NAME, WANDB_NAMESPACE)],
        resource_deps=wandb_deps,
        labels=[GROUP_WANDB_APP],
    )

    endpoint_port = url_port(WANDB_HOSTNAME)
    endpoint_host = url_host(WANDB_HOSTNAME)

    if LOCAL_NETWORKING_MODE == "gateway":
        managed_endpoint_resource(
            name="Wandb-Endpoint",
            anchor_object="wandb-endpoint-anchor:configmap:default",
            deps=["Wandb", "nginx-gateway-fabric"],
            local_port=endpoint_port,
            remote_port=endpoint_port,
            link_name="W&B gateway",
            local_host=endpoint_host,
            pod_selector={
                "app.kubernetes.io/instance": "nginx-gateway-fabric",
                "app.kubernetes.io/name": "nginx-gateway-fabric",
            },
            labels=[GROUP_WANDB_APP],
        )
    elif LOCAL_NETWORKING_MODE == "ingress":
        managed_endpoint_resource(
            name="Wandb-Endpoint",
            anchor_object="wandb-endpoint-anchor:configmap:default",
            deps=["Wandb", "ingress-nginx-controller"],
            local_port=endpoint_port,
            remote_port=80,
            link_name="W&B ingress",
            local_host=endpoint_host,
            pod_selector={
                "app.kubernetes.io/component": "controller",
                "app.kubernetes.io/instance": "ingress-nginx",
                "app.kubernetes.io/name": "ingress-nginx",
            },
            labels=[GROUP_WANDB_APP],
        )
if settings.get("observabilityMode") == "full":
    managed_endpoint_resource(
        name="Telemetry-Endpoint-Grafana",
        anchor_object="telemetry-grafana-endpoint-anchor:configmap:default",
        deps=["wandb-operator"],
        local_port=3000,
        remote_port=3000,
        link_name="Grafana",
        pod_selector={"app": "grafana"},
        labels=[GROUP_TELEMETRY],
    )
    managed_endpoint_resource(
        name="Telemetry-Endpoint-VictoriaMetrics",
        anchor_object="telemetry-victoria-metrics-endpoint-anchor:configmap:default",
        deps=["wandb-operator"],
        local_port=8428,
        remote_port=8429,
        link_name="VictoriaMetrics UI",
        pod_selector={
            "app.kubernetes.io/name": "vmsingle",
            "app.kubernetes.io/instance": "victoria-instance",
        },
        labels=[GROUP_TELEMETRY],
    )
    managed_endpoint_resource(
        name="Telemetry-Endpoint-VictoriaLogs",
        anchor_object="telemetry-victoria-logs-endpoint-anchor:configmap:default",
        deps=["wandb-operator"],
        local_port=9428,
        remote_port=9428,
        link_name="VictoriaLogs",
        pod_selector={
            "app.kubernetes.io/name": "vlsingle",
            "app.kubernetes.io/instance": "victoria-logs",
        },
        labels=[GROUP_TELEMETRY],
    )
    managed_endpoint_resource(
        name="Telemetry-Endpoint-VictoriaTraces",
        anchor_object="telemetry-victoria-traces-endpoint-anchor:configmap:default",
        deps=["wandb-operator"],
        local_port=10428,
        remote_port=10428,
        link_name="VictoriaTraces",
        pod_selector={
            "app.kubernetes.io/name": "vtsingle",
            "app.kubernetes.io/instance": "victoria-traces",
        },
        labels=[GROUP_TELEMETRY],
    )

manager_entrypoint = ["/manager", "--log-format=" + settings.get("logFormat")]
if settings.get("observabilityMode") != "off":
    manager_entrypoint += ["--telemetry-enabled=true"]

docker_only = ["./tilt_bin/manager"]
live_update_steps = [
    sync("./tilt_bin/manager", "/manager"),
]

if settings.get("manifestSource") == "local":
    docker_only.append(repo_path(settings.get("localManifestPath")))
    live_update_steps.append(sync(settings.get("localManifestPath"), "/server-manifest"))

docker_build_with_restart(
    IMG,
    ".",
    dockerfile_contents=operator_dockerfile(),
    entrypoint=manager_entrypoint,
    only=docker_only,
    live_update=live_update_steps,
)
