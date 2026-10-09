# /// script
# requires-python = ">=3.10"
# dependencies = ["wandb==0.30.0"]
# ///
"""Prepare or smoke-test Launch with the same user created by wandb_dev.py.

Run with uv run so the pinned SDK is isolated from the developer's environment.
"""

import argparse
import base64
import hashlib
import json
import os
import sys
import tempfile
import time
import uuid
from pathlib import Path

from wandb_dev import (
    APIError,
    BootstrapError,
    CredentialStore,
    Kubernetes,
    Server,
    labels,
)

PROFILE = (
    Path(__file__).resolve().parents[1] / "testing-manifests/launch-agent/values.yaml"
)
CA_PATH = "/etc/wandb-launch/ca.crt"


def job_config(namespace):
    return {
        "metadata": {
            "namespace": namespace,
            "labels": {"wandb.ai/test-workload": "launch"},
        },
        "spec": {
            "backoffLimit": 0,
            "activeDeadlineSeconds": 600,
            "ttlSecondsAfterFinished": 600,
            "template": {
                "spec": {
                    "restartPolicy": "Never",
                    "automountServiceAccountToken": False,
                    "containers": [
                        {
                            "name": "launch",
                            "resources": {
                                "requests": {"cpu": "100m", "memory": "256Mi"},
                                "limits": {"cpu": "500m", "memory": "512Mi"},
                            },
                            "env": [
                                {"name": "REQUESTS_CA_BUNDLE", "value": CA_PATH},
                                {"name": "SSL_CERT_FILE", "value": CA_PATH},
                            ],
                            "volumeMounts": [
                                {
                                    "name": "ca",
                                    "mountPath": "/etc/wandb-launch",
                                    "readOnly": True,
                                }
                            ],
                        }
                    ],
                    "volumes": [
                        {"name": "ca", "configMap": {"name": "wandb-launch-ca"}}
                    ],
                }
            },
        },
    }


def launch_values(profile, args, data):
    import copy

    import yaml

    values = copy.deepcopy(profile)
    values["namespace"] = args.launch_namespace
    values["baseUrl"] = data["baseUrl"]
    values["launchConfig"] = yaml.safe_dump(
        {
            "entity": data["entity"],
            "queues": [args.queue],
            "max_jobs": 1,
            "max_schedulers": 0,
            "builder": {"type": "noop"},
        }
    )
    # External Secret/CA changes must roll the agent, whose env is set at startup.
    values["agent"]["podAnnotations"] = {
        "wandb.ai/credential-checksum": hashlib.sha256(
            data["apiKey"].encode()
        ).hexdigest(),
        "wandb.ai/ca-checksum": hashlib.sha256(data["caBundle"].encode()).hexdigest(),
    }
    return values


def prepare(args, kube, data, api, server):
    ownership = labels(args)
    namespace = kube.get("namespace", args.launch_namespace)
    if namespace is None:
        kube.put(
            {
                "apiVersion": "v1",
                "kind": "Namespace",
                "metadata": {
                    "name": args.launch_namespace,
                    "labels": ownership,
                    "annotations": {"wandb.ai/launch-release": args.release},
                },
            }
        )
    elif any(
        namespace["metadata"].get("labels", {}).get(k) != v
        for k, v in ownership.items()
    ):
        raise BootstrapError(
            "Launch namespace is not owned by this test instance; choose an unused launch namespace"
        )
    elif (
        namespace["metadata"].get("annotations", {}).get("wandb.ai/launch-release")
        != args.release
    ):
        raise BootstrapError("Launch namespace belongs to another Helm release")

    kube.put(
        {
            "apiVersion": "v1",
            "kind": "Secret",
            "type": "kubernetes.io/basic-auth",
            "metadata": {
                "name": "wandb-api-key-" + args.release,
                "namespace": args.launch_namespace,
                "labels": ownership,
            },
            "data": {"password": base64.b64encode(data["apiKey"].encode()).decode()},
        }
    )
    kube.put(
        {
            "apiVersion": "v1",
            "kind": "ConfigMap",
            "metadata": {
                "name": "wandb-launch-ca",
                "namespace": args.launch_namespace,
                "labels": ownership,
            },
            "data": {"ca.crt": data["caBundle"]},
        }
    )

    api.create_project(args.project, data["entity"])
    api.create_project("model-registry", data["entity"])
    try:
        result = server.graphql(
            'query BootstrapQueue($entity: String!, $queue: String!) { project(name: "model-registry", entityName: $entity) { runQueue(name: $queue) { id } } }',
            {"entity": data["entity"], "queue": args.queue},
            api_key=data["apiKey"],
        )
    except APIError as exc:
        if not exc.missing_queue:
            raise
        result = {}
    config = job_config(args.launch_namespace)
    if (result.get("project") or {}).get("runQueue"):
        queue = api.run_queue(data["entity"], args.queue)
        actual = queue.default_resource_config
        if isinstance(actual, str):
            actual = json.loads(actual)
        # The API stores queue defaults inside resource_args.
        actual = actual.get("resource_args", {}).get("kubernetes", actual)
        if queue.type != "kubernetes" or actual != config:
            raise BootstrapError(
                "Existing queue configuration differs; choose a new test queue or update it explicitly"
            )
    else:
        api.create_run_queue(
            args.queue, "kubernetes", entity=data["entity"], config=config
        )
    print(f"Launch queue ready: {data['entity']}/{args.queue}; project {args.project}")


SMOKE_PROGRAM = """
import pathlib, tempfile, wandb
with wandb.init() as run:
    run.log({"launch_smoke": 1})
    artifact = wandb.Artifact("launch-smoke-" + run.id, type="test")
    with artifact.new_file("check.txt") as f:
        f.write("launch artifact roundtrip\\n")
    run.log_artifact(artifact).wait()
    downloaded = run.use_artifact(artifact).download(root=tempfile.mkdtemp())
    assert pathlib.Path(downloaded, "check.txt").read_text() == "launch artifact roundtrip\\n"
    run.summary["artifact_roundtrip"] = True
"""


def smoke(args, data, api, image):
    from wandb.sdk.launch import launch_add

    run_id = uuid.uuid4().hex[:8]
    queued = launch_add(
        entity=data["entity"],
        project=args.project,
        queue_name=args.queue,
        docker_image=image,
        resource="kubernetes",
        run_id=run_id,
        entry_point=["python", "-c", SMOKE_PROGRAM],
    )
    print(f"Queued smoke run {run_id} ({queued.id})", flush=True)
    deadline = time.monotonic() + args.timeout
    while time.monotonic() < deadline:
        state = queued.state
        if state in ("failed", "cancelled", "canceled"):
            raise BootstrapError(
                f"Launch queue item {queued.id} entered {state}; inspect agent and Job logs"
            )
        if state not in ("pending", "queued"):
            try:
                run = api.run(f"{data['entity']}/{args.project}/{run_id}")
            except Exception as exc:
                # A queue item can be claimed before the run has been created.
                from wandb.errors import CommError

                if not isinstance(exc, CommError):
                    raise
            else:
                # Api.run caches objects; refresh while waiting for completion.
                run.load(force=True)
                if run.state == "finished":
                    if run.summary.get("launch_smoke") != 1 or not run.summary.get(
                        "artifact_roundtrip"
                    ):
                        raise BootstrapError(
                            "Smoke run finished without the expected metric and artifact roundtrip"
                        )
                    print(f"Launch smoke test passed: {run.url}")
                    return
                if run.state in ("failed", "crashed", "killed"):
                    raise BootstrapError(f"Smoke run ended with state {run.state}")
        time.sleep(3)
    # Remove an unclaimed item so it cannot start unexpectedly after a timeout.
    if queued.state in ("pending", "queued"):
        queued.delete()
    raise BootstrapError(
        f"Smoke test timed out after {args.timeout}s; running Jobs have a 600s deadline"
    )


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("command", choices=["prepare", "smoke"])
    p.add_argument("--context")
    p.add_argument("--namespace", default="wandb")
    p.add_argument("--name", default="wandb")
    p.add_argument("--secret", default="")
    p.add_argument("--launch-namespace", default="wandb-launch-test")
    p.add_argument("--release", default="wandb-launch-test")
    p.add_argument("--project", default="launch-test")
    p.add_argument("--queue", default="operator-test")
    p.add_argument("--timeout", type=int, default=600)
    p.add_argument("--image", default="")
    p.add_argument("--values-output", default="")
    args = p.parse_args()
    try:
        import ssl

        import wandb
        import yaml
        from wandb.sdk.launch.utils import validate_kubernetes_resource_args

        kube = Kubernetes(args.namespace, args.context)
        data = CredentialStore(kube, args).load()
        if not data or not data.get("apiKey"):
            raise BootstrapError("Run wandb_dev.py bootstrap before preparing Launch")
        profile = yaml.safe_load(PROFILE.read_text())
        validate_kubernetes_resource_args(job_config(args.launch_namespace))
        image = args.image or profile["agent"]["image"]
        with tempfile.TemporaryDirectory(prefix="wandb-launch-") as directory:
            ca_file = Path(directory) / "ca.crt"
            ca_file.write_text(data["caBundle"])
            os.environ.update(
                WANDB_BASE_URL=data["baseUrl"],
                WANDB_API_KEY=data["apiKey"],
                WANDB_ENTITY=data["entity"],
                REQUESTS_CA_BUNDLE=str(ca_file),
                SSL_CERT_FILE=str(ca_file),
            )
            server = Server(
                data["baseUrl"], ssl.create_default_context(cafile=str(ca_file))
            )
            viewer = server.viewer(data["apiKey"])
            if not viewer or viewer["id"] != data["userId"]:
                raise BootstrapError(
                    "Stored credential is no longer valid; rerun bootstrap"
                )
            api = wandb.Api(
                overrides={"base_url": data["baseUrl"]},
                api_key=data["apiKey"],
                timeout=30,
            )
            if args.command == "prepare":
                prepare(args, kube, data, api, server)
                if args.values_output:
                    values = launch_values(profile, args, data)
                    values["agent"]["image"] = image
                    target = Path(args.values_output)
                    target.parent.mkdir(parents=True, exist_ok=True)
                    temporary = target.with_suffix(".tmp")
                    temporary.write_text(yaml.safe_dump(values))
                    temporary.replace(target)
            else:
                smoke(args, data, api, image)
    except Exception as exc:
        print(
            str(exc)
            if isinstance(exc, BootstrapError)
            else f"Launch {args.command} failed ({type(exc).__name__}); inspect server/agent logs",
            file=sys.stderr,
        )
        sys.exit(1)


if __name__ == "__main__":
    main()
