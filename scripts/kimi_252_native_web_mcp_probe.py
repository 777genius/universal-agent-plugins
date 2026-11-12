#!/usr/bin/env python3
"""Observe native Kimi 2.5.2 MCP over its authenticated, loopback-only API.

Only an isolated CI-owned project is trusted through the documented native
workspace-trust endpoint; trust is explicitly revoked. No vendor login,
model, --dangerous-bypass-auth, or trust environment override is used.
"""
import json
import os
from pathlib import Path
import re
import signal
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

SERVER_NAME = "uap-kimi-stable-mcp-proof"
MAX_CAPTURE = 1 << 20
TOKEN = re.compile(rb"(?:^|\n)Token:\s*([A-Za-z0-9_.~+/=-]{16,})", re.M)


def response(req, base, token, method="GET", data=None):
    if not base.startswith("http://127.0.0.1:"):
        raise ValueError("native management endpoint must be loopback")
    body = None if data is None else json.dumps(data).encode("utf-8")
    request = urllib.request.Request(base + req, data=body, method=method,
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=7) as reply:
        raw = reply.read(MAX_CAPTURE + 1)
        if len(raw) > MAX_CAPTURE:
            raise ValueError("oversized native API response")
    result = json.loads(raw)
    if not isinstance(result, dict) or result.get("code") != 0:
        raise ValueError("native API request refused: " + req)
    return result.get("data")


def list_servers(base, token, project):
    query = urllib.parse.urlencode({"cwd": str(project)})
    servers = response("/api/v2/mcp/servers?" + query, base, token)
    if not isinstance(servers, list):
        raise ValueError("native MCP catalog is not a list")
    return {row.get("name"): row for row in servers if isinstance(row, dict)}


def ensure_project_server(servers, config_path):
    row = servers.get(SERVER_NAME)
    if not isinstance(row, dict) or row.get("source") != "global" or row.get("origin") != str(config_path):
        raise ValueError("native Kimi did not attribute UAP MCP entry to the Project file")
    if row.get("mutable") is not False:
        raise ValueError("native Project server was wrongly reported mutable")


def start_server(client, env, project, timeout=45):
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    with tempfile.TemporaryFile(mode="w+b") as capture:
        process = subprocess.Popen([str(client), "web", "--no-open", "--host", "127.0.0.1", "--port", str(port)],
            cwd=project, env=env, stdout=capture, stderr=subprocess.STDOUT,
            stdin=subprocess.DEVNULL, start_new_session=True)
        try:
            deadline = time.monotonic() + timeout
            base = f"http://127.0.0.1:{port}"
            while time.monotonic() < deadline:
                if process.poll() is not None:
                    raise ValueError("native web process exited before authenticated startup")
                capture.flush()
                capture.seek(0)
                banner = capture.read(MAX_CAPTURE + 1)
                if len(banner) > MAX_CAPTURE:
                    raise ValueError("native startup output exceeded bound")
                match = TOKEN.search(banner)
                if match:
                    token = match.group(1).decode("ascii")
                    try:
                        result = response("/api/v1/meta", base, token)
                        if isinstance(result, dict):
                            return base, token, process
                    except (urllib.error.URLError, TimeoutError, ValueError):
                        pass
                time.sleep(.3)
            raise ValueError("native authenticated management startup timed out")
        except BaseException:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait(timeout=5)
            raise


def probe(client, env, project, config_path):
    """Untrusted -> native project trust -> real MCP initialize/list -> revoke."""
    base, token, process = start_server(client, env, project)
    trusted = False
    workspace = None
    try:
        if SERVER_NAME in list_servers(base, token, project):
            raise ValueError("untrusted project MCP was visible before consent")
        workspace = response("/api/v1/workspaces", base, token, "POST", {"root": str(project), "name": "UAP native E2E"})
        if not isinstance(workspace, dict) or not isinstance(workspace.get("id"), str):
            raise ValueError("native workspace registration failed")
        wid = urllib.parse.quote(workspace["id"], safe="")
        trust_path = "/api/v1/workspaces/" + wid
        state = response(trust_path + "/trust", base, token)
        if state != {"trusted": False}:
            raise ValueError("native Project was unexpectedly trusted")
        state = response(trust_path + "/trust", base, token, "POST", {})
        trusted = True
        if state != {"trusted": True}:
            raise ValueError("native workspace trust grant was not acknowledged")
        ensure_project_server(list_servers(base, token, project), config_path)
        tested = response("/api/v2/mcp/servers:test", base, token, "POST",
                          {"name": SERVER_NAME, "cwd": str(project)})
        if not isinstance(tested, dict) or tested.get("success") is not True:
            raise ValueError("real native MCP connection/list probe failed")
        if "uap_ping" not in str(tested.get("output")):
            raise ValueError("native MCP connection did not list expected tool")
        state = response(trust_path + "/untrust", base, token, "POST", {})
        trusted = False
        if state != {"trusted": False} or SERVER_NAME in list_servers(base, token, project):
            raise ValueError("native project trust revocation did not suppress MCP")
        return {"native_mcp_started": True, "tools_list": True, "tools_call": False,
                "authentication": "server-issued-loopback-bearer-only",
                "workspace_trust": "native-API-granted-and-revoked-test-only",
                "native_client": "kimi-code", "native_version": "2.5.2"}
    finally:
        if trusted and workspace and isinstance(workspace.get("id"), str):
            try:
                response("/api/v1/workspaces/" + urllib.parse.quote(workspace["id"], safe="") + "/untrust",
                         base, token, "POST", {})
            except (urllib.error.URLError, ValueError):
                pass
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait(timeout=5)
