"""Test-only MCP client for an already verified, UAP-owned stdio projection.

No vendor auth, model, shell, config mutation, or environment inheritance.
"""
import json
import os
from pathlib import Path
import selectors
import signal
import stat
import subprocess
import time

LIMIT = 1 << 20


def require(value, message):
    if not value:
        raise ValueError(message)


def document(body):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "duplicate JSON key")
            result[key] = value
        return result

    def invalid_constant(_value):
        raise ValueError("nonfinite JSON number")

    require(isinstance(body, bytes) and 0 < len(body) <= LIMIT, "JSON byte bound")
    return json.loads(body.decode("utf-8"), object_pairs_hook=unique,
                      parse_constant=invalid_constant)


def read_owned(path, root):
    path, root = Path(path), Path(root)
    require(path.is_absolute() and path.is_relative_to(root) and path != root,
            "path outside owned fixture")
    require(root.resolve() == root and path.resolve() == path, "aliased fixture path")
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
            and before.st_uid == os.getuid() and before.st_size <= LIMIT,
            "nonordinary or oversized fixture file")
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        held = os.fstat(fd)
        require((held.st_dev, held.st_ino) == (before.st_dev, before.st_ino), "file replaced")
        chunks, size = [], 0
        while chunk := os.read(fd, min(65536, LIMIT + 1 - size)):
            chunks.append(chunk)
            size += len(chunk)
            require(size <= LIMIT, "fixture file grew")
        after = os.fstat(fd)
        require((held.st_size, held.st_mtime_ns, held.st_ctime_ns)
                == (after.st_size, after.st_mtime_ns, after.st_ctime_ns), "file changed")
    finally:
        os.close(fd)
    after = path.lstat()
    require(path.resolve() == path and (after.st_dev, after.st_ino)
            == (before.st_dev, before.st_ino), "fixture namespace changed")
    return b"".join(chunks)


def selected_installation(project, name):
    project = Path(project)
    state = document(read_owned(project / ".uap/state-v2.json", project))
    require(isinstance(state, dict) and state.get("schema_version") == 2, "state schema")
    installs = state.get("installations")
    require(isinstance(installs, list), "installation inventory")
    matches = [item for item in installs if item.get("declared_name") == name]
    require(len(matches) == 1, "exact selected installation missing or duplicated")
    installation = matches[0]
    clients = installation.get("clients")
    require(isinstance(clients, dict) and len(clients) == 1, "exact Grok binding required")
    binding = next(iter(clients.values()))
    require(binding.get("client_id") == "grok" and binding.get("scope") == "project"
            and binding.get("activation") == "prepared"
            and binding.get("verification") == "package_validated", "binding authority differs")
    artifact = binding.get("physical_artifact_id")
    require(isinstance(artifact, str) and artifact not in ("", ".", "..")
            and Path(artifact).name == artifact and "\\" not in artifact, "artifact leaf")
    active = project / ".grok/plugins" / artifact
    data = project / ".uap/plugin-data" / artifact
    require(binding.get("target_locator") == str(active)
            and binding.get("native_profile_root") == str(project / ".grok"), "foreign binding path")
    receipt = installation.get("data_receipts", {}).get(binding.get("data_receipt_id"), {})
    require(receipt.get("scope") == "project" and receipt.get("locator") == str(data)
            and receipt.get("physical_backend_id") == artifact and receipt.get("state") == "owned",
            "stdio data receipt differs")
    return installation, binding, active, data


def installed_server(active, data, root, name, expected_nonce):
    config = document(read_owned(Path(active) / ".mcp.json", root))
    servers = config.get("mcpServers")
    require(isinstance(servers, dict) and set(servers) == {name}, "exact installed server required")
    server = servers[name]
    require(isinstance(server, dict) and server.get("type", "stdio") == "stdio", "stdio only")
    command = Path(server.get("command", ""))
    require(command.is_absolute() and command.is_relative_to(active)
            and command.resolve() == command and command.is_file(), "foreign installed command")
    args, env = server.get("args", []), server.get("env", {})
    require(isinstance(args, list) and len(args) <= 32 and all(isinstance(x, str) for x in args),
            "invalid installed args")
    require(isinstance(env, dict) and all(isinstance(k, str) and isinstance(v, str)
                                        for k, v in env.items()), "invalid installed env")
    allowed = {"PLUGIN_ROOT", "PLUGIN_DATA", "UAP_TEST_ROOT", "UAP_TEST_EVENTS",
               "UAP_TEST_NONCE", "UAP_TEST_REVISION"}
    require(set(env) <= allowed and env.get("PLUGIN_ROOT") == str(active)
            and env.get("PLUGIN_DATA") == str(data) and env.get("UAP_TEST_ROOT") == str(root)
            and env.get("UAP_TEST_NONCE") == expected_nonce, "unexpected fixture environment")
    require(server.get("cwd") == str(active), "installed cwd differs")
    return server


class Protocol:
    """Bounded, sequential JSON-RPC over the exact installed stdio command."""
    def __init__(self, server, base_env):
        self.env = dict(base_env, **server["env"])
        self.transcript, self.buffer, self.stderr = [], b"", b""
        self.total, self.deadline = 0, time.monotonic() + 15
        self.process = subprocess.Popen([server["command"], *server.get("args", [])],
                                        cwd=server["cwd"], env=self.env,
                                        stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, start_new_session=True)
        self.selector = selectors.DefaultSelector()
        for label, stream in (("out", self.process.stdout), ("err", self.process.stderr)):
            os.set_blocking(stream.fileno(), False)
            self.selector.register(stream, selectors.EVENT_READ, label)

    def send(self, message):
        body = json.dumps(message, separators=(",", ":")).encode() + b"\n"
        require(len(body) <= 4096, "request bound")
        self.process.stdin.write(body)
        self.process.stdin.flush()
        self.transcript.append({"direction": "request", "message": message})

    def response(self, expected_id):
        while b"\n" not in self.buffer:
            remaining = self.deadline - time.monotonic()
            require(remaining > 0, "MCP response deadline")
            ready = self.selector.select(min(0.1, remaining))
            require(ready or self.process.poll() is None, "MCP exited before response")
            for key, _ in ready:
                chunk = os.read(key.fd, 65536)
                if not chunk:
                    self.selector.unregister(key.fileobj)
                    continue
                self.total += len(chunk)
                require(self.total <= LIMIT, "MCP stream bound")
                if key.data == "out":
                    self.buffer += chunk
                else:
                    self.stderr += chunk
        line, self.buffer = self.buffer.split(b"\n", 1)
        value = document(line)
        require(isinstance(value, dict) and value.get("jsonrpc") == "2.0"
                and type(value.get("id")) is type(expected_id) and value["id"] == expected_id,
                "MCP response ID/envelope mismatch")
        require(("result" in value) != ("error" in value), "MCP result/error ambiguity")
        self.transcript.append({"direction": "response", "message": value})
        return value

    def call(self, ident, method, params=None, expected_error=False):
        self.send({"jsonrpc": "2.0", "id": ident, "method": method, "params": params or {}})
        value = self.response(ident)
        require(("error" in value) == expected_error, "unexpected MCP result/error")
        return value.get("error") if expected_error else value["result"]

    def close(self):
        try:
            if not self.process.stdin.closed:
                self.process.stdin.close()
            self.process.wait(timeout=3)
            require(self.process.returncode == 0, "MCP child exited unsuccessfully")
        finally:
            try:
                os.killpg(self.process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            self.process.wait(timeout=3)
            self.selector.close()
            self.process.stdout.close()
            self.process.stderr.close()


def exercise(server, base_env, nonce, revision, marker, write):
    rpc = Protocol(server, base_env)
    try:
        init = rpc.call(1, "initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                                        "clientInfo": {"name": "uap-no-model-e2e", "version": "1"}})
        require(init.get("protocolVersion") == "2025-06-18"
                and init.get("serverInfo", {}).get("name") == "agentplugins-native-probe",
                "unexpected initialized server")
        rpc.send({"jsonrpc": "2.0", "method": "notifications/initialized"})
        tools = rpc.call(2, "tools/list").get("tools", [])
        require(len(tools) == 1 and tools[0].get("name") == "inspect_runtime", "unexpected tools")
        result = rpc.call(3, "tools/call", {"name": "inspect_runtime", "arguments": {
            "nonce": nonce, "operation": "write" if write else "read", "marker": marker}})
        require(result.get("isError") is False and len(result.get("content", [])) == 1,
                "tool result not successful")
        facts = document(result["content"][0]["text"].encode())
        for key, expected in {"nonce": nonce, "revision": revision, "marker": marker,
                              "plugin_root": server["env"]["PLUGIN_ROOT"],
                              "plugin_data": server["env"]["PLUGIN_DATA"],
                              "cwd": server["cwd"]}.items():
            require(facts.get(key) == expected, "tool result differs: " + key)
        negative = rpc.call(4, "tools/call", {"name": "inspect_runtime", "arguments": {
            "nonce": "wrong-" + nonce}}, expected_error=True)
        require(negative.get("code") == -32602, "bad nonce was not rejected")
        return {"facts": facts, "transcript": rpc.transcript,
                "caller": "direct-installed-stdio-client", "grok_model_invoked": False}
    finally:
        rpc.close()
