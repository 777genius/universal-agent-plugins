"""Pinned Kimi help observations; project discovery and runtime stay unqualified.

The shared frontend owns acquisition, genuine hosted admission, child bounds,
private capture and artifact publication. This leaf never starts a web/ACP
service, grants trust, configures a provider or reads a native bearer token.
"""
import re

CLIENT = "kimi"
VERSION = "2.3.2"
AVAILABLE_PHASES = ("acquire", "help")
ACQUISITION = {
    "url": "https://github.com/MoonshotAI/kimi-code/releases/download/"
           "%40moonshot-ai/kimi-code%402.3.2/kimi-code-linux-x64.tar.gz",
    "integrity": "sha256:4715ccd1a591b64683929a14b26b3d07ac4c484e4da3881b22a2ccca56304042",
    "max_bytes": 96 << 20,
    "expected_bytes": 65_938_009,
    "format": "tar.gz",
    "entrypoint": "kimi",
    "runtime": "native",
}

# The exact release's produce-manifest.mjs packages the sole executable as
# `tar -C <work> -czf <archive> kimi`. The frontend must verify the archive pin
# and regular entry before executing it. Never reuse a 2.3.1 binary digest.
SOURCE_COMMIT = "b8dd3a134060c6e80ba4a1659335cd63b68483f2"
PROBES = (
    ("kimi-version", ("--version",), None),
    ("kimi-help", ("--help",), ""),
    ("kimi-doctor-help", ("doctor", "--help"), "doctor"),
    ("kimi-acp-help", ("acp", "--help"), "acp"),
    ("kimi-web-help", ("web", "--help"), "web"),
)


def _accepted_streams(record, stdout, stderr):
    if (not isinstance(record, dict) or type(record.get("exit")) is not int
            or record["exit"] != 0 or "error" not in record
            or record["error"] not in (None, "")):
        raise ValueError("Kimi help probe failed; bounded capture retained")
    if not isinstance(stdout, bytes) or not isinstance(stderr, bytes) or stderr:
        raise ValueError("Kimi help probe emitted unexpected stream data")
    try:
        return stdout.decode("utf-8", errors="strict")
    except UnicodeError as error:
        raise ValueError("Kimi help probe is not UTF-8") from error


def _validate_help(text, command):
    name = "kimi" + (" " + command if command else "")
    if not re.search(r"^Usage: " + re.escape(name) + r"(?:[ \t]+\[|[ \t]*$)", text, re.MULTILINE):
        raise ValueError("Kimi help usage does not identify the requested command")
    if "--help" not in text:
        raise ValueError("Kimi command help is missing its help option")
    if not command and any(word not in text for word in ("--version", "doctor", "acp", "web")):
        raise ValueError("Kimi main help is missing the selected command surface")


def collect(ctx):
    """Observe only fixed read-only version/help commands in project A."""
    observations = {}
    for label, arguments, command in PROBES:
        record, stdout, stderr = ctx.probe(label, list(arguments), project="a", timeout=30)
        text = _accepted_streams(record, stdout, stderr)
        if command is None:
            if stdout not in (b"2.3.2", b"2.3.2\n", b"2.3.2\r\n"):
                raise ValueError("Kimi executable version differs from the frozen 2.3.2 pin")
        else:
            _validate_help(text, command)
        observations[label] = {"surface": "version" if command is None else "help",
                               "arguments": list(arguments)}
    return {
        "observations": observations,
        "cases": [{"id": "kimi.help", "status": "passed",
                   "reason": "Exact pinned Kimi 2.3.2 version and fixed main/doctor/ACP/web help observed; no service or project discovery was invoked"}],
        "unobserved": ["kimi.scope-observation", "kimi.project-lifecycle",
                       "kimi.native-restart-removal", "kimi.mcp-initialize",
                       "kimi.mcp-tools-list", "kimi.mcp-tools-call"],
    }
