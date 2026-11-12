"""Inert protocol/ownership contracts, never a genuine native-client PASS."""
import base64
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import subprocess
import signal
import unittest
from unittest import mock
import urllib.error
import urllib.request

import kimi252_noauth_call_e2e as journey
import kimi252_noauth_provider as native


def expected():
    return {"nonce": "owned-nonce", "marker": "owned-marker", "operation": "read", "revision": "0",
            "cwd": "/owned/package", "plugin_root": "/owned/package", "plugin_data": "/owned/data"}


def request():
    return {"model": native.MODEL, "stream": True, "messages": [{"role": "user", "content": "fixture"}],
        "tools": [{"type": "function", "function": {"name": native.TOOL,
        "description": "Inspect only disposable fixture runtime facts.", "parameters": {"type": "object",
        "required": ["nonce"], "additionalProperties": False,
        "properties": {key: {"type": "string"} for key in ("nonce", "operation", "marker")}}}}]}


def result(facts=None, reject=False):
    body = request()
    body["messages"].append({"role": "assistant", "tool_calls": [{"id": native.CALL_ID, "type": "function",
        "function": {"name": native.TOOL, "arguments": json.dumps(native.invocation(expected()))}}]})
    if reject:
        payload = '<system>ERROR: Tool execution failed.</system>\nTool "' + native.TOOL + '" was not run because the user rejected the approval request.'
    else:
        payload = json.dumps({**expected(), "pid": 123, "session": "123-owned"} if facts is None else facts)
    body["messages"].append({"role": "tool", "tool_call_id": native.CALL_ID,
                              "content": "Wall time: 0.001 seconds\n" + payload})
    return body


def approval():
    return {"type": "interaction", "status": "pending", "kind": "approval", "session_id": "session-owned",
        "agent_id": "main", "tool_call_id": native.CALL_ID,
        "interaction_id": "approval_00000000-0000-0000-0000-000000000001", "request": {
        "tool_name": native.TOOL, "action": "Approve " + native.TOOL,
        "tool_input_display": {"kind": "generic", "summary": "Approve " + native.TOOL,
                               "detail": native.invocation(expected())}}}


class ProtocolTests(unittest.TestCase):
    def test_exact_native_tool_then_replay_and_server_facts(self):
        driver = native.Driver(expected())
        message, finish = driver.respond(request())
        self.assertEqual(finish, "tool_calls")
        self.assertEqual(message["tool_calls"][0]["function"]["name"], native.TOOL)
        self.assertEqual(json.loads(message["tool_calls"][0]["function"]["arguments"]), native.invocation(expected()))
        self.assertFalse(driver.completed)
        self.assertEqual(driver.respond(result())[1], "stop")
        self.assertEqual(driver.result["pid"], 123)
        self.assertTrue(driver.completed)
        with self.assertRaisesRegex(ValueError, "after terminal"): driver.respond(result())

    def test_wrong_provider_stream_tool_schema_and_duplicate_refused(self):
        cases = []
        body = request(); body["model"] = "vendor"; cases.append(body)
        body = request(); body["stream"] = 1; cases.append(body)
        body = request(); body["tools"] *= 2; cases.append(body)
        body = request(); body["tools"][0]["function"]["name"] = "foreign"; cases.append(body)
        body = request(); body["tools"][0]["function"]["description"] = "foreign"; cases.append(body)
        for key, value in (("required", []), ("additionalProperties", True), ("properties", {})):
            body = request(); body["tools"][0]["function"]["parameters"][key] = value; cases.append(body)
        for body in cases:
            with self.subTest(body=body), self.assertRaises(ValueError): native.Driver(expected()).respond(body)

    def test_wrong_native_replay_operation_or_call_identity_refused(self):
        for change in ("operation", "nonce", "marker", "id", "name", "duplicate", "missing"):
            body = result(); call = body["messages"][-2]["tool_calls"][0]
            if change in ("operation", "nonce", "marker"):
                args = native.invocation(expected()); args[change] = "foreign"
                call["function"]["arguments"] = json.dumps(args)
            if change == "id": call["id"] = "foreign"
            if change == "name": call["function"]["name"] = "Bash"
            if change == "duplicate": body["messages"][-2]["tool_calls"] *= 2
            if change == "missing": body["messages"].pop(-2)
            driver = native.Driver(expected()); driver.respond(request())
            with self.subTest(change=change), self.assertRaises(ValueError): driver.respond(body)
            self.assertFalse(driver.completed)

    def test_wrong_result_roots_nonce_revision_marker_process_or_framing_refused(self):
        for key in ("nonce", "revision", "marker", "cwd", "plugin_root", "plugin_data", "pid", "session"):
            facts = {**expected(), "pid": 123, "session": "owned", key: False}
            driver = native.Driver(expected()); driver.respond(request())
            with self.subTest(key=key), self.assertRaises(ValueError): driver.respond(result(facts))
        body = result(); body["messages"][-1]["content"] = '{"nonce":"unframed"}'
        driver = native.Driver(expected()); driver.respond(request())
        with self.assertRaisesRegex(ValueError, "framing"): driver.respond(body)

    def test_absent_and_rejected_cases_cannot_create_positive_facts(self):
        driver = native.Driver(); body = request(); body["tools"] = []
        self.assertEqual(driver.respond(body)[1], "stop")
        self.assertFalse(driver.called); self.assertIsNone(driver.result)
        with self.assertRaises(ValueError): native.Driver().respond(request())
        driver = native.Driver(expected(), reject=True); driver.respond(request()); driver.respond(result(reject=True))
        self.assertTrue(driver.completed); self.assertIsNone(driver.result)
        driver = native.Driver(expected(), reject=True); driver.respond(request())
        with self.assertRaisesRegex(ValueError, "refusal result differs"): driver.respond(result())

    def test_exact_one_shot_approval_authority(self):
        self.assertEqual(native.approval(approval(), "session-owned", expected()), approval()["interaction_id"])
        for key in ("session_id", "agent_id", "tool_call_id", "kind", "status", "interaction_id"):
            frame = approval(); frame[key] = "foreign"
            with self.subTest(key=key), self.assertRaises(ValueError): native.approval(frame, "session-owned", expected())
        for key in ("nonce", "operation", "marker"):
            frame = approval(); frame["request"]["tool_input_display"]["detail"][key] = "foreign"
            with self.subTest(key=key), self.assertRaises(ValueError): native.approval(frame, "session-owned", expected())
        frame = approval(); frame["request"]["tool_name"] = "Bash"
        with self.assertRaises(ValueError): native.approval(frame, "session-owned", expected())

    def test_actual_http_stream_original_joins(self):
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with native.provider(expected()) as (url, driver):
            for body in (request(), result()):
                req = urllib.request.Request(url + "/chat/completions", data=json.dumps(body).encode(),
                                             headers={"Content-Type": "application/json"})
                with opener.open(req, timeout=3) as response: wire = response.read(native.LIMIT)
                self.assertTrue(wire.endswith(b"data: [DONE]\n\n"))
            self.assertTrue(driver.completed); self.assertIsNone(driver.error)
            self.assertEqual(len(driver.requests), 2)
            for row in driver.requests:
                for key in ("request", "response"):
                    raw = base64.b64decode(row[key]["base64"], validate=True)
                    self.assertEqual(hashlib.sha256(raw).hexdigest(), row[key]["sha256"])
                    self.assertEqual(len(raw), row[key]["bytes"])

    def test_rejected_original_json_schema_routes_and_bearer_are_fail_closed(self):
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        wrong = request(); wrong["model"] = "foreign"
        for path, raw in (("/chat/completions", b'{"model":'),
                          ("/chat/completions", json.dumps(wrong).encode()),
                          ("/responses", b'{}')):
            with self.subTest(path=path), native.provider(expected()) as (url, driver):
                req = urllib.request.Request(url + path, data=raw, headers={"Content-Type": "application/json"})
                with self.assertRaises(urllib.error.HTTPError): opener.open(req, timeout=3)
                self.assertIsNotNone(driver.error); self.assertFalse(driver.completed)
                if path.endswith("completions"):
                    self.assertEqual(base64.b64decode(driver.requests[0]["request"]["base64"]), raw)
        with native.provider(expected()) as (url, driver):
            driver.secret = "test-owned-private-bearer"
            raw = json.dumps({**request(), "private": driver.secret}).encode()
            req = urllib.request.Request(url + "/chat/completions", data=raw, headers={"Content-Type": "application/json"})
            with self.assertRaises(urllib.error.HTTPError): opener.open(req, timeout=3)
            self.assertNotIn("request", driver.requests[0]); self.assertFalse(driver.completed)

    def test_actual_http_malformed_schema_then_valid_retry_stays_failed(self):
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        for invalid in (None, [], "foreign"):
            with self.subTest(invalid=invalid), native.provider(expected()) as (url, driver):
                bad = request(); bad["tools"][0]["function"]["parameters"] = invalid
                for body in (bad, request(), result()):
                    req = urllib.request.Request(url + "/chat/completions", data=json.dumps(body).encode(),
                        headers={"Content-Type": "application/json"})
                    with self.assertRaises(urllib.error.HTTPError): opener.open(req, timeout=3)
                self.assertIsNotNone(driver.error); self.assertFalse(driver.completed)
                self.assertFalse(driver.called)
            with self.assertRaisesRegex(ValueError, "provider refused"): native.admit(driver)
        driver = native.Driver(expected()); driver.completed = True
        driver.requests = [{}, {}, {}]
        with self.assertRaisesRegex(ValueError, "request count differs"): native.admit(driver)

    def test_absent_native_stage_grants_trust_and_refuses_surviving_owned_catalog(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            evidence = {"commands": [], "native_sessions": []}
            obj = journey.Journey(root, root / "installer", root / "probe", root / "kimi", root / "node", evidence)
            process = subprocess.Popen(["/usr/bin/python3", "-c", "import time;time.sleep(60)"],
                stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
            calls = []
            def response(path, _base, _token, method="GET", data=None):
                calls.append((path, method))
                if path == "/api/v1/workspaces": return {"id": "owned"}
                return {"trusted": method == "POST" and path.endswith("/trust")}
            try:
                with mock.patch.object(journey.native, "start_server", return_value=("http://127.0.0.1:1234", "owned-private-token", process)), \
                     mock.patch.object(journey.native, "response", side_effect=response), \
                     mock.patch.object(journey.native, "list_servers", side_effect=[{}, {journey.NAME: {}}, {}]):
                    with self.assertRaisesRegex(ValueError, "trusted absent source advertised"):
                        obj.session("trusted-removed-negative", project=obj.b)
                self.assertIn(("/api/v1/workspaces/owned/trust", "POST"), calls)
                self.assertIn(("/api/v1/workspaces/owned/untrust", "POST"), calls)
                self.assertEqual(calls[-1], ("/api/v1/workspaces/owned/trust", "GET"))
                row = evidence["native_sessions"][0]
                self.assertTrue(row["trust_granted"]); self.assertTrue(row["trust_revoked"])
                self.assertFalse(row["passed"]); self.assertFalse(row["native_project_config"]["present"])
                self.assertEqual(process.poll(), -signal.SIGKILL)
            finally:
                journey.kill(process)

    def test_real_process_groups_are_reaped_even_when_event_reader_close_fails(self):
        class BrokenReader:
            def close(self):
                raise ValueError("owned reader close failure")
        children = [subprocess.Popen(["/usr/bin/python3", "-c", "import time;time.sleep(60)"],
                    stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                    start_new_session=True) for _ in range(2)]
        try:
            with self.assertRaisesRegex(ValueError, "reader close failure"):
                journey.stop_runtime(BrokenReader(), *children)
            self.assertEqual([child.poll() for child in children], [-signal.SIGKILL, -signal.SIGKILL])
        finally:
            for child in children: journey.kill(child)

    def test_config_mutation_refused_before_replacement_and_marker_metadata_observable(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            obj = journey.Journey(root, root / "installer", root / "probe", root / "bin/kimi", root / "node",
                                  {"commands": [], "native_sessions": []})
            obj.configure("http://127.0.0.1:1234/v1")
            config = obj.home / ".kimi-code/config.toml"
            config.write_text('foreign=true\n'); held = config.read_bytes()
            with self.assertRaisesRegex(ValueError, "protected"): obj.configure("http://127.0.0.1:5678/v1")
            self.assertEqual(config.read_bytes(), held)
            marker = root / "marker"; marker.write_text("owned")
            before = journey.marker_identity(marker, root, "owned")
            marker.write_text("different")
            with self.assertRaisesRegex(ValueError, "bytes differ"): journey.marker_identity(marker, root, "owned")
            self.assertEqual(before[2], 5)


if __name__ == "__main__":
    unittest.main()
