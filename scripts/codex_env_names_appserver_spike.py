#!/usr/bin/env python3
"""REAL Codex app-server MCP env-vars + shell snapshot oracle; synthetic only.

Uses the same source-verified client JSON-RPC methods and local model/MCP
fixture as UAP's native Codex project tests. No auth, no remote model, no
plaintext canary in output or artifacts. Fail closed on incomplete evidence.
"""
import hashlib
import http.server
import json
import os
from pathlib import Path
import queue
import secrets
import shutil
import subprocess
import sys
import tempfile
import threading
import time

from codex_env_names_native_spike import CHILD, ENV_NAME, Model, inspect

def rpc_send(proc, q, serial, method, params, timeout=35):
    proc.stdin.write((json.dumps({"id": serial, "method": method,
                                  "params": params}) + "\n").encode())
    proc.stdin.flush()
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            row = q.get(timeout=min(1, max(0.01, deadline - time.monotonic())))
        except queue.Empty:
            if proc.poll() is not None:
                raise RuntimeError("codex_rpc_process_ended")
            continue
        if row.get("id") != serial:
            continue
        if row.get("error"):
            raise RuntimeError("codex_rpc_error_" + method.replace("/", "_"))
        return row.get("result", {})
    raise RuntimeError("codex_rpc_timeout_" + method.replace("/", "_"))

def main():
    codex = Path(sys.argv[1]).resolve(strict=True)
    evidence = Path(sys.argv[2]).resolve()
    evidence.parent.mkdir(parents=True, exist_ok=True)
    mode = sys.argv[3] if len(sys.argv) > 3 else "default"
    if mode not in ("default", "filtered", "default_excludes"):
        raise ValueError("unsupported native shell policy control mode")
    secret = "uap_appserver_spike_" + secrets.token_hex(24)
    Model.expected_digest = hashlib.sha256(secret.encode()).hexdigest()
    root = Path(tempfile.mkdtemp(prefix="uap-codex-appserver-"))
    home, project = root / "home", root / "project"
    (home / ".codex").mkdir(parents=True)
    (project / ".codex").mkdir(parents=True)
    (project / ".git").mkdir()
    (root / "tmp").mkdir()
    child = root / "child.py"
    child.write_text(CHILD)
    model = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Model)
    model_thread = threading.Thread(target=model.serve_forever, daemon=True)
    model_thread.start()
    user_config = (
        'cli_auth_credentials_store = "file"\n'
        'mcp_oauth_credentials_store = "file"\n'
        'model_provider = "fixture"\nmodel = "fixture-model"\n'
        f'[model_providers.fixture]\nname = "Fixture"\nbase_url = "http://127.0.0.1:{model.server_port}/v1"\n'
        'wire_api = "responses"\nrequires_openai_auth = false\n'
        '[analytics]\nenabled = false\n[feedback]\nenabled = false\n'
        f'[projects.{json.dumps(str(project))}]\ntrust_level = "trusted"\n'
    )
    (home / ".codex" / "config.toml").write_text(user_config)
    project_server = (
        '[mcp_servers.env_spike]\ncommand = "python3"\n'
        f'args = [{json.dumps(str(child))}]\n'
        f'env_vars = ["{ENV_NAME}"]\n'
    )
    if mode == "filtered":
        # Research-only comparison; this is NOT an approved UAP auto-mutation.
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
    (project / ".codex" / "config.toml").write_text(project_server)
    env = {"HOME": str(home), "CODEX_HOME": str(home / ".codex"),
           "TMPDIR": str(root / "tmp"), "TEMP": str(root / "tmp"),
           "TMP": str(root / "tmp"), "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
           "LANG": "C.UTF-8", "NO_COLOR": "1", ENV_NAME: secret}
    report = {"policy_mode": mode, "version": "", "real_codex_shell_called": False,
              "mcp_native_connected": False, "real_codex_mcp_called": False,
              "mcp_child_saw_secret": False, "leak": False,
              "secrecy_pass": False, "failure": None}
    proc = None
    stop = threading.Event()
    live = {"samples": 0, "hits": {}, "errors": 0, "snapshots_seen": False}
    def observer():
        while not stop.is_set():
            try:
                hits, _ = inspect(root, secret)
                live["samples"] += 1
                for hit in hits:
                    live["hits"][hit["path_sha256"]] = hit
                snap = home / ".codex" / "shell_snapshots"
                if snap.is_dir() and any(snap.iterdir()):
                    live["snapshots_seen"] = True
            except OSError:
                live["errors"] += 1
            stop.wait(.035)
    watch = threading.Thread(target=observer, daemon=True)
    watch.start()
    try:
        version = subprocess.run([str(codex), "--version"], env=env,
                                 capture_output=True, timeout=15, text=True)
        report["version"] = version.stdout.strip()[:100]
        proc = subprocess.Popen([str(codex), "app-server"], cwd=project, env=env,
                                stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                stderr=subprocess.DEVNULL)
        messages = queue.Queue(maxsize=512)
        def receive():
            for raw in proc.stdout:
                try:
                    if len(raw) > 1 << 20:
                        continue
                    messages.put(json.loads(raw), timeout=1)
                except (ValueError, queue.Full):
                    break
        threading.Thread(target=receive, daemon=True).start()
        rpc_send(proc, messages, 1, "initialize", {
            "clientInfo": {"name": "uap_codex_appserver_spike", "version": "1.0.0"},
            "capabilities": {"experimentalApi": True}
        })
        proc.stdin.write(b'{"method":"initialized"}\n')
        proc.stdin.flush()
        thread = rpc_send(proc, messages, 2, "thread/start", {
            "cwd": str(project), "ephemeral": True, "approvalPolicy": "never",
            "sandbox": "read-only", "model": "fixture-model", "modelProvider": "fixture"
        })
        thread_id = thread.get("thread", {}).get("id")
        if not thread_id:
            raise RuntimeError("codex_thread_id_missing")
        # Codex 0.167 can stall mcpServerStatus/list while the global MCP
        # registry initializes. Avoid making registry listing a precondition
        # for a genuine native tool/call. The only acceptable success oracle
        # remains a correlated native MCP function output + digest.
        # The model turn itself exercises the actual MCP child and shell.
        # Independent current Codex app-server native stdio MCP call.
        # The official API does not require a model tool search; this gives
        # precise evidence if the scripted model path fails separately.
        # The actual tool output is never saved, only a synthetic digest bool.
        report["direct_mcp_call_verified"] = False
        try:
            direct = rpc_send(proc, messages, 99, "mcpServer/tool/call", {
                "threadId": thread_id, "server": "env_spike",
                "tool": "confirm_env", "arguments": {}
            }, timeout=22)
            report["direct_mcp_call_verified"] = (
                direct.get("isError") is not True and
                Model.expected_digest in json.dumps(direct, sort_keys=True)
            )
        except RuntimeError as exc:
            report["direct_mcp_call_error"] = str(exc)[:96]
        rpc_send(proc, messages, 100, "turn/start", {
            "threadId": thread_id,
            "input": [{"type": "text",
                       "text": "Run a shell tool once, then call env_spike confirm_env.",
                       "text_elements": []}]
        })
        deadline = time.monotonic() + 65
        while not Model.mcp_output_seen and time.monotonic() < deadline:
            if Model.error:
                raise RuntimeError("mock_model_" + Model.error.replace(" ", "_"))
            if proc.poll() is not None:
                raise RuntimeError("codex_ended_while_turn_active")
            time.sleep(.05)
        report["shell_tool_output_seen"] = Model.shell_output_seen
        report["real_codex_shell_called"] = Model.shell_requested and Model.shell_execution_verified
        report["real_codex_mcp_called"] = Model.mcp_requested and Model.mcp_output_seen
        report["mcp_native_connected"] = Model.mcp_digest_verified
        report["native_mcp_digest_seen"] = Model.mcp_digest_verified
        report["mcp_child_saw_secret"] = Model.mcp_digest_verified
        if not (report["real_codex_shell_called"] and report["real_codex_mcp_called"]
                and report["mcp_child_saw_secret"]):
            raise RuntimeError("native_runtime_or_child_receipt_unproven")
    except (OSError, RuntimeError, subprocess.TimeoutExpired, ValueError) as exc:
        report["failure"] = str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__
    finally:
        if proc is not None:
            proc.terminate()
            try:
                proc.communicate(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.communicate(timeout=5)
        stop.set()
        watch.join(timeout=3)
        found, symlinks = inspect(root, secret)
        all_hits = {x["path_sha256"]: x for x in [*found, *live["hits"].values()]}
        report["leak"] = bool(all_hits)
        report["matching_path_hashes"] = list(all_hits.values())[:12]
        report["live_scans"] = live["samples"]
        report["live_snapshot_files_seen"] = live["snapshots_seen"]
        report["scan_errors"] = live["errors"]
        report["symlinks_observed_not_followed"] = symlinks
        report["model_requests"] = Model.requests
        report["model_advertised_tools"] = sorted(Model.advertised_tool_names)[:25]
        report["secrecy_pass"] = (report["failure"] is None and not all_hits
                                  and live["errors"] == 0 and live["samples"] >= 20)
        if report["failure"] is None and not report["secrecy_pass"]:
            report["failure"] = "canary_persistence_or_incomplete_live_scan"
        evidence.write_text(json.dumps(report, sort_keys=True, indent=2))
        model.shutdown()
        shutil.rmtree(root, ignore_errors=True)
    print(json.dumps({k: v for k, v in report.items() if k != "matching_path_hashes"}, sort_keys=True))
    return 0 if report["secrecy_pass"] else 1

if __name__ == "__main__":
    raise SystemExit(main())
