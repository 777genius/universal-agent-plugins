#!/usr/bin/env python3
"""Actual public npm 0.1.101 + signed Marketing Kit Project lifecycle, no mocks."""
import argparse
import json
import os
from pathlib import Path
import runpy
import shutil
import subprocess
import tempfile
import traceback

ROOT = Path(__file__).resolve().parents[1]
audit = runpy.run_path(str(ROOT / "scripts/live-install-audit.py"))
isolated_env = audit["isolated_client_env"]
invoke = audit["call"]
EXPECTED = {
    "777genius/marketing-kit",
    "777genius/notion",
    "777genius/hubspot-crm",
    "777genius/playwright-bridge",
}


def verified(result, stage):
    if result["exit_code"] != 0 or result["timeout"] or not isinstance(result.get("_json"), dict):
        raise AssertionError(f"{stage}: CLI did not succeed with JSON: {result}")
    return result["_json"]


def require_exact_published_version(result):
    """Reject a dev/newer/wrong binary even if its output contains the pinned version."""
    stdout = result.get("stdout_tail")
    if (result.get("exit_code") != 0 or result.get("timeout")
            or not isinstance(stdout, str)
            or stdout.strip() != "agentplugins 0.1.101"):
        raise AssertionError("not exact published agentplugins 0.1.101")


def inventory(cli, env, project, target, scope="project"):
    if scope not in ("user", "project"):
        raise ValueError(f"unsupported scope: {scope}")
    scope_args = ["--scope", scope]
    if scope == "project":
        scope_args.extend(["--project-root", str(project)])
    result = invoke([cli, "list", "--target", target, *scope_args,
                     "--format", "json"], env, project, 35)
    data = verified(result, "list").get("data")
    if not isinstance(data, dict) or not isinstance(data.get("installations"), list):
        raise AssertionError(f"list returned invalid installations: {data}")
    return data["installations"]


def identities(items, target, scope="project"):
    result = {}
    for row in items:
        if not isinstance(row, dict):
            raise AssertionError("invalid installation record")
        directory = row.get("directory") or {}
        dist = directory.get("recorded_distribution")
        if not dist:
            # Recorded signed-v1 identity uses recorded_distribution, not distribution_id.
            raise AssertionError(f"installation lacks signed Directory identity: {row.get('name')}; fields={sorted(directory)}")
        bindings = row.get("clients") or []
        if isinstance(bindings, dict):
            bindings = list(bindings.values())
        scope_bindings = [b for b in bindings if isinstance(b, dict)
                          and b.get("client_id") == target and b.get("scope") == scope]
        if not scope_bindings:
            raise AssertionError(f"{dist}: missing {scope} binding for {target}")
        if not all(audit["recorded_signed_release_matches"](
                directory, b.get("package_revision")) for b in scope_bindings):
            raise AssertionError(f"{dist}: {scope} binding lacks exact signed release provenance")
        if dist in result:
            raise AssertionError(f"duplicate distribution: {dist}")
        installation_id = row.get("installation_id")
        if not isinstance(installation_id, str) or not installation_id.strip():
            raise AssertionError(f"{dist}: missing installation ID")
        if installation_id in result.values():
            raise AssertionError(f"duplicate installation ID: {installation_id}")
        result[dist] = installation_id
    return result


def lock_ids(project):
    lock = json.loads((project / "uap.lock").read_text(encoding="utf-8"))
    packages = lock.get("packages")
    if not isinstance(packages, list):
        raise AssertionError("locked project has no package entries")
    keys = []
    for item in packages:
        if not isinstance(item, dict) or not isinstance(item.get("source_key"), str) or not item["source_key"]:
            raise AssertionError("invalid signed lock package source")
        keys.append(item["source_key"])
    if len(keys) != len(set(keys)):
        raise AssertionError("duplicate signed lock package source")
    return set(keys)


def run_case(cli, target, root, name, portable=None):
    case = root / name
    home, data, project = case / "home", case / "data", case / "project"
    for directory in (home, data, project):
        directory.mkdir(parents=True)
    # Reuse the existing published audit's native-profile root inventory.
    native_foreign = []
    for relative in audit["NATIVE_ROOTS"][target]:
        native_root = home / relative
        native_root.mkdir(parents=True, exist_ok=True)
        marker = native_root / "unmanaged-native-do-not-change.txt"
        marker.write_text("unmanaged native settings\n", encoding="utf-8")
        native_foreign.append((marker, "unmanaged native settings\n"))
    # The fixture must never read or modify the operator's home or native config.
    user_foreign = home / "foreign-user-do-not-change.txt"
    user_foreign.write_text("unmanaged user data\n", encoding="utf-8")
    project_foreign = project / "foreign-project-do-not-change.txt"
    project_foreign.write_text("unmanaged project data\n", encoding="utf-8")
    env = isolated_env(home, case, data)
    subprocess.run(["git", "init", "-q", str(project)], check=True, env=env, capture_output=True)
    if portable is not None:
        for filename in ("uap.toml", "uap.lock"):
            shutil.copyfile(portable / filename, project / filename)
    return env, project, user_foreign, project_foreign, native_foreign


def check_foreign(entries, case_root):
    """Reject changed foreign state and symlinked ancestors before reading."""
    for path, content in entries:
        # Reuse the live audit's fail-closed containment oracle (BUG-044).
        if (not audit["path_inside_case"](path, case_root)
                or not path.is_file()
                or path.read_bytes() != content.encode("utf-8")):
            raise AssertionError(f"foreign content modified: {path.name}")


def check_project_a_unchanged(observed, expected, current_lock, expected_lock, stage):
    """Fail if Project B's lifecycle rekeys or rewrites Project A."""
    if observed != expected:
        raise AssertionError(f"{stage}: Project A installation identities changed")
    if current_lock != expected_lock:
        raise AssertionError(f"{stage}: Project A signed lock changed")


def bind_pinned_launcher_cache(env, cache, sandbox):
    """Share only the SHA-checked released CLI binary, not native client state."""
    if (not cache.is_dir() or cache.is_symlink()
            or not audit["path_inside_case"](cache, sandbox)):
        raise AssertionError("released CLI cache must be a real in-sandbox directory")
    env["AGENTPLUGINS_CACHE_DIR"] = str(cache)


def execute(cli, env, project, label, *args):
    result = invoke([cli, *args, "--format", "json"], env, project, 180)
    verified(result, label)
    return {"stage": label, "exit_code": result["exit_code"], "result": "passed"}


def codex_user_native_remove_attestation(env, project):
    """Only attest after real native HubSpot removal from this disposable profile."""
    case_root = project.parent
    home = case_root / "home"
    if (env.get("HOME") != str(home)
            or env.get("CODEX_HOME") != str(home / ".codex")
            or not audit["path_inside_case"](home, case_root)
            or not audit["path_inside_case"](project, case_root)):
        raise AssertionError("Codex native removal requires the exact disposable profile")
    native = audit["uninstall_native_codex_plugin"](
        "hubspot-crm", home, case_root, env, project)
    if native.get("completed") is not True:
        raise AssertionError(
            "Codex native HubSpot uninstall unverified: " +
            str(native.get("reason") or "native removal and absence not confirmed"))
    return ("--external-uninstalled",)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--target", required=True, choices=["codex", "cursor", "claude", "gemini", "opencode", "copilot"])
    parser.add_argument("--report", required=True)
    parser.add_argument("--cli", default="agentplugins")
    options = parser.parse_args()
    report_path = Path(options.report)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report = {"cli_version": "0.1.101", "target": options.target, "source": "real signed marketing-kit", "steps": [], "status": "failed"}
    try:
        with tempfile.TemporaryDirectory(prefix="uap-real-marketing-kit-") as location:
            root = Path(location)
            # The npm 0.1.101 launcher validates the SHA-256 on every cache hit.
            # Keep one private executable cache for A/B: cold restore is still
            # independent in lock, receipts, clients and native HOME/profile.
            launcher_cache = root / "released-cli-cache"
            launcher_cache.mkdir(mode=0o700)
            env, project_a, foreign_a, project_foreign_a, native_foreign_a = run_case(options.cli, options.target, root, "A")
            bind_pinned_launcher_cache(env, launcher_cache, root)
            foreign_a_files = [(foreign_a, "unmanaged user data\n"),
                               (project_foreign_a, "unmanaged project data\n"),
                               *native_foreign_a]
            foreign_files = list(foreign_a_files)
            version = invoke([options.cli, "version"], env, project_a, 25)
            require_exact_published_version(version)
            def run(e, p, label, *args):
                try:
                    report["steps"].append(execute(options.cli, e, p, label, *args))
                finally:
                    # Also protect foreign state when a lifecycle command refuses/fails.
                    check_foreign(foreign_files, root)
            scoped = ("--scope", "project", "--project-root", str(project_a))
            run(env, project_a, "add", "add", "marketing-kit", "--target", options.target, *scoped)
            first = identities(inventory(options.cli, env, project_a, options.target), options.target)
            if set(first) != EXPECTED:
                raise AssertionError(f"mandatory Marketing Kit closure incomplete: {sorted(first)}")
            for name in ("uap.toml", "uap.lock"):
                if not (project_a / name).is_file():
                    raise AssertionError(f"missing Project agreement: {name}")
            expected_pins = {"directory:" + item for item in EXPECTED}
            if lock_ids(project_a) != expected_pins:
                raise AssertionError(f"wrong locked signed source set: {sorted(lock_ids(project_a))}")
            portable_lock = (project_a / "uap.lock").read_bytes()
            run(env, project_a, "repeat add", "add", "marketing-kit", "--target", options.target, *scoped)
            if identities(inventory(options.cli, env, project_a, options.target), options.target) != first:
                raise AssertionError("repeat add changed installation identities")
            if (project_a / "uap.lock").read_bytes() != portable_lock:
                raise AssertionError("repeat add changed the immutable lock")
            # Real update command, not a mocked planner; unchanged upstream pins
            # are still an observable no-update lifecycle.
            run(env, project_a, "update", "update", "marketing-kit", *scoped)
            identities_after_update = identities(inventory(options.cli, env, project_a, options.target), options.target)
            if set(identities_after_update) != EXPECTED:
                raise AssertionError("update lost a mandatory dependency")
            # Real repair on intact signed pins: this is not drift-recovery proof.
            # A repair may refresh owned receipts, but not the locked closure.
            pinned_after_update = (project_a / "uap.lock").read_bytes()
            run(env, project_a, "repair intact", "repair", "marketing-kit",
                "--target", options.target, *scoped)
            if identities(inventory(options.cli, env, project_a, options.target), options.target) != identities_after_update:
                raise AssertionError("repair changed installation identities")
            if (project_a / "uap.lock").read_bytes() != pinned_after_update:
                raise AssertionError("repair changed the signed lock")
            env_b, project_b, foreign_b, project_foreign_b, native_foreign_b = run_case(options.cli, options.target, root, "B", project_a)
            bind_pinned_launcher_cache(env_b, launcher_cache, root)
            foreign_b_files = [(foreign_b, "unmanaged user data\n"),
                               (project_foreign_b, "unmanaged project data\n"),
                               *native_foreign_b]
            foreign_files.extend(foreign_b_files)
            scoped_b = ("--scope", "project", "--project-root", str(project_b))
            run(env_b, project_b, "cold locked restore", "install", "--locked", "--yes", *scoped_b)
            restored = identities(inventory(options.cli, env_b, project_b, options.target), options.target)
            if set(restored) != EXPECTED:
                raise AssertionError("cold restore lost required closure")
            if set(restored.values()) & set(identities_after_update.values()):
                raise AssertionError("cold restore reused Project A installation IDs")
            if (project_b / "uap.lock").read_bytes() != (project_a / "uap.lock").read_bytes():
                raise AssertionError("cold restore modified pinned lock")
            check_project_a_unchanged(
                identities(inventory(options.cli, env, project_a, options.target), options.target),
                identities_after_update, (project_a / "uap.lock").read_bytes(),
                pinned_after_update, "cold restore B")
            run(env_b, project_b, "remove B", "remove", "marketing-kit", *scoped_b)
            if identities(inventory(options.cli, env_b, project_b, options.target), options.target):
                raise AssertionError("Project B retains root or mandatory children after remove")
            check_project_a_unchanged(
                identities(inventory(options.cli, env, project_a, options.target), options.target),
                identities_after_update, (project_a / "uap.lock").read_bytes(),
                pinned_after_update, "remove B")
            run(env, project_a, "remove A", "remove", "marketing-kit", *scoped)
            if identities(inventory(options.cli, env, project_a, options.target), options.target):
                raise AssertionError("Project A retains root or mandatory children after remove")
            for e in (env, env_b):
                if (Path(e["AGENTPLUGINS_HOME"]) / "uap.toml").exists():
                    raise AssertionError("Project installation unexpectedly wrote User manifest")
            # Check the same signed closure through a real, separate User lifecycle.
            # Keep the Project artifacts unchanged while the disposable User profile
            # installs, repeats, updates, repairs and removes its own copies.
            project_files = {
                name: (project_a / name).read_bytes() if (project_a / name).exists() else None
                for name in ("uap.toml", "uap.lock")
            }
            user_scoped = ("--scope", "user")
            def user_inventory():
                return identities(inventory(options.cli, env, project_a, options.target, "user"),
                                  options.target, "user")
            run(env, project_a, "User add", "add", "marketing-kit",
                "--target", options.target, *user_scoped)
            user_first = user_inventory()
            if set(user_first) != EXPECTED:
                raise AssertionError("User add lost mandatory signed Marketing Kit closure")
            if identities(inventory(options.cli, env, project_a, options.target), options.target):
                raise AssertionError("User add recreated removed Project installations")
            run(env, project_a, "User repeat add", "add", "marketing-kit",
                "--target", options.target, *user_scoped)
            if user_inventory() != user_first:
                raise AssertionError("User repeat add changed installation identities")
            run(env, project_a, "User update", "update", "marketing-kit", *user_scoped)
            user_after_update = user_inventory()
            if set(user_after_update) != EXPECTED:
                raise AssertionError("User update lost mandatory signed closure")
            run(env, project_a, "User repair intact", "repair", "marketing-kit",
                "--target", options.target, *user_scoped)
            if user_inventory() != user_after_update:
                raise AssertionError("User repair changed installation identities")
            # Codex user plugin dependencies require genuine external native removal.
            # Reuse the audited, pinned Codex CLI path and exact native absence oracle.
            user_remove_attestation = ()
            if options.target == "codex":
                try:
                    user_remove_attestation = codex_user_native_remove_attestation(env, project_a)
                finally:
                    check_foreign(foreign_files, root)
                report["steps"].append({
                    "stage": "User native HubSpot uninstall",
                    "exit_code": 0, "result": "passed",
                })
            run(env, project_a, "User remove", "remove", "marketing-kit",
                *user_scoped, *user_remove_attestation)
            if user_inventory():
                raise AssertionError("User removal retained Marketing Kit or mandatory children")
            for name, expected_bytes in project_files.items():
                path = project_a / name
                current_bytes = path.read_bytes() if path.exists() else None
                if current_bytes != expected_bytes:
                    raise AssertionError(f"User lifecycle modified Project artifact {name}")
            # The last inventory/read assertions are outside run()'s finally.
            check_foreign(foreign_files, root)
            report["status"] = "passed"
    except Exception as error:
        report["failure"] = str(error)[:2500]
        traceback.print_exc()
    finally:
        report_path.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        print(json.dumps(report, indent=2, ensure_ascii=False))
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
