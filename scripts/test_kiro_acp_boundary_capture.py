"""Inert pin, service and receipt admission checks; no native/privileged child."""
import base64
import hashlib
import json
import os
from pathlib import Path
import stat
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import kiro_acp_boundary_capture as capture


class KiroACPBoundaryContracts(unittest.TestCase):
    def test_workflow_selects_changes_to_the_imported_process_contract(self):
        workflow = (capture.SOURCE / ".github/workflows/kiro-acp-boundary.yml").read_text()
        helper = (capture.SOURCE / "repotests/agentplugins_kiro_acp_boundary_linux_test.go").read_text()
        self.assertIn('"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"', helper)
        self.assertIn("      - 'install/integrationctl/ports/**'", workflow)

    def test_archive_requires_published_pin_and_exact_ordinary_member(self):
        member = {"name": capture.ARCHIVE["entrypoint"], "kind": "file", "bytes": 123, "sha256": "a" * 64}
        receipt = {"url": capture.ARCHIVE["url"], "integrity": capture.ARCHIVE["integrity"],
                   "sha256": capture.ARCHIVE["integrity"][7:], "pin_verified": True,
                   "bytes": capture.ARCHIVE["expected_bytes"], "installer_executed": False, "members": [member]}
        self.assertEqual(capture.archive_identity(receipt), member)
        for key, value in (("url", capture.ARCHIVE["url"].replace("3.0.0", "2.34.0")),
                           ("integrity", "sha256:" + "b" * 64), ("sha256", "b" * 64),
                           ("bytes", True), ("bytes", receipt["bytes"] - 1), ("pin_verified", 1),
                           ("installer_executed", True), ("members", []), ("members", [member, member]),
                           ("members", [{**member, "kind": "link"}]),
                           ("members", [{**member, "name": "kirocli/bin/other"}])):
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                capture.archive_identity({**receipt, key: value})

    def test_service_keeps_ordinary_identity_readonly_inputs_and_exact_leaf(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            (root / "inputs").mkdir()
            helper = root / "inputs/boundary.test"
            helper.write_bytes(b"inert")
            env = {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted",
                   "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1", "UAP_KIRO_BOUNDARY_CAPTURE": "1",
                   "UAP_KIRO_BOUNDARY_IDENTITY": str(root / "inputs/identity.json"), "UAP_KIRO_BOUNDARY_RUNTIME": str(root / "runtime")}
            unit = "uap-kiro-acp-boundary-" + "a" * 32 + ".service"
            argv = capture.service_argv(helper, env, root, unit, 1001, 1001, 123)
            for token in ("--uid=1001", "--gid=1001", "--property=Delegate=yes", "--property=NoNewPrivileges=yes",
                          "--property=ReadOnlyPaths=" + str(root) + " " + str(capture.SOURCE), "--property=ReadWritePaths=" + str(root / "runtime"),
                          "--property=KillMode=control-group", "--property=RuntimeMaxSec=180s", "UAP_KIRO_BOUNDARY_ORIGIN_NS=123",
                          "-test.run=^TestAgentpluginsKiroACPBoundaryCapture$"):
                self.assertIn(token, argv)
            for changed in ({**env, "KIRO_API_KEY": "inert"}, {**env, "LD_PRELOAD": "/inert"},
                            {**env, "PATH": "/ambient"}, {**env, "UAP_KIRO_BOUNDARY_CAPTURE": "0"},
                            {**env, "UAP_KIRO_BOUNDARY_RUNTIME": "/foreign"}):
                with self.subTest(changed=changed), self.assertRaises(ValueError):
                    capture.service_argv(helper, changed, root, unit, 1001, 1001, 123)
            for uid, gid, origin, name in ((0, 1001, 123, unit), (True, 1001, 123, unit),
                                          (1001, 0, 123, unit), (1001, 1001, 0, unit),
                                          (1001, 1001, 123, "uap-cline-project-" + "a" * 32 + ".service")):
                with self.subTest(uid=uid, gid=gid, origin=origin, name=name), self.assertRaises(ValueError):
                    capture.service_argv(helper, env, root, name, uid, gid, origin)

    def test_exact_unit_query_rejects_other_service_duplicate_and_invented_absence(self):
        unit = "uap-kiro-acp-boundary-" + "a" * 32 + ".service"
        body = b"LoadState=not-found\nActiveState=inactive\nControlGroup=\n"
        self.assertEqual(capture.unit_state(body, unit)["LoadState"], "not-found")
        for altered in (body + b"ActiveState=inactive\n", body.replace(b"inactive", b"active"),
                        body.replace(b"ControlGroup=\n", b"ControlGroup=/system.slice/foreign.service\n"),
                        body + b"Secret=inert\n", body.replace(b"not-found", b"unknown")):
            with self.subTest(body=altered), self.assertRaises(ValueError):
                capture.unit_state(altered, unit)

    @staticmethod
    def exchange_receipt():
        identity = {"source": {"commit": "a" * 40, "tree": "b" * 40}, "binary": "/inert/kiro-cli",
                    "helper": "/inert/inputs/boundary.test"}
        service = {"unit": "uap-kiro-acp-boundary-" + "c" * 32 + ".service", "original_start_monotonic_ns": 123,
                   "closed": True, "forced_stop": False, "forced_kill": False, "elapsed_ns": 1000, "uid": 1001, "gid": 1001}
        report = {"diagnostic_only": True, "qualification": dict(capture.QUALIFICATION), "identity": identity,
                  "stderr_scope": "unavailable-from-duplex-api", "stdout_scope": "read-prefix-through-exchange-stop",
                  "stdout_complete": False, "delegation": {"unit": service["unit"], "origin_ns": 123, "uid": 1001, "gid": 1001,
                      "membership": "0::/system.slice/" + service["unit"] + "\n", "no_new_privileges": True}, "capture_complete": True,
                  "observed_records": 1, "phase": "initialize", "cwd": "/inert/runtime/acp/cwd",
                  "duplex_capability": True, "planned_shutdown_verified": True, "stop_reason": "initialize-server-error",
                  "argv": [identity["binary"], "acp", "--agent-engine", "v3", "--auth-method", "cli"]}
        request = {"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": {
            "protocolVersion": 1, "clientCapabilities": {}, "clientInfo": {"name": "agentplugins", "version": "1"}}}
        response = {"jsonrpc": "2.0", "id": 0, "error": {"code": -1, "message": "unclassified native error"}}
        for name, body in (("stdin", json.dumps(request).encode() + b"\n"),
                           ("stdout", json.dumps(response).encode() + b"\n")):
            report[name] = base64.b64encode(body).decode()
            report[name + "_sha256"] = hashlib.sha256(body).hexdigest()
        return identity, service, report

    @staticmethod
    def stream(report, field, body):
        report[field] = base64.b64encode(body).decode()
        report[field + "_sha256"] = hashlib.sha256(body).hexdigest()

    def test_receipt_cannot_promote_diagnostic_or_claim_complete_native_stream(self):
        identity, service, report = self.exchange_receipt()
        encode = lambda value: capture.MARKER + json.dumps(value).encode() + b"\n"
        self.assertEqual(capture.admit_report(encode(report), identity, service), report)
        for key, value in (("diagnostic_only", 1), ("stdout_complete", True), ("stderr_scope", "empty"),
                           ("capture_complete", 1), ("identity", {}), ("stdout_sha256", "f" * 64),
                           ("argv", [identity["binary"], "acp"]), ("planned_shutdown_verified", False),
                           ("runner_error", "forced cleanup"), ("stop_reason", "authenticated"),
                           ("observed_records", True), ("observed_records", 1.0),
                           ("delegation", {"unit": service["unit"], "origin_ns": 124}),
                           ("qualification", {**capture.QUALIFICATION, "mcp-initialize": True}),
                           ("qualification", {**capture.QUALIFICATION, "mcp-initialize": 0})):
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                capture.admit_report(encode({**report, key: value}), identity, service)
        for key, value in (("closed", False), ("closed", 1), ("forced_stop", True), ("forced_kill", True),
                           ("elapsed_ns", True), ("elapsed_ns", 180_000_000_000)):
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                capture.admit_report(encode(report), identity, {**service, key: value})
        for body in (encode(report) * 2, encode(report).replace(b'"diagnostic_only": true', b'"diagnostic_only": true, "diagnostic_only": true'),
                     encode(report).replace(b'"capture_complete": true', b'"capture_complete": NaN')):
            with self.subTest(body=body), self.assertRaises(ValueError):
                capture.admit_report(body, identity, service)

    def test_complete_receipts_require_actual_requests_and_observed_stop_records(self):
        identity, service, original = self.exchange_receipt()
        request = json.loads(base64.b64decode(original["stdin"]))
        response = json.loads(base64.b64decode(original["stdout"]))
        cases = [
            ("stdin", b""), ("stdout", b""), ("stdin", b"initialize\n"),
            ("stdout", b"unclassified boundary\n"),
            ("stdin", json.dumps({**request, "id": False}).encode() + b"\n"),
            ("stdin", json.dumps({**request, "id": 0.0}).encode() + b"\n"),
            ("stdin", json.dumps(request).encode().replace(b'"id": 0', b'"id": -0') + b"\n"),
            ("stdin", json.dumps({**request, "method": "authenticate"}).encode() + b"\n"),
            ("stdout", json.dumps({**response, "id": False}).encode() + b"\n"),
            ("stdout", json.dumps({**response, "id": 1}).encode() + b"\n"),
            ("stdout", json.dumps(response).encode().replace(b'"id": 0', b'"id": -0') + b"\n"),
            ("stdout", json.dumps({**response, "error": {"code": 1.0, "message": "boundary"}}).encode() + b"\n"),
            ("stdout", json.dumps(response).encode()),
            ("stdout", b'{"jsonrpc":"2.0","id":0,"id":0,"error":{"code":-1,"message":"boundary"}}\n'),
            ("stdout", b'{"jsonrpc":"2.0","id":0,"error":{"code":NaN,"message":"boundary"}}\n'),
        ]
        for field, body in cases:
            with self.subTest(field=field, body=body):
                report = json.loads(json.dumps(original))
                self.stream(report, field, body)
                with self.assertRaises(ValueError):
                    capture.admit_report(capture.MARKER + json.dumps(report).encode() + b"\n", identity, service)
        for key, value in (("phase", "session-new"), ("stop_reason", "session-new-result-observed"),
                           ("observed_records", 2), ("cwd", "/unowned/cwd")):
            with self.subTest(key=key), self.assertRaises(ValueError):
                capture.admit_report(capture.MARKER + json.dumps({**original, key: value}).encode() + b"\n", identity, service)
        partial = {**original, "capture_complete": False, "observed_records": 0, "phase": "initialize", "stop_reason": ""}
        self.stream(partial, "stdin", b'{"jsonrpc":')
        self.stream(partial, "stdout", b"incomplete diagnostic prefix")
        self.assertEqual(capture.admit_report(capture.MARKER + json.dumps(partial).encode() + b"\n", identity, service), partial)

    def test_complete_exchange_allows_notifications_and_scanner_read_ahead_only_after_stop(self):
        identity, service, original = self.exchange_receipt()
        initialize = base64.b64decode(original["stdin"])
        initial = {"jsonrpc": "2.0", "id": 0, "result": {"protocolVersion": 1}}
        session = {"jsonrpc": "2.0", "id": 1, "method": "session/new", "params": {
            "cwd": original["cwd"], "mcpServers": []}}
        notice = {"jsonrpc": "2.0", "method": "notification", "params": {"message": "inert"}}
        stops = [
            ("initialize", "initialize-server-error", {"jsonrpc": "2.0", "id": 0, "error": {"code": -1, "message": "boundary"}}),
            ("initialize", "initialize-server-request-unanswered", {"jsonrpc": "2.0", "id": 7, "method": "session/request_permission", "params": {}}),
            ("initialize", "initialize-result-unrecognized", {"jsonrpc": "2.0", "id": 0, "result": {"protocolVersion": 2}}),
            ("session-new", "session-new-server-error", {"jsonrpc": "2.0", "id": 1, "error": {"code": -2, "message": "boundary"}}),
            ("session-new", "session-new-server-request-unanswered", {"jsonrpc": "2.0", "id": 8, "method": "session/request_permission", "params": {}}),
            ("session-new", "session-new-result-observed", {"jsonrpc": "2.0", "id": 1, "result": {"sessionId": "inert"}}),
        ]
        for phase, stop, terminal in stops:
            with self.subTest(stop=stop):
                records = [notice] + ([initial, notice] if phase == "session-new" else []) + [terminal]
                report = {**original, "phase": phase, "stop_reason": stop, "observed_records": len(records)}
                sent = initialize + (json.dumps(session).encode() + b"\n" if phase == "session-new" else b"")
                wire = b"".join(json.dumps(row).encode() + b"\n" for row in records)
                self.stream(report, "stdin", sent)
                self.stream(report, "stdout", wire + b"unparsed scanner read-ahead including a partial record")
                packet = capture.MARKER + json.dumps(report).encode() + b"\n"
                self.assertEqual(capture.admit_report(packet, identity, service), report)
                past_stop = {**report, "observed_records": len(records) + 1}
                self.stream(past_stop, "stdout", wire + json.dumps(notice).encode() + b"\n")
                with self.assertRaises(ValueError):
                    capture.admit_report(capture.MARKER + json.dumps(past_stop).encode() + b"\n", identity, service)
                if phase == "session-new":
                    for changed in ({**session, "params": {"cwd": original["cwd"], "mcpServers": [{}]}},
                                    {**session, "params": {"cwd": "/foreign", "mcpServers": []}},
                                    {**session, "method": "session/prompt"}):
                        broken = dict(report)
                        self.stream(broken, "stdin", initialize + json.dumps(changed).encode() + b"\n")
                        with self.assertRaises(ValueError):
                            capture.admit_report(capture.MARKER + json.dumps(broken).encode() + b"\n", identity, service)

    def service_fixture(self, mode, seconds=180):
        """Only manager transports/host facts are inert; real input hashing remains active."""
        clock = [100.0]
        class Budget:
            def __init__(self, seconds): self.deadline = clock[0] + seconds
            def check(self):
                remaining = self.deadline - clock[0]
                if remaining <= 0: raise ValueError("inert budget exhausted")
                return remaining
            def __enter__(self): self.check(); return self
            def __exit__(self, *_): return False
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            (root / "inputs").mkdir()
            helper = root / "inputs/boundary.test"
            helper.write_bytes(b"inert helper bytes; never executed")
            initial_budget = Budget(seconds)
            ctx = SimpleNamespace(root=root, budget=initial_budget, summary={},
                identities={helper: hashlib.sha256(helper.read_bytes()).hexdigest()})
            env = {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted",
                   "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1", "UAP_KIRO_BOUNDARY_CAPTURE": "1",
                   "UAP_KIRO_BOUNDARY_IDENTITY": str(root / "inputs/identity.json"), "UAP_KIRO_BOUNDARY_RUNTIME": str(root / "runtime")}
            system = {Path("/usr/bin") / leaf for leaf in ("sudo", "systemd-run", "systemctl", "env")}
            directories = {Path(p) for p in ("/run/systemd/system", "/sys/fs/cgroup", "/sys/fs/cgroup/system.slice")}
            old_stat = Path.lstat
            names = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_size", "st_mtime_ns", "st_ctime_ns", "st_uid")
            state = {"active": False, "system_changed": False, "commands": [], "transport_timeout": None}
            def info(path):
                if path in system or path in directories:
                    return SimpleNamespace(st_dev=1, st_ino=1, st_mode=(stat.S_IFDIR | 0o755) if path in directories else (stat.S_IFREG | 0o755),
                                           st_nlink=1, st_size=12, st_mtime_ns=1, st_ctime_ns=1, st_uid=0)
                original = old_stat(path)
                return SimpleNamespace(**{k: (1001 if k == "st_uid" else getattr(original, k)) for k in names})
            def system_bytes(path, *_args, **_kwargs):
                self.assertIn(path, system)
                return b"changed system" if state["system_changed"] else b"inert system"
            def command(label, argv, cwd, child_env, timeout=30):
                allowed = min(timeout, ctx.budget.check())
                self.assertGreater(allowed, 0)
                self.assertEqual(cwd, root)
                self.assertEqual(set(child_env), {"PATH", "LANG", "SYSTEMD_PAGER", "SYSTEMD_COLORS"})
                state["commands"].append((label, list(argv)))
                if label == "kiro-boundary-service":
                    state["active"] = True
                    state["transport_timeout"] = allowed
                    if mode.startswith("exhausted-transport"):
                        clock[0] += allowed
                        raise ValueError("inert transport timed out")
                    if mode == "input-replaced": helper.write_bytes(b"changed input")
                    elif mode == "input-unreadable": helper.unlink()
                    elif mode == "system-replaced": state["system_changed"] = True
                    else: state["active"] = False
                if label in ("kiro-unit-stop", "kiro-unit-kill"):
                    self.assertEqual(argv[-1], ctx.summary["delegated_service"]["unit"])
                    if label == "kiro-unit-kill" or mode != "exhausted-transport-kill":
                        state["active"] = False
                clock[0] += 0.1
                if label == "systemd-version": return {"exit": 0}, b"systemd 255\n", b""
                if label.startswith("kiro-unit-") and label not in ("kiro-unit-stop", "kiro-unit-kill"):
                    unit = argv[-1]
                    if label == "kiro-unit-before": body = b"LoadState=not-found\nActiveState=inactive\nControlGroup=\n"
                    else: body = ("LoadState=loaded\nActiveState=" + ("active" if state["active"] else "inactive") + "\nControlGroup=/system.slice/" + unit + "\n").encode()
                    return {"exit": 0}, body, b""
                return {"exit": 0}, b"inert bounded transport", b""
            ctx.command = command
            error = None
            with (patch.object(capture.shared.native, "require_hosted"), patch.object(capture.shared, "Budget", Budget),
                  patch.object(capture.time, "monotonic_ns", side_effect=lambda: int(clock[0] * 1e9)),
                  patch.object(capture, "ordinary_bytes", side_effect=system_bytes), patch.object(Path, "lstat", info),
                  patch.object(capture, "cgroup_empty", side_effect=lambda *_: not state["active"]),
                  patch.object(os, "getuid", return_value=1001), patch.object(os, "geteuid", return_value=1001),
                  patch.object(os, "getgid", return_value=1001), patch.object(os, "getegid", return_value=1001)):
                try: capture.service(ctx, helper, env)
                except Exception as caught: error = caught
            return state, ctx.summary, error, ctx.budget is initial_budget, clock[0] - 100

    def test_transport_timeout_preserves_original_budget_for_explicit_cleanup(self):
        for mode in ("exhausted-transport", "exhausted-transport-kill"):
            with self.subTest(mode=mode):
                state, summary, error, restored, elapsed = self.service_fixture(mode, seconds=25)
                self.assertIsNotNone(error)
                self.assertIn("kiro-unit-stop", [label for label, _ in state["commands"]])
                self.assertTrue(summary["delegated_service"]["forced_stop"])
                self.assertEqual(summary["delegated_service"]["forced_kill"], mode.endswith("-kill"))
                self.assertTrue(summary["delegated_service"]["closed"])
                self.assertTrue(restored)
                self.assertLess(elapsed, 25)

    def test_insufficient_cleanup_reserve_never_starts_the_unit(self):
        state, _summary, error, restored, elapsed = self.service_fixture("normal", seconds=20)
        self.assertIsNotNone(error)
        self.assertNotIn("kiro-boundary-service", [label for label, _ in state["commands"]])
        self.assertTrue(restored)
        self.assertLess(elapsed, 20)

    def test_input_custody_failure_cannot_suppress_exact_unit_cleanup(self):
        for mode in ("input-replaced", "input-unreadable"):
            with self.subTest(mode=mode):
                state, summary, error, restored, elapsed = self.service_fixture(mode)
                self.assertIsNotNone(error)
                self.assertIn("kiro-unit-stop", [label for label, _ in state["commands"]])
                self.assertTrue(summary["delegated_service"]["closed"])
                self.assertTrue(summary["delegated_service"]["forced_stop"])
                self.assertTrue(restored)
                self.assertLess(elapsed, 180)

    def test_cleanup_does_not_bypass_system_tool_custody_or_promote_forced_stop(self):
        state, summary, error, restored, _ = self.service_fixture("system-replaced")
        self.assertIsNotNone(error)
        self.assertNotIn("kiro-unit-stop", [label for label, _ in state["commands"]])
        self.assertFalse(summary["delegated_service"]["closed"])
        self.assertTrue(restored)
        state, summary, error, restored, elapsed = self.service_fixture("normal")
        self.assertIsNone(error)
        self.assertTrue(summary["delegated_service"]["closed"])
        self.assertFalse(summary["delegated_service"]["forced_stop"])
        self.assertTrue(restored)
        self.assertLess(elapsed, 180)


if __name__ == "__main__":
    unittest.main()
