"""Inert native envelope and exact selection regressions; no client execution."""
import io
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import project_native_runtime as runtime


class RuntimeTests(unittest.TestCase):
    source = {"source_commit": "a" * 40, "source_tree": "b" * 40}

    def result(self):
        return {"client": "copilot", "provider_kind": "scripted_loopback", "activation": "prepared",
                "runtime_attempt_complete": False, "vendor_authentication_qualified": False,
                "real_model_qualified": False, "test_failed": True,
                "qualification": {key: False for key in runtime.shared.QUALIFICATION_CASES}}

    def envelope(self, path, members, *, mode=stat.S_IFREG | 0o600):
        with zipfile.ZipFile(path, "w") as archive:
            for name, body in members:
                entry = zipfile.ZipInfo(name)
                entry.external_attr = mode << 16
                archive.writestr(entry, body)

    def test_failed_attempt_artifact_is_preserved_without_qualification(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "research.zip"
            self.envelope(path, [("runtime-attempt.json", json.dumps(self.result()).encode()),
                                 ("mcp.json.installed", b"{}"), ("probe-events.jsonl", b"")])
            bodies, result = runtime.admit_runtime(path, "copilot", self.source)
            self.assertTrue(result["test_failed"])
            self.assertFalse(any(result["qualification"].values()))
            self.assertIn("mcp.json.installed", bodies)

    def test_unknown_duplicate_special_non_utf8_and_large_members_are_refused(self):
        summary = ("runtime-attempt.json", json.dumps(self.result()).encode())
        cases = [([summary, ("profile.json", b"{}")], stat.S_IFREG | 0o600),
                 ([summary, summary], stat.S_IFREG | 0o600),
                 ([summary], stat.S_IFLNK | 0o600),
                 ([summary, ("probe-events.jsonl", b"\xff")], stat.S_IFREG | 0o600),
                 ([summary, ("probe-events.jsonl", b"x" * (runtime.MAX_MEMBER + 1))], stat.S_IFREG | 0o600)]
        for members, mode in cases:
            with self.subTest(mode=mode, names=[row[0] for row in members]), tempfile.TemporaryDirectory() as temporary:
                path = Path(temporary) / "research.zip"
                self.envelope(path, members, mode=mode)
                with self.assertRaises((ValueError, UnicodeError)):
                    runtime.admit_runtime(path, "copilot", self.source)

    def test_partial_command_and_provider_sequence_gaps_are_refused(self):
        summary = {"runtime-attempt.json": json.dumps(self.result()).encode()}
        for extra in ({"version.stdout.log": b""}, {"provider-request-2.json": b"{}"}):
            with self.assertRaises(ValueError):
                runtime.copilot_members({**summary, **extra}, self.source)

    def test_attempt_cannot_promote_qualification_or_claim_another_source(self):
        for change in ({"runtime_attempt_complete": True}, {"vendor_authentication_qualified": True},
                       {"source_identity": {"installer_base_commit": "c" * 40}},
                       {"qualification": {key: True for key in runtime.shared.QUALIFICATION_CASES}}):
            with self.assertRaises(ValueError):
                runtime.copilot_members({"runtime-attempt.json": json.dumps({**self.result(), **change}).encode()}, self.source)

    def test_selection_requires_actual_declared_test_and_leaf(self):
        with tempfile.TemporaryDirectory() as temporary, patch.object(runtime, "SOURCE", Path(temporary)):
            self.assertEqual(runtime.selected_cases(), {"include": []})
            leaf = runtime.SOURCE / "scripts/project_native_collectors/copilot.py"
            test = runtime.SOURCE / runtime.CASES["copilot"]["file"]
            leaf.parent.mkdir(parents=True)
            test.parent.mkdir(parents=True)
            leaf.write_text("inert")
            test.write_text("func Other(t *testing.T) {}")
            with self.assertRaises(ValueError):
                runtime.selected_cases()
            test.write_text("func TestAgentpluginsCopilotNativeProjectRuntimeAttempt(t *testing.T) {\n}")
            self.assertEqual(runtime.selected_cases(), {"include": [{"client": "copilot", "phase": "runtime-attempt"}]})

    def test_host_guard_runs_before_creating_output_or_native_child(self):
        with tempfile.TemporaryDirectory() as temporary, patch.dict(os.environ, {"GITHUB_ACTIONS": "false"}):
            path = Path(temporary) / "must-not-exist"
            with self.assertRaises(RuntimeError):
                runtime.run("copilot", path)
            self.assertFalse(path.exists())

    def test_json_rejects_nonfinite_numbers_and_duplicate_keys(self):
        for body in (b'{"x":1e999}', b'{"x":-1e999}', b'{"x":NaN}', b'{"x":1,"x":2}'):
            with self.assertRaises(ValueError):
                runtime.strict_json(body)


if __name__ == "__main__":
    unittest.main()
