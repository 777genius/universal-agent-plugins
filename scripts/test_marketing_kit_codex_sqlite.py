"""Inert Marketing Kit regressions; no native client or network is executed."""
import argparse
from contextlib import redirect_stderr, redirect_stdout
from contextlib import closing
import io
import json
from pathlib import Path
from runpy import run_path
import sqlite3
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch


class MarketingKitCodexSQLiteTests(unittest.TestCase):
    def test_codex_workflow_provisions_native_sandbox_before_lifecycle(self):
        root = Path(__file__).resolve().parents[1]
        workflow = (root / ".github/workflows/marketing-kit-project-release-e2e.yml").read_text()
        prerequisite = "      - name: Install Codex Linux sandbox prerequisites\n"
        step = workflow.split(prerequisite, 1)[1].split("      - name:", 1)[0]
        self.assertIn("if: ${{ matrix.target == 'codex' }}", step)
        self.assertIn("run: bash scripts/prepare-codex-linux-sandbox.sh", step)
        self.assertNotIn("continue-on-error", step)
        positions = [workflow.index(marker) for marker in (
            "      - uses: actions/setup-node@",
            prerequisite,
            "      - name: Install real released UAP and native client",
            "      - name: Exercise actual signed Marketing Kit",
        )]
        self.assertEqual(positions, sorted(positions))
        events = workflow.split("permissions:", 1)[0]
        self.assertEqual(events.count("      - 'scripts/prepare-codex-linux-sandbox.sh'"), 2)
        for event, next_event in (("pull_request", "push"), ("push", "workflow_dispatch")):
            event_filters = events.split(f"  {event}:\n", 1)[1].split(f"  {next_event}:\n", 1)[0]
            self.assertIn("scripts/codex_runtime_audit.py", event_filters)

    def setUp(self):
        self.runner = run_path(str(Path(__file__).with_name("marketing-kit-project-release-e2e.py")))
        self.globals = self.runner["main"].__globals__
        self.audit = self.runner["audit"]
        temporary = TemporaryDirectory(prefix="marketing-sqlite-inert-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.report = {}
        self.observers = self.runner["CodexProfileAudits"](self.report)
        self.exchange_globals = self.audit["CODEX_RUNTIME_AUDIT"]["CodexPluginSQLiteAudit"].__init__.__globals__

    def fixture(self, name):
        case = self.root / name
        home, project, data = case / "home", case / "project", case / "data"
        for directory in (home / ".codex", project, data):
            directory.mkdir(parents=True)
        (home / ".codex/foreign.txt").write_text("foreign native bytes")
        (project / "foreign.txt").write_text("foreign project bytes")
        return self.runner["isolated_env"](home, case, data), project

    def add(self, name, mutation=None):
        env, project = self.fixture(name)

        async def discover(_binary, selected_project, profile, environment, **options):
            self.assertEqual(options, {"sqlite_baseline": True})
            self.assertEqual(profile, Path(env["HOME"]) / ".codex")
            self.assertEqual(environment["CODEX_HOME"], str(profile))
            database = Path(env["HOME"]) / ".local/state/codex-sqlite/state_5.sqlite"
            with closing(sqlite3.connect(database)) as connection:
                connection.execute("CREATE TABLE fixture_rows (value TEXT)")
                connection.execute("INSERT INTO fixture_rows VALUES ('stable')")
                connection.commit()
            if mutation is not None:
                mutation(env, project)
            return {"data": [{"cwd": str(selected_project), "errors": [], "skills": []}]}

        with (patch.dict(self.audit["AGENT_BINARIES"], {"codex": sys.executable}),
              patch.dict(self.exchange_globals, {"_exchange": discover})):
            self.observers.add(name, env, project)
        return env, project

    def database(self, name):
        return self.observers.observers[name].runtime / "state_5.sqlite"

    def test_exact_version_selection_preserves_legacy_default_and_rejects_lookalikes(self):
        require = self.runner["require_exact_published_version"]
        result = {"exit_code": 0, "timeout": False, "stdout_tail": "agentplugins 0.1.104\n"}
        require(result)
        token = self.runner["_CLI_VERSION"].set("0.1.116")
        try:
            require({**result, "stdout_tail": "agentplugins 0.1.116\n"})
            with patch.dict(self.globals, {"invoke": lambda *_: {
                    **result, "stdout_tail": "agentplugins 0.1.116\n"}}):
                self.runner["verify_pinned_launcher"]("inert-cli", {}, self.root)
            require(result, "0.1.104")
            for value in ("0.1.104", "0.1.116-dev", "0.1.1161", "0.1.117"):
                with self.subTest(value=value), self.assertRaisesRegex(AssertionError, "not exact"):
                    require({**result, "stdout_tail": "agentplugins " + value})
            for value in ("latest", "0.1.116-dev", "01.1.116", "0.1.116\n"):
                with self.subTest(pin=value), self.assertRaises(argparse.ArgumentTypeError):
                    self.runner["published_cli_version"](value)
        finally:
            self.runner["_CLI_VERSION"].reset(token)
        require(result)

    def test_large_owned_wal_keeps_semantics_and_strict_native_scanner(self):
        env, project = self.add("A")
        native_before = self.runner["scoped_snapshot"]((Path(env["HOME"]) / ".codex",))
        with closing(sqlite3.connect(self.database("A"))) as connection:
            connection.execute("PRAGMA journal_mode=WAL")
            connection.execute("PRAGMA wal_autocheckpoint=0")
            for _ in range(300):
                connection.execute("UPDATE fixture_rows SET value='temporary'")
                connection.execute("UPDATE fixture_rows SET value='stable'")
                connection.commit()
            wal = Path(str(self.database("A")) + "-wal")
            self.assertGreater(wal.stat().st_size, 1 << 20)
            self.observers.command(lambda: {"exit_code": 1})
            self.runner["check_removed_native_components"]("codex", Path(env["HOME"]),
                                                           self.root, "owned SQLite")
            self.assertEqual(native_before, self.runner["scoped_snapshot"](
                (Path(env["HOME"]) / ".codex",)))
            proof = self.report["codex_sqlite_profiles"]["A"]
            self.assertEqual(proof["status"], "observed")
            self.assertEqual(proof["observations"][-1]["phase"], "after-command")
            self.assertIn("state_5.sqlite-wal", proof["observations"][-1]["runtime_files_fully_read"])
            rogue = Path(env["HOME"]) / ".codex/state_5.sqlite-wal"
            rogue.write_bytes(b"x" * ((1 << 20) + 1))
            with self.assertRaisesRegex(AssertionError, "unscanned-too-large"):
                self.runner["check_removed_native_components"](
                    "codex", Path(env["HOME"]), self.root, "unowned native WAL")

    def test_separate_a_b_c_authorities_check_peers_even_on_command_exception(self):
        profiles = [self.add(name) for name in ("A", "B", "C")]
        self.assertEqual(len({str(x.runtime) for x in self.observers.observers.values()}), 3)
        native_before = self.runner["profile_snapshot"](*profiles[0])
        with self.assertRaisesRegex(RuntimeError, "inert command refused"):
            self.observers.command(lambda: (_ for _ in ()).throw(RuntimeError("inert command refused")))
        for proof in self.report["codex_sqlite_profiles"].values():
            self.assertEqual(proof["observations"][-1]["phase"], "after-command")
        self.runner["check_profile_unchanged"](*profiles[0], native_before, "peer read")

        def mutate_peer():
            with closing(sqlite3.connect(self.database("B"))) as connection:
                connection.execute("UPDATE fixture_rows SET value='foreign mutation'")
                connection.commit()
            raise RuntimeError("inert command refused")
        with self.assertRaisesRegex(ValueError, "table contents changed"):
            self.observers.command(mutate_peer)
        self.assertEqual(self.report["codex_sqlite_profiles"]["B"]["status"], "failed")
        self.assertEqual(self.report["codex_sqlite_profiles"]["C"]["observations"][-1]["phase"],
                         "after-command")
        reached = []
        with self.assertRaisesRegex(AssertionError, "prior failed checkpoint"):
            self.observers.command(lambda: reached.append(True))
        self.assertFalse(reached)

    def test_prebaseline_foreign_project_and_managed_mutations_are_not_admitted(self):
        for index, location in enumerate(("native", "home", "project", "managed", "config")):
            def mutate(env, project):
                if location == "native":
                    (Path(env["HOME"]) / ".codex/foreign.txt").write_text("changed")
                elif location == "home":
                    (Path(env["HOME"]) / "unexpected-directory").mkdir()
                elif location == "project":
                    (project / "foreign.txt").write_text("changed")
                elif location == "managed":
                    (Path(env["AGENTPLUGINS_HOME"]) / "unowned.txt").write_text("changed")
                else:
                    (Path(env["HOME"]) / ".codex/config.toml").write_text('sqlite_home="/foreign"')
            with self.subTest(location=location), self.assertRaises((AssertionError, ValueError)):
                self.add(str(index), mutate)
            self.assertEqual(self.report["codex_sqlite_profiles"][str(index)]["status"], "failed")

    def test_public_call_refusal_cannot_bypass_sqlite_checkpoint(self):
        self.add("A")
        token = self.runner["_CODEX_SQLITE"].set(self.observers)
        try:
            with patch.dict(self.audit, {"call": lambda *_: {"exit_code": 1}}):
                self.assertEqual(self.runner["invoke"]("inert"), {"exit_code": 1})
            self.assertEqual(self.report["codex_sqlite_profiles"]["A"]["observations"][-1]["phase"],
                             "after-command")
        finally:
            self.runner["_CODEX_SQLITE"].reset(token)

    def test_preinstall_rejects_cross_profile_project_before_native_execution(self):
        env_a, project_a = self.fixture("A")
        _, project_b = self.fixture("B")
        with patch.dict(self.audit["AGENT_BINARIES"], {"codex": sys.executable}):
            with self.assertRaisesRegex(AssertionError, "exact disposable profile"):
                self.runner["initialize_codex_sqlite"](env_a, project_b)
        self.assertFalse((Path(env_a["HOME"]) / ".codex/installation_id").exists())
        self.assertFalse((Path(env_a["HOME"]) / ".codex/config.toml").exists())
        self.assertEqual((project_a / "foreign.txt").read_text(), "foreign project bytes")

    def test_failed_initialization_emits_failure_and_restores_legacy_context(self):
        env, project = self.fixture("A")
        report = self.root / "failure-report.json"
        def refused(*_):
            raise ValueError("inert prebaseline refusal")
        with (patch.object(sys, "argv", ["marketing-e2e", "--target", "codex",
                                         "--report", str(report), "--cli-version", "0.1.116"]),
              patch.dict(self.globals, {"run_case": lambda *_: (env, project, None, None, []),
                  "bind_pinned_launcher_cache": lambda *_: None,
                  "initialize_codex_sqlite": refused}),
              redirect_stdout(io.StringIO()), redirect_stderr(io.StringIO())):
            self.assertEqual(self.runner["main"](), 1)
        evidence = json.loads(report.read_text())
        self.assertEqual(evidence["cli_version"], "0.1.116")
        self.assertEqual(evidence["status"], "failed")
        self.assertFalse(evidence["two_scope_lifecycle_qualified"])
        self.assertEqual(evidence["codex_sqlite_profiles"]["A"]["status"], "failed")
        self.assertEqual(self.runner["_CLI_VERSION"].get(), "0.1.104")
        self.assertIsNone(self.runner["_CODEX_SQLITE"].get())

    def test_c_native_initialization_cannot_rebaseline_a_non_sqlite_config(self):
        # Import locally: exposing the shared TestCase at module scope would
        # make unittest collect its unrelated acceptance suite a second time.
        from scripts.test_live_install_audit_env import IsolatedClientEnvironmentTest

        for mutation, native_exception in ((False, False), (True, False), (True, True)):
            with self.subTest(mutation=mutation, native_exception=native_exception):
                initialized = []
                contexts = []

                def initialize(runner, env, project):
                    contexts[:] = [runner["_CLI_VERSION"], runner["_CODEX_SQLITE"]]
                    runtime_helpers = runner["audit"]["CODEX_RUNTIME_AUDIT"]
                    exchange_globals = runtime_helpers["CodexPluginSQLiteAudit"].__init__.__globals__

                    async def discover(_binary, selected_project, profile, environment, **options):
                        self.assertEqual(options, {"sqlite_baseline": True})
                        self.assertEqual(profile, Path(environment["CODEX_HOME"]))
                        case = project.parent.name
                        database = Path(env["HOME"]) / ".local/state/codex-sqlite/state_5.sqlite"
                        with closing(sqlite3.connect(database)) as connection:
                            connection.execute("CREATE TABLE fixture_rows (value TEXT)")
                            connection.execute("INSERT INTO fixture_rows VALUES ('stable')")
                            connection.commit()
                        initialized.append(case)
                        if case == "C" and mutation:
                            peer = project.parent.parent / "A/home/.codex/config.toml"
                            before = exchange_globals["tomllib"].loads(peer.read_text())
                            with peer.open("a") as stream:
                                stream.write('model = "peer-mutated"\n')
                            after = exchange_globals["tomllib"].loads(peer.read_text())
                            self.assertEqual(before["sqlite_home"], after["sqlite_home"])
                            with closing(sqlite3.connect(Path(before["sqlite_home"]) / "state_5.sqlite")) as connection:
                                self.assertEqual(connection.execute("SELECT * FROM fixture_rows").fetchall(),
                                                 [("stable",)])
                            if native_exception:
                                raise RuntimeError("inert C native initialization refused")
                        return {"data": [{"cwd": str(selected_project), "errors": [], "skills": []}]}

                    with (patch.dict(runner["audit"]["AGENT_BINARIES"], {"codex": sys.executable}),
                          patch.dict(exchange_globals, {"_exchange": discover})):
                        return runner["initialize_codex_sqlite"](env, project)

                fixture = IsolatedClientEnvironmentTest("runTest")
                code, report, _ = fixture._exercise_marketing_kit_scope_report(
                    "codex", codex_initializer=initialize)
                self.assertEqual(initialized, ["A", "B", "C"])
                self.assertEqual(contexts[0].get(), "0.1.104")
                self.assertIsNone(contexts[1].get())
                if mutation:
                    self.assertEqual(code, 1, "peer config mutation was falsely qualified")
                    self.assertEqual(report["status"], "failed")
                    self.assertFalse(report["two_scope_lifecycle_qualified"])
                    self.assertIn("original isolated profile changed", report["failure"])
                    self.assertIn("config.toml", report["failure"])
                else:
                    self.assertEqual(code, 0, report.get("failure"))
                    self.assertTrue(report["two_scope_lifecycle_qualified"])
                    for proof in report["codex_sqlite_profiles"].values():
                        self.assertEqual(proof["status"], "observed")
                        self.assertTrue(proof["runtime_semantics_unchanged"])


if __name__ == "__main__":
    unittest.main()
