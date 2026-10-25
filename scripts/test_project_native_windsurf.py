"""Inert current-Devin contracts; no vendor binary is downloaded or executed."""
import importlib.util
from pathlib import Path
import unittest

SPEC = importlib.util.spec_from_file_location(
    "windsurf_project_collector", Path(__file__).with_name("project_native_collectors") / "windsurf.py")
devin = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(devin)


def help_body(usage, required=()):
    return ("Usage: " + usage + " [OPTIONS]\nOptions:\n  --help\n" + "\n".join(required)).encode()


class FakeContext:
    def __init__(self, replacement=None):
        self.calls = []
        self.replacement = replacement or {}

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        self.calls.append((label, arguments, project, extra_env, timeout))
        _, _, usage, required = next(probe for probe in devin.PROBES if probe[0] == label)
        body = b"devin 3000.11.205 (6dea39440ae3)\n" if usage is None else help_body(usage, required)
        return self.replacement.get(label, ({"exit": 0, "error": None}, body, b""))


class DevinCollectorTests(unittest.TestCase):
    def test_exact_current_harness_archive_and_executable_are_pinned(self):
        self.assertEqual(devin.CLIENT, "windsurf")
        self.assertEqual(devin.VERSION, "3000.11.205")
        self.assertEqual(devin.ACQUISITION, {
            "url": "https://static.devin.ai/cli/3000.11.205/"
                   "devin-3000.11.205-x86_64-unknown-linux.tar.gz",
            "integrity": "sha256:2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4",
            "expected_bytes": 73_679_375, "max_bytes": 96 << 20,
            "max_expanded_bytes": 256 << 20, "format": "tar.gz", "entrypoint": "bin/devin",
            "runtime": "native",
            "binary_sha256": "642c82e32cb3dec90cf5ab01975520b425307872c7a650c9fb5b0653f37681f4",
        })

    def test_only_previous_help_forms_run_and_no_scope_or_runtime_case_passes(self):
        ctx = FakeContext()
        result = devin.collect(ctx)
        self.assertEqual(devin.AVAILABLE_PHASES, ("acquire", "help"))
        expected = [
            ["--version"], ["--help"], ["mcp", "--help"], ["skills", "--help"],
            ["mcp", "list", "--help"], ["mcp", "get", "--help"],
            ["mcp", "add", "--help"], ["mcp", "remove", "--help"],
            ["skills", "list", "--help"], ["skills", "paths", "--help"],
        ]
        self.assertEqual([call[1] for call in ctx.calls], expected)
        self.assertTrue(all(call[2:] == ("a", None, 30) for call in ctx.calls))
        self.assertEqual(list(result["observations"]), [call[0] for call in ctx.calls])
        self.assertEqual([row["id"] for row in result["cases"]], ["windsurf.help"])
        self.assertEqual(result["cases"][0]["status"], "passed")
        self.assertIn("windsurf.scope-observation", result["unobserved"])
        self.assertIn("windsurf.mcp-tools-call", result["unobserved"])

    def test_wrong_harness_version_build_and_prerelease_stop_before_help(self):
        for body in (b"windsurf 3000.11.205 (6dea39440ae3)\n",
                     b"devin 3000.11.205 (other)\n", b"devin 3000.11.20 (6dea39440ae3)\n",
                     b"devin 3000.11.205-alpha (6dea39440ae3)\n", b"devin 3000.11.205\n"):
            with self.subTest(body=body):
                ctx = FakeContext({"devin-version": ({"exit": 0, "error": None}, body, b"")})
                with self.assertRaisesRegex(ValueError, "harness/version/build"):
                    devin.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_failed_or_missing_process_result_is_not_an_auth_boundary(self):
        for record in ({"exit": 1, "error": None}, {"exit": 0, "error": "deadline"},
                       {"exit": False, "error": None}, {"exit": 0}, {}):
            with self.subTest(record=record):
                ctx = FakeContext({"devin-help": (record, b"Authentication required", b"")})
                with self.assertRaisesRegex(ValueError, "probe failed"):
                    devin.collect(ctx)

    def test_mismatched_usage_unknown_option_and_unexpected_stream_fail(self):
        for stdout, stderr in ((help_body("devin mcp get"), b""),
                               (help_body("devin skills list"), b""),
                               (b"Authentication required", b""), (b"\xff", b""),
                               (help_body("devin skills list", ("--json",)), b"warning"),
                               (help_body("devin skills list", ("--json",)) + b"\x1b[2J", b"")):
            with self.subTest(stdout=stdout, stderr=stderr):
                ctx = FakeContext({"devin-skills-list-help": ({"exit": 0, "error": None}, stdout, stderr)})
                with self.assertRaises(ValueError):
                    devin.collect(ctx)


if __name__ == "__main__":
    unittest.main()
