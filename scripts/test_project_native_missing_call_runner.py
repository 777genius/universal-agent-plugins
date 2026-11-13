"""Auth-free source/contracts for the selected real-client native runner.

These tests verify test harness safety, not actual vendor login or tools/call.
"""
import json
from pathlib import Path
import tempfile
import unittest

from project_native_missing_call_runner import (
    agent_command, installation_id, installed_config, permission_fixture,
)


class RealClientRunnerContractTest(unittest.TestCase):
    def test_exact_named_tool_approval(self):
        with tempfile.TemporaryDirectory() as temp:
            project = Path(temp)
            permission_fixture("cursor", project, "only_cursor")
            permissions = json.loads((project / ".cursor" / "cli.json").read_text())["permissions"]
            self.assertEqual(permissions["allow"], ["Mcp(only_cursor:inspect_runtime)"])
            self.assertIn("Shell(*)", permissions["deny"])
            self.assertIn("Read(**/*)", permissions["deny"])
        with tempfile.TemporaryDirectory() as temp:
            project = Path(temp)
            permission_fixture("qoder", project, "only_qoder")
            settings = json.loads((project / ".qoder" / "settings.json").read_text())
            self.assertEqual(settings["mcp"]["enabledProjectMcpServers"], ["only_qoder"])
            self.assertIs(settings["mcp"]["enableAllProjectMcpServers"], False)
            self.assertEqual(settings["permissions"]["allow"], ["mcp__only_qoder__inspect_runtime"])
            self.assertIn("Read", settings["permissions"]["deny"])
            self.assertNotIn("bypass_permissions", json.dumps(settings))

    def test_agent_invocation_only_uses_native_model_session(self):
        binary = Path("/checked/native/agent")
        cursor = agent_command("cursor", binary, "read-only")
        qoder = agent_command("qoder", binary, "read-only")
        self.assertEqual(cursor[:2], [binary, "-p"])
        self.assertEqual(qoder[:2], [binary, "-p"])
        for argv in (cursor, qoder):
            self.assertNotIn("--force", argv)
            self.assertNotIn("--yolo", argv)
            self.assertNotIn("--strict-mcp-config", argv)
            self.assertNotIn("--mcp-config", argv)

    def test_requires_public_uap_ownership_receipt(self):
        self.assertEqual(installation_id({"data": {"targets": [{
            "target": "cursor", "output": {"result": {"installation_id": "a" * 36}}
        }]}}, "cursor"), "a" * 36)
        self.assertEqual(installation_id({"data": {"result": {"installation_id": "b" * 36}}}, "qoder"), "b" * 36)
        with self.assertRaises(ValueError):
            installation_id({"data": {"targets": [{"target": "qoder", "output": {"result": {"installation_id": "a" * 36}}}]}}, "cursor")

    def fake_project(self, root, client):
        project = root / "a"
        name = "uap_" + client + "_call"
        active = project / ".uap" / "managed" / "clients" / client / "owned-package"
        executable = active / "io.github.777genius.agentplugins" / "managed-stdio-v1" / "agentplugins"
        executable.parent.mkdir(parents=True, exist_ok=True)
        executable.write_bytes(b"authored UAP build bytes")
        probe = active / "bin" / "probe"
        probe.parent.mkdir(parents=True, exist_ok=True)
        probe.write_bytes(b"native fixture bytes")
        installer = root / "source-installer"
        source_probe = root / "source-probe"
        installer.write_bytes(executable.read_bytes())
        source_probe.write_bytes(probe.read_bytes())
        events = root / "events.jsonl"
        nonce = "c" * 32
        config = project / (".cursor/mcp.json" if client == "cursor" else ".mcp.json")
        config.parent.mkdir(parents=True, exist_ok=True)
        members = {
            "foreign_keep": {"command": "/usr/bin/true", "args": []},
            name: {"type": "stdio", "command": str(executable),
                   "args": ["--internal-stdio-v1", str(active)],
                   "env": {"UAP_TEST_NONCE": nonce, "UAP_TEST_EVENTS": str(events)}}
        }
        config.write_text(json.dumps({"mcpServers": members}))
        return project, name, nonce, events, installer, source_probe, active, config

    def test_managed_source_byte_identity_cursor_and_qoder(self):
        for client in ("cursor", "qoder"):
            with self.subTest(client=client), tempfile.TemporaryDirectory() as temp:
                p, name, nonce, events, installer, probe, active, _ = self.fake_project(Path(temp), client)
                self.assertEqual(installed_config(client, p, name, nonce, events, installer, probe), active)

    def test_foreign_source_substitution_refused(self):
        with tempfile.TemporaryDirectory() as temp:
            p, name, nonce, events, installer, probe, active, config = self.fake_project(Path(temp), "qoder")
            config_obj = json.loads(config.read_text())
            config_obj["mcpServers"][name]["env"]["UAP_TEST_NONCE"] = "wrong"
            config.write_text(json.dumps(config_obj))
            with self.assertRaises(ValueError):
                installed_config("qoder", p, name, nonce, events, installer, probe)
        with tempfile.TemporaryDirectory() as temp:
            p, name, nonce, events, installer, probe, active, config = self.fake_project(Path(temp), "cursor")
            (active / "bin" / "probe").write_bytes(b"substituted executable")
            with self.assertRaises(ValueError):
                installed_config("cursor", p, name, nonce, events, installer, probe)


if __name__ == "__main__":
    unittest.main()
