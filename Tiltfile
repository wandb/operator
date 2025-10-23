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
    "installWandb": True,
    "installMysqlOperator": False,
    "mysqlOperatorType": "percona",  # or "ndb"
    "installRedisOperator": False,
    "installKafkaOperator": False,
    "autoDeployOperator": True,
    "wandbCrName": "wandb-default-v1",
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
# STEP 2: INFRA COMPONENTS
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
        ],
        labels="infra",
    )

if settings.get("installMysqlOperator"):
    if settings.get("mysqlOperatorType") == "percona":
        print("==> Installing Percona MySQL Operator...")
        local_resource(
            'percona-mysql-op-helm-install',
            cmd='if ! helm repo list | grep -q "^percona"; then ' +
                'helm repo add percona https://percona.github.io/percona-helm-charts/ && ' +
                'helm repo update; ' +
                'fi && ' +
                'helm install percona-mysql-operator percona/pxc-operator --namespace=percona-mysql-operator --create-namespace',
            labels=['infra'],
            auto_init=False,
            trigger_mode=TRIGGER_MODE_MANUAL,
        )

        local_resource(
            'percona-mysql-op-helm-uninstall',
            cmd='helm uninstall percona-mysql-operator --namespace percona-mysql-operator',
            labels=['infra'],
            auto_init=False,
            trigger_mode=TRIGGER_MODE_MANUAL,
        )
    elif settings.get("mysqlOperatorType") == "ndb":
        print("==> Installing MySQL NDB Operator...")
        local_resource(
            'mysql-op-helm-install',
            cmd='if ! helm repo list | grep -q "^ndb-operator-repo"; then ' +
                'helm repo add ndb-operator-repo https://mysql.github.io/mysql-ndb-operator/ && ' +
                'helm repo update; ' +
                'fi && ' +
                'helm install ndb-operator ndb-operator-repo/ndb-operator --namespace=ndb-operator --create-namespace',
            labels=['infra'],
            auto_init=False,
            trigger_mode=TRIGGER_MODE_MANUAL,
        )

        local_resource(
            'mysql-op-helm-uninstall',
            cmd='helm uninstall ndb-operator --namespace ndb-operator',
            labels=['infra'],
            auto_init=False,
            trigger_mode=TRIGGER_MODE_MANUAL,
        )

    else:
        fail("Unknown mysqlOperatorType: " + settings.get("mysqlOperatorType"))

if settings.get("installRedisOperator"):
    print("==> Installing Redis Operator...")
    local_resource(
        'redis-op-helm-install',
        cmd='if ! helm repo list | grep -q \"^ot-helm\"; then ' +
            'helm repo add ot-helm https://ot-container-kit.github.io/helm-charts/ && ' +
            'helm repo update; ' +
            'fi && ' +
            'helm install redis-operator ot-helm/redis-operator --namespace=ot-operators --create-namespace',
        labels=['infra'],
        auto_init=False,
        trigger_mode=TRIGGER_MODE_MANUAL,
    )

    local_resource(
        'redis-op-helm-uninstall',
        cmd='helm uninstall redis-operator --namespace ot-operators',
        labels=['infra'],
        auto_init=False,
        trigger_mode=TRIGGER_MODE_MANUAL,
    )

if settings.get("installKafkaOperator"):
    print("==> Installing Strimzi Kafka Operator...")
    local_resource(
        'kafka-op-helm-install',
        cmd='if ! helm repo list | grep -q \"^strimzi\"; then ' +
            'helm repo add strimzi https://strimzi.io/charts/ && ' +
            'helm repo update; ' +
            'fi && ' +
            'helm install strimzi-kafka-operator strimzi/strimzi-kafka-operator --namespace=kafka --create-namespace --set 'watchNamespaces={default}'',
        labels=['infra'],
        auto_init=False,
        trigger_mode=TRIGGER_MODE_MANUAL,
    )

    local_resource(
        'kafka-op-helm-uninstall',
        cmd='helm uninstall strimzi-kafka-operator --namespace kafka',
        labels=['infra'],
        auto_init=False,
        trigger_mode=TRIGGER_MODE_MANUAL,
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

local_resource('Regenerate-RBAC',
               'echo "==> Regenerating RBAC from controller annotations..." && ' +
               manifests() +
               'echo "==> Building controller and RBAC manifests..." && ' +
               'kustomize build config/default > ' + DIST_DIR + '/controller-and-rbac.yaml && ' +
               'echo "==> RBAC manifests updated in ' + DIST_DIR + '/controller-and-rbac.yaml"',
               deps=['internal/controller'],
               labels="wandb",
               auto_init=False,
               trigger_mode=TRIGGER_MODE_MANUAL)

k8s_resource(
    new_name='RBAC',
    objects=[
        'operator-manager-role:clusterrole',
        'operator-manager-rolebinding:clusterrolebinding',
        'operator-leader-election-role:role',
        'operator-leader-election-rolebinding:rolebinding'
    ],
    labels="wandb"
)

################################################################################
# STEP 6: WATCH AND COMPILE CONTROLLER
# Automatically recompile controller binary when source code changes
################################################################################

deps = ['controllers', 'pkg', 'cmd/main.go', 'api', 'internal']

local_resource('Watch&Rebuild', rebuild() + "; " + binary(),
               deps=deps,
               labels="wandb",
               ignore=['*/*/zz_generated.deepcopy.go', '*/*/*/zz_generated.deepcopy.go'],)

################################################################################
# STEP 7: WATCH AND REGENERATE CRDs
# Automatically regenerate CRDs when API types change
################################################################################

local_resource('Regenerate-CRDs',
               'echo "==> Regenerating CRDs from api/ types..." && ' +
               manifests() +
               generate() +
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
               auto_init=False,
               trigger_mode=TRIGGER_MODE_MANUAL,
               labels="wandb",
               ignore=[
                   '*/*/zz_generated.deepcopy.go',
                   'config/crd/bases'
               ])

################################################################################
# STEP 8: DEPLOY TEST CUSTOM RESOURCES
# Install a Wandb CR for testing if enabled
################################################################################

if settings.get("installWandb"):
    local_resource(
        'Install dev CR',
        cmd='cp ./hack/testing-manifests/wandb/' + settings.get('wandbCrName') + '.yaml ' + DIST_DIR + '/test-wandb-cr.yaml && ' +
            'kubectl apply -f ' + DIST_DIR + '/test-wandb-cr.yaml',
        labels="wandb",
        auto_init=False,
        trigger_mode=TRIGGER_MODE_MANUAL
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
                          ],
                          )

if not settings.get("autoDeployOperator"):
    k8s_resource('operator-controller-manager',
                 auto_init=False,
                 trigger_mode=TRIGGER_MODE_MANUAL)


# ============================================================================
# OLM - Operator Lifecycle Manager
# ============================================================================

# Install OLM on the Kind cluster
local_resource(
    "olm-install",
    cmd="curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.28.0/install.sh | bash -s v0.28.0",
    labels=["OLM"],
    auto_init=False,
    trigger_mode=TRIGGER_MODE_MANUAL,
)

# Check OLM status
local_resource(
    "olm-status",
    cmd="echo '=== OLM Namespaces ===' && kubectl get namespaces olm operators 2>/dev/null || echo 'OLM namespaces not found' && echo '' && echo '=== OLM Pods ===' && kubectl get pods -n olm 2>/dev/null || echo 'No pods in olm namespace' && echo '' && echo '=== Operator Pods ===' && kubectl get pods -n operators 2>/dev/null || echo 'No pods in operators namespace'",
    labels=["OLM"],
    auto_init=False,
    trigger_mode=TRIGGER_MODE_MANUAL,
)

# View OLM package manifests
local_resource(
    "olm-packages",
    cmd="kubectl get packagemanifest -n olm",
    labels=["OLM"],
    auto_init=False,
    trigger_mode=TRIGGER_MODE_MANUAL,
)

# Uninstall OLM from the Kind cluster
local_resource(
    "olm-uninstall",
    cmd="kubectl delete apiservices.apiregistration.k8s.io v1.packages.operators.coreos.com && kubectl delete namespace olm && kubectl delete namespace operators",
    labels=["OLM"],
    auto_init=False,
    trigger_mode=TRIGGER_MODE_MANUAL,
)

