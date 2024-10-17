# Testing Wandb on OLM

## Introduction

This guide is intended for new contributors who are beginning to work with Weights and Biases (Wandb) and are looking to test Wandb on the Operator Lifecycle Manager (OLM). 
It provides a step-by-step walkthrough of each component within the OLM environment and demonstrates how to deploy Wandb using it.


## Installing Wandb on OLM via CatalogSource

A `CatalogSource` is a registry in OLM that hosts operators in the form of bundles. For internal releases, we provide a `CatalogSource` image that hosts the bundle required for OLM to deploy the operator.

To create a `CatalogSource` for version `v4.14` in the `openshift-marketplace` namespace, use the following configuration:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: wandb-operator-catalog
  labels:
    app.kubernetes.io/part-of: wandb-operator
spec:
  sourceType: grpc
  image: quay.io/wandb_tools/wandb-operator-index:<release_tag>
  updateStrategy:
    registryPoll:
      interval: 5m
```

Note: If you create the above CatalogSource in the openshift-marketplace namespace, the operator will be available cluster-wide.
If you create it in another namespace, it will only be available within that namespace.


## Installing Wandb on OLM.

Once the CatalogSource is deployed, you should see a pod with the catalogsource name running, indicating that your operator is ready to be served. You can then navigate to the OpenShift UI and install the Weights and Biases operator by selecting it from the list of operators provided by your CatalogSource. Refer to the following image for guidance:

![][operators]

After selecting the operator, proceed with the installation of the Weights and Biases operator. Upon successful installation, you should see the following output:
![][Successful Installation]

Next, you can create a Weights and Biases custom resource (CR) from the UI, and it should be deployed successfully:
![][Wandb Installation]

## Debugging a Failed Wandb Install on OLM

OLM follows a sequential process when installing an operator, which involves three key components. If any of these components fail, the installation will not proceed to the next stage:

These are the three components:
1. Subscription
2. InstallPlan
3. ClusterServiceVersion

If your installation fails, first verify if the InstallPlan exists. If it does, it indicates that the Subscription is functioning correctly. Next, check if the ClusterServiceVersion (CSV) exists. If the CSV is missing, the failure occurred at the InstallPlan level. To diagnose the issue, describe the InstallPlan and check for errors.

If the CSV exists, the failure occurred at the CSV level. In this case, describe the CSV and look for errors to identify the cause.

If the operator installation succeeds but the Wandb custom resource (CR) fails, check the operator logs and the Wandb CR status for further debugging information.

Now, if your install failed you check that installplan exist or not, if it exists means that subscription is
fine. Now you check ClusterServiceVersion exists or not. If it does not exist it means the failure happen at
installplan level. Now describe the installplan and look for error.


[operators]: images/provider.png?raw=true "Community Operators"
[Successful Installation]: images/InstallSuccess.png?raw=true "Successful Installation"
[Wandb Installation]: images/WandbSuccess.png?raw=true "Wandb installed successfully"

## OLM Integration Tests Locally

You can install OLM on any Kubernetes cluster using the following command:

```shell
operator-sdk olm install
```

## OLM Scorecard Tests Locally
OLM scorecard tests can be run locally to validate the functionality of your operator.
