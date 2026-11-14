#!/usr/bin/env python3
"""Observe native Kimi 2.5.2 MCP over its authenticated, loopback-only API.

Only an isolated CI-owned project is trusted through the documented native
workspace-trust endpoint; trust is explicitly revoked. No vendor login,
model, --dangerous-bypass-auth, or trust environment override is used.
"""
import http.client
import json
import os
from pathlib import Path
import stat
import signal
import socket
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request

SERVER_NAME = "uap-kimi-stable-mcp-proof"
MAX_CAPTURE = 1 << 20
def native_private_token(home):
    """Only read the test-owned Kimi home-issued bearer; never print its bytes."""
    target = home / "server.token"
    try:
        entry = target.lstat()
    except FileNotFoundError:
        return None
    if (not stat.S_ISREG(entry.st_mode) or entry.st_nlink != 1
            or entry.st_uid != os.getuid() or entry.st_mode & 0o077
            or entry.st_size < 16 or entry.st_size > 128):
        raise ValueError("native bearer file permissions or identity invalid")
    if not hasattr(os, "O_NOFOLLOW"):
        raise ValueError("native bearer requires no-follow descriptor")
    fd = os.open(target, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        held = os.fstat(fd)
        if (held.st_ino, held.st_dev, held.st_size, held.st_ctime_ns) != (
                entry.st_ino, entry.st_dev, entry.st_size, entry.st_ctime_ns):
            raise ValueError("native bearer file changed while opened")
        raw = os.read(fd, 129)
        after = os.fstat(fd)
        if (after.st_ino, after.st_dev, after.st_size, after.st_ctime_ns) != (
                entry.st_ino, entry.st_dev, entry.st_size, entry.st_ctime_ns):
            raise ValueError("native bearer changed during read")
    finally:
        os.close(fd)
    if len(raw) != entry.st_size:
        raise ValueError("native bearer short read")
    try:
        token = raw.decode("ascii").strip()
    except UnicodeDecodeError as exc:
        raise ValueError("native bearer encoding rejected") from exc
    if len(token) < 16 or len(token) > 128 or not all(
        char.isalnum() or char in "-_" for char in token
    ):
        raise ValueError("native bearer shape rejected")
    return token



class RefuseRedirect(urllib.request.HTTPRedirectHandler):
    """A native bearer must never leave the exact loopback server."""
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        raise ValueError("native loopback API redirect refused")


def local_opener():
    # The runner may supply HTTP_PROXY; never forward the native bearer to it.
    return urllib.request.build_opener(urllib.request.ProxyHandler({}), RefuseRedirect())


def response(req, base, token, method="GET", data=None):
    origin = urllib.parse.urlsplit(base)
    if (origin.scheme != "http" or origin.hostname != "127.0.0.1"
            or origin.port is None or origin.username is not None
            or origin.password is not None or origin.path or origin.query
            or origin.fragment):
        raise ValueError("native management endpoint must be exact loopback origin")
    body = None if data is None else json.dumps(data).encode("utf-8")
    request = urllib.request.Request(base + req, data=body, method=method,
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"})
    with local_opener().open(request, timeout=7) as reply:
        raw = reply.read(MAX_CAPTURE + 1)
        if len(raw) > MAX_CAPTURE:
            raise ValueError("oversized native API response")
    result = json.loads(raw)
    if (not isinstance(result, dict) or type(result.get("code")) is not int
            or result["code"] != 0):
        raise ValueError("native API request refused: " + req)
    return result.get("data")


def list_servers(base, token, project):
    query = urllib.parse.urlencode({"cwd": str(project)})
    servers = response("/api/v2/mcp/servers?" + query, base, token)
    if not isinstance(servers, list):
        raise ValueError("native MCP catalog is not a list")
    catalog = {}
    for row in servers:
        if not isinstance(row, dict) or not isinstance(row.get("name"), str) or not row["name"]:
            raise ValueError("native MCP catalog entry invalid")
        if row["name"] in catalog:
            raise ValueError("native MCP catalog contains duplicate runtime name")
        catalog[row["name"]] = row
    return catalog


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
    # No captured stdout: the native banner includes the private bearer and
    # should never reach CI logs/artifacts or an unbounded temporary file.
    process = subprocess.Popen([str(client), "web", "--no-open", "--host", "127.0.0.1", "--port", str(port)],
        cwd=project, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        stdin=subprocess.DEVNULL, start_new_session=True)
    try:
        deadline = time.monotonic() + timeout
        base = f"http://127.0.0.1:{port}"
        while time.monotonic() < deadline:
            if process.poll() is not None:
                raise ValueError("native web process exited before authenticated startup")
            # The real Kimi 2.5.2 server writes a private home-wide bearer.
            # The isolated test's own auth state is read strictly read-only.
            token = native_private_token(Path(env["KIMI_CODE_HOME"]))
            if token:
                try:
                    result = response("/api/v1/meta", base, token)
                    if isinstance(result, dict):
                        return base, token, process
                except (urllib.error.URLError, TimeoutError, ValueError):
                    pass
            time.sleep(.3)
        raise ValueError("native authenticated management startup timed out (token-file="
                         + ("present" if native_private_token(Path(env["KIMI_CODE_HOME"])) else "absent")
                         + ")")
    except BaseException:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
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
        # A grant can take effect even if its response is lost. Mark it pending
        # before issuing the request so every ambiguous outcome attempts revoke.
        trusted = True
        state = response(trust_path + "/trust", base, token, "POST", {})
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
        # The mutation response is not an independent read of persisted trust.
        # Require both fresh native trust state and the effective MCP catalog.
        if (state != {"trusted": False}
                or response(trust_path + "/trust", base, token) != {"trusted": False}
                or SERVER_NAME in list_servers(base, token, project)):
            raise ValueError("native project trust revocation did not suppress MCP")
        # Only clear the pending cleanup after the native state and catalog
        # both prove revocation. A failed/ambiguous revoke is retried in finally.
        trusted = False
        return {"native_mcp_started": True, "tools_list": True, "tools_call": False,
                "authentication": "server-issued-loopback-bearer-only",
                "workspace_trust": "native-API-granted-and-revoked-test-only",
                "native_client": "kimi-code", "native_version": "2.5.2"}
    finally:
        try:
            if trusted and workspace and isinstance(workspace.get("id"), str):
                try:
                    response("/api/v1/workspaces/" + urllib.parse.quote(workspace["id"], safe="") + "/untrust",
                             base, token, "POST", {})
                except (OSError, ValueError, http.client.HTTPException):
                    # Any ambiguous response remains a failed qualification.
                    # Cleanup must still terminate the isolated process group.
                    pass
        finally:
            # Even unforeseen exceptions during /untrust must not strand the
            # native Kimi process or its test-only trusted workspace.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait(timeout=5)
