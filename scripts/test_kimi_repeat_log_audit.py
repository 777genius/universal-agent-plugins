"""Inert filesystem checks for one bounded owned Kimi diagnostic append."""
import copy
import os
import stat
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch

from test_live_install_audit_env import load_audit


class KimiRepeatLogAuditTest(unittest.TestCase):
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

    def capture(self):
        return self.audit.kimi_repeat_log_capture(self.home, self.project, self.root)

    def append(self):
        with self.log.open("ab") as stream:
            stream.write(b"catalog completed\n")

    def classify(self, before, after, listed):
        return self.audit.kimi_repeat_log_append(before, after, listed, self.added, self.repeated, self.command)

    def test_same_inode_append_retains_ready_and_exports_only_hashes(self):
        self.fixture()
        before = self.capture()
        self.assertIsNotNone(before)
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
                self.assertIsNone(self.audit.kimi_repeat_log_append(before, after, after, added, repeated, command))

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
