"""Resolve one published stable Core release and its public server manifest."""

import json
import os
from pathlib import Path
import re
import subprocess
from urllib.parse import quote


STABLE_VERSION = r"(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)"
MANIFEST_REPOSITORY = "us-docker.pkg.dev/wandb-production/public/wandb/server-manifest"


def select_version(releases, requested=""):
    if requested and not re.fullmatch(STABLE_VERSION, requested):
        raise ValueError("Application version must be a stable X.Y.Z release; prereleases are not allowed")
    versions = set()
    for release in releases:
        match = re.fullmatch(rf"server/v({STABLE_VERSION})", release.get("tag_name", ""))
        if (match and release.get("draft") is False and release.get("prerelease") is False
                and release.get("published_at")):
            versions.add(match[1])
    if requested:
        if requested not in versions:
            raise ValueError(f"Application version {requested} is not a published stable Core server release")
        return requested
    if not versions:
        raise ValueError("No published stable Core server releases found")
    return max(versions, key=lambda version: tuple(int(part) for part in version.split(".")))


def command_output(args):
    return subprocess.run(args, check=True, capture_output=True, text=True, timeout=180).stdout.strip()


def resolve(requested=""):
    # Do not retrieve release bodies from the private repository into public logs.
    # Collect metadata from every page before sorting by semantic version.
    try:
        release_lines = command_output([
            "gh", "api", "--paginate", "repos/wandb/core/releases?per_page=100",
            "--jq", ".[] | {tag_name, draft, prerelease, published_at} | @json",
        ])
    except subprocess.CalledProcessError as error:
        raise RuntimeError("Could not read Core releases; check GitHub App contents:read access to wandb/core") from error
    releases = [json.loads(line) for line in release_lines.splitlines() if line.strip()]
    version = select_version(releases, requested)
    tag = f"server/v{version}"
    commit = command_output(["gh", "api", f"repos/wandb/core/commits/{quote(tag, safe='')}", "--jq", ".sha"])
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("Core release did not resolve to a commit SHA")
    # Core creates its GitHub release before publishing the stable manifest.
    # An incomplete release must fail setup, never silently select an older app.
    try:
        manifest = json.loads(command_output([
            "docker", "buildx", "imagetools", "inspect", "--format", "{{json .Manifest}}",
            f"{MANIFEST_REPOSITORY}:{version}",
        ]))
    except subprocess.CalledProcessError as error:
        raise RuntimeError(f"Server manifest for stable release {version} is unavailable; retry after publication completes") from error
    digest = manifest.get("digest", "")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
        raise ValueError("Server manifest did not resolve to a SHA-256 digest")
    return {"app_version": version, "app_core_commit": commit, "app_manifest_digest": digest}


def main(env):
    resolved = resolve(env.get("APP_VERSION", ""))
    with Path(env["GITHUB_OUTPUT"]).open("a") as output:
        for key, value in resolved.items():
            output.write(f"{key}={value}\n")
    with Path(env["GITHUB_STEP_SUMMARY"]).open("a") as summary:
        version = resolved["app_version"]
        summary.write("### Application release\n\n"
                      f"Version: [{version}](https://github.com/wandb/core/releases/tag/server/v{version})\n\n"
                      f"Core commit: `{resolved['app_core_commit']}`\n\n"
                      f"Server manifest digest: `{resolved['app_manifest_digest']}`\n\n")
    print(f"Testing published stable application version: {resolved['app_version']}")


if __name__ == "__main__":
    main(os.environ)
