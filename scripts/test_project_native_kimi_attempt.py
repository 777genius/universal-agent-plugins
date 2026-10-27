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
                    "agent_id": "main", "turn_id": 1, "tool_call_id": "provider_owned",
                    "tool_name": attempt.TOOL, "action": action,
                    "tool_input_display": {"kind": "generic", "summary": action, "detail": detail},
                    "created_at": STAMP, "expires_at": "2026-10-27T12:00:00Z"}]}
            if self.ctx.fault in ("permission", "permission-outer"):
                data["items"][0]["tool_name"] = "Bash"
            if self.ctx.fault == "permission-outer":
                data["unmeasured_field"] = TOKEN + KEY
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
        value = {"approval_id": "approval_owned", "session_id": "session_owned", "agent_id": "main", "turn_id": 1,
                 "tool_call_id": "provider_owned", "tool_name": attempt.TOOL, "action": action,
                 "tool_input_display": {"kind": "generic", "summary": action, "detail": args},
                 "created_at": "2026-10-26T12:00:00Z", "expires_at": "2026-10-27T12:00:00Z"}
        return value, args

    def test_approval_requires_exact_session_turn_provider_call_and_arguments(self):
        value, args = self.approval()
        self.assertEqual(attempt.approval(value, "session_owned", "provider_owned", args), ("approval_owned", 1))
        for key, wrong in (("session_id", "session_foreign"), ("agent_id", "foreign"), ("turn_id", True),
                           ("turn_id", 0), ("tool_call_id", "foreign-call"), ("tool_name", "Bash"),
                           ("action", "Approve everything"), ("approval_id", 7), ("scope", "session")):
            with self.subTest(key=key, wrong=wrong):
                changed = copy.deepcopy(value)
                changed[key] = wrong
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


if __name__ == "__main__":
    unittest.main()
