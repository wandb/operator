"""Run WESTest and stage allowlisted metadata and opt-in external diagnostics."""

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


def docker_args(root, provider, env, operator_repo=None):
    work = root / "work"
    airgap = provider == "local" and env.get("SCENARIO") in {
        "local-kind-airgap", "local-kind-airgap-enforce",
    }
    if airgap:
        work = work.resolve()
    container_work = work if airgap else Path("/work")
    args = ["docker", "run", "--rm", "--init", "--mount",
            f"type=bind,src={work},dst={container_work}",
            "--env", f"WESTEST_ARTIFACT_ROOT={container_work / 'artifacts'}"]
    if airgap:
        # The sibling registry bind-mounts TLS files through Terraform module
        # symlinks into WESTest's asset cache. Keep both the artifacts and cache
        # at the same absolute paths for the host Docker daemon and container.
        args += ["--workdir", str(container_work),
                 "--env", f"WESTEST_WSM_REPO={container_work}",
                 "--env", f"XDG_CACHE_HOME={container_work / 'cache'}"]
    if provider == "local":
        args += ["--network", "host", "--mount",
                 "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock"]
        if operator_repo is not None:
            args += ["--mount", f"type=bind,src={operator_repo},dst=/src/operator,readonly",
                     "--env", "WESTEST_OPERATOR_BUILD_LOCAL=true"]
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
    version = env.get("OPERATOR_VERSION", "")
    repo = env.get("OPERATOR_REPO", "")
    if bool(version) == bool(repo):
        raise ValueError("Supply exactly one of operator-version or operator-repo")
    operator_repo = None
    if repo:
        if provider != "local":
            raise ValueError("Local operator checkouts can only run with local scenarios")
        operator_repo = Path(repo).resolve()
        if "," in str(operator_repo):
            raise ValueError("Operator checkout path cannot contain commas (Docker mount syntax)")
        for required in ["Dockerfile", "deploy/operator/Chart.yaml"]:
            if not (operator_repo / required).is_file():
                raise ValueError(f"Operator checkout is missing {required}")
    elif not re.fullmatch(r"[a-zA-Z0-9.+-]+", version):
        raise ValueError("Invalid operator chart version")
    app_version = env.get("APP_VERSION", "")
    if not re.fullmatch(r"(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", app_version):
        raise ValueError("Application version must be a published stable X.Y.Z release")
    app_digest = env.get("APP_MANIFEST_DIGEST", "")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", app_digest):
        raise ValueError("Missing or invalid application release metadata from workflow setup")
    upgrade = scenario == "local-kind-operator-upgrade"
    if upgrade and operator_repo is not None:
        raise ValueError("Operator upgrade scenarios require published chart versions")
    params = (["--operator-repo", "/src/operator"] if operator_repo is not None
              else ["--param", f"operatorVersion={version}"])
    if upgrade:
        from_version = env.get("OPERATOR_UPGRADE_FROM_VERSION", "")
        if not re.fullmatch(r"[a-zA-Z0-9.+-]+", from_version):
            raise ValueError("Invalid or missing operator upgrade starting chart version")
        if from_version == version:
            raise ValueError("Operator upgrade starting and destination chart versions must differ")
        params = ["--param", f"fromOperatorVersion={from_version}",
                  "--param", f"toOperatorVersion={version}"]
    # The upgrade scenario maps appVersion to both phases, holding the app fixed.
    params += ["--param", f"appVersion={app_version}"]
    image = image_for_tag(env["WESTEST_IMAGE_TAG"])
    root = Path(env["RUNNER_TEMP"]) / f"westest-{scenario}"
    (root / "work").mkdir(parents=True, exist_ok=True)
    name = f"westest-{scenario}-{env['GITHUB_RUN_ID']}-{env['GITHUB_RUN_ATTEMPT']}"
    with open(env["GITHUB_OUTPUT"], "a") as output:
        output.write(f"work-root={root}\nartifact-dir={root / 'public'}\n")
    summary = {"scenario": scenario, "provider": provider, "image": image,
               "operator_version": version, "app_version": app_version,
               "app_manifest_digest": app_digest,
               "container": name, "exit_code": 1}
    if operator_repo is not None:
        summary.update(operator_source="checkout", operator_revision=env.get("GITHUB_SHA", ""))
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
        args = docker_args(root, provider, env, operator_repo)
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


def redact_diagnostic(text):
    # kubectl describe prints literal environment values and may include
    # multiline commands or last-applied annotations. Omit those sections.
    lines = []
    section_indent = None
    for line in text.splitlines(keepends=True):
        indent = len(line) - len(line.lstrip())
        if section_indent is not None:
            if not line.strip() or indent > section_indent:
                continue
            section_indent = None
        match = re.match(r"^(\s*)(Environment(?: Variables from)?|Annotations|Command|Args):", line)
        if match:
            lines.append(f"{match[1]}{match[2]}: [REDACTED]\n")
            section_indent = indent
        else:
            lines.append(line)
    text = "".join(lines)
    text = re.sub(r"-----BEGIN ([A-Z ]*PRIVATE KEY)-----.*?-----END \1-----",
                  "[REDACTED PRIVATE KEY]", text, flags=re.DOTALL)
    text = re.sub(r"(://)[^/\s@]+@", r"\1[REDACTED]@", text)
    return re.sub(
        r'''(?i)(\b(?:[a-z0-9_]*password|passwd|secret(?:[_-]?key)?|token|authorization|api[_-]?key|access[_-]?key)\s*[:=]\s*)(?:(?:Bearer|Basic)\s+[^\s,;]+|"[^"]*"|'[^']*'|[^\s,;]+)''',
        r"\1[REDACTED]", text,
    )


def stage_external_diagnostics(root, public):
    # Exact WESTest 0.8.0 capture names, plus the four datastore pod log names.
    # Never recursively copy the artifact tree: it also contains credentials.
    patterns = [f"*/cluster/{name}" for name in (
        "nodes.txt", "nodes.describe.txt", "pods.txt", "pvc.txt", "events.txt",
    )]
    patterns += [f"*/external-datastores/{name}" for name in (
        "resources.txt", "pods.describe.txt", "pvc.describe.txt", "events.txt",
        "mysql-external-*.logs.txt", "redis-external-*.logs.txt",
        "clickhouse-external-*.logs.txt", "seaweedfs-external-*.logs.txt",
    )]
    source = root / "work/artifacts/local-kind-external"
    count = 0
    for pattern in patterns:
        for path in sorted(source.glob(pattern)):
            # Reject symlinked files and directories, including the source root.
            relative = path.relative_to(root)
            if any((root / Path(*relative.parts[:i])).is_symlink()
                   for i in range(1, len(relative.parts) + 1)) or not path.is_file():
                continue
            target = public / "diagnostics" / path.relative_to(source)
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(redact_diagnostic(path.read_text(errors="replace")))
            target.chmod(0o644)
            count += 1
    if not count:
        print("::warning::No external Kubernetes diagnostics were collected; "
              "the cluster may not have been reachable.", file=sys.stderr)
    return count


def collect(root, external_diagnostics=False):
    summary = json.loads((root / "summary.json").read_text())
    if external_diagnostics and (summary["provider"], summary["scenario"]) != ("local", "local-kind-external"):
        raise ValueError("External diagnostics are only supported for local-kind-external")
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
    shutil.rmtree(public, ignore_errors=True)
    public.mkdir()
    if external_diagnostics:
        summary["diagnostic_file_count"] = stage_external_diagnostics(root, public)
    (public / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    operator = ("checkout " + summary["operator_revision"] if summary.get("operator_source") == "checkout"
                else summary["operator_version"])
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
        output.write(f"### {summary['scenario']}\n\n"
                     f"Operator: {operator}\n\n"
                     f"Application: {summary['app_version']}\n\n"
                     f"Server manifest digest: `{summary['app_manifest_digest']}`\n\n"
                     f"Exit code: {summary['exit_code']}\n\n"
                     f"Cleanup checkpoints remaining: {summary['cleanup_checkpoint_count']}\n\n")
        if summary.get("operator_from_version"):
            output.write(f"Operator upgrade: {summary['operator_from_version']} -> {summary['operator_version']}\n\n")
        if external_diagnostics:
            output.write(f"External diagnostic files: {summary['diagnostic_file_count']}\n\n")
        if summary["cleanup_checkpoint_count"]:
            output.write("Cleanup needs investigation in the cloud account. Recovery bundles are a follow-up; "
                         "runner-local state is lost when the runner is deleted.\n")
    shutil.rmtree(root / "auth", ignore_errors=True)


if __name__ == "__main__":
    if sys.argv[1:] and sys.argv[1] == "collect":
        collect(Path(sys.argv[2]), external_diagnostics=os.environ.get("INCLUDE_EXTERNAL_DIAGNOSTICS") == "true")
    else:
        sys.exit(run(os.environ))
