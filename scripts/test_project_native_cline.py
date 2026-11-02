"""Inert Cline leaf contracts; fake capture only, no vendor execution."""

import importlib.util
from pathlib import Path
import unittest


SPEC = importlib.util.spec_from_file_location(
    "cline_project_help_collector", Path(__file__).with_name("project_native_collectors") / "cline.py")
cline = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(cline)


def help_body(command):
    name = "cline" + (" " + command if command else "")
    return ("Usage: " + name + " [options]\nOptions:\n  --help\n"
            + ("  --version\n  --auto-approve <boolean>\nCommands:\n  config\n  mcp\n" if not command else "")).encode()


class FakeContext:
    def __init__(self, replacements=None):
        self.replacements = replacements or {}
        self.calls = []
        self.env = {"CLINE_NO_AUTO_UPDATE": "0", "GITHUB_TOKEN": "inert-ambient-value"}

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        self.calls.append((label, arguments, project, extra_env, timeout))
        defaults = {
            "cline-version": b"3.0.73\n", "cline-help": help_body(""),
            "cline-config-help": help_body("config"), "cline-mcp-help": help_body("mcp"),
        }
        result = self.replacements.get(label, ({"exit": 0, "error": None}, defaults[label], b""))
        if isinstance(result, Exception):
            raise result
        return result


class ClineHelpContract(unittest.TestCase):
    def test_fixed_help_only_argv_always_suppresses_updater_and_auto_approval(self):
        ctx = FakeContext()
        original_env = dict(ctx.env)
        result = cline.collect(ctx)
        self.assertEqual(ctx.calls, [
            ("cline-version", ["--auto-approve", "false", "--version"], "a", {"CLINE_NO_AUTO_UPDATE": "1"}, 30),
            ("cline-help", ["--auto-approve", "false", "--help"], "a", {"CLINE_NO_AUTO_UPDATE": "1"}, 30),
            ("cline-config-help", ["--auto-approve", "false", "config", "--help"], "a", {"CLINE_NO_AUTO_UPDATE": "1"}, 30),
            ("cline-mcp-help", ["--auto-approve", "false", "mcp", "--help"], "a", {"CLINE_NO_AUTO_UPDATE": "1"}, 30),
        ])
        self.assertEqual(ctx.env, original_env)
        self.assertEqual(cline.AVAILABLE_PHASES, ("acquire", "help"))
        self.assertEqual(result["cases"][0]["id"], "cline.help")
        self.assertEqual(result["cases"][0]["status"], "passed")
        self.assertEqual(len(result["cases"]), 1)
        self.assertEqual(list(result["observations"]), [row[0] for row in ctx.calls])
        self.assertIn("cline.scope-observation", result["unobserved"])
        self.assertIn("cline.mcp-tools-call", result["unobserved"])

    def test_whole_version_identity_precedes_every_help_command(self):
        for body in (b"3.0.72\n", b"3.0.730\n", b"3.0.73-beta\n", b"Cline 3.0.73\n", b"3.0.73\nextra\n"):
            with self.subTest(body=body):
                ctx = FakeContext({"cline-version": ({"exit": 0, "error": None}, body, b"")})
                with self.assertRaisesRegex(ValueError, "version differs"):
                    cline.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_process_failures_do_not_become_auth_boundary_or_partial_pass(self):
        for record in ({"exit": 1, "error": None}, {"exit": 0, "error": "timeout"},
                       {"exit": False, "error": None}, {"exit": 0}, {}):
            with self.subTest(record=record):
                ctx = FakeContext({"cline-config-help": (record, help_body("config"), b"")})
                with self.assertRaisesRegex(ValueError, "probe failed"):
                    cline.collect(ctx)
                self.assertEqual(len(ctx.calls), 3)
        ctx = FakeContext({"cline-version": TimeoutError("shared capture deadline")})
        with self.assertRaises(TimeoutError):
            cline.collect(ctx)
        self.assertEqual(len(ctx.calls), 1)

    def test_main_and_subcommand_help_cannot_substitute_for_one_another(self):
        replacements = (
            ("cline-help", help_body("config") + b"--version --auto-approve config mcp\n"),
            ("cline-config-help", help_body("")),
            ("cline-mcp-help", help_body("config")),
            ("cline-mcp-help", b"Usage: cline mcp-extra [options]\n--help\n"),
        )
        for label, body in replacements:
            with self.subTest(label=label, body=body):
                ctx = FakeContext({label: ({"exit": 0, "error": None}, body, b"")})
                with self.assertRaisesRegex(ValueError, "requested command"):
                    cline.collect(ctx)

    def test_empty_non_utf8_stderr_or_incomplete_help_fails(self):
        for stdout, stderr in ((b"", b""), (b"\xff", b""),
                               (b"Usage: cline mcp [options]\n", b""),
                               (help_body("mcp"), b"authentication required")):
            with self.subTest(stdout=stdout, stderr=stderr):
                ctx = FakeContext({"cline-mcp-help": ({"exit": 0, "error": None}, stdout, stderr)})
                with self.assertRaises(ValueError):
                    cline.collect(ctx)


if __name__ == "__main__":
    unittest.main()
