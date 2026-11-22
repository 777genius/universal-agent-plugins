"""Inert published-wrapper ownership/routing tests; never native qualification."""

import json
import os
from pathlib import Path
from runpy import run_path
import shutil
import subprocess
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[1]


class PublishedRemotionTempRootTest(unittest.TestCase):
    def setUp(self):
        self.child_patch = patch.object(subprocess, "Popen", side_effect=AssertionError(
            "native/vendor child forbidden in this inert wrapper regression"))
        self.children = self.child_patch.start()
        self.addCleanup(self.child_patch.stop)
        self.temporary = tempfile.TemporaryDirectory(prefix="remotion-published-inert-")
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name).resolve(strict=True)
        self.runner = self.base / "runner"
        self.default = self.base / "default-windows-profile-with-a-longer-physical-spelling"
        self.runner.mkdir()
        self.default.mkdir()
        self.cli = self.base / "inert-published-cli.cmd"
        self.cli.write_bytes(b"inert fixture, never executed\n")
        self.script = run_path(str(ROOT / "scripts/remotion_published_windows_long_temp_e2e.py"))
        self.audit = run_path(str(ROOT / "scripts/live-install-audit.py"))
        self.codex = run_path(str(ROOT / "scripts/codex_runtime_audit.py"))
        self.fixture_bytes = (ROOT / "tests/live-install/fixtures.json").read_bytes()
        self.created, self.calls, self.moved = [], [], []

    def tearDown(self):
        self.children.assert_not_called()

    def replace(self, path):
        original = path.with_name(path.name + "-original")
        path.rename(original)
        self.moved.append(original)
        path.mkdir()
        (path / "foreign.txt").write_bytes(b"foreign replacement must survive\n")

    def run_main(self, *, aliases=False, fault=None, outcome=None, version="agentplugins 0.1.102\n"):
        """Run actual main and real authority helpers, replacing child transports only."""
        original_resolve, original_lstat = Path.resolve, Path.lstat
        foreign = self.base / "foreign-resolution"
        foreign.mkdir(exist_ok=True)
        (foreign / "foreign.txt").write_bytes(b"unrelated object must survive\n")
        injected = False

        def create(**options):
            parent = Path(options.get("dir", self.default))
            physical = Path(tempfile.mkdtemp(prefix=options["prefix"], dir=parent))
            (physical / "owned.txt").write_bytes(b"original owned bytes\n")
            lexical = physical
            if aliases:
                spelling = parent / "spelling"
                spelling.mkdir(exist_ok=True)
                lexical = spelling / ".." / physical.name
            self.created.append((lexical, physical, physical.lstat()))
            return str(lexical)

        def resolve(path, *args, **kwargs):
            nonlocal injected
            if (not injected and len(self.created) == 2 and path == self.created[1][0]
                    and fault in ("foreign-resolution", "replace-during-handoff")):
                injected = True
                if fault == "foreign-resolution":
                    return foreign
                self.replace(self.created[1][1])
            return original_resolve(path, *args, **kwargs)

        def lstat(path, *args, **kwargs):
            info = original_lstat(path, *args, **kwargs)
            if fault == "reparse" and len(self.created) == 2 and path == self.created[1][0]:
                return SimpleNamespace(st_mode=info.st_mode, st_file_attributes=0x400,
                                       st_reparse_tag=0)
            return info

        def native_case(*args):
            self.assertEqual(len(self.created), 2, "both handoffs must precede the first case")
            cli, target, kind, item, index, selected, names, pins = args
            self.assertEqual((cli, target, kind, index), (str(self.cli), "codex", "skill", 24))
            fixtures = json.loads(self.fixture_bytes)
            self.assertEqual(item, fixtures["skills"][24])
            self.assertEqual(names, fixtures["plugin_declared_names"])
            self.assertEqual(pins, fixtures["signed_release_pins"])
            lexical, physical, previous = self.created[len(self.calls)]
            observed = self.codex["_real_directory"](selected)
            self.assertEqual(selected, physical)
            self.assertEqual(selected, selected.resolve(strict=True))
            self.assertTrue(os.path.samestat(previous, observed))
            if aliases:
                self.assertNotEqual(lexical, selected)
                with self.assertRaisesRegex(ValueError, "directory is redirected"):
                    self.codex["_real_directory"](lexical)
            self.calls.append(args)
            if fault == "replace-after-case" and len(self.calls) == 1:
                self.replace(physical)
            if outcome is not None:
                return outcome(len(self.calls))
            # This synthetic row tests wrapper routing only, not the native verifier.
            return {"status": "passed", "unit_transport_only": True}

        native = SimpleNamespace(validate_fixtures=self.audit["validate_fixtures"],
                                 bounded_error_traceback=self.audit["bounded_error_traceback"],
                                 do_case=native_case)
        loader = SimpleNamespace(exec_module=lambda _: None)
        imports = SimpleNamespace(util=SimpleNamespace(
            spec_from_file_location=lambda *args: SimpleNamespace(loader=loader),
            module_from_spec=lambda _: native))
        versions = []

        def version_call(argv, **kwargs):
            self.assertEqual(argv, [str(self.cli), "version"])
            self.assertEqual(kwargs["timeout"], 30)
            versions.append(argv)
            return SimpleNamespace(returncode=0, stdout=version)

        commands = SimpleNamespace(run=version_call, check_output=lambda *args, **kwargs:
                                  "inert-wrapper-source" if kwargs.get("text") else self.fixture_bytes)
        environment = SimpleNamespace(name="nt", environ={"RUNNER_TEMP": str(self.runner)})
        main = self.script["main"]
        error = None
        with patch.dict(main.__globals__, {
                "os": environment, "subprocess": commands, "importlib": imports,
                "tempfile": SimpleNamespace(mkdtemp=create)}), \
                patch.object(shutil, "which", return_value=str(self.cli)), \
                patch.object(Path, "resolve", resolve), patch.object(Path, "lstat", lstat), \
                patch.object(shutil, "rmtree") as delete, patch("builtins.print"):
            try:
                main()
            except SystemExit as caught:
                error = str(caught)
            delete.assert_not_called()
        self.assertEqual(len(versions), 1)
        report = self.runner / "remotion-windows-published-0.1.102.json"
        return error, json.loads(report.read_text()) if report.exists() else None

    def test_same_object_alias_reaches_actual_main_as_physical_root_before_cases(self):
        error, report = self.run_main(aliases=True)
        self.assertIsNone(error)
        self.assertEqual(len(self.calls), 2)
        self.assertEqual(report["published_version"], "agentplugins 0.1.102")
        self.assertEqual(report["published_tag"], "agentplugins-v0.1.102")
        self.assertEqual(report["cli_path_kind"], "published-npm-windows")
        for label, (lexical, physical, previous) in zip(
                ("runner-temp", "default-windows-temp"), self.created):
            retained = report["temp_roots"][label]
            self.assertEqual(retained["created_path"], str(lexical))
            self.assertEqual(retained["created_root_chars"], len(str(lexical / "skill-24")))
            self.assertEqual(retained["root_chars"], len(str(physical / "skill-24")))
            self.assertEqual(retained["cleanup"], "retained-for-evidence")
            self.assertEqual(report["results"][label]["root_chars"], retained["root_chars"])
            self.assertTrue(os.path.samestat(previous, physical.lstat()))
            self.assertEqual((physical / "owned.txt").read_bytes(), b"original owned bytes\n")

    def test_handoff_rejects_changed_foreign_or_reparse_roots_before_any_case(self):
        # Fresh fixture per subcase: a failed first main must not become a new baseline.
        for fault in ("foreign-resolution", "replace-during-handoff", "reparse"):
            with self.subTest(fault=fault):
                separate = PublishedRemotionTempRootTest(methodName="test_same_object_alias_reaches_actual_main_as_physical_root_before_cases")
                separate.setUp()
                try:
                    error, report = separate.run_main(fault=fault)
                    self.assertIn("TEMP setup: ValueError", error)
                    self.assertEqual(separate.calls, [])
                    self.assertEqual(report["results"], {})
                    self.assertEqual(report["setup_error"]["error_type"], "ValueError")
                    self.assertLessEqual(len(report["setup_error"]["error_traceback"]), 8192)
                    expected = "retained-for-evidence" if fault == "foreign-resolution" else "retained-unverified"
                    self.assertEqual(report["temp_roots"]["default-windows-temp"]["cleanup"], expected)
                    self.assertTrue(all(physical.is_dir() for _, physical, _ in separate.created))
                    self.assertEqual((separate.base / "foreign-resolution/foreign.txt").read_bytes(),
                                     b"unrelated object must survive\n")
                    if fault == "replace-during-handoff":
                        self.assertEqual((separate.created[1][1] / "foreign.txt").read_bytes(),
                                         b"foreign replacement must survive\n")
                        self.assertEqual((separate.moved[0] / "owned.txt").read_bytes(), b"original owned bytes\n")
                finally:
                    separate.tearDown()
                    separate.doCleanups()

    def test_final_ownership_failure_retains_original_and_replacement_and_fails(self):
        error, report = self.run_main(fault="replace-after-case")
        self.assertEqual(len(self.calls), 2)
        self.assertIn("runner-temp: TEMP ownership ValueError", error)
        self.assertEqual(report["temp_roots"]["runner-temp"]["cleanup"], "retained-unverified")
        self.assertEqual(report["temp_roots"]["default-windows-temp"]["cleanup"], "retained-for-evidence")
        self.assertEqual((self.created[0][1] / "foreign.txt").read_bytes(), b"foreign replacement must survive\n")
        self.assertEqual((self.moved[0] / "owned.txt").read_bytes(), b"original owned bytes\n")

    def test_original_exception_classes_remain_nonzero_with_bounded_traceback(self):
        def failure(index):
            kind = AssertionError if index == 1 else ValueError
            raise kind("inert failure marker " + "x" * 12000)
        error, report = self.run_main(outcome=failure)
        self.assertIn("runner-temp: AssertionError", error)
        self.assertIn("default-windows-temp: ValueError", error)
        for label, kind in (("runner-temp", "AssertionError"), ("default-windows-temp", "ValueError")):
            row = report["results"][label]
            self.assertEqual(row["error_type"], kind)
            self.assertIn("inert failure marker", row["error_traceback"])
            self.assertIn(kind, row["error_traceback"])
            self.assertLessEqual(len(row["error_traceback"]), 8192)
            self.assertNotIn("case", row)
            self.assertEqual(report["temp_roots"][label]["cleanup"], "retained-for-evidence")

    def test_nonpassed_row_and_wrong_published_version_still_fail(self):
        error, report = self.run_main(outcome=lambda _: {"status": "incomplete-native-proof"})
        self.assertIn("native E2E case did not PASS", error)
        self.assertEqual({r["case"]["status"] for r in report["results"].values()}, {"incomplete-native-proof"})
        # A separate case asserts version refusal before any TEMP/profile is authored.
        separate = PublishedRemotionTempRootTest(methodName="test_same_object_alias_reaches_actual_main_as_physical_root_before_cases")
        separate.setUp()
        try:
            error, report = separate.run_main(version="agentplugins 0.1.103\n")
            self.assertIn("does not match 0.1.102", error)
            self.assertEqual(separate.created, [])
            self.assertEqual(separate.calls, [])
            self.assertIsNone(report)
        finally:
            separate.tearDown()
            separate.doCleanups()


if __name__ == "__main__":
    unittest.main()
