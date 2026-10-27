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

    def grok_provider_bodies(self):
        row = {"transport": "scripted loopback OpenAI chat-completions SSE; no real model",
               "model": "uap-grok-fixture", "started": True, "requests": 1, "responses": 1,
               "credential_matched": True, "failure": "none", "shutdown_complete": True, "forced_close": False}
        return {"runtime-attempt.json": json.dumps(self.cli_result("grok")).encode(),
                "grok-provider.json": json.dumps(row).encode(),
                "grok-provider.request.json": b'{"model":"uap-grok-fixture","messages":[{"role":"user","content":"inert"}],"stream":true,"opaque":"captured only"}',
                "grok-provider.response.sse": b"inert provider response envelope; no native qualification"}

    def test_grok_provider_closed_capture_remains_a_failed_unqualified_attempt(self):
        bodies = self.grok_provider_bodies()
        result = runtime.cli_attempt_members(bodies, self.source, "grok")
        self.assertTrue(result["test_failed"])
        self.assertFalse(result["runtime_attempt_complete"])
        self.assertFalse(any(result["qualification"].values()))
        for missing in bodies:
            with self.subTest(missing=missing), self.assertRaises(ValueError):
                runtime.cli_attempt_members({key: body for key, body in bodies.items() if key != missing}, self.source, "grok")
        for client in ("qoder", "kiro", "windsurf"):
            with self.subTest(client=client), self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, "runtime-attempt.json": json.dumps(self.cli_result(client)).encode()}, self.source, client)
        for extra in ("profile.json", "grok-provider-2.request.json", "grok-provider.headers.json"):
            with self.subTest(extra=extra), self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, extra: b"{}"}, self.source, "grok")

    def test_grok_provider_receipt_rejects_forged_counts_fields_and_cleanup(self):
        bodies = self.grok_provider_bodies()
        original = json.loads(bodies["grok-provider.json"])
        changes = ({"requests": True}, {"responses": 2}, {"requests": 0}, {"responses": 0},
                   {"started": False}, {"credential_matched": False}, {"shutdown_complete": 1},
                   {"shutdown_complete": False}, {"forced_close": True}, {"failure": "private arbitrary error"},
                   {"failure": []}, {"model": "foreign"}, {"transport": "real model"}, {"extra": "private"})
        for change in changes:
            with self.subTest(change=change), self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, "grok-provider.json": json.dumps({**original, **change}).encode()}, self.source, "grok")
        for name, body in (("grok-provider.json", b" " * 2049 + bodies["grok-provider.json"]),
                           ("grok-provider.request.json", b"[]"), ("grok-provider.request.json", b'{"x":1,"x":2}'),
                           ("grok-provider.response.sse", b"\n")):
            with self.subTest(name=name), self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, name: body}, self.source, "grok")

    def test_grok_provider_partial_failures_keep_only_their_actual_members(self):
        bodies = self.grok_provider_bodies()
        original = json.loads(bodies["grok-provider.json"])
        for requests, failure, started in ((0, "listen", False), (0, "credential", True), (1, "request-shape", True), (1, "response-write", True)):
            row = {**original, "requests": requests, "responses": 0, "failure": failure, "started": started,
                   "credential_matched": bool(requests), "shutdown_complete": started}
            present = {key: value for key, value in bodies.items() if key not in
                       ({"grok-provider.response.sse"} | ({"grok-provider.request.json"} if not requests else set()))}
            present["grok-provider.json"] = json.dumps(row).encode()
            result = runtime.cli_attempt_members(present, self.source, "grok")
            self.assertTrue(result["test_failed"])
            self.assertFalse(any(result["qualification"].values()))
            with self.assertRaises(ValueError):
                runtime.cli_attempt_members({**present, "grok-provider.response.sse": b"unrecorded response"}, self.source, "grok")

    def grok_route_bodies(self):
        bodies = self.grok_provider_bodies()
        row = json.loads(bodies["grok-provider.json"])
        row.update(requests=0, responses=0, credential_matched=False, failure="route")
        row["route_observation"] = {
            "method": "POST", "uri": "/v1/chat/completions", "peer_loopback": True, "host_matches": True,
            "relative_url": True, "empty_url_host": True, "empty_raw_path": True, "content_type_exact": False,
            "content_type_count": "one", "content_type_kind": "json-utf8"}
        del bodies["grok-provider.request.json"]
        del bodies["grok-provider.response.sse"]
        return bodies, row

    def test_grok_route_observation_is_optional_closed_and_never_qualifies(self):
        bodies, row = self.grok_route_bodies()
        for change in ({}, {"method": "GET", "content_type_exact": True, "content_type_kind": "json-bare"},
                       {"uri": "/models", "content_type_exact": True, "content_type_kind": "json-bare"},
                       {"content_type_count": "none", "content_type_kind": "missing"},
                       {"content_type_count": "multiple", "content_type_kind": "other"}):
            updated = {**row, "route_observation": {**row["route_observation"], **change}}
            result = runtime.cli_attempt_members({**bodies, "grok-provider.json": json.dumps(updated).encode()},
                                                 self.source, "grok")
            self.assertFalse(any(result["qualification"].values()))
        del row["route_observation"]
        runtime.cli_attempt_members({**bodies, "grok-provider.json": json.dumps(row).encode()}, self.source, "grok")

    def test_grok_route_observation_refuses_forged_types_enums_fields_and_contradictions(self):
        bodies, row = self.grok_route_bodies()
        observation = row["route_observation"]
        for change in ({"method": []}, {"method": "PUT"}, {"uri": "/private-query?token=private"},
                       {"peer_loopback": 1}, {"content_type_count": 1}, {"content_type_kind": None},
                       {"extra": "private-value"}, {"content_type_count": "none"},
                       {"content_type_kind": "missing"}, {"content_type_count": "multiple"},
                       {"content_type_exact": True}, {"content_type_exact": True, "content_type_kind": "json-bare"}):
            updated = {**row, "route_observation": {**observation, **change}}
            with self.subTest(change=change), self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, "grok-provider.json": json.dumps(updated).encode()},
                                           self.source, "grok")
        for key in observation:
            incomplete = {name: value for name, value in observation.items() if name != key}
            with self.subTest(missing=key), self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, "grok-provider.json": json.dumps({**row, "route_observation": incomplete}).encode()},
                                           self.source, "grok")
        for change in ({"failure": "serve"}, {"route_observation": None}, {"route_observation": []},
                       {"started": False, "failure": "listen", "shutdown_complete": False}):
            with self.subTest(row=change), self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, "grok-provider.json": json.dumps({**row, **change}).encode()},
                                           self.source, "grok")

    def test_grok_origin_route_is_only_failed_evidence_and_never_normalizes_uris(self):
        bodies, row = self.grok_route_bodies()
        observation = {**row["route_observation"], "method": "GET", "uri": "/",
                       "content_type_count": "none", "content_type_kind": "missing"}
        row["route_observation"] = observation
        result = runtime.cli_attempt_members({**bodies, "grok-provider.json": json.dumps(row).encode()},
                                             self.source, "grok")
        self.assertTrue(result["test_failed"])
        self.assertFalse(result["runtime_attempt_complete"])
        self.assertFalse(any(result["qualification"].values()))
        for uri in ("/?credential=private-query", "//", "/v1/", "/%2F", "http://private-host/", "/private-root"):
            rejected = {**row, "route_observation": {**observation, "uri": uri}}
            with self.subTest(uri=uri), self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, "grok-provider.json": json.dumps(rejected).encode()},
                                           self.source, "grok")

    def test_grok_provider_private_canary_is_refused_in_every_capture_surface(self):
        bodies = self.grok_provider_bodies()
        canary = b"uap-grok-private-" + b"a" * 32
        for name in bodies:
            with self.subTest(member=name), self.assertRaises(ValueError):
                runtime.cli_attempt_members({**bodies, name: bodies[name] + canary}, self.source, "grok")
        encoded = b'{"opaque":"\\u0075ap-grok-private-' + b"a" * 32 + b'"}'
        for stream in (b'{"event":"start"}\n' + encoded, encoded + b'\n{"event":"end"}\n'):
            self.assertNotIn(b"uap-grok-private-", stream)
            self.assertTrue(runtime.grok_private_credential(stream))
            for present in (bodies, {"runtime-attempt.json": bodies["runtime-attempt.json"]}):
                with self.subTest(provider="grok-provider.json" in present), self.assertRaises(ValueError):
                    runtime.cli_attempt_members({**present, "probe-events.jsonl": stream}, self.source, "grok")
        self.assertFalse(runtime.grok_private_credential(b'{"event":"start"}\n{"event":"end"}\n'))
        self.assertTrue(runtime.grok_private_credential(b" " * ((1 << 20) + 1)))
        with self.assertRaises(ValueError):
            runtime.cli_attempt_members({**bodies, "grok-provider.request.json": encoded}, self.source, "grok")
        with self.assertRaises(ValueError):
            runtime.cli_attempt_members({"runtime-attempt.json": json.dumps(self.cli_result("grok")).encode(),
                                         "probe-events.jsonl": canary}, self.source, "grok")

    def test_grok_nested_semantic_json_strings_are_refused_before_admission(self):
        bodies = self.grok_provider_bodies()
        for seed in (b'{"key":"\\u0075ap-grok-private-inert"}',
                     b'{"\\u0075ap-grok-private-inert":"ordinary"}', b'["\\u0075ap-grok-private-inert"]'):
            nested = seed
            for _ in range(8):
                nested = json.dumps({"content": nested.decode()}).encode()
                for raw in (nested, b'{"first":true}\n' + nested + b"\n"):
                    self.assertNotIn(b"uap-grok-private-", raw)
                    self.assertTrue(runtime.grok_private_credential(raw))
                    with self.assertRaises(ValueError):
                        runtime.cli_attempt_members({**bodies, "probe-events.jsonl": raw}, self.source, "grok")
        benign = json.dumps({"content": '{"key":"ordinary"}\n{"tail":true}'}).encode()
        self.assertFalse(runtime.grok_private_credential(benign))
        self.assertFalse(runtime.grok_private_credential(b"ordinary non-JSON diagnostic"))

    def test_grok_privacy_budget_is_shared_across_lines_siblings_and_nested_text(self):
        for raw in (b"\xff", b'{"same":1,"same":2}', b"[" * 33 + b"0" + b"]" * 33,
                    b"[" + b"0," * 17000 + b"0]", json.dumps({"k" * 4097: True}).encode()):
            self.assertTrue(runtime.grok_private_credential(raw))
        self.assertTrue(runtime.grok_private_credential(
            b'{"n":' + b"9" * 5000 + b',"key":"\\u0075ap-grok-private-inert"}'))
        nested = json.dumps("x" * 4096).encode()
        for _ in range(2):
            nested = json.dumps({"content": nested.decode()}).encode()
        self.assertFalse(runtime.grok_private_credential(nested))
        self.assertTrue(runtime.grok_private_credential(nested + b"\n" + nested + b"\n"))
        for keys in (("first", "last"), ("last", "first")):
            siblings = json.dumps({key: nested.decode() for key in keys}).encode()
            self.assertTrue(runtime.grok_private_credential(siblings))
        self.assertFalse(runtime.grok_private_credential(b" " * (1 << 20)))

    def test_grok_numeric_privacy_refusal_requires_complete_json_syntax(self):
        text = b"1e999" + b"a" * 59
        for raw in (text, b"-1e999ordinary", b"1e999 []", b"1e999 1e999", b"NaN", b"Infinity",
                    json.dumps({"metadata": text.decode()}).encode(),
                    json.dumps({text.decode(): "ordinary"}).encode(),
                    text + b'\n{"event":"ordinary"}\n'):
            self.assertFalse(runtime.grok_private_credential(raw))
        for raw in (b"1e999", b'{"n":1e999}', b"[1e999]", json.dumps("1e999").encode(),
                    text + b'\n{"opaque":"\\u0075ap-grok-private-inert"}\n',
                    b"[1e999," + b"[" * 10050 + b"]" * 10050 + b"]"):
            self.assertTrue(runtime.grok_private_credential(raw))
        bodies = self.grok_provider_bodies()
        runtime.cli_attempt_members({**bodies, "probe-events.jsonl": json.dumps({"metadata": text.decode()}).encode()},
                                    self.source, "grok")
        with self.assertRaises(ValueError):
            runtime.cli_attempt_members({**bodies, "probe-events.jsonl": text +
                b'\n{"opaque":"\\u0075ap-grok-private-inert"}\n'}, self.source, "grok")
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


def load_tests(loader, tests, pattern):
    # The existing runtime workflow deliberately selects this exact module.
    # Broad research discovery already selects the new client-specific module.
    if pattern in (None, "test_project_native_runtime.py"):
        tests.addTests(loader.loadTestsFromName("test_project_native_cline_cycle.CompleteCycleTests"))
    return tests


if __name__ == "__main__":
    unittest.main()
