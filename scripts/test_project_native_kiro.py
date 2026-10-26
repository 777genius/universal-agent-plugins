"""Inert Kiro help/raw-list contracts, without native or hosted execution."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from project_native_collectors import kiro


class FakeContext:
    def __init__(self, root):
        self.root = Path(root)
        self.home = self.root / "home"
        self.projects = {scope: self.root / scope for scope in ("a", "b", "user")}
        self.calls, self.replace, self.env = [], {}, {"PATH": "/usr/bin:/bin"}
        self.probe_envs, self.identities = [], {}
        for path in (self.home, *self.projects.values()):
            path.mkdir()
        directory = self.root / "client/kirocli/bin"
        directory.mkdir(parents=True)
        for name, digest in kiro._RUNTIME_MEMBERS.items():
            path = directory / name
            path.write_text("inert metadata fixture; never executed\n")
            path.chmod(0o700)
            self.identities[path] = digest
        self.argv = [str(directory / "kiro-cli")]

    def probe(self, label, arguments, project="a", extra_env=None, timeout=30):
        self.calls.append((label, arguments, project, extra_env, timeout))
        self.probe_envs.append(dict(self.env))
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
        self.assertIsNone(facts["read_only_authentication_boundary"])
        self.assertEqual(len(facts["raw_source_list_observations"]), 6)
        self.assertEqual(len(ctx.calls), 14)
        for env in ctx.probe_envs:
            self.assertEqual(env, {"PATH": str(ctx.root / "client/kirocli/bin") + ":/usr/bin:/bin"})
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

    def test_companion_path_identity_and_mode_are_required_before_any_child(self):
        for mutation in ("argv", "missing", "identity", "mode", "link", "hardlink", "owner", "path"):
            ctx = self.context()
            companion = ctx.root / "client/kirocli/bin/kiro-cli-chat"
            if mutation == "argv":
                ctx.argv = [str(companion)]
            elif mutation == "missing":
                companion.unlink()
            elif mutation == "identity":
                ctx.identities[companion] = "0" * 64
            elif mutation == "mode":
                companion.chmod(0o600)
            elif mutation == "link":
                companion.unlink()
                companion.symlink_to("kiro-cli")
            elif mutation == "hardlink":
                (companion.parent / "unexpected-hardlink").hardlink_to(companion)
            elif mutation == "path":
                ctx.env["PATH"] = "/unexpected:/usr/bin:/bin"
            with self.subTest(mutation=mutation):
                if mutation == "owner":
                    with patch.object(kiro.os, "getuid", return_value=companion.stat().st_uid + 1):
                        with self.assertRaises(ValueError):
                            kiro.collect(ctx)
                else:
                    with self.assertRaises(ValueError):
                        kiro.collect(ctx)
                self.assertEqual(ctx.calls, [])

    def test_companion_change_after_child_and_path_change_before_next_are_refused(self):
        for mutation in ("mode", "path"):
            ctx = self.context()
            def changed():
                if mutation == "mode":
                    (ctx.root / "client/kirocli/bin/kiro-cli-chat").chmod(0o600)
                else:
                    ctx.env["PATH"] = "/usr/bin:/bin"
                return {"exit": 0, "error": None}, b"kiro-cli 2.34.0\n", b""
            ctx.replace["version"] = changed
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), 1)

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

    def test_only_exact_measured_first_list_authentication_refusal_stops_research(self):
        ctx = self.context()
        ctx.replace["a-workspace-raw-list"] = (
            {"exit": 1, "error": None}, b"", kiro._LIST_AUTH_STDERR)
        result = kiro.collect(ctx)
        self.assertEqual(len(ctx.calls), 9)
        self.assertEqual(ctx.calls[-1], (
            "a-workspace-raw-list", ["mcp", "list", "workspace"], "a", None, 30))
        self.assertTrue(all(call[-1] == 30 for call in ctx.calls))
        self.assertEqual([row["id"] for row in result["cases"]], ["kiro.help"])
        facts = result["observations"]
        self.assertEqual(facts["raw_source_list_observations"], [])
        self.assertEqual(facts["read_only_authentication_boundary"], {
            "probe": "a-workspace-raw-list", "cwd_selection": "a",
            "arguments": ["mcp", "list", "workspace"],
            "kind": "authentication-required", "exit": 1,
            "remaining_list_commands_unobserved": 5})
        self.assertFalse(any(value for key, value in facts.items() if key.endswith("_qualified")))
        self.assertEqual(result["unobserved"], [
            "kiro.scope-observation", "kiro.project-lifecycle",
            "kiro.native-restart-removal", "kiro.mcp-initialize",
            "kiro.mcp-tools-list", "kiro.mcp-tools-call"])

    def test_changed_or_missing_authentication_tuple_is_refused(self):
        good = ({"exit": 1, "error": None}, b"", kiro._LIST_AUTH_STDERR)
        replacements = [
            ({"exit": value, "error": None}, good[1], good[2])
            for value in (0, 2, True, "1", None)]
        replacements += [
            ({"exit": 1}, good[1], good[2]),
            ({"error": None}, good[1], good[2]),
            ({"exit": 1, "error": "timeout"}, good[1], good[2]),
            (None, good[1], good[2]),
            (good[0], b"unexpected output", good[2]),
            (good[0], "", good[2])]
        replacements += [(good[0], good[1], body) for body in (
            b"", good[2].rstrip(), good[2].replace(b"\n", b"\r\n"),
            good[2] + b"warning\n", b" " + good[2],
            b"\x1b[31m" + good[2], b"Authentication required\n",
            good[2].decode(), None)]
        for replacement in replacements:
            ctx = self.context()
            ctx.replace["a-workspace-raw-list"] = replacement
            with self.subTest(replacement=replacement), self.assertRaises(ValueError):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), 9)

    def test_authentication_tuple_cannot_be_misrouted_to_other_operations(self):
        for label in ("version", "top-help", "all-help", "mcp-help",
                      "a-global-raw-list", "b-workspace-raw-list", "user-global-raw-list"):
            ctx = self.context()
            ctx.replace[label] = ({"exit": 1, "error": None}, b"", kiro._LIST_AUTH_STDERR)
            with self.subTest(label=label), self.assertRaises(ValueError):
                kiro.collect(ctx)
        for arguments, project in ((["mcp", "list", "global"], "a"),
                                   (["mcp", "list", "workspace"], "b"),
                                   (["mcp", "list", "workspace", "--extra"], "a")):
            ctx = self.context()
            with self.subTest(arguments=arguments, project=project), self.assertRaisesRegex(ValueError, "exact argv/project"):
                kiro._probe(ctx, {}, "a-workspace-raw-list", arguments, project)
            self.assertEqual(ctx.calls, [])

    def test_authentication_refusal_still_checks_companions_path_and_authored_configs(self):
        for mutation in ("companion", "path", "config-content", "config-mode", "config-link"):
            ctx = self.context()
            def changed():
                config = ctx.home / ".kiro/settings/mcp.json"
                if mutation == "companion":
                    (ctx.root / "client/kirocli/bin/kiro-cli-chat").chmod(0o600)
                elif mutation == "path":
                    ctx.env["PATH"] = "/usr/bin:/bin"
                elif mutation == "config-content":
                    config.write_text("{}\n")
                elif mutation == "config-mode":
                    config.chmod(0o644)
                else:
                    config.unlink()
                    config.symlink_to(ctx.projects["a"] / ".kiro/settings/mcp.json")
                return {"exit": 1, "error": None}, b"", kiro._LIST_AUTH_STDERR
            ctx.replace["a-workspace-raw-list"] = changed
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), 9)

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
