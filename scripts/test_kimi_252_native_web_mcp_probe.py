"""Contract checks for bounded native Kimi 2.5.2 MCP management evidence."""
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
import tempfile
import threading
import unittest
from unittest.mock import patch
from pathlib import Path

import kimi_252_native_web_mcp_probe as native


@contextmanager
def local_api(*, code=0, status=200, location=None):
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append(self.headers.get("Authorization"))
            self.send_response(status)
            if location is not None:
                self.send_header("Location", location)
            self.end_headers()
            self.wfile.write(json.dumps({"code": code, "data": {"observed": True}}).encode())

        def log_message(self, *_):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=lambda: server.serve_forever(poll_interval=.01), daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:" + str(server.server_port), requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


class KimiNativeWebProbeTests(unittest.TestCase):
    def test_redirect_never_forwards_bearer_to_another_origin(self):
        for status in (301, 302, 303, 307, 308):
            with self.subTest(status=status), local_api() as (destination, received):
                with local_api(status=status, location=destination + "/capture") as (base, initial):
                    with self.assertRaisesRegex(ValueError, "redirect refused"):
                        native.response("/api/v1/meta", base, "inert-local-token")
                    self.assertEqual(initial, ["Bearer inert-local-token"])
                self.assertEqual(received, [])

    def test_ambient_proxy_never_receives_local_bearer(self):
        with local_api() as (proxy, intercepted), local_api() as (base, received):
            environment = {"http_proxy": proxy, "https_proxy": proxy,
                           "HTTP_PROXY": proxy, "HTTPS_PROXY": proxy,
                           "no_proxy": "", "NO_PROXY": ""}
            with patch.dict(os.environ, environment), patch.object(native.urllib.request, "_opener", None):
                self.assertEqual(native.response("/api/v1/meta", base, "inert-local-token"),
                                 {"observed": True})
            self.assertEqual(received, ["Bearer inert-local-token"])
            self.assertEqual(intercepted, [])

    def test_native_api_success_code_is_an_actual_integer(self):
        with local_api() as (base, _):
            self.assertEqual(native.response("/api/v1/meta", base, "inert-local-token"),
                             {"observed": True})
        for code in (False, 0.0, "0", None, 1):
            with self.subTest(code=code), local_api(code=code) as (base, _):
                with self.assertRaisesRegex(ValueError, "native API request refused"):
                    native.response("/api/v1/meta", base, "inert-local-token")

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
