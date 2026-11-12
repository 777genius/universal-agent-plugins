"""Real Grok doctor + direct installed MCP calls; no model or account required."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import platform
import secrets
import shutil
import stat
import subprocess
import tempfile

from grok_project_protocol_client import (document, exercise, installed_server,
                                          read_owned, require, selected_installation, retained_data)

NAME = "uap-grok-no-model"
SERVER = "uap_probe"
SOURCE = Path(__file__).resolve().parent.parent


def digest(path):
    with Path(path).open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def write(path, value):
    path = Path(path)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    body = value.encode() if isinstance(value, str) else json.dumps(value, sort_keys=True).encode()
    with path.open("wb") as target:
        target.write(body)
    path.chmod(0o600)


def snapshot(root):
    result = {}
    for path in [Path(root), *sorted(Path(root).rglob("*"))]:
        info = path.lstat()
        require(not stat.S_ISLNK(info.st_mode), "snapshot contains an alias")
        require(stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode),
                "snapshot contains a special node")
        entry = {"kind": "directory" if stat.S_ISDIR(info.st_mode) else "file",
                 "mode": stat.S_IMODE(info.st_mode)}
        if stat.S_ISREG(info.st_mode):
            require(info.st_size <= 256 << 20, "snapshot file bound")
            entry["sha256"] = digest(path)
        result[str(path.relative_to(root))] = entry
    return result


def events(path, root):
    if not path.exists():
        return []
    return [document(line) for line in read_owned(path, root).splitlines() if line]


def observed_protocol(rows, nonce, methods):
    sessions = {}
    for row in rows:
        require(isinstance(row, dict) and row.get("nonce") == nonce
                and isinstance(row.get("session"), str) and bool(row["session"])
                and type(row.get("pid")) is int and row["pid"] > 0,
                "uncorrelated MCP probe event")
        sessions.setdefault((row["session"], row["pid"]), []).append(row.get("method"))
    def ordered(sequence):
        positions = [sequence.index(method) for method in methods if method in sequence]
        return len(positions) == len(methods) and positions == sorted(set(positions))
    require(any(ordered(sequence) for sequence in sessions.values()),
            "missing ordered same-session native MCP traffic")
    return [{"session": key[0], "pid": key[1], "methods": sequence}
            for key, sequence in sessions.items()]


def repair_refusal(body, stderr):
    """Accept only the existing non-mutating ownership refusal, not any failure."""
    value = document(body)
    require(isinstance(value, dict) and type(value.get("schema_version")) is int
            and value["schema_version"] == 1
            and value.get("command") == "repair" and value.get("result") == "failure",
            "repair refusal envelope differs")
    data = value.get("data", {})
    require(data.get("status") == "preflight_failed"
            and type(data.get("succeeded")) is int and data["succeeded"] == 0
            and type(data.get("failed")) is int and data["failed"] == 1,
            "repair refusal outcome differs")
    targets = data.get("targets", [])
    require(len(targets) == 1 and targets[0].get("target") == "grok"
            and targets[0].get("output", {}).get("result", {}).get("mutated") is False,
            "repair refusal changed a target")
    require(stderr.endswith("native identity ownership is indeterminate; refusing repair\n"),
            "repair failed for an unrelated reason")
    return value


class Journey:
    def __init__(self, root, installer, probe, evidence):
        self.root, self.installer, self.probe, self.evidence = root, installer, probe, evidence
        self.home, self.a, self.b = [root / name for name in ("home", "project-a", "project-b")]
        self.source, self.log = root / "package", root / "mcp-events.jsonl"
        self.nonce = secrets.token_hex(16)
        for path in (self.home, self.a, self.b, self.source, root / "bin", root / "tmp"):
            path.mkdir(mode=0o700)
        self.env = {"HOME": str(self.home), "PATH": str(root / "bin") + ":/usr/bin:/bin",
                    "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8", "NO_COLOR": "1",
                    "TMPDIR": str(root / "tmp"), "AGENTPLUGINS_HOME": str(root / "installer-state"),
                    "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null",
                    "GIT_TERMINAL_PROMPT": "0", "GROK_DISABLE_AUTOUPDATER": "1"}
        for suffix in ("CONFIG", "DATA", "CACHE", "STATE"):
            path = root / ("xdg-" + suffix.lower())
            path.mkdir(mode=0o700)
            self.env["XDG_" + suffix + "_HOME"] = str(path)
        self.grok = root / "bin/grok"
        self.protected = {}

    def run(self, label, argv, cwd=None, expected=(0,), timeout=60):
        from native_client_research import run_probe
        self.check()
        record, stdout, stderr = run_probe([str(arg) for arg in argv], cwd or self.a,
                                          self.env, self.root, label, timeout=timeout,
                                          limit=1 << 20, memory_only=True)
        self.evidence["commands"].append(dict(record, stdout=stdout.decode("utf-8", "replace"),
            stderr=stderr.decode("utf-8", "replace"), stdout_base64=base64.b64encode(stdout).decode(),
            stderr_base64=base64.b64encode(stderr).decode()))
        self.check()
        require(record["error"] is None and record["exit"] in expected,
                "command failed: " + label)
        return stdout

    def check(self):
        for path, expected in self.protected.items():
            require(path.resolve() == path and path.is_file() and digest(path) == expected,
                    "protected fixture changed: " + str(path))

    def author(self, revision):
        write(self.source / "plugin.json", {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": NAME, "version": "1.0." + revision, "description": "Credential-free MCP E2E"})
        target = self.source / "bin/probe"
        target.parent.mkdir(mode=0o700, exist_ok=True)
        if not target.exists():
            shutil.copyfile(self.probe, target)
            target.chmod(0o700)
        write(self.source / "skills" / NAME / "SKILL.md",
              "---\nname: " + NAME + "\ndescription: Local protocol fixture\n---\nNo model required.\n")
        write(self.source / "mcp.json", {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {SERVER: {"type": "stdio", "command": "./bin/probe", "args": [],
                "env": {"UAP_TEST_ROOT": str(self.root), "UAP_TEST_EVENTS": str(self.log),
                        "UAP_TEST_NONCE": self.nonce, "UAP_TEST_REVISION": revision}}}})

    def cli(self, label, verb, selector):
        argv = [self.installer, verb, selector, "--scope", "project", "--project-root", self.a,
                "--target", "grok", "--format", "json"]
        if verb == "add":
            argv.append("--no-save")
        value = document(self.run(label, argv, timeout=120))
        require(value.get("result") == "success", "public UAP JSON is not success")
        return value

    def inspect(self, label, active, present, trusted):
        value = document(self.run(label, [self.grok, "inspect", "--json"]))
        require(value.get("grokVersion") == "1.0.70" and value.get("cwd") == str(self.a)
                and value.get("projectRoot", "").rstrip("/") == str(self.a)
                and value.get("projectTrusted") is trusted, "native project/version/trust differs")
        selected = [p for p in value.get("plugins", []) if p.get("name", "").casefold() == NAME]
        require(len(selected) == int(present), "native plugin presence differs")
        if present:
            item = selected[0]
            require(item.get("name") == NAME and item.get("path") == str(active)
                    and item.get("scope") == "project" and item.get("enabled") is trusted,
                    "native selected plugin authority differs")
        if not trusted:
            require(value.get("mcpServers") == [], "untrusted project activated MCP")
        return value

    def enable_fixture(self):
        # Explicit test fixture only: this is NOT an installer side effect.
        config = self.home / ".grok/config.toml"
        write(config, '[marketplace]\ndefault_skills_installs_purged = true\n'
              '[features]\nturn_summary = false\n[plugins]\nenabled = ["' + NAME + '"]\n')
        trust = self.home / ".grok/trusted_folders.toml"
        write(trust, "[folders." + json.dumps(str(self.a)) + "]\ntrusted = true\n")
        self.protected.update({config: digest(config), trust: digest(trust)})

    def protocol(self, stage, revision, marker, create=False):
        installation, binding, active, data = selected_installation(self.a, NAME)
        server = installed_server(active, data, self.root, SERVER, self.nonce)
        before = events(self.log, self.root)
        self.run(stage + "-native-doctor", [self.grok, "mcp", "doctor", "--json"], timeout=45)
        native_rows = events(self.log, self.root)[len(before):]
        native = observed_protocol(native_rows, self.nonce, ("initialize", "tools/list"))
        before = events(self.log, self.root)
        self.check()
        direct = exercise(server, self.env, self.nonce, revision, marker, create)
        self.check()
        direct_rows = events(self.log, self.root)[len(before):]
        correlated = observed_protocol(direct_rows, self.nonce, ("initialize", "tools/list", "tools/call"))
        require(len(correlated) == 1 and correlated[0]["session"] == direct["facts"]["session"]
                and correlated[0]["pid"] == direct["facts"]["pid"]
                and correlated[0]["methods"].count("tools/call") == 1,
                "tool result identity differs or bad nonce caused a tool event")
        require(all(row["session"] != direct["facts"]["session"] for row in native),
                "native and direct callers were conflated")
        self.evidence["stages"].append({"stage": stage, "binding": binding,
            "installed_native_config": document(read_owned(active / ".mcp.json", self.root)),
            "native_doctor_sessions": native, "native_original_events": native_rows,
            "direct_client": direct, "direct_original_events": direct_rows})
        return installation, binding, active, data

    def repair_owned(self, active, data, installation):
        # Partial edits intentionally invalidate package ownership. Prove the
        # no-effect refusal before testing recovery of a wholly absent package.
        self.check()
        identity = active.lstat()
        require(active.resolve() == active and stat.S_ISDIR(identity.st_mode)
                and identity.st_uid == os.getuid(), "repair fixture root differs")
        intact, data_before, source_before = snapshot(active), snapshot(data), snapshot(self.source)
        state_path = self.a / ".uap/state-v2.json"
        state_before = read_owned(state_path, self.root)
        read_owned(active / ".mcp.json", self.root)
        (active / ".mcp.json").unlink()
        damaged = dict(intact)
        del damaged[".mcp.json"]
        require(snapshot(active) == damaged, "partial repair fault changed unrelated files")
        self.run("damaged-inspect", [self.grok, "inspect", "--json"])
        project_before, other_before = snapshot(self.a), snapshot(self.b)
        before_events = events(self.log, self.root)
        argv = [self.installer, "repair", installation["installation_id"], "--scope", "project",
                "--project-root", self.a, "--target", "grok", "--format", "json"]
        refusal = self.run("partial-repair-refusal", argv, expected=(1,), timeout=120)
        repair_refusal(refusal, self.evidence["commands"][-1]["stderr"])
        require(read_owned(state_path, self.root) == state_before and snapshot(active) == damaged
                and snapshot(data) == data_before and snapshot(self.source) == source_before
                and snapshot(self.a) == project_before and snapshot(self.b) == other_before
                and events(self.log, self.root) == before_events, "refused repair had side effects")
        # Only the test moves its exact held artifact into a fresh private
        # quarantine. UAP does not delete or adopt the partially edited tree.
        current = active.lstat()
        require((current.st_dev, current.st_ino) == (identity.st_dev, identity.st_ino)
                and active.resolve() == active, "owned repair artifact was replaced")
        quarantine = self.root / "repair-quarantine"
        quarantine.mkdir(mode=0o700)
        saved = quarantine / "package"
        active.rename(saved)
        require(not os.path.lexists(active) and snapshot(saved) == damaged, "absence fault differs")
        self.inspect("missing-package-inspect", active, False, True)
        self.cli("repair-missing-package", "repair", installation["installation_id"])
        require(snapshot(active) == intact and snapshot(saved) == damaged
                and snapshot(data) == data_before and snapshot(self.source) == source_before,
                "repair did not restore exact package or preserved data")
        self.evidence["repair"] = {"partial_change_refused_without_mutation": True,
            "missing_owned_package_restored": True, "intact_inventory": intact,
            "quarantined_inventory": damaged,
            "original_state_sha256": hashlib.sha256(state_before).hexdigest()}
        self.check()

    def execute(self):
        self.run("git-a", ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"])
        self.run("git-b", ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"], self.b)
        write(self.home / ".grok/config.toml", '[marketplace]\ndefault_skills_installs_purged = true\n'
              '[features]\nturn_summary = false\n')
        for path in (self.home / ".grok/foreign.txt", self.a / "foreign.txt", self.b / "foreign.txt"):
            write(path, "never modify " + self.nonce)
            self.protected[path] = digest(path)
        untouched_b = snapshot(self.b)
        self.author("0")
        self.cli("install", "add", self.source)
        self.evidence["prepared_owned_state"] = document(read_owned(self.a / ".uap/state-v2.json", self.root))
        first, first_binding, active, data = selected_installation(self.a, NAME)
        self.inspect("untrusted-inspect", active, True, False)
        before = events(self.log, self.root)
        self.run("untrusted-doctor", [self.grok, "mcp", "doctor", "--json"], expected=(0, 1))
        require(events(self.log, self.root) == before, "untrusted native doctor started the fixture")
        initial = snapshot(active)
        initial_receipt = read_owned(self.a / ".uap/state-v2.json", self.root)
        self.cli("repeat-install", "add", self.source)
        require(snapshot(active) == initial, "repeat installation changed package bytes")
        require(read_owned(self.a / ".uap/state-v2.json", self.root) == initial_receipt,
                "repeat installation changed ownership state")
        self.enable_fixture()
        self.inspect("trusted-inspect", active, True, True)
        marker = "persisted-" + self.nonce
        self.protocol("installed", "0", marker, create=True)
        self.author("1")
        self.cli("update", "update", self.source)
        updated, binding, replacement, _ = self.protocol("updated", "1", marker)
        require(replacement == active and binding["client_binding_id"] == first_binding["client_binding_id"]
                and updated["source"]["tree_digest"] != first["source"]["tree_digest"],
                "update identity or source pin differs")
        self.repair_owned(active, data, updated)
        repaired, repaired_binding, repaired_active, repaired_data = self.protocol("repaired", "1", marker)
        require(repaired["installation_id"] == updated["installation_id"]
                and repaired["source"] == updated["source"]
                and repaired["data_receipts"] == updated["data_receipts"]
                and repaired_binding["client_binding_id"] == binding["client_binding_id"]
                and repaired_active == active and repaired_data == data,
                "repair changed the held package, binding or data authority")
        self.evidence["pre_removal_data_receipts"] = repaired["data_receipts"]
        self.cli("remove", "remove", NAME)
        require(not os.path.lexists(active), "owned package survives removal")
        self.inspect("removed-inspect", active, False, True)
        before = events(self.log, self.root)
        self.run("removed-doctor", [self.grok, "mcp", "doctor", "--json"], expected=(0, 1))
        require(events(self.log, self.root) == before, "removed native plugin still starts MCP")
        self.evidence["post_removal_installation"] = retained_data(self.a, repaired)
        require(snapshot(self.b) == untouched_b, "Project B changed")
        require(read_owned(data / "native-marker.txt", self.root).decode() == marker,
                "persistent user data lost on removal")
        self.check()
        self.evidence.update({"project_b_unchanged": True, "protected_user_files_unchanged": True,
                              "untrusted_refusal_verified": True, "removed_absent": True,
                              "persistent_data_retained": True})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--installer", required=True, type=Path)
    parser.add_argument("--probe", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    require(os.environ.get("GITHUB_ACTIONS") == "true"
            and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted"
            and platform.system() == "Linux" and platform.machine() == "x86_64",
            "actual disposable GitHub-hosted Linux x64 required")
    for path in (args.installer, args.probe):
        require(path.is_absolute() and path.resolve() == path and path.is_file()
                and stat.S_ISREG(path.stat().st_mode) and path.stat().st_uid == os.getuid(),
                "expected owned compiled binary")
    expected = os.environ.get("EXPECTED_COMMIT", "")
    git_env = {"PATH": "/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null"}
    def git(*argv):
        return subprocess.check_output(["/usr/bin/git", *argv], cwd=SOURCE, env=git_env,
                                       timeout=10).decode().strip()
    require(len(expected) == 40 and git("rev-parse", "HEAD") == expected
            and not git("status", "--porcelain", "--untracked-files=normal"), "clean exact source required")
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    evidence = {"schema_version": 1, "source_commit": expected,
                "source_tree": git("rev-parse", "HEAD^{tree}"), "passed": False,
                "commands": [], "stages": [], "model_invoked": False,
                "vendor_authentication_used": False, "grok_tools_call_qualified": False,
                "transport": "stdio", "installer_sha256": digest(args.installer),
                "probe_sha256": digest(args.probe)}
    try:
        from project_native_research import Budget, acquire, materialize, verify_files
        from project_native_collectors.grok import ACQUISITION
        with tempfile.TemporaryDirectory(prefix="grok-no-model-", dir=os.environ["RUNNER_TEMP"]) as temporary:
            root = Path(temporary).resolve()
            journey = Journey(root, args.installer, args.probe, evidence)
            budget = Budget(900)
            receipt, contents = acquire(ACQUISITION, budget)
            binary, identities = materialize(root / "bin", ACQUISITION, receipt, contents)
            require(binary == journey.grok, "acquired Grok path differs")
            evidence["acquisition"] = receipt
            verify_files(identities, budget)
            version = journey.run("grok-version", [binary, "--version"], root)
            require(version == b"grok 1.0.70 (f50904eaee6c)\n", "pinned native Grok version differs")
            try:
                journey.execute()
                verify_files(identities, budget)
            finally:
                # Capture before TemporaryDirectory cleans up, on failure too.
                # Invalid/aliased event files must fail rather than be adopted.
                evidence["all_original_events"] = events(journey.log, root)
                if journey.log.exists():
                    original = read_owned(journey.log, root)
                    evidence["events_sha256"] = hashlib.sha256(original).hexdigest()
                    evidence["events_base64"] = base64.b64encode(original).decode()
        require(not git("status", "--porcelain", "--untracked-files=normal"), "source changed")
        evidence["passed"] = True
        evidence["native_doctor_initialize_and_list_qualified"] = True
        evidence["direct_installed_config_tools_call_qualified"] = True
    except Exception as error:
        evidence["error"] = type(error).__name__ + ": " + str(error)
        raise
    finally:
        write(args.output / "evidence.json", evidence)
        print(json.dumps({"passed": evidence["passed"], "evidence": str(args.output / "evidence.json")}, sort_keys=True))


if __name__ == "__main__":
    main()
