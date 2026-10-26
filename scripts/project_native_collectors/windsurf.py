"""Current Devin Local harness help for UAP's canonical Windsurf target.

Legacy Cascade, a Windsurf editor launcher and a configured native entry are
separate surfaces. This initial collector makes no project discovery, trust,
MCP connection, model or account claim. The shared runner owns all execution.
"""
import re

CLIENT = "windsurf"
VERSION = "3000.11.205"
AVAILABLE_PHASES = ("acquire", "help")
ACQUISITION = {
    "url": "https://static.devin.ai/cli/3000.11.205/"
           "devin-3000.11.205-x86_64-unknown-linux.tar.gz",
    "integrity": "sha256:2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4",
    "expected_bytes": 73_679_375,
    "max_bytes": 96 << 20,
    "max_expanded_bytes": 256 << 20,
    "format": "tar.gz",
    "entrypoint": "bin/devin",
    "runtime": "native",
    "binary_sha256": "642c82e32cb3dec90cf5ab01975520b425307872c7a650c9fb5b0653f37681f4",
}
_VERSION = "devin 3000.11.205 (6dea39440ae3)"
# Measured run 40235292067, artifact 12616906453. Only this command uses
# literal options in its Usage forms; do not broaden every command boundary.
_ADD_USAGE = (
    "Usage: devin mcp add --transport http <NAME> <URL>",
    "       devin mcp add <NAME> -- <COMMAND> [ARGS...]",
    "       devin mcp add <NAME> --url <URL>",
    "       devin mcp add <NAME> --transport <stdio|sse|http> [OPTIONS]",
)
PROBES = (
    ("devin-version", ("--version",), None, ()),
    ("devin-help", ("--help",), "devin", ("mcp", "skills")),
    ("devin-mcp-help", ("mcp", "--help"), "devin mcp", ("list", "get")),
    ("devin-skills-help", ("skills", "--help"), "devin skills", ("list", "paths")),
    ("devin-mcp-list-help", ("mcp", "list", "--help"), "devin mcp list", ()),
    ("devin-mcp-get-help", ("mcp", "get", "--help"), "devin mcp get", ()),
    ("devin-mcp-add-help", ("mcp", "add", "--help"), "devin mcp add", ("--scope",)),
    ("devin-mcp-remove-help", ("mcp", "remove", "--help"), "devin mcp remove", ("--scope",)),
    ("devin-skills-list-help", ("skills", "list", "--help"), "devin skills list", ("--json",)),
    ("devin-skills-paths-help", ("skills", "paths", "--help"), "devin skills paths", ()),
)


def _text(record, stdout, stderr):
    if (not isinstance(record, dict) or type(record.get("exit")) is not int
            or record["exit"] != 0 or "error" not in record
            or record["error"] is not None):
        raise ValueError("Devin help probe failed; bounded capture retained")
    if not isinstance(stdout, bytes) or not isinstance(stderr, bytes) or stderr:
        raise ValueError("Devin help probe emitted unexpected stream data")
    try:
        text = stdout.decode("utf-8", errors="strict")
    except UnicodeError as error:
        raise ValueError("Devin help probe is not UTF-8") from error
    if "\x1b" in text or "\x00" in text:
        raise ValueError("Devin help contains unexpected control sequences")
    return text


def collect(ctx):
    """Use only the ten previously observed version/help argument forms."""
    observations = {}
    for label, arguments, usage, required in PROBES:
        record, stdout, stderr = ctx.probe(label, list(arguments), project="a", timeout=30)
        text = _text(record, stdout, stderr)
        if usage is None:
            if text not in (_VERSION, _VERSION + "\n", _VERSION + "\r\n"):
                raise ValueError("Devin executable differs from the frozen harness/version/build")
        else:
            if usage == "devin mcp add":
                lines = text.splitlines()
                starts = [index for index, line in enumerate(lines) if line.startswith("Usage:")]
                if len(starts) != 1:
                    raise ValueError("Devin mcp-add help has an unexpected Usage block")
                start, end = starts[0], starts[0] + 1
                while end < len(lines) and lines[end].startswith("       "):
                    end += 1
                if tuple(lines[start:end]) != _ADD_USAGE:
                    raise ValueError("Devin mcp-add help Usage forms differ from observation")
            else:
                pattern = r"^Usage: " + re.escape(usage) + r"(?:[ \t]+[\[<]|[ \t]*$)"
                if not re.search(pattern, text, re.MULTILINE):
                    raise ValueError("Devin help does not identify the requested command")
            if "--help" not in text:
                raise ValueError("Devin help does not identify the requested command")
            if any(token not in text for token in required):
                raise ValueError("Devin help is missing a previously observed option")
        observations[label] = {"arguments": list(arguments),
                               "surface": "version" if usage is None else "help"}
    return {
        "observations": observations,
        "cases": [{"id": "windsurf.help", "status": "passed",
                   "reason": "Exact pinned Devin 3000.11.205 build and fixed MCP/skills help observed for the Windsurf target; no discovery or service invocation"}],
        "unobserved": ["windsurf.scope-observation", "windsurf.project-lifecycle",
                       "windsurf.native-restart-removal", "windsurf.mcp-initialize",
                       "windsurf.mcp-tools-list", "windsurf.mcp-tools-call"],
    }
