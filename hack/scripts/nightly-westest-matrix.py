"""Select the supported nightly scenarios without granting cloud access off main."""

import json
import os


LOCAL_SCENARIOS = [
    "local-kind-ingress",
    "local-kind-gateway",
    "local-kind-oidc-pkce",
    "local-kind-oidc-implicit",
    "local-kind-proxy",
    "local-kind-external",
    "local-kind-proxy-auth",
    "local-kind-proxy-tls",
    "local-kind-operator-upgrade",
]
CLOUD_SCENARIOS = [
    # AWS scenarios remain disabled. Legacy GCP scaffolds have no end-to-end validators.
    {"scenario": "gcp-gke-gcs-hmac", "provider": "gcp", "environment": "westest-gcp"},
    {"scenario": "gcp-gke-gcs-byob", "provider": "gcp", "environment": "westest-gcp"},
    {"scenario": "gcp-gke-provider-services", "provider": "gcp", "environment": "westest-gcp"},
    {"scenario": "gcp-gke-gcs-letsencrypt", "provider": "gcp", "environment": "westest-gcp"},
    {"scenario": "gcp-gke-ingress-managed-cert", "provider": "gcp", "environment": "westest-gcp"},
    {"scenario": "gcp-gke-gateway-managed-cert-regional", "provider": "gcp", "environment": "westest-gcp"},
    {"scenario": "gcp-gke-gateway-managed-cert-global", "provider": "gcp", "environment": "westest-gcp"},
]


def select_scenarios(ref, scenario=""):
    local = LOCAL_SCENARIOS.copy()
    cloud = CLOUD_SCENARIOS.copy() if ref == "refs/heads/main" else []
    if scenario:
        if scenario in LOCAL_SCENARIOS:
            local, cloud = [scenario], []
        elif scenario in {entry["scenario"] for entry in CLOUD_SCENARIOS}:
            if ref != "refs/heads/main":
                raise ValueError("Cloud scenarios can only run from refs/heads/main")
            local, cloud = [], [entry for entry in cloud if entry["scenario"] == scenario]
        else:
            raise ValueError(f"Unsupported nightly scenario: {scenario!r}")
    return {
        "local_matrix": json.dumps({"scenario": local}, separators=(",", ":")),
        "cloud_matrix": json.dumps({"include": cloud}, separators=(",", ":")),
        "has_local": str(bool(local)).lower(),
        "has_cloud": str(bool(cloud)).lower(),
    }


if __name__ == "__main__":
    outputs = select_scenarios(os.environ["GITHUB_REF"], os.environ.get("SCENARIO", ""))
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        for name, value in outputs.items():
            output.write(f"{name}={value}\n")
