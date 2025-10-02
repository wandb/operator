################################################################################
# CONFIGURATION
################################################################################

# Default values
settings = {
    "allowedContexts": [
        "docker-desktop",
        "minikube",
        "kind-kind",
    ],
    "installMinio": True,
    "installStrimzi": False,
    "installWandb": True,
    "wandbCrName": "wandb-default-v1",
    "strimziVersion": "0.44.0",
}

# Override with user settings from tilt-settings.json
settings.update(read_json(
    "tilt-settings.json",
    default={},
))

# Configure global watch settings with a 2-second debounce
# Ignore dist/ files from triggering local_resource rebuilds (but k8s_yaml still watches them)
watch_settings(ignore=["**/.git", "**/*.out", "dist/**"])

# Validate k8s context
if k8s_context() in settings.get("allowedContexts"):
    print("Context is allowed: " + k8s_context())
else:
    fail("Selected context is not in allow list: " + k8s_context())

allow_k8s_contexts(settings.get("allowed_k8s_contexts"))

# Add local bin directory to PATH
os.putenv('PATH', './bin:' + os.getenv('PATH'))

# Load Tilt extensions
load('ext://restart_process', 'docker_build_with_restart')

################################################################################
# COMMON CONSTANTS
################################################################################

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
DIST_DIR = 'dist'

################################################################################
# HELPER FUNCTIONS
################################################################################

def ensure_dist_dir():
    return 'mkdir -p ' + DIST_DIR


def manifests():
    return 'controller-gen ' + CONTROLLERGEN


def generate():
    return 'controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'

def rebuild():
    return 'make'

def vetfmt():
    return 'go vet ./...; go fmt ./...'


def binary():
    return 'CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -o tilt_bin/manager cmd/main.go'

################################################################################
# PREREQUISITES CHECK
################################################################################

installed = local("which kubebuilder")
print("kubebuilder is present:", installed)

DIRNAME = os.path.basename(os. getcwd())

################################################################################
# STEP 1: INITIAL CODE GENERATION
# Generate CRDs and deepcopy methods from API types at startup
################################################################################

local(ensure_dist_dir() + ' && ' + manifests() + generate() + 'mkdir -p ' + DIST_DIR + '/crd-bases && cp config/crd/bases/*.yaml ' + DIST_DIR + '/crd-bases/')

################################################################################
# STEP 2: OPTIONAL DEPENDENCIES
# Install optional components for development/testing
################################################################################

# Install Minio for local S3-compatible storage
if settings.get("installMinio"):
    local('cp ./hack/testing-manifests/minio/minio.yaml ' + DIST_DIR + '/minio.yaml')
    k8s_yaml(DIST_DIR + '/minio.yaml')
    k8s_resource(
        'minio',
        'Minio',
        objects=[
            'minio:service',
            'minio:namespace'
        ]
    )

# Install Strimzi Kafka Operator
if settings.get("installStrimzi"):
    strimzi_version = settings.get("strimziVersion")
    strimzi_url = 'https://github.com/strimzi/strimzi-kafka-operator/releases/download/' + strimzi_version + '/strimzi-cluster-operator-' + strimzi_version + '.yaml'

    local('curl -sL ' + strimzi_url + ' > ' + DIST_DIR + '/strimzi-operator.yaml')
    k8s_yaml(DIST_DIR + '/strimzi-operator.yaml')
    k8s_resource(
        workload='strimzi-cluster-operator',
        new_name='Strimzi Kafka Operator',
        labels=['strimzi']
    )

################################################################################
# STEP 3: DEPLOY CONTROLLER AND RBAC
# Build controller deployment and RBAC from config/default
# This includes CRDs, but we'll apply them again from the watched file below
################################################################################

print("==> Building controller and RBAC manifests from config/default...")
local('kustomize build config/default > ' + DIST_DIR + '/controller-and-rbac.yaml')
k8s_yaml(DIST_DIR + '/controller-and-rbac.yaml')

################################################################################
# STEP 4: GENERATE AND APPLY PATCHED CRDs
# Generate initial patched CRDs file with kustomize
################################################################################

local(
    'echo "==> Generating initial patched CRDs..." && ' +
    'kustomize build config/crd > ' + DIST_DIR + '/patched-crds.yaml'
)

# Apply the patched CRDs file - Tilt watches this file and reapplies when Regenerate-CRDs updates it
# CRDs will be applied twice initially (once from controller-and-rbac.yaml, once from this file)
# but subsequent updates only come from this watched file
k8s_yaml(DIST_DIR + '/patched-crds.yaml', allow_duplicates=True)

################################################################################
# STEP 5: CONFIGURE RBAC RESOURCES
# Group RBAC resources under a single Tilt resource for easier management
################################################################################

k8s_resource(
    new_name='RBAC',
    objects=[
        'operator-manager-role:clusterrole',
        'operator-manager-rolebinding:clusterrolebinding',
        'operator-leader-election-role:role',
        'operator-leader-election-rolebinding:rolebinding'
    ]
)

################################################################################
# STEP 6: WATCH AND COMPILE CONTROLLER
# Automatically recompile controller binary when source code changes
################################################################################

deps = ['controllers', 'pkg', 'cmd/main.go', 'api', 'internal']

local_resource('Watch&Rebuild', rebuild() + "; " + binary(),
               deps=deps, ignore=['*/*/zz_generated.deepcopy.go'])

################################################################################
# STEP 7: WATCH AND REGENERATE CRDs
# Automatically regenerate CRDs when API types change
################################################################################

local_resource('Regenerate-CRDs',
               'echo "==> Regenerating CRDs from api/ types..." && ' +
               manifests() +
               'echo "==> Generated CRDs written to config/crd/bases/:" && ' +
               'ls -1 config/crd/bases/ && ' +
               'echo "==> Copying CRD bases to dist/crd-bases/..." && ' +
               'mkdir -p ' + DIST_DIR + '/crd-bases && ' +
               'cp config/crd/bases/*.yaml ' + DIST_DIR + '/crd-bases/ && ' +
               'echo "==> Building patched CRDs with kustomize..." && ' +
               'kustomize build config/crd > ' + DIST_DIR + '/patched-crds.yaml && ' +
               'echo "==> Patched CRDs written to ' + DIST_DIR + '/patched-crds.yaml" && ' +
               'echo "==> Tilt will automatically reapply CRDs to cluster"',
               deps=['api'],
               ignore=[
                   '*/*/zz_generated.deepcopy.go',
                   'config/crd/bases'
               ])

################################################################################
# STEP 8: DEPLOY TEST CUSTOM RESOURCES
# Install a Wandb CR for testing if enabled
################################################################################

if settings.get("installWandb"):
    local('cp ./hack/testing-manifests/wandb/' + settings.get('wandbCrName') + '.yaml ' + DIST_DIR + '/test-wandb-cr.yaml')
    k8s_yaml(DIST_DIR + '/test-wandb-cr.yaml')
    k8s_resource(
        new_name='Wandb CR',
        objects=[
            settings.get('wandbCrName') + ':weightsandbiases'
        ],
        resource_deps=["operator-controller-manager"]
    )

################################################################################
# STEP 9: BUILD AND DEPLOY CONTROLLER IMAGE
# Build controller image with live update support for fast iteration
################################################################################

docker_build_with_restart(IMG, '.',
                          dockerfile_contents=DOCKERFILE,
                          entrypoint='/manager',
                          only=['./tilt_bin/manager'],
                          live_update=[
                              sync('./tilt_bin/manager', '/manager'),
                          ]
                          )
