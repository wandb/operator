"""Offline checks for the retention matrix and published-to-candidate upgrades."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

REPO = Path(__file__).resolve().parents[2]


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


matrix = load("retention_matrix", REPO / "hack/scripts/retention-westest-matrix.py")
runner = load("retention_runner", REPO / ".github/actions/westest-run/run.py")


class RetentionAcceptanceTests(unittest.TestCase):
    def test_full_matrix_covers_all_policies_propagations_and_argo_modes(self):
        full = matrix.scenarios("full")
        self.assertEqual(len(full), 22)
        self.assertEqual(len(set(full)), 22)
        self.assertTrue(set(matrix.scenarios("pr")).issubset(full))
        for policy in ("detach", "purge", "mixed"):
            for propagation in ("background", "foreground", "orphan"):
                self.assertIn(f"local-kind-retention-{policy}-{propagation}", full)
        for policy in ("detach", "purge"):
            for operation in ("prune", "cascade"):
                for tracking in ("annotation", "combined"):
                    self.assertIn(f"local-kind-retention-argocd-{policy}-{operation}-{tracking}", full)
        self.assertIn("local-kind-retention-upgrade", full)
        for suffix in ("gateway-detach-foreground", "gateway-detach-orphan",
                       "external-gateway-detach-foreground", "external-gateway-purge-foreground"):
            self.assertIn(f"local-kind-retention-{suffix}", full)

    def test_invalid_profile_fails(self):
        with self.assertRaises(ValueError):
            matrix.scenarios("nightyly")

    def test_upgrade_uses_published_baseline_and_selected_candidate(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            checkout = root / "operator"
            (checkout / "deploy/operator").mkdir(parents=True)
            (checkout / "Dockerfile").write_text("FROM scratch\n")
            (checkout / "deploy/operator/Chart.yaml").write_text("name: operator\n")
            for source in ("checkout", "published"):
                with self.subTest(source=source):
                    env = {
                        "RUNNER_TEMP": str(root / source), "GITHUB_OUTPUT": str(root / "outputs"),
                        "GITHUB_STEP_SUMMARY": str(root / "step-summary"),
                        "GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "1", "GITHUB_SHA": "abc123",
                        "SCENARIO": "local-kind-retention-upgrade", "PROVIDER": "local",
                        "OPERATOR_REPO": str(checkout) if source == "checkout" else "",
                        "OPERATOR_VERSION": "" if source == "checkout" else "2.0.0-test",
                        "OPERATOR_UPGRADE_FROM_VERSION": "2.0.0-beta.9",
                        "WESTEST_IMAGE_TAG": "test-retention", "APP_VERSION": "0.85.1",
                        "APP_MANIFEST_DIGEST": "sha256:" + "b" * 64,
                    }
                    with patch.object(runner.shutil, "which", return_value="/tool"):
                        with patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)) as command:
                            self.assertEqual(runner.run(env), 0)
                    calls = [call.args[0] for call in command.call_args_list]
                    config = next(args for args in calls if "config" in args)
                    run = next(args for args in calls if args[0] == "timeout")
                    for args in (config, run):
                        self.assertIn("fromOperatorVersion=2.0.0-beta.9", args)
                        self.assertIn("appVersion=0.85.1", args)
                        self.assertFalse(any(arg.startswith("operatorVersion=") for arg in args))
                        self.assertFalse(any("dst=/usr/local/bin/westest" in arg for arg in args))
                        if source == "checkout":
                            self.assertIn("--operator-repo", args)
                            self.assertIn(f"type=bind,src={checkout.resolve()},dst=/src/operator,readonly", args)
                            self.assertIn("WESTEST_OPERATOR_BUILD_LOCAL=true", args)
                            self.assertFalse(any(arg.startswith("toOperatorVersion=") for arg in args))
                        else:
                            self.assertIn("toOperatorVersion=2.0.0-test", args)
                            self.assertNotIn("--operator-repo", args)
                    self.assertEqual(run[:4], ["timeout", "--signal=INT", "--kill-after=11m", "90m"])
                    run_root = root / source / "westest-local-kind-retention-upgrade"
                    with patch.dict(os.environ, env), patch.object(runner.subprocess, "run"):
                        runner.collect(run_root)
                    summary = json.loads((run_root / "public/summary.json").read_text())
                    self.assertEqual(summary["operator_from_version"], "2.0.0-beta.9")
                    candidate = "checkout abc123" if source == "checkout" else "2.0.0-test"
                    self.assertIn(f"Operator upgrade: 2.0.0-beta.9 -> {candidate}", (root / "step-summary").read_text())
                    if source == "checkout":
                        self.assertEqual(summary["operator_revision"], "abc123")
                    else:
                        self.assertEqual(summary["operator_version"], "2.0.0-test")


if __name__ == "__main__":
    unittest.main()
