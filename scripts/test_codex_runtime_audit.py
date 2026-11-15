"""Fail-closed tests for the Codex test auditor; these are not native proofs."""

import asyncio
import copy
from contextlib import closing
import json
import os
from pathlib import Path
import shutil
import signal
import sqlite3
import subprocess
import sys
import tempfile
import textwrap
import time
import unittest
from unittest.mock import patch
from runpy import run_path

import codex_runtime_audit as audit


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


class CodexProjectTrustGuardTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.case = Path(self.temporary.name).resolve()
        self.home, self.project = self.case / "home", self.case / "project"
        profile = self.home / ".codex"
        for path in (self.home, profile, self.project, self.case / "tmp"):
            path.mkdir()
        foreign = self.project / ".agents/skills/foreign-neighbor/SKILL.md"
        foreign.parent.mkdir(parents=True)
        foreign.write_text("independently owned fixture\n")
        existing = run_path(str(Path(__file__).with_name("live-install-audit.py")))
        # Only native subprocess discovery is replaced; the real constructor,
        # path/FD checks and parsed full-configuration guard remain under test.
        with patch.object(audit.CodexRuntimeAudit, "checkpoint", return_value=True):
            self.observer = audit.CodexRuntimeAudit(
                "/TEST/codex", self.case, self.home, self.project, {}, "selected", foreign,
                existing["codex_marketplace_source_in_case"])
        self.original = self.observer.config_bytes.decode()
        self.observer.guard()

    def configured(self, project=None, trust="trusted", extra=""):
        return (self.original + "\n[projects." + json.dumps(str(project or self.project))
                + "]\ntrust_level = " + json.dumps(trust) + "\n" + extra)

    def write(self, body):
        self.observer.config.write_text(body, encoding="utf-8")

    def test_exact_trust_cannot_precede_confirmed_install_boundary(self):
        self.write(self.configured())
        with self.assertRaisesRegex(ValueError, "outside the confirmed installation"):
            self.observer.guard()
        with self.assertRaises(ValueError):
            self.observer.begin_install()
        self.assertFalse(self.observer.trust_authorized)

    def test_authorized_grant_is_required_for_install_and_retained_on_remove(self):
        self.observer.begin_install()
        self.observer.guard()  # A refused add need not have granted trust.
        with patch.object(audit, "_exchange") as exchange:
            with self.assertRaisesRegex(ValueError, "trust is missing"):
                self.observer.checkpoint("installed", True)
            exchange.assert_not_called()
        self.write(self.configured())
        self.observer.guard(require_trust=True)
        self.observer.guard()  # Repeat/removal retain this persistent preference.
        self.assertTrue(self.observer.trust_observed)
        self.write(self.original)
        with self.assertRaisesRegex(ValueError, "revoked"):
            self.observer.guard()

    def test_only_exact_trust_and_unchanged_full_runtime_configuration_are_admitted(self):
        self.observer.begin_install()
        other = self.case / "other-project"
        other.mkdir()
        cases = {
            "other project": self.configured(other),
            "broader case": self.configured(self.case),
            "relative project": self.configured(Path("project")),
            "untrusted": self.configured(trust="untrusted"),
            "extra preference": self.configured(extra="extra = true\n"),
            "second project": self.configured() + "\n[projects." + json.dumps(str(other))
                              + ']\ntrust_level = "trusted"\n',
            "unexpected root key": 'model = "foreign"\n' + self.configured(),
            "runtime changed": self.configured().replace("codex-sqlite", "foreign-sqlite"),
            "runtime omitted": "\n".join(self.configured().splitlines()[1:]),
        }
        for name, body in cases.items():
            with self.subTest(name=name):
                self.write(body)
                with self.assertRaises(ValueError):
                    self.observer.guard(require_trust=True)
        # TOML formatting may change; the complete parsed document may not.
        self.write(self.original + "\nprojects = {" + json.dumps(str(self.project))
                   + ' = { trust_level = "trusted" }}\n')
        self.observer.guard(require_trust=True)

    def test_physical_project_identity_rejects_redirected_aliases(self):
        self.observer.begin_install()
        alias = self.case / "alias"
        try:
            alias.symlink_to(self.project, target_is_directory=True)
        except OSError as error:
            self.skipTest(str(error))
        self.write(self.configured(alias))
        with self.assertRaises(ValueError):
            self.observer.guard(require_trust=True)
        alias.unlink()
        alias.symlink_to(self.case, target_is_directory=True)
        self.write(self.configured(alias / "project"))
        with self.assertRaises(ValueError):
            self.observer.guard(require_trust=True)
        if os.name == "nt":
            self.write(self.configured(Path("\\\\?\\" + str(self.project))))
            self.observer.guard(require_trust=True)


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
            try:
                with self.assertRaises(TimeoutError):
                    asyncio.run(audit._exchange(Path(sys.executable), root, profile, environment))
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
