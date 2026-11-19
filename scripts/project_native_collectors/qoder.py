"""Pinned Qoder version/fixed help research, without discovery or runtime proof.

The genuine acquisition selected qodercli; shared code owns immutable bytes,
host admission, bounded execution and captures. Never import or list a plugin.
"""
import hashlib
import json
import re

CLIENT = "qoder"
VERSION = "1.1.78"
AVAILABLE_PHASES = ("acquire", "help", "observe")
ACQUISITION = {
    "url": "https://qoder-ide.oss-accelerate.aliyuncs.com/qodercli/releases/1.1.78/qodercli-linux-x64.tar.gz",
    "integrity": "sha256:5e3213d2d4256c86c5ad2c8bf73fd392f92ba80d0d38f822806542805a48a8ea",
    "max_bytes": 96 << 20,
    "expected_bytes": 78_115_289,
    "format": "tar.gz",
    "entrypoint": "qodercli",
    "runtime": "native",
    "binary_sha256": "0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953",
}
HELP = (
    ("top-help", ("--help",), "Usage: qodercli [options] [command] [query...]"),
    ("plugins-help", ("plugins", "--help"), "Usage: qodercli plugins|plugin [options] [command]"),
    ("plugins-list-help", ("plugins", "list", "--help"), "Usage: qodercli plugins list [options]"),
    ("mcp-help", ("mcp", "--help"), "Usage: qodercli mcp [options] [command]"),
)


def parse_version(body):
    if body != b"1.1.78\n":
        raise ValueError("Qoder version differs from immutable pin")
    return VERSION


def parse_help(body, expected_usage):
    text = body.decode("utf-8", errors="strict")
    if any((ord(char) < 32 and char not in "\n\t") or ord(char) == 127 for char in text):
        raise ValueError("Qoder help contains control characters")
    lines = text.splitlines()
    if (not lines or lines[0] != expected_usage
            or sum(line.startswith("Usage:") for line in lines) != 1
            or not re.search(r"(?m)^  -h, --help[ \t]+[^\n]+$", text)):
        raise ValueError("Qoder command-specific help grammar differs")
    return lines[0]


def _probe(ctx, label, arguments):
    record, stdout, stderr = ctx.probe(label, list(arguments), project="a", timeout=30)
    if (not isinstance(record, dict) or type(record.get("exit")) is not int
            or record["exit"] != 0 or "error" not in record or record["error"] is not None
            or not isinstance(stdout, bytes) or not isinstance(stderr, bytes) or stderr != b""):
        raise ValueError("Qoder probe did not finish cleanly: " + label)
    return stdout


def _help_section(body, heading):
    text = body.decode("utf-8", errors="strict")
    sections = text.split("\n" + heading + ":\n")
    return sections[1].lstrip("\n").split("\n\n", 1)[0].splitlines() if len(sections) == 2 else []


def _help_entry(body, heading, token):
    rows = [line for line in _help_section(body, heading)
            if re.match(r"  " + token + r"(?:[ \t]|$)", line)]
    return rows[0] if len(rows) == 1 else ""


def _skill_help(body, listing=False):
    usage = body.decode("utf-8", errors="strict").splitlines()
    suffix = r" list(?: \[options\])?" if listing else r"(?: \[options\])?(?: \[command\])?"
    if not usage or re.fullmatch(r"Usage: qodercli skills(?:\|skill)?" + suffix, usage[0]) is None:
        raise ValueError("Qoder Skills command-specific help grammar differs")
    return parse_help(body, usage[0])


def _skill_catalogs(ctx, stage):
    rows = {}
    for project in ("a", "b", "user"):
        label = "skills-" + stage + "-" + project
        record, stdout, stderr = ctx.probe(label, ["skills", "list", "--json"],
                                           project=project, timeout=30)
        if (not isinstance(record, dict) or type(record.get("exit")) is not int
                or record["exit"] != 0 or record.get("error") is not None
                or "error" not in record or type(stdout) is not bytes
                or type(stderr) is not bytes or stderr != b""):
            raise ValueError("Qoder opaque Skills capture did not finish cleanly")
        rows[project] = {"probe": label, "arguments": ["skills", "list", "--json"],
                         "project": project, "surface": "opaque-skills-catalog"}
    return rows


def _author_inert_skills(ctx):
    authored = []
    for label, base in (("a", ctx.projects["a"]), ("b", ctx.projects["b"]), ("user", ctx.home)):
        for parent in (base / ".qoder", base / ".qoder" / "skills"):
            if parent.is_symlink() or parent.resolve() != parent:
                raise ValueError("Qoder inert Skill parent is redirected")
            parent.mkdir(mode=0o700, exist_ok=True)
        name = "uap-qoder-inert-" + label
        directory = base / ".qoder" / "skills" / name
        directory.mkdir(mode=0o700)
        path = directory / "SKILL.md"
        body = ("---\nname: " + name + "\ndescription: Inert Qoder " + label
                + " discovery marker\n---\nDo not execute; discovery fixture only.\n").encode()
        with path.open("xb") as stream:
            stream.write(body)
        path.chmod(0o600)
        ctx.identities[path] = hashlib.sha256(body).hexdigest()
        authored.append({"project": label, "name": name, "path": str(path),
                         "sha256": ctx.identities[path]})
    return authored


def observe_skills_surface(ctx, top_help):
    """Help gates a closed read-only vector; raw catalogs never qualify discovery."""
    result = {"status": "not-advertised", "native_skill_discovery_qualified": False,
              "public_ready_qualified": False}
    skills = _help_entry(top_help, "Commands", r"skills(?:\|skill)?")
    if re.fullmatch(r"  skills(?:\|skill)?(?: \[options\])?(?: \[command\])?[ \t]{2,}[^\n]+", skills) is None:
        return result
    body = _probe(ctx, "skills-help", ("skills", "--help"))
    result.update(status="help-observed", usage=_skill_help(body))
    listing = _help_entry(body, "Commands", "list")
    if re.fullmatch(r"  list(?: \[options\])?[ \t]{2,}(?:List\b[^\n]*|Lists discovered agent skills\.)", listing) is None:
        return result
    body = _probe(ctx, "skills-list-help", ("skills", "list", "--help"))
    result["list_usage"] = _skill_help(body, listing=True)
    option = _help_entry(body, "Options", r"(?:-[A-Za-z], )?--json")
    if re.fullmatch(r"  (?:-[A-Za-z], )?--json[ \t]{2,}[^\n]+", option) is None:
        return result
    result.update(status="opaque-catalog-captured", baseline=_skill_catalogs(ctx, "empty"))
    if getattr(ctx, "phase", "help") == "observe":
        result["authored_inert_skills"] = _author_inert_skills(ctx)
        result["populated"] = _skill_catalogs(ctx, "populated")
    return result


def observe_project_mcp_sources(ctx):
    """Record genuine Qoder MCP inventory, without a model or MCP tool call."""
    a, b = (ctx.projects[name] for name in ("a", "b"))
    # The common hosted collector has already initialized A/B as Git roots.
    # Reusing its git-init-a/b labels would fail the unique-capture contract.
    for project in (a, b):
        git = project / ".git"
        if (not project.is_dir() or project.is_symlink()
                or not git.is_dir() or git.is_symlink()):
            raise ValueError("Qoder selected project is not an existing ordinary Git root")
    path = a / ".mcp.json"
    if path.exists() or path.is_symlink():
        raise ValueError("Qoder project MCP fixture would replace existing state")
    names = ("uap_qoder_a_stdio", "uap_qoder_a_http")
    body = (json.dumps({"mcpServers": {
        names[0]: {"command": "/usr/bin/true", "args": []},
        names[1]: {"type": "http", "url": "http://127.0.0.1:9/mcp"}
    }}, sort_keys=True) + "\n").encode()
    with path.open("xb") as stream:
        stream.write(body)
    path.chmod(0o600)
    observations = {}
    for name in ("a", "b"):
        record, stdout, stderr = ctx.probe("mcp-list-" + name, ["mcp", "list"],
                                           project=name, timeout=30)
        if (type(record.get("exit")) is not int or record["exit"] != 0
                or record.get("error") is not None
                or type(stdout) is not bytes or type(stderr) is not bytes):
            raise ValueError("Qoder native MCP source listing failed for " + name)
        listing = (stdout + b"\n" + stderr).decode("utf-8", errors="strict")
        if "\x00" in listing or "\x1b" in listing:
            raise ValueError("Qoder native MCP listing contains control codes")
        selected = {key: key in listing for key in names}
        if any(value != (name == "a") for value in selected.values()):
            raise ValueError("Qoder project MCP source/priority was not isolated to A")
        observations[name] = {"selected": selected, "command": ["mcp", "list"],
                              "source_only": True}
    if path.is_symlink() or path.read_bytes() != body:
        raise ValueError("Qoder native MCP source listing changed authored config")
    return observations


def collect(ctx):
    """Preserve fixed help/MCP probes before optional opaque Skills research."""
    version = parse_version(_probe(ctx, "version", ("--version",)))
    help_rows, top_help = [], b""
    for label, arguments, usage in HELP:
        body = _probe(ctx, label, arguments)
        observed = parse_help(body, usage)
        if label == "top-help":
            top_help = body
        help_rows.append({"label": label, "arguments": list(arguments), "usage": observed})
    result = {"observations": {"client_version_measured": version, "help": help_rows,
                               "surface": "fixed version and command help only",
                               "scope_discovery_qualified": False, "uap_lifecycle_qualified": False,
                               "mcp_initialization_qualified": False, "mcp_tools_list_qualified": False,
                               "mcp_tools_call_qualified": False, "authentication_qualified": False,
                               "real_model_qualified": False},
              "cases": [{"id": "qoder.help", "status": "passed",
                         "reason": "Pinned fixed version/help table observed; no plugin operation, scope or runtime qualification"}]}
    if getattr(ctx, "phase", "help") == "observe":
        result["observations"]["project_mcp_inventory"] = observe_project_mcp_sources(ctx)
        result["observations"]["scope_discovery_qualified"] = True
        result["observations"]["surface"] = "genuine pinned Qoder mcp list A/B project source inventory only"
        result["cases"].append({"id": "qoder.scope-observation", "status": "passed",
                                "reason": "Genuine Qoder A-only project stdio/HTTP configuration listed; no MCP tools/call"})
        result["unobserved"] = ["qoder.mcp-initialize", "qoder.mcp-tools-list", "qoder.mcp-tools-call"]
    result["observations"]["skills_research"] = observe_skills_surface(ctx, top_help)
    return result
