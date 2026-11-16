#!/usr/bin/env python3
"""Pinned official Hermes file installation and explicit inspection; no tools/model calls."""
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from hermes_install_acceptance import ordinary, payload_contract, snapshot, supervised

PIN = "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19"


def require(value, message):
    if not value:
        raise RuntimeError(message)


def main():
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
    (profile / "config.yaml").write_text("theme: retained-user\nmcp_servers: {}\n")
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
    protected = {str(home): snapshot(home), str(b): snapshot(b), str(a / ".git"): snapshot(a / ".git")}

    def unchanged():
        for path, before in protected.items():
            require(snapshot(Path(path)) == before, "unselected authority changed: " + path)
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
    # Git root. Read the decision back through Hermes itself, then re-baseline the
    # selected profile: only that one list may have changed, nothing else.
    trusted = json.loads(run("project-trust-readback", [bundle, "config", "get", "skills.trusted_project_dirs", "--json"], cwd=root))
    trusted = [] if trusted is None else [trusted] if isinstance(trusted, str) else trusted
    require(isinstance(trusted, list) and all(isinstance(item, str) for item in trusted),
            "native Hermes trust list is not a list of paths")
    expected = os.path.normcase(os.path.realpath(a))
    require([os.path.normcase(os.path.realpath(item)) for item in trusted] == [expected],
            "Hermes did not record exactly the installed project root as trusted: " + json.dumps(trusted))
    require("theme: retained-user" in (profile / "config.yaml").read_text(), "retained profile setting changed by trust")
    protected[str(home)] = snapshot(home)
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
