"""Inert Gemini collector contracts. No native process or hosted spoofing."""
import json
from pathlib import Path
import tempfile
import unittest

from project_native_collectors import gemini


def skill_text(rows):
    if not rows:
        return b"No skills discovered.\n"
    text = "Discovered Agent Skills:\n\n"
    for name, path, enabled in rows:
        text += (name + " [" + ("Enabled" if enabled else "Disabled") + "]\n"
                 "  Description: Inert source marker\n  Location:    " + str(path) + "\n\n")
    return text.encode()


def mcp_text(rows, untrusted=False):
    if not rows:
        return b"No MCP servers configured.\n"
    text = "\n".join(gemini._UNTRUSTED) + "\n\n" if untrusted else ""
    text += "Configured MCP servers:\n\n"
    for name, configuration, status in rows:
        indicator = {value: key for key, value in gemini._INDICATORS.items()}[status]
        text += indicator + " " + name + ": " + configuration + " - " + status + "\n"
    return text.encode()


class FakeContext:
    """Supply captured-shaped bytes; this never establishes vendor evidence."""
    def __init__(self, root):
        self.root = Path(root)
        self.home = self.root / "home"
        self.projects = {name: self.root / ("project-" + name) for name in ("a", "b", "user")}
        self.private, self.output = self.root / "private", self.root / "output"
        self.env, self.argv, self.calls, self.replace = {}, ["/not/executed/node", "/not/executed/gemini.js"], [], {}
        for path in (self.home, self.private, self.output, *self.projects.values()):
            path.mkdir()

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        self.calls.append((label, arguments, project, extra_env, timeout))
        if label in self.replace:
            replacement = self.replace[label]
            return replacement() if callable(replacement) else replacement
        record = {"exit": 0, "error": None}
        if label == "version":
            return record, b"0.65.0\n", b""
        if label.endswith("help"):
            return record, b"gemini skills mcp list --all\n", b""
        if label.startswith("empty-"):
            return record, skill_text([]) if label.endswith("skills") else mcp_text([]), b""
        trust = json.loads((self.home / ".gemini/trustedFolders.json").read_text())
        untrusted = trust[str(self.projects[project])] == "DO_NOT_TRUST"
        scopes = ["user"] + ([project] if project != "user" else [])
        if label.endswith("skills"):
            roots = {scope: (self.home if scope == "user" else self.projects[scope]) for scope in scopes}
            visible = [scope for scope in scopes if not untrusted or scope == "user"]
            body = skill_text([(gemini.NAMES[scope], roots[scope] / ".gemini/skills" / gemini.NAMES[scope] / "SKILL.md", True)
                               for scope in visible])
        else:
            body = mcp_text([(gemini.NAMES[scope], gemini.URLS[scope] + " (http)", "Blocked")
                             for scope in scopes], untrusted)
        return record, body, b""


class GeminiCollectorTests(unittest.TestCase):
    def context(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        return FakeContext(directory.name)

    def test_complete_source_observation_is_explicitly_not_runtime(self):
        ctx = self.context()
        result = gemini.collect(ctx)
        self.assertEqual({row["id"] for row in result["cases"]}, {"gemini.help", "gemini.scope-observation"})
        self.assertTrue(all(row["status"] == "passed" for row in result["cases"]))
        facts = result["observations"]
        self.assertFalse(any(value for key, value in facts.items() if key.endswith("_qualified")))
        self.assertEqual(set(facts["scopes"]["a"]["skills"]), {gemini.NAMES["a"], gemini.NAMES["user"]})
        self.assertEqual(set(facts["scopes"]["b"]["skills"]), {gemini.NAMES["b"], gemini.NAMES["user"]})
        self.assertEqual(set(facts["scopes"]["user"]["mcp"]), {gemini.NAMES["user"]})
        self.assertEqual(set(facts["untrusted_a"]["skills"]), {gemini.NAMES["user"]})
        self.assertTrue(all(row["status"] == "Blocked" for row in facts["untrusted_a"]["mcp"].values()))
        self.assertEqual(len(ctx.calls), 18)
        self.assertEqual(len({call[0] for call in ctx.calls}), 18)
        for _, arguments, _, extra_env, timeout in ctx.calls:
            self.assertIn(arguments, [["--version"], *[list(args) for _, args, _ in gemini.HELP],
                                      ["skills", "list", "--all"], ["mcp", "list"]])
            self.assertIsNone(extra_env)
            self.assertEqual(timeout, 30)
        settings = json.loads((ctx.home / ".gemini/settings.json").read_text())
        self.assertEqual(set(settings["mcp"]["excluded"]), set(gemini.NAMES.values()))
        self.assertEqual(set(settings["mcpServers"]), {gemini.NAMES["user"]})

    def test_native_errors_auth_and_wrong_version_fail_without_boundary(self):
        for replacement in [({"exit": 1}, b"Authentication required", b""),
                            ({"exit": 0, "error": None}, b"0.65.0\n", b"Authentication required"),
                            ({"exit": 0, "error": "timeout"}, b"0.65.0", b""),
                            ({}, b"0.65.0", b""),
                            ({"exit": 0, "error": None}, b"0.65.0-preview.1", b"")]:
            with self.subTest(replacement=replacement):
                ctx = self.context()
                ctx.replace["version"] = replacement
                with self.assertRaises(ValueError):
                    gemini.collect(ctx)
                self.assertEqual(len(ctx.calls), 1)

    def test_unknown_help_fails_before_scope_probes(self):
        ctx = self.context()
        ctx.replace["skills-list-help"] = ({"exit": 0, "error": None}, b"unknown command", b"")
        with self.assertRaises(ValueError):
            gemini.collect(ctx)
        self.assertFalse(any(call[0].startswith("empty-") for call in ctx.calls))

    def test_exact_optional_ripgrep_warning_is_limited_to_config_commands(self):
        for arguments in gemini._CONFIG_COMMANDS:
            ctx = self.context()
            ctx.replace["observed"] = ({"exit": 0, "error": None}, b"actual output", gemini._RIPGREP_FALLBACK)
            self.assertEqual(gemini._probe(ctx, {}, "observed", arguments), b"actual output")
        for arguments in (("--version",), ("--help",), ("skills", "list"), ("mcp", "list", "extra")):
            ctx = self.context()
            ctx.replace["observed"] = ({"exit": 0, "error": None}, b"actual output", gemini._RIPGREP_FALLBACK)
            with self.subTest(arguments=arguments), self.assertRaises(ValueError):
                gemini._probe(ctx, {}, "observed", arguments)

    def test_optional_warning_does_not_hide_failure_or_other_diagnostics(self):
        cases = [(0, None, gemini._RIPGREP_FALLBACK + b"error\n"),
                 (0, None, gemini._RIPGREP_FALLBACK.rstrip(b"\n")),
                 (0, None, gemini._RIPGREP_FALLBACK.replace(b"\n", b"\r\n")),
                 (0, None, b" \n"), (0, None, b"Authentication required\n"),
                 (1, None, gemini._RIPGREP_FALLBACK),
                 (False, None, gemini._RIPGREP_FALLBACK),
                 (0, "timeout", gemini._RIPGREP_FALLBACK)]
        for exit_code, error, stderr in cases:
            ctx = self.context()
            ctx.replace["observed"] = ({"exit": exit_code, "error": error}, b"actual output", stderr)
            with self.subTest(exit=exit_code, error=error, stderr=stderr), self.assertRaises(ValueError):
                gemini._probe(ctx, {}, "observed", ("skills", "list", "--all"))

    def test_measured_empty_mcp_stderr_selects_only_existing_empty_parser(self):
        ctx = self.context()
        captured = ({"exit": 0, "error": None}, b"", b"No MCP servers configured.\n")
        ctx.replace["observed"] = captured
        body = gemini._probe(ctx, {}, "observed", ("mcp", "list"))
        self.assertEqual(gemini.parse_mcp(body), ({}, False))
        self.assertEqual(captured, ({"exit": 0, "error": None}, b"", gemini._EMPTY_MCP))
        ctx.replace["empty-a-mcp"] = captured
        result = gemini.collect(ctx)
        self.assertEqual(len(result["cases"]), 2)
        self.assertFalse(result["observations"]["mcp_tools_call_qualified"])

    def test_empty_mcp_stderr_never_hides_other_output_or_capture_failure(self):
        cases = [(("mcp", "list"), {"exit": 0, "error": None}, b"output", gemini._EMPTY_MCP),
                 (("mcp", "list"), {"exit": 0, "error": None}, b"", gemini._EMPTY_MCP + b"error\n"),
                 (("mcp", "list"), {"exit": 0, "error": None}, b"", gemini._EMPTY_MCP.rstrip(b"\n")),
                 (("mcp", "list"), {"exit": 0, "error": None}, b"", gemini._EMPTY_MCP + b"\n"),
                 (("mcp", "list"), {"exit": False}, b"", gemini._EMPTY_MCP),
                 (("mcp", "list"), {"exit": 1}, b"", gemini._EMPTY_MCP),
                 (("mcp", "list"), {"exit": 0, "error": "timeout"}, b"", gemini._EMPTY_MCP),
                 (("mcp", "list"), {"exit": 0}, b"", gemini._EMPTY_MCP),
                 (("mcp", "list"), {"exit": 0, "error": ""}, b"", gemini._EMPTY_MCP),
                 (("mcp", "list"), {"exit": 0, "error": False}, b"", gemini._EMPTY_MCP),
                 (("mcp", "list"), [], b"", gemini._EMPTY_MCP),
                 (("mcp", "list"), {"exit": 0, "error": None}, "", gemini._EMPTY_MCP),
                 (("mcp", "list"), {"exit": 0, "error": None}, b"", bytearray(gemini._EMPTY_MCP)),
                 (("mcp", "list", "extra"), {"exit": 0, "error": None}, b"", gemini._EMPTY_MCP),
                 (("skills", "list", "--all"), {"exit": 0, "error": None}, b"", gemini._EMPTY_MCP)]
        for arguments, record, stdout, stderr in cases:
            ctx = self.context()
            ctx.replace["observed"] = (record, stdout, stderr)
            with self.subTest(arguments=arguments, record=record, stdout=stdout, stderr=stderr), self.assertRaises(ValueError):
                gemini._probe(ctx, {}, "observed", arguments)

    def test_successful_builtin_listing_with_exact_warning_remains_empty_control(self):
        ctx = self.context()
        body = (b"Discovered Agent Skills:\n\nantigravity-support [Enabled] [Built-in]\n"
                b"  Description: Publisher-provided skill\n  Location:    /bundle/builtin/antigravity-support/SKILL.md\n\n"
                b"skill-creator [Enabled] [Built-in]\n  Description: Publisher-provided skill\n"
                b"  Location:    /bundle/builtin/skill-creator/SKILL.md\n\n")
        ctx.replace["empty-a-skills"] = ({"exit": 0, "error": None}, body, gemini._RIPGREP_FALLBACK)
        result = gemini.collect(ctx)
        self.assertEqual(len(result["cases"]), 2)
        self.assertFalse(result["observations"]["mcp_tools_call_qualified"])

    def test_cross_project_skill_is_not_discovery_success(self):
        ctx = self.context()
        ctx.replace["configured-a-skills"] = ({"exit": 0, "error": None}, skill_text([
            (gemini.NAMES["a"], ctx.projects["b"] / ".gemini/skills" / gemini.NAMES["a"] / "SKILL.md", True),
            (gemini.NAMES["user"], ctx.home / ".gemini/skills" / gemini.NAMES["user"] / "SKILL.md", True),
        ]), b"")
        with self.assertRaisesRegex(ValueError, "path or enablement"):
            gemini.collect(ctx)

    def test_unexpected_connection_does_not_pass_config_only_case(self):
        ctx = self.context()
        ctx.replace["configured-a-mcp"] = ({"exit": 0, "error": None}, mcp_text([
            (gemini.NAMES[scope], gemini.URLS[scope] + " (http)", "Connected") for scope in ("a", "user")
        ]), b"")
        with self.assertRaisesRegex(ValueError, "policy differs"):
            gemini.collect(ctx)

    def test_missing_untrusted_warning_fails(self):
        ctx = self.context()
        ctx.replace["untrusted-a-mcp"] = ({"exit": 0, "error": None}, mcp_text([
            (gemini.NAMES[scope], gemini.URLS[scope] + " (http)", "Blocked") for scope in ("a", "user")
        ]), b"")
        with self.assertRaisesRegex(ValueError, "warning was not observed"):
            gemini.collect(ctx)

    def test_untrusted_mcp_exclusion_precedes_trust_status(self):
        ctx = self.context()
        ctx.replace["untrusted-a-mcp"] = ({"exit": 0, "error": None}, mcp_text([
            (gemini.NAMES[scope], gemini.URLS[scope] + " (http)", "Disabled") for scope in ("a", "user")
        ], untrusted=True), b"")
        with self.assertRaisesRegex(ValueError, "policy differs"):
            gemini.collect(ctx)

    def test_authored_configuration_mutation_fails(self):
        ctx = self.context()
        def mutated():
            (ctx.home / ".gemini/settings.json").write_text("{}\n")
            return {"exit": 0, "error": None}, b"0.65.0\n", b""
        ctx.replace["version"] = mutated
        with self.assertRaisesRegex(ValueError, "fixture bytes"):
            gemini.collect(ctx)

    def test_home_cannot_be_selected_as_a_project(self):
        ctx = self.context()
        ctx.projects["a"] = ctx.home
        with self.assertRaisesRegex(ValueError, "original user home"):
            gemini.collect(ctx)
        self.assertEqual(ctx.calls, [])

    def test_native_parent_alias_is_refused_before_following_it(self):
        ctx = self.context()
        outside = self.context().root
        sentinel = outside / "settings.json"
        def aliased():
            original = ctx.home / ".gemini"
            sentinel.write_bytes((original / "settings.json").read_bytes())
            sentinel.chmod(0o600)
            original.rename(ctx.home / ".gemini-original")
            original.symlink_to(outside, target_is_directory=True)
            return {"exit": 0, "error": None}, b"0.65.0\n", b""
        ctx.replace["version"] = aliased
        with self.assertRaisesRegex(ValueError, "parent alias"):
            gemini.collect(ctx)
        self.assertEqual(sentinel.read_bytes(), (ctx.home / ".gemini-original/settings.json").read_bytes())
        self.assertFalse((outside / "trustedFolders.json").exists())


class GeminiParserTests(unittest.TestCase):
    def test_pinned_paths_and_sgr_are_preserved(self):
        body = b"\x1b[1mDiscovered Agent Skills:\x1b[0m\n\nname [Enabled]\n  Description: x\n  Location:    /a path/SKILL.md\n"
        self.assertEqual(gemini.parse_skills(body)["name"]["path"], "/a path/SKILL.md")

    def test_skills_reject_malformed_duplicate_empty_and_control_output(self):
        row = "name [Enabled]\n  Description: x\n  Location:    /a/SKILL.md\n"
        for text in ["", "Authentication required", "Discovered Agent Skills:", "Discovered Agent Skills:\npartial",
                     "Discovered Agent Skills:\n" + row + row,
                     "Discovered Agent Skills:\nname [Enabled]\n  Description: x\n  Location:    \n",
                     "\x1b[2JNo skills discovered."]:
            with self.subTest(text=text), self.assertRaises(ValueError):
                gemini.parse_skills(text.encode())
        with self.assertRaises(UnicodeDecodeError):
            gemini.parse_skills(b"\xff")

    def test_mcp_rejects_partial_duplicate_and_status_mismatch(self):
        row = "○ name: http://127.0.0.1:9/a (http) - Disabled\n"
        for text in ["", "Configured MCP servers:", "Configured MCP servers:\n" + row + row,
                     "Configured MCP servers:\n" + row.replace("○", "✓"),
                     "Configured MCP servers:\n○ name: local - Unknown"]:
            with self.subTest(text=text), self.assertRaises(ValueError):
                gemini.parse_mcp(text.encode())


if __name__ == "__main__":
    unittest.main()
