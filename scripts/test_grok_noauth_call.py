"""Inert contracts for the local driver; never native Grok qualification."""
import copy
import base64
import hashlib
import json
from pathlib import Path
import tempfile
import tomllib
import unittest
import urllib.error
import urllib.request

import grok_noauth_call_e2e as native


def expected():
    return {"nonce": "owned-nonce", "revision": "0", "marker": "owned-marker",
            "operation": "read", "cwd": "/owned/project/plugin", "plugin_root": "/owned/project/plugin",
            "plugin_data": "/owned/project/data"}


def request():
    return {"model": native.MODEL, "stream": False, "messages": [{"role": "user", "content": "fixture"}],
            "tools": [{"type": "function", "function": {"name": name, "parameters": {
                "type": "object", "required": required}}} for name, required in (
                    ("search_tool", ["query"]), ("use_tool", ["tool_name", "tool_input"]))]}


def discovered(body, available=True):
    value = copy.deepcopy(body)
    tool = {"tool_name": native.TOOL, "description": "Inspect only disposable fixture runtime facts.",
            "input_schema": {"type": "object", "required": ["nonce"], "properties": {
                "nonce": {"type": "string"}, "operation": {"type": "string"},
                "marker": {"type": "string"}}, "additionalProperties": False}}
    actual = {"status": "ready", "results": [{"server": "uap_probe", "tools": [tool]}] if available else []}
    value["messages"].append({"role": "tool", "tool_call_id": "uap_owned_search", "content": json.dumps(actual)})
    return value


def result(body, facts=None):
    value = discovered(body)
    actual = {**expected(), "pid": 123, "session": "123-owned"} if facts is None else facts
    value["messages"].append({"role": "assistant", "tool_calls": [{"id": "uap_owned_call", "type": "function",
        "function": {"name": "use_tool", "arguments": json.dumps({"tool_name": native.TOOL,
            "tool_input": {key: expected()[key] for key in ("nonce", "marker", "operation")}})}}]})
    value["messages"].append({"role": "tool", "tool_call_id": "uap_owned_call",
                              "content": json.dumps(actual)})
    return value


class DriverTests(unittest.TestCase):
    def test_discovery_then_call_requires_exact_correlated_native_facts(self):
        driver = native.Driver(expected())
        message, reason = driver.respond(request())
        self.assertEqual(reason, "tool_calls")
        self.assertEqual(message["tool_calls"][0]["function"]["name"], "search_tool")
        self.assertFalse(driver.called)
        message, reason = driver.respond(discovered(request()))
        self.assertEqual(message["tool_calls"][0]["function"]["name"], "use_tool")
        self.assertEqual(json.loads(message["tool_calls"][0]["function"]["arguments"]), {
            "tool_name": native.TOOL, "tool_input": {"nonce": "owned-nonce", "marker": "owned-marker", "operation": "read"}})
        self.assertFalse(driver.completed)
        self.assertEqual(driver.respond(result(request()))[1], "stop")
        self.assertTrue(driver.completed)
        with self.assertRaisesRegex(ValueError, "after terminal"):
            driver.respond(result(request()))

    def test_wrong_model_wrapper_schema_duplicate_or_stream_refused(self):
        bad = []
        value = request(); value["model"] = "foreign"; bad.append(value)
        value = request(); value["stream"] = 1; bad.append(value)
        value = request(); value["tools"] *= 2; bad.append(value)
        value = request(); value["tools"][0]["function"]["name"] = "foreign"; bad.append(value)
        value = request(); value["tools"][0]["function"]["parameters"]["required"] = []; bad.append(value)
        for value in bad:
            with self.subTest(value=value), self.assertRaises(ValueError): native.Driver(expected()).respond(value)

    def test_wrong_nonce_root_revision_marker_and_missing_result_refused(self):
        for key in ("nonce", "revision", "marker", "cwd", "plugin_root", "plugin_data"):
            driver = native.Driver(expected()); driver.respond(request()); driver.respond(discovered(request()))
            facts = {**expected(), "pid": 123, "session": "owned", key: "foreign"}
            with self.subTest(key=key), self.assertRaisesRegex(ValueError, "authority differs"):
                driver.respond(result(request(), facts))
        driver = native.Driver(expected()); driver.respond(request())
        with self.assertRaisesRegex(ValueError, "missing or duplicated"): driver.respond(request())

    def test_absent_source_requires_empty_native_discovery(self):
        driver = native.Driver(); driver.respond(request())
        self.assertFalse(driver.completed)
        self.assertEqual(driver.respond(discovered(request(), False))[1], "stop")
        self.assertTrue(driver.completed); self.assertFalse(driver.called)
        driver = native.Driver(); driver.respond(request())
        with self.assertRaisesRegex(ValueError, "discovered owned tool"): driver.respond(discovered(request()))

    def test_auxiliary_is_not_proof_and_bad_discovery_refuses(self):
        driver = native.Driver(expected()); aux = request(); aux["tools"] = []; driver.respond(aux)
        self.assertFalse(driver.completed); self.assertFalse(driver.searched)
        for mutation in ("partial", "foreign", "duplicate", "schema", "type", "extra"):
            body = discovered(request()); row = body["messages"][-1]; value = json.loads(row["content"])
            if mutation == "partial": value["status"] = "partial"
            if mutation == "foreign": value["results"][0]["server"] = "foreign"
            if mutation == "duplicate": value["results"][0]["tools"] *= 2
            if mutation == "schema": value["results"][0]["tools"][0]["input_schema"]["required"] = []
            if mutation == "type": value["results"][0]["tools"][0]["input_schema"]["properties"]["nonce"] = {"type": "number"}
            if mutation == "extra": value["results"][0]["tools"][0]["input_schema"]["additionalProperties"] = True
            row["content"] = json.dumps(value); driver = native.Driver(expected()); driver.respond(request())
            with self.subTest(mutation=mutation), self.assertRaises(ValueError): driver.respond(body)

    def test_wrong_native_replayed_operation_is_rejected_before_result(self):
        driver = native.Driver(expected()); driver.respond(request()); driver.respond(discovered(request()))
        body = result(request())
        call = body["messages"][-2]["tool_calls"][0]
        arguments = json.loads(call["function"]["arguments"])
        arguments["tool_input"]["operation"] = "write"
        call["function"]["arguments"] = json.dumps(arguments)
        with self.assertRaisesRegex(ValueError, "operation differs"): driver.respond(body)
        self.assertFalse(driver.completed)

    def test_native_content_envelope_and_duplicate_keys(self):
        facts = {**expected(), "pid": 123, "session": "owned"}
        wrapped = json.dumps({"isError": False, "content": [{"type": "text", "text": json.dumps(facts)}]})
        self.assertEqual(native.result_facts(wrapped), facts)
        for body in ('{"nonce":"a","nonce":"b"}', '{"isError":true,"content":[]}', "a" * (native.LIMIT + 1)):
            with self.subTest(body=body[:80]), self.assertRaises(ValueError):
                native.result_facts(body)

    def test_real_loopback_json_and_streaming_protocol(self):
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        for stream in (False, True):
            with self.subTest(stream=stream), native.provider(expected()) as (url, driver):
                value = request(); value["stream"] = stream
                for index, payload in enumerate((value, discovered(value), result(value))):
                    req = urllib.request.Request(url + "/chat/completions", data=json.dumps(payload).encode(),
                                                 headers={"Content-Type": "application/json"})
                    with opener.open(req, timeout=3) as response:
                        body = response.read(native.LIMIT)
                    self.assertIn(b"Owned MCP receipt verified" if index == 2 else b"tool_calls", body)
                    if stream:
                        self.assertTrue(body.endswith(b"data: [DONE]\n\n"))
                self.assertTrue(driver.completed)
                self.assertIsNone(driver.error)
                self.assertEqual(len(driver.records), 3)
                for record in driver.records:
                    original = base64.b64decode(record["request_base64"], validate=True)
                    response = base64.b64decode(record["response_base64"], validate=True)
                    self.assertEqual(len(original), record["bytes"])
                    self.assertEqual(hashlib.sha256(original).hexdigest(), record["sha256"])
                    self.assertEqual(len(response), record["response_bytes"])
                    self.assertEqual(hashlib.sha256(response).hexdigest(), record["response_sha256"])

    def test_provider_config_preserves_fixture_trust_plugin_and_foreign_tables(self):
        with tempfile.TemporaryDirectory() as temp:
            journey = native.NativeJourney(Path(temp), Path("/usr/bin/false"), Path("/usr/bin/false"), {"commands": []})
            config = journey.home / ".grok/config.toml"
            native.existing.write(config, '[features]\nturn_summary=false\n[plugins]\nenabled=["owned"]\n')
            journey.configure_provider("http://127.0.0.1:1234/v1")
            journey.configure_provider("http://127.0.0.1:5678/v1")
            value = tomllib.loads(config.read_text())
            self.assertEqual(value["plugins"]["enabled"], ["owned"])
            self.assertIs(value["features"]["turn_summary"], False)
            self.assertEqual(value["model"][native.MODEL]["base_url"], "http://127.0.0.1:5678/v1")

    def test_config_mutation_is_refused_before_cached_config_replacement(self):
        with tempfile.TemporaryDirectory() as temp:
            journey = native.NativeJourney(Path(temp), Path("/usr/bin/false"), Path("/usr/bin/false"), {"commands": []})
            config = journey.home / ".grok/config.toml"
            native.existing.write(config, '[plugins]\nenabled=["owned"]\n')
            journey.protected[config] = native.existing.digest(config)
            journey.configure_provider("http://127.0.0.1:1234/v1")
            native.existing.write(config, 'changed=true\n')
            held = config.read_bytes()
            with self.assertRaisesRegex(ValueError, "protected fixture changed"):
                journey.configure_provider("http://127.0.0.1:5678/v1")
            self.assertEqual(config.read_bytes(), held)

    def test_exact_root_probe_is_only_recorded_404_and_never_qualifies(self):
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with native.provider(expected()) as (url, driver):
            with self.assertRaises(urllib.error.HTTPError) as error:
                opener.open(url[:-3] + "/", timeout=3)
            self.assertEqual(error.exception.code, 404)
            self.assertIsNone(driver.error)
            self.assertFalse(driver.completed)
            self.assertEqual(driver.records, [{"method": "GET", "path": "/", "expected_denial": 404}])
            with self.assertRaises(urllib.error.HTTPError): opener.open(url[:-3] + "/?foreign", timeout=3)
            self.assertIsNotNone(driver.error)

    def test_rejected_json_or_native_schema_retains_original_bounded_request(self):
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        wrong = request(); wrong["tools"][0]["function"]["parameters"]["required"] = []
        for body in (b'{"model":', json.dumps(wrong).encode()):
            with self.subTest(body=body[:40]), native.provider(expected()) as (url, driver):
                req = urllib.request.Request(url + "/chat/completions", data=body,
                    headers={"Content-Type": "application/json"})
                with self.assertRaises(urllib.error.HTTPError): opener.open(req, timeout=3)
                original = next(row for row in driver.records if "request_base64" in row)
                self.assertEqual(base64.b64decode(original["request_base64"], validate=True), body)
                self.assertEqual(original["sha256"], hashlib.sha256(body).hexdigest())
                self.assertNotIn("finish_reason", original)
                self.assertIsNotNone(driver.error)
                self.assertFalse(driver.completed)

    def test_actual_http_bad_route_size_and_duplicate_json_fail_closed(self):
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        for path, body, headers in (("/foreign", b"{}", {}),
                                   ("/chat/completions", b"x" * (native.LIMIT + 1), {}),
                                   ("/chat/completions", b'{"model":"a","model":"b"}', {})):
            with self.subTest(path=path), native.provider(expected()) as (url, driver):
                req = urllib.request.Request(url + path, data=body,
                                             headers={"Content-Type": "application/json", **headers})
                with self.assertRaises((urllib.error.HTTPError, urllib.error.URLError, ConnectionResetError)):
                    opener.open(req, timeout=3)
                self.assertIsNotNone(driver.error)
                self.assertFalse(driver.completed)


if __name__ == "__main__":
    unittest.main()
