"""Regression tests for false-green acceptance of portable E2E evidence."""

import importlib.util
import json
import os
from pathlib import Path
import select
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


SPEC = importlib.util.spec_from_file_location("package_manager_e2e", Path(__file__).with_name("package-manager-e2e.py"))
RUNNER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUNNER)


class EvidenceAcceptanceTests(unittest.TestCase):
    def test_empty_duplicate_or_unsafe_case_inventory_is_refused(self):
        for cases in [[], {}, ["same", "same"], ["a/b"], [""], [1], ["PASS"]]:
            with self.subTest(cases=cases), self.assertRaises(ValueError):
                RUNNER.validate_case_ids(cases)
        self.assertEqual(RUNNER.validate_case_ids(["install", "cold-restore"]), ["install", "cold-restore"])

    def test_unknown_group_cannot_silently_enter_archive_subset(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "catalog.json"
            path.write_text(json.dumps({**{group: ["case"] for group in RUNNER.CATALOG_GROUPS},
                                        "unknown-native-runtime": ["case"]}), encoding="utf-8")
            with patch.object(RUNNER, "CATALOG", path), self.assertRaisesRegex(ValueError, "explicitly classify"):
                RUNNER.load_case_inventories()

    def check(self, events, required=("TestCatalog", "TestCatalog/leaf"), exit_code=0):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "events.jsonl"
            path.write_text("".join(json.dumps(event) + "\n" for event in events), encoding="utf-8")
            return RUNNER.validate_go_events(path, required, "fixture/package", exit_code)

    def events(self):
        return [{"Action": "pass", "Package": "fixture/package", "Test": "TestCatalog/leaf"},
                {"Action": "pass", "Package": "fixture/package", "Test": "TestCatalog"},
                {"Action": "pass", "Package": "fixture/package"}]

    def test_complete_execution(self):
        self.assertEqual(self.check(self.events()), ["TestCatalog", "TestCatalog/leaf"])

    def test_zero_matched_tests_is_not_success(self):
        with self.assertRaisesRegex(ValueError, "missing="):
            self.check(self.events()[-1:])

    def test_parent_pass_does_not_replace_missing_leaf(self):
        with self.assertRaisesRegex(ValueError, "TestCatalog/leaf"):
            self.check(self.events()[1:])

    def test_required_skip_and_failed_descendant_are_refused(self):
        for action in ["skip", "fail"]:
            with self.subTest(action=action), self.assertRaises(ValueError):
                self.check(self.events() + [{"Action": action, "Package": "fixture/package", "Test": "TestCatalog/leaf/child"}])

    def test_missing_package_completion_and_nonzero_exit(self):
        with self.assertRaises(ValueError):
            self.check(self.events()[:-1])
        with self.assertRaises(ValueError):
            self.check(self.events(), exit_code=1)

    def test_other_package_cannot_supply_missing_proof(self):
        events = self.events()
        events[0]["Package"] = "another/package"
        with self.assertRaises(ValueError):
            self.check(events)

    def test_empty_and_truncated_json_are_refused(self):
        with self.assertRaises(ValueError):
            self.check([])
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "events.jsonl"
            path.write_text('{"Action":', encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "invalid Go JSON"):
                RUNNER.validate_go_events(path, ["TestCatalog"], "fixture/package", 0)

    @unittest.skipIf(os.name == "nt", "POSIX process-group cleanup; Windows uses taskkill")
    def test_cleanup_kills_descendant_after_leader_exits(self):
        child = "import signal,time; signal.signal(signal.SIGTERM, signal.SIG_IGN); print('ready',flush=True); time.sleep(60)"
        parent = ("import subprocess,sys,time; "
                  f"subprocess.Popen([sys.executable,'-c',{child!r}],stdout=sys.stdout); time.sleep(60)")
        process = subprocess.Popen([sys.executable, "-c", parent], stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, start_new_session=True)
        try:
            self.assertTrue(select.select([process.stdout], [], [], 5)[0], "child never became ready")
            self.assertEqual(process.stdout.readline(), b"ready\n")
            RUNNER.stop_owned_process(process)
            self.assertTrue(select.select([process.stdout], [], [], 5)[0], "descendant kept the pipe open")
            self.assertEqual(process.stdout.read(), b"")
        finally:
            RUNNER.stop_owned_process(process)
            process.stdout.close()
            process.stderr.close()


    def test_smoke_exception_preserves_independent_qualification(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            run = RUNNER.Run.__new__(RUNNER.Run)
            run.directory = root
            run.env = {"GOPATH": str(root / "gopath"), "GOCACHE": str(root / "gocache")}
            run.groups = RUNNER.CATALOG_GROUPS
            run.catalog_required = ["catalog-case"]
            run.install_required = ["installation-case"]
            run.composed_required = ["composition-case"]
            run.report = {"suite": "full", "source": {"commit": "unchanged"}, "phases": []}
            (root / ("agentplugins.exe" if os.name == "nt" else "agentplugins")).write_bytes(b"fixture")
            called = []

            def command(name, *args, **kwargs):
                called.append(name)
                return True

            with patch.object(run, "command", side_effect=command), \
                    patch.object(run, "smoke", side_effect=KeyError("missing smoke diagnostic")), \
                    patch.object(RUNNER, "run_installations", return_value=True), \
                    patch.object(RUNNER, "source_identity", return_value={"commit": "unchanged"}):
                self.assertFalse(run.execute(root))

            self.assertIn("missing smoke diagnostic", run.report.get("smoke_error", ""))
            self.assertEqual(called, ["build", "catalog", "installations", "composition"])
            self.assertEqual(run.report["status"], "fail")
            self.assertFalse(run.report["full_qualification"])
            self.assertEqual(json.loads((root / "report.json").read_text())["status"], "fail")

    def test_nested_binary_installation_error_is_reported(self):
        import package_manager_binary

        class Recorder:
            def __init__(self):
                self.report = {}

            def save(self):
                pass

        run = Recorder()
        with patch.object(package_manager_binary, "_installations",
                          side_effect=AttributeError("unexpected null nested JSON")):
            with self.assertRaisesRegex(RuntimeError, "unchanged-binary installation proof failed"):
                package_manager_binary.run_installations(run, None, None, None)
        self.assertEqual(run.report["binary_installation"]["status"], "fail")
        self.assertIn("unexpected null nested JSON", run.report["binary_installation"]["error"])

if __name__ == "__main__":
    unittest.main()
