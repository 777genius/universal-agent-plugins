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
                runtime.attempt_members({**summary, **extra}, self.source)

    def test_attempt_cannot_promote_qualification_or_claim_another_source(self):
        for change in ({"runtime_attempt_complete": True}, {"vendor_authentication_qualified": True},
                       {"source_identity": {"installer_base_commit": "c" * 40}},
                       {"qualification": {key: True for key in runtime.shared.QUALIFICATION_CASES}}):
            with self.assertRaises(ValueError):
                runtime.attempt_members({"runtime-attempt.json": json.dumps({**self.result(), **change}).encode()}, self.source)

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

    def test_gemini_requires_its_own_exact_snapshot_names(self):
        result = {**self.result(), "client": "gemini", "runtime_attempt_complete": False}
        bodies = {"runtime-attempt.json": json.dumps(result).encode(), ".gemini-settings.json.installed": b"{}"}
        self.assertEqual(runtime.attempt_members(bodies, self.source, "gemini")["client"], "gemini")
        with self.assertRaises(ValueError):
            runtime.attempt_members(bodies, self.source, "copilot")
        result.update({"runtime_attempt_complete": True, "test_failed": False})
        bodies["runtime-attempt.json"] = json.dumps(result).encode()
        with self.assertRaisesRegex(ValueError, "missing required"):
            runtime.attempt_members(bodies, self.source, "gemini")

    def test_kimi_failed_lifecycle_keeps_no_unobserved_success_or_tool_call(self):
        result = {key: value for key, value in self.result().items() if key not in ("provider_kind", "runtime_attempt_complete")}
        result["client"] = "kimi"
        self.assertTrue(runtime.kimi_members({"lifecycle.json": json.dumps(result).encode()}, self.source)["test_failed"])
        result["qualification"]["mcp-tools-call"] = True
        with self.assertRaises(ValueError):
            runtime.kimi_members({"lifecycle.json": json.dumps(result).encode()}, self.source)

    def cli_result(self, client):
        result = {key: value for key, value in self.result().items() if key != "provider_kind"}
        result.update({"client": client, "version": runtime.CLI_ATTEMPTS[client],
                       "source_identity": {"installer_base_commit": self.source["source_commit"],
                                           "installer_tree": self.source["source_tree"],
                                           "installer_source_state": "committed", "installer_patch_sha256": []}})
        return result

    def test_fixed_cli_attempts_cannot_turn_captured_output_into_success(self):
        for client in runtime.CLI_ATTEMPTS:
            with self.subTest(client=client):
                result = self.cli_result(client)
                self.assertTrue(runtime.cli_attempt_members({"runtime-attempt.json": json.dumps(result).encode()}, self.source, client)["test_failed"])
                for change in ({"test_failed": False}, {"runtime_attempt_complete": True}, {"version": "latest"},
                               {"source_identity": {}}, {"qualification": {key: 0 for key in runtime.shared.QUALIFICATION_CASES}}):
                    with self.assertRaises(ValueError):
                        runtime.cli_attempt_members({"runtime-attempt.json": json.dumps({**result, **change}).encode()}, self.source, client)

    def test_cli_member_table_rejects_partial_commands_provider_and_profiles(self):
        summary = {"runtime-attempt.json": json.dumps(self.cli_result("qoder")).encode()}
        for extra in ({"git-a.stdout.log": b""}, {"provider-request-1.json": b"{}"}, {"profile.json": b"{}"}):
            with self.assertRaises(ValueError):
                runtime.cli_attempt_members({**summary, **extra}, self.source, "qoder")
        command = {"argv": ["/owned/qodercli", "--version"], "cwd": "/owned/outside", "exit": 0,
                   "error": False, "deadline": False, "elapsed_ms": 1,
                   "cleanup": "full descendant supervision; forced cleanup fails"}
        bodies = {**summary, "version.stdout.log": b"1.1.78\n", "version.stderr.log": b"",
                  "version.command.json": json.dumps(command).encode()}
        runtime.cli_attempt_members(bodies, self.source, "qoder")
        for field, value in (("deadline", 0), ("elapsed_ms", -1), ("cleanup", "assumed"), ("extra", True)):
            with self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, "version.command.json": json.dumps({**command, field: value}).encode()}, self.source, "qoder")

    def cline_result(self):
        return {**self.cli_result("qoder"), "client": "cline", "version": "3.0.73"}

    def test_catalog_triple_is_admitted_only_for_qoder_and_never_qualifies_it(self):
        command = {"argv": ["/owned/qodercli", "plugins", "list", "--json", "--plugin-dir", "/owned/A/.agentplugins-qoder-native"],
                   "cwd": "/owned/A", "exit": 0, "error": False, "deadline": False, "elapsed_ms": 1,
                   "cleanup": "full descendant supervision; forced cleanup fails"}
        triple = {"native-catalog.command.json": json.dumps(command).encode(), "native-catalog.stdout.log": b"[]\n",
                  "native-catalog.stderr.log": b""}
        for client in runtime.CLI_ATTEMPTS:
            summary = {"runtime-attempt.json": json.dumps(self.cli_result(client)).encode()}
            if client == "qoder":
                result = runtime.cli_attempt_members({**summary, **triple}, self.source, client)
                self.assertTrue(result["test_failed"])
                self.assertFalse(any(result["qualification"].values()))
                for missing in triple:
                    with self.assertRaises(ValueError):
                        runtime.cli_attempt_members({**summary, **{key: value for key, value in triple.items() if key != missing}}, self.source, client)
            else:
                with self.assertRaises(ValueError):
                    runtime.cli_attempt_members({**summary, **triple}, self.source, client)

    def test_inspect_triple_is_admitted_only_for_grok_without_qualification(self):
        command = {"argv": ["/owned/grok", "inspect", "--json"], "cwd": "/owned/A", "exit": 0,
                   "error": False, "deadline": False, "elapsed_ms": 1,
                   "cleanup": "full descendant supervision; forced cleanup fails"}
        triple = {"native-inspect.command.json": json.dumps(command).encode(),
                  "native-inspect.stdout.log": b'{"grokVersion":"1.0.70"}\n',
                  "native-inspect.stderr.log": b""}
        for client in runtime.CLI_ATTEMPTS:
            summary = {"runtime-attempt.json": json.dumps(self.cli_result(client)).encode()}
            if client != "grok":
                with self.subTest(client=client), self.assertRaises(ValueError):
                    runtime.cli_attempt_members({**summary, **triple}, self.source, client)
                continue
            with tempfile.TemporaryDirectory() as temporary:
                path = Path(temporary) / "research.zip"
                self.envelope(path, list({**summary, **triple}.items()))
                bodies, result = runtime.admit_runtime(path, client, self.source)
            self.assertEqual(set(bodies), set(summary) | set(triple))
            self.assertTrue(result["test_failed"])
            self.assertFalse(result["runtime_attempt_complete"])
            self.assertFalse(any(result["qualification"].values()))
            for missing in triple:
                with self.subTest(missing=missing), self.assertRaises(ValueError):
                    runtime.cli_attempt_members({**summary, **{key: value for key, value in triple.items() if key != missing}}, self.source, client)
            with self.assertRaises(ValueError):
                runtime.cli_attempt_members({**summary, **triple, "native-inspect.extra.log": b""}, self.source, client)

    def test_cline_capture_is_unqualified_and_provider_key_is_never_admitted(self):
        result = self.cline_result()
        base = {"runtime-attempt.json": json.dumps(result).encode()}
        self.assertTrue(runtime.cline_members(base, self.source)["test_failed"])
        for change in ({"test_failed": False}, {"runtime_attempt_complete": True}, {"version": "3.0.74"},
                       {"source_identity": {}}, {"qualification": {key: 0 for key in runtime.shared.QUALIFICATION_CASES}}):
            with self.assertRaises(ValueError):
                runtime.cline_members({"runtime-attempt.json": json.dumps({**result, **change}).encode()}, self.source)
        for extra in ({"profile.json": b"{}"}, {"provider-3.request.json": b"{}"}, {"version.stdout.log": b""},
                      {"native-acp.jsonl": b""}, {"native-acp.error.log": b"failure"},
                      {"probe-events.jsonl": b"uap-cline-private-0123456789abcdef"}):
            with self.assertRaises(ValueError):
                runtime.cline_members({**base, **extra}, self.source)

    def test_cline_provider_captures_require_actual_contiguous_request_receipts(self):
        base = {"runtime-attempt.json": json.dumps(self.cline_result()).encode()}
        provider = {"requests": 1, "responses": 0, "exact_call_issued": False, "single_call_approved": False,
                    "error": True, "transport": "loopback scripted OpenAI chat-completions SSE; no real model"}
        body = {**base, "provider.json": json.dumps(provider).encode(), "provider-1.request.json": b"{}"}
        runtime.cline_members(body, self.source)
        for extra in ({"provider-2.response.log": b"data"}, {"provider-2.request.json": b"{}"},
                      {"provider.json": json.dumps({**provider, "requests": True}).encode()}):
            with self.assertRaises(ValueError):
                runtime.cline_members({**body, **extra}, self.source)

    def test_cline_unit_receipts_cannot_select_foreign_or_inconsistent_control_groups(self):
        unit = "uap-cline-project-" + "a" * 32 + ".service"
        absent = b"LoadState=not-found\nActiveState=inactive\nControlGroup=\n"
        self.assertEqual(runtime.cline_unit_state(absent, unit)["LoadState"], "not-found")
        for body in (absent + b"LoadState=loaded\n", absent.replace(b"inactive", b"active"),
                     b"LoadState=loaded\nActiveState=active\nControlGroup=/system.slice/ssh.service\n",
                     absent + b"MainPID=1\n"):
            with self.assertRaises(ValueError):
                runtime.cline_unit_state(body, unit)
        for name in ("ssh.service", "uap-cline-project-*.service", "../" + unit):
            with self.assertRaises(ValueError):
                runtime.cline_unit_state(absent, name)

    def test_cline_service_is_exact_nonroot_delegation_with_original_clock_and_empty_environment(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            test = root / "project-native.test"
            test.write_bytes(b"inert never executed")
            env = {key: "fixture" for key in runtime.CLINE_ENV}
            env.update({"PATH": "/usr/bin:/bin", "AGENTPLUGINS_CLINE_PROJECT_RUNTIME_ATTEMPT": "1", "HOME": "/fixture/${LITERAL}"})
            unit = "uap-cline-project-" + "b" * 32 + ".service"
            origin = 123456789012345
            argv = runtime.cline_service_argv(test, env, root, unit, 1001, 1001, origin)
            self.assertEqual(argv[:3], ["/usr/bin/sudo", "-n", "/usr/bin/systemd-run"])
            for argument in ("--uid=1001", "--gid=1001", "--property=Delegate=yes", "--property=NoNewPrivileges=yes",
                             "--property=KillMode=control-group", "--property=RuntimeMaxSec=180s",
                             "--expand-environment=no", "HOME=/fixture/${LITERAL}",
                             "AGENTPLUGINS_PROJECT_NATIVE_STARTED_NS=" + str(origin), "AGENTPLUGINS_CLINE_SYSTEMD_UNIT=" + unit):
                self.assertIn(argument, argv)
            index = argv.index("/usr/bin/env")
            self.assertEqual(argv[index + 1], "-i")
            self.assertEqual(argv[-5:], [str(test), "-test.run=^TestAgentpluginsClineNativeProjectRuntimeAttempt$", "-test.count=1", "-test.v", "-test.timeout=180s"])
            for selected, selected_unit, uid, stamp in (({**env, "LD_PRELOAD": "unexpected"}, unit, 1001, origin),
                    ({**env, "HOME": "first\nsecond"}, unit, 1001, origin), (env, "ssh.service", 1001, origin),
                    (env, unit, 0, origin), (env, unit, 1001, 0)):
                with self.assertRaises(ValueError):
                    runtime.cline_service_argv(test, selected, root, selected_unit, uid, 1001, stamp)

    def test_kimi_single_call_is_selected_separately_without_replacing_existing_lifecycle(self):
        with tempfile.TemporaryDirectory() as temporary, patch.object(runtime, "SOURCE", Path(temporary)):
            leaf = runtime.SOURCE / "scripts/project_native_collectors/kimi.py"
            leaf.parent.mkdir(parents=True)
            leaf.write_text("inert acquisition metadata")
            tests = runtime.SOURCE / "repotests"
            tests.mkdir()
            for selector in ("kimi", "kimi-call"):
                case = runtime.CASES[selector]
                (runtime.SOURCE / case["file"]).write_text("func " + case["test"] + "(t *testing.T) {\n}")
            self.assertEqual(runtime.selected_cases()["include"], [{"client": "kimi", "phase": "installed-components-lifecycle"}])
            (leaf.parent / "kimi_attempt.py").write_text("inert installed-tool observer")
            self.assertEqual(runtime.selected_cases()["include"], [{"client": "kimi", "phase": "installed-components-lifecycle"},
                                {"client": "kimi-call", "phase": "installed-single-tool-attempt"}])
            self.assertEqual(runtime.CASES["kimi"]["seconds"], 900)
            self.assertEqual(runtime.CASES["kimi-call"]["seconds"], 180)
            self.assertNotEqual(runtime.CASES["kimi"]["optin"], runtime.CASES["kimi-call"]["optin"])

    def test_kimi_single_call_cannot_inherit_lifecycle_success_or_unknown_members(self):
        result = {**self.cli_result("qoder"), "client": "kimi", "version": "2.3.2", "phase": "installed-single-tool-attempt",
                  "provider_kind": "scripted-openai-chat-completions", "elapsed_ms": 100}
        base = {"runtime-attempt.json": json.dumps(result).encode()}
        self.assertTrue(runtime.kimi_call_members(base, self.source)["test_failed"])
        for change in ({"phase": "installed-components-lifecycle"}, {"client": "kimi-call"}, {"test_failed": False},
                       {"runtime_attempt_complete": True}, {"source_identity": {}}, {"elapsed_ms": True},
                       {"qualification": {key: True for key in runtime.shared.QUALIFICATION_CASES}}):
            with self.assertRaises(ValueError):
                runtime.kimi_call_members({"runtime-attempt.json": json.dumps({**result, **change}).encode()}, self.source)
        for extra in ({"lifecycle.json": b"{}"}, {"profile.json": b"{}"}, {"provider-request-2.json": b"{}"},
                      {"provider-request-9.json": b"{}"}, {"git-c.stdout.log": b""},
                      {"python-api.json": b'{"token":"uap-kimi-key-inert-canary"}'},
                      {"provider-request-1.json": b'{"content":"uap-kimi-key-inert-canary"}'}):
            with self.assertRaises(ValueError):
                runtime.kimi_call_members({**base, **extra}, self.source)
        command = {"argv": ["/owned/git", "init", "--quiet"], "cwd": "/owned/C", "exit": 0, "error": False,
                   "deadline": False, "cleanup": "ordinary full-descendant supervision; forced termination fails"}
        bodies = {**base, "git-c.command.json": json.dumps(command).encode(), "git-c.stdout.log": b"", "git-c.stderr.log": b""}
        runtime.kimi_call_members(bodies, self.source)
        with self.assertRaises(ValueError):
            runtime.kimi_call_members({**bodies, "git-c.command.json": json.dumps({**command, "cleanup": "assumed"}).encode()}, self.source)


if __name__ == "__main__":
    unittest.main()
