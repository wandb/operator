import sys
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from wandb_dev import APIError, BootstrapError, labels
from wandb_launch import job_config, launch_values, prepare, smoke


class LaunchPreparationTests(unittest.TestCase):
    def setUp(self):
        self.args = SimpleNamespace(
            name="wandb",
            namespace="wandb",
            launch_namespace="wandb-launch-test",
            release="wandb-launch-test",
            project="launch-test",
            queue="operator-test",
        )
        self.data = {"apiKey": "test-key", "caBundle": "test-ca", "entity": "tester"}
        self.kube = Mock()
        self.kube.get.return_value = None
        self.api = Mock()
        self.server = Mock()
        self.server.graphql.return_value = {"project": {"runQueue": None}}

    def test_fresh_queue_uses_existing_identity_and_external_secret(self):
        prepare(self.args, self.kube, self.data, self.api, self.server)
        self.api.create_run_queue.assert_called_once_with(
            "operator-test",
            "kubernetes",
            entity="tester",
            config=job_config("wandb-launch-test"),
        )
        objects = [call.args[0] for call in self.kube.put.call_args_list]
        credential = next(obj for obj in objects if obj["kind"] == "Secret")
        self.assertEqual(
            credential["metadata"]["name"], "wandb-api-key-wandb-launch-test"
        )
        self.assertEqual(credential["data"], {"password": "dGVzdC1rZXk="})

    def test_existing_matching_queue_is_reused(self):
        self.server.graphql.return_value = {"project": {"runQueue": {"id": "queue-1"}}}
        self.api.run_queue.return_value = SimpleNamespace(
            type="kubernetes",
            default_resource_config={
                "resource_args": {"kubernetes": job_config("wandb-launch-test")}
            },
        )
        prepare(self.args, self.kube, self.data, self.api, self.server)
        self.api.create_run_queue.assert_not_called()

    def test_server_queue_not_found_response_creates_queue(self):
        self.server.graphql.side_effect = APIError(
            404, "queue lookup", missing_queue=True
        )
        prepare(self.args, self.kube, self.data, self.api, self.server)
        self.api.create_run_queue.assert_called_once()

    def test_other_lookup_failures_do_not_create_queue(self):
        self.server.graphql.side_effect = APIError(404, "project not found")
        with self.assertRaises(APIError):
            prepare(self.args, self.kube, self.data, self.api, self.server)
        self.api.create_run_queue.assert_not_called()

    def test_existing_queue_in_another_namespace_is_not_overwritten(self):
        self.server.graphql.return_value = {"project": {"runQueue": {"id": "queue-1"}}}
        self.api.run_queue.return_value = SimpleNamespace(
            type="kubernetes", default_resource_config=job_config("production")
        )
        with self.assertRaisesRegex(BootstrapError, "configuration differs"):
            prepare(self.args, self.kube, self.data, self.api, self.server)
        self.api.create_run_queue.assert_not_called()

    def test_existing_unowned_namespace_is_not_modified(self):
        self.kube.get.return_value = {"metadata": {"name": "wandb-launch-test"}}
        with self.assertRaisesRegex(BootstrapError, "not owned"):
            prepare(self.args, self.kube, self.data, self.api, self.server)
        self.kube.put.assert_not_called()

    def test_different_release_is_rejected(self):
        self.kube.get.return_value = {
            "metadata": {
                "labels": labels(self.args),
                "annotations": {"wandb.ai/launch-release": "other"},
            }
        }
        with self.assertRaisesRegex(BootstrapError, "another Helm release"):
            prepare(self.args, self.kube, self.data, self.api, self.server)
        self.kube.put.assert_not_called()

    def test_credential_rotation_changes_rollout_without_exposing_key(self):
        profile = {"agent": {"image": "test-image"}}
        data = dict(self.data, baseUrl="https://wandb.example")
        with patch.dict(sys.modules, {"yaml": SimpleNamespace(safe_dump=str)}):
            first = launch_values(profile, self.args, data)
            second = launch_values(profile, self.args, dict(data, apiKey="new-key"))
        self.assertNotEqual(
            first["agent"]["podAnnotations"], second["agent"]["podAnnotations"]
        )
        self.assertNotIn("test-key", str(first))
        self.assertNotIn("new-key", str(second))
        self.assertEqual(profile, {"agent": {"image": "test-image"}})


class SmokeTests(unittest.TestCase):
    def setUp(self):
        self.args = SimpleNamespace(project="launch-test", queue="test", timeout=30)
        self.data = {"entity": "tester"}
        self.queued = Mock(id="queue-1", state="running")
        self.launch_add = Mock(return_value=self.queued)
        self.api = Mock()
        self.modules = patch.dict(
            sys.modules,
            {"wandb.sdk.launch": SimpleNamespace(launch_add=self.launch_add)},
        )
        self.modules.start()
        self.addCleanup(self.modules.stop)

    def test_refreshes_cached_run_until_metric_and_artifact_are_available(self):
        run = Mock(state="running", summary={}, url="https://wandb.example/run")
        self.api.run.return_value = run

        def refresh(force):
            self.assertTrue(force)
            if run.load.call_count == 2:
                run.state = "finished"
                run.summary = {"launch_smoke": 1, "artifact_roundtrip": True}

        run.load.side_effect = refresh
        with patch("wandb_launch.time.sleep"):
            smoke(self.args, self.data, self.api, "test-image")
        self.assertEqual(run.load.call_count, 2)
        self.queued.delete.assert_not_called()

    def test_finished_run_without_artifact_is_not_a_pass(self):
        self.api.run.return_value = Mock(state="finished", summary={"launch_smoke": 1})
        with self.assertRaisesRegex(BootstrapError, "artifact roundtrip"):
            smoke(self.args, self.data, self.api, "test-image")

    def test_timed_out_unclaimed_job_is_removed(self):
        self.args.timeout = 0
        self.queued.state = "pending"
        with self.assertRaisesRegex(BootstrapError, "timed out"):
            smoke(self.args, self.data, self.api, "test-image")
        self.queued.delete.assert_called_once()


if __name__ == "__main__":
    unittest.main()
