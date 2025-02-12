allowed_k8s_contexts = []

# Remember to add `local.tiltfile` to .gitignore
if os.path.exists('local.tiltfile'):
  load_dynamic('local.tiltfile')

allow_k8s_contexts(allowed_k8s_contexts)

os.putenv('PATH', './bin:' + os.getenv('PATH'))

load('ext://restart_process', 'docker_build_with_restart')

DOCKERFILE = '''
FROM registry.access.redhat.com/ubi9/ubi

ADD tilt_bin/manager /manager

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

def yaml():
    data = local('cd config/manager; kustomize edit set image controller=' + IMG + '; cd ../..; kustomize build config/manager')
    if DISABLE_SECURITY_CONTEXT:
        decoded = decode_yaml_stream(data)
        if decoded:
            for d in decoded:
                # Live update conflicts with SecurityContext, until a better solution, just remove it
                if d["kind"] == "Deployment":
                    if "securityContext" in d['spec']['template']['spec']:
                        d['spec']['template']['spec'].pop('securityContext')
                    for c in d['spec']['template']['spec']['containers']:
                        if "securityContext" in c:
                            c.pop('securityContext')

        return encode_yaml_stream(decoded)
    return data

def manifests():
    return 'controller-gen ' + CONTROLLERGEN

def generate():
    return 'controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'

def vetfmt():
    return 'go vet ./...; go fmt ./...'

# build to tilt_bin beause kubebuilder has a dockerignore for bin/
def binary():
    return 'CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -o tilt_bin/manager main.go'

installed = local("which kubebuilder")
print("kubebuilder is present:", installed)

DIRNAME = os.path.basename(os. getcwd())

local(manifests() + generate())

local_resource('CRD', manifests() + 'kustomize build config/crd | kubectl apply -f -', deps=["api"])

local_resource('RBAC', 'kustomize build config/rbac | kubectl apply -f -', deps=["config/rbac"])

k8s_yaml(yaml())

deps = ['controllers', 'pkg', 'main.go']
deps.append('api')

local_resource('Watch&Compile', generate() + binary(), deps=deps, ignore=['*/*/zz_generated.deepcopy.go'])

local_resource('Sample YAML', 'kustomize build ./config/samples | kubectl apply -f -', deps=["./config/samples"], resource_deps=["controller-manager"])

docker_build_with_restart(IMG, '.',
 dockerfile_contents=DOCKERFILE,
 entrypoint='/manager',
 only=['./tilt_bin/manager'],
 live_update=[
       sync('./tilt_bin/manager', '/manager'),
   ]
)