"""Real pinned Remotion Skill lifecycle on short and default-long Windows TEMP.

Reuses the existing published/native case auditor with a published npm CLI pinned to release 0.1.102.
No alternate success interpretation, fixture edit or native trust bypass.
"""

import hashlib
import importlib.util
import json
import os
import re
import time
from pathlib import Path
from runpy import run_path
import shutil
import subprocess
import tempfile

FIXTURE_SHA256 = "e22cb3b5b23e74a9e03b55e24a684ab93038f89cf18e28fd5d9755175754c0d9"
REMOTION_SHA = "c7cb2fdcacd134fb2ccb9b7a44f9ffa44e467271"
_WINDOWS_CUSTODY = run_path(str(Path(__file__).with_name("gemini_windows_custody.py")))
_TEMP_ROOTS = run_path(str(Path(__file__).with_name("remotion_windows_long_temp_e2e.py")))
temp_root_authority = _TEMP_ROOTS["temp_root_authority"]
physical_temp_root = _TEMP_ROOTS["physical_temp_root"]


def acquire_published_cli():
    """Authenticate the installed npm release before creating custody profiles."""
    npm = shutil.which("npm.cmd")
    node = shutil.which("node")
    if not npm or not node:
        raise SystemExit("published npm acquisition tools missing")
    root = subprocess.run([npm, "root", "--global"], capture_output=True, text=True,
                          errors="strict", timeout=30)
    if root.returncode != 0 or root.stderr or len(root.stdout) > 32768:
        raise SystemExit("published npm package root query failed")
    package = Path(root.stdout.strip()) / "universal-agent-plugins"
    if not package.is_absolute():
        raise SystemExit("published npm package root is not absolute")
    script = r"""
const fs = require("node:fs");
const path = require("node:path");
const packageRoot = process.argv[1];
const pkg = JSON.parse(fs.readFileSync(path.join(packageRoot, "package.json"), "utf8"));
if (pkg.name !== "universal-agent-plugins" || pkg.version !== "0.1.102")
  throw new Error("installed npm package identity mismatch");
const bootstrap = require(path.join(packageRoot, "lib", "bootstrap.js"));
const platform = require(path.join(packageRoot, "lib", "platform.js")).detectPlatform();
if (platform.key !== "windows-amd64") throw new Error("published Windows platform mismatch");
const release = bootstrap.loadRelease(packageRoot, platform);
if (release.version !== "0.1.102") throw new Error("published release version mismatch");
bootstrap.ensureBinary({packageRoot}).then(selected => {
  if (selected.version !== release.version || selected.publicAuthoring === true)
    throw new Error("acquired release mismatch");
  process.stdout.write(JSON.stringify({binary_path: selected.binaryPath, version: release.version,
    asset: release.asset, package_name: pkg.name, package_version: pkg.version}));
}).catch(error => { process.stderr.write(String(error.message)); process.exitCode = 1; });
"""
    environment = dict(os.environ)
    for key in ("AGENTPLUGINS_INTERNAL_PROOF_MODE", "AGENTPLUGINS_INTERNAL_PROOF_BINARY"):
        environment.pop(key, None)
    acquired = subprocess.run([node, "-e", script, str(package)], capture_output=True,
                              text=True, errors="strict", timeout=30, env=environment)
    if acquired.returncode != 0 or acquired.stderr or len(acquired.stdout) > 65536:
        raise SystemExit("published npm binary acquisition failed")
    receipt = json.loads(acquired.stdout)
    if (type(receipt) is not dict or receipt.get("version") != "0.1.102"
            or receipt.get("package_name") != "universal-agent-plugins"
            or receipt.get("package_version") != "0.1.102"):
        raise SystemExit("published npm binary identity mismatch")
    asset = receipt.get("asset")
    if (type(asset) is not dict
            or asset.get("file") != "agentplugins_0.1.102_windows_amd64.exe"
            or type(asset.get("size")) is not int or asset["size"] <= 0
            or type(asset.get("sha256")) is not str
            or not re.fullmatch("[0-9a-f]{64}", asset["sha256"])):
        raise SystemExit("published npm Windows asset pin invalid")
    cli = Path(receipt.get("binary_path", ""))
    if not cli.is_absolute() or cli.suffix.lower() != ".exe":
        raise SystemExit("published npm binary path invalid")
    # Reuse the Windows observer's held NTFS parent/member authority.
    # This claim ends when these verification handles close; lifecycle calls
    # retain the existing auditor's own custody and acceptance requirements.
    deadline = time.monotonic() + 30
    custody = _WINDOWS_CUSTODY["Custody"](lambda: deadline)
    try:
        custody.hold(cli.parent)
        fd, size = custody.open_file(cli, asset["size"])
        if size != asset["size"]:
            raise SystemExit("published npm binary size mismatch")
        custody.guard()
        digest = hashlib.sha256()
        remaining = asset["size"] + 1
        observed = 0
        while remaining:
            custody.guard()
            chunk = os.read(fd, min(1 << 20, remaining))
            if not chunk:
                break
            observed += len(chunk)
            remaining -= len(chunk)
            digest.update(chunk)
        custody.guard()
        if observed != asset["size"] or digest.hexdigest() != asset["sha256"]:
            raise SystemExit("published npm binary digest or custody mismatch")
    finally:
        custody.close()
    return cli, {"package_name": receipt["package_name"], "package_version": receipt["package_version"],
                 "asset": asset, "binary_path": str(cli), "sha256_verified": True,
                 "acquisition_scope": "runner-before-isolated-cases",
                 "binary_verification": "held-ntfs-parents-and-file",
                 "verification_custody_closed_before_lifecycle": True}


def main():
    if os.name != "nt":
        raise SystemExit("genuine Windows runner required")
    repo = Path(__file__).resolve().parent.parent
    fixture_path = repo / "tests/live-install/fixtures.json"
    report = Path(os.environ["RUNNER_TEMP"]) / "remotion-windows-published-0.1.102.json"
    # Windows checkout may convert LF to CRLF. Bind the immutable fixture to
    # Git's committed object, then accept only that exact platform conversion
    # in the disposable worktree; never invent or rewrite fixture bytes.
    fixture_bytes = subprocess.check_output(
        ["git", "show", "HEAD:tests/live-install/fixtures.json"], cwd=repo)
    checkout_bytes = fixture_path.read_bytes()
    if hashlib.sha256(fixture_bytes).hexdigest() != FIXTURE_SHA256:
        report.write_text(json.dumps({"stage": "fixture", "reason": "git-blob-digest"}) + "\n")
        raise SystemExit("immutable Git fixture SHA-256 mismatch")
    if checkout_bytes not in (fixture_bytes, fixture_bytes.replace(b"\n", b"\r\n")):
        report.write_text(json.dumps({"stage": "fixture", "reason": "unexpected-worktree-bytes"}) + "\n")
        raise SystemExit("checkout modified immutable fixture beyond Windows EOL conversion")
    fixtures = json.loads(fixture_bytes)
    item = fixtures["skills"][24]
    if item["name"] != "remotion-best-practices" or REMOTION_SHA not in item["source"]:
        raise SystemExit("wrong immutable Remotion skill source")

    module_path = repo / "scripts/live-install-audit.py"
    spec = importlib.util.spec_from_file_location("uap_remotion_audit", module_path)
    audit = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(audit)
    audit.validate_fixtures(fixtures)

    shim = shutil.which("agentplugins")
    if not shim:
        raise SystemExit("published npm CLI shim missing")
    launcher = Path(shim).resolve(strict=True)
    if not launcher.is_file() or launcher.suffix.lower() not in (".cmd", ".exe"):
        raise SystemExit("published Windows CLI shim invalid")
    launcher_version = subprocess.run([str(launcher), "version"], capture_output=True, text=True,
                                     errors="replace", timeout=30)
    if launcher_version.returncode != 0 or launcher_version.stdout.strip() != "agentplugins 0.1.102":
        raise SystemExit("published npm CLI version does not match 0.1.102")
    cli, acquisition = acquire_published_cli()
    version = subprocess.run([str(cli), "version"], capture_output=True, text=True,
                             errors="replace", timeout=30)
    if version.returncode != 0 or version.stdout.strip() != "agentplugins 0.1.102":
        raise SystemExit("published npm CLI version does not match 0.1.102")
    evidence = {
        "source_checkout_sha": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
        "fixture_sha256": FIXTURE_SHA256,
        "fixture_worktree_eol": ("unchanged" if checkout_bytes == fixture_bytes else "crlf-only"),
        "remotion_commit": REMOTION_SHA,
        "cli_path_kind": "published-npm-acquired-windows-executable",
        "npm_acquisition": acquisition,
        "launcher_preflight": {"path": str(launcher), "version": launcher_version.stdout.strip()},
        "lifecycle_execution": "verified-published-binary-direct",
        "published_version": version.stdout.strip(),
        "published_tag": "agentplugins-v0.1.102",
        "results": {},
        "temp_roots": {},
    }
    failures = []
    created_roots, physical_roots = {}, {}
    try:
        for label, options in (
                ("runner-temp", {"prefix": "uap-remotion-short-", "dir": os.environ["RUNNER_TEMP"]}),
                ("default-windows-temp", {"prefix": "uap-live-30x30-"})):
            created = Path(tempfile.mkdtemp(**options))
            created_roots[label] = (created, None)
            evidence["temp_roots"][label] = {
                "created_path": str(created), "created_root_chars": len(str(created / "skill-24")),
                "cleanup": "retained-unverified",
            }
            authority = temp_root_authority(created)
            created_roots[label] = (created, authority)
            physical_roots[label] = physical_temp_root(created, authority)
            evidence["temp_roots"][label]["root_chars"] = len(str(physical_roots[label] / "skill-24"))
        short, long = physical_roots["runner-temp"], physical_roots["default-windows-temp"]
        created_lengths = {label: row["created_root_chars"]
                           for label, row in evidence["temp_roots"].items()}
        short_length = len(str(short / "skill-24"))
        long_length = len(str(long / "skill-24"))
        for shorter, longer in ((created_lengths["runner-temp"], created_lengths["default-windows-temp"]),
                                (short_length, long_length)):
            if longer < 65 or longer <= shorter:
                raise RuntimeError("default Windows TEMP is not longer than runner TEMP")
        for label, root in (("runner-temp", short), ("default-windows-temp", long)):
            try:
                row = audit.do_case(str(cli), "codex", "skill", item, 24, root,
                                    fixtures["plugin_declared_names"], fixtures["signed_release_pins"])
                evidence["results"][label] = {"root_chars": len(str(root / "skill-24")),
                                              "created_root_chars": created_lengths[label], "case": row}
                if row.get("status") != "passed":
                    failures.append(f"{label}: native E2E case did not PASS")
            except Exception as error:
                failures.append(f"{label}: {type(error).__name__}")
                evidence["results"][label] = {"root_chars": len(str(root / "skill-24")),
                                              "created_root_chars": created_lengths[label],
                                              "error_type": type(error).__name__,
                                              "error_traceback": audit.bounded_error_traceback(error)}
    except Exception as error:
        failures.append(f"TEMP setup: {type(error).__name__}")
        evidence["setup_error"] = {"error_type": type(error).__name__,
                                   "error_traceback": audit.bounded_error_traceback(error)}
    finally:
        # Keep both created objects for evidence; a later pathname deletion
        # cannot be authorized by an earlier same-object identity check.
        # The unchanged native auditor still requires product uninstall.
        for label, (created, authority) in created_roots.items():
            try:
                if authority is None or temp_root_authority(created) != authority:
                    raise ValueError("created TEMP root authority is unverified or changed")
                evidence["temp_roots"][label]["cleanup"] = "retained-for-evidence"
            except Exception as error:
                failures.append(f"{label}: TEMP ownership {type(error).__name__}")
                evidence["temp_roots"][label].update({
                    "cleanup": "retained-unverified", "cleanup_error_type": type(error).__name__,
                    "cleanup_error_traceback": audit.bounded_error_traceback(error),
                })
        report.write_text(json.dumps(evidence, indent=2) + "\n", encoding="utf-8")
    if failures:
        raise SystemExit("; ".join(failures))
    print("Published npm 0.1.102 Codex Remotion lifecycle PASS in both Windows TEMP profiles")


if __name__ == "__main__":
    main()
