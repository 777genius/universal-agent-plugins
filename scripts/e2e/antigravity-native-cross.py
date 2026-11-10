#!/usr/bin/env python3
"""Real agy 1.5.3 + UAP native lifecycle on Windows and macOS.

No browser/model authentication, no foreign profile mutation, no IDE activation claim.
All fixtures, binaries and state live below RUNNER_TEMP.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


NAME = "uap-agy-native-smoke"


def run(binary, *args, env, cwd):
    result = subprocess.run(
        [str(binary), *map(str, args)],
        cwd=str(cwd),
        env=env,
        text=True,
        encoding="utf-8",
        errors="replace",
        capture_output=True,
        timeout=240,
        check=False,
    )
    if result.returncode:
        raise RuntimeError(
            f"{Path(binary).name} {args[0] if args else ''}: exit {result.returncode}\n"
            f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return result.stdout


def profile(base, executable_dir):
    home = base / "home"
    env = dict(os.environ)
    env.update(
        HOME=str(home),
        USERPROFILE=str(home),
        APPDATA=str(home / "AppData" / "Roaming"),
        LOCALAPPDATA=str(home / "AppData" / "Local"),
        XDG_CONFIG_HOME=str(base / "config"),
        XDG_CACHE_HOME=str(base / "cache"),
        PATH=str(executable_dir) + os.pathsep + os.environ.get("PATH", ""),
    )
    for key in ("HOME", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME"):
        Path(env[key]).mkdir(parents=True, exist_ok=True)
    return env


def checked_list(cli, env, project):
    rows = json.loads(run(cli, "plugin", "list", "--json", env=env, cwd=project))
    if not isinstance(rows, list):
        raise AssertionError("agy plugin list did not return its documented array")
    return [item.get("name") for item in rows if isinstance(item, dict)]


def main():
    agy = Path(os.environ["AGY_BIN"]).resolve(strict=True)
    uap = Path(os.environ["UAP_BIN"]).resolve(strict=True)
    root_parent = Path(os.environ["RUNNER_TEMP"]).resolve(strict=True)
    with tempfile.TemporaryDirectory(prefix="uap-antigravity-native-", dir=root_parent) as temp:
        root = Path(temp)
        executable_dir = root / "bin"
        executable_dir.mkdir()
        shim = executable_dir / ("agy.exe" if os.name == "nt" else "agy")
        shutil.copy2(agy, shim)
        if os.name != "nt":
            shim.chmod(0o700)
        user_env = profile(root / "uap-user", executable_dir)
        vendor_env = profile(root / "agy-vendor-only", executable_dir)
        project = root / "project-a"
        project_b = root / "project-b"
        source = root / "source"
        for directory in (project, project_b, source / "skills" / "native-smoke"):
            directory.mkdir(parents=True)
        portable = {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": NAME,
            "version": "1.0.0",
            "description": "Isolated cross-platform Antigravity native lifecycle smoke",
        }
        manifest = source / "plugin.json"
        manifest.write_text(json.dumps(portable, indent=2) + "\n", encoding="utf-8")
        source_hash = hashlib.sha256(manifest.read_bytes()).hexdigest()
        (source / "skills" / "native-smoke" / "SKILL.md").write_text(
            "---\nname: native-smoke\n"
            "description: Disposable native plugin validation.\n"
            "---\nDo not run commands or connect to external services.\n",
            encoding="utf-8",
        )
        foreign = Path(user_env["HOME"]) / ".gemini" / "config" / "plugins" / "existing-foreign-plugin"
        foreign.mkdir(parents=True)
        sentinel = foreign / "sentinel"
        sentinel.write_text("foreign must survive\n", encoding="utf-8")
        print("Pinned agy:", run(agy, "--version", env=user_env, cwd=project).strip(), flush=True)

        output = run(
            uap, "add", source, "--target", "antigravity", "--scope", "project",
            "--project-root", project, "--no-save", "--yes", "--format", "json",
            env=user_env, cwd=project,
        )
        if not json.loads(output):
            raise AssertionError("UAP returned an empty add result")
        plugin_root = project / ".agents" / "plugins"
        if not plugin_root.is_dir():
            raise AssertionError("UAP did not create its project-native plugins root")
        owned = [p for p in plugin_root.iterdir()
                 if p.is_dir() and not p.is_symlink()
                 and not p.name.startswith(".agentplugins-staging-")]
        if len(owned) != 1:
            raise AssertionError(f"Expected one owned native plugin, found {len(owned)}")
        plugin = owned[0]
        vendor_manifest = plugin / "plugin.json"
        if (plugin / ".antigravity-plugin").exists() or not vendor_manifest.is_file():
            raise AssertionError("UAP emitted missing or duplicate native plugin manifest")
        native = json.loads(vendor_manifest.read_text(encoding="utf-8"))
        if native.get("name") != NAME or native.get("$schema") != "https://antigravity.google/schemas/v1/plugin.json":
            raise AssertionError("Native schema or identity was not projected")
        if not (plugin / "skills" / "native-smoke" / "SKILL.md").is_file():
            raise AssertionError("Selected native Skill was not staged")
        projected_hash = hashlib.sha256(vendor_manifest.read_bytes()).hexdigest()

        run(agy, "plugin", "validate", "--strict", plugin, env=user_env, cwd=project)
        if NAME in checked_list(agy, vendor_env, project):
            raise AssertionError("CLI profile unexpectedly owns a project-only plugin")
        run(agy, "plugin", "install", plugin, env=vendor_env, cwd=project)
        if NAME not in checked_list(agy, vendor_env, project):
            raise AssertionError("Pinned agy did not list its explicitly imported plugin")
        run(agy, "plugin", "uninstall", NAME, env=vendor_env, cwd=project)
        if NAME in checked_list(agy, vendor_env, project):
            raise AssertionError("Pinned agy retained the uninstalled test plugin")

        if hashlib.sha256(vendor_manifest.read_bytes()).hexdigest() != projected_hash:
            raise AssertionError("Native CLI changed UAP-owned staged source")
        if hashlib.sha256(manifest.read_bytes()).hexdigest() != source_hash:
            raise AssertionError("UAP or native CLI changed the portable source")
        if list((project_b / ".agents").glob("plugins/*")) if (project_b / ".agents").exists() else False:
            raise AssertionError("Project A installation mutated independent project B")
        if sentinel.read_text(encoding="utf-8") != "foreign must survive\n":
            raise AssertionError("Project installation modified foreign User plugin")
        if (Path(user_env["HOME"]) / ".gemini" / "config" / "plugins" / NAME).exists():
            raise AssertionError("Project installation registered global User plugin")

        result = run(
            uap, "remove", NAME, "--target", "antigravity", "--scope", "project",
            "--project-root", project, "--yes", "--format", "json",
            env=user_env, cwd=project,
        )
        if not json.loads(result) or plugin.exists():
            raise AssertionError("UAP did not remove only its exact-owned project plugin")
        if sentinel.read_text(encoding="utf-8") != "foreign must survive\n":
            raise AssertionError("UAP removal modified a foreign User plugin")
        print("PASS: real agy strict validation + separate CLI import/list/uninstall; "
              "UAP project add/remove and foreign User state preserved. "
              "IDE activation was not observed.", flush=True)


if __name__ == "__main__":
    try:
        main()
    except (AssertionError, KeyError, OSError, RuntimeError, subprocess.TimeoutExpired,
            json.JSONDecodeError) as error:
        print(f"Antigravity native E2E failed: {error}", file=sys.stderr)
        sys.exit(1)
