"""Unit negative controls for Codex native secrecy evidence, no network/vendor client."""
import http.client
import hashlib
import base64
import json
import os
import subprocess
import sys
from pathlib import Path
import tempfile
import threading
import unittest
from types import SimpleNamespace
from unittest.mock import patch
import http.server

from codex_env_names_appserver_spike import terminal_turn_status, classify_native_stdio_line
from codex_env_names_native_spike import (_scan_stat_identity, _windows_path_within_root, _capture_scan_root, _read_regular_bounded, stop_live_observer, synthetic_canary_patterns, contains_synthetic_canary, verified_vendor_version, PINNED_VENDOR_VERSION, CHILD, Model, MCP_CALL, SHELL_CALL, SHELL_MARKER,
                                        SHELL_ENV_PRESENT, SHELL_FOREIGN_PRESENT,
                                        ENV_NAME, FOREIGN_ENV_NAME, inspect, aggregate_findings, windows_shell_command, isolated_env, isolated_root, structured_mcp_receipt_matches, fixed_windows_spawn_error_category, WINDOWS_NATIVE_SANDBOX_CONFIG)


class TestSyntheticModelOracle(unittest.TestCase):
    def setUp(self):
        Model.shell_only = False
        Model.requests = 0
        Model.shell_requested = False
        Model.mcp_requested = False
        Model.search_requested = False
        Model.mcp_output_seen = False
        Model.mcp_digest_verified = False
        Model.mcp_output_indicators = []
        Model.mcp_output_type = "not_observed"
        Model.shell_output_seen = False
        Model.shell_execution_verified = False
        Model.shell_secret_visible = False
        Model.shell_foreign_visible = False
        Model.shell_response_category = "not_observed"
        Model.shell_response_indicators = []
        Model.shell_win32_failure_category = "not_reported"
        Model.shell_requested_at = 0.0
        Model.error = ""
        Model.expected_digest = "UNITTEST_SYNTHETIC_SHA_ONLY"
        Model.advertised_tool_names = set()
        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Model)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=3)

    def call(self, data, status=200):
        client = http.client.HTTPConnection("127.0.0.1", self.server.server_port, timeout=5)
        try:
            client.request("POST", "/v1/responses", json.dumps(data),
                           {"Content-Type": "application/json"})
            response = client.getresponse()
            content = response.read().decode()
            self.assertEqual(response.status, status)
            if status != 200:
                return None
            events = [json.loads(line[6:]) for line in content.splitlines()
                      if line.startswith("data: ")]
            self.assertEqual(len(events), 3)
            return events[1]["item"]
        finally:
            client.close()

    def test_vendor_tool_names_are_bounded_before_artifact_reporting(self):
        # A provider could put a synthetic value or arbitrary data in a tool
        # name; this must never reach redacted proof artifacts.
        untrusted = "PRIVATE_SYNTHETIC_CANARY_NEVER_STORE"
        shell = self.call({"tools": [
            {"type": "function", "name": "exec_command"},
            {"type": "function", "name": untrusted},
            {"type": "function", "name": ["not-a-string"]},
        ]})
        self.assertEqual(shell["name"], "exec_command")
        self.assertIn("exec_command", Model.advertised_tool_names)
        self.assertNotIn(untrusted, Model.advertised_tool_names)
        self.assertNotIn(untrusted, json.dumps(sorted(Model.advertised_tool_names)))

    def test_real_call_ids_and_matching_digest_must_be_observed(self):
        shell = self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        self.assertEqual(shell["call_id"], SHELL_CALL)
        self.assertEqual(shell["name"], "exec_command")
        # A native async session result is not a completed shell invocation.
        self.assertEqual(json.loads(shell["arguments"])["yield_time_ms"], 10000)
        self.assertIn(ENV_NAME, json.loads(shell["arguments"])["cmd"])
        self.assertIn(FOREIGN_ENV_NAME, json.loads(shell["arguments"])["cmd"])
        tool = self.call({
            "tools": [{"type": "namespace", "name": "mcp__env_spike",
                       "tools": [{"name": "confirm_env"}]}],
            "input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                       "output": "bounded shell done"}],
        })
        self.assertEqual(tool["call_id"], MCP_CALL)
        self.assertEqual(tool["namespace"], "mcp__env_spike")
        self.assertFalse(Model.mcp_digest_verified)
        complete = self.call({
            "input": [{"type": "function_call_output", "call_id": MCP_CALL,
                       "output": {"content": [{"type": "text",
                                   "text": '{"sha256":"UNITTEST_SYNTHETIC_SHA_ONLY","observed":true}'}]}}],
        })
        self.assertEqual(complete["type"], "message")
        self.assertTrue(Model.mcp_digest_verified)
        self.assertTrue(Model.mcp_output_seen)

    def test_shell_only_mode_finishes_without_mcp_but_does_not_prove_shell(self):
        Model.shell_only = True
        shell = self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        self.assertEqual(shell["call_id"], SHELL_CALL)
        final = self.call({"input": [{"type": "function_call_output",
                                      "call_id": SHELL_CALL,
                                      "output": "native tool refused"}]})
        self.assertEqual(final["type"], "message")
        self.assertFalse(Model.shell_execution_verified)
        self.assertFalse(Model.mcp_requested)

    def test_shell_only_probe_does_not_mention_any_credential_name(self):
        Model.shell_only = True
        shell = self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        cmd = json.loads(shell["arguments"])["cmd"]
        self.assertIn("7", cmd)
        self.assertNotIn(ENV_NAME, cmd)
        self.assertNotIn(FOREIGN_ENV_NAME, cmd)
        self.assertNotIn("PASSWORD", cmd.upper())
        self.assertNotIn("TOKEN", cmd.upper())
        self.assertNotIn(SHELL_MARKER, cmd)
        self.assertNotIn(SHELL_ENV_PRESENT, cmd)
        self.assertNotIn(SHELL_FOREIGN_PRESENT, cmd)
        self.assertIn("Write-Output" if os.name == "nt" else "printf", cmd)

    def test_typed_vendor_failure_diagnostics_never_log_raw_message(self):
        Model.shell_only = True
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        self.call({"input": [{"type": "function_call_output",
                             "call_id": SHELL_CALL,
                             "output": "exec_command failed: CreateProcess: SYNTHETIC_PRIVATE_PATH"}]})
        self.assertIn("createprocess", Model.shell_response_indicators)
        self.assertFalse(Model.shell_execution_verified)
        self.assertNotIn("SYNTHETIC_PRIVATE_PATH", str(Model.shell_response_indicators))

    def test_windows_spawn_os_error_category_is_bounded_and_redacted(self):
        text = ('exec_command failed: CreateProcess { message: '
                'The system cannot find the file specified. (os error 2) '
                'C:\\\\synthetic-private-path\\\\never-log }')
        self.assertEqual(fixed_windows_spawn_error_category(text),
                         "file_not_found")
        self.assertNotIn("synthetic-private-path",
                         fixed_windows_spawn_error_category(text))

    def test_windows_spawn_unlabelled_numbers_are_not_vendor_evidence(self):
        self.assertEqual(
            fixed_windows_spawn_error_category(
                "exec_command failed: CreateProcess at synthetic path 267"),
            "not_reported")
        self.assertEqual(
            fixed_windows_spawn_error_category(
                "exec_command failed: CreateProcess: (os error 99999)"),
            "other_win32_error")

    def test_windows_spawn_os_error_is_reported_without_raw_output(self):
        Model.shell_only = True
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        self.call({"input": [{"type": "function_call_output",
                             "call_id": SHELL_CALL,
                             "output": ("exec_command failed: CreateProcess "
                                        "os error 5 at SYNTHETIC_PRIVATE_PATH")}]})
        self.assertEqual(Model.shell_win32_failure_category, "access_denied")
        self.assertFalse(Model.shell_execution_verified)
        self.assertNotIn("SYNTHETIC_PRIVATE_PATH",
                         str((Model.shell_response_indicators,
                              Model.shell_win32_failure_category)))

    def test_native_windows_ci_fixture_requires_restricted_token_backend(self):
        # Only a disposable research profile is modified, never the user's.
        self.assertEqual(WINDOWS_NATIVE_SANDBOX_CONFIG,
                         '[windows]\nsandbox = "unelevated"\n')
        self.assertNotIn("danger-full-access", WINDOWS_NATIVE_SANDBOX_CONFIG)

    def test_windows_shell_command_only_emits_fixed_markers(self):
        command = windows_shell_command()
        self.assertIn("Start-Sleep -Seconds 7", command)
        self.assertIn("Test-Path Env:" + ENV_NAME, command)
        self.assertIn("Test-Path Env:" + FOREIGN_ENV_NAME, command)
        self.assertNotIn(SHELL_ENV_PRESENT, command)
        self.assertNotIn(SHELL_FOREIGN_PRESENT, command)
        self.assertNotIn(SHELL_MARKER, command)
        self.assertIn("Write-Output", command)
        self.assertNotIn("not_a_real_secret_fixture_123", command)

    def test_isolated_env_contains_no_runner_auth(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, home = Path(tmp), Path(tmp) / "home"
            env = isolated_env(root, home, "synthetic_only", "other_synthetic")
            self.assertEqual(env[ENV_NAME], "synthetic_only")
            self.assertEqual(env[FOREIGN_ENV_NAME], "other_synthetic")
            self.assertEqual(env["CODEX_HOME"], str(home / ".codex"))
            self.assertNotIn("GITHUB_TOKEN", env)
            self.assertNotIn("GH_TOKEN", env)

    def test_windows_core_fields_do_not_import_arbitrary_runner_env(self):
        if os.name != "nt":
            self.skipTest("Windows-only core environment contract")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with patch.dict(os.environ, {
                "SYSTEMDRIVE": "Z:",
                "PROGRAMDATA": "ProgramDataUnitFixture",
                "UAP_FORBIDDEN_RUNNER_SECRET": "DO_NOT_IMPORT",
            }):
                env = isolated_env(root, root / "home", "synthetic", "foreign")
            self.assertEqual(env.get("SYSTEMDRIVE"), "Z:")
            self.assertEqual(env.get("PROGRAMDATA"), "ProgramDataUnitFixture")
            self.assertNotIn("UAP_FORBIDDEN_RUNNER_SECRET", env)
            self.assertNotIn("GH_TOKEN", env)
            self.assertNotIn("GITHUB_TOKEN", env)

    def test_wrong_digest_is_never_treated_as_receipt(self):
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        self.call({"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                             "output": "shell complete"}],
                   "tools": [{"type": "namespace", "name": "mcp__env_spike",
                              "tools": [{"name": "confirm_env"}]}]})
        self.call({"input": [{"type": "function_call_output", "call_id": MCP_CALL,
                              "output": {"content": [{"type": "text",
                                        "text": '{"observed":false,"sha256":""}'}]}}]})
        self.assertTrue(Model.mcp_output_seen)
        self.assertFalse(Model.mcp_digest_verified)
        self.assertIn("observed", Model.mcp_output_indicators)
        self.assertEqual(Model.mcp_output_type, "object")

    def test_delayed_command_echo_without_error_word_is_not_execution(self):
        # A tool may return a delayed diagnostic echoing all command arguments
        # without any of the error words; no oracle marker may appear in argv.
        shell = self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        command = json.loads(shell["arguments"])["cmd"]
        self.assertNotIn(SHELL_MARKER, command)
        self.assertNotIn(SHELL_ENV_PRESENT, command)
        self.assertNotIn(SHELL_FOREIGN_PRESENT, command)
        Model.shell_requested_at -= 7.0
        self.call({"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                             "output": "rejected command payload: " + command}],
                   "tools": [{"type": "namespace", "name": "mcp__env_spike",
                              "tools": [{"name": "confirm_env"}]}]})
        self.assertTrue(Model.shell_output_seen)
        self.assertFalse(Model.shell_execution_verified)
        self.assertFalse(Model.shell_secret_visible)
        self.assertFalse(Model.shell_foreign_visible)

    def test_unix_runtime_marker_executes_without_command_literal(self):
        if os.name == "nt":
            self.skipTest("POSIX shell not available")
        from codex_env_names_native_spike import _sh_print_marker
        command = _sh_print_marker(SHELL_MARKER)
        self.assertNotIn(SHELL_MARKER, command)
        proc = subprocess.run(["sh", "-c", command], capture_output=True,
                              text=True, timeout=5)
        self.assertEqual(proc.returncode, 0)
        self.assertEqual(proc.stdout, SHELL_MARKER)

    def test_shell_error_response_does_not_prove_execution(self):
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        self.call({"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                              "output": "native shell tool denied"}],
                   "tools": [{"type": "namespace", "name": "mcp__env_spike",
                              "tools": [{"name": "confirm_env"}]}]})
        self.assertTrue(Model.shell_output_seen)
        self.assertFalse(Model.shell_execution_verified)
        self.assertIn(Model.shell_response_category, ("tool_error", "completion_unverified", "sandbox_error", "permission_error"))

    def test_shell_marker_only_counts_after_bounded_execution(self):
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        payload = {"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                              "output": SHELL_MARKER}],
                   "tools": [{"type": "namespace", "name": "mcp__env_spike",
                              "tools": [{"name": "confirm_env"}]}]}
        self.call(payload)
        self.assertFalse(Model.shell_execution_verified)
        Model.shell_requested_at -= 7.0
        self.call(payload)
        self.assertTrue(Model.shell_execution_verified)

    def test_failed_shell_echo_of_marker_does_not_claim_environment_exposure(self):
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        echoed_command = ("failed exec_command: cmd contains " + SHELL_ENV_PRESENT
                          + " " + SHELL_FOREIGN_PRESENT + " " + SHELL_MARKER)
        self.call({"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                             "output": echoed_command}],
                   "tools": [{"type": "namespace", "name": "mcp__env_spike",
                              "tools": [{"name": "confirm_env"}]}]})
        self.assertFalse(Model.shell_execution_verified)
        self.assertFalse(Model.shell_secret_visible)
        self.assertFalse(Model.shell_foreign_visible)
        self.assertEqual(Model.shell_response_category, "tool_error")

    def test_delayed_failed_shell_echo_is_not_execution_or_exposure(self):
        # Waiting long enough is NOT proof when Codex eventually rejects the
        # shell tool and repeats its arguments (including every fixed marker).
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        Model.shell_requested_at -= 7.0
        echoed_command = ("failed exec_command: cmd contains " + SHELL_ENV_PRESENT
                          + " " + SHELL_FOREIGN_PRESENT + " " + SHELL_MARKER)
        self.call({"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                             "output": echoed_command}],
                   "tools": [{"type": "namespace", "name": "mcp__env_spike",
                              "tools": [{"name": "confirm_env"}]}]})
        self.assertTrue(Model.shell_output_seen)
        self.assertFalse(Model.shell_execution_verified)
        self.assertFalse(Model.shell_secret_visible)
        self.assertFalse(Model.shell_foreign_visible)
        self.assertEqual(Model.shell_response_category, "tool_error")

    def test_shell_credential_presence_is_recorded_without_secret_value(self):
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        Model.shell_requested_at -= 7.0
        self.call({"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                             "output": SHELL_ENV_PRESENT + SHELL_MARKER}],
                   "tools": [{"type": "namespace", "name": "mcp__env_spike",
                              "tools": [{"name": "confirm_env"}]}]})
        self.assertTrue(Model.shell_execution_verified)
        self.assertTrue(Model.shell_secret_visible)

    def test_shell_without_credential_is_not_marked_exposed(self):
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        Model.shell_requested_at -= 7.0
        self.call({"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                             "output": SHELL_MARKER}],
                   "tools": [{"type": "namespace", "name": "mcp__env_spike",
                              "tools": [{"name": "confirm_env"}]}]})
        self.assertTrue(Model.shell_execution_verified)
        self.assertFalse(Model.shell_secret_visible)
        self.assertFalse(Model.shell_foreign_visible)

    def test_legacy_global_exclusion_is_visible_to_the_oracle(self):
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        Model.shell_requested_at -= 7.0
        self.call({"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                             "output": SHELL_FOREIGN_PRESENT + SHELL_MARKER}],
                   "tools": [{"type": "namespace", "name": "mcp__env_spike",
                              "tools": [{"name": "confirm_env"}]}]})
        self.assertTrue(Model.shell_execution_verified)
        self.assertFalse(Model.shell_secret_visible)
        self.assertTrue(Model.shell_foreign_visible)

    def test_missing_mcp_advertisement_refuses(self):
        self.call({"tools": [{"type": "function", "name": "exec_command"}]})
        self.call({"input": [{"type": "function_call_output", "call_id": SHELL_CALL,
                             "output": "shell complete"}]},
                  status=409)
        self.assertEqual(Model.error, "MCP tool not advertised")


class TestStructuredMcpReceipt(unittest.TestCase):
    def setUp(self):
        # Deliberately synthetic and reproducible: no credentials or live MCP.
        self.digest = "UNITTEST_SYNTHETIC_SHA_ONLY"

    def proof(self, observed=True, digest=None):
        return {"content": [{"type": "text", "text": json.dumps({
            "observed": observed,
            "sha256": self.digest if digest is None else digest,
        })}]}

    def test_genuine_nested_receipt_is_accepted(self):
        self.assertTrue(structured_mcp_receipt_matches(self.proof(), self.digest))
        self.assertTrue(structured_mcp_receipt_matches(
            {"result": self.proof()}, self.digest))

    def test_matching_digest_with_observed_false_is_rejected(self):
        self.assertFalse(structured_mcp_receipt_matches(
            self.proof(observed=False), self.digest))

    def test_incidental_digest_in_plain_diagnostic_is_rejected(self):
        text_only = {"content": [{"type": "text",
                                  "text": "not connected; digest " + self.digest}]}
        self.assertFalse(structured_mcp_receipt_matches(text_only, self.digest))

    def test_error_wrapping_valid_receipt_is_rejected(self):
        self.assertFalse(structured_mcp_receipt_matches(
            {"isError": True, "content": self.proof()["content"]}, self.digest))
        self.assertFalse(structured_mcp_receipt_matches(
            {"error": "native tool refused", "result": self.proof()}, self.digest))

    def test_wrong_or_missing_digest_is_rejected(self):
        self.assertFalse(structured_mcp_receipt_matches(
            self.proof(digest="incorrect"), self.digest))
        self.assertFalse(structured_mcp_receipt_matches(self.proof(), ""))
        self.assertFalse(structured_mcp_receipt_matches(
            {"content": [{"type": "text", "text": "not-json"}]}, self.digest))


class TestSyntheticMcpChild(unittest.TestCase):
    def test_name_only_fixture_exposes_inert_read_only_tool(self):
        # This child receives NO inherited or synthetic token in this unit test.
        proc = subprocess.run([sys.executable, "-c", CHILD], input=(
            json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/list"}) + "\n"
        ), capture_output=True, text=True, timeout=5, env={"PATH": os.environ.get("PATH", ""), **({"SYSTEMROOT": os.environ.get("SYSTEMROOT", "C:\\Windows")} if os.name == "nt" else {})})
        self.assertEqual(proc.returncode, 0)
        tool = json.loads(proc.stdout.strip())["result"]["tools"][0]
        self.assertEqual(tool["name"], "confirm_env")
        self.assertEqual(tool["annotations"]["readOnlyHint"], True)
        self.assertEqual(tool["annotations"]["destructiveHint"], False)


class TestNativeSecurityLifecycle(unittest.TestCase):
    def test_vendor_version_artifacts_reject_raw_or_tampered_output(self):
        good = SimpleNamespace(returncode=0, stdout=PINNED_VENDOR_VERSION + "\n",
                               stderr="")
        self.assertTrue(verified_vendor_version(good))
        for value in (
            SimpleNamespace(returncode=1, stdout=PINNED_VENDOR_VERSION,
                            stderr=""),
            SimpleNamespace(returncode=0, stdout=PINNED_VENDOR_VERSION,
                            stderr="unexpected native output"),
            SimpleNamespace(returncode=0,
                            stdout=PINNED_VENDOR_VERSION + " UAP_FAKE_CANARY",
                            stderr=""),
            SimpleNamespace(returncode=0, stdout="codex-cli 0.167.2",
                            stderr="")
        ):
            self.assertFalse(verified_vendor_version(value))


    def test_real_cli_captured_stdio_checks_encoded_synthetic_canary(self):
        secret = "uap_inert_stdout_fixture_56789"
        patterns = synthetic_canary_patterns(secret)
        for encoded in (secret.encode(), secret.encode().hex().encode(),
                        base64.b64encode(secret.encode()),
                        secret.encode("utf-16le"),
                        base64.b64encode(secret.encode("utf-16le"))):
            self.assertTrue(contains_synthetic_canary(encoded, patterns))
        self.assertFalse(contains_synthetic_canary(b"unrelated output", patterns))

    def test_appserver_jsonrpc_canary_detection_is_boolean_only(self):
        secret = "uap_inert_appserver_line_4321"
        patterns = synthetic_canary_patterns(secret)
        raw = json.dumps({"method": "turn/completed",
                          "params": {"threadId": "thread-1", "debug": secret}}).encode()
        parsed, seen, complete = classify_native_stdio_line(raw, patterns)
        self.assertTrue(seen)
        self.assertTrue(complete)
        self.assertEqual(parsed["method"], "turn/completed")
        clean, seen, complete = classify_native_stdio_line(
            b'{"method":"turn/completed"}', patterns
        )
        self.assertTrue(complete)
        self.assertFalse(seen)
        self.assertEqual(clean["method"], "turn/completed")

    def test_appserver_invalid_or_oversize_jsonrpc_cannot_pass_coverage(self):
        patterns = synthetic_canary_patterns("synthetic_value_fixture")
        for raw in (b"not json", b"x" * ((1 << 20) + 1)):
            parsed, seen, complete = classify_native_stdio_line(raw, patterns)
            self.assertIsNone(parsed)
            self.assertFalse(complete)
            self.assertFalse(seen)


    def test_incomplete_observer_join_never_certifies_secrecy(self):
        class DelayedScan:
            def __init__(self):
                self.waited = None
            def join(self, timeout):
                self.waited = timeout
            def is_alive(self):
                return True

        observer = DelayedScan()
        stopped = threading.Event()
        self.assertFalse(stop_live_observer(stopped, observer))
        self.assertTrue(stopped.is_set())
        self.assertGreaterEqual(observer.waited, 10)

    def test_completed_observer_join_is_acknowledged(self):
        class FinishedScan:
            def join(self, timeout):
                self.timeout = timeout
            def is_alive(self):
                return False

        observer = FinishedScan()
        stopped = threading.Event()
        self.assertTrue(stop_live_observer(stopped, observer))
        self.assertTrue(stopped.is_set())
        self.assertGreaterEqual(observer.timeout, 10)


    def test_turn_terminal_correlation_rejects_unrelated_and_failed_events(self):
        match = {"method": "turn/completed", "params": {
            "threadId": "thread-a", "turn": {"id": "turn-a", "status": "completed"}}}
        self.assertTrue(terminal_turn_status(match, "thread-a", "turn-a"))
        self.assertIsNone(terminal_turn_status(match, "thread-b", "turn-a"))
        self.assertIsNone(terminal_turn_status(match, "thread-a", "turn-b"))
        self.assertIsNone(terminal_turn_status({"method": "turn/started"},
                                               "thread-a", "turn-a"))
        self.assertFalse(terminal_turn_status({
            "method": "turn/completed", "params": {
                "threadId": "thread-a", "turn": {"id": "turn-a", "status": "failed"}}},
            "thread-a", "turn-a"))

    def test_windows_open_handle_must_be_beneath_local_disposable_root(self):
        root = r"C:\runner\temp\uap-proof"
        self.assertTrue(_windows_path_within_root(
            r"\\?\C:\runner\temp\uap-proof\home\scan.txt", root))
        self.assertFalse(_windows_path_within_root(
            r"\\?\C:\runner\temp\uap-proof-other\secret.txt", root))
        self.assertFalse(_windows_path_within_root(
            r"\\?\D:\other\scan.txt", root))
        self.assertFalse(_windows_path_within_root(
            r"\\?\UNC\server\share\file.txt", root))

    def test_root_junction_swap_fails_closed_without_reading_target(self):
        # Analogous to Windows root junction replacement; the initial anchor
        # must remain authoritative even after filesystem path resolution moves.
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "isolated_root"
            root.mkdir()
            (root / "benign").write_text("safe")
            pinned = _capture_scan_root(root)
            external = Path(temp) / "foreign_tree"
            external.mkdir()
            (external / "outside").write_text("EXTERNAL_SYNTHETIC_CANARY")
            (root / "benign").unlink()
            root.rmdir()
            try:
                root.symlink_to(external, target_is_directory=True)
            except OSError:
                self.skipTest("symlink/junction creation unsupported")
            findings, _ = inspect(root, "EXTERNAL_SYNTHETIC_CANARY",
                                  root_anchor=pinned)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0]["result"], "unscanned_directory")
            self.assertEqual(findings[0]["io_reason"], "root_identity_changed")
            self.assertNotIn("EXTERNAL_SYNTHETIC_CANARY", str(findings))
            self.assertNotIn(str(external), str(findings))

    def test_root_directory_replacement_without_symlink_fails_closed(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "isolated_root"
            root.mkdir()
            pinned = _capture_scan_root(root)
            root.rename(Path(temp) / "moved_original")
            root.mkdir()
            (root / "potential_outside").write_text("not_readable_by_scanner")
            findings, _ = inspect(root, "not_readable_by_scanner",
                                  root_anchor=pinned)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0]["result"], "unscanned_directory")
            self.assertEqual(findings[0]["io_reason"], "root_identity_changed")
            self.assertNotIn("not_readable_by_scanner", str(findings))

    def test_root_anchor_pins_lexical_and_canonical_at_creation(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            pinned = _capture_scan_root(root)
            self.assertEqual(pinned[0], str(root))
            self.assertEqual(pinned[1], str(root.resolve(strict=True)))
            self.assertEqual(len(pinned), 4)
            self.assertNotEqual(pinned[3], 0)

    def test_root_swap_during_posix_open_fails_on_descriptor_inode(self):
        if os.name == "nt":
            self.skipTest("POSIX root descriptor identity control")
        if os.open not in os.supports_dir_fd:
            self.skipTest("POSIX dir_fd not supported")
        with tempfile.TemporaryDirectory() as temp:
            base = Path(temp)
            root = base / "disposable_root"
            root.mkdir()
            target = root / "probe"
            target.write_text("benign")
            observed = target.lstat()
            anchor = _capture_scan_root(root)
            moved = base / "original_moved"
            replacement = base / "replacement_moved"
            original_open = os.open
            swapped = False

            def swap_during_root_open(path, flags, *args, **kwargs):
                nonlocal swapped
                if Path(path) == root and not swapped:
                    swapped = True
                    root.rename(moved)
                    root.mkdir()
                    (root / "probe").write_text("OUTSIDE_SYNTHETIC_CANARY")
                    fd = original_open(path, flags, *args, **kwargs)
                    (root / "probe").unlink()
                    root.rename(replacement)
                    moved.rename(root)
                    # The lexical root pathname is now ORIGINAL again:
                    # comparing only before/after names misses this swap.
                    self.assertEqual(_capture_scan_root(root), anchor)
                    return fd
                return original_open(path, flags, *args, **kwargs)

            with patch("codex_env_names_native_spike.os.open",
                       side_effect=swap_during_root_open):
                with self.assertRaises(OSError):
                    _read_regular_bounded(target, observed, root, anchor)
            self.assertTrue(swapped)
            self.assertEqual(target.read_text(), "benign")
            self.assertEqual((replacement / "probe").exists(), False)

    def test_windows_path_identity_excludes_only_incompatible_ctime(self):
        baseline = dict(st_dev=1, st_ino=5, st_size=8, st_mtime_ns=99)
        before = SimpleNamespace(**baseline, st_ctime_ns=10)
        after = SimpleNamespace(**baseline, st_ctime_ns=20)
        self.assertEqual(_scan_stat_identity(before, include_change_time=False),
                         _scan_stat_identity(after, include_change_time=False))
        self.assertNotEqual(_scan_stat_identity(before),
                            _scan_stat_identity(after))
        changed = SimpleNamespace(**(baseline | {"st_mtime_ns": 100}),
                                  st_ctime_ns=20)
        self.assertNotEqual(_scan_stat_identity(before, include_change_time=False),
                            _scan_stat_identity(changed, include_change_time=False))


class TestIsolatedProjectTrustRoot(unittest.TestCase):
    def test_new_spike_root_resolves_disposable_temp_alias(self):
        # macOS usually spells TMPDIR /var/folders/..., while realpath is
        # /private/var/folders/...; Codex CLI trust lookup may differ.
        with tempfile.TemporaryDirectory() as base:
            physical = Path(base) / "physical"
            physical.mkdir()
            alias = Path(base) / "alias"
            try:
                alias.symlink_to(physical, target_is_directory=True)
            except OSError:
                self.skipTest("cannot create disposable directory symlink")
            with patch("codex_env_names_native_spike.tempfile.mkdtemp",
                       return_value=str(alias)) as make:
                result = isolated_root("uap-test-")
                make.assert_called_once_with(prefix="uap-test-")
            self.assertEqual(result, physical.resolve(strict=True))
            self.assertNotEqual(str(result), str(alias))


class TestSyntheticSnapshotScanner(unittest.TestCase):
    def test_unreadable_walk_directory_is_a_fail_closed_finding(self):
        # os.walk's default onerror=None drops unreadable subtrees silently.
        # Simulate scandir failure without changing runner permissions.
        with tempfile.TemporaryDirectory() as temp:
            unreadable = str(Path(temp) / "unreadable")
            def fake_walk(_root, *, followlinks, onerror):
                self.assertFalse(followlinks)
                self.assertIsNotNone(onerror)
                onerror(PermissionError(13, "denied", unreadable))
                return iter(())
            with patch("codex_env_names_native_spike.os.walk",
                       side_effect=fake_walk):
                findings, symlinks = inspect(Path(temp), "synthetic_only")
            self.assertEqual(symlinks, 0)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0]["result"], "unscanned_directory")
            self.assertEqual(findings[0]["io_reason"], "access_denied")
            self.assertEqual(findings[0]["scope"], "other")
            self.assertEqual(findings[0]["path_sha256"],
                             hashlib.sha256(b"unreadable").hexdigest())
            self.assertNotIn(unreadable, str(findings))
            self.assertNotIn("synthetic_only", str(findings))

    def test_same_path_live_canary_then_external_symlink_keeps_both(self):
        path_digest = "a" * 64
        findings, counts = aggregate_findings(
            [{"path_sha256": path_digest, "result": "synthetic_value_persisted",
              "shell_snapshot_path": True}],
            [{"path_sha256": path_digest, "result": "unscanned_external_symlink"}],
        )
        self.assertEqual(len(findings), 2)
        self.assertEqual(counts["synthetic_value_persisted"], 1)
        self.assertEqual(counts["unscanned_external_symlink"], 1)
        self.assertTrue(next(f for f in findings
                             if f["result"] == "synthetic_value_persisted")["shell_snapshot_path"])

    def test_same_category_deduplicates_without_erasing_snapshot_origin(self):
        digest = "b" * 64
        findings, counts = aggregate_findings(
            [{"path_sha256": digest, "result": "synthetic_value_persisted",
              "shell_snapshot_path": False}],
            [{"path_sha256": digest, "result": "synthetic_value_persisted",
              "shell_snapshot_path": True}],
        )
        self.assertEqual(len(findings), 1)
        self.assertTrue(findings[0]["shell_snapshot_path"])
        self.assertEqual(counts, {"synthetic_value_persisted": 1})

    def test_untruncated_counts_survive_bounded_evidence_samples(self):
        observations = [{"path_sha256": f"{i:064x}",
                         "result": "synthetic_value_persisted"} for i in range(17)]
        findings, counts = aggregate_findings(observations)
        self.assertEqual(len(findings[:10]), 10)
        self.assertEqual(len(findings), 17)
        self.assertEqual(counts["synthetic_value_persisted"], 17)

    def test_raw_and_common_encoded_canary_are_detected(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            snapshot = root / "home" / ".codex" / "shell_snapshots" / "test.sh"
            snapshot.parent.mkdir(parents=True)
            token = "not_a_real_secret_fixture_123"
            for content in [token.encode(), token.encode().hex().encode(),
                            token.encode().hex().upper().encode(),
                            base64.b64encode(token.encode()),
                            token.encode("utf-16le"), token.encode("utf-16be"),
                            base64.b64encode(token.encode("utf-16le")),
                            base64.b64encode(token.encode("utf-16be"))]:
                snapshot.write_bytes(content)
                findings, symlinks = inspect(root, token)
                self.assertEqual(symlinks, 0)
                self.assertEqual(len(findings), 1)
                # Win32 may refuse a just-rewritten fixture when the
                # descriptor/path metadata cannot establish stable identity.
                # That is a SECURITY NO-GO, never a false no-leak PASS; a
                # separate stable-fixture test still requires positive matches.
                if os.name == "nt" and findings[0]["result"] == "unscanned_file":
                    self.assertEqual(findings[0]["scope"], "codex_home")
                else:
                    self.assertEqual(findings[0]["result"], "synthetic_value_persisted")
                    self.assertTrue(findings[0]["shell_snapshot_path"])
                self.assertNotIn(token, str(findings))
            snapshot.write_text("nonsecret")
            final_findings, _ = inspect(root, token)
            if os.name == "nt":
                # A Win32 descriptor/path identity mismatch is an incomplete
                # scan, not evidence that the canary is absent. Preserve NO-GO.
                self.assertTrue(not final_findings or all(
                    item["result"] == "unscanned_file"
                    and item["scope"] == "codex_home"
                    for item in final_findings))
            else:
                self.assertEqual(final_findings, [])

    def test_second_independent_synthetic_value_is_scanned(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            first, second = "synthetic_one_value", "synthetic_other_value"
            file = root / "foreign_snapshot"
            file.write_text(second)
            findings, symlinks = inspect(root, (first, second))
            self.assertEqual(symlinks, 0)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0]["result"], "synthetic_value_persisted")
            self.assertNotIn(second, str(findings))

    def test_external_symlink_fails_closed_without_following_target(self):
        with tempfile.TemporaryDirectory() as temp, tempfile.TemporaryDirectory() as foreign:
            root = Path(temp)
            outside = Path(foreign) / "synthetic_outside"
            outside.write_text("secret_in_unscanned_external_target")
            link = root / "external_link"
            try:
                link.symlink_to(outside)
            except (OSError, NotImplementedError):
                self.skipTest("Symlink creation not supported on this runner")
            hits, symlinks = inspect(root, "secret_in_unscanned_external_target")
            self.assertEqual(symlinks, 1)
            self.assertEqual(len(hits), 1)
            self.assertEqual(hits[0]["result"], "unscanned_external_symlink")
            self.assertNotIn(str(outside), str(hits))

    def test_junction_like_directory_outside_root_is_pruned_fail_closed(self):
        # Mock a Windows junction on every OS, even without symlink privileges.
        with tempfile.TemporaryDirectory() as temp, tempfile.TemporaryDirectory() as foreign:
            root = Path(temp)
            junction = root / "simulated_junction"
            junction.mkdir()
            (junction / "must_not_read").write_text("synthetic_canary_test")
            native_resolve = Path.resolve

            def resolve(path, *args, **kwargs):
                if path == junction:
                    return Path(foreign)
                return native_resolve(path, *args, **kwargs)

            with patch.object(Path, "is_junction", new=lambda path: path == junction,
                              create=True), patch.object(Path, "resolve", new=resolve), \
                    patch.object(Path, "read_bytes",
                                 side_effect=AssertionError("junction target must not be read")):
                findings, links = inspect(root, "synthetic_canary_test")
            self.assertEqual(links, 1)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0]["result"], "unscanned_external_symlink")
            self.assertEqual(len(findings[0]["path_sha256"]), 64)
            self.assertNotIn(str(junction), str(findings))
            self.assertNotIn(str(Path(foreign)), str(findings))

    def test_internal_symlink_is_not_followed_or_false_positive(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / "safe"
            source.write_text("not the canary")
            link = root / "in_tree"
            try:
                link.symlink_to(source)
            except (OSError, NotImplementedError):
                self.skipTest("Symlink creation not supported on this runner")
            hits, symlinks = inspect(root, "canary_value")
            self.assertEqual(symlinks, 1)
            self.assertEqual(hits, [])

    def test_fifo_is_fail_closed_without_blocking_on_read(self):
        if not hasattr(os, "mkfifo"):
            self.skipTest("Named pipes unavailable on this OS")
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            fifo = root / "synthetic_nonregular_fixture"
            try:
                os.mkfifo(fifo)
            except OSError:
                self.skipTest("Named pipe creation unavailable on this runner")
            with patch("codex_env_names_native_spike._read_regular_bounded",
                              side_effect=AssertionError("special file must not be opened")):
                findings, symlinks = inspect(root, "only_a_synthetic_test_value")
            self.assertEqual(symlinks, 0)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0]["result"], "unscanned_special_file")
            self.assertEqual(len(findings[0]["path_sha256"]), 64)
            self.assertNotIn(str(fifo), str(findings))
            self.assertNotIn("only_a_synthetic_test_value", str(findings))

    def test_locked_file_is_fail_closed_without_logging_sensitive_metadata(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            protected = root / "private_canary_test"
            protected.write_text("only_a_test")
            with patch("codex_env_names_native_spike._read_regular_bounded",
                              side_effect=PermissionError(13, "test_denied", str(protected))):
                findings, symlinks = inspect(root, "synthetic_only")
            self.assertEqual(symlinks, 0)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0]["result"], "unscanned_file")
            self.assertEqual(len(findings[0]["path_sha256"]), 64)
            self.assertNotIn("private_canary_test", str(findings))
            self.assertNotIn("test_denied", str(findings))
            self.assertNotIn("synthetic_only", str(findings))

    def test_unreadable_sqlite_shm_still_fails_closed(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            db_dir = root / "home" / ".codex"
            db_dir.mkdir(parents=True)
            locked = db_dir / "state_5.sqlite-shm"
            locked.write_bytes(b"benign")
            with patch("codex_env_names_native_spike._read_regular_bounded",
                              side_effect=OSError(13, "sensitive path and message", str(locked))):
                findings, links = inspect(root, "synthetic_token")
            self.assertEqual(links, 0)
            self.assertEqual(len(findings), 1)
            item = findings[0]
            self.assertEqual(item["result"], "unscanned_file")
            self.assertEqual(item["io_reason"], "access_denied")
            self.assertEqual(item["scope"], "codex_home")
            self.assertTrue(item["sqlite_shm_candidate"])
            self.assertNotIn(str(locked), str(findings))
            self.assertNotIn("sensitive path and message", str(findings))
            self.assertNotIn("synthetic_token", str(findings))

    def test_unreadable_project_file_does_not_mislabel_sqlite(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            project = root / "project"
            project.mkdir()
            blocked = project / "report.txt"
            blocked.write_text("benign")
            with patch("codex_env_names_native_spike._read_regular_bounded",
                              side_effect=OSError(5, "do not serialize this")):
                findings, _ = inspect(root, "synthetic_token")
            self.assertEqual(findings[0]["io_reason"], "other_io_error")
            self.assertEqual(findings[0]["scope"], "project")
            self.assertFalse(findings[0]["sqlite_shm_candidate"])
            self.assertNotIn("do not serialize this", str(findings))

    def test_atomic_unlink_during_read_fails_closed_without_raw_data(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            file = root / "transient"
            file.write_text("nonsecret")
            with patch("codex_env_names_native_spike._read_regular_bounded",
                              side_effect=FileNotFoundError(
                                  2, "untrusted error text", str(file))):
                findings, _ = inspect(root, "synthetic_token")
            self.assertEqual(len(findings), 1)
            item = findings[0]
            self.assertEqual(item["result"], "unscanned_file")
            self.assertEqual(item["io_reason"], "vanished_during_walk")
            self.assertEqual(item["scope"], "other")
            self.assertFalse(item["sqlite_shm_candidate"])
            self.assertNotIn(str(file), str(findings))
            self.assertNotIn("untrusted error text", str(findings))
            self.assertNotIn("synthetic_token", str(findings))


    def test_path_swap_to_external_symlink_never_reads_external_file(self):
        if os.name == "nt":
            self.skipTest("final-component symlink swap requires POSIX O_NOFOLLOW")
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "scan"
            root.mkdir()
            outside = Path(temp) / "foreign_credential_fixture"
            outside.write_text("outside_only_synthetic_canary")
            victim = root / "mutable"
            victim.write_text("safe")
            original_open = os.open
            swapped = False

            def swap_before_open(path, flags, *args, **kwargs):
                nonlocal swapped
                if Path(path).name == victim.name and not swapped:
                    swapped = True
                    victim.unlink()
                    victim.symlink_to(outside)
                return original_open(path, flags, *args, **kwargs)

            with patch("codex_env_names_native_spike.os.open",
                       side_effect=swap_before_open):
                findings, _ = inspect(root, "outside_only_synthetic_canary")
            self.assertTrue(swapped)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0]["result"], "unscanned_file")
            self.assertNotIn("foreign_credential_fixture", str(findings))
            self.assertNotIn("outside_only_synthetic_canary", str(findings))

    def test_ancestor_swap_to_external_symlink_never_reads_outside_root(self):
        if os.name == "nt":
            self.skipTest("POSIX parent descriptor control; Windows uses handle final path")
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "scan"
            branch = root / "branch"
            branch.mkdir(parents=True)
            file = branch / "report"
            file.write_text("initially_safe")
            external = Path(temp) / "external"
            external.mkdir()
            (external / "report").write_text("FOREIGN_SYNTHETIC_VALUE")
            open_original = os.open
            swapped = False

            def swap_directory(path, flags, *args, **kwargs):
                nonlocal swapped
                if path == "branch" and "dir_fd" in kwargs and not swapped:
                    swapped = True
                    file.unlink()
                    branch.rmdir()
                    branch.symlink_to(external, target_is_directory=True)
                return open_original(path, flags, *args, **kwargs)

            with patch("codex_env_names_native_spike.os.open",
                       side_effect=swap_directory):
                findings, _ = inspect(root, "FOREIGN_SYNTHETIC_VALUE")
            self.assertTrue(swapped)
            self.assertTrue(any(x["result"] == "unscanned_file" for x in findings))
            self.assertFalse(any(x["result"] == "synthetic_value_persisted"
                                 for x in findings))
            self.assertNotIn("FOREIGN_SYNTHETIC_VALUE", str(findings))
            self.assertNotIn(str(external), str(findings))

    def test_file_mutation_between_stat_and_open_fails_closed(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            victim = root / "mutable"
            victim.write_text("safe")
            original_open = os.open
            mutated = False

            def mutate_before_open(path, flags, *args, **kwargs):
                nonlocal mutated
                if Path(path).name == victim.name and not mutated:
                    mutated = True
                    victim.write_text("new_inert_contents_different_length")
                return original_open(path, flags, *args, **kwargs)

            with patch("codex_env_names_native_spike.os.open",
                       side_effect=mutate_before_open):
                findings, _ = inspect(root, "no_matching_canary")
            self.assertTrue(mutated)
            self.assertEqual(len(findings), 1)
            self.assertEqual(findings[0]["result"], "unscanned_file")


if __name__ == "__main__":
    unittest.main()
