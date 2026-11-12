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
import threading
import time

from codex_env_names_native_spike import CHILD, ENV_NAME, FOREIGN_ENV_NAME, Model, inspect, isolated_env, aggregate_findings, structured_mcp_receipt_matches, WINDOWS_NATIVE_SANDBOX_CONFIG, isolated_root, _capture_scan_root, stop_live_observer, synthetic_canary_patterns, contains_synthetic_canary, verified_vendor_version, PINNED_VENDOR_VERSION

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

def terminal_turn_status(message, thread_id, turn_id):
    """Only a correlated terminal turn notification can certify completion."""
    if not isinstance(message, dict) or message.get("method") != "turn/completed":
        return None
    params = message.get("params")
    if not isinstance(params, dict) or params.get("threadId") != thread_id:
        return None
    turn = params.get("turn")
    if not isinstance(turn, dict) or turn.get("id") != turn_id:
        return None
    return turn.get("status") == "completed"


def wait_for_turn_terminal(proc, messages, thread_id, turn_id, deadline):
    while time.monotonic() < deadline:
        try:
            event = messages.get(timeout=min(.25, max(.01, deadline - time.monotonic())))
        except queue.Empty:
            if proc.poll() is not None:
                return False
            continue
        outcome = terminal_turn_status(event, thread_id, turn_id)
        if outcome is not None:
            return outcome
    return False


def classify_native_stdio_line(raw, patterns):
    """Parse bounded JSON-RPC while returning only canary/coverage flags."""
    matched = contains_synthetic_canary(raw, patterns)
    if len(raw) > 1 << 20:
        return None, matched, False
    try:
        message = json.loads(raw)
    except (TypeError, ValueError):
        return None, matched, False
    return message, matched, isinstance(message, dict)


def main():
    codex = Path(sys.argv[1]).resolve(strict=True)
    evidence = Path(sys.argv[2]).resolve()
    evidence.parent.mkdir(parents=True, exist_ok=True)
    mode = sys.argv[3] if len(sys.argv) > 3 else "default"
    if mode not in ("default", "filtered", "default_excludes", "core_inherit"):
        raise ValueError("unsupported native shell policy control mode")
    if len(sys.argv) > 4:
        raise ValueError("unexpected native spike parameter")
    secret = "uap_appserver_spike_" + secrets.token_hex(24)
    foreign_secret = "uap_foreign_appserver_" + secrets.token_hex(24)
    stdio_patterns = synthetic_canary_patterns((secret, foreign_secret))
    Model.expected_digest = hashlib.sha256(secret.encode()).hexdigest()
    root = isolated_root("uap-codex-appserver-")
    root_anchor = _capture_scan_root(root)
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
        f'[shell_environment_policy]\nexclude = ["{FOREIGN_ENV_NAME}"]\n'
    )
    if os.name == "nt":
        user_config += WINDOWS_NATIVE_SANDBOX_CONFIG
    (home / ".codex" / "config.toml").write_text(user_config)
    project_server = (
        '[mcp_servers.env_spike]\n'
        f'command = {json.dumps(sys.executable)}\n'
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
    # Research-only scalar inheritance mode; never mutate actual Codex homes.
    if mode == "core_inherit":
        project_server += (
            '[shell_environment_policy]\n'
            'inherit = "core"\n'
        )
    (project / ".codex" / "config.toml").write_text(project_server)
    env = isolated_env(root, home, secret, foreign_secret)
    report = {"policy_mode": mode, "sandbox_backend": ("bwrap" if sys.platform.startswith("linux") else "native"), "version": "", "real_codex_shell_called": False,
              "mcp_native_connected": False, "real_codex_mcp_called": False,
              "mcp_child_saw_secret": False, "leak": False,
              "secrecy_pass": False, "failure": None,
              "turn_completed": False, "graceful_shutdown": False,
              "shutdown_mode": "not_run", "stdio_canary_observed": False,
              "stdio_reader_stopped": False, "stdio_read_complete": False}
    proc = None
    receiver = None
    stdio = {"canary_observed": False, "incomplete": False}
    stop = threading.Event()
    live = {"samples": 0, "hits": {}, "errors": 0, "snapshots_seen": False}
    def observer():
        while not stop.is_set():
            try:
                hits, _ = inspect(
                    root, (secret, foreign_secret), root_anchor=root_anchor
                )
                live["samples"] += 1
                for hit in hits:
                    live["hits"][(hit["path_sha256"], hit["result"])] = hit
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
        report["version"] = (
            PINNED_VENDOR_VERSION if verified_vendor_version(version)
            else "unverified_vendor_version"
        )
        stdio["canary_observed"] = (
            contains_synthetic_canary(version.stdout.encode(), stdio_patterns)
            or contains_synthetic_canary(version.stderr.encode(), stdio_patterns)
        )
        if report["version"] != PINNED_VENDOR_VERSION:
            raise RuntimeError("vendor_version_unverified")
        proc = subprocess.Popen([str(codex), "app-server"], cwd=project, env=env,
                                stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                stderr=subprocess.DEVNULL)
        messages = queue.Queue(maxsize=512)
        def receive():
            try:
                for raw in proc.stdout:
                    message, matched, complete = classify_native_stdio_line(
                        raw, stdio_patterns
                    )
                    stdio["canary_observed"] |= matched
                    if not complete:
                        stdio["incomplete"] = True
                        break
                    try:
                        messages.put(message, timeout=1)
                    except queue.Full:
                        stdio["incomplete"] = True
                        break
            except (OSError, ValueError):
                stdio["incomplete"] = True
        receiver = threading.Thread(target=receive, daemon=True)
        receiver.start()
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
            report["direct_mcp_call_verified"] = structured_mcp_receipt_matches(
                direct, Model.expected_digest
            )
        except RuntimeError as exc:
            report["direct_mcp_call_error"] = str(exc)[:96]
        started = rpc_send(proc, messages, 100, "turn/start", {
            "threadId": thread_id,
            "input": [{"type": "text",
                       "text": "Run a shell tool once, then call env_spike confirm_env.",
                       "text_elements": []}]
        })
        turn_id = started.get("turn", {}).get("id")
        if not isinstance(turn_id, str) or not turn_id:
            raise RuntimeError("codex_turn_id_missing")
        deadline = time.monotonic() + 65
        while not Model.mcp_output_seen and time.monotonic() < deadline:
            if Model.error:
                raise RuntimeError("mock_model_" + Model.error.replace(" ", "_"))
            if proc.poll() is not None:
                raise RuntimeError("codex_ended_while_turn_active")
            time.sleep(.05)
        if not Model.mcp_output_seen or not wait_for_turn_terminal(
                proc, messages, thread_id, turn_id, deadline):
            raise RuntimeError("native_turn_terminal_unproven")
        report["turn_completed"] = True
        report["shell_tool_output_seen"] = Model.shell_output_seen
        report["shell_secret_visible"] = Model.shell_secret_visible
        report["shell_foreign_visible"] = Model.shell_foreign_visible
        report["shell_response_category"] = Model.shell_response_category
        report["shell_response_indicators"] = Model.shell_response_indicators
        report["real_codex_shell_called"] = Model.shell_requested and Model.shell_execution_verified
        # Do not conflate genuine vendor app-server MCP tool receipt with
        # model-turn tool routing. Both results are reported independently;
        # either real MCP response can prove native child delivery, never
        # shell nonpersistence.
        report["model_mcp_tool_called"] = Model.mcp_requested and Model.mcp_output_seen
        report["real_codex_mcp_called"] = (report["model_mcp_tool_called"]
                                           or report["direct_mcp_call_verified"])
        report["mcp_native_connected"] = report["direct_mcp_call_verified"]
        report["native_mcp_digest_seen"] = Model.mcp_digest_verified
        report["mcp_output_type"] = Model.mcp_output_type
        report["mcp_output_indicators"] = Model.mcp_output_indicators
        report["mcp_child_saw_secret"] = (report["direct_mcp_call_verified"]
                                           or Model.mcp_digest_verified)
        # Both independent native MCP routes must prove the same synthetic
        # digest. A direct-only success cannot conceal a failed model turn.
        if not (report["real_codex_shell_called"] and
                report["direct_mcp_call_verified"] and
                Model.mcp_digest_verified):
            raise RuntimeError("native_shell_or_both_mcp_routes_unproven")
    except (OSError, RuntimeError, subprocess.TimeoutExpired, ValueError) as exc:
        report["failure"] = str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__
    finally:
        if proc is not None:
            # Official Codex stdio transport drains pending RPC/rollout on EOF.
            # Never use SIGTERM as the happy path before the terminal turn.
            if proc.poll() is None:
                try:
                    proc.stdin.close()
                    proc.stdin = None
                    proc.wait(timeout=38)
                    report["graceful_shutdown"] = proc.returncode == 0
                    report["shutdown_mode"] = (
                        "eof_graceful" if report["graceful_shutdown"]
                        else "nonzero_exit"
                    )
                except (OSError, subprocess.TimeoutExpired):
                    report["shutdown_mode"] = "forced_terminate"
                    if proc.poll() is None:
                        proc.terminate()
                        try:
                            proc.wait(timeout=5)
                        except subprocess.TimeoutExpired:
                            report["shutdown_mode"] = "forced_kill"
                            proc.kill()
                            proc.wait(timeout=5)
            else:
                report["shutdown_mode"] = "already_exited"
            if receiver is not None:
                receiver.join(timeout=5)
                report["stdio_reader_stopped"] = not receiver.is_alive()
            if proc.stdout is not None and report["stdio_reader_stopped"]:
                proc.stdout.close()
        report["stdio_canary_observed"] = stdio["canary_observed"]
        report["stdio_read_complete"] = (
            report["stdio_reader_stopped"] and not stdio["incomplete"]
        )
        observer_stopped = stop_live_observer(stop, watch)
        report["observer_stopped"] = observer_stopped
        if report["failure"] is None and not report["graceful_shutdown"]:
            report["failure"] = "graceful_shutdown_unverified"
        found, symlinks = inspect(
            root, (secret, foreign_secret), root_anchor=root_anchor
        )
        # A still-running observer may hold an unseen transient canary;
        # do not consume its mutable findings or certify a security PASS.
        all_hits, counts = aggregate_findings(
            found, tuple(live["hits"].values()) if observer_stopped else ()
        )
        report["leak"] = bool(all_hits)  # legacy name: any unsafe finding
        report["canary_persistence_observed"] = counts.get("synthetic_value_persisted", 0) > 0
        report["unsafe_scan_observed"] = (not observer_stopped or any(
            category.startswith("unscanned_") and total > 0
            for category, total in counts.items()
        ))
        report["finding_counts"] = counts
        report["finding_total"] = len(all_hits)
        report["matching_path_hashes"] = all_hits[:12]
        report["live_scans"] = live["samples"]
        report["live_snapshot_files_seen"] = live["snapshots_seen"]
        report["scan_errors"] = live["errors"]
        report["symlinks_observed_not_followed"] = symlinks
        report["model_requests"] = Model.requests
        report["model_advertised_tools"] = sorted(Model.advertised_tool_names)[:25]
        report["secrecy_pass"] = (report["failure"] is None
                                  and report["turn_completed"]
                                  and report["graceful_shutdown"]
                                  and report["stdio_read_complete"]
                                  and not report["stdio_canary_observed"]
                                  and observer_stopped and not all_hits
                                  and not Model.shell_secret_visible
                                  and not Model.shell_foreign_visible
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
