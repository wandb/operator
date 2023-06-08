# Configuration Propagation

- [Configuration Propagation](#configuration-propagation)
  - [Controller and ConfigMap Management](#controller-and-configmap-management)
  - [Why Exposing an Endpoint is Better then Direct COnfigMap Modification](#why-exposing-an-endpoint-is-better-then-direct-configmap-modification)

In this design document, we talk about why we implemented a Kubernetes operator
with an endpoint that facilitates updates to a ConfigMap. We will describe the
role of the controller in managing ConfigMaps and why exposing an endpoint is a
better practice than allowing deployments to modify the ConfigMap directly.

## Controller and ConfigMap Management

The custom Kubernetes controller plays a crucial role in managing the ConfigMap
and ensuring that any changes are propagated to the relevant deployments. The
central responsibilities of the controller in relation to the ConfigMap and the
endpoint are:

1. **Watching for ConfigMap updates:** The controller monitors the ConfigMap for
   any changes made to the configuration data and reacts accordingly. When an
   update is detected, the controller triggers a rolling update for the
   associated deployments, ensuring that the new configuration is applied
   seamlessly.

2. **Exposing an update endpoint:** The controller exposes an HTTP endpoint
   (e.g., /update-config) that allows authorized pods to send requests to update
   the ConfigMap. The controller receives these requests, processes them, and
   updates the ConfigMap with the received configuration data. This endpoint
   provides a controlled way for the deployments to request a ConfigMap update.

3. **Securing the update endpoint:** The controller is responsible for
   implementing authentication and authorization mechanisms that ensure only
   authorized pods within the deployments can modify the ConfigMap. This is
   crucial for maintaining security and preventing unauthorized access or
   configuration changes.

4. **Logging and auditability:** By centralizing the control of ConfigMap
   modifications within the controller, it is possible to log all changes to the
   configuration data, along with the origin of the request, timestamps, and
   other relevant information. This can be invaluable for auditing purposes and
   troubleshooting any issues that may arise in the update process.

## Why Exposing an Endpoint is Better then Direct COnfigMap Modification

Creating an endpoint in the controller for updating the ConfigMap offers several
advantages compared to allowing deployments to modify the ConfigMap directly:

1. **Centralized management:** Leveraging the controller to manage ConfigMap
   updates centralizes the process, providing better control over the behavior
   of your operator. This makes it easier to maintain, monitor, and debug any
   potentialissues that may arise during the update process. Centralized
   management also ensures that the ConfigMap's state is managed consistently
   throughout the operator, reducing the possibility of synchronization
   problems.

2. **Security:** Exposing an endpoint in the operator enables the implementation
   of robust authentication and authorization mechanisms to secure the
   ConfigMap. By verifying the identity and permissions of the requesting pods,
   the operator can prevent unauthorized access and limit the potential attack
   surface for malicious actors. Allowing deployments to modify the ConfigMap
   directly would pose a significant security risk.

3. **Abstraction:** An API endpoint for updating the ConfigMap provides a layer
   of abstraction, separating the underlying implementation details of managing
   ConfigMaps from the applications and services that rely on it. This approach
   allows you to evolve your operator's behavior or switch to alternative
   mechanisms for managing configuration without affecting the dependent
   deployments.

4. **Auditability and logging:** By exposing an endpoint in the operator, you
   can collect logs and audit information for all access to the ConfigMap,
   including the origin of each request, the changes made, and the timestamps
   for each event. This comprehensive information can be valuable for auditing,
   troubleshooting, and maintaining visibility into the ConfigMap's activity
   over time.

In summary, using an exposed endpoint in the Kubernetes operator for updating
the ConfigMap provides a more secure, maintainable, and streamlined approach
compared to direct modification by deployments. By centralizing the update
process, implementing security measures, and providing abstraction and logging
capabilities, this approach ensures a scalable and robust solutionfor managing
configuration data in your Kubernetes cluster.
