#!/usr/bin/env python3

import argparse
import json
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any


def normalize_metadata(metadata: dict[str, Any]) -> dict[str, Any]:
    if not isinstance(metadata, dict):
        raise ValueError("metadata must be an object")
    return {key: value for key, value in metadata.items() if key != "scraped_at"}


def flatten(obj: Any, prefix: str = "") -> dict[str, Any]:
    if isinstance(obj, dict):
        flattened: dict[str, Any] = {}
        for key in sorted(obj):
            child_prefix = f"{prefix}.{key}" if prefix else key
            flattened.update(flatten(obj[key], child_prefix))
        return flattened
    if isinstance(obj, list):
        return {prefix: json.dumps(obj, sort_keys=True)}
    return {prefix: obj}


def stable_json(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"))


def iter_matching_files(folder: Path, pattern: str) -> list[Path]:
    return sorted(path for path in folder.rglob(pattern) if path.is_file())


def load_folder(folder: Path, pattern: str) -> dict[str, dict[str, Any]]:
    groups: dict[str, dict[str, Any]] = {}

    for path in iter_matching_files(folder, pattern):
        payload = json.loads(path.read_text())
        metadata = normalize_metadata(payload.get("metadata", {}))
        contents = payload.get("contents", {})
        if not isinstance(contents, dict):
            raise ValueError(f"{path}: contents must be an object")

        source_id = stable_json(metadata)
        group = groups.setdefault(
            source_id,
            {
                "metadata": metadata,
                "metadata_flat": flatten(metadata),
                "files": [],
                "key_values": defaultdict(Counter),
            },
        )
        group["files"].append(str(path.relative_to(folder)))
        for key, value in contents.items():
            group["key_values"][key][stable_json(value)] += 1

    return groups


def compare_groups(
    left_name: str,
    right_name: str,
    left_groups: dict[str, dict[str, Any]],
    right_groups: dict[str, dict[str, Any]],
    include_value_diffs: bool,
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []

    for source_id in sorted(set(left_groups) | set(right_groups)):
        left_group = left_groups.get(source_id)
        right_group = right_groups.get(source_id)
        metadata_flat = (
            left_group["metadata_flat"] if left_group else right_group["metadata_flat"]
        )
        left_keys = set(left_group["key_values"]) if left_group else set()
        right_keys = set(right_group["key_values"]) if right_group else set()

        missing_left = sorted(right_keys - left_keys)
        missing_right = sorted(left_keys - right_keys)
        shared_keys = sorted(left_keys & right_keys)

        mismatched_values = []
        if include_value_diffs:
            for key in shared_keys:
                left_values = left_group["key_values"][key]
                right_values = right_group["key_values"][key]
                if left_values != right_values:
                    mismatched_values.append(
                        {
                            "key": key,
                            "left_values": dict(sorted(left_values.items())),
                            "right_values": dict(sorted(right_values.items())),
                        }
                    )

        if left_group and right_group:
            status = "match"
            if missing_left or missing_right or mismatched_values:
                status = "diff"
        elif left_group:
            status = "left_only"
        else:
            status = "right_only"

        base_row = {
            "left_folder": left_name,
            "right_folder": right_name,
            "comparison_status": status,
            "metadata": metadata_flat,
            "left_file_count": len(left_group["files"]) if left_group else 0,
            "right_file_count": len(right_group["files"]) if right_group else 0,
            "left_files": left_group["files"] if left_group else [],
            "right_files": right_group["files"] if right_group else [],
            "missing_from_left": missing_left,
            "missing_from_right": missing_right,
            "mismatched_key_count": len(missing_left) + len(missing_right),
            "mismatched_value_count": len(mismatched_values),
            "mismatched_values": mismatched_values,
        }

        diff_rows = []
        for key in missing_left:
            diff_rows.append(
                {
                    **base_row,
                    "diff_type": "key",
                    "diff_key": key,
                    "left_value": "__missing_on_left__",
                    "right_value": json.dumps(
                        dict(sorted(right_group["key_values"][key].items())),
                        sort_keys=True,
                    ),
                }
            )
        for key in missing_right:
            diff_rows.append(
                {
                    **base_row,
                    "diff_type": "key",
                    "diff_key": key,
                    "left_value": json.dumps(
                        dict(sorted(left_group["key_values"][key].items())),
                        sort_keys=True,
                    ),
                    "right_value": "__missing_on_right__",
                }
            )
        for value_diff in mismatched_values:
            diff_rows.append(
                {
                    **base_row,
                    "diff_type": "value",
                    "diff_key": value_diff["key"],
                    "left_value": json.dumps(value_diff["left_values"], sort_keys=True),
                    "right_value": json.dumps(value_diff["right_values"], sort_keys=True),
                }
            )

        if diff_rows:
            rows.extend(diff_rows)
        else:
            rows.append(
                {
                    **base_row,
                    "diff_type": "",
                    "diff_key": "",
                    "left_value": "",
                    "right_value": "",
                }
            )

    return rows


def render_markdown(rows: list[dict[str, Any]]) -> str:
    metadata_columns = sorted(
        {key for row in rows for key in row["metadata"].keys()}
    )
    headers = [
        "left_folder",
        "right_folder",
        *metadata_columns,
        "comparison_status",
        "diff_type",
        "diff_key",
        "left_value",
        "right_value",
        "left_file_count",
        "right_file_count",
        "mismatched_key_count",
        "mismatched_value_count",
    ]

    def escape(value: Any) -> str:
        if value is None:
            return ""
        text = str(value)
        return text.replace("|", "\\|").replace("\n", " ")

    lines = [
        "| " + " | ".join(headers) + " |",
        "| " + " | ".join(["---"] * len(headers)) + " |",
    ]

    for row in rows:
        cells = [
            row["left_folder"],
            row["right_folder"],
            *[row["metadata"].get(column, "") for column in metadata_columns],
            row["comparison_status"],
            row["diff_type"],
            row["diff_key"],
            row["left_value"],
            row["right_value"],
            row["left_file_count"],
            row["right_file_count"],
            row["mismatched_key_count"],
            row["mismatched_value_count"],
        ]
        lines.append("| " + " | ".join(escape(cell) for cell in cells) + " |")

    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("left_dir", type=Path)
    parser.add_argument("right_dir", type=Path)
    parser.add_argument(
        "--include-value-diffs",
        action="store_true",
        help="compare values for shared keys in addition to key presence",
    )
    parser.add_argument(
        "--glob",
        default="*",
        help="glob used within each folder to select files",
    )
    parser.add_argument(
        "--format",
        choices=("markdown", "json"),
        default="markdown",
    )
    args = parser.parse_args()

    left_groups = load_folder(args.left_dir, args.glob)
    right_groups = load_folder(args.right_dir, args.glob)
    rows = compare_groups(
        args.left_dir.name,
        args.right_dir.name,
        left_groups,
        right_groups,
        args.include_value_diffs,
    )

    if args.format == "json":
        print(json.dumps(rows, indent=2, sort_keys=True))
        return

    print(render_markdown(rows))


if __name__ == "__main__":
    main()
