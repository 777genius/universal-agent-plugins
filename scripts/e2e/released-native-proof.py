#!/usr/bin/env python3
"""Bind the existing Hermes/Antigravity harnesses to a frozen release bundle.

No acquisition, compilation, publication, arbitrary command or credential input.
The producer's existing nine-asset verifier remains the asset authority.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import signal
import subprocess
import sys
import zipfile

TARGETS = ("linux-amd64", "darwin-arm64", "windows-amd64")
REPOSITORY = "777genius/universal-agent-plugins"
HERMES_PIN = "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19"
AGY_PINS = {
    "linux-amd64": "1b4850599cce73a17f94ffb422df3bf9531531b77dbddc40c1a435e0c2c16807",
    "darwin-arm64": "b47034e1367fc3c9f2856f1e1ae30bf12819a7213821ba3cce2123b090f2df8a",
    "windows-amd64": "9419c970c7988b33a3b61ce0c83ad0fdfb4c254103694ad9f1f565afe43f56aa",
}
DIGEST = r"[0-9a-f]{64}"


def require(ok, message):
    if not ok:
        raise ValueError(message)


def sha(path):
    path = Path(path)
    require(path.is_file() and not path.is_symlink(), "proof input must be a regular file")
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def client_environment():
    # npm config can hold literal auth as well as a path to an authenticated rc.
    return {key: value for key, value in os.environ.items()
            if not key.upper().endswith("_TOKEN")
            and not key.upper().startswith("NPM_CONFIG_")
            and not key.upper().startswith("ACTIONS_ID_TOKEN_")}


def output(command, env):
    return subprocess.check_output(command, env=env, timeout=60, text=True,
                                   encoding="utf-8", errors="replace").strip()


def harness_files(client, target):
    if client == "hermes":
        return ["scripts/e2e/hermes-native.py", "scripts/e2e/hermes-native-lifecycle.py",
                "scripts/pi_project_components_native_e2e.py"]
    return ["scripts/e2e/antigravity-native.sh" if target == "linux-amd64"
            else "scripts/e2e/antigravity-native-cross.py"]


def identity(args, env):
    require(env.get("GITHUB_REPOSITORY") == REPOSITORY, "unexpected producer repository")
    require(re.fullmatch(r"agentplugins-v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", args.tag),
            "invalid release tag")
    require(re.fullmatch(r"[0-9a-f]{40}", args.commit), "invalid frozen commit")
    require(re.fullmatch(DIGEST, args.checksums_sha256), "invalid checksums digest")
    require(args.run_id > 0 and args.run_attempt > 0, "invalid producer run identity")
    require(args.assets_artifact == f"agentplugins-draft-assets-{args.commit}",
            "unexpected same-run release assets artifact")
    source = Path(args.source).resolve(strict=True)
    require(output(["git", "-C", str(source), "rev-parse", "HEAD"], env) == args.commit
            and not output(["git", "-C", str(source), "status", "--porcelain",
                            "--untracked-files=no"], env), "checkout differs from frozen commit")
    assets = Path(args.assets).resolve(strict=True)
    require(len(list(assets.iterdir())) == 9, "release proof requires the complete nine-asset bundle")
    require(sha(assets / "checksums.txt") == args.checksums_sha256, "checksums digest mismatch")
    verified = json.loads(output(["node", str(source / "npm/agentplugins/scripts/release-assets.js"),
                                  "verify", str(assets), args.tag, args.commit], env))
    require(verified.get("gate_eligible") is True and verified.get("manifest_schema") == 2,
            "bundle is not eligible for release qualification")
    return {"schema_version": 1, "repository": REPOSITORY, "client": args.client, "tag": args.tag,
            "version": args.tag.removeprefix("agentplugins-v"), "source_commit": args.commit,
            "producer_run_id": args.run_id, "producer_run_attempt": args.run_attempt,
            "assets_artifact": args.assets_artifact,
            "checksums_sha256": args.checksums_sha256,
            "manifest_sha256": verified["manifest_sha256"]}, verified


def target_identity(common, verified, source, target):
    asset = verified["assets"][target]
    version = common["version"]
    filename = f"agentplugins_{version}_{target.replace('-', '_')}" + (
        ".exe" if target.startswith("windows-") else "")
    require(asset["file"] == filename, "release asset filename mismatch")
    return {**common, "target": target, "asset": asset,
            "harness": {name: sha(Path(source) / name)
                        for name in harness_files(common["client"], target)},
            "vendor_pin": HERMES_PIN if common["client"] == "hermes" else AGY_PINS[target]}


def host_target():
    os_name = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}.get(platform.system())
    arch = {"x86_64": "amd64", "AMD64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine())
    return f"{os_name}-{arch}"


def pinned_vendor_version(client, version):
    expected = "0.23.0" if client == "hermes" else "1.5.3"
    if client == "hermes":
        first_line = version.splitlines()[0] if version else ""
        match = re.match(r"Hermes Agent v((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))"
                         r"(?= \(| ·|$)", first_line)
        valid = match is not None and match[1] == expected
    else:
        valid = re.search(r"(?<![\d.])v?" + re.escape(expected) + r"(?![\d.])", version)
    require(valid, "native vendor version differs from pinned release")
    return expected


def windows_hermes_launcher_content_sha256(binary):
    """Hash the full distlib EXE, excluding only ZIP DOS timestamp fields.

    The pinned Hermes launcher is a distlib self-extracting EXE. Its upstream
    _launchers.py treats ZIP timestamps as non-identity. Preserve every other
    byte, including the PE loader, compressed entrypoint, ZIP flags, metadata
    and archive trailer; never normalize an arbitrary ZIP modification.
    """
    require(binary.suffix.lower() == ".exe", "Hermes Windows launcher must be an EXE")
    raw = bytearray(binary.read_bytes())
    with zipfile.ZipFile(binary) as archive:
        entries = archive.infolist()
        require(len(entries) == 1 and entries[0].filename == "__main__.py"
                and not entries[0].is_dir(), "unexpected Hermes launcher ZIP structure")
        entry = entries[0]
        require(entry.file_size <= (1 << 20) and not (entry.flag_bits & 1),
                "unexpected Hermes launcher entrypoint size or encryption")
        archive.read(entry)  # Verify the sole entrypoint and its CRC.
        local, central = entry.header_offset, archive.start_dir
    require(0 < local and local + 30 <= central and central + 46 <= len(raw)
            and raw[local:local + 4] == b"PK\x03\x04"
            and raw[central:central + 4] == b"PK\x01\x02",
            "invalid Hermes launcher ZIP header offsets")
    require(raw.startswith(b"MZ") and b"#!" in raw[:local],
            "invalid Hermes launcher executable preamble")
    # Only these local and central DOS time/date fields are normalized.
    raw[local + 10:local + 14] = b"\0" * 4
    raw[central + 12:central + 16] = b"\0" * 4
    return hashlib.sha256(raw).hexdigest()

def vendor_identity(client, env):
    binary = Path(env["HERMES_BIN" if client == "hermes" else "AGY_BIN"]).resolve(strict=True)
    command = [str(binary), "--version"]
    if os.name == "nt" and binary.suffix.lower() == ".cmd":
        command = ["cmd.exe", "/d", "/c", *command]
    version = output(command, env)
    pinned_vendor_version(client, version)
    runtime = binary
    if client == "hermes":
        source = Path(env["RUNNER_TEMP"]) / "hermes-agent"
        require(output(["git", "-C", str(source), "rev-parse", "HEAD"], env) == HERMES_PIN,
                "Hermes source pin mismatch")
        runtime_file = env.get("HERMES_NATIVE_RUNTIME_COMMAND_FILE")
        argv = json.loads(Path(runtime_file).read_text(encoding="utf-8-sig") if runtime_file else
                          output([str(binary), "--print-runtime-command"], env))
        require(isinstance(argv, list) and len(argv) >= 4 and all(isinstance(x, str) for x in argv)
                and Path(argv[0]).is_absolute() and argv[1:3] == ["-I", "-c"],
                "Hermes native runtime command is invalid")
        runtime = Path(argv[0]).resolve(strict=True)
    identity = {"version_output": version, "executable_sha256": sha(binary),
                "runtime_sha256": sha(runtime)}
    if client == "hermes" and os.name == "nt":
        # The official Windows installer admits an EXE or CMD launcher. Only
        # distlib EXEs may ignore two ZIP DOS timestamps; CMD stays byte-strict.
        kind = {".exe": "distlib-exe", ".cmd": "cmd"}.get(binary.suffix.lower())
        require(kind is not None, "unsupported Hermes Windows launcher kind")
        identity["launcher_kind"] = kind
        if kind == "distlib-exe":
            identity["launcher_content_sha256"] = windows_hermes_launcher_content_sha256(binary)
    return identity


def windows_hermes_non_loader_sha256(binary):
    """During pinned first startup, require the whole EXE after the PE loader to stay exact.

    vendor_identity() already verifies the single-entry distlib ZIP and CRC.
    Even during native self-repair, the interpreter shebang, compressed Python
    entrypoint, ZIP headers and trailing bytes must remain unmodified.
    """
    raw = Path(binary).read_bytes()
    with zipfile.ZipFile(binary) as archive:
        zip_start = archive.infolist()[0].header_offset
    shebang = raw.rfind(b"#!", 0, zip_start)
    require(shebang > 0 and raw.startswith(b"MZ")
            and raw[shebang:zip_start].rstrip(b"\\r\\n").endswith(b" -I"),
            "invalid pinned Hermes Windows EXE shebang")
    return hashlib.sha256(raw[shebang:]).hexdigest()


def settle_windows_hermes_launcher(env):
    """Complete pinned Hermes' ordinary Windows CLI startup BEFORE E2E baseline.

    --version and --print-runtime-command are early paths: they can succeed
    before the official CLI's first-launch repair republishes its PE stub.
    This happens once, under the pinned checkout and runtime. Native lifecycle
    then remains full-byte strict through vendor_comparison().
    """
    binary = Path(env["HERMES_BIN"]).resolve(strict=True)
    before = vendor_identity("hermes", env)  # Source, version, real Python pinned first.
    kind = before["launcher_kind"]
    unchanged = (windows_hermes_non_loader_sha256(binary)
                 if kind == "distlib-exe" else sha(binary))
    command = ([str(binary)] if kind == "distlib-exe"
               else ["cmd.exe", "/d", "/c", str(binary)])
    for args in (("skills", "list", "--source", "local", "--enabled-only"),
                 ("mcp", "list")):
        output([*command, *args], env)  # Real vendor binary, 60s per operation.
    settled = vendor_identity("hermes", env)
    require(kind == settled["launcher_kind"]
            and before["runtime_sha256"] == settled["runtime_sha256"]
            and pinned_vendor_version("hermes", before["version_output"])
            == pinned_vendor_version("hermes", settled["version_output"]),
            "Hermes Windows startup changed pinned runtime, version or launcher kind")
    require(unchanged == (windows_hermes_non_loader_sha256(binary)
                          if kind == "distlib-exe" else sha(binary)),
            "Hermes Windows startup changed launcher code or ZIP metadata")
    return settled


def vendor_comparison(client, vendor):
    if client != "hermes":
        return vendor
    # Version advisories do not define identity. Runtime bytes are always exact;
    # only Windows distlib ZIP metadata is excluded from launcher code identity.
    comparison = {"version": pinned_vendor_version(client, vendor["version_output"]),
                  "runtime_sha256": vendor["runtime_sha256"]}
    # An explicit native launcher kind prevents an EXE from silently falling
    # back to a CMD/raw-hash receipt, or vice versa, in release aggregation.
    kind = vendor.get("launcher_kind")
    if kind is not None:
        require(kind in ("distlib-exe", "cmd"), "unsupported Hermes Windows launcher kind")
        comparison["launcher_kind"] = kind
    if kind == "distlib-exe":
        require("launcher_content_sha256" in vendor, "Windows distlib digest missing")
        comparison["launcher_content_sha256"] = vendor["launcher_content_sha256"]
    else:
        require("launcher_content_sha256" not in vendor, "unexpected Windows distlib digest")
        comparison["executable_sha256"] = vendor["executable_sha256"]
    return comparison


def vendor_diagnostics(transcript, before, after=None, error=None):
    data = {"vendor_before": before}
    if error is not None:
        data["vendor_after_error"] = f"{type(error).__name__}: {error}"
    else:
        data.update(vendor_after=after, changed_fields=sorted(
            key for key in before.keys() | after.keys() if before.get(key) != after.get(key)))
    with transcript.open("a", encoding="utf-8") as log:
        log.write("\n" + json.dumps(data, sort_keys=True) + "\n")
    return data.get("changed_fields", [])


def execute(command, env, source, transcript):
    # Fixed harnesses already bound each individual operation; the outer bound
    # also kills the whole owned tree, retaining its transcript on failure.
    options = ({"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP}
               if os.name == "nt" else {"start_new_session": True})
    with transcript.open("wb") as log:
        child = subprocess.Popen(command, cwd=source, env=env, stdout=log,
                                 stderr=subprocess.STDOUT, **options)
        try:
            return child.wait(timeout=1200)
        except subprocess.TimeoutExpired:
            if os.name == "nt":
                subprocess.run(["taskkill", "/PID", str(child.pid), "/T", "/F"],
                               check=False, timeout=30, stdout=log, stderr=log)
            else:
                os.killpg(child.pid, signal.SIGKILL)
            child.wait(timeout=30)
            raise ValueError("native harness timed out; inspect transcript")


def run(args):
    env = client_environment()
    directory = Path(args.proofs)
    directory.mkdir(parents=True, exist_ok=True)
    receipt = directory / "receipt.json"
    require(not receipt.exists(), "refusing to reuse a prior pass receipt")
    common, verified = identity(args, env)
    require(args.target == host_target(), "release target does not match native host")
    proof = target_identity(common, verified, args.source, args.target)
    binary = Path(args.assets).resolve() / proof["asset"]["file"]
    require(sha(binary) == proof["asset"]["sha256"] and binary.stat().st_size == proof["asset"]["size"],
            "selected binary differs from release asset")
    if os.name != "nt":
        binary.chmod(binary.stat().st_mode | 0o100)
    env.update(UAP_BIN=str(binary), HERMES_NATIVE_UAP_HEAD=args.commit)
    version = output([str(binary), "version"], env)
    require(version == f"agentplugins {common['version']}", "actual UAP version differs from release tag")
    vendor = (settle_windows_hermes_launcher(env)
              if args.client == "hermes" and os.name == "nt" else vendor_identity(args.client, env))
    files = harness_files(args.client, args.target)
    command = ([sys.executable, str(Path(args.source) / files[0]), "run",
                str(Path(args.source) / files[1])] if args.client == "hermes" else
               (["bash", str(Path(args.source) / files[0])] if args.target == "linux-amd64"
                else [sys.executable, str(Path(args.source) / files[0])]))
    transcript = directory / "transcript.log"
    code = execute(command, env, args.source, transcript)
    require(code == 0, f"native harness exited {code}; inspect transcript")
    require(sha(binary) == proof["asset"]["sha256"], "release binary changed during native harness")
    try:
        vendor_after = vendor_identity(args.client, env)
    except Exception as error:
        vendor_diagnostics(transcript, vendor, error=error)
        raise
    changed = vendor_diagnostics(transcript, vendor, vendor_after)
    require(vendor_comparison(args.client, vendor_after) == vendor_comparison(args.client, vendor),
            "native vendor changed during harness: " + ", ".join(changed))
    common_after, verified_after = identity(args, env)
    require(proof == target_identity(common_after, verified_after, args.source, args.target),
            "release or harness identity changed during execution")
    proof.update(status="pass", exit_code=code, uap_version_output=version,
                 binary_sha256_before=proof["asset"]["sha256"], binary_sha256_after=sha(binary),
                 vendor=vendor, transcript_sha256=sha(transcript))
    receipt.write_text(json.dumps(proof, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def aggregate(args):
    common, verified = identity(args, client_environment())
    root = Path(args.proofs)
    receipts = list(root.rglob("receipt.json"))
    require(len(receipts) == 3, "expected exactly three native platform receipts")
    require(set(path for path in root.rglob("*") if path.is_file()) ==
            {file for receipt in receipts for file in (receipt, receipt.with_name("transcript.log"))},
            "unexpected native proof files")
    seen = set()
    for path in receipts:
        proof = json.loads(path.read_text(encoding="utf-8"))
        target = proof.get("target")
        require(target in TARGETS and target not in seen, "unexpected or duplicate native target")
        seen.add(target)
        expected = target_identity(common, verified, args.source, target)
        require(set(proof) == set(expected) | {"status", "exit_code", "uap_version_output",
                "binary_sha256_before", "binary_sha256_after", "vendor", "transcript_sha256"},
                "unexpected or missing native proof fields")
        require(json.dumps({key: proof.get(key) for key in expected}, sort_keys=True) ==
                json.dumps(expected, sort_keys=True),
                "mixed or modified native proof identity")
        require(proof.get("status") == "pass" and type(proof.get("exit_code")) is int
                and proof["exit_code"] == 0
                and proof.get("uap_version_output") == f"agentplugins {common['version']}"
                and proof.get("binary_sha256_before") == expected["asset"]["sha256"]
                and proof.get("binary_sha256_after") == expected["asset"]["sha256"]
                and proof.get("transcript_sha256") == sha(path.with_name("transcript.log")),
                "native proof did not pass with the exact binary and transcript")
        vendor = proof.get("vendor", {})
        required_vendor = {"version_output", "executable_sha256", "runtime_sha256"}
        if args.client == "hermes" and target == "windows-amd64":
            kind = vendor.get("launcher_kind")
            require(kind in ("distlib-exe", "cmd"), "invalid Hermes Windows launcher kind")
            required_vendor.add("launcher_kind")
            if kind == "distlib-exe":
                required_vendor.add("launcher_content_sha256")
        require(set(vendor) == required_vendor
                and all(re.fullmatch(DIGEST, vendor[key]) for key in required_vendor - {"version_output", "launcher_kind"}),
                "native vendor receipt is incomplete")
        pinned_vendor_version(args.client, vendor["version_output"])
    require(seen == set(TARGETS), "native target set is incomplete")
    print(f"PASS: {args.client}: exact release bundle and all three native platforms")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("run", "aggregate"))
    parser.add_argument("--client", choices=("hermes", "antigravity"), required=True)
    parser.add_argument("--target", choices=TARGETS)
    for name in ("tag", "commit", "checksums-sha256", "assets-artifact", "source", "assets", "proofs"):
        parser.add_argument("--" + name, required=True)
    for name in ("run-id", "run-attempt"):
        parser.add_argument("--" + name, required=True, type=int)
    args = parser.parse_args()
    (run if args.mode == "run" else aggregate)(args)


if __name__ == "__main__":
    main()
