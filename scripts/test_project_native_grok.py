"""Inert Grok pinned help contracts; no vendor download or execution."""
import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch
import urllib.request
from urllib.parse import urlsplit


LEAF = Path(__file__).with_name("project_native_collectors") / "grok.py"


def load_leaf():
    spec = importlib.util.spec_from_file_location("grok_project_help", LEAF)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


grok = load_leaf()


def help_body(command):
    name = "grok" + (" " + command if command else "")
    suffix = " <COMMAND>" if command in ("mcp", "plugin") else " [OPTIONS]"
    return ("Usage: " + name + suffix + "\nOptions:\n  --help\n"
            + ("Commands:\n  inspect\n  mcp\n  plugin\n" if not command else "")).encode()


class FakeContext:
    phase = "help"

    def __init__(self, replacement=None):
        self.calls, self.replacement = [], replacement or {}

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        self.calls.append((label, arguments, project, extra_env, timeout))
        command = next(command for name, _, command in grok.PROBES if name == label)
        # Inert grammar example, not a vendor commit observation.
        stdout = b"grok 1.0.70 (0123456789ab)\n" if command is None else help_body(command)
        return self.replacement.get(label, ({"exit": 0, "error": None}, stdout, b""))


class GrokCollectorTests(unittest.TestCase):
    def test_measured_official_archive_and_executable_pins_and_bounds(self):
        self.assertEqual(grok.CLIENT, "grok")
        self.assertEqual(grok.VERSION, "1.0.70")
        self.assertEqual(grok.AVAILABLE_PHASES, ("acquire", "help"))
        self.assertFalse(getattr(grok, "ACQUISITION_ONLY", False))
        expected = {"url": "https://x.ai/cli/grok-1.0.70-linux-x86_64.gz",
                    "integrity": "sha256:11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc",
                    "expected_bytes": 76_760_282,
                    "binary_sha256": "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9",
                    "max_bytes": 96 << 20, "max_expanded_bytes": 256 << 20,
                    "format": "gzip", "entrypoint": "grok", "runtime": "native"}
        self.assertEqual(grok.ACQUISITION, expected)
        url = urlsplit(grok.ACQUISITION["url"])
        self.assertFalse(url.query or url.fragment or url.username or url.password)

    def test_import_and_fake_help_have_no_process_network_or_profile_effect(self):
        with (patch.object(subprocess, "Popen", side_effect=AssertionError("process execution")) as process,
              patch.object(urllib.request, "urlopen", side_effect=AssertionError("network acquisition")) as download):
            leaf = load_leaf()
            ctx = FakeContext()
            result = leaf.collect(ctx)
            process.assert_not_called()
            download.assert_not_called()
        self.assertEqual(ctx.calls, [(label, list(args), "a", None, 30) for label, args, _ in grok.PROBES])
        self.assertEqual(len(ctx.calls), 9)
        self.assertEqual([row["id"] for row in result["cases"]], ["grok.help"])
        self.assertEqual(result["cases"][0]["status"], "passed")
        self.assertIn("grok.scope-observation", result["unobserved"])
        self.assertIn("grok.mcp-tools-call", result["unobserved"])
        version = result["observations"]["grok-version"]
        self.assertEqual(version["compiled_commit_observed"], "0123456789ab")
        self.assertIsNone(version["channel_observed"])
        self.assertIs(version["shipping_source_equivalence_claimed"], False)

    def test_unregistered_phases_cannot_become_scope_support(self):
        for phase in (None, "acquire", "observe", "runtime-attempt"):
            with self.subTest(phase=phase):
                ctx = FakeContext()
                ctx.phase = phase
                with self.assertRaisesRegex(ValueError, "explicit help phase"):
                    grok.collect(ctx)
                self.assertEqual(ctx.calls, [])

    def test_source_version_grammar_is_whole_exact_and_does_not_invent_commit(self):
        row = grok._version(b"grok 1.0.70 (abcdef0) [stable]\n")
        self.assertEqual(row["compiled_commit_observed"], "abcdef0")
        self.assertEqual(row["channel_observed"], "stable")
        for body in (b"grok 1.0.67 (abcdef0)\n", b"grok 1.0.700 (abcdef0)\n",
                     b"grok 1.0.70 (abcdef0) [alpha]\n", b"grok 1.0.70 (abcdef0)",
                     b"grok 1.0.70 (ABCDEF0)\n", b"grok 1.0.70 (abcdef)\n",
                     b"grok 1.0.70 (" + b"a" * 41 + b")\n",
                     b"grok 1.0.70 (abcdef0)\nextra\n", b"1.0.70\n"):
            with self.subTest(body=body):
                ctx = FakeContext({"grok-version": ({"exit": 0, "error": None}, body, b"")})
                with self.assertRaisesRegex(ValueError, "version differs"):
                    grok.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_wrong_command_duplicate_usage_and_missing_option_fail(self):
        bodies = [help_body("mcp list"), b"Usage: grok mcpish [OPTIONS]\n--help\n",
                  help_body("mcp") + help_body("mcp"), b"Usage: grok mcp <COMMAND>\n",
                  b"Usage: fake grok mcp <COMMAND>\n--help\n",
                  b"Usage: grok mcp <BROKEN\n  --help\n",
                  b"Usage: grok mcp [BROKEN\n  --help\n",
                  b"Usage: grok mcp <COMMAND> extra\n  --help\n",
                  help_body("mcp").replace(b"--help", b"--helper"),
                  help_body("mcp").replace(b"--help", b"--helpful"),
                  b"Usage: grok mcp <BROKEN\n  --helper\n"]
        for body in bodies:
            with self.subTest(body=body):
                ctx = FakeContext({"grok-mcp-help": ({"exit": 0, "error": None}, body, b"")})
                with self.assertRaises(ValueError):
                    grok.collect(ctx)

    def test_clap_parent_metavariable_and_leaf_options_are_both_legitimate(self):
        grok._help("Usage: grok mcp <COMMAND>\n  --help\n", "mcp")
        grok._help("Usage: grok mcp list [OPTIONS]\n  --help\n", "mcp list")
        grok._help("Usage: grok inspect\n  --help\n", "inspect")
        with self.assertRaisesRegex(ValueError, "selected command surface"):
            grok._help("Usage: grok [OPTIONS]\n  --help\nCommands:\n  inspect\n  mcp\n", "")
        with self.assertRaisesRegex(ValueError, "selected command surface"):
            grok._help("Usage: grok [OPTIONS]\n  --help\nCommands:\n  inspectish\n  mcpish\n  pluginish\n", "")
        grok._help("Usage: grok mcp doctor [OPTIONS] [NAME]\n  -h, --help  Print help\n", "mcp doctor")
        grok._help("Usage: grok inspect <FILES>...\n  -h, --help  Print help\n", "inspect")

    def test_nonzero_missing_error_bool_exit_stderr_and_non_utf8_fail(self):
        failures = [({"exit": 1, "error": None}, help_body("doctor"), b""),
                    ({"exit": 0, "error": "deadline"}, help_body("doctor"), b""),
                    ({"exit": False, "error": None}, help_body("doctor"), b""),
                    ({"exit": 0}, help_body("doctor"), b""),
                    ({"exit": 0, "error": None}, help_body("doctor"), b"Sign in"),
                    ({"exit": 0, "error": None}, b"\xff", b""),
                    ({"exit": 0, "error": None}, b"", b"")]
        for response in failures:
            with self.subTest(response=response):
                ctx = FakeContext({"grok-doctor-help": response})
                with self.assertRaises(ValueError):
                    grok.collect(ctx)


if __name__ == "__main__":
    unittest.main()
