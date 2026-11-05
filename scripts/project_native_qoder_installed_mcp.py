"""Genuine pinned Qoder discovery of an actual UAP-installed mixed Project package.

This is a source/configuration E2E, not authenticated MCP transport or tool calls.
Reuse the existing current-source hosted runner, pinned CLI and process capture.
"""
import hashlib
import json
from pathlib import Path


NAME = "uap-qoder-install-mcp"
STDIO = "uap_qoder_installed_stdio"
HTTP = "uap_qoder_installed_http"
PIN = "0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953"


def _write(path, content):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    with path.open("xb") as stream:
        stream.write(content)
    path.chmod(0o600)


def _json(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()


def run(ctx, original_env):
    """Require real native Qoder A/B listing across UAP add/replay/update/repair/remove."""
    root = ctx.root / "qoder-mcp-install-discovery"
    root.mkdir(mode=0o700)
    home, project_a, project_b, source = (
        root / "home", root / "a", root / "b", root / "source")
    for folder in (home, project_a, project_b, source, root / "tmp"):
        folder.mkdir(mode=0o700)
    env = dict(original_env)
    env.update({
        "HOME": str(home),
        "USERPROFILE": str(home),
        "XDG_CONFIG_HOME": str(root / "config"),
        "XDG_CACHE_HOME": str(root / "cache"),
        "XDG_DATA_HOME": str(root / "data"),
        "XDG_STATE_HOME": str(root / "state"),
        "TMPDIR": str(root / "tmp"),
        "GIT_CONFIG_GLOBAL": "/dev/null",
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_TERMINAL_PROMPT": "0",
        "NO_COLOR": "1",
        "TERM": "dumb",
    })
    for key in ("XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"):
        Path(env[key]).mkdir(mode=0o700)
    client = Path(env["AGENTPLUGINS_QODER_BIN"])
    installer = Path(env["AGENTPLUGINS_INSTALLER_BIN"])
    if (not client.is_absolute() or not installer.is_absolute()
            or not client.is_file() or not installer.is_file()
            or ctx.root not in client.parents or ctx.root not in installer.parents):
        raise ValueError("Qoder mixed discovery needs the selected immutable native/client executables")
    digest = hashlib.sha256()
    with client.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1 << 20), b""):
            digest.update(chunk)
    if digest.hexdigest() != PIN:
        raise ValueError("Qoder mixed discovery executable hash is not the reviewed pin")

    _write(source / "plugin.json", _json({
        "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
        "name": NAME, "version": "1.0.0", "description": "Disposable mixed project discovery",
    }))
    _write(source / "skills" / NAME / "SKILL.md",
           ("---\nname: " + NAME + "\ndescription: Disposable installed discovery\n---\n").encode())
    _write(source / "mcp.json", _json({
        "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
        "mcpServers": {
            STDIO: {"type": "stdio", "command": "python3", "args": ["-c", "pass"]},
            HTTP: {"type": "streamable-http", "url": "http://127.0.0.1:9/mcp"},
        },
    }))
    foreign = {"command": "true", "args": ["keep"]}
    _write(project_a / ".mcp.json", _json({
        "mcpServers": {"foreign_keep": foreign}, "other": "unchanged",
    }))
    marker = root / "foreign-marker"
    _write(marker, b"do-not-change\n")
    seen = []

    def invoke(label, argv, cwd, seconds=30):
        record, stdout, stderr = ctx.command(
            "qoder-mcp-" + label, [str(v) for v in argv], cwd, env, seconds)
        if record["exit"] != 0 or record.get("error") or record.get("deadline"):
            raise ValueError("Qoder installed-source command failed: " + label)
        return stdout, stderr

    for key, project in (("a", project_a), ("b", project_b)):
        invoke("git-" + key, ["/usr/bin/git", "-c", "core.hooksPath=/dev/null",
                               "init", "--quiet"], project)

    def listing(key, project, present):
        stdout, stderr = invoke("observe-" + key, [client, "mcp", "list"], project)
        text = (stdout + b"\n" + stderr).decode("utf-8", errors="strict")
        if "\x00" in text or "\x1b" in text:
            raise ValueError("Qoder native inventory emitted control bytes")
        found = {name: name in text for name in (STDIO, HTTP)}
        if any(value != present for value in found.values()):
            raise ValueError("genuine Qoder did not discover UAP-written stdio/HTTP in " + key)
        seen.append({"project": key.split("-")[0], "installed": present, "both_seen": all(found.values())})

    def operation(label, verb, selector, project):
        stdout, _ = invoke(label, [
            installer, verb, selector, "--scope", "project", "--project-root", project,
            "--target", "qoder", "--format", "json",
            *(["--no-save"] if verb == "add" else []),
        ], project, 45)
        answer = json.loads(stdout)
        if (not isinstance(answer, dict) or answer.get("command") != verb
                or answer.get("result") != "success"):
            raise ValueError("UAP returned a non-success project operation: " + label)
        if verb != "add":
            return None
        targets = answer.get("data", {}).get("targets")
        if not isinstance(targets, list) or len(targets) != 1 or targets[0].get("target") != "qoder":
            raise ValueError("Qoder add has no exact selected client receipt")
        value = targets[0].get("output", {}).get("result", {}).get("installation_id")
        if not isinstance(value, str) or len(value) != 36:
            raise ValueError("Qoder add has no stable installation ID")
        return value

    # All configurations in the project are produced by the source-built UAP,
    # never by this observer. Qoder only reads them.
    listing("a-before", project_a, False)
    listing("b-before", project_b, False)
    aid = operation("add-a", "add", source, project_a)
    listing("a-installed", project_a, True)
    listing("b-unaffected", project_b, False)
    bid = operation("add-b", "add", source, project_b)
    if aid == bid:
        raise ValueError("Qoder mixed Project A/B shared installation identity")
    listing("b-installed", project_b, True)
    if operation("repeat-add-a", "add", source, project_a) != aid:
        raise ValueError("Qoder repeated installation changed its receipt identity")
    operation("update-a", "update", source, project_a)
    listing("a-updated", project_a, True)

    native_skill = project_a / ".qoder" / "skills" / NAME / "SKILL.md"
    if not native_skill.is_file() or native_skill.is_symlink():
        raise ValueError("UAP-installed Qoder native Project Skill missing")
    native_skill.unlink()
    operation("repair-a", "repair", aid, project_a)
    if not native_skill.is_file() or native_skill.is_symlink():
        raise ValueError("Qoder repair did not restore its native Skill")
    listing("a-repaired", project_a, True)
    operation("remove-a", "remove", aid, project_a)
    listing("a-removed", project_a, False)
    listing("b-retained", project_b, True)
    operation("remove-b", "remove", bid, project_b)
    listing("b-removed", project_b, False)

    content = json.loads((project_a / ".mcp.json").read_text())
    if content.get("other") != "unchanged" or content.get("mcpServers", {}).get("foreign_keep") != foreign:
        raise ValueError("UAP changed foreign Qoder Project MCP")
    if marker.read_bytes() != b"do-not-change\n":
        raise ValueError("UAP changed independent foreign user data")
    if native_skill.exists() or native_skill.is_symlink():
        raise ValueError("Qoder removed Project Skill is still installed")
    return {"passed": True, "kind": "genuine-uap-installed-native-config-discovery",
            "runtime_mcp_calls_qualified": False, "observations": seen,
            "operations": ["add-a", "add-b", "repeat-add-a", "update-a",
                           "repair-a", "remove-a", "remove-b"]}
