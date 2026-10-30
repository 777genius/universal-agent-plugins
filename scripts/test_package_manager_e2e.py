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


if __name__ == "__main__":
    unittest.main()
