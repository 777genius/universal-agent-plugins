"""Inert contracts; these tests do not claim native Grok acceptance."""
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest

from grok_project_protocol_client import (document, exercise, installed_server,
                                          read_owned, selected_installation)
from grok_project_protocol_e2e import observed_protocol


class ContractTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name).resolve()
        self.active = self.root / ".grok/plugins/owned"
        self.data = self.root / ".uap/plugin-data/owned"
        self.active.mkdir(parents=True)
        self.data.mkdir(parents=True)
        self.binding = {"client_id": "grok", "scope": "project", "activation": "prepared",
            "verification": "package_validated", "physical_artifact_id": "owned",
            "target_locator": str(self.active), "native_profile_root": str(self.root / ".grok"),
            "data_receipt_id": "receipt", "client_binding_id": "binding"}
        self.installation = {"declared_name": "fixture", "installation_id": "install",
            "clients": {"grok": self.binding}, "data_receipts": {"receipt": {
                "scope": "project", "locator": str(self.data), "physical_backend_id": "owned", "state": "owned"}}}
        self.state = {"schema_version": 2, "installations": [self.installation]}
        self.state_path = self.root / ".uap/state-v2.json"
        self.write_state()
        command = self.active / "probe"
        command.write_text("fixture")
        self.server = {"command": str(command), "args": [], "cwd": str(self.active), "env": {
            "PLUGIN_ROOT": str(self.active), "PLUGIN_DATA": str(self.data),
            "UAP_TEST_ROOT": str(self.root), "UAP_TEST_NONCE": "nonce"}}

    def write_state(self):
        self.state_path.write_text(json.dumps(self.state))

    def select(self):
        self.write_state()
        return selected_installation(self.root, "fixture")

    def test_exact_owned_receipt(self):
        installation, binding, active, data = self.select()
        self.assertEqual(installation["installation_id"], "install")
        self.assertEqual(binding["client_binding_id"], "binding")
        self.assertEqual((active, data), (self.active, self.data))

    def test_foreign_scope_path_activation_and_receipt_refused(self):
        for key, value in (("scope", "user"), ("client_id", "codex"), ("activation", "active"),
                           ("target_locator", "/outside"), ("physical_artifact_id", ".."),
                           ("native_profile_root", "/outside"), ("data_receipt_id", "foreign")):
            with self.subTest(key=key):
                saved = self.binding[key]
                self.binding[key] = value
                with self.assertRaises(ValueError):
                    self.select()
                self.binding[key] = saved

    def test_duplicate_and_absent_installation_refused(self):
        for entries in ([], [self.installation, self.installation]):
            with self.subTest(entries=len(entries)):
                self.state["installations"] = entries
                with self.assertRaises(ValueError):
                    self.select()

    def test_json_duplicates_nonfinite_and_limit_refused(self):
        for body in (b'{"a":1,"a":2}', b'{"a":NaN}', b'{"a":Infinity}', b"", b"a" * ((1 << 20) + 1)):
            with self.subTest(body=body[:20]), self.assertRaises(ValueError):
                document(body)

    def test_symlink_and_foreign_read_refused(self):
        alias = self.root / "alias"
        alias.symlink_to(self.state_path)
        for path in (alias, Path("/etc/passwd")):
            with self.subTest(path=path), self.assertRaises(ValueError):
                read_owned(path, self.root)

    def test_installed_config_is_exact_and_credential_free(self):
        sidecar = self.active / ".mcp.json"
        sidecar.write_text(json.dumps({"mcpServers": {"probe": self.server}}))
        self.assertEqual(installed_server(self.active, self.data, self.root, "probe", "nonce"), self.server)
        self.server["env"]["XAI_API_KEY"] = "not-a-real-key"
        sidecar.write_text(json.dumps({"mcpServers": {"probe": self.server}}))
        with self.assertRaisesRegex(ValueError, "environment"):
            installed_server(self.active, self.data, self.root, "probe", "nonce")

    def test_foreign_command_and_extra_server_refused(self):
        for command, extra in (("/bin/true", False), (self.server["command"], True)):
            self.server["command"] = command
            servers = {"probe": self.server}
            if extra:
                servers["foreign"] = self.server
            (self.active / ".mcp.json").write_text(json.dumps({"mcpServers": servers}))
            with self.assertRaises(ValueError):
                installed_server(self.active, self.data, self.root, "probe", "nonce")

    def test_native_events_must_share_session_and_nonce(self):
        good = [{"session": "s", "pid": 1, "nonce": "n", "method": method}
                for method in ("initialize", "tools/list")]
        self.assertEqual(len(observed_protocol(good, "n", ("initialize", "tools/list"))), 1)
        for bad in (good[:1], list(reversed(good)), [dict(row, session="") for row in good],
                    [dict(row, pid=0) for row in good],
                    [good[0], dict(good[1], session="other")],
                    [good[0], dict(good[1], nonce="wrong")]):
            with self.subTest(rows=bad), self.assertRaises(ValueError):
                observed_protocol(bad, "n", ("initialize", "tools/list"))

    def test_sequential_direct_protocol_and_bad_nonce(self):
        # Contract responder only; the genuine hosted E2E uses the unchanged Go probe.
        child = self.active / "contract_server.py"
        child.write_text('''import json, os, sys
for line in sys.stdin:
 r=json.loads(line)
 if "id" not in r: continue
 m=r["method"]; response={"jsonrpc":"2.0","id":r["id"]}
 if m=="initialize": value={"protocolVersion":"2025-06-18","serverInfo":{"name":"agentplugins-native-probe"}}
 elif m=="tools/list": value={"tools":[{"name":"inspect_runtime"}]}
 else:
  if r["params"]["arguments"]["nonce"]!="nonce":
   response["error"]={"code":-32602}; print(json.dumps(response),flush=True); continue
  facts={"nonce":"nonce","revision":"0","marker":"marker","plugin_root":os.environ["PLUGIN_ROOT"],"plugin_data":os.environ["PLUGIN_DATA"],"cwd":os.getcwd(),"session":"contract"}
  value={"isError":False,"content":[{"type":"text","text":json.dumps(facts)}]}
 response["result"]=value; print(json.dumps(response),flush=True)
''')
        server = dict(self.server, command=sys.executable, args=[str(child)])
        result = exercise(server, {"PATH": "/usr/bin:/bin"}, "nonce", "0", "marker", True)
        self.assertEqual(result["caller"], "direct-installed-stdio-client")
        self.assertFalse(result["grok_model_invoked"])
        self.assertEqual(len(result["transcript"]), 9)
        self.assertEqual(result["transcript"][-1]["message"]["error"]["code"], -32602)


if __name__ == "__main__":
    unittest.main()
