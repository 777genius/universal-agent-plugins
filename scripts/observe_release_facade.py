#!/usr/bin/env python3
"""Execute one manifest-bound release facade and emit challenge-correlated evidence."""

from __future__ import annotations

import argparse
import hashlib
import json
import platform
import shutil
import subprocess
from datetime import datetime, timezone
from pathlib import Path


def now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--context", type=Path, required=True)
    parser.add_argument("--executable", required=True)
    parser.add_argument("--asset-name", required=True)
    parser.add_argument("--kind", choices=("binary", "npm"), required=True)
    parser.add_argument("--os", required=True)
    parser.add_argument("--architecture", required=True)
    parser.add_argument("--node-major", type=int)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    context = json.loads(args.context.read_text())
    executable = shutil.which(args.executable) or args.executable
    started = now()
    completed = subprocess.run([executable, "version"], text=True, capture_output=True, timeout=30, check=False)
    observed = now()
    version = completed.stdout.strip().removeprefix("agentplugins ").strip()
    if completed.returncode or version != context["release_manifest"]["version"]:
        raise RuntimeError("manifest-bound facade returned an unexpected version")
    if args.kind == "npm":
        declared = context.get("npm_package")
        if not declared or declared.get("name") != "universal-agent-plugins" or declared.get("version") != version:
            raise RuntimeError("npm facade is not bound to exact registry package metadata")
        asset_path = args.context.parent / "npm" / f"universal-agent-plugins-{version}.tgz"
        declared_digest = declared["sha256"]
    else:
        declared = next(item for item in context["release_manifest"]["assets"].values() if item["file"] == args.asset_name)
        asset_path = args.context.parent / "release" / args.asset_name
        declared_digest = declared["sha256"]
    asset_body = asset_path.read_bytes()
    if len(asset_body) != declared["size"] or hashlib.sha256(asset_body).hexdigest() != declared_digest:
        raise RuntimeError("executed facade asset differs from its authenticated distribution identity")
    value = {
        "schema_version": 1, "kind": args.kind, "os": args.os, "architecture": args.architecture,
        "node_major": args.node_major, "executed": True, "version": version,
        "catalog_repository": context["catalog_repository"], "catalog_sha": context["github"]["sha"],
        "cli_release_repository": context["cli_release_repository"], "cli_release_tag": context["cli_release_tag"],
        "release_manifest_digest": context["release_manifest_digest"],
        "release_checksums_digest": context["release_checksums_digest"],
        "directory_digest": context["directory"]["digest"],
        "asset_name": args.asset_name, "asset_digest": "sha256:" + declared_digest,
        "challenge": context["challenge"]["value"], "challenge_context": context["challenge"],
        "started_at": started, "observed_at": observed,
        "command_trace": {
            "challenge": context["challenge"]["value"], "argv": ["agentplugins", "version"],
            "started_at": started, "ended_at": observed, "exit_code": completed.returncode,
            "stdout_digest": "sha256:" + hashlib.sha256(completed.stdout.encode()).hexdigest(),
            "stderr_digest": "sha256:" + hashlib.sha256(completed.stderr.encode()).hexdigest(),
        },
        "runner_platform": platform.platform(),
    }
    if args.kind == "npm":
        value["npm_package"] = {
            "name": declared["name"], "version": declared["version"],
            "integrity": declared["integrity"], "tarball": declared["tarball"],
            "metadata_digest": declared["metadata_digest"],
        }
    args.output.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
