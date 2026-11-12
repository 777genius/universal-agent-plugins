#!/usr/bin/env python3
"""Bounded MCP-only UAP Project delivery smoke against the real pinned Kimi binary.

This test does not grant trust, start a native session or claim tools/call.
Only disposable test-owned paths are written. The honest result is Installed/Prepared.
"""
import argparse
import hashlib
import json
import signal
import os
from pathlib import Path
import stat
import subprocess
import tempfile

NAME = "uap-kimi-stable-mcp-proof"
FOREIGN_NAME = "foreign-sentinel"
FOREIGN_SERVER = {"transport": "http", "url": "https://foreign.invalid/mcp"}
MAX_OUTPUT = 1 << 20


def sha(body):
    return hashlib.sha256(body).hexdigest()


def safe_read(path):
    """Refuse aliases and changes while reading an owned bounded native JSON file."""
    if not path.is_absolute() or path.resolve() != path:
        raise ValueError("native MCP path alias refused")
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or before.st_uid != os.getuid() or before.st_size > MAX_OUTPUT:
        raise ValueError("native MCP file type/owner/size refused")
    if not hasattr(os, "O_NOFOLLOW"):
        raise ValueError("no-follow file descriptor support required")
    # A raced-in FIFO must not block the native-evidence reader indefinitely.
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK)
    try:
        held = os.fstat(fd)
        if (not stat.S_ISREG(held.st_mode)
                or (held.st_dev, held.st_ino, held.st_size, held.st_ctime_ns) !=
                (before.st_dev, before.st_ino, before.st_size, before.st_ctime_ns)):
            raise ValueError("native MCP identity changed before read")
        data = os.read(fd, MAX_OUTPUT + 1)
        after = os.fstat(fd)
        if (after.st_dev, after.st_ino, after.st_size, after.st_ctime_ns) != (before.st_dev, before.st_ino, before.st_size, before.st_ctime_ns):
            raise ValueError("native MCP identity changed after read")
    finally:
        os.close(fd)
    current = path.lstat()
    if len(data) != before.st_size or (current.st_dev, current.st_ino, current.st_ctime_ns) != (before.st_dev, before.st_ino, before.st_ctime_ns):
        raise ValueError("native MCP short read or replaced path")
    return data


def run(argv, *, cwd, env, seconds=90):
    # Native clients may leave subprocesses holding stdout open. Use bounded
    # temporary-file captures, a private process group and a hard deadline.
    with tempfile.TemporaryFile(mode="w+b") as stdout, tempfile.TemporaryFile(mode="w+b") as stderr:
        p = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                             stdout=stdout, stderr=stderr, start_new_session=True)
        timed_out = False
        try:
            p.wait(timeout=seconds)
        except subprocess.TimeoutExpired:
            timed_out = True
        finally:
            # A native parent can exit while background children retain file
            # descriptors or write into the disposable profile. Reap its group
            # on success and failure; never leave a detached fixture process.
            try:
                os.killpg(p.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            p.wait(timeout=5)
        if timed_out:
            raise ValueError("bounded native command timed out")
        stdout.seek(0)
        stderr.seek(0)
        out, err = stdout.read(MAX_OUTPUT + 1), stderr.read(MAX_OUTPUT + 1)
    if len(out) > MAX_OUTPUT or len(err) > MAX_OUTPUT:
        raise ValueError("bounded native CLI output exceeded")
    if p.returncode != 0:
        raise ValueError(f"native CLI command failed: {Path(argv[0]).name} exit {p.returncode}")
    return out, err


def read_servers(path):
    pairs = lambda rows: _unique_pairs(rows)
    value = json.loads(safe_read(path).decode("utf-8"), object_pairs_hook=pairs)
    if type(value) is not dict or set(value) != {"mcpServers"} or type(value["mcpServers"]) is not dict:
        raise ValueError("Kimi native MCP root shape differs")
    return value["mcpServers"]


def _unique_pairs(rows):
    result = {}
    for key, value in rows:
        if key in result:
            raise ValueError("duplicate MCP JSON member")
        result[key] = value
    return result


def verify(path, *, foreign=None):
    servers = read_servers(path)
    expected = {NAME}
    if foreign is not None:
        expected.add(FOREIGN_NAME)
        if servers.get(FOREIGN_NAME) != foreign:
            raise ValueError("Kimi foreign MCP entry changed during installation")
    if set(servers) != expected:
        raise ValueError("Kimi MCP not exactly the selected owned server")
    server = servers[NAME]
    if not isinstance(server, dict) or server.get("transport") != "stdio":
        raise ValueError("Kimi stdio MCP projection differs")
    command = server.get("command")
    if not isinstance(command, str) or not Path(command).is_absolute():
        raise ValueError("Kimi MCP lost its absolute source-bound command")
    # Lexical membership is insufficient: '..' and linked launchers can escape
    # the UAP-owned package even though is_relative_to() returns true.
    selected = Path(command)
    if (not selected.is_relative_to(path.parents[1] / ".uap")
            or selected.resolve() != selected or not selected.is_file()):
        raise ValueError("Kimi MCP command escaped selected UAP-managed project tree")
    return sha(safe_read(path))


def verify_removed(path, *, foreign=None):
    """Require owned removal and preservation of any preexisting foreign server."""
    try:
        path.lstat()
    except FileNotFoundError:
        if foreign is not None:
            raise ValueError("Kimi removal lost the foreign MCP configuration")
        return
    if not path.is_file():
        raise ValueError("Kimi removal left unexpected native MCP object")
    remaining = read_servers(path)
    if NAME in remaining:
        raise ValueError("Kimi owned MCP member persisted after removal")
    if foreign is not None and remaining != {FOREIGN_NAME: foreign}:
        raise ValueError("Kimi removal changed the foreign MCP configuration")


def smoke(installer, client, report_dir):
    if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("RUNNER_ENVIRONMENT") != "github-hosted":
        raise ValueError("genuine hosted disposable environment required")
    if not installer.is_file() or not client.is_file() or not installer.is_absolute() or not client.is_absolute():
        raise ValueError("exact selected source CLI and Kimi binary required")
    if not report_dir.is_dir() or report_dir.is_symlink():
        raise ValueError("ordinary artifact output directory required")
    with tempfile.TemporaryDirectory(prefix="uap-kimi-stable-mcp-", dir=os.environ["RUNNER_TEMP"]) as scratch:
        root = Path(scratch)
        home = root / "home"
        a, b = root / "project-a", root / "project-b"
        source = root / "source"
        for directory in (home, a, b, source / "bin"):
            directory.mkdir(parents=True, mode=0o700)
        for project in (a, b):
            run(["git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"], cwd=project,
                env={"PATH": os.environ["PATH"], "HOME": str(home), "GIT_CONFIG_NOSYSTEM": "1",
                     "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_TERMINAL_PROMPT": "0"}, seconds=15)
        (source / "plugin.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": NAME, "version": "1.0.0"}), encoding="utf-8")
        (source / "bin" / "probe").write_text("#!/usr/bin/env python3\n# Inert: never invoked by this qualification.\n", encoding="utf-8")
        (source / "bin" / "probe").chmod(0o700)
        (source / "mcp.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": {NAME: {"type": "stdio", "command": "./bin/probe", "args": ["--inert"]}}}), encoding="utf-8")
        # Do not inherit an ambient vendor credential, workspace-trust override
        # or developer profile from the hosted runner. No model authorization.
        env = {key: os.environ[key] for key in ("PATH", "LANG", "LC_ALL", "CI", "GITHUB_ACTIONS", "RUNNER_ENVIRONMENT")
               if key in os.environ}
        env.update({"HOME": str(home), "USERPROFILE": str(home),
                    "KIMI_CODE_HOME": str(home / ".kimi-code"),
                    "KIMI_CODE_NO_AUTO_UPDATE": "1", "GIT_CONFIG_NOSYSTEM": "1",
                    "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_TERMINAL_PROMPT": "0",
                    "XDG_CONFIG_HOME": str(home / ".config"),
                    "XDG_CACHE_HOME": str(home / ".cache"),
                    "XDG_DATA_HOME": str(home / ".local/share"),
                    "XDG_STATE_HOME": str(home / ".local/state"),
                    "TMPDIR": str(root / "tmp")})
        (root / "tmp").mkdir(mode=0o700)
        for name in (".config", ".cache", ".local/share", ".local/state", ".kimi-code"):
            (home / name).mkdir(parents=True, exist_ok=True, mode=0o700)
        version, _ = run([str(client), "--version"], cwd=a, env=env, seconds=45)
        if version.strip() != b"2.5.2":
            raise ValueError("real Kimi stable version is not 2.5.2")
        base = [str(installer), "--target", "kimi", "--format", "json", "--scope", "project", "--project-root", str(a)]
        before_b = (b / ".kimi-code").exists()
        if before_b or (a / ".kimi-code").exists():
            raise ValueError("disposable native project already contains Kimi state")
        # Exercise the real native merge path: UAP must preserve this existing
        # non-owned Project MCP entry through add, repeat and removal.
        native_mcp = a / ".kimi-code" / "mcp.json"
        native_mcp.parent.mkdir(mode=0o700)
        native_mcp.write_text(json.dumps({"mcpServers": {FOREIGN_NAME: FOREIGN_SERVER}}), encoding="utf-8")
        user_mcp = home / ".kimi-code" / "mcp.json"
        events = []
        for label, command in (("first", [str(installer), "add", str(source), "--no-save", *base[1:]]),
                               ("repeat", [str(installer), "add", str(source), "--no-save", *base[1:]])):
            out, _ = run(command, cwd=a, env=env)
            reply = json.loads(out)
            if not isinstance(reply, dict) or reply.get("result") != "success":
                raise ValueError("UAP native MCP add did not report success")
            digest = verify(native_mcp, foreign=FOREIGN_SERVER)
            if user_mcp.exists() or user_mcp.is_symlink():
                raise ValueError("Kimi Project add modified User MCP configuration")
            events.append({"stage": label, "mcp_sha256": digest})
        if events[0]["mcp_sha256"] != events[1]["mcp_sha256"]:
            raise ValueError("Kimi repeat add mutated native MCP config")
        if (b / ".kimi-code").exists():
            raise ValueError("Project B Kimi profile changed")
        # No login, trust override, session, MCP tool execution or model API.
        _, _ = run([str(installer), "remove", NAME, *base[1:]], cwd=a, env=env)
        remaining = a / ".kimi-code" / "mcp.json"
        verify_removed(remaining, foreign=FOREIGN_SERVER)
        if user_mcp.exists() or user_mcp.is_symlink():
            raise ValueError("Kimi Project-only install modified User MCP config")
        if (b / ".kimi-code").exists():
            raise ValueError("Kimi removal changed Project B")
        evidence = {"schema_version": 1, "client": "kimi", "real_client_version": "2.5.2",
                    "selected_scope": "project", "stages": events,
                    "native_mcp_config_prepared": True, "project_b_untouched": True,
                    "foreign_native_mcp_preserved": True,
                    "native_mcp_started": False, "tools_list": False, "tools_call": False,
                    "authentication_proven": False, "workspace_trust_granted": False}
        (report_dir / "kimi-mcp-prepared.json").write_text(json.dumps(evidence, indent=2) + "\n", encoding="utf-8")



LIFECYCLE_CASES = (
    "TestProjectKimiComposedSavedLifecycle",
    "TestProjectKimiPublicIntactRepairPreservesMCPSelection",
    "TestProjectKimiColdSavedImportPreservesPinnedAgreement",
)


def validate_lifecycle_json(filename, selected_executable):
    """Accept only three actual PASS cases with genuine probes and source records."""
    with filename.open("rb") as stream:
        body = stream.read(8 * MAX_OUTPUT + 1)
    if len(body) > 8 * MAX_OUTPUT:
        raise ValueError("Go lifecycle evidence exceeded bound")
    # go test -json may split a long t.Logf line across several Output events
    # (including in the middle of a JSON string). Reassemble complete lines per
    # test before decoding evidence; never accept a truncated marker at EOF.
    cases = {name: {"outcomes": [], "probes": [], "sources": [], "_pending": "",
                    "_started": False, "_finished": False} for name in LIFECYCLE_CASES}
    for line in body.splitlines():
        event = json.loads(line)
        if event.get("Test") not in cases:
            continue
        row = cases[event["Test"]]
        action = event.get("Action")
        if action == "run":
            if row["_started"] or row["_finished"]:
                raise ValueError("duplicate Go lifecycle test start")
            row["_started"] = True
        elif action in ("pass", "fail", "skip"):
            if not row["_started"] or row["_finished"] or row["_pending"]:
                raise ValueError("invalid Go lifecycle terminal event")
            row["_finished"] = True
            row["outcomes"].append(action)
        elif action not in ("output", "pause", "cont"):
            raise ValueError("unexpected Go lifecycle test action")
        if action == "run":
            continue
        if not row["_started"] or row["_finished"] and action not in ("pass", "fail", "skip"):
            raise ValueError("out-of-order Go lifecycle event")
        if action != "output":
            continue
        output = event.get("Output", "")
        if not isinstance(output, str):
            raise ValueError("invalid Go lifecycle Output field")
        pending = row["_pending"] + output
        if len(pending) > MAX_OUTPUT:
            raise ValueError("Go lifecycle output line exceeded bound")
        *complete_lines, row["_pending"] = pending.split("\n")
        for complete_line in complete_lines:
            for marker, key in (("KIMI_GENUINE_PROBE ", "probes"), ("KIMI_PREPARED_SOURCE ", "sources")):
                if marker in complete_line:
                    try:
                        record = json.loads(complete_line.split(marker, 1)[1])
                    except json.JSONDecodeError as exc:
                        raise ValueError("malformed genuine lifecycle evidence") from exc
                    if not isinstance(record, dict):
                        raise ValueError("genuine lifecycle evidence must be an object")
                    row[key].append(record)
    for name, row in cases.items():
        if row.pop("_pending"):
            raise ValueError("incomplete Go lifecycle output line: " + name)
        if not row.pop("_started") or not row.pop("_finished"):
            raise ValueError("missing genuine Go lifecycle run/finish: " + name)
        if row["outcomes"] != ["pass"] or not row["probes"] or not row["sources"]:
            raise ValueError("missing genuine lifecycle PASS/probe/source: " + name)
        if any(probe.get("version") != "2.5.2" or not selected_executable or probe.get("executable") != selected_executable or not probe.get("config_root") for probe in row["probes"]):
            raise ValueError("genuine lifecycle version/selection differs")
        if any(not source.get("source_binding_id") or not source.get("tree_digest") or not source.get("native_profile_root") for source in row["sources"]):
            raise ValueError("genuine lifecycle source attribution missing")
    cold = cases[LIFECYCLE_CASES[2]]
    if len({probe["config_root"] for probe in cold["probes"]}) < 2:
        raise ValueError("cold lifecycle did not probe both private applications")
    if len(cold["sources"]) < 2 or len({source["native_profile_root"] for source in cold["sources"]}) < 2:
        raise ValueError("cold lifecycle did not retain both source records")
    if len({(source["source_binding_id"], source["tree_digest"]) for source in cold["sources"]}) != 1:
        raise ValueError("cold lifecycle source agreement differs")
    return cases


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--cli", required=True, type=Path)
    p.add_argument("--client", required=True, type=Path)
    p.add_argument("--report-dir", required=True, type=Path)
    args = p.parse_args()
    smoke(args.cli, args.client, args.report_dir)


if __name__ == "__main__":
    main()
