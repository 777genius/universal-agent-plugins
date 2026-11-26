"""Execute the real stable Kimi workflow oracle against inert held-byte proofs."""

import contextlib
import copy
import io
import json
import os
from pathlib import Path
import sys
from tempfile import TemporaryDirectory
import textwrap
import unittest
from unittest.mock import patch

from test_live_install_audit_env import load_audit


HEAD = "a" * 40
WORKFLOW = Path(__file__).resolve().parents[1] / ".github/workflows/kimi-stable-project-native-e2e.yml"


class KimiStableProjectAuditPolicyTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        source = WORKFLOW.read_text(encoding="utf-8")
        marker = '          python3 - "$AUDIT_REPORT_DIR/kimi-skill.json" "$AUDIT_PR_HEAD_SHA" <<\'PY\'\n'
        if source.count(marker) != 1:
            raise AssertionError("expected exactly one real Kimi workflow postchecker")
        body = source.split(marker, 1)[1].split("          PY\n", 1)[0]
        cls.oracle = compile(textwrap.dedent(body), str(WORKFLOW), "exec")

    def setUp(self):
        temporary = TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()

    def row(self, index=0):
        state = {"status": "observed", "selected_installations": 1, "selected_bindings": 1,
                 "invalid_rows": 0, "truncated": False,
                 "states": [{"materialization": "materialized", "activation": "active",
                             "authentication": "not_required", "policy": "allowed",
                             "verification": "installation_verified"}]}
        row = {key: True for key in (
            "skill_bytes_match", "lock_written", "lock_contains_identity", "lock_exact_source_pinned",
            "listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
            "removal_verified", "sentinels_preserved", "post_list_schema_valid")}
        row.update(target="kimi", kind="skill", index=index, identity=f"skill-{index}", status="passed",
                   repeat_add={"exit_code": 0, "timeout": False},
                   public_skill_ready={"after-add": True, "repeat-add": True},
                   public_target_states={phase: copy.deepcopy(state) for phase in ("after-add", "repeat-add")},
                   post_remove_identity_registered=False)
        return row

    def evidence(self):
        return {"pr_head_sha": HEAD, "passed": 30, "verified_negative_coverage": 0,
                "cases_attempted": 30, "full_coverage": True, "source_selection_complete": True,
                "client_version": {"status": "reported", "value": "2.5.2"},
                "results": [self.row(index) for index in range(30)]}

    def run_oracle(self, evidence, *, accepted):
        report = self.root / "report.json"
        report.write_text(json.dumps(evidence), encoding="utf-8")
        output = io.StringIO()
        with patch.object(sys, "argv", ["-", str(report), HEAD]), contextlib.redirect_stdout(output):
            if accepted:
                exec(self.oracle, {"__name__": "__main__"})
            else:
                with self.assertRaises(SystemExit):
                    exec(self.oracle, {"__name__": "__main__"})
        if accepted:
            self.assertIn(f"QUALIFIED: 30/30 source-backed Kimi 2.5.2 project Skills", output.getvalue())
        else:
            self.assertNotIn("QUALIFIED", output.getvalue())

    def captured_append(self, *, coordinator):
        audit = load_audit()
        case = self.root / ("with-coordinator" if coordinator else "log-only")
        home, project = case / "home", case / "project"
        log = home / ".kimi-code/logs/kimi-code.log"
        for directory in (log.parent, project / ".kimi-code/skills/docs", project / ".uap/managed/package"):
            directory.mkdir(parents=True)
        log.write_bytes(b"original\n")
        (home / "foreign").write_bytes(b"foreign state")
        lock = home / ".kimi-code/.projects-write-lock"
        if coordinator:
            lock.write_bytes(b"")
            lock.chmod(0o600)
        facts = {"captures": {phase: {} for phase in ("before", "after-add", "post-list")}, "append": {}}
        before = audit.kimi_repeat_log_capture(home, project, case, facts["captures"]["before"])
        self.assertIsNotNone(before)
        with log.open("ab") as stream:
            stream.write(b"catalog complete\n")
        if coordinator:
            replacement = lock.with_suffix(".temporary")
            replacement.write_bytes(b"")
            replacement.chmod(0o600)
            os.replace(replacement, lock)
        after = audit.kimi_repeat_log_capture(home, project, case, facts["captures"]["after-add"])
        listed = audit.kimi_repeat_log_capture(home, project, case, facts["captures"]["post-list"])
        added = {"schema_version": 1, "command": "skills add", "result": "success", "status": "succeeded",
                 "scope": "project", "readiness": "ready", "items": [{"targetId": "kimi", "status": "succeeded"}]}
        repeated = copy.deepcopy(added)
        repeated["items"][0]["status"] = "unchanged"
        row = self.row()
        proof = audit.kimi_repeat_log_append(before, after, listed, added, repeated, row["repeat_add"],
                                            before["native"], listed["native"], facts["append"])
        self.assertIsNotNone(proof)
        row.update(repeat_native_unchanged=False, repeat_native_runtime_only=True,
                   kimi_repeat_log_proof=proof, kimi_repeat_log_facts=facts,
                   repeat_native_changes=audit.snapshot_change_evidence(before["native"], listed["native"]))
        return row

    def test_thirty_unchanged_native_repeats_pass(self):
        self.run_oracle(self.evidence(), accepted=True)

    def test_real_held_log_and_coordinator_proofs_pass_all_thirty_slots(self):
        evidence = self.evidence()
        rows = [self.captured_append(coordinator=False), self.captured_append(coordinator=True)]
        for index in range(30):
            row = copy.deepcopy(rows[index % 2])
            row.update(index=index, identity=f"skill-{index}")
            evidence["results"][index] = row
        self.run_oracle(evidence, accepted=True)

    @staticmethod
    def change(document, path, value):
        target = document
        for key in path[:-1]:
            target = target[key]
        target[path[-1]] = value

    def test_runtime_only_and_malformed_or_unbound_log_proofs_refuse(self):
        original = self.captured_append(coordinator=True)
        mutations = [
            (("repeat_native_unchanged",), None), (("repeat_native_runtime_only",), 1),
            (("kimi_repeat_log_proof",), None), (("kimi_repeat_log_proof",), {}),
            (("kimi_repeat_log_proof", "status"), "observed"),
            (("kimi_repeat_log_proof", "path"), ".kimi-code/config.json"),
            (("kimi_repeat_log_proof", "same_inode_mode_parents"), 1),
            (("kimi_repeat_log_proof", "post_list_unchanged"), False),
            (("kimi_repeat_log_proof", "workspace_lock"), "unverified"),
            (("kimi_repeat_log_proof", "extra"), "unknown authority"),
            (("kimi_repeat_log_proof", "before_bytes"), True),
            (("kimi_repeat_log_proof", "before_bytes"), -1),
            (("kimi_repeat_log_proof", "after_bytes"), original["kimi_repeat_log_proof"]["before_bytes"]),
            (("kimi_repeat_log_proof", "after_bytes"), (1 << 20) + 1),
            (("kimi_repeat_log_proof", "after_sha256"), original["kimi_repeat_log_proof"]["before_sha256"]),
            (("kimi_repeat_log_proof", "append_sha256"), "A" * 64),
            (("kimi_repeat_log_proof", "before_sha256"), "0" * 63),
            (("repeat_native_changes", "total"), True),
            (("repeat_native_changes", "total"), 2),
            (("repeat_native_changes", "omitted"), False),
            (("repeat_native_changes", "omitted"), 1),
            (("repeat_native_changes", "entries", 0, "path_truncated"), 0),
            (("repeat_native_changes", "entries", 0, "path"), ".kimi-code/other"),
            (("repeat_native_changes", "entries", 0, "after"), "0" * 64),
            (("repeat_native_changes", "entries", 0, "extra"), True),
            (("kimi_repeat_log_facts", "append", "reason"), "protected-scope-changed"),
            (("kimi_repeat_log_facts", "captures", "post-list", "log_bytes"), 1),
            (("kimi_repeat_log_facts", "captures", "before", "reason"), "capture-unavailable"),
            (("kimi_repeat_log_facts", "captures", "extra"), {}),
        ]
        for path, value in mutations:
            with self.subTest(path=path, value=value):
                evidence = self.evidence()
                row = copy.deepcopy(original)
                self.change(row, path, value)
                evidence["results"][17] = row
                self.run_oracle(evidence, accepted=False)
        for key in ("repeat_native_runtime_only", "kimi_repeat_log_proof", "kimi_repeat_log_facts", "repeat_native_changes"):
            with self.subTest(missing=key):
                evidence = self.evidence()
                evidence["results"][29] = copy.deepcopy(original)
                evidence["results"][29].pop(key)
                self.run_oracle(evidence, accepted=False)

    def test_ready_closed_command_pins_removal_and_foreign_guards_still_required(self):
        rows = [self.row(), self.captured_append(coordinator=True)]
        mutations = [
            (("target",), "other"), (("kind",), "plugin"), (("status",), "prepared"),
            (("repeat_add", "timeout"), True), (("repeat_add", "exit_code"), None),
            (("repeat_add", "exit_code"), False), (("repeat_add", "exit_code"), 1),
            (("public_skill_ready", "repeat-add"), False),
            (("public_target_states", "after-add", "states", 0, "activation"), "prepared"),
            (("public_target_states", "repeat-add", "selected_bindings"), 2),
            (("public_target_states", "repeat-add", "invalid_rows"), 1),
            (("public_target_states", "repeat-add", "truncated"), True),
            (("post_remove_identity_registered",), True),
        ]
        mutations.extend(((key,), False) for key in (
            "skill_bytes_match", "lock_written", "lock_contains_identity", "lock_exact_source_pinned",
            "listed_identity", "repeat_verified", "repeat_lock_unchanged", "removal_verified",
            "sentinels_preserved", "post_list_schema_valid"))
        for index, original in enumerate(rows):
            for path, value in mutations:
                with self.subTest(append=index, path=path):
                    evidence = self.evidence()
                    row = copy.deepcopy(original)
                    self.change(row, path, value)
                    evidence["results"][0] = row
                    self.run_oracle(evidence, accepted=False)

    def test_exact_head_version_and_full_positive_coverage_still_required(self):
        mutations = [(("pr_head_sha",), "b" * 40), (("passed",), 29),
                     (("verified_negative_coverage",), 1), (("cases_attempted",), 29),
                     (("full_coverage",), False), (("source_selection_complete",), False),
                     (("client_version", "value"), "2.5.1"),
                     (("client_version", "status"), "inferred"), (("results",), [])]
        for path, value in mutations:
            with self.subTest(path=path):
                evidence = self.evidence()
                self.change(evidence, path, value)
                self.run_oracle(evidence, accepted=False)


if __name__ == "__main__":
    unittest.main()
