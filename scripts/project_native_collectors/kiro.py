"""Kiro 2.34 help and opaque read-only config-list research, never runtime proof.

The measured first read-only authentication refusal remains a boundary only.
Nonempty list schema still requires review before a scope-observation case can
be registered. No ACP session is started.
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
    ("agent-help", ("agent", "--help"), "Usage:"),
)
NAMES = {scope: "uap-kiro-research-" + scope for scope in ("a", "b", "user")}
_ALL_HELP_STDERR = b"\nUSAGE:\n    kiro-cli [OPTIONS] [SUBCOMMAND]\n\n"
_LIST_AUTH_STDERR = b"error: You are not logged in, please log in with kiro-cli login\n"
_MCP_LIST_ROUTES = {
    project + "-" + scope + "-raw-list": (project, ("mcp", "list", scope))
    for project in ("a", "b", "user") for scope in ("workspace", "global")
}
_READ_ONLY_ROUTES = {
    "version": ("a", ("--version",)),
    "a-agent-raw-list": ("a", ("agent", "list")),
    **{label: ("a", args) for label, args, _ in HELP},
    **_MCP_LIST_ROUTES,
}
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


def _directory_identity(path):
    """Snapshot disposable fixture directory identity, owner, mode and ctime."""
    info = path.lstat()
    if not stat.S_ISDIR(info.st_mode):
        raise ValueError("Kiro authored fixture directory is no longer a directory")
    return (info.st_dev, info.st_ino, info.st_uid, info.st_gid,
            stat.S_IMODE(info.st_mode), info.st_ctime_ns)


def _require_clean_user_cwd(ctx):
    # Native MCP inventory also selects the "user" cwd, which is distinct
    # from the HOME-owned User fixture. Never adopt native output there.
    cwd = Path(ctx.projects["user"])
    if not cwd.is_absolute() or not cwd.is_dir() or cwd.resolve() != cwd:
        raise ValueError("Kiro selected user cwd changed during native research")
    marker = cwd / ".kiro"
    if marker.exists() or marker.is_symlink():
        raise ValueError("Kiro selected user cwd has unowned native config")


def _probe(ctx, owned, label, arguments, project="a"):
    route = _READ_ONLY_ROUTES.get(label)
    if route is None or (project, tuple(arguments)) != route:
        raise ValueError("Kiro read-only probe requires exact argv/project")
    _require_clean_user_cwd(ctx)
    directory = _runtime_bin(ctx)
    if ctx.env.get("PATH") != str(directory) + ":/usr/bin:/bin":
        raise ValueError("Kiro acquired companion PATH changed")
    key = getattr(ctx, "kiro_api_key", "")
    private_env = {"KIRO_API_KEY": key} if key and label.endswith("raw-list") else None
    try:
        record, stdout, stderr = ctx.probe(label, list(arguments), project=project, extra_env=private_env, timeout=30)
    finally:
        _runtime_bin(ctx)
    if ctx.env.get("PATH") != str(directory) + ":/usr/bin:/bin":
        raise ValueError("Kiro acquired companion PATH changed")
    _require_clean_user_cwd(ctx)
    for path, (body, device, inode, created_ctime_ns, parents) in owned.items():
        _parents(path)
        info = path.lstat()
        if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size != len(body)
                or info.st_uid != os.getuid() or (info.st_dev, info.st_ino) != (device, inode)
                or stat.S_IMODE(info.st_mode) != 0o600 or info.st_ctime_ns != created_ctime_ns
                or path.read_bytes() != body):
            raise ValueError("Kiro changed authored fixture identity/bytes/mode")
        # Report an unexpected member before checking parent ctime: creating
        # a new file also changes directory ctime, but is more precisely an
        # unowned-tree violation. Both cases must still fail closed.
        if ({entry.name for entry in path.parent.parent.iterdir()} != {"settings"}
                or {entry.name for entry in path.parent.iterdir()} != {path.name}):
            raise ValueError("Kiro native probe added an unowned fixture tree member")
        # A native child must not silently relax or replace the directories
        # holding the inert marker while leaving the file itself unchanged.
        for parent, original_identity in parents:
            if _directory_identity(parent) != original_identity:
                raise ValueError("Kiro changed authored fixture directory identity/mode")
        # ctime also catches in-place rewrites followed by restoring identical
        # bytes, size, permissions and mtime without changing the inode.
    if (label in ("a-workspace-raw-list", "a-agent-raw-list") and isinstance(record, dict)
            and type(record.get("exit")) is int and record["exit"] == 1
            and "error" in record and record["error"] is None
            and isinstance(stdout, bytes) and stdout == b""
            and isinstance(stderr, bytes) and stderr == _LIST_AUTH_STDERR):
        # Only these two exact, read-only authentication refusals are boundaries.
        # None cannot become an available-agent or MCP inventory claim.
        return None
    expected_stderr = _ALL_HELP_STDERR if label == "all-help" else b""
    if (not isinstance(record, dict) or type(record.get("exit")) is not int
            or record["exit"] != 0 or "error" not in record or record["error"] is not None
            or not isinstance(stdout, bytes) or not isinstance(stderr, bytes)
            or stderr != expected_stderr):
        raise ValueError("Kiro probe did not finish cleanly: " + label)
    return _text(stdout)


def collect(ctx):
    root, home = Path(ctx.root).resolve(), Path(ctx.home)
    projects = {name: Path(ctx.projects[name]) for name in ("a", "b", "user")}
    # Resolving source roots silently would adopt a symlinked foreign tree as a
    # research fixture. Refuse aliases before launching any native child.
    if (not home.is_absolute() or home.resolve() != home or not home.is_dir()
            or any(not path.is_absolute() or path.resolve() != path or not path.is_dir()
                   for path in projects.values())):
        # Do not silently manufacture an absent project or HOME when the
        # native client is selected; only existing disposable roots qualify.
        raise ValueError("Kiro requires existing canonical home and project directories without aliases")
    project_roots = tuple(projects.values())
    if (len(set(project_roots)) != 3 or not home.is_relative_to(root)
            or any(not path.is_relative_to(root) or home.is_relative_to(path) or path.is_relative_to(home)
                   for path in project_roots)
            or any(first.is_relative_to(second) or second.is_relative_to(first)
                   for i, first in enumerate(project_roots)
                   for second in project_roots[i + 1:])):
        raise ValueError("Kiro requires disjoint projects and original user home")
    # A preexisting native profile is foreign, even in disposable roots.
    # Do not launch vendor help/list commands that might rewrite it before
    # the later fixture-creation conflict check.
    _require_clean_user_cwd(ctx)
    for selected_root in (home, projects["a"], projects["b"]):
        marker = selected_root / ".kiro"
        if marker.exists() or marker.is_symlink():
            raise FileExistsError("Kiro research refuses preexisting native config")
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
    agent_text = _probe(ctx, owned, "a-agent-raw-list", ("agent", "list"))
    agent_observation = None if agent_text is None else {
        "probe": "a-agent-raw-list", "cwd_selection": "a",
        "sha256_normalized_text": hashlib.sha256(agent_text.encode()).hexdigest(),
        "interpretation": "opaque available-agent listing; no skill/MCP selection or runtime qualification",
    }
    captures, auth_boundary = [], None
    if agent_text is None:
        # A read-only authentication refusal cannot conceal native config
        # created by an earlier version/help/list probe. Preserve any files
        # for forensic review; never classify the refusal as a clean boundary.
        selected_roots = (("home", home), *((scope, projects[scope]) for scope in ("a", "b", "user")))
        unexpected = []
        for scope, selected_root in selected_roots:
            marker = selected_root / ".kiro"
            if marker.exists() or marker.is_symlink():
                unexpected.append(scope)
        if unexpected:
            # Only record synthetic disposable root labels, never private
            # native file names, bytes, environment variables or credentials.
            raise ValueError("Kiro authentication refusal left unexpected native config in: "
                             + ", ".join(unexpected))
        auth_boundary = {"probe": "a-agent-raw-list", "cwd_selection": "a",
                         "arguments": ["agent", "list"],
                         "kind": "authentication-required", "exit": 1,
                         "remaining_list_commands_unobserved": 6}
    else:
        # Reject a foreign marker in *any* disposable root before writing the
        # first fixture, so a late conflict cannot leave partial research state.
        destinations = {
            scope: (home if scope == "user" else projects[scope]) / ".kiro/settings/mcp.json"
            for scope in ("a", "b", "user")
        }
        for native in destinations.values():
            _parents(native)
            # This research profile owns the entire disposable .kiro subtree.
            # A foreign parent directory (even without mcp.json), or a file
            # blocking mkdir, must fail before *any* project fixture is written.
            marker = native.parent.parent
            if marker.exists() or marker.is_symlink():
                raise FileExistsError("Kiro research fixture root already exists: " + str(marker))
            if native.exists() or native.is_symlink():
                raise FileExistsError("Kiro research fixture already exists: " + str(native))
        for scope in ("a", "b", "user"):
            native = destinations[scope]
            _parents(native)
            native.parent.mkdir(parents=True, exist_ok=True)
            _parents(native)
            body = (json.dumps({"mcpServers": {NAMES[scope]: {
                "url": "http://127.0.0.1:9/" + NAMES[scope], "disabled": True}}}, indent=2) + "\n").encode()
            with native.open("xb") as stream:
                stream.write(body)
            native.chmod(0o600)
            identity = native.lstat()
            parents = tuple((parent, _directory_identity(parent))
                            for parent in (native.parent.parent, native.parent))
            owned[native] = (body, identity.st_dev, identity.st_ino, identity.st_ctime_ns, parents)
        for project in ("a", "b", "user"):
            for scope in ("workspace", "global"):
                label = project + "-" + scope + "-raw-list"
                text = _probe(ctx, owned, label, ("mcp", "list", scope), project)
                if text is None:
                    auth_boundary = {"probe": label, "cwd_selection": project,
                                     "arguments": ["mcp", "list", scope],
                                     "kind": "authentication-required", "exit": 1,
                                     "remaining_list_commands_unobserved": 5}
                    break
                captures.append({"probe": label, "cwd_selection": project, "scope_argument": scope,
                                 "sha256_normalized_text": hashlib.sha256(text.encode()).hexdigest(),
                                 "fixture_name_tokens": [name for name in NAMES.values() if name in text],
                                 "interpretation": "opaque native listing; no source schema or selection qualification"})
            if auth_boundary is not None:
                break
    facts = {"client_version_measured": version,
             "advertised": {"acp_agent_engine": "--agent-engine" in help_text["acp-help"],
                            "acp_auth_method": "--auth-method" in help_text["acp-help"],
                            "chat_trust_workspace": "--trust-workspace" in help_text["chat-help"]},
             "raw_agent_list_observation": agent_observation,
             "raw_source_list_observations": captures,
             "read_only_authentication_boundary": auth_boundary,
             "surface": "pinned help, read-only available-agent and MCP lists or exact authentication-required boundary; inert remote markers only when listing permitted",
             "scope_observation_qualified": False, "uap_lifecycle_qualified": False,
             "mcp_initialization_qualified": False, "mcp_tools_list_qualified": False,
             "mcp_tools_call_qualified": False, "authentication_qualified": False, "real_model_qualified": False,
             "remaining": "read-only authentication boundary does not qualify discovery; review genuine 2.34 selected scope and actual mode before registering scope or ACP/runtime cases"}
    return {"observations": facts, "cases": [
        {"id": "kiro.help", "status": "passed",
         "reason": "exact version/fixed help observed; read-only list text or its exact measured authentication boundary remains unqualified; no session, startup or tool command"},
    ], "unobserved": ["kiro.scope-observation", "kiro.project-lifecycle",
                       "kiro.native-restart-removal", "kiro.mcp-initialize",
                       "kiro.mcp-tools-list", "kiro.mcp-tools-call"]}
