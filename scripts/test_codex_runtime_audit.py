"""Fail-closed tests for the Codex test auditor; these are not native proofs."""

import asyncio
import copy
from contextlib import closing
import errno
import json
import os
from pathlib import Path
import shutil
import signal
import sqlite3
import stat
import subprocess
import sys
import tempfile
import textwrap
import time
import unittest
from types import SimpleNamespace
from unittest.mock import patch
from runpy import run_path

import codex_runtime_audit as audit


class RemotionTemporaryRootTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="remotion-root-inert-")
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name).resolve(strict=True)
        self.root = self.base / "owned"
        self.root.mkdir()
        self.script = run_path(str(Path(__file__).with_name("remotion_windows_long_temp_e2e.py")))
        self.handoff = self.script["physical_temp_root"]

    def test_same_object_lexical_alias_uses_physical_root_before_baseline(self):
        # A real same-object lexical alias; this is not a Windows 8.3 proof.
        (self.base / "spelling").mkdir()
        alias = self.base / "spelling" / ".." / self.root.name
        before = self.root.lstat()
        with self.assertRaisesRegex(ValueError, "Codex audit directory is redirected"):
            audit._real_directory(alias)
        physical = self.handoff(alias)
        self.assertEqual(physical, self.root, "fresh TEMP spelling did not retain its physical object")
        self.assertTrue(os.path.samestat(before, audit._real_directory(physical)))
        self.assertEqual(self.root.lstat(), before)
        with self.assertRaisesRegex(ValueError, "Codex audit directory is redirected"):
            audit._real_directory(alias)

    def test_resolving_to_another_real_directory_refuses_original_identity(self):
        foreign = self.base / "foreign"
        foreign.mkdir()
        original_resolve = Path.resolve

        def redirected(path, *args, **kwargs):
            return foreign if path == self.root else original_resolve(path, *args, **kwargs)

        with patch.object(Path, "resolve", redirected):
            with self.assertRaisesRegex(ValueError, "differs from the created original"):
                self.handoff(self.root)
        self.assertTrue(self.root.is_dir() and foreign.is_dir())

    def test_original_reparse_or_symlink_metadata_refuses_before_resolving(self):
        original_lstat = Path.lstat
        for member in (self.root, self.base):
            for kind in ("reparse", "symlink"):
                with self.subTest(member=member.name, kind=kind):
                    def marked(path):
                        info = original_lstat(path)
                        if path != member:
                            return info
                        return SimpleNamespace(st_mode=(info.st_mode if kind == "reparse"
                            else stat.S_IFLNK | stat.S_IMODE(info.st_mode)),
                            st_file_attributes=0x400 if kind == "reparse" else 0,
                            st_reparse_tag=0)

                    with patch.object(Path, "lstat", marked), patch.object(Path, "resolve") as resolve:
                        with self.assertRaisesRegex(ValueError, "ancestor is redirected"):
                            self.handoff(self.root)
                        resolve.assert_not_called()

    def run_setup_failure(self, failed_handoff, replace=False, late_replace=False):
        """Exercise real main/creation/retention, without running a native case."""
        runner = self.base / "runner-temp"
        default = self.base / "default-temp-with-a-longer-physical-spelling"
        runner.mkdir()
        default.mkdir()
        cli = self.base / "inert-source-cli.exe"
        cli.write_bytes(b"never executed by this setup-only unit fixture")
        fixture = Path(__file__).resolve().parents[1] / "tests/live-install/fixtures.json"
        fixture_bytes = fixture.read_bytes()
        created, moved, late = [], [], []
        calls = 0
        capture_authority = self.script["temp_root_authority"]

        def create(**options):
            path = Path(tempfile.mkdtemp(prefix=options["prefix"], dir=options.get("dir", default)))
            created.append(path)
            return str(path)

        def replace_root(path):
            (path / "owned.txt").write_text("original owned content")
            retained = path.with_name(path.name + "-moved-original")
            path.rename(retained)
            moved.append(retained)
            path.mkdir()
            (path / "foreign.txt").write_text("foreign replacement must survive")

        def handoff(path, authority):
            nonlocal calls
            calls += 1
            if calls != failed_handoff:
                return self.handoff(path, authority)
            if late_replace:
                late.append(path)
            if not replace:
                raise ValueError("injected handoff refusal")
            replace_root(path)
            return self.handoff(path, authority)

        def observe_authority(path):
            observed = capture_authority(path)
            if path in late:
                late.remove(path)
                replace_root(path)  # Swap after the final authority observation.
            return observed

        native = SimpleNamespace(
            validate_fixtures=lambda _: None,
            do_case=lambda *args: self.fail("native case ran after TEMP setup failed"))
        loader = SimpleNamespace(exec_module=lambda _: None)
        imports = SimpleNamespace(util=SimpleNamespace(
            spec_from_file_location=lambda *args: SimpleNamespace(loader=loader),
            module_from_spec=lambda _: native))
        environment = SimpleNamespace(name="nt", environ={
            "RUNNER_TEMP": str(runner), "UAP_SOURCE_CLI": str(cli)})
        git = SimpleNamespace(check_output=lambda *args, **kwargs:
                              "inert-test-head" if kwargs.get("text") else fixture_bytes)
        main = self.script["main"]
        with patch.dict(main.__globals__, {
                "os": environment, "subprocess": git, "importlib": imports,
                "tempfile": SimpleNamespace(mkdtemp=create), "physical_temp_root": handoff,
                "temp_root_authority": observe_authority}), patch.object(shutil, "rmtree") as remove:
            with self.assertRaisesRegex(SystemExit, "TEMP setup"):
                main()
            remove.assert_not_called()
        report = runner / "remotion-windows-source-longtemp.json"
        self.assertTrue(report.is_file(), "TEMP setup failure lost its evidence")
        return json.loads(report.read_text()), created, moved

    def test_first_handoff_failure_reports_and_retains_unchanged_owned_root(self):
        evidence, created, _ = self.run_setup_failure(1)
        self.assertEqual(len(created), 1)
        self.assertTrue(created[0].is_dir())
        self.assertEqual(evidence["results"], {})
        self.assertEqual(evidence["temp_roots"]["runner-temp"]["cleanup"], "retained-for-evidence")
        self.assertEqual(evidence["temp_roots"]["runner-temp"]["created_root_chars"],
                         len(str(created[0] / "skill-24")))
        self.assertEqual(evidence["setup_error"]["error_type"], "ValueError")

    def test_second_handoff_failure_reports_and_retains_both_owned_roots(self):
        evidence, created, _ = self.run_setup_failure(2)
        self.assertEqual(len(created), 2)
        self.assertTrue(all(path.is_dir() for path in created))
        self.assertEqual({row["cleanup"] for row in evidence["temp_roots"].values()},
                         {"retained-for-evidence"})
        self.assertEqual(evidence["temp_roots"]["runner-temp"]["root_chars"],
                         len(str(created[0] / "skill-24")))
        self.assertEqual(evidence["temp_roots"]["default-windows-temp"]["created_root_chars"],
                         len(str(created[1] / "skill-24")))

    def test_changed_root_is_retained_and_foreign_replacement_is_not_removed(self):
        evidence, created, moved = self.run_setup_failure(2, replace=True)
        self.assertTrue(created[0].is_dir(), "verified first root was not retained")
        self.assertEqual((created[1] / "foreign.txt").read_text(), "foreign replacement must survive")
        self.assertEqual((moved[0] / "owned.txt").read_text(), "original owned content")
        retained = evidence["temp_roots"]["default-windows-temp"]
        self.assertEqual(retained["cleanup"], "retained-unverified")
        self.assertEqual(retained["created_path"], str(created[1]))
        self.assertEqual(retained["cleanup_error_type"], "ValueError")

    def test_late_swap_after_authority_observation_cannot_delete_foreign_bytes(self):
        evidence, created, moved = self.run_setup_failure(2, late_replace=True)
        self.assertTrue(all(path.is_dir() for path in created))
        self.assertEqual((created[1] / "foreign.txt").read_text(), "foreign replacement must survive")
        self.assertEqual((moved[0] / "owned.txt").read_text(), "original owned content")
        self.assertEqual(evidence["temp_roots"]["default-windows-temp"]["cleanup"],
                         "retained-for-evidence")


class RemotionRepeatNativePhaseTest(unittest.TestCase):
    def setUp(self):
        self.live = run_path(str(Path(__file__).with_name("live-install-audit.py")))

    def test_every_phase_keeps_the_original_snapshot_without_adopting_new_files(self):
        with tempfile.TemporaryDirectory(prefix="remotion-phases-inert-") as temporary:
            root = Path(temporary) / ".codex"
            root.mkdir()
            (root / "foreign").write_bytes(b"private original contents")
            before = self.live["snapshot"]([root])
            original = dict(before)
            snapshots = []
            for name in ("after-add", "after-list", "after-native"):
                member = root / ".tmp" / name / "HEAD"
                member.parent.mkdir(parents=True)
                member.write_bytes(b"ref: refs/heads/main\n")
                snapshots.append(self.live["snapshot"]([root]))
            phases = self.live["repeat_native_phase_evidence"](before, *snapshots)
            self.assertEqual([row["phase"] for row in phases], [
                "after-public-repeat-add", "after-public-repeat-list", "after-native-checkpoint"])
            self.assertEqual([row["total"] for row in phases], [1, 2, 3])
            for row in phases:
                self.assertEqual(row["omitted"], 0)
                self.assertTrue(all(entry["before"] is None for entry in row["entries"]))
                self.assertTrue(all(len(entry["after"]) == 64 for entry in row["entries"]))
            self.assertEqual(before, original)
            self.assertNotEqual(snapshots[-1], before, "the original final equality must still fail")
            text = json.dumps(phases)
            self.assertNotIn(temporary, text)
            self.assertNotIn("private original contents", text)
            self.assertNotIn("ref: refs/heads/main", text)

    def test_each_phase_retains_existing_bounds_and_unsafe_markers(self):
        long_path = ".codex/000-" + "x" * 3000
        before = {long_path: "symlink"}
        after = {f".codex/{index:03d}": "a" * 64 for index in range(1, 105)}
        after[long_path] = "unreadable"
        phases = self.live["repeat_native_phase_evidence"](before, after, after, after)
        self.assertEqual(len(phases), 3)
        for row in phases:
            self.assertEqual(row["total"], 105)
            self.assertEqual(row["omitted"], 5)
            self.assertEqual(len(row["entries"]), 100)
            first = row["entries"][0]
            self.assertEqual(len(first["path"]), 2048)
            self.assertTrue(first["path_truncated"])
            self.assertEqual((first["before"], first["after"]), ("symlink", "unreadable"))
        self.assertEqual(before, {long_path: "symlink"})

    def test_unchanged_phases_are_empty_and_failure_projection_preserves_verdict(self):
        before = {".codex/foreign": "a" * 64}
        phases = self.live["repeat_native_phase_evidence"](before, before, before, before)
        self.assertTrue(all(row["total"] == 0 and row["omitted"] == 0
                            and row["entries"] == [] for row in phases))
        failed = {"repeat_native_phases": phases, "repeat_native_unchanged": False,
                  "repeat_verified": False, "sentinels_preserved": True}
        evidence = self.live["failed_case_diagnostics"](failed)
        self.assertEqual(evidence, failed)
        self.assertFalse(evidence["repeat_verified"])
        self.assertFalse(evidence["repeat_native_unchanged"])



class CodexTemporaryGitStateTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="codex-git-audit-inert-")
        self.addCleanup(self.temporary.cleanup)
        self.profile = Path(self.temporary.name).resolve() / ".codex"
        self.profile.mkdir()
        self.root = self.profile / ".tmp"
        self.root.mkdir()
        self.snapshot = run_path(str(Path(__file__).with_name("live-install-audit.py")))["snapshot"]

    def repository(self, name="git-Abc123", directories=True):
        path = self.root / name
        path.mkdir()
        (path / "HEAD").write_bytes(audit.TEMPORARY_GIT_HEAD)
        if directories:
            (path / "objects").mkdir()
            (path / "refs").mkdir()
        return path

    def test_only_new_exact_empty_native_repositories_qualify(self):
        self.repository()
        previous = audit._temporary_git_state(self.profile)
        before = self.snapshot([self.profile])
        self.repository("git-Def456", directories=False)
        after = self.snapshot([self.profile])
        self.assertNotEqual(before, after, "raw native mutation must stay visible")
        self.assertTrue(audit._repeat_temporary_git_only(self.profile, before, after, previous))
        self.assertFalse(audit._repeat_temporary_git_only(self.profile, after, after,
                                                       audit._temporary_git_state(self.profile)))
        for path, value in (("config.toml", "foreign configuration"),
                            ("skills/foreign/SKILL.md", "foreign Skill")):
            file = self.profile / path
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text(value)
            self.assertFalse(audit._repeat_temporary_git_only(
                self.profile, before, self.snapshot([self.profile]), previous))
            file.unlink()

    def test_unqualified_names_report_bounded_escaped_listing_evidence(self):
        # Invalid/control names are listing inputs, never platform-specific files.
        names = ("foreign", "git-abc", "git-1234567", "git-Abc12_",
                 "git-\x00\n\r\t\x1b", "git-\ud800", "git-" + "\U0001f642" * 80)
        for name in names:
            with self.subTest(name=ascii(name)), patch.object(
                    audit, "_bounded_member_names", return_value=[name]):
                with self.assertRaises(ValueError) as caught:
                    audit._temporary_git_state(self.profile)
                message = str(caught.exception)
                self.assertTrue(message.startswith("Codex temporary Git directory name is not qualified; "))
                self.assertIn("checkpoint='direct-scan'", message)
                self.assertIn("basename=" + ascii(name[:64]), message)
                self.assertIn(f"basename_chars={len(name)}", message)
                self.assertIn(f"basename_truncated={len(name) > 64}", message)
                self.assertTrue(message.isascii())
                self.assertTrue(all(32 <= ord(character) < 127 for character in message))
                self.assertLess(len(message), 1200)
                self.assertNotIn(str(self.profile), message)
        # Both independently bounded fields remain below the wrapper's 1800 cap.
        diagnostic = audit._temporary_git_name_diagnostic("\U0001f642" * 1000, "\U0001f642" * 1000)
        self.assertLess(len(diagnostic), 1200)
        self.assertIn("basename_chars=1000", diagnostic)
        self.assertIn("basename_truncated=True", diagnostic)

    def test_existing_repository_removal_or_replacement_does_not_qualify(self):
        original = self.repository()
        previous = audit._temporary_git_state(self.profile)
        before = self.snapshot([self.profile])
        self.repository("git-Def456")
        retired = self.profile / "retired"
        original.rename(retired)
        self.assertFalse(audit._repeat_temporary_git_only(
            self.profile, before, self.snapshot([self.profile]), previous))
        self.repository()
        self.assertFalse(audit._repeat_temporary_git_only(
            self.profile, before, self.snapshot([self.profile]), previous))

    def test_pinned_sync_lock_stays_outside_the_allowed_repeat_delta(self):
        lock = self.root / "plugins.sync.lock"
        lock.touch()
        self.repository()
        previous = audit._temporary_git_state(self.profile)
        before = self.snapshot([self.profile])
        self.repository("git-Def456")
        self.assertTrue(audit._repeat_temporary_git_only(
            self.profile, before, self.snapshot([self.profile]), previous))
        self.assertNotIn(f"{self.profile.name}/{lock.relative_to(self.profile)}", previous[0])

        lock.rename(self.profile.parent / "retained-lock")
        self.assertFalse(audit._repeat_temporary_git_only(
            self.profile, before, self.snapshot([self.profile]), previous))
        lock.touch()
        after = self.snapshot([self.profile])
        lock_key = f"{self.profile.name}/{lock.relative_to(self.profile)}"
        self.assertEqual(before[lock_key], after[lock_key], "replacement retains the same raw bytes")
        self.assertFalse(audit._repeat_temporary_git_only(
            self.profile, before, after, previous))

    def test_sync_lock_must_be_empty_regular_and_exclusively_linked(self):
        lock = self.root / "plugins.sync.lock"
        lock.write_bytes(b"unexpected native state")
        with self.assertRaises(ValueError):
            audit._temporary_git_state(self.profile)
        lock.unlink()
        lock.mkdir()
        with self.assertRaises(ValueError):
            audit._temporary_git_state(self.profile)
        lock.rmdir()
        foreign = self.profile / "foreign-lock"
        foreign.touch()
        os.link(foreign, lock)
        with self.assertRaisesRegex(ValueError, "exclusively linked"):
            audit._temporary_git_state(self.profile)
        lock.unlink()
        # Exercise unsafe metadata even when Windows cannot create a symlink.
        lock.touch()
        original_lstat = Path.lstat
        info = lock.lstat()
        marked = SimpleNamespace(**{name: getattr(info, name) for name in dir(info) if name.startswith("st_")})
        marked.st_mode = stat.S_IFLNK | stat.S_IMODE(info.st_mode)
        marked.st_file_attributes = getattr(info, "st_file_attributes", 0) | 0x400

        def redirected(path):
            return marked if path == lock else original_lstat(path)

        with patch.object(Path, "lstat", redirected), patch.dict(
                audit._FILES, {"open_member": lambda path: self.fail("redirected lock must not be opened")}):
            with self.assertRaisesRegex(ValueError, "regular"):
                audit._temporary_git_state(self.profile)
        lock.unlink()
        try:
            lock.symlink_to(foreign)
        except OSError as error:
            if os.name != "nt" or getattr(error, "winerror", None) != 1314:
                raise
            return  # The deterministic metadata rejection above still ran.
        with self.assertRaises(ValueError):
            audit._temporary_git_state(self.profile)

    def test_new_sync_lock_is_not_a_qualified_repeat_addition(self):
        self.repository()
        previous = audit._temporary_git_state(self.profile)
        before = self.snapshot([self.profile])
        self.repository("git-Def456")
        (self.root / "plugins.sync.lock").touch()
        self.assertFalse(audit._repeat_temporary_git_only(
            self.profile, before, self.snapshot([self.profile]), previous))

    def test_sync_lock_added_after_raw_repeat_snapshot_is_refused(self):
        self.repository()
        previous = audit._temporary_git_state(self.profile)
        before = self.snapshot([self.profile])
        self.repository("git-Def456")
        after = self.snapshot([self.profile])
        lock = self.root / "plugins.sync.lock"
        lock.touch()
        lock_key = f"{self.profile.name}/{lock.relative_to(self.profile)}"
        self.assertNotIn(lock_key, before)
        self.assertNotIn(lock_key, after)
        self.assertFalse(audit._repeat_temporary_git_only(self.profile, before, after, previous))

    def test_unrecognized_names_members_and_head_bytes_fail(self):
        for name in ("foreign", "git-abc", "git-1234567"):
            path = self.repository(name)
            with self.assertRaisesRegex(ValueError, "directory name"):
                audit._temporary_git_state(self.profile)
            shutil.rmtree(path)
        path = self.repository()
        for name in ("config", "objects/object", "refs/heads/main"):
            member = path / name
            member.parent.mkdir(parents=True, exist_ok=True)
            member.write_bytes(b"foreign repository data")
            with self.assertRaises(ValueError):
                audit._temporary_git_state(self.profile)
            if name.startswith("refs/"):
                shutil.rmtree(path / "refs" / "heads")
            else:
                member.unlink()
        for body in (b"ref: refs/heads/other\n", b"ref: refs/heads/main\r\n", b"", b"x" * 4096):
            (path / "HEAD").write_bytes(body)
            with self.assertRaises(ValueError):
                audit._temporary_git_state(self.profile)

    def test_symlinks_and_hardlinked_heads_fail(self):
        path = self.repository()
        foreign = self.profile / "foreign"
        foreign.write_bytes(audit.TEMPORARY_GIT_HEAD)
        (path / "HEAD").unlink()
        os.link(foreign, path / "HEAD")
        with self.assertRaisesRegex(ValueError, "exclusively linked"):
            audit._temporary_git_state(self.profile)
        (path / "HEAD").unlink()
        try:
            (path / "HEAD").symlink_to(foreign)
        except OSError as error:
            if os.name == "nt":
                self.skipTest(f"Windows symlink privilege unavailable: {error}")
            raise
        with self.assertRaises(ValueError):
            audit._temporary_git_state(self.profile)

    def test_unreadable_or_replaced_namespace_is_refused(self):
        path = self.repository()
        original_scandir = os.scandir
        def unreadable(member):
            if Path(member) == path / "refs":
                raise PermissionError("injected native scratch enumeration error")
            return original_scandir(member)
        with patch.object(os, "scandir", unreadable):
            with self.assertRaises(PermissionError):
                audit._temporary_git_state(self.profile)
        read = audit._regular_bytes
        def replaced(member, limit):
            body = read(member, limit)
            if member == path / "HEAD":
                path.rename(self.profile / "retained-original")
                self.repository()
            return body
        with patch.object(audit, "_regular_bytes", replaced):
            with self.assertRaisesRegex(ValueError, "changed during inspection"):
                audit._temporary_git_state(self.profile)

    def test_member_added_after_last_enumeration_is_refused(self):
        self.repository()
        previous = audit._temporary_git_state(self.profile)
        before = self.snapshot([self.profile])
        added = self.repository("git-Def456")
        after = self.snapshot([self.profile])
        original = audit._bounded_member_names
        calls = 0

        def late_member(path, limit):
            nonlocal calls
            names = original(path, limit)
            if path == added:
                calls += 1
                if calls == 2:
                    (path / "config").write_bytes(b"[inert]\nforeign=true\n")
            return names

        with patch.object(audit, "_bounded_member_names", side_effect=late_member):
            with self.assertRaisesRegex(ValueError, "changed during inspection"):
                audit._repeat_temporary_git_only(self.profile, before, after, previous)
        self.assertEqual(calls, 2)
        self.assertTrue((added / "config").is_file())
        self.assertNotEqual(self.snapshot([self.profile]), after)

    def test_repository_count_is_bounded(self):
        self.repository()
        self.repository("git-Def456")
        with patch.object(audit, "TEMPORARY_GIT_LIMIT", 1):
            with self.assertRaisesRegex(ValueError, "member count"):
                audit._temporary_git_state(self.profile)


class CodexRuntimeStateTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.runtime, self.scratch = self.root / "runtime", self.root / "scratch"
        self.runtime.mkdir()
        self.scratch.mkdir()
        self.database = self.runtime / "state_5.sqlite"
        connection = sqlite3.connect(self.database)
        connection.execute("CREATE TABLE fixture_rows (id INTEGER PRIMARY KEY, value TEXT)")
        connection.execute("INSERT INTO fixture_rows VALUES (1, 'unchanged')")
        connection.commit()
        connection.close()
        self.baseline, _ = audit.runtime_state(self.runtime, self.scratch)

    def check(self):
        return audit.runtime_state(self.runtime, self.scratch, self.baseline)

    def test_wal_representation_changes_require_identical_full_logical_state(self):
        connection = sqlite3.connect(self.database)
        self.addCleanup(connection.close)
        connection.execute("PRAGMA journal_mode=WAL")
        connection.execute("UPDATE fixture_rows SET value='unchanged'")
        connection.commit()
        first, files = self.check()
        self.assertIn("state_5.sqlite-wal", files)
        connection.execute("UPDATE fixture_rows SET value='transient'")
        connection.execute("UPDATE fixture_rows SET value='unchanged'")
        connection.commit()
        second, changed = self.check()
        self.assertEqual(first, second)
        self.assertNotEqual(files, changed)
        self.assertEqual(list(self.scratch.iterdir()), [])

    def test_row_schema_and_persistent_metadata_mutations_fail(self):
        for sql, message in (
                ("UPDATE fixture_rows SET value='foreign plugin registration'", "contents changed"),
                ("CREATE TABLE active_plugins (plugin TEXT)", "schema changed"),
                ("PRAGMA user_version=8", "contents changed")):
            with self.subTest(sql=sql):
                saved = self.database.read_bytes()
                with closing(sqlite3.connect(self.database)) as connection:
                    connection.execute(sql)
                    connection.commit()
                with self.assertRaisesRegex(ValueError, message):
                    self.check()
                self.database.write_bytes(saved)

    def test_unknown_active_config_and_subdirectories_fail(self):
        for name, directory in (("config.toml", False), ("plugins", True)):
            with self.subTest(name=name):
                path = self.runtime / name
                if directory:
                    path.mkdir()
                else:
                    path.write_text('[plugins.foreign]\nenabled=true\n')
                with self.assertRaisesRegex(ValueError, "unexpected member"):
                    self.check()
                path.rmdir() if directory else path.unlink()

    def test_chained_companion_suffixes_cannot_hide_uninterpreted_payloads(self):
        for suffix in ("-shm-wal", "-wal-shm", "-wal-wal", "-shm-shm", "-journal"):
            with self.subTest(suffix=suffix):
                path = self.runtime / (self.database.name + suffix)
                path.write_bytes(b"foreign plugin configuration SQLite would never read")
                with self.assertRaisesRegex(ValueError, "unexpected member"):
                    self.check()
                path.unlink()

    def test_new_database_and_missing_primary_database_fail(self):
        extra = self.runtime / "thread_history_1.sqlite"
        shutil.copyfile(self.database, extra)
        with self.assertRaisesRegex(ValueError, "inventory changed"):
            self.check()
        extra.unlink()
        self.database.rename(self.runtime / "state_5.sqlite-wal")
        with self.assertRaisesRegex(ValueError, "unexpected member"):
            self.check()

    def test_large_unreadable_or_corrupt_files_fail(self):
        saved = self.database.read_bytes()
        with patch.object(audit, "FILE_LIMIT", len(saved) - 1):
            with self.assertRaisesRegex(ValueError, "bounded"):
                self.check()
        with patch.dict(audit._FILES, {"open_member": lambda _: (_ for _ in ()).throw(PermissionError("denied"))}):
            with self.assertRaises(PermissionError):
                self.check()
        self.database.write_bytes(b"not a SQLite database")
        with self.assertRaises(sqlite3.DatabaseError):
            self.check()

    def test_total_and_semantic_row_limits_fail(self):
        with patch.object(audit, "TOTAL_LIMIT", 1):
            with self.assertRaisesRegex(ValueError, "bounded"):
                self.check()
        with closing(sqlite3.connect(self.database)) as connection:
            connection.execute("INSERT INTO fixture_rows VALUES (2, 'extra')")
            connection.commit()
        with patch.object(audit, "ROW_LIMIT", 1):
            with self.assertRaisesRegex(ValueError, "semantic audit bound"):
                self.check()

    def test_hardlink_cannot_adopt_foreign_bytes(self):
        foreign = self.root / "foreign.sqlite"
        self.database.rename(foreign)
        os.link(foreign, self.database)
        with self.assertRaisesRegex(ValueError, "exclusively linked"):
            self.check()
        self.assertTrue(foreign.is_file())

    def test_symlink_member_and_runtime_root_are_refused(self):
        foreign = self.root / "foreign.sqlite"
        self.database.rename(foreign)
        try:
            self.database.symlink_to(foreign)
        except OSError as error:
            self.skipTest(str(error))
        with self.assertRaises(ValueError):
            self.check()
        self.database.unlink()
        foreign.rename(self.database)
        moved = self.root / "moved-runtime"
        self.runtime.rename(moved)
        self.runtime.symlink_to(moved, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "redirected"):
            self.check()

    def test_replacement_root_with_identical_bytes_is_not_adopted(self):
        original = audit._database_state
        moved = self.root / "moved-runtime"

        def replace_after_copy(path, expected):
            self.runtime.rename(moved)
            shutil.copytree(moved, self.runtime)
            return original(path, expected)

        with patch.object(audit, "_database_state", side_effect=replace_after_copy):
            with self.assertRaisesRegex(ValueError, "changed while its copy"):
                self.check()
        self.assertEqual((moved / self.database.name).read_bytes(), self.database.read_bytes())


class CodexNativeQualificationTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.project = Path(self.temporary.name).resolve()
        self.selected = self.project / ".agents/skills/selected/SKILL.md"
        self.foreign = self.project / ".agents/skills/foreign-neighbor/SKILL.md"
        for path in (self.selected, self.foreign):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("inert native inventory fixture\n")
        self.listing = {"data": [{"cwd": str(self.project), "errors": [], "skills": [
            {"name": "selected", "path": str(self.selected), "scope": "repo", "enabled": True, "pluginId": None},
            {"name": "foreign-neighbor", "path": str(self.foreign), "scope": "repo", "enabled": True, "pluginId": None},
        ]}]}
        self.added = {"schema_version": 1, "command": "skills add", "result": "success", "status": "succeeded",
                      "scope": "project", "readiness": "ready", "next_actions": [],
                      "items": [{"alias": "selected", "targetId": "codex", "status": "succeeded"}]}
        self.matches = [{"clients": [{"client_id": "codex", "scope": "project", "materialization": "materialized",
                                     "activation": "active", "verification": "installation_verified",
                                     "authentication": "not_required", "policy": "allowed"}]}]

    def verify(self, listing, present=True):
        audit.verify_skill_inventory(listing, self.project, "selected", self.selected, self.foreign, present)

    def test_presence_absence_and_foreign_inventory_are_all_required(self):
        self.verify(self.listing)
        absent = copy.deepcopy(self.listing)
        absent["data"][0]["skills"].pop(0)
        self.verify(absent, False)
        with self.assertRaisesRegex(ValueError, "still natively discoverable"):
            self.verify(self.listing, False)
        with self.assertRaises(ValueError):
            self.verify(absent, True)
        no_foreign = copy.deepcopy(self.listing)
        no_foreign["data"][0]["skills"].pop()
        with self.assertRaises(ValueError):
            self.verify(no_foreign)

    def test_wrong_path_scope_enablement_and_duplicate_rows_fail(self):
        for key, value in (("path", str(self.foreign)), ("scope", "user"), ("enabled", False), ("pluginId", "other@market")):
            with self.subTest(key=key):
                listing = copy.deepcopy(self.listing)
                listing["data"][0]["skills"][0][key] = value
                with self.assertRaises(ValueError):
                    self.verify(listing)
        for change in ("project", "duplicate", "errors"):
            listing = copy.deepcopy(self.listing)
            if change == "project":
                listing["data"][0]["cwd"] = str(self.project.parent)
            elif change == "duplicate":
                listing["data"].append(copy.deepcopy(listing["data"][0]))
            else:
                listing["data"][0]["errors"] = [{"message": "failed"}]
            with self.assertRaises(ValueError):
                self.verify(listing)

    def test_samefile_hardlinks_at_another_native_location_do_not_qualify(self):
        self.verify(self.listing)
        with tempfile.TemporaryDirectory() as temporary:
            elsewhere = Path(temporary).resolve() / self.selected.name
            renamed = self.selected.with_name("different-skill.md")
            for linked in (elsewhere, renamed):
                with self.subTest(path=str(linked)):
                    os.link(self.selected, linked)
                    try:
                        self.assertTrue(linked.samefile(self.selected))
                        listing = copy.deepcopy(self.listing)
                        listing["data"][0]["skills"][0]["path"] = str(linked)
                        with self.assertRaises(ValueError):
                            self.verify(listing)
                    finally:
                        linked.unlink()
        self.verify(self.listing)

    def test_public_ready_requires_exact_native_states(self):
        self.assertTrue(audit.public_skill_ready(self.added, self.matches, "selected"))
        repeat = copy.deepcopy(self.added)
        repeat["items"][0]["status"] = "unchanged"
        self.assertTrue(audit.public_skill_ready(repeat, self.matches, "selected"))
        for key, value in (("activation", "prepared"), ("verification", "package_validated"),
                           ("authentication", "not_checked"), ("scope", "user"), ("policy", "unknown")):
            with self.subTest(key=key):
                matches = copy.deepcopy(self.matches)
                matches[0]["clients"][0][key] = value
                self.assertFalse(audit.public_skill_ready(self.added, matches, "selected"))
        for key, value in (("readiness", "needs_action"), ("scope", "user"), ("status", "partial"),
                           ("next_actions", [{"code": "verify_native"}]), ("result", "failure")):
            with self.subTest(key=key):
                added = copy.deepcopy(self.added)
                added[key] = value
                self.assertFalse(audit.public_skill_ready(added, self.matches, "selected"))


class CodexSkillProfileGuardTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.case = Path(self.temporary.name).resolve()
        self.home, self.project = self.case / "home", self.case / "project"
        self.profile = self.home / ".codex"
        for path in (self.home, self.profile, self.project, self.case / "tmp"):
            path.mkdir()
        self.foreign = self.project / ".agents/skills/foreign-neighbor/SKILL.md"
        self.foreign.parent.mkdir(parents=True)
        self.foreign.write_text("independently owned fixture\n", encoding="utf-8")
        self.selected = self.project / ".agents/skills/selected/SKILL.md"
        # Mock only the finite native exchange. Constructor, four-phase guard,
        # exact inventory checks and complete SQLite reads remain real.
        observer = patch.object(audit, "_exchange", side_effect=self.discover)
        self.exchange = observer.start()
        self.addCleanup(observer.stop)
        self.observer = audit.CodexRuntimeAudit(
            Path(sys.executable), self.case, self.home, self.project, {}, "selected", self.foreign)
        self.original = self.observer.config_bytes

    async def discover(self, binary, project, profile, environment):
        self.assertEqual(binary, Path(sys.executable))
        self.assertEqual(project, self.project)
        self.assertEqual(profile, self.profile)
        self.assertEqual(environment["CODEX_HOME"], str(self.profile))
        database = self.home / ".local/state/codex-sqlite/state_5.sqlite"
        if not database.exists():
            with closing(sqlite3.connect(database)) as connection:
                connection.execute("CREATE TABLE fixture_rows (id INTEGER PRIMARY KEY, value TEXT)")
                connection.execute("INSERT INTO fixture_rows VALUES (1, 'unchanged')")
                connection.commit()
        skills = [{"name": "foreign-neighbor", "path": str(self.foreign),
                   "scope": "repo", "enabled": True, "pluginId": None}]
        if self.selected.exists():
            skills.append({"name": "selected", "path": str(self.selected),
                           "scope": "repo", "enabled": True, "pluginId": None})
        return {"data": [{"cwd": str(self.project), "errors": [], "skills": skills}]}

    def test_raw_skill_lifecycle_requires_unchanged_profile_without_trust(self):
        baseline = copy.deepcopy(self.observer.baseline)
        self.selected.parent.mkdir()
        self.selected.write_text("inert selected Skill\n", encoding="utf-8")
        self.assertTrue(self.observer.checkpoint("installed", True))
        self.assertTrue(self.observer.checkpoint("repeat", True))
        self.selected.unlink()
        self.assertTrue(self.observer.checkpoint("removed", False))
        self.assertEqual(self.exchange.call_count, 4)
        self.assertEqual(self.observer.config.read_bytes(), self.original)
        self.assertEqual(self.observer.baseline, baseline)
        self.assertEqual(self.foreign.read_text(encoding="utf-8"), "independently owned fixture\n")
        observations = self.observer.evidence()["observations"]
        self.assertEqual([row["phase"] for row in observations],
                         ["before-install", "installed", "repeat", "removed"])
        self.assertEqual([row["selected_skill_present"] for row in observations], [False, True, True, False])
        self.assertTrue(all(row["foreign_skill_present"] and row["native_exit_code"] == 0
                            and row["runtime_integrity_verified"] and row["runtime_semantics_unchanged"]
                            and set(row["runtime_files_fully_read"]) == {"state_5.sqlite"}
                            for row in observations))

    def test_temporary_git_name_refusal_keeps_actual_checkpoint_and_baseline(self):
        rejected = self.profile / ".tmp" / "unqualified"
        rejected.mkdir(parents=True)
        baseline = copy.deepcopy(self.observer.baseline)
        observations = copy.deepcopy(self.observer.observations)
        for phase, present in (("before-install", False), ("installed", True),
                               ("repeat-installed", True), ("removed", False)):
            with self.subTest(phase=phase):
                if present:
                    self.selected.parent.mkdir(exist_ok=True)
                    self.selected.write_text("inert selected Skill\n", encoding="utf-8")
                elif self.selected.exists():
                    self.selected.unlink()
                calls = self.exchange.call_count
                with self.assertRaises(ValueError) as caught:
                    self.observer.checkpoint(phase, present)
                self.assertIn("checkpoint=" + ascii(phase), str(caught.exception))
                self.assertIn("basename='unqualified'", str(caught.exception))
                self.assertEqual(self.exchange.call_count, calls + 1)
                self.assertEqual(self.observer.baseline, baseline)
                self.assertEqual(self.observer.observations, observations)
                self.assertEqual(self.observer.config.read_bytes(), self.original)
                self.assertEqual(self.foreign.read_text(encoding="utf-8"), "independently owned fixture\n")

    def test_temporary_git_repeat_refusals_keep_context_without_another_exchange(self):
        rejected = self.profile / ".tmp" / "unqualified"
        rejected.mkdir(parents=True)
        baseline = copy.deepcopy(self.observer.baseline)
        observations = copy.deepcopy(self.observer.observations)
        calls = self.exchange.call_count
        for phase, operation in (("before-repeat", self.observer.temporary_git_state),
                                 ("repeat-comparison", lambda: self.observer.repeat_runtime_only({}, {}, ({}, {})))):
            with self.subTest(phase=phase), self.assertRaises(ValueError) as caught:
                operation()
            self.assertIn("checkpoint=" + ascii(phase), str(caught.exception))
            self.assertIn("basename='unqualified'", str(caught.exception))
            self.assertEqual(self.exchange.call_count, calls)
            self.assertEqual(self.observer.baseline, baseline)
            self.assertEqual(self.observer.observations, observations)
            self.assertEqual(self.observer.config.read_bytes(), self.original)
            self.assertEqual(self.foreign.read_text(encoding="utf-8"), "independently owned fixture\n")

    def test_all_unexpected_profile_bytes_fail_before_or_after_discovery(self):
        trust = ("\n[projects." + json.dumps(str(self.project)) + ']\ntrust_level = "trusted"\n').encode()
        cases = {
            "selected trust": self.original + trust,
            "formatting": self.original + b"\n",
            "unknown preference": self.original + b'model = "foreign"\n',
            "runtime changed": self.original.replace(b"codex-sqlite", b"foreign-sqlite"),
            "runtime omitted": b"",
        }
        calls = self.exchange.call_count
        for name, body in cases.items():
            with self.subTest(name=name):
                self.observer.config.write_bytes(body)
                with self.assertRaisesRegex(ValueError, "runtime configuration changed"):
                    self.observer.checkpoint("refused", False)
                self.assertEqual(self.exchange.call_count, calls)
                self.observer.config.write_bytes(self.original)
                self.observer.guard()

        async def mutate_during_discovery(*args):
            listing = await self.discover(*args)
            self.observer.config.write_bytes(self.original + trust)
            return listing

        with patch.object(audit, "_exchange", side_effect=mutate_during_discovery):
            with self.assertRaisesRegex(ValueError, "runtime configuration changed"):
                self.observer.checkpoint("changed-during-discovery", False)
        self.assertEqual(len(self.observer.observations), 1)

    def test_redirected_configuration_is_rejected_without_a_native_process(self):
        foreign = self.case / "foreign-config.toml"
        self.observer.config.rename(foreign)
        calls = self.exchange.call_count
        os.link(foreign, self.observer.config)
        try:
            with self.assertRaisesRegex(ValueError, "exclusively linked"):
                self.observer.checkpoint("hardlinked-config", False)
            self.assertEqual(self.exchange.call_count, calls)
        finally:
            self.observer.config.unlink()
        try:
            self.observer.config.symlink_to(foreign)
        except OSError as error:
            self.skipTest(str(error))
        with self.assertRaises(ValueError):
            self.observer.checkpoint("redirected-config", False)
        self.assertEqual(self.exchange.call_count, calls)
        self.assertEqual(foreign.read_bytes(), self.original)


def process_running(pid):
    if os.name == "nt":
        import ctypes
        from ctypes import wintypes
        api = ctypes.WinDLL("kernel32", use_last_error=True)
        api.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
        api.OpenProcess.restype = wintypes.HANDLE
        api.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
        api.WaitForSingleObject.restype = wintypes.DWORD
        api.CloseHandle.argtypes = [wintypes.HANDLE]
        api.CloseHandle.restype = wintypes.BOOL
        handle = api.OpenProcess(0x00100000, False, pid)  # SYNCHRONIZE, no mutation.
        if not handle:
            if ctypes.get_last_error() == 87:
                return False
            raise ctypes.WinError(ctypes.get_last_error())
        try:
            result = api.WaitForSingleObject(handle, 0)
            if result not in (0, 258):
                raise RuntimeError("owned test process liveness is unknown")
            return result == 258
        finally:
            api.CloseHandle(handle)
    try:
        status = Path(f"/proc/{pid}/stat")
        if status.is_file() and status.read_text().rsplit(")", 1)[1].split()[0] in ("Z", "X"):
            return False
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False


class CodexNativeProcessCleanupTest(unittest.TestCase):
    def observed_exit(self, exit_code, stderr):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            profile = root / "profile"
            profile.mkdir()
            # Valid finite protocol replies isolate the exit/diagnostic gate;
            # this real process fixture is not native Codex discovery evidence.
            script = textwrap.dedent("""
                import json, os, sys
                first = json.loads(sys.stdin.readline())
                print(json.dumps({'id': first['id'], 'result': {
                    'codexHome': os.environ['CODEX_HOME'], 'userAgent': 'agentplugins/0.153.4 (fixture)'}}), flush=True)
                json.loads(sys.stdin.readline())
                second = json.loads(sys.stdin.readline())
                print(json.dumps({'id': second['id'], 'result': {'data': []}}), flush=True)
                sys.stdin.read()
            """)
            script += f"sys.stderr.buffer.write(bytes.fromhex({stderr.hex()!r}))\n"
            script += f"sys.stderr.buffer.flush()\nraise SystemExit({exit_code})\n"
            (root / "app-server").write_text(script, encoding="utf-8")
            environment = {key: value for key, value in os.environ.items()
                           if key.upper() in {"PATH", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR"}}
            environment.update(HOME=str(root), USERPROFILE=str(root), CODEX_HOME=str(profile))
            return asyncio.run(audit._exchange(Path(sys.executable), root, profile, environment))

    def test_nonzero_exit_still_refuses_with_observed_diagnostic(self):
        self.assertEqual(self.observed_exit(0, b""), {"data": []})
        with self.assertRaises(ValueError) as raised:
            self.observed_exit(7, b"")
        self.assertIn("did not exit cleanly: exit_code=7, stderr_bytes=0, stderr_prefix=b''", str(raised.exception))

    def test_stderr_still_refuses_with_bounded_escaped_diagnostic(self):
        stderr = b"\x1b[31mfirst\n" + b"x" * 4096 + b"omitted-tail"
        with self.assertRaises(ValueError) as raised:
            self.observed_exit(0, stderr)
        message = str(raised.exception)
        self.assertIn(f"exit_code=0, stderr_bytes={len(stderr)}", message)
        self.assertIn("\\x1b[31mfirst\\n", message)
        self.assertNotIn("\x1b", message)
        self.assertNotIn("omitted-tail", message)
        self.assertLess(len(message), 4 * 2048 + 160)

    def test_exited_launcher_with_child_held_pipes_cannot_pass_or_leak(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            profile = root / "profile"
            profile.mkdir()
            # Real process behavior, deliberately not a native-client proof:
            # the launcher emits valid replies and exits, leaving a child that
            # holds both pipes. A direct parent-only kill cannot clean this up.
            (root / "app-server").write_text(textwrap.dedent("""
                import json, os, subprocess, sys
                from pathlib import Path
                first = json.loads(sys.stdin.readline())
                print(json.dumps({'id': first['id'], 'result': {
                    'codexHome': os.environ['CODEX_HOME'], 'userAgent': 'agentplugins/0.153.4 (fixture)'}}), flush=True)
                json.loads(sys.stdin.readline())
                second = json.loads(sys.stdin.readline())
                child = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(120)'], stdin=subprocess.DEVNULL)
                Path('owned-pids.json').write_text(json.dumps({'launcher': os.getpid(), 'child': child.pid}))
                print(json.dumps({'id': second['id'], 'result': {'data': []}}), flush=True)
                os._exit(0)
            """), encoding="utf-8")
            environment = {key: value for key, value in os.environ.items()
                           if key.upper() in {"PATH", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR"}}
            environment.update(HOME=str(root), USERPROFILE=str(root), CODEX_HOME=str(profile))
            child_pid = None
            started = time.monotonic()
            processes, pipes = [], []
            start_native = audit._start_native

            async def capture_owned_pipes(*args):
                process, job = await start_native(*args)
                processes.append(process)  # Prevent GC from supplying cleanup.
                for descriptor in (1, 2):
                    pipe = process._transport.get_pipe_transport(descriptor).get_extra_info("pipe")
                    pipes.append((descriptor, pipe, pipe.fileno()))
                return process, job

            async def exercise_fault():
                with patch.object(audit, "_start_native", side_effect=capture_owned_pipes):
                    with self.assertRaises(TimeoutError):
                        await audit._exchange(Path(sys.executable), root, profile, environment)
                self.assertEqual([descriptor for descriptor, _, _ in pipes], [1, 2])
                # Check at the observer boundary, before asyncio.run closes the
                # loop or garbage collection can close a retained pipe object.
                for descriptor, pipe, owned_fd in pipes:
                    with self.subTest(descriptor=descriptor):
                        with self.assertRaises(ValueError):
                            pipe.fileno()
                        if os.name != "nt":
                            with self.assertRaises(OSError) as closed:
                                os.fstat(owned_fd)
                            self.assertEqual(closed.exception.errno, errno.EBADF)

            try:
                asyncio.run(exercise_fault())
                import json
                pids = json.loads((root / "owned-pids.json").read_text())
                child_pid = pids["child"]
                deadline = time.monotonic() + 3
                while process_running(child_pid) and time.monotonic() < deadline:
                    time.sleep(.02)
                self.assertFalse(process_running(pids["launcher"]))
                self.assertFalse(process_running(child_pid), "native observer leaked a child after its launcher exited")
                self.assertLess(time.monotonic() - started, 30)
            finally:
                marker = root / "owned-pids.json"
                if child_pid is None and marker.exists():
                    import json
                    child_pid = json.loads(marker.read_text())["child"]
                if child_pid is not None and process_running(child_pid):
                    if os.name == "nt":
                        subprocess.run(["taskkill", "/PID", str(child_pid), "/T", "/F"],
                                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=5, check=False)
                    else:
                        os.kill(child_pid, signal.SIGKILL)


if __name__ == "__main__":
    unittest.main()
