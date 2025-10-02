# default values
settings = {
    "allowedContexts": [
        "docker-desktop",
        "minikube",
        "kind-kind",
    ],
    "installMinio": True,
    "installWandb": True,
    "wandbCrName": "wandb-default-v1",
}

# global settings
settings.update(read_json(
    "tilt-settings.json",
    default={},
))

# Configure global watch settings with a 2-second debounce
# Ignore patched-crds.yaml from triggering local_resource rebuilds (but k8s_yaml still watches it)
watch_settings(ignore=["**/.git", "**/*.out", "config/crd/patched-crds.yaml"])

if k8s_context() in settings.get("allowedContexts"):
    print("Context is allowed: " + k8s_context())
else:
    fail("Selected context is not in allow list: " + k8s_context())

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

# Build controller deployment and RBAC from config/default
# This includes CRDs, but we'll apply them again from the watched file below
print("==> Building controller and RBAC manifests from config/default...")
default_yaml = local('kustomize build config/default')
k8s_yaml(default_yaml)

# Generate initial patched CRDs file
local(
    'echo "==> Generating initial patched CRDs..." && ' +
    'kustomize build config/crd > config/crd/patched-crds.yaml'
)

# Apply the patched CRDs file - Tilt watches this file and reapplies when Regenerate-CRDs updates it
# CRDs will be applied twice initially (once from default_yaml, once from this file)
# but subsequent updates only come from this watched file
k8s_yaml('config/crd/patched-crds.yaml', allow_duplicates=True)

# Note: We don't create a separate k8s_resource for the CRDs from patched-crds.yaml
# because they're already tracked from the default_yaml above
# The CRD resources will appear as:
# - weightsandbiases.apps.wandb.com (from config/default)
# - applications.apps.wandb.com (from config/default)
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

local_resource('Regenerate-CRDs',
               'echo "==> Regenerating CRDs from api/ types..." && ' +
               manifests() +
               'echo "==> Generated CRDs written to config/crd/bases/:" && ' +
               'ls -1 config/crd/bases/ && ' +
               'echo "==> Building patched CRDs with kustomize..." && ' +
               'kustomize build config/crd > config/crd/patched-crds.yaml && ' +
               'echo "==> Patched CRDs written to config/crd/patched-crds.yaml" && ' +
               'echo "==> Tilt will automatically reapply CRDs to cluster"',
               deps=['api'],
               ignore=[
                   '*/*/zz_generated.deepcopy.go',
                   'config/crd/bases',
                   'config/crd/patched-crds.yaml'
               ])

if settings.get("installWandb"):
    testing_yaml = read_file('./hack/testing-manifests/wandb/' + settings.get('wandbCrName') + '.yaml')
    #print(testing_yaml)
    k8s_yaml(testing_yaml)
    k8s_resource(
        new_name='Wandb',
        objects=[
            settings.get('wandbCrName') + ':weightsandbiases'
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
