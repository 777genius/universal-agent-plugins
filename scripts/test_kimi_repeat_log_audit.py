"""Inert filesystem checks for one bounded owned Kimi diagnostic append."""
import copy
import json
import os
import stat
from pathlib import Path
from tempfile import TemporaryDirectory
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from test_live_install_audit_env import load_audit


class KimiRepeatLogAuditTest(unittest.TestCase):
    def test_pinned_native_skill_log_gate_refuses_other_versions_and_contexts(self):
        audit = load_audit()
        admits = audit.kimi_log_evidence_eligible
        for version in ("2.3.2", "2.5.2"):
            with self.subTest(qualified=version):
                self.assertTrue(admits("kimi", "skill", "linux",
                    {"status": "reported", "value": version, "exit_code": 0}))
        for version in ("2.3.1", "2.4.1", "2.5.3", "2.3.2\\n", None, False, 232):
            with self.subTest(unqualified=version):
                self.assertFalse(admits("kimi", "skill", "linux",
                    {"status": "reported", "value": version, "exit_code": 0}))
        valid = {"status": "reported", "value": "2.3.2", "exit_code": 0}
        for target, kind, platform in (
            ("claude", "skill", "linux"), ("kimi", "plugin", "linux"),
            ("kimi", "skill", "darwin"), ("kimi", "skill", "win32")):
            with self.subTest(target=target, kind=kind, platform=platform):
                self.assertFalse(admits(target, kind, platform, valid))
        for invalid in (None, [], {}, {"status": "reported", "value": "2.3.2"},
                        {"status": "not-probed", "value": "2.3.2", "exit_code": 0},
                        {"status": "reported", "value": "2.3.2", "exit_code": False},
                        {"status": "reported", "value": "2.3.2", "exit_code": 1}):
            with self.subTest(invalid=invalid):
                self.assertFalse(admits("kimi", "skill", "linux", invalid))

    def fixture(self):
        temporary = TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.audit = load_audit()
        self.root = Path(temporary.name).resolve()
        self.home, self.project = self.root / "home", self.root / "project"
        self.log = self.home / ".kimi-code/logs/kimi-code.log"
        for path in (self.log.parent, self.project / ".kimi-code/skills/docs", self.project / ".uap/managed/package"):
            path.mkdir(parents=True)
        self.log.write_bytes(b"original log\n")
        (self.home / "foreign").write_bytes(b"foreign")
        (self.project / ".kimi-code/skills/docs/SKILL.md").write_bytes(b"owned Skill")
        self.command = {"exit_code": 0, "timeout": False}
        self.added = {"schema_version": 1, "command": "skills add", "result": "success", "status": "succeeded",
                      "scope": "project", "readiness": "ready", "items": [{"targetId": "kimi", "status": "succeeded"}]}
        self.repeated = copy.deepcopy(self.added)
        self.repeated["items"][0]["status"] = "unchanged"

    def capture(self, facts=None):
        return self.audit.kimi_repeat_log_capture(self.home, self.project, self.root, facts)

    def append(self):
        with self.log.open("ab") as stream:
            stream.write(b"catalog completed\n")

    def classify(self, before, after, listed, *, native_before=None, native_after=None, facts=None):
        return self.audit.kimi_repeat_log_append(
            before, after, listed, self.added, self.repeated, self.command,
            (before or {}).get("native") if native_before is None else native_before,
            (listed or {}).get("native") if native_after is None else native_after, facts)

    def workspace_lock(self):
        target = self.home / ".kimi-code/.projects-write-lock"
        target.write_bytes(b"")
        target.chmod(0o600)
        return target

    def test_native_empty_workspace_lock_replacement_with_log_append(self):
        self.fixture()
        target = self.workspace_lock()
        before = self.capture()
        self.assertIsNotNone(before)
        self.append()
        temporary = target.with_name(target.name + ".tmp")
        temporary.write_bytes(b"")
        temporary.chmod(0o600)
        os.replace(temporary, target)
        after = self.capture()
        self.assertIsNotNone(after)
        self.assertNotEqual(before["workspace_lock"][6], after["workspace_lock"][6])
        self.assertEqual(before["workspace_lock"][:6], after["workspace_lock"][:6])
        proof = self.classify(before, after, self.capture())
        self.assertIsNotNone(proof)
        self.assertEqual(proof["status"], "verified-owned-log-append")
        self.assertTrue(proof["post_list_unchanged"])

    def test_workspace_lock_refuses_unsafe_capture_and_read_replacement(self):
        for mutation in ("content", "mode", "uid", "hardlink", "symlink", "directory",
                         "companion", "read-replacement"):
            with self.subTest(mutation=mutation):
                self.fixture()
                target = self.workspace_lock()
                if mutation == "content":
                    target.write_bytes(b"private-coordinator-canary")
                elif mutation == "mode":
                    target.chmod(0o640)
                elif mutation == "hardlink":
                    os.link(target, self.home / "another-link")
                elif mutation == "symlink":
                    target.unlink()
                    target.symlink_to(self.home / "foreign")
                elif mutation == "directory":
                    target.unlink()
                    target.mkdir()
                elif mutation == "companion":
                    target.with_name(target.name + ".lock").mkdir()
                read, lstat = self.audit.copilot_settings_read, Path.lstat
                reached = []
                def changed_owner(path, *args, **kwargs):
                    value = lstat(path, *args, **kwargs)
                    if mutation == "uid" and path == target:
                        reached.append(True)
                        fields = {key: getattr(value, key) for key in dir(value) if key.startswith("st_")}
                        fields["st_uid"] += 1
                        return SimpleNamespace(**fields)
                    return value
                def changed_read(path, *args, **kwargs):
                    value = read(path, *args, **kwargs)
                    if mutation == "read-replacement" and path == target:
                        reached.append(True)
                        temporary = target.with_name(target.name + ".tmp")
                        temporary.write_bytes(b"")
                        temporary.chmod(0o600)
                        os.replace(temporary, target)
                    return value
                facts = {}
                with patch.object(Path, "lstat", changed_owner), \
                     patch.object(self.audit, "copilot_settings_read", changed_read):
                    self.assertIsNone(self.capture(facts))
                if mutation in ("uid", "read-replacement"):
                    self.assertTrue(reached)
                self.assertNotIn("private-coordinator-canary", json.dumps(facts))
                self.assertNotIn(str(self.root), json.dumps(facts))

    def test_workspace_companion_created_during_final_held_read_refuses(self):
        self.fixture()
        target = self.workspace_lock()
        before = self.capture()
        self.append()
        after = self.capture()
        self.assertIsNotNone(self.classify(before, after, self.capture()))
        read, reached = self.audit.copilot_settings_read, []
        def create_companion(path, *args, **kwargs):
            result = read(path, *args, **kwargs)
            if path == target:
                reached.append(True)
                target.with_name(target.name + ".lock").mkdir()
            return result
        with patch.object(self.audit, "copilot_settings_read", create_companion):
            listed = self.capture()
        self.assertEqual(reached, [True])
        self.assertIsNone(listed)
        self.assertIsNone(self.classify(before, after, listed))

    def test_workspace_lock_exception_preserves_all_other_scope_guards(self):
        for mutation in ("unexpected-path", "parent", "config", "missing", "created",
                         "post-list", "public-ready", "closure"):
            with self.subTest(mutation=mutation):
                self.fixture()
                target = self.workspace_lock()
                config = self.home / ".kimi-code/config.toml"
                config.write_bytes(b"telemetry = false\n")
                other = target.with_name("unrelated-empty-file")
                other.write_bytes(b"")
                other.chmod(0o600)
                if mutation == "created":
                    target.unlink()
                before = self.capture()
                self.assertIsNotNone(before)
                self.append()
                selected = other if mutation == "unexpected-path" else config if mutation == "config" else target
                temporary = selected.with_name(selected.name + ".tmp")
                temporary.write_bytes(selected.read_bytes() if selected.exists() else b"")
                temporary.chmod(0o600)
                os.replace(temporary, selected)
                if mutation == "parent":
                    selected.parent.chmod(stat.S_IMODE(selected.parent.stat().st_mode) ^ stat.S_IRGRP)
                elif mutation == "missing":
                    target.unlink()
                elif mutation == "public-ready":
                    self.repeated["readiness"] = "needs_action"
                elif mutation == "closure":
                    self.command["timeout"] = True
                after = self.capture()
                if mutation == "post-list":
                    target.write_bytes(b"changed after repeat")
                self.assertIsNone(self.classify(before, after, self.capture()))

    def test_same_inode_append_retains_ready_and_exports_only_hashes(self):
        self.fixture()
        before = self.capture()
        self.assertIsNotNone(before)
        self.assertEqual(before["native"], self.audit.snapshot(
            [self.home / name for name in self.audit.NATIVE_ROOTS["kimi"]]))
        self.append()
        after = self.capture()
        for status in ("unchanged", "succeeded"):
            self.repeated["items"][0]["status"] = status
            proof = self.classify(before, after, self.capture())
            self.assertEqual(proof["status"], "verified-owned-log-append")
            self.assertEqual(proof["path"], ".kimi-code/logs/kimi-code.log")
            self.assertTrue(proof["same_inode_mode_parents"])
            self.assertEqual(proof["after_bytes"] - proof["before_bytes"], len(b"catalog completed\n"))
            self.assertNotIn("body", proof)

    def test_refuses_every_other_mutation_and_post_list_write(self):
        for mutation in ("prefix", "truncate", "replace", "rotate", "mode", "parent-mode", "profile", "skill", "managed", "new-file", "new-directory", "post-list"):
            with self.subTest(mutation=mutation):
                self.fixture()
                before = self.capture()
                self.append()
                if mutation == "prefix":
                    self.log.write_bytes(b"changed original\n")
                elif mutation == "truncate":
                    self.log.write_bytes(b"")
                elif mutation == "replace":
                    replacement = self.log.with_suffix(".new")
                    replacement.write_bytes(self.log.read_bytes())
                    replacement.replace(self.log)
                elif mutation == "rotate":
                    self.log.rename(self.log.with_suffix(".log.1"))
                    self.log.write_bytes(b"new rotation\n")
                elif mutation == "mode":
                    self.log.chmod(0o400)
                elif mutation == "parent-mode":
                    original = stat.S_IMODE(self.log.parent.stat().st_mode)
                    self.log.parent.chmod(original ^ stat.S_IRGRP)
                    self.assertNotEqual(stat.S_IMODE(self.log.parent.stat().st_mode), original)
                elif mutation == "profile":
                    (self.home / "foreign").write_bytes(b"changed")
                elif mutation == "skill":
                    (self.project / ".kimi-code/skills/docs/SKILL.md").write_bytes(b"changed")
                elif mutation == "managed":
                    (self.project / ".uap/managed/package/payload").write_bytes(b"changed")
                elif mutation == "new-file":
                    (self.log.parent / "unexpected").write_bytes(b"new")
                elif mutation == "new-directory":
                    (self.log.parent / "unexpected").mkdir()
                after = self.capture()
                if mutation == "post-list":
                    self.append()
                self.assertIsNone(self.classify(before, after, self.capture()))

    def test_refuses_links_absence_oversize_and_read_race(self):
        for mutation in ("symlink", "hardlink", "absent", "oversize", "read-race"):
            with self.subTest(mutation=mutation):
                self.fixture()
                if mutation == "symlink":
                    target = self.home / "target"
                    self.log.rename(target)
                    self.log.symlink_to(target)
                elif mutation == "hardlink":
                    os.link(self.log, self.home / "linked")
                elif mutation == "absent":
                    self.log.unlink()
                elif mutation == "oversize":
                    with self.log.open("ab") as stream:
                        stream.truncate((1 << 20) + 1)
                if mutation == "read-race":
                    read = self.audit.copilot_settings_read
                    def changing(*args, **kwargs):
                        value = read(*args, **kwargs)
                        self.append()
                        return value
                    with patch.object(self.audit, "copilot_settings_read", changing):
                        self.assertIsNone(self.capture())
                else:
                    self.assertIsNone(self.capture())

    def test_ready_and_closed_command_remain_mandatory(self):
        self.fixture()
        before = self.capture()
        self.append()
        after = self.capture()
        for boundary in ("fresh", "repeat", "actions", "item", "exit", "timeout"):
            with self.subTest(boundary=boundary):
                added, repeated, command = copy.deepcopy(self.added), copy.deepcopy(self.repeated), dict(self.command)
                if boundary == "fresh":
                    added["readiness"] = "needs_action"
                elif boundary == "repeat":
                    repeated["readiness"] = "needs_action"
                elif boundary == "actions":
                    repeated["next_actions"] = [{"code": "verify_native"}]
                elif boundary == "item":
                    repeated["items"][0]["status"] = "completed"
                elif boundary == "exit":
                    command["exit_code"] = 1
                elif boundary == "timeout":
                    command["timeout"] = True
                self.assertIsNone(self.audit.kimi_repeat_log_append(
                    before, after, after, added, repeated, command, before["native"], after["native"]))

    def test_refuses_mutations_between_native_and_log_captures(self):
        for phase in ("before", "after"):
            for name in ("foreign", "logs/kimi-code.log"):
                with self.subTest(phase=phase, name=name):
                    self.fixture()
                    native = self.home / ".kimi-code"
                    neighbor = native / "foreign"
                    neighbor.write_bytes(b"original")
                    before_native = self.audit.snapshot([native, self.home / ".kimi"])
                    if phase == "before":
                        with (native / name).open("ab") as stream:
                            stream.write(b"unverified before capture\n")
                    before = self.capture()
                    self.append()
                    after = self.capture()
                    if phase == "after":
                        with (native / name).open("ab") as stream:
                            stream.write(b"unverified after capture\n")
                    after_native = self.audit.snapshot([native, self.home / ".kimi"])
                    self.assertIsNotNone(self.classify(before, after, after))
                    self.assertIsNone(self.classify(before, after, after,
                        native_before=before_native, native_after=after_native))

    def test_append_bound_matches_existing_removal_scan(self):
        for size in ((1 << 20) - 1, 1 << 20, (1 << 20) + 1):
            with self.subTest(size=size):
                self.fixture()
                self.log.write_bytes(b"x" * ((1 << 20) - 2))
                before = self.capture()
                self.assertIsNotNone(before)
                with self.log.open("ab") as stream:
                    stream.write(b"y" * (size - ((1 << 20) - 2)))
                after = self.capture()
                proof = self.classify(before, after, self.capture())
                native = self.home / ".kimi-code"
                leftovers = self.audit.native_identity_residuals(
                    [native], "removed-skill", self.audit.snapshot([native]), "kimi")
                if size <= 1 << 20:
                    self.assertIsNotNone(proof)
                    self.assertEqual(proof["after_bytes"], size)
                    self.assertEqual(leftovers, [])
                else:
                    self.assertIsNone(after)
                    self.assertIsNone(proof)
                    self.assertEqual(leftovers, [".kimi-code/logs/kimi-code.log: unscanned-too-large"])


    def test_capture_failure_facts_never_export_private_bytes_paths_or_errors(self):
        cases = {"reader": "log-read-refused", "unsafe": "log-read-unsafe", "absent": "log-absent",
                 "bound": "log-read-type-or-bound", "os-error": "filesystem-read-refused",
                 "path-error": "parent-or-path-refused", "scope": "scope-unreadable",
                 "parent": "log-parent-changed", "identity": "log-identity-changed",
                 "hash": "log-hash-mismatch", "late-identity": "log-identity-after-scope-changed",
                 "late-parent": "log-parent-after-scope-changed"}
        for mode, expected in cases.items():
            with self.subTest(mode=mode):
                self.fixture()
                canary = "private-profile-and-log-canary"
                self.log.write_text(canary)
                (self.home / canary).write_text(canary)
                read, scope = self.audit.copilot_settings_read, self.audit.scoped_snapshot
                def change_identity():
                    info = self.log.stat()
                    os.utime(self.log, ns=(info.st_atime_ns, info.st_mtime_ns + 1))
                def change_parent():
                    self.log.parent.chmod(stat.S_IMODE(self.log.parent.stat().st_mode) ^ stat.S_IRGRP)
                def observed_read(*args):
                    if mode in ("reader", "unsafe", "absent", "bound"):
                        return None, {"reader": canary, "unsafe": "unsafe-path", "absent": "absent",
                                      "bound": "not-regular-or-too-large"}[mode]
                    if mode == "os-error":
                        raise OSError(canary)
                    if mode == "path-error":
                        raise ValueError(canary)
                    result = read(*args)
                    if mode == "parent":
                        change_parent()
                    elif mode == "identity":
                        change_identity()
                    return result
                def observed_scope(*args):
                    if mode == "scope":
                        raise AssertionError(canary)
                    result = scope(*args)
                    if mode == "hash":
                        result[self.home.name + "/.kimi-code/logs/kimi-code.log"] = canary
                    elif mode == "late-identity":
                        change_identity()
                    elif mode == "late-parent":
                        change_parent()
                    return result
                facts = {}
                with patch.object(self.audit, "copilot_settings_read", observed_read), \
                     patch.object(self.audit, "scoped_snapshot", observed_scope):
                    self.assertIsNone(self.capture(facts))
                self.assertEqual(facts, {"reason": expected})
                diagnostic = self.audit.failed_case_diagnostics({"kimi_repeat_log_facts": facts})
                self.assertEqual(diagnostic["kimi_repeat_log_facts"], facts)
                self.assertNotIn(canary, json.dumps(diagnostic))
                self.assertNotIn(str(self.root), json.dumps(diagnostic))

    def test_protected_scope_diagnostics_identify_only_fixed_metadata_fields(self):
        for scope in ("home", "project-native", "managed"):
            with self.subTest(scope=scope):
                self.fixture()
                canary = "private-profile-path-and-content-canary"
                target = {"home": self.home / canary,
                          "project-native": self.project / ".kimi-code/skills/docs/SKILL.md",
                          "managed": self.project / ".uap/managed/package/payload"}[scope]
                target.write_text(canary)
                before = self.capture()
                self.append()
                info = target.stat()
                os.utime(target, ns=(info.st_atime_ns, info.st_mtime_ns + 1000000))
                after = self.capture()
                facts = {}
                with patch.object(Path, "lstat", side_effect=AssertionError("no diagnostic filesystem reads")):
                    self.assertIsNone(self.classify(before, after, after, facts=facts))
                self.assertEqual(facts["reason"], "protected-scope-changed")
                self.assertIn("scope_changes", facts)
                self.assertEqual(len(facts["scope_changes"]), 1)
                record = facts["scope_changes"][0]
                self.assertEqual(record["scope"], scope)
                self.assertEqual(record["key_kind"], "metadata")
                self.assertEqual(record["change_kind"], "changed")
                self.assertEqual(record["metadata_fields"], ["mtime_ns"])
                self.assertRegex(record["key_sha256"], r"^[0-9a-f]{64}$")
                published = json.dumps(self.audit.failed_case_diagnostics({"kimi_repeat_log_facts": facts}))
                self.assertNotIn(canary, published)
                self.assertNotIn(str(self.root), published)

    def test_protected_scope_diagnostics_cap_records_hash_keys_and_hide_values(self):
        self.fixture()
        before = self.capture()
        self.append()
        after = self.capture()
        canary = "private-path-and-value-canary"
        for index in range(10):
            key = f"metadata:2:{canary}-{index:02d}"
            before["scope"][key] = (stat.S_IFREG | 0o600, 11, 22, 33)
            after["scope"][key] = (stat.S_IFREG | 0o600, 11, 22, 34)
        facts = {}
        with patch.object(Path, "lstat", side_effect=AssertionError("no diagnostic filesystem reads")):
            self.assertIsNone(self.classify(before, after, after, facts=facts))
        self.assertIn("scope_changes", facts)
        self.assertEqual(len(facts["scope_changes"]), 8)
        self.assertEqual(facts["scope_changes_omitted"], 2)
        self.assertEqual(facts["changed_entries_capped"], 10)
        key = f"metadata:2:{canary}-00"
        self.assertEqual(facts["scope_changes"][0]["key_sha256"], self.audit.hashlib.sha256(key.encode()).hexdigest())
        self.assertTrue(all(record["metadata_fields"] == ["mtime_ns"] for record in facts["scope_changes"]))
        self.assertNotIn(canary, json.dumps(facts))
        self.assertNotIn(str(self.root), json.dumps(facts))
        self.assertLess(len(json.dumps(facts)), 2500)

    def test_protected_scope_surrogate_key_remains_private_refusal(self):
        self.fixture()
        before = self.capture()
        self.append()
        after = self.capture()
        # A single changed key ensures it reaches the hash, inside the cap.
        key = "metadata:0:private-name-" + chr(0xDCFF)
        before["scope"][key] = (stat.S_IFREG | 0o600, 11, 22, 33)
        after["scope"][key] = (stat.S_IFREG | 0o600, 11, 22, 34)
        facts = {}
        with patch.object(Path, "lstat", side_effect=AssertionError("no diagnostic filesystem reads")):
            self.assertIsNone(self.classify(before, after, after, facts=facts))
        self.assertEqual(facts["reason"], "protected-scope-changed")
        self.assertEqual(len(facts["scope_changes"]), 1)
        self.assertEqual(facts["scope_changes"][0]["key_sha256"],
                         self.audit.hashlib.sha256(key.encode("utf-8", errors="surrogatepass")).hexdigest())
        published = json.dumps(facts)
        self.assertNotIn("private-name", published)
        self.assertNotIn("\\udcff", published)
        self.assertLess(len(published), 500)

    def test_append_first_refusal_facts_are_bounded_without_changing_qualification(self):
        self.fixture()
        canary = "private-native-path-and-body-canary"
        self.log.write_text(canary)
        (self.home / canary).write_text(canary)
        captured = {}
        original = self.capture(captured)
        self.assertEqual(captured, {"reason": "captured", "log_bytes": len(canary)})
        self.append()
        completed = self.capture()
        cases = ("capture-unavailable-before", "capture-unavailable-after-add", "capture-unavailable-post-list",
                 "post-list-changed", "repeat-command-refused", "native-before-mismatch", "native-after-mismatch",
                 "fresh-public-refused", "repeat-public-refused", "log-identity-changed", "log-parent-changed",
                 "log-key-changed", "log-prefix-changed", "log-over-one-mib", "log-not-append", "protected-scope-changed")
        for reason in cases:
            with self.subTest(reason=reason):
                before, after = copy.deepcopy(original), copy.deepcopy(completed)
                listed, added, repeated, command = after, copy.deepcopy(self.added), copy.deepcopy(self.repeated), dict(self.command)
                native_before, native_after = before["native"], after["native"]
                if reason == "capture-unavailable-before":
                    before = None
                elif reason == "capture-unavailable-after-add":
                    after = None
                elif reason == "capture-unavailable-post-list":
                    listed = None
                elif reason == "post-list-changed":
                    listed = copy.deepcopy(after)
                    listed["body"] += canary.encode()
                elif reason == "repeat-command-refused":
                    command["exit_code"] = False
                elif reason == "native-before-mismatch":
                    native_before = {canary: canary}
                elif reason == "native-after-mismatch":
                    native_after = {canary: canary}
                elif reason == "fresh-public-refused":
                    added["readiness"] = "needs_action"
                elif reason == "repeat-public-refused":
                    repeated["readiness"] = "needs_action"
                elif reason == "log-identity-changed":
                    after["identity"] = (after["identity"][0] + 1, *after["identity"][1:])
                elif reason == "log-parent-changed":
                    after["parents"] = []
                elif reason == "log-key-changed":
                    after["log_key"] = canary
                elif reason == "log-prefix-changed":
                    after["body"] = b"different"
                elif reason == "log-over-one-mib":
                    after["body"] = before["body"] + b"x" * (1 << 20)
                elif reason == "log-not-append":
                    after["body"] = before["body"]
                elif reason == "protected-scope-changed":
                    after["scope"][canary] = canary
                arguments = (before, after, listed, added, repeated, command, native_before, native_after)
                facts = {}
                self.assertIsNone(self.audit.kimi_repeat_log_append(*arguments))
                self.assertIsNone(self.audit.kimi_repeat_log_append(*arguments, facts))
                self.assertEqual(facts["reason"], reason)
                if reason == "protected-scope-changed":
                    self.assertEqual(facts["changed_entries_capped"], 1)
                diagnostic = self.audit.failed_case_diagnostics({"kimi_repeat_log_facts": facts})
                self.assertNotIn(canary, json.dumps(diagnostic))
                self.assertLess(len(json.dumps(diagnostic)), 500)
        facts = {}
        expected = self.classify(original, completed, completed)
        with patch.object(Path, "lstat", side_effect=AssertionError("diagnostics must not reread files")):
            self.assertEqual(self.classify(original, completed, completed, facts=facts), expected)
        self.assertEqual(facts, {"reason": "verified-append"})


class NativeSnapshotReadTest(unittest.TestCase):
    def test_snapshot_refuses_replacement_before_open_and_closes_descriptors(self):
        mutations = ["symlink", "replacement"] + (["fifo"] if hasattr(os, "mkfifo") else [])
        for mutation in mutations:
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                audit = load_audit()
                root = Path(temporary) / "profile"
                root.mkdir()
                target = root / "entry"
                target.write_bytes(b"original")
                outside = Path(temporary) / "outside"
                outside.write_bytes(b"foreign bytes")
                open_member = audit.NATIVE_ARTIFACTS["open_member"]
                opened, reached = [], []
                def replace_before_open(path):
                    reached.append(path)
                    path.unlink()
                    if mutation == "symlink":
                        path.symlink_to(outside)
                    elif mutation == "fifo":
                        os.mkfifo(path)
                    else:
                        path.write_bytes(b"replacement")
                    descriptor = open_member(path)
                    opened.append(descriptor)
                    return descriptor
                with patch.dict(audit.NATIVE_ARTIFACTS, {"open_member": replace_before_open}):
                    found = audit.snapshot([root])
                self.assertEqual(reached, [target])
                self.assertEqual(found["profile/entry"], "non-regular" if mutation == "fifo" else "unreadable")
                for descriptor in opened:
                    with self.assertRaises(OSError):
                        os.fstat(descriptor)
                self.assertEqual(outside.read_bytes(), b"foreign bytes")

    def test_snapshot_refuses_a_write_during_the_held_read(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / "profile"
            root.mkdir()
            target = root / "entry"
            target.write_bytes(b"original")
            read, changed = os.read, []
            def write_after_read(descriptor, limit):
                body = read(descriptor, limit)
                if not changed:
                    changed.append(True)
                    target.write_bytes(b"different bytes")
                return body
            with patch.object(os, "read", write_after_read):
                self.assertEqual(audit.snapshot([root])["profile/entry"], "unreadable")
            self.assertTrue(changed)
