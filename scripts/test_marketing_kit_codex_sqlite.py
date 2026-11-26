"""Marketing runner regressions against its current CodexProfile authority.

Real WAL semantics, bootstrap custody and failed-checkpoint persistence are
covered by test_marketing_kit_codex_profile.py. These cases retain the runner
version, cross-profile admission, failure-report and peer-initialization checks
without recreating the retired CodexProfileAudits orchestration layer.
"""
import argparse
from contextlib import closing, redirect_stderr, redirect_stdout
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
    def setUp(self):
        self.runner = run_path(str(Path(__file__).with_name("marketing-kit-project-release-e2e.py")))
        self.globals = self.runner["main"].__globals__
        self.audit = self.runner["audit"]
        temporary = TemporaryDirectory(prefix="marketing-sqlite-inert-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()

    def test_codex_workflow_provisions_native_sandbox_before_lifecycle(self):
        root = Path(__file__).resolve().parents[1]
        workflow = (root / ".github/workflows/marketing-kit-project-release-e2e.yml").read_text()
        prerequisite = "      - name: Install native Codex sandbox prerequisite\n"
        step = workflow.split(prerequisite, 1)[1].split("      - name:", 1)[0]
        self.assertIn("if: ${{ matrix.target == 'codex' }}", step)
        self.assertIn("run: bash scripts/prepare-codex-linux-sandbox.sh", step)
        self.assertNotIn("continue-on-error", step)
        positions = [workflow.index(marker) for marker in (
            "      - uses: actions/setup-node@", prerequisite,
            "      - name: Install real released UAP and native client",
            "      - name: Exercise actual signed Marketing Kit",
        )]
        self.assertEqual(positions, sorted(positions))
        events = workflow.split("permissions:", 1)[0]
        self.assertEqual(events.count("      - 'scripts/prepare-codex-linux-sandbox.sh'"), 2)
        for event, next_event in (("pull_request", "push"), ("push", "workflow_dispatch")):
            filters = events.split(f"  {event}:\n", 1)[1].split(f"  {next_event}:\n", 1)[0]
            self.assertIn("scripts/codex_runtime_audit.py", filters)
        # The separately installed historical package must match its verifier;
        # a newer lifecycle pin must not silently move the historical proof.
        script = (root / "scripts/marketing-kit-published-launcher-abort.js").read_text()
        self.assertIn('assert.equal(pkg.version, "0.1.115"', script)
        self.assertIn('universal-agent-plugins@0.1.115', workflow)
        self.assertIn('universal-agent-plugins@0.1.116', workflow)

    def test_exact_version_selection_preserves_default_and_rejects_lookalikes(self):
        require = self.runner["require_exact_published_version"]
        result = {"exit_code": 0, "timeout": False, "stdout_tail": "agentplugins 0.1.115\n"}
        require(result)
        token = self.runner["_CLI_VERSION"].set("0.1.116")
        try:
            require({**result, "stdout_tail": "agentplugins 0.1.116\n"})
            with patch.dict(self.globals, {"invoke": lambda *_: {
                    **result, "stdout_tail": "agentplugins 0.1.116\n"}}):
                self.runner["verify_pinned_launcher"]("inert-cli", {}, self.root)
            require(result, "0.1.115")
            for value in ("0.1.115", "0.1.116-dev", "0.1.1161", "0.1.117"):
                with self.subTest(value=value), self.assertRaisesRegex(AssertionError, "not exact"):
                    require({**result, "stdout_tail": "agentplugins " + value})
            for value in ("latest", "0.1.116-dev", "01.1.116", "0.1.116\n"):
                with self.subTest(pin=value), self.assertRaises(argparse.ArgumentTypeError):
                    self.runner["published_cli_version"](value)
        finally:
            self.runner["_CLI_VERSION"].reset(token)
        require(result)

    def test_preinstall_rejects_cross_profile_project_before_native_execution(self):
        env_a, project_a, *_ = self.runner["run_case"]("inert", "codex", self.root, "A")
        _, project_b, *_ = self.runner["run_case"]("inert", "codex", self.root, "B")
        with (patch.dict(self.audit["AGENT_BINARIES"], {"codex": sys.executable}),
              patch.dict(self.audit, {"initialize_codex_plugin_profile": lambda *_:
                         self.fail("cross-profile request reached native initialization")})):
            with self.assertRaisesRegex(ValueError, "selected disposable context"):
                self.runner["CodexProfile"](self.audit, self.root, env_a, project_b)
        self.assertFalse((Path(env_a["HOME"]) / ".codex/installation_id").exists())
        self.assertFalse((Path(env_a["HOME"]) / ".codex/config.toml").exists())
        self.assertTrue(project_a.is_dir())

    def test_failed_initialization_emits_failure_and_restores_version_context(self):
        report = self.root / "failure-report.json"
        reached = []

        def refused(*_):
            reached.append(True)
            raise ValueError("inert prebaseline refusal")

        with (patch.object(sys, "argv", ["marketing-e2e", "--target", "codex",
                                         "--report", str(report), "--cli-version", "0.1.116"]),
              patch.dict(self.audit["AGENT_BINARIES"], {"codex": sys.executable}),
              patch.dict(self.globals, {"CodexProfile": refused, "invoke": lambda *_args, **_kwargs: {
                  "exit_code": 0, "timeout": False, "stdout_full": "codex-cli 0.153.4\n"}}),
              redirect_stdout(io.StringIO()), redirect_stderr(io.StringIO())):
            self.assertEqual(self.runner["main"](), 1)
        evidence = json.loads(report.read_text())
        self.assertEqual(reached, [True])
        self.assertEqual(evidence["cli_version"], "0.1.116")
        self.assertEqual(evidence["status"], "failed")
        self.assertIn("inert prebaseline refusal", evidence["failure"])
        self.assertFalse(evidence["two_scope_lifecycle_qualified"])
        self.assertEqual(evidence["steps"], [])
        self.assertEqual(self.runner["_CLI_VERSION"].get(), "0.1.115")

    def test_c_native_initialization_cannot_rebaseline_a_non_sqlite_config(self):
        # Do not expose the shared TestCase at module scope: unittest would
        # otherwise collect its unrelated acceptance suite a second time.
        from scripts.test_live_install_audit_env import IsolatedClientEnvironmentTest

        for mutation, native_exception in ((False, False), (True, False), (True, True)):
            with self.subTest(mutation=mutation, native_exception=native_exception):
                initialized, contexts = [], []

                def initialize(runner, env, project):
                    contexts[:] = [runner["_CLI_VERSION"]]
                    helpers = runner["audit"]["CODEX_RUNTIME_AUDIT"]
                    globals_ = helpers["CodexPluginSQLiteAudit"].__init__.__globals__

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
                            before = globals_["tomllib"].loads(peer.read_text())
                            with peer.open("a") as stream:
                                stream.write('model = "peer-mutated"\n')
                            after = globals_["tomllib"].loads(peer.read_text())
                            self.assertEqual(before["sqlite_home"], after["sqlite_home"])
                            with closing(sqlite3.connect(Path(before["sqlite_home"]) / "state_5.sqlite")) as connection:
                                self.assertEqual(connection.execute("SELECT * FROM fixture_rows").fetchall(), [("stable",)])
                            if native_exception:
                                raise RuntimeError("inert C native initialization refused")
                        return {"data": [{"cwd": str(selected_project), "errors": [], "skills": []}]}

                    with (patch.dict(runner["audit"]["AGENT_BINARIES"], {"codex": sys.executable}),
                          patch.dict(globals_, {"_exchange": discover})):
                        return runner["CodexProfile"](runner["audit"], project.parent.parent, env, project)

                fixture = IsolatedClientEnvironmentTest("runTest")
                code, report, _ = fixture._exercise_marketing_kit_scope_report(
                    "codex", codex_initializer=initialize)
                self.assertEqual(initialized, ["A", "B", "C"])
                self.assertEqual(contexts[0].get(), "0.1.115")
                if mutation:
                    self.assertEqual(code, 1, "peer config mutation was falsely qualified")
                    self.assertEqual(report["status"], "failed")
                    self.assertFalse(report["two_scope_lifecycle_qualified"])
                    self.assertIn("original isolated profile changed", report["failure"])
                    self.assertIn("config.toml", report["failure"])
                else:
                    self.assertEqual(code, 0, report.get("failure"))
                    self.assertTrue(report["two_scope_lifecycle_qualified"])
                    self.assertEqual(set(report["codex_runtime_profiles"]), {"A", "B", "C"})
                    for proof in report["codex_runtime_profiles"].values():
                        self.assertEqual(proof["final_checkpoint"], "passed")
                        self.assertTrue(proof["runtime_semantics_unchanged"])


if __name__ == "__main__":
    unittest.main()
