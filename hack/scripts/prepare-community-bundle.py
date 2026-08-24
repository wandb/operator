#!/usr/bin/env python3
"""Stamp community-operators-prod metadata onto a generated operator-sdk bundle."""

from __future__ import annotations

import argparse
import shutil
import sys
from pathlib import Path


def require_file(path: Path, label: str) -> None:
    if not path.is_file():
        print(f"Error: {label} not found at {path}", file=sys.stderr)
        sys.exit(1)


def set_annotation(annotations_text: str, key: str, value: str) -> str:
    prefix = f"  {key}:"
    line = f"  {key}: {value}"
    lines = annotations_text.splitlines()
    for i, existing in enumerate(lines):
        if existing.startswith(prefix):
            lines[i] = line
            return "\n".join(lines) + "\n"
    # Insert after the opening "annotations:" line when present.
    if lines and lines[0].startswith("annotations:"):
        lines.insert(1, line)
        return "\n".join(lines) + "\n"
    lines.append(line)
    return "\n".join(lines) + "\n"


def set_indented_field(text: str, indent: str, field: str, value: str) -> str:
    prefix = f"{indent}{field}:"
    replacement = f"{indent}{field}: {value}"
    lines = text.splitlines()
    for i, existing in enumerate(lines):
        if existing.startswith(prefix):
            lines[i] = replacement
            return "\n".join(lines) + "\n"
    return text


def set_csv_field(csv_text: str, field: str, value: str) -> str:
    updated = set_indented_field(csv_text, "  ", field, value)
    if updated != csv_text:
        return updated
    # Place before the trailing version field when possible.
    lines = csv_text.splitlines()
    for i, existing in enumerate(lines):
        if existing.startswith("  version:"):
            lines.insert(i, f"  {field}: {value}")
            return "\n".join(lines) + "\n"
    lines.append(f"  {field}: {value}")
    return "\n".join(lines) + "\n"


def set_dockerfile_label(dockerfile_text: str, key: str, value: str) -> str:
    label = f"LABEL {key}={value}"
    lines = dockerfile_text.splitlines()
    prefix = f"LABEL {key}="
    for i, existing in enumerate(lines):
        if existing.startswith(prefix):
            lines[i] = label
            return "\n".join(lines) + "\n"
    lines.append(label)
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle-dir", required=True, type=Path)
    parser.add_argument("--dependencies", required=True, type=Path)
    parser.add_argument("--openshift-versions", required=True)
    parser.add_argument("--replaces", required=True)
    parser.add_argument("--stage-dir", required=True, type=Path)
    parser.add_argument(
        "--container-image",
        default="",
        help="CSV metadata.annotations.containerImage (defaults to the manager image).",
    )
    args = parser.parse_args()

    bundle = args.bundle_dir
    annotations = bundle / "metadata" / "annotations.yaml"
    dockerfile = bundle / "bundle.Dockerfile"
    if not dockerfile.is_file():
        # operator-sdk writes bundle.Dockerfile at the repo root by default.
        dockerfile = bundle.parent / "bundle.Dockerfile"
    manifests = bundle / "manifests"
    require_file(annotations, "bundle annotations")
    require_file(args.dependencies, "dependencies.yaml")
    if not manifests.is_dir():
        print(f"Error: bundle manifests directory not found at {manifests}", file=sys.stderr)
        return 1

    csvs = list(manifests.glob("*.clusterserviceversion.yaml"))
    if len(csvs) != 1:
        print(f"Error: expected one CSV in {manifests}, found {len(csvs)}", file=sys.stderr)
        return 1

    annotations.write_text(
        set_annotation(
            annotations.read_text(),
            "com.redhat.openshift.versions",
            args.openshift_versions,
        )
    )
    shutil.copyfile(args.dependencies, bundle / "metadata" / "dependencies.yaml")
    csv_text = set_csv_field(csvs[0].read_text(), "replaces", args.replaces)
    container_image = args.container_image
    if not container_image:
        for line in csv_text.splitlines():
            stripped = line.strip()
            if stripped.startswith("image:") and ":" in stripped:
                container_image = stripped.split("image:", 1)[1].strip()
                break
    if container_image:
        csv_text = set_indented_field(csv_text, "    ", "containerImage", container_image)
    csvs[0].write_text(csv_text)

    if dockerfile.is_file():
        dockerfile.write_text(
            set_dockerfile_label(
                dockerfile.read_text(),
                "com.redhat.openshift.versions",
                args.openshift_versions,
            )
        )
        # Community layout expects bundle.Dockerfile inside the version directory.
        staged_dockerfile_src = dockerfile
    else:
        staged_dockerfile_src = None

    if args.stage_dir.exists():
        shutil.rmtree(args.stage_dir)
    args.stage_dir.mkdir(parents=True)
    shutil.copytree(bundle / "manifests", args.stage_dir / "manifests")
    shutil.copytree(bundle / "metadata", args.stage_dir / "metadata")
    tests = bundle / "tests"
    if tests.is_dir():
        shutil.copytree(tests, args.stage_dir / "tests")
    if staged_dockerfile_src is not None:
        dest = args.stage_dir / "bundle.Dockerfile"
        text = staged_dockerfile_src.read_text()
        # Paths in the generated Dockerfile are relative to the repo root
        # (./bundle/manifests). Rewrite them for the community version dir.
        text = text.replace("COPY bundle/manifests", "COPY ./manifests")
        text = text.replace("COPY bundle/metadata", "COPY ./metadata")
        text = text.replace("COPY bundle/tests/scorecard", "COPY ./tests/scorecard")
        dest.write_text(text)

    print(f"Staged community bundle at {args.stage_dir}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
