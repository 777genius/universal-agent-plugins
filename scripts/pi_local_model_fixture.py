"""A bounded loopback model for Pi's genuine model-driven MCP tools/call E2E.

The fixture supplies a scripted OpenAI-compatible response, not an agent
simulator: only Pi may start its installed project MCP server and call it.
"""
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MODEL_ID = "uap-scripted"
CALL_ID = "uap-pi-codemode-1"
CODE = (
    'const hits = ALL_TOOLS.filter(x => x.name.startsWith("mcp__pi") && '
    'x.name.endsWith("__echo")); '
    'if (hits.length !== 1) throw new Error("exact Pi MCP tool unavailable"); '
    'const name = hits[0].name.replace(/[^A-Za-z0-9_$]/g, "_"); '
    'return await tools[name]({});'
)


class ScriptedPiModel:
    def __init__(self):
        self.lock = threading.Lock()
        self.requests = 0
        self.tool_seen = 0
        self.calls = 0
        self.results = 0
        self.invalid = 0
        outer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_args):
                pass

            def do_POST(self):
                if self.path != "/v1/chat/completions":
                    with outer.lock:
                        outer.invalid += 1
                    self.send_error(404)
                    return
                try:
                    length = int(self.headers.get("Content-Length", "0"))
                    if length <= 0 or length > 1 << 20:
                        raise ValueError("request size")
                    request = json.loads(self.rfile.read(length))
                    messages = request.get("messages", [])
                    tools = request.get("tools", [])
                    if (not isinstance(request, dict) or MODEL_ID not in str(request.get("model", "")) or
                            not isinstance(messages, list) or len(messages) > 128 or
                            not isinstance(tools, list) or len(tools) > 128):
                        raise ValueError("invalid model request")
                except (TypeError, ValueError, json.JSONDecodeError):
                    with outer.lock:
                        outer.invalid += 1
                    self.send_error(400)
                    return
                declared = any(isinstance(tool, dict) and
                               isinstance(tool.get("function"), dict) and
                               tool["function"].get("name") == "codemode" for tool in tools)
                result = any(isinstance(message, dict) and message.get("role") == "tool" and
                             message.get("tool_call_id") == CALL_ID for message in messages)
                with outer.lock:
                    outer.requests += 1
                    if declared:
                        outer.tool_seen += 1
                    call = declared and not result and outer.calls == 0
                    if call:
                        outer.calls += 1
                    if result:
                        outer.results += 1
                    if outer.requests > 8:
                        outer.invalid += 1
                        self.send_error(429)
                        return
                name = "codemode"
                delta = {"role": "assistant", "content": "Local model finished."}
                finish = "stop"
                if call:
                    delta = {"role": "assistant", "tool_calls": [{
                        "index": 0, "id": CALL_ID, "type": "function",
                        "function": {"name": name, "arguments": json.dumps({"code": CODE})}
                    }]}
                    finish = "tool_calls"
                if request.get("stream"):
                    self.send_response(200)
                    self.send_header("Content-Type", "text/event-stream")
                    self.send_header("Cache-Control", "no-cache")
                    self.end_headers()
                    for choice in ({"index": 0, "delta": delta, "finish_reason": None},
                                   {"index": 0, "delta": {}, "finish_reason": finish}):
                        event = {"id": "chatcmpl-uap-pi-fixture", "object": "chat.completion.chunk",
                                 "created": 1, "model": MODEL_ID, "choices": [choice]}
                        self.wfile.write(b"data: " + json.dumps(event).encode() + b"\n\n")
                    self.wfile.write(b"data: [DONE]\n\n")
                    self.wfile.flush()
                else:
                    message = {"role": "assistant", "content": delta.get("content")}
                    if call:
                        message["tool_calls"] = [{k: v for k, v in delta["tool_calls"][0].items() if k != "index"}]
                    event = {"id": "chatcmpl-uap-pi-fixture", "object": "chat.completion",
                             "created": 1, "model": MODEL_ID,
                             "choices": [{"index": 0, "message": message, "finish_reason": finish}],
                             "usage": {"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12}}
                    body = json.dumps(event).encode()
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    @property
    def base_url(self):
        return f"http://127.0.0.1:{self.server.server_port}/v1"

    def stats(self):
        with self.lock:
            return self.requests, self.tool_seen, self.calls, self.results, self.invalid

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=3)
