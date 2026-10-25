"""Inert Kimi collector contracts; never acquires or executes vendor bytes."""
import importlib.util
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import tempfile
import time
import unittest
from unittest.mock import patch
from urllib.parse import parse_qs, urlsplit

SPEC = importlib.util.spec_from_file_location(
    "kimi_project_collector", Path(__file__).with_name("project_native_collectors") / "kimi.py")
kimi = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(kimi)


def help_body(command):
    name = "kimi" + (" " + command if command else "")
    return ("Usage: " + name + " [options]\nOptions:\n  --help\n"
            + ("  --version\nCommands:\n  doctor\n  acp\n  web\n" if not command else "")).encode()


class FakeContext:
    def __init__(self, replacement=None):
        self.phase = "help"
        self.calls = []
        self.replacement = replacement or {}

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        self.calls.append((label, arguments, project, extra_env, timeout))
        command = next(command for name, _, command in kimi.PROBES if name == label)
        default = ({"exit": 0, "error": None}, b"2.3.2\n" if command is None else help_body(command), b"")
        return self.replacement.get(label, default)


class FakeService:
    """Inert API double, not a native client or an MCP implementation."""
    token = "A" * 43

    def __init__(self, ctx):
        self.ctx, self.calls, self.trust = ctx, [], {key: False for key in kimi.WORKSPACES}
        self.ready, self.shutdown, self.bound = False, False, None
        self.published = False
        self.ticks = 0
        self.ids = {scope: "wd_project-" + scope + "_" + (str(index) * 12)
                    for index, scope in enumerate(kimi.WORKSPACES, 1)}
        self.token_path = ctx.home / ".kimi-code" / "server.token"
        self.token_path.write_bytes(self.token.encode())
        self.token_path.chmod(0o600)
        self.stdout = ("  ▐█▛█▛█▌  Kimi server ready  2.3.2\n"
                       "  Local:    http://127.0.0.1:31415/#token=" + self.token
                       + "\n  Token:    " + self.token + "\n").encode()
        self.stderr = b""
        if ctx.startup_change:
            ctx.startup_change(self)

    def snapshot(self):
        return self.stdout, self.stderr

    def remaining(self):
        self.ticks += 1
        return 50.0 if self.ticks < 200 else 0.0

    def bind_origin(self, origin):
        if self.bound is not None or origin != "http://127.0.0.1:31415":
            raise ValueError("inert origin refusal")
        self.bound = origin

    def capture_ready(self):
        self.ready = True

    def mark_shutdown(self):
        self.shutdown = True

    def config(self, scope):
        root = self.ctx.home if scope == "user" else self.ctx.projects[scope]
        path = root / ".kimi-code" / "mcp.json"
        name = kimi.PREFIX + scope
        return path, name, json.loads(path.read_bytes())["mcpServers"][name]

    def skills(self, scope):
        rows = []
        for selected in (scope, "user"):
            root = self.ctx.home if selected == "user" else self.ctx.projects[selected]
            name = kimi.PREFIX + selected
            rows.append({"name": name, "description": "Owned Kimi API research marker for " + selected,
                         "path": str(root / ".kimi-code" / "skills" / name / "SKILL.md"),
                         "source": "user" if selected == "user" else "project", "scopes": ["tui", "web"]})
        return {"skills": rows}

    def catalog(self, scope):
        rows = []
        for selected in ((scope, "user") if scope in ("a", "b") else ("user",)):
            path, name, config = self.config(selected)
            if selected != "user":
                config["envKeys"] = sorted(config.pop("env"))
            rows.append({"name": name, "config": config, "source": "global",
                         "origin": str(path), "mutable": selected == "user"})
        return rows

    def probe_events(self, scope):
        _, _, config = self.config(scope)
        event = Path(config["env"]["UAP_TEST_EVENTS"])
        pid = 1200 + ord(scope)
        session = str(pid) + "-" + str(time.time_ns())
        with event.open("ab") as stream:
            for method in ("initialize", "tools/list"):
                row = {"method": method, "pid": pid, "session": session,
                       "nonce": config["env"]["UAP_TEST_NONCE"],
                       "timestamp": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
                       "facts": None}
                stream.write(json.dumps(row).encode() + b"\n")
        event.chmod(0o600)

    def http(self, label, method, path, token=None, json_body=None, timeout=5):
        self.calls.append((label, method, path, token, json_body, timeout))
        if not self.ready or not self.bound or not 0 < timeout <= 5:
            raise ValueError("inert service used before readiness")
        status, code, data = 200, 0, None
        if path == "/api/v1/healthz":
            self.assert_no_auth(token)
            data = {"ok": True}
        elif path == "/api/v1/meta":
            if token is None:
                status, code = 401, 40101
            else:
                data = {"server_version": "2.3.2", "capabilities": dict.fromkeys(
                    ("websocket", "file_upload", "fs_query", "mcp", "tasks", "terminal"), True),
                    "server_id": "inert-server", "started_at": "2026-10-25T00:00:00.000Z",
                    "open_in_apps": [], "dangerous_bypass_auth": False, "backend": "v2",
                    "features": [{"name": "inert", "state": "Active", "meta": {}}],
                    "experimental_flags": {}, "web_title": ""}
        elif path == "/api/v1/workspaces":
            scope = next(key for key in kimi.WORKSPACES if json_body["root"] == str(self.ctx.projects[key]))
            data = {"id": self.ids[scope], "root": str(self.ctx.projects[scope]),
                    "name": kimi.PREFIX + scope, "created_at": "2026-10-25T00:00:00.000Z",
                    "last_opened_at": "2026-10-25T00:00:00.000Z", "session_count": 0}
        elif path.startswith("/api/v1/workspaces/"):
            scope = next(key for key, value in self.ids.items() if "/" + value + "/" in path)
            if path.endswith("/trust"):
                if method == "POST":
                    if scope == "c" or json_body != {}:
                        raise ValueError("inert trust mutation outside A/B")
                    self.trust[scope] = True
                data = {"trusted": self.trust[scope]}
            elif path.endswith("/skills"):
                data = self.skills(scope)
        elif path.startswith("/api/v2/mcp/servers?"):
            cwd = parse_qs(urlsplit(path).query)["cwd"][0]
            scope = next(key for key, value in self.ctx.projects.items() if str(value) == cwd)
            data = self.catalog(scope)
        elif path == "/api/v2/mcp/servers:test":
            if set(json_body) != {"name", "cwd"}:
                raise ValueError("inert MCP test must use native name and cwd")
            scope = json_body["name"].removeprefix(kimi.PREFIX)
            if scope == "c":
                code = 40408
            else:
                self.probe_events(scope)
                data = {"success": True,
                        "output": ('Connected to MCP server "' + kimi.PREFIX + scope
                                   + '".\nAvailable tools: 1\n- inspect_runtime: Inspect only disposable fixture runtime facts.')}
        elif path == "/api/v1/shutdown":
            if json_body != {}:
                raise ValueError("inert shutdown body differs")
            data = {"ok": True}
        else:
            raise ValueError("inert unplanned API request")
        if label not in ("kimi-health", "kimi-meta-unauthorized") and token != self.token:
            raise ValueError("inert bearer differs")
        envelope = {"code": code, "msg": "success" if code == 0 else "planned negative",
                    "data": data, "request_id": "request-" + str(len(self.calls))}
        if self.ctx.response_change:
            changed = self.ctx.response_change(self, label, status, envelope)
            if changed is not None:
                return changed
        return status, json.dumps(envelope).encode()

    @staticmethod
    def assert_no_auth(token):
        if token is not None:
            raise ValueError("inert public health unexpectedly authenticated")


class ApiContext(FakeContext):
    def __init__(self, root, *, startup_change=None, response_change=None, cleanup_failure=False):
        super().__init__()
        self.phase, self.root = "observe", Path(root)
        self.home, self.private = self.root / "home", self.root / "private"
        self.projects = {key: self.root / ("outside" if key == "user" else "project-" + key)
                         for key in (*kimi.WORKSPACES, "user")}
        for path in (self.home, self.private, *self.projects.values()):
            path.mkdir(mode=0o700)
        self.native_probe = self.root / "native-probe"
        self.native_probe.write_bytes(b"inert probe identity; never executed")
        self.secrets, self.services = [], []
        self.startup_change, self.response_change = startup_change, response_change
        self.cleanup_failure = cleanup_failure

    def register_secret(self, value):
        self.secrets.append(value.encode() if isinstance(value, str) else value)

    def service(self, label, args, observe, project="a", timeout=60):
        if (label, args, project, timeout) != ("kimi-web-api", ["web", "--no-open", "--host", "127.0.0.1", "--port", "0"], "a", 60):
            raise ValueError("inert service selection differs")
        service = FakeService(self)
        self.services.append(service)
        try:
            result = observe(service)
            if not service.ready or not service.shutdown or self.cleanup_failure:
                raise ValueError("inert unsuccessful native cleanup")
            service.published = True
            return result
        except Exception:
            service.published = False
            raise


class KimiCollectorTests(unittest.TestCase):
    def test_frozen_archive_is_current_official_native_entry(self):
        self.assertEqual(kimi.CLIENT, "kimi")
        self.assertEqual(kimi.VERSION, "2.3.2")
        self.assertEqual(kimi.ACQUISITION, {
            "url": "https://github.com/MoonshotAI/kimi-code/releases/download/"
                   "%40moonshot-ai/kimi-code%402.3.2/kimi-code-linux-x64.tar.gz",
            "integrity": "sha256:4715ccd1a591b64683929a14b26b3d07ac4c484e4da3881b22a2ccca56304042",
            "max_bytes": 96 << 20, "expected_bytes": 65_938_009,
            "format": "tar.gz", "entrypoint": "kimi", "runtime": "native",
        })

    def test_only_fixed_help_surface_is_observed_without_scope_claim(self):
        ctx = FakeContext()
        result = kimi.collect(ctx)
        self.assertEqual(kimi.AVAILABLE_PHASES, ("acquire", "help", "observe"))
        self.assertEqual(ctx.calls, [
            ("kimi-version", ["--version"], "a", None, 30),
            ("kimi-help", ["--help"], "a", None, 30),
            ("kimi-doctor-help", ["doctor", "--help"], "a", None, 30),
            ("kimi-acp-help", ["acp", "--help"], "a", None, 30),
            ("kimi-web-help", ["web", "--help"], "a", None, 30),
        ])
        self.assertEqual([row["id"] for row in result["cases"]], ["kimi.help"])
        self.assertEqual(result["cases"][0]["status"], "passed")
        self.assertIn("kimi.scope-observation", result["unobserved"])
        self.assertIn("kimi.mcp-tools-call", result["unobserved"])
        self.assertEqual(list(result["observations"]), [call[0] for call in ctx.calls])
        self.assertEqual(result["observations"]["kimi-version"]["surface"], "version")

    def test_wrong_or_ambiguous_version_stops_before_any_help(self):
        for body in (b"2.3.1\n", b"2.3.20\n", b"2.3.2-alpha\n", b"inert 2.3.2\n", b"2.3.2\nextra\n"):
            with self.subTest(body=body):
                ctx = FakeContext({"kimi-version": ({"exit": 0, "error": None}, body, b"")})
                with self.assertRaisesRegex(ValueError, "version differs"):
                    kimi.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_failed_or_missing_process_result_never_becomes_boundary_or_pass(self):
        for record in ({"exit": 1, "error": None}, {"exit": 0, "error": "deadline"},
                       {"exit": False, "error": None}, {"exit": 0}, {}):
            with self.subTest(record=record):
                ctx = FakeContext({"kimi-web-help": (record, help_body("web"), b"")})
                with self.assertRaisesRegex(ValueError, "probe failed"):
                    kimi.collect(ctx)

    def test_wrong_command_empty_non_utf8_and_stderr_help_fail(self):
        for stdout, stderr in ((b"", b""), (help_body("doctor"), b""),
                               (b"Usage: kimi web [options]\n", b""),
                               (b"\xff", b""), (help_body("web"), b"auth required")):
            with self.subTest(stdout=stdout, stderr=stderr):
                ctx = FakeContext({"kimi-web-help": ({"exit": 0, "error": None}, stdout, stderr)})
                with self.assertRaises(ValueError):
                    kimi.collect(ctx)

    def test_subcommand_help_cannot_satisfy_main_help_by_mentioning_commands(self):
        body = help_body("doctor") + b"--version acp web\n"
        ctx = FakeContext({"kimi-help": ({"exit": 0, "error": None}, body, b"")})
        with self.assertRaisesRegex(ValueError, "requested command"):
            kimi.collect(ctx)
        self.assertEqual(len(ctx.calls), 2)

    def test_observe_requires_explicit_phase_before_any_vendor_probe(self):
        ctx = FakeContext()
        ctx.phase = "acquire"
        with self.assertRaisesRegex(ValueError, "explicit"):
            kimi.collect(ctx)
        self.assertEqual(ctx.calls, [])

    def test_complete_inert_api_observation_keeps_runtime_and_product_unqualified(self):
        with tempfile.TemporaryDirectory() as root:
            ctx = ApiContext(root)
            result = kimi.collect(ctx)
            self.assertTrue(kimi.NEEDS_NATIVE_PROBE)
            self.assertEqual([row["id"] for row in result["cases"]], ["kimi.help", "kimi.scope-observation"])
            facts = result["observations"]["kimi-web-api"]
            self.assertEqual(facts["api_request_count"], 31)
            self.assertEqual(facts["native_trust_setup"], {"a": True, "b": True, "c": False})
            self.assertEqual(facts["tools_call_count"], 0)
            self.assertFalse(facts["provider_configured"])
            self.assertIn("kimi.project-lifecycle", result["unobserved"])
            self.assertIn("kimi.mcp-tools-call", result["unobserved"])
            self.assertTrue(ctx.services[0].published)
            self.assertTrue(ctx.services[0].shutdown)
            self.assertIn(FakeService.token.encode(), ctx.secrets)
            self.assertNotIn(FakeService.token, json.dumps(result))
            self.assertEqual(set(facts["probe_events"]), {"a", "b"})
            self.assertEqual(len(facts["protected_sha256"]), 10)

    def test_native_token_permissions_and_banner_mismatch_quarantine_before_http(self):
        changes = [lambda s: s.token_path.chmod(0o644),
                   lambda s: s.token_path.write_bytes(b"B" * 43),
                   lambda s: s.token_path.write_bytes(s.token.encode() + b"\n"),
                   lambda s: setattr(s, "stdout", s.stdout.replace(b"127.0.0.1", b"0.0.0.0")),
                   lambda s: setattr(s, "stdout", s.stdout.replace(b"31415", b"70000")),
                   lambda s: setattr(s, "stdout", s.stdout + b"  Token:    " + s.token.encode() + b"\n")]
        for change in changes:
            with self.subTest(change=change), tempfile.TemporaryDirectory() as root:
                ctx = ApiContext(root, startup_change=change)
                with self.assertRaises((ValueError, OSError)):
                    kimi.collect(ctx)
                self.assertFalse(ctx.services[0].published)
                self.assertFalse(ctx.services[0].ready)
                self.assertEqual(ctx.services[0].calls, [])
                self.assertIn(FakeService.token.encode(), ctx.secrets)

    def test_native_token_alias_refused_without_reading_outside(self):
        with tempfile.TemporaryDirectory() as root:
            def alias(service):
                outside = service.ctx.root / "outside-token"
                outside.write_bytes(b"B" * 43)
                outside.chmod(0o600)
                service.token_path.unlink()
                service.token_path.symlink_to(outside)
            ctx = ApiContext(root, startup_change=alias)
            with self.assertRaisesRegex(ValueError, "type, mode or size"):
                kimi.collect(ctx)
            self.assertFalse(ctx.services[0].ready)
            self.assertNotIn(b"B" * 43, ctx.secrets)
            self.assertEqual((ctx.root / "outside-token").read_bytes(), b"B" * 43)

    def test_incomplete_startup_does_not_publish_or_make_http_requests(self):
        with tempfile.TemporaryDirectory() as root, patch.object(kimi.time, "sleep"):
            ctx = ApiContext(root, startup_change=lambda s: setattr(s, "stdout", s.stdout.split(b"  Token:")[0]))
            with self.assertRaisesRegex(ValueError, "readiness deadline"):
                kimi.collect(ctx)
            self.assertEqual(ctx.services[0].calls, [])
            self.assertFalse(ctx.services[0].published)
            self.assertIn(FakeService.token.encode(), ctx.secrets)

    def test_owned_parent_alias_is_refused_before_fixture_writes(self):
        with tempfile.TemporaryDirectory() as root:
            ctx = ApiContext(root)
            outside = ctx.root / "outside-sentinel"
            outside.mkdir()
            (outside / "sentinel").write_bytes(b"preserve")
            (ctx.projects["a"] / ".kimi-code").symlink_to(outside, target_is_directory=True)
            with self.assertRaises((ValueError, OSError)):
                kimi.collect(ctx)
            self.assertEqual(list(outside.iterdir()), [outside / "sentinel"])
            self.assertEqual(ctx.services, [])

    def test_duplicate_json_bool_code_wrong_auth_and_version_are_never_boundaries(self):
        mutations = [
            ("kimi-meta", lambda s, e: (200, b'{"code":0,"code":0,"msg":"success","data":{},"request_id":"r"}')),
            ("kimi-meta", lambda s, e: (200, json.dumps(dict(e, code=False)).encode())),
            ("kimi-meta-unauthorized", lambda s, e: (403, json.dumps(e).encode())),
            ("kimi-meta", lambda s, e: (200, json.dumps(dict(e, data=dict(e["data"], server_version="2.3.1"))).encode())),
        ]
        for target, mutation in mutations:
            with self.subTest(target=target), tempfile.TemporaryDirectory() as root:
                ctx = ApiContext(root, response_change=lambda service, label, status, env:
                                 mutation(service, env) if label == target else None)
                with self.assertRaises(ValueError):
                    kimi.collect(ctx)
                self.assertFalse(ctx.services[0].published)
                self.assertFalse(ctx.services[0].shutdown)

    def test_cross_project_mcp_origin_or_unredacted_env_refused(self):
        def wrong_origin(service, env):
            env["data"][0]["origin"] = str(service.ctx.projects["b"] / ".kimi-code" / "mcp.json")
        def leaked_env(service, env):
            env["data"][0]["config"]["env"] = {"UAP_TEST_NONCE": "unexpected"}
        def folded_duplicate(service, env):
            row = dict(env["data"][0], name=env["data"][0]["name"].upper())
            env["data"].append(row)
        for mutation in (wrong_origin, leaked_env, folded_duplicate):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as root:
                def change(service, label, status, env):
                    if label == "kimi-mcp-a":
                        mutation(service, env)
                ctx = ApiContext(root, response_change=change)
                with self.assertRaisesRegex(ValueError, "MCP"):
                    kimi.collect(ctx)
                self.assertFalse(ctx.services[0].published)

    def test_wrong_probe_nonce_session_or_tool_call_refuses_scope_pass(self):
        for field, value in (("nonce", "wrong"), ("session", "999-1234567890123456"),
                             ("method", "tools/call")):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as root:
                def change(service, label, status, env):
                    if label == "kimi-test-a":
                        _, _, config = service.config("a")
                        path = Path(config["env"]["UAP_TEST_EVENTS"])
                        rows = [json.loads(line) for line in path.read_bytes().splitlines()]
                        rows[0][field] = value
                        path.write_bytes(b"".join(json.dumps(row).encode() + b"\n" for row in rows))
                ctx = ApiContext(root, response_change=change)
                with self.assertRaisesRegex(ValueError, "probe"):
                    kimi.collect(ctx)
                self.assertFalse(ctx.services[0].published)

    def test_stale_probe_event_timestamp_does_not_prove_current_request(self):
        with tempfile.TemporaryDirectory() as root:
            def change(service, label, status, env):
                if label == "kimi-test-a":
                    _, _, config = service.config("a")
                    path = Path(config["env"]["UAP_TEST_EVENTS"])
                    rows = [json.loads(line) for line in path.read_bytes().splitlines()]
                    for row in rows:
                        row["timestamp"] = "2020-01-01T00:00:00Z"
                    path.write_bytes(b"".join(json.dumps(row).encode() + b"\n" for row in rows))
            ctx = ApiContext(root, response_change=change)
            with self.assertRaisesRegex(ValueError, "request window"):
                kimi.collect(ctx)

    def test_untrusted_negative_exact_code_and_no_events_are_required(self):
        def wrong_code(service, env):
            env["code"] = 40401
        def ran_c(service, env):
            service.probe_events("c")
        for mutation in (wrong_code, ran_c):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as root:
                def change(service, label, status, env):
                    if label == "kimi-test-untrusted-c":
                        mutation(service, env)
                ctx = ApiContext(root, response_change=change)
                with self.assertRaises(ValueError):
                    kimi.collect(ctx)
                self.assertFalse(ctx.services[0].published)

    def test_protected_foreign_bytes_and_successful_cleanup_are_mandatory(self):
        with tempfile.TemporaryDirectory() as root:
            def change(service, label, status, env):
                if label == "kimi-skills-c-repeat":
                    (service.ctx.private / "kimi-foreign-preservation.txt").write_bytes(b"changed")
            ctx = ApiContext(root, response_change=change)
            with self.assertRaisesRegex(ValueError, "protected fixture"):
                kimi.collect(ctx)
            self.assertFalse(ctx.services[0].shutdown)
        with tempfile.TemporaryDirectory() as root:
            ctx = ApiContext(root, cleanup_failure=True)
            with self.assertRaisesRegex(ValueError, "cleanup"):
                kimi.collect(ctx)
            self.assertTrue(ctx.services[0].shutdown)
            self.assertFalse(ctx.services[0].published)


if __name__ == "__main__":
    unittest.main()
