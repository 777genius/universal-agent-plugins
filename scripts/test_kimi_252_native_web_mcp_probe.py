"""Contract checks for bounded native Kimi 2.5.2 MCP management evidence."""
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import http.client
import json
import os
import tempfile
import threading
import unittest
from unittest.mock import MagicMock, patch
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

    @patch.object(native, "response")
    def test_native_catalog_fails_closed_on_ambiguous_or_malformed_rows(self, api):
        row = {"name": native.SERVER_NAME, "source": "global",
               "origin": "/test/project/.kimi-code/mcp.json", "mutable": False}
        for rows, reason in (
            ([row, row], "duplicate"),
            ([row, dict(row, source="plugin")], "duplicate"),
            ([row, None], "invalid"),
            ([row, {"source": "global"}], "invalid"),
            ([row, {"name": ""}], "invalid"),
            ([row, {"name": 123}], "invalid"),
        ):
            with self.subTest(rows=rows), self.assertRaisesRegex(ValueError, reason):
                api.return_value = rows
                native.list_servers("http://127.0.0.1:12345", "secret", Path("/test/project"))
        api.return_value = [row]
        self.assertEqual(native.list_servers("http://127.0.0.1:12345", "secret",
                                              Path("/test/project"))[native.SERVER_NAME], row)


    @patch.object(native.os, "killpg")
    @patch.object(native, "start_server")
    @patch.object(native, "list_servers")
    @patch.object(native, "response")
    def test_lost_grant_ack_still_attempts_revoke_and_kills_native_child(
            self, api, catalog, startup, killpg):
        child = MagicMock(pid=1234)
        startup.return_value = ("http://127.0.0.1:12345", "private-inert-token", child)
        catalog.return_value = {}
        calls = []
        def reply(path, base, token, method="GET", data=None):
            calls.append((path, method))
            if path == "/api/v1/workspaces":
                return {"id": "isolated-project"}
            if path.endswith("/trust") and method == "GET":
                return {"trusted": False}
            if path.endswith("/trust") and method == "POST":
                # The native server may have granted trust before losing the ack.
                raise TimeoutError("injected lost acknowledgment")
            if path.endswith("/untrust"):
                return {"trusted": False}
            raise AssertionError("unexpected native API route")
        api.side_effect = reply
        with self.assertRaises(TimeoutError):
            native.probe(Path("/test/kiro"), {}, Path("/test/project"),
                         Path("/test/project/.kimi-code/mcp.json"))
        self.assertEqual([p for p, _ in calls if p.endswith("/untrust")],
                         ["/api/v1/workspaces/isolated-project/untrust"])
        killpg.assert_called_once_with(1234, native.signal.SIGKILL)
        child.wait.assert_called_once_with(timeout=5)

    @patch.object(native.os, "killpg")
    @patch.object(native, "start_server")
    @patch.object(native, "list_servers")
    @patch.object(native, "response")
    def test_ambiguous_revoke_retries_and_never_reports_tools_list_success(
            self, api, catalog, startup, killpg):
        child = MagicMock(pid=4567)
        startup.return_value = ("http://127.0.0.1:12345", "private-inert-token", child)
        config = Path("/test/project/.kimi-code/mcp.json")
        row = {"name": native.SERVER_NAME, "source": "global",
               "origin": str(config), "mutable": False}
        catalog.side_effect = [{}, {native.SERVER_NAME: row}, {native.SERVER_NAME: row}]
        calls = []
        def reply(path, base, token, method="GET", data=None):
            calls.append((path, method))
            if path == "/api/v1/workspaces":
                return {"id": "isolated-project"}
            if path.endswith("/trust"):
                return {"trusted": method == "POST"}
            if path == "/api/v2/mcp/servers:test":
                return {"success": True, "output": "uap_ping"}
            if path.endswith("/untrust"):
                return {"trusted": False}
            raise AssertionError("unexpected native API route")
        api.side_effect = reply
        with self.assertRaisesRegex(ValueError, "trust revocation did not suppress"):
            native.probe(Path("/test/kimi"), {}, Path("/test/project"), config)
        self.assertEqual(sum(path.endswith("/untrust") for path, _ in calls), 2)
        killpg.assert_called_once_with(4567, native.signal.SIGKILL)
        child.wait.assert_called_once_with(timeout=5)


    @patch.object(native.os, "killpg")
    @patch.object(native, "start_server")
    @patch.object(native, "list_servers")
    @patch.object(native, "response")
    def test_untrust_ack_without_persisted_revocation_fails_closed(
            self, api, catalog, startup, killpg):
        child = MagicMock(pid=6812)
        startup.return_value = ("http://127.0.0.1:12345", "inert-token", child)
        config = Path("/test/project/.kimi-code/mcp.json")
        row = {"name": native.SERVER_NAME, "source": "global",
               "origin": str(config), "mutable": False}
        catalog.side_effect = [{}, {native.SERVER_NAME: row}]
        trust_reads = 0
        revokes = []

        def reply(path, base, token, method="GET", data=None):
            nonlocal trust_reads
            if path == "/api/v1/workspaces":
                return {"id": "isolated-project"}
            if path.endswith("/trust"):
                if method == "POST":
                    return {"trusted": True}
                trust_reads += 1
                return {"trusted": trust_reads > 1}
            if path == "/api/v2/mcp/servers:test":
                return {"success": True, "output": "uap_ping"}
            if path.endswith("/untrust"):
                revokes.append(path)
                return {"trusted": False}
            raise AssertionError("unexpected native API route")

        api.side_effect = reply
        with self.assertRaisesRegex(ValueError, "trust revocation did not suppress"):
            native.probe(Path("/test/kimi"), {}, Path("/test/project"), config)
        self.assertEqual(trust_reads, 2)
        self.assertEqual(len(revokes), 2)
        killpg.assert_called_once_with(6812, native.signal.SIGKILL)
        child.wait.assert_called_once_with(timeout=5)

    @patch.object(native.os, "killpg")
    @patch.object(native, "start_server")
    @patch.object(native, "list_servers")
    @patch.object(native, "response")
    def test_failed_untrust_network_cleanup_still_kills_native_child(
            self, api, catalog, startup, killpg):
        child = MagicMock(pid=5791)
        startup.return_value = ("http://127.0.0.1:12345", "private-inert-token", child)
        config = Path("/test/project/.kimi-code/mcp.json")
        row = {"name": native.SERVER_NAME, "source": "global",
               "origin": str(config), "mutable": False}
        catalog.side_effect = [{}, {native.SERVER_NAME: row}]
        paths = []

        def reply(path, base, token, method="GET", data=None):
            paths.append(path)
            if path == "/api/v1/workspaces":
                return {"id": "isolated-project"}
            if path.endswith("/trust"):
                return {"trusted": method == "POST"}
            if path == "/api/v2/mcp/servers:test":
                return {"success": True, "output": "uap_ping"}
            if path.endswith("/untrust"):
                raise TimeoutError("native revoke acknowledgment lost")
            raise AssertionError("unexpected native API route")

        api.side_effect = reply
        with self.assertRaisesRegex(TimeoutError, "native revoke acknowledgment lost"):
            native.probe(Path("/test/kimi"), {}, Path("/test/project"), config)
        self.assertEqual(sum(path.endswith("/untrust") for path in paths), 2)
        killpg.assert_called_once_with(5791, native.signal.SIGKILL)
        child.wait.assert_called_once_with(timeout=5)


    @patch.object(native.os, "killpg")
    @patch.object(native, "start_server")
    @patch.object(native, "list_servers")
    @patch.object(native, "response")
    def test_http_bad_status_during_untrust_cleanup_kills_child(
            self, api, catalog, startup, killpg):
        """An HTTPException during cleanup must not strand Kimi's process group."""
        child = MagicMock(pid=6821)
        startup.return_value = ("http://127.0.0.1:12345", "inert-token", child)
        catalog.return_value = {}

        def reply(path, base, token, method="GET", data=None):
            if path == "/api/v1/workspaces":
                return {"id": "test-owned-project"}
            if path.endswith("/trust") and method == "GET":
                return {"trusted": False}
            if path.endswith("/trust") and method == "POST":
                raise TimeoutError("grant acknowledgment lost")
            if path.endswith("/untrust"):
                raise http.client.BadStatusLine("malformed native HTTP response")
            raise AssertionError("unexpected native API route")

        api.side_effect = reply
        with self.assertRaisesRegex(TimeoutError, "grant acknowledgment lost"):
            native.probe(Path("/test/kimi"), {}, Path("/test/project"),
                         Path("/test/project/.kimi-code/mcp.json"))
        killpg.assert_called_once_with(6821, native.signal.SIGKILL)
        child.wait.assert_called_once_with(timeout=5)

    @patch.object(native.os, "killpg")
    @patch.object(native, "start_server")
    @patch.object(native, "list_servers")
    @patch.object(native, "response")
    def test_unexpected_cleanup_exception_still_kills_child(
            self, api, catalog, startup, killpg):
        """Even a non-network cleanup exception must run termination in finally."""
        child = MagicMock(pid=6822)
        startup.return_value = ("http://127.0.0.1:12345", "inert-token", child)
        catalog.return_value = {}

        def reply(path, base, token, method="GET", data=None):
            if path == "/api/v1/workspaces":
                return {"id": "test-owned-project"}
            if path.endswith("/trust") and method == "GET":
                return {"trusted": False}
            if path.endswith("/trust") and method == "POST":
                raise TimeoutError("grant acknowledgment lost")
            if path.endswith("/untrust"):
                raise RuntimeError("unexpected cleanup failure")
            raise AssertionError("unexpected native API route")

        api.side_effect = reply
        with self.assertRaisesRegex(RuntimeError, "unexpected cleanup failure"):
            native.probe(Path("/test/kimi"), {}, Path("/test/project"),
                         Path("/test/project/.kimi-code/mcp.json"))
        killpg.assert_called_once_with(6822, native.signal.SIGKILL)
        child.wait.assert_called_once_with(timeout=5)

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
