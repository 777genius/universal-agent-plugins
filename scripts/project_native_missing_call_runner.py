"""Opt-in real Cursor/Qoder Project MCP tools/call E2E on disposable GitHub Linux.

Requires an authorized disposable vendor account. No synthetic JSON-RPC caller
is used: only the pinned native CLI may start/call the UAP-installed stdio probe.
All vendor output is private and never uploaded. No blanket MCP permissions.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import sys
import tempfile

import project_native_research as native
from native_client_research import run_probe
from project_native_missing_call_proof import (
    require_server_call, require_unchanged_after_negative, fresh_native_model_events,
)

ROOT = Path(__file__).resolve().parent.parent
AUTH = {"cursor": "CURSOR_API_KEY", "qoder": "QODER_PERSONAL_ACCESS_TOKEN"}
NAMES = {"cursor": "uap_cursor_call", "qoder": "uap_qoder_call"}
CONFIG = {"cursor": ".cursor/mcp.json", "qoder": ".mcp.json"}
EXPECTED_VERSIONS = {"cursor": "2026.10.22-27dcf5f", "qoder": "1.1.78"}
HEX = re.compile(r"[0-9a-f]{40}\Z")


def utc():
    return datetime.now(timezone.utc)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def emit(path, value):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    if path.exists() or path.is_symlink():
        raise ValueError("refusing to overwrite fixture state")
    body = value if isinstance(value, bytes) else (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()
    with path.open("xb") as stream:
        stream.write(body)
    path.chmod(0o600)


def invoke(label, argv, cwd, env, private, timeout=35):
    if len(label) > 64 or not label.replace("-", "").isalnum():
        raise ValueError("unregistered command label")
    # The existing bounded native process supervisor closes descendants and
    # captures raw stdout/stderr only in ephemeral private memory/directory.
    record, stdout, stderr = run_probe(
        [str(item) for item in argv], cwd, env, private, label,
        timeout=timeout, limit=1 << 20, memory_only=True)
    if record.get("error") is not None or record.get("deadline") or record.get("exit") != 0:
        raise ValueError("genuine native command failed at stage: " + label)
    return stdout, stderr


def isolated_env(work, go=False):
    keep = {key: os.environ[key] for key in (
        "PATH", "LANG", "LC_ALL", "GOPROXY", "GOSUMDB", "RUNNER_OS") if key in os.environ}
    keep.update({
        "HOME": str(work / "home"), "USERPROFILE": str(work / "home"),
        "XDG_CONFIG_HOME": str(work / "xdg-config"),
        "XDG_CACHE_HOME": str(work / "xdg-cache"),
        "XDG_DATA_HOME": str(work / "xdg-data"),
        "XDG_STATE_HOME": str(work / "xdg-state"),
        "TMPDIR": str(work / "tmp"), "GIT_CONFIG_GLOBAL": "/dev/null",
        "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0",
        "CI": "true", "NO_COLOR": "1", "TERM": "dumb",
    })
    if go:
        keep.update({"GOCACHE": str(work / "go-cache"),
                     "GOMODCACHE": str(work / "go-mod-cache"),
                     "GOTOOLCHAIN": "local", "GOMAXPROCS": "2"})
    for key in ("home", "xdg-config", "xdg-cache", "xdg-data", "xdg-state", "tmp", "private"):
        (work / key).mkdir(mode=0o700, exist_ok=True)
    return keep


def verify_clean_head(expected, env, private):
    if not HEX.fullmatch(expected):
        raise ValueError("expected immutable source HEAD required")
    sha, _ = invoke("source-head", ["/usr/bin/git", "rev-parse", "HEAD"], ROOT, env, private)
    if sha.decode("ascii").strip() != expected:
        raise ValueError("source HEAD does not match dispatch request")
    dirty, _ = invoke("source-status", ["/usr/bin/git", "status", "--porcelain", "--untracked-files=normal"], ROOT, env, private)
    if dirty:
        raise ValueError("source tree is not clean")


def source_build(work, env, private):
    compiler = shutil.which("go")
    if not compiler:
        raise ValueError("pinned Go toolchain missing")
    version, _ = invoke("go-version", [compiler, "version"], ROOT, env, private)
    if version != b"go version go1.26.11 linux/amd64\n":
        raise ValueError("required pinned Go 1.26.11 not selected")
    installer, probe = work / "bin" / "agentplugins", work / "source" / "bin" / "probe"
    installer.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    probe.parent.mkdir(mode=0o700, parents=True)
    invoke("build-installer", [compiler, "build", "-p=1", "-trimpath", "-o", installer, "./cmd/agentplugins"], ROOT / "cli", env, private, 240)
    invoke("build-probe", [compiler, "build", "-p=1", "-trimpath", "-o", probe, "./repotests/testdata/agentplugins_native_probe"], ROOT, env, private, 120)
    if not installer.is_file() or not probe.is_file():
        raise ValueError("exact source-built fixture binaries absent")
    return installer, probe


def acquire_vendor(client, work, env, private):
    leaf = native.adjacent("call_fixture_" + client, "project_native_collectors/" + client + ".py")
    if leaf.CLIENT != client or leaf.VERSION != EXPECTED_VERSIONS[client]:
        raise ValueError("unreviewed native client specification")
    with native.Budget(400) as budget:
        acquisition, files = native.acquire(leaf.ACQUISITION, budget)
        binary, held = native.materialize(work / "native" / client, leaf.ACQUISITION, acquisition, files,
                                         selected_only=(client == "qoder"))
        native.verify_files(held, budget)
    if not acquisition["pin_verified"] or not binary.is_file():
        raise ValueError("pinned original native executable unavailable")
    # Cursor's verified launcher depends on its sibling bundled Node/index.
    # Detect it via the original distribution directory, never a relocated
    # alias whose shell-relative directory could point to missing resources.
    if client == "cursor":
        search = binary.parent
    else:
        alias = work / "native-alias"
        alias.mkdir(mode=0o700, exist_ok=True)
        (alias / "qoder").symlink_to(binary)
        search = alias
    env["PATH"] = str(search) + ":" + env.get("PATH", "/usr/bin:/bin")
    selected = shutil.which("cursor-agent" if client == "cursor" else "qoder", path=env["PATH"])
    if selected is None or Path(selected).resolve(strict=True) != binary.resolve(strict=True):
        raise ValueError("native UAP detector resolves to the wrong pinned client")
    text, _ = invoke("native-version", [binary, "--version"], work / "a", env, private, 30)
    if text.strip().decode("utf-8", "strict") != EXPECTED_VERSIONS[client]:
        raise ValueError("selected native version differs")
    return binary, held, acquisition


def installation_id(payload, client):
    data = payload.get("data", {})
    output = None
    targets = data.get("targets")
    if isinstance(targets, list) and len(targets) == 1 and targets[0].get("target") == client:
        output = targets[0].get("output")
    elif isinstance(data.get("result"), dict):
        output = data
    value = (output or {}).get("result", {}).get("installation_id")
    if not isinstance(value, str) or len(value) != 36:
        raise ValueError("owned public UAP installation ID missing")
    return value


def native_inventory(client, binary, folder, env, private, label, absent):
    out, err = invoke(label, [binary, "mcp", "list"], folder, env, private, 40)
    if absent.encode() in out + b"\n" + err:
        raise ValueError("unexpected project MCP source in " + label)


def native_selected_inventory(binary, folder, env, private, server):
    out, err = invoke("selected-native-mcp-list", [binary, "mcp", "list"], folder, env, private, 40)
    if server.encode() not in out + b"\n" + err:
        raise ValueError("UAP-installed selected MCP server not listed by genuine native client")


def installed_config(client, project, name, nonce, events, installer, probe):
    target = project / CONFIG[client]
    content = json.loads(target.read_bytes())
    members = content.get("mcpServers")
    if not isinstance(members, dict) or name not in members or "foreign_keep" not in members:
        raise ValueError("installed UAP-owned MCP or foreign member absent")
    if members["foreign_keep"] != {"command": "/usr/bin/true", "args": []}:
        raise ValueError("foreign MCP entry modified")
    selected = members[name]
    if not isinstance(selected, dict) or selected.get("type", "stdio") not in ("stdio", None):
        raise ValueError("selected project stdio MCP identity differs")
    env = selected.get("env")
    if (not isinstance(env, dict) or env.get("UAP_TEST_NONCE") != nonce
            or env.get("UAP_TEST_EVENTS") != str(events)):
        raise ValueError("different MCP source or nonce selected")
    command, arguments = selected.get("command"), selected.get("args")
    if not isinstance(command, str) or not isinstance(arguments, list) or arguments[:1] != ["--internal-stdio-v1"]:
        raise ValueError("UAP-managed stdio transport not selected")
    executable = Path(command)
    if not executable.is_absolute() or not executable.is_file() or executable.is_symlink():
        raise ValueError("managed wrapper executable differs")
    managed = project / ".uap" / "managed" / "clients" / client
    if managed not in executable.parents or executable.name != "agentplugins":
        raise ValueError("wrapper not inside the owned Project installation")
    if digest(executable.read_bytes()) != digest(installer.read_bytes()):
        raise ValueError("managed wrapper differs from source-built UAP")
    active = executable.parents[2]
    actual_probe = active / "bin" / "probe"
    if (not actual_probe.is_file() or actual_probe.is_symlink()
            or digest(actual_probe.read_bytes()) != digest(probe.read_bytes())):
        raise ValueError("owned MCP probe differs from source-built fixture")
    return active


def permission_fixture(client, project, server):
    if client == "cursor":
        emit(project / ".cursor" / "cli.json", {
            "permissions": {
                "allow": ["Mcp(" + server + ":inspect_runtime)"],
                "deny": ["Shell(*)", "Read(**/*)", "Write(**/*)"],
            },
        })
    else:
        emit(project / ".qoder" / "settings.json", {
            "mcp": {"enableAllProjectMcpServers": False,
                    "enabledProjectMcpServers": [server]},
            "permissions": {
                "allow": ["mcp__" + server + "__inspect_runtime"],
                "deny": ["Bash", "Read", "Edit", "Write", "WebFetch", "WebSearch"],
            },
        })


def agent_command(client, binary, prompt):
    if client == "cursor":
        return [binary, "-p", prompt, "--output-format", "json"]
    return [binary, "-p", "--allowed-mcp-server-names", NAMES["qoder"],
            "--output-format", "json", "--max-turns", "4",
            "--no-session-persistence", prompt]


def preflight_checkpoint(enabled, label):
    """Public, fixed-name stage receipt only; never native stdout or credentials."""
    if enabled:
        if label not in {
            "source-built", "disposable-git-ready", "pinned-vendor-ready",
            "before-install-negative-ok", "uap-installed", "managed-wrapper-verified",
            "selected-native-listed", "uap-removed", "after-remove-negative-ok",
            "pin-and-foreign-state-verified",
        }:
            raise ValueError("unregistered preflight checkpoint")
        print("credential-free-preflight-stage: " + label, flush=True)


def run(client, output, expected, *, preflight_only=False):
    # No CI-host bypass, no local user profile, no unapproved vendor account.
    native.native.require_hosted("linux-amd64")
    if preflight_only:
        # This lane must be independently runnable with zero vendor credentials.
        if any(os.environ.get(name) for name in AUTH.values()):
            raise ValueError("credential-free preflight cannot inherit vendor authentication")
        key = None
    else:
        key = os.environ.get(AUTH[client], "")
        if not 16 <= len(key) <= 4096 or any(char in key for char in "\x00\r\n"):
            raise ValueError("authorized disposable " + client + " CI credential absent/invalid")
    if (not output.is_absolute() or output.resolve() != output or ROOT in output.parents
            or output.exists() or output.is_symlink()):
        raise ValueError("fresh canonical evidence destination outside checkout required")
    scratch = Path(os.environ.get("RUNNER_TEMP", "/tmp")).resolve(strict=True)
    with tempfile.TemporaryDirectory(prefix="uap-mcp-call-", dir=scratch) as temporary:
        work = Path(temporary)
        for name in ("a", "b", "outside", "source", "bin", "private"):
            (work / name).mkdir(mode=0o700, exist_ok=True)
        private = work / "private"
        build_env = isolated_env(work, go=True)
        verify_clean_head(expected, build_env, private)
        installer, probe = source_build(work, build_env, private)
        preflight_checkpoint(preflight_only, "source-built")
        env = isolated_env(work)
        for project in ("a", "b", "outside"):
            invoke("git-" + project, ["/usr/bin/git", "init", "--quiet", "--initial-branch=main"],
                   work / project, env, private)
        if client == "qoder":
            # Explicit trust covers only disposable A/B/outside test roots.
            # B/outside still have no owned MCP source or named-tool approval:
            # a native negative must not pass merely because trust was denied.
            emit(work / "home" / ".qoder" / "settings.json", {
                "permissions": {"trustDirectories": [
                    str(work / "a"), str(work / "b"), str(work / "outside")]}
            })
        preflight_checkpoint(preflight_only, "disposable-git-ready")
        binary, vendor_files, acquisition = acquire_vendor(client, work, env, private)
        preflight_checkpoint(preflight_only, "pinned-vendor-ready")
        server = NAMES[client]
        nonce = secrets.token_hex(16)
        server_only = secrets.token_hex(16)
        events = work / "events.jsonl"
        project_a, project_b, outside = (work / name for name in ("a", "b", "outside"))
        source = work / "source"
        emit(source / "plugin.json", {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": "uap-native-mcp-call", "version": "1.0.0",
            "description": "Isolated authorized MCP call proof",
        })
        emit(source / "mcp.json", {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {server: {
                "type": "stdio", "command": "./bin/probe", "args": ["call-fixture"],
                "cwd": "${PLUGIN_ROOT}",
                "env": {"UAP_TEST_ROOT": str(work), "UAP_TEST_EVENTS": str(events),
                        "UAP_TEST_NONCE": nonce, "UAP_TEST_REVISION": "call-a",
                        "UAP_TEST_LITERAL": server_only},
            }},
        })
        foreign = {"mcpServers": {"foreign_keep": {"command": "/usr/bin/true", "args": []}}}
        if client == "cursor":
            a_config = project_a / ".cursor" / "mcp.json"
            b_config = project_b / ".cursor" / "mcp.json"
        else:
            a_config, b_config = project_a / ".mcp.json", project_b / ".mcp.json"
        emit(a_config, foreign)
        emit(b_config, foreign)
        emit(work / "home" / "foreign-user-control", b"foreign-user-do-not-touch\n")
        before_b = b_config.read_bytes()
        native_inventory(client, binary, project_b, env, private, "empty-b", server)
        native_inventory(client, binary, outside, env, private, "empty-outside", server)

        preflight_checkpoint(preflight_only, "before-install-negative-ok")
        stdout, _ = invoke("uap-add-a", [
            installer, "add", source, "--no-save", "--scope", "project",
            "--project-root", project_a, "--target", client, "--format", "json",
        ], project_a, env, private, 80)
        result = json.loads(stdout)
        if result.get("command") != "add" or result.get("result") != "success":
            raise ValueError("UAP add returned no success receipt")
        preflight_checkpoint(preflight_only, "uap-installed")
        identifier = installation_id(result, client)
        selected_root = installed_config(client, project_a, server, nonce, events, installer, probe)
        preflight_checkpoint(preflight_only, "managed-wrapper-verified")
        if b_config.read_bytes() != before_b:
            raise ValueError("Project B changed after Project A install")
        permission_fixture(client, project_a, server)
        if client == "cursor":
            invoke("cursor-enable-one", [binary, "mcp", "enable", server], project_a, env, private, 40)
        native_selected_inventory(binary, project_a, env, private, server)

        preflight_checkpoint(preflight_only, "selected-native-listed")
        if preflight_only:
            # Source/ownership/native-listing proof ONLY. No model or MCP call
            # can be inferred from this lane, even if a client starts its
            # server during a read-only inventory request.
            invoke("preflight-remove-a", [
                installer, "remove", identifier, "--scope", "project",
                "--project-root", project_a, "--target", client, "--format", "json",
            ], project_a, env, private, 80)
            preflight_checkpoint(preflight_only, "uap-removed")
            after = json.loads(a_config.read_bytes())
            if after.get("mcpServers") != foreign["mcpServers"]:
                raise ValueError("foreign Project A config changed on removal")
            native_inventory(client, binary, project_a, env, private,
                             "preflight-removed-a", server)
            native_inventory(client, binary, project_b, env, private,
                             "preflight-empty-b", server)
            preflight_checkpoint(preflight_only, "after-remove-negative-ok")
            if (b_config.read_bytes() != before_b
                    or (work / "home" / "foreign-user-control").read_bytes()
                    != b"foreign-user-do-not-touch\n"):
                raise ValueError("Project B or foreign user bytes changed")
            with native.Budget(180) as budget:
                native.verify_files(vendor_files, budget)
            preflight_checkpoint(preflight_only, "pin-and-foreign-state-verified")
            output.mkdir(mode=0o700, parents=True)
            emit(output / "source-only-preflight.json", {
                "schema_version": 1, "client": client, "source_commit": expected,
                "native_version": EXPECTED_VERSIONS[client],
                "vendor_archive_sha256": acquisition["sha256"],
                "native_executable_sha256": digest(binary.read_bytes()),
                "source_built_uap_and_fixture": True,
                "genuine_native_server_listed": True,
                "uap_owned_project_a_installed_and_removed": True,
                "other_project_and_foreign_state_preserved": True,
                "vendor_authentication_qualified": False,
                "native_tools_call_qualified": False,
                "real_model_qualified": False,
            })
            print(client + ": source-built UAP and native MCP listing/removal verified; tools/call NOT qualified")
            return

        # Discovery may have initialized the server before the model starts;
        # retain those original bytes but never count them as model evidence.
        before_model_events = events.read_bytes() if events.is_file() else b""
        start = utc()
        # Authorization is passed only to selected genuine agent sessions,
        # never to source builds, public UAP commands, or native inventories.
        session_env = {**env, AUTH[client]: key}
        command = agent_command(client, binary,
            'Invoke the existing Project MCP tool inspect_runtime from server '
            + server + ' exactly once with arguments {"nonce":"' + nonce
            + '"}. Do not use shell, browser, or file tools. Return the literal '
              'field from that MCP result as your complete response.')
        answer, _ = invoke("native-call-a", command, project_a, session_env, private, 180)
        end = utc()
        if server_only.encode() not in answer:
            raise ValueError("native model response did not carry server-only result")
        log = events.read_bytes() if events.is_file() else b""
        fresh_log = fresh_native_model_events(before_model_events, log, nonce=nonce)
        observed = require_server_call(fresh_log, nonce=nonce, started=start, ended=end)
        from project_native_missing_call_proof import read_events
        calls = [event["facts"] for event in read_events(fresh_log) if event["method"] == "tools/call"]
        if (len(calls) != 1 or calls[0].get("literal") != server_only
                or calls[0].get("plugin_root") != str(selected_root)
                or calls[0].get("revision") != "call-a"):
            raise ValueError("server call belongs to another plugin or produced uncorrelated result")
        before_negative = log

        # The two negative model sessions use the same real binary, but
        # neither can see Project A's MCP after source selection changes.
        no_call = ("Try to find the Project MCP server " + server +
                   " and invoke inspect_runtime. If unavailable say UNAVAILABLE."
                   " Do not use any other tool.")
        invoke("native-b-negative", agent_command(client, binary, no_call),
               project_b, session_env, private, 120)
        native_inventory(client, binary, project_b, env, private, "still-empty-b", server)
        require_unchanged_after_negative(before_negative, events.read_bytes())
        invoke("native-outside-negative", agent_command(client, binary, no_call),
               outside, session_env, private, 120)
        native_inventory(client, binary, outside, env, private, "still-empty-outside", server)
        require_unchanged_after_negative(before_negative, events.read_bytes())

        invoke("uap-remove-a", [
            installer, "remove", identifier, "--scope", "project",
            "--project-root", project_a, "--target", client, "--format", "json",
        ], project_a, env, private, 80)
        after = json.loads(a_config.read_bytes())
        if after.get("mcpServers") != foreign["mcpServers"]:
            raise ValueError("foreign Project A MCP changed on removal")
        native_inventory(client, binary, project_a, env, private, "removed-a", server)
        invoke("native-removed-negative", agent_command(client, binary, no_call),
               project_a, session_env, private, 120)
        require_unchanged_after_negative(before_negative, events.read_bytes())
        if b_config.read_bytes() != before_b or (work / "home" / "foreign-user-control").read_bytes() != b"foreign-user-do-not-touch\n":
            raise ValueError("foreign/Project B bytes changed")
        with native.Budget(180) as budget:
            native.verify_files(vendor_files, budget)

        # Only original server events and small redacted provenance are public.
        # No native model output or vendor token is ever retained.
        output.mkdir(mode=0o700, parents=True)
        emit(output / "server-events.jsonl", log)
        emit(output / "proof.json", {
            "schema_version": 1, "client": client,
            "source_commit": expected, "original_vendor_sha256": digest(binary.read_bytes()),
            "native_version": EXPECTED_VERSIONS[client],
            "original_vendor_archive_sha256": acquisition["sha256"],
            "server": server, "nonce": nonce,
            "server_events_sha256": digest(log),
            "pre_model_readonly_events_bytes": len(before_model_events),
            "model_events_sha256": digest(fresh_log),
            "server_call": observed,
            "client_model_returned_private_server_literal": True,
            "selected_project_a_uap_installed": True,
            "project_b_native_negative_executed": True,
            "outside_native_negative_executed": True,
            "post_remove_native_negative_executed": True,
            "foreign_project_and_user_bytes_preserved": True,
            "secret_value_recorded": False,
        })
    print(client + ": genuine selected-client Project tools/call; one server-side receipt; A/B/remove checked")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client", choices=("cursor", "qoder"), required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--expected-head", required=True)
    parser.add_argument("--preflight-only", action="store_true",
                        help="qualify only UAP install/native listing/remove, never MCP call")
    args = parser.parse_args()
    try:
        run(args.client, args.output, args.expected_head,
            preflight_only=args.preflight_only)
    except Exception as error:
        # Fail closed with a stable stage-free diagnostic. Never print a
        # vendor traceback, raw model output or credentials.
        reason = str(error) if str(error).startswith("genuine native command failed at stage: ") else type(error).__name__
        print("selected-client native MCP qualification failed: " + reason, file=sys.stderr)
        raise SystemExit(1) from None


if __name__ == "__main__":
    main()
