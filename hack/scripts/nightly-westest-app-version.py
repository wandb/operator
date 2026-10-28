"""Resolve one stable application version from the public server-manifest registry."""

import hashlib
import json
import os
from pathlib import Path
import re
from urllib.error import URLError
from urllib.parse import urljoin, urlsplit
from urllib.request import Request, urlopen


STABLE_VERSION = r"(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)"
MANIFEST_REPOSITORY = "us-docker.pkg.dev/wandb-production/public/wandb/server-manifest"
REGISTRY_API = "https://us-docker.pkg.dev/v2/wandb-production/public/wandb/server-manifest"
MANIFEST_TYPES = (
    "application/vnd.oci.image.index.v1+json",
    "application/vnd.oci.image.manifest.v1+json",
    "application/vnd.docker.distribution.manifest.list.v2+json",
    "application/vnd.docker.distribution.manifest.v2+json",
)


def select_version(tags, requested=""):
    if requested and not re.fullmatch(STABLE_VERSION, requested):
        raise ValueError("Application version must be a stable X.Y.Z release; prereleases are not allowed")
    versions = {tag for tag in tags if re.fullmatch(STABLE_VERSION, tag)}
    if requested:
        if requested not in versions:
            raise ValueError(f"Application version {requested} has no published stable server manifest")
        return requested
    if not versions:
        raise ValueError("No published stable server manifests found")
    return max(versions, key=lambda version: tuple(int(part) for part in version.split(".")))


def fetch(url, accept="application/json"):
    # Direct anonymous HTTPS requests never load GitHub or Docker credentials.
    request = Request(url, headers={"Accept": accept})
    try:
        with urlopen(request, timeout=180) as response:
            return response.read(), response.headers
    except URLError as error:
        raise RuntimeError(f"Could not read public server manifest metadata at {url}") from error


def published_tags():
    endpoint = f"{REGISTRY_API}/tags/list"
    url = f"{endpoint}?n=1000"
    tags, visited = set(), set()
    while url:
        if url in visited or urlsplit(url)[:3] != urlsplit(endpoint)[:3]:
            raise ValueError("Invalid server manifest tag pagination link")
        visited.add(url)
        body, headers = fetch(url)
        page_tags = json.loads(body).get("tags")
        if not isinstance(page_tags, list) or any(not isinstance(tag, str) for tag in page_tags):
            raise ValueError("Invalid server manifest tag listing")
        tags.update(page_tags)
        # GAR currently returns all tags; honor Registry V2 pagination if present.
        link = headers.get("Link", "")
        next_pages = re.findall(r'<([^>]+)>;\s*rel="next"', link)
        if link and len(next_pages) != 1:
            raise ValueError("Invalid server manifest tag pagination link")
        url = urljoin(url, next_pages[0]) if next_pages else ""
    return tags


def resolve(requested=""):
    # Stable X.Y.Z manifests are published only after Core's release job succeeds.
    version = select_version(published_tags(), requested)
    body, headers = fetch(f"{REGISTRY_API}/manifests/{version}", ", ".join(MANIFEST_TYPES))
    manifest = json.loads(body)
    if manifest.get("schemaVersion") != 2 or manifest.get("mediaType") not in MANIFEST_TYPES:
        raise ValueError("Invalid server manifest document")
    digest = headers.get("Docker-Content-Digest", "")
    if digest != f"sha256:{hashlib.sha256(body).hexdigest()}":
        raise ValueError("Server manifest digest is missing or does not match its contents")
    return {"app_version": version, "app_manifest_digest": digest}


def main(env):
    resolved = resolve(env.get("APP_VERSION", ""))
    with Path(env["GITHUB_OUTPUT"]).open("a") as output:
        for key, value in resolved.items():
            output.write(f"{key}={value}\n")
    with Path(env["GITHUB_STEP_SUMMARY"]).open("a") as summary:
        version = resolved["app_version"]
        summary.write("### Application release\n\n"
                      f"Version: {version}\n\n"
                      f"Source: `{MANIFEST_REPOSITORY}`\n\n"
                      f"Server manifest digest: `{resolved['app_manifest_digest']}`\n\n")
    print(f"Testing published stable application version: {resolved['app_version']}")


if __name__ == "__main__":
    main(os.environ)
