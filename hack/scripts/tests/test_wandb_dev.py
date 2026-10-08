import base64
import contextlib
import copy
import io
import json
import os
import sys
import unittest
import urllib.request
from pathlib import Path
from unittest.mock import Mock, patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from wandb_dev import (
    APIError,
    BootstrapError,
    Kubernetes,
    SameOriginRedirect,
    Server,
    bootstrap,
    load_admin_key,
    wait_for_cr,
    show_credentials,
)


class MemoryStore:
    def __init__(self):
        self.data = None
        self.saved = []

    def load(self):
        return copy.deepcopy(self.data)

    def save(self, data):
        self.data = copy.deepcopy(data)
        self.saved.append(copy.deepcopy(data))


class TestServer:
    base_url = "http://wandb.localhost:8080"
    disable_local_user = Server.disable_local_user

    def __init__(self, store):
        self.store = store
        self.user = None
        self.password = None
        self.keys = []
        self.login_count = 0
        self.create_count = 0
        self.entity_count = 0
        self.session = None
        self.fail_after_create = False
        self.admin_key = "a" * 40
        self.admin_checks = 0
        self.local_user = {"email": "local@wandb.com", "deletedAt": None}
        self.disable_count = 0
        self.disable_key = None
        self.user_admin = True
        self.fail_before_disable = False
        self.fail_after_disable = False

    def login(self, email, password, username):
        self.login_count += 1
        if not self.user or self.user["email"] != email or self.password != password:
            raise APIError(401, "login")
        self.session = "user"

    def require_empty_installation(self, admin_key):
        self.admin_checks += 1
        if admin_key != self.admin_key:
            raise APIError(401, "admin")
        if self.user:
            raise BootstrapError("Supply the existing user's credentials")

    def request(self, path, payload, method, api_key):
        assert path == "/oidc/users" and method == "PUT"
        if payload == [{"email": "local@wandb.com", "delete": True}]:
            assert api_key == self.admin_key or (api_key in self.keys and self.user_admin)
            # Never remove the bootstrap login before the real credentials are saved.
            assert self.user["entity"] and self.store.data["apiKey"] in self.keys
            if self.fail_before_disable:
                self.fail_before_disable = False
                raise APIError(503, "disable local user")
            self.local_user["deletedAt"] = "2026-10-02T00:00:00Z"
            self.disable_count += 1
            self.disable_key = api_key
            if self.fail_after_disable:
                self.fail_after_disable = False
                raise APIError(0, "lost response after disabling local user")
            return {}
        assert api_key == self.admin_key and self.user is None
        # The password must already be durably saved before account creation.
        assert self.store.data["loginPassword"] == payload[0]["password"]
        self.password = payload[0]["password"]
        self.user = {
            "id": "user-1",
            "email": payload[0]["email"],
            "username": None,
            "entity": None,
        }
        self.create_count += 1
        if self.fail_after_create:
            self.fail_after_create = False
            raise APIError(0, "lost response after creating user")

    def viewer(self, api_key=None):
        if api_key and api_key not in self.keys:
            raise APIError(401, "viewer")
        assert api_key or self.session == "user"
        return copy.deepcopy(self.user)

    def graphql(self, query, variables=None, api_key=None):
        if "BootstrapLocalUser" in query:
            if api_key != self.admin_key and api_key not in self.keys:
                raise APIError(401, "local user lookup")
            return {
                "viewer": {"admin": api_key == self.admin_key or self.user_admin},
                "user": copy.deepcopy(self.local_user),
            }
        assert self.session == "user"
        if "BootstrapEntity" in query:
            self.user.update(username=variables["name"], entity=variables["name"])
            self.entity_count += 1
            return {}
        assert "BootstrapKey" in query
        key = f"test-api-key-{len(self.keys)}"
        self.keys.append(key)
        return {
            "generateApiKey": {
                "secretApiKey": key,
                "apiKey": {"id": f"key-{len(self.keys)}"},
            }
        }


class BootstrapTests(unittest.TestCase):
    def setUp(self):
        self.store = MemoryStore()
        self.server = TestServer(self.store)

    def run_bootstrap(self, **kwargs):
        kwargs.setdefault("admin_key", self.server.admin_key)
        return bootstrap(
            self.server,
            self.store,
            email="tester@example.test",
            username="tester",
            ca_bundle="test-ca",
            **kwargs,
        )

    def test_fresh_install_and_rerun_use_one_user_and_one_key(self):
        first = self.run_bootstrap()
        login_count = self.server.login_count
        second = self.run_bootstrap()
        self.assertEqual(first, second)
        self.assertEqual(self.server.create_count, 1)
        self.assertEqual(len(self.server.keys), 1)
        self.assertEqual(self.server.login_count, login_count)
        self.assertEqual(self.server.admin_checks, 1)
        self.assertNotIn(self.server.admin_key, str(first))
        self.assertIsNotNone(self.server.local_user["deletedAt"])
        self.assertEqual(self.server.disable_count, 1)
        self.assertEqual(self.server.disable_key, self.server.admin_key)

    def test_saved_credentials_repair_previous_bootstrap_without_changing_user_or_key(self):
        first = self.run_bootstrap()
        self.server.local_user["deletedAt"] = None
        self.server.session = None
        result = self.run_bootstrap()
        self.assertEqual(result, first)
        self.assertIsNotNone(self.server.local_user["deletedAt"])
        self.assertEqual(self.server.disable_count, 2)
        self.assertEqual(self.server.create_count, 1)
        self.assertEqual(len(self.server.keys), 1)

    def test_existing_user_key_disables_local_without_global_admin_key(self):
        first = self.run_bootstrap()
        self.server.local_user["deletedAt"] = None
        self.server.session = None
        self.run_bootstrap(admin_key=None)
        self.assertEqual(self.server.disable_key, first["apiKey"])
        self.assertEqual(self.server.disable_count, 2)

    def test_existing_password_disables_local_without_global_admin_key(self):
        first = self.run_bootstrap()
        self.server.local_user["deletedAt"] = None
        self.store.data = None
        result = self.run_bootstrap(password=first["loginPassword"], admin_key=None)
        self.assertEqual(self.server.disable_key, result["apiKey"])
        self.assertEqual(self.server.create_count, 1)

    def test_missing_local_user_needs_no_deletion(self):
        self.server.local_user = None
        self.run_bootstrap()
        self.assertEqual(self.server.disable_count, 0)

    def test_cleanup_failure_and_lost_response_resume_with_saved_key(self):
        for failure, deleted in [("fail_before_disable", False), ("fail_after_disable", True)]:
            with self.subTest(failure=failure):
                self.setUp()
                setattr(self.server, failure, True)
                with self.assertRaises(APIError):
                    self.run_bootstrap()
                saved = self.store.load()
                self.assertTrue(saved["apiKey"])
                self.assertEqual(bool(self.server.local_user["deletedAt"]), deleted)
                self.assertEqual(self.run_bootstrap(), saved)
                self.assertEqual(len(self.server.keys), 1)
                self.assertEqual(self.server.create_count, 1)
                self.assertEqual(self.server.disable_count, 1)

    def test_failed_onboarding_does_not_disable_local_user(self):
        with patch.object(self.server, "graphql", side_effect=APIError(503, "onboarding")):
            with self.assertRaises(APIError):
                self.run_bootstrap()
        self.assertIsNone(self.server.local_user["deletedAt"])
        self.assertEqual(self.server.disable_count, 0)

    def test_non_admin_cannot_disable_active_local_user(self):
        self.run_bootstrap()
        self.server.local_user["deletedAt"] = None
        self.server.user_admin = False
        with self.assertRaisesRegex(BootstrapError, "server administrator access"):
            self.run_bootstrap(admin_key=None)
        self.assertIsNone(self.server.local_user["deletedAt"])
        self.assertEqual(self.server.disable_count, 1)

    def test_different_local_identity_is_not_disabled(self):
        self.server.local_user["email"] = "someone@example.test"
        with self.assertRaisesRegex(BootstrapError, "built-in local user"):
            self.run_bootstrap()
        self.assertEqual(self.server.disable_count, 0)

    def test_builtin_identity_cannot_be_used_as_the_human_user(self):
        with self.assertRaisesRegex(BootstrapError, "real user email"):
            bootstrap(
                self.server,
                self.store,
                email="service@wandb.com",
                username="service",
                admin_key=self.server.admin_key,
            )
        self.assertIsNone(self.store.data)
        self.assertEqual(self.server.create_count, 0)

    def test_missing_admin_key_does_not_create_user(self):
        with self.assertRaisesRegex(BootstrapError, "GLOBAL_ADMIN_API_KEY"):
            self.run_bootstrap(admin_key=None)
        self.assertEqual(self.server.create_count, 0)

    def test_invalid_admin_key_does_not_create_user(self):
        with self.assertRaises(APIError):
            self.run_bootstrap(admin_key="invalid-key")
        self.assertEqual(self.server.create_count, 0)

    def test_wrong_password_is_not_reset_by_admin_key(self):
        password = self.run_bootstrap()["loginPassword"]
        self.store.data["apiKey"] = ""
        with self.assertRaisesRegex(BootstrapError, "existing user's"):
            self.run_bootstrap(password="incorrect")
        self.assertEqual(self.server.password, password)
        self.assertEqual(self.server.create_count, 1)

    def test_existing_user_credentials_do_not_require_admin_key(self):
        password = self.run_bootstrap()["loginPassword"]
        self.store.data = None
        self.run_bootstrap(password=password, admin_key=None)
        self.assertEqual(self.server.create_count, 1)
        self.assertEqual(self.server.admin_checks, 1)

    def test_lost_create_response_resumes_with_persisted_password(self):
        self.server.fail_after_create = True
        with self.assertRaises(APIError):
            self.run_bootstrap()
        password = self.store.data["loginPassword"]
        result = self.run_bootstrap()
        self.assertEqual(result["loginPassword"], password)
        self.assertEqual(self.server.create_count, 1)
        self.assertEqual(self.server.entity_count, 1)

    def test_resume_after_onboarding_before_key_generation(self):
        original = self.server.graphql

        def interrupt(query, variables):
            if "BootstrapKey" in query:
                raise APIError(503, "generate key")
            return original(query, variables)

        with patch.object(self.server, "graphql", side_effect=interrupt):
            with self.assertRaises(APIError):
                self.run_bootstrap()
        self.run_bootstrap()
        self.assertEqual(self.server.entity_count, 1)
        self.assertEqual(self.server.create_count, 1)

    def test_revoked_key_is_replaced_using_same_user(self):
        self.run_bootstrap()
        self.server.keys.clear()
        self.run_bootstrap()
        self.assertEqual(self.server.create_count, 1)
        self.assertEqual(self.server.entity_count, 1)
        self.assertEqual(len(self.server.keys), 1)

    def test_missing_secret_recovers_with_existing_password(self):
        password = self.run_bootstrap()["loginPassword"]
        self.store.data = None
        self.run_bootstrap(password=password)
        self.assertEqual(self.server.create_count, 1)

    def test_existing_user_without_credentials_does_not_create_another(self):
        self.run_bootstrap()
        self.store.data = None
        with self.assertRaisesRegex(BootstrapError, "existing user's"):
            self.run_bootstrap()
        self.assertEqual(self.server.create_count, 1)

    def test_existing_api_key_does_not_invent_login_password(self):
        key = self.run_bootstrap()["apiKey"]
        self.store.data = None
        result = self.run_bootstrap(api_key=key)
        self.assertEqual(result["loginPassword"], "")
        self.assertEqual(len(self.server.keys), 1)

    def test_mismatched_identity_fails_without_mutation(self):
        self.run_bootstrap()
        self.server.local_user["deletedAt"] = None
        self.server.user["id"] = "another-user"
        with self.assertRaisesRegex(BootstrapError, "different user"):
            self.run_bootstrap()
        self.assertEqual(len(self.server.keys), 1)
        self.assertIsNone(self.server.local_user["deletedAt"])

    def test_network_failure_does_not_trigger_signup_or_new_key(self):
        self.run_bootstrap()
        with patch.object(self.server, "viewer", side_effect=APIError(503, "viewer")):
            with self.assertRaises(APIError):
                self.run_bootstrap()
        self.assertEqual(self.server.create_count, 1)
        self.assertEqual(len(self.server.keys), 1)

    def test_different_server_is_rejected(self):
        self.run_bootstrap()
        self.server.base_url = "http://different.test"
        with self.assertRaisesRegex(BootstrapError, "differs"):
            self.run_bootstrap()

    def test_credentials_are_shown_locally_and_suppressed_in_ci(self):
        data = self.run_bootstrap()
        for ci, should_show in [
            ("", True),
            ("true", False),
            ("1", False),
            ("false", True),
        ]:
            with (
                self.subTest(ci=ci),
                patch.dict(os.environ, {"CI": ci}),
                contextlib.redirect_stdout(io.StringIO()) as output,
            ):
                show_credentials(data)
            self.assertEqual(data["apiKey"] in output.getvalue(), should_show)
            self.assertEqual(data["loginPassword"] in output.getvalue(), should_show)


class TransportTests(unittest.TestCase):
    def test_failed_local_user_lookup_does_not_delete(self):
        server = Server("https://wandb.example")
        with (
            patch.object(server, "graphql", return_value={"viewer": {"admin": True}}),
            patch.object(server, "request") as request,
        ):
            with self.assertRaisesRegex(BootstrapError, "Unable to check"):
                server.disable_local_user("admin-key")
            request.assert_not_called()

    def test_admin_user_check_excludes_only_builtin_identities(self):
        server = Server("https://wandb.example")
        builtin = [
            {"node": {"email": email}}
            for email in ("local@wandb.com", "restore@wandb.com", "service@wandb.com")
        ]
        for extra, more, admin, succeeds in [
            ([], False, True, True),
            ([{"node": {"email": "existing@example.test"}}], False, True, False),
            ([{"node": {"email": None}}], False, True, False),
            ([], True, True, False),
            ([], False, False, False),
        ]:
            result = {
                "viewer": {"admin": admin},
                "users": {"edges": builtin + extra, "pageInfo": {"hasNextPage": more}},
            }
            with (
                self.subTest(extra=extra, more=more, admin=admin),
                patch.object(server, "graphql", return_value=result) as query,
            ):
                if succeeds:
                    server.require_empty_installation("admin-key")
                else:
                    with self.assertRaises(BootstrapError):
                        server.require_empty_installation("admin-key")
                self.assertEqual(query.call_args.kwargs["api_key"], "admin-key")

    def test_failed_user_listing_prevents_creation(self):
        server = Server("https://wandb.example")
        with patch.object(server, "graphql", return_value={"viewer": {"admin": True}}):
            with self.assertRaisesRegex(BootstrapError, "Unable to verify"):
                server.require_empty_installation("admin-key")

    def test_only_exact_queue_not_found_response_allows_queue_creation(self):
        server = Server("https://wandb.example")
        for message, path, missing_queue in [
            ("queue not found", ["project", "runQueue"], True),
            ("project not found", ["project"], False),
            ("queue not found", ["anotherOperation"], False),
        ]:
            body = json.dumps({"errors": [{"message": message, "path": path}]}).encode()
            error = urllib.error.HTTPError(
                "https://wandb.example/graphql", 404, "not found", {}, io.BytesIO(body)
            )
            with (
                self.subTest(message=message, path=path),
                patch.object(server.opener, "open", side_effect=error),
            ):
                with self.assertRaises(APIError) as raised:
                    server.viewer()
                self.assertEqual(raised.exception.missing_queue, missing_queue)

    def test_cross_origin_redirect_is_blocked(self):
        request = urllib.request.Request("https://wandb.example/graphql")
        with self.assertRaisesRegex(BootstrapError, "another origin"):
            SameOriginRedirect().redirect_request(
                request, None, 302, "", {}, "https://other.example/graphql"
            )

    def test_same_origin_headers_and_api_key_auth(self):
        server = Server("https://wandb.example")
        with patch.object(server.opener, "open") as opened:
            opened.return_value.__enter__.return_value.read.return_value = (
                b'{"data":{"viewer":null}}'
            )
            server.viewer("test-key")
            request = opened.call_args.args[0]
            self.assertEqual(request.headers["Origin"], "https://wandb.example")
            self.assertEqual(request.headers["Authorization"], "Basic YXBpOnRlc3Qta2V5")

    def test_graphql_errors_do_not_leak_response(self):
        server = Server("https://wandb.example")
        with patch.object(
            server, "request", return_value={"errors": [{"message": "secret-value"}]}
        ):
            with self.assertRaises(BootstrapError) as err:
                server.viewer()
        self.assertNotIn("secret-value", str(err.exception))

    @patch("wandb_dev.shutil.which", return_value="kubectl")
    def test_kubectl_failure_does_not_echo_secret(self, _):
        kube = Kubernetes("wandb", "kind-operator")
        with patch("wandb_dev.subprocess.run") as run:
            run.return_value.returncode = 1
            run.return_value.stderr = "invalid Secret: secret-value"
            with self.assertRaises(BootstrapError) as err:
                kube.run(["create", "-f", "-"], {"secret": "secret-value"})
            self.assertIn("kind-operator", run.call_args.args[0])
            self.assertNotIn("secret-value", str(err.exception))


class AdminKeyTests(unittest.TestCase):
    def setUp(self):
        self.cr = {
            "metadata": {"name": "wandb", "uid": "instance", "generation": 2},
            "spec": {"wandb": {"enableGlobalAdminAPIKey": True}},
            "status": {
                "conditions": [{"type": "Ready", "status": "True", "observedGeneration": 2}],
                "generatedSecrets": {"global-admin-api-key": {"name": "generated-admin", "key": "key"}},
            },
        }
        self.secret = {
            "metadata": {
                "ownerReferences": [{"uid": "instance"}],
                "labels": {"app.kubernetes.io/managed-by": "wandb-operator"},
            },
            "data": {"key": base64.b64encode(b"a" * 40).decode()},
        }
        self.kube = Mock()
        self.kube.get.return_value = self.secret

    def test_reads_operator_selector_without_mutating_secret(self):
        self.assertEqual(load_admin_key(self.kube, self.cr), "a" * 40)
        self.kube.get.assert_called_once_with("secret", "generated-admin")
        self.kube.put.assert_not_called()

    def test_disabled_does_not_read_admin_secret(self):
        self.cr["spec"]["wandb"]["enableGlobalAdminAPIKey"] = False
        self.assertIsNone(load_admin_key(self.kube, self.cr))
        self.kube.get.assert_not_called()

    def test_rejects_missing_selector_foreign_owner_and_invalid_key(self):
        for change in ["selector", "owner", "key"]:
            with self.subTest(change=change):
                self.setUp()
                if change == "selector":
                    self.cr["status"]["generatedSecrets"] = {}
                elif change == "owner":
                    self.secret["metadata"]["ownerReferences"] = [{"uid": "another-instance"}]
                else:
                    self.secret["data"]["key"] = base64.b64encode(b"invalid").decode()
                with self.assertRaises(BootstrapError):
                    load_admin_key(self.kube, self.cr)
                self.kube.put.assert_not_called()

    @patch("wandb_dev.time.sleep")
    def test_waits_for_ready_current_generation(self, sleep):
        old = copy.deepcopy(self.cr)
        old["status"]["conditions"][0]["observedGeneration"] = 1
        self.kube.get.side_effect = [old, self.cr]
        self.assertEqual(wait_for_cr(self.kube, "wandb", 10), self.cr)
        sleep.assert_called_once()

    @patch("wandb_dev.time.monotonic", side_effect=[0, 0, 11])
    @patch("wandb_dev.time.sleep")
    def test_readiness_timeout(self, sleep, monotonic):
        self.kube.get.return_value = None
        with self.assertRaisesRegex(BootstrapError, "current generation"):
            wait_for_cr(self.kube, "wandb", 10)


if __name__ == "__main__":
    unittest.main()
