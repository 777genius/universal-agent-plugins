"""Inert Kiro help/raw-list contracts, without native or hosted execution."""
import json
from pathlib import Path
import tempfile
import unittest

from project_native_collectors import kiro


class FakeContext:
    def __init__(self, root):
        self.root = Path(root)
        self.home = self.root / "home"
        self.projects = {scope: self.root / scope for scope in ("a", "b", "user")}
        self.calls, self.replace, self.env, self.argv = [], {}, {}, ["/not/executed/kiro-cli"]
        for path in (self.home, *self.projects.values()):
            path.mkdir()

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        self.calls.append((label, arguments, project, extra_env, timeout))
        if label in self.replace:
            replacement = self.replace[label]
            return replacement() if callable(replacement) else replacement
        if label == "version":
            body = b"kiro-cli 2.34.0\n"
        elif label == "all-help":
            return {"exit": 0, "error": None}, b"Commands:\n  mcp\nOptions:\n --help-all\n --help\n", kiro._ALL_HELP_STDERR
        elif label.endswith("help"):
            body = b"Usage: kiro-cli list workspace --name --agent-engine --auth-method --trust-workspace"
        else:
            # Deliberately not an assumed vendor schema: it cannot pass a
            # scope-observation ID regardless of which marker names occur.
            body = b"Opaque read-only listing.\n"
        return {"exit": 0, "error": None}, body, b""


class KiroCollectorTests(unittest.TestCase):
    def context(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        return FakeContext(directory.name)

    def test_only_help_is_passed_and_source_text_never_qualifies(self):
        ctx = self.context()
        result = kiro.collect(ctx)
        self.assertEqual(kiro.AVAILABLE_PHASES, ("acquire", "help"))
        self.assertEqual([row["id"] for row in result["cases"]], ["kiro.help"])
        facts = result["observations"]
        self.assertFalse(any(value for key, value in facts.items() if key.endswith("_qualified")))
        self.assertEqual(len(facts["raw_source_list_observations"]), 6)
        self.assertEqual(len(ctx.calls), 14)
        for _, arguments, _, extra_env, timeout in ctx.calls:
            self.assertIn(arguments, [["--version"], *[list(args) for _, args, _ in kiro.HELP],
                                      ["mcp", "list", "workspace"], ["mcp", "list", "global"]])
            self.assertIsNone(extra_env)
            self.assertEqual(timeout, 30)
        for scope in ("a", "b", "user"):
            root = ctx.home if scope == "user" else ctx.projects[scope]
            document = json.loads((root / ".kiro/settings/mcp.json").read_text())
            self.assertEqual(set(document["mcpServers"]), {kiro.NAMES[scope]})
            self.assertIs(document["mcpServers"][kiro.NAMES[scope]]["disabled"], True)

    def test_help_advertisement_does_not_claim_selected_runtime_mode(self):
        ctx = self.context()
        ctx.replace["chat-help"] = ({"exit": 0, "error": None}, b"Usage: kiro-cli chat", b"")
        result = kiro.collect(ctx)
        self.assertFalse(result["observations"]["advertised"]["chat_trust_workspace"])
        self.assertNotIn("selected_engine", result["observations"])
        self.assertNotIn("kiro.runtime-attempt", [row["id"] for row in result["cases"]])

    def test_opaque_marker_tokens_do_not_establish_scope_success(self):
        ctx = self.context()
        ctx.replace["a-workspace-raw-list"] = ({"exit": 0, "error": None}, " ".join(kiro.NAMES.values()).encode(), b"")
        result = kiro.collect(ctx)
        self.assertEqual(result["observations"]["raw_source_list_observations"][0]["fixture_name_tokens"], list(kiro.NAMES.values()))
        self.assertFalse(result["observations"]["scope_observation_qualified"])
        self.assertEqual([row["id"] for row in result["cases"]], ["kiro.help"])

    def test_process_capture_auth_and_wrong_version_fail_without_boundary(self):
        for replacement in [({"exit": 1}, b"Authentication required", b""),
                            ({"exit": 0, "error": None}, b"kiro-cli 2.34.0", b"warning"),
                            ({"exit": 0, "error": "timeout"}, b"kiro-cli 2.34.0", b""),
                            ({}, b"kiro-cli 2.34.0", b""),
                            ({"exit": 0, "error": None}, b"kiro-cli 2.33.1", b"")]:
            with self.subTest(replacement=replacement):
                ctx = self.context()
                ctx.replace["version"] = replacement
                with self.assertRaises(ValueError):
                    kiro.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_only_all_help_accepts_the_exact_measured_stderr(self):
        body = b"Commands:\nOptions:\n  --help-all\n  --help\n"
        ctx = self.context()
        with self.assertRaisesRegex(ValueError, "exact argv"):
            kiro._probe(ctx, {}, "all-help", ("mcp", "--help"))
        self.assertEqual(ctx.calls, [])
        for stderr in [b"", kiro._ALL_HELP_STDERR + b"warning\n",
                       kiro._ALL_HELP_STDERR.rstrip(),
                       kiro._ALL_HELP_STDERR.replace(b"    kiro", b"   kiro")]:
            ctx = self.context()
            ctx.replace["all-help"] = {"exit": 0, "error": None}, body, stderr
            with self.subTest(stderr=stderr), self.assertRaises(ValueError):
                kiro.collect(ctx)
        for label in ("version", "top-help", "mcp-help"):
            ctx = self.context()
            ctx.replace[label] = {"exit": 0, "error": None}, body, kiro._ALL_HELP_STDERR
            with self.subTest(label=label), self.assertRaises(ValueError):
                kiro.collect(ctx)
        for invalid in [b"Usage: kiro-cli\n" + body, body.replace(b"Commands:", b"Unknown:"),
                        body.replace(b"--help-all", b"--other")]:
            ctx = self.context()
            ctx.replace["all-help"] = {"exit": 0, "error": None}, invalid, kiro._ALL_HELP_STDERR
            with self.subTest(body=invalid), self.assertRaises(ValueError):
                kiro.collect(ctx)

    def test_source_list_error_is_not_a_successful_auth_boundary(self):
        for body in [b"", b"Authentication required", b"Please log in", b"Error: invalid config", b"\xff", b"\x1b[2Junknown"]:
            ctx = self.context()
            ctx.replace["a-workspace-raw-list"] = ({"exit": 0, "error": None}, body, b"")
            with self.subTest(body=body), self.assertRaises(ValueError):
                kiro.collect(ctx)

    def test_config_mutation_fails(self):
        ctx = self.context()
        def mutated():
            (ctx.home / ".kiro/settings/mcp.json").write_text("{}\n")
            return {"exit": 0, "error": None}, b"opaque", b""
        ctx.replace["a-workspace-raw-list"] = mutated
        with self.assertRaisesRegex(ValueError, "authored fixture"):
            kiro.collect(ctx)

    def test_original_user_home_cannot_be_project(self):
        ctx = self.context()
        ctx.projects["a"] = ctx.home
        with self.assertRaisesRegex(ValueError, "original user home"):
            kiro.collect(ctx)
        self.assertEqual(ctx.calls, [])


if __name__ == "__main__":
    unittest.main()
