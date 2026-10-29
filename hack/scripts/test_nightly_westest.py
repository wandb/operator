"""Offline regression tests for nightly selection and the Docker credential boundary."""

import hashlib
import importlib.util
import io
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


matrix = load("matrix", REPO / "hack/scripts/nightly-westest-matrix.py")
runner = load("runner", REPO / ".github/actions/westest-run/run.py")
app_release = load("app_release", REPO / "hack/scripts/nightly-westest-app-version.py")


class ApplicationReleaseTests(unittest.TestCase):
    @staticmethod
    def tag_page(tags, **headers):
        return json.dumps({"tags": tags}).encode(), headers

    def test_selects_highest_stable_server_version_not_tag_order(self):
        tags = ["0.9.9", "0.85.1", "0.86.0-rc.1", "0.86.0-beta.1", "0.86.0-daily.123",
                "0.86.0-pre.123", "latest", "buildcache", "v10.0.0", "0.85.0", "00.99.0"]
        self.assertEqual(app_release.select_version(tags), "0.85.1")
        self.assertEqual(app_release.select_version(tags, "0.85.0"), "0.85.0")

    def test_manual_override_must_be_a_published_stable_release(self):
        for version in ["0.85.2", "0.86.0-rc.1", "0.86.0-beta.1", "0.86.0-daily.1",
                        "0.86.0-pre.1", "latest", "v0.85.1", "0.85.1+build", "00.85.1", "0.85.1\n"]:
            with self.subTest(version=version), self.assertRaisesRegex(ValueError, "stable"):
                app_release.select_version(["0.85.1"], version)

    def test_no_stable_release_does_not_fall_back_to_a_prerelease(self):
        with self.assertRaisesRegex(ValueError, "No published stable"):
            app_release.select_version(["0.85.0-rc.1"])

    def test_resolver_follows_pagination_and_verifies_selected_manifest_digest(self):
        body = json.dumps({"schemaVersion": 2, "mediaType": app_release.MANIFEST_TYPES[0],
                           "manifests": []}).encode()
        digest = "sha256:" + hashlib.sha256(body).hexdigest()
        next_page = f"{app_release.REGISTRY_API}/tags/list?n=1000&last=0.85.0"
        responses = [self.tag_page(["0.85.0"], Link=f'<{next_page}>; rel="next"'),
                     self.tag_page(["0.85.1", "0.9.9", "0.86.0-rc.1"]),
                     (body, {"Docker-Content-Digest": digest})]
        with patch.object(app_release, "fetch", side_effect=responses) as fetch:
            self.assertEqual(app_release.resolve(), {
                "app_version": "0.85.1", "app_manifest_digest": digest,
            })
        self.assertEqual([call.args[0] for call in fetch.call_args_list], [
            f"{app_release.REGISTRY_API}/tags/list?n=1000", next_page,
            f"{app_release.REGISTRY_API}/manifests/0.85.1",
        ])

    def test_missing_latest_manifest_fails_without_selecting_an_older_release(self):
        responses = [self.tag_page(["0.85.1", "0.85.0"]), RuntimeError("manifest unavailable")]
        with patch.object(app_release, "fetch", side_effect=responses) as fetch:
            with self.assertRaisesRegex(RuntimeError, "manifest unavailable"):
                app_release.resolve()
            self.assertEqual(fetch.call_count, 2)

    def test_invalid_manifest_and_missing_or_mismatched_digest_are_rejected(self):
        valid = {"schemaVersion": 2, "mediaType": app_release.MANIFEST_TYPES[0], "manifests": []}
        for manifest, digest in [({}, ""), (valid, ""), (valid, "sha256:" + "b" * 64)]:
            responses = [self.tag_page(["0.85.1"]),
                         (json.dumps(manifest).encode(), {"Docker-Content-Digest": digest})]
            with self.subTest(manifest=manifest, digest=digest):
                with patch.object(app_release, "fetch", side_effect=responses):
                    with self.assertRaises(ValueError):
                        app_release.resolve()

    def test_invalid_listing_and_pagination_fail_instead_of_selecting_from_partial_results(self):
        for page in [self.tag_page(None), self.tag_page([None]),
                     self.tag_page(["0.85.1"], Link="malformed"),
                     self.tag_page(["0.85.1"], Link='<https://example.com/tags>; rel="next"'),
                     self.tag_page(["0.85.1"], Link='<?n=1000>; rel="next"')]:
            with self.subTest(page=page), patch.object(app_release, "fetch", return_value=page) as fetch:
                with self.assertRaisesRegex(ValueError, "Invalid server manifest tag"):
                    app_release.resolve()
                self.assertEqual(fetch.call_count, 1)

    def test_fetch_sends_no_authorization(self):
        response = io.BytesIO(b'{"tags": []}')
        response.headers = {}
        with patch.dict(os.environ, {"GH_TOKEN": "must-not-send", "GITHUB_TOKEN": "must-not-send"}):
            with patch.object(app_release, "urlopen", return_value=response) as request:
                self.assertEqual(app_release.fetch(f"{app_release.REGISTRY_API}/tags/list"),
                                 (b'{"tags": []}', {}))
        self.assertIsNone(request.call_args.args[0].get_header("Authorization"))

    def test_registry_request_failure_is_reported(self):
        with patch.object(app_release, "urlopen", side_effect=app_release.URLError("unavailable")):
            with self.assertRaisesRegex(RuntimeError, "Could not read public server manifest metadata"):
                app_release.fetch(f"{app_release.REGISTRY_API}/tags/list")


class MatrixTests(unittest.TestCase):
    local_scenarios = [
        "local-kind-ingress",
        "local-kind-gateway",
        "local-kind-oidc-pkce",
        "local-kind-oidc-implicit",
        "local-kind-proxy",
        "local-kind-external",
        "local-kind-proxy-auth",
        "local-kind-proxy-tls",
        "local-kind-operator-upgrade",
    ]
    gcp_scenarios = [
        "gcp-gke-gcs-hmac",
        "gcp-gke-gcs-byob",
        "gcp-gke-provider-services",
        "gcp-gke-gcs-letsencrypt",
        "gcp-gke-ingress-managed-cert",
    ]

    def test_main_runs_nine_local_and_five_gcp_scenarios(self):
        result = matrix.select_scenarios("refs/heads/main")
        self.assertEqual(json.loads(result["local_matrix"]), {"scenario": self.local_scenarios})
        self.assertEqual(json.loads(result["cloud_matrix"])["include"], [
            {"scenario": scenario, "provider": "gcp", "environment": "westest-gcp"}
            for scenario in self.gcp_scenarios
        ])
        self.assertEqual(result["has_local"], "true")
        self.assertEqual(result["has_cloud"], "true")

    def test_other_refs_cannot_select_cloud_even_manually(self):
        for ref in ["refs/heads/feature", "refs/tags/main", "refs/pull/42/merge"]:
            with self.subTest(ref=ref):
                result = matrix.select_scenarios(ref)
                self.assertEqual(result["has_cloud"], "false")
                self.assertEqual(json.loads(result["local_matrix"]), {"scenario": self.local_scenarios})
                for scenario in self.gcp_scenarios:
                    with self.subTest(scenario=scenario), self.assertRaisesRegex(ValueError, "only run"):
                        matrix.select_scenarios(ref, scenario)

    def test_single_scenarios_skip_the_unused_job(self):
        for scenario in self.gcp_scenarios:
            with self.subTest(scenario=scenario):
                result = matrix.select_scenarios("refs/heads/main", scenario)
                self.assertEqual(result["has_local"], "false")
                self.assertEqual(result["has_cloud"], "true")
                self.assertEqual(json.loads(result["local_matrix"]), {"scenario": []})
                self.assertEqual(json.loads(result["cloud_matrix"]), {"include": [
                    {"scenario": scenario, "provider": "gcp", "environment": "westest-gcp"},
                ]})
        for ref in ["refs/heads/main", "refs/heads/feature"]:
            for scenario in self.local_scenarios:
                with self.subTest(ref=ref, scenario=scenario):
                    result = matrix.select_scenarios(ref, scenario)
                    self.assertEqual(result["has_local"], "true")
                    self.assertEqual(result["has_cloud"], "false")
                    self.assertEqual(json.loads(result["local_matrix"]), {"scenario": [scenario]})
                    self.assertEqual(json.loads(result["cloud_matrix"]), {"include": []})

    def test_disabled_scenarios_are_rejected(self):
        for scenario in ["local-kind-app-test-data", "aws-eks-s3-irsa", "gcp-gke-ingress-in-cluster-services",
                         "gcp-gke-gateway-in-cluster-services", "gcp-gke-gateway-managed-cert-regional",
                         "gcp-gke-gateway-managed-cert-global"]:
            with self.subTest(scenario=scenario), self.assertRaisesRegex(ValueError, "Unsupported"):
                matrix.select_scenarios("refs/heads/main", scenario)

    def test_unknown_or_injected_scenario_is_rejected(self):
        for scenario in ["local-kind-unknown", 'x"]}\nother=value', "../../etc/passwd"]:
            with self.assertRaisesRegex(ValueError, "Unsupported"):
                matrix.select_scenarios("refs/heads/main", scenario)


class RunnerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.env = {
            "RUNNER_TEMP": str(self.root), "GITHUB_OUTPUT": str(self.root / "output"),
            "GITHUB_REF": "refs/heads/main", "GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "1",
            "SCENARIO": "local-kind-ingress", "PROVIDER": "local",
            "OPERATOR_VERSION": "2.0.0-dev-test-20261016-120000", "WESTEST_IMAGE_TAG": "0.4.0",
            "APP_VERSION": "0.85.1",
            "APP_MANIFEST_DIGEST": "sha256:" + "b" * 64,
            "AWS_ACCESS_KEY_ID": "test-id", "AWS_SECRET_ACCESS_KEY": "test-secret",
            "AWS_SESSION_TOKEN": "test-session-token",
        }
        tools = patch.object(runner.shutil, "which", side_effect=lambda tool: f"/usr/bin/{tool}")
        self.which = tools.start()
        self.addCleanup(tools.stop)

    def test_version_tags_cannot_change_the_image_repository(self):
        repository = "us-docker.pkg.dev/wandb-production/public/westest/westest"
        # A legacy image variable must not override the repository either.
        self.env["WESTEST_IMAGE"] = "untrusted.example/westest:latest"
        for tag in ["0.4.0", "0.4.1-rc.1", "nightly_20261016"]:
            with self.subTest(tag=tag):
                self.env["WESTEST_IMAGE_TAG"] = tag
                with patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)) as command:
                    self.assertEqual(runner.run(self.env), 0)
                calls = [call.args[0] for call in command.call_args_list]
                expected = f"{repository}:{tag}"
                self.assertEqual(calls[0], ["docker", "pull", expected])
                for args in calls:
                    self.assertIn(expected, args)
                    self.assertNotIn(self.env["WESTEST_IMAGE"], args)
                summary = json.loads((self.root / "westest-local-kind-ingress/summary.json").read_text())
                self.assertEqual(summary["image"], expected)

    def test_invalid_tags_fail_before_credentials_or_docker(self):
        self.env.update(PROVIDER="aws", SCENARIO="aws-eks-s3-irsa")
        for tag in ["", "evil.example/westest:0.4.0", "../westest", "0.4.0@sha256:abc",
                    "sha256:abc", "--privileged", ".hidden", "0.4.0\n", "a" * 129, "тest"]:
            with self.subTest(tag=tag):
                self.env["WESTEST_IMAGE_TAG"] = tag
                with patch.object(runner.subprocess, "run") as command:
                    with self.assertRaisesRegex(ValueError, "Invalid WESTest image tag"):
                        runner.run(self.env)
                    command.assert_not_called()
                self.assertFalse((self.root / "westest-aws-eks-s3-irsa").exists())

    def test_missing_tools_fail_before_credentials_and_still_collect_summary(self):
        self.env.update(PROVIDER="aws", SCENARIO="aws-eks-s3-irsa")
        root = self.root / "westest-aws-eks-s3-irsa"
        for missing in ["docker", "timeout"]:
            with self.subTest(missing=missing):
                self.which.side_effect = lambda tool: None if tool == missing else f"/usr/bin/{tool}"
                with patch.object(runner.subprocess, "run") as command, patch("sys.stderr", new_callable=io.StringIO) as stderr:
                    self.assertEqual(runner.run(self.env), 1)
                    command.assert_not_called()
                    self.assertIn(f"Required runner tools not found on PATH: {missing}", stderr.getvalue())
                    self.assertFalse((root / "auth").exists())
                    with patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(self.root / "step-summary")}):
                        runner.collect(root)
                    if missing == "docker":
                        command.assert_not_called()
                summary = json.loads((root / "public/summary.json").read_text())
                self.assertEqual(summary["exit_code"], 1)

    def test_local_uses_host_network_and_docker_socket(self):
        args = runner.docker_args(self.root, "local", self.env)
        self.assertIn("host", args)
        self.assertIn("type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock", args)
        self.assertNotIn("WESTEST_CLOUD_OPT_IN=1", args)
        self.assertFalse((self.root / "auth").exists())

    def test_aws_uses_private_profile_with_session_token_and_boundary(self):
        args = runner.docker_args(self.root, "aws", self.env)
        credentials = self.root / "auth/aws/credentials"
        self.assertIn("[Sandbox]", credentials.read_text())
        self.assertIn("aws_session_token = test-session-token", credentials.read_text())
        self.assertEqual(credentials.stat().st_mode & 0o777, 0o600)
        self.assertIn("WESTEST_AWS_IAM_PATH=/westest/", args)
        self.assertTrue(any("WestestWorkloadBoundary" in arg for arg in args))
        for secret in ["test-id", "test-secret", "test-session-token"]:
            self.assertNotIn(secret, " ".join(args))
        self.assertNotIn("docker.sock", " ".join(args))
        self.assertNotIn("--network", args)

    def test_gcp_preserves_refreshable_adc_and_remaps_both_clients(self):
        source = self.root / "gha-creds-test.json"
        adc = {"type": "external_account", "credential_source": {
            "url": "https://example.invalid/oidc", "headers": {"Authorization": "Bearer test-token"}}}
        source.write_text(json.dumps(adc))
        self.env["GOOGLE_APPLICATION_CREDENTIALS"] = str(source)
        args = runner.docker_args(self.root, "gcp", self.env)
        target = self.root / "auth/google.json"
        self.assertEqual(json.loads(target.read_text()), adc)
        self.assertEqual(target.stat().st_mode & 0o777, 0o600)
        self.assertIn("GOOGLE_APPLICATION_CREDENTIALS=/credentials/google.json", args)
        self.assertIn("CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE=/credentials/google.json", args)
        self.assertIn("WESTEST_GCP_REGION=us-west1", args)
        self.assertNotIn("test-token", " ".join(args))
        self.assertNotIn("docker.sock", " ".join(args))

    def test_cloud_runner_rejects_non_main_before_credentials_or_docker(self):
        self.env.update(PROVIDER="aws", SCENARIO="aws-eks-s3-irsa", GITHUB_REF="refs/heads/feature")
        with patch.object(runner.subprocess, "run") as command:
            with self.assertRaisesRegex(ValueError, "only run"):
                runner.run(self.env)
            command.assert_not_called()

    def test_chart_override_reaches_config_and_run_and_failure_propagates(self):
        # The workflow supplies the upgrade baseline to all local jobs. Ordinary
        # scenarios must still receive only their existing operatorVersion param.
        self.env["OPERATOR_UPGRADE_FROM_VERSION"] = "2.0.0-beta.8"
        for provider, scenario, duration, grace in [
            ("local", "local-kind-ingress", "60m", "11m"),
            ("aws", "aws-eks-s3-irsa", "75m", "46m"),
        ]:
            with self.subTest(provider=provider):
                self.env.update(PROVIDER=provider, SCENARIO=scenario)
                with patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess([], 124)) as command:
                    self.assertEqual(runner.run(self.env), 124)
                calls = [call.args[0] for call in command.call_args_list]
                config = next(args for args in calls if "config" in args)
                run = next(args for args in calls if args[0] == "timeout")
                for args in [config, run]:
                    self.assertEqual(args[-4:], ["--param", f"operatorVersion={self.env['OPERATOR_VERSION']}",
                                                 "--param", f"appVersion={self.env['APP_VERSION']}"])
                self.assertEqual(run[:4], ["timeout", "--signal=INT", f"--kill-after={grace}", duration])
                summary = json.loads((self.root / f"westest-{scenario}/summary.json").read_text())
                self.assertEqual(summary["exit_code"], 124)
                self.assertEqual(summary["app_version"], self.env["APP_VERSION"])
                self.assertEqual(summary["app_manifest_digest"], self.env["APP_MANIFEST_DIGEST"])
                self.assertIn("result=failed", (self.root / "output").read_text())

    def test_operator_upgrade_passes_both_versions_and_reports_them(self):
        scenario = "local-kind-operator-upgrade"
        self.env.update(SCENARIO=scenario, GITHUB_REF="refs/heads/feature")
        for from_version, to_version, exit_code in [
            ("2.0.0-beta.8", self.env["OPERATOR_VERSION"], 0),
            ("2.0.0-beta.7", "2.0.0-beta.9", 124),
        ]:
            with self.subTest(from_version=from_version, to_version=to_version):
                self.env.update(OPERATOR_UPGRADE_FROM_VERSION=from_version, OPERATOR_VERSION=to_version)
                with patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess([], exit_code)) as command:
                    self.assertEqual(runner.run(self.env), exit_code)
                calls = [call.args[0] for call in command.call_args_list]
                config = next(args for args in calls if "config" in args)
                run = next(args for args in calls if args[0] == "timeout")
                for args in [config, run]:
                    self.assertEqual(args[-6:], ["--param", f"fromOperatorVersion={from_version}",
                                                 "--param", f"toOperatorVersion={to_version}",
                                                 "--param", f"appVersion={self.env['APP_VERSION']}"])
                    self.assertNotIn(f"operatorVersion={to_version}", args)
                self.assertEqual(run[:4], ["timeout", "--signal=INT", "--kill-after=11m", "90m"])
                root = self.root / f"westest-{scenario}"
                step_summary = self.root / "step-summary"
                with patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(step_summary)}):
                    with patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)):
                        runner.collect(root)
                summary = json.loads((root / "public/summary.json").read_text())
                self.assertEqual(summary["operator_from_version"], from_version)
                self.assertEqual(summary["operator_version"], to_version)
                self.assertEqual(summary["exit_code"], exit_code)
                self.assertIn(f"Operator upgrade: {from_version} -> {to_version}", step_summary.read_text())
                self.assertIn(f"Application: {self.env['APP_VERSION']}", step_summary.read_text())
                self.assertIn(self.env["APP_MANIFEST_DIGEST"], step_summary.read_text())

    def test_runner_rejects_prerelease_apps_and_missing_release_metadata_before_docker(self):
        for key, value in [("APP_VERSION", ""), ("APP_VERSION", "0.86.0-rc.1"),
                           ("APP_VERSION", "0.86.0-beta.1"), ("APP_VERSION", "0.86.0-daily.1"),
                           ("APP_MANIFEST_DIGEST", "")]:
            with self.subTest(key=key, value=value):
                env = dict(self.env, **{key: value})
                with patch.object(runner.subprocess, "run") as command:
                    with self.assertRaisesRegex(ValueError, "[Aa]pplication"):
                        runner.run(env)
                    command.assert_not_called()
                self.assertFalse((self.root / "westest-local-kind-ingress").exists())

    def test_operator_upgrade_rejects_missing_invalid_or_identical_baseline_before_docker(self):
        self.env["SCENARIO"] = "local-kind-operator-upgrade"
        for baseline in [None, "", "2.0.0-beta.8\n", "../chart", self.env["OPERATOR_VERSION"]]:
            with self.subTest(baseline=baseline):
                if baseline is None:
                    self.env.pop("OPERATOR_UPGRADE_FROM_VERSION", None)
                else:
                    self.env["OPERATOR_UPGRADE_FROM_VERSION"] = baseline
                with patch.object(runner.subprocess, "run") as command:
                    with self.assertRaisesRegex(ValueError, "[Oo]perator upgrade"):
                        runner.run(self.env)
                    command.assert_not_called()
                self.assertFalse((self.root / "westest-local-kind-operator-upgrade").exists())

    def test_preflight_failure_still_records_failed_summary(self):
        with patch.object(runner.subprocess, "run", side_effect=subprocess.CalledProcessError(1, ["docker"])):
            with self.assertRaises(subprocess.CalledProcessError):
                runner.run(self.env)
        summary = json.loads((self.root / "westest-local-kind-ingress/summary.json").read_text())
        self.assertEqual(summary["exit_code"], 1)
        self.assertIn("result=failed", (self.root / "output").read_text())

    def test_collect_exports_only_metadata_and_reports_failed_cleanup(self):
        with patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)):
            runner.run(self.env)
        root = self.root / "westest-local-kind-ingress"
        runner.write_private(root / "auth/license", "private-license")
        state = root / "work/artifacts/_state/local-kind-ingress/cluster.json"
        state.parent.mkdir(parents=True)
        state.write_text('{"password": "state-secret"}')
        (root / "work/run.env").write_text("TOKEN=env-secret")
        step_summary = self.root / "step-summary"
        with patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(step_summary)}):
            with patch.object(runner.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)):
                runner.collect(root)
        self.assertEqual([path.name for path in (root / "public").iterdir()], ["summary.json"])
        public = (root / "public/summary.json").read_text()
        self.assertEqual(json.loads(public)["cleanup_checkpoint_count"], 1)
        for secret in ["private-license", "state-secret", "env-secret"]:
            self.assertNotIn(secret, public)
        self.assertIn("Recovery bundles are a follow-up", step_summary.read_text())
        self.assertFalse((root / "auth").exists())
        self.assertTrue(state.exists())


if __name__ == "__main__":
    unittest.main()
