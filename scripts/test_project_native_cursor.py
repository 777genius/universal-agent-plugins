"""Inert contracts for the Cursor acquisition boundary; no vendor execution."""

import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch
from urllib.parse import urlsplit
import urllib.request


LEAF = Path(__file__).with_name("project_native_collectors") / "cursor.py"


def load_leaf():
    spec = importlib.util.spec_from_file_location("cursor_acquisition_contract", LEAF)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ForbiddenContext:
    def __getattribute__(self, name):
        raise AssertionError("acquisition-only collector accessed context: " + name)


class CursorAcquisitionContract(unittest.TestCase):
    def test_import_and_collect_never_start_a_process_or_download(self):
        with (
            patch.object(subprocess, "Popen", side_effect=AssertionError("process execution")) as process,
            patch.object(urllib.request, "urlopen", side_effect=AssertionError("network acquisition")) as download,
        ):
            leaf = load_leaf()
            for context in (None, ForbiddenContext(), {"native_opt_in": True, "hosted": True}):
                with self.subTest(context_type=type(context).__name__):
                    with self.assertRaisesRegex(RuntimeError, "acquisition-only.*separately reviewed collector"):
                        leaf.collect(context)
            process.assert_not_called()
            download.assert_not_called()

    def test_measured_pin_keeps_native_execution_disabled(self):
        leaf = load_leaf()
        self.assertIs(leaf.ACQUISITION_ONLY, True)
        self.assertEqual(leaf.ACQUISITION["integrity"], "sha256:85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294")
        self.assertEqual(leaf.ACQUISITION["entrypoint"], "dist-package/cursor-agent")
        self.assertEqual(leaf.ACQUISITION["binary_sha256"], "2ccc9a8e167797641448b5e5c936f006ba137a2555f117f38c5eb76a5238a233")
        self.assertEqual(leaf.ACQUISITION["expected_bytes"], 184859176)
        self.assertEqual(leaf.ACQUISITION["max_expanded_bytes"], 640 << 20)
        self.assertEqual(leaf.ACQUISITION["transport"], "cursor-publisher-curl")
        # A flag change alone must not accidentally expose native execution.
        leaf.ACQUISITION_ONLY = False
        with self.assertRaisesRegex(RuntimeError, "acquisition-only"):
            leaf.collect(ForbiddenContext())

    def test_acquisition_is_bounded_and_uses_an_immutable_official_candidate(self):
        leaf = load_leaf()
        candidate = leaf.ACQUISITION
        url = urlsplit(candidate["url"])
        self.assertEqual(url.scheme, "https")
        self.assertEqual(url.netloc, "downloads.cursor.com")
        self.assertFalse(url.query or url.fragment or url.username or url.password)
        self.assertRegex(leaf.VERSION, r"\A\d{4}\.\d{2}\.\d{2}-[0-9a-f]{7}\Z")
        self.assertEqual(
            url.path.split("/"),
            ["", "lab", leaf.VERSION, "linux", "x64", "agent-cli-package.tar.gz"],
        )
        self.assertIs(type(candidate["max_bytes"]), int)
        self.assertGreater(candidate["max_bytes"], 0)
        self.assertLessEqual(candidate["max_bytes"], 256 * 1024 * 1024)
        self.assertEqual(candidate["format"], "tar.gz")
        self.assertEqual(candidate["runtime"], "native")


if __name__ == "__main__":
    unittest.main()
