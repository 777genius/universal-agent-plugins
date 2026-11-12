"""Inert decoder/authority contracts; no vendor client is mocked as a PASS."""
import json
import base64
import hashlib
import os
import sys
import types
from unittest.mock import patch
import client_automation_native_probe as probe
from pathlib import Path
import tempfile
import unittest
from client_automation_native_probe import CLIENTS, NAME, exact_plugin_id, put, snapshot


class NativeAdmissionContracts(unittest.TestCase):
    def test_exact_project_full_id(self):
        path = Path("/private/project/.grok/plugins/probe")
        row = {"name": NAME, "scope": "project", "path": str(path), "id": "project/abcdef12/" + NAME}
        for value in ([row], {"plugins": [row]}):
            self.assertEqual(exact_plugin_id(json.dumps(value), path), row["id"])

    def test_name_only_user_foreign_and_duplicate_refused(self):
        path = Path("/private/project/.grok/plugins/probe")
        row = {"name": NAME, "scope": "project", "path": str(path), "id": "project/abcdef12/" + NAME}
        for value in ([dict(row, id=NAME)], [dict(row, scope="user")], [dict(row, path="/foreign")],
                      [row, row], [dict(row, id="user/abcdef12/" + NAME)],
                      [dict(row, id="project/../" + NAME)], [], {}, None):
            with self.subTest(value=value), self.assertRaises(ValueError):
                exact_plugin_id(json.dumps(value), path)

    def test_duplicate_json_keys_refused(self):
        with self.assertRaisesRegex(ValueError, "duplicate"):
            exact_plugin_id('{"plugins":[],"plugins":[]}', Path("/private"))

    def test_fixture_does_not_overwrite_or_follow_alias(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            put(root / "owned", "sentinel")
            with self.assertRaises(FileExistsError):
                put(root / "owned", "replace")
            (root / "alias").symlink_to(root / "owned")
            with self.assertRaisesRegex(ValueError, "alias"):
                snapshot(root)
            self.assertEqual((root / "owned").read_text(), "sentinel")

    @unittest.skipUnless(os.name == "posix" and hasattr(os, "O_NOFOLLOW")
                         and hasattr(os, "O_DIRECTORY"), "requires POSIX no-follow dirfd")
    def test_snapshot_refuses_leaf_alias_swap_before_open(self):
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            root = parent / "owned"
            root.mkdir()
            leaf = root / "config"
            leaf.write_text("safe")
            foreign = parent / "foreign-secret"
            foreign.write_text("do-not-capture")
            original = os.open

            def swap(path, flags, *args, **kwargs):
                if path == "config" and kwargs.get("dir_fd") is not None:
                    leaf.unlink()
                    leaf.symlink_to(foreign)
                return original(path, flags, *args, **kwargs)

            with patch.object(os, "open", side_effect=swap):
                with self.assertRaises(OSError):
                    snapshot(root)
            self.assertEqual(foreign.read_text(), "do-not-capture")

    @unittest.skipUnless(os.name == "posix" and hasattr(os, "O_NOFOLLOW")
                         and hasattr(os, "O_DIRECTORY"), "requires POSIX no-follow dirfd")
    def test_snapshot_refuses_ancestor_alias_and_fifo_swap(self):
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            root = parent / "owned"
            root.mkdir()
            child = root / "sub"
            child.mkdir()
            (child / "config").write_text("safe")
            foreign = parent / "foreign"
            foreign.mkdir()
            (foreign / "config").write_text("do-not-capture")
            original = os.open

            def swap_ancestor(path, flags, *args, **kwargs):
                if path == "sub" and kwargs.get("dir_fd") is not None:
                    (child / "config").unlink()
                    child.rmdir()
                    child.symlink_to(foreign, target_is_directory=True)
                return original(path, flags, *args, **kwargs)

            with patch.object(os, "open", side_effect=swap_ancestor):
                with self.assertRaises(OSError):
                    snapshot(root)
            child.unlink()
            child.mkdir()
            leaf = child / "config"
            leaf.write_text("safe")

            def swap_fifo(path, flags, *args, **kwargs):
                if path == "config" and kwargs.get("dir_fd") is not None:
                    leaf.unlink()
                    os.mkfifo(leaf)
                return original(path, flags, *args, **kwargs)

            with patch.object(os, "open", side_effect=swap_fifo):
                with self.assertRaisesRegex(ValueError, "config capture bound"):
                    snapshot(root)

    @unittest.skipUnless(os.name == "posix" and hasattr(os, "O_NOFOLLOW")
                         and hasattr(os, "O_DIRECTORY"), "native snapshot is POSIX-only")
    def test_only_digest_and_modes_preserved(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            put(root / "config", {"name": NAME})
            item = snapshot(root)["config"]
            self.assertEqual(item["mode"], 0o600)
            self.assertEqual(item["bytes"], (root / "config").stat().st_size)
            self.assertEqual(item["sha256"], hashlib.sha256((root / "config").read_bytes()).hexdigest())
            self.assertEqual(set(item), {"sha256", "mode", "bytes"})

    @unittest.skipUnless(os.name == "posix" and hasattr(os, "O_NOFOLLOW")
                         and hasattr(os, "O_DIRECTORY"), "requires POSIX no-follow dirfd")
    def test_snapshot_refuses_hardlinked_external_file_without_serializing_it(self):
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            root = parent / "owned"
            root.mkdir()
            (root / "inert").write_bytes(b"owned")
            foreign = parent / "foreign-secret"
            foreign.write_bytes(b"do-not-capture")
            os.link(foreign, root / "external-link")
            with self.assertRaisesRegex(ValueError, "linked file"):
                snapshot(root)
            self.assertEqual(foreign.read_bytes(), b"do-not-capture")

    def test_excluded_clients_are_not_admitted(self):
        for target in ("vscode", "chatgptweb", "chatgpt", "codex", "kimi"):
            self.assertNotIn(target, CLIENTS)


class GrokProjectInspectContracts(unittest.TestCase):
    def setUp(self):
        self.project = Path("/private/project-a")
        self.path = self.project / ".grok/plugins/project-probe"
        self.plugin = {"name": NAME, "scope": "project", "path": str(self.path),
                       "enabled": False, "provides": {"skills": 1}}
        self.facts = {"grokVersion": "1.0.70", "cwd": str(self.project),
                      "projectRoot": str(self.project) + "/", "projectTrusted": False,
                      "plugins": [self.plugin]}

    def inspect(self, value):
        return probe.grok_readonly_project_inventory(json.dumps(value), self.project, self.path)

    def test_native_readonly_exact_project_is_not_enablement(self):
        self.assertEqual(self.inspect(self.facts), {"name": NAME, "scope": "project",
                            "path": str(self.path), "enabled": False})

    def test_unknown_project_or_trust_refuses(self):
        for field, value in (("cwd", "/private/project-b"),
                             ("projectRoot", "/private/project-b/"),
                             ("projectTrusted", True), ("grokVersion", "1.0.71")):
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.inspect(dict(self.facts, **{field: value}))

    def test_foreign_duplicate_or_enabled_project_refuses(self):
        for rows in ([dict(self.plugin, path="/private/foreign")],
                     [dict(self.plugin, scope="user")],
                     [dict(self.plugin, enabled=True)],
                     [self.plugin, self.plugin],
                     [self.plugin, dict(self.plugin, name=NAME.upper())],
                     [None], [], [{}]):
            with self.subTest(rows=rows), self.assertRaises(ValueError):
                self.inspect(dict(self.facts, plugins=rows))

    def test_nonobject_or_duplicate_native_fields_refuse(self):
        with self.assertRaises(ValueError):
            probe.grok_readonly_project_inventory("null", self.project, self.path)
        with self.assertRaisesRegex(ValueError, "duplicate"):
            probe.grok_readonly_project_inventory('{"plugins":[],"plugins":[]}',
                                                   self.project, self.path)


class ProcessResultContracts(unittest.TestCase):
    def setUp(self):
        self.probe = object.__new__(probe.Probe)
        self.probe.root = Path("/fixture")
        self.probe.a = self.probe.root / "project-a"
        self.probe.env = {"HOME": "/fixture/home"}
        self.probe.evidence = {"commands": []}

    def invoke(self, record, stdout=b"diagnostic", stderr=b"error"):
        runner = types.SimpleNamespace(run_probe=lambda *args, **kwargs: (record, stdout, stderr))
        with patch.dict(sys.modules, {"native_client_research": runner}):
            return self.probe.run("observed", ["/fixture/bin/cline", "config", "mcp", "--json"])

    def test_nonzero_native_exit_is_not_successful_capture(self):
        # Original 43220283215: all three cline mcp list calls exited 1.
        for code in (1, 2, -9):
            with self.subTest(code=code), self.assertRaisesRegex(ValueError, "native command failed"):
                self.invoke({"probe": "mcp-default", "error": None, "exit": code})
        self.assertEqual(len(self.probe.evidence["commands"]), 3)

    def test_noninteger_and_missing_exit_are_refused(self):
        for code in (False, 0.0, "0", None):
            with self.subTest(code=code), self.assertRaises(ValueError):
                self.invoke({"probe": "observed", "error": None, "exit": code})

    def test_capture_error_preserves_original_bytes(self):
        with self.assertRaisesRegex(ValueError, "capture failed"):
            self.invoke({"probe": "observed", "error": "timeout", "exit": 0}, b"partial", b"timed out")
        row = self.probe.evidence["commands"][0]
        self.assertEqual(base64.b64decode(row["stdout_base64"]), b"partial")
        self.assertEqual(base64.b64decode(row["stderr_base64"]), b"timed out")

    def test_missing_capture_error_field_is_refused(self):
        with self.assertRaises(ValueError):
            self.invoke({"exit": 0})

    def test_zero_exit_and_captured_stdout_are_preserved(self):
        self.assertEqual(self.invoke({"error": None, "exit": 0}, b"[]", b""), (0, b"[]"))

    def test_override_does_not_mutate_base_environment(self):
        captured = []
        def run(*args, **kwargs):
            captured.append(args[2])
            return {"error": None, "exit": 0}, b"[]", b""
        with patch.dict(sys.modules, {"native_client_research": types.SimpleNamespace(run_probe=run)}):
            self.probe.run("selected", ["/fixture/bin/cline"], overrides={"CLINE_MCP_SETTINGS_PATH": "/selected"})
        self.assertNotIn("CLINE_MCP_SETTINGS_PATH", self.probe.env)
        self.assertEqual(captured[0]["CLINE_MCP_SETTINGS_PATH"], "/selected")


class InventoryContracts(unittest.TestCase):
    def setUp(self):
        self.path = Path("/private/project/.uap/clients/cline/mcp.json")
        self.row = {"name": "project-sentinel", "path": str(self.path),
                    "transportType": "stdio", "disabled": True}

    def test_source_path_name_transport_and_disabled_are_required(self):
        self.assertEqual(probe.cline_inventory(json.dumps([self.row]), self.path, "project-sentinel"), self.row)
        for key, bad in (("name", "user-sentinel"), ("path", "/foreign"),
                         ("transportType", "http"), ("disabled", False), ("disabled", 1)):
            with self.subTest(key=key, bad=bad), self.assertRaises(ValueError):
                probe.cline_inventory(json.dumps([dict(self.row, **{key: bad})]), self.path, "project-sentinel")

    def test_empty_duplicate_mixed_and_nonobject_entries_refused(self):
        for value in ([], [self.row, self.row], [self.row, dict(self.row, name="user-sentinel")],
                      {"mcpServers": {}}, [None], None, "help"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                probe.cline_inventory(json.dumps(value), self.path, "project-sentinel")

    def test_exit_zero_help_is_not_inventory(self):
        with self.assertRaises(ValueError):
            probe.cline_inventory("Usage: cline mcp [options] [command]", self.path, "project-sentinel")

    def test_duplicate_nonfinite_and_oversized_json_refused(self):
        for value in ('{"x":1,"x":2}', '[NaN]', '[Infinity]', ' ' * ((1 << 20) + 1), ''):
            with self.subTest(value=value[:30]), self.assertRaises(ValueError):
                probe.native_json(value)

    def test_exact_pinned_versions_only(self):
        probe.check_version("cline", b"3.0.73\n")
        probe.check_version("grok", b"grok 1.0.70 (f50904eaee6c)\n")
        for client, body in (("cline", b"3.0.74\n"), ("grok", b"grok 1.0.70 (foreign)\n"),
                             ("cline", b"3.0.73\nextra"), ("vscode", b"3.0.73")):
            with self.subTest(client=client, body=body), self.assertRaises(ValueError):
                probe.check_version(client, body)

    def test_collection_without_native_admission_is_rejected(self):
        # Original Grok result is explicitly admitted:false; do not promote it.
        evidence = {"research_completed": True, "qualified_product_e2e": False}
        for flag in (None, False, 1, "true"):
            with self.subTest(flag=flag), self.assertRaises(ValueError):
                probe.require_admission(dict(evidence, native_api_admitted=flag))
        probe.require_admission(dict(evidence, native_api_admitted=True))

    def test_research_must_not_claim_product_acceptance(self):
        with self.assertRaises(ValueError):
            probe.require_admission({"research_completed": True, "native_api_admitted": True,
                                     "qualified_product_e2e": True})

    def test_empty_and_idless_grok_native_inventories_refused(self):
        path = Path("/private/project/.grok/plugins/project-probe")
        # Shapes retained in artifact 13944589293: list=[], inspect has no ID.
        for body in ('[]', json.dumps({"plugins": [{"name": NAME, "scope": "project",
                                                    "path": str(path), "enabled": False}]})):
            with self.subTest(body=body), self.assertRaises(ValueError):
                exact_plugin_id(body, path)

    def test_snapshot_missing_root_or_root_alias_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            (root / "alias").symlink_to(root, target_is_directory=True)
            for candidate in (root / "alias", root / "missing"):
                with self.subTest(candidate=candidate), self.assertRaises(ValueError):
                    snapshot(candidate)


class ClineRouteContracts(unittest.TestCase):
    """Inert routing tests only; the responder is not a native Cline binary."""
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name).resolve()

    def fixture(self, mode="ok"):
        class Inert(probe.Probe):
            def run(inner, label, args, cwd=None, overrides=None):
                inner.calls.append((label, list(args), cwd, dict(overrides or {})))
                if label == "version":
                    return 0, b"3.0.73\n"
                if "--help" in args:
                    return 0, b"Usage: cline [options] [command]"
                user = inner.home / ".cline/data/settings/cline_mcp_settings.json"
                selected = Path((overrides or {}).get("CLINE_MCP_SETTINGS_PATH", str(user)))
                if mode == "misroute" and overrides:
                    selected = user
                if mode == "write":
                    user.write_text('{"mcpServers":{}}')
                if mode == "write-then-fail":
                    (inner.b / "unexpected").write_text("changed")
                    raise ValueError("native command failed")
                if mode == "help":
                    return 0, b"Usage: cline mcp [options] [command]"
                rows = [{"name": name, "path": str(selected), "transportType": "stdio",
                         "disabled": config.get("disabled") is True}
                        for name, config in json.loads(selected.read_text())["mcpServers"].items()]
                return 0, json.dumps(rows).encode()
        item = object.__new__(Inert)
        item.root, item.binary = self.root, self.root / "bin/cline"
        item.home, item.a, item.b = [self.root / name for name in ("home", "a", "b")]
        for path in (item.home, item.a, item.b):
            path.mkdir()
        item.calls = []
        item.evidence = {"commands": [], "native_api_admitted": False, "qualified_product_e2e": False}
        return item

    def test_select_restore_and_project_b_observed_without_mcp_wizard(self):
        item = self.fixture()
        item.cline()
        inventories = [call for call in item.calls if call[0].startswith("mcp-")]
        self.assertEqual([call[0] for call in inventories],
                         ["mcp-default", "mcp-selected-a", "mcp-default-after-a", "mcp-default-b"])
        for _, argv, _, _ in inventories:
            self.assertEqual(argv[1:], ["--auto-approve", "false", "config", "mcp", "--json"])
        self.assertEqual([bool(call[3]) for call in inventories], [False, True, False, False])
        self.assertEqual(len(item.evidence["source_selection"]), 4)
        self.assertTrue(item.evidence["native_api_admitted"])
        self.assertFalse(item.evidence["qualified_product_e2e"])
        self.assertEqual(item.evidence["configuration"], item.evidence["configuration_before"])

    def test_shadowed_project_override_does_not_qualify(self):
        item = self.fixture("misroute")
        with self.assertRaisesRegex(ValueError, "selected source"):
            item.cline()
        self.assertFalse(item.evidence["native_api_admitted"])

    def test_success_exit_help_does_not_qualify(self):
        item = self.fixture("help")
        with self.assertRaises(ValueError):
            item.cline()
        self.assertFalse(item.evidence["native_api_admitted"])

    def test_foreign_config_mutation_refused(self):
        item = self.fixture("write")
        with self.assertRaisesRegex(ValueError, "changed fixture"):
            item.cline()
        self.assertFalse(item.evidence["native_api_admitted"])
        self.assertNotEqual(item.evidence["configuration"], item.evidence["configuration_before"])

    def test_failed_native_command_still_checks_project_b(self):
        item = self.fixture("write-then-fail")
        with self.assertRaisesRegex(ValueError, "changed fixture"):
            item.cline()
        self.assertFalse(item.evidence["native_api_admitted"])
        self.assertIn("unexpected", item.evidence["configuration"]["b"])


class GrokRouteContracts(unittest.TestCase):
    """Inert authority negatives, not a replacement for a genuine native run."""
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name).resolve()

    def fixture(self, mode="missing-id"):
        class Inert(probe.Probe):
            def run(inner, label, args, cwd=None, overrides=None):
                inner.calls.append((label, list(args)))
                if label == "version":
                    return 0, b"grok 1.0.70 (f50904eaee6c)\n"
                if "--help" in args:
                    return 0, b"Usage: grok plugin [OPTIONS] <COMMAND>"
                if label == "list-before-a":
                    if mode in ("write-b", "write-b-and-fail"):
                        (inner.b / "unexpected").write_text("foreign mutation")
                        if mode == "write-b-and-fail":
                            raise ValueError("native command failed")
                    if mode == "write-config":
                        (inner.home / ".grok/config.toml").write_text("changed=true")
                    if mode == "full-id":
                        return 0, json.dumps([{"name": NAME, "scope": "project",
                            "path": str(inner.a / ".grok/plugins/project-probe"),
                            "id": "project/abcdef12/" + NAME}]).encode()
                if label in ("inspect-before-a", "inspect-before-b"):
                    project = inner.a if label == "inspect-before-a" else inner.b
                    return 0, json.dumps({"grokVersion": "1.0.70", "cwd": str(project),
                                          "projectRoot": str(project) + "/",
                                          "projectTrusted": mode == "trusted",
                                          "plugins": [{"name": NAME, "scope": "project",
                                                       "path": str(project / ".grok/plugins/project-probe"),
                                                       "enabled": False}]}).encode()
                return 0, b"[]"
        item = object.__new__(Inert)
        item.root, item.binary = self.root, self.root / "bin/grok"
        item.home, item.a, item.b = [self.root / name for name in ("home", "a", "b")]
        for path in (item.home, item.a, item.b):
            path.mkdir()
        item.calls = []
        item.evidence = {"commands": [], "native_api_admitted": False, "qualified_product_e2e": False}
        return item

    def test_observed_empty_inventory_does_not_enable_or_qualify(self):
        item = self.fixture()
        item.grok()
        self.assertFalse(item.evidence["enable_admission"]["admitted"])
        self.assertFalse(item.evidence["native_api_admitted"])
        self.assertEqual(item.evidence["after"], item.evidence["before"])
        self.assertFalse(any("enable" in args and "--help" not in args for _, args in item.calls))
        with self.assertRaises(ValueError):
            probe.require_admission(dict(item.evidence, research_completed=True))

    def test_full_identity_alone_does_not_authorize_mutation(self):
        item = self.fixture("full-id")
        item.grok()
        self.assertEqual(item.evidence["enable_admission"]["id"], "project/abcdef12/" + NAME)
        self.assertFalse(item.evidence["enable_admission"]["admitted"])
        self.assertFalse(any("enable" in args and "--help" not in args for _, args in item.calls))

    def test_first_discovery_cannot_modify_project_b(self):
        item = self.fixture("write-b")
        with self.assertRaisesRegex(ValueError, "changed fixture"):
            item.grok()

    def test_failed_discovery_still_checks_project_b(self):
        item = self.fixture("write-b-and-fail")
        with self.assertRaisesRegex(ValueError, "changed fixture"):
            item.grok()
        self.assertIn("unexpected", item.evidence["after"]["b"])

    def test_first_discovery_cannot_modify_user_config(self):
        item = self.fixture("write-config")
        with self.assertRaisesRegex(ValueError, "changed Grok user configuration"):
            item.grok()

    def test_unexpected_trusted_project_refused(self):
        item = self.fixture("trusted")
        with self.assertRaisesRegex(ValueError, "untrusted project differs"):
            item.grok()


if __name__ == "__main__":
    unittest.main()
