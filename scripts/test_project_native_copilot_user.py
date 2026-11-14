"""Inert failed-only envelope and projection controls; never a hosted/native result."""
import copy
import json
from pathlib import Path
import unittest

import project_native_runtime as runtime
import project_native_copilot_user as user
from test_project_native_copilot_readiness import actual, SOURCE


def encoded(value):
    return json.dumps(value, separators=(",", ":")).encode()


def failed_prefix(n=4, stage=None):
    original = json.loads(actual()["runtime-attempt.json"])
    root = "/held/fixture"
    layout = {"root": root, "home": root + "/home", "a": root + "/project", "b": root + "/project-B",
              "outside": root + "/outside", "package": root + "/package", "installer": "/held/agentplugins",
              "client": original["acquisition"]["runtime_files"][0]["path"], "git": "/usr/bin/git"}
    stage = stage or ("review-barrier" if n == 4 else "prepare")
    summary = {"client": "copilot", "mode": user.MODE, "acquisition": original["acquisition"], "scanner": original["scanner"],
               "source_identity": original["source_identity"], "runtime_attempt_complete": False,
               "vendor_authentication_qualified": False, "real_model_qualified": False,
               "qualification": dict.fromkeys(user.FLAGS, False), "commands": list(user.COMMANDS[:n]),
               "stage": stage, "layout": layout, "probe_status": "absent", "test_failed": True, "elapsed_ms": 100}
    result = {"runtime-attempt.json": encoded(summary)}
    for label in user.COMMANDS[:n]:
        cwd = layout["outside"]
        if label.startswith("git-"):
            cwd = layout["a"] if label == "git-A" else layout["b"]
            argv = [layout["git"], "-c", "core.hooksPath=/dev/null", "init", "--quiet"]
        elif label == "version":
            argv = [layout["client"], "--version"]
        else:
            argv = [layout["installer"], "add", layout["package"], "--no-save", "--scope", "user", "--target", "copilot", "--format", "json"]
        result[label + ".command.json"] = encoded({"argv": argv, "cwd": cwd, "exit": 0, "elapsed_ms": 1,
                "error": False, "context_error": False, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"})
        result[label + ".stdout.log"] = b""
        result[label + ".stderr.log"] = b""
    if n >= 3:
        result["version.stdout.log"] = b"GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n"
    if n == 4:
        # A parser fixture only: no public installer was run or reported passed.
        result["add-user.stdout.log"] = encoded({"schema_version": 1, "command": "add", "result": "success", "data": {}})
    if stage != "prepare":
        result["user-profile-metadata.json"] = encoded({"stage": "add-user", "status": "observed", "before_entries": 1,
                     "after_entries": 1, "changed_entries": 0, "omitted_count": 0, "entries": []})
    return result


def change():
    return {"path_sha256": "a" * 64, "before_kind": "absent", "after_kind": "file", "before_permissions": 0,
            "after_permissions": 0o600, "before_present": False, "after_present": True, "owner_equal": False,
            "group_equal": False, "identity_equal": False, "mode_equal": False}


class CopilotUserCaptureContracts(unittest.TestCase):
    def refused(self, bodies):
        with self.assertRaises((ValueError, TypeError, KeyError)):
            runtime.attempt_members(bodies, SOURCE, "copilot")

    def test_each_genuine_ordered_failed_prefix_and_never_qualification(self):
        for n in range(5):
            result = runtime.attempt_members(failed_prefix(n), SOURCE, "copilot")
            self.assertTrue(result["test_failed"])
            self.assertFalse(result["runtime_attempt_complete"])
            self.assertFalse(any(result["qualification"].values()))
            self.assertNotIn("provider_kind", result)
            self.assertNotIn("activation", result)

    def test_each_promotion_foreign_source_and_inherited_claim_refused(self):
        for key, value in (("test_failed", False), ("runtime_attempt_complete", True), ("vendor_authentication_qualified", True),
                           ("real_model_qualified", True), ("provider_kind", "scripted_loopback"), ("activation", "prepared"),
                           ("elapsed_ms", True), ("elapsed_ms", 180001), ("mode", "other"), ("source_identity", {})):
            bodies = failed_prefix()
            row = json.loads(bodies["runtime-attempt.json"])
            row[key] = value
            bodies["runtime-attempt.json"] = encoded(row)
            self.refused(bodies)
        for flag in user.FLAGS:
            bodies = failed_prefix()
            row = json.loads(bodies["runtime-attempt.json"])
            row["qualification"][flag] = True
            bodies["runtime-attempt.json"] = encoded(row)
            self.refused(bodies)

    def test_partial_foreign_command_scope_cwd_order_and_extra_member_refused(self):
        for name in failed_prefix():
            bodies = failed_prefix()
            del bodies[name]
            self.refused(bodies)
        for extra in ("provider-request-1.json", "native-attempt.stdout.log", "mcp.json.installed", "uapcopilota-SKILL.md.installed", "private-value"):
            bodies = failed_prefix()
            bodies[extra] = b"PRIVATE-BODY"
            self.refused(bodies)
        for fault in ("scope", "cwd", "argv", "timeout", "source-pin", "scanner-pin", "prefix"):
            bodies = failed_prefix()
            row = json.loads(bodies["add-user.command.json"])
            summary = json.loads(bodies["runtime-attempt.json"])
            if fault == "scope":
                row["argv"][5] = "project"
            elif fault == "cwd":
                row["cwd"] = summary["layout"]["a"]
            elif fault == "argv":
                row["argv"].append("--external-uninstalled")
            elif fault == "timeout":
                row["elapsed_ms"] = 35001
            elif fault == "source-pin":
                summary["acquisition"]["acquisition"]["sha256"] = "0" * 64
            elif fault == "scanner-pin":
                summary["scanner"]["version"] = "unknown"
            else:
                summary["commands"][1:3] = reversed(summary["commands"][1:3])
            bodies["add-user.command.json"], bodies["runtime-attempt.json"] = encoded(row), encoded(summary)
            self.refused(bodies)

    def test_original_failure_and_refused_metadata_cannot_become_review_barrier(self):
        bodies = failed_prefix(4, "public-user")
        row = json.loads(bodies["add-user.command.json"])
        row.update(exit=-1, error=True, context_error=True)
        bodies["add-user.command.json"] = encoded(row)
        bodies["user-profile-metadata.json"] = b'{"stage":"add-user","status":"refused"}'
        result = runtime.attempt_members(bodies, SOURCE)
        self.assertEqual(result["stage"], "public-user")
        summary = json.loads(bodies["runtime-attempt.json"])
        summary["stage"] = "review-barrier"
        bodies["runtime-attempt.json"] = encoded(summary)
        self.refused(bodies)
        for key in ("exit", "error", "context_error"):
            bad = failed_prefix()
            row = json.loads(bad["git-A.command.json"])
            row[key] = 1 if key == "exit" else True
            bad["git-A.command.json"] = encoded(row)
            self.refused(bad)

    def test_metadata_total_byte_count_type_sorted_prefix_and_privacy_closed(self):
        bodies = failed_prefix()
        metadata = {"stage": "add-user", "status": "observed", "before_entries": 1, "after_entries": 2,
                    "changed_entries": 1, "omitted_count": 0, "entries": [change()]}
        bodies["user-profile-metadata.json"] = encoded(metadata)
        runtime.attempt_members(bodies, SOURCE)
        for key, wrong in (("before_entries", True), ("after_entries", 1025), ("changed_entries", 9), ("omitted_count", -1),
                           ("status", "passed"), ("private", "PRIVATE-BODY"), ("entries", [change()] * 16)):
            bad = copy.deepcopy(metadata)
            bad[key] = wrong
            bodies["user-profile-metadata.json"] = encoded(bad)
            self.refused(bodies)
        for key, wrong in (("path_sha256", "PRIVATE-PATH"), ("after_permissions", True), ("identity_equal", "false"),
                           ("before_present", True), ("after_kind", "symlink"), ("private_value", "PRIVATE-BODY")):
            bad = copy.deepcopy(metadata)
            bad["entries"][0][key] = wrong
            bodies["user-profile-metadata.json"] = encoded(bad)
            self.refused(bodies)
        bodies["user-profile-metadata.json"] = encoded(metadata) + b" " * 2048
        self.refused(bodies)

    def test_finite_config_diagnostic_only_and_optional_probe_keeps_failure(self):
        bodies = failed_prefix(4, "public-user")
        summary = json.loads(bodies["runtime-attempt.json"])
        summary["original_config_change"] = {"status": "deadline", "expected_sha256": "a" * 64, "baseline_bytes": 12}
        summary["probe_status"] = "captured"
        bodies["probe-events.jsonl"] = b"{}\n"
        bodies["runtime-attempt.json"] = encoded(summary)
        self.assertTrue(runtime.attempt_members(bodies, SOURCE)["test_failed"])
        for key, wrong in (("status", "PRIVATE-ERROR"), ("expected_sha256", "PRIVATE-VALUE"), ("private", "PRIVATE-BODY")):
            bad = copy.deepcopy(summary)
            bad["original_config_change"][key] = wrong
            bodies["runtime-attempt.json"] = encoded(bad)
            self.refused(bodies)
        bodies["runtime-attempt.json"] = encoded(summary)
        del bodies["probe-events.jsonl"]
        self.refused(bodies)

    def test_exact_single_canonical_selection_and_preserved_readiness_entrypoint(self):
        case = runtime.CASES["copilot"]
        self.assertEqual(case, {"test": "TestAgentpluginsCopilotNativeProjectCompleteCycle",
            "file": "repotests/agentplugins_copilot_cycle_lifecycle_test.go",
            "optin": "AGENTPLUGINS_COPILOT_PROJECT_RUNTIME_ATTEMPT", "seconds": 180, "phase": "runtime-attempt"})
        source = Path(__file__).resolve().parents[1]
        self.assertIn("func TestAgentpluginsCopilotNativeProjectRuntimeAttempt(t *testing.T)",
                      (source / "repotests/agentplugins_copilot_project_native_attempt_test.go").read_text())
        self.assertIn("func TestAgentpluginsCopilotNativeUserRegistrationFirstObservation(t *testing.T)",
                      (source / "repotests/agentplugins_copilot_user_registration_attempt_test.go").read_text())
        self.assertEqual(len([key for key in runtime.CASES if key == "copilot"]), 1)
        # Existing old failed-only and measured readiness readers remain callable.
        self.assertTrue(runtime.attempt_members(actual(), SOURCE)["test_failed"])


if __name__ == "__main__":
    unittest.main()
