"""Kiro 2.34 help and opaque read-only config-list research, never runtime proof.

The new version's nonempty list schema must be reviewed from genuine artifacts
before a scope-observation case can be registered. No ACP session is started.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat

CLIENT = "kiro"
VERSION = "2.34.0"
AVAILABLE_PHASES = ("acquire", "help")
ACQUISITION = {
    "url": "https://prod.download.cli.kiro.dev/stable/2.34.0/kirocli-x86_64-linux.tar.xz",
    "integrity": "sha256:11dce9286cf7401079cf4eac02a3d5982daac16a346d880109ec7b4a0b941040",
    "expected_bytes": 132018592,
    "max_bytes": 144 << 20,
    "max_expanded_bytes": 512 << 20,
    "format": "tar.xz",
    "entrypoint": "kirocli/bin/kiro-cli",
    "runtime": "native",
}
HELP = (
    ("top-help", ("--help",), "Usage:"),
    ("all-help", ("--help-all",), "Commands:"),
    ("mcp-help", ("mcp", "--help"), "list"),
    ("mcp-list-help", ("mcp", "list", "--help"), "workspace"),
    ("mcp-status-help", ("mcp", "status", "--help"), "--name"),
    ("acp-help", ("acp", "--help"), "--agent-engine"),
    ("chat-help", ("chat", "--help"), "Usage:"),
)
NAMES = {scope: "uap-kiro-research-" + scope for scope in ("a", "b", "user")}
_ALL_HELP_STDERR = b"\nUSAGE:\n    kiro-cli [OPTIONS] [SUBCOMMAND]\n\n"
_RUNTIME_MEMBERS = {
    "kiro-cli": "94cae7e349f76869a19eae335a5b392fba8515743b1c2822b9ea33128c067cc6",
    "kiro-cli-chat": "596c48b3cba6db1f5c043434c20a5fd1d23ea7d35eb0b70e3bcfe4144d93a79c",
    "kiro-cli-term": "b43d2c9b15accdac83724225660ae9d02214598f88897d87abacac1f0208b7d3",
}
_SGR = re.compile(r"\x1b\[[0-9;]*m")
_ERROR = re.compile(r"authentication required|not logged in|please log ?in|login required|\berror:", re.I)


def _text(body):
    value = _SGR.sub("", body.decode("utf-8", errors="strict")).strip()
    if not value or "\x1b" in value or "\x00" in value or _ERROR.search(value):
        raise ValueError("empty, unsafe or error-bearing Kiro output")
    return value


def _parents(path):
    if any(parent.is_symlink() for parent in path.parents):
        raise ValueError("Kiro fixture parent alias refused")


def _runtime_bin(ctx):
    root = Path(ctx.root)
    if not root.is_absolute() or root.resolve() != root:
        raise ValueError("Kiro runtime root alias refused")
    directory = root / "client/kirocli/bin"
    if ctx.argv != [str(directory / "kiro-cli")]:
        raise ValueError("Kiro requires the exact acquired launcher path")
    for name, digest in _RUNTIME_MEMBERS.items():
        path = directory / name
        _parents(path)
        try:
            info = path.lstat()
        except OSError as error:
            raise ValueError("Kiro pinned runtime companion absent") from error
        if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1
                or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700
                or ctx.identities.get(path) != digest):
            raise ValueError("Kiro pinned runtime companion identity/mode refused")
    # Context.probe independently rehashes every acquired regular member before
    # and after each child. These three exact hashes bind PATH to that closure.
    return directory


def _probe(ctx, owned, label, arguments, project="a"):
    if label == "all-help" and tuple(arguments) != ("--help-all",):
        raise ValueError("Kiro all-help stderr exception requires exact argv")
    directory = _runtime_bin(ctx)
    if ctx.env.get("PATH") != str(directory) + ":/usr/bin:/bin":
        raise ValueError("Kiro acquired companion PATH changed")
    try:
        record, stdout, stderr = ctx.probe(label, list(arguments), project=project, timeout=30)
    finally:
        _runtime_bin(ctx)
    expected_stderr = _ALL_HELP_STDERR if label == "all-help" else b""
    if (not isinstance(record, dict) or type(record.get("exit")) is not int
            or record["exit"] != 0 or "error" not in record or record["error"] is not None
            or not isinstance(stdout, bytes) or not isinstance(stderr, bytes)
            or stderr != expected_stderr):
        raise ValueError("Kiro probe did not finish cleanly: " + label)
    for path, body in owned.items():
        _parents(path)
        info = path.lstat()
        if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size != len(body)
                or stat.S_IMODE(info.st_mode) != 0o600 or path.read_bytes() != body):
            raise ValueError("Kiro changed authored fixture bytes/mode")
    return _text(stdout)


def collect(ctx):
    root, home = Path(ctx.root).resolve(), Path(ctx.home).resolve()
    projects = {name: Path(ctx.projects[name]).resolve() for name in ("a", "b", "user")}
    if (len(set(projects.values())) != 3 or not home.is_relative_to(root)
            or any(not path.is_relative_to(root) or home.is_relative_to(path) or path.is_relative_to(home)
                   for path in projects.values())):
        raise ValueError("Kiro requires distinct projects and original user home")
    owned = {}
    directory = _runtime_bin(ctx)
    if ctx.env.get("PATH") != "/usr/bin:/bin":
        raise ValueError("Kiro research requires the original closed system PATH")
    ctx.env["PATH"] = str(directory) + ":/usr/bin:/bin"
    version = _probe(ctx, owned, "version", ("--version",))
    if version != "kiro-cli " + VERSION:
        raise ValueError("Kiro version differs from immutable pin")
    help_text = {}
    for label, arguments, token in HELP:
        value = _probe(ctx, owned, label, arguments)
        if (token not in value or (label == "all-help" and (
                not re.search(r"(?m)^[ \t]+--help-all[ \t]*$", value)
                or not re.search(r"(?m)^[ \t]+(?:-h,[ \t]+)?--help[ \t]*$", value)
                or "Usage:" in value or "USAGE:" in value))):
            raise ValueError("unrecognized pinned Kiro help: " + label)
        help_text[label] = value
    for scope in ("a", "b", "user"):
        native = (home if scope == "user" else projects[scope]) / ".kiro/settings/mcp.json"
        _parents(native)
        native.parent.mkdir(parents=True, exist_ok=True)
        _parents(native)
        body = (json.dumps({"mcpServers": {NAMES[scope]: {
            "url": "http://127.0.0.1:9/" + NAMES[scope], "disabled": True}}}, indent=2) + "\n").encode()
        with native.open("xb") as stream:
            stream.write(body)
        native.chmod(0o600)
        owned[native] = body
    captures = []
    for project in ("a", "b", "user"):
        for scope in ("workspace", "global"):
            label = project + "-" + scope + "-raw-list"
            text = _probe(ctx, owned, label, ("mcp", "list", scope), project)
            captures.append({"probe": label, "cwd_selection": project, "scope_argument": scope,
                             "sha256_normalized_text": hashlib.sha256(text.encode()).hexdigest(),
                             "fixture_name_tokens": [name for name in NAMES.values() if name in text],
                             "interpretation": "opaque native listing; no source schema or selection qualification"})
    facts = {"client_version_measured": version,
             "advertised": {"acp_agent_engine": "--agent-engine" in help_text["acp-help"],
                            "acp_auth_method": "--auth-method" in help_text["acp-help"],
                            "chat_trust_workspace": "--trust-workspace" in help_text["chat-help"]},
             "raw_source_list_observations": captures,
             "surface": "pinned help and opaque read-only mcp list workspace/global; disabled inert remote markers",
             "scope_observation_qualified": False, "uap_lifecycle_qualified": False,
             "mcp_initialization_qualified": False, "mcp_tools_list_qualified": False,
             "mcp_tools_call_qualified": False, "authentication_qualified": False, "real_model_qualified": False,
             "remaining": "review genuine 2.34 nonempty text schema and actual mode before registering scope or ACP/runtime cases"}
    return {"observations": facts, "cases": [
        {"id": "kiro.help", "status": "passed",
         "reason": "exact version/fixed help observed; opaque positional source-list captures remain unqualified; no session, startup or tool command"},
    ]}
