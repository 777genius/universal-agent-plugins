"""Inert Kiro help/raw-list contracts, without native or hosted execution."""
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import patch

from project_native_collectors import kiro


class FakeContext:
    def __init__(self, root):
        self.root = Path(root)
        self.home = self.root / "home"
        self.projects = {scope: self.root / scope for scope in ("a", "b", "user")}
        self.calls, self.replace, self.env, self.summary = [], {}, {"PATH": "/usr/bin:/bin"}, {}
        (self.root / "private").mkdir(mode=0o700)
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

    def test_dedicated_key_enters_only_read_only_list_children(self):
        ctx = self.context()
        ctx.kiro_api_key = "ksk_inert_private_key_12345"
        result = kiro.collect(ctx)
        for label, _, _, extra, _ in ctx.calls:
            expected = ({"KIRO_HOME": str(ctx.root / "private" / "kiro-help-only")}
                        if label in ("all-help", "mcp-help", "mcp-list-help", "mcp-status-help") else
                        {"KIRO_API_KEY": ctx.kiro_api_key} if label.endswith("raw-list") else None)
            self.assertEqual(extra, expected)
        self.assertNotIn("KIRO_API_KEY", ctx.env)
        self.assertFalse(any(value for key, value in result["observations"].items() if key.endswith("_qualified")))

    def test_only_help_is_passed_and_source_text_never_qualifies(self):
        ctx = self.context()
        result = kiro.collect(ctx)
        self.assertEqual(kiro.AVAILABLE_PHASES, ("acquire", "help"))
        self.assertEqual([row["id"] for row in result["cases"]], ["kiro.help"])
        facts = result["observations"]
        self.assertFalse(any(value for key, value in facts.items() if key.endswith("_qualified")))
        self.assertIsNone(facts["read_only_authentication_boundary"])
        self.assertEqual(len(facts["raw_source_list_observations"]), 6)
        self.assertEqual(len(ctx.calls), 16)
        for env in ctx.probe_envs:
            self.assertEqual(env, {"PATH": str(ctx.root / "client/kirocli/bin") + ":/usr/bin:/bin"})
        for _, arguments, _, extra_env, timeout in ctx.calls:
            self.assertIn(arguments, [["--version"], *[list(args) for _, args, _ in kiro.HELP],
                                      ["agent", "list"],
                                      ["mcp", "list", "workspace"], ["mcp", "list", "global"]])
            expected = ({"KIRO_HOME": str(ctx.root / "private" / "kiro-help-only")}
                        if arguments in (["--help-all"], ["mcp", "--help"], ["mcp", "list", "--help"],
                                         ["mcp", "status", "--help"]) else None)
            self.assertEqual(extra_env, expected)
            self.assertEqual(timeout, 30)
        for scope in ("a", "b", "user"):
            root = ctx.home if scope == "user" else ctx.projects[scope]
            document = json.loads((root / ".kiro/settings/mcp.json").read_text())
            self.assertEqual(set(document["mcpServers"]), {kiro.NAMES[scope]})
            self.assertIs(document["mcpServers"][kiro.NAMES[scope]]["disabled"], True)

    def test_help_all_uses_only_disposable_kiro_home_with_opaque_metadata(self):
        ctx = self.context()
        profile = ctx.root / "private" / "kiro-help-only"

        def native_help():
            self.assertEqual(ctx.calls[-1][3], {"KIRO_HOME": str(profile)})
            (profile / "native-cache").mkdir()
            (profile / "native-secret").write_text("not a test report")
            return ({"exit": 0, "error": None},
                    b"Commands:\n  mcp\nOptions:\n --help-all\n --help\n",
                    kiro._ALL_HELP_STDERR)

        ctx.replace["all-help"] = native_help
        result = kiro.collect(ctx)
        self.assertEqual(ctx.summary["kiro_help_private_metadata"], {
            "kind": "directory",
            "direct_entries": {"regular": 1, "directory": 1, "symlink": 0, "special": 0},
            "truncated": False,
        })
        self.assertNotIn("native-cache", str(ctx.summary))
        self.assertNotIn("native-secret", str(ctx.summary))
        self.assertFalse(any(value for key, value in result["observations"].items()
                             if key.endswith("_qualified")))
        self.assertEqual((profile / "native-secret").read_text(), "not a test report")

    def test_mcp_subcommand_help_side_effect_stays_in_disposable_profile(self):
        # All three observed MCP help probes can initialize HOME in pinned 2.34.
        # The isolated profile cannot count as discovery or activation.
        for label in ("mcp-help", "mcp-list-help", "mcp-status-help"):
            with self.subTest(label=label):
                ctx = self.context()
                profile = ctx.root / "private" / "kiro-help-only"

                def native_help():
                    self.assertEqual(ctx.calls[-1][3], {"KIRO_HOME": str(profile)})
                    # Native help must not touch User HOME before fixture setup.
                    self.assertFalse((ctx.home / ".kiro").exists())
                    (profile / "mcp-native-cache").mkdir()
                    body = (b"Usage: kiro-cli mcp status --name NAME\n"
                            if label == "mcp-status-help"
                            else b"Usage: kiro-cli mcp list workspace\n")
                    return {"exit": 0, "error": None}, body, b""

                ctx.replace[label] = native_help
                result = kiro.collect(ctx)
                self.assertTrue((profile / "mcp-native-cache").is_dir())
                # collect() authors a disabled User MCP fixture only later.
                self.assertEqual(
                    set(json.loads((ctx.home / ".kiro/settings/mcp.json").read_text())["mcpServers"]),
                    {kiro.NAMES["user"]})
                self.assertNotIn("mcp-native-cache", str(ctx.summary))
                self.assertFalse(any(value for key, value in result["observations"].items()
                                     if key.endswith("_qualified")))

    def test_mcp_subcommand_help_unowned_home_write_still_fails_closed(self):
        for label in ("mcp-help", "mcp-list-help", "mcp-status-help"):
            with self.subTest(label=label):
                ctx = self.context()

                def native_help():
                    marker = ctx.home / ".kiro"
                    marker.mkdir()
                    (marker / "foreign-sentinel").write_text("keep untouched")
                    return {"exit": 0, "error": None}, b"Usage: kiro-cli mcp list workspace\n", b""

                ctx.replace[label] = native_help
                with self.assertRaisesRegex(ValueError, "unexpected config in: home"):
                    kiro.collect(ctx)
                self.assertEqual(ctx.calls[-1][0], label)
                self.assertEqual((ctx.home / ".kiro/foreign-sentinel").read_text(), "keep untouched")

    def test_help_all_refuses_unowned_home_mutation_even_with_kiro_home(self):
        ctx = self.context()

        def native_help():
            marker = ctx.home / ".kiro"
            marker.mkdir()
            (marker / "foreign-sentinel").write_text("leave untouched")
            return ({"exit": 0, "error": None},
                    b"Commands:\n  mcp\nOptions:\n --help-all\n --help\n",
                    kiro._ALL_HELP_STDERR)

        ctx.replace["all-help"] = native_help
        with self.assertRaisesRegex(ValueError, "unexpected config in: home"):
            kiro.collect(ctx)
        self.assertEqual(ctx.calls[-1][0], "all-help")
        self.assertEqual((ctx.home / ".kiro/foreign-sentinel").read_text(), "leave untouched")

    def test_help_all_profile_conflicts_fail_before_any_native_probe(self):
        for kind in ("existing", "symlink", "private-alias"):
            ctx = self.context()
            profile = ctx.root / "private" / "kiro-help-only"
            if kind == "existing":
                profile.mkdir()
                (profile / "foreign-state").write_text("unchanged")
            elif kind == "symlink":
                outside = ctx.root / "foreign"
                outside.mkdir()
                profile.symlink_to(outside, target_is_directory=True)
            else:
                existing = ctx.root / "private"
                renamed = ctx.root / "private-original"
                existing.rename(renamed)
                existing.symlink_to(renamed, target_is_directory=True)
            with self.subTest(kind=kind), self.assertRaises((ValueError, FileExistsError)):
                kiro.collect(ctx)
            self.assertEqual(ctx.calls, [])

    def test_help_all_replaced_private_profile_fails_closed(self):
        ctx = self.context()
        profile = ctx.root / "private" / "kiro-help-only"

        def native_help():
            original = profile.with_name("retained-native")
            profile.rename(original)
            profile.symlink_to(original, target_is_directory=True)
            return ({"exit": 0, "error": None},
                    b"Commands:\n  mcp\nOptions:\n --help-all\n --help\n",
                    kiro._ALL_HELP_STDERR)

        ctx.replace["all-help"] = native_help
        with self.assertRaisesRegex(ValueError, "disposable root alias refused"):
            kiro.collect(ctx)
        self.assertEqual(ctx.calls[-1][0], "all-help")
        self.assertTrue(profile.is_symlink())


    def test_agent_help_is_exact_and_precedes_configuration_and_listing(self):
        ctx = self.context()

        def agent_help():
            self.assertEqual(ctx.calls[-1], (
                "agent-help", ["agent", "--help"], "a", None, 30))
            for root in (ctx.home, ctx.projects["a"], ctx.projects["b"]):
                self.assertFalse((root / ".kiro/settings/mcp.json").exists())
            self.assertFalse(any("raw-list" in call[0] for call in ctx.calls))
            # Opaque help is a capture only, not an inventory schema.
            return {"exit": 0, "error": None}, b"Usage: kiro-cli agent [unreviewed help]\n", b""

        ctx.replace["agent-help"] = agent_help
        result = kiro.collect(ctx)
        self.assertEqual(len(kiro.HELP), 8)
        self.assertEqual(ctx.calls[8], (
            "agent-help", ["agent", "--help"], "a", None, 30))
        self.assertEqual(ctx.calls[9][0], "a-agent-raw-list")
        self.assertEqual(ctx.calls[10][0], "a-workspace-raw-list")
        self.assertEqual([case["id"] for case in result["cases"]], ["kiro.help"])
        self.assertFalse(any(value for key, value in result["observations"].items()
                             if key.endswith("_qualified")))
        self.assertEqual(result["unobserved"], [
            "kiro.scope-observation", "kiro.project-lifecycle",
            "kiro.native-restart-removal", "kiro.mcp-initialize",
            "kiro.mcp-tools-list", "kiro.mcp-tools-call"])

    def test_agent_list_is_exact_opaque_and_precedes_authored_markers(self):
        ctx = self.context()

        def agent_list():
            self.assertEqual(ctx.calls[-1], (
                "a-agent-raw-list", ["agent", "list"], "a", None, 30))
            self.assertEqual([call[0] for call in ctx.calls[:-1]],
                             ["version", *[label for label, _, _ in kiro.HELP]])
            for root in (ctx.home, *ctx.projects.values()):
                self.assertFalse((root / ".kiro/settings/mcp.json").exists())
                self.assertFalse((root / ".kiro/agents").exists())
            return {"exit": 0, "error": None}, b"Opaque read-only listing.\n", b""

        ctx.replace["a-agent-raw-list"] = agent_list
        result = kiro.collect(ctx)
        self.assertEqual(len(ctx.calls), 16)
        self.assertEqual(ctx.calls[10][0], "a-workspace-raw-list")
        self.assertEqual(result["observations"]["raw_agent_list_observation"], {
            "probe": "a-agent-raw-list", "cwd_selection": "a",
            "sha256_normalized_text": "e6616a2becdf2dabb3a944c65dcce5bc4ddf730b350aaa269974fb0d1a9617b2",
            "interpretation": "opaque available-agent listing; no skill/MCP selection or runtime qualification",
        })
        self.assertEqual([row["id"] for row in result["cases"]], ["kiro.help"])
        self.assertFalse(any(value for key, value in result["observations"].items()
                             if key.endswith("_qualified")))
        for root in (ctx.home, *ctx.projects.values()):
            self.assertFalse((root / ".kiro/agents").exists())

    def test_agent_list_wrong_route_refuses_before_any_child(self):
        routes = [(("agent", verb), "a") for verb in (
            "--help", "create", "edit", "validate", "migrate", "upgrade", "set-default")]
        routes += [(("mcp", "list", "workspace"), "a"),
                   (("agent", "list", "--extra"), "a")]
        routes += [(("agent", "list"), project) for project in ("b", "user", "outside")]
        for arguments, project in routes:
            ctx = self.context()
            with self.subTest(arguments=arguments, project=project):
                with self.assertRaisesRegex(ValueError, "exact argv/project"):
                    kiro._probe(ctx, {}, "a-agent-raw-list", arguments, project)
                self.assertEqual(ctx.calls, [])

    def test_agent_list_failures_stop_before_configuration_or_later_lists(self):
        good = ({"exit": 0, "error": None}, b"Opaque agent list\n", b"")
        replacements = [({"exit": value, "error": None}, good[1], good[2])
                        for value in (1, True, "0", None)]
        replacements += [({}, good[1], good[2]), ({"exit": 0}, good[1], good[2]),
                         ({"exit": 0, "error": "capture failed"}, good[1], good[2]),
                         (None, good[1], good[2])]
        replacements += [(good[0], body, good[2]) for body in (
            "Opaque agent list", bytearray(good[1]), None, b"", b" \n",
            b"error: unknown\n", b"Please log in\n", b"\xff", b"\x1b[2Jagents", b"agents\x00")]
        replacements += [(good[0], good[1], body) for body in (
            "", None, bytearray(), b"warning\n", kiro._ALL_HELP_STDERR, kiro._LIST_AUTH_STDERR)]
        for replacement in replacements:
            ctx = self.context()
            ctx.replace["a-agent-raw-list"] = replacement
            with self.subTest(replacement=replacement), self.assertRaises(ValueError):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), 10)
            self.assertEqual(ctx.calls[-1][0], "a-agent-raw-list")
            for root in (ctx.home, *ctx.projects.values()):
                self.assertFalse((root / ".kiro/settings/mcp.json").exists())
                self.assertFalse((root / ".kiro/agents").exists())

    def test_exact_agent_list_auth_refusal_is_observed_without_fabricated_inventory(self):
        ctx = self.context()
        ctx.replace["a-agent-raw-list"] = (
            {"exit": 1, "error": None}, b"", kiro._LIST_AUTH_STDERR)
        result = kiro.collect(ctx)
        self.assertEqual(len(ctx.calls), 10)
        self.assertEqual(ctx.calls[-1], (
            "a-agent-raw-list", ["agent", "list"], "a", None, 30))
        self.assertEqual([case["id"] for case in result["cases"]], ["kiro.help"])
        facts = result["observations"]
        self.assertIsNone(facts["raw_agent_list_observation"])
        self.assertEqual(facts["raw_source_list_observations"], [])
        self.assertEqual(facts["read_only_authentication_boundary"], {
            "probe": "a-agent-raw-list", "cwd_selection": "a",
            "arguments": ["agent", "list"],
            "kind": "authentication-required", "exit": 1,
            "remaining_list_commands_unobserved": 6})
        self.assertFalse(any(value for key, value in facts.items()
                             if key.endswith("_qualified")))
        for root in (ctx.home, *ctx.projects.values()):
            self.assertFalse((root / ".kiro/settings/mcp.json").exists())
            self.assertFalse((root / ".kiro/agents").exists())

    def test_agent_list_auth_refusal_cannot_hide_unexpected_native_config(self):
        for scope in ("a", "b", "user"):
            for kind in ("native-created", "preexisting"):
                ctx = self.context()
                root = ctx.home if scope == "user" else ctx.projects[scope]
                marker = root / ".kiro"

                if kind == "preexisting":
                    marker.mkdir()
                    (marker / "foreign.txt").write_text("foreign; preserve me\\n")

                def refused():
                    if kind == "native-created":
                        marker.mkdir()
                        (marker / "foreign.txt").write_text("created by native probe\\n")
                    return {"exit": 1, "error": None}, b"", kiro._LIST_AUTH_STDERR

                ctx.replace["a-agent-raw-list"] = refused
                with self.subTest(scope=scope, kind=kind):
                    if kind == "preexisting":
                        with self.assertRaisesRegex(FileExistsError, "preexisting native config"):
                            kiro.collect(ctx)
                        self.assertEqual(ctx.calls, [])
                    else:
                        with self.assertRaisesRegex(ValueError, "native probe left unexpected config"):
                            kiro.collect(ctx)
                        self.assertEqual(len(ctx.calls), 10)
                        self.assertEqual(ctx.calls[-1][0], "a-agent-raw-list")
                self.assertEqual((marker / "foreign.txt").read_text(),
                                 "foreign; preserve me\\n" if kind == "preexisting"
                                 else "created by native probe\\n")
                # Detection must not remove a foreign or unexpected file.
                for other in ("a", "b", "user"):
                    if other != scope:
                        other_root = ctx.home if other == "user" else ctx.projects[other]
                        self.assertFalse((other_root / ".kiro").exists())

    def test_auth_refusal_reports_only_isolated_roots_not_native_file_names(self):
        # Real 2.34.0 can create .kiro during an unauthenticated agent list.
        # Keep the run failed, but identify which disposable root changed
        # without leaking native filenames or config contents into evidence.
        for roots, expected in (
                (("home",), "Kiro native probe left unexpected config in: home"),
                # The post-child cwd guard rejects unexpected user config before
                # collect can summarize the separate auth-refusal boundary.
                (("user",), "Kiro selected user cwd has unowned native config"),
                (("home", "a"), "Kiro native probe left unexpected config in: home, a")):
            ctx = self.context()
            markers = []
            for scope in roots:
                root = ctx.home if scope == "home" else ctx.projects[scope]
                marker = root / ".kiro"
                markers.append(marker)

            def refused():
                for marker in markers:
                    marker.mkdir()
                    (marker / "private-native-output.json").write_text("private fixture secret")
                return {"exit": 1, "error": None}, b"", kiro._LIST_AUTH_STDERR

            ctx.replace["a-agent-raw-list"] = refused
            with self.subTest(roots=roots), self.assertRaises(ValueError) as result:
                kiro.collect(ctx)
            self.assertEqual(str(result.exception), expected)
            self.assertNotIn("private-native-output", str(result.exception))
            self.assertNotIn("private fixture secret", str(result.exception))
            self.assertEqual(len(ctx.calls), 10)
            for marker in markers:
                self.assertEqual((marker / "private-native-output.json").read_text(),
                                 "private fixture secret")

    def test_native_marker_metadata_is_bounded_without_names_or_contents(self):
        ctx = self.context()
        ctx.summary = {}
        marker = ctx.home / ".kiro"
        marker.mkdir()
        (marker / "private-native-output.json").write_text("secret fixture")
        (marker / "child").mkdir()
        outside = ctx.root / "foreign-secret"
        outside.write_text("private outside bytes")
        (marker / "linked-secret").symlink_to(outside)
        with self.assertRaisesRegex(ValueError, "unexpected config in: home"):
            kiro._require_clean_unowned_roots(ctx)
        self.assertEqual(ctx.summary["kiro_unowned_native_metadata"], [{
            "root": "home", "kind": "directory",
            "direct_entries": {"regular": 1, "directory": 1, "symlink": 1, "special": 0},
            "truncated": False,
        }])
        self.assertNotIn("private-native-output", str(ctx.summary))
        self.assertNotIn("private outside", str(ctx.summary))
        self.assertEqual(outside.read_text(), "private outside bytes")

    def test_native_marker_metadata_refuses_symlink_without_following(self):
        ctx = self.context()
        ctx.summary = {}
        outside = ctx.root / "external-private"
        outside.mkdir()
        (outside / "secret.json").write_text("foreign")
        (ctx.home / ".kiro").symlink_to(outside, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "unexpected config in: home"):
            kiro._require_clean_unowned_roots(ctx)
        self.assertEqual(ctx.summary["kiro_unowned_native_metadata"],
                         [{"root": "home", "kind": "symlink"}])
        self.assertEqual((outside / "secret.json").read_text(), "foreign")

    def test_native_marker_metadata_limits_directory_enumeration(self):
        ctx = self.context()
        ctx.summary = {}
        marker = ctx.home / ".kiro"
        marker.mkdir()
        for index in range(20):
            (marker / ("private-" + str(index))).write_text("secret")
        with self.assertRaisesRegex(ValueError, "unexpected config in: home"):
            kiro._require_clean_unowned_roots(ctx)
        self.assertEqual(ctx.summary["kiro_unowned_native_metadata"], [{
            "root": "home", "kind": "directory",
            "direct_entries": {"regular": 16, "directory": 0, "symlink": 0, "special": 0},
            "truncated": True,
        }])
        self.assertNotIn("private-", str(ctx.summary))

    def test_pre_fixture_native_config_write_stops_before_next_read_only_command(self):
        # Version/help can also initialize a native profile. Detect the first
        # side effect rather than launching later native commands on that state.
        good = {
            "version": ({"exit": 0, "error": None}, b"kiro-cli 2.34.0\n", b""),
            "top-help": ({"exit": 0, "error": None}, b"Usage: kiro-cli\n", b""),
            "agent-help": ({"exit": 0, "error": None},
                           b"Usage: kiro-cli agent\n", b""),
        }
        for label, count in (("version", 1), ("top-help", 2), ("agent-help", 9)):
            for scopes in (("home",), ("a",), ("home", "b")):
                ctx = self.context()
                roots = [(ctx.home if scope == "home" else ctx.projects[scope]) / ".kiro"
                         for scope in scopes]

                def changed():
                    for marker in roots:
                        marker.mkdir()
                        (marker / "private-native-output.json").write_text("secret fixture")
                    return good[label]

                ctx.replace[label] = changed
                expected = "Kiro native probe left unexpected config in: " + ", ".join(scopes)
                with self.subTest(label=label, scopes=scopes), self.assertRaises(ValueError) as result:
                    kiro.collect(ctx)
                self.assertEqual(str(result.exception), expected)
                self.assertNotIn("private-native-output", str(result.exception))
                self.assertEqual(len(ctx.calls), count)
                self.assertEqual(ctx.calls[-1][0], label)
                for marker in roots:
                    self.assertEqual((marker / "private-native-output.json").read_text(), "secret fixture")

    def test_native_probe_exception_still_checks_unowned_home(self):
        ctx = self.context()

        def interrupted():
            (ctx.home / ".kiro").mkdir()
            raise RuntimeError("simulated terminated native command")

        ctx.replace["version"] = interrupted
        with self.assertRaisesRegex(ValueError, "Kiro native probe left unexpected config in: home"):
            kiro.collect(ctx)
        self.assertEqual([call[0] for call in ctx.calls], ["version"])
        self.assertTrue((ctx.home / ".kiro").is_dir())

    def test_agent_list_rechecks_companions_and_path_before_markers(self):
        for mutation in ("companion", "path"):
            ctx = self.context()

            def changed():
                if mutation == "companion":
                    (ctx.root / "client/kirocli/bin/kiro-cli-chat").chmod(0o600)
                else:
                    ctx.env["PATH"] = "/usr/bin:/bin"
                return {"exit": 0, "error": None}, b"Opaque agent list\n", b""

            ctx.replace["a-agent-raw-list"] = changed
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), 10)
            self.assertEqual(ctx.calls[-1][0], "a-agent-raw-list")
            for root in (ctx.home, *ctx.projects.values()):
                self.assertFalse((root / ".kiro/settings/mcp.json").exists())

    def test_preexisting_authored_marker_conflict_remains_unadopted(self):
        ctx = self.context()
        original = ctx.home / ".kiro/settings/mcp.json"
        original.parent.mkdir(parents=True)
        original.write_bytes(b"original user bytes\n")
        original.chmod(0o600)
        before = original.stat()
        with self.assertRaises(FileExistsError):
            kiro.collect(ctx)
        self.assertEqual(ctx.calls, [])
        self.assertEqual(original.read_bytes(), b"original user bytes\n")
        after = original.stat()
        for field in ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_nlink",
                      "st_size", "st_mtime_ns", "st_ctime_ns"):
            self.assertEqual(getattr(after, field), getattr(before, field))
        # A late user-config conflict must not leave earlier project fixtures.
        for project in ("a", "b"):
            self.assertFalse((ctx.projects[project] / ".kiro").exists())

    def test_agent_help_wrong_route_refuses_before_any_child(self):
        routes = [(("agent", "list"), "a"), (("chat", "--help"), "a"),
                  (("login", "--help"), "a"), (("acp", "--help"), "a"),
                  (("agent", "--help", "--extra"), "a"),
                  (("agent", "--help"), "b"), (("agent", "--help"), "user"),
                  (("agent", "--help"), "outside")]
        for arguments, project in routes:
            ctx = self.context()
            with self.subTest(arguments=arguments, project=project):
                with self.assertRaisesRegex(ValueError, "exact argv/project"):
                    kiro._probe(ctx, {}, "agent-help", arguments, project)
                self.assertEqual(ctx.calls, [])

    def test_agent_help_keeps_strict_process_text_and_authentication_gates(self):
        good = ({"exit": 0, "error": None}, b"Usage: kiro-cli agent\n", b"")
        replacements = [({"exit": value, "error": None}, good[1], good[2])
                        for value in (1, True, "0", None)]
        replacements += [({}, good[1], good[2]),
                         ({"exit": 0}, good[1], good[2]),
                         ({"exit": 0, "error": "capture failed"}, good[1], good[2]),
                         (None, good[1], good[2]),
                         ({"exit": 1, "error": None}, b"", kiro._LIST_AUTH_STDERR)]
        replacements += [(good[0], body, good[2]) for body in (
            "Usage: kiro-cli agent", bytearray(good[1]), None, b"", b"Commands: agent",
            b"Usage: kiro-cli agent\nerror: unknown\n", b"Usage: please log in\n",
            b"\xff", b"\x1b[2JUsage: agent", b"Usage: agent\x00")]
        replacements += [(good[0], good[1], body) for body in (
            "", None, bytearray(), b"warning\n", kiro._ALL_HELP_STDERR,
            kiro._LIST_AUTH_STDERR)]
        for replacement in replacements:
            ctx = self.context()
            ctx.replace["agent-help"] = replacement
            with self.subTest(replacement=replacement), self.assertRaises(ValueError):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), 9)
            self.assertEqual(ctx.calls[-1][0], "agent-help")
            for root in (ctx.home, ctx.projects["a"], ctx.projects["b"]):
                self.assertFalse((root / ".kiro/settings/mcp.json").exists())

    def test_agent_help_rechecks_companions_and_path_before_any_list(self):
        for mutation in ("companion", "path"):
            ctx = self.context()

            def changed():
                if mutation == "companion":
                    (ctx.root / "client/kirocli/bin/kiro-cli-chat").chmod(0o600)
                else:
                    ctx.env["PATH"] = "/usr/bin:/bin"
                return {"exit": 0, "error": None}, b"Usage: kiro-cli agent\n", b""

            ctx.replace["agent-help"] = changed
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), 9)
            self.assertEqual(ctx.calls[-1][0], "agent-help")

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
        self.assertEqual(len(ctx.calls), 11)
        self.assertEqual(ctx.calls[-3], (
            "agent-help", ["agent", "--help"], "a", None, 30))
        self.assertEqual(ctx.calls[-2], (
            "a-agent-raw-list", ["agent", "list"], "a", None, 30))
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
            self.assertEqual(len(ctx.calls), 11)

    def test_version_help_and_agent_inventory_have_exact_read_only_routes(self):
        routes = [
            ("version", ("--version",)),
            *[(label, args) for label, args, _ in kiro.HELP],
            ("a-agent-raw-list", ("agent", "list")),
        ]
        for label, arguments in routes:
            for changed, project in (
                    (arguments + ("--extra",), "a"),
                    (("chat", "login"), "a"),
                    (arguments, "b"),
                    (arguments, "user")):
                ctx = self.context()
                with self.subTest(label=label, argv=changed, project=project):
                    with self.assertRaisesRegex(ValueError, "exact argv/project"):
                        kiro._probe(ctx, {}, label, changed, project)
                    self.assertEqual(ctx.calls, [])
        for label in ("unknown-probe", "native-install", "agent-login"):
            ctx = self.context()
            with self.subTest(label=label):
                with self.assertRaisesRegex(ValueError, "exact argv/project"):
                    kiro._probe(ctx, {}, label, ("--help",))
                self.assertEqual(ctx.calls, [])

    def test_every_mcp_list_route_refuses_wrong_scope_or_command_before_child(self):
        # No unreviewed native command may run through a read-only capture label.
        # The successful six-route traversal is already covered by
        # test_only_help_is_passed_and_source_text_never_qualifies.
        for project in ("a", "b", "user"):
            for scope in ("workspace", "global"):
                label = project + "-" + scope + "-raw-list"
                other_scope = "global" if scope == "workspace" else "workspace"
                other_project = "b" if project == "a" else "a"
                bad = (
                    (("mcp", "list", other_scope), project),
                    (("mcp", "list", scope, "--extra"), project),
                    (("mcp", "add", scope), project),
                    (("chat", "--help"), project),
                    (("mcp", "list", scope), other_project),
                    (("mcp", "list", scope), "outside"),
                )
                for arguments, selected_project in bad:
                    ctx = self.context()
                    with self.subTest(label=label, argv=arguments, project=selected_project):
                        with self.assertRaisesRegex(ValueError, "exact argv/project"):
                            kiro._probe(ctx, {}, label, arguments, selected_project)
                        self.assertEqual(ctx.calls, [])

    def test_unregistered_read_only_list_label_cannot_launch_child(self):
        for label in ("unknown-raw-list", "a-agent-other-raw-list", "b-user-raw-list"):
            ctx = self.context()
            with self.subTest(label=label):
                with self.assertRaisesRegex(ValueError, "exact argv/project"):
                    kiro._probe(ctx, {}, label, ("mcp", "list", "workspace"), "a")
                self.assertEqual(ctx.calls, [])

    def test_authentication_tuple_cannot_be_misrouted_to_other_operations(self):
        for label in ("version", "top-help", "all-help", "mcp-help",
                      "agent-help", "a-global-raw-list", "b-workspace-raw-list", "user-global-raw-list"):
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
            self.assertEqual(len(ctx.calls), 11)

    def test_config_mutation_fails(self):
        ctx = self.context()
        def mutated():
            (ctx.home / ".kiro/settings/mcp.json").write_text("{}\n")
            return {"exit": 0, "error": None}, b"opaque", b""
        ctx.replace["a-workspace-raw-list"] = mutated
        with self.assertRaisesRegex(ValueError, "authored fixture"):
            kiro.collect(ctx)

    def test_interrupted_probe_still_checks_all_authored_fixtures(self):
        # A failed/terminated native child can also mutate an authored
        # fixture. It must not skip the no-effect checks in that case.
        for scope in ("a", "b", "user"):
            for kind in ("file", "directory"):
                ctx = self.context()
                root = ctx.home if scope == "user" else ctx.projects[scope]
                fixture = root / ".kiro/settings/mcp.json"

                def interrupted():
                    if kind == "file":
                        fixture.write_text('{"private":"not part of the fixture"}')
                    else:
                        parent = fixture.parent
                        parent.chmod(stat.S_IMODE(parent.stat().st_mode) ^ stat.S_IWGRP)
                    raise RuntimeError("private terminated-native diagnostic")

                ctx.replace["a-workspace-raw-list"] = interrupted
                with self.subTest(scope=scope, kind=kind), self.assertRaisesRegex(
                        ValueError, "Kiro changed authored fixture"):
                    kiro.collect(ctx)
                self.assertEqual(len(ctx.calls), 11)
                self.assertEqual(ctx.calls[-1][0], "a-workspace-raw-list")

        # Without mutation, the original process exception is still reported.
        ctx = self.context()

        def clean_interruption():
            raise RuntimeError("transient native failure")

        ctx.replace["a-workspace-raw-list"] = clean_interruption
        with self.assertRaisesRegex(RuntimeError, "transient native failure"):
            kiro.collect(ctx)
        self.assertEqual(len(ctx.calls), 11)

    def test_native_probe_rejects_modified_authored_fixture_directory_modes(self):
        # A read-only native inventory must not relax or rewrite either parent
        # directory while preserving the exact owned mcp.json bytes and inode.
        for scope in ("a", "b", "user"):
            for relative in (".kiro", ".kiro/settings"):
                ctx = self.context()
                root = ctx.home if scope == "user" else ctx.projects[scope]
                parent = root / relative

                def changed_directory():
                    before = parent.lstat()
                    parent.chmod(stat.S_IMODE(before.st_mode) ^ stat.S_IWGRP)
                    return {"exit": 0, "error": None}, b"Opaque read-only listing.\\n", b""

                ctx.replace["a-workspace-raw-list"] = changed_directory
                with self.subTest(scope=scope, relative=relative), self.assertRaisesRegex(
                        ValueError, "fixture directory identity/mode"):
                    kiro.collect(ctx)
                self.assertEqual(len(ctx.calls), 11)
                self.assertEqual(ctx.calls[-1][0], "a-workspace-raw-list")
                self.assertTrue((root / ".kiro/settings/mcp.json").exists())

    def test_replaced_fixture_with_identical_bytes_is_not_accepted_as_owned(self):
        # A native child must not substitute a different inode even when its
        # bytes and permissions mimic an authored disposable research marker.
        for scope in ("a", "b", "user"):
            ctx = self.context()
            root = ctx.home if scope == "user" else ctx.projects[scope]
            fixture = root / ".kiro/settings/mcp.json"

            def substituted():
                original = fixture.lstat()
                body = fixture.read_bytes()
                replacement = fixture.with_name("replacement.json")
                replacement.write_bytes(body)
                replacement.chmod(0o600)
                self.assertNotEqual(replacement.lstat().st_ino, original.st_ino)
                replacement.replace(fixture)
                self.assertEqual(fixture.read_bytes(), body)
                return {"exit": 0, "error": None}, b"Opaque read-only listing.\\n", b""

            ctx.replace["a-workspace-raw-list"] = substituted
            with self.subTest(scope=scope), self.assertRaisesRegex(ValueError, "fixture identity"):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), 11)

    def test_authored_fixture_open_refuses_symlink_swap_between_lstat_and_read(self):
        # The native child can replace the leaf *after* lstat. Its content
        # must never be read through an unverified path-following open.
        for scope in ("a", "b", "user"):
            ctx = self.context()
            root = ctx.home if scope == "user" else ctx.projects[scope]
            fixture = root / ".kiro/settings/mcp.json"
            foreign = ctx.root / ("foreign-" + scope + ".json")
            foreign.write_bytes(b"private foreign data - must never be adopted")
            original_open = os.open
            swapped = False

            def race(target, flags, *args, **kwargs):
                nonlocal swapped
                if not swapped and Path(target) == fixture and flags & os.O_NOFOLLOW:
                    fixture.rename(fixture.with_name("original-owned.json"))
                    fixture.symlink_to(foreign)
                    swapped = True
                return original_open(target, flags, *args, **kwargs)

            with self.subTest(scope=scope), patch.object(kiro.os, "open", side_effect=race):
                with self.assertRaisesRegex(ValueError, "authored fixture"):
                    kiro.collect(ctx)
            self.assertTrue(swapped)
            self.assertEqual(len(ctx.calls), 11)
            self.assertTrue(fixture.is_symlink())
            self.assertEqual(foreign.read_bytes(), b"private foreign data - must never be adopted")

    def test_in_place_fixture_rewrite_cannot_pass_with_original_bytes_and_mtime(self):
        # An untrusted native child could rewrite the authored file in place
        # and restore its contents/mtime. The inode and bytes alone are not
        # sufficient evidence that the read-only probe left it untouched.
        for scope in ("a", "b", "user"):
            ctx = self.context()
            root = ctx.home if scope == "user" else ctx.projects[scope]
            fixture = root / ".kiro/settings/mcp.json"

            def overwritten():
                before = fixture.stat()
                body = fixture.read_bytes()
                fixture.write_bytes(body)
                os.utime(fixture, ns=(before.st_atime_ns, before.st_mtime_ns))
                after = fixture.stat()
                self.assertEqual((before.st_dev, before.st_ino),
                                 (after.st_dev, after.st_ino))
                self.assertEqual(before.st_mtime_ns, after.st_mtime_ns)
                self.assertEqual(fixture.read_bytes(), body)
                return {"exit": 0, "error": None}, b"Opaque read-only listing.\\n", b""

            ctx.replace["a-workspace-raw-list"] = overwritten
            with self.subTest(scope=scope), self.assertRaisesRegex(
                    ValueError, "authored fixture identity"):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), 11)

    def test_fixture_clock_refuses_stalled_timestamp_without_leaving_files(self):
        with tempfile.TemporaryDirectory() as temporary:
            private = Path(temporary).resolve()
            fixture = private / "fixture"
            fixture.write_bytes(b"fixture")
            info = fixture.stat()
            owned = {fixture: (b"fixture", info.st_dev, info.st_ino,
                               info.st_ctime_ns + 10**12, ())}
            with patch.object(kiro.time, "monotonic", side_effect=(0, 3)):
                with self.assertRaisesRegex(ValueError, "clock did not advance"):
                    kiro._wait_for_fixture_clock(private, owned)
            self.assertEqual({item.name for item in private.iterdir()}, {"fixture"})

    def test_foreign_kiro_parent_cannot_be_adopted_or_partially_written(self):
        # Even when mcp.json itself is absent, an existing .kiro tree belongs
        # to something else. The last (User) conflict must not leave earlier
        # Project A/B research markers behind.
        for scope in ("a", "b", "user"):
            for kind in ("kiro-file", "foreign-directory", "settings-file"):
                ctx = self.context()
                selected = ctx.home if scope == "user" else ctx.projects[scope]
                marker = selected / ".kiro"
                if kind == "kiro-file":
                    sentinel = marker
                    sentinel.write_bytes(b"foreign marker file\\n")
                else:
                    marker.mkdir()
                    sentinel = marker / ("foreign.json" if kind == "foreign-directory" else "settings")
                    sentinel.write_bytes(b"foreign settings; do not modify\\n")
                before = sentinel.stat()
                with self.subTest(scope=scope, kind=kind), self.assertRaises(FileExistsError):
                    kiro.collect(ctx)
                self.assertEqual(ctx.calls, [])
                self.assertEqual(sentinel.read_bytes(),
                                 b"foreign marker file\\n" if kind == "kiro-file"
                                 else b"foreign settings; do not modify\\n")
                after = sentinel.stat()
                for field in ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid",
                              "st_nlink", "st_size", "st_mtime_ns", "st_ctime_ns"):
                    self.assertEqual(getattr(after, field), getattr(before, field))
                for other in ("a", "b", "user"):
                    if other != scope:
                        root = ctx.home if other == "user" else ctx.projects[other]
                        self.assertFalse((root / ".kiro").exists())

    def test_native_read_only_inventory_rejects_unapproved_fixture_members(self):
        # The selected CLI may read our inert marker, but must not silently
        # create other project/user configuration while being observed.
        for scope in ("a", "b", "user"):
            for kind in ("settings-file", "agent-directory"):
                ctx = self.context()
                root = ctx.home if scope == "user" else ctx.projects[scope]
                unexpected = (root / ".kiro/settings/extra.json" if kind == "settings-file"
                              else root / ".kiro/agents")

                def observed_list():
                    if kind == "settings-file":
                        unexpected.write_text("foreign sentinel\\n")
                    else:
                        unexpected.mkdir()
                    return {"exit": 0, "error": None}, b"opaque native list\\n", b""

                ctx.replace["a-workspace-raw-list"] = observed_list
                with self.subTest(scope=scope, kind=kind), self.assertRaisesRegex(
                        ValueError, "unowned fixture tree member"):
                    kiro.collect(ctx)
                self.assertEqual(ctx.calls[-1][0], "a-workspace-raw-list")
                self.assertTrue(unexpected.exists())
                # Detection is read-only: never remove or rewrite the unexpected member.
                if kind == "settings-file":
                    self.assertEqual(unexpected.read_text(), "foreign sentinel\\n")

    def test_root_alias_refuses_before_native_probe_or_fixture_write(self):
        # The research root is an authority boundary too: resolving it on
        # entry could silently adopt a foreign symlink target as disposable.
        for kind in ("root-symlink", "relative-root"):
            ctx = self.context()
            original = ctx.root
            if kind == "root-symlink":
                alias = original.parent / (original.name + "-alias")
                alias.symlink_to(original, target_is_directory=True)
                ctx.root = alias
            else:
                ctx.root = Path(original.name)
            with self.subTest(kind=kind), self.assertRaisesRegex(
                    ValueError, "canonical home and project directories"):
                kiro.collect(ctx)
            self.assertEqual(ctx.calls, [])
            for selected in (ctx.home, *ctx.projects.values()):
                self.assertFalse((selected / ".kiro").exists())
            self.assertTrue(original.is_dir())

    def test_alias_and_nested_project_roots_refuse_before_native_probes(self):
        for kind in ("project-symlink", "project-parent-symlink",
                     "home-symlink", "nested-project-a", "nested-project-b",
                     "relative-project"):
            ctx = self.context()
            if kind == "project-symlink":
                target = ctx.root / "a-original"
                ctx.projects["a"].rename(target)
                ctx.projects["a"].symlink_to(target, target_is_directory=True)
            elif kind == "project-parent-symlink":
                alias = ctx.root / "alias"
                alias.symlink_to(ctx.root, target_is_directory=True)
                ctx.projects["a"] = alias / "a"
            elif kind == "home-symlink":
                target = ctx.root / "home-original"
                ctx.home.rename(target)
                ctx.home.symlink_to(target, target_is_directory=True)
            elif kind == "nested-project-a":
                ctx.projects["a"] = ctx.projects["b"] / "nested"
                ctx.projects["a"].mkdir()
            elif kind == "nested-project-b":
                ctx.projects["b"] = ctx.projects["a"] / "nested"
                ctx.projects["b"].mkdir()
            else:
                ctx.projects["a"] = Path("relative-project")
            with self.subTest(kind=kind), self.assertRaisesRegex(
                    ValueError, "canonical home and project (roots|directories)|disjoint projects"):
                kiro.collect(ctx)
            self.assertEqual(ctx.calls, [])
            self.assertEqual(list(ctx.root.rglob(".kiro")), [])

    def test_absent_or_non_directory_roots_refuse_before_native_probes(self):
        # A typo or removed temporary project must never create a new
        # workspace, nor run a native child in an implicitly-created HOME.
        for kind in ("missing-home", "missing-project", "file-home", "file-project"):
            ctx = self.context()
            if kind.endswith("home"):
                target = ctx.home
            else:
                target = ctx.projects["b"]
            target.rmdir()
            if kind.startswith("file"):
                target.write_text("foreign sentinel; not a directory\\n")
            with self.subTest(kind=kind), self.assertRaisesRegex(
                    ValueError, "existing canonical home and project directories"):
                kiro.collect(ctx)
            self.assertEqual(ctx.calls, [])
            self.assertEqual(list(ctx.root.rglob(".kiro")), [])
            if kind.startswith("file"):
                self.assertTrue(target.read_text().startswith("foreign sentinel;"))

    def test_preexisting_native_config_symlink_fails_before_any_native_child(self):
        for scope in ("a", "b", "user"):
            ctx = self.context()
            selected = ctx.home if scope == "user" else ctx.projects[scope]
            marker = selected / ".kiro"
            marker.symlink_to("absent-foreign-native-profile", target_is_directory=True)
            with self.subTest(scope=scope), self.assertRaisesRegex(
                    FileExistsError, "preexisting native config"):
                kiro.collect(ctx)
            self.assertEqual(ctx.calls, [])
            self.assertTrue(marker.is_symlink())

    def test_original_user_home_cannot_be_project(self):
        ctx = self.context()
        ctx.projects["a"] = ctx.home
        with self.assertRaisesRegex(ValueError, "original user home"):
            kiro.collect(ctx)
        self.assertEqual(ctx.calls, [])


    def test_foreign_user_cwd_config_refuses_before_first_native_child(self):
        # This fourth root is selected for native listings but deliberately
        # has no UAP-owned marker. Never adopt even an empty foreign .kiro.
        for kind in ("directory", "dangling-symlink"):
            ctx = self.context()
            marker = ctx.projects["user"] / ".kiro"
            if kind == "directory":
                marker.mkdir()
                (marker / "foreign.txt").write_text("preserve foreign data")
            else:
                marker.symlink_to("nonexistent-foreign-target", target_is_directory=True)
            with self.subTest(kind=kind), self.assertRaisesRegex(
                    ValueError, "unowned native config"):
                kiro.collect(ctx)
            self.assertEqual(ctx.calls, [])
            if kind == "directory":
                self.assertEqual((marker / "foreign.txt").read_text(), "preserve foreign data")
            else:
                self.assertTrue(marker.is_symlink())

    def test_native_probe_cannot_replace_or_relax_disposable_roots(self):
        # Before fixture writes, no owned mcp.json exists. A vendor child
        # replacing an empty workspace must be detected before UAP adopts it.
        for label, call_count in (("version", 1), ("a-agent-raw-list", 10),
                                  ("a-workspace-raw-list", 11)):
            for scope in ("a", "b", "user", "home"):
                for mutation in ("replacement", "mode"):
                    ctx = self.context()
                    selected = ctx.home if scope == "home" else ctx.projects[scope]

                    def changed():
                        if mutation == "replacement":
                            selected.rename(ctx.root / ("relocated-" + scope))
                            selected.mkdir()
                        else:
                            selected.chmod(stat.S_IMODE(selected.stat().st_mode) ^ stat.S_IWGRP)
                        body = (b"kiro-cli 2.34.0\\n" if label == "version"
                                else b"Opaque read-only listing.\\n")
                        return {"exit": 0, "error": None}, body, b""

                    ctx.replace[label] = changed
                    with self.subTest(label=label, scope=scope, mutation=mutation):
                        with self.assertRaisesRegex(ValueError, "disposable root identity/mode"):
                            kiro.collect(ctx)
                    self.assertEqual(len(ctx.calls), call_count)
                    self.assertEqual(ctx.calls[-1][0], label)
                    if mutation == "replacement":
                        self.assertTrue((ctx.root / ("relocated-" + scope)).is_dir())


    def test_native_commands_cannot_write_to_unowned_user_cwd(self):
        # A successful or authentication-refused read-only command must never
        # author native state in the separate "user" selected project cwd.
        for label, call_count in (("version", 1), ("a-agent-raw-list", 10),
                                  ("user-global-raw-list", 16)):
            ctx = self.context()
            marker = ctx.projects["user"] / ".kiro"

            def changed():
                marker.mkdir()
                (marker / "native.json").write_text("unexpected native data")
                body = (b"kiro-cli 2.34.0\\n" if label == "version"
                        else b"Opaque read-only listing.\\n")
                return {"exit": 0, "error": None}, body, b""

            ctx.replace[label] = changed
            with self.subTest(label=label), self.assertRaisesRegex(
                    ValueError, "unowned native config"):
                kiro.collect(ctx)
            self.assertEqual(len(ctx.calls), call_count)
            self.assertEqual(ctx.calls[-1][0], label)
            self.assertEqual((marker / "native.json").read_text(), "unexpected native data")

    def test_native_command_cannot_replace_selected_user_cwd_with_alias(self):
        ctx = self.context()

        def changed():
            original = ctx.projects["user"]
            relocated = ctx.root / "relocated-user"
            original.rename(relocated)
            original.symlink_to(relocated, target_is_directory=True)
            return {"exit": 0, "error": None}, b"kiro-cli 2.34.0\\n", b""

        ctx.replace["version"] = changed
        with self.assertRaisesRegex(ValueError, "disposable root alias refused"):
            kiro.collect(ctx)
        self.assertEqual(len(ctx.calls), 1)
        self.assertTrue(ctx.projects["user"].is_symlink())


if __name__ == "__main__":
    unittest.main()
