"""Inert no-auth route/effect/evidence contracts, never vendor observations."""
import os
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import project_native_kiro_no_auth as noauth
import project_native_research as research

STATUS_HELP = b"Get status\n\nUsage: kiro-cli mcp status [OPTIONS] --name <NAME>\n\nOptions:\n  --name <NAME>  Owned server name\n"


class NoAuthContracts(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.summary = {"commands": []}
        self.ctx = research.Context(self.root, self.summary, None)
        self.ctx.env["PATH"] = str(self.root / "client/kirocli/bin") + ":/usr/bin:/bin"
        self.owned, self.fixtures = noauth.prepare(self.ctx)
        self.table = noauth.routes(self.fixtures, self.ctx)
        self.ids = {root: noauth.kiro._research_root_identity(root) for root in self.owned}

    def invoke(self, label="validate-valid", arguments=None, project="a"):
        if arguments is None:
            arguments = self.table[label][1]
        with patch.object(noauth.kiro, "_runtime_bin"), patch.object(self.ctx, "command", return_value=({"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL)) as child:
            result = noauth.probe(self.ctx, self.table, self.owned, self.ids, label, arguments, project)
            return result, child.call_args

    def test_closed_routes_contain_only_finite_non_session_commands(self):
        self.assertEqual(len(self.table), 18)
        self.assertFalse(any(set(args) & {"login", "chat", "acp", "create", "update", "restart"}
                             for _, args in self.table.values()))
        self.assertEqual(self.table["validate-missing"][1][-1], str(self.fixtures / "missing.json"))
        self.assertFalse((self.fixtures / "missing.json").exists())

    def test_probe_passes_only_explicit_private_profile(self):
        (_, stdout, stderr), call = self.invoke()
        self.assertEqual(stdout, b"")
        self.assertEqual(stderr, noauth.AUTH_REFUSAL)
        self.assertEqual(call.kwargs, {"timeout": 20})
        self.assertEqual(call.args[2], self.ctx.projects["a"])
        env = call.args[3]
        profile = self.ctx.private / "no-auth-validate-valid"
        self.assertEqual(env["KIRO_HOME"], str(profile))
        self.assertEqual(env["HOME"], str(self.ctx.home))
        for key, name in (("XDG_CONFIG_HOME", "config"), ("XDG_CACHE_HOME", "cache"),
                          ("XDG_STATE_HOME", "state"), ("XDG_DATA_HOME", "data")):
            self.assertEqual(env[key], str(profile / name))
            self.assertEqual(self.ids[profile / name], noauth.kiro._research_root_identity(profile / name))

    def test_wrong_argv_scope_and_session_refuse_before_child(self):
        for args, project in ((["agent", "list"], "a"), (self.table["validate-valid"][1], "b"),
                              (["chat", "--no-interactive"], "a"), (["agent", "validate", "--path", "/foreign"], "a")):
            with self.subTest(args=args, project=project), patch.object(self.ctx, "command") as child:
                with self.assertRaisesRegex(ValueError, "exact argv/cwd"):
                    noauth.probe(self.ctx, self.table, self.owned, self.ids, "validate-valid", args, project)
                child.assert_not_called()

    def test_added_credential_loader_or_changed_home_refuses(self):
        for key, value in (("KIRO_API_KEY", "inert-contract-value"), ("AWS_PROFILE", "inert"),
                           ("LD_PRELOAD", "/inert"), ("HOME", "/foreign"), ("PATH", "/foreign")):
            with self.subTest(key=key), patch.dict(self.ctx.env, {key: value}), patch.object(self.ctx, "command") as child:
                with self.assertRaisesRegex(ValueError, "closed environment"):
                    noauth.probe(self.ctx, self.table, self.owned, self.ids, "validate-valid", self.table["validate-valid"][1], "a")
                child.assert_not_called()

    def test_registered_secret_refuses_even_with_closed_environment(self):
        self.ctx.secrets.append(b"inert-contract-only")
        with self.assertRaisesRegex(ValueError, "closed environment"):
            self.invoke()

    def test_foreign_bytes_new_files_and_modes_refuse(self):
        marker = self.ctx.home / ".kiro/sentinel.txt"
        marker.write_text("modified\n")
        with self.assertRaisesRegex(ValueError, "changed before probe"):
            self.invoke()
        marker.write_text("foreign owned-test sentinel; preserve bytes\n")
        # Restoring bytes cannot erase the held inode/time identity change.
        with self.assertRaisesRegex(ValueError, "changed before probe"):
            self.invoke()

    def test_native_mutation_and_exception_still_run_post_guard(self):
        def mutate(*args, **kwargs):
            (self.ctx.projects["b"] / "new-unowned").write_text("bad")
            raise TimeoutError("inert timeout")
        with patch.object(noauth.kiro, "_runtime_bin"), patch.object(self.ctx, "command", side_effect=mutate):
            with self.assertRaisesRegex(ValueError, "changed owned/foreign"):
                noauth.probe(self.ctx, self.table, self.owned, self.ids, "validate-valid", self.table["validate-valid"][1], "a")

    def test_mode_root_replacement_and_new_file_refuse(self):
        marker = self.ctx.projects["b"] / ".kiro"
        marker.chmod(0o500)
        with self.assertRaisesRegex(ValueError, "changed before probe"):
            self.invoke()
        marker.chmod(0o700)
        added = self.ctx.projects["user"] / "unexpected"
        added.write_text("new file")
        with self.assertRaisesRegex(ValueError, "changed before probe"):
            self.invoke()
        added.unlink()
        root = self.ctx.projects["a"]
        root.rename(root.with_name("replaced-a"))
        root.mkdir(mode=0o700)
        with self.assertRaises(ValueError):
            self.invoke()

    def test_preexisting_native_config_and_overlapping_roots_refuse(self):
        with self.assertRaisesRegex(ValueError, "preexisting"):
            noauth.prepare(self.ctx)
        self.ctx.projects["b"] = self.ctx.projects["a"]
        with self.assertRaisesRegex(ValueError, "disjoint"):
            noauth.prepare(self.ctx)

    def test_private_alias_or_wrong_mode_refuses_before_profile_creation(self):
        private = self.ctx.private
        moved = private.with_name("private-held")
        private.rename(moved)
        private.symlink_to(moved, target_is_directory=True)
        with patch.object(self.ctx, "command") as child:
            with self.assertRaises(ValueError):
                self.invoke()
            child.assert_not_called()
        private.unlink()
        moved.rename(private)
        private.chmod(0o755)
        with self.assertRaisesRegex(ValueError, "ownership/mode"):
            self.invoke()
        self.assertFalse((private / "no-auth-validate-valid").exists())

    def test_native_private_and_profile_replacement_fail_post_guard(self):
        for target_name in ("profile", "private", "xdg"):
            with self.subTest(target=target_name), tempfile.TemporaryDirectory() as other:
                ctx = research.Context(Path(other).resolve(), {"commands": []}, None)
                ctx.env["PATH"] = str(Path(other) / "client/kirocli/bin") + ":/usr/bin:/bin"
                owned, fixtures = noauth.prepare(ctx)
                ids = {root: noauth.kiro._research_root_identity(root) for root in owned}
                def replace(*args, **kwargs):
                    target = ctx.private if target_name == "private" else ctx.private / "no-auth-validate-valid"
                    if target_name == "xdg":
                        target = target / "data"
                    target.rename(target.with_name(target.name + "-held"))
                    target.mkdir(mode=0o700)
                    return {"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL
                with patch.object(noauth.kiro, "_runtime_bin"), patch.object(ctx, "command", side_effect=replace):
                    with self.assertRaisesRegex(ValueError, "identity/mode"):
                        noauth.probe(ctx, noauth.routes(fixtures, ctx), owned, ids, "validate-valid", noauth.routes(fixtures, ctx)["validate-valid"][1], "a")

    def test_private_database_initialization_allowed_original_home_preserved(self):
        def initialize(label, argv, cwd, env, timeout):
            database = Path(env["XDG_DATA_HOME"]) / "kiro-cli/data.sqlite3"
            database.parent.mkdir(mode=0o700)
            database.write_bytes(b"inert database fixture; not a real session")
            return {"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL
        with patch.object(noauth.kiro, "_runtime_bin"), patch.object(self.ctx, "command", side_effect=initialize):
            result = noauth.probe(self.ctx, self.table, self.owned, self.ids, "validate-valid", self.table["validate-valid"][1], "a")
        self.assertTrue(noauth.exact_refusal(*result))
        self.assertFalse((self.ctx.home / "data/kiro-cli").exists())
        route = self.ctx.summary["private_profile_routes"][0]
        self.assertEqual(route["xdg"]["XDG_DATA_HOME"], str(self.ctx.private / "no-auth-validate-valid/data"))
        for root, expected in self.owned.items():
            self.assertEqual(noauth.tree_identity(root), expected)
    def test_owned_regular_open_is_nofollow_and_nonblocking(self):
        # A regular file can be exchanged for a FIFO after lstat; open must not hang.
        target = self.fixtures / "valid.json"
        actual_open = os.open
        seen = []

        def guard_open(path, flags, *args, **kwargs):
            if Path(path) == target:
                self.assertTrue(flags & os.O_NOFOLLOW)
                self.assertTrue(flags & os.O_NONBLOCK)
                seen.append(flags)
            return actual_open(path, flags, *args, **kwargs)

        with patch.object(noauth.os, "open", side_effect=guard_open):
            identity, digest = noauth.file_identity(target)
        self.assertTrue(seen)
        self.assertIsNotNone(identity)
        self.assertEqual(len(digest), 64)

    def test_symlink_hardlink_and_special_inventory_refuse(self):
        for kind in ("symlink", "hardlink", "fifo"):
            with self.subTest(kind=kind):
                target = self.root / ("unsafe-" + kind)
                if kind == "symlink":
                    target.symlink_to(self.fixtures / "valid.json")
                elif kind == "hardlink":
                    os.link(self.fixtures / "valid.json", target)
                else:
                    os.mkfifo(target)
                with self.assertRaises(ValueError):
                    noauth.file_identity(target)
                target.unlink()

    def test_refusal_requires_exact_types_bytes_exit_and_null_error(self):
        self.assertTrue(noauth.exact_refusal({"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL))
        for record, out, err in (({"exit": True, "error": None}, b"", noauth.AUTH_REFUSAL),
                                 ({"exit": 1}, b"", noauth.AUTH_REFUSAL),
                                 ({"exit": 1, "error": "timeout"}, b"", noauth.AUTH_REFUSAL),
                                 ({"exit": 0, "error": None}, b"", noauth.AUTH_REFUSAL),
                                 ({"exit": 1, "error": None}, b"inventory", noauth.AUTH_REFUSAL),
                                 ({"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL + b"warning")):
            with self.subTest(record=record, out=out, err=err):
                self.assertFalse(noauth.exact_refusal(record, out, err))

    def test_complete_boundary_keeps_validation_and_discovery_unobserved(self):
        # Metadata/streams below are inert test inputs only, not original evidence.
        def child(label, *args, **kwargs):
            if label == "version":
                return {"exit": 0, "error": None}, b"kiro-cli 2.34.0\n", b""
            if label == "validate-help":
                return {"exit": 0, "error": None}, noauth.VALIDATE_HELP, b""
            if label == "mcp-status-help":
                return {"exit": 0, "error": None}, STATUS_HELP, b""
            return {"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL
        # collect authors its own fresh fixture; our setUp fixtures are separate.
        with tempfile.TemporaryDirectory() as other:
            ctx = research.Context(Path(other).resolve(), {"commands": []}, None)
            with patch.object(noauth.kiro, "_runtime_bin", return_value=Path(other) / "client/kirocli/bin"), patch.object(ctx, "command", side_effect=child):
                noauth.collect(ctx)
            self.assertTrue(ctx.summary["finite_capture_complete"])
            self.assertEqual(len(ctx.summary["expected_auth_refusals"]), 9)
            self.assertEqual(len(ctx.summary["validation_observations"]), 3)
            self.assertTrue(all(not x["validation_qualified"] for x in ctx.summary["validation_observations"]))
            self.assertFalse(ctx.summary["validation_schema_observed"])
            self.assertFalse(ctx.summary["native_discovery_observed"])
            self.assertEqual(ctx.summary["expected_auth_refusal_count"], 15)
            self.assertEqual(len(ctx.summary["status_observations"]), 3)
            self.assertFalse(ctx.summary["status_observation_qualified"])
            self.assertTrue(all(row["native_status_qualified"] is False for row in ctx.summary["status_observations"]))

    def test_success_requires_exact_measured_help_and_typed_exit(self):
        self.assertEqual(len(noauth.VALIDATE_HELP), 206)
        self.assertTrue(noauth.exact_success({"exit": 0, "error": None}, noauth.VALIDATE_HELP, b"", noauth.VALIDATE_HELP))
        for record, out, err in (({"exit": False, "error": None}, noauth.VALIDATE_HELP, b""),
                                 ({"exit": 0}, noauth.VALIDATE_HELP, b""),
                                 ({"exit": 0, "error": None}, noauth.VALIDATE_HELP + b"warning", b""),
                                 ({"exit": 0, "error": None}, noauth.VALIDATE_HELP, b"warning")):
            self.assertFalse(noauth.exact_success(record, out, err, noauth.VALIDATE_HELP))

    def test_status_routes_select_authored_sources_and_preserve_other_roots(self):
        for project in ("a", "b", "user"):
            selected = self.ctx.home if project == "user" else self.ctx.projects[project]
            name = "uap-noauth-" + selected.name
            self.assertEqual(self.table[project + "-mcp-status"], (project, ("mcp", "status", "--name", name)))
            source = json.loads((selected / ".kiro/settings/mcp.json").read_text())
            self.assertIs(source["mcpServers"][name]["disabled"], True)
            _, call = self.invoke(label=project + "-mcp-status", project=project)
            self.assertEqual(call.args[1], ["mcp", "status", "--name", name])
            self.assertEqual(call.args[2], self.ctx.projects[project])
            self.assertEqual(call.args[3]["HOME"], str(self.ctx.home))
            for root, expected in self.owned.items():
                self.assertEqual(noauth.tree_identity(root), expected)
        with patch.object(self.ctx, "command") as child:
            with self.assertRaisesRegex(ValueError, "exact argv/cwd"):
                noauth.probe(self.ctx, self.table, self.owned, self.ids, "a-mcp-status",
                             self.table["b-mcp-status"][1], "a")
            child.assert_not_called()

    def test_status_help_requires_pinned_child_syntax_and_typed_success(self):
        self.assertTrue(noauth.status_help_supported({"exit": 0, "error": None}, STATUS_HELP, b""))
        invalid = (({"exit": False, "error": None}, STATUS_HELP, b""),
                   ({"exit": 0}, STATUS_HELP, b""),
                   ({"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL),
                   ({"exit": 0, "error": None}, STATUS_HELP, b"warning"))
        bodies = (STATUS_HELP.replace(b"mcp status", b"mcp status-other"),
                  STATUS_HELP.replace(b"--name", b"--names"), STATUS_HELP + STATUS_HELP,
                  b"x" * 4097, b"\xff", STATUS_HELP + b"\x00")
        for record, out, err in (*invalid, *(({"exit": 0, "error": None}, body, b"") for body in bodies)):
            self.assertFalse(noauth.status_help_supported(record, out, err))

    def test_status_help_or_unknown_status_stops_without_other_status_calls(self):
        for mode in ("help", "success", "changed-denial", "timeout"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as other:
                ctx = research.Context(Path(other).resolve(), {"commands": []}, None)
                calls = []
                def child(label, *args, **kwargs):
                    calls.append(label)
                    if label == "version": return {"exit": 0, "error": None}, b"kiro-cli 2.34.0\n", b""
                    if label == "validate-help": return {"exit": 0, "error": None}, noauth.VALIDATE_HELP, b""
                    if label == "mcp-status-help":
                        return {"exit": 0, "error": None}, b"wrong grammar" if mode == "help" else STATUS_HELP, b""
                    if label == "a-mcp-status":
                        if mode == "success": return {"exit": 0, "error": None}, b"disabled", b""
                        if mode == "timeout": return {"exit": None, "error": "timeout"}, b"", b""
                        return {"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL + b"warning"
                    return {"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL
                with patch.object(noauth.kiro, "_runtime_bin", return_value=Path(other) / "client/kirocli/bin"), patch.object(ctx, "command", side_effect=child):
                    with self.assertRaisesRegex(ValueError, "unreviewed.*status"):
                        noauth.collect(ctx)
                self.assertEqual(len(ctx.summary["expected_auth_refusals"]), 9)
                self.assertEqual(ctx.summary["status_observations"], [])
                self.assertIs(ctx.summary["status_observation_qualified"], False)
                self.assertFalse(any(label in calls for label in ("b-mcp-status", "user-mcp-status")))
                if mode == "help": self.assertNotIn("a-mcp-status", calls)

    def test_status_auth_denial_cannot_hide_foreign_mutation(self):
        def mutate(*args, **kwargs):
            (self.ctx.projects["b"] / ".kiro/sentinel.txt").write_text("unexpected effect")
            return {"exit": 1, "error": None}, b"", noauth.AUTH_REFUSAL
        with patch.object(noauth.kiro, "_runtime_bin"), patch.object(self.ctx, "command", side_effect=mutate):
            with self.assertRaisesRegex(ValueError, "changed owned/foreign"):
                noauth.probe(self.ctx, self.table, self.owned, self.ids, "a-mcp-status", self.table["a-mcp-status"][1], "a")

    def test_unexpected_validation_success_fails_before_any_inventory(self):
        calls = []
        def child(label, *args, **kwargs):
            calls.append(label)
            out = b"kiro-cli 2.34.0\n" if label == "version" else noauth.VALIDATE_HELP
            if label == "validate-valid":
                out = b"configuration valid\n"
            return {"exit": 0, "error": None}, out, b""
        with tempfile.TemporaryDirectory() as other:
            ctx = research.Context(Path(other).resolve(), {"commands": []}, None)
            with patch.object(noauth.kiro, "_runtime_bin", return_value=Path(other) / "client/kirocli/bin"), patch.object(ctx, "command", side_effect=child):
                with self.assertRaisesRegex(ValueError, "unreviewed no-auth validation"):
                    noauth.collect(ctx)
        self.assertEqual(calls, ["version", "validate-help", "validate-valid"])


if __name__ == "__main__":
    unittest.main()
