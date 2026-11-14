#!/usr/bin/env python3
"""Exact public-CLI source lifecycle evidence; never a signed target grant.

The receipt qualifies only its source/digests, client, scope, platform and tested
release. A consumer must separately check current active policy and denials.
Native activation, project trust, MCP runtime and account access are not proved.
"""
import argparse
import base64
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path, PurePosixPath
import stat
import re
import subprocess
import tarfile
import tempfile
import tomllib
import urllib.request
import zipfile
from datetime import datetime, timezone

import yaml

ROOT = Path(__file__).resolve().parents[1]
RELEASE_TAG = "agentplugins-v0.1.108"
RELEASE_COMMIT = "8c3cfa28e332c533b0cc0845739c0275730a4285"
REPOSITORY = "777genius/universal-agent-plugins"
LEDGER = "617ebdec67a58af3d265cae49e90c43497299db0"
ORIGIN = f"https://raw.githubusercontent.com/777genius/universal-agent-plugins-registry/{LEDGER}/registry/schemas/1/"
SNAPSHOT_DIGEST = "6ab80575a51378f6643b387a8eefe332e84493bcad86183370c46801243ab277"
SNAPSHOT_SOURCE = "121633366dc5255aaf638845fb7cb0cb50013340"
CLIENTS = ("hermes", "antigravity", "pi")
MAX_NATIVE_ARCHIVE_BYTES = 200 << 20
# Digest-pinned AGY 1.5.3 payloads are 246837648 (Linux), 208900592 (macOS),
# and 225159320 (Windows) bytes. Their compressed archives are all below 200 MiB.
MAX_NATIVE_EXECUTABLE_BYTES = 256 << 20
NPM_PACKAGE = "universal-agent-plugins"
NPM_VERSION = "0.1.108"
NPM_PREFIX = ["npx", "-y", f"{NPM_PACKAGE}@{NPM_VERSION}"]
NPM_TARBALL = f"https://registry.npmjs.org/{NPM_PACKAGE}/-/{NPM_PACKAGE}-{NPM_VERSION}.tgz"
NPM_INTEGRITY = "sha512-vpRRhLzGdQIaPOL7Ihf4abitwLyOmf7wUgz6W4j0WAlAQnwRZqFUvrQCDyMvoMiythdfTPulu6MSC1YUrcpv9g=="
MAX_NPM_BYTES = 1 << 20


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, ROOT / "scripts" / filename)
    loaded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(loaded)
    return loaded


live = module("source_live_audit", "live-install-audit.py")
native = module("source_native_matrix", "run-native-client-matrix.py")


def require(value, message):
    if not value:
        raise ValueError(message)


def digest(body):
    return "sha256:" + hashlib.sha256(body).hexdigest()


def authenticated_snapshot(directory):
    """Use the production reader through its existing bootstrap verifier."""
    paths = {}
    for kind, suffix, limit in (("snapshot", ".json", 4 << 20), ("envelope", ".envelope.json", 16 << 10)):
        path = directory / (kind + ".json")
        with urllib.request.urlopen(ORIGIN + "snapshots/00000000000000000047" + suffix, timeout=45) as response:
            body = response.read(limit + 1)
        require(len(body) <= limit, "oversize signed input")
        path.write_bytes(body)
        paths[kind] = path
    body = paths["snapshot"].read_bytes()
    require(digest(body) == "sha256:" + SNAPSHOT_DIGEST, "signed source snapshot changed")
    subprocess.run([
        "go", "run", "./cmd/agentplugins/bootstrapgen",
        "-snapshot", str(paths["snapshot"]), "-envelope", str(paths["envelope"]),
        "-trust", str(ROOT / "cli/cmd/agentplugins/directory_bootstrap_inputs/trusted-keys.json"),
        "-expected-key-id", "uap-directory-2026-01",
        "-expected-public-key", "HalXARjat+v3ylTPLMAnvuavRo4ZfrF+DbWwsjlp2bI=",
        "-release-at", datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "-output", str(directory / "verified-bootstrap.go"),
    ], cwd=ROOT / "cli", check=True, timeout=180)
    snapshot = json.loads(body)
    require(snapshot["sequence"] == 47 and snapshot["source_commit"] == SNAPSHOT_SOURCE, "wrong signed provenance")
    return snapshot


def source_case(snapshot, distribution, sequence, client, scope, refusal=False):
    require(client in CLIENTS and scope in ("user", "project"), "unknown qualification tuple")
    rows = [row for row in snapshot["distributions"] if row["id"] == distribution]
    require(len(rows) == 1 and rows[0]["status"] == "active", "source distribution is not active")
    row = rows[0]
    releases = [release for release in row["releases"] if release["sequence"] == sequence]
    policies = [policy for policy in row["release_policies"] if policy["release_sequence"] == sequence]
    require(len(releases) == len(policies) == 1 and policies[0]["status"] == "active", "source release is not active")
    require(not any(item["distribution_id"] == distribution and item["release_sequence"] == sequence
                    for item in snapshot["revocations"]), "source release is revoked")
    # An exact Git alternative must never reinterpret a signed target or denial.
    require(not any(target["client"] == client for policy in row["release_policies"] for target in policy["targets"]), "signed target needs its normal policy route")
    require(not any(item.get("client") == client and item.get("distribution_id") == distribution
                    for item in snapshot["evidence"]), "existing target evidence needs policy review")
    release = releases[0]
    require(release["components"] and set(release["components"]) <= {"skills", "mcp"}, "nonportable mandatory component")
    require(not any(target.get("app_binding") for target in policies[0]["targets"]), "app-bound release cannot use source qualification")
    source = release["package_source"]
    require(re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", source["repository"]), "invalid repository")
    require(re.fullmatch(r"[0-9a-f]{40}", source["revision"]), "mutable source")
    require(source["path"] and all(re.fullmatch(r"[A-Za-z0-9_.-]+", part) and part not in (".", "..")
                                  for part in source["path"].split("/")), "unsafe source subdirectory")
    for key in ("tree_digest", "manifest_digest"):
        require(re.fullmatch(r"sha256:[0-9a-f]{64}", release[key]), "missing signed digest")
    return {"distribution": distribution, "release_sequence": sequence,
            "name": release["manifest_name"], "source": source,
            "tree_digest": release["tree_digest"], "manifest_digest": release["manifest_digest"],
            "components": release["components"], "client": client, "scope": scope,
            "selector": f'github:{source["repository"]}@{source["revision"]}//{source["path"]}',
            "expected": "dependency-refusal" if refusal else "materialized-lifecycle"}


def checked_ui_commands(value, case):
    """Bind real UI output to the independently authenticated release tuple."""
    selected, commands = value["selection"], value["commands"]
    expected_source = {**case["source"], "tree_sha256": case["tree_digest"],
                       "manifest_sha256": case["manifest_digest"]}
    require(selected == {"selector": case["selector"], "source": expected_source,
                         "components": case["components"], "client": case["client"],
                         "scope": case["scope"], "manifestName": case["name"]},
            "UI selected a different signed source, manifest name or placement")
    require(commands.get("saveMode") == "saved", "UI did not select saved lifecycle")
    for operation in ("add", "update", "repair", "remove"):
        operand = case["selector"] if operation == "add" else case["name"]
        expected = [*NPM_PREFIX, operation, operand, "--target", case["client"]]
        if case["scope"] == "project":
            expected += ["--scope", "project"]
        require(commands.get(operation, "").split(" ") == expected,
                "UI command changed exact source, manifest name, target, scope or invocation: " + operation)
    return value


def frontend_commands(snapshot_path, case):
    # The production helper owns both selection and command text. No copied
    # resolver, fixture index or independently reconstructed command is run.
    program = """
import { readFileSync } from 'node:fs';
import { parseDirectoryData } from './landing/utils/registry.ts';
import { pinnedCatalogSelection, pinnedCatalogCommands } from './landing/utils/projectCatalogSource.ts';
const [path, distribution, client, scope] = process.argv.slice(1);
const index = parseDirectoryData(JSON.parse(readFileSync(path, 'utf8')), 'published_snapshot');
const plugins = index.plugins.filter(plugin => plugin.distributions.some(row => row.id === distribution));
if (plugins.length !== 1) throw new Error('Expected one selected catalog product');
const selection = pinnedCatalogSelection(plugins[0], client, scope);
if (!selection) throw new Error('UI refuses this explicit source placement');
console.log(JSON.stringify({ selection, commands: pinnedCatalogCommands(plugins[0], client, scope) }));
"""
    output = subprocess.run(["node", "--experimental-strip-types", "--input-type=module", "--eval", program,
                             str(snapshot_path), case["distribution"], case["client"], case["scope"]],
                            cwd=ROOT, capture_output=True, check=True, timeout=45)
    require(len(output.stdout) <= 64 << 10, "oversize UI command output")
    return checked_ui_commands(json.loads(output.stdout), case)


def npm_reference(body):
    """Read pinned public bytes for comparison; npx remains the only installer."""
    require(len(body) <= MAX_NPM_BYTES, "oversize npm tarball")
    integrity = "sha512-" + base64.b64encode(hashlib.sha512(body).digest()).decode("ascii")
    require(integrity == NPM_INTEGRITY, "public npm tarball integrity changed")
    files, contents = {}, {}
    with tarfile.open(fileobj=io.BytesIO(body), mode="r:gz") as archive:
        members = archive.getmembers()
        require(0 < len(members) <= 32 and sum(item.size for item in members) <= MAX_NPM_BYTES,
                "oversize npm package contents")
        for item in members:
            parts = PurePosixPath(item.name).parts
            require(item.isfile() and len(parts) > 1 and parts[0] == "package" and item.name == PurePosixPath(item.name).as_posix() and
                    all(part not in (".", "..") for part in parts) and "\\" not in item.name,
                    "unsafe npm package member")
            name = "/".join(parts[1:])
            require(name not in files and 0 <= item.size <= MAX_NPM_BYTES, "duplicate or oversize npm member")
            content = archive.extractfile(item).read(MAX_NPM_BYTES + 1)
            require(len(content) == item.size, "npm member size changed")
            files[name] = {"sha256": digest(content), "size": len(content)}
            if name in ("package.json", "assets.json"):
                contents[name] = json.loads(content)
    package = contents["package.json"]
    require(package["name"] == NPM_PACKAGE and package["version"] == NPM_VERSION and
            package["bin"] == {"agentplugins": "bin/agentplugins.js"}, "unexpected public npm package")
    return {"integrity": integrity, "tarball_sha256": digest(body), "files": files,
            "manifest": contents["assets.json"]}


def acquire_npm_reference():
    with urllib.request.urlopen(NPM_TARBALL, timeout=45) as response:
        body = response.read(MAX_NPM_BYTES + 1)
    return npm_reference(body)


def checked_npm_installation(root, env, target, reference, installer):
    """Observe what the genuine npx launcher installed, without invoking it."""
    cache = in_case(Path(env["NPM_CONFIG_CACHE"]), root)
    packages = list(cache.glob("_npx/*/node_modules/universal-agent-plugins/package.json"))
    require(len(packages) == 1, "expected exactly one public npm package installation")
    package = in_case(packages[0].parent, root)
    observed = {}
    for path in package.rglob("*"):
        in_case(path, root)
        if path.is_dir():
            continue
        require(path.is_file() and path.stat().st_size <= MAX_NPM_BYTES, "unsafe installed npm member")
        with path.open("rb") as stream:
            body = stream.read(MAX_NPM_BYTES + 1)
        require(len(body) <= MAX_NPM_BYTES, "oversize installed npm member")
        observed[path.relative_to(package).as_posix()] = {"sha256": digest(body), "size": len(body)}
    require(observed == reference["files"], "installed npm contents differ from public integrity pin")
    lock_path = in_case(package.parents[1] / "package-lock.json", root)
    require(lock_path.is_file() and lock_path.stat().st_size <= MAX_NPM_BYTES, "npm lock missing or oversize")
    lock = json.loads(lock_path.read_text(encoding="utf-8"))["packages"]["node_modules/" + NPM_PACKAGE]
    require(lock.get("version") == NPM_VERSION and lock.get("integrity") == NPM_INTEGRITY and
            lock.get("resolved") == NPM_TARBALL, "npm lock refers to a different public package")
    manifest, asset = reference["manifest"], reference["manifest"]["assets"][target]
    require(manifest["version"] == NPM_VERSION and manifest["tag"] == RELEASE_TAG and
            manifest["producer"]["commit"] == installer["commit"] == RELEASE_COMMIT and
            manifest["producer"]["release_manifest"]["sha256"] == installer["manifest_sha256"] and
            asset == {"file": installer["file"], "sha256": installer["binary_sha256"], "size": installer["size"]},
            "npm embedded asset differs from attested public release")
    windows = target.startswith("windows-")
    binary_root = Path(env["LOCALAPPDATA"]) / "agentplugins/Cache" if windows else Path(env["XDG_CACHE_HOME"]) / "agentplugins"
    binary = in_case(binary_root / NPM_VERSION / target / ("agentplugins.exe" if windows else "agentplugins"), root)
    require(binary.is_file() and binary.stat().st_size == installer["size"], "npm bootstrap binary missing or wrong size")
    with binary.open("rb") as stream:
        body = stream.read(installer["size"] + 1)
    require(len(body) == installer["size"], "npm bootstrap binary size changed")
    actual = hashlib.sha256(body).hexdigest()
    require(actual == installer["binary_sha256"], "npm bootstrap binary differs from attested release")
    return {"package": NPM_PACKAGE, "version": NPM_VERSION, "integrity": reference["integrity"],
            "tarball_sha256": reference["tarball_sha256"], "verified_package_files": len(observed),
            "producer_repository": manifest["producer"]["repository"],
            "cache_binary": binary.relative_to(root).as_posix(), "binary_sha256": actual, "size": installer["size"]}


def npm_command(command, operation, env):
    argv = command.split(" ")
    require(argv[:4] == [*NPM_PREFIX, operation] and all(re.fullmatch(r"[A-Za-z0-9_.:/@-]+", word) for word in argv),
            "unsafe copied npm command")
    argv += ["--format", "json"]
    if operation in ("add", "update", "repair", "remove"):
        argv += ["--yes"]
    # Windows resolves the public npx.cmd through its ordinary command shell.
    # No shell syntax is accepted from the UI; paths remain in the process cwd.
    process = [env.get("COMSPEC", "cmd.exe"), "/d", "/s", "/c", " ".join(argv)] if os.name == "nt" else argv
    return argv, process


def state_file(root, scope):
    path = root / ("uap-data" if scope == "user" else "project/.uap") / "state-v2.json"
    require(live.path_inside_case(path, root), "managed state escaped case")
    return json.loads(path.read_text(encoding="utf-8")) if path.exists() else {"installations": []}


def protected_scope_snapshot(root, scope, native_root):
    """Observe the other native profile and its saved, mutable environment."""
    documents = root / ("project" if scope == "user" else "uap-data")
    state = documents / ".uap" if scope == "user" else documents
    files = [documents / "uap.toml", documents / "uap.lock"]
    files += [state / name for name in ("state-v2.json", "mutation.lock", "environment.lock")]
    directories = [native_root, *[state / name for name in
                   ("managed", "operations-v2", "plugin-data", "installer-tmp", "clients")]]
    for path in [*files, *directories]:
        in_case(path, root)
    # The content store, acquisition and signed-feed/security caches are shared.
    # They are deliberately absent from these scope-owned paths.
    observed = {"directories": live.scoped_snapshot(directories), "files": {}}
    for path in files:
        observed["files"][path.relative_to(root).as_posix()] = protected_saved_record(path)
    return observed


def read_saved_record(path):
    """Read owned manifest/lock/state bytes only after a held identity join."""
    try:
        before = path.lstat()
    except FileNotFoundError:
        return None, None
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
            and 0 <= before.st_size <= 1 << 20,
            "protected saved record must be a bounded single-link regular file")
    # Windows os.read must not translate CRLF before the exact size/digest join.
    flags = (os.O_RDONLY | getattr(os, "O_BINARY", 0) | getattr(os, "O_NOFOLLOW", 0)
             | getattr(os, "O_NONBLOCK", 0))
    fd = os.open(path, flags)
    try:
        held = os.fstat(fd)
        def same(left, right, *, compare_ctime=True):
            return (stat.S_ISREG(right.st_mode) and right.st_nlink == 1
                    and os.path.samestat(left, right) and right.st_mode == left.st_mode
                    and right.st_size == left.st_size and right.st_mtime_ns == left.st_mtime_ns
                    and (not compare_ctime or right.st_ctime_ns == left.st_ctime_ns))
        # CPython on Windows can expose birth time through lstat and change time
        # through fstat (python/cpython#157671). Compare ctime within each API.
        require(same(before, held, compare_ctime=False), "protected saved record changed before read")
        body = bytearray()
        while len(body) <= before.st_size:
            chunk = os.read(fd, min(65536, before.st_size + 1 - len(body)))
            if not chunk:
                break
            body.extend(chunk)
        require(len(body) == before.st_size
                and same(held, os.fstat(fd)) and same(before, path.lstat()),
                "protected saved record changed during read")
        return bytes(body), before
    finally:
        os.close(fd)


def protected_saved_record(path):
    body, before = read_saved_record(path)
    if before is None:
        return None
    return {"digest": digest(body), "mtime_ns": before.st_mtime_ns,
            "inode": before.st_ino, "mode": before.st_mode}


def checked_saved_removal(documents, alias, case, root):
    bodies = []
    for path in documents:
        in_case(path, root)
        body, _ = read_saved_record(path)
        require(body is not None, "saved removal lost the manifest or lock")
        bodies.append(body)
    manifest = tomllib.loads(bodies[0].decode("utf-8"))
    locked = json.loads(bodies[1].decode("utf-8"))
    dependencies, targets = manifest.get("dependencies", {}), manifest.get("targets", {})
    require(manifest.get("schema_version") == 1 and isinstance(dependencies, dict)
            and isinstance(targets, dict), "saved removal produced an invalid manifest")
    require(alias not in dependencies, "remove retained a saved dependency")
    require(all(isinstance(aliases, list) and alias not in aliases for aliases in targets.values()),
            "remove retained a saved target assignment")
    packages = locked.get("packages")
    require(locked.get("lock_version") == 1 and isinstance(packages, list), "saved removal produced an invalid lock")
    source_key = f'github:{case["source"]["repository"].lower()}:{case["source"]["path"]}'
    require(all(isinstance(package, dict) and package.get("alias") != alias
                and package.get("source_key") != source_key for package in packages),
            "remove retained a locked package")


def checked_saved_intent(documents, alias, case, root):
    """Observe saved immutable intent, independently of reported managed state."""
    require(alias == case["name"], "saved alias differs from release manifest name")
    bodies = []
    for path in documents:
        body, _ = read_saved_record(in_case(path, root))
        require(body is not None, "saved manifest or lock is absent")
        bodies.append(body)
    manifest = tomllib.loads(bodies[0].decode("utf-8"))
    locked = json.loads(bodies[1], object_pairs_hook=live._unique_json_object)
    source = case["source"]
    key = f'github:{source["repository"].lower()}:{source["path"]}'
    selector = {"kind": "commit", "value": source["revision"]}
    # environment_session.go writes the canonical GitHub HTTPS endpoint.
    expected = {"schema_version": 1, "dependencies": {alias: {
        "git": f'https://github.com/{source["repository"]}.git',
        "path": source["path"], "commit": source["revision"]}},
        "targets": {case["client"]: [alias]}}
    require(type(manifest.get("schema_version")) is int and manifest == expected,
            "saved manifest differs from exact source, commit or target intent")
    # This is the existing environment.ManifestDigest wire, for this one root.
    intent = {"schema_version": 1, "dependencies": [{"alias": alias, "source_key": key,
              "path": source["path"], "selector": selector}],
              "targets": [{"id": case["client"], "aliases": [alias]}]}
    intent_digest = digest(json.dumps(intent, separators=(",", ":")).encode())
    require(isinstance(locked, dict) and type(locked.get("lock_version")) is int
            and locked["lock_version"] == 1 and locked.get("manifest_digest") == intent_digest,
            "saved lock does not bind the exact manifest intent")
    packages = locked.get("packages")
    require(isinstance(packages, list) and len(packages) == 1 and isinstance(packages[0], dict),
            "saved lock must contain exactly the requested root package")
    package = packages[0]
    expected_source = {"git": {"repository": source["repository"], "path": source["path"],
                              "requested": selector, "commit": source["revision"]}}
    require(package.get("source") == expected_source, "saved lock lost the immutable requested Git source")
    for field, value in {"alias": alias, "targets": [case["client"]], "source_key": key,
                         "digest_algorithm": "agentplugins-tree-sha256-v1",
                         "tree_digest": case["tree_digest"], "manifest_digest": case["manifest_digest"]}.items():
        require(package.get(field) == value, "saved lock has incorrect " + field)
    require(not package.get("required_by"), "saved root unexpectedly became a dependency")
    return {"manifest_sha256": digest(bodies[0]), "lock_sha256": digest(bodies[1]),
            "intent_digest": intent_digest, "alias": alias, "source_key": key}


def checked_native_materialization(selected_root, locator, case, binding, root):
    """Read the owned native Notion MCP entry; file changes alone are no proof."""
    pin = live.MARKETING_KIT_SIGNED_CLOSURE["777genius/notion"]
    require(case["distribution"] == "777genius/notion" and case["name"] == "notion"
            and case["release_sequence"] == pin["release_sequence"] and case["components"] == ["mcp"]
            and case["source"] == {"repository": pin["repository"], "revision": pin["resolved_revision"],
                                   "path": pin["package_path"]}
            and all(case[field] == pin[field] for field in ("tree_digest", "manifest_digest")),
            "native content oracle requires the reviewed immutable Notion source")
    client, scope = case["client"], case["scope"]
    require(client in CLIENTS and scope in ("user", "project")
            and (client != "hermes" or scope == "user"), "native MCP scope has no reviewed oracle")
    require(binding.get("client_id") == client and binding.get("scope") == scope
            and Path(binding["target_locator"]) == locator, "native proof differs from the selected binding")
    in_case(selected_root, root)
    in_case(locator, root)
    objects = binding.get("native_objects", [])
    require(isinstance(objects, list) and all(isinstance(item, dict) for item in objects),
            "native ownership is malformed")
    packages = [item for item in objects if item.get("kind") == "managed_package_directory"]
    members = [item for item in objects if item.get("kind") != "managed_package_directory"]
    physical = binding.get("physical_artifact_id")
    require(isinstance(physical, str) and physical and len(packages) == 1
            and packages[0].get("object_id") == f"package:{client}:{physical}"
            and packages[0].get("logical_name") == "notion" and Path(packages[0].get("path", "")) == locator
            and packages[0].get("protection_class") == "managed" and not packages[0].get("user_modified")
            and re.fullmatch(r"sha256:[0-9a-f]{64}", packages[0].get("managed_digest", "")),
            "managed package directory ownership differs from the selected binding")
    if client == "antigravity":
        # AGY owns the entire managed plugin, not a separate global MCP entry.
        require(not members and locator.parent == selected_root / "plugins", "native plugin is in another scope or has foreign ownership")
        manifest_body, _ = read_saved_record(in_case(locator / "plugin.json", root))
        require(manifest_body is not None, "native plugin manifest is absent")
        manifest = json.loads(manifest_body, object_pairs_hook=live._unique_json_object)
        require(isinstance(manifest, dict) and manifest.get("name") == "notion"
                and manifest.get("$schema") == "https://antigravity.google/schemas/v1/plugin.json"
                and manifest.get("version") == pin["package_version"]
                and set(manifest) <= {"$schema", "name", "displayName", "version", "description"},
                "native plugin manifest does not load the reviewed default MCP config")
        path, collection, entry = locator / "mcp_config.json", "mcpServers", {"serverUrl": "https://mcp.notion.com/mcp"}
    else:
        path = selected_root / ("config.yaml" if client == "hermes" else "mcp.json")
        collection, entry = ("mcp_servers" if client == "hermes" else "mcpServers"), {"url": "https://mcp.notion.com/mcp"}
        kind = "hermes_user_mcp" if client == "hermes" else f"pi_{scope}_mcp_server"
        require(len(members) == 1 and members[0].get("kind") == kind
                and members[0].get("logical_name") == "notion" and members[0].get("object_id") == kind + ":notion"
                and Path(members[0].get("path", "")) == path and members[0].get("protection_class") == "managed",
                "native MCP entry is not owned at the selected scope and path")
    body, _ = read_saved_record(in_case(path, root))
    require(body is not None, "selected native MCP config is absent")
    value = yaml.safe_load(body) if client == "hermes" else json.loads(body, object_pairs_hook=live._unique_json_object)
    require(isinstance(value, dict) and isinstance(value.get(collection), dict)
            and value[collection].get("notion") == entry, "owned native MCP entry differs from the reviewed transport and URL")
    return {"path": path.relative_to(root).as_posix(), "logical_name": "notion",
            "entry": entry, "config_sha256": digest(body)}


def checked_binding(state, case):
    installations = state.get("installations", [])
    require(len(installations) == 1, "expected one saved installation")
    installation = installations[0]
    require(installation["declared_name"] == case["name"] and installation.get("origin_mode") == "direct", "wrong direct source identity")
    source = installation["source"]
    require(source.get("resolved_revision") == case["source"]["revision"], "wrong immutable source revision")
    require(source.get("repository", "").removesuffix(".git").removeprefix("https://github.com/").casefold() == case["source"]["repository"].casefold(), "wrong source repository")
    require(source.get("package_subpath") == case["source"]["path"], "wrong source subdirectory")
    bindings = list(installation.get("clients", {}).values())
    require(len(bindings) == 1, "unexpected sibling target")
    binding = bindings[0]
    require(binding["client_id"] == case["client"] and binding["scope"] == case["scope"], "wrong saved placement")
    require(binding["materialization"] == "materialized", "native package not materialized")
    revision = binding.get("package_revision", {})
    for field in ("tree_digest", "manifest_digest"):
        require(revision.get(field) == case[field], "materialized package differs from signed source digest")
    require(revision.get("resolved_revision") == case["source"]["revision"], "binding revision changed")
    receipts = state.get("environment_receipts", [])
    owned = [receipt for receipt in receipts if receipt["installation_id"] == installation["installation_id"]
             and receipt["client_id"] == case["client"] and receipt["client_binding_id"] == binding["client_binding_id"]]
    require(len(owned) == 1 and owned[0].get("phase") == "committed", "saved environment ownership is missing")
    owner = owned[0]
    require(owner.get("source_key") == f'github:{case["source"]["repository"].lower()}:{case["source"]["path"]}'
            and owner.get("package_digest") == case["tree_digest"], "saved owner belongs to another source")
    require(owner.get("environment_id") == case["scope"] + "-default"
            and owner.get("alias") and owner.get("receipt_id"), "saved owner has wrong environment or incomplete identity")
    return installation, binding, owned[0]


def read_native_executable(stream, size, magic):
    require(0 < size <= MAX_NATIVE_EXECUTABLE_BYTES, "native executable exceeds payload size bound")
    body = stream.read(MAX_NATIVE_EXECUTABLE_BYTES + 1)
    require(len(body) == size, "native executable size differs from archive metadata")
    require(any(body.startswith(prefix) for prefix in magic), "native executable has invalid native magic")
    return body


def extract_antigravity_executable(body, archive):
    # Select exactly one regular executable. Archive paths never become
    # filesystem paths, and symlinks are never extracted or executed.
    if archive.endswith(".zip"):
        with zipfile.ZipFile(io.BytesIO(body)) as package:
            candidates = [item for item in package.infolist() if not item.is_dir()
                          and item.filename.lower().endswith(".exe")
                          and (item.external_attr >> 16) & 0o170000 in (0, 0o100000)]
            require(len(candidates) == 1, "native archive does not contain exactly one executable")
            item = candidates[0]
            with package.open(item) as stream:
                return read_native_executable(stream, item.file_size, (b"MZ",))
    magic = (b"\x7fELF", b"\xcf\xfa\xed\xfe", b"\xfe\xed\xfa\xcf", b"\xca\xfe\xba\xbe")
    with tarfile.open(fileobj=io.BytesIO(body), mode="r:gz") as package:
        candidates = []
        for item in package.getmembers():
            if item.isfile():
                with package.extractfile(item) as stream:
                    if stream.read(4) in magic:
                        candidates.append(item)
        require(len(candidates) == 1, "native archive does not contain exactly one executable")
        item = candidates[0]
        with package.extractfile(item) as stream:
            return read_native_executable(stream, item.size, magic)


def provision_native(client, target, directory, selected):
    if client == "hermes":
        require(selected and selected.is_file(), "real Hermes bootstrap executable is required")
        return selected.absolute(), {"commit": "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19", "acquisition": "workflow pinned official bootstrap", "binary_sha256": hashlib.sha256(selected.read_bytes()).hexdigest()}
    if client == "pi":
        return module("source_pi", "pi_project_native_e2e.py").checked_release(directory, target, dict(os.environ))
    proof = module("source_released_native", "e2e/released-native-proof.py")
    archive = {"linux-amd64": "agy_cli_linux_x64.tar.gz", "darwin-arm64": "agy_cli_mac_arm64.tar.gz", "windows-amd64": "agy_cli_windows_x64.zip"}[target]
    url = "https://github.com/google-antigravity/antigravity-cli/releases/download/1.5.3/" + archive
    with urllib.request.urlopen(url, timeout=90) as response:
        body = response.read(MAX_NATIVE_ARCHIVE_BYTES + 1)
    require(len(body) <= MAX_NATIVE_ARCHIVE_BYTES, "native archive exceeds bound")
    native.verify_digest(body, "sha256:" + proof.AGY_PINS[target])
    executable = extract_antigravity_executable(body, archive)
    path = directory / ("agy.exe" if target == "windows-amd64" else "agy")
    path.write_bytes(executable)
    path.chmod(0o700)
    return path, {"tag": "1.5.3", "archive": archive, "sha256": proof.AGY_PINS[target], "binary_sha256": hashlib.sha256(executable).hexdigest()}


def in_case(path, root):
    require(live.path_inside_case(path, root), "native path escaped disposable case")
    return path


def checked_dependency_refusal(output, case):
    require(type(output.get("exit_code")) is int and output["exit_code"] != 0
            and not output.get("timeout"), "mandatory closure was not refused")
    require(isinstance(output.get("stderr_full"), str) and isinstance(output.get("stdout_full"), str),
            "bounded complete refusal output is required")
    require(case["distribution"] == "777genius/marketing-kit" and case["name"] == "marketing-kit",
            "no reviewed dependency refusal for this source")
    # Reuse the existing immutable Marketing Kit closure. A root failure,
    # transport failure, or a different dependency/version is not this proof.
    diagnostic = output["stderr_full"].strip()
    for distribution, pin in live.MARKETING_KIT_SIGNED_CLOSURE.items():
        if distribution == case["distribution"]:
            continue
        expected = (f"agentplugins: dependency conflict for directory:{distribution}: "
                    f'marketing-kit version {pin["package_version"]}')
        if diagnostic == expected and not output["stdout_full"].strip():
            return {"code": "dependency_conflict", "distribution": distribution,
                    "version": pin["package_version"], "message": diagnostic}
    raise ValueError("refusal did not identify the exact mandatory dependency conflict")


def checked_native_cleanup(selected_root, case, binding, root):
    in_case(selected_root, root)
    names = []
    for item in binding.get("native_objects", []):
        if item.get("path"):
            in_case(Path(item["path"]), root)
        if item.get("logical_name"):
            names.append(item["logical_name"])
    # Observe all remaining files, including config created by add/remove;
    # looking only at the original paths can miss a newly retained MCP entry.
    residuals = live.native_identity_residuals(
        [selected_root], case["name"], live.snapshot([selected_root]), case["client"],
        component_names=names)
    require(not residuals, "owned native entries remain after remove: " + ", ".join(residuals))
    return {"owned_logical_names": sorted(set(names)), "remaining": residuals}


def run_case(binary, case, root, executable, npm=None):
    root.mkdir()
    home, project, data = (root / name for name in ("home", "project", "uap-data"))
    for path in (home, project, data):
        path.mkdir()
    env = live.isolated_client_env(home, root, data)
    if npm:
        require(not (root / "npm-cache").exists(), "public npm case must start cold")
        for name in ("user.npmrc", "global.npmrc"):
            (root / name).write_text("", encoding="utf-8")
        env.update({"NPM_CONFIG_CACHE": str(root / "npm-cache"), "NPM_CONFIG_REGISTRY": "https://registry.npmjs.org/",
                    "NPM_CONFIG_USERCONFIG": str(root / "user.npmrc"), "NPM_CONFIG_GLOBALCONFIG": str(root / "global.npmrc"),
                    "NPM_CONFIG_UPDATE_NOTIFIER": "false"})
    env["HERMES_HOME"] = str(home / ".hermes")
    if executable:
        env["PATH"] = str(executable.parent) + os.pathsep + env.get("PATH", "")
    native_root = home / {"hermes": ".hermes", "antigravity": ".gemini/config", "pi": ".pi/agent"}[case["client"]]
    native_root.mkdir(parents=True)
    project_native = project / {"hermes": ".hermes", "antigravity": ".agents", "pi": ".pi"}[case["client"]]
    project_native.mkdir()
    sentinels = [home / "foreign.txt", project / "foreign.txt", native_root / "foreign.txt", project_native / "foreign.txt"]
    for path in sentinels:
        path.write_text("unmanaged source audit neighbor\n", encoding="utf-8")
    live.initialize_project_skill_git_root(project, env, root)
    scope = ["--scope", case["scope"]]
    if case["scope"] == "project":
        scope += ["--project-root", str(project)]
    trace = []
    saved_root = data if case["scope"] == "user" else project
    documents = [saved_root / "uap.toml", saved_root / "uap.lock"]

    def run(operation, selector=None, success=True):
        copied = None
        if npm:
            copied = case["ui"]["commands"].get(operation)
            if copied is None:
                # info/list are audit observations, not advertised UI commands.
                copied = " ".join([*NPM_PREFIX, operation, *([selector] if selector else []), "--target", case["client"],
                                   *(["--scope", "project"] if case["scope"] == "project" else [])])
            argv, process = npm_command(copied, operation, env)
        else:
            argv = [str(binary), operation, *([selector] if selector else []), "--target", case["client"], *scope, "--format", "json"]
            if operation in ("add", "update", "repair", "remove"):
                argv += ["--yes"]
            process = argv
        for path in (root, home, project, data, native_root, project_native):
            in_case(path, root)
        result = live.call(process, env, project, 180, capture_files=True, capture_full=not success)
        parsed = result.pop("_json", None)
        trace.append({"operation": operation, "argv": argv, "process_argv": process,
                      "copied_ui_command": copied if operation in ("add", "update", "repair", "remove") else None, **result})
        for path in sentinels:
            require(live.preserved_regular_text(path, "unmanaged source audit neighbor\n", root), "foreign state changed")
        require(protected_scope_snapshot(root, case["scope"], protected_root) == protected,
                operation + " changed the other scope")
        if success:
            require(result["exit_code"] == 0 and not result.get("timeout") and isinstance(parsed, dict), operation + " did not complete")
        return parsed, result

    selected_root = native_root if case["scope"] == "user" else project_native
    protected_root = project_native if case["scope"] == "user" else native_root
    protected = protected_scope_snapshot(root, case["scope"], protected_root)
    before = live.snapshot([selected_root])
    try:
        _, added = run("add", case["selector"], case["expected"] != "dependency-refusal")
        npm_evidence = checked_npm_installation(root, env, npm["target"], npm["reference"], npm["installer"]) if npm else None
        if case["expected"] == "dependency-refusal":
            refusal = checked_dependency_refusal(added, case)
            require(not state_file(root, case["scope"]).get("installations"), "refused closure left managed installation")
            require(not any(path.exists() for path in documents), "refused closure published saved state")
            require(live.snapshot([selected_root]) == before, "refused closure mutated native state")
            outcome = {"status": "expected-dependency-refusal", "refusal": refusal}
        else:
            installation, binding, receipt = checked_binding(state_file(root, case["scope"]), case)
            saved_intent = {"add": checked_saved_intent(documents, receipt["alias"], case, root)}
            locator = in_case(Path(binding["target_locator"]), root)
            require(locator.is_dir() and bool(live.snapshot([locator])), "materialized artifact is missing")
            native_content = checked_native_materialization(selected_root, locator, case, binding, root)
            native_after = live.snapshot([selected_root])
            require(native_after != before, "native materialization was not independently observed")
            run("info", installation["installation_id"])
            run("list")
            # Exercise the actual updater; repeating add is not update evidence.
            run("update", receipt["alias"])
            updated, updated_binding, updated_receipt = checked_binding(state_file(root, case["scope"]), case)
            saved_intent["update"] = checked_saved_intent(documents, updated_receipt["alias"], case, root)
            require(updated["installation_id"] == installation["installation_id"], "update changed saved identity")
            require(checked_native_materialization(selected_root, locator, case, updated_binding, root) == native_content,
                    "update changed owned native MCP content")
            require(live.snapshot([selected_root]) == native_after, "immutable no-change update changed native files")
            run("repair", receipt["alias"])
            repaired, repaired_binding, repaired_receipt = checked_binding(state_file(root, case["scope"]), case)
            saved_intent["repair"] = checked_saved_intent(documents, repaired_receipt["alias"], case, root)
            require(repaired["installation_id"] == installation["installation_id"], "repair changed saved identity")
            require(checked_native_materialization(selected_root, locator, case, repaired_binding, root) == native_content,
                    "repair changed owned native MCP content")
            require(live.snapshot([selected_root]) == native_after, "healthy repair changed native files")
            run("remove", receipt["alias"])
            listed, _ = run("list")
            require(listed.get("data", {}).get("installations") == [], "removed package is still listed")
            checked_saved_removal(documents, receipt["alias"], case, root)
            require(not any(item.get("clients") for item in state_file(root, case["scope"]).get("installations", [])), "remove retained a native binding")
            require(not locator.exists(), "remove retained the managed package")
            cleanup = checked_native_cleanup(selected_root, case, binding, root)
            outcome = {"status": "materialized-lifecycle-passed", "installation_id": installation["installation_id"],
                       "activation_observed": [binding["activation"], updated_binding["activation"], repaired_binding["activation"]],
                       "native_files_after_add": len(native_after), "native_cleanup": cleanup,
                       "saved_intent": saved_intent, "native_content": native_content,
                       "update": "immutable-no-change", "repair": "healthy-repair"}
        require(protected_scope_snapshot(root, case["scope"], protected_root) == protected, "other scope changed")
        if npm:
            require(checked_npm_installation(root, env, npm["target"], npm["reference"], npm["installer"]) == npm_evidence,
                    "public npm identity changed during lifecycle")
        return {**case, **outcome, "trace": trace, "public_npm": npm_evidence,
                "activation_qualified": False, "runtime_qualified": False}
    except Exception as error:
        return {**case, "status": "failed", "reason": str(error), "trace": trace,
                "activation_qualified": False, "runtime_qualified": False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client", choices=CLIENTS, required=True)
    parser.add_argument("--target", choices=("linux-amd64", "darwin-arm64", "windows-amd64"), required=True)
    parser.add_argument("--native-bin", type=Path)
    parser.add_argument("--entrypoint", choices=("binary", "public-npm"), default="binary")
    parser.add_argument("--report-dir", type=Path, required=True)
    args = parser.parse_args()
    native.require_hosted(args.target)
    if args.client == "hermes":
        require(args.native_bin and args.native_bin.is_file(), "real native client executable required for detection")
    args.report_dir.mkdir(parents=True, exist_ok=False)
    report = {"schema_version": 1, "kind": "released-exact-source-lifecycle",
              "generated_at": datetime.now(timezone.utc).isoformat(),
              "workflow_sha": os.environ.get("AUDIT_WORKFLOW_SHA"), "harness_sha": os.environ.get("EXPECTED_HEAD"), "run_id": os.environ.get("GITHUB_RUN_ID"),
              "run_attempt": os.environ.get("GITHUB_RUN_ATTEMPT"), "platform": args.target, "entrypoint": args.entrypoint,
              "source_snapshot": {"ledger_commit": LEDGER, "sequence": 47, "digest": "sha256:" + SNAPSHOT_DIGEST},
              "claims": {"signed_grants": False, "native_activation": False, "mcp_runtime": False, "oauth": False},
              "status": "failed", "cases": []}
    try:
        with tempfile.TemporaryDirectory(prefix="released-source-", dir=os.environ["RUNNER_TEMP"]) as temp:
            directory = Path(temp).resolve()
            snapshot = authenticated_snapshot(directory)
            binary, report["installer"] = native.provision_release(ROOT, directory, args.target, RELEASE_TAG, RELEASE_COMMIT, REPOSITORY)
            npm = None
            if args.entrypoint == "public-npm":
                npm = {"target": args.target, "installer": report["installer"], "reference": acquire_npm_reference()}
                report["ui_helper"] = {"path": "landing/utils/projectCatalogSource.ts",
                                       "sha256": digest((ROOT / "landing/utils/projectCatalogSource.ts").read_bytes())}
            executable, report["native_client"] = provision_native(args.client, args.target, directory, args.native_bin)
            scopes = ("user",) if args.client == "hermes" else ("user", "project")
            for scope in scopes:
                for distribution, refusal in (("777genius/notion", False), ("777genius/marketing-kit", True)):
                    case = source_case(snapshot, distribution, 1, args.client, scope, refusal)
                    if npm:
                        case["ui"] = frontend_commands(directory / "snapshot.json", case)
                    report["cases"].append(run_case(binary, case, directory / f'{case["name"]}-{scope}', executable, npm))
            require(all(case["status"] != "failed" for case in report["cases"]), "some exact-source lifecycle cases failed")
            report["status"] = "passed"
    except Exception as error:
        report["reason"] = str(error)
    finally:
        (args.report_dir / "receipt.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"status": report["status"], "cases": [{"client": row["client"], "scope": row["scope"], "name": row["name"], "status": row["status"]} for row in report["cases"]]}))
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
