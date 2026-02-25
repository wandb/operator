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
CONTROLLERGEN = 'rbac:roleName=manager-role crd:allowDangerousTypes=true,generateEmbeddedObjectMeta=true webhook paths="{./api/v1,./api/v2,./internal/controller/...}" output:crd:artifacts:config=config/crd/bases'
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

helm_resource(
    'third-party-operators',
    chart='./deploy/operator',
    resource_deps=['helm-dep-update'],
    namespace='wandb-operator',
    flags=[
        '--set=wandb-operator.enabled=false',
        '--create-namespace',
    ],
    labels=["Third-Party-Operators"],
)

k8s_yaml(local('kustomize build config/tilt-dev'))

k8s_resource(
    new_name='Application CRD',
    objects=['applications.apps.wandb.com:customresourcedefinition'],
    labels=["Operator-Resources"],
)

k8s_resource(
    new_name='Wandb CRD',
    objects=['weightsandbiases.apps.wandb.com:customresourcedefinition'],
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
    resource_deps=["manifests", "generate", "operator-crds-ready", "third-party-operators"],
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
    k8s_yaml('./hack/testing-manifests/telemetry/victoria-dev.yaml')
    k8s_resource(
        new_name='Victoria-Metrics',
        objects=[
            'victoria-instance:vmsingle',
            'victoria-agent:vmagent',
        ],
        resource_deps=["vm-crds-ready"],
        labels=["Telemetry"],
    )
    k8s_resource(
        new_name='Victoria-Logs',
        objects=[
            'victoria-logs:vlsingle',
        ],
        resource_deps=["vm-crds-ready"],
        labels=["Telemetry"],
    )
    k8s_resource(
        new_name='Victoria-Traces',
        objects=[
            'victoria-traces:vtsingle',
        ],
        resource_deps=["vm-crds-ready"],
        labels=["Telemetry"],
    )
    k8s_yaml('./hack/testing-manifests/telemetry/wandb-otel-connection-dev.yaml')
    k8s_resource(
        new_name='OTEL-Connection-Secret',
        objects=[
            'wandb-otel-connection:secret',
        ],
        resource_deps=["Victoria-Metrics", "Victoria-Logs", "Victoria-Traces"],
        labels=["Telemetry"],
    )
    k8s_yaml('./hack/testing-manifests/telemetry/kube-metrics-dev.yaml')
    k8s_resource(
        new_name='Kubernetes-Metrics',
        objects=[
            'kubelet-cadvisor:vmnodescrape',
        ],
        resource_deps=["vm-crds-ready", "Victoria-Metrics"],
        labels=["Telemetry"],
    )
    k8s_yaml('./hack/testing-manifests/telemetry/operator-metrics-dev.yaml')
    k8s_resource(
        new_name='Operator-Metrics',
        objects=[
            'wandb-operator:vmservicescrape',
            'clickhouse-operator:vmservicescrape',
            'grafana-operator:vmservicescrape',
            'victoria-metrics-operator:vmservicescrape',
        ],
        resource_deps=["vm-crds-ready", "Victoria-Metrics"],
        labels=["Telemetry"],
    )
    k8s_yaml('./hack/testing-manifests/telemetry/infra-metrics-dev.yaml')
    k8s_resource(
        new_name='Infrastructure-Metrics',
        objects=[
            'mysql-pxc:vmpodscrape',
            'mysql-proxysql:vmpodscrape',
            'kafka-brokers:vmpodscrape',
            'minio-tenant:vmpodscrape',
            'redis:vmpodscrape',
        ],
        resource_deps=["vm-crds-ready", "Victoria-Metrics"],
        labels=["Telemetry"],
    )
    k8s_yaml('./hack/testing-manifests/telemetry/grafana-dev.yaml')
    k8s_resource(
        new_name='Grafana',
        objects=[
            'grafana:grafana',
        ],
        resource_deps=["grafana-crds-ready"],
        port_forwards="3000:3000",
        labels=["Telemetry"],
    )
    k8s_resource(
        new_name='Grafana-Datasources',
        objects=[
            'victoria-metrics:grafanadatasource',
            'victoria-logs:grafanadatasource',
            'victoria-traces:grafanadatasource',
        ],
        resource_deps=["grafana-crds-ready", "Grafana", "Victoria-Metrics", "Victoria-Logs", "Victoria-Traces"],
        labels=["Telemetry"],
    )

docker_build_with_restart(
    IMG, '.',
    dockerfile_contents=DOCKERFILE,
    entrypoint=['/manager', '--log-format=' + settings['logFormat']],
    only=['./tilt_bin/manager', './hack/testing-manifests/server-manifest'],
    live_update=[
        sync('./tilt_bin/manager', '/manager'),
    ],
)
