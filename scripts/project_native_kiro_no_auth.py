"""Finite pinned Kiro observations without credentials, sessions or model calls.

Exact measured authentication refusals never qualify validation or discovery.
UAP installation lifecycle tests run independently in the dedicated workflow.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat

import project_native_research as research
from project_native_collectors import kiro

HELP = ("agent", "validate", "--help")
STATUS_HELP = ("mcp", "status", "--help")
VALIDATE_HELP = (b"Validate a config with the given path\n\n"
                 b"Usage: kiro-cli agent validate [OPTIONS] --path <PATH>\n\nOptions:\n"
                 b"  -p, --path <PATH>  \n  -v, --verbose...   Increase logging verbosity\n"
                 b"  -h, --help         Print help\n")
AUTH_REFUSAL = b"error: You are not logged in, please log in with kiro-cli login\n"
QUALIFICATIONS = ("authentication", "real_model", "native_discovery", "uap_native_lifecycle",
                  "mcp_initialize", "mcp_tools_list", "mcp_tools_call")
VALID = {"name": "uap-no-auth-owned", "description": "Owned inert validation fixture",
         "prompt": "Inert configuration, never launch a session.", "tools": [],
         "allowedTools": [], "resources": [], "hooks": {}, "mcpServers": {}}


def file_identity(path):
    before = path.lstat()
    if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
            or before.st_uid != os.getuid() or before.st_size > 65536
            or path.resolve() != path):
        raise ValueError("owned fixture identity refused")
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        if research.artifacts.identity(before) != research.artifacts.identity(os.fstat(fd)):
            raise ValueError("owned fixture changed on open")
        body = os.read(fd, 65537)
        if research.artifacts.identity(before) != research.artifacts.identity(os.fstat(fd)):
            raise ValueError("owned fixture changed on read")
    finally:
        os.close(fd)
    if len(body) != before.st_size or research.artifacts.identity(before) != research.artifacts.identity(path.lstat()):
        raise ValueError("owned fixture changed")
    return research.artifacts.identity(before), hashlib.sha256(body).hexdigest()


def tree_identity(root):
    """Bounded owned fixture inventory; no symlink or special-file traversal."""
    result = {}
    def visit(path):
        if len(result) >= 128:
            raise ValueError("owned fixture inventory limit")
        info = path.lstat()
        key = str(path.relative_to(root))
        if stat.S_ISDIR(info.st_mode):
            result[key] = ("directory", kiro._research_root_identity(path))
            with os.scandir(path) as entries:
                for entry in entries:
                    visit(path / entry.name)
        else:
            result[key] = ("file", file_identity(path))
    visit(root)
    return result


def prepare(ctx):
    roots = [ctx.home, *(ctx.projects[key] for key in ("a", "b", "user"))]
    canonical = [Path(path) for path in roots]
    if (len(set(canonical)) != 4 or any(path.resolve() != path for path in canonical)
            or any(a.is_relative_to(b) for a in canonical for b in canonical if a != b)):
        raise ValueError("disjoint canonical owned roots required")
    owned = {}
    for root in canonical:
        marker = root / ".kiro"
        if marker.exists() or marker.is_symlink():
            raise ValueError("preexisting native fixture refused")
        settings = marker / "settings"
        settings.mkdir(mode=0o700, parents=True)
        (settings / "mcp.json").write_text(json.dumps({"mcpServers": {
            "uap-noauth-" + root.name: {"url": "http://127.0.0.1:9/inert", "disabled": True}}}) + "\n")
        (marker / "sentinel.txt").write_text("foreign owned-test sentinel; preserve bytes\n")
        owned[root] = tree_identity(root)
    fixtures = Path(ctx.root) / "validation"
    fixtures.mkdir(mode=0o700)
    for name, value in (("valid.json", VALID), ("invalid.json", {"name": ["wrong-schema"]})):
        path = fixtures / name
        path.write_text(json.dumps(value) + "\n")
        path.chmod(0o600)
    owned[fixtures] = tree_identity(fixtures)
    return owned, fixtures


def routes(fixtures, ctx):
    table = {"version": ("a", ("--version",)),
             "validate-help": ("a", HELP)}
    for name in ("valid", "invalid", "missing"):
        table["validate-" + name] = ("a", ("agent", "validate", "--path", str(fixtures / (name + ".json"))))
    for project in ("a", "b", "user"):
        table[project + "-agent-list"] = (project, ("agent", "list"))
        for scope in ("workspace", "global"):
            table[project + "-mcp-" + scope] = (project, ("mcp", "list", scope))
    table["mcp-status-help"] = ("a", STATUS_HELP)
    for project in ("a", "b", "user"):
        selected = ctx.home if project == "user" else ctx.projects[project]
        table[project + "-mcp-status"] = (project, ("mcp", "status", "--name", "uap-noauth-" + selected.name))
    return table


def closed_environment(ctx):
    expected = {"HOME": str(ctx.home), "USERPROFILE": str(ctx.home),
                "PATH": str(Path(ctx.root) / "client/kirocli/bin") + ":/usr/bin:/bin",
                "TMPDIR": str(Path(ctx.root) / "tmp"), "LANG": "C.UTF-8",
                "NO_COLOR": "1", "TERM": "dumb", "GIT_CONFIG_NOSYSTEM": "1",
                "GIT_TERMINAL_PROMPT": "0"}
    for key, name in (("XDG_CONFIG_HOME", "config"), ("XDG_CACHE_HOME", "cache"),
                      ("XDG_STATE_HOME", "state"), ("XDG_DATA_HOME", "data"),
                      ("NPM_CONFIG_CACHE", "npm-cache"), ("NPM_CONFIG_USERCONFIG", "npm-user-config"),
                      ("NPM_CONFIG_GLOBALCONFIG", "npm-global-config"), ("GIT_CONFIG_GLOBAL", "git-config")):
        expected[key] = str(Path(ctx.home) / name)
    if ctx.secrets or ctx.env != expected:
        raise ValueError("no-auth closed environment refused")


def probe(ctx, table, owned, root_ids, label, arguments, project):
    if table.get(label) != (project, tuple(arguments)):
        raise ValueError("no-auth exact argv/cwd route refused")
    closed_environment(ctx)
    kiro._runtime_bin(ctx)
    kiro._verify_root_identities(root_ids)
    for root, expected in owned.items():
        if tree_identity(root) != expected:
            raise ValueError("owned fixture changed before probe")
    # The native client may initialize only this fresh private profile.
    profile = kiro._private_probe_home(ctx, root_ids, "no-auth-" + label)
    env = dict(ctx.env)
    env["KIRO_HOME"] = str(profile)
    # Measured 2.34 validate --help initializes XDG_DATA_HOME even with KIRO_HOME.
    # Keep that initialization inside held private roots, preserving original HOME.
    for key, name in (("XDG_CONFIG_HOME", "config"), ("XDG_CACHE_HOME", "cache"),
                      ("XDG_STATE_HOME", "state"), ("XDG_DATA_HOME", "data")):
        directory = profile / name
        directory.mkdir(mode=0o700)
        root_ids[directory] = kiro._research_root_identity(directory)
        env[key] = str(directory)
    kiro._verify_root_identities(root_ids)
    research.verify_files(ctx.identities, ctx.budget)
    ctx.summary.setdefault("private_profile_routes", []).append({
        "probe": label, "home": env["HOME"], "kiro_home": env["KIRO_HOME"],
        "xdg": {key: env[key] for key in ("XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME")}})
    try:
        return ctx.command(label, [*ctx.argv, *arguments], ctx.projects[project], env, timeout=20)
    finally:
        research.verify_files(ctx.identities, ctx.budget)
        kiro._runtime_bin(ctx)
        kiro._verify_root_identities(root_ids)
        for root, expected in owned.items():
            if tree_identity(root) != expected:
                raise ValueError("native probe changed owned/foreign fixture")


def exact_refusal(record, stdout, stderr):
    return (isinstance(record, dict) and type(record.get("exit")) is int and record["exit"] == 1
            and "error" in record and record["error"] is None
            and stdout == b"" and stderr == AUTH_REFUSAL)


def exact_success(record, stdout, stderr, expected):
    return (isinstance(record, dict) and type(record.get("exit")) is int and record["exit"] == 0
            and "error" in record and record["error"] is None
            and stdout == expected and stderr == b"")


def status_help_supported(record, stdout, stderr):
    """Syntax gate only; the first original help still needs independent review."""
    if (not exact_success(record, stdout, stderr, stdout) or not isinstance(stdout, bytes)
            or not 0 < len(stdout) <= 4096):
        return False
    try:
        text = kiro._text(stdout)
    except (ValueError, UnicodeError):
        return False
    return (len(re.findall(r"(?m)^Usage: kiro-cli mcp status(?:[ \t][^\n]*)?$", text)) == 1
            and re.search(r"(?m)^[ \t]+(?:-[A-Za-z],[ \t]+)?--name[ =]<[A-Z_]+>(?:[ \t]|$)", text) is not None)


def collect(ctx):
    owned, fixtures = prepare(ctx)
    roots = [Path(ctx.root), ctx.home, ctx.private, *(ctx.projects[key] for key in ("a", "b", "user"))]
    root_ids = {Path(path): kiro._research_root_identity(Path(path)) for path in roots}
    if ctx.env.get("PATH") != "/usr/bin:/bin":
        raise ValueError("original closed PATH required")
    ctx.env["PATH"] = str(kiro._runtime_bin(ctx)) + ":/usr/bin:/bin"
    table = routes(fixtures, ctx)
    record, stdout, stderr = probe(ctx, table, owned, root_ids, "version", ("--version",), "a")
    if not exact_success(record, stdout, stderr, b"kiro-cli 2.34.0\n"):
        raise ValueError("exact pinned Kiro version required")
    record, stdout, stderr = probe(ctx, table, owned, root_ids, "validate-help", HELP, "a")
    if not exact_success(record, stdout, stderr, VALIDATE_HELP):
        raise ValueError("unreviewed pinned validation grammar")
    validations = []
    ctx.summary["validation_observations"] = validations
    for name in ("valid", "invalid", "missing"):
        label = "validate-" + name
        project, arguments = table[label]
        record, stdout, stderr = probe(ctx, table, owned, root_ids, label, arguments, project)
        if not exact_refusal(record, stdout, stderr):
            raise ValueError("unreviewed no-auth validation result: " + label)
        validations.append({"probe": label, "fixture": name,
                            "exact_auth_refusal_observed": True,
                            "validation_qualified": False, "validation_schema_observed": False,
                            "status": "expected-auth-refusal"})
    refusals = []
    for label, (project, arguments) in table.items():
        if label.startswith("validate") or label in ("version", "mcp-status-help") or label.endswith("-mcp-status"):
            continue
        record, stdout, stderr = probe(ctx, table, owned, root_ids, label, arguments, project)
        if not exact_refusal(record, stdout, stderr):
            raise ValueError("unreviewed no-auth inventory result: " + label)
        refusals.append({"probe": label, "status": "expected-auth-refusal",
                         "native_discovery_qualified": False})
    ctx.summary["expected_auth_refusals"] = refusals
    status = []
    ctx.summary["status_observations"] = status
    ctx.summary["status_observation_qualified"] = False
    record, stdout, stderr = probe(ctx, table, owned, root_ids, "mcp-status-help", STATUS_HELP, "a")
    if not status_help_supported(record, stdout, stderr):
        raise ValueError("unreviewed pinned MCP status grammar")
    ctx.summary["status_help_syntax_observed"] = True
    for project in ("a", "b", "user"):
        label = project + "-mcp-status"
        selected_project, arguments = table[label]
        record, stdout, stderr = probe(ctx, table, owned, root_ids, label, arguments, selected_project)
        if not exact_refusal(record, stdout, stderr):
            raise ValueError("unreviewed no-auth status result: " + label)
        status.append({"probe": label, "status": "expected-auth-refusal", "native_status_qualified": False})
    ctx.summary["fixture_preservation_verified"] = True
    ctx.summary["finite_capture_complete"] = True
    ctx.summary["expected_auth_refusal_count"] = len(validations) + len(refusals) + len(status)
    ctx.summary["validation_schema_observed"] = False
    ctx.summary["native_discovery_observed"] = False
    ctx.summary["boundary"] = "exact original help/12 denials plus status syntax gate/3 status denials; no native status, schema validation or loading qualified"


def run(output):
    research.native.require_hosted("linux-amd64")
    if output.resolve() != output or output.exists() or output == research.SOURCE or research.SOURCE in output.parents:
        raise ValueError("fresh output outside source required")
    output.mkdir(mode=0o700, parents=True)
    summary = {"client": "kiro", "phase": "no-auth-installation-observation",
               "collection_complete": False, "commands": [], "cases": [],
               "qualification": {name: False for name in QUALIFICATIONS},
               "uap_lifecycle_suite": "separate workflow jobs; no native qualification inferred"}
    ctx = None
    try:
        with research.Budget(360) as budget:
            ctx = research.Context(output, summary, budget)
            # Never read or register KIRO_API_KEY or any ambient credential.
            ctx.source_and_projects()
            receipt, contents = research.acquire(kiro.ACQUISITION, budget)
            summary["acquisition"] = receipt
            binary, identities = research.materialize(output / "client", kiro.ACQUISITION, receipt, contents)
            ctx.argv, ctx.identities = [str(binary)], identities
            collect(ctx)
            summary["collection_complete"] = True
    except Exception as error:
        summary["error"] = type(error).__name__ + ": " + str(error)[:512]
    finally:
        research.publish(summary, ctx, output)
        print(json.dumps({key: summary.get(key) for key in
                          ("client", "collection_complete", "finite_capture_complete", "source_commit", "source_tree", "error")}))
    return 0 if summary["collection_complete"] else 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    raise SystemExit(run(parser.parse_args().output))
