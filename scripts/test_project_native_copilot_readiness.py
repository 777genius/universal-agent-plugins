"""Replay actual failed bytes inertly; no hosted guard or vendor execution."""
import copy
import hashlib
import json
from pathlib import Path
import unittest

import project_native_runtime as runtime
from project_native_copilot_events import MODE
from project_native_copilot_readiness import members

FIXTURE = Path(__file__).resolve().parents[1] / "repotests/testdata/copilot-readiness-40553325878"
SOURCE = {"source_commit": "9b8ddc5332d70ca0c8d39c41c5ce60f3bba1c6e7", "source_tree": "8386a313607a2d1acfd5583bd9335d9ce7af1340"}


def encoded(value):
    return json.dumps(value, separators=(",", ":")).encode()


def actual():
    raw = (FIXTURE / "manifest.json").read_bytes()
    if hashlib.sha256(raw).hexdigest() != "807b961c1f4280081cc4a9c2d2c28c4997147b1df0985678cf0ace91e938703a":
        raise AssertionError("actual capture manifest changed")
    manifest = json.loads(raw)
    if set(path.name for path in FIXTURE.iterdir()) != set(manifest["files"]) | {"manifest.json"}:
        raise AssertionError("actual capture fixture member closure changed")
    bodies = {name: (FIXTURE / name).read_bytes() for name in manifest["files"]}
    if any(hashlib.sha256(body).hexdigest() != manifest["files"][name] for name, body in bodies.items()):
        raise AssertionError("actual capture fixture bytes changed")
    return bodies


def candidate():
    bodies = actual()
    summary = json.loads(bodies["runtime-attempt.json"])
    # A hypothetical new-mode envelope over held old bytes is a validator test,
    # never a relabeling of old evidence. Actual old command instants are absent;
    # these explicit test-only bounds enclose its exported wire events.
    summary.update(mode=MODE, runtime_attempt_complete=True, test_failed=False,
                   provider_closed=True, readiness_observed=True,
                   native_command_started_at="2026-10-27T18:31:21.7Z",
                   native_command_ended_at="2026-10-27T18:31:21.9Z",
                   native_session_id="86dc5cc4-2f2d-47c3-8831-52eb0b5e7ad6",
                   provider_call_id="uap_fb6cd26aaf0bd16f7f5e5dfb7be2efd7_attempt")
    bodies["runtime-attempt.json"] = encoded(summary)
    return bodies


class CopilotReadinessContracts(unittest.TestCase):
    def refused(self, bodies):
        with self.assertRaises((ValueError, TypeError, KeyError)):
            runtime.attempt_members(bodies, SOURCE, "copilot")

    def test_actual_old_source_remains_failed_and_unqualified(self):
        bodies = actual()
        result = runtime.attempt_members(bodies, SOURCE, "copilot")
        self.assertTrue(result["test_failed"])
        self.assertFalse(result["runtime_attempt_complete"])
        self.assertNotIn("mode", result)
        self.assertFalse(any(result["qualification"].values()))
        self.assertFalse(result["vendor_authentication_qualified"])
        self.assertFalse(result["real_model_qualified"])

    def test_new_mode_measured_all_join_reader_keeps_all_five_false(self):
        result = runtime.attempt_members(candidate(), SOURCE, "copilot")
        self.assertEqual(result["mode"], MODE)
        self.assertTrue(result["runtime_attempt_complete"])
        self.assertFalse(result["test_failed"])
        self.assertFalse(any(result["qualification"].values()))
        self.assertFalse(result["vendor_authentication_qualified"])
        self.assertFalse(result["real_model_qualified"])

    def test_every_required_member_and_unknown_member_refused_for_success(self):
        for name in candidate():
            bodies = candidate()
            del bodies[name]
            with self.subTest(member=name):
                self.refused(bodies)
        bodies = candidate()
        bodies["private-extra"] = b"uap-copilot-private-canary"
        self.refused(bodies)

    def test_failed_prefix_and_late_failure_cannot_complete(self):
        bodies = candidate()
        summary = json.loads(bodies["runtime-attempt.json"])
        summary.update(test_failed=True, runtime_attempt_complete=False)
        bodies["runtime-attempt.json"] = encoded(summary)
        self.assertTrue(members(bodies, SOURCE)["test_failed"])
        for key in ("runtime_attempt_complete", "provider_closed", "readiness_observed", "test_failed"):
            for wrong in (None, "false", 0, 1):
                bad = candidate()
                row = json.loads(bad["runtime-attempt.json"])
                row[key] = wrong
                bad["runtime-attempt.json"] = encoded(row)
                with self.subTest(key=key, wrong=wrong):
                    self.refused(bad)
        for key in ("runtime_attempt_complete", "provider_closed", "readiness_observed", "provider_returned"):
            bad = candidate()
            row = json.loads(bad["runtime-attempt.json"])
            row[key] = False
            bad["runtime-attempt.json"] = encoded(row)
            self.refused(bad)
        for key in ("vendor_authentication_qualified", "real_model_qualified"):
            bad = candidate()
            row = json.loads(bad["runtime-attempt.json"])
            row[key] = True
            bad["runtime-attempt.json"] = encoded(row)
            self.refused(bad)
        for key in json.loads(candidate()["runtime-attempt.json"])["qualification"]:
            bad = candidate()
            row = json.loads(bad["runtime-attempt.json"])
            row["qualification"][key] = True
            bad["runtime-attempt.json"] = encoded(row)
            self.refused(bad)
        prefix = {"runtime-attempt.json": encoded({"mode": MODE, "client": "copilot", "provider_kind": "scripted_loopback",
                  "activation": "prepared", "qualification": {key: False for key in row["qualification"]},
                  "vendor_authentication_qualified": False, "real_model_qualified": False,
                  "test_failed": True, "runtime_attempt_complete": False, "provider_closed": False,
                  "readiness_observed": False, "elapsed_ms": 1})}
        self.assertTrue(members(prefix, SOURCE)["test_failed"])

    def test_source_command_acquisition_and_owned_authority_mismatch_refused(self):
        for key, value in (("source_identity", {}), ("native_session_id", "foreign"), ("provider_call_id", "foreign"),
                           ("elapsed_ms", 150000), ("native_command_started_at", "2026-10-27T19:31:21Z"),
                           ("native_command_ended_at", "2026-10-27T18:31:21Z"), ("mode", "unknown"),
                           ("original_config_change", {"private": "uap-copilot-private-canary"})):
            bodies = candidate()
            row = json.loads(bodies["runtime-attempt.json"])
            row[key] = value
            bodies["runtime-attempt.json"] = encoded(row)
            self.refused(bodies)
        for label in ("git-A", "git-B", "version", "add-A", "native-attempt"):
            for key, value in (("exit", 1), ("error", True), ("context_error", True), ("cwd", "/foreign"), ("elapsed_ms", True)):
                bodies = candidate()
                row = json.loads(bodies[label + ".command.json"])
                row[key] = value
                bodies[label + ".command.json"] = encoded(row)
                self.refused(bodies)
        for field in ("acquisition", "binding", "data_locator", "installation_source", "probe_facts"):
            bodies = candidate()
            row = json.loads(bodies["runtime-attempt.json"])
            row[field] = {"private": "uap-copilot-private-canary"}
            bodies["runtime-attempt.json"] = encoded(row)
            self.refused(bodies)
        for name in ("mcp.json.installed", "uapcopilota-SKILL.md.installed", "native-attempt.stderr.log"):
            bodies = candidate()
            bodies[name] += b"uap-copilot-private-canary"
            self.refused(bodies)

    def test_each_event_field_is_closed_and_every_native_edge_is_bound(self):
        original = [json.loads(line) for line in candidate()["native-attempt.stdout.log"].splitlines()]
        for index, row in enumerate(original):
            for key in row:
                rows = copy.deepcopy(original)
                del rows[index][key]
                bodies = candidate()
                bodies["native-attempt.stdout.log"] = b"".join(encoded(item) + b"\n" for item in rows)
                with self.subTest(index=index, key=key):
                    self.refused(bodies)
        changes = ((0, "id", True), (1, "id", original[0]["id"]), (15, "parentId", original[12]["id"]),
                   (26, "sessionId", original[0]["parentId"]), (26, "exitCode", False),
                   (25, "timestamp", "2026-10-27T18:32:21Z"))
        for index, key, wrong in changes:
            rows = copy.deepcopy(original)
            rows[index][key] = wrong
            bodies = candidate()
            bodies["native-attempt.stdout.log"] = b"".join(encoded(item) + b"\n" for item in rows)
            self.refused(bodies)
        for fault in ("unknown", "turn", "call", "args", "result", "sentinel", "skill", "builtin", "server", "telemetry", "change"):
            rows = copy.deepcopy(original)
            if fault == "unknown":
                rows[25]["data"]["private"] = "uap-copilot-private-canary"
            elif fault == "turn":
                rows[17]["data"]["turnId"] = 1
            elif fault == "call":
                rows[13]["data"]["toolCallId"] = "foreign"
            elif fault == "args":
                rows[10]["data"]["inputDelta"] = "{}"
            elif fault == "result":
                rows[15]["data"]["result"]["contents"] = []
            elif fault == "sentinel":
                rows[22]["data"]["content"] = "foreign"
            elif fault == "skill":
                rows[3]["data"]["skills"][0]["source"] = "builtin"
            elif fault == "builtin":
                rows[3]["data"]["skills"][1]["description"] = "foreign"
            elif fault == "server":
                rows[5]["data"]["servers"][-1]["source"] = "user"
            elif fault == "telemetry":
                rows[15]["data"]["toolTelemetry"]["metrics"]["mcp_result_content_bytes"] = True
            else:
                rows[26]["usage"]["codeChanges"]["filesModified"] = ["foreign"]
            bodies = candidate()
            bodies["native-attempt.stdout.log"] = b"".join(encoded(item) + b"\n" for item in rows)
            self.refused(bodies)

    def test_history_probe_reordering_foreign_facts_and_private_extra_refused(self):
        for fault in ("prompt", "parameters", "prefix", "call", "result", "extra"):
            bodies = candidate()
            row = json.loads(bodies["provider-request-2.json"])
            if fault == "prompt":
                row["messages"][1]["content"] = "Call only the installed inspect_runtime tool once for the fixture challenge."
            elif fault == "parameters":
                row["temperature"] = 1
            elif fault == "prefix":
                row["messages"][:2] = list(reversed(row["messages"][:2]))
            elif fault == "call":
                row["messages"][2]["tool_calls"][0]["id"] = "foreign"
            elif fault == "result":
                row["messages"][3]["content"] = "{}"
            else:
                row["private"] = "uap-copilot-private-canary"
            bodies["provider-request-2.json"] = encoded(row)
            self.refused(bodies)
        for fault in ("root", "pid", "session", "timestamp", "extra", "reorder", "duplicate"):
            bodies = candidate()
            rows = [json.loads(line) for line in bodies["probe-events.jsonl"].splitlines()]
            if fault == "root":
                rows[2]["facts"]["plugin_root"] = "/foreign"
            elif fault == "pid":
                rows[0]["pid"] = False
            elif fault == "session":
                rows[0]["session"] = "foreign"
            elif fault == "timestamp":
                rows[2]["timestamp"] = "2026-10-27T18:32:21Z"
            elif fault == "extra":
                rows[2]["facts"]["private"] = "uap-copilot-private-canary"
            elif fault == "reorder":
                rows[0], rows[1] = rows[1], rows[0]
            else:
                rows.append(rows[-1])
            bodies["probe-events.jsonl"] = b"".join(encoded(item) + b"\n" for item in rows)
            self.refused(bodies)


if __name__ == "__main__":
    unittest.main()
