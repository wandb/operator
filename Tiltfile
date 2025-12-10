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
}

# global settings
settings.update(read_json(
    "tilt-settings.json",
    default={},
))

# Configure global watch settings with a 2-second debounce
watch_settings(ignore=["**/.git", "**/*.out"])

if k8s_context() in settings.get("allowedContexts"):
    print("Context is allowed")
else:
    fail("Selected context is not in allow list")

allow_k8s_contexts(settings.get("allowed_k8s_contexts"))

os.putenv('PATH', './bin:' + os.getenv('PATH'))

load('ext://restart_process', 'docker_build_with_restart')
load('ext://helm_resource', 'helm_resource', 'helm_repo')

DOCKERFILE = '''
FROM registry.access.redhat.com/ubi9/ubi

ADD tilt_bin/manager /manager

RUN mkdir -p /helm/.cache/helm /helm/.config/helm /helm/.local/share/helm && chown -R 65532:65532 /helm

USER 65532:65532

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
    'percona-repo',
    'https://percona.github.io/percona-helm-charts/',
    labels=["Helm-Repos"],
)
helm_resource(
    'mysql-operator',
    chart='percona-repo/pxc-operator',
    labels=["Third-Party-Operators"],
)

helm_repo(
    'redis-operator-repo',
    'https://ot-container-kit.github.io/helm-charts/',
    labels=["Helm-Repos"],
)
helm_resource(
    'redis-operator',
    chart='redis-operator-repo/redis-operator',
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
    labels=["Third-Party-Operators"],
)

k8s_yaml(local('kustomize build config/default'))

k8s_resource(
    new_name='CRD',
    objects=['weightsandbiases.apps.wandb.com:customresourcedefinition', 'applications.apps.wandb.com:customresourcedefinition'],
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

if settings.get("installWandb"):
    k8s_yaml('./hack/testing-manifests/wandb/' + settings.get('wandbCRD') + '.yaml')
    k8s_resource(
        new_name='Wandb',
        objects=[
            'wandb-default:weightsandbiases'
        ],
        resource_deps=["operator-controller-manager"],
        labels=["Operator-Resources"],
    )

docker_build_with_restart(IMG, '.',
                          dockerfile_contents=DOCKERFILE,
                          entrypoint='/manager',
                          only=['./tilt_bin/manager'],
                          live_update=[
                              sync('./tilt_bin/manager', '/manager'),
                          ],
                          )
