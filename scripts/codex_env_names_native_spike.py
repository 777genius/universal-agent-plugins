#!/usr/bin/env python3
"""Bounded REAL Codex native secrecy spike; synthetic secret only, never logs its value.

Pass requires an actual Codex shell tool call, then an MCP stdio tools/call
through name-only env_vars; inspect the entire isolated Codex/home/project tree.
This is not a product feature, and never reads real user credentials.
"""
import base64
import hashlib
import http.server
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import urllib.parse

ENV_NAME = "UAP_E2E_SYNTHETIC_TOKEN"
FOREIGN_ENV_NAME = "UAP_E2E_FOREIGN_PASSWORD"
SHELL_CALL = "call_uap_shell_snapshot"
SHELL_MARKER = "UAP_SHELL_COMPLETED"
SHELL_ENV_PRESENT = "UAP_SHELL_ENV_PRESENT"
SHELL_FOREIGN_PRESENT = "UAP_SHELL_FOREIGN_PRESENT"
MCP_CALL = "call_uap_env_mcp"
SEARCH_CALL = "call_uap_tool_search"

CHILD = r'''
import hashlib, json, os, sys
name = "UAP_E2E_SYNTHETIC_TOKEN"
for line in sys.stdin:
    try:
        q = json.loads(line)
        method = q.get("method")
        if method == "initialize":
            out = {"protocolVersion": "2025-03-26",
                   "capabilities": {"tools": {}},
                   "serverInfo": {"name": "uap-env-spike", "version": "1"}}
        elif method == "tools/list":
            out = {"tools": [{"name": "confirm_env",
                  "description": "Return inert value-presence proof, no plaintext",
                  "inputSchema": {"type": "object", "properties": {}},
                  "annotations": {"readOnlyHint": True, "destructiveHint": False,
                                  "idempotentHint": True, "openWorldHint": False}}]}
        elif method == "tools/call":
            value = os.environ.get(name)
            evidence = {"observed": bool(value),
                        "sha256": hashlib.sha256(value.encode()).hexdigest() if value else ""}
            out = {"content": [{"type": "text",
                        "text": json.dumps(evidence, sort_keys=True)}]}
        elif method == "ping":
            out = {}
        else:
            continue
        if q.get("id") is not None:
            print(json.dumps({"jsonrpc": "2.0", "id": q["id"], "result": out}), flush=True)
    except Exception:
        break
'''


def isolated_env(root, home, secret, foreign_secret):
    """Disposable Codex profile: no real runner secrets are inherited."""
    env = {"HOME": str(home), "CODEX_HOME": str(home / ".codex"),
           "TMPDIR": str(root / "tmp"), "TEMP": str(root / "tmp"),
           "TMP": str(root / "tmp"), "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
           "LANG": "C.UTF-8", "NO_COLOR": "1", ENV_NAME: secret,
           FOREIGN_ENV_NAME: foreign_secret}
    if os.name == "nt":
        # Only necessary nonsecret OS process/shell fields on Windows.
        env.update({key: os.environ[key] for key in
                    ("SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "OS")
                    if key in os.environ})
        env["USERPROFILE"] = str(home)
        env["APPDATA"] = str(home / "AppData" / "Roaming")
        env["LOCALAPPDATA"] = str(home / "AppData" / "Local")
        (home / "AppData" / "Roaming").mkdir(parents=True, exist_ok=True)
        (home / "AppData" / "Local").mkdir(parents=True, exist_ok=True)
    return env


def windows_shell_command():
    """PowerShell checks name presence only, never reads a canary value."""
    return ("Start-Sleep -Seconds 7; "
            f"if (Test-Path Env:{ENV_NAME}) {{ Write-Output '{SHELL_ENV_PRESENT}' }}; "
            f"if (Test-Path Env:{FOREIGN_ENV_NAME}) {{ Write-Output '{SHELL_FOREIGN_PRESENT}' }}; "
            f"Write-Output '{SHELL_MARKER}'")

class Model(http.server.BaseHTTPRequestHandler):
    requests = 0
    shell_requested = False
    mcp_requested = False
    search_requested = False
    mcp_output_seen = False
    mcp_digest_verified = False
    mcp_output_indicators = []
    mcp_output_type = "not_observed"
    expected_digest = None
    shell_output_seen = False
    shell_execution_verified = False
    shell_secret_visible = False
    shell_foreign_visible = False
    shell_response_category = "not_observed"
    shell_response_indicators = []
    shell_requested_at = 0.0
    error = ""
    advertised_tool_names = set()

    def log_message(self, *_args):
        pass

    def do_POST(self):
        if self.path != "/v1/responses":
            self.send_error(404)
            return
        length = int(self.headers.get("Content-Length", 0))
        if length <= 0 or length > 1 << 20:
            self.send_error(413)
            return
        try:
            req = json.loads(self.rfile.read(length))
        except ValueError:
            self.send_error(400)
            return
        cls = type(self)
        cls.requests += 1
        outputs = [x for x in req.get("input", []) if isinstance(x, dict)]
        for response in outputs:
            if response.get("type") == "function_call_output" and response.get("call_id") == SHELL_CALL:
                cls.shell_output_seen = True
                # A native tool returning an error is not proof the command ran.
                # Require an execution-only sentinel AND elapsed sleep time.
                rendered = json.dumps(response.get("output"), sort_keys=True)
                # Marker reveals only whether the shell received the name,
                # never the canary value. A successful shell with access to
                # the forwarded credential is unsafe even without a snapshot.
                cls.shell_secret_visible |= SHELL_ENV_PRESENT in rendered
                cls.shell_foreign_visible |= SHELL_FOREIGN_PRESENT in rendered
                if (SHELL_MARKER in rendered and
                        time.monotonic() - cls.shell_requested_at >= 6.0):
                    cls.shell_execution_verified = True
                    cls.shell_response_category = "completed"
                else:
                    # Only fixed categories leave this synthetic-only harness;
                    # never persist raw native tool responses or environment.
                    lower = rendered.lower()
                    # Bounded, fixed-vocabulary diagnostics. Never persist the
                    # native response, command output, paths or synthetic value.
                    indicators = (
                        "sandbox", "permission denied", "operation not permitted",
                        "read-only", "landlock", "bwrap", "seccomp", "namespace",
                        "not supported", "not found", "invalid", "unrecognized",
                        "failed", "error", "denied", "session id", "session_id",
                        "running", "yield_time_ms", "tool_call", "exec_command",
                        "expected", "arguments", "json", "timeout", "exited",
                    )
                    cls.shell_response_indicators = [
                        word for word in indicators if word in lower
                    ]
                    cls.shell_response_category = (
                        "sandbox_error" if "sandbox" in lower else
                        "permission_error" if "permission" in lower else
                        "tool_error" if "error" in lower or "failed" in lower else
                        "completion_unverified"
                    )
        for response in outputs:
            if (response.get("type") == "function_call_output"
                    and response.get("call_id") == MCP_CALL):
                cls.mcp_output_seen = True
                # SHA256 of a fresh synthetic-only canary, never real credentials.
                # Correlate the ACTUAL native tool result rather than relying
                # on a sandbox-blocked child sidecar receipt.
                output = response.get("output")
                rendered = json.dumps(output, sort_keys=True)
                cls.mcp_output_type = (
                    "text" if isinstance(output, str) else
                    "object" if isinstance(output, dict) else
                    "array" if isinstance(output, list) else
                    "other"
                )
                # Fixed vocabulary only: never record the child's output,
                # synthetic canary, digest string, process paths or argv.
                cls.mcp_output_indicators = [
                    term for term in (
                        "sha256", "observed", "unknown", "not found", "unsupported",
                        "not registered", "unavailable", "not connected", "failed",
                        "error", "permission denied", "denied", "namespace",
                        "server", "mcp", "tool", "timeout", "call", "result",
                    ) if term in rendered.lower()
                ]
                if cls.expected_digest and cls.expected_digest in rendered:
                    cls.mcp_digest_verified = True
        tools = req.get("tools", [])
        function_names = {x.get("name") for x in tools if isinstance(x, dict)}
        cls.advertised_tool_names.update(name for name in function_names if name)
        namespace = next((x for x in tools if isinstance(x, dict)
                          and x.get("type") == "namespace"
                          and x.get("name") == "mcp__env_spike"), None)
        # Versions using deferred tool discovery expose the namespace in input.
        if namespace is None:
            for output in outputs:
                if output.get("type") == "tool_search_output":
                    namespace = next((t for t in output.get("tools", [])
                                      if isinstance(t, dict)
                                      and t.get("name") == "mcp__env_spike"), None)
                    if namespace is not None:
                        break
        if cls.requests > 10:
            cls.error = "model request budget"
            self.send_error(409)
            return
        if not cls.shell_requested:
            shell = "exec_command" if "exec_command" in function_names else (
                "shell_command" if "shell_command" in function_names else None
            )
            if shell is None:
                cls.error = "no advertised shell tool"
                self.send_error(409)
                return
            command = ("sleep 7; if test -n \"${" + ENV_NAME + "+x}\"; then printf "
                       + SHELL_ENV_PRESENT + "; fi; if test -n \"${"
                       + FOREIGN_ENV_NAME + "+x}\"; then printf "
                       + SHELL_FOREIGN_PRESENT + "; fi; printf " + SHELL_MARKER)
            if os.name == "nt":
                command = windows_shell_command()
            # Codex exec_command can yield before a long command has finished.
            # Request a full bounded wait so the completion marker is observed
            # from the real native shell rather than mistaking an async
            # session-creation response for command execution.
            args = ({"cmd": command, "yield_time_ms": 10000} if shell == "exec_command"
                    else {"command": command})
            item = {"type": "function_call", "name": shell,
                    "call_id": SHELL_CALL, "arguments": json.dumps(args)}
            cls.shell_requested = True
            cls.shell_requested_at = time.monotonic()
        elif not cls.shell_output_seen:
            cls.error = "shell tool output missing"
            self.send_error(409)
            return
        elif cls.mcp_output_seen:
            item = {"type": "message", "id": "msg_uap_spike", "role": "assistant",
                    "content": [{"type": "output_text", "text": "Proof complete."}]}
        elif namespace and any(x.get("name") == "confirm_env"
                               for x in namespace.get("tools", [])):
            item = {"type": "function_call", "namespace": "mcp__env_spike",
                    "name": "confirm_env", "call_id": MCP_CALL, "arguments": "{}"}
            cls.mcp_requested = True
        elif "mcp__env_spike__confirm_env" in function_names:
            item = {"type": "function_call", "name": "mcp__env_spike__confirm_env",
                    "call_id": MCP_CALL, "arguments": "{}"}
            cls.mcp_requested = True
        elif not cls.search_requested and "tool_search" in function_names:
            item = {"type": "tool_search_call", "call_id": SEARCH_CALL,
                    "execution": "client",
                    "arguments": {"query": "env_spike confirm_env", "limit": 8}}
            cls.search_requested = True
        else:
            cls.error = "MCP tool not advertised"
            self.send_error(409)
            return
        events = [
            ("response.created", {"type": "response.created",
                                  "response": {"id": "resp_uap_spike"}}),
            ("response.output_item.done", {"type": "response.output_item.done",
                                           "item": item}),
            ("response.completed", {"type": "response.completed",
                                    "response": {"id": "resp_uap_spike",
                                                 "usage": {"input_tokens": 1,
                                                           "output_tokens": 1,
                                                           "total_tokens": 2}}}),
        ]
        body = "".join(f"event: {event}\ndata: {json.dumps(payload)}\n\n"
                       for event, payload in events).encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

def inspect(root, secret):
    values = (secret,) if isinstance(secret, str) else secret
    patterns = []
    for value in values:
        patterns.extend([value.encode(), value.encode().hex().encode(),
                         base64.b64encode(value.encode()),
                         urllib.parse.quote(value).encode()])
    found, symlinks = [], 0
    for directory, subdirs, files in os.walk(root, followlinks=False):
        for name in list(subdirs):
            if (Path(directory) / name).is_symlink():
                symlinks += 1
                subdirs.remove(name)
        for name in files:
            path = Path(directory) / name
            if path.is_symlink():
                symlinks += 1
                continue
            try:
                if path.stat().st_size > 32 * 1024 * 1024:
                    found.append({"path_sha256": hashlib.sha256(str(path.relative_to(root)).encode()).hexdigest(),
                                  "result": "unscanned_oversize"})
                    continue
                data = path.read_bytes()
            except FileNotFoundError:
                # Codex atomically replaces/deletes temporary snapshot files.
                # A vanished file is not an inaccessible persisted file.
                continue
            if any(p and p in data for p in patterns):
                found.append({"path_sha256": hashlib.sha256(str(path.relative_to(root)).encode()).hexdigest(),
                              "result": "synthetic_value_persisted",
                              "shell_snapshot_path": "shell_snapshots" in path.parts})
    return found, symlinks

def main():
    codex = Path(sys.argv[1]).resolve(strict=True)
    report_path = Path(sys.argv[2]).resolve()
    report_path.parent.mkdir(parents=True, exist_ok=True)
    mode = sys.argv[3] if len(sys.argv) > 3 else "default"
    if mode not in ("default", "filtered", "default_excludes", "core_inherit"):
        raise ValueError("unsupported native shell policy control mode")
    if len(sys.argv) > 4:
        raise ValueError("unexpected native spike parameter")
    secret = "uap_spike_" + secrets.token_hex(24)
    foreign_secret = "uap_foreign_spike_" + secrets.token_hex(24)
    Model.expected_digest = hashlib.sha256(secret.encode()).hexdigest()
    root = Path(tempfile.mkdtemp(prefix="uap-codex-spike-"))
    home, project = root / "home", root / "project"
    (home / ".codex").mkdir(parents=True)
    (project / ".codex").mkdir(parents=True)
    (project / ".git").mkdir()
    (root / "tmp").mkdir()
    child = root / "child.py"
    child.write_text(CHILD)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Model)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    config = (
        'cli_auth_credentials_store = "file"\n'
        'mcp_oauth_credentials_store = "file"\n'
        'model_provider = "fixture"\nmodel = "fixture-model"\n'
        '[model_providers.fixture]\nname = "Fixture"\n'
        f'base_url = "http://127.0.0.1:{server.server_port}/v1"\n'
        'wire_api = "responses"\nrequires_openai_auth = false\n'
        '[analytics]\nenabled = false\n[feedback]\nenabled = false\n'
        f'[projects.{json.dumps(str(project))}]\ntrust_level = "trusted"\n'
        f'[shell_environment_policy]\nexclude = ["{FOREIGN_ENV_NAME}"]\n'
    )
    (home / ".codex" / "config.toml").write_text(config)
    project_server = (
        '[mcp_servers.env_spike]\n'
        f'command = {json.dumps(sys.executable)}\n'
        f'args = [{json.dumps(str(child))}]\n'
        f'env_vars = ["{ENV_NAME}"]\n'
    )
    # Research-only per-project control. It tests whether a native Codex
    # shell policy could protect the shell while stdio MCP receives env_vars.
    # Never install this policy in real user files without foreign-policy proof.
    if mode == "filtered":
        project_server += (
            '[shell_environment_policy.filters]\n'
            f'{json.dumps(ENV_NAME)} = "exclude"\n'
        )
    # Research-only stricter native shell policy: requires a credential-like
    # variable NAME and deliberately does not replace lower-layer legacy
    # exclude/include_only arrays. Never install without scoped consent/proof.
    if mode == "default_excludes":
        project_server += (
            '[shell_environment_policy]\n'
            'ignore_default_excludes = false\n'
        )
    # Research-only scalar inheritance control. It preserves standard core
    # shell variables but prevents arbitrary parent environment forwarding to
    # shell tools; stdio MCP env_vars must continue to work independently.
    if mode == "core_inherit":
        project_server += (
            '[shell_environment_policy]\n'
            'inherit = "core"\n'
        )
    (project / ".codex" / "config.toml").write_text(project_server)
    env = isolated_env(root, home, secret, foreign_secret)
    commands = [
        str(codex), "exec", "--skip-git-repo-check",
        "--sandbox", "read-only", "Call the available shell tool once and then confirm_env."
    ]
    report = {"policy_mode": mode, "sandbox_backend": ("bwrap" if sys.platform.startswith("linux") else "native"), "version": "", "real_codex_shell_called": False,
              "real_codex_mcp_called": False, "mcp_child_saw_secret": False,
              "leak": False, "secrecy_pass": False, "failure": None}
    try:
        version = subprocess.run([str(codex), "--version"], env=env,
                                 capture_output=True, timeout=15, text=True)
        report["version"] = version.stdout.strip()[:100]
        # Observe LIVE named snapshots during the deliberately held shell tool.
        # Post-exit scans alone miss transient files deleted by normal cleanup.
        monitor_stop = threading.Event()
        live = {"samples": 0, "hits": {}, "symlinks": 0, "errors": 0,
                "snapshot_files_seen": False}
        def monitor():
            while not monitor_stop.is_set():
                try:
                    matches, links = inspect(root, (secret, foreign_secret))
                    live["samples"] += 1
                    live["symlinks"] = max(live["symlinks"], links)
                    for match in matches:
                        live["hits"][match["path_sha256"]] = match
                    snapshot_dir = home / ".codex" / "shell_snapshots"
                    if snapshot_dir.is_dir() and any(snapshot_dir.iterdir()):
                        live["snapshot_files_seen"] = True
                except OSError:
                    live["errors"] += 1
                monitor_stop.wait(0.035)
        observer = threading.Thread(target=monitor, daemon=True)
        observer.start()
        try:
            result = subprocess.run(commands, cwd=project, env=env,
                                    capture_output=True, timeout=115)
        finally:
            monitor_stop.set()
            observer.join(timeout=3)
        report["codex_exit_code"] = result.returncode
        report["live_scan_samples"] = live["samples"]
        report["live_snapshot_files_seen"] = live["snapshot_files_seen"]
        report["live_scan_errors"] = live["errors"]
        report["shell_tool_output_seen"] = Model.shell_output_seen
        report["shell_secret_visible"] = Model.shell_secret_visible
        report["shell_foreign_visible"] = Model.shell_foreign_visible
        report["shell_response_category"] = Model.shell_response_category
        report["shell_response_indicators"] = Model.shell_response_indicators
        report["real_codex_shell_called"] = Model.shell_requested and Model.shell_execution_verified
        report["real_codex_mcp_called"] = Model.mcp_requested and Model.mcp_output_seen
        report["native_mcp_digest_seen"] = Model.mcp_digest_verified
        report["mcp_output_type"] = Model.mcp_output_type
        report["mcp_output_indicators"] = Model.mcp_output_indicators
        report["requests"] = Model.requests
        report["advertised_tool_names"] = sorted(Model.advertised_tool_names)[:35]
        report["mcp_child_saw_secret"] = Model.mcp_digest_verified
        found, symlinks = inspect(root, (secret, foreign_secret))
        combined = {entry["path_sha256"]: entry for entry in
                    [*found, *live["hits"].values()]}
        report["leak"] = bool(combined)
        report["matches"] = list(combined.values())[:10]
        report["symlinks"] = max(symlinks, live["symlinks"])
        if result.returncode != 0:
            report["failure"] = Model.error or "native_codex_exit_nonzero"
        elif not all([report["real_codex_shell_called"], report["real_codex_mcp_called"],
                      report["mcp_child_saw_secret"]]):
            report["failure"] = Model.error or "native_proof_incomplete"
        elif (combined or live["errors"] or live["samples"] < 20
              or Model.shell_secret_visible or Model.shell_foreign_visible):
            report["failure"] = "live_canary_persisted_or_unsafe_scan"
        else:
            report["secrecy_pass"] = True
    except (OSError, subprocess.TimeoutExpired, ValueError) as exc:
        report["failure"] = type(exc).__name__
    finally:
        server.shutdown()
        report_path.write_text(json.dumps(report, indent=2, sort_keys=True))
        shutil.rmtree(root, ignore_errors=True)
    print(json.dumps({k: v for k, v in report.items() if k != "matches"}, sort_keys=True))
    return 0 if report["secrecy_pass"] else 1

if __name__ == "__main__":
    raise SystemExit(main())
