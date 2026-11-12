"""Inert protocol/process contracts; these tests are never native Hermes proof."""
import ast
import base64
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import runpy
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch
import urllib.error
import urllib.request

from hermes_local_provider_fixture import MODEL, TOOL, MAX_BODY, ScriptedHermesProvider, result_token, strict_json

ROOT = Path(__file__).resolve().parents[1]
NONCE = "a" * 64


def probe_class():
    """Load the actual stdlib-only fixture class without importing the Pi runner."""
    source = ast.parse((ROOT / "scripts/pi_project_components_native_e2e.py").read_text())
    node = next(item for item in source.body if isinstance(item, ast.ClassDef) and item.name == "PiHTTPMCPProbe")
    namespace = {"json": json, "re": re, "threading": threading,
                 "BaseHTTPRequestHandler": BaseHTTPRequestHandler, "ThreadingHTTPServer": ThreadingHTTPServer}
    exec(compile(ast.Module(body=[node], type_ignores=[]), "actual PiHTTPMCPProbe", "exec"), namespace)
    return namespace["PiHTTPMCPProbe"]


def post(url, value):
    request = urllib.request.Request(url, json.dumps(value).encode(), headers={"Content-Type": "application/json"})
    with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(request, timeout=3) as response:
        return response.read()


def request(stream=False):
    return {"model": MODEL, "stream": stream, "messages": [{"role": "user", "content": "inert query " + NONCE}],
            "tools": [{"type": "function", "function": {"name": TOOL, "parameters": {"type": "object"}}}]}


class ProviderContracts(unittest.TestCase):
    def test_actual_http_json_and_stream_round_trip(self):
        for stream in (False, True):
            with self.subTest(stream=stream), ScriptedHermesProvider(NONCE, "user-add") as provider:
                first = request(stream)
                body = post(provider.base_url + "/chat/completions", first)
                if stream:
                    chunks = [json.loads(line[6:]) for line in body.decode().splitlines() if line.startswith("data: {")]
                    message = chunks[0]["choices"][0]["delta"]
                    self.assertTrue(body.endswith(b"data: [DONE]\n\n"))
                else:
                    message = json.loads(body)["choices"][0]["message"]
                self.assertEqual(message["tool_calls"][0]["function"]["name"], TOOL)
                second = request(stream)
                second["messages"] += [message, {"role": "tool", "tool_call_id": provider.call_id,
                    "content": json.dumps({"result": result_token(NONCE, "user-add")})}]
                final = post(provider.base_url + "/chat/completions", second)
                self.assertIn(provider.final_token.encode(), final)
                receipt = provider.receipt()
                self.assertEqual((receipt["request_count"], receipt["completed"], receipt["invalid"]), (2, True, 0))
                with self.assertRaises(urllib.error.HTTPError):
                    post(provider.base_url + "/chat/completions", second)
                self.assertEqual(provider.receipt()["invalid"], 1)
                provider.close()
                receipt = provider.receipt()
                self.assertTrue(receipt["closed"])
                self.assertEqual((receipt["exchange_count"], len(receipt["exchanges"]), receipt["active_requests"]), (3, 3, 0))
                self.assertEqual([x["status"] for x in receipt["exchanges"]], [200, 200, 400])
                for exchange in receipt["exchanges"]:
                    for direction in ("request", "response"):
                        original = exchange[direction]
                        body = base64.b64decode(original["base64"], validate=True)
                        self.assertEqual((len(body), hashlib.sha256(body).hexdigest()), (original["bytes"], original["sha256"]))

    def test_native_tool_and_result_identity_are_required(self):
        for kind in ("absent-tool", "duplicate-tool", "foreign-tool", "wrong-model", "missing-query-nonce"):
            with self.subTest(kind=kind), ScriptedHermesProvider(NONCE, "user-add") as provider:
                value = request()
                if kind == "absent-tool": value["tools"] = []
                elif kind == "duplicate-tool": value["tools"] *= 2
                elif kind == "foreign-tool": value["tools"][0]["function"]["name"] = "terminal"
                elif kind == "wrong-model": value["model"] = "vendor/model"
                else: value["messages"][0]["content"] = "unbound query"
                with self.assertRaises(ValueError): provider.advance(value)
        for kind in ("wrong-call-id", "wrong-result", "prefix-result", "extra-result-field", "missing-result", "duplicate-result", "wrong-arguments"):
            with self.subTest(kind=kind), ScriptedHermesProvider(NONCE, "user-add") as provider:
                first = request()
                message = provider.advance(first)["choices"][0]["message"]
                result = {"role": "tool", "tool_call_id": provider.call_id, "content": json.dumps({"result": result_token(NONCE, "user-add")})}
                second = request()
                second["messages"] += [message, result]
                if kind == "wrong-call-id": result["tool_call_id"] = "foreign"
                elif kind == "wrong-result": result["content"] = "unbound result"
                elif kind == "prefix-result": result["content"] = "pretend " + result["content"]
                elif kind == "extra-result-field": result["content"] = json.dumps({"result": result_token(NONCE, "user-add"), "error": "failed"})
                elif kind == "missing-result": second["messages"].pop()
                elif kind == "duplicate-result": second["messages"].append(dict(result))
                else: message["tool_calls"][0]["function"]["arguments"] = '{"nonce":"wrong"}'
                with self.assertRaises(ValueError): provider.advance(second)

    def test_http_request_bounds_routes_and_duplicate_json(self):
        with ScriptedHermesProvider(NONCE, "user-add") as provider:
            for path, data, headers in (("/other", b"{}", {}), ("/chat/completions", b'{}', {"Content-Length": str(MAX_BODY + 1)}),
                                        ("/chat/completions", b'{"model":1,"model":2}', {}),
                                        ("/chat/completions", b'{}', {"Authorization": "Bearer unexpected-inert-value"})):
                with self.subTest(path=path, headers=headers):
                    req = urllib.request.Request(provider.base_url + path, data, headers=headers)
                    with self.assertRaises(urllib.error.HTTPError):
                        urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req, timeout=3)
            self.assertEqual(provider.receipt()["invalid"], 4)
        for value in ('{"x":1,"x":2}', '{"x":NaN}'):
            with self.assertRaises(ValueError): strict_json(value)

    def test_default_shared_fixture_wire_is_exact_unchanged_golden(self):
        Probe = probe_class()
        goldens = [
            ("initialize", {}, b'{"jsonrpc": "2.0", "id": 1, "result": {"protocolVersion": "2025-11-25", "capabilities": {"tools": {}}, "serverInfo": {"name": "pi-http-probe", "version": "1.0.0"}}}'),
            ("tools/list", {}, b'{"jsonrpc": "2.0", "id": 1, "result": {"tools": [{"name": "ping", "description": "Inert HTTP proof", "inputSchema": {"type": "object", "properties": {}}}]}}'),
            ("tools/call", {"name": "ping"}, b'{"jsonrpc": "2.0", "id": 1, "result": {"content": [{"type": "text", "text": "pi-http-e2e"}]}}')]
        with Probe() as probe:
            for method, params, expected in goldens:
                self.assertEqual(post(probe.url, {"jsonrpc": "2.0", "id": 1, "method": method, "params": params}), expected)
            self.assertEqual(probe.methods(), ("initialize", "tools/list", "tools/call"))
            self.assertEqual(probe.call_records(), ([], 0))

    def test_optional_mcp_nonce_captures_real_http_envelope_and_rejects_mismatch(self):
        with probe_class()(nonce=NONCE) as probe:
            packet = {"jsonrpc": "2.0", "id": 73, "method": "tools/call", "params": {
                "name": "ping", "arguments": {"nonce": NONCE, "stage": "user-repair"}}}
            reply = json.loads(post(probe.url, packet))
            self.assertEqual(reply["result"]["content"][0]["text"], result_token(NONCE, "user-repair"))
            calls, invalid = probe.call_records()
            self.assertEqual(invalid, 0)
            self.assertEqual(calls[0]["id"], 73)
            calls[0]["arguments"]["nonce"] = "foreign"
            self.assertEqual(probe.call_records()[0][0]["arguments"]["nonce"], NONCE)
            packet["params"]["arguments"]["nonce"] = "foreign"
            with self.assertRaises(urllib.error.HTTPError): post(probe.url, packet)
            self.assertEqual(probe.call_records()[1], 1)
            for params in ({"name": "foreign"}, {"name": "ping", "arguments": {"nonce": NONCE, "stage": []}}, []):
                packet["params"] = params
                with self.assertRaises(urllib.error.HTTPError): post(probe.url, packet)
            self.assertEqual(probe.call_records()[1], 4)
            self.assertEqual(probe.methods().count("tools/call"), 5)

    def test_capture_is_bounded_and_keeps_original_stream_hashes(self):
        driver = runpy.run_path(str(ROOT / "scripts/e2e/hermes-noauth-tool-call.py"), run_name="inert_driver_contracts")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            output = root / "capture"
            process_env = {key: os.environ[key] for key in ("SystemRoot", "WINDIR") if key in os.environ}
            self.assertEqual(driver["bounded"]([sys.executable, "-I", "-c", "import sys;sys.stdout.buffer.write(b'inert\\n')"], root, process_env, output), b"inert\n")
            with self.assertRaises(RuntimeError):
                driver["bounded"]([sys.executable, "-I", "-c", "import sys;sys.stdout.write('x'*(1048576+1))"], root, process_env, output)
            self.assertLessEqual(output.with_suffix(".stdout").stat().st_size, 1048576)
            with patch.dict(os.environ, {"HERMES_BIN": "/inert/hermes", "UAP_BIN": "/inert/uap", "RUNNER_TEMP": temporary,
                                        "HERMES_NATIVE_RUNTIME_COMMAND_FILE": "/inert/runtime.json", "OPENAI_API_KEY": "inert-do-not-inherit"}):
                env = driver["closed_environment"](root / "home", root, NONCE, "http://127.0.0.1:9/mcp")
                self.assertNotIn("OPENAI_API_KEY", env)
                self.assertEqual(env["HERMES_HOME"], str(root / "home/selected-profile"))


if __name__ == "__main__":
    unittest.main()
