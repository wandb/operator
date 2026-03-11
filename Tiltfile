# default values
settings = {
    "allowedContexts": [
        "docker-desktop",
        "minikube",
        "kind-kind",
        "orbstack",
    ],
    "installWandb": True,
    "wandbCRD": "wandb-default-v1",
    "installTelemetry": False,
    "logFormat": "pretty",  # pretty, text, json
}

# global settings
settings.update(read_json(
    "tilt-settings.json",
    default={},
))

# Configure global watch settings with a 2-second debounce
watch_settings(ignore=["**/.git", "**/*.out"])

# Increase timeout for helm installations and apply operations
update_settings(k8s_upsert_timeout_secs=300)

currentContext = k8s_context()

if currentContext in settings.get("allowedContexts"):
    print("Context is allowed")
else:
    fail("Selected context is not in allow list")

allow_k8s_contexts(settings.get("allowed_k8s_contexts"))

os.putenv('PATH', './bin:' + os.getenv('PATH'))

load('ext://restart_process', 'docker_build_with_restart')
load('ext://helm_resource', 'helm_resource')
load('ext://cert_manager', 'deploy_cert_manager')

DOCKERFILE = '''
FROM registry.access.redhat.com/ubi9/ubi

ADD tilt_bin/manager /manager
ADD hack/testing-manifests/server-manifest /server-manifest

RUN mkdir -p /helm/.cache/helm /helm/.config/helm /helm/.local/share/helm

ENV HELM_CACHE_HOME=/helm/.cache/helm
ENV HELM_CONFIG_HOME=/helm/.config/helm
ENV HELM_DATA_HOME=/helm/.local/share/helm
'''
DOMAIN = "wandb.com"
GROUP = "apps"
VERSION = "v1"
KIND = "wandb"
IMG = 'controller:latest'
CONTROLLERGEN = 'rbac:roleName=manager-role crd:allowDangerousTypes=true,generateEmbeddedObjectMeta=true,maxDescLen=0 webhook paths="{./api/v1,./api/v2,./internal/controller/...}" output:crd:artifacts:config=config/crd/bases'
DISABLE_SECURITY_CONTEXT = True

def manifests():
    return 'make manifests'


def generate():
    return 'make generate'


def vetfmt():
    return 'go vet ./...; go fmt ./...'

# build to tilt_bin because kubebuilder has a dockerignore for bin/

def binary():
    return 'CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -o tilt_bin/manager cmd/main.go'


installed = local("which kubebuilder")
print("kubebuilder is present:", installed)

DIRNAME = os.path.basename(os. getcwd())

local_resource("manifests", manifests(), labels=["Operator-Resources"])
local_resource("generate", generate(), labels=["Operator-Resources"])

deploy_cert_manager()

local_resource(
    'helm-dep-update',
    'helm dependency update ./deploy/operator',
    labels=["Helm-Repos"],
)

third_party_operator_flags = [
    '--set=wandb-operator.enabled=false',
    '--set=telemetry.enabled=false',
    '--create-namespace',
]

helm_resource(
    'third-party-operators',
    chart='./deploy/operator',
    resource_deps=['helm-dep-update'],
    namespace='wandb-operator',
    flags=third_party_operator_flags,
    labels=["Third-Party-Operators"],
)

k8s_yaml(local('kustomize build config/tilt-dev'))

k8s_resource(
    new_name='Operator-Certs',
    objects=[
        'operator-system:namespace',
        'operator-metrics-certs:certificate',
        'operator-serving-cert:certificate',
        'operator-selfsigned-issuer:issuer',
    ],
    # deploy_cert_manager() runs local() commands and registers no Tilt resource,
    # so a resource_dep cannot be declared here. Tilt retries on failure.
    labels=["Operator-Resources"],
)

local_resource(
    'Application CRD',
    'kustomize build config/crd | kubectl apply --server-side=true --force-conflicts -f -',
    resource_deps=["manifests", "generate"],
    labels=["Operator-Resources"],
)

local_resource(
    'Wandb CRD',
    'echo "Wandb CRD is applied by Application CRD server-side apply step."',
    resource_deps=["Application CRD"],
    # wandb-operator is disabled in this Tilt setup; label is for 3rd party operator CRD grouping only
    labels=["Operator-Resources", "third-party-operators"],
)
k8s_resource(
    new_name='RBAC',
    objects=[
        'operator-manager-role:clusterrole',
        'operator-manager-rolebinding:clusterrolebinding',
        'operator-leader-election-role:role',
        'operator-leader-election-rolebinding:rolebinding',
        'operator-application-admin-role:clusterrole',
        'operator-application-editor-role:clusterrole',
        'operator-application-viewer-role:clusterrole',
        'operator-metrics-auth-role:clusterrole',
        'operator-metrics-reader:clusterrole',
        'operator-weightsandbiases-admin-role:clusterrole',
        'operator-weightsandbiases-editor-role:clusterrole',
        'operator-weightsandbiases-viewer-role:clusterrole',
        'operator-metrics-auth-rolebinding:clusterrolebinding',
    ],
    resource_deps=["manifests", "generate"],
    labels=["Operator-Resources"],
)

local_resource(
    'operator-crds-ready',
    'kubectl wait --for=condition=established --timeout=120s ' +
    'crd/applications.apps.wandb.com ' +
    'crd/weightsandbiases.apps.wandb.com',
    resource_deps=["Application CRD", "Wandb CRD"],
    labels=["Operator-Resources"],
)

k8s_resource(
    'operator-controller-manager',
    'operator-controller-manager',
    objects=[
        'operator-mutating-webhook-configuration:mutatingwebhookconfiguration',
        'operator-validating-webhook-configuration:validatingwebhookconfiguration',
        'operator-controller-manager:serviceaccount',
    ],
    # manifests/generate transitively satisfied via operator-crds-ready → Application CRD / Wandb CRD
    resource_deps=["operator-crds-ready", "third-party-operators"],
    labels=["Operator-Resources"],
)

deps = ['internal', 'pkg', 'api', 'cmd']

local_resource(
    'Watch&Compile',
    binary(),
    deps=deps,
    resource_deps=["manifests", "generate"],
    ignore=['*/*/zz_generated.deepcopy.go'],
    labels=["Operator-Resources"],
)

local_resource(
    'webhook-ready',
    cmd='until kubectl get mutatingwebhookconfiguration operator-mutating-webhook-configuration -o jsonpath=\'{.webhooks[0].clientConfig.caBundle}\' | grep -q .; do echo "Waiting for webhook CA bundle to be injected..."; sleep 2; done && echo "Webhook is ready!"',
    resource_deps=["operator-controller-manager"],
    labels=["Operator-Resources"],
)

if settings.get("installWandb"):
    crdName = read_yaml('./hack/testing-manifests/wandb/' + settings.get('wandbCRD') + '.yaml')['metadata']['name']
    k8s_yaml('./hack/testing-manifests/wandb/' + settings.get('wandbCRD') + '.yaml')
    k8s_resource(
        new_name='Wandb',
        objects=[
            '%s:weightsandbiases' % crdName
        ],
        resource_deps=["webhook-ready"],
        labels=["Operator-Resources"],
    )
    local_resource(
        'Wandb-PortForward-Nginx',
        cmd='echo "Ensuring W&B nginx endpoint is running"',
        serve_cmd='sh -c "until kubectl get svc -n default ' + crdName + '-nginx-proxy >/dev/null 2>&1; do sleep 2; done; exec kubectl port-forward -n default svc/' + crdName + '-nginx-proxy 8080:8080"',
        resource_deps=["Wandb"],
        links=[link('http://localhost:8080', 'W&B nginx')],
        labels=["Operator-Resources"],
    )

if settings.get("installTelemetry"):
    local_resource(
        'vm-crds-ready',
        'kubectl wait --for=condition=established --timeout=120s ' +
        'crd/vmsingles.operator.victoriametrics.com ' +
        'crd/vmagents.operator.victoriametrics.com ' +
        'crd/vlsingles.operator.victoriametrics.com ' +
        'crd/vtsingles.operator.victoriametrics.com ' +
        'crd/vmservicescrapes.operator.victoriametrics.com ' +
        'crd/vmpodscrapes.operator.victoriametrics.com ' +
        'crd/vmnodescrapes.operator.victoriametrics.com',
        resource_deps=["third-party-operators"],
        labels=["Telemetry"],
    )
    local_resource(
        'grafana-crds-ready',
        'kubectl wait --for=condition=established --timeout=120s ' +
        'crd/grafanas.grafana.integreatly.org ' +
        'crd/grafanadatasources.grafana.integreatly.org',
        resource_deps=["third-party-operators"],
        labels=["Telemetry"],
    )
    local_resource(
        'Telemetry-Stack',
        cmd='helm upgrade --install third-party-operators ./deploy/operator ' +
        '--namespace wandb-operator --create-namespace ' +
        '--set=wandb-operator.enabled=false ' +
        '--set=telemetry.enabled=true ' +
        '--set=telemetry.mode=managed ' +
        '--set=telemetry.namespace=default ' +
        '--set=telemetry.ui.grafana.enabled=true',
        resource_deps=["vm-crds-ready", "grafana-crds-ready"],
        labels=["Telemetry"],
    )

    local_resource(
        'Telemetry-PortForward-Grafana',
        cmd='echo "Ensuring Grafana port-forward is running"',
        serve_cmd='sh -c "until kubectl get svc -n default grafana-service >/dev/null 2>&1; do sleep 2; done; exec kubectl port-forward -n default svc/grafana-service 3000:3000"',
        resource_deps=["Telemetry-Stack"],
        links=[link('http://localhost:3000', 'Grafana')],
        labels=["Telemetry"],
    )
    local_resource(
        'Telemetry-PortForward-VictoriaMetrics',
        cmd='echo "Ensuring VictoriaMetrics port-forward is running"',
        serve_cmd='sh -c "until kubectl get svc -n default vmsingle-victoria-instance >/dev/null 2>&1; do sleep 2; done; exec kubectl port-forward -n default svc/vmsingle-victoria-instance 8428:8428"',
        resource_deps=["Telemetry-Stack"],
        links=[link('http://localhost:8428/vmui/', 'VictoriaMetrics UI')],
        labels=["Telemetry"],
    )
    local_resource(
        'Telemetry-PortForward-VictoriaLogs',
        cmd='echo "Ensuring VictoriaLogs port-forward is running"',
        serve_cmd='sh -c "until kubectl get svc -n default vlsingle-victoria-logs >/dev/null 2>&1; do sleep 2; done; exec kubectl port-forward -n default svc/vlsingle-victoria-logs 9428:9428"',
        resource_deps=["Telemetry-Stack"],
        links=[link('http://localhost:9428', 'VictoriaLogs')],
        labels=["Telemetry"],
    )
    local_resource(
        'Telemetry-PortForward-VictoriaTraces',
        cmd='echo "Ensuring VictoriaTraces port-forward is running"',
        serve_cmd='sh -c "until kubectl get svc -n default vtsingle-victoria-traces >/dev/null 2>&1; do sleep 2; done; exec kubectl port-forward -n default svc/vtsingle-victoria-traces 10428:10428"',
        resource_deps=["Telemetry-Stack"],
        links=[link('http://localhost:10428', 'VictoriaTraces')],
        labels=["Telemetry"],
    )

manager_entrypoint = ['/manager', '--log-format=' + settings['logFormat']]
if settings.get("installTelemetry"):
    manager_entrypoint += [
        '--telemetry-enabled=true',
        '--telemetry-mode=managed',
    ]

docker_build_with_restart(
    IMG, '.',
    dockerfile_contents=DOCKERFILE,
    entrypoint=manager_entrypoint,
    only=['./tilt_bin/manager', './hack/testing-manifests/server-manifest'],
    live_update=[
        sync('./tilt_bin/manager', '/manager'),
    ],
)
