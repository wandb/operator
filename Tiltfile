# default values
settings = {
    "allowedContexts": [
        "docker-desktop",
        "minikube",
        "kind-kind",
        "orbstack",
    ],
    "installWandb": True,
    "wandbCR": "hack/testing-manifests/wandb/.generated/wandb-cr.yaml",
    "wandbOverlays": [],
    "installTelemetry": True,
    "installNginxGateway": True,
    "logFormat": "pretty",  # pretty, text, json
}

if os.path.exists("tilt-settings.json"):
    fail("tilt-settings.json is no longer supported. Migrate to tilt-settings.star (see tilt-settings.sample.star).")

if not os.path.exists("tilt-settings.star"):
    local("cp tilt-settings.sample.star tilt-settings.star")

load("./tilt-settings.star", "SETTINGS")
settings.update(SETTINGS)

# Configure global watch settings with a 2-second debounce
watch_settings(ignore=["**/.git", "**/*.out", "hack/testing-manifests/wandb/.generated/**"])

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

GROUP_WANDB_APP = "Wandb-App"
GROUP_TELEMETRY = "Telemetry"
GROUP_WANDB_OPERATOR = "Wandb-Operator"
GROUP_THIRD_PARTY_OPERATORS = "Third-Party-Operators"

def manifests():
    return 'make manifests'


def generate():
    return 'make generate'


def vetfmt():
    return 'go vet ./...; go fmt ./...'

# build to tilt_bin because kubebuilder has a dockerignore for bin/

def binary():
    return 'CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -o tilt_bin/manager cmd/main.go'

def managed_endpoint_resource(name, anchor_object, dep, local_port, remote_port, link_name, link_url, pod_selector, labels):
    k8s_resource(
        new_name=name,
        objects=[anchor_object],
        discovery_strategy='selectors-only',
        extra_pod_selectors=[pod_selector],
        resource_deps=[dep],
        port_forwards=[
            port_forward(local_port, remote_port, name=link_name),
        ],
        labels=labels,
    )


installed = local("which kubebuilder")
print("kubebuilder is present:", installed)

DIRNAME = os.path.basename(os. getcwd())

local_resource("Operator-Manifests", manifests(), labels=[GROUP_WANDB_OPERATOR])
local_resource("Operator-Generate", generate(), labels=[GROUP_WANDB_OPERATOR])

deploy_cert_manager()

if settings.get("installNginxGateway"):
    local_resource(
        'gateway-api-crds',
        'kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml',
        labels=["Gateway"],
    )
    helm_resource(
        'nginx-gateway-fabric',
        chart='oci://ghcr.io/nginx/charts/nginx-gateway-fabric',
        namespace='nginx-gateway',
        flags=[
            '--create-namespace',
            '--version=2.4.2',
        ],
        resource_deps=['gateway-api-crds'],
        labels=["Gateway"],
    )

local_resource(
    'ThirdParty-Chart-Deps',
    'helm dependency update ./deploy/operator',
    labels=[GROUP_THIRD_PARTY_OPERATORS],
)

third_party_operator_flags = [
    '--set=wandb-operator.enabled=false',
    '--set=telemetry.enabled=false',
    '--create-namespace',
]

helm_resource(
    'ThirdParty-Operators',
    chart='./deploy/operator',
    release_name='third-party-operators',
    resource_deps=['ThirdParty-Chart-Deps', 'WandB-CRDs-Apply'],
    namespace='wandb-operator',
    flags=third_party_operator_flags,
    labels=[GROUP_THIRD_PARTY_OPERATORS],
)

k8s_yaml(local('kustomize build config/tilt-dev'))
k8s_yaml('hack/tilt/endpoint-anchors.yaml')

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
    labels=[GROUP_WANDB_OPERATOR],
)

local_resource(
    'WandB-CRDs-Apply',
    'kubectl apply --server-side=true --force-conflicts --field-manager=helm ' +
    '-f deploy/operator/crds/apps.wandb.com_applications.yaml ' +
    '-f deploy/operator/crds/apps.wandb.com_weightsandbiases.yaml',
    resource_deps=["ThirdParty-Chart-Deps"],
    labels=[GROUP_WANDB_OPERATOR],
)

k8s_resource(
    new_name='Operator-RBAC',
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
    resource_deps=["Operator-Manifests", "Operator-Generate"],
    labels=[GROUP_WANDB_OPERATOR],
)

local_resource(
    'WandB-CRDs-Ready',
    'kubectl wait --for=condition=established --timeout=120s ' +
    'crd/applications.apps.wandb.com ' +
    'crd/weightsandbiases.apps.wandb.com',
    resource_deps=["WandB-CRDs-Apply"],
    labels=[GROUP_WANDB_OPERATOR],
)

k8s_resource(
    workload='operator-controller-manager',
    new_name='Operator-Controller',
    objects=[
        'operator-mutating-webhook-configuration:mutatingwebhookconfiguration',
        'operator-validating-webhook-configuration:validatingwebhookconfiguration',
        'operator-controller-manager:serviceaccount',
    ],
    # manifests/generate transitively satisfied via WandB-CRDs-Ready → WandB-CRDs-Apply
    resource_deps=["WandB-CRDs-Ready", "ThirdParty-Operators"],
    labels=[GROUP_WANDB_OPERATOR],
)

deps = ['internal', 'pkg', 'api', 'cmd']

local_resource(
    'Operator-Build',
    binary(),
    deps=deps,
    resource_deps=["Operator-Manifests", "Operator-Generate"],
    ignore=['*/*/zz_generated.deepcopy.go'],
    labels=[GROUP_WANDB_OPERATOR],
)

local_resource(
    'Operator-Webhook-Ready',
    cmd='until kubectl get mutatingwebhookconfiguration operator-mutating-webhook-configuration -o jsonpath=\'{.webhooks[0].clientConfig.caBundle}\' | grep -q .; do echo "Waiting for webhook CA bundle to be injected..."; sleep 2; done && echo "Webhook is ready!"',
    resource_deps=["Operator-Controller"],
    labels=[GROUP_WANDB_OPERATOR],
)

# Dev-only cleanup helper. Use this before `tilt down` when you want a truly clean
# rebuild of stateful services. It waits for W&B finalizers while operators are still up.
local_resource(
    'Dev-Clean',
    './hack/scripts/tilt-dev-clean.sh',
    auto_init=False,
    labels=[GROUP_WANDB_APP],
)

GENERATED_DIR = 'hack/testing-manifests/wandb/.generated'

def build_wandb_cr():
    overlays = settings.get('wandbOverlays', [])
    components_lines = ''
    for o in overlays:
        components_lines += '  - ../kustomize/overlays/' + o + '\n'

    kustomization = 'apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - ../kustomize/base\n'
    if components_lines:
        kustomization += 'components:\n' + components_lines

    local('mkdir -p ' + GENERATED_DIR)
    local("cat > %s/kustomization.yaml << 'KEOF'\n%sKEOF" % (GENERATED_DIR, kustomization))
    local('kustomize build %s > %s/wandb-cr.yaml' % (GENERATED_DIR, GENERATED_DIR))

if settings.get("installWandb"):
    build_wandb_cr()
    wandbCR = settings.get('wandbCR')
    crName = read_yaml(wandbCR)['metadata']['name']

    k8s_yaml(wandbCR)

    k8s_resource(
        new_name='Wandb',
        objects=[
            '%s:weightsandbiases' % crName
        ],
        resource_deps=["Operator-Webhook-Ready"],
        labels=[GROUP_WANDB_APP],
    )
    managed_endpoint_resource(
        name='Wandb-Endpoint-Nginx',
        anchor_object='wandb-nginx-endpoint-anchor:configmap:default',
        dep='Wandb',
        local_port=8080,
        remote_port=8080,
        link_name='W&B nginx',
        link_url='http://localhost:8080',
        pod_selector={'app.kubernetes.io/name': crName + '-nginx-proxy'},
        labels=[GROUP_WANDB_APP],
    )

if settings.get("installTelemetry"):
    local_resource(
        'Telemetry-CRDs-Ready',
        'kubectl wait --for=condition=established --timeout=120s ' +
        'crd/vmsingles.operator.victoriametrics.com ' +
        'crd/vmagents.operator.victoriametrics.com ' +
        'crd/vlsingles.operator.victoriametrics.com ' +
        'crd/vtsingles.operator.victoriametrics.com ' +
        'crd/vmservicescrapes.operator.victoriametrics.com ' +
        'crd/vmpodscrapes.operator.victoriametrics.com ' +
        'crd/vmnodescrapes.operator.victoriametrics.com ' +
        'crd/grafanas.grafana.integreatly.org ' +
        'crd/grafanadatasources.grafana.integreatly.org' +
        ' && kubectl wait --for=condition=available --timeout=180s -n wandb-operator ' +
        'deploy/third-party-operators-victoria-metrics-operator ' +
        'deploy/third-party-operators-grafana-operator',
        resource_deps=["ThirdParty-Operators"],
        labels=[GROUP_TELEMETRY],
    )
    helm_resource(
        'Telemetry-Stack',
        chart='./deploy/telemetry',
        release_name='telemetry-stack',
        namespace='wandb-operator',
        flags=[
            '--set=enabled=true',
            '--set=namespace=default',
            '--create-namespace',
        ],
        resource_deps=["Telemetry-CRDs-Ready"],
        labels=[GROUP_TELEMETRY],
    )
    managed_endpoint_resource(
        name='Telemetry-Endpoint-Grafana',
        anchor_object='telemetry-grafana-endpoint-anchor:configmap:default',
        dep='Telemetry-Stack',
        local_port=3000,
        remote_port=3000,
        link_name='Grafana',
        link_url='http://localhost:3000',
        pod_selector={'app': 'grafana'},
        labels=[GROUP_TELEMETRY],
    )
    managed_endpoint_resource(
        name='Telemetry-Endpoint-VictoriaMetrics',
        anchor_object='telemetry-victoria-metrics-endpoint-anchor:configmap:default',
        dep='Telemetry-Stack',
        local_port=8428,
        remote_port=8429,
        link_name='VictoriaMetrics UI',
        link_url='http://localhost:8428/vmui/',
        pod_selector={
            'app.kubernetes.io/name': 'vmsingle',
            'app.kubernetes.io/instance': 'victoria-instance',
        },
        labels=[GROUP_TELEMETRY],
    )
    managed_endpoint_resource(
        name='Telemetry-Endpoint-VictoriaLogs',
        anchor_object='telemetry-victoria-logs-endpoint-anchor:configmap:default',
        dep='Telemetry-Stack',
        local_port=9428,
        remote_port=9428,
        link_name='VictoriaLogs',
        link_url='http://localhost:9428',
        pod_selector={
            'app.kubernetes.io/name': 'vlsingle',
            'app.kubernetes.io/instance': 'victoria-logs',
        },
        labels=[GROUP_TELEMETRY],
    )
    managed_endpoint_resource(
        name='Telemetry-Endpoint-VictoriaTraces',
        anchor_object='telemetry-victoria-traces-endpoint-anchor:configmap:default',
        dep='Telemetry-Stack',
        local_port=10428,
        remote_port=10428,
        link_name='VictoriaTraces',
        link_url='http://localhost:10428',
        pod_selector={
            'app.kubernetes.io/name': 'vtsingle',
            'app.kubernetes.io/instance': 'victoria-traces',
        },
        labels=[GROUP_TELEMETRY],
    )

manager_entrypoint = ['/manager', '--log-format=' + settings['logFormat']]
if settings.get("installTelemetry"):
    manager_entrypoint += [
        '--telemetry-enabled=true',
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
