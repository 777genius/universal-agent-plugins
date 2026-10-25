"""Inert Qoder acquisition contracts; no vendor payload is acquired or run."""

import ast
import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch
import urllib.request


SCRIPTS = Path(__file__).parent


def load_leaf():
    spec = importlib.util.spec_from_file_location(
        "qoder_acquisition_contract", SCRIPTS / "project_native_collectors" / "qoder.py")
    leaf = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(leaf)
    return leaf


class ForbiddenContext:
    def __getattribute__(self, name):
        raise AssertionError("acquisition-only collector accessed context: " + name)


class QoderAcquisitionContract(unittest.TestCase):
    def test_pending_member_cannot_become_a_native_observation(self):
        with (
            patch.object(subprocess, "Popen", side_effect=AssertionError("vendor process")) as process,
            patch.object(urllib.request, "urlopen", side_effect=AssertionError("network download")) as download,
        ):
            leaf = load_leaf()
            self.assertEqual(leaf.AVAILABLE_PHASES, ("acquire",))
            self.assertIs(leaf.ACQUISITION_ONLY, True)
            self.assertIsNone(leaf.ACQUISITION["entrypoint"])
            for context in (None, ForbiddenContext(), {"hosted": True, "native_opt_in": True}):
                with self.subTest(context_type=type(context).__name__):
                    with self.assertRaisesRegex(RuntimeError, "acquisition-only.*phase amendment"):
                        leaf.collect(context)
            process.assert_not_called()
            download.assert_not_called()

    def test_flag_or_guessed_entrypoint_cannot_enable_the_collector(self):
        leaf = load_leaf()
        leaf.ACQUISITION_ONLY = False
        leaf.ACQUISITION["entrypoint"] = "guessed/qodercli"
        with self.assertRaisesRegex(RuntimeError, "acquisition-only"):
            leaf.collect(ForbiddenContext())

    def test_reuses_preserved_reviewed_archive_and_binary_identities(self):
        # Read literal pins without importing the legacy acquisition machinery.
        tree = ast.parse((SCRIPTS / "research-qoder-native.py").read_text(encoding="utf-8"))
        preserved = {
            node.targets[0].id: ast.literal_eval(node.value)
            for node in tree.body
            if isinstance(node, ast.Assign) and len(node.targets) == 1
            and isinstance(node.targets[0], ast.Name)
            and node.targets[0].id in {"VERSION", "URL", "ARCHIVE_PIN", "BINARY_SHA256"}
        }
        leaf = load_leaf()
        self.assertEqual(leaf.CLIENT, "qoder")
        self.assertEqual(leaf.VERSION, preserved["VERSION"])
        self.assertEqual(leaf.ACQUISITION["url"], preserved["URL"])
        self.assertEqual(leaf.ACQUISITION["integrity"], preserved["ARCHIVE_PIN"])
        self.assertEqual(leaf.ACQUISITION["binary_sha256"], preserved["BINARY_SHA256"])
        self.assertIs(type(leaf.ACQUISITION["expected_bytes"]), int)
        self.assertGreater(leaf.ACQUISITION["expected_bytes"], 0)
        self.assertLess(leaf.ACQUISITION["expected_bytes"], leaf.ACQUISITION["max_bytes"])
        self.assertLessEqual(leaf.ACQUISITION["max_bytes"], 96 << 20)


if __name__ == "__main__":
    unittest.main()
