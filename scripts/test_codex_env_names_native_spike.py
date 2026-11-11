"""Unit negative controls for Codex native secrecy evidence, no network/vendor client."""
import http.client
import base64
import json
import os
import subprocess
import sys
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import patch
import http.server

from codex_env_names_native_spike import (CHILD, Model, MCP_CALL, SHELL_CALL, SHELL_MARKER,
                                        SHELL_ENV_PRESENT, SHELL_FOREIGN_PRESENT,
                                        ENV_NAME, FOREIGN_ENV_NAME, inspect, aggregate_findings, windows_shell_command, isolated_env)


class TestSyntheticModelOracle(unittest.TestCase):
    def setUp(self):
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

    def test_windows_shell_command_only_emits_fixed_markers(self):
        command = windows_shell_command()
        self.assertIn("Start-Sleep -Seconds 7", command)
        self.assertIn("Test-Path Env:" + ENV_NAME, command)
        self.assertIn("Test-Path Env:" + FOREIGN_ENV_NAME, command)
        self.assertIn(SHELL_ENV_PRESENT, command)
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
            self.assertEqual(len(findings[0]["path_sha256"]), 64)
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
                            token.encode("utf-16le"), token.encode("utf-16be"),
                            base64.b64encode(token.encode("utf-16le"))]:
                snapshot.write_bytes(content)
                findings, symlinks = inspect(root, token)
                self.assertEqual(symlinks, 0)
                self.assertEqual(len(findings), 1)
                self.assertEqual(findings[0]["result"], "synthetic_value_persisted")
                self.assertTrue(findings[0]["shell_snapshot_path"])
                self.assertNotIn(token, str(findings))
            snapshot.write_text("nonsecret")
            self.assertEqual(inspect(root, token)[0], [])

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

    def test_atomic_unlink_during_read_is_not_false_positive(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            file = root / "transient"
            file.write_text("nonsecret")
            with patch.object(Path, "read_bytes", side_effect=FileNotFoundError):
                findings, _ = inspect(root, "synthetic_token")
            self.assertEqual(findings, [])


if __name__ == "__main__":
    unittest.main()
