"""Filesystem and SQLite controls for the Marketing Kit Codex profile owner."""

from contextlib import closing, contextmanager
import hashlib
from pathlib import Path
import runpy
import shutil
import sqlite3
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
CodexProfile = runpy.run_path(str(ROOT / "scripts/marketing_kit_codex_profile.py"))["CodexProfile"]


class MarketingCodexProfileContracts(unittest.TestCase):
    @contextmanager
    def fixture(self, *, system=False, bootstrap_mutation=None):
        audit = runpy.run_path(str(ROOT / "scripts/live-install-audit.py"))
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "A"
            home, project, data = (case / name for name in ("home", "project", "data"))
            for path in (home, project, data):
                path.mkdir(parents=True)
            for name in audit["NATIVE_ROOTS"]["codex"]:
                (home / name).mkdir(parents=True, exist_ok=True)
            foreign = home / ".codex/foreign.txt"
            foreign.write_bytes(b"unmanaged original\n")
            env = audit["isolated_client_env"](home, case, data)
            runtime = home / ".local/state/codex-sqlite"
            database = runtime / "state_5.sqlite"
            body = b'ACRONYMS = {\n    "playwright": "Playwright",\n}\n'
            relative = "control/SKILL.md"
            globals_ = audit["CODEX_RUNTIME_AUDIT"]["CodexPluginSQLiteAudit"].__init__.__globals__

            async def exchange(binary, cwd, profile, environment, *, sqlite_baseline=False):
                self.assertTrue(sqlite_baseline)
                self.assertEqual(cwd, project)
                self.assertEqual(environment["CODEX_HOME"], str(home / ".codex"))
                with closing(sqlite3.connect(database)) as connection:
                    connection.execute("CREATE TABLE owned_state(value TEXT)")
                    connection.execute("INSERT INTO owned_state VALUES ('unchanged')")
                    connection.commit()
                if system:
                    system_root = profile / "skills/.system"
                    (system_root / "control").mkdir(parents=True)
                    (system_root / relative).write_bytes(body)
                    (system_root / ".codex-system-skills.marker").write_bytes(globals_["SYSTEM_SKILL_MARKER"])
                if bootstrap_mutation is not None:
                    bootstrap_mutation(home, project, data)
                return {"data": [{"cwd": str(project), "errors": [], "skills": []}]}

            replacements = {"_exchange": exchange}
            if system:
                replacements.update(SYSTEM_SKILL_FILES={relative: (len(body), hashlib.sha256(body).hexdigest())},
                                    SYSTEM_SKILL_DIRECTORIES=frozenset(("control",)))
            with (patch.dict(audit["AGENT_BINARIES"], {"codex": "/inert/codex"}, clear=True),
                  patch.dict(globals_, replacements)):
                observer = CodexProfile(audit, root, env, project)
                yield audit, root, home, project, data, database, observer

    def test_shared_bootstrap_protects_foreign_bytes_and_records_exact_runtime(self):
        with self.fixture() as (_, _, home, _, _, database, observer):
            self.assertEqual((home / ".codex/foreign.txt").read_bytes(), b"unmanaged original\n")
            self.assertTrue(database.is_file())
            self.assertFalse((home / ".codex/state_5.sqlite").exists())
            self.assertEqual(list((home / ".codex/tmp/arg0").iterdir()), [])
            observer.checkpoint("after-public-add")
            proof = observer.evidence()
            self.assertEqual(proof["native_version"], "0.153.4")
            self.assertFalse(proof["runtime_semantics_unchanged"])
            self.assertEqual(proof["final_checkpoint"], "not_run")
            self.assertIsNone(proof["failed_phase"])
            self.assertEqual([row["phase"] for row in proof["observations"]],
                             ["before-install", "profile-initialized", "after-public-add"])

    def test_evidence_requires_actual_final_lifecycle_checkpoint(self):
        with self.fixture() as (_, _, _, _, _, _, observer):
            self.assertFalse(observer.evidence()["runtime_semantics_unchanged"])
            observer.checkpoint("final-lifecycle")
            proof = observer.evidence()
            self.assertTrue(proof["runtime_semantics_unchanged"])
            self.assertEqual(proof["final_checkpoint"], "passed")
            self.assertIsNone(proof["failed_phase"])
            self.assertEqual(proof["observations"][-1]["phase"], "final-lifecycle")

    def test_real_sqlite_failure_remains_sticky_after_restoration_and_final(self):
        with self.fixture() as (_, _, _, _, _, database, observer):
            with closing(sqlite3.connect(database)) as connection:
                connection.execute("UPDATE owned_state SET value='changed'")
                connection.commit()
            with self.assertRaisesRegex(ValueError, "table contents changed"):
                observer.checkpoint("after-public-mutation")
            proof = observer.evidence()
            self.assertFalse(proof["runtime_semantics_unchanged"])
            self.assertEqual(proof["final_checkpoint"], "failed")
            self.assertEqual(proof["failed_phase"], "after-public-mutation")
            with closing(sqlite3.connect(database)) as connection:
                connection.execute("UPDATE owned_state SET value='unchanged'")
                connection.commit()
            observer.checkpoint("final-lifecycle")
            self.assertTrue(observer.runtime.evidence()["runtime_semantics_unchanged"])
            self.assertEqual(observer.evidence()["failed_phase"], "after-public-mutation")
            self.assertFalse(observer.evidence()["runtime_semantics_unchanged"])

    def test_checkpoint_interruption_is_reraised_and_never_positive_evidence(self):
        with self.fixture() as (_, _, _, _, _, _, observer):
            observer.checkpoint("final-lifecycle")
            self.assertTrue(observer.evidence()["runtime_semantics_unchanged"])
            with patch.object(observer.runtime, "checkpoint", side_effect=KeyboardInterrupt("inert interruption")):
                with self.assertRaisesRegex(KeyboardInterrupt, "inert interruption"):
                    observer.checkpoint("interrupted-after-final")
            proof = observer.evidence()
            self.assertFalse(proof["runtime_semantics_unchanged"])
            self.assertEqual(proof["final_checkpoint"], "failed")
            self.assertEqual(proof["failed_phase"], "interrupted-after-final")

    def test_successful_runtime_observation_cannot_mask_outer_postguard_failure(self):
        with self.fixture() as (_, _, _, _, _, _, observer):
            original, calls = observer.guard, []
            def fail_postguard():
                original()
                calls.append(True)
                if len(calls) == 2:
                    raise ValueError("inert outer postguard failure")
            with patch.object(observer, "guard", side_effect=fail_postguard):
                with self.assertRaisesRegex(ValueError, "inert outer postguard failure"):
                    observer.checkpoint("postguard-failed")
            proof = observer.evidence()
            self.assertEqual(proof["observations"][-1]["phase"], "postguard-failed")
            self.assertFalse(proof["runtime_semantics_unchanged"])
            self.assertEqual(proof["final_checkpoint"], "failed")
            self.assertEqual(proof["failed_phase"], "postguard-failed")

    def test_bootstrap_cannot_rebaseline_foreign_home_project_or_data_changes(self):
        for name in ("home", "project", "data"):
            with self.subTest(root=name):
                def mutate(home, project, data):
                    {"home": home, "project": project, "data": data}[name].joinpath("unexpected").write_text("changed")
                with self.assertRaisesRegex(ValueError, "protected fixture authority"):
                    with self.fixture(bootstrap_mutation=mutate):
                        self.fail("an unowned bootstrap effect was accepted")

    def test_sqlite_wal_representation_is_checked_by_unchanged_semantics(self):
        with self.fixture() as (_, _, _, _, _, database, observer):
            with closing(sqlite3.connect(database)) as connection:
                connection.execute("PRAGMA journal_mode=WAL")
                connection.execute("PRAGMA wal_autocheckpoint=0")
                for _ in range(300):
                    connection.execute("UPDATE owned_state SET value='temporary'")
                    connection.execute("UPDATE owned_state SET value='unchanged'")
                    connection.commit()
                self.assertGreater(Path(str(database) + "-wal").stat().st_size, 1 << 20)
                observer.checkpoint("same-logical-state")
                connection.execute("UPDATE owned_state SET value='changed'")
                connection.commit()
                with self.assertRaisesRegex(ValueError, "table contents changed"):
                    observer.checkpoint("changed-logical-state")

    def test_directory_replacement_and_config_retargeting_refuse(self):
        for mutation in ("project", "data", "profile", "config", "primary"):
            with self.subTest(mutation=mutation), self.fixture() as (_, _, home, project, data, database, observer):
                if mutation == "config":
                    (home / ".codex/config.toml").write_text('sqlite_home = "/foreign"\n')
                elif mutation == "primary":
                    saved = database.read_bytes()
                    database.rename(database.with_name("old-primary"))
                    database.write_bytes(saved)
                else:
                    path = {"project": project, "data": data, "profile": home / ".codex"}[mutation]
                    held = path.with_name(path.name + "-original")
                    path.rename(held)
                    shutil.copytree(held, path)
                with self.assertRaises(ValueError):
                    observer.checkpoint("retargeted")

    def test_cleanup_proof_accepts_only_complete_unchanged_system_files(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with self.fixture(system=True) as (_, root, home, _, _, _, observer):
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("codex", home, root, "without-proof")
            proof = check("codex", home, root, "with-proof", codex_profile=observer)
            self.assertEqual(proof["status"], "verified-unchanged-pinned-system-files-at-checkpoint")
            self.assertEqual(set(proof["residuals"]), {".codex/skills/.system/control/SKILL.md"})
            self.assertFalse(proof["writer_closure_claimed"])
            (home / ".codex/state_5.sqlite-wal").write_bytes(b"x" * ((1 << 20) + 1))
            with self.assertRaisesRegex(AssertionError, "unscanned-too-large"):
                check("codex", home, root, "unproved-runtime-path", codex_profile=observer)

    def test_cleanup_rescans_for_non_system_state_created_during_proof(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        for has_system in (False, True):
            with self.subTest(system=has_system), self.fixture(system=has_system) as (_, root, home, _, _, _, observer):
                original = observer.system_files

                def inject():
                    result = original()
                    (home / ".codex/mcp.json").write_text('{"mcpServers":{"playwright":{}}}')
                    return result

                with patch.object(observer, "system_files", side_effect=inject):
                    with self.assertRaisesRegex(AssertionError, "residuals changed"):
                        check("codex", home, root, "late-registration", codex_profile=observer)

    def test_every_independent_guard_runs_after_a_failure(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        seen = []

        def guard(name, fail=False):
            seen.append(name)
            if fail:
                raise ValueError(name)

        with self.assertRaisesRegex(ValueError, "first"):
            runner["check_all_guards"]((lambda: guard("first", True), lambda: guard("peer"),
                                        lambda: guard("second", True), lambda: guard("foreign")))
        self.assertEqual(seen, ["first", "peer", "second", "foreign"])


if __name__ == "__main__":
    unittest.main()
