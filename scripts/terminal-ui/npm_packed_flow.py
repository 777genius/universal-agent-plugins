#!/usr/bin/env python3
"""Pack, install and execute the current npm CLI against a disposable HOME."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess

from harness import Fixture, check


REPO = Path(__file__).resolve().parent.parent.parent
PACKAGE_SOURCE = REPO / "npm/agentplugins"
VERSION = "0.0.0-resilience.0"
PROOF_MODE = "local-frozen-release-asset-v1"
HISTORICAL_COMMIT = "01f02cb51cfe5f664d4d5f52b295c59c7ea03495"
HISTORICAL_DOCUMENT = "0cecbf12a96cf578d12e1cb2f582aa13dca2d9d900ccaefd223f5d7c1030ea65"
HISTORICAL_RECORD = "437da1bc7423a85b231be139ff9bfbd7e89c942ef216a61ebde668c08a9c2ee3"


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n",
                    encoding="utf-8")


def platform_contract():
    os_name = {"Darwin": "darwin", "Linux": "linux",
               "Windows": "windows"}.get(platform.system())
    arch_name = {"x86_64": "amd64", "AMD64": "amd64",
                 "arm64": "arm64", "aarch64": "arm64"}.get(
                     platform.machine())
    check(os_name is not None and arch_name is not None,
          "unsupported npm proof platform")
    suffix = ".exe" if os_name == "windows" else ""
    return (f"{os_name}-{arch_name}",
            f"agentplugins_{VERSION}_{os_name}_{arch_name}{suffix}",
            "agentplugins.exe" if os_name == "windows" else "agentplugins")


def historical_evidence():
    revision = "1" * 40
    return {
        "schema_version": 1,
        "kind": "agentplugins_client_lifecycle",
        "recorded_at": "2026-08-30",
        "document_sha256": HISTORICAL_DOCUMENT,
        "record_sha256": HISTORICAL_RECORD,
        "source": {
            "repository": "777genius/plugin-kit-ai",
            "commit": HISTORICAL_COMMIT,
            "document": {
                "path": "docs/AGENTPLUGINS_CLIENT_E2E.md",
                "sha256": HISTORICAL_DOCUMENT,
            },
            "record": {
                "path": "docs/evidence/agentplugins-client-e2e-2026-08-30.json",
                "sha256": HISTORICAL_RECORD,
            },
        },
        "installer": {
            "repository": "777genius/plugin-kit-ai",
            "commit": HISTORICAL_COMMIT,
            "tree": "e" * 40,
            "version": "0.1.22",
            "binary_sha256": "f" * 64,
        },
        "package": {
            "selector": f"owner/package@{revision}",
            "revision": revision,
            "tree_digest": "sha256:" + "2" * 64,
            "manifest_digest": "sha256:" + "3" * 64,
        },
        "claim_boundary": {
            "lifecycle_e2e": True,
            "client_discovery_e2e": True,
            "browser_tool_runtime_e2e": False,
            "model_turn_e2e": False,
            "login_e2e": False,
            "oauth_e2e": False,
            "windsurf_skill_activation_claimed": False,
        },
    }


def run(evidence, label, argv, cwd, env):
    result = subprocess.run(
        [str(value) for value in argv], cwd=cwd, env=env,
        capture_output=True, text=True, timeout=120)
    write_json(evidence / f"{label}.command.json",
               [str(value) for value in argv])
    (evidence / f"{label}.stdout").write_text(
        result.stdout, encoding="utf-8")
    (evidence / f"{label}.stderr").write_text(
        result.stderr, encoding="utf-8")
    check(result.returncode == 0,
          f"{label} failed ({result.returncode}): {result.stderr!r}")
    return result


def envelope(result, label):
    try:
        value = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        raise AssertionError(
            f"{label} did not emit one JSON envelope: {error}") from error
    check(value.get("schema_version") == 1 and
          value.get("result") == "success",
          f"{label} returned a failure envelope: {value}")
    return value


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--npm", type=Path, required=True)
    parser.add_argument("--node", type=Path, required=True)
    parser.add_argument("--artifacts", type=Path, required=True)
    args = parser.parse_args()

    binary = args.binary.resolve()
    check(binary.is_file(), "current-source binary is missing")
    artifacts = args.artifacts.resolve()
    check(not artifacts.exists(), "artifacts directory already exists")
    artifacts.mkdir(parents=True)
    evidence = artifacts / "evidence"
    evidence.mkdir()
    fixture = Fixture(artifacts / "fixture")

    staged = artifacts / "staged-package"
    shutil.copytree(PACKAGE_SOURCE, staged,
                    ignore=shutil.ignore_patterns("node_modules"))
    package_file = staged / "package.json"
    package_data = json.loads(package_file.read_text(encoding="utf-8"))
    package_data["version"] = VERSION
    write_json(package_file, package_data)

    platform_key, asset_name, cached_name = platform_contract()
    release = artifacts / "release"
    release.mkdir()
    asset = release / asset_name
    shutil.copy2(binary, asset)
    asset.chmod(0o700)
    head = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=REPO, text=True).strip()
    manifest = {
        "schema_version": 2,
        "version": VERSION,
        "npm_package": "universal-agent-plugins",
        "repository": "777genius/plugin-kit-ai",
        "tag": f"agentplugins-v{VERSION}",
        "producer": {
            "repository": "777genius/plugin-kit-ai",
            "tag": f"agentplugins-v{VERSION}",
            "commit": head,
            "release_manifest": {
                "schema_version": 2,
                "sha256": digest(binary),
                "version": VERSION,
            },
        },
        "client_evidence": historical_evidence(),
        "assets": {
            platform_key: {
                "file": asset_name,
                "sha256": digest(binary),
                "size": binary.stat().st_size,
            },
        },
    }
    write_json(staged / "assets.json", manifest)

    npm_env = {
        "HOME": str(fixture.home),
        "USERPROFILE": str(fixture.home),
        "PATH": os.environ.get("PATH", ""),
        "TMPDIR": str(fixture.root / "tmp"),
        "TMP": str(fixture.root / "tmp"),
        "TEMP": str(fixture.root / "tmp"),
        "NPM_CONFIG_CACHE": str(artifacts / "npm-cache"),
        "NPM_CONFIG_USERCONFIG": str(artifacts / "empty-user.npmrc"),
        "NPM_CONFIG_GLOBALCONFIG": str(artifacts / "empty-global.npmrc"),
        "NPM_CONFIG_IGNORE_SCRIPTS": "true",
        "NPM_CONFIG_AUDIT": "false",
        "NPM_CONFIG_FUND": "false",
    }
    if os.name == "nt":
        for key in ("SystemRoot", "WINDIR", "COMSPEC"):
            npm_env[key] = os.environ[key]
    (artifacts / "empty-user.npmrc").write_text("", encoding="utf-8")
    (artifacts / "empty-global.npmrc").write_text("", encoding="utf-8")
    packed = artifacts / "packed"
    packed.mkdir()
    packed_result = run(
        evidence, "npm-pack",
        [args.npm, "pack", "--ignore-scripts", "--json",
         "--pack-destination", packed], staged, npm_env)
    pack_rows = json.loads(packed_result.stdout)
    check(isinstance(pack_rows, list) and len(pack_rows) == 1,
          "npm pack did not return exactly one artifact")
    tarball = packed / pack_rows[0]["filename"]
    check(tarball.is_file(), "npm pack tarball is missing")

    consumer = artifacts / "consumer"
    consumer.mkdir()
    run(evidence, "npm-install",
        [args.npm, "install", "--prefix", consumer, "--ignore-scripts",
         "--offline", "--no-audit", "--no-fund", tarball],
        consumer, npm_env)
    package_root = consumer / "node_modules/universal-agent-plugins"
    shim = package_root / "bin/agentplugins.js"
    check(shim.is_file(), "installed npm launcher is missing")
    bin_root = consumer / "node_modules/.bin"
    installed_bin = bin_root / (
        "agentplugins.cmd" if os.name == "nt" else "agentplugins")
    check(installed_bin.is_file(),
          "npm did not create the expected agentplugins bin shim")

    cli_env = dict(fixture.env)
    cli_env.update({
        "AGENTPLUGINS_CACHE_DIR": str(artifacts / "binary-cache"),
        "AGENTPLUGINS_INTERNAL_PROOF_MODE": PROOF_MODE,
        "AGENTPLUGINS_INTERNAL_PROOF_BINARY": str(asset),
    })
    if os.name == "posix":
        (fixture.bin / "node").symlink_to(args.node.resolve())
        cli = [installed_bin]
    else:
        cli = [args.node, shim]
    version = run(evidence, "version", cli + ["version"],
                  fixture.project, cli_env)
    check(version.stdout.startswith("agentplugins ") and
          version.stdout.count("\n") == 1,
          f"packed launcher executed the wrong binary: {version.stdout!r}")
    add = envelope(run(
        evidence, "add", cli + ["add", fixture.package,
        "--target=cursor", "--format=json"], fixture.project, cli_env),
        "add")
    check(add["data"]["targets"][0]["target"] == "cursor",
          "packed add targeted the wrong client")
    doctor = envelope(run(
        evidence, "doctor-installed", cli + ["doctor", "--format=json"],
        fixture.project, cli_env), "doctor-installed")
    check(doctor["data"]["installation_count"] == 1,
          "packed add was not recorded")
    envelope(run(
        evidence, "remove", cli + ["remove", "pty-synthetic",
        "--target=cursor", "--external-uninstalled", "--purge-data",
        "--format=json"], fixture.project, cli_env), "remove")
    clean = envelope(run(
        evidence, "doctor-removed", cli + ["doctor", "--format=json"],
        fixture.project, cli_env), "doctor-removed")
    check(clean["data"]["installation_count"] == 0,
          "packed remove left installation state")

    cached = (artifacts / "binary-cache" / VERSION / platform_key /
              cached_name)
    check(cached.is_file() and digest(cached) == digest(binary),
          "npm bootstrap cache does not match the source binary")
    write_json(artifacts / "report.json", {
        "schema_version": 1,
        "status": "pass",
        "version": VERSION,
        "platform": platform_key,
        "source_binary_sha256": digest(binary),
        "cached_binary_sha256": digest(cached),
        "tarball_sha256": digest(tarball),
        "isolation": "disposable HOME, npm cache, installer state and clients",
    })
    print(json.dumps({"status": "pass", "tarball": tarball.name}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
