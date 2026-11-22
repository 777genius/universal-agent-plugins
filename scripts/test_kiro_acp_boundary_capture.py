"""Inert pin, service and receipt admission checks; no native/privileged child."""
import base64
import hashlib
import json
from pathlib import Path
import tempfile
import unittest

import kiro_acp_boundary_capture as capture


class KiroACPBoundaryContracts(unittest.TestCase):
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

    def test_receipt_cannot_promote_diagnostic_or_claim_complete_native_stream(self):
        identity = {"source": {"commit": "a" * 40, "tree": "b" * 40}, "binary": "/inert/kiro-cli"}
        service = {"unit": "uap-kiro-acp-boundary-" + "c" * 32 + ".service", "original_start_monotonic_ns": 123,
                   "closed": True, "forced_stop": False, "forced_kill": False, "elapsed_ns": 1000, "uid": 1001, "gid": 1001}
        report = {"diagnostic_only": True, "qualification": dict(capture.QUALIFICATION), "identity": identity,
                  "stderr_scope": "unavailable-from-duplex-api", "stdout_scope": "read-prefix-through-exchange-stop",
                  "stdout_complete": False, "delegation": {"unit": service["unit"], "origin_ns": 123, "uid": 1001, "gid": 1001,
                      "membership": "0::/system.slice/" + service["unit"] + "\n", "no_new_privileges": True}, "capture_complete": True,
                  "observed_records": 1,
                  "duplex_capability": True, "planned_shutdown_verified": True, "stop_reason": "initialize-server-error",
                  "argv": [identity["binary"], "acp", "--agent-engine", "v3", "--auth-method", "cli"]}
        for name, body in (("stdin", b"initialize\n"), ("stdout", b"unclassified boundary\n")):
            report[name] = base64.b64encode(body).decode()
            report[name + "_sha256"] = hashlib.sha256(body).hexdigest()
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


if __name__ == "__main__":
    unittest.main()
