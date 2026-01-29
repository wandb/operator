# default values
settings = {
    "allowedContexts": [
        "docker-desktop",
        "minikube",
        "kind-kind",
        "orbstack",
    ],
    "installMinio": False,
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
load('ext://helm_resource', 'helm_resource', 'helm_repo')
load('ext://cert_manager', 'deploy_cert_manager')

DOCKERFILE = '''
FROM registry.access.redhat.com/ubi9/ubi

ADD tilt_bin/manager /manager
ADD hack/testing-manifests/server-manifest/0.76.1.yaml /0.76.1.yaml

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

if settings.get("installMinio"):
    k8s_yaml('./hack/testing-manifests/minio/minio.yaml')
    k8s_resource(
        'minio',
        'Minio',
        objects=[
            'minio:service',
            'minio:namespace'
        ]
    )

helm_repo(
    'mysql-operator-repo',
    'https://mysql.github.io/mysql-operator',
    labels=["Helm-Repos"],
)

# helm_resource(
#     'mariadb-operator-crds',
#     chart='mariadb-operator-repo/mariadb-operator-crds',
#     resource_deps=['mariadb-operator-repo'],
#     pod_readiness='ignore',
#     labels=["Third-Party-Operators"],
# )

helm_resource(
    'mysql-operator',
    chart='mysql-operator-repo/mysql-operator',
    resource_deps=['mysql-operator-repo'],
    labels=["Third-Party-Operators"],
    flags=['--set-string=image.tag=8.4.7-2.1.9']
)

helm_repo(
    'redis-operator-repo',
    'https://ot-container-kit.github.io/helm-charts/',
    labels=["Helm-Repos"],
)
helm_resource(
    'redis-operator',
    chart='redis-operator-repo/redis-operator',
    resource_deps=['redis-operator-repo'],
    labels=["Third-Party-Operators"],
)

helm_repo(
    'strimzi-repo',
    'https://strimzi.io/charts/',
    labels=["Helm-Repos"],
)
helm_resource(
    'kafka-operator',
    chart='strimzi-repo/strimzi-kafka-operator',
    resource_deps=['strimzi-repo'],
    labels=["Third-Party-Operators"],
)

helm_repo(
    'minio-repo',
    'https://operator.min.io',
    labels=["Helm-Repos"],
)
helm_resource(
    'minio-operator',
    chart='minio-repo/operator',
    resource_deps=['minio-repo'],
    labels=["Third-Party-Operators"],
)

helm_repo(
    'clickhouse-repo',
    'https://helm.altinity.com',
    labels=["Helm-Repos"],
)
helm_resource(
    'clickhouse-operator',
    chart='clickhouse-repo/altinity-clickhouse-operator',
    resource_deps=['clickhouse-repo'],
    labels=["Third-Party-Operators"],
)

helm_repo(
    'victoria-metrics-repo',
    'https://victoriametrics.github.io/helm-charts/',
    labels=["Helm-Repos"],
)
helm_resource(
    'victoria-metrics-operator',
    chart='victoria-metrics-repo/victoria-metrics-operator',
    resource_deps=['victoria-metrics-repo'],
    labels=["Third-Party-Operators"],
)

helm_repo(
    'grafana-repo',
    'https://grafana.github.io/helm-charts',
    labels=["Helm-Repos"],
)
helm_resource(
    'grafana-operator',
    chart='grafana-repo/grafana-operator',
    resource_deps=['grafana-repo'],
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
    labels=["Operator-Resources"],
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

k8s_resource(
    'operator-controller-manager',
    'operator-controller-manager',
    objects=[
        'operator-mutating-webhook-configuration:mutatingwebhookconfiguration',
        'operator-validating-webhook-configuration:validatingwebhookconfiguration',
        'operator-controller-manager:serviceaccount',
    ],
    resource_deps=["manifests", "generate", "redis-operator", "kafka-operator", "minio-operator", "clickhouse-operator"],
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
    k8s_yaml('./hack/testing-manifests/telemetry/victoria-dev.yaml')
    k8s_resource(
        new_name='Victoria-Metrics',
        objects=[
            'victoria-instance:vmsingle',
            'victoria-agent:vmagent',
        ],
        resource_deps=["victoria-metrics-operator"],
        labels=["Telemetry"],
    )
    k8s_resource(
        new_name='Victoria-Logs',
        objects=[
            'victoria-logs:vlsingle',
        ],
        resource_deps=["victoria-metrics-operator"],
        labels=["Telemetry"],
    )
    k8s_resource(
        new_name='Victoria-Traces',
        objects=[
            'victoria-traces:vtsingle',
        ],
        resource_deps=["victoria-metrics-operator"],
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
        resource_deps=["Victoria-Metrics"],
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
        resource_deps=["Victoria-Metrics"],
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
        resource_deps=["Victoria-Metrics"],
        labels=["Telemetry"],
    )
    k8s_yaml('./hack/testing-manifests/telemetry/grafana-dev.yaml')
    k8s_resource(
        new_name='Grafana',
        objects=[
            'grafana:grafana',
        ],
        resource_deps=["grafana-operator"],
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
        resource_deps=["Grafana", "Victoria-Metrics", "Victoria-Logs", "Victoria-Traces"],
        labels=["Telemetry"],
    )

docker_build_with_restart(
    IMG, '.',
    dockerfile_contents=DOCKERFILE,
    entrypoint=['/manager', '--log-format=' + settings['logFormat']],
    only=['./tilt_bin/manager', './hack/testing-manifests/server-manifest/0.76.1.yaml'],
    live_update=[
        sync('./tilt_bin/manager', '/manager'),
    ],
)
