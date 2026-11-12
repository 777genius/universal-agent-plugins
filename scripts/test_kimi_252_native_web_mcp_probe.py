"""Contract checks for bounded native Kimi 2.5.2 MCP management evidence."""
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path

import kimi_252_native_web_mcp_probe as native


class KimiNativeWebProbeTests(unittest.TestCase):
    def test_rejects_wrong_project_attribution(self):
        path = Path("/private/project/.kimi-code/mcp.json")
        valid = {"name": native.SERVER_NAME, "source": "global",
                 "origin": str(path), "mutable": False}
        native.ensure_project_server({native.SERVER_NAME: valid}, path)
        for changed in ({"origin": "/foreign/mcp.json"}, {"mutable": True}, {"source": "plugin"}):
            broken = {**valid, **changed}
            with self.assertRaises(ValueError):
                native.ensure_project_server({native.SERVER_NAME: broken}, path)
        with self.assertRaises(ValueError):
            native.ensure_project_server({}, path)

    @patch.object(native, "response")
    def test_list_shape_and_token_required(self, api):
        api.return_value = [{"name": native.SERVER_NAME, "source": "global"}]
        rows = native.list_servers("http://127.0.0.1:12345", "secret", Path("/a"))
        self.assertIn(native.SERVER_NAME, rows)
        api.return_value = {"name": native.SERVER_NAME}
        with self.assertRaises(ValueError):
            native.list_servers("http://127.0.0.1:12345", "secret", Path("/a"))

    def test_private_token_requires_owned_regular_nonworld_readable_file(self):
        with tempfile.TemporaryDirectory() as temp:
            home = Path(temp)
            token_file = home / "server.token"
            self.assertIsNone(native.native_private_token(home))
            token_file.write_text("a" * 43, encoding="ascii")
            token_file.chmod(0o600)
            self.assertEqual(native.native_private_token(home), "a" * 43)
            token_file.chmod(0o644)
            with self.assertRaises(ValueError):
                native.native_private_token(home)
            token_file.unlink()
            secret = home / "not-the-token"
            secret.write_text("b" * 43, encoding="ascii")
            secret.chmod(0o600)
            token_file.symlink_to(secret)
            with self.assertRaises(ValueError):
                native.native_private_token(home)

    def test_never_queries_a_non_loopback_address(self):
        with self.assertRaises(ValueError):
            native.response("/api/v1/meta", "http://127.0.0.2:1", "abc")
        with self.assertRaises(ValueError):
            native.response("/api/v1/meta", "https://127.0.0.1:1", "abc")
        with self.assertRaises(ValueError):
            native.response("/api/v1/meta", "http://127.0.0.1:1@evil.example:80", "abc")


if __name__ == "__main__":
    unittest.main()
