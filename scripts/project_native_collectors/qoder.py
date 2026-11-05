"""Pinned Qoder version/fixed help research, without discovery or runtime proof.

The genuine acquisition selected qodercli; shared code owns immutable bytes,
host admission, bounded execution and captures. Never import or list a plugin.
"""
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


def observe_project_mcp_sources(ctx):
    """Record genuine Qoder MCP inventory, without a model or MCP tool call."""
    a, b = (ctx.projects[name] for name in ("a", "b"))
    for name, project in (("a", a), ("b", b)):
        if not project.is_dir() or project.is_symlink():
            raise ValueError("Qoder project root is not an ordinary directory")
        record, _, _ = ctx.command("git-init-" + name,
                                   ["/usr/bin/git", "init", "--quiet"],
                                   project, ctx.env, 10)
        if record.get("exit") != 0 or record.get("error") is not None:
            raise ValueError("Qoder project Git root initialization failed")
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
    """Only the reviewed five version/help commands; no source selection claim."""
    version = parse_version(_probe(ctx, "version", ("--version",)))
    help_rows = []
    for label, arguments, usage in HELP:
        observed = parse_help(_probe(ctx, label, arguments), usage)
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
    return result
