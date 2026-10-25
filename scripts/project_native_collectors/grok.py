"""Measured Grok 1.0.70 acquisition and fixed native help research only.

The shared runner owns genuine hosted admission, immutable acquisition,
resource limits and capture. No inspection, trust, doctor operation, provider
or MCP connection runs here; observed help makes no project-support claim.
"""
import re

CLIENT = "grok"
VERSION = "1.0.70"
AVAILABLE_PHASES = ("acquire", "help")
ACQUISITION = {
    "url": "https://x.ai/cli/grok-1.0.70-linux-x86_64.gz",
    # Genuine run 40235269281/attempt 2, artifact 12615677952; reviewed freeze
    # b157c54c086a3eb17f285c6013749de59e26ed2f. No 1.0.67 identity is reused.
    "integrity": "sha256:11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc",
    "expected_bytes": 76_760_282,
    "binary_sha256": "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9",
    "max_bytes": 96 << 20,
    "max_expanded_bytes": 256 << 20,
    "format": "gzip",
    # A gzip stream has no archive member: this is the shared runner's fixed
    # synthetic output name, never an authority derived from its gzip header.
    "entrypoint": "grok",
    "runtime": "native",
}

# This periodically synced September snapshot guides candidate commands;
# it is not shipping-source provenance for the measured executable.
COMMAND_SOURCE = "https://github.com/xai-org/grok-build/tree/2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8"
PROBES = (
    ("grok-version", ("--version",), None),
    ("grok-help", ("--help",), ""),
    ("grok-inspect-help", ("inspect", "--help"), "inspect"),
    ("grok-mcp-help", ("mcp", "--help"), "mcp"),
    ("grok-mcp-list-help", ("mcp", "list", "--help"), "mcp list"),
    ("grok-mcp-doctor-help", ("mcp", "doctor", "--help"), "mcp doctor"),
    ("grok-plugin-help", ("plugin", "--help"), "plugin"),
    ("grok-plugin-list-help", ("plugin", "list", "--help"), "plugin list"),
    ("grok-doctor-help", ("doctor", "--help"), "doctor"),
)


def _streams(record, stdout, stderr):
    if (not isinstance(record, dict) or type(record.get("exit")) is not int
            or record["exit"] != 0 or "error" not in record
            or record["error"] not in (None, "")):
        raise ValueError("Grok fixed help probe failed its bounded capture")
    if not isinstance(stdout, bytes) or not isinstance(stderr, bytes) or stderr:
        raise ValueError("Grok fixed help probe has unexpected stream data")
    try:
        return stdout.decode("utf-8", "strict")
    except UnicodeError:
        raise ValueError("Grok fixed help probe is not UTF-8") from None


def _version(stdout):
    match = re.fullmatch(rb"grok 1\.0\.70 \(([0-9a-f]{7,40})\)( \[stable\])?\n", stdout)
    if not match:
        raise ValueError("Grok version differs from the frozen 1.0.70 grammar")
    return {"version": VERSION, "compiled_commit_observed": match[1].decode("ascii"),
            "channel_observed": "stable" if match[2] else None,
            "shipping_source_equivalence_claimed": False}


def _help(text, command):
    name = "grok" + (" " + command if command else "")
    # Complete Clap metavariables only, including a variadic suffix. A parent
    # <COMMAND> is legitimate; different commands or unclosed delimiters are not.
    metavar = r"(?:\[[A-Za-z0-9_.:|= /-]+\]|<[A-Za-z0-9_.:|= /-]+>)(?:\.\.\.)?"
    usage = re.findall(r"^Usage: ([^\r\n]+)$", text, re.MULTILINE)
    if (len(usage) != 1 or not re.fullmatch(re.escape(name)
            + r"(?:[ \t]+" + metavar + r")*[ \t]*", usage[0])):
        raise ValueError("Grok help Usage does not identify the requested command")
    if not re.search(r"^[ \t]+(?:-h,[ \t]+)?--help(?:[ \t]+[^\r\n]*)?$", text, re.MULTILINE):
        raise ValueError("Grok fixed command help is missing its help option")
    if not command:
        if not re.search(r"^Commands:$", text, re.MULTILINE) or any(
                not re.search(r"^[ \t]{2,}" + word + r"(?:[ \t]+[^\r\n]*)?$", text, re.MULTILINE)
                for word in ("inspect", "mcp", "plugin")):
            raise ValueError("Grok main help is missing the selected command surface")


def collect(ctx):
    """Run only fixed help after the shared hosted and pin admission."""
    if ctx.phase != "help":
        raise ValueError("Grok collector requires the explicit help phase")
    observations = {}
    for label, arguments, command in PROBES:
        record, stdout, stderr = ctx.probe(label, list(arguments), project="a", timeout=30)
        text = _streams(record, stdout, stderr)
        row = {"surface": "version" if command is None else "help", "arguments": list(arguments)}
        if command is None:
            row.update(_version(stdout))
        else:
            _help(text, command)
        observations[label] = row
    return {"observations": observations,
            "cases": [{"id": "grok.help", "status": "passed",
                       "reason": "Measured pinned Grok 1.0.70 fixed version/help table observed; no scope discovery, trust, doctor operation or MCP runtime was invoked"}],
            "unobserved": ["grok.scope-observation", "grok.project-lifecycle",
                           "grok.native-restart-removal", "grok.mcp-initialize",
                           "grok.mcp-tools-list", "grok.mcp-tools-call"]}
