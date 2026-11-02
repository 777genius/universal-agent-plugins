"""Pinned read-only Cursor help and opaque empty MCP captures; no runtime proof."""

CLIENT = "cursor"
VERSION = "2026.10.22-27dcf5f"
AVAILABLE_PHASES = ("acquire", "help")
ACQUISITION = {
    "url": "https://downloads.cursor.com/lab/2026.10.22-27dcf5f/linux/x64/agent-cli-package.tar.gz",
    "integrity": "sha256:85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294",
    "expected_bytes": 184859176,
    "max_bytes": 256 * 1024 * 1024,
    "max_expanded_bytes": 640 * 1024 * 1024,
    "format": "tar.gz",
    "entrypoint": "dist-package/cursor-agent",
    "binary_sha256": "2ccc9a8e167797641448b5e5c936f006ba137a2555f117f38c5eb76a5238a233",
    # This exact shell launcher selects its sibling bundled Node and index.js.
    "runtime": "native",
    "transport": "cursor-publisher-curl",
}

PROBES = (
    ("cursor-version", ("--version",), "a", "version"),
    ("cursor-help", ("--help",), "a", "help"),
    ("cursor-mcp-help", ("mcp", "--help"), "a", "help"),
    ("cursor-plugin-help", ("plugin", "--help"), "a", "help"),
    ("cursor-empty-a-mcp", ("mcp", "list"), "a", "opaque-component-capture"),
    ("cursor-empty-b-mcp", ("mcp", "list"), "b", "opaque-component-capture"),
    ("cursor-empty-outside-mcp", ("mcp", "list"), "user", "opaque-component-capture"),
)


def collect(ctx):
    """Use the frozen bundled launcher without seeding settings or approving MCP."""
    if getattr(ctx, "phase", None) != "help":
        raise ValueError("Cursor collector accepts only its reviewed help phase")
    observations = {}
    for label, arguments, project, surface in PROBES:
        record, stdout, stderr = ctx.probe(label, list(arguments), project=project, timeout=30)
        if (not isinstance(record, dict) or type(record.get("exit")) is not int
                or record["exit"] != 0 or "error" not in record or record["error"] is not None
                or not isinstance(stdout, bytes) or not isinstance(stderr, bytes)):
            raise ValueError("Cursor read-only probe failed; no supported boundary is inferred")
        try:
            text = stdout.decode("utf-8", errors="strict")
            stderr.decode("utf-8", errors="strict")
        except UnicodeError as error:
            raise ValueError("Cursor read-only capture is not UTF-8") from error
        if surface == "version" and (stdout != (VERSION + "\n").encode() or stderr):
            raise ValueError("Cursor executable version differs from frozen publisher version")
        if surface in ("version", "help") and not text.strip():
            raise ValueError("Cursor version/help capture is empty")
        observations[label] = {"arguments": list(arguments), "project": project, "surface": surface}
    return {
        "observations": observations,
        "cases": [{"id": "cursor.help", "status": "passed", "reason":
                   "Exact pinned version and read-only help observed; three empty-context MCP outputs remain opaque, without scope or runtime qualification"}],
        "unobserved": ["cursor.scope-observation", "cursor.project-lifecycle", "cursor.native-restart-removal",
                       "cursor.mcp-initialize", "cursor.mcp-tools-list", "cursor.mcp-tools-call",
                       "cursor.vendor-authentication", "cursor.real-model"],
    }
