"""Inert Kimi collector contracts; never acquires or executes vendor bytes."""
import importlib.util
from pathlib import Path
import unittest

SPEC = importlib.util.spec_from_file_location(
    "kimi_project_collector", Path(__file__).with_name("project_native_collectors") / "kimi.py")
kimi = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(kimi)


def help_body(command):
    name = "kimi" + (" " + command if command else "")
    return ("Usage: " + name + " [options]\nOptions:\n  --help\n"
            + ("  --version\nCommands:\n  doctor\n  acp\n  web\n" if not command else "")).encode()


class FakeContext:
    def __init__(self, replacement=None):
        self.calls = []
        self.replacement = replacement or {}

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        self.calls.append((label, arguments, project, extra_env, timeout))
        command = next(command for name, _, command in kimi.PROBES if name == label)
        default = ({"exit": 0, "error": None}, b"2.3.2\n" if command is None else help_body(command), b"")
        return self.replacement.get(label, default)


class KimiCollectorTests(unittest.TestCase):
    def test_frozen_archive_is_current_official_native_entry(self):
        self.assertEqual(kimi.CLIENT, "kimi")
        self.assertEqual(kimi.VERSION, "2.3.2")
        self.assertEqual(kimi.ACQUISITION, {
            "url": "https://github.com/MoonshotAI/kimi-code/releases/download/"
                   "%40moonshot-ai/kimi-code%402.3.2/kimi-code-linux-x64.tar.gz",
            "integrity": "sha256:4715ccd1a591b64683929a14b26b3d07ac4c484e4da3881b22a2ccca56304042",
            "max_bytes": 96 << 20, "expected_bytes": 65_938_009,
            "format": "tar.gz", "entrypoint": "kimi", "runtime": "native",
        })

    def test_only_fixed_help_surface_is_observed_without_scope_claim(self):
        ctx = FakeContext()
        result = kimi.collect(ctx)
        self.assertEqual(kimi.AVAILABLE_PHASES, ("acquire", "help"))
        self.assertEqual(ctx.calls, [
            ("kimi-version", ["--version"], "a", None, 30),
            ("kimi-help", ["--help"], "a", None, 30),
            ("kimi-doctor-help", ["doctor", "--help"], "a", None, 30),
            ("kimi-acp-help", ["acp", "--help"], "a", None, 30),
            ("kimi-web-help", ["web", "--help"], "a", None, 30),
        ])
        self.assertEqual([row["id"] for row in result["cases"]], ["kimi.help"])
        self.assertEqual(result["cases"][0]["status"], "passed")
        self.assertIn("kimi.scope-observation", result["unobserved"])
        self.assertIn("kimi.mcp-tools-call", result["unobserved"])
        self.assertEqual(list(result["observations"]), [call[0] for call in ctx.calls])
        self.assertEqual(result["observations"]["kimi-version"]["surface"], "version")

    def test_wrong_or_ambiguous_version_stops_before_any_help(self):
        for body in (b"2.3.1\n", b"2.3.20\n", b"2.3.2-alpha\n", b"inert 2.3.2\n", b"2.3.2\nextra\n"):
            with self.subTest(body=body):
                ctx = FakeContext({"kimi-version": ({"exit": 0, "error": None}, body, b"")})
                with self.assertRaisesRegex(ValueError, "version differs"):
                    kimi.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_failed_or_missing_process_result_never_becomes_boundary_or_pass(self):
        for record in ({"exit": 1, "error": None}, {"exit": 0, "error": "deadline"},
                       {"exit": False, "error": None}, {"exit": 0}, {}):
            with self.subTest(record=record):
                ctx = FakeContext({"kimi-web-help": (record, help_body("web"), b"")})
                with self.assertRaisesRegex(ValueError, "probe failed"):
                    kimi.collect(ctx)

    def test_wrong_command_empty_non_utf8_and_stderr_help_fail(self):
        for stdout, stderr in ((b"", b""), (help_body("doctor"), b""),
                               (b"Usage: kimi web [options]\n", b""),
                               (b"\xff", b""), (help_body("web"), b"auth required")):
            with self.subTest(stdout=stdout, stderr=stderr):
                ctx = FakeContext({"kimi-web-help": ({"exit": 0, "error": None}, stdout, stderr)})
                with self.assertRaises(ValueError):
                    kimi.collect(ctx)

    def test_subcommand_help_cannot_satisfy_main_help_by_mentioning_commands(self):
        body = help_body("doctor") + b"--version acp web\n"
        ctx = FakeContext({"kimi-help": ({"exit": 0, "error": None}, body, b"")})
        with self.assertRaisesRegex(ValueError, "requested command"):
            kimi.collect(ctx)
        self.assertEqual(len(ctx.calls), 2)


if __name__ == "__main__":
    unittest.main()
