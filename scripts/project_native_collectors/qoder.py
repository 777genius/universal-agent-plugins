"""Pinned Qoder version/fixed help research, without discovery or runtime proof.

The genuine acquisition selected qodercli; shared code owns immutable bytes,
host admission, bounded execution and captures. Never import or list a plugin.
"""
import re

CLIENT = "qoder"
VERSION = "1.1.78"
AVAILABLE_PHASES = ("acquire", "help")
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
    ("top-help", ("--help",), "Usage: qodercli [options] [command]"),
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


def collect(ctx):
    """Only the reviewed five version/help commands; no source selection claim."""
    version = parse_version(_probe(ctx, "version", ("--version",)))
    help_rows = []
    for label, arguments, usage in HELP:
        observed = parse_help(_probe(ctx, label, arguments), usage)
        help_rows.append({"label": label, "arguments": list(arguments), "usage": observed})
    return {"observations": {"client_version_measured": version, "help": help_rows,
                             "surface": "fixed version and command help only",
                             "scope_discovery_qualified": False, "uap_lifecycle_qualified": False,
                             "mcp_initialization_qualified": False, "mcp_tools_list_qualified": False,
                             "mcp_tools_call_qualified": False, "authentication_qualified": False,
                             "real_model_qualified": False},
            "cases": [{"id": "qoder.help", "status": "passed",
                       "reason": "Pinned fixed version/help table observed; no plugin operation, scope or runtime qualification"}]}
