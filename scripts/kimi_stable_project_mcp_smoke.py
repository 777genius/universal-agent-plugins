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
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
    try:
        held = os.fstat(fd)
        if (held.st_dev, held.st_ino, held.st_size, held.st_ctime_ns) != (before.st_dev, before.st_ino, before.st_size, before.st_ctime_ns):
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
        try:
            p.wait(timeout=seconds)
        except subprocess.TimeoutExpired:
            os.killpg(p.pid, signal.SIGKILL)
            p.wait(timeout=5)
            raise ValueError("bounded native command timed out") from None
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


def verify(path):
    servers = read_servers(path)
    if set(servers) != {NAME}:
        raise ValueError("Kimi MCP not exactly the selected owned server")
    server = servers[NAME]
    if not isinstance(server, dict) or server.get("transport") != "stdio":
        raise ValueError("Kimi stdio MCP projection differs")
    command = server.get("command")
    if not isinstance(command, str) or not Path(command).is_absolute():
        raise ValueError("Kimi MCP lost its absolute source-bound command")
    if not Path(command).is_relative_to(path.parents[1] / ".uap"):
        raise ValueError("Kimi MCP command escaped selected UAP-managed project tree")
    return sha(safe_read(path))


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
        events = []
        for label, command in (("first", [str(installer), "add", str(source), "--no-save", *base[1:]]),
                               ("repeat", [str(installer), "add", str(source), "--no-save", *base[1:]])):
            out, _ = run(command, cwd=a, env=env)
            reply = json.loads(out)
            if not isinstance(reply, dict) or reply.get("result") != "success":
                raise ValueError("UAP native MCP add did not report success")
            digest = verify(a / ".kimi-code" / "mcp.json")
            events.append({"stage": label, "mcp_sha256": digest})
        if events[0]["mcp_sha256"] != events[1]["mcp_sha256"]:
            raise ValueError("Kimi repeat add mutated native MCP config")
        if (b / ".kimi-code").exists():
            raise ValueError("Project B Kimi profile changed")
        # No login, trust override, session, MCP tool execution or model API.
        _, _ = run([str(installer), "remove", NAME, *base[1:]], cwd=a, env=env)
        remaining = a / ".kimi-code" / "mcp.json"
        if remaining.is_file() and NAME in read_servers(remaining):
            raise ValueError("Kimi owned MCP member persisted after removal")
        if (b / ".kimi-code").exists():
            raise ValueError("Kimi removal changed Project B")
        evidence = {"schema_version": 1, "client": "kimi", "real_client_version": "2.5.2",
                    "selected_scope": "project", "stages": events,
                    "native_mcp_config_prepared": True, "project_b_untouched": True,
                    "native_mcp_started": False, "tools_list": False, "tools_call": False,
                    "authentication_proven": False, "workspace_trust_granted": False}
        (report_dir / "kimi-mcp-prepared.json").write_text(json.dumps(evidence, indent=2) + "\n", encoding="utf-8")


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--cli", required=True, type=Path)
    p.add_argument("--client", required=True, type=Path)
    p.add_argument("--report-dir", required=True, type=Path)
    args = p.parse_args()
    smoke(args.cli, args.client, args.report_dir)


if __name__ == "__main__":
    main()
