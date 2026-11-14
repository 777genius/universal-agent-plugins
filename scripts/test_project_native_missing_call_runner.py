"""Auth-free source/contracts for the selected real-client native runner.

These tests verify test harness safety, not actual vendor login or tools/call.
"""
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from project_native_missing_call_runner import (
    agent_command, digest, installation_id, installed_config, permission_fixture,
    require_both_preflight_receipts, run_both_preflight, source_build, stage_shared_source,
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
        self.assertIn("--allowed-mcp-server-names", qoder)
        self.assertIn("uap_qoder_call", qoder)
        for argv in (cursor, qoder):
            self.assertNotIn("--force", argv)
            self.assertNotIn("--yolo", argv)
            self.assertNotIn("--strict-mcp-config", argv)
            self.assertNotIn("--mcp-config", argv)

    def test_source_build_uses_prepared_scratch_bin(self):
        # The runner prepares work/bin before source_build. Its mkdir must
        # accept that existing directory rather than failing before Go runs.
        with tempfile.TemporaryDirectory() as temp:
            work = Path(temp)
            (work / "bin").mkdir()
            (work / "source").mkdir()
            labels = []

            def simulated_invoke(label, argv, cwd, env, private, timeout=35):
                labels.append(label)
                if label == "go-version":
                    return b"go version go1.26.11 linux/amd64\n", b""
                self.assertIn(label, ("build-installer", "build-probe"))
                Path(argv[argv.index("-o") + 1]).write_bytes(b"source-built")
                return b"", b""

            with patch("project_native_missing_call_runner.shutil.which", return_value="/pinned/go"), \
                    patch("project_native_missing_call_runner.invoke", side_effect=simulated_invoke):
                installer, probe = source_build(work, {}, work / "private")
            self.assertTrue(installer.is_file())
            self.assertTrue(probe.is_file())
            self.assertEqual(labels, ["go-version", "build-installer", "build-probe"])

    def test_shared_source_copies_exact_bytes_without_symlinks(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source_installer = root / "installer"
            source_probe = root / "probe"
            source_installer.write_bytes(b"pinned real source installer bytes")
            source_probe.write_bytes(b"pinned source probe bytes")
            prepared = (source_installer, source_probe,
                        digest(source_installer.read_bytes()), digest(source_probe.read_bytes()))
            first, second = root / "cursor", root / "qoder"
            first.mkdir()
            second.mkdir()
            for target in (first, second):
                installer, probe = stage_shared_source(target, prepared)
                self.assertEqual(installer.read_bytes(), source_installer.read_bytes())
                self.assertEqual(probe.read_bytes(), source_probe.read_bytes())
                self.assertNotEqual(installer, source_installer)
                self.assertFalse(installer.is_symlink())
                self.assertTrue(installer.stat().st_mode & 0o111)
            with self.assertRaises(ValueError):
                stage_shared_source(first, prepared)
            source_probe.write_bytes(b"tampered after first client")
            third = root / "tampered"
            third.mkdir()
            with self.assertRaises(ValueError):
                stage_shared_source(third, prepared)
            link = root / "bad-symlink"
            link.symlink_to(source_installer)
            fourth = root / "symlink"
            fourth.mkdir()
            with self.assertRaises(ValueError):
                stage_shared_source(fourth, (link, source_probe, prepared[2], digest(source_probe.read_bytes())))

    def test_combined_batch_builds_once_and_keeps_clients_separate(self):
        expected = "a" * 40
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            evidence = root / "evidence"
            calls = []
            builds = []

            def fake_build(work, env, private):
                builds.append(work)
                installer, probe = work / "bin" / "agentplugins", work / "source" / "bin" / "probe"
                installer.parent.mkdir(parents=True)
                probe.parent.mkdir(parents=True)
                installer.write_bytes(b"real source installer")
                probe.write_bytes(b"real source probe")
                return installer, probe

            def fake_client(client, output, commit, *, preflight_only=False, prepared_source=None):
                self.assertTrue(preflight_only)
                self.assertEqual(commit, expected)
                self.assertEqual(output, evidence / client)
                self.assertEqual(prepared_source[2], digest(b"real source installer"))
                self.assertEqual(prepared_source[3], digest(b"real source probe"))
                calls.append(client)
                output.mkdir(parents=True)
                (output / "source-only-preflight.json").write_text(json.dumps({
                    "schema_version": 1, "client": client, "source_commit": expected,
                    "native_version": {"cursor": "2026.10.22-27dcf5f", "qoder": "1.1.78"}[client],
                    "source_built_uap_and_fixture": True,
                    "genuine_native_server_listed": True,
                    "uap_owned_project_a_installed_and_removed": True,
                    "other_project_and_foreign_state_preserved": True,
                    "vendor_authentication_qualified": False,
                    "native_tools_call_qualified": False,
                    "real_model_qualified": False,
                    "vendor_archive_sha256": "b" * 64,
                    "native_executable_sha256": "c" * 64,
                }))

            with patch.dict(os.environ, {"RUNNER_TEMP": str(root),
                    "CURSOR_API_KEY": "", "QODER_PERSONAL_ACCESS_TOKEN": ""}, clear=False), \
                    patch("project_native_missing_call_runner.native.native.require_hosted"), \
                    patch("project_native_missing_call_runner.verify_clean_head"), \
                    patch("project_native_missing_call_runner.source_build", side_effect=fake_build), \
                    patch("project_native_missing_call_runner.run", side_effect=fake_client):
                run_both_preflight(evidence, expected)
            self.assertEqual(len(builds), 1)
            self.assertEqual(calls, ["cursor", "qoder"])
            self.assertEqual(sorted(path.name for path in evidence.iterdir()), ["cursor", "qoder"])

    def test_failed_second_client_never_creates_successful_batch(self):
        expected = "a" * 40
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            evidence = root / "evidence"
            calls = []

            def fake_build(work, env, private):
                installer, probe = work / "bin" / "agentplugins", work / "source" / "bin" / "probe"
                installer.parent.mkdir(parents=True)
                probe.parent.mkdir(parents=True)
                installer.write_bytes(b"installer")
                probe.write_bytes(b"probe")
                return installer, probe

            def fake_client(client, output, commit, *, preflight_only=False, prepared_source=None):
                self.assertTrue(preflight_only)
                self.assertEqual(commit, expected)
                calls.append(client)
                if client == "qoder":
                    raise RuntimeError("Qoder failed")
                output.mkdir(parents=True)
                (output / "source-only-preflight.json").write_text("{}")

            with patch.dict(os.environ, {
                    "RUNNER_TEMP": str(root), "CURSOR_API_KEY": "",
                    "QODER_PERSONAL_ACCESS_TOKEN": ""}, clear=False), \
                    patch("project_native_missing_call_runner.native.native.require_hosted"), \
                    patch("project_native_missing_call_runner.verify_clean_head"), \
                    patch("project_native_missing_call_runner.source_build", side_effect=fake_build), \
                    patch("project_native_missing_call_runner.run", side_effect=fake_client):
                with self.assertRaisesRegex(RuntimeError, "Qoder failed"):
                    run_both_preflight(evidence, expected)
            self.assertEqual(calls, ["cursor", "qoder"])
            self.assertTrue((evidence / "cursor" / "source-only-preflight.json").is_file())
            self.assertFalse((evidence / "qoder").exists())
            with self.assertRaises(ValueError):
                require_both_preflight_receipts(evidence, expected)

    def test_batch_refuses_partial_and_false_model_receipts(self):
        expected = "a" * 40
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp)
            (output / "cursor").mkdir()
            with self.assertRaises(ValueError):
                require_both_preflight_receipts(output, expected)
            for client in ("cursor", "qoder"):
                (output / client).mkdir(exist_ok=True)
                (output / client / "source-only-preflight.json").write_text(json.dumps({
                    "schema_version": 1, "client": client, "source_commit": expected,
                    "native_version": {"cursor": "2026.10.22-27dcf5f", "qoder": "1.1.78"}[client],
                    "source_built_uap_and_fixture": True,
                    "genuine_native_server_listed": True,
                    "uap_owned_project_a_installed_and_removed": True,
                    "other_project_and_foreign_state_preserved": True,
                    "vendor_authentication_qualified": False,
                    "native_tools_call_qualified": False,
                    "real_model_qualified": False,
                    "vendor_archive_sha256": "b" * 64,
                    "native_executable_sha256": "c" * 64,
                }))
            require_both_preflight_receipts(output, expected)
            qoder = output / "qoder" / "source-only-preflight.json"
            data = json.loads(qoder.read_text())
            data["native_tools_call_qualified"] = True
            qoder.write_text(json.dumps(data))
            with self.assertRaises(ValueError):
                require_both_preflight_receipts(output, expected)
            data["native_tools_call_qualified"] = False
            data["native_executable_sha256"] = "not-an-original-hash"
            qoder.write_text(json.dumps(data))
            with self.assertRaises(ValueError):
                require_both_preflight_receipts(output, expected)

    def test_combined_batch_refuses_vendor_credentials_before_build(self):
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp) / "evidence"
            with patch.dict(os.environ, {"CURSOR_API_KEY": "nonempty",
                                        "QODER_PERSONAL_ACCESS_TOKEN": "",
                                        "RUNNER_TEMP": temp}), \
                    patch("project_native_missing_call_runner.native.native.require_hosted"), \
                    patch("project_native_missing_call_runner.source_build") as build:
                with self.assertRaises(ValueError):
                    run_both_preflight(output, "a" * 40)
                build.assert_not_called()
            self.assertFalse(output.exists())

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
        if client == "qoder":
            selected_command, selected_args = str(probe), ["call-fixture"]
        else:
            selected_command, selected_args = str(executable), ["--internal-stdio-v1", str(active)]
        members = {
            "foreign_keep": {"command": "/usr/bin/true", "args": []},
            name: {"type": "stdio", "command": selected_command,
                   "args": selected_args,
                   "env": {"UAP_TEST_NONCE": nonce, "UAP_TEST_EVENTS": str(events),
                           "PLUGIN_ROOT": str(active),
                           "PLUGIN_DATA": str(project / ".uap" / "plugin-data" / active.name)}}
        }
        config.write_text(json.dumps({"mcpServers": members}))
        return project, name, nonce, events, installer, source_probe, active, config

    def test_managed_source_byte_identity_cursor_and_qoder(self):
        for client in ("cursor", "qoder"):
            with self.subTest(client=client), tempfile.TemporaryDirectory() as temp:
                p, name, nonce, events, installer, probe, active, _ = self.fake_project(Path(temp), client)
                self.assertEqual(installed_config(client, p, name, nonce, events, installer, probe), active)

    def test_qoder_project_refuses_cursor_helper_or_foreign_executable(self):
        with tempfile.TemporaryDirectory() as temp:
            project, name, nonce, events, installer, probe, active, path = self.fake_project(Path(temp), "qoder")
            doc = json.loads(path.read_text())
            doc["mcpServers"][name]["command"] = str(active / "io.github.777genius.agentplugins" / "managed-stdio-v1" / "agentplugins")
            doc["mcpServers"][name]["args"] = ["--internal-stdio-v1", str(active)]
            path.write_text(json.dumps(doc))
            with self.assertRaises(ValueError):
                installed_config("qoder", project, name, nonce, events, installer, probe)
            doc["mcpServers"][name]["command"] = str(probe)
            doc["mcpServers"][name]["args"] = ["call-fixture"]
            doc["mcpServers"][name]["env"]["PLUGIN_DATA"] = str(Path(temp) / "foreign-data")
            path.write_text(json.dumps(doc))
            with self.assertRaises(ValueError):
                installed_config("qoder", project, name, nonce, events, installer, probe)

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
