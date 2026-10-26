"""Pinned Gemini source/trust observations, without installation or runtime proof.

The shared runner owns admission, acquisition, process bounds and artifacts.
The exact v0.65.0 publish-release action places gemini.js at the ZIP root;
the adjacent chunks/assets remain part of that verified distribution.
"""
import json
import re
import stat
from pathlib import Path

CLIENT = "gemini"
VERSION = "0.65.0"
AVAILABLE_PHASES = ("acquire", "observe")
ACQUISITION = {
    "url": "https://github.com/google-gemini/gemini-cli/releases/download/v0.65.0/gemini-cli-bundle.zip",
    "integrity": "sha256:bb5e7e879608bd9b3fe261930d3886fa56aacbf471da1655f2978eb35b705a73",
    "expected_bytes": 21215960,
    "max_bytes": 32 << 20,
    "format": "zip",
    "entrypoint": "gemini.js",
    "runtime": "node",
}
PREFIX = "uap-gemini-research-"
NAMES = {scope: PREFIX + scope for scope in ("a", "b", "user")}
URLS = {scope: "http://127.0.0.1:9/" + name for scope, name in NAMES.items()}
HELP = (
    ("top-help", ("--help",), "gemini"),
    ("skills-help", ("skills", "--help"), "list"),
    ("skills-list-help", ("skills", "list", "--help"), "all"),
    ("mcp-help", ("mcp", "--help"), "list"),
    ("mcp-list-help", ("mcp", "list", "--help"), "list"),
)
_SGR = re.compile(r"\x1b\[[0-9;]*m")
_SKILL = re.compile(r"([^\s].*?) \[(Enabled|Disabled)\]( \[Built-in\])?")
_MCP = re.compile(r"([✓✗○⛔…]) ([^:]+): (.+) - (Connected|Disconnected|Disabled|Blocked|Connecting)")
_INDICATORS = {"✓": "Connected", "✗": "Disconnected", "○": "Disabled", "⛔": "Blocked", "…": "Connecting"}
_UNTRUSTED = (
    "Warning: MCP servers are configured but disabled because this folder is untrusted.",
    "User-level servers are also suppressed in untrusted folders to prevent accidental side-effects.",
)
_RIPGREP_FALLBACK = b"Ripgrep is not available. Falling back to GrepTool.\n"
_EMPTY_MCP = b"No MCP servers configured.\n"
_CONFIG_COMMANDS = (("skills", "list", "--all"), ("mcp", "list"))


def _text(body):
    text = _SGR.sub("", body.decode("utf-8", errors="strict"))
    if "\x1b" in text or "\x00" in text:
        raise ValueError("unrecognized Gemini control sequence")
    return text.replace("\r\n", "\n").strip()


def parse_skills(body):
    """The exact pinned commands/skills/list.ts text shape, including paths."""
    text = _text(body)
    if text == "No skills discovered.":
        return {}
    lines = [line for line in text.splitlines() if line.strip()]
    if not lines or lines.pop(0) != "Discovered Agent Skills:" or not lines or len(lines) % 3:
        raise ValueError("unrecognized Gemini skill listing")
    result = {}
    for index in range(0, len(lines), 3):
        match = _SKILL.fullmatch(lines[index])
        description, location = lines[index + 1:index + 3]
        if (not match or not description.startswith("  Description: ")
                or not location.startswith("  Location:    ")):
            raise ValueError("malformed Gemini skill row")
        name = match[1]
        path = location.removeprefix("  Location:    ")
        if not path or name in result or len(result) >= 256:
            raise ValueError("ambiguous or excessive Gemini skills")
        result[name] = {"enabled": match[2] == "Enabled", "builtin": bool(match[3]), "path": path}
    return result


def parse_mcp(body):
    """Parse configured source visibility; connection labels are not proof."""
    text = _text(body)
    if text == "No MCP servers configured.":
        return {}, False
    lines = [line for line in text.splitlines() if line.strip()]
    untrusted = tuple(lines[:2]) == _UNTRUSTED
    if untrusted:
        lines = lines[2:]
    if not lines or lines.pop(0) != "Configured MCP servers:" or not lines:
        raise ValueError("unrecognized Gemini MCP listing")
    result = {}
    for line in lines:
        match = _MCP.fullmatch(line)
        if not match or _INDICATORS[match[1]] != match[4] or match[2] in result or len(result) >= 256:
            raise ValueError("malformed or ambiguous Gemini MCP row")
        result[match[2]] = {"configuration": match[3], "status": match[4]}
    return result, untrusted


def _parents(path):
    if any(parent.is_symlink() for parent in path.parents):
        raise ValueError("Gemini fixture parent alias refused")


def _unchanged(path, body):
    _parents(path)
    info = path.lstat()
    if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1
            or info.st_size != len(body) or stat.S_IMODE(info.st_mode) != 0o600
            or path.read_bytes() != body):
        raise ValueError("Gemini modified authored fixture bytes/mode")


def _write(path, document, owned):
    body = (json.dumps(document, indent=2, sort_keys=True) + "\n").encode()
    _parents(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    _parents(path)
    if path in owned:
        _unchanged(path, owned[path])
        path.write_bytes(body)
    else:
        with path.open("xb") as stream:
            stream.write(body)
    path.chmod(0o600)
    owned[path] = body


def _probe(ctx, owned, label, arguments, project="a"):
    record, stdout, stderr = ctx.probe(label, list(arguments), project=project, timeout=30)
    if (not isinstance(record, dict) or type(record.get("exit")) is not int or record["exit"] != 0
            or "error" not in record or record["error"] is not None
            or type(stdout) is not bytes or type(stderr) is not bytes):
        raise ValueError("Gemini probe did not finish cleanly: " + label)
    # The pinned client reports its optional native search-backend fallback
    # while loading configuration. Actual hosted run 40238929259 measured this
    # exact 52-byte warning on a successful skills command. Keep those bytes in
    # evidence; version/help and every other diagnostic remain strict.
    known_stderr = tuple(arguments) in _CONFIG_COMMANDS and stderr == _RIPGREP_FALLBACK
    # Actual hosted run 40240850774 emitted the empty-list sentinel on stderr,
    # with zero stdout bytes and exit zero. Preserve both raw capture channels;
    # only this whole measured response supplies the existing empty-list parser.
    empty_mcp = tuple(arguments) == ("mcp", "list") and stdout == b"" and stderr == _EMPTY_MCP
    if stderr and not known_stderr and not empty_mcp:
        raise ValueError("Gemini probe did not finish cleanly: " + label)
    for path, body in owned.items():
        _unchanged(path, body)
    return stderr if empty_mcp else stdout


def _require_skills(rows, scopes, paths):
    selected = {name: row for name, row in rows.items() if name.startswith(PREFIX)}
    if set(selected) != {NAMES[scope] for scope in scopes}:
        raise ValueError("Gemini selected another project/user skill source")
    for scope in scopes:
        if selected[NAMES[scope]] != {"enabled": True, "builtin": False, "path": str(paths[scope])}:
            raise ValueError("Gemini skill path or enablement differs")
    return selected


def _require_mcp(rows, scopes, status):
    if set(rows) != {NAMES[scope] for scope in scopes}:
        raise ValueError("Gemini selected another project/user MCP source")
    for scope in scopes:
        if rows[NAMES[scope]] != {"configuration": URLS[scope] + " (http)", "status": status}:
            raise ValueError("Gemini MCP configuration or policy differs")
    return rows


def collect(ctx):
    """Observe inert named sources and genuine documented trust-policy effects."""
    root, home = Path(ctx.root).resolve(), Path(ctx.home).resolve()
    projects = {name: Path(ctx.projects[name]).resolve() for name in ("a", "b", "user")}
    if (len(set(projects.values())) != 3 or not home.is_relative_to(root)
            or any(not path.is_relative_to(root) or home.is_relative_to(path) or path.is_relative_to(home)
                   for path in projects.values())):
        raise ValueError("Gemini requires distinct projects and original user home")
    owned = {}
    settings = {"security": {"folderTrust": {"enabled": True}},
                "privacy": {"usageStatisticsEnabled": False}, "telemetry": {"enabled": False},
                "mcp": {"excluded": list(NAMES.values())}}
    user_settings, trust_path = home / ".gemini/settings.json", home / ".gemini/trustedFolders.json"
    trust = {str(path): "TRUST_FOLDER" for path in projects.values()}
    _write(user_settings, settings, owned)
    _write(trust_path, trust, owned)
    version = _text(_probe(ctx, owned, "version", ("--version",)))
    if version != VERSION:
        raise ValueError("Gemini version differs from immutable pin")
    for label, arguments, expected in HELP:
        if expected not in _text(_probe(ctx, owned, label, arguments)).lower():
            raise ValueError("Gemini pinned command help is unrecognized: " + label)
    for scope in ("a", "b"):
        _require_skills(parse_skills(_probe(ctx, owned, "empty-" + scope + "-skills", ("skills", "list", "--all"), scope)), (), {})
        rows, untrusted = parse_mcp(_probe(ctx, owned, "empty-" + scope + "-mcp", ("mcp", "list"), scope))
        if rows or untrusted:
            raise ValueError("Gemini empty baseline contains MCP configuration")
    paths = {}
    for scope in ("a", "b", "user"):
        native = (home if scope == "user" else projects[scope]) / ".gemini"
        path = native / "skills" / NAMES[scope] / "SKILL.md"
        _parents(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        _parents(path)
        body = ("---\nname: " + NAMES[scope] + "\ndescription: Inert source marker\n---\n\nText only.\n").encode()
        with path.open("xb") as stream:
            stream.write(body)
        path.chmod(0o600)
        paths[scope], owned[path] = path, body
        server = {"mcpServers": {NAMES[scope]: {"httpUrl": URLS[scope]}}}
        _write(user_settings if scope == "user" else native / "settings.json", {**settings, **server} if scope == "user" else server, owned)
    observations = {"client_version_measured": version, "scopes": {},
                    "surface": "native skills and policy-blocked MCP configuration source listing",
                    "trust_setup": "documented exact fixture-owned trustedFolders.json; no trust dialog observed",
                    "uap_lifecycle_qualified": False, "mcp_initialization_qualified": False,
                    "mcp_tools_list_qualified": False, "mcp_tools_call_qualified": False,
                    "authentication_qualified": False, "real_model_qualified": False}
    for scope in ("a", "b", "user"):
        selected = ("user", scope) if scope != "user" else ("user",)
        skills = parse_skills(_probe(ctx, owned, "configured-" + scope + "-skills", ("skills", "list", "--all"), scope))
        servers, untrusted = parse_mcp(_probe(ctx, owned, "configured-" + scope + "-mcp", ("mcp", "list"), scope))
        if untrusted:
            raise ValueError("Gemini ignored authored exact folder trust")
        observations["scopes"][scope] = {"skills": _require_skills(skills, selected, paths),
                                           "mcp": _require_mcp(servers, selected, "Blocked")}
    trust[str(projects["a"])] = "DO_NOT_TRUST"
    _write(trust_path, trust, owned)
    skills = parse_skills(_probe(ctx, owned, "untrusted-a-skills", ("skills", "list", "--all")))
    servers, untrusted = parse_mcp(_probe(ctx, owned, "untrusted-a-mcp", ("mcp", "list")))
    if not untrusted:
        raise ValueError("Gemini untrusted MCP warning was not observed")
    # v0.65.0 checks the exclusion policy before the folder-trust branch.
    # These servers stay Blocked; the warning and skill suppression prove the
    # independently authored untrusted selection, not a different MCP status.
    observations["untrusted_a"] = {"skills": _require_skills(skills, ("user",), paths),
                                      "mcp": _require_mcp(servers, ("user", "a"), "Blocked")}
    return {"observations": observations, "cases": [
        {"id": "gemini.help", "status": "passed", "reason": "exact pinned version and fixed command help observed"},
        {"id": "gemini.scope-observation", "status": "passed",
         "reason": "fresh native commands selected exact A/B/user skills and policy-blocked MCP sources; explicit untrusted control observed; no runtime or UAP lifecycle claim"},
    ]}
