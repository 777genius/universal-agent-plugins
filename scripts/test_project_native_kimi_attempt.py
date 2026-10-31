"""Inert admission regressions; never launches Kimi, a provider or a probe."""
import copy
import contextlib
import hashlib
import io
import json
import os
from pathlib import Path
import runpy
import sys
import tempfile
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from project_native_collectors import kimi_attempt as attempt
import project_native_service as live


STAMP = "2026-10-26T12:00:00Z"
TOKEN = "A" * 43
KEY = "uap-kimi-key-" + "b" * 32
# Only this inert Python process is executed by the cleanup regression. It is
# not a native Kimi process and produces no native qualification evidence.
CHILD = "import signal,time; signal.signal(signal.SIGTERM,lambda *_:exit(0)); print('inert-ready',flush=True); time.sleep(30)"


class CaptureService(live.Service):
    """Inert HTTP receipts with the unchanged real process/cleanup supervisor."""

    def __init__(self, ctx):
        super().__init__("kimi-single-tool", [sys.executable, "-u", "-c", CHILD],
                         ctx.root, {}, 10, ctx.secrets)
        self.ctx, self.calls = ctx, []

    def http(self, label, method, path, token=None, json_body=None, timeout=5):
        self.calls.append((label, method, path, token, json_body))
        if token is not None and token != TOKEN:
            raise ValueError("inert bearer mismatch")
        if self.ctx.fault in ("transport", "private-transport") and label == "create-session":
            raise ValueError(TOKEN + KEY + " /private/path nonce-session" if self.ctx.fault == "private-transport"
                             else "inert transport failure")
        status, code = (401, 40101) if label == "unauthorized" else (200, 0)
        if label == "health" or label == "shutdown":
            data = {"ok": not (label == "shutdown" and self.ctx.fault == "shutdown")}
        elif label == "create-session":
            data = {"id": "session_owned", "workspace_id": "workspace-a", "title": "",
                    "created_at": STAMP, "updated_at": STAMP, "busy": False,
                    "metadata": {"cwd": str(self.ctx.projects["a"])}, "agent_config": {},
                    "usage": {}, "permission_rules": [], "message_count": 0, "last_seq": 0}
        elif label == "prompt":
            data = {"prompt_id": "prompt-owned", "user_message_id": "message-owned",
                    "status": "running", "content": json_body["content"], "created_at": STAMP}
        elif label.startswith("approvals-"):
            action = "Approve " + attempt.TOOL
            detail = {"nonce": "owned-nonce", "operation": "write", "marker": "owned-marker"}
            data = {"items": [{"approval_id": "approval_owned", "session_id": "session_owned",
                    "agent_id": "main", "turn_id": 0, "tool_call_id": "provider_owned",
                    "tool_name": attempt.TOOL, "action": action,
                    "tool_input_display": {"kind": "generic", "summary": action, "detail": detail},
                    "created_at": STAMP, "expires_at": "2026-10-27T12:00:00Z"}]}
            if self.ctx.fault in ("permission", "permission-outer"):
                data["items"][0]["tool_name"] = "Bash"
            if self.ctx.fault == "permission-outer":
                data["unmeasured_field"] = TOKEN + KEY
            if self.ctx.fault == "later-turn":
                data["items"][0]["turn_id"] = 1
            if self.ctx.fault == "multiple-pending":
                data["items"].append({"private-pending-row": TOKEN + KEY + " /private/path"})
            elif self.ctx.fault == "empty-pending":
                data["items"] = []
        elif label == "approve-once":
            data = {"resolved": True, "resolved_at": STAMP}
        elif label.startswith("snapshot-"):
            data = {"as_of_seq": 1, "epoch": 1, "session": {}, "messages": [],
                    "in_flight_turn": None, "pending_approvals": [], "pending_questions": []}
        elif label == "history":
            data = {"items": [], "has_more": False}
        else:
            data = {}
        if code != 0:
            data = None
        stage = {"create-session": "session", "prompt": "prompt", "approvals-0": "approval",
                 "snapshot-0": "snapshot", "history": "history"}.get(label)
        if stage is not None and stage == self.ctx.stage:
            target = data["items"][0] if stage == "approval" else data
            target["unmeasured_field"] = "bounded response " + TOKEN + " " + KEY
        if self.ctx.fault == "identity" and label == "create-session":
            data["workspace_id"] = "workspace-foreign"
        body = json.dumps({"code": code, "msg": "success" if code == 0 else "inert unauthorized", "data": data,
                           "request_id": "inert-" + str(len(self.calls))}).encode()
        row = {"label": label, "method": method, "path": path, "authenticated": token is not None,
               "status": status, "error": None, "response_file": self.label + "-" + label + ".http.json",
               "response_bytes": len(body), "response_sha256": hashlib.sha256(body).hexdigest()}
        self._http.append(row)
        self._bodies[row["response_file"]] = body
        return status, body


class CaptureContext:
    def __init__(self, root, stage=None, fault=None):
        self.root, self.stage, self.fault = Path(root), stage, fault
        self.home = self.root / "home"
        self.projects = {scope: self.root / scope for scope in (*attempt.kimi.WORKSPACES, "user")}
        for path in (self.home, *self.projects.values()):
            path.mkdir()
        self.secrets, self.captures, self.services = [KEY.encode()], {}, []
        self.summary = {"commands": [], "qualification": dict.fromkeys(attempt.shared.QUALIFICATION_CASES, False)}
        self.config = self.root / "installed-mcp.json"
        self.config.write_text('{"mcpServers":{"uap-kimi-native-a":{"command":"inert"}}}')
        self.skill = self.root / "SKILL.md"
        self.skill.write_bytes(b"inert installed skill")
        self.data = self.root / "data"
        self.data.mkdir()

    def request(self):
        return {"server": {"name": "uap-kimi-native-a", "origin": str(self.config),
                           "config": {"command": "inert"}, "sha256": hashlib.sha256(self.config.read_bytes()).hexdigest(),
                           "event": str(self.root / "events.jsonl"), "nonce": "owned-nonce", "cwd": str(self.projects["a"])},
                "skill": {"name": "inert", "description": "inert", "path": str(self.skill),
                          "source": "project", "sha256": hashlib.sha256(self.skill.read_bytes()).hexdigest()},
                "call_id": "provider_owned", "provider_key": KEY, "nonce": "owned-nonce",
                "marker": "owned-marker", "data": str(self.data)}

    def service(self, label, args, observe, project="a", timeout=60):
        if (label, args, project, timeout) != ("kimi-single-tool",
                ["web", "--no-open", "--host", "127.0.0.1", "--port", "0"], "a", 60):
            raise ValueError("inert service selection differs")
        service = CaptureService(self)
        self.services.append(service)
        record, stdout, stderr, result, requests, bodies = service.run(observe)
        self.summary["commands"].append(record)
        self.captures.update({record["stdout_file"]: stdout, record["stderr_file"]: stderr})
        self.summary.setdefault("http_requests", []).extend(requests)
        self.captures.update(bodies)
        if record["error"] or record["exit"] != 0:
            raise ValueError("inert common service refused observation/cleanup")
        return result

    def ready(self, service):
        while b"inert-ready\n" not in service.snapshot()[0]:
            time.sleep(0.01)
        self.secrets.append(TOKEN.encode())
        if self.fault in ("readiness", "private-readiness"):
            raise ValueError(TOKEN + KEY + " /private/path nonce-session" if self.fault == "private-readiness"
                             else "inert failed readiness")
        service.bind_origin("http://127.0.0.1:31415")
        service.capture_ready()
        return TOKEN


class KimiAttemptContracts(unittest.TestCase):
    def test_single_tool_context_uses_original_existing_profile_without_initialization(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            for name in ("home", "tmp", "xdg-config", "xdg-cache", "xdg-state", "xdg-data"):
                (root / name).mkdir(mode=0o700)
            config = root / "home" / "original-provider-config"
            config.write_bytes(b"private existing provider canary\n")
            before = config.lstat()
            held = {"HOME": str(root / "home"), "USERPROFILE": str(root / "home"),
                    "TMPDIR": str(root / "tmp"), "TEMP": str(root / "tmp"), "TMP": str(root / "tmp"),
                    "XDG_CONFIG_HOME": str(root / "xdg-config"), "XDG_CACHE_HOME": str(root / "xdg-cache"),
                    "XDG_STATE_HOME": str(root / "xdg-state"), "XDG_DATA_HOME": str(root / "xdg-data"),
                    "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull, "GIT_TERMINAL_PROMPT": "0"}
            for _ in range(2):
                with patch.dict(os.environ, held):
                    env = attempt.kimi._installed_environment_from_process(root, root / "home")
                self.assertEqual({key: env[key] for key in held}, held)
                self.assertEqual(len(env), 17)
                self.assertEqual(env["KIMI_CODE_NO_AUTO_UPDATE"], "1")
                self.assertNotIn("GITHUB_ACTIONS", env)
            self.assertEqual(config.read_bytes(), b"private existing provider canary\n")
            self.assertEqual(attempt.kimi._identity(config.lstat()), attempt.kimi._identity(before))
            self.assertFalse((root / "home" / "config").exists())

    def observe_capture(self, ctx, closed=True, diagnostic=None):
        # Previously qualified workspace/catalog validators are separate inert
        # contracts. Exercise only the new response stages and real cleanup.
        with (
            patch.object(attempt.kimi, "_ready", side_effect=lambda _ctx, service: ctx.ready(service)),
            patch.object(attempt.kimi, "_meta"),
            patch.object(attempt.kimi, "_workspace", side_effect=lambda row, fixture:
                         "workspace-" + fixture["name"].removeprefix("uap-kimi-native-")),
            patch.object(attempt.kimi, "_trust"),
            patch.object(attempt.kimi, "_installed_catalog"),
            patch.object(attempt.kimi, "_installed_skills"),
            patch.object(live, "port_closed", return_value=closed),
        ):
            return attempt.observe(ctx, ctx.request(), diagnostic)

    def test_generic_failure_diagnostic_retains_actual_stage_and_cleanup_without_private_capture(self):
        cases = (("private-readiness", None, "ready", "enter", None),
                 ("private-transport", None, "create-session", "request", "create-session"),
                 ("identity", "session", "create-session", "validate", "create-session"),
                 ("permission", "approval", "approvals", "validate", "approvals-0"),
                 ("shutdown", "session", "shutdown", "validate", "shutdown"),
                 ("cleanup", "history", "service-cleanup", "complete", "shutdown"))
        for fault, stage, failed_stage, phase, last_call in cases:
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as root:
                ctx = CaptureContext(root, stage, fault)
                diagnostic = attempt.kimi._KimiFailureDiagnostic("single-tool")
                with self.assertRaisesRegex(ValueError, "common service refused") as caught:
                    self.observe_capture(ctx, closed=fault != "cleanup", diagnostic=diagnostic)
                line = diagnostic.line(caught.exception)
                value = json.loads(line.removeprefix("Kimi collector failure: "))
                self.assertEqual((value["stage"], value["phase"]), (failed_stage, phase))
                self.assertEqual(value["exception_kind"], "validation")
                self.assertTrue(value["observer_entered"])
                self.assertTrue(value["service_recorded"])
                self.assertTrue(value["cleanup_reported"])
                self.assertEqual(value["cleanup"], ctx.summary["commands"][0]["cleanup"])
                calls = [row[0] for row in ctx.services[0].calls]
                self.assertEqual(calls[-1] if calls else None, last_call)
                if failed_stage not in ("shutdown", "service-cleanup"):
                    self.assertNotIn("shutdown", calls)
                for private in (TOKEN, KEY, "/private/path", "nonce-session", root, "provider_owned"):
                    self.assertNotIn(private, line)
                self.assertLessEqual(len(line.encode()), 2048)
                self.assertEqual(line.count("\n"), 1)
                self.assertEqual(ctx.summary["http_requests"], [])
                self.assertFalse(any(name.endswith(".http.json") for name in ctx.captures))
                self.assertTrue(all(TOKEN.encode() not in body and KEY.encode() not in body
                                    for body in ctx.captures.values()))

    def test_fresh_turn_zero_reaches_only_exact_one_call_approval_and_later_turn_stops(self):
        for fault in (None, "later-turn"):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as root:
                ctx = CaptureContext(root, fault=fault)
                if fault is None:
                    result = self.observe_capture(ctx)
                    self.assertEqual(result["native_turn_id"], 0)
                    self.assertIs(result["native_capture_complete"], False)
                    self.assertIs(result["native_history_grammar_frozen"], False)
                    approved = [row for row in ctx.services[0].calls if row[0] == "approve-once"]
                    self.assertEqual(len(approved), 1)
                    self.assertEqual(approved[0][1:3], ("POST", "/api/v1/sessions/session_owned/approvals/approval_owned"))
                    self.assertEqual(approved[0][4], {"decision": "approved"})
                    self.assertTrue(ctx.summary["commands"][0]["cleanup"]["shutdown_acknowledged"])
                    self.assertFalse((ctx.data / "native-marker.txt").exists())
                else:
                    with self.assertRaisesRegex(ValueError, "common service refused"):
                        self.observe_capture(ctx)
                    labels = [row[0] for row in ctx.services[0].calls]
                    self.assertEqual(labels[-1], "approvals-0")
                    self.assertNotIn("approve-once", labels)
                    self.assertNotIn("shutdown", labels)
                    self.assertEqual(ctx.summary["http_requests"], [])

    def test_approval_facts_survive_the_real_callback_and_supervisor_error_replacement(self):
        for fault in ("permission", "permission-outer", "later-turn"):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as root:
                ctx = CaptureContext(root, fault=fault)
                diagnostic = attempt.kimi._KimiFailureDiagnostic("single-tool")
                with self.assertRaisesRegex(ValueError, "common service refused") as caught:
                    self.observe_capture(ctx, diagnostic=diagnostic)
                self.assertNotIsInstance(caught.exception, attempt.kimi_approval_facts.Refusal)
                line = diagnostic.line(caught.exception)
                value = json.loads(line.removeprefix("Kimi collector failure: "))
                facts = value["approval_refusal"]
                self.assertEqual((facts["topic"], facts["status"]), ("single-approval", "captured"))
                self.assertEqual(facts["turn"], "positive" if fault == "later-turn" else "zero")
                self.assertEqual(facts["equals"]["tool"], fault == "later-turn")
                self.assertEqual(value["cleanup"], ctx.summary["commands"][0]["cleanup"])
                self.assertEqual(ctx.services[0].calls[-1][0], "approvals-0")
                self.assertNotIn("approve-once", [row[0] for row in ctx.services[0].calls])
                self.assertEqual(ctx.summary["http_requests"], [])
                self.assertFalse(any(name.endswith(".http.json") for name in ctx.captures))
                self.assertLessEqual(len(line.encode()), 2048)
                for private in (TOKEN, KEY, root, "session_owned", "provider_owned", "approval_owned", "owned-nonce"):
                    self.assertNotIn(private, line)

    def test_both_real_entrypoint_failure_branches_emit_only_finite_diagnostic_then_fail(self):
        for module, argument, collector, reason in (
                (attempt.kimi, "--installed-request", "installed", "Kimi installed API observation failed; private capture quarantined"),
                (attempt, "--attempt-request", "single-tool", "Kimi single-tool observation failed; private native streams quarantined")):
            stdout, stderr = io.StringIO(), io.StringIO()
            private = "/private/request-bearer-nonce-session"
            with self.subTest(collector=collector), patch.dict(os.environ, {"GITHUB_ACTIONS": "false"}), \
                    patch.object(sys, "argv", [module.__file__, argument, private]), \
                    contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                with self.assertRaises(SystemExit) as caught:
                    runpy.run_path(module.__file__, run_name="__main__")
            self.assertEqual(caught.exception.code, reason)
            self.assertEqual(stdout.getvalue(), "")
            line = stderr.getvalue()
            self.assertEqual(line.count("\n"), 1)
            self.assertNotIn(private, line)
            value = json.loads(line.removeprefix("Kimi collector failure: "))
            self.assertEqual((value["collector"], value["stage"]), (collector, "host-admission"))
            self.assertFalse(value["observer_entered"])
            self.assertFalse(value["service_recorded"])
            self.assertTrue(all(item is None for item in value["cleanup"].values()))

    def test_finite_schema_refusal_stops_actions_then_retains_only_admitted_redacted_failure(self):
        for stage, refused in (("session", "create-session"), ("prompt", "prompt"),
                               ("approval", "approvals-0"), ("snapshot", "snapshot-0"),
                               ("history", "history")):
            with self.subTest(stage=stage), tempfile.TemporaryDirectory() as temporary:
                ctx = CaptureContext(temporary, stage)
                result = self.observe_capture(ctx)
                ctx.summary["observations"] = result
                self.assertIs(result["response_schema_refused"], True)
                self.assertEqual(result["failed_stage"], stage)
                self.assertIs(result["native_capture_complete"], False)
                self.assertIs(result["native_history_grammar_frozen"], False)
                labels = [row[0] for row in ctx.services[0].calls]
                self.assertEqual(labels[labels.index(refused) + 1:], ["shutdown"])
                self.assertEqual(ctx.services[0].calls[-1],
                                 ("shutdown", "POST", "/api/v1/shutdown", TOKEN, {}))
                cleanup = ctx.summary["commands"][0]["cleanup"]
                self.assertTrue(cleanup["shutdown_acknowledged"])
                self.assertTrue(cleanup["group_reaped"])
                self.assertTrue(cleanup["port_closed"])
                self.assertFalse(cleanup["forced_kill"])
                output = io.BytesIO()
                with patch.object(attempt.sys, "stdout", SimpleNamespace(buffer=output)):
                    self.assertEqual(attempt.emit_capture(ctx), 1)
                body = output.getvalue()
                self.assertNotIn(TOKEN.encode(), body)
                self.assertNotIn(KEY.encode(), body)
                envelope = json.loads(body)
                raw = envelope["files"]["kimi-single-tool-" + refused + ".http.json"]
                self.assertIn("unmeasured_field", raw)
                self.assertIn("[redacted]", raw)
                summary = json.loads(envelope["files"]["research.json"])
                self.assertTrue(all(flag is False for flag in summary["qualification"].values()))
                self.assertEqual(set(result) & {"native_session_id", "native_turn_id", "approval_id"},
                                 set() if stage == "session" else {"native_session_id"} if stage in ("prompt", "approval")
                                 else {"native_session_id", "native_turn_id", "approval_id"})

    def test_identity_permission_readiness_transport_shutdown_or_cleanup_failure_remains_quarantined(self):
        for fault, stage in (("identity", "session"), ("permission", "approval"), ("permission-outer", None),
                             ("transport", None), ("readiness", None),
                             ("shutdown", "session"), ("cleanup", "history")):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as temporary:
                ctx = CaptureContext(temporary, stage, fault)
                with self.assertRaisesRegex(ValueError, "common service refused"):
                    self.observe_capture(ctx, closed=fault != "cleanup")
                self.assertEqual(ctx.summary["http_requests"], [])
                self.assertTrue(all(not name.endswith(".http.json") for name in ctx.captures))
                self.assertTrue(all(TOKEN.encode() not in body and KEY.encode() not in body
                                    for body in ctx.captures.values()))
                if fault in ("identity", "permission", "permission-outer", "transport", "readiness"):
                    self.assertNotIn("shutdown", [row[0] for row in ctx.services[0].calls])

    def approval(self):
        action = "Approve " + attempt.TOOL
        args = {"nonce": "owned-nonce", "operation": "write", "marker": "owned-marker"}
        value = {"approval_id": "approval_owned", "session_id": "session_owned", "agent_id": "main", "turn_id": 0,
                 "tool_call_id": "provider_owned", "tool_name": attempt.TOOL, "action": action,
                 "tool_input_display": {"kind": "generic", "summary": action, "detail": args},
                 "created_at": "2026-10-26T12:00:00Z", "expires_at": "2026-10-27T12:00:00Z"}
        return value, args

    def test_approval_requires_exact_session_turn_provider_call_and_arguments(self):
        value, args = self.approval()
        self.assertEqual(attempt.approval(value, "session_owned", "provider_owned", args), ("approval_owned", 0))
        for key, wrong in (("session_id", "session_foreign"), ("agent_id", "foreign"), ("turn_id", True),
                           ("turn_id", -1), ("turn_id", 1), ("turn_id", 100), ("turn_id", "0"), ("turn_id", None),
                           ("tool_call_id", "foreign-call"), ("tool_name", "Bash"),
                           ("action", "Approve everything"), ("approval_id", 7), ("scope", "session")):
            with self.subTest(key=key, wrong=wrong):
                changed = copy.deepcopy(value)
                changed[key] = wrong
                with self.assertRaises(ValueError):
                    attempt.approval(changed, "session_owned", "provider_owned", args)
        changed = copy.deepcopy(value)
        del changed["turn_id"]
        with self.assertRaises(ValueError):
            attempt.approval(changed, "session_owned", "provider_owned", args)
        changed = copy.deepcopy(value)
        changed["tool_input_display"]["detail"]["operation"] = "read"
        with self.assertRaises(ValueError):
            attempt.approval(changed, "session_owned", "provider_owned", args)

    def test_json_refuses_nonfinite_duplicate_and_excessive_structure(self):
        for body in (b'{"number":1e999}', b'{"number":NaN}', b'{"name":"owned","name":"foreign"}',
                     b'[' * 34 + b'0' + b']' * 34):
            with self.subTest(body=body[:40]), self.assertRaises(ValueError):
                attempt.strict_json(body)
        self.assertEqual(attempt.strict_json(b'{"value":1}'), {"value": 1})

    def test_probe_call_or_marker_before_approval_is_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            ctx = SimpleNamespace(root=root)
            event, marker = root / "events.jsonl", root / "native-marker.txt"
            attempt.no_call(ctx, event, marker)
            row = {"method": "initialize", "pid": 123, "session": "123-1", "nonce": "owned", "timestamp": "2026-10-26T12:00:00Z", "facts": None}
            event.write_text(json.dumps(row) + "\n")
            attempt.no_call(ctx, event, marker)
            row["method"] = "tools/call"
            event.write_text(json.dumps(row) + "\n")
            with self.assertRaises(ValueError):
                attempt.no_call(ctx, event, marker)
            event.unlink()
            marker.write_bytes(b"")
            with self.assertRaises(ValueError):
                attempt.no_call(ctx, event, marker)


class KimiApprovalFactsContracts(unittest.TestCase):
    def fixture(self):
        args = {"nonce": "PRIVATE-NONCE", "operation": "write", "marker": "PRIVATE-MARKER"}
        action = "Approve " + attempt.TOOL
        return {"approval_id": "approval_private", "session_id": "session_private", "agent_id": "main", "turn_id": 0,
                "tool_call_id": "call_private", "tool_name": attempt.TOOL, "action": action,
                "tool_input_display": {"kind": "generic", "summary": action, "detail": dict(args)},
                "created_at": STAMP, "expires_at": "2026-10-27T12:00:00Z"}, args

    def project(self, value, args):
        return attempt.kimi_approval_facts.project(value, "session_private", "call_private", args,
                                                 attempt.TOOL, attempt.kimi._instant)

    def test_success_and_schema_refusal_keep_their_exact_original_boundary(self):
        value, args = self.fixture()
        with patch.object(attempt.kimi_approval_facts, "project", side_effect=AssertionError("must not project")):
            self.assertEqual(attempt.approval(value, "session_private", "call_private", args), ("approval_private", 0))
            value["unknown"] = "PRIVATE-UNKNOWN"
            with self.assertRaises(attempt.ResponseSchemaRefusal) as caught:
                attempt.approval(value, "session_private", "call_private", args)
            self.assertIs(type(caught.exception), attempt.ResponseSchemaRefusal)
            self.assertEqual(caught.exception.stage, "approval")
            sentinel = attempt.ResponseSchemaRefusal("approval")
            with patch.object(attempt, "_strict_approval", side_effect=sentinel):
                with self.assertRaises(attempt.ResponseSchemaRefusal) as exact:
                    attempt.approval(value, "session_private", "call_private", args)
                self.assertIs(exact.exception, sentinel)
            for error in (TypeError("PRIVATE-TYPE"), KeyError("PRIVATE-KEY"), OSError("PRIVATE-OS")):
                with patch.object(attempt, "_strict_approval", side_effect=error):
                    with self.assertRaises(type(error)) as exact:
                        attempt.approval(value, "session_private", "call_private", args)
                    self.assertIs(exact.exception, error)

    def test_each_old_identity_and_type_refusal_stays_refused_with_finite_facts(self):
        mutations = [(key, wrong) for key, wrong in (
            ("session_id", "session_foreign"), ("agent_id", "other"), ("tool_call_id", "other"),
            ("tool_name", "Bash"), ("action", "PRIVATE-ACTION"), ("approval_id", "wrong"),
            ("tool_input_display", {}), ("created_at", "PRIVATE-TIME"), ("expires_at", False))]
        mutations.extend(("turn_id", turn) for turn in (-1, 1, 100, True, False, None, "0", [], {}))
        mutations.extend((key, None) for key in attempt.kimi_approval_facts.FIELDS)
        for key, wrong in mutations:
            with self.subTest(key=key, wrong=wrong):
                value, args = self.fixture()
                value[key] = wrong
                with self.assertRaises(ValueError) as original:
                    attempt._strict_approval(value, "session_private", "call_private", args)
                with self.assertRaises(ValueError) as wrapped:
                    attempt.approval(value, "session_private", "call_private", args)
                if type(original.exception) is attempt.ResponseSchemaRefusal:
                    self.assertIs(type(wrapped.exception), attempt.ResponseSchemaRefusal)
                else:
                    self.assertIs(type(wrapped.exception), attempt.kimi_approval_facts.Refusal)
                    facts = wrapped.exception.public()
                    self.assertEqual(facts["fields"][key]["type"], attempt.kimi_approval_facts._type(wrong))
                    self.assertTrue(facts["fields"][key]["present"])
                    self.assertEqual(attempt.kimi_approval_facts.validate(facts), facts)
                    self.assertNotIn("PRIVATE", json.dumps(facts))
        value, args = self.fixture()
        value.update(unknown="PRIVATE-UNKNOWN", tool_name="Bash")
        with self.assertRaises(attempt.kimi_approval_facts.Refusal) as caught:
            attempt.approval(value, "session_private", "call_private", args)
        self.assertFalse(caught.exception.public()["exact_keys"])
        self.assertFalse(caught.exception.public()["equals"]["tool"])
        del value["turn_id"]
        with self.assertRaises(attempt.kimi_approval_facts.Refusal) as caught:
            attempt.approval(value, "session_private", "call_private", args)
        self.assertEqual(caught.exception.public()["fields"]["turn_id"], {"present": False, "type": "absent"})

    def test_projection_only_exports_fixed_booleans_and_categories_not_candidate_values(self):
        value, args = self.fixture()
        for key in attempt.kimi_approval_facts.FIELDS:
            value[key] = "/PRIVATE-PATH bearer-secret\nPRIVATE-CONTROL PRIVATE-ID"
        value["PRIVATE-DYNAMIC-KEY"] = "PRIVATE-VALUE"
        facts = self.project(value, args)
        serialized = json.dumps(facts)
        for canary in ("PRIVATE", "bearer-secret", "session_private", "call_private", attempt.TOOL, STAMP):
            self.assertNotIn(canary, serialized)
        self.assertFalse(facts["exact_keys"])
        self.assertEqual(facts["turn"], "noninteger")
        self.assertFalse(facts["display_object"])
        self.assertTrue(all(result is False for result in facts["equals"].values()))
        for row, kind in ((None, "null"), (True, "boolean"), (0, "integer"), (0.5, "number"),
                          ("PRIVATE", "string"), ([], "array")):
            projected = self.project(row, args)
            self.assertEqual(projected["row_type"], kind)
            self.assertTrue(all(field == {"present": False, "type": "absent"} for field in projected["fields"].values()))

    def test_projection_rejects_excessive_nonbuiltin_and_hostile_objects_before_comparison(self):
        class Hostile:
            def __eq__(self, _):
                raise AssertionError("hostile equality called")

            def __repr__(self):
                raise AssertionError("hostile rendering called")

        class HostileDict(dict):
            def items(self):
                raise AssertionError("hostile iteration called")

        _, args = self.fixture()
        deep = None
        for _ in range(34):
            deep = [deep]
        cycle = []
        cycle.append(cycle)
        for value in (Hostile(), HostileDict(), {"tool_input_display": Hostile()},
                      [0] * 16385, deep, cycle, "PRIVATE" * (1 << 18), float("nan"), {1: "PRIVATE"}):
            self.assertEqual(self.project(value, args), {"topic": "single-approval", "status": "projection-refused"})

    def test_projection_failure_and_malformed_facts_do_not_change_the_original_failure(self):
        value, args = self.fixture()
        value["turn_id"] = 1
        for failure in (ValueError("PRIVATE-EXCEPTION"), {"topic": "PRIVATE-TOPIC"}, ["PRIVATE-FIELD"]):
            patching = {"side_effect": failure} if isinstance(failure, Exception) else {"return_value": failure}
            with patch.object(attempt.kimi_approval_facts, "project", **patching):
                with self.assertRaises(attempt.kimi_approval_facts.Refusal) as caught:
                    attempt.approval(value, "session_private", "call_private", args)
                self.assertEqual(caught.exception.public(), {"topic": "single-approval", "status": "projection-refused"})
        valid = self.project(value, args)
        for path, wrong in ((["topic"], "PRIVATE"), (["row_type"], []), (["turn"], "PRIVATE"),
                            (["exact_keys"], 1), (["fields", "turn_id", "type"], "PRIVATE"),
                            (["fields", "turn_id", "present"], False), (["equals", "call"], "PRIVATE")):
            changed = copy.deepcopy(valid)
            parent = changed
            for key in path[:-1]:
                parent = parent[key]
            parent[path[-1]] = wrong
            with self.assertRaises(ValueError):
                attempt.kimi_approval_facts.validate(changed)
            diagnostic = attempt.kimi._KimiFailureDiagnostic("single-tool")
            diagnostic.approval_refusal = changed
            diagnostic.step("approvals", "validate")
            line = diagnostic.line(ValueError("PRIVATE-ORIGINAL"))
            self.assertNotIn("PRIVATE", line)
            self.assertNotIn("approval_refusal", json.loads(line.removeprefix("Kimi collector failure: ")))
        diagnostic = attempt.kimi._KimiFailureDiagnostic("single-tool")
        diagnostic.step("approvals", "validate")
        first = attempt.kimi_approval_facts.Refusal(valid)

        def refuse(_):
            raise first

        with self.assertRaises(attempt.kimi_approval_facts.Refusal) as caught:
            diagnostic.observer(refuse)(None)
        self.assertIs(caught.exception, first)
        first.facts["unknown"] = "PRIVATE-MUTATION"
        self.assertNotIn("unknown", diagnostic.approval_refusal)
        self.assertLessEqual(len(diagnostic.line(ValueError("PRIVATE-REPLACEMENT")).encode()), 2048)


if __name__ == "__main__":
    unittest.main()
