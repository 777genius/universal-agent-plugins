"""Finite localhost OpenAI driver for genuine Kimi; never an MCP client."""
import base64
from contextlib import contextmanager
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import re
import threading

from grok_project_protocol_client import document, require
from kimi_252_native_web_mcp_probe import SERVER_NAME

LIMIT = 1 << 20
MODEL = "uap-local-driver"
TOOL = "mcp__" + SERVER_NAME + "__inspect_runtime"
CALL_ID = "uap_owned_call"


def original(body):
    require(isinstance(body, bytes) and len(body) <= LIMIT, "original byte bound")
    return {"bytes": len(body), "sha256": hashlib.sha256(body).hexdigest(),
            "base64": base64.b64encode(body).decode()}


def invocation(expected):
    return {key: expected[key] for key in ("nonce", "operation", "marker")}


def approval(frame, session, expected):
    """Approve only one observed native call, never a session-wide rule."""
    require(isinstance(frame, dict) and frame.get("type") == "interaction"
            and frame.get("status") == "pending" and frame.get("kind") == "approval"
            and frame.get("session_id") == session and frame.get("agent_id") == "main"
            and frame.get("tool_call_id") == CALL_ID, "foreign native approval identity")
    identity = frame.get("interaction_id")
    require(isinstance(identity, str) and re.fullmatch(r"approval_[0-9a-f-]{36}", identity),
            "native approval ID differs")
    request = frame.get("request", {})
    require(isinstance(request, dict), "native approval request shape differs")
    require(request.get("tool_name") == TOOL and request.get("action") == "Approve " + TOOL
            and request.get("tool_input_display") == {
                "kind": "generic", "summary": "Approve " + TOOL, "detail": invocation(expected)},
            "native approval arguments or operation differ")
    return identity


class Driver:
    def __init__(self, expected=None, reject=False):
        require(not reject or expected is not None, "rejection requires an owned invocation")
        self.expected, self.reject = expected, reject
        self.called, self.completed, self.result = False, False, None
        self.requests, self.error = [], None
        self.secret = None
        self.lock = threading.Lock()

    def respond(self, body):
        require(isinstance(body, dict) and body.get("model") == MODEL
                and body.get("stream") is True, "native provider identity/protocol differs")
        messages, tools = body.get("messages"), body.get("tools", [])
        require(isinstance(messages, list) and len(messages) <= 128
                and isinstance(tools, list) and len(tools) <= 128, "native request bounds differ")
        names, matches = set(), []
        for tool in tools:
            require(isinstance(tool, dict) and tool.get("type") == "function"
                    and isinstance(tool.get("function"), dict), "native tool declaration malformed")
            function = tool["function"]
            name = function.get("name")
            require(isinstance(name, str) and name and name not in names, "native tool identity ambiguous")
            names.add(name)
            if name == TOOL:
                matches.append(function)
        require(not self.completed, "native request after terminal result")
        if self.expected is None:
            require(not matches, "unselected source advertised owned native tool")
            self.completed = True
            return {"role": "assistant", "content": "No owned MCP source is available."}, "stop"
        require(len(matches) == 1, "owned native tool missing or duplicated")
        schema = matches[0].get("parameters", {})
        require(isinstance(schema, dict), "native owned tool parameters are not an object")
        require(matches[0].get("description") == "Inspect only disposable fixture runtime facts."
                and schema.get("type") == "object" and schema.get("required") == ["nonce"]
                and schema.get("additionalProperties") is False
                and schema.get("properties") == {key: {"type": "string"}
                                                  for key in ("nonce", "operation", "marker")},
                "native owned tool schema differs")
        if not self.called:
            self.called = True
            return {"role": "assistant", "content": None, "tool_calls": [{"id": CALL_ID,
                    "type": "function", "function": {"name": TOOL,
                    "arguments": json.dumps(invocation(self.expected), sort_keys=True)}}]}, "tool_calls"
        calls = [call for row in messages if isinstance(row, dict) and row.get("role") == "assistant"
                 for call in row.get("tool_calls", []) if isinstance(call, dict) and call.get("id") == CALL_ID]
        require(len(calls) == 1 and calls[0].get("function", {}).get("name") == TOOL,
                "native invocation replay missing or ambiguous")
        args = calls[0]["function"].get("arguments")
        require(isinstance(args, str) and document(args.encode()) == invocation(self.expected),
                "native invocation arguments/operation differ")
        results = [row for row in messages if isinstance(row, dict) and row.get("role") == "tool"
                   and row.get("tool_call_id") == CALL_ID]
        require(len(results) == 1 and isinstance(results[0].get("content"), str),
                "native tool result missing or duplicated")
        # Exact pinned contextMemory/toolResultRender.ts adds only this header.
        content = results[0]["content"]
        require(re.match(r"\AWall time: [0-9]+\.[0-9]{3} seconds\n", content),
                "native tool-result framing differs")
        payload = content.split("\n", 1)[1]
        if self.reject:
            require(payload == '<system>ERROR: Tool execution failed.</system>\nTool "' + TOOL
                    + '" was not run because the user rejected the approval request.',
                    "native refusal result differs")
        else:
            facts = document(payload.encode())
            require(isinstance(facts, dict), "native facts are not an object")
            for key in ("nonce", "revision", "marker", "cwd", "plugin_root", "plugin_data"):
                require(facts.get(key) == self.expected[key], "native result authority differs: " + key)
            require(type(facts.get("pid")) is int and facts["pid"] > 0
                    and isinstance(facts.get("session"), str) and facts["session"],
                    "native result process identity absent")
            self.result = facts
        self.completed = True
        return {"role": "assistant", "content": "Owned native MCP receipt verified."}, "stop"


def admit(driver):
    """Call only after server shutdown/join; every request must be accounted for."""
    require(driver.error is None and driver.completed, "provider refused or incomplete")
    require(len(driver.requests) == (1 if driver.expected is None else 2),
            "native provider request count differs")


@contextmanager
def provider(expected=None, reject=False):
    driver = Driver(expected, reject)

    class Handler(BaseHTTPRequestHandler):
        def setup(self):
            super().setup()
            self.connection.settimeout(5)

        def log_message(self, *_args):
            pass

        def serve(self):
            record = {"method": self.command[:16], "path": self.path[:256]}
            with driver.lock:
                driver.requests.append(record)
            try:
                require(len(driver.requests) <= 3, "provider request count exceeded")
                require(self.client_address[0] == "127.0.0.1"
                        and self.headers.get("Host") == f"127.0.0.1:{self.server.server_port}",
                        "provider origin differs")
                require(self.command == "POST" and self.path == "/v1/chat/completions"
                        and not self.headers.get("Transfer-Encoding"), "unknown provider route/protocol")
                require(self.headers.get("Content-Type", "").split(";", 1)[0] == "application/json",
                        "provider content type differs")
                length = self.headers.get("Content-Length", "")
                require(length.isascii() and length.isdigit() and 0 < int(length) <= LIMIT,
                        "provider request size refused")
                raw = self.rfile.read(int(length))
                require(len(raw) == int(length), "truncated provider request")
                require(driver.secret is None or driver.secret.encode() not in raw,
                        "private native bearer reached provider payload")
                record["request"] = original(raw)
                with driver.lock:
                    require(driver.error is None, "provider already refused a request")
                    message, reason = driver.respond(document(raw))
                record["finish_reason"] = reason
                if "tool_calls" in message:
                    message["tool_calls"] = [dict(call, index=index)
                                             for index, call in enumerate(message["tool_calls"])]
                wire = bytearray()
                for delta, finish in ((message, None), ({}, reason)):
                    chunk = {"id": "owned-local", "object": "chat.completion.chunk", "created": 0,
                             "model": MODEL, "choices": [{"index": 0, "delta": delta,
                             "finish_reason": finish}]}
                    wire.extend(b"data: " + json.dumps(chunk).encode() + b"\n\n")
                wire.extend(b"data: [DONE]\n\n")
                record["response"] = original(bytes(wire))
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Connection", "close")
                self.end_headers()
                self.wfile.write(wire)
                self.wfile.flush()
            except (ValueError, OSError, TimeoutError) as error:
                driver.error = type(error).__name__ + ": " + str(error)
                record["refused"] = True
                self.close_connection = True
                try:
                    self.send_error(400, "owned provider admission refused")
                except OSError:
                    pass

        do_POST = do_GET = do_PUT = do_PATCH = do_DELETE = do_CONNECT = do_HEAD = do_OPTIONS = serve

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = False
    thread = threading.Thread(target=lambda: server.serve_forever(poll_interval=.05), daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}/v1", driver
    finally:
        server.shutdown()
        server.server_close()
        thread.join(2)
        require(not thread.is_alive(), "owned provider survived cleanup")
