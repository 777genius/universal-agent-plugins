"""Inert native envelope and exact selection regressions; no client execution."""
import io
import hashlib
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
            test.write_text("func " + runtime.CASES["copilot"]["test"] + "(t *testing.T) {\n}")
            self.assertEqual(runtime.selected_cases(), {"include": [{"client": "copilot", "phase": "runtime-attempt"}]})

    def test_host_guard_runs_before_creating_output_or_native_child(self):
        with tempfile.TemporaryDirectory() as temporary, patch.dict(os.environ, {"GITHUB_ACTIONS": "false"}):
            path = Path(temporary) / "must-not-exist"
            with self.assertRaises(RuntimeError):
                runtime.run("copilot", path)
            self.assertFalse(path.exists())

    def test_copilot_full_inert_process_gate_precedes_native_matrix(self):
        workflow = (Path(__file__).resolve().parent.parent /
                    ".github/workflows/project-native-runtime.yml").read_text()
        contracts, native = workflow.split("\n  native:\n", 1)
        prepare = "      - name: Prepare pinned Go for selected Copilot inert process contracts\n"
        execute = "      - name: Verify selected Copilot inert process contracts before native execution\n"
        selection, toolchain = contracts.split(prepare, 1)
        toolchain, gate = toolchain.split(execute, 1)
        condition = "        if: ${{ contains(fromJSON(steps.selection.outputs.matrix).include.*.client, 'copilot') }}\n"
        self.assertIn("        id: selection\n", selection)
        self.assertTrue(toolchain.startswith(condition))
        self.assertTrue(gate.startswith(condition))
        self.assertIn("actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e", toolchain)
        self.assertIn("          go-version: '1.26.10'\n          cache: false\n", toolchain)
        self.assertEqual(gate.removeprefix(condition).strip(),
                         "run: go test -json -p=1 -count=1 -timeout=120s ./repotests -run '^(TestCopilot|TestProjectNativeRuntime)'")
        self.assertIn("    runs-on: ubuntu-24.04\n    timeout-minutes: 5\n", contracts)
        self.assertNotIn("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED", contracts)
        self.assertIn("    needs: contracts\n", native)

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

    def qoder_log_observation(self):
        run_id = "2026-10-28T15-30-45-123Z-abc123-p1234"
        return {"status": "captured", "run": {"run_id": run_id, "latest_target": "runs/" + run_id,
            "manifest_bytes": 991, "manifest_sha256": "a" * 64, "pid": 1234, "ppid": 5678,
            "started_at": "2026-10-28T15:30:45.123Z", "cli_version": "1.1.78", "platform": "linux", "arch": "x64",
            "process_role": "main", "pid_provenance": "manifest-only", "run_id_matches_manifest": True,
            "cwd_matches": True, "argv_matches": True, "exact_added_run_closure": True, "preexisting_records_stable": True}}

    def test_qoder_log_projection_is_optional_finite_and_failed_only(self):
        for client in runtime.CLI_ATTEMPTS:
            original = {**self.cli_result(client), "native_inspection": {"enabled": client != "grok"}}
            self.assertEqual(runtime.cli_attempt_members({"runtime-attempt.json": json.dumps(original).encode()}, self.source, client), original)
        for status in ("captured", "context-refused", "profile-refused", "layout-refused", "manifest-refused", "observation-bound"):
            projection = self.qoder_log_observation() if status == "captured" else {"status": status}
            result = {**self.cli_result("qoder"), "qoder_log_run_observation": projection}
            admitted = runtime.cli_attempt_members({"runtime-attempt.json": json.dumps(result).encode()}, self.source, "qoder")
            self.assertEqual(admitted, result)
            self.assertFalse(admitted["runtime_attempt_complete"])
            self.assertTrue(admitted["test_failed"])
            self.assertFalse(any(admitted["qualification"].values()))
            if status != "captured":
                with self.assertRaises(ValueError):
                    runtime.qoder_log_run_observation({"qoder_log_run_observation": {**projection, "PRIVATE": "PRIVATE"}}, "qoder")
        for client in ("grok", "kiro", "windsurf"):
            with self.assertRaises(ValueError):
                runtime.qoder_log_run_observation({"qoder_log_run_observation": self.qoder_log_observation()}, client)

    def test_qoder_log_projection_refuses_unbounded_private_and_forged_provenance(self):
        changes = {"latest_target": "../PRIVATE", "run_id": "PRIVATE", "manifest_bytes": 4097,
                   "manifest_sha256": "PRIVATE", "pid": True, "ppid": 0, "started_at": "PRIVATE",
                   "cli_version": "PRIVATE", "platform": "PRIVATE", "arch": "PRIVATE", "process_role": "PRIVATE",
                   "pid_provenance": "admitted-native-pid", "argv_matches": 1, "PRIVATE": "PRIVATE"}
        for key, value in changes.items():
            projection = self.qoder_log_observation()
            projection["run"][key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                runtime.qoder_log_run_observation({"qoder_log_run_observation": projection}, "qoder")
        for key in ("status", "run"):
            projection = self.qoder_log_observation()
            del projection[key]
            with self.assertRaises(ValueError):
                runtime.qoder_log_run_observation({"qoder_log_run_observation": projection}, "qoder")
        for key, value in (("pid", 1235), ("started_at", "2026-10-28T15:30:45.124Z")):
            projection = self.qoder_log_observation()
            projection["run"][key] = value
            with self.assertRaises(ValueError):
                runtime.qoder_log_run_observation({"qoder_log_run_observation": projection}, "qoder")
            projection["run"]["run_id_matches_manifest"] = False
            runtime.qoder_log_run_observation({"qoder_log_run_observation": projection}, "qoder")

    def test_qoder_log_projection_calendar_and_offset_match_source_date(self):
        for stamp in ("2026-10-28T15:30:60.123Z", "2026-10-28T15:30:61.123Z", "2026-02-30T15:30:45.123Z",
                      "2026-10-28T15:30:45.123+24:00", "2026-10-28T15:30:45.123+05:60", "2026-10-28T15:30:45.123+00:00"):
            projection = self.qoder_log_observation()
            run_id = stamp.replace(":", "-").replace(".", "-") + "-abc123-p1234"
            projection["run"].update({"started_at": stamp, "run_id": run_id, "latest_target": "runs/" + run_id})
            with self.subTest(stamp=stamp), self.assertRaises(ValueError):
                runtime.qoder_log_run_observation({"qoder_log_run_observation": projection}, "qoder")

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

    def cline_cycle_bodies(self, complete=True):
        result = {**self.cline_result(), "capture_mode": "cline-complete-cycle-first-capture",
                  "cycle_capture_review_required": True, "cycle_observed_complete": complete,
                  "cycle_public_stages": list(runtime.CLINE_CYCLE_PUBLIC) if complete else [],
                  "cycle_listing_stages": list(runtime.CLINE_CYCLE_LISTINGS) if complete else [],
                  "cycle_acp_stages": list(runtime.CLINE_CYCLE_ACP) if complete else []}
        records = {name: [] for name in runtime.CLINE_CYCLE_AGGREGATES}
        bodies = {}
        def stream(body):
            return {"bytes": len(body), "sha256": hashlib.sha256(body).hexdigest(),
                    "body": runtime.base64.b64encode(body).decode()}
        def add(name, stage, value):
            records[name].append({"stage": stage, "value": value})
        if complete:
            result["attempt_stage"] = "complete-cycle-capture-review"
            result["cycle_provider_cleanup"] = {"shutdown_complete": True, "forced_close": False, "error": False,
                                                 "completed_stages": list(runtime.CLINE_CYCLE_ACP)}
            for label in runtime.CLINE_CYCLE_PUBLIC:
                absent = label == "repeat-remove-a"
                row = {"argv": ["/inert/public", label], "cwd": "/inert/A", "exit": int(absent),
                       "error": False, "error_kind": "none", "deadline": False}
                bodies[label + ".command.json"] = json.dumps(row).encode()
                bodies[label + ".stdout.log"] = b"" if absent else b"{}"
                bodies[label + ".stderr.log"] = (b'agentplugins: plugin is not installed for target "cline" in project scope; no target was changed\n' if absent else b"")
            for index, stage in enumerate(runtime.CLINE_CYCLE_ACP):
                empty = stage in runtime.CLINE_CYCLE_EMPTY
                add("cycle-native-acp.jsonl", stage, stream(b'{"inert":"admission only"}\n'))
                add("cycle-probe-events.jsonl", stage, stream(b"" if empty else b'{"inert":"admission only"}\n'))
                add("cycle-native-commands.jsonl", stage, {"argv": ["/inert/cline", "--auto-approve", "false", "--cwd", "/inert/A", "--acp"],
                    "cwd": "/inert/A", "mcp_settings_override": "/inert/A/mcp.json", "auto_approve": False,
                    "injected_mcp_servers": [], "error": False, "deadline": False, "error_stream": stream(b""),
                    "started_at": "2026-10-27T00:00:00Z", "ended_at": "2026-10-27T00:00:01Z",
                    "session": f"1793012345678_{index:05d}_cli", "cleanup": "existing OS strict planned duplex shutdown; entire delegated descendant cgroup"})
                count = 1 if empty else 2
                for number in range(count):
                    for name in ("cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl"):
                        add(name, stage, {"index": number, "stream": stream(b"{}")})
                add("cycle-provider-receipts.jsonl", stage, {"requests": count, "responses": count, "empty": empty,
                    "exact_call_issued": not empty, "single_call_approved": not empty, "error": False,
                    "call_id": stage, "prompt": stage, "sentinel": stage, "operation": "empty" if empty else ("write" if stage.startswith("installed-") else "read")})
            for stage in runtime.CLINE_CYCLE_LISTINGS:
                for kind in ("skills", "mcp"):
                    add("cycle-discovery.jsonl", stage, {"kind": kind, "argv": ["/inert/cline", "--auto-approve", "false", "--cwd", "/inert/A", "config", kind, "--json"],
                        "cwd": "/inert/A", "mcp_settings_override": True, "stdout": stream(b"[]"), "stderr": stream(b""),
                        "exit": 0, "error": False, "deadline": False})
            for stage in runtime.CLINE_CYCLE_PROOF:
                if stage.startswith("source-"):
                    value = {"source": "/inert/source", "digest": "sha256:inert", "files": {}}
                else:
                    value = {"scope": "a", "revision": "a-one", "removed": stage.startswith("remove"),
                             "state": stream(b'{"schema_version":5,"installations":[]}'), "binding": {}, "source_digest": "sha256:inert"}
                add("cycle-proof.jsonl", stage, value)
        for name, rows in records.items():
            bodies[name] = b"".join(json.dumps(row).encode() + b"\n" for row in rows)
        bodies["runtime-attempt.json"] = json.dumps(result).encode()
        return bodies

    def test_cline_cycle_capture_is_closed_and_always_unqualified(self):
        for complete in (False, True):
            bodies = self.cline_cycle_bodies(complete)
            result = runtime.cline_members(bodies, self.source)
            self.assertTrue(result["test_failed"])
            self.assertFalse(result["runtime_attempt_complete"])
            self.assertFalse(any(result["qualification"].values()))
        bodies = self.cline_cycle_bodies()
        self.assertEqual(len(bodies), 54)
        for member in bodies:
            with self.subTest(missing=member), self.assertRaises(ValueError):
                runtime.cline_members({name: body for name, body in bodies.items() if name != member}, self.source)
        for change in ({"runtime_attempt_complete": True}, {"test_failed": False}, {"cycle_capture_review_required": False},
                       {"cycle_public_stages": list(runtime.CLINE_CYCLE_PUBLIC[:-1])}, {"cycle_listing_stages": []},
                       {"cycle_acp_stages": []}, {"source_identity": {}}, {"cycle_provider_cleanup": {}}):
            result = json.loads(bodies["runtime-attempt.json"])
            with self.subTest(change=change), self.assertRaises(ValueError):
                runtime.cline_members({**bodies, "runtime-attempt.json": json.dumps({**result, **change}).encode()}, self.source)

    def test_cline_cycle_numeric_prefix_requires_complete_json_syntax(self):
        benign = b"1e999" + b"a" * 59
        self.assertEqual(len(benign), 64)
        self.assertFalse(runtime.cline_cycle_private(benign))
        self.assertFalse(runtime.cline_cycle_private(b"1e999 1e999"))
        outer = json.dumps({benign.decode(): [benign.decode(), {"nested": benign.decode()}]}).encode()
        self.assertFalse(runtime.cline_cycle_private(outer))
        self.assertFalse(runtime.cline_cycle_private(outer + b"\n" + outer))
        budget = [4 * len(benign), 16384]
        self.assertFalse(runtime.cline_cycle_complete_numeric_json(benign, budget))
        self.assertEqual(budget[0], 3 * len(benign))
        for raw in (b"1e999", b'{"n":1e999}', b"[1e999]"):
            self.assertTrue(runtime.cline_cycle_private(raw))
        for raw in (benign, b"1e999 1e999", b"NaN", b"Infinity", b"-Infinity"):
            self.assertFalse(runtime.cline_cycle_complete_numeric_json(raw, [4 * len(raw), 16384]))
        self.assertTrue(runtime.cline_cycle_private(benign + b'\n{"key":"\\u0075ap-cline-private-inert"}'))
        deep = b"[" * 10050 + b"]" * 10050
        self.assertTrue(runtime.cline_cycle_complete_numeric_json(deep, [4 * len(deep), 16384]))
        self.assertTrue(runtime.cline_cycle_complete_numeric_json(benign, [len(benign) - 1, 16384]))

    def test_cline_cycle_rejects_stage_skip_duplicates_receipts_and_encoded_canaries(self):
        bodies = self.cline_cycle_bodies()
        for member in runtime.CLINE_CYCLE_AGGREGATES:
            rows = [json.loads(line) for line in bodies[member].splitlines()]
            for changed in (rows[1:], [rows[0], *rows], [{**rows[0], "stage": "unregistered"}, *rows[1:]]):
                with self.subTest(member=member), self.assertRaises(ValueError):
                    runtime.cline_members({**bodies, member: b"".join(json.dumps(row).encode() + b"\n" for row in changed)}, self.source)
        member = "cycle-native-acp.jsonl"
        rows = [json.loads(line) for line in bodies[member].splitlines()]
        for value in ({**rows[0]["value"], "bytes": True}, {**rows[0]["value"], "sha256": "0" * 64},
                      {**rows[0]["value"], "body": "%%"}):
            changed = [{**rows[0], "value": value}, *rows[1:]]
            with self.assertRaises(ValueError):
                runtime.cline_members({**bodies, member: b"".join(json.dumps(row).encode() + b"\n" for row in changed)}, self.source)
        raw = b'{"start":true}\n{"opaque":"\\u0075ap-cline-private-inert"}\n'
        self.assertNotIn(b"uap-cline-private-", raw)
        self.assertTrue(runtime.cline_cycle_private(raw))
        nested = b'{"key":"\\u0075ap-cline-private-inert"}'
        for _ in range(8):
            nested = json.dumps({"content": nested.decode()}).encode()
            self.assertNotIn(b"uap-cline-private-", nested)
            self.assertTrue(runtime.cline_cycle_private(nested))
            self.assertTrue(runtime.cline_cycle_private(b'{"first":true}\n' + nested))
        benign = json.dumps({"content": '{"key":"ordinary"}\n{"tail":true}'}).encode()
        self.assertFalse(runtime.cline_cycle_private(benign))
        for ambiguous in (b"\xff", b'{"same":1,"same":2}', b"[" * 65 + b"0" + b"]" * 65,
                          b"[" + b"0," * 17000 + b"0]"):
            self.assertTrue(runtime.cline_cycle_private(ambiguous))
        value = {"bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest(), "body": runtime.base64.b64encode(raw).decode()}
        with self.assertRaises(ValueError):
            runtime.cline_cycle_stream(value)

    def cline_complete_bodies(self):
        # Inert envelope receipts; actual native grammar is replayed by the Go
        # contracts from the two exact measured nonsecret JSONL fixtures.
        fixture = Path(__file__).resolve().parent.parent / "repotests/testdata/cline-3.0.73-installed-acp"
        result = {**self.cline_result(), "runtime_attempt_complete": True, "test_failed": False,
                  "native_exchange_observed": True, "attempt_stage": "native-installed-acp",
                  "native_session_id": "1793077736317_iF4rU_cli"}
        bodies = {"runtime-attempt.json": json.dumps(result).encode(),
                  "native-acp.jsonl": (fixture / "native-acp.jsonl").read_bytes(),
                  "probe-events.jsonl": (fixture / "probe-events.jsonl").read_bytes()}
        for label in runtime.CLINE_COMMANDS:
            command = {"argv": ["/inert/never-executed", label], "cwd": "/inert/project", "exit": 0,
                       "error": False, "deadline": False}
            bodies[label + ".command.json"] = json.dumps(command).encode()
            bodies[label + ".stdout.log"] = b""
            bodies[label + ".stderr.log"] = b""
        command = {"argv": ["/inert/cline", "--acp"], "cwd": "/inert/project", "mcp_settings_override": "/inert/mcp.json",
                   "auto_approve": False, "injected_mcp_servers": [], "error": False, "elapsed_ms": 1,
                   "cleanup": "existing OS strict planned duplex shutdown; entire delegated descendant cgroup",
                   "stderr_contract": "existing OS returns bounded 32KiB diagnostic on error; successful planned shutdown exposes no diagnostic stream"}
        bodies["native-acp.command.json"] = json.dumps(command).encode()
        provider = {"requests": 2, "responses": 2, "exact_call_issued": True, "single_call_approved": True,
                    "error": False, "transport": "loopback scripted OpenAI chat-completions SSE; no real model"}
        bodies["provider.json"] = json.dumps(provider).encode()
        for number in (1, 2):
            bodies[f"provider-{number}.request.json"] = b'{"inert-envelope":true}'
            bodies[f"provider-{number}.response.log"] = b"inert envelope bytes; never a native observation"
        return bodies

    def test_cline_complete_attempt_requires_the_exact_closed_success_members(self):
        bodies = self.cline_complete_bodies()
        result = runtime.cline_members(bodies, self.source)
        self.assertTrue(result["runtime_attempt_complete"])
        self.assertFalse(result["test_failed"])
        self.assertFalse(any(result["qualification"].values()))
        for missing in bodies:
            with self.subTest(missing=missing), self.assertRaises(ValueError):
                runtime.cline_members({key: body for key, body in bodies.items() if key != missing}, self.source)
        for empty in ("native-acp.jsonl", "probe-events.jsonl", "provider-1.request.json", "provider-2.response.log"):
            with self.subTest(empty=empty), self.assertRaises(ValueError):
                runtime.cline_members({**bodies, empty: b"\n"}, self.source)
        with self.assertRaises(ValueError):
            runtime.cline_members({**bodies, "native-acp.error.log": b"failed"}, self.source)

    def test_cline_complete_attempt_cannot_borrow_failed_or_foreign_receipts(self):
        bodies = self.cline_complete_bodies()
        result = json.loads(bodies["runtime-attempt.json"])
        for change in ({"test_failed": True}, {"runtime_attempt_complete": False}, {"runtime_attempt_complete": 1},
                       {"native_exchange_observed": False}, {"attempt_stage": "unobserved"}, {"native_session_id": "foreign"},
                       {"vendor_authentication_qualified": True}, {"real_model_qualified": True},
                       {"qualification": {key: True for key in runtime.shared.QUALIFICATION_CASES}}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                runtime.cline_members({**bodies, "runtime-attempt.json": json.dumps({**result, **change}).encode()}, self.source)
        for member, changes in (("provider.json", ({"requests": 1}, {"responses": 1}, {"exact_call_issued": False},
                                                  {"single_call_approved": False}, {"error": True}, {"error_text": "failed"})),
                                ("native-acp.command.json", ({"error": True}, {"auto_approve": True}, {"injected_mcp_servers": [{}]})),
                                ("add-a.command.json", ({"exit": 1}, {"error": True}, {"deadline": True}))):
            receipt = json.loads(bodies[member])
            for change in changes:
                with self.subTest(member=member, change=change), self.assertRaises(ValueError):
                    runtime.cline_members({**bodies, member: json.dumps({**receipt, **change}).encode()}, self.source)

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
            self.assertEqual(argv[-5:], [str(test), "-test.run=^TestAgentpluginsClineNativeProjectCompleteCycle$", "-test.count=1", "-test.v", "-test.timeout=180s"])
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

    def kimi_synthetic_completed_capture(self, probe_pid=17):
        """Independent inert protocol records, never a measured native capture."""
        encode = lambda value: json.dumps(value, separators=(",", ":")).encode()
        digest = lambda body: hashlib.sha256(body).hexdigest()
        nonce, root = "c" * 32, "/inert/kimi-python-contract"
        project, physical = root + "/project", "uap-kimi-native-a-20242674556d"
        profile = project + "/.kimi-code"
        active, data = project + "/.uap/managed/clients/kimi/" + physical, project + "/.uap/plugin-data/" + physical
        client, session = "/inert/never-executed/kimi", "session_11111111-2222-4333-8444-555555555555"
        call, approval, prompt_id = "uap_" + nonce + "_attempt", "approval_inert-owned", "msg_inert_prompt"
        stamp = "2026-01-02T03:04:07Z"
        prompt = "Call the one installed fixture tool once with the exact nonce and marker; then report completion. " + nonce + "-a"
        sentinel, marker = "UAP_KIMI_DONE_" + nonce, "retained-" + nonce
        args = {"nonce": nonce + "-a", "operation": "write", "marker": marker}
        facts = {"nonce": nonce + "-a", "revision": "a-one", "cwd": active, "argv": ["Kimi owned a", active + "/argument"],
                 "plugin_root": active, "plugin_data": data, "literal": "", "once": "", "marker": marker, "pid": probe_pid, "session": str(probe_pid) + "-1767323048000000000"}
        server = {"transport": "stdio", "command": active + "/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins",
                  "args": ["--internal-stdio-v1", active, data, active, "plugin", "--", "./bin/probe", "Kimi owned a", active + "/argument"],
                  "env": {"PLUGIN_ROOT": active, "PLUGIN_DATA": data, "UAP_TEST_ROOT": root, "UAP_TEST_EVENTS": root + "/a-events.jsonl", "UAP_TEST_NONCE": nonce + "-a", "UAP_TEST_REVISION": "a-one"}}
        bodies = {".kimi-code-mcp.json.installed": encode({"mcpServers": {"uap-kimi-native-a": server}}),
                  "uap-kimi-native-a-SKILL.md.installed": ("---\nname: uap-kimi-native-a\ndescription: Kimi installed a-one\n---\nUAP_KIMI_a-one_" + nonce + "\n").encode()}
        for label in runtime.KIMI_CALL_COMMANDS:
            command = {"argv": ["/inert/never-executed/" + label], "cwd": project, "exit": 0, "error": False,
                       "deadline": False, "cleanup": "ordinary full-descendant supervision; forced termination fails"}
            bodies[label + ".command.json"], bodies[label + ".stdout.log"], bodies[label + ".stderr.log"] = encode(command), b"", b""
        projects_for_commands = {"a": project, "b": root + "/project-b", "c": root + "/project-c", "user": root + "/outside"}
        for scope in ("a", "b", "c"):
            command = json.loads(bodies["git-" + scope + ".command.json"])
            command.update({"argv": ["/inert/never-executed/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"], "cwd": projects_for_commands[scope]})
            bodies["git-" + scope + ".command.json"] = encode(command)
        for label, argv in (("version", [client, "--version"]),
                ("add-a", ["/inert/never-executed/agentplugins", "add", root + "/sources/a", "--no-save", "--target", "kimi", "--format", "json", "--scope", "project", "--project-root", project]),
                ("native-attempt", ["/inert/never-executed/python", "/inert/checked-source/scripts/project_native_collectors/kimi_attempt.py", "--attempt-request", root + "/attempt-request.json"])):
            command = json.loads(bodies[label + ".command.json"])
            command.update({"argv": argv, "cwd": project if label == "add-a" else projects_for_commands["user"]})
            bodies[label + ".command.json"] = encode(command)
        bodies["add-a.stdout.log"] = encode({"schema_version": 1, "command": "add", "result": "success", "data": {"targets": [{"target": "kimi", "output": {"result": {"installation_id": "inert-python-installation", "mutated": True}}}]}})
        bodies["version.stdout.log"] = b"2.3.2\n"
        bodies["probe-events.jsonl"] = b"".join(encode({"method": method, "pid": probe_pid, "session": facts["session"], "nonce": nonce + "-a",
             "timestamp": f"2026-01-02T03:04:08.00{index}Z", "facts": facts if method == "tools/call" else None}) + b"\n"
             for index, method in enumerate(("initialize", "tools/list", "tools/call")))
        declaration = {"type": "function", "function": {"name": "mcp__uap-kimi-native-a__inspect_runtime", "description": "Inspect only disposable fixture runtime facts.",
             "parameters": {"type": "object", "properties": {key: {"type": "string"} for key in ("nonce", "operation", "marker")}, "required": ["nonce"], "additionalProperties": False}}}
        first = {"model": "uap-kimi-native-model", "messages": [{"role": "user", "content": prompt}], "tools": [declaration], "stream": True,
                 "stream_options": {"include_usage": True}, "prompt_cache_key": session}
        second = {**first, "messages": first["messages"] + [{"role": "assistant", "content": None, "tool_calls": [{"id": call, "type": "function", "function": {"name": declaration["function"]["name"], "arguments": encode(args).decode()}}]},
                 {"role": "tool", "tool_call_id": call, "content": "Wall time: 0.004 seconds\n" + encode(facts).decode()}]}
        bodies["provider-request-1.json"], bodies["provider-request-2.json"] = encode(first), encode(second)
        messages = [{"id": prompt_id, "session_id": session, "role": "user", "content": [{"type": "text", "text": prompt}], "created_at": stamp, "metadata": {"origin": {"kind": "user"}}},
                    {"id": "msg_inert_call", "session_id": session, "role": "assistant", "content": [{"type": "tool_use", "tool_call_id": call, "tool_name": declaration["function"]["name"], "input": args}], "created_at": stamp},
                    {"id": "msg_inert_result", "session_id": session, "role": "tool", "content": [{"type": "tool_result", "tool_call_id": call, "output": encode(facts).decode()}], "created_at": "2026-01-02T03:04:08.003Z"},
                    {"id": "msg_inert_final", "session_id": session, "role": "assistant", "content": [{"type": "text", "text": sentinel}], "created_at": "2026-01-02T03:04:08.004Z"}]
        state = {"id": session, "workspace_id": "wd_project_111111111111", "metadata": {"cwd": project}, "agent_config": {"model": "uap-kimi-native-model"},
                 "permission_rules": [], "last_turn_reason": "completed", "busy": False, "main_turn_active": False, "pending_interaction": "none", "archived": False}
        state.update({"title": prompt, "created_at": stamp, "updated_at": "2026-01-02T03:04:10Z", "usage": {key: 0 for key in ("input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens", "context_tokens")}, "message_count": 0, "last_seq": 0})
        raw = {"kimi-single-tool.stdout.log": "independently authored inert service stream; never executed\n", "kimi-single-tool.stderr.log": ""}
        native = {"argv": [client, "web", "--no-open", "--host", "127.0.0.1", "--port", "0"], "cwd": project, "probe": "kimi-single-tool", "elapsed_seconds": 0.01, "error": None, "exit": 0,
                  "cleanup": {"forced_kill": False, "group_reaped": True, "port_closed": True, "shutdown_acknowledged": True, "sigterm_sent": False}}
        for channel in ("stdout", "stderr"):
            name = "kimi-single-tool." + channel + ".log"
            native.update({channel + "_file": name, channel + "_bytes": len(raw[name].encode()), channel + "_sha256": digest(raw[name].encode())})
        receipts = []
        def response(label, method, path, value, auth=True, status=200, code=0):
            body = encode({"code": code, "msg": "success" if code == 0 else "inert unauthorized", "data": value, "request_id": "inert-request-" + label})
            name = "kimi-single-tool-" + label + ".http.json"
            raw[name] = body.decode()
            receipts.append({"authenticated": auth, "error": None, "label": label, "method": method, "path": path, "response_bytes": len(body), "response_file": name, "response_sha256": digest(body), "status": status})
        response("health", "GET", "/api/v1/healthz", {"ok": True}, False)
        response("unauthorized", "GET", "/api/v1/meta", None, False, 401, 40101)
        response("meta", "GET", "/api/v1/meta", {"server_version": "2.3.2", "backend": "v2", "dangerous_bypass_auth": False, "capabilities": {key: True for key in ("websocket", "file_upload", "fs_query", "mcp", "tasks", "terminal")}, "server_id": "inert-server", "started_at": stamp, "open_in_apps": []})
        projects = {"a": project, "b": root + "/project-b", "c": root + "/project-c", "user": root + "/outside"}
        workspaces = {"a": "wd_project_111111111111", "b": "wd_project-b_222222222222", "c": "wd_project-c_333333333333"}
        for scope in ("a", "b", "c"):
            response("workspace-" + scope, "POST", "/api/v1/workspaces", {"id": workspaces[scope], "root": projects[scope], "name": "uap-kimi-native-" + scope, "created_at": stamp, "last_opened_at": stamp, "session_count": 0})
            response("trust-before-" + scope, "GET", "/api/v1/workspaces/" + workspaces[scope] + "/trust", {"trusted": False})
        for scope in ("a", "b"):
            response("trust-set-" + scope, "POST", "/api/v1/workspaces/" + workspaces[scope] + "/trust", {"trusted": True})
            response("trust-after-" + scope, "GET", "/api/v1/workspaces/" + workspaces[scope] + "/trust", {"trusted": True})
        response("trust-untrusted-c", "GET", "/api/v1/workspaces/" + workspaces["c"] + "/trust", {"trusted": False})
        projected = {key: value for key, value in server.items() if key != "env"}; projected["envKeys"] = sorted(server["env"])
        for scope in ("a", "b", "c", "user"):
            catalog = [{"name": "uap-kimi-native-a", "config": projected, "source": "global", "origin": profile + "/mcp.json", "mutable": False}] if scope == "a" else []
            response("catalog-" + scope, "GET", "/api/v2/mcp/servers?" + runtime.urlencode({"cwd": projects[scope]}), catalog)
        response("skills-a", "GET", "/api/v1/workspaces/" + workspaces["a"] + "/skills", {"skills": [{"name": "uap-kimi-native-a", "description": "Kimi installed a-one", "path": profile + "/skills/uap-kimi-native-a/SKILL.md", "source": "project"}]})
        response("create-session", "POST", "/api/v1/sessions", {**{key: value for key, value in state.items() if key != "last_turn_reason"}, "agent_config": {"model": ""}, "title": ""})
        base = "/api/v1/sessions/" + session
        response("prompt", "POST", base + "/prompts", {"prompt_id": prompt_id, "user_message_id": prompt_id, "status": "running", "content": [{"type": "text", "text": prompt}], "created_at": stamp})
        action = "Approve " + declaration["function"]["name"]
        response("approvals-0", "GET", base + "/approvals?status=pending", {"items": [{"approval_id": approval, "session_id": session, "agent_id": "main", "turn_id": 0, "tool_call_id": call, "tool_name": declaration["function"]["name"], "action": action, "tool_input_display": {"kind": "generic", "summary": action, "detail": args}, "created_at": stamp, "expires_at": "2026-01-03T03:04:07Z"}]})
        response("approve-once", "POST", base + "/approvals/" + approval, {"resolved": True, "resolved_at": stamp})
        response("snapshot-0", "GET", base + "/snapshot", {"as_of_seq": 1, "epoch": "inert-epoch", "session": state, "messages": {"items": messages, "has_more": False}, "in_flight_turn": None, "pending_approvals": [], "pending_questions": [], "subagents": []})
        response("history", "GET", base + "/messages?page_size=20", {"items": messages[::-1], "has_more": False})
        response("shutdown", "POST", "/api/v1/shutdown", {"ok": True})
        raw["research.json"] = encode({"schema_version": 1, "client": "kimi", "phase": "installed-single-tool-attempt", "commands": [native], "http_requests": receipts,
              "qualification": dict.fromkeys(runtime.shared.QUALIFICATION_CASES, False), "observations": {"native_capture_complete": False, "native_history_grammar_frozen": False,
              "native_session_id": session, "native_turn_id": 0, "provider_call_id": call, "approval_id": approval, "snapshot_count": 1, "api_request_count": len(receipts), "boundary": "Bounded initial capture; no native history or qualification inferred from provider/probe success"}}).decode()
        bodies["python-api.json"] = bodies["native-attempt.stdout.log"] = encode({"schema_version": 1, "files": raw})
        inventory = {".": "020000000700:directory", ".agentplugins-data-owner.json": "0600:" + "e" * 64}
        bodies["native-data-proof.json"] = encode({"schema_version": 1, "before": inventory, "after_without_marker": inventory, "marker_body": marker, "marker_bytes": len(marker), "marker_sha256": digest(marker.encode())})
        proof = {"native_session_id": session, "native_turn_id": 0, "provider_call_id": call, "approval_id": approval, "prompt_id": prompt_id, "terminal_reason": "completed",
                 "message_count": 4, "snapshot_count": 1, "history_sha256": digest(raw["kimi-single-tool-history.http.json"].encode()), "terminal_snapshot_sha256": digest(raw["kimi-single-tool-snapshot-0.http.json"].encode()),
                 "probe_pid": probe_pid, "probe_session": facts["session"], "started_at": "2026-01-02T03:04:05Z", "ended_at": "2026-01-02T03:04:11Z", "data_proof_sha256": digest(bodies["native-data-proof.json"])}
        result = {**self.cli_result("qoder"), "client": "kimi", "version": "2.3.2", "phase": "installed-single-tool-attempt", "provider_kind": "scripted-openai-chat-completions", "elapsed_ms": 8000,
                  "runtime_attempt_complete": True, "test_failed": False, "native_capture_complete": True, "native_history_grammar_frozen": True, "provider_returned": True,
                  "provider_call_id": call, "native_session_id": session, "native_turn_id": 0, "probe_facts": facts, "native_history": proof,
                  "binding": {"client_binding_id": "client_6e8a61a9620b87be9881f1ef", "data_receipt_id": "data_9797d862dba4e5336eb02e08", "client_id": "kimi", "scope": "project", "activation": "prepared", "authentication": "not_checked", "verification": "installation_verified", "materialization": "materialized", "target_locator": active, "native_profile_root": profile, "physical_artifact_id": physical, "policy": "allowed", "updated_at": stamp,
                              "package_revision": {"version": "1.0.0", "tree_digest": "sha256:" + "a" * 64, "manifest_digest": "sha256:" + "b" * 64}, "affected_surfaces": ["kimi"],
                              "native_objects": [{"object_id": "package:kimi:" + physical, "kind": "managed_package_directory", "path": active, "protection_class": "managed"}],
                              "receipts": [{"operation_id": "inert-operation", "sequence": 1, "mutation_type": "directory_swap", "client_binding_id": "client_6e8a61a9620b87be9881f1ef", "phase": "committed"}]},
                  "data_locator": data, "provider_cleanup": {"shutdown_complete": True, "forced_close": False, "error": False, "requests": 2, "issued": True, "returned": True},
                  "qualification": {key: key in ("mcp-initialize", "mcp-tools-list", "mcp-tools-call") for key in runtime.shared.QUALIFICATION_CASES}}
        bodies["runtime-attempt.json"] = encode(result)
        return bodies

    def kimi_mutate_raw_coherently(self, bodies, mutate):
        envelope = json.loads(bodies["python-api.json"])
        raw = envelope["files"]
        mutate(raw)
        research = json.loads(raw["research.json"])
        for receipt in research["http_requests"]:
            name = receipt["response_file"]
            if name in raw:
                body = raw[name].encode()
                receipt["response_bytes"] = len(body)
                receipt["response_sha256"] = hashlib.sha256(body).hexdigest()
        raw["research.json"] = json.dumps(research)
        bodies["python-api.json"] = bodies["native-attempt.stdout.log"] = json.dumps(envelope).encode()
        result = json.loads(bodies["runtime-attempt.json"])
        proof = result["native_history"]
        proof["history_sha256"] = hashlib.sha256(raw["kimi-single-tool-history.http.json"].encode()).hexdigest()
        name = "kimi-single-tool-snapshot-" + str(proof["snapshot_count"] - 1) + ".http.json"
        proof["terminal_snapshot_sha256"] = hashlib.sha256(raw[name].encode()).hexdigest()
        bodies["runtime-attempt.json"] = json.dumps(result).encode()

    def kimi_mutate_http_coherently(self, bodies, label, mutate):
        def update(raw):
            name = "kimi-single-tool-" + label + ".http.json"
            response = json.loads(raw[name])
            mutate(response)
            raw[name] = json.dumps(response)
        self.kimi_mutate_raw_coherently(bodies, update)

    def test_kimi_synthetic_complete_reader_recomputes_raw_proof(self):
        bodies = self.kimi_synthetic_completed_capture()
        result = runtime.kimi_call_members(bodies, self.source)
        self.assertTrue(result["runtime_attempt_complete"])
        self.assertFalse(result["qualification"]["project-lifecycle"])
        self.assertFalse(result["qualification"]["native-restart-removal"])
        for name in bodies:
            with self.subTest(missing=name), self.assertRaises(ValueError):
                runtime.kimi_call_members({key: value for key, value in bodies.items() if key != name}, self.source)

    def test_kimi_public_receipt_and_derived_ids_are_independent_authority(self):
        cases = ("empty", "failed", "schema", "target", "missing-result", "not-mutated", "installation", "physical", "client", "data")
        for kind in cases:
            with self.subTest(kind=kind), self.assertRaisesRegex(ValueError, "public|JSON byte bound"):
                bodies = self.kimi_synthetic_completed_capture()
                result = json.loads(bodies["runtime-attempt.json"])
                public = json.loads(bodies["add-a.stdout.log"])
                target = public["data"]["targets"][0]
                if kind == "empty": bodies["add-a.stdout.log"] = b""
                elif kind == "failed": public["result"] = "failure"
                elif kind == "schema": public["schema_version"] = True
                elif kind == "target": target["target"] = "foreign"
                elif kind == "missing-result": del target["output"]["result"]
                elif kind == "not-mutated": target["output"]["result"]["mutated"] = False
                elif kind == "installation": target["output"]["result"]["installation_id"] = "foreign-installation"
                elif kind == "physical": result["binding"]["physical_artifact_id"] = "uap-kimi-native-a-" + "f" * 12
                elif kind == "client": result["binding"]["client_binding_id"] = "client_" + "f" * 24
                elif kind == "data": result["binding"]["data_receipt_id"] = "data_" + "f" * 24
                if kind != "empty": bodies["add-a.stdout.log"] = json.dumps(public).encode()
                bodies["runtime-attempt.json"] = json.dumps(result).encode()
                runtime.kimi_call_members(bodies, self.source)

    def test_kimi_native_mandatory_data_semantics_survive_recomputed_hashes(self):
        cases = (("health", "ok", "health/shutdown"), ("shutdown", "ok", "health/shutdown"),
                 ("meta", "version", "pinned native metadata"), ("meta", "backend", "pinned native metadata"),
                 ("meta", "auth", "pinned native metadata"), ("meta", "missing", "raw object"),
                 ("meta", "unknown", "raw object"), ("skills-a", "empty", "selected native skill"),
                 ("skills-a", "foreign", "selected native skill"), ("skills-a", "path", "selected native skill"),
                 ("skills-a", "source", "selected native skill"), ("skills-a", "duplicate", "selected native skill"))
        for label, kind, reason in cases:
            with self.subTest(label=label, kind=kind), self.assertRaisesRegex(ValueError, reason):
                bodies = self.kimi_synthetic_completed_capture()
                def mutate(response):
                    data = response["data"]
                    if kind == "ok": data["ok"] = False
                    elif kind == "version": data["server_version"] = "future"
                    elif kind == "backend": data["backend"] = "foreign"
                    elif kind == "auth": data["dangerous_bypass_auth"] = True
                    elif kind == "missing": del data["server_id"]
                    elif kind == "unknown": data["unknown"] = True
                    elif kind == "empty": data["skills"] = []
                    elif kind == "foreign": data["skills"][0]["name"] = "uap-kimi-native-foreign"
                    elif kind == "path": data["skills"][0]["path"] = "/foreign/SKILL.md"
                    elif kind == "source": data["skills"][0]["source"] = "extra"
                    elif kind == "duplicate": data["skills"].append(data["skills"][0])
                self.kimi_mutate_http_coherently(bodies, label, mutate)
                runtime.kimi_call_members(bodies, self.source)

    def test_kimi_native_command_and_http_schemas_are_closed_typed_and_bounded(self):
        for kind in ("research-version", "research-bool", "command-extra", "command-missing", "probe", "elapsed-bool", "elapsed-high", "elapsed-negative"):
            with self.subTest(kind=kind), self.assertRaisesRegex(ValueError, "capture boundary|raw object|command schema"):
                bodies = self.kimi_synthetic_completed_capture()
                def mutate(raw):
                    research = json.loads(raw["research.json"])
                    command = research["commands"][0]
                    if kind == "research-version": research["schema_version"] = 2
                    elif kind == "research-bool": research["schema_version"] = True
                    elif kind == "command-extra": command["unknown"] = True
                    elif kind == "command-missing": del command["elapsed_seconds"]
                    elif kind == "probe": command["probe"] = "foreign"
                    elif kind == "elapsed-bool": command["elapsed_seconds"] = True
                    elif kind == "elapsed-high": command["elapsed_seconds"] = 60.001
                    elif kind == "elapsed-negative": command["elapsed_seconds"] = -0.001
                    raw["research.json"] = json.dumps(research)
                self.kimi_mutate_raw_coherently(bodies, mutate)
                runtime.kimi_call_members(bodies, self.source)
        for label, kind in (("health", "request-empty"), ("health", "request-type"), ("health", "unknown"),
                            ("unauthorized", "data"), ("unauthorized", "msg"), ("unauthorized", "stack")):
            with self.subTest(label=label, kind=kind), self.assertRaisesRegex(ValueError, "response identity/data|raw object|negative stack"):
                bodies = self.kimi_synthetic_completed_capture()
                def mutate(response):
                    if kind == "request-empty": response["request_id"] = ""
                    elif kind == "request-type": response["request_id"] = True
                    elif kind == "unknown": response["unknown"] = True
                    elif kind == "data": response["data"] = {"unobserved": True}
                    elif kind == "msg": response["msg"] = {"unobserved": True}
                    elif kind == "stack": response["stack"] = ["unobserved"]
                self.kimi_mutate_http_coherently(bodies, label, mutate)
                runtime.kimi_call_members(bodies, self.source)

    def test_kimi_provider_nested_grammar_is_closed_before_prefix_comparison(self):
        kinds = ("message-extra", "message-missing", "role", "content-null", "content-empty", "content-count", "content-part-extra",
                 "content-nul", "message-count", "call-extra", "function-extra", "function-missing", "tool-result-id")
        for kind in kinds:
            with self.subTest(kind=kind), self.assertRaisesRegex(ValueError, "raw object|provider message|provider null|provider content|provider result"):
                bodies = self.kimi_synthetic_completed_capture()
                first, second = (json.loads(bodies[f"provider-request-{number}.json"]) for number in (1, 2))
                if kind.startswith("call-") or kind.startswith("function-"):
                    call = second["messages"][-2]["tool_calls"][0]
                    if kind == "call-extra": call["unknown"] = True
                    elif kind == "function-extra": call["function"]["unknown"] = True
                    elif kind == "function-missing": del call["function"]["arguments"]
                elif kind == "tool-result-id": del second["messages"][-1]["tool_call_id"]
                else:
                    if kind == "message-extra": first["messages"][0]["unknown"] = True
                    elif kind == "message-missing": del first["messages"][0]["content"]
                    elif kind == "role": first["messages"][0]["role"] = "foreign"
                    elif kind == "content-null": first["messages"][0]["content"] = None
                    elif kind == "content-empty": first["messages"][0]["content"] = []
                    elif kind == "content-count": first["messages"][0]["content"] = [{"type": "text", "text": "inert"}] * 129
                    elif kind == "content-part-extra": first["messages"][0]["content"] = [{"type": "text", "text": "inert", "unknown": True}]
                    elif kind == "content-nul": first["messages"][0]["content"] = "inert\x00"
                    elif kind == "message-count": first["messages"] *= 257
                    # Preserve the same malformed prefix in request2. Rejection
                    # must come from closed grammar, not an easy unequal prefix.
                    second["messages"] = first["messages"] + second["messages"][-2:]
                bodies["provider-request-1.json"], bodies["provider-request-2.json"] = json.dumps(first).encode(), json.dumps(second).encode()
                runtime.kimi_call_members(bodies, self.source)

    def test_kimi_prompt_identity_and_absent_response_metadata_are_source_bound(self):
        bodies = self.kimi_synthetic_completed_capture()
        prompt = json.loads(bodies["runtime-attempt.json"])["native_history"]["prompt_id"]
        self.kimi_mutate_http_coherently(bodies, "snapshot-0", lambda response: response["data"]["session"].update(current_prompt_id=prompt))
        self.assertTrue(runtime.kimi_call_members(bodies, self.source)["runtime_attempt_complete"])
        for label, value in (("snapshot-0", "foreign"), ("snapshot-0", None), ("create-session", prompt), ("create-session", None)):
            with self.subTest(label=label, value=value), self.assertRaisesRegex(ValueError, "current prompt"):
                bodies = self.kimi_synthetic_completed_capture()
                self.kimi_mutate_http_coherently(bodies, label, lambda response: response["data"].update(current_prompt_id=value) if label == "create-session" else response["data"]["session"].update(current_prompt_id=value))
                runtime.kimi_call_members(bodies, self.source)
        for metadata in (None, {}, {"origin": {"kind": "user"}}, {"origin": {"kind": "injection"}}):
            with self.subTest(metadata=metadata), self.assertRaisesRegex(ValueError, "raw object"):
                bodies = self.kimi_synthetic_completed_capture()
                self.kimi_mutate_http_coherently(bodies, "prompt", lambda response: response["data"].update(metadata=metadata))
                runtime.kimi_call_members(bodies, self.source)

    def test_kimi_success_bounds_binding_and_boolean_types_are_semantic(self):
        for kind in ("elapsed-high", "sequence-high", "sequence-negative", "profile-leaf", "profile-relative", "profile-parent", "profile-double-root",
                     "binding-unknown", "binding-authority", "binding-missing", "binding-scalar", "revision-extra", "revision-type", "surfaces-type",
                     "object-extra", "object-type", "object-modified", "receipt-extra", "receipt-type", "receipt-sequence"):
            reason = "native turn/time|snapshot authority|canonical native profile|raw object|binding"
            with self.subTest(kind=kind), self.assertRaisesRegex(ValueError, reason):
                bodies = self.kimi_synthetic_completed_capture()
                if kind.startswith("sequence-"):
                    self.kimi_mutate_http_coherently(bodies, "snapshot-0", lambda response: response["data"].update(as_of_seq=(1 << 40) + 1 if kind == "sequence-high" else -1))
                else:
                    result = json.loads(bodies["runtime-attempt.json"])
                    binding = result["binding"]
                    if kind == "elapsed-high": result["elapsed_ms"] = 180001
                    elif kind == "profile-leaf": binding["native_profile_root"] = "/inert/kimi-python-contract/project/.foreign"
                    elif kind == "profile-relative": binding["native_profile_root"] = "relative/.kimi-code"
                    elif kind == "profile-parent": binding["native_profile_root"] = "/inert/kimi-python-contract/project/../project/.kimi-code"
                    elif kind == "profile-double-root": binding["native_profile_root"] = "/" + binding["native_profile_root"]
                    elif kind == "binding-unknown": binding["unknown"] = True
                    elif kind == "binding-authority": binding["profile_authority"] = {}
                    elif kind == "binding-missing": del binding["policy"]
                    elif kind == "binding-scalar": binding["updated_at"] = None
                    elif kind == "revision-extra": binding["package_revision"]["unknown"] = True
                    elif kind == "revision-type": binding["package_revision"]["version"] = 1
                    elif kind == "surfaces-type": binding["affected_surfaces"] = [False]
                    elif kind == "object-extra": binding["native_objects"][0]["unknown"] = True
                    elif kind == "object-type": binding["native_objects"][0]["path"] = None
                    elif kind == "object-modified": binding["native_objects"][0]["user_modified"] = 0
                    elif kind == "receipt-extra": binding["receipts"][0]["unknown"] = True
                    elif kind == "receipt-type": binding["receipts"][0]["phase"] = False
                    elif kind == "receipt-sequence": binding["receipts"][0]["sequence"] = True
                    bodies["runtime-attempt.json"] = json.dumps(result).encode()
                runtime.kimi_call_members(bodies, self.source)
        labels = ("trust-before-a", "trust-before-b", "trust-before-c", "trust-set-a", "trust-after-a", "trust-set-b", "trust-after-b", "trust-untrusted-c", "catalog-a")
        for label in labels:
            with self.subTest(label=label), self.assertRaisesRegex(ValueError, "trust differs|catalog differs"):
                bodies = self.kimi_synthetic_completed_capture()
                def mutate(response):
                    if label == "catalog-a": response["data"][0]["mutable"] = 0
                    else: response["data"]["trusted"] = int(response["data"]["trusted"])
                self.kimi_mutate_http_coherently(bodies, label, mutate)
                runtime.kimi_call_members(bodies, self.source)
    def test_kimi_success_json_numbers_and_keys_match_go_byte_reader(self):
        for kind in ("integer-overflow", "negative-integer-overflow", "key-utf8"):
            with self.subTest(kind=kind), self.assertRaisesRegex(ValueError, "finite float64 number|JSON key bound"):
                bodies = self.kimi_synthetic_completed_capture()
                def mutate(response):
                    key = "é" * 2049 if kind == "key-utf8" else "inert-number"
                    response["data"]["capabilities"][key] = (-(10 ** 400) if kind == "negative-integer-overflow" else 10 ** 400) if kind != "key-utf8" else True
                self.kimi_mutate_http_coherently(bodies, "meta", mutate)
                runtime.kimi_call_members(bodies, self.source)

        bodies = self.kimi_synthetic_completed_capture()
        self.kimi_mutate_http_coherently(bodies, "meta", lambda response: response["data"]["capabilities"].update({"é" * 2048: 10 ** 308}))
        self.assertTrue(runtime.kimi_call_members(bodies, self.source)["runtime_attempt_complete"])

    def test_kimi_native_identifier_and_epoch_limits_are_utf8_bytes(self):
        cases = (("message", "msg_\x00owned", False), ("message", "é" * 200, False), ("message", "é" * 128, True),
                 ("epoch", "é" * 200, False), ("epoch", "é" * 128, True),
                 ("workspace", "wd_" + "a" * 250 + "_" + "1" * 12, False), ("workspace", "wd_" + "a" * 240 + "_" + "1" * 12, True))
        for kind, replacement, accepted in cases:
            with self.subTest(kind=kind, utf8_bytes=len(replacement.encode())):
                bodies = self.kimi_synthetic_completed_capture()
                def mutate(raw):
                    if kind == "epoch":
                        response = json.loads(raw["kimi-single-tool-snapshot-0.http.json"])
                        response["data"]["epoch"] = replacement
                        raw["kimi-single-tool-snapshot-0.http.json"] = json.dumps(response)
                    else:
                        old = "msg_inert_call" if kind == "message" else "wd_project_111111111111"
                        before, after = (json.dumps(old), json.dumps(replacement)) if kind == "message" else (old, replacement)
                        for name in raw: raw[name] = raw[name].replace(before, after)
                self.kimi_mutate_raw_coherently(bodies, mutate)
                if accepted:
                    self.assertTrue(runtime.kimi_call_members(bodies, self.source)["runtime_attempt_complete"])
                else:
                    with self.assertRaisesRegex(ValueError, "message identity|snapshot authority|workspace/trust"):
                        runtime.kimi_call_members(bodies, self.source)

    def test_kimi_success_outer_commands_have_go_text_and_arity_bounds(self):
        for kind in ("argv-nul", "argv-ascii-long", "argv-utf8-long", "argv-empty", "argc", "cwd-nul", "cwd-ascii-long", "cwd-utf8-long", "cwd-empty"):
            with self.subTest(kind=kind), self.assertRaisesRegex(ValueError, "command text/arity"):
                bodies = self.kimi_synthetic_completed_capture()
                command = json.loads(bodies["git-a.command.json"])
                if kind == "argc": command["argv"] += ["inert"] * 28
                else:
                    field = "cwd" if kind.startswith("cwd") else "argv"
                    value = "/inert/\x00git" if kind.endswith("nul") else "/" + "a" * 4096 if kind.endswith("ascii-long") else "/" + "é" * 2048 if kind.endswith("utf8-long") else ""
                    if field == "cwd": command[field] = value
                    else: command[field][0] = value
                bodies["git-a.command.json"] = json.dumps(command).encode()
                runtime.kimi_call_members(bodies, self.source)
        bodies = self.kimi_synthetic_completed_capture()
        command = json.loads(bodies["git-a.command.json"])
        command["argv"][0] = "/" + "a" * 4095
        bodies["git-a.command.json"] = json.dumps(command).encode()
        self.assertTrue(runtime.kimi_call_members(bodies, self.source)["runtime_attempt_complete"])

    def test_kimi_native_proof_integers_refuse_float_and_boolean_equality(self):
        self.assertTrue(runtime.kimi_call_members(self.kimi_synthetic_completed_capture(probe_pid=1), self.source)["runtime_attempt_complete"])
        for field, value, pid in (("message_count", 4.0, 17), ("message_count", True, 17), ("message_count", False, 17),
                                  ("probe_pid", 17.0, 17), ("probe_pid", True, 1), ("probe_pid", False, 1)):
            with self.subTest(field=field, value=value, pid=pid), self.assertRaisesRegex(ValueError, "proof integer types/bounds"):
                bodies = self.kimi_synthetic_completed_capture(probe_pid=pid)
                result = json.loads(bodies["runtime-attempt.json"])
                result["native_history"][field] = value
                bodies["runtime-attempt.json"] = json.dumps(result).encode()
                runtime.kimi_call_members(bodies, self.source)

    def test_kimi_owned_inventory_nul_and_utf8_paths_refuse_coherent_proof(self):
        for path in ("owned\x00name", "é" * 2049):
            with self.subTest(utf8_bytes=len(path.encode())), self.assertRaisesRegex(ValueError, "owned inventory entry|JSON key bound"):
                bodies = self.kimi_synthetic_completed_capture()
                data = json.loads(bodies["native-data-proof.json"])
                for key in ("before", "after_without_marker"): data[key][path] = "0600:" + "a" * 64
                bodies["native-data-proof.json"] = json.dumps(data).encode()
                result = json.loads(bodies["runtime-attempt.json"])
                result["native_history"]["data_proof_sha256"] = hashlib.sha256(bodies["native-data-proof.json"]).hexdigest()
                bodies["runtime-attempt.json"] = json.dumps(result).encode()
                runtime.kimi_call_members(bodies, self.source)

    def test_kimi_synthetic_success_cannot_cover_failed_or_foreign_raw(self):
        for kind in ("terminal-failed", "empty-sentinel", "wrong-call", "extra-message", "history-more", "forced", "changed-data", "wrong-marker", "wrong-proof", "foreign-source", "lifecycle-flag", "wrong-cache", "unknown-file"):
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                bodies = self.kimi_synthetic_completed_capture()
                result = json.loads(bodies["runtime-attempt.json"])
                if kind in ("terminal-failed", "empty-sentinel", "wrong-call", "extra-message", "history-more"):
                    envelope = json.loads(bodies["python-api.json"])
                    raw = envelope["files"]
                    snapshot = json.loads(raw["kimi-single-tool-snapshot-0.http.json"])
                    if kind == "terminal-failed": snapshot["data"]["session"]["last_turn_reason"] = "failed"
                    elif kind == "empty-sentinel": snapshot["data"]["messages"]["items"][-1]["content"] = []
                    elif kind == "wrong-call": snapshot["data"]["messages"]["items"][1]["content"][0]["tool_call_id"] = "foreign"
                    elif kind == "extra-message": snapshot["data"]["messages"]["items"].append(snapshot["data"]["messages"]["items"][0])
                    elif kind == "history-more": snapshot["data"]["messages"]["has_more"] = True
                    raw["kimi-single-tool-snapshot-0.http.json"] = json.dumps(snapshot)
                    research = json.loads(raw["research.json"])
                    for receipt in research["http_requests"]:
                        body = raw[receipt["response_file"]].encode(); receipt["response_bytes"] = len(body); receipt["response_sha256"] = hashlib.sha256(body).hexdigest()
                    raw["research.json"] = json.dumps(research)
                    bodies["python-api.json"] = bodies["native-attempt.stdout.log"] = json.dumps(envelope).encode()
                elif kind == "forced": result["provider_cleanup"]["forced_close"] = True
                elif kind in ("changed-data", "wrong-marker"):
                    data = json.loads(bodies["native-data-proof.json"])
                    if kind == "changed-data": data["after_without_marker"]["foreign"] = "0600:" + "e" * 64
                    else: data["marker_body"] = "foreign"
                    bodies["native-data-proof.json"] = json.dumps(data).encode()
                elif kind == "wrong-proof": result["native_history"]["history_sha256"] = "foreign"
                elif kind == "foreign-source": result["source_identity"]["installer_tree"] = "f" * 40
                elif kind == "lifecycle-flag": result["qualification"]["project-lifecycle"] = True
                elif kind == "wrong-cache":
                    request = json.loads(bodies["provider-request-2.json"]); request["prompt_cache_key"] = "foreign"; bodies["provider-request-2.json"] = json.dumps(request).encode()
                elif kind == "unknown-file": bodies["foreign.json"] = b"{}"
                bodies["runtime-attempt.json"] = json.dumps(result).encode()
                runtime.kimi_call_members(bodies, self.source)


def load_tests(loader, tests, pattern):
    # The existing runtime workflow deliberately selects this exact module.
    # Broad research discovery already selects the new client-specific module.
    if pattern in (None, "test_project_native_runtime.py"):
        tests.addTests(loader.loadTestsFromName("test_project_native_cline_cycle.CompleteCycleTests"))
        tests.addTests(loader.loadTestsFromName("test_project_native_windsurf_components.WindsurfComponentTests"))
    return tests


if __name__ == "__main__":
    unittest.main()
