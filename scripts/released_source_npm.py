"""Public npm acquisition; lifecycle callers must execute the returned JS prefix.

Reuse the released public-contract verifier and bounded package inspector. No
proof-mode binary injection, product rebuild, npm hooks, or ambient npm config.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import threading
import time
import re
import sys
from runpy import run_path

VERSION = "0.1.116"
COMMIT = "4858d973afcf6cf31b49bba98bdf06482dce6226"
TAG = "agentplugins-v" + VERSION
PACKAGE = "universal-agent-plugins"
CONTRACT_SHA256 = "778f8206c6cd04c614f7a8b44dc2df4d630d477ee143e83695e302b34f73f576"


def require(value, message):
    if not value:
        raise ValueError(message)


OUTPUT_LIMIT = 4 << 20
PROCESS_TIMEOUT = 300
DIAGNOSTIC_CODE_LIMIT = 16


def capture(command, *, cwd, env):
    """Drain both pipes concurrently with fixed memory budgets on every OS."""
    argv = [str(part) for part in command]
    streams = [bytearray(), bytearray()]
    overflow, read_error = threading.Event(), threading.Event()
    scripts = Path(__file__).resolve().parent
    job, live = None, None
    if os.name == "nt":
        live = run_path(str(scripts / "live-install-audit.py"))
        WindowsJob = run_path(str(scripts / "windows_owned_job.py"))["WindowsJob"]
        # The existing bootstrap blocks before spawning the verifier. Assign
        # its job before releasing the gate, so every descendant is owned.
        process = subprocess.Popen(
            [sys.executable, str(scripts / "live-install-audit.py"),
             "--windows-owned-file-command", json.dumps(argv)], cwd=cwd, env=env,
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            creationflags=subprocess.CREATE_NEW_PROCESS_GROUP)
    else:
        kill_group = run_path(str(scripts / "native_client_research.py"))["kill_group"]
        process = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)

    def drain(pipe, body):
        try:
            with pipe:
                while True:
                    chunk = pipe.read1(min(64 << 10, OUTPUT_LIMIT + 1 - len(body)))
                    if not chunk:
                        return
                    if len(body) + len(chunk) > OUTPUT_LIMIT:
                        overflow.set()
                        return
                    body.extend(chunk)
        except OSError:
            read_error.set()

    readers = [threading.Thread(target=drain, args=(pipe, body), daemon=True)
               for pipe, body in zip((process.stdout, process.stderr), streams)]
    deadline = time.monotonic() + PROCESS_TIMEOUT
    failure = None
    try:
        if live is not None:
            job = WindowsJob(process)
            process.stdin.write(b"!")
            process.stdin.close()
        for reader in readers:
            reader.start()
        while process.poll() is None or any(reader.is_alive() for reader in readers):
            if overflow.is_set():
                failure = "npm verification output exceeds bound"
                break
            if read_error.is_set():
                failure = "npm verification output capture failed"
                break
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                failure = "npm verification subprocess timed out"
                break
            overflow.wait(min(0.01, remaining))
    finally:
        cleanup_failure = None
        try:
            if live is not None:
                if not process.stdin.closed:
                    process.stdin.close()
                if job is not None:
                    leader_done = process.poll() is not None
                    retained = live["_windows_file_close"](job, time.monotonic() + 5)
                    if leader_done and retained and failure is None:
                        cleanup_failure = "npm verification retained owned descendants"
                elif process.poll() is None:
                    process.kill()  # Unreleased bootstrap cannot have descendants.
            else:
                if process.poll() is not None and failure is None:
                    try:
                        os.killpg(process.pid, 0)
                    except ProcessLookupError:
                        pass
                    else:
                        cleanup_failure = "npm verification retained owned descendants"
                kill_group(process)
        except Exception:
            cleanup_failure = "npm verification owned process cleanup failed"
        finally:
            process.wait(timeout=5)
            for reader, pipe in zip(readers, (process.stdout, process.stderr)):
                if reader.ident is not None:
                    reader.join(timeout=5)
                else:
                    pipe.close()
            if any(reader.is_alive() for reader in readers):
                cleanup_failure = "npm verification output readers did not stop"
        require(cleanup_failure is None, cleanup_failure)
    require(not failure, failure)
    require(not overflow.is_set(), "npm verification output exceeds bound")
    require(not read_error.is_set(), "npm verification output capture failed")
    return process.returncode, bytes(streams[0]), bytes(streams[1])


def run(command, *, cwd, env):
    returncode, stdout, stderr = capture(command, cwd=cwd, env=env)
    if returncode:
        # Only a bounded set of diagnostic codes crosses into receipts. Never
        # forward npm's raw output, URLs, paths, config or token-bearing text.
        codes = set()
        for match in re.finditer(rb"(?m)^npm (?:error|ERR!) code ([A-Z][A-Z0-9_]{1,63})\s*$", stderr):
            codes.add(match.group(1).decode("ascii"))
            if len(codes) == DIAGNOSTIC_CODE_LIMIT:
                break
        details = ",".join(sorted(codes)) or "no-safe-code"
        raise ValueError(f"public npm subprocess failed: exit={returncode}; codes={details}")
    return stdout


def tools():
    node = shutil.which("node")
    require(node, "Node prerequisite unavailable")
    node = Path(node).resolve(strict=True)
    configured = os.environ.get("SOURCE_AUDIT_NPM_CLI")
    if configured is not None:
        npm = Path(configured)
        require(npm.is_absolute() and npm.name == "npm-cli.js",
                "SOURCE_AUDIT_NPM_CLI must be an absolute npm-cli.js")
        try:
            info = npm.lstat()
        except OSError as error:
            raise ValueError("SOURCE_AUDIT_NPM_CLI is unavailable") from error
        require(stat.S_ISREG(info.st_mode) and not npm.is_symlink(),
                "SOURCE_AUDIT_NPM_CLI must be a regular non-link file")
        # The workflow supplies the exact isolated npm12 installation. Never
        # fall back to an older setup-node shim when an explicit input is bad.
        return node, npm.resolve(strict=True)
    candidates = [node.parent / "node_modules/npm/bin/npm-cli.js",
                  node.parent.parent / "lib/node_modules/npm/bin/npm-cli.js"]
    npm = shutil.which("npm")
    if npm:
        shim = Path(npm).resolve(strict=True)
        candidates.insert(0, shim.parent / "node_modules/npm/bin/npm-cli.js")
        if os.name != "nt":
            candidates.insert(0, shim)
    npm = next((p.resolve(strict=True) for p in candidates if p.is_file() and p.name == "npm-cli.js"), None)
    require(npm is not None, "real npm-cli.js prerequisite unavailable")
    return node, npm


def npm_environment(audit, root):
    home, data = root / "home", root / "state"
    home.mkdir()
    data.mkdir()
    env = audit.live.isolated_client_env(home, root, data)
    cache = root / "npm-cache"
    cache.mkdir()
    user, global_config = root / "npm-user-config", root / "npm-global-config"
    user.write_text("registry=https://registry.npmjs.org/\n", encoding="utf-8")
    global_config.write_text("", encoding="utf-8")
    # isolated_client_env uses a runtime allowlist; still remove every npm or
    # launcher override before constructing the exact public acquisition env.
    env = {k: v for k, v in env.items() if not k.lower().startswith("npm_config_")
           and not k.startswith("AGENTPLUGINS_INTERNAL_PROOF_")
           and k not in ("NODE_OPTIONS", "NODE_PATH", "NPM_TOKEN", "NODE_AUTH_TOKEN", "GH_TOKEN", "GITHUB_TOKEN")}
    env.update({"NPM_CONFIG_CACHE": str(cache), "NPM_CONFIG_USERCONFIG": str(user),
                "NPM_CONFIG_GLOBALCONFIG": str(global_config),
                "NPM_CONFIG_REGISTRY": "https://registry.npmjs.org/",
                "NPM_CONFIG_IGNORE_SCRIPTS": "true",
                "AGENTPLUGINS_CACHE_DIR": str(root / "binary-cache")})
    return env


def provision(audit, directory, target, release_context):
    """Install and authenticate exact public npm bytes, then execute its wrapper.

    release_context is the result of the existing public binary verifier, never
    an alternate binary source. Returned env contains only npm-specific config;
    callers keep their own HOME/state/temp and use a case-owned binary cache.
    """
    proof = release_context["proof"]
    require(proof.get("acquisition") == "public GitHub release download"
            and proof.get("tag") == TAG and proof.get("commit") == COMMIT
            and proof.get("version") == VERSION, "npm requires verified exact public release")
    assets = directory / "release-assets"
    node, npm = tools()
    contract = audit.ROOT / "npm/agentplugins/scripts/npm-public-contract.js"
    require(contract.is_file() and not contract.is_symlink()
            and hashlib.sha256(contract.read_bytes()).hexdigest() == CONTRACT_SHA256,
            "public npm contract differs from frozen release")
    root = directory / "public-npm"
    root.mkdir()
    env = npm_environment(audit, root)
    env["PATH"] = str(node.parent) + os.pathsep + env.get("PATH", "")
    require(run([node, npm, "--version"], cwd=root, env=env).decode().strip() == "12.0.2",
            "public npm verification requires npm 12.0.2")
    project, packed = root / "project", root / "packed"
    project.mkdir()
    packed.mkdir()
    (project / "package.json").write_text('{"private":true}', encoding="utf-8")
    retained = {}

    def capture(name, command):
        body = run(command, cwd=project, env=env)
        path = root / (name + ".json")
        path.write_bytes(body)
        retained[name] = path
        return json.loads(body)

    metadata = capture("metadata", [node, npm, "view", PACKAGE + "@" + VERSION, "--json"])
    if isinstance(metadata, list) and len(metadata) == 1:
        metadata = metadata[0]
    require(isinstance(metadata, dict) and isinstance(metadata.get("dist"), dict), "npm metadata is not one exact package")
    integrity, shasum = metadata["dist"].get("integrity"), metadata["dist"].get("shasum")
    require(isinstance(integrity, str) and isinstance(shasum, str), "npm integrity unavailable")
    run([node, contract, "metadata", retained["metadata"], VERSION, integrity, shasum], cwd=project, env=env)
    capture("pack", [node, npm, "pack", "--ignore-scripts", "--json", "--pack-destination", packed,
                     PACKAGE + "@" + VERSION])
    run([node, contract, "download", retained["pack"], packed, VERSION, integrity, shasum], cwd=project, env=env)
    packed_record = json.loads(retained["pack"].read_bytes())
    # The frozen verifier accepts an exact package-named npm12 record as
    # well as the earlier singleton array. Keep its captured input unchanged.
    if isinstance(packed_record, dict) and set(packed_record) == {PACKAGE}:
        packed_record = packed_record[PACKAGE]
    elif isinstance(packed_record, list) and len(packed_record) == 1:
        packed_record = packed_record[0]
    else:
        raise ValueError("npm pack inventory differs")
    require(isinstance(packed_record, dict), "npm pack inventory differs")
    filename = packed_record.get("filename")
    require(filename == PACKAGE + "-" + VERSION + ".tgz", "npm pack filename differs")
    tarball = packed / filename
    verified = json.loads(run([node, audit.ROOT / "npm/agentplugins/scripts/release-assets.js", "verify",
                              assets, TAG, COMMIT], cwd=project, env=env))
    package = audit.native.draft.inspect_package(tarball, verified)
    # Registry install preserves the registry resolved URL in package-lock;
    # audit signatures then authenticates that exact public package, not a file URL.
    run([node, npm, "install", "--ignore-scripts", "--no-audit", "--no-fund", "--save-exact",
         PACKAGE + "@" + VERSION], cwd=project, env=env)
    audit_result = capture("audit", [node, npm, "audit", "signatures", "--json", "--include-attestations"])
    records = audit_result.get("verified", [])
    require(isinstance(records, list) and len(records) == 1 and isinstance(records[0], dict),
            "npm cryptographic verification inventory differs")
    attestation = root / "attestation.json"
    attestation.write_text(json.dumps({"attestations": records[0].get("attestationBundles")}), encoding="utf-8")
    run([node, contract, "audit", retained["audit"], attestation, VERSION], cwd=project, env=env)
    run([node, contract, "attestation", attestation, VERSION, integrity, TAG, COMMIT], cwd=project, env=env)
    installed = project / "node_modules" / PACKAGE
    actual = {}
    for path in installed.rglob("*"):
        require(not path.is_symlink(), "installed npm package contains a link")
        if path.is_file():
            actual["package/" + path.relative_to(installed).as_posix()] = path.read_bytes()
        else:
            require(path.is_dir(), "installed npm package contains a special file")
    require(actual == package, "installed npm bytes differ from verified public tarball")
    launcher = installed / "bin/agentplugins.js"
    command = [str(node), str(launcher)]
    require(run([*command, "version"], cwd=project, env=env).decode().strip() == "agentplugins " + VERSION,
            "public npm wrapper version differs")
    selected = verified["assets"][target]
    binary = root / "binary-cache" / VERSION / target / ("agentplugins.exe" if target.startswith("windows") else "agentplugins")
    require(binary.is_file() and not binary.is_symlink() and binary.stat().st_size == selected["size"]
            and hashlib.sha256(binary.read_bytes()).hexdigest() == selected["sha256"], "public npm downloaded binary differs")
    return {"command": command,
            "env": {},
            "proof": {"acquisition": "public npm registry; verified npm Sigstore provenance",
                      "package": PACKAGE, "version": VERSION, "tag": TAG, "commit": COMMIT,
                      "npm_version": "12.0.2", "integrity": integrity, "shasum": shasum,
                      "tarball_sha256": hashlib.sha256(tarball.read_bytes()).hexdigest(),
                      "launcher_sha256": hashlib.sha256(launcher.read_bytes()).hexdigest(),
                      "binary_sha256": selected["sha256"], "contract_sha256": CONTRACT_SHA256,
                      "verified_attestation_bundles": records[0]["attestationBundles"],
                      "command": command, "npm_ignore_scripts": True,
                      "public_cold_wrapper": True, "lifecycle_must_use_wrapper": True}}
