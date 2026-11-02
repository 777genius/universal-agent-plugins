"""Inert fixed empty-list evidence contracts; never native qualification."""
import copy
import json
import unittest

import project_native_runtime as runtime
import project_native_copilot_empty_registry as empty
import project_native_copilot_user as user
from test_project_native_copilot_user import encoded, failed_prefix
from test_project_native_copilot_readiness import SOURCE


def empty_prefix(count=4, stage=None):
    bodies = failed_prefix(min(count, 3), "prepare")
    summary = json.loads(bodies["runtime-attempt.json"])
    summary.update(mode=empty.MODE, commands=list(empty.COMMANDS[:count]),
                   stage=stage or ("review-barrier" if count == 4 else "prepare"))
    if count == 4:
        layout = summary["layout"]
        bodies["native-list.command.json"] = encoded({"argv": [layout["client"], "plugin", "list"],
            "cwd": layout["outside"], "exit": 0, "elapsed_ms": 1, "error": False, "context_error": False,
            "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"})
        bodies["native-list.stdout.log"], bodies["native-list.stderr.log"] = b"OPAQUE INERT LIST\n", b"opaque diagnostic\n"
    if summary["stage"] != "prepare":
        bodies["user-profile-metadata.json"] = encoded({"stage": "native-list", "status": "observed", "before_entries": 1,
            "after_entries": 1, "changed_entries": 0, "omitted_count": 0, "entries": []})
    bodies["runtime-attempt.json"] = encoded(summary)
    return bodies


class CopilotEmptyRegistryContracts(unittest.TestCase):
    def refused(self, bodies):
        with self.assertRaises((ValueError, TypeError, KeyError)):
            runtime.attempt_members(bodies, SOURCE, "copilot")

    def test_exact_failed_prefixes_and_opaque_output_never_qualify(self):
        for count in range(5):
            bodies = empty_prefix(count)
            held = copy.deepcopy(bodies)
            row = runtime.attempt_members(bodies, SOURCE, "copilot")
            self.assertTrue(row["test_failed"])
            self.assertFalse(row["runtime_attempt_complete"])
            self.assertFalse(any(row["qualification"].values()))
            self.assertEqual(bodies, held)
            self.assertNotIn("provider_kind", row)
        for text in (b"No plugins installed.\n", b"unrecognized listing\n", b"{not-json}"):
            bodies = empty_prefix()
            bodies["native-list.stdout.log"] = text
            self.assertTrue(runtime.attempt_members(bodies, SOURCE)["test_failed"])

    def test_missing_extra_probe_provider_and_unbounded_members_refused(self):
        for name in empty_prefix():
            bodies = empty_prefix()
            del bodies[name]
            self.refused(bodies)
        for name in ("probe-events.jsonl", "provider-request-1.json", "add-user.stdout.log", "private-file"):
            bodies = empty_prefix()
            bodies[name] = b"PRIVATE-BODY"
            self.refused(bodies)
        for name, size in (("native-list.stdout.log", (1 << 20) + 1), ("native-list.stderr.log", (1 << 20) + 1),
                           ("user-profile-metadata.json", 2049)):
            bodies = empty_prefix()
            bodies[name] = b" " * size
            self.refused(bodies)

    def test_every_success_claim_unknown_stage_and_inherited_authority_refused(self):
        changes = [(key, True) for key in ("runtime_attempt_complete", "vendor_authentication_qualified", "real_model_qualified")]
        changes += [("test_failed", False), ("stage", "public-user"), ("mode", "unknown"), ("provider_kind", "scripted_loopback"),
                    ("activation", "prepared"), ("probe_status", "captured"), ("source_identity", {}), ("elapsed_ms", 180001)]
        for key, value in changes:
            bodies = empty_prefix()
            summary = json.loads(bodies["runtime-attempt.json"])
            summary[key] = value
            bodies["runtime-attempt.json"] = encoded(summary)
            self.refused(bodies)
        for flag in user.FLAGS:
            bodies = empty_prefix()
            summary = json.loads(bodies["runtime-attempt.json"])
            summary["qualification"][flag] = True
            bodies["runtime-attempt.json"] = encoded(summary)
            self.refused(bodies)

    def test_fixed_command_path_count_source_and_original_fatal_prefix(self):
        for key, value in (("argv", ["/other/copilot", "plugin", "list"]), ("argv", ["/held/copilot", "plugin", "install"]),
                           ("cwd", "/held/fixture/project"), ("elapsed_ms", 35001), ("cleanup", "forced close")):
            bodies = empty_prefix()
            command = json.loads(bodies["native-list.command.json"])
            command[key] = value
            bodies["native-list.command.json"] = encoded(command)
            self.refused(bodies)
        for label in empty.COMMANDS:
            bodies = empty_prefix(4, "native-list")
            command = json.loads(bodies[label + ".command.json"])
            command["exit"] = 7
            bodies[label + ".command.json"] = encoded(command)
            if label != "native-list":
                self.refused(bodies)
                continue
            self.assertEqual(runtime.attempt_members(bodies, SOURCE)["stage"], "native-list")
            summary = json.loads(bodies["runtime-attempt.json"])
            summary["stage"] = "review-barrier"
            bodies["runtime-attempt.json"] = encoded(summary)
            self.refused(bodies)

    def test_exact_before_list_refusal_and_only_two_metadata_stages(self):
        bodies = empty_prefix(3, "before-list")
        self.refused(bodies)
        bodies["user-profile-metadata.json"] = b'{"stage":"native-list","status":"refused"}'
        self.assertTrue(runtime.attempt_members(bodies, SOURCE)["test_failed"])
        user.metadata(b'{"stage":"add-user","status":"refused"}')
        user.metadata(b'{"stage":"native-list","status":"refused"}', "native-list")
        for body, stage in ((b'{"stage":"add-user","status":"refused"}', "native-list"),
                            (b'{"stage":"native-list","status":"refused"}', "add-user"),
                            (b'{"stage":"unknown","status":"refused"}', "unknown")):
            with self.assertRaises(ValueError):
                user.metadata(body, stage)


if __name__ == "__main__":
    unittest.main()
