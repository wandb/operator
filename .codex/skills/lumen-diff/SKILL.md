---
name: lumen-diff
description: Compare two folders of `lumen` JSON results by grouping files with matching `metadata` values, ignoring `metadata.scraped_at`, then diffing the key-value pairs in `contents`. Use this when a user wants a repeatable report of missing sources, mismatched content keys, or mismatched content values across two result folders.
---

# Lumen Folder Diff

Use this skill when the user wants to compare two folders of `lumen` tool output.

## What counts as the same source

- A file belongs to a source group keyed by its `metadata` object.
- Ignore `metadata.scraped_at` when deciding whether two files match.
- Files in the same folder may land in the same source group. Treat them as part of one grouped comparison.

## What gets compared

- Only compare the `contents` object.
- By default, only report mismatched keys: keys that only appear on one side of the grouped comparison.
- Optionally include mismatched values for keys present on both sides whose values differ.

## Default workflow

1. Ask for the two folders if they were not provided.
2. Run `scripts/compare_lumen_results.py` from this skill.
3. Return the markdown table for the high-level report.
4. If the user asks for details, also return the JSON output from the script so the exact key and value mismatches are inspectable.

## Script

Run:

```bash
python3 .codex/skills/lumen-diff/scripts/compare_lumen_results.py LEFT_DIR RIGHT_DIR
```

Useful flags:

- `--format markdown` prints the summary table
- `--format json` prints the full structured diff
- `--include-value-diffs` adds value mismatch reporting
- `--glob 'pattern'` limits scanned files in each folder. By default the script scans all files. Example: `--glob '*.json'`

## Output shape

The markdown table expands to one row per normalized source group when there are no diffs, and one row per diff item when there are diffs. A diff item row represents either:

- the compared folder names
- flattened metadata fields so the source is identifiable
- a mismatched key
- or, when `--include-value-diffs` is used, a mismatched value for a shared key

Each row includes:

- file counts per folder
- a `comparison_status` field (`match`, `left_only`, `right_only`, or `diff`)
- a `diff_type` field (`key`, `value`, or empty for matches)
- the relevant `diff_key`
- optional `left_value` and `right_value` columns for value diffs

## Assumptions

- Input files are JSON files with top-level `metadata` and `contents` objects.
- `metadata.scraped_at` is ignored entirely during source matching.
- When multiple files in the same folder share metadata, the script compares grouped `contents` values as multisets per key so duplicate files are preserved instead of overwritten.
- If `--include-value-diffs` is not passed, the summary still includes a `mismatched_value_count` column, but it remains `0` and no value-diff details are computed.
