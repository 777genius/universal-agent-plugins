"""Public npm acquisition; lifecycle callers must execute the returned JS prefix.

Reuse the released public-contract verifier and bounded package inspector. No
proof-mode binary injection, product rebuild, npm hooks, or ambient npm config.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

VERSION = "0.1.116"
COMMIT = "4858d973afcf6cf31b49bba98bdf06482dce6226"
TAG = "agentplugins-v" + VERSION
PACKAGE = "universal-agent-plugins"
CONTRACT_SHA256 = "778f8206c6cd04c614f7a8b44dc2df4d630d477ee143e83695e302b34f73f576"


def require(value, message):
    if not value:
        raise ValueError(message)


def run(command, *, cwd, env):
    result = subprocess.run([str(part) for part in command], cwd=cwd, env=env,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=300)
    if result.returncode:
        # Only diagnostic codes cross into receipts. npm stderr can echo a
        # remote URL, local paths or config; never forward its raw contents.
        import re
        codes = re.findall(rb"(?m)^npm (?:error|ERR!) code ([A-Z][A-Z0-9_]{1,63})\s*$", result.stderr)
        details = ",".join(sorted({code.decode("ascii") for code in codes})) or "no-safe-code"
        raise ValueError(f"public npm subprocess failed: exit={result.returncode}; codes={details}")
    require(len(result.stdout) <= 4 << 20, "npm verification output exceeds bound")
    return result.stdout


def tools():
    node = shutil.which("node")
    require(node, "Node prerequisite unavailable")
    node = Path(node).resolve(strict=True)
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
    require(isinstance(metadata, dict) and isinstance(metadata.get("dist"), dict), "npm metadata is not one exact package")
    integrity, shasum = metadata["dist"].get("integrity"), metadata["dist"].get("shasum")
    require(isinstance(integrity, str) and isinstance(shasum, str), "npm integrity unavailable")
    run([node, contract, "metadata", retained["metadata"], VERSION, integrity, shasum], cwd=project, env=env)
    capture("pack", [node, npm, "pack", "--ignore-scripts", "--json", "--pack-destination", packed,
                     PACKAGE + "@" + VERSION])
    run([node, contract, "download", retained["pack"], packed, VERSION, integrity, shasum], cwd=project, env=env)
    packed_record = json.loads(retained["pack"].read_bytes())
    require(isinstance(packed_record, list) and len(packed_record) == 1, "npm pack inventory differs")
    filename = packed_record[0]["filename"]
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
