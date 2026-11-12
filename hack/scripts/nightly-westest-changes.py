"""Avoid scheduled WESTest builds and deployments when the operator commit is unchanged."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
from urllib.parse import urlencode


# Keep this aligned with the workflow job name; it identifies a completed decision.
GATE_JOB_NAME = "Check nightly changes"


def api(endpoint):
    if not shutil.which("gh"):
        raise RuntimeError("GitHub CLI (gh) is required to read nightly history; refusing to start paid work")
    try:
        result = subprocess.run(["gh", "api", endpoint], check=True, capture_output=True,
                                text=True, timeout=60)
        return json.loads(result.stdout)
    except (subprocess.SubprocessError, ValueError) as error:
        raise RuntimeError("Could not read nightly history; refusing to start paid work") from error


def completed_gate(repository, run_id):
    # Include earlier attempts: a later failed rerun must not erase a decision.
    page = 1
    while True:
        jobs = api(f"repos/{repository}/actions/runs/{run_id}/jobs?filter=all&per_page=100&page={page}")["jobs"]
        if any(job["name"] == GATE_JOB_NAME and job["conclusion"] == "success" for job in jobs):
            return True
        if len(jobs) < 100:
            return False
        page += 1


def previous_decision(repository, current_id, current_number):
    # GitHub lists runs newest first. Only schedule events count: a manual run
    # might have tested one scenario or an explicitly supplied older chart.
    for page in range(1, 11):
        query = urlencode({"event": "schedule", "branch": "main", "per_page": 100, "page": page})
        runs = api(f"repos/{repository}/actions/workflows/nightly.yaml/runs?{query}")["workflow_runs"]
        for run in sorted(runs, key=lambda run: run["run_number"], reverse=True):
            if (run["id"] == current_id or run["run_number"] >= current_number
                    or run["event"] != "schedule" or run["head_branch"] != "main"):
                continue
            if run["status"] != "completed":
                raise RuntimeError("An earlier nightly is still active; refusing to start paid work")
            if completed_gate(repository, run["id"]):
                if not re.fullmatch(r"[0-9a-f]{40}", run["head_sha"]):
                    raise ValueError("Invalid commit in nightly history")
                return run
        if len(runs) < 100:
            return None
    # The filtered runs API is limited to 1,000 results. Do not mistake truncated
    # or unusable history for a first run and spend money on an unproven change.
    raise RuntimeError("No completed change check in available nightly history; dispatch manually")


def decide(env):
    if env["GITHUB_EVENT_NAME"] == "workflow_dispatch":
        return {"should_run": "true", "reason": "Manual dispatch bypasses the change check"}
    if env["GITHUB_EVENT_NAME"] != "schedule" or env["GITHUB_REF"] != "refs/heads/main":
        raise ValueError("Scheduled WESTest must run on main")
    sha = env["GITHUB_SHA"]
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("Invalid operator commit")
    previous = previous_decision(env["GITHUB_REPOSITORY"], int(env["GITHUB_RUN_ID"]),
                                 int(env["GITHUB_RUN_NUMBER"]))
    if previous is None:
        return {"should_run": "true", "reason": "First scheduled run with available history"}
    changed = sha != previous["head_sha"]
    return {
        "should_run": str(changed).lower(),
        "reason": "Operator commit changed" if changed else "Operator commit unchanged; skipping build and tests",
        "previous_run_id": str(previous["id"]),
        "previous_sha": previous["head_sha"],
    }


def main(env):
    decision = decide(env)
    with Path(env["GITHUB_OUTPUT"]).open("a") as output:
        for key, value in decision.items():
            output.write(f"{key}={value}\n")
    with Path(env["GITHUB_STEP_SUMMARY"]).open("a") as summary:
        summary.write(f"### Nightly change check\n\n{decision['reason']}.\n\n")
        if decision.get("previous_run_id"):
            summary.write(f"Previous scheduled decision: run {decision['previous_run_id']}\n\n"
                          f"Previous operator commit: `{decision['previous_sha']}`\n\n")
    print(decision["reason"])


if __name__ == "__main__":
    main(os.environ)
