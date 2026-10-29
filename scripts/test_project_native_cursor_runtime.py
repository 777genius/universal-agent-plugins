"""Inert first Cursor component admission; no vendor process is executed."""
import json
import unittest

import project_native_runtime as runtime


class CursorRuntimeTests(unittest.TestCase):
    source = {"source_commit": "a" * 40, "source_tree": "b" * 40}

    def result(self):
        return {"client": "cursor", "version": "2026.10.22-27dcf5f", "activation": "prepared",
                "runtime_attempt_complete": False, "test_failed": True,
                "vendor_authentication_qualified": False, "real_model_qualified": False,
                "qualification": {key: False for key in runtime.shared.QUALIFICATION_CASES},
                "source_identity": {"installer_base_commit": "a" * 40, "installer_tree": "b" * 40,
                                    "installer_source_state": "committed", "installer_patch_sha256": []}}

    def test_first_capture_registers_exact_cursor_test_and_only_cursor_labels(self):
        self.assertEqual(runtime.CASES["cursor"], {
            "test": "TestAgentpluginsCursorNativeProjectRuntimeAttempt",
            "file": "repotests/agentplugins_cursor_project_native_attempt_test.go",
            "optin": "AGENTPLUGINS_CURSOR_PROJECT_RUNTIME_ATTEMPT", "seconds": 180, "phase": "runtime-attempt"})
        labels = ("empty-a-mcp", "empty-b-mcp", "empty-outside-mcp", "control-b-mcp", "control-outside-mcp")
        row = {"argv": ["/pin/cursor-agent", "mcp", "list"], "cwd": "/owned/A", "exit": 0,
               "error": False, "deadline": False, "elapsed_ms": 1,
               "cleanup": "full descendant supervision; forced cleanup fails"}
        for label in labels:
            bodies = {"runtime-attempt.json": json.dumps(self.result()).encode(),
                      label + ".command.json": json.dumps(row).encode(),
                      label + ".stdout.log": b"opaque native capture\n", label + ".stderr.log": b""}
            self.assertTrue(runtime.cli_attempt_members(bodies, self.source, "cursor")["test_failed"])
            for client in ("grok", "qoder", "kiro", "windsurf"):
                other = {**self.result(), "client": client, "version": runtime.CLI_ATTEMPTS[client]}
                with self.assertRaises(ValueError):
                    runtime.cli_attempt_members({**bodies, "runtime-attempt.json": json.dumps(other).encode()}, self.source, client)
            with self.assertRaises(ValueError):
                runtime.cli_attempt_members({k: v for k, v in bodies.items() if k != label + ".stderr.log"}, self.source, "cursor")

    def test_native_text_cannot_qualify_initial_capture(self):
        for change in ({"test_failed": False}, {"runtime_attempt_complete": True},
                       {"vendor_authentication_qualified": True}, {"real_model_qualified": True},
                       {"version": "latest"}, {"source_identity": {}},
                       {"qualification": {key: True for key in runtime.shared.QUALIFICATION_CASES}}):
            with self.assertRaises(ValueError):
                runtime.cli_attempt_members({"runtime-attempt.json": json.dumps({**self.result(), **change}).encode()}, self.source, "cursor")


if __name__ == "__main__":
    unittest.main()
