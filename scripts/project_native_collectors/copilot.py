"""Copilot's pinned, account-free project skill and MCP source observations.

Listings are configuration discovery, not a connected session or tool call.
The shared runner owns all acquisition, admission, process and evidence bounds.
"""
import json
import os
import stat
from pathlib import Path

CLIENT = "copilot"
VERSION = "1.0.103"
AVAILABLE_PHASES = ("acquire", "observe")
ACQUISITION = {
    "url": "https://github.com/github/copilot-cli/releases/download/v1.0.103/copilot-linux-x64.tar.gz",
    "integrity": "sha256:9aeb4430e9ac9523546db07b227c7aba179a4710a55f9ac29ad143b0f9e1b325",
    "expected_bytes": 129018878,
    "binary_sha256": "f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d",
    "max_bytes": 144 << 20,
    "max_expanded_bytes": 256 << 20,
    "format": "tar.gz",
    "entrypoint": "copilot",
    "runtime": "native",
}
PREFIX = "uap-copilot-research-"
NAMES = {scope: PREFIX + scope for scope in ("a", "b", "user", "shadow")}
URLS = {scope: "http://127.0.0.1:9/" + name for scope, name in NAMES.items()}
HELP = (
    ("top-help", ("--help",), "copilot"),
    ("providers-help", ("help", "providers"), "COPILOT_PROVIDER_BASE_URL"),
    ("skill-list-help", ("skill", "list", "--help"), "--json"),
    ("mcp-list-help", ("mcp", "list", "--help"), "--json"),
    ("mcp-get-help", ("mcp", "get", "--help"), "--json"),
)


def _unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate Copilot JSON key")
        result[key] = value
    return result


def _json(body):
    return json.loads(body.decode("utf-8", errors="strict"), object_pairs_hook=_unique_object,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite Copilot JSON")))


def parse_skills(body):
    rows = _json(body)
    if not isinstance(rows, list) or len(rows) > 256:
        raise ValueError("unrecognized Copilot skill list")
    result = {}
    for row in rows:
        if (not isinstance(row, dict) or not isinstance(row.get("name"), str) or not row["name"]
                or not isinstance(row.get("source"), str) or not row["source"]
                or type(row.get("enabled")) is not bool or row["name"] in result):
            raise ValueError("malformed or ambiguous Copilot skill row")
        # Built-ins need not have a disk path. Fixture project rows must below.
        path = row.get("path")
        if path is not None and not isinstance(path, str):
            raise ValueError("invalid Copilot skill path")
        result[row["name"]] = {"source": row["source"], "enabled": row["enabled"], "path": path}
    return result


def parse_mcp(body):
    document = _json(body)
    if not isinstance(document, dict) or not isinstance(document.get("mcpServers"), dict):
        raise ValueError("unrecognized Copilot MCP list")
    rows = document["mcpServers"]
    if len(rows) > 256:
        raise ValueError("excessive Copilot MCP list")
    result = {}
    for name, row in rows.items():
        if (not name or not isinstance(row, dict) or not isinstance(row.get("source"), str)
                or not row["source"] or type(row.get("enabled")) is not bool):
            raise ValueError("malformed Copilot MCP row")
        result[name] = {"source": row["source"], "enabled": row["enabled"],
                        "type": row.get("type"), "url": row.get("url")}
    return result


def _parents(path):
    if any(parent.is_symlink() for parent in path.parents):
        raise ValueError("Copilot fixture parent alias refused")


def _read_trust(path):
    _parents(path)
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, "rb") as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size > 64 << 10:
            raise ValueError("unsafe Copilot native trust state")
        body = stream.read((64 << 10) + 1)
        if len(body) != info.st_size or _identity(os.fstat(stream.fileno())) != _identity(info):
            raise ValueError("Copilot native trust changed during read")
    if _identity(path.lstat()) != _identity(info):
        raise ValueError("Copilot native trust path changed during read")
    return _json(body), info


def _identity(info):
    return info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns


def _write_trust(path, body, previous):
    _parents(path)
    flags = os.O_WRONLY | os.O_NOFOLLOW | os.O_NONBLOCK
    if previous is None:
        flags |= os.O_CREAT | os.O_EXCL
    descriptor = os.open(path, flags, 0o600)
    with os.fdopen(descriptor, "wb") as stream:
        current = os.fstat(stream.fileno())
        if previous is not None and _identity(current) != _identity(previous):
            raise ValueError("Copilot native trust identity changed before write")
        if not stat.S_ISREG(current.st_mode) or current.st_nlink != 1:
            raise ValueError("unsafe Copilot native trust write")
        _parents(path)
        stream.truncate(0)
        stream.write(body)
        os.fchmod(stream.fileno(), 0o600)


def _write(path, document, owned):
    body = (json.dumps(document, sort_keys=True, indent=2) + "\n").encode()
    _parents(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    _parents(path)
    with path.open("xb") as stream:
        stream.write(body)
    path.chmod(0o600)
    owned[path] = body


def _probe(ctx, owned, label, arguments, project="a"):
    record, stdout, stderr = ctx.probe(label, list(arguments), project=project,
                                      extra_env={"COPILOT_OFFLINE": "true"}, timeout=30)
    if record.get("exit") != 0 or record.get("error") or stderr.strip():
        raise ValueError("Copilot probe did not finish cleanly: " + label)
    for path, body in owned.items():
        _parents(path)
        info = path.lstat()
        if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size != len(body)
                or stat.S_IMODE(info.st_mode) != 0o600 or path.read_bytes() != body):
            raise ValueError("Copilot changed authored source bytes/mode")
    return stdout


def _skills(rows, scope, paths):
    selected = {name: row for name, row in rows.items() if name.startswith(PREFIX)}
    expected = {} if scope == "user" else {
        NAMES[scope]: {"source": "project", "enabled": True, "path": str(paths[scope])}}
    if selected != expected:
        raise ValueError("Copilot skill source/path/enablement differs")
    return selected


def _mcp(rows, scope, trusted):
    selected = {name: row for name, row in rows.items() if name.startswith(PREFIX)}
    scopes = ("user", scope) if trusted and scope != "user" else ("user",)
    expected = {NAMES[item]: {"source": "user" if item == "user" else "workspace", "enabled": True,
                             "type": "http", "url": URLS[item]} for item in scopes}
    if selected != expected:
        raise ValueError("Copilot MCP source/selection/enablement differs")
    return selected


def collect(ctx):
    root, home = Path(ctx.root).resolve(), Path(ctx.home).resolve()
    projects = {name: Path(ctx.projects[name]).resolve() for name in ("a", "b", "user")}
    if (len(set(projects.values())) != 3 or not home.is_relative_to(root)
            or any(not path.is_relative_to(root) or home.is_relative_to(path) or path.is_relative_to(home)
                   for path in projects.values())):
        raise ValueError("Copilot requires distinct projects and original user home")
    owned = {}
    version = _probe(ctx, owned, "version", ("--version",)).decode("utf-8", errors="strict").strip()
    if version != "GitHub Copilot CLI " + VERSION + ".":
        raise ValueError("Copilot version differs from immutable pin")
    for label, arguments, token in HELP:
        if token not in _probe(ctx, owned, label, arguments).decode("utf-8", errors="strict"):
            raise ValueError("unrecognized pinned Copilot help: " + label)
    for scope in ("a", "b"):
        if any(name.startswith(PREFIX) for name in parse_skills(_probe(ctx, owned, "empty-" + scope + "-skills", ("skill", "list", "--json"), scope))):
            raise ValueError("Copilot empty baseline contains fixture skill")
        if any(name.startswith(PREFIX) for name in parse_mcp(_probe(ctx, owned, "empty-" + scope + "-mcp", ("mcp", "list", "--json"), scope))):
            raise ValueError("Copilot empty baseline contains fixture MCP")
    paths = {}
    for scope in ("a", "b"):
        paths[scope] = projects[scope] / ".github/skills" / NAMES[scope]
        path = paths[scope] / "SKILL.md"
        _parents(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        _parents(path)
        body = ("---\nname: " + NAMES[scope] + "\ndescription: Inert source marker\n---\n\nText only.\n").encode()
        with path.open("xb") as stream:
            stream.write(body)
        path.chmod(0o600)
        owned[path] = body
    server = lambda scope: {"type": "http", "url": URLS[scope], "tools": ["*"]}
    _write(home / ".copilot/mcp-config.json", {"mcpServers": {NAMES["user"]: server("user")}}, owned)
    _write(projects["a"] / ".mcp.json", {"mcpServers": {NAMES["a"]: server("a")}}, owned)
    _write(projects["a"] / ".github/mcp.json", {"mcpServers": {NAMES["shadow"]: server("shadow")}}, owned)
    _write(projects["b"] / ".github/mcp.json", {"mcpServers": {NAMES["b"]: server("b")}}, owned)
    facts = {"client_version_measured": version, "scopes": {}, "provider": "none",
             "surface": "native skill and configured MCP source listings; no session startup",
             "uap_lifecycle_qualified": False, "mcp_initialization_qualified": False,
             "mcp_tools_list_qualified": False, "mcp_tools_call_qualified": False,
             "authentication_qualified": False, "real_model_qualified": False}
    skills = parse_skills(_probe(ctx, owned, "untrusted-a-skills", ("skill", "list", "--json")))
    servers = parse_mcp(_probe(ctx, owned, "untrusted-a-mcp", ("mcp", "list", "--json")))
    facts["untrusted_a"] = {"skills": _skills(skills, "a", paths), "mcp": _mcp(servers, "a", False)}
    # This is documented original-user trust selection, not observed UI consent.
    trust_path = home / ".copilot/config.json"
    _parents(trust_path)
    try:
        previous = trust_path.lstat()
    except FileNotFoundError:
        previous = None
    if previous is not None:
        if not stat.S_ISREG(previous.st_mode):
            raise ValueError("unsafe Copilot native trust state")
        existing, previous = _read_trust(trust_path)
        if not isinstance(existing, dict) or existing.get("trustedFolders"):
            raise ValueError("Copilot baseline has unexpected folder authority")
    else:
        existing = {}
    existing["trustedFolders"] = [str(projects[scope]) for scope in ("a", "b")]
    body = (json.dumps(existing, sort_keys=True, indent=2) + "\n").encode()
    trust_path.parent.mkdir(parents=True, exist_ok=True)
    _parents(trust_path)
    _write_trust(trust_path, body, previous)
    for scope in ("a", "b", "user"):
        skills = parse_skills(_probe(ctx, owned, "trusted-" + scope + "-skills", ("skill", "list", "--json"), scope))
        servers = parse_mcp(_probe(ctx, owned, "trusted-" + scope + "-mcp", ("mcp", "list", "--json"), scope))
        current, _ = _read_trust(trust_path)
        if not isinstance(current, dict) or current.get("trustedFolders") != existing["trustedFolders"]:
            raise ValueError("Copilot changed exact authored folder trust")
        facts["scopes"][scope] = {"skills": _skills(skills, scope, paths), "mcp": _mcp(servers, scope, True)}
    facts["trust_setup"] = "documented exact original-profile trustedFolders; no trust dialog observed"
    return {"observations": facts, "cases": [
        {"id": "copilot.help", "status": "passed", "reason": "exact pinned version and fixed CLI/provider help observed"},
        {"id": "copilot.scope-observation", "status": "passed",
         "reason": "native A/B skill paths and trusted/untrusted user/workspace MCP source selection observed; root .mcp.json precedence; no runtime or UAP lifecycle claim"},
    ]}
