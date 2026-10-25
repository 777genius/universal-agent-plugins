"""Inert Grok acquisition-only contracts; no vendor download or execution."""
import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch
import urllib.request
from urllib.parse import urlsplit


LEAF = Path(__file__).with_name("project_native_collectors") / "grok.py"


def load_leaf():
    spec = importlib.util.spec_from_file_location("grok_project_acquisition", LEAF)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ForbiddenContext:
    def __getattribute__(self, name):
        raise AssertionError("acquisition-only collector accessed context: " + name)


class GrokAcquisitionTests(unittest.TestCase):
    def test_import_and_collect_have_no_process_network_or_context_effect(self):
        with (
            patch.object(subprocess, "Popen", side_effect=AssertionError("process execution")) as process,
            patch.object(urllib.request, "urlopen", side_effect=AssertionError("network acquisition")) as download,
        ):
            leaf = load_leaf()
            for context in (None, ForbiddenContext(), {"hosted": True, "native_opt_in": True}):
                with self.subTest(context_type=type(context).__name__):
                    with self.assertRaisesRegex(RuntimeError, "acquisition-only.*reviewed pin amendment"):
                        leaf.collect(context)
            process.assert_not_called()
            download.assert_not_called()

    def test_no_execution_phase_or_borrowed_identity_can_enable_observation(self):
        leaf = load_leaf()
        self.assertEqual(leaf.AVAILABLE_PHASES, ("acquire",))
        self.assertIs(leaf.ACQUISITION_ONLY, True)
        self.assertIsNone(leaf.ACQUISITION["integrity"])
        self.assertNotIn("binary_sha256", leaf.ACQUISITION)
        self.assertNotIn("expected_bytes", leaf.ACQUISITION)
        # Shared phase handling must refuse too, but the leaf remains closed
        # even if an unrelated caller changes its declarative guard fields.
        leaf.ACQUISITION_ONLY = False
        leaf.AVAILABLE_PHASES = ("acquire", "observe")
        leaf.ACQUISITION["integrity"] = "sha256:" + "0" * 64
        with self.assertRaisesRegex(RuntimeError, "acquisition-only"):
            leaf.collect(ForbiddenContext())

    def test_versioned_official_candidate_declares_both_read_bounds(self):
        leaf = load_leaf()
        self.assertEqual(leaf.CLIENT, "grok")
        self.assertEqual(leaf.VERSION, "1.0.70")
        candidate = leaf.ACQUISITION
        url = urlsplit(candidate["url"])
        self.assertEqual((url.scheme, url.netloc), ("https", "x.ai"))
        self.assertEqual(url.path, "/cli/grok-1.0.70-linux-x86_64.gz")
        self.assertFalse(url.query or url.fragment or url.username or url.password)
        self.assertEqual(candidate["max_bytes"], 96 << 20)
        self.assertEqual(candidate["max_expanded_bytes"], 256 << 20)
        self.assertEqual(candidate["format"], "gzip")
        self.assertEqual(candidate["runtime"], "native")
        self.assertEqual(candidate["entrypoint"], "grok")


if __name__ == "__main__":
    unittest.main()
