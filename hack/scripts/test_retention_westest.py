"""Offline checks for the retention acceptance matrix and local-only mounts."""

import importlib.util
from pathlib import Path
import tempfile
import unittest

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

    def test_source_mounts_are_read_only_and_local_only(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            binary = root / "westest"
            binary.write_text("test executable")
            env = {"WESTEST_RUNNER_BINARY": str(binary), "OPERATOR_LOCAL_REPO": str(root)}
            args = runner.docker_args(root, "local", env)
            self.assertIn(f"type=bind,src={binary},dst=/usr/local/bin/westest,readonly", args)
            self.assertIn(f"type=bind,src={root},dst=/operator,readonly", args)
            env["WESTEST_RUNNER_BINARY"] = "relative/path"
            with self.assertRaisesRegex(ValueError, "absolute local-only"):
                runner.docker_args(root, "local", env)


if __name__ == "__main__":
    unittest.main()
