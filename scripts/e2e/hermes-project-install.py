#!/usr/bin/env python3
"""Pinned official Hermes file installation and explicit inspection; no tools/model calls."""
from contextlib import ExitStack
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from hermes_install_acceptance import ordinary, payload_contract, snapshot, supervised
import native_client_project_artifacts as files

PIN = "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19"
YAML_POLICY_GIT_BLOB = "4cd43d3cbda1f22974c29fc59dbabef50455f892"


def require(value, message):
    if not value:
        raise RuntimeError(message)


# The fixed upstream command uses hermes_yaml's YAML 1.1 safe loader. Decode
# captured bytes with that same pinned payload, not the host Python environment.
CONFIG_PARSER = r"""
import copy, importlib.util, json, pathlib, sys, warnings
try:
    code, packages = sys.argv[1:]
    sys.path.insert(0, packages)
    warnings.simplefilter("error")
    spec = importlib.util.spec_from_file_location("hermes_yaml", pathlib.Path(code) / "hermes_yaml.py")
    policy = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(policy)
    safe_load = policy.safe_load
    packet = sys.stdin.buffer.read((1 << 20) + 1)
    if len(packet) > 1 << 20:
        raise ValueError()
    packet = json.loads(packet)
    if type(packet) is not dict or set(packet) != {"before", "after", "project"} or any(type(value) is not str for value in packet.values()):
        raise ValueError()
    before, after = (safe_load(packet[key]) for key in ("before", "after"))
    count = 0
    def canonical(value, depth=0):
        global count
        count += 1
        if depth > 32 or count > 8192:
            raise ValueError()
        if type(value) is dict:
            if any(type(key) is not str for key in value):
                raise ValueError()
            return {key: canonical(item, depth + 1) for key, item in value.items()}
        if type(value) is list:
            return [canonical(item, depth + 1) for item in value]
        if value is None or type(value) in (str, int, bool, float):
            return value
        raise ValueError()
    before, after = canonical(before), canonical(after)
    if type(before) is not dict or type(after) is not dict or before.get("theme") != "retained-user":
        raise ValueError()
    if type(before.get("_config_version")) is not int or before["_config_version"] != 52:
        raise ValueError()
    expected = copy.deepcopy(before)
    skills = expected.setdefault("skills", {})
    if type(skills) is not dict or skills.get("trusted_project_dirs", []) != []:
        raise ValueError()
    skills["trusted_project_dirs"] = [packet["project"]]
    encode = lambda value: json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)
    if encode(after) != encode(expected):
        raise ValueError()
    print("config-preservation-ok")
except Exception:
    sys.stderr.write("config-preservation-refused\n")
    sys.exit(1)
"""


def directory_identity(info):
    return info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid


def directory_info(path):
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode) and not path.is_symlink()
            and not getattr(info, "st_file_attributes", 0) & 0x400
            and path.resolve() == path, "retained fixture directory changed")
    return info


def hold_directory(path, cleanup):
    before = directory_info(path)
    if os.name != "nt":
        descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    else:
        import ctypes
        from ctypes import wintypes
        import msvcrt
        kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        create, close = kernel.CreateFileW, kernel.CloseHandle
        create.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD,
                           ctypes.c_void_p, wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
        create.restype = wintypes.HANDLE
        close.argtypes, close.restype = [wintypes.HANDLE], wintypes.BOOL
        # Read attributes; share read/write/delete; open the directory itself.
        handle = create(str(path), 0x80, 7, None, 3, 0x02200000, None)
        require(handle not in (None, ctypes.c_void_p(-1).value), "fixture directory open failed")
        try:
            descriptor = msvcrt.open_osfhandle(handle, os.O_RDONLY | os.O_BINARY)
            if descriptor < 0:
                raise OSError("fixture directory descriptor transfer failed")
        except BaseException:
            if not close(handle):
                raise ctypes.WinError(ctypes.get_last_error())
            raise
    cleanup.callback(os.close, descriptor)
    identity = directory_identity(before)
    require(identity == directory_identity(os.fstat(descriptor))
            == directory_identity(directory_info(path)), "fixture directory identity changed")
    return path, descriptor, identity


def captured_config(path):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1, "selected config is not one ordinary file")
    body = ordinary(path, 64 << 10)
    require(files.identity(info) == files.identity(path.lstat()), "selected config changed during capture")
    return body, info, [*files.identity(info), hashlib.sha256(body).hexdigest()]


def config_file_policy(info):
    return (info.st_dev, info.st_mode, info.st_nlink, info.st_uid, info.st_gid,
            getattr(info, "st_file_attributes", None), getattr(info, "st_reparse_tag", None))


class ProjectHomePreservation:
    """Retain the original HOME; authorize only the one validated trust write."""

    def __init__(self, home, profile, evidence, payload, provenance, env, deadline, cleanup):
        self.home, self.profile, self.evidence = home, profile, evidence
        self.payload, self.provenance, self.env, self.deadline = payload, provenance, env, deadline
        self.config = profile / "config.yaml"
        self.relative = str(self.config.relative_to(home))
        parent, self.backups = profile / "backups", profile / "backups/config"
        parent.mkdir(mode=0o700, exist_ok=True)
        directory_info(parent)
        self.archive = evidence / "user-config-backups"
        self.archived = None
        if self.backups.exists() or self.backups.is_symlink():
            before = snapshot(self.backups)
            require(not self.archive.exists() and not self.archive.is_symlink(), "backup evidence already exists")
            self.backups.rename(self.archive)
            require(snapshot(self.archive) == before, "User backup archive changed during preparation")
            self.archived = before
        self.backups.mkdir(mode=0o700)
        sentinel = self.backups / "foreign-sentinel"
        with sentinel.open("xb") as stream:
            stream.write(b"retained foreign backup member\n")
        sentinel.chmod(0o600)
        paths = [home, profile, parent, self.backups] + ([self.archive] if self.archived is not None else [])
        self.directories = [hold_directory(path, cleanup) for path in paths]
        self.before_body, self.before_info, self.before_config = captured_config(self.config)
        self.original = snapshot(home)
        require(self.original[self.relative] == self.before_config, "initial config snapshot differs")
        self.final = None
        self.admitted_backups = {}
        self.started = time.time()
        (evidence / "project-backup-preparation.json").write_text(json.dumps({
            "phase": "before-project-baseline", "config_schema_authored_before_native": 52,
            "archived_user_backups": self.archived,
            "retention_limit": 5, "initial_good_backups": 0,
            "foreign_sentinel": str(sentinel.relative_to(home))}, sort_keys=True) + "\n")

    def directories_unchanged(self):
        for path, descriptor, expected in self.directories:
            require(directory_identity(directory_info(path)) == expected
                    == directory_identity(os.fstat(descriptor)), "original fixture directory replaced")
        if self.archived is not None:
            require(snapshot(self.archive) == self.archived, "retained User backup archive changed")

    def authorize_trust(self, project):
        self.directories_unchanged()
        body, info, row = captured_config(self.config)
        after = snapshot(self.home)
        require(after[self.relative] == row, "trusted config snapshot differs")
        require(config_file_policy(info) == config_file_policy(self.before_info),
                "trust changed selected config type, mode or ownership")
        manifest_body = ordinary(self.payload / "manifest.json", 64 << 10)
        require(hashlib.sha256(manifest_body).hexdigest() == self.provenance["manifest_sha256"],
                "parser runtime manifest changed")
        runtime = json.loads(manifest_body)["runtime"]
        code, packages, python = (self.payload / runtime[key] for key in ("repoDir", "sitePackages", "storePython"))
        # payload_contract already joins these paths to the pinned prepared lock.
        for path in (code, packages, python):
            require(path.resolve().is_relative_to(self.payload), "config parser runtime escapes")
        parser = ordinary(code / "hermes_yaml.py", 16 << 10)
        require(hashlib.sha1(b"blob " + str(len(parser)).encode() + b"\0" + parser).hexdigest()
                == YAML_POLICY_GIT_BLOB, "pinned YAML policy changed")
        packet = json.dumps({"before": self.before_body.decode("utf-8"), "after": body.decode("utf-8"),
                             "project": str(project.resolve())}).encode()
        require(len(packet) <= 1 << 20, "captured config parser input exceeds bound")
        remaining = min(5, self.deadline - time.monotonic())
        require(remaining > 0, "native lifecycle deadline expired")
        # This isolated, fixed safe-parser child has no application imports or
        # child launches. subprocess.run kills and reaps it on its five-second
        # deadline; every real Hermes/UAP command still uses the original owner.
        parsed = subprocess.run([str(python), "-I", "-S", "-B", "-c", CONFIG_PARSER, str(code), str(packages)],
                                input=packet, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                cwd=self.payload, env=self.env, timeout=remaining, check=False)
        require(parsed.returncode == 0 and parsed.stdout == b"config-preservation-ok\n"
                and not parsed.stderr, "selected config changed beyond trust and schema 52")
        require(ordinary(code / "hermes_yaml.py", 16 << 10) == parser, "YAML policy changed during validation")
        require(snapshot(self.home) == after, "HOME changed during captured config validation")
        self.final = body, info, row
        require(self.checked(after) == self.original, "unselected HOME authority changed")

    def checked(self, after):
        self.directories_unchanged()
        require(self.final is not None and after.get(self.relative) == self.final[2],
                "validated selected config changed again")
        new = set(after) - set(self.original)
        require(len(new) <= 2 and set(self.admitted_backups) <= new, "unexpected backup retention or removal")
        for relative, row in self.admitted_backups.items():
            require(after[relative] == row, "previously admitted backup changed")
        elapsed = time.time() - self.started
        require(0 <= elapsed <= 360, "backup timestamp interval exceeds fixture deadline")
        stamps = {time.strftime("%Y%m%d-%H%M%S", time.localtime(second))
                  for second in range(int(self.started), int(self.started + elapsed) + 1)}
        frames = [(self.before_body, self.before_info), (self.final[0], self.final[1])]
        candidates, states = {}, []
        for relative in sorted(new):
            path = self.home / relative
            require(path.parent == self.backups
                    and re.fullmatch(r"config\.yaml\.good\.[0-9]{8}-[0-9]{6}", path.name)
                    and path.name[len("config.yaml.good."):] in stamps, "unknown HOME or backup member")
            body, info, row = captured_config(path)
            require(row == after[relative], "backup capture differs from HOME snapshot")
            matching = [index for index, (content, source) in enumerate(frames)
                        if body == content and config_file_policy(info) == config_file_policy(source)
                        and info.st_mtime_ns == source.st_mtime_ns]
            require(len(matching) == 1 and matching[0] not in states, "backup is not an exact declared config state")
            states.append(matching[0])
            candidates[relative] = row
        require(states == sorted(states), "backup states are out of writer order")
        projected = {key: value for key, value in after.items() if key not in candidates}
        projected[self.relative] = self.before_config
        require(projected == self.original, "original HOME member changed")
        self.directories_unchanged()
        require(snapshot(self.home) == after, "HOME changed during backup validation")
        self.admitted_backups = candidates
        return projected


def main():
    with ExitStack() as cleanup:
        return run_lifecycle(cleanup)


def run_lifecycle(cleanup):
    runner = Path(os.environ["RUNNER_TEMP"]).resolve()
    evidence = Path(tempfile.mkdtemp(prefix="hermes-project-install-", dir=runner))
    root = evidence / "fixture"
    root.mkdir(mode=0o700)
    home, a, b, source = (root / name for name in ("home", "project-a", "project-b", "source"))
    for directory in (home, a, b, source):
        directory.mkdir(mode=0o700)
    env = dict(os.environ)
    bundle = Path(env["HERMES_PROJECT_BUNDLE"]).resolve()
    uap = Path(env["UAP_BIN"]).resolve()
    require(bundle.name == ("hermes.exe" if os.name == "nt" else "hermes"), "official launcher required")
    env.update(HOME=str(home), USERPROFILE=str(home), HERMES_HOME=str(home / "selected"),
               PATH=str(bundle.parent) + os.pathsep + env["PATH"], AGENTPLUGINS_HOME=str(root / "uap"))
    for key, relative in (("APPDATA", "AppData/Roaming"), ("LOCALAPPDATA", "AppData/Local"),
                          ("XDG_CONFIG_HOME", "config"), ("XDG_CACHE_HOME", "cache"),
                          ("XDG_DATA_HOME", "data"), ("XDG_STATE_HOME", "state")):
        directory = home / relative
        directory.mkdir(parents=True, mode=0o700, exist_ok=True)
        env[key] = str(directory)
    provenance = payload_contract(bundle.parent.parent, runner / "hermes-agent", PIN)
    for member in (bundle.parent.parent / "manifest.json", bundle.parent.parent.with_name(bundle.parent.parent.name + ".prepared.json")):
        (evidence / member.name).write_bytes(ordinary(member, 1 << 20))
    records = []
    deadline = time.monotonic() + 360

    def run(label, args, cwd=a, selected=env):
        remaining = deadline - time.monotonic()
        require(remaining > 0, "native lifecycle deadline expired")
        record, stdout, stderr = supervised(args, cwd, selected, timeout=max(1, int(min(60, remaining))))
        (evidence / (label + ".stdout")).write_bytes(stdout)
        (evidence / (label + ".stderr")).write_bytes(stderr)
        records.append(dict(record, label=label))
        return stdout

    def cli(label, *args, project=True):
        scope = ["--scope", "project", "--project-root", str(a)] if project else ["--scope", "user"]
        return run(label, [uap, *args, *scope, "--target", "hermes", "--yes", "--format", "json"])

    for project in (a, b):
        run("git-init-" + project.name, ["git", "init", "-q", project], cwd=root)
        (project / "foreign.txt").write_text("retained sibling/foreign file\n")
    profile = home / "selected"
    profile.mkdir(mode=0o700)
    # DEFAULT_CONFIG at PIN declares schema 52. Author it before either native
    # lifecycle; the Project phase may change only the selected trust list.
    (profile / "config.yaml").write_text("theme: retained-user\nmcp_servers: {}\n_config_version: 52\n")
    (profile / "config.yaml").chmod(0o600)
    (profile / "auth-sentinel").write_text("inert credentials sentinel\n")
    skill = source / "skills" / "hermes-project-install" / "SKILL.md"
    skill.parent.mkdir(parents=True)

    def package(version):
        (source / "plugin.json").write_text(json.dumps({
            "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": "hermes-project-install", "version": version}))
        skill.write_text("---\nname: hermes-project-install\ndescription: Offline installation fixture.\n---\n" + version)
        (source / "mcp.json").write_text(json.dumps({
            "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {"inventory-only": {"type": "streamable-http", "url": "http://127.0.0.1:1/mcp"}}}))

    package("1.0.0")
    run("version", [bundle, "--version"], cwd=root)
    require(b"Hermes Agent vunknown (2026.9.24)" in (evidence / "version.stdout").read_bytes(), "pinned banner differs")
    # Existing User support is exercised with the same official bundle. These
    # observations read config only; no transport or native tools command runs.
    cli("user-add", "add", str(source), "--no-save", project=False)
    user_inventory = json.loads(run("user-native-config", [bundle, "config", "get", "mcp_servers", "--json"], cwd=root))
    require(user_inventory == {"inventory-only": {"url": "http://127.0.0.1:1/mcp"}}, "complete User inventory differs")
    cli("user-repeat", "add", str(source), "--no-save", project=False)
    package("1.1.0")
    cli("user-update", "update", "hermes-project-install", project=False)
    owned_user_skill = profile / "skills" / "hermes-project-install" / "SKILL.md"
    owned_user_skill.unlink()
    owned_user_skill.parent.rmdir()
    cli("user-repair", "repair", "hermes-project-install", project=False)
    cli("user-remove", "remove", "hermes-project-install", project=False)
    require((profile / "auth-sentinel").read_text() == "inert credentials sentinel\n", "User sentinel changed")
    home_guard = ProjectHomePreservation(home, profile, evidence, bundle.parent.parent, provenance, env, deadline, cleanup)
    protected = {str(home): home_guard.original, str(b): snapshot(b), str(a / ".git"): snapshot(a / ".git")}

    def unchanged():
        for path, before in protected.items():
            current = snapshot(Path(path))
            if path == str(home):
                current = home_guard.checked(current)
            require(current == before, "unselected authority changed: " + path)
        require((a / "foreign.txt").read_text() == "retained sibling/foreign file\n", "Project foreign file changed")

    def inspect(label):
        body = run(label, [uap, "run", "--target", "hermes", "--scope", "project", "--project-root", a,
                           "--inspect", "--confirm-client-start"])
        inventory = json.loads(body)
        require(inventory == {"inventory-only": {"url": "http://127.0.0.1:1/mcp"}}, "complete selected native MCP inventory differs")
        unchanged()

    package("1.0.0")
    cli("project-add", "add", str(source), "--no-save")
    # A confirmed explicit project add asks the real Hermes to trust exactly this
    # Git root. Read the decision back through Hermes itself, then validate the
    # exact config effect against the original HOME; never replace that baseline.
    trusted = json.loads(run("project-trust-readback", [bundle, "config", "get", "skills.trusted_project_dirs", "--json"], cwd=root))
    trusted = [] if trusted is None else [trusted] if isinstance(trusted, str) else trusted
    require(isinstance(trusted, list) and all(isinstance(item, str) for item in trusted),
            "native Hermes trust list is not a list of paths")
    expected = os.path.normcase(os.path.realpath(a))
    require([os.path.normcase(os.path.realpath(item)) for item in trusted] == [expected],
            "Hermes did not record exactly the installed project root as trusted: " + json.dumps(trusted))
    home_guard.authorize_trust(a)
    inspect("project-inspect")
    cli("project-repeat", "add", str(source), "--no-save")
    package("1.1.0")
    cli("project-update", "update", "hermes-project-install")
    inspect("project-update-inspect")
    installed = a / ".hermes" / "skills" / "hermes-project-install" / "SKILL.md"
    installed.unlink()
    installed.parent.rmdir()
    cli("project-repair", "repair", "hermes-project-install")
    inspect("project-repair-inspect")
    cli("project-remove", "remove", "hermes-project-install")
    unchanged()
    require(not installed.exists(), "owned Skill remains after removal")
    require((a / ".hermes" / "uap-session" / ".uap-owner").is_file(), "retained runtime ownership marker missing")
    require(payload_contract(bundle.parent.parent, runner / "hermes-agent", PIN) == provenance, "native provenance changed")
    report = {"completed": True, "native_provenance": provenance, "pin": PIN, "model_command_requested": False, "native_tools_command_requested": False,
              "source_head": os.environ["EXPECTED_HEAD"], "source_tree": os.environ["EXPECTED_TREE"],
              "bundle_manifest_sha256": hashlib.sha256(ordinary(bundle.parent.parent / "manifest.json", 64 << 10)).hexdigest(),
              "launcher_sha256": hashlib.sha256(ordinary(bundle, 2 << 20)).hexdigest(),
              "commands": records, "fixture": str(root), "uap_sha256": hashlib.sha256(ordinary(uap, 128 << 20)).hexdigest()}
    (evidence / "report.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({"completed": True, "evidence": str(evidence)}), flush=True)
    # Retain the complete disposable tree as reviewable native evidence.


if __name__ == "__main__":
    main()
