"""Run the published WESTest image; only allowlisted metadata becomes an artifact."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time


WESTEST_IMAGE_REPOSITORY = "us-docker.pkg.dev/wandb-production/public/westest/westest"


def image_for_tag(tag):
    # Accept only a Docker tag, never a registry, repository, or digest override.
    if not re.fullmatch(r"[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}", tag):
        raise ValueError("Invalid WESTest image tag: supply a tag such as 0.4.0, not an image reference")
    return f"{WESTEST_IMAGE_REPOSITORY}:{tag}"


def write_private(path, content):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.touch(mode=0o600)
    path.write_text(content)
    path.chmod(0o600)


def docker_args(root, provider, env):
    args = ["docker", "run", "--rm", "--init", "--mount",
            f"type=bind,src={root / 'work'},dst=/work",
            "--env", "WESTEST_ARTIFACT_ROOT=/work/artifacts"]
    if provider == "local":
        args += ["--network", "host", "--mount",
                 "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock"]
    elif provider == "aws":
        credentials = "[Sandbox]\n" + "".join(
            f"{key} = {env[name]}\n" for key, name in [
                ("aws_access_key_id", "AWS_ACCESS_KEY_ID"),
                ("aws_secret_access_key", "AWS_SECRET_ACCESS_KEY"),
                ("aws_session_token", "AWS_SESSION_TOKEN"),
            ]
        )
        write_private(root / "auth/aws/credentials", credentials)
        write_private(root / "auth/aws/config", "[profile Sandbox]\nregion = us-west-2\n")
        args += ["--mount", f"type=bind,src={root / 'auth/aws'},dst=/root/.aws,readonly",
                 "--env", "WESTEST_AWS_PROFILE=Sandbox",
                 "--env", "WESTEST_AWS_REGION=us-west-2",
                 "--env", "AWS_EC2_METADATA_DISABLED=true",
                 "--env", "WESTEST_AWS_IAM_PATH=/westest/",
                 "--env", "WESTEST_AWS_ROLE_PERMISSIONS_BOUNDARY_ARN="
                 "arn:aws:iam::770934259321:policy/WestestWorkloadBoundary"]
    elif provider == "gcp":
        # Preserve the refreshable external-account config, including its OIDC
        # credential_source URL and headers. A standalone access token expires.
        source = Path(env["GOOGLE_APPLICATION_CREDENTIALS"])
        write_private(root / "auth/google.json", source.read_text())
        args += ["--mount", f"type=bind,src={root / 'auth/google.json'},dst=/credentials/google.json,readonly",
                 "--env", "GOOGLE_APPLICATION_CREDENTIALS=/credentials/google.json",
                 "--env", "CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE=/credentials/google.json",
                 "--env", "WESTEST_GCP_PROJECT_ID=playground-111",
                 "--env", "WESTEST_GCP_REGION=us-central1"]
    else:
        raise ValueError(f"Unsupported provider: {provider!r}")
    if provider != "local":
        args += ["--env", "WESTEST_CLOUD_OPT_IN=1"]
    if env.get("WESTEST_LICENSE"):
        write_private(root / "auth/license", env["WESTEST_LICENSE"])
        args += ["--mount", f"type=bind,src={root / 'auth/license'},dst=/credentials/license,readonly",
                 "--env", "WANDB_LICENSE_FILE=/credentials/license"]
    return args


def run(env):
    scenario, provider = env["SCENARIO"], env["PROVIDER"]
    if not re.fullmatch(r"[a-z0-9-]+", scenario):
        raise ValueError("Invalid scenario name")
    prefixes = {"local": "local-kind-", "aws": "aws-eks-", "gcp": "gcp-gke-"}
    if provider not in prefixes or not scenario.startswith(prefixes[provider]):
        raise ValueError("Scenario does not match provider")
    if provider != "local" and env["GITHUB_REF"] != "refs/heads/main":
        raise ValueError("Cloud scenarios can only run from refs/heads/main")
    version = env["OPERATOR_VERSION"]
    if not re.fullmatch(r"[a-zA-Z0-9.+-]+", version):
        raise ValueError("Invalid operator chart version")
    upgrade = scenario == "local-kind-operator-upgrade"
    params = ["--param", f"operatorVersion={version}"]
    if upgrade:
        from_version = env.get("OPERATOR_UPGRADE_FROM_VERSION", "")
        if not re.fullmatch(r"[a-zA-Z0-9.+-]+", from_version):
            raise ValueError("Invalid or missing operator upgrade starting chart version")
        if from_version == version:
            raise ValueError("Operator upgrade starting and destination chart versions must differ")
        params = ["--param", f"fromOperatorVersion={from_version}",
                  "--param", f"toOperatorVersion={version}"]
    image = image_for_tag(env["WESTEST_IMAGE_TAG"])
    root = Path(env["RUNNER_TEMP"]) / f"westest-{scenario}"
    (root / "work").mkdir(parents=True, exist_ok=True)
    name = f"westest-{scenario}-{env['GITHUB_RUN_ID']}-{env['GITHUB_RUN_ATTEMPT']}"
    with open(env["GITHUB_OUTPUT"], "a") as output:
        output.write(f"work-root={root}\nartifact-dir={root / 'public'}\n")
    summary = {"scenario": scenario, "provider": provider, "image": image,
               "operator_version": version, "container": name, "exit_code": 1}
    if upgrade:
        summary["operator_from_version"] = from_version
    # Write before starting so an always() step can also stop an interrupted run.
    (root / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    start = time.monotonic()
    try:
        missing = [tool for tool in ("docker", "timeout") if shutil.which(tool) is None]
        if missing:
            print("::error::Required runner tools not found on PATH: " + ", ".join(missing)
                  + ". Install Docker CLI and GNU coreutils before running WESTest.", file=sys.stderr)
            return 1
        args = docker_args(root, provider, env)
        subprocess.run(["docker", "pull", image], check=True)
        subprocess.run(args + [image, "version"], check=True)
        subprocess.run(args + [image, "config", scenario] + params, check=True)
        # SIGINT cancels the test while WESTest tears down on a fresh context.
        # Leave its full cleanup allowance (10m local / 45m cloud) before SIGKILL.
        duration, grace = ("60m", "11m") if provider == "local" else ("75m", "46m")
        if upgrade:
            duration = "90m"
        result = subprocess.run(
            ["timeout", "--signal=INT", f"--kill-after={grace}", duration]
            + args + ["--name", name, image, "run", scenario, "--plain"] + params
        )
        summary["exit_code"] = result.returncode
        return result.returncode
    finally:
        summary["duration_seconds"] = round(time.monotonic() - start)
        (root / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
        with open(env["GITHUB_OUTPUT"], "a") as output:
            output.write("result=" + ("passed" if summary["exit_code"] == 0 else "failed") + "\n")


def collect(root):
    summary = json.loads((root / "summary.json").read_text())
    # Usually already removed by --rm. If the docker client was interrupted,
    # allow the still-running container to perform its normal teardown.
    grace = "660" if summary["provider"] == "local" else "2760"
    if shutil.which("docker"):
        subprocess.run(["docker", "stop", "--time", grace, summary["container"]],
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
    else:
        print("::warning::Docker CLI is unavailable; cannot stop the WESTest container.", file=sys.stderr)
    summary["cleanup_checkpoint_count"] = len(list((root / "work/artifacts/_state").glob("*/*.json")))
    public = root / "public"
    public.mkdir(exist_ok=True)
    # No recursive artifact copy: state, checkpoints, kubeconfigs, logs and
    # run.env can contain credentials in this public repository.
    (public / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
        output.write(f"### {summary['scenario']}\n\n"
                     f"Operator: {summary['operator_version']}\n\n"
                     f"Exit code: {summary['exit_code']}\n\n"
                     f"Cleanup checkpoints remaining: {summary['cleanup_checkpoint_count']}\n\n")
        if summary.get("operator_from_version"):
            output.write(f"Operator upgrade: {summary['operator_from_version']} -> {summary['operator_version']}\n\n")
        if summary["cleanup_checkpoint_count"]:
            output.write("Cleanup needs investigation in the cloud account. Recovery bundles are a follow-up; "
                         "runner-local state is lost when the runner is deleted.\n")
    shutil.rmtree(root / "auth", ignore_errors=True)


if __name__ == "__main__":
    if sys.argv[1:] and sys.argv[1] == "collect":
        collect(Path(sys.argv[2]))
    else:
        sys.exit(run(os.environ))
