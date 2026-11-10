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
REMOTE_URL = "https://example.invalid/uap-native-smoke"


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


def tree_snapshot(root):
    """Record every fixture entry, including empty directories and file bytes."""
    result = {}
    for entry in sorted(root.rglob("*")):
        if entry.is_symlink():
            raise AssertionError("Unexpected symlink in the disposable native fixture")
        result[entry.relative_to(root).as_posix()] = (
            "directory" if entry.is_dir() else hashlib.sha256(entry.read_bytes()).hexdigest()
        )
    return result


def assert_native_mcp(plugin, fixture_root):
    config = json.loads((plugin / "mcp_config.json").read_text(encoding="utf-8"))
    servers = config.get("mcpServers", {})
    if set(servers) != {"native-stdio", "native-http"}:
        raise AssertionError("Native MCP projection lost or added a selected server")
    stdio = servers["native-stdio"]
    if stdio.get("command") != "node" or "type" in stdio or "url" in stdio or "serverUrl" in stdio:
        raise AssertionError("Native stdio transport was not projected unambiguously")
    if Path(stdio["cwd"]).resolve() != plugin.resolve():
        raise AssertionError("Native stdio working directory is not the active plugin")
    if Path(stdio["env"]["PLUGIN_ROOT"]).resolve() != plugin.resolve():
        raise AssertionError("Native stdio environment retained a staging path")
    data = Path(stdio["env"]["PLUGIN_DATA"])
    if not data.is_absolute() or not data.is_dir() or fixture_root.resolve() not in data.resolve().parents:
        raise AssertionError("Native stdio data directory is not an owned disposable directory")
    if stdio["env"]["UAP_SMOKE_DATA"] != str(data):
        raise AssertionError("Native stdio plugin data variable was not expanded")
    args = stdio.get("args", [])
    if (len(args) != 3 or Path(args[0]).resolve() != (plugin / "scripts" / "mcp-smoke.cjs").resolve()
            or args[1] != "--data-dir" or Path(args[2]).resolve() != data.resolve()):
        raise AssertionError("Native stdio arguments do not reference committed plugin/data paths")
    remote = servers["native-http"]
    if remote != {"serverUrl": REMOTE_URL, "headers": {"X-UAP-Smoke": "native-e2e"}}:
        raise AssertionError("Native Streamable HTTP URL or headers were not preserved")
    if "${PLUGIN_" in json.dumps(config):
        raise AssertionError("A client-managed placeholder remained in native MCP configuration")


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
        scripts = source / "scripts"
        scripts.mkdir()
        helper = scripts / "mcp-smoke.cjs"
        helper.write_text(
            "// Native validation fixture; no network access or model calls.\n"
            "const lines = require('node:readline').createInterface({ input: process.stdin });\n"
            "lines.on('line', line => {\n"
            "  const request = JSON.parse(line);\n"
            "  if (request.id === undefined) return;\n"
            "  const result = request.method === 'initialize'\n"
            "    ? { protocolVersion: request.params.protocolVersion, capabilities: { tools: {} },\n"
            "        serverInfo: { name: 'uap-native-smoke', version: '1.0.0' } }\n"
            "    : { tools: [] };\n"
            "  process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: request.id, result }) + '\\n');\n"
            "});\n",
            encoding="utf-8",
        )
        if os.name != "nt":
            helper.chmod(0o755)
        (source / "mcp.json").write_text(json.dumps({
            "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {
                "native-stdio": {
                    "type": "stdio", "command": "node",
                    "args": ["${PLUGIN_ROOT}/scripts/mcp-smoke.cjs", "--data-dir", "${PLUGIN_DATA}"],
                    "cwd": "${PLUGIN_ROOT}", "env": {"UAP_SMOKE_DATA": "${PLUGIN_DATA}"},
                },
                "native-http": {
                    "type": "streamable-http", "url": REMOTE_URL,
                    "headers": {"X-UAP-Smoke": "native-e2e"},
                },
            },
        }, indent=2) + "\n", encoding="utf-8")
        source_snapshot = tree_snapshot(source)
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
        assert_native_mcp(plugin, root)
        projected_snapshot = tree_snapshot(plugin)

        # Use the public CLI with a genuine tracked native Project installation.
        # Repeated update must preserve the exact source and native manifest.
        updated = run(
            uap, "update", NAME, "--target", "antigravity", "--scope", "project",
            "--project-root", project, "--yes", "--format", "json",
            env=user_env, cwd=project,
        )
        if not json.loads(updated):
            raise AssertionError("UAP update produced no structured result")
        if hashlib.sha256(vendor_manifest.read_bytes()).hexdigest() != projected_hash:
            raise AssertionError("Idempotent Project update changed the native manifest")
        skill_path = plugin / "skills" / "native-smoke" / "SKILL.md"
        original_skill = (source / "skills" / "native-smoke" / "SKILL.md").read_bytes()
        skill_path.unlink()
        damaged_snapshot = tree_snapshot(plugin)
        repair_args = (
            "repair", NAME, "--target", "antigravity", "--scope", "project",
            "--project-root", project, "--yes", "--format", "json",
        )
        # Grouped repair deliberately refuses partially changed packages.
        # Preserve this ownership gate and prove refusal does not touch bytes.
        try:
            run(uap, *repair_args, env=user_env, cwd=project)
        except RuntimeError as error:
            if "native identity ownership is indeterminate; refusing repair" not in str(error):
                raise
        else:
            raise AssertionError("Repair silently replaced a partially changed native package")
        if tree_snapshot(plugin) != damaged_snapshot or tree_snapshot(source) != source_snapshot:
            raise AssertionError("Refused partial-package repair changed fixture bytes")
        if sentinel.read_text(encoding="utf-8") != "foreign must survive\n":
            raise AssertionError("Refused partial-package repair changed a foreign User plugin")
        # The exact-owned bundle is disposable test data. A fully absent bundle
        # is the supported recovery case, with its receipt and revision intact.
        shutil.rmtree(plugin)
        repaired = run(uap, *repair_args, env=user_env, cwd=project)
        if not json.loads(repaired) or skill_path.read_bytes() != original_skill:
            raise AssertionError("Exact-revision absent Project repair did not restore owned Skill bytes")
        if tree_snapshot(plugin) != projected_snapshot:
            raise AssertionError("Absent Project repair did not restore the exact native package")
        assert_native_mcp(plugin, root)
        if sentinel.read_text(encoding="utf-8") != "foreign must survive\n":
            raise AssertionError("Update or repair changed a foreign User plugin")

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

        # Exercise the public UAP User lifecycle independently of the Project
        # receipt and the separate vendor-only import/uninstall profile.
        user_native_env = profile(root / "uap-native-user-scope", executable_dir)
        native_user_root = Path(user_native_env["HOME"]) / ".gemini" / "config" / "plugins"
        user_result = run(
            uap, "add", source, "--target", "antigravity", "--scope", "user",
            "--no-save", "--yes", "--format", "json",
            env=user_native_env, cwd=project_b,
        )
        if not json.loads(user_result) or not native_user_root.is_dir():
            raise AssertionError("UAP User installation did not create native plugin root")
        user_plugins = [
            entry for entry in native_user_root.iterdir()
            if entry.is_dir() and not entry.is_symlink()
            and not entry.name.startswith(".agentplugins-staging-")
        ]
        if len(user_plugins) != 1:
            raise AssertionError("UAP User scope did not create exactly one owned plugin")
        user_plugin = user_plugins[0]
        user_manifest = json.loads((user_plugin / "plugin.json").read_text(encoding="utf-8"))
        if (user_manifest.get("name") != NAME or
                user_manifest.get("$schema") != "https://antigravity.google/schemas/v1/plugin.json"):
            raise AssertionError("UAP User scope did not project a native manifest")
        if not (user_plugin / "skills" / "native-smoke" / "SKILL.md").is_file():
            raise AssertionError("UAP User scope omitted the selected Skill")
        assert_native_mcp(user_plugin, root)
        run(agy, "plugin", "validate", "--strict", user_plugin, env=user_native_env, cwd=project_b)
        # Current agy can validate a prepared native User bundle without
        # automatically registering it. Record discovery, but never equate
        # native files or list membership with runtime enablement.
        user_listed = NAME in checked_list(agy, user_native_env, project_b)
        print(f"Native UAP User bundle validated; agy listed={user_listed}; "
              "activation was not attested.", flush=True)
        user_removed = run(
            uap, "remove", NAME, "--target", "antigravity", "--scope", "user",
            "--yes", "--format", "json", env=user_native_env, cwd=project_b,
        )
        if not json.loads(user_removed) or user_plugin.exists():
            raise AssertionError("UAP User-scoped plugin survived exact-owned removal")
        if not plugin.is_dir() or hashlib.sha256(vendor_manifest.read_bytes()).hexdigest() != projected_hash:
            raise AssertionError("UAP User-scoped removal changed Project-scoped plugin")

        result = run(
            uap, "remove", NAME, "--target", "antigravity", "--scope", "project",
            "--project-root", project, "--yes", "--format", "json",
            env=user_env, cwd=project,
        )
        if not json.loads(result) or plugin.exists():
            raise AssertionError("UAP did not remove only its exact-owned project plugin")
        if sentinel.read_text(encoding="utf-8") != "foreign must survive\n":
            raise AssertionError("UAP removal modified a foreign User plugin")
        if tree_snapshot(source) != source_snapshot:
            raise AssertionError("Native lifecycle changed the verified portable source")
        print("PASS: real agy strict validation of Skills + stdio/HTTP MCP, separate CLI "
              "import/list/uninstall; UAP User/Project add/remove, idempotent update, "
              "partial-drift refusal and exact absent-bundle repair. Foreign User state "
              "preserved. IDE activation and live MCP execution were not observed.", flush=True)


if __name__ == "__main__":
    try:
        main()
    except (AssertionError, KeyError, OSError, RuntimeError, subprocess.TimeoutExpired,
            json.JSONDecodeError) as error:
        print(f"Antigravity native E2E failed: {error}", file=sys.stderr)
        sys.exit(1)
