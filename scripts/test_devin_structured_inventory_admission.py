"""Inert contracts: no native client, network acquisition or hosted spoofing."""
import copy
import hashlib
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import devin_structured_inventory_admission as case


class DevinAdmissionContracts(unittest.TestCase):
    def test_unique_bounded_native_json(self):
        self.assertEqual(case.strict_json(b'[{"name":"owned","enabled":false}]'), [{"name": "owned", "enabled": False}])
        for body in (b'{"a":1,"a":2}', b'{} {}', b'{"a":NaN}', b'{"x":1e9999}', b'{"x":-1e9999}', b'null', b'x' * ((1 << 20) + 1), b'[' * 18 + b'0' + b']' * 18):
            with self.subTest(body=body[:40]), self.assertRaises(ValueError):
                case.strict_json(body)

    def test_archive_requires_reviewed_binary_not_old_pin_or_executable_companion(self):
        files = [{"name": "bin/devin", "kind": "file", "bytes": 213734376, "executable": True, "sha256": case.BINARY}]
        files += [{"name": "companion-" + str(i), "kind": "file", "bytes": 0, "executable": False} for i in range(233)]
        files[1]["bytes"] = 214280841 - files[0]["bytes"]
        files += [{"name": "dir-" + str(i), "kind": "directory", "bytes": 0, "executable": True} for i in range(16)]
        receipt = {"pin_verified": True, "sha256": case.ARCHIVE, "bytes": 72925933, "members": files}
        case.admitted_archive(receipt)
        for change in ("old-pin", "wrong-binary", "special", "executable", "duplicate-count"):
            bad = copy.deepcopy(receipt)
            if change == "old-pin": bad["sha256"] = "0" * 64
            elif change == "wrong-binary": bad["members"][0]["sha256"] = "0" * 64
            elif change == "special": bad["members"][1]["kind"] = "link"
            elif change == "executable": bad["members"][1]["executable"] = True
            else: bad["members"].pop()
            with self.subTest(change=change), self.assertRaises(ValueError): case.admitted_archive(bad)

    def test_connection_guard_includes_non_rpc_connections_and_stdio_start(self):
        probe = SimpleNamespace(methods=lambda: (), call_records=lambda: ([], 0), accepted_connections=[])
        with tempfile.TemporaryDirectory() as temporary:
            marker = Path(temporary) / "started"
            case.zero_effects({"owned": probe}, marker)
            probe.accepted_connections.append(("127.0.0.1", 1234))
            with self.assertRaises(ValueError): case.zero_effects({"owned": probe}, marker)
            probe.accepted_connections.clear()
            marker.write_text("unexpected")
            with self.assertRaises(ValueError): case.zero_effects({"owned": probe}, marker)

    def test_probe_observer_preserves_existing_handler_and_sets_connection_deadline(self):
        connection = Mock()
        address = ("127.0.0.1", 1234)
        server = SimpleNamespace(get_request=lambda: (connection, address))
        probe = case.observe_connections(SimpleNamespace(server=server))
        self.assertEqual(probe.server.get_request(), (connection, address))
        connection.settimeout.assert_called_once_with(2)
        self.assertEqual(probe.accepted_connections, [address])

    def test_runner_exception_keeps_process_cleanup_fence_closed(self):
        ctx = object.__new__(case.AdmissionContext)
        ctx.summary = {}
        with patch.object(case.shared.Context, "command", side_effect=RuntimeError("cleanup failed")):
            with self.assertRaisesRegex(RuntimeError, "cleanup failed"):
                ctx.command("native-list", [])
        self.assertFalse(ctx.process_cleanup_proved)
        self.assertEqual(ctx.summary["in_flight_command"], "native-list")
        ctx = object.__new__(case.AdmissionContext)
        ctx.summary = {}
        negative = ({"exit": 1}, b"", b"authentication refused")
        with patch.object(case.shared.Context, "command", return_value=negative):
            self.assertEqual(ctx.command("native-list", []), negative)
        self.assertTrue(ctx.process_cleanup_proved)
        self.assertNotIn("in_flight_command", ctx.summary)

    def test_outer_inventory_detects_new_runtime_companion_and_unknown_directory(self):
        with tempfile.TemporaryDirectory() as temporary, case.shared.Budget(10) as budget:
            root = Path(temporary).resolve()
            runtime = root / "client"
            runtime.mkdir()
            (runtime / "devin").write_bytes(b"inert original runtime")
            before = case.metadata_inventory(root, budget)
            companion = runtime / "new-companion"
            companion.write_bytes(b"unexpected")
            self.assertNotEqual(before, case.metadata_inventory(root, budget))
            companion.unlink()
            unknown = root / "unknown-native-directory"
            unknown.mkdir()
            self.assertNotEqual(before, case.metadata_inventory(root, budget))
            unknown.rmdir()
            (runtime / "alias").symlink_to(runtime / "devin")
            with self.assertRaisesRegex(ValueError, "alias/special"):
                case.metadata_inventory(root, budget)

    def test_fixture_preserves_original_profile_legacy_and_collision_sources(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            home = root / "home"; home.mkdir()
            projects = {name: root / ("project-" + name) for name in ("a", "b", "user")}
            for path in projects.values(): path.mkdir()
            ctx = SimpleNamespace(root=root, home=home, projects=projects, env={"XDG_CONFIG_HOME": str(home / "config")})
            urls = {name: "http://127.0.0.1:" + str(1000 + i) + "/mcp" for i, name in enumerate(("a", "b", "user", "local", "legacy"))}
            expected, startup, marker = case.fixture(ctx, urls, "f" * 64)
            self.assertFalse(startup.exists())
            self.assertIn("unexpected native start", marker.read_text())
            for name in ("a", "b", "user"):
                entry = expected[name]
                body = (Path(entry["root"]) / "mcp_config.json").read_bytes()
                self.assertEqual(hashlib.sha256(body).hexdigest(), entry["mcp_config_sha256"])
                self.assertEqual(json.loads(body)["mcpServers"]["same-name"]["url"], urls[name])
                self.assertFalse(json.loads(body)["mcpServers"]["same-name"]["disabled"])
            self.assertEqual(json.loads(Path(expected["local"]["path"]).read_text())["mcpServers"]["same-name"]["url"], urls["local"])
            self.assertEqual(json.loads((home / ".codeium/windsurf/mcp_config.json").read_text())["mcpServers"]["legacy-foreign"], {"url": urls["legacy"], "disabled": True})
            self.assertIn("inert_legacy_sentinel", (home / ".codeium/windsurf/config.json").read_text())
            self.assertIn("legacy-foreign", (home / ".codeium/windsurf/skills/legacy-foreign/SKILL.md").read_text())


if __name__ == "__main__": unittest.main()
