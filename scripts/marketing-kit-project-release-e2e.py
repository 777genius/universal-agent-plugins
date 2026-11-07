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


def inventory(cli, env, project, target):
    result = invoke([cli, "list", "--target", target, "--scope", "project",
                     "--project-root", str(project), "--format", "json"], env, project, 35)
    data = verified(result, "list").get("data")
    if not isinstance(data, dict) or not isinstance(data.get("installations"), list):
        raise AssertionError(f"list returned invalid installations: {data}")
    return data["installations"]


def identities(items, target):
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
        if not any(isinstance(b, dict) and b.get("client_id") == target and
                   b.get("scope") == "project" for b in bindings):
            raise AssertionError(f"{dist}: missing project binding for {target}")
        if not any(isinstance(b, dict) and audit["recorded_signed_release_matches"](
                   directory, b.get("package_revision")) for b in bindings
                   if b.get("client_id") == target):
            raise AssertionError(f"{dist}: exact signed release provenance does not match the native binding")
        if dist in result:
            raise AssertionError(f"duplicate distribution: {dist}")
        result[dist] = row.get("installation_id")
    return result


def lock_ids(project):
    lock = json.loads((project / "uap.lock").read_text(encoding="utf-8"))
    packages = lock.get("packages")
    if not isinstance(packages, list):
        raise AssertionError("locked project has no package entries")
    return {item.get("source_key", "") for item in packages}


def run_case(cli, target, root, name, portable=None):
    case = root / name
    home, data, project = case / "home", case / "data", case / "project"
    for directory in (home, data, project):
        directory.mkdir(parents=True)
    (home / f".{target}").mkdir(exist_ok=True)
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
    return env, project, user_foreign, project_foreign


def execute(cli, env, project, label, *args):
    result = invoke([cli, *args, "--format", "json"], env, project, 180)
    verified(result, label)
    return {"stage": label, "exit_code": result["exit_code"], "result": "passed"}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--target", required=True, choices=["codex", "claude", "gemini", "opencode"])
    parser.add_argument("--report", required=True)
    parser.add_argument("--cli", default="agentplugins")
    options = parser.parse_args()
    report_path = Path(options.report)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report = {"cli_version": "0.1.101", "target": options.target, "source": "real signed marketing-kit", "steps": [], "status": "failed"}
    try:
        with tempfile.TemporaryDirectory(prefix="uap-real-marketing-kit-") as location:
            root = Path(location)
            env, project_a, foreign_a, project_foreign_a = run_case(options.cli, options.target, root, "A")
            version = invoke([options.cli, "version"], env, project_a, 25)
            if version["exit_code"] != 0 or "0.1.101" not in version["stdout_tail"]:
                raise AssertionError(f"not published 0.1.101: {version}")
            def run(e, p, label, *args):
                report["steps"].append(execute(options.cli, e, p, label, *args))
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
            if set(identities(inventory(options.cli, env, project_a, options.target), options.target)) != EXPECTED:
                raise AssertionError("update lost a mandatory dependency")
            env_b, project_b, foreign_b, project_foreign_b = run_case(options.cli, options.target, root, "B", project_a)
            scoped_b = ("--scope", "project", "--project-root", str(project_b))
            run(env_b, project_b, "cold locked restore", "install", "--locked", "--yes", *scoped_b)
            if set(identities(inventory(options.cli, env_b, project_b, options.target), options.target)) != EXPECTED:
                raise AssertionError("cold restore lost required closure")
            if (project_b / "uap.lock").read_bytes() != (project_a / "uap.lock").read_bytes():
                raise AssertionError("cold restore modified pinned lock")
            run(env_b, project_b, "remove B", "remove", "marketing-kit", *scoped_b)
            if "777genius/marketing-kit" in identities(inventory(options.cli, env_b, project_b, options.target), options.target):
                raise AssertionError("Project B retains root after remove")
            run(env, project_a, "remove A", "remove", "marketing-kit", *scoped)
            if "777genius/marketing-kit" in identities(inventory(options.cli, env, project_a, options.target), options.target):
                raise AssertionError("Project A retains root after remove")
            for path, content in ((foreign_a, "unmanaged user data\n"),
                                  (foreign_b, "unmanaged user data\n"),
                                  (project_foreign_a, "unmanaged project data\n"),
                                  (project_foreign_b, "unmanaged project data\n")):
                if not path.is_file() or path.is_symlink() or path.read_text() != content:
                    raise AssertionError(f"foreign content modified: {path.name}")
            for e in (env, env_b):
                if (Path(e["AGENTPLUGINS_HOME"]) / "uap.toml").exists():
                    raise AssertionError("Project installation unexpectedly wrote User manifest")
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
