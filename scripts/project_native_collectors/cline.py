"""Pinned Cline version/help only; no task, MCP connection or approval grant.

The shared hosted frontend owns acquisition, host admission, explicit child
environment, bounded capture and process/runtime identity checks. Every probe
suppresses the native updater and explicitly disables tool auto-approval.
"""

import re

CLIENT = "cline"
VERSION = "3.0.73"
AVAILABLE_PHASES = ("acquire", "help")
ACQUISITION = {
    "url": "https://registry.npmjs.org/@cline/cli-linux-x64/-/cli-linux-x64-3.0.73.tgz",
    "integrity": "sha512-K+T94s0kPHLoMSANOJJ7KHb/etzYXZGQUbarKDKUPoZsvy4+oD1MAqYnZlfbKFM8tpnRQWouFngD9TB3mFqqeA==",
    "max_bytes": 64 << 20,
    "expected_bytes": 50_817_241,
    "format": "tar.gz",
    "entrypoint": "package/bin/cline",
    "runtime": "native",
    "binary_sha256": "1ff8841d17d4d83763134c193d414e4f7fcf2ef56af759e5ebbe96727fc2c8f7",
}

PROBES = (
    ("cline-version", ("--version",), None),
    ("cline-help", ("--help",), ""),
    ("cline-config-help", ("config", "--help"), "config"),
    ("cline-mcp-help", ("mcp", "--help"), "mcp"),
)


def _text(record, stdout, stderr):
    if (not isinstance(record, dict) or type(record.get("exit")) is not int
            or record["exit"] != 0 or "error" not in record
            or record["error"] not in (None, "")):
        raise ValueError("Cline help probe failed; no supported boundary is inferred")
    if not isinstance(stdout, bytes) or not isinstance(stderr, bytes) or stderr:
        raise ValueError("Cline help probe has unexpected stream data")
    try:
        return stdout.decode("utf-8", errors="strict")
    except UnicodeError as error:
        raise ValueError("Cline help probe is not UTF-8") from error


def _help(text, command):
    name = "cline" + (" " + command if command else "")
    if not re.search(r"^Usage: " + re.escape(name) + r"[ \t]+\[", text, re.MULTILINE):
        raise ValueError("Cline help usage does not identify the requested command")
    if "--help" not in text:
        raise ValueError("Cline help option is missing")
    if not command and any(word not in text for word in ("--version", "--auto-approve", "config", "mcp")):
        raise ValueError("Cline main help is missing the selected safe command surface")


def collect(ctx):
    """Run the closed version/help table without reading native user settings."""
    observations = {}
    for label, arguments, command in PROBES:
        argv = ["--auto-approve", "false", *arguments]
        record, stdout, stderr = ctx.probe(
            label, argv, project="a", extra_env={"CLINE_NO_AUTO_UPDATE": "1"}, timeout=30,
        )
        text = _text(record, stdout, stderr)
        if command is None:
            if stdout not in (b"3.0.73", b"3.0.73\n", b"3.0.73\r\n"):
                raise ValueError("Cline executable version differs from frozen 3.0.73")
        else:
            _help(text, command)
        observations[label] = {"surface": "version" if command is None else "help", "arguments": argv}
    return {
        "observations": observations,
        "cases": [{"id": "cline.help", "status": "passed",
                   "reason": "Exact pinned Cline version and main/config/MCP help observed with updater and auto-approval disabled; no task or MCP operation was requested"}],
        "unobserved": ["cline.scope-observation", "cline.project-lifecycle",
                       "cline.native-restart-removal", "cline.mcp-initialize",
                       "cline.mcp-tools-list", "cline.mcp-tools-call"],
    }
