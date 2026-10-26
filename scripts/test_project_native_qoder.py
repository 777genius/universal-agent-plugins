"""Inert Qoder acquisition/help contracts; no vendor payload is acquired or run."""

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


class FakeContext:
    def __init__(self, leaf):
        self.leaf, self.calls, self.replace = leaf, [], {}

    def probe(self, label, arguments, project="a", timeout=30):
        self.calls.append((label, arguments, project, timeout))
        if label in self.replace:
            return self.replace[label]
        if label == "version":
            return {"exit": 0, "error": None}, b"1.1.78\n", b""
        usage = next(row[2] for row in self.leaf.HELP if row[0] == label)
        return {"exit": 0, "error": None}, (usage + "\n\nOptions:\n  -h, --help  display help for command\n").encode(), b""


class QoderAcquisitionContract(unittest.TestCase):
    def test_import_and_fake_collection_cannot_launch_or_acquire_a_vendor(self):
        with (
            patch.object(subprocess, "Popen", side_effect=AssertionError("vendor process")) as process,
            patch.object(urllib.request, "urlopen", side_effect=AssertionError("network download")) as download,
        ):
            leaf = load_leaf()
            self.assertEqual(leaf.AVAILABLE_PHASES, ("acquire", "help"))
            self.assertFalse(getattr(leaf, "ACQUISITION_ONLY", False))
            self.assertEqual(leaf.ACQUISITION["entrypoint"], "qodercli")
            leaf.collect(FakeContext(leaf))
            process.assert_not_called()
            download.assert_not_called()

    def test_only_five_fixed_probes_and_help_case_are_emitted(self):
        leaf = load_leaf()
        ctx = FakeContext(leaf)
        result = leaf.collect(ctx)
        self.assertEqual(ctx.calls, [
            ("version", ["--version"], "a", 30),
            ("top-help", ["--help"], "a", 30),
            ("plugins-help", ["plugins", "--help"], "a", 30),
            ("plugins-list-help", ["plugins", "list", "--help"], "a", 30),
            ("mcp-help", ["mcp", "--help"], "a", 30),
        ])
        self.assertEqual(result["cases"], [{"id": "qoder.help", "status": "passed",
                         "reason": "Pinned fixed version/help table observed; no plugin operation, scope or runtime qualification"}])
        self.assertEqual(result["observations"]["client_version_measured"], "1.1.78")
        self.assertTrue(all(value is False for key, value in result["observations"].items() if key.endswith("_qualified")))

    def test_process_capture_errors_and_malformed_stream_types_fail_at_first_probe(self):
        for record, stdout, stderr in [
            ({"exit": 1, "error": None}, b"Authentication required", b""),
            ({"exit": 0, "error": "overflow"}, b"1.1.78\n", b""),
            ({"exit": 0, "error": None}, b"1.1.78\n", b"warning\n"),
            ({"exit": 0, "error": None}, b"1.1.78\n", b"\n"),
            ({"exit": None, "error": None}, b"1.1.78\n", b""),
            ({"exit": False, "error": None}, b"1.1.78\n", b""),
            ({"exit": 0}, b"1.1.78\n", b""), ({}, b"1.1.78\n", b""),
            (None, b"1.1.78\n", b""),
            ({"exit": 0, "error": None}, "1.1.78\n", b""),
            ({"exit": 0, "error": None}, b"1.1.78\n", ""),
        ]:
            with self.subTest(record=record, stdout=stdout, stderr=stderr):
                leaf = load_leaf()
                ctx = FakeContext(leaf)
                ctx.replace["version"] = record, stdout, stderr
                with self.assertRaisesRegex(ValueError, "probe did not finish cleanly"):
                    leaf.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_wrong_version_or_unknown_help_never_becomes_a_boundary(self):
        for label, stdout, calls in [("version", b"1.1.79\n", 1),
                                     ("top-help", b"Authentication required\n", 2),
                                     ("mcp-help", b"Usage: qodercli mcp-new [options]\n  -h, --help  help\n", 5)]:
            with self.subTest(label=label):
                leaf = load_leaf()
                ctx = FakeContext(leaf)
                ctx.replace[label] = {"exit": 0, "error": None}, stdout, b""
                with self.assertRaises(ValueError):
                    leaf.collect(ctx)
                self.assertEqual(len(ctx.calls), calls)

    def test_exact_retained_version_bytes(self):
        leaf = load_leaf()
        self.assertEqual(leaf.parse_version(b"1.1.78\n"), "1.1.78")
        for body in [b"1.1.78", b"1.1.78\r\n", b"1.1.79\n", b" 1.1.78\n", b"1.1.78\nextra\n"]:
            with self.subTest(body=body), self.assertRaisesRegex(ValueError, "version differs"):
                leaf.parse_version(body)

    def test_command_usage_is_anchored_and_help_is_required(self):
        leaf = load_leaf()
        parent = "Usage: qodercli plugins|plugin [options] [command]"
        listing = "Usage: qodercli plugins list [options]"
        for usage in (parent, listing):
            self.assertEqual(leaf.parse_help((usage + "\n\n  -h, --help  display help for command\n").encode(), usage), usage)
        good = (listing + "\n  -h, --help  display help for command\n").encode()
        for body in [b"prefix\n" + good, good.replace(b"plugins list", b"plugins listings"),
                     good.replace(b"plugins list", b"plugins"), good + (listing + "\n").encode(),
                     good.replace(b"--help", b"--helper"), good.replace(b"--help", b"--hel"),
                     good + b"\x1b[0m", good + b"\x00", good + b"\xff"]:
            with self.subTest(body=body), self.assertRaises(ValueError):
                leaf.parse_help(body, listing)

    def test_measured_main_usage_requires_exact_query_suffix(self):
        leaf = load_leaf()
        usage = "Usage: qodercli [options] [command] [query...]"
        body = (usage + "\n\nOptions:\n  -h, --help  display help for command\n").encode()
        self.assertEqual(leaf.HELP[0][2], usage)
        self.assertEqual(leaf.parse_help(body, usage), usage)
        for wrong in [body.replace(b" [query...]", b""),
                      body.replace(b" [query...]", b" [query...] [extra]"),
                      body.replace(b"[command] [query...]", b"[query...] [command]"),
                      body.replace(b"qodercli ", b"qodercli-other ")]:
            with self.subTest(body=wrong), self.assertRaises(ValueError):
                leaf.parse_help(wrong, usage)

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
