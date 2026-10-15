SETTINGS = {
    "allowedContexts": ["docker-desktop", "minikube", "kind-kind", "kind-wandb-operator", "orbstack", "crc-admin"],

    # Operator install settings.
    "operatorNamespace": "wandb-operators",
    "openshiftSCC": False,

    # W&B instance settings.
    "includeCR": True,

    # Optional base WeightsAndBiases CR YAML. Tilt patches it with the scalar
    # settings below.
    "crFile": "",
    "wandbName": "wandb",
    "wandbNamespace": "wandb",
    "wandbHostname": "http://localhost:8080",
    "wandbVersion": "0.85.1",
    "size": "dev",
    "retentionPolicy": "detach",
    "licenseFile": "",

    # Download Watchtower and enable Console v2 at /console. Uses gh auth token;
    # authenticate gh with read access to wandb/watchtower.
    "adminConsoleEnabled": False,

    # Creates/reuses ONE W&B user for both human login and Launch. Credentials
    # are saved in <wandbName>-dev-credentials; local output includes the password
    # and API key. CI suppresses credential output. Existing installations can
    # supply WANDB_DEV_PASSWORD or WANDB_DEV_API_KEY in the environment.
    # Enables spec.wandb.enableGlobalAdminAPIKey; a supporting server manifest
    # supplies the operator-generated key to the API and Gorilla migrations.
    "bootstrapUserEnabled": False,
    "bootstrapUsername": "wandb-dev",
    "bootstrapEmail": "wandb-dev@example.test",
    "bootstrapShowCredentials": True,
    "bootstrapCAFile": "",  # Optional CA PEM; Tilt's generated CA is auto-detected.

    # Launch implies bootstrapUserEnabled. Requires uv and a pod-reachable
    # wandbHostname (e.g. http://wandb.localhost:8080), plus CoreDNS rewrites.
    # Kind/Kubernetes only; the upstream agent chart pins UID 1000.
    "launchAgentEnabled": False,
    "launchNamespace": "wandb-launch-test",
    "launchRelease": "wandb-launch-test",
    "launchQueue": "operator-test",
    "launchProject": "launch-test",

    # Default to the published server manifest repository. Use
    # local mode only when developing against repo-local manifest definitions.
    "manifestSource": "published",
    "localManifestPath": "hack/testing-manifests/server-manifest",

    # Choose the local networking path. Tilt installs the matching local
    # dependency automatically: nginx-gateway-fabric for "gateway", or
    # ingress-nginx for "ingress". Ingress mode defaults to
    # http://wandb.localhost:8080 unless wandbHostname is set explicitly.
    "networkMode": "gateway",

    # Defaults for the generated CR. These usually only need to change when
    # matching an existing local GatewayClass or IngressClass.
    "gatewayClass": "nginx",
    "ingressClass": "nginx",

    # Make a non-loopback W&B hostname (for example, wandb.localhost) resolve
    # to the local gateway/ingress from inside the cluster.
    "enableCoreDNSRewrite": True,

    # off, full, or forward. "full" enables VictoriaMetrics/Grafana operators
    # and exposes local telemetry endpoint resources.
    "observabilityMode": "off",

    "logFormat": "pretty",

    # Optional composable Tilt infra settings. External infra installs the
    # local test-infra chart for the selected service. useCustomCA generates
    # test CA material through the normal W&B CR and user ConfigMap inputs.
    "useExternalMysql": False,
    "useExternalRedis": False,
    "useExternalObjectStore": False,
    # useExternalObjectStore publishes direct presigned URLs at this endpoint.
    # Tilt forwards the port locally and rewrites the hostname inside the cluster.
    "externalObjectStoreHostname": "s3.localhost",
    "externalObjectStorePort": 8333,
    "useCustomCA": False,

    # CRC/OpenShift Local uses the crc-admin context. Tilt auto-enables
    # openshiftSCC on CRC; set it explicitly for other OpenShift clusters.
    # "allowedContexts": ["crc-admin"],
    # "openshiftSCC": True,
}
