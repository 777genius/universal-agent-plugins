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
    def __init__(self, phase):
        object.__setattr__(self, "phase", phase)

    def __getattribute__(self, name):
        if name == "phase":
            return object.__getattribute__(self, name)
        raise AssertionError("unreviewed phase accessed native context: " + name)


class CursorAcquisitionContract(unittest.TestCase):
    def test_import_and_unreviewed_phases_never_start_a_process_or_download(self):
        with (
            patch.object(subprocess, "Popen", side_effect=AssertionError("process execution")) as process,
            patch.object(urllib.request, "urlopen", side_effect=AssertionError("network acquisition")) as download,
        ):
            leaf = load_leaf()
            for context in (None, ForbiddenContext("acquire"), ForbiddenContext("observe"), ForbiddenContext("unknown")):
                with self.subTest(context_type=type(context).__name__):
                    with self.assertRaisesRegex(ValueError, "only its reviewed help phase"):
                        leaf.collect(context)
            process.assert_not_called()
            download.assert_not_called()

    def test_measured_pin_limits_native_execution_to_reviewed_help(self):
        leaf = load_leaf()
        self.assertEqual(leaf.AVAILABLE_PHASES, ("acquire", "help"))
        self.assertEqual(leaf.ACQUISITION["integrity"], "sha256:85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294")
        self.assertEqual(leaf.ACQUISITION["entrypoint"], "dist-package/cursor-agent")
        self.assertEqual(leaf.ACQUISITION["binary_sha256"], "2ccc9a8e167797641448b5e5c936f006ba137a2555f117f38c5eb76a5238a233")
        self.assertEqual(leaf.ACQUISITION["expected_bytes"], 184859176)
        self.assertEqual(leaf.ACQUISITION["max_expanded_bytes"], 640 << 20)
        self.assertEqual(leaf.ACQUISITION["transport"], "cursor-publisher-curl")
        with self.assertRaisesRegex(ValueError, "only its reviewed help phase"):
            leaf.collect(ForbiddenContext("observe"))

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

    def test_closed_probe_table_preserves_raw_component_capture_without_scope_claim(self):
        leaf = load_leaf()
        calls = []
        class Context:
            phase = "help"
            def probe(self, label, arguments, **options):
                calls.append((label, arguments, options))
                body = (leaf.VERSION + "\n").encode() if label == "cursor-version" else b"opaque native text\n"
                return {"exit": 0, "error": None}, body, b""
        result = leaf.collect(Context())
        expected = [
            ("cursor-version", ["--version"], {"project": "a", "timeout": 30}),
            ("cursor-help", ["--help"], {"project": "a", "timeout": 30}),
            ("cursor-mcp-help", ["mcp", "--help"], {"project": "a", "timeout": 30}),
            ("cursor-plugin-help", ["plugin", "--help"], {"project": "a", "timeout": 30}),
            ("cursor-empty-a-mcp", ["mcp", "list"], {"project": "a", "timeout": 30}),
            ("cursor-empty-b-mcp", ["mcp", "list"], {"project": "b", "timeout": 30}),
            ("cursor-empty-outside-mcp", ["mcp", "list"], {"project": "user", "timeout": 30}),
        ]
        self.assertEqual(calls, expected)
        self.assertEqual([row["id"] for row in result["cases"]], ["cursor.help"])
        self.assertEqual(len(result["observations"]), 7)
        for case in ("scope-observation", "project-lifecycle", "native-restart-removal",
                     "mcp-initialize", "mcp-tools-list", "mcp-tools-call", "vendor-authentication", "real-model"):
            self.assertIn("cursor." + case, result["unobserved"])

    def test_unknown_version_process_failure_empty_help_and_non_utf8_fail_closed(self):
        leaf = load_leaf()
        variants = (("cursor-version", {"exit": 0, "error": None}, b"other\n", b""),
                    ("cursor-version", {"exit": 0, "error": None}, (leaf.VERSION + "\n").encode(), b"unexpected"),
                    ("cursor-help", {"exit": 1, "error": None}, b"help", b""),
                    ("cursor-help", {"exit": 0, "error": "deadline"}, b"help", b""),
                    ("cursor-help", {"exit": False, "error": None}, b"help", b""),
                    ("cursor-help", {"exit": 0, "error": None}, b" \n", b""),
                    ("cursor-help", {"exit": 0, "error": None}, b"\xff", b""),
                    ("cursor-empty-a-mcp", {"exit": 0, "error": None}, b"raw", b"\xff"))
        for label, record, stdout, stderr in variants:
            class Context:
                phase = "help"
                def probe(self, current, arguments, **options):
                    if current == label:
                        return record, stdout, stderr
                    body = (leaf.VERSION + "\n").encode() if current == "cursor-version" else b"opaque\n"
                    return {"exit": 0, "error": None}, body, b""
            with self.subTest(label=label, stdout=stdout), self.assertRaises(ValueError):
                leaf.collect(Context())


if __name__ == "__main__":
    unittest.main()
