"""Pinned native API admission research; never product or activation acceptance.

Read-only native selection probes run inside fresh disposable HOME/projects.
Grok enablement stays refused until identity, read-back and recovery are proven.
No MCP process, provider or real profile is used.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile

CLIENTS = ("cline", "grok")
NAME = "uap-automation-probe"
SOURCE = Path(__file__).resolve().parent.parent


def require(ok, message):
    if not ok:
        raise ValueError(message)


def put(path, body):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    with path.open("x", encoding="utf-8") as stream:
        stream.write(body if isinstance(body, str) else json.dumps(body, sort_keys=True))
    path.chmod(0o600)


def read_anchored_snapshot(root_fd, parts, expected):
    """Open each component beneath the owned directory without following aliases."""
    flags = os.O_RDONLY | os.O_NOFOLLOW | getattr(os, "O_CLOEXEC", 0)
    opened = []
    parent = root_fd
    try:
        for segment in parts[:-1]:
            child = os.open(segment, flags | os.O_DIRECTORY, dir_fd=parent)
            opened.append(child)
            parent = child
        leaf = os.open(parts[-1], flags | os.O_NONBLOCK, dir_fd=parent)
        opened.append(leaf)
        before = os.fstat(leaf)
        require(stat.S_ISREG(before.st_mode) and before.st_size <= 1 << 20
                and before.st_nlink == 1,
                "config capture bound or linked file")
        identity = lambda meta: (meta.st_dev, meta.st_ino, meta.st_mode, meta.st_size,
                                 meta.st_mtime_ns, meta.st_ctime_ns, meta.st_nlink)
        require(identity(before) == identity(expected), "config changed before capture")
        with os.fdopen(os.dup(leaf), "rb") as stream:
            raw = stream.read((1 << 20) + 1)
        require(len(raw) <= 1 << 20 and identity(os.fstat(leaf)) == identity(before),
                "config changed during capture")
        return raw
    finally:
        for descriptor in reversed(opened):
            os.close(descriptor)


def read_anchored_directory_mode(root_fd, parts, expected):
    """Record owned directory metadata only after no-follow identity validation."""
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | getattr(os, "O_CLOEXEC", 0)
    opened = []
    parent = root_fd
    try:
        for segment in parts:
            child = os.open(segment, flags, dir_fd=parent)
            opened.append(child)
            parent = child
        actual = os.fstat(parent)
        identity = lambda meta: (meta.st_dev, meta.st_ino, meta.st_mode,
                                 meta.st_mtime_ns, meta.st_ctime_ns)
        require(stat.S_ISDIR(actual.st_mode) and identity(actual) == identity(expected),
                "config directory changed before capture")
        return stat.S_IMODE(actual.st_mode)
    finally:
        for descriptor in reversed(opened):
            os.close(descriptor)


def snapshot(root):
    """Capture only hashes/metadata, never native-generated file contents or foreign data."""
    require(root.is_dir() and root.resolve() == root and not root.is_symlink(),
            "snapshot root is missing or aliased")
    require(os.name == "posix" and all(hasattr(os, flag) for flag in
            ("O_DIRECTORY", "O_NOFOLLOW", "O_NONBLOCK"))
            , "secure snapshot unavailable")
    root_meta = root.lstat()
    directory_flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | getattr(os, "O_CLOEXEC", 0)
    root_fd = os.open(root, directory_flags)
    try:
        observed = os.fstat(root_fd)
        require((root_meta.st_dev, root_meta.st_ino, root_meta.st_mode)
                == (observed.st_dev, observed.st_ino, observed.st_mode)
                and stat.S_ISDIR(observed.st_mode),
                "snapshot root changed")
        # Include the watched directory itself. Without this record, a native
        # command could chmod a watched Project/User root without adding or
        # changing any child and the no-effect comparison would still pass.
        out = {".": {"kind": "directory", "mode": stat.S_IMODE(observed.st_mode)}}
        # Bound directory entries as well as regular files. Eager sorting or
        # counting only captured files lets a native-generated directory tree
        # exhaust memory/CPU before the snapshot limit can fire.
        for count, path in enumerate(root.rglob("*"), start=1):
            relative = path.relative_to(root)
            require(count <= 256 and 0 < len(relative.parts) <= 32,
                    "config inventory bound")
            meta = path.lstat()
            require(not stat.S_ISLNK(meta.st_mode), "native probe created an alias")
            if stat.S_ISDIR(meta.st_mode):
                # Empty directories and mode changes are foreign-state changes,
                # even when no regular file has been added or removed.
                out[str(relative)] = {
                    "kind": "directory",
                    "mode": read_anchored_directory_mode(root_fd, relative.parts, meta),
                }
                continue
            require(stat.S_ISREG(meta.st_mode) and meta.st_size <= 1 << 20,
                    "config capture bound")
            raw = read_anchored_snapshot(root_fd, relative.parts, meta)
            out[str(relative)] = {
                "sha256": hashlib.sha256(raw).hexdigest(), "mode": stat.S_IMODE(meta.st_mode),
                "bytes": len(raw),
            }
        require(os.fstat(root_fd).st_mode == observed.st_mode,
                "snapshot root mode changed during capture")
        return out
    finally:
        os.close(root_fd)


def native_json(body):
    """Strict bounded native JSON, never help text or an exit-code substitute."""
    require(isinstance(body, (str, bytes)) and 0 < len(body) <= 1 << 20, "native JSON bound")
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, "duplicate native JSON key")
            result[key] = value
        return result
    def invalid_constant(_value):
        raise ValueError("nonfinite native JSON value")
    try:
        return json.loads(body, object_pairs_hook=pairs, parse_constant=invalid_constant)
    except (UnicodeError, RecursionError) as error:
        raise ValueError("invalid native JSON encoding or depth") from error


def successful_capture(record):
    require("error" in record and record["error"] is None, "native capture failed")
    require(type(record.get("exit")) is int and record["exit"] == 0,
            "native command failed: " + str(record.get("probe", "unknown")))


def check_version(client, stdout):
    expected = {"cline": b"3.0.73", "grok": b"grok 1.0.70 (f50904eaee6c)"}
    require(client in expected and stdout in (expected[client], expected[client] + b"\n",
                                              expected[client] + b"\r\n"),
            "native version differs from pinned acquisition")


def cline_inventory(body, path, name):
    # Candidate schema from cline/cline@50e77bc1 apps/cli/src/commands/config.ts.
    # The frozen executable must confirm it; there is no help/text fallback.
    rows = native_json(body)
    require(isinstance(rows, list) and len(rows) == 1 and isinstance(rows[0], dict),
            "Cline exact single-server inventory required")
    row = rows[0]
    require(row.get("name") == name and row.get("path") == str(path)
            and row.get("transportType") == "stdio" and row.get("disabled") is True,
            "Cline selected source or disabled sentinel differs")
    return row


def require_admission(evidence):
    require(evidence.get("research_completed") is True
            and evidence.get("native_api_admitted") is True,
            "native API not admitted; collected diagnostics are not acceptance")
    require(evidence.get("qualified_product_e2e") is False,
            "research cannot qualify product E2E")


def exact_plugin_id(body, path):
    """Accept only an explicit scoped native ID joined to one exact path."""
    value = native_json(body)
    rows = value if isinstance(value, list) else value.get("plugins") if isinstance(value, dict) else None
    require(isinstance(rows, list), "unqualified native list envelope")
    matches = [row for row in rows if isinstance(row, dict) and row.get("path") == str(path)]
    require(len(matches) == 1, "exact native plugin path missing or duplicated")
    selected = matches[0]
    ident = selected.get("id")
    require(selected.get("name") == NAME and selected.get("scope") == "project"
            and isinstance(ident, str) and re.fullmatch(r"project/[A-Za-z0-9_-]{4,128}/" + NAME, ident),
            "no qualified project-native full ID")
    return ident


def grok_readonly_project_inventory(body, project, selected_path):
    """Prove the exact disabled project plugin was *seen*, not enabled."""
    facts = native_json(body)
    require(isinstance(facts, dict) and facts.get("grokVersion") == "1.0.70"
            and facts.get("cwd") == str(project)
            and facts.get("projectRoot") in (str(project), str(project) + os.sep)
            and facts.get("projectTrusted") is False,
            "Grok native selected untrusted project differs")
    rows = facts.get("plugins")
    require(isinstance(rows, list) and len(rows) <= 256,
            "Grok native project plugin inventory invalid")
    seen = set()
    matched = []
    for row in rows:
        require(isinstance(row, dict) and isinstance(row.get("name"), str)
                and bool(row["name"].strip()), "Grok native plugin identity invalid")
        key = row["name"].casefold()
        require(key not in seen, "Grok native plugin identity duplicated")
        seen.add(key)
        if row["name"] == NAME:
            require(row.get("scope") == "project"
                    and row.get("path") == str(selected_path)
                    and row.get("enabled") is False,
                    "Grok native project source or disabled state differs")
            matched.append({"name": NAME, "scope": "project",
                            "path": str(selected_path), "enabled": False})
    require(len(matched) == 1, "Grok exact disabled Project plugin not discovered")
    return matched[0]


class Probe:
    def __init__(self, root, binary, evidence):
        self.root, self.binary, self.evidence = root, binary, evidence
        self.home, self.a, self.b = [root / part for part in ("home", "project-a", "project-b")]
        for path in (self.home, self.a, self.b, root / "tmp"):
            path.mkdir(mode=0o700)
        self.env = {"HOME": str(self.home), "PATH": str(binary.parent) + ":/usr/bin:/bin",
                    "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8", "NO_COLOR": "1", "TERM": "dumb",
                    "TMPDIR": str(root / "tmp"), "GIT_CONFIG_NOSYSTEM": "1",
                    "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_TERMINAL_PROMPT": "0",
                    "GROK_DISABLE_AUTOUPDATER": "1", "CLINE_NO_AUTO_UPDATE": "1"}
        for part in ("CONFIG", "DATA", "CACHE", "STATE"):
            directory = root / ("xdg-" + part.lower())
            directory.mkdir(mode=0o700)
            self.env["XDG_" + part + "_HOME"] = str(directory)
        for project in (self.a, self.b):
            self.run("git-" + project.name, ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"], project)

    def run(self, label, args, cwd=None, overrides=None):
        from native_client_research import run_probe
        record, stdout, stderr = run_probe([str(item) for item in args], cwd or self.a,
            dict(self.env, **(overrides or {})), self.root, label, timeout=30,
            limit=1 << 20, memory_only=True)
        self.evidence["commands"].append(dict(record,
            stdout_base64=base64.b64encode(stdout).decode(),
            stderr_base64=base64.b64encode(stderr).decode()))
        successful_capture(record)
        return record["exit"], stdout

    def grok(self):
        for label, args in (("version", ["--version"]), ("plugin-help", ["plugin", "--help"]),
                            ("enable-help", ["plugin", "enable", "--help"]),
                            ("list-help", ["plugin", "list", "--help"])):
            _, output = self.run(label, [self.binary, *args])
            if label == "version":
                check_version("grok", output)
        put(self.home / ".grok/config.toml",
            '[marketplace]\ndefault_skills_installs_purged = true\n[features]\nturn_summary = false\n')
        roots = [self.home / ".grok/plugins/user-probe", self.a / ".grok/plugins/project-probe",
                 self.b / ".grok/plugins/project-probe"]
        for index, root in enumerate(roots):
            put(root / ".grok-plugin/plugin.json", {"name": NAME, "version": "1.0.0", "skills": "./skills/"})
            put(root / ("skills/probe-" + str(index) + "/SKILL.md"),
                "---\nname: probe-" + str(index) + "\ndescription: Inert scope identity fixture\n---\nText only.\n")
        config = self.home / ".grok/config.toml"
        original_config = config.read_bytes()
        watched = {"user_plugin": roots[0], "a": self.a, "b": self.b}
        before = {name: snapshot(path) for name, path in watched.items()}
        self.evidence["before"] = before
        self.evidence["native_api_admitted"] = False
        try:
            _, listing = self.run("list-before-a", [self.binary, "plugin", "list", "--json"])
            self.run("list-before-b", [self.binary, "plugin", "list", "--json"], self.b)
            _, inspected_a = self.run("inspect-before-a", [self.binary, "inspect", "--json"])
            _, inspected_b = self.run("inspect-before-b", [self.binary, "inspect", "--json"], self.b)
            self.evidence["project_inventory"] = {
                "a": grok_readonly_project_inventory(inspected_a, self.a, roots[1]),
                "b": grok_readonly_project_inventory(inspected_b, self.b, roots[2]),
            }
            # Inspect proves exact prepared native discovery, not a safe
            # scope-qualified enable command; `plugin enable` accepts a name.
            # Never invent a native ID or fall back to a name-only mutator.
            try:
                ident = exact_plugin_id(listing, roots[1])
            except (ValueError, TypeError) as error:
                self.evidence["enable_admission"] = {"admitted": False, "reason": str(error)}
            else:
                # Identity alone does not prove selective mutation or recovery.
                self.evidence["enable_admission"] = {"admitted": False, "id": ident,
                    "reason": "exact enable/read-back/recovery remain unqualified"}
        finally:
            after = {name: snapshot(path) for name, path in watched.items()}
            self.evidence["after"] = after
            require(after == before, "read-only collection changed fixture state")
            require(not config.is_symlink() and config.read_bytes() == original_config,
                    "read-only collection changed Grok user configuration")
            trust = self.home / ".grok/trusted_folders.toml"
            require(not trust.exists() and not trust.is_symlink(), "native experiment changed trust")

    def cline(self):
        self.evidence["native_api_admitted"] = False
        for label, args in (("version", ["--version"]), ("help", ["--help"]),
                            ("config-help", ["config", "--help"])):
            _, output = self.run(label, [self.binary, "--auto-approve", "false", *args])
            if label == "version":
                check_version("cline", output)
        # The command reads configuration, with both sentinels disabled; no
        # server connection or model/credential is used to prove selection.
        user = self.home / ".cline/data/settings/cline_mcp_settings.json"
        project = self.a / ".uap/clients/cline/mcp.json"
        for path, name in ((user, "user-sentinel"), (project, "project-sentinel")):
            put(path, {"mcpServers": {name: {"command": "/usr/bin/false", "disabled": True,
                "autoApprove": [], "args": []}}})
        watched = {"user": user.parent, "project": project.parent, "b": self.b}
        before = {name: snapshot(path) for name, path in watched.items()}
        self.evidence["configuration_before"] = before
        selected = {"CLINE_MCP_SETTINGS_PATH": str(project)}
        observations = []
        self.evidence["source_selection"] = observations
        cases = (("mcp-default", self.a, {}, user, "user-sentinel"),
                 ("mcp-selected-a", self.a, selected, project, "project-sentinel"),
                 ("mcp-default-after-a", self.a, {}, user, "user-sentinel"),
                 ("mcp-default-b", self.b, {}, user, "user-sentinel"))
        for label, cwd, overrides, path, name in cases:
            try:
                _, body = self.run(label, [self.binary, "--auto-approve", "false",
                                          "config", "mcp", "--json"], cwd, overrides)
                observations.append({"probe": label, "server": cline_inventory(body, path, name)})
            finally:
                after = {name: snapshot(path) for name, path in watched.items()}
                self.evidence["configuration"] = after
                require(after == before, "Cline observation changed fixture configuration or Project B")
        self.evidence["selected_environment"] = dict(selected, CLINE_NO_AUTO_UPDATE="1")
        self.evidence["native_api_admitted"] = True


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client", required=True, choices=CLIENTS)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    require(os.environ.get("GITHUB_ACTIONS") == "true"
            and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted", "disposable hosted runner required")
    expected = os.environ.get("EXPECTED_COMMIT", "")
    require(re.fullmatch(r"[a-f0-9]{40}", expected), "exact source SHA required")
    actual = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD"], cwd=SOURCE, timeout=10).decode().strip()
    require(actual == expected, "checkout differs from reviewed head")
    tree = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD^{tree}"], cwd=SOURCE, timeout=10).decode().strip()
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    evidence = {"schema_version": 1, "client": args.client, "source_commit": actual,
                "source_tree": tree, "commands": [], "research_completed": False,
                "native_api_admitted": False, "qualified_product_e2e": False, "provider_authentication": False, "model_invoked": False}
    try:
        from project_native_research import Budget, acquire, materialize, verify_files
        if args.client == "grok":
            from project_native_collectors.grok import ACQUISITION
        else:
            from project_native_collectors.cline import ACQUISITION
        budget = Budget(600)
        receipt, contents = acquire(ACQUISITION, budget)
        evidence["acquisition"] = receipt
        with tempfile.TemporaryDirectory(prefix="uap-native-api-", dir=os.environ["RUNNER_TEMP"]) as temporary:
            root = Path(temporary).resolve()
            (root / "bin").mkdir(mode=0o700)
            binary, identities = materialize(root / "bin", ACQUISITION, receipt, contents)
            probe = Probe(root, binary, evidence)
            getattr(probe, args.client)()
            verify_files(identities, budget)
        evidence["research_completed"] = True
        require_admission(evidence)
    except Exception as error:
        evidence["error"] = type(error).__name__ + ": " + str(error)
        raise
    finally:
        put(args.output / "native-api.json", evidence)
        print(json.dumps({"research_completed": evidence["research_completed"],
                          "native_api_admitted": evidence["native_api_admitted"],
                          "qualified_product_e2e": False, "client": args.client}))


if __name__ == "__main__":
    main()
