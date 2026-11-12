"""Real pinned Grok calls an installed MCP tool via a deterministic local driver."""
import argparse
import base64
from contextlib import contextmanager
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import platform
import stat
import subprocess
import tempfile
import threading

import grok_project_protocol_e2e as existing
from grok_project_protocol_client import installed_server, require, selected_installation

MODEL = "uap-local-driver"
TOOL = "uap_probe__inspect_runtime"
LIMIT = 1 << 20


def result_facts(content):
    """Decode native tool result content, never infer execution from prose."""
    if isinstance(content, list):
        require(len(content) == 1 and isinstance(content[0], dict)
                and content[0].get("type") == "text", "native tool content shape differs")
        content = content[0].get("text")
    require(isinstance(content, str) and len(content) <= LIMIT, "native tool result bound")
    value = existing.document(content.encode("utf-8"))
    if isinstance(value, dict) and set(value) >= {"content", "isError"}:
        require(value["isError"] is False, "native MCP returned an error")
        return result_facts(value["content"])
    require(isinstance(value, dict), "native tool facts are not an object")
    return value


class Driver:
    def __init__(self, expected=None):
        self.expected = expected
        self.records, self.error, self.requests = [], None, 0
        self.called, self.completed, self.searched = False, False, False
        self.auxiliary = 0
        self.primary_requests = 0
        self.result = None
        self.invocation = None
        self.call_id = "uap_owned_call"
        self.search_id = "uap_owned_search"
        self.lock = threading.Lock()

    def call(self, name, arguments, identity):
        return {"role": "assistant", "content": None, "tool_calls": [{
            "id": identity, "type": "function", "function": {
                "name": name, "arguments": json.dumps(arguments, sort_keys=True)}}]}, "tool_calls"

    def receipt(self, messages, identity):
        rows = [row for row in messages if isinstance(row, dict)
                and row.get("role") == "tool" and row.get("tool_call_id") == identity]
        require(len(rows) == 1, "native result for exact requested call missing or duplicated")
        return result_facts(rows[0].get("content"))

    def respond(self, request):
        require(isinstance(request, dict) and request.get("model") == MODEL,
                "native custom model identity differs")
        require(type(request.get("stream", False)) is bool, "stream flag is not boolean")
        messages, tools = request.get("messages"), request.get("tools", [])
        require(isinstance(messages, list) and len(messages) <= 128
                and isinstance(tools, list) and len(tools) <= 256, "native provider shape differs")
        functions = {}
        for tool in tools:
            require(isinstance(tool, dict) and tool.get("type") == "function", "malformed advertised tool")
            function = tool.get("function", {})
            require(isinstance(function, dict) and isinstance(function.get("name"), str),
                    "malformed advertised function")
            require(function["name"] not in functions, "duplicate advertised native tool")
            functions[function["name"]] = function
        # Native session title and summary requests never drive or qualify MCP.
        if not functions or set(functions) == {"session_title"}:
            self.auxiliary += 1
            require(self.auxiliary <= 3, "auxiliary request count exceeded")
            return {"role": "assistant", "content": "Owned disposable protocol test."}, "stop"
        require("search_tool" in functions and "use_tool" in functions and TOOL not in functions,
                "documented deferred MCP wrappers differ")
        for name, required in (("search_tool", ["query"]), ("use_tool", ["tool_name", "tool_input"])):
            parameters = functions[name].get("parameters", {})
            require(isinstance(parameters, dict) and parameters.get("type") == "object"
                    and parameters.get("required") == required,
                    "native wrapper schema differs")
        require(not self.completed, "unexpected provider request after terminal result")
        self.primary_requests += 1
        if not self.searched:
            self.searched = True
            return self.call("search_tool", {"query": TOOL, "limit": 5}, self.search_id)
        if not self.called:
            discovery = self.receipt(messages, self.search_id)
            require(discovery.get("status") == "ready" and isinstance(discovery.get("results"), list),
                    "native discovery is not complete")
            matches = []
            for group in discovery["results"]:
                require(isinstance(group, dict) and isinstance(group.get("tools"), list),
                        "native discovery group differs")
                for tool in group["tools"]:
                    require(isinstance(tool, dict), "malformed discovered tool")
                    if tool.get("tool_name") == TOOL:
                        require(group.get("server") == existing.SERVER, "discovered server differs")
                        matches.append(tool)
            if self.expected is None:
                require(not matches, "unselected project discovered owned tool")
                self.completed = True
                return {"role": "assistant", "content": "No owned fixture tool is available."}, "stop"
            require(len(matches) == 1, "exact installed native tool missing or duplicated")
            tool = matches[0]
            require(tool.get("description") == "Inspect only disposable fixture runtime facts.",
                    "discovered native tool description differs")
            schema = tool.get("input_schema", {})
            require(isinstance(schema, dict) and schema.get("type") == "object"
                    and schema.get("required") == ["nonce"]
                    and isinstance(schema.get("properties"), dict)
                    and set(schema["properties"]) == {"nonce", "operation", "marker"}
                    and all(schema["properties"][key] == {"type": "string"}
                            for key in ("nonce", "operation", "marker"))
                    and schema.get("additionalProperties") is False,
                    "discovered native MCP input schema differs")
            self.called = True
            return self.call("use_tool", {"tool_name": TOOL, "tool_input": {
                key: self.expected[key] for key in ("nonce", "marker", "operation")}}, self.call_id)
        invocations = [call for row in messages if isinstance(row, dict) and row.get("role") == "assistant"
                       for call in row.get("tool_calls", []) if isinstance(call, dict)
                       and call.get("id") == self.call_id]
        require(len(invocations) == 1, "native invocation replay missing or duplicated")
        function = invocations[0].get("function", {})
        require(function.get("name") == "use_tool", "native invocation wrapper differs")
        arguments = function.get("arguments")
        require(isinstance(arguments, str), "native invocation arguments differ")
        self.invocation = existing.document(arguments.encode())
        require(self.invocation == {"tool_name": TOOL, "tool_input": {
            key: self.expected[key] for key in ("nonce", "marker", "operation")}},
            "native invocation authority or operation differs")
        facts = self.receipt(messages, self.call_id)
        for key in ("nonce", "revision", "marker", "cwd", "plugin_root", "plugin_data"):
            require(facts.get(key) == self.expected[key], "native result authority differs: " + key)
        require(type(facts.get("pid")) is int and facts["pid"] > 0
                and isinstance(facts.get("session"), str) and facts["session"],
                "native result process identity missing")
        self.result = facts
        self.completed = True
        return {"role": "assistant", "content": "Owned MCP receipt verified."}, "stop"


@contextmanager
def provider(expected=None):
    driver = Driver(expected)

    class Handler(BaseHTTPRequestHandler):
        def setup(self):
            super().setup()
            self.connection.settimeout(5)

        def log_message(self, *_args):
            pass

        def handle_request(self):
            require(self.client_address[0] == "127.0.0.1"
                    and self.headers.get("Host") == f"127.0.0.1:{self.server.server_port}",
                    "provider request is not exact owned loopback")
            require(not self.headers.get("Transfer-Encoding"), "chunked provider request refused")
            with driver.lock:
                driver.requests += 1
                require(driver.requests <= 12, "provider request count exceeded")
                if self.command == "GET" and self.path == "/":
                    require(not self.headers.get("Content-Length"), "root probe body refused")
                    driver.records.append({"method": "GET", "path": "/", "expected_denial": 404})
                    self.send_error(404, "no root service")
                    return None, False
                if self.command == "GET":
                    require(self.path == "/v1/models", "unexpected provider GET route")
                    response = {"object": "list", "data": [{"id": MODEL, "object": "model",
                                "created": 0, "owned_by": "local-test-only"}]}
                    self.record = {"method": "GET", "path": self.path}
                    driver.records.append(self.record)
                    return response, False
                require(self.command == "POST" and self.path == "/v1/chat/completions",
                        "unexpected provider protocol or route")
                require(self.headers.get("Content-Type", "").split(";", 1)[0] == "application/json",
                        "provider content type differs")
                length = self.headers.get("Content-Length", "")
                require(length.isascii() and length.isdigit() and 0 < int(length) <= LIMIT,
                        "provider request size refused")
                body = self.rfile.read(int(length))
                require(len(body) == int(length), "truncated provider request")
                self.record = {"method": "POST", "path": self.path,
                    "bytes": len(body), "sha256": hashlib.sha256(body).hexdigest(),
                    "request_base64": base64.b64encode(body).decode()}
                driver.records.append(self.record)
                request = existing.document(body)
                require(driver.error is None, "provider already refused a request")
                message, finish = driver.respond(request)
                self.record.update(finish_reason=finish, requested_function=message.get(
                    "tool_calls", [{}])[0].get("function", {}).get("name"))
                return {"id": "uap-local-response", "object": "chat.completion", "created": 0,
                        "model": MODEL, "choices": [{"index": 0, "message": message,
                        "finish_reason": finish}], "usage": {"prompt_tokens": 1,
                        "completion_tokens": 1, "total_tokens": 2}}, request.get("stream", False)

        def serve(self):
            try:
                body, streaming = self.handle_request()
                if body is None:
                    return
                wire = bytearray()
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream" if streaming else "application/json")
                self.send_header("Connection", "close")
                self.end_headers()
                if streaming:
                    choice = body["choices"][0]
                    message = dict(choice["message"])
                    if "tool_calls" in message:
                        message["tool_calls"] = [dict(item, index=index)
                                                 for index, item in enumerate(message["tool_calls"])]
                    for delta, finish in ((message, None), ({}, choice["finish_reason"])):
                        chunk = {"id": body["id"], "object": "chat.completion.chunk", "created": 0,
                                 "model": MODEL, "choices": [{"index": 0, "delta": delta,
                                 "finish_reason": finish}]}
                        wire.extend(b"data: " + json.dumps(chunk).encode() + b"\n\n")
                    wire.extend(b"data: [DONE]\n\n")
                else:
                    wire.extend(json.dumps(body).encode())
                require(len(wire) <= LIMIT, "provider response size refused")
                self.record.update(response_bytes=len(wire), response_sha256=hashlib.sha256(wire).hexdigest(),
                                   response_base64=base64.b64encode(wire).decode())
                self.wfile.write(wire)
                self.wfile.flush()
            except (ValueError, OSError, TimeoutError, TypeError, AttributeError, RecursionError) as error:
                driver.records.append({"method": self.command[:16], "path": self.path[:256],
                                       "refused": True})
                driver.error = type(error).__name__ + ": " + str(error)
                self.close_connection = True
                try:
                    self.send_error(400, "local protocol admission refused")
                except OSError:
                    pass

        do_GET = do_POST = do_PUT = do_DELETE = do_PATCH = do_OPTIONS = do_HEAD = do_CONNECT = serve

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = False
    server.timeout = 1
    thread = threading.Thread(target=lambda: server.serve_forever(poll_interval=.05), daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}/v1", driver
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)
        require(not thread.is_alive(), "provider thread survived cleanup")


class NativeJourney(existing.Journey):
    def enable_fixture(self):
        super().enable_fixture()
        self.check()
        trust = self.home / ".grok/trusted_folders.toml"
        first = "[folders." + json.dumps(str(self.a)) + "]\ntrusted = true\n"
        require(existing.read_owned(trust, self.root).decode() == first,
                "unexpected test-owned trust baseline")
        # Project B needs ordinary explicit test trust too: suppression of an
        # untrusted source cannot establish cross-project absence.
        existing.write(trust, first + "[folders." + json.dumps(str(self.b)) + "]\ntrusted = true\n")
        self.protected[trust] = existing.digest(trust)

    def configure_provider(self, url):
        self.check()
        config = self.home / ".grok/config.toml"
        if not hasattr(self, "provider_base_config"):
            self.provider_base_config = config.read_text()
        original = self.provider_base_config
        controls = "telemetry = false\ntitle_refresh = false\n"
        if "[features]\n" in original:
            original = original.replace("[features]\n", "[features]\n" + controls, 1)
        else:
            original += "\n[features]\n" + controls
        # Existing test-owned trust/enablement is retained exactly. No real profile.
        added = ("\n[models]\ndefault = " + json.dumps(MODEL)
                 + "\nsession_summary = " + json.dumps(MODEL)
                 + "\nprompt_suggestion = " + json.dumps(MODEL)
                 + "\n[endpoints]\nmodels_base_url = " + json.dumps(url) + "\n"
                 "[model." + MODEL + "]\nmodel = " + json.dumps(MODEL) + "\nbase_url = "
                 + json.dumps(url) + "\napi_backend = \"chat_completions\"\n"
                 "api_key = \"uap-inert-local-driver\"\ncontext_window = 64000\n")
        existing.write(config, original + added)
        if config in self.protected:
            self.protected[config] = existing.digest(config)
        # An inert token is only a protocol field for this owned localhost endpoint.
        self.env["XAI_API_KEY"] = "uap-inert-local-driver"
        self.env.update({"HTTP_PROXY": url.rsplit("/v1", 1)[0],
                         "HTTPS_PROXY": url.rsplit("/v1", 1)[0], "NO_PROXY": "127.0.0.1,localhost"})

    def native_session(self, label, expected=None, cwd=None):
        if expected is None:
            project = cwd or self.a
            selected = existing.document(self.run(label + "-selection", [self.grok, "inspect", "--json"], cwd=project))
            require(selected.get("grokVersion") == "1.0.70" and selected.get("cwd") == str(project)
                    and selected.get("projectRoot", "").rstrip("/") == str(project)
                    and selected.get("projectTrusted") is True, "negative project selection/trust differs")
            plugins = selected.get("plugins")
            require(isinstance(plugins, list) and all(isinstance(p, dict)
                    and isinstance(p.get("name"), str) for p in plugins)
                    and not any(p.get("name", "").casefold() == existing.NAME for p in plugins),
                    "trusted negative project still advertises owned plugin")
        marker_before = None
        if expected is not None:
            marker = Path(expected["plugin_data"]) / "native-marker.txt"
            require(expected["operation"] in ("read", "write"), "unknown owned marker operation")
            if expected["operation"] == "write":
                require(not os.path.lexists(marker), "initial marker already exists")
            else:
                require(existing.read_owned(marker, self.root).decode() == expected["marker"],
                        "read marker differs before native invocation")
                marker_before = marker.stat()
        driver, session_record = None, None
        try:
            with provider(expected) as (url, driver):
                self.configure_provider(url)
                before = existing.events(self.log, self.root)
                try:
                    self.run(label, [self.grok, "-p", "Use only the owned fixture MCP tool once, then stop.",
                                    "-m", MODEL, "--permission-mode", "dontAsk",
                                    "--allow", "MCPTool(" + TOOL + ")"], cwd=cwd, timeout=60)
                finally:
                    session_record = {
                        "label": label, "requests": driver.records, "error": driver.error,
                        "completed": driver.completed, "called": driver.called,
                        "invocation": driver.invocation, "result": driver.result}
                    if expected is None:
                        session_record["trusted_negative_selection"] = selected
                    self.evidence.setdefault("native_provider_sessions", []).append(session_record)
                rows = existing.events(self.log, self.root)[len(before):]
                require(driver.error is None and driver.completed, "native local-provider protocol unqualified")
                if expected is None:
                    require(not rows and not driver.called, "absent owned source still executed MCP")
                else:
                    sessions = existing.observed_protocol(rows, self.nonce,
                                                          ("initialize", "tools/list", "tools/call"))
                    calls = [row for row in rows if row.get("method") == "tools/call"]
                    require(len(sessions) == 1 and len(calls) == 1, "native MCP session/call count differs")
                    require(driver.invocation is not None
                            and driver.invocation["tool_input"]["operation"] == expected["operation"],
                            "native marker operation differs")
                    require(existing.read_owned(marker, self.root).decode() == expected["marker"],
                            "native marker effect differs")
                    if marker_before is not None:
                        after = marker.stat()
                        require((after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns)
                                == (marker_before.st_dev, marker_before.st_ino, marker_before.st_size,
                                    marker_before.st_mtime_ns, marker_before.st_ctime_ns),
                                "native read mutated marker")
                    facts = calls[0].get("facts", {})
                    require(driver.result is not None and facts.get("pid") == driver.result.get("pid")
                            and facts.get("session") == driver.result.get("session"),
                            "native result and original server process identity differ")
                    for key in ("nonce", "revision", "marker", "cwd", "plugin_root", "plugin_data"):
                        require(facts.get(key) == expected[key], "original MCP authority differs: " + key)
                result = {"requests": driver.records, "native_original_events": rows,
                        "owned_call_requested": driver.called, "native_result_verified": driver.completed,
                        "native_result": driver.result, "native_invocation": driver.invocation,
                        "marker_effect_verified": expected is not None,
                        "auxiliary_requests": driver.auxiliary}
        finally:
            # Keep cleanup-time failures even if the native command or receipt
            # admission raised before reaching the post-join qualification.
            if driver is not None and session_record is not None:
                session_record["error"] = driver.error
        # ThreadingHTTPServer joins all workers on exit; a late refusal must
        # still invalidate the completed native exchange.
        session_record["error"] = driver.error
        require(driver.error is None and driver.completed, "late native provider failure")
        require(driver.primary_requests == (2 if expected is None else 3),
                "native primary request count differs")
        return result

    def protocol(self, stage, revision, marker, create=False):
        installation, binding, active, data = selected_installation(self.a, existing.NAME)
        self.capture_installed_config(stage, installation, binding, active)
        expected_cwd = self.expected_runtime_cwd(active, data)
        installed_server(active, data, self.root, existing.SERVER, self.nonce,
                         existing.digest(self.installer), expected_cwd)
        expected = {"nonce": self.nonce, "revision": revision, "marker": marker,
                    "operation": "write" if create else "read", "cwd": str(expected_cwd),
                    "plugin_root": str(active), "plugin_data": str(data)}
        result = self.native_session(stage + "-native-call", expected)
        self.evidence["stages"].append({"stage": stage, "binding": binding, **result})
        if stage == "installed":
            self.evidence["project_b_negative"] = self.native_session("project-b-negative", cwd=self.b)
        return installation, binding, active, data

    def execute(self):
        help_text = self.run("native-help-admission", [self.grok, "--help"], self.root).decode()
        require("--permission-mode" in help_text and "--allow" in help_text,
                "pinned CLI does not admit narrow native permissions")
        super().execute()
        self.evidence["removed_native_negative"] = self.native_session("removed-native-negative")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("installer", "probe", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    args = parser.parse_args()
    require(os.environ.get("GITHUB_ACTIONS") == "true"
            and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted"
            and platform.system() == "Linux" and platform.machine() == "x86_64",
            "disposable hosted Linux x64 required")
    expected = os.environ.get("EXPECTED_COMMIT", "")
    def git(*argv):
        return subprocess.check_output(["/usr/bin/git", *argv], cwd=existing.SOURCE,
                                       timeout=10).decode().strip()
    require(len(expected) == 40 and git("rev-parse", "HEAD") == expected
            and not git("status", "--porcelain", "--untracked-files=normal"), "clean exact source required")
    for path in (args.installer, args.probe):
        require(path.is_absolute() and path.resolve() == path and path.is_file()
                and stat.S_ISREG(path.stat().st_mode) and path.stat().st_uid == os.getuid(),
                "expected owned compiled binary")
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    evidence = {"schema_version": 1, "github_run_id": os.environ.get("GITHUB_RUN_ID"),
                "github_run_attempt": os.environ.get("GITHUB_RUN_ATTEMPT"),
                "github_event": os.environ.get("GITHUB_EVENT_NAME"),
                "platform": "Linux-x86_64", "source_commit": expected, "source_tree": git("rev-parse", "HEAD^{tree}"),
                "passed": False, "commands": [], "stages": [], "vendor_authentication_used": False,
                "actual_model_used": False, "scripted_local_provider": True,
                "grok_native_tools_call_qualified": False,
                "installer_sha256": existing.digest(args.installer), "probe_sha256": existing.digest(args.probe)}
    try:
        from project_native_research import Budget, acquire, materialize, verify_files
        from project_native_collectors.grok import ACQUISITION
        with tempfile.TemporaryDirectory(prefix="grok-native-local-", dir=os.environ["RUNNER_TEMP"]) as temp:
            root = Path(temp).resolve()
            journey = NativeJourney(root, args.installer, args.probe, evidence)
            budget = Budget(900)
            receipt, contents = acquire(ACQUISITION, budget)
            binary, identities = materialize(root / "bin", ACQUISITION, receipt, contents)
            require(binary == journey.grok, "native acquisition path differs")
            evidence["acquisition"] = receipt
            require(journey.run("version", [binary, "--version"], root)
                    == b"grok 1.0.70 (f50904eaee6c)\n", "native pin/version differs")
            try:
                journey.execute()
                verify_files(identities, budget)
            finally:
                evidence["all_original_events"] = existing.events(journey.log, root)
                if journey.log.exists():
                    original = existing.read_owned(journey.log, root)
                    evidence["events_sha256"] = hashlib.sha256(original).hexdigest()
                    evidence["events_base64"] = base64.b64encode(original).decode()
        require(git("rev-parse", "HEAD") == expected
                and git("rev-parse", "HEAD^{tree}") == evidence["source_tree"]
                and not git("status", "--porcelain", "--untracked-files=normal"), "source changed")
        require(existing.digest(args.installer) == evidence["installer_sha256"]
                and existing.digest(args.probe) == evidence["probe_sha256"], "compiled executable changed")
        evidence.update(passed=True, grok_native_tools_call_qualified=True)
    except Exception as error:
        evidence["error"] = type(error).__name__ + ": " + str(error)
        raise
    finally:
        existing.write(args.output / "evidence.json", evidence)
        print(json.dumps({"passed": evidence["passed"], "native_call": evidence["grok_native_tools_call_qualified"]}))


if __name__ == "__main__":
    main()
