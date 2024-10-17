
# OLM Bundle Overview

Operator Lifecycle Manager (OLM) has updated the method for storing operator bundles. Bundles are now packaged as container images, which include operator manifests and associated metadata. These images are compliant with OCI (Open Container Initiative) specifications, enabling them to be stored and pulled from any OCI-compliant container registry.

The operator bundle image is designed as a scratch-based (non-runnable) container image. This bundle is utilized by OLM to install operators in OLM-enabled clusters, ensuring a streamlined and automated deployment process.

The directory structure for an operator bundle is as follows:

```
$ tree bundle

bundle
├── ci.yaml
├── manifests
│   ├── apps.wandb.com_weightsandbiases.yaml
│   ├── wandb-operator-manager_rbac.authorization.k8s.io_v1_clusterrole.yaml
│   ├── wandb-operator-manager_rbac.authorization.k8s.io_v1_clusterrolebinding.yaml
│   └── wandb-operator.clusterserviceversion.yaml
├── metadata
│   └── annotations.yaml
└── tests
    └── scorecard
        └── config.yaml

```

Each operator bundle must include a Cluster Service Version (CSV) file. Bundle metadata is stored in `bundle/metadata/annotations.yaml`, which provides essential information about the specific version of the operator available in the registry.

Example content of `annotations.yaml`:

```
$ cat metadata/annotations.yaml

annotations:
  com.redhat.openshift.versions: v4.12
  # Core bundle annotations.
  operators.operatorframework.io.bundle.mediatype.v1: registry+v1
  operators.operatorframework.io.bundle.manifests.v1: manifests/
  operators.operatorframework.io.bundle.metadata.v1: metadata/
  operators.operatorframework.io.bundle.package.v1: wandb-operator
  operators.operatorframework.io.bundle.channels.v1: stable
  operators.operatorframework.io.bundle.channel.default.v1: stable
  operators.operatorframework.io.metrics.builder: operator-sdk-v1.37.0
  operators.operatorframework.io.metrics.mediatype.v1: metrics+v1
  operators.operatorframework.io.metrics.project_layout: go.kubebuilder.io/v3

  # Annotations for testing.
  operators.operatorframework.io.test.mediatype.v1: scorecard+v1
  operators.operatorframework.io.test.config.v1: tests/scorecard/
```

## Building the Operator Bundle Image

You can create a bundle image using the following `Dockerfile`:

```
$ cat bundle.Dockerfile

FROM scratch

# Core bundle labels.
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=wandb-operator
LABEL operators.operatorframework.io.bundle.channels.v1=stable
LABEL operators.operatorframework.io.bundle.channel.default.v1=stable
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.37.0
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v3

# Labels for testing.
LABEL operators.operatorframework.io.test.mediatype.v1=scorecard+v1
LABEL operators.operatorframework.io.test.config.v1=tests/scorecard/

# Copy files to locations specified by labels.
COPY ./manifests /manifests/
COPY ./metadata /metadata/
COPY ./tests/scorecard /tests/scorecard/

LABEL com.redhat.openshift.versions=v4.12
```

To build the image and push it to a public repository, use the following command:

```
docker build -f bundle.Dockerfile -t quay.io/wandb_tools/wandb-operator:v1.0.0 .
```

At this point, you have built the operator bundle image. To integrate it into Red Hat's certification pipeline, you can push the image for Red Hat verification. However, this image is not deployable as a CatalogSource on OpenShift yet.

## Creating a CatalogSource for OpenShift

To deploy the operator on OpenShift, you must create a `CatalogSource` from the bundle image. First, ensure that the `opm` tool is installed.

Run the following command to create the `CatalogSource`:

```
opm index add --container-tool docker --bundles quay.io/wandb_tools/wandb-operator:v1.0.0  --tag quay.io/wandb_tools/wandb-operator-index:v1.0.0
```

This will generate an image that can be used as a `CatalogSource`: `quay.io/wandb_tools/wandb-operator-index:v1.0.0`.

### Updating CSVs for New Versions

If you want to replace an old CSV with a new one in your catalog, you can use the following command to include both bundles:

```
opm index add --container-tool=docker --bundles=quay.io/wandb_tools/wandb-operator:v1.0.0,quay.io/wandb_tools/wandb-operator:v1.0.1 --tag  quay.io/wandb_tools/wandb-operator-index:v1.0.1
```

This command creates an image (`v1.0.1`) that supersedes the older CSV (`v1.0.0`).

## OLM Catalog Source Upgrade Chain

When managing multiple versions of an operator, it is crucial for OLM to automatically upgrade to the latest version. To achieve this, the `CSV` must include a `replaces` field, indicating which previous CSV version it is replacing.

Consider the following example where the initial version (`v1.0.0`) of the operator is created:

```
opm index add --container-tool docker --bundles quay.io/wandb_tools/wandb-operator:v1.0.0  --tag quay.io/wandb_tools/wandb-operator-index:v1.0.0
```

Once a new version (`v1.0.1`) is released, you can specify the `--from-index` option to ensure the upgrade chain links to the previous version:

```
opm index add --container-tool=docker --bundles=quay.io/wandb_tools/wandb-operator:v1.0.1 --from-index=quay.io/wandb_tools/wandb-operator-index:v1.0.0 --tag quay.io/wandb_tools/wandb-operator-index:v1.0.1

```

This ensures that `v1.0.1` will automatically replace `v1.0.0` when upgrading.

You can continue this process for subsequent versions, and `opm` will manage the version upgrade chain via the `CSV` definitions.
