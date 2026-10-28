"""Independent inert post-add/list prefixes; no client or installer execution."""
import copy
import json
from pathlib import Path
import unittest

import project_native_runtime as runtime
import project_native_copilot_post_add_registry as post
from test_project_native_copilot_user import encoded, failed_prefix
from test_project_native_copilot_readiness import SOURCE


def fixture(count=5, stage="review-barrier"):
    bodies = failed_prefix(min(count, 4), "public-result" if count >= 4 else "prepare")
    result = json.loads(bodies["runtime-attempt.json"])
    result.update(mode=post.MODE, commands=list(post.COMMANDS[:count]), stage=stage)
    metadata_count = post.STAGES[stage][1]
    bodies.pop(post.USER_META, None)
    for name, label in ((post.USER_META, "add-user"), (post.LIST_META, "native-list"))[:metadata_count]:
        refused = stage in ("before-user", "after-user-metadata") and name == post.USER_META or stage in ("before-list", "after-list-metadata") and name == post.LIST_META
        bodies[name] = encoded({"stage": label, "status": "refused"} if refused else {"stage": label, "status": "observed", "before_entries": 1, "after_entries": 1, "changed_entries": 0, "omitted_count": 0, "entries": []})
    if count == 5:
        layout = result["layout"]
        bodies["native-list.command.json"] = encoded({"argv": [layout["client"], "plugin", "list"], "cwd": layout["outside"], "exit": 0, "elapsed_ms": 1, "error": False, "context_error": False, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"})
        bodies["native-list.stdout.log"], bodies["native-list.stderr.log"] = b"opaque independently authored post-add list\n", b"opaque native diagnostic\n"
    bodies["runtime-attempt.json"] = encoded(result)
    return bodies


class CopilotPostAddRegistryTests(unittest.TestCase):
    def refused(self, bodies):
        with self.assertRaises((ValueError, KeyError, TypeError)):
            runtime.attempt_members(bodies, SOURCE, "copilot")

    def test_all_finite_failed_prefixes_and_full18_remain_unqualified_and_read_only(self):
        for stage, (lengths, _count, _validated) in post.STAGES.items():
            for n in lengths:
                bodies = fixture(n, stage)
                held = copy.deepcopy(bodies)
                with self.subTest(stage=stage, count=n):
                    result = runtime.attempt_members(bodies, SOURCE, "copilot")
                    self.assertTrue(result["test_failed"])
                    self.assertFalse(result["runtime_attempt_complete"])
                    self.assertFalse(any(result["qualification"].values()))
                    self.assertEqual(bodies, held)
        self.assertEqual(len(fixture()), 18)
        for text in (b"", b"unknown native listing\n", b"{not-json}", b"\x1b[31mopaque\x1b[0m\r\n"):
            bodies = fixture()
            bodies["native-list.stdout.log"] = text
            self.assertTrue(runtime.attempt_members(bodies, SOURCE)["test_failed"])

    def test_missing_extra_partial_and_size_boundaries_refuse(self):
        for name in fixture():
            bodies = fixture()
            del bodies[name]
            with self.subTest(missing=name):
                self.refused(bodies)
        for name in ("probe-events.jsonl", "provider-request-1.json", "native-list-2.stdout.log", "mcp.json.installed", "private-file"):
            self.refused({**fixture(), name: b"{}"})
        for name, size in ((post.USER_META, 2049), (post.LIST_META, 2049), ("native-list.stdout.log", (1 << 20) + 1), ("native-list.stderr.log", (1 << 20) + 1)):
            self.refused({**fixture(), name: b" " * size})
        self.refused({**fixture(), "native-list.stdout.log": b"\xff"})

    def test_success_authority_selection_and_stage_spoofs_refuse(self):
        changes = {"runtime_attempt_complete": True, "test_failed": False, "vendor_authentication_qualified": True,
                   "real_model_qualified": True, "stage": "unknown", "mode": "unknown", "provider_kind": "scripted_loopback",
                   "activation": "prepared", "probe_status": "captured", "elapsed_ms": 180001, "source_identity": {}}
        for key, value in changes.items():
            bodies = fixture()
            row = json.loads(bodies["runtime-attempt.json"])
            row[key] = value
            bodies["runtime-attempt.json"] = encoded(row)
            with self.subTest(key=key):
                self.refused(bodies)
        for flag in post.FLAGS:
            bodies = fixture()
            row = json.loads(bodies["runtime-attempt.json"])
            row["qualification"][flag] = True
            bodies["runtime-attempt.json"] = encoded(row)
            self.refused(bodies)
        for sequence in (["git-B", "git-A", "version", "add-user", "native-list"], list(post.COMMANDS) + ["native-list"], list(post.COMMANDS[:3]) + ["native-list", "add-user"]):
            bodies = fixture()
            row = json.loads(bodies["runtime-attempt.json"])
            row["commands"] = sequence
            bodies["runtime-attempt.json"] = encoded(row)
            self.refused(bodies)

    def test_failed_current_command_is_truthful_but_no_later_command_may_follow(self):
        for stage, label, count in (("prepare", "version", 3), ("public-user", "add-user", 4), ("native-list", "native-list", 5)):
            bodies = fixture(count, stage)
            row = json.loads(bodies[label + ".command.json"])
            row.update(exit=7, error=True)
            bodies[label + ".command.json"] = encoded(row)
            self.assertTrue(runtime.attempt_members(bodies, SOURCE)["test_failed"])
        for label in post.COMMANDS:
            for key, value in (("exit", 7), ("error", True), ("context_error", True), ("cleanup", "forced close"), ("elapsed_ms", 35001), ("elapsed_ms", True)):
                bodies = fixture()
                row = json.loads(bodies[label + ".command.json"])
                row[key] = value
                bodies[label + ".command.json"] = encoded(row)
                with self.subTest(label=label, key=key):
                    self.refused(bodies)
        bodies = fixture(5, "native-list")
        row = json.loads(bodies["add-user.command.json"])
        row["exit"] = 7
        bodies["add-user.command.json"] = encoded(row)
        self.refused(bodies)

    def test_both_metadata_and_valid_public_progress_are_required(self):
        for name in (post.USER_META, post.LIST_META):
            for change in ({"stage": "unknown"}, {"status": "unknown"}, {"before_entries": True}, {"omitted_count": 1}, {"private": "forbidden"}):
                bodies = fixture()
                row = json.loads(bodies[name])
                row.update(change)
                bodies[name] = encoded(row)
                self.refused(bodies)
        for name, stage in ((post.USER_META, "add-user"), (post.LIST_META, "native-list")):
            self.refused({**fixture(), name: encoded({"stage": stage, "status": "refused"})})
        for body in (b"{}", b"{not-json}", b'{"schema_version":1,"schema_version":1,"command":"add","result":"success","data":{}}', encoded({"schema_version": True, "command": "add", "result": "success", "data": {}}), encoded({"schema_version": 1, "command": "remove", "result": "success", "data": {}})):
            self.refused({**fixture(), "add-user.stdout.log": body})
        bodies = fixture(4, "public-result")
        bodies["add-user.stdout.log"] = b"{actual failed JSON validation"
        self.assertTrue(runtime.attempt_members(bodies, SOURCE)["test_failed"])

    def test_exact_canonical_selection_and_old_mode_isolation(self):
        self.assertEqual(runtime.CASES["copilot"], {"test": "TestAgentpluginsCopilotNativePostAddRegistryFirstObservation", "file": "repotests/agentplugins_copilot_post_add_registry_attempt_test.go", "optin": "AGENTPLUGINS_COPILOT_POST_ADD_REGISTRY_FIRST_CAPTURE", "seconds": 180, "phase": "runtime-attempt"})
        root = Path(__file__).resolve().parents[1]
        self.assertIn("func TestAgentpluginsCopilotNativePostAddRegistryFirstObservation(t *testing.T)", (root / runtime.CASES["copilot"]["file"]).read_text())
        old = failed_prefix()
        self.assertTrue(runtime.attempt_members(old, SOURCE)["test_failed"])
        row = json.loads(old["runtime-attempt.json"])
        row["mode"] = post.MODE
        old["runtime-attempt.json"] = encoded(row)
        self.refused(old)


if __name__ == "__main__":
    unittest.main()
