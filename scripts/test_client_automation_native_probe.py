"""Inert decoder/authority contracts; no vendor client is mocked as a PASS."""
import json
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

    def test_raw_bytes_and_modes_preserved(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            put(root / "config", {"name": NAME})
            item = snapshot(root)["config"]
            self.assertEqual(item["mode"], 0o600)
            self.assertEqual(item["bytes"], (root / "config").stat().st_size)
            self.assertEqual(len(item["sha256"]), 64)

    def test_excluded_clients_are_not_admitted(self):
        for target in ("vscode", "chatgptweb", "chatgpt", "codex", "kimi"):
            self.assertNotIn(target, CLIENTS)


if __name__ == "__main__":
    unittest.main()
