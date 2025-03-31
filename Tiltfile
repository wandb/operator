# default values
settings = {
    "allowedContexts": [
        "docker-desktop",
        "minikube",
        "kind-kind",
    ],
    "installMinio": True,
    "installWandb": True,
    "wandbCRD": "default",
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

DOCKERFILE = '''
FROM registry.access.redhat.com/ubi9/ubi-minimal

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
CONTROLLERGEN = 'rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'
DISABLE_SECURITY_CONTEXT = True

def manifests():
    return 'controller-gen ' + CONTROLLERGEN


def generate():
    return 'controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'


def vetfmt():
    return 'go vet ./...; go fmt ./...'

# build to tilt_bin because kubebuilder has a dockerignore for bin/

def binary():
    return 'CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -o tilt_bin/manager cmd/main.go'


installed = local("which kubebuilder")
print("kubebuilder is present:", installed)

DIRNAME = os.path.basename(os. getcwd())

local(manifests() + generate())

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

k8s_yaml(local(manifests() + 'kustomize build config/default'))

k8s_resource(
    new_name='CRD',
    objects=['weightsandbiases.apps.wandb.com:customresourcedefinition', 'applications.apps.wandb.com:customresourcedefinition'])
k8s_resource(
    new_name='RBAC',
    objects=[
        'operator-manager-role:clusterrole',
        'operator-manager-rolebinding:clusterrolebinding',
        'operator-leader-election-role:role',
        'operator-leader-election-rolebinding:rolebinding'
    ]
)

deps = ['controllers', 'pkg', 'cmd/main.go']
deps.append('api')

local_resource('Watch&Compile', generate() + binary(),
               deps=deps, ignore=['*/*/zz_generated.deepcopy.go'])

if settings.get("installWandb"):
    k8s_yaml('./hack/testing-manifests/wandb/' + settings.get('wandbCRD') + '.yaml')
    k8s_resource(
        new_name='Wandb',
        objects=[
            'wandb-default:weightsandbiases'
        ],
        resource_deps=["operator-controller-manager"]
    )

docker_build_with_restart(IMG, '.',
                          dockerfile_contents=DOCKERFILE,
                          entrypoint='/manager',
                          only=['./tilt_bin/manager'],
                          live_update=[
                              sync('./tilt_bin/manager', '/manager'),
                          ]
                          )
