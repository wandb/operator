#!/usr/bin/env python3
"""Bootstrap the single W&B user used by local tests and Launch (stdlib only)."""

import argparse
import base64
import copy
import http.cookiejar
import json
import os
import secrets
import shutil
import ssl
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

MANAGED_BY = "wandb-dev-bootstrap"
VIEWER = "query BootstrapViewer { viewer { id username entity email } }"
LOCAL_USER_EMAIL = "local@wandb.com"
# Built-in identities are excluded from the server's licensed user count.
HIDDEN_USER_EMAILS = {LOCAL_USER_EMAIL, "restore@wandb.com", "service@wandb.com"}


class BootstrapError(Exception):
    pass


class APIError(BootstrapError):
    def __init__(self, status, operation, missing_queue=False):
        self.status = status
        self.missing_queue = missing_queue
        super().__init__(
            f"{operation} failed (HTTP {status}); response omitted to protect credentials"
        )


class SameOriginRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        old = urllib.parse.urlsplit(request.full_url)
        new = urllib.parse.urlsplit(newurl)
        if (old.scheme, old.netloc) != (new.scheme, new.netloc):
            raise BootstrapError(
                "Authentication redirected to another origin; use existing credentials for this installation"
            )
        return super().redirect_request(request, fp, code, msg, headers, newurl)


class Server:
    def __init__(self, base_url, context=None):
        parsed = urllib.parse.urlsplit(base_url)
        if (
            parsed.scheme not in ("http", "https")
            or not parsed.hostname
            or parsed.username
            or parsed.query
            or parsed.fragment
        ):
            raise BootstrapError(
                "base URL must be an http(s) URL without credentials, query, or fragment"
            )
        self.base_url = base_url.rstrip("/")
        self.origin = f"{parsed.scheme}://{parsed.netloc}"
        self.opener = urllib.request.build_opener(
            SameOriginRedirect(),
            urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()),
            urllib.request.HTTPSHandler(
                context=context or ssl.create_default_context()
            ),
        )

    def request(self, path, payload, *, method="POST", api_key=None):
        headers = {
            "Content-Type": "application/json",
            "Origin": self.origin,
            "X-Origin": self.origin,
            "Referer": self.origin + "/",
        }
        if api_key:
            headers["Authorization"] = (
                "Basic " + base64.b64encode(f"api:{api_key}".encode()).decode()
            )
        request = urllib.request.Request(
            self.base_url + path,
            data=json.dumps(payload).encode(),
            headers=headers,
            method=method,
        )
        try:
            with self.opener.open(request, timeout=15) as response:
                body = response.read()
        except urllib.error.HTTPError as exc:
            missing_queue = False
            if exc.code == 404:
                try:
                    errors = json.loads(exc.read()).get("errors", [])
                    missing_queue = bool(errors) and all(
                        error.get("message") == "queue not found"
                        and error.get("path") == ["project", "runQueue"]
                        for error in errors
                    )
                except ValueError:
                    pass
            raise APIError(exc.code, f"{method} {path}", missing_queue) from None
        except (urllib.error.URLError, TimeoutError, OSError):
            raise APIError(0, f"{method} {path}") from None
        try:
            return json.loads(body) if body else {}
        except ValueError:
            raise BootstrapError(
                f"{path} did not return JSON; check the server URL and routing"
            ) from None

    def graphql(self, query, variables=None, api_key=None):
        result = self.request(
            "/graphql", {"query": query, "variables": variables or {}}, api_key=api_key
        )
        if result.get("errors"):
            raise BootstrapError(
                "GraphQL operation failed; check server version and permissions (response omitted)"
            )
        return result.get("data") or {}

    def viewer(self, api_key=None):
        return self.graphql(VIEWER, api_key=api_key).get("viewer")

    def login(self, email, password, username):
        self.request(
            "/oidc/login", {"email": email, "password": password, "username": username}
        )

    def require_empty_installation(self, admin_key):
        result = self.graphql(
            "query BootstrapUsers { viewer { admin } users(first: 100) { edges { node { email } } pageInfo { hasNextPage } } }",
            api_key=admin_key,
        )
        if (result.get("viewer") or {}).get("admin") is not True:
            raise BootstrapError(
                "Bootstrap key does not have server administrator access"
            )
        users = result.get("users") or {}
        if "edges" not in users or "hasNextPage" not in users.get("pageInfo", {}):
            raise BootstrapError("Unable to verify that the installation has no users")
        emails = [(edge.get("node") or {}).get("email") for edge in users["edges"]]
        if users["pageInfo"]["hasNextPage"] or any(
            email not in HIDDEN_USER_EMAILS for email in emails
        ):
            raise BootstrapError(
                "This installation already has users. Supply the existing user's email/username and WANDB_DEV_PASSWORD or WANDB_DEV_API_KEY; no user or password was changed"
            )

    def disable_local_user(self, api_key):
        result = self.graphql(
            'query BootstrapLocalUser { viewer { admin } user(userName: "local") { email deletedAt } }',
            api_key=api_key,
        )
        if "user" not in result:
            raise BootstrapError("Unable to check the built-in local user")
        local_user = result["user"]
        if local_user is None or local_user.get("deletedAt"):
            return
        if (result.get("viewer") or {}).get("admin") is not True:
            raise BootstrapError(
                "Disabling the built-in local user requires server administrator access; enable spec.wandb.enableGlobalAdminAPIKey or use an administrator's credentials"
            )
        if local_user.get("email") != LOCAL_USER_EMAIL:
            raise BootstrapError("The local username does not identify the built-in local user")
        # Database-backed initialization leaves this account active even with a
        # global admin key, so complete the cleanup normally done by browser signup.
        self.request(
            "/oidc/users",
            [{"email": LOCAL_USER_EMAIL, "delete": True}],
            method="PUT",
            api_key=api_key,
        )

    def wait_ready(self, timeout):
        deadline = time.monotonic() + timeout
        while True:
            try:
                self.request(
                    "/graphql", {"query": "query BootstrapReady { viewer { id } }"}
                )
                return
            except APIError as exc:
                if (
                    exc.status not in (0, 502, 503, 504, 500)
                    or time.monotonic() >= deadline
                ):
                    raise
                time.sleep(min(2, max(0, deadline - time.monotonic())))


class Kubernetes:
    def __init__(self, namespace, context=None):
        if not shutil.which("kubectl"):
            raise BootstrapError("kubectl is required")
        self.namespace = namespace
        self.command = ["kubectl"] + (["--context", context] if context else [])

    def run(self, args, obj=None):
        result = subprocess.run(
            self.command + ["--request-timeout=30s"] + args,
            input=json.dumps(obj) if obj is not None else None,
            text=True,
            capture_output=True,
            timeout=45,
        )
        if result.returncode:
            # kubectl validation errors can include the complete Secret manifest.
            raise BootstrapError(
                "kubectl operation failed; check context, namespace, permissions, or concurrent bootstrap (output omitted)"
            )
        return result.stdout

    def get(self, kind, name, namespace=None):
        raw = self.run(
            [
                "-n",
                namespace or self.namespace,
                "get",
                kind,
                name,
                "--ignore-not-found",
                "-o",
                "json",
            ]
        )
        return json.loads(raw) if raw.strip() else None

    def put(self, obj):
        obj = copy.deepcopy(obj)
        name = obj["metadata"]["name"]
        namespace = obj["metadata"].get("namespace", self.namespace)
        old = self.get(obj["kind"], name, namespace)
        if old:
            expected = obj["metadata"].get("labels", {})
            actual = old["metadata"].get("labels", {})
            if any(actual.get(k) != v for k, v in expected.items()):
                raise BootstrapError(
                    f"Refusing to replace unmanaged {obj['kind']} {namespace}/{name}"
                )
            obj["metadata"]["resourceVersion"] = old["metadata"]["resourceVersion"]
        self.run(["replace" if old else "create", "-f", "-"], obj)


def labels(args):
    return {
        "app.kubernetes.io/managed-by": MANAGED_BY,
        "wandb.ai/test-instance": args.name,
        "wandb.ai/test-namespace": args.namespace,
    }


def decode_secret(obj):
    return {k: base64.b64decode(v).decode() for k, v in obj.get("data", {}).items()}


class CredentialStore:
    def __init__(self, kube, args):
        self.kube, self.args = kube, args
        self.name = args.secret or args.name + "-dev-credentials"

    def load(self):
        obj = self.kube.get("secret", self.name)
        if obj is None:
            return None
        if any(
            obj["metadata"].get("labels", {}).get(k) != v
            for k, v in labels(self.args).items()
        ):
            raise BootstrapError(
                "Credential Secret belongs to a different owner; choose another Secret name"
            )
        return decode_secret(obj)

    def save(self, data):
        self.kube.put(
            {
                "apiVersion": "v1",
                "kind": "Secret",
                "type": "Opaque",
                "metadata": {
                    "name": self.name,
                    "namespace": self.args.namespace,
                    "labels": labels(self.args),
                },
                "data": {
                    k: base64.b64encode(v.encode()).decode() for k, v in data.items()
                },
            }
        )


def wait_for_cr(kube, name, timeout):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        cr = kube.get("weightsandbiases.v2.apps.wandb.com", name)
        if cr:
            generation = cr.get("metadata", {}).get("generation")
            for condition in cr.get("status", {}).get("conditions", []):
                if (
                    condition.get("type") == "Ready"
                    and condition.get("status") == "True"
                    and generation is not None
                    and condition.get("observedGeneration") == generation
                ):
                    return cr
        time.sleep(2)
    raise BootstrapError("Timed out waiting for the W&B CR to report Ready for its current generation")


def load_admin_key(kube, cr):
    if not cr.get("spec", {}).get("wandb", {}).get("enableGlobalAdminAPIKey", False):
        return None
    selector = cr.get("status", {}).get("generatedSecrets", {}).get("global-admin-api-key", {})
    if not selector.get("name") or not selector.get("key"):
        raise BootstrapError("The ready W&B CR does not publish its generated global-admin-api-key Secret")
    obj = kube.get("secret", selector["name"])
    if not obj:
        raise BootstrapError("The operator-managed global admin Secret is missing")
    metadata = obj.get("metadata", {})
    uid = cr.get("metadata", {}).get("uid")
    if (
        not uid
        or not any(owner.get("uid") == uid for owner in metadata.get("ownerReferences", []))
        or metadata.get("labels", {}).get("app.kubernetes.io/managed-by") != "wandb-operator"
    ):
        raise BootstrapError("The global admin Secret is not owned by this W&B deployment")
    key = decode_secret(obj).get(selector["key"], "")
    if len(key) != 40 or any(c not in "0123456789abcdefABCDEF" for c in key):
        raise BootstrapError("The operator-managed global admin Secret must contain a 40-character hex key")
    return key


def tls_context(args, kube):
    context = ssl.create_default_context()
    pem = ""
    if args.ca_file:
        pem = Path(args.ca_file).read_text()
    elif args.ca_secret:
        obj = kube.get("secret", args.ca_secret)
        if not obj:
            raise BootstrapError(f"CA Secret {args.ca_secret} is not available yet")
        pem = decode_secret(obj).get("ca.crt", "")
        if not pem:
            raise BootstrapError("CA Secret must contain ca.crt")
    if pem:
        context.load_verify_locations(cadata=pem)
    bundle = "".join(
        ssl.DER_cert_to_PEM_cert(cert)
        for cert in context.get_ca_certs(binary_form=True)
    )
    return context, bundle


def matches_user(viewer, data):
    if not viewer or (viewer.get("email") or "").lower() != data["email"].lower():
        return False
    if data.get("userId") and data["userId"] != viewer["id"]:
        return False
    return not viewer.get("username") or viewer["username"] == data["username"]


def bootstrap(
    server,
    store,
    *,
    email,
    username,
    password=None,
    api_key=None,
    admin_key=None,
    ca_bundle="",
):
    if email.lower() in HIDDEN_USER_EMAILS:
        raise BootstrapError(
            "Configure a real user email, not a built-in server identity"
        )
    data = store.load()
    if data:
        if (
            data["email"] != email
            or data["username"] != username
            or data["baseUrl"] != server.base_url
        ):
            raise BootstrapError(
                "Saved identity/server differs from configuration; reuse its settings or choose a different Secret"
            )
    else:
        data = {
            "baseUrl": server.base_url,
            "email": email,
            "username": username,
            "loginPassword": password or ("" if api_key else secrets.token_urlsafe(24)),
            "apiKey": "",
        }
        # Save before the first server mutation so an interrupted signup can resume.
        store.save(data)

    key = api_key or data.get("apiKey")
    if key:
        try:
            viewer = server.viewer(key)
        except APIError as exc:
            if exc.status not in (401, 403):
                raise
            viewer = None
        if viewer:
            if not matches_user(viewer, data):
                raise BootstrapError(
                    "API key belongs to a different user; refusing to change the configured identity"
                )
            if viewer.get("username") != username or viewer.get("entity") != username:
                raise BootstrapError(
                    "The supplied user's personal entity must be onboarded and match the configured username"
                )
            data.update(
                apiKey=key,
                userId=viewer["id"],
                entity=viewer["entity"],
                caBundle=ca_bundle,
            )
            store.save(data)
            server.disable_local_user(admin_key or key)
            return data
        if api_key:
            raise BootstrapError("The supplied W&B API key is not valid on this server")

    try:
        server.login(email, password or data["loginPassword"], username)
    except APIError as exc:
        if exc.status not in (401, 403):
            raise
        if not admin_key:
            raise BootstrapError(
                "Initial-user creation requires the deployment's GLOBAL_ADMIN_API_KEY Secret. Enable spec.wandb.enableGlobalAdminAPIKey with a supporting manifest, or supply the existing user's WANDB_DEV_PASSWORD or WANDB_DEV_API_KEY"
            ) from None
        # The admin endpoint also resets passwords, so never call it on an
        # existing installation. Lost create responses resume via login above.
        server.require_empty_installation(admin_key)
        if password:
            data["loginPassword"] = password
            store.save(data)
        server.request(
            "/oidc/users",
            [{"email": email, "password": data["loginPassword"], "delete": False}],
            method="PUT",
            api_key=admin_key,
        )
        server.login(email, data["loginPassword"], username)

    viewer = server.viewer()
    if not matches_user(viewer, data):
        raise BootstrapError("Logged-in identity differs from the configured user")
    if not viewer.get("username"):
        server.graphql(
            "mutation BootstrapEntity($name: String!) { createEntity(input: {name: $name}) { entity { name } } }",
            {"name": username},
        )
        viewer = server.viewer()
    if (
        not matches_user(viewer, data)
        or viewer.get("username") != username
        or not viewer.get("entity")
    ):
        raise BootstrapError("User onboarding did not complete")
    data.update(userId=viewer["id"], entity=viewer["entity"], caBundle=ca_bundle)
    if password:
        data["loginPassword"] = password
    store.save(data)
    result = server.graphql(
        "mutation BootstrapKey($description: String!) { generateApiKey(input: {description: $description}) { secretApiKey apiKey { id } } }",
        {"description": "operator development and Launch"},
    )
    generated = result.get("generateApiKey") or {}
    if not generated.get("secretApiKey"):
        raise BootstrapError("API-key generation returned no key")
    data.update(apiKey=generated["secretApiKey"], apiKeyId=generated["apiKey"]["id"])
    store.save(data)
    server.disable_local_user(admin_key or data["apiKey"])
    return data


def show_credentials(data):
    if os.environ.get("CI", "").lower() not in ("", "0", "false"):
        print("Credentials saved in Kubernetes; credential output suppressed in CI.")
        return
    print(
        f"W&B URL:  {data['baseUrl']}\nUsername: {data['username']}\nEmail:    {data['email']}"
    )
    print(
        "Password: "
        + (data.get("loginPassword") or "not stored (use your existing login password)")
    )
    print("API key:  " + (data.get("apiKey") or "not generated yet"))


def parser():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument(
        "command", choices=["bootstrap", "show-credentials"]
    )
    p.add_argument("--context")
    p.add_argument("--namespace", default="wandb")
    p.add_argument("--name", default="wandb")
    p.add_argument("--secret", default="")
    p.add_argument("--base-url", default="http://wandb.localhost:8080")
    p.add_argument("--username", default="wandb-dev")
    p.add_argument("--email", default="wandb-dev@example.test")
    p.add_argument("--timeout", type=int, default=600)
    ca = p.add_mutually_exclusive_group()
    ca.add_argument("--ca-file")
    ca.add_argument("--ca-secret")
    p.add_argument("--show-credentials", action="store_true")
    return p


def main():
    args = parser().parse_args()
    try:
        kube = Kubernetes(args.namespace, args.context)
        store = CredentialStore(kube, args)
        if args.command == "show-credentials":
            data = store.load()
            if not data:
                raise BootstrapError("No saved credentials; run bootstrap first")
            show_credentials(data)
            return
        cr = wait_for_cr(kube, args.name, args.timeout)
        admin_key = load_admin_key(kube, cr)
        context, bundle = tls_context(args, kube)
        server = Server(args.base_url, context)
        server.wait_ready(args.timeout)
        data = bootstrap(
            server,
            store,
            email=args.email,
            username=args.username,
            password=os.environ.get("WANDB_DEV_PASSWORD"),
            api_key=os.environ.get("WANDB_DEV_API_KEY"),
            admin_key=admin_key,
            ca_bundle=bundle,
        )
        print(f"W&B user ready; credentials stored in {args.namespace}/{store.name}")
        if args.show_credentials:
            show_credentials(data)
    except (BootstrapError, OSError, ValueError, subprocess.TimeoutExpired) as exc:
        # Unexpected OS errors must not print subprocess input or authentication data.
        print(
            str(exc)
            if isinstance(exc, BootstrapError)
            else "Bootstrap failed; check connectivity, CA input, and Kubernetes access",
            file=sys.stderr,
        )
        sys.exit(1)


if __name__ == "__main__":
    main()
