#!/usr/bin/env python3
"""Exact public-CLI source lifecycle evidence; never a signed target grant.

The receipt qualifies only its source/digests, client, scope, platform and tested
release. A consumer must separately check current active policy and denials.
Native activation, project trust, MCP runtime and account access are not proved.
"""
import argparse
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import stat
import re
import subprocess
import tarfile
import tempfile
import tomllib
import urllib.request
import zipfile
from types import SimpleNamespace
from datetime import datetime, timezone

import yaml

ROOT = Path(__file__).resolve().parents[1]
RELEASE_TAG = "agentplugins-v0.1.116"
RELEASE_COMMIT = "4858d973afcf6cf31b49bba98bdf06482dce6226"
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


class StrictNativeYAMLLoader(yaml.SafeLoader):
    """Audit only: reject ambiguous mappings before comparing native MCP."""


def strict_native_yaml_mapping(loader, node):
    result = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=True)
        require(type(key) is str and key not in result,
                "duplicate or nonstring native YAML key")
        result[key] = loader.construct_object(value_node, deep=True)
    return result


StrictNativeYAMLLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
                                        strict_native_yaml_mapping)


def strict_native_yaml(body):
    return yaml.load(body, Loader=StrictNativeYAMLLoader)


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


def protected_scope_changes(before, after, root, scope, native_root):
    """Describe only bounded disposable-relative paths; never include values."""
    documents = root / ("project" if scope == "user" else "uap-data")
    state = documents / ".uap" if scope == "user" else documents
    directories = [native_root, *[state / name for name in
                   ("managed", "operations-v2", "plugin-data", "installer-tmp", "clients")]]

    def safe_path(section, key):
        if not isinstance(key, str):
            return "<redacted>"
        path = None
        if section == "files":
            path = key
        else:
            match = re.fullmatch(r"(?:metadata|directory|link|root-kind|root-link):(\d+)(?::(.*))?", key)
            if match and int(match.group(1)) < len(directories):
                path = str(directories[int(match.group(1))].relative_to(root) / (match.group(2) or ""))
            else:
                for directory in directories:
                    prefix = directory.name + "/"
                    if key.startswith(prefix):
                        path = str(directory.relative_to(root) / key[len(prefix):])
                        break
        if path is not None:
            path = path.replace("\\", "/")
        if (path is None or len(path) > 200 or not re.fullmatch(r"[A-Za-z0-9._/-]+", path)
                or path.startswith("/") or any(part in ("", ".", "..") for part in path.split("/"))):
            return "<redacted>"
        return path

    changes, total = [], 0
    for section in ("directories", "files"):
        old, new = before[section], after[section]
        for key in sorted(set(old) | set(new)):
            if key not in old or key not in new or old[key] != new[key]:
                kind = "added" if key not in old else "removed" if key not in new else "changed"
                field = "metadata" if section == "directories" and isinstance(key, str) and key.startswith("metadata:") else "entry"
                total += 1
                if len(changes) < 32:
                    try:
                        path = safe_path(section, key)
                    except ValueError:
                        path = "<redacted>"
                    entry = {"path": path, "kind": kind, "field": field}
                    if kind == "changed" and field == "metadata":
                        left, right = old[key], new[key]
                        labels = ("mode", "device", "inode", "mtime")
                        if isinstance(left, (list, tuple)) and isinstance(right, (list, tuple)) and len(left) == len(right):
                            entry["changed_fields"] = [name for name, a, b in zip(labels, left, right) if a != b]
                    elif kind == "changed" and section == "files" and isinstance(old[key], dict) and isinstance(new[key], dict):
                        labels = ("digest", "mtime_ns", "inode", "mode")
                        entry["changed_fields"] = [name for name in labels if old[key].get(name) != new[key].get(name)]
                    changes.append(entry)
    return {"entries": changes, "total": total, "truncated": total > len(changes)}


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


def declared_runtime_lineage(root, case):
    """Bounded, untrusted state hints; this never establishes signed provenance."""
    unresolved = {"status": "unresolved", "reason": "state_unavailable_or_invalid"}
    def reject_constant(_):
        raise ValueError("non-finite JSON number")
    try:
        require(case.get("scope") in ("user", "project"), "unknown scope")
        path = root / ("uap-data" if case["scope"] == "user" else "project/.uap") / "state-v2.json"
        before = path.lstat()
        # This existing reader holds every POSIX ancestor by no-follow dirfd;
        # on Windows it verifies the opened handle's final path. Unsupported
        # platforms fail closed rather than falling back to pathname reads.
        body, status = live.copilot_settings_read(path, root)
        require(status is None and isinstance(body, str), "state unavailable")
        after = path.lstat()
        require(os.path.samestat(before, after) and before.st_mode == after.st_mode
                and before.st_nlink == after.st_nlink == 1 and before.st_size == after.st_size
                and before.st_mtime_ns == after.st_mtime_ns and before.st_ctime_ns == after.st_ctime_ns,
                "state changed while observed")
        state = json.loads(body, object_pairs_hook=live._unique_json_object, parse_constant=reject_constant)
        require(isinstance(state, dict) and type(state.get("schema_version")) is int
                and state["schema_version"] == 5 and isinstance(state.get("installations"), list), "invalid state")
        records = state["installations"]
        children = {"777genius/notion": "notion", "777genius/hubspot-crm": "hubspot-crm",
                    "777genius/playwright-bridge": "playwright"}
        entries = []
        for record in records[:8]:
            require(isinstance(record, dict), "invalid installation")
            origin = record.get("origin_mode")
            require(origin in ("direct", "directory"), "unknown origin")
            directory = record.get("directory")
            if origin == "direct":
                require(directory is None and isinstance(record.get("source"), dict), "invalid direct source")
                source, expected = record["source"], case["source"]
                match = (record.get("declared_name") == "marketing-kit"
                         and source.get("repository") == expected["repository"]
                         and source.get("package_subpath") == expected["path"]
                         and source.get("resolved_revision") == expected["revision"])
                entries.append({"origin_mode": "direct", "distribution_id": None,
                                "snapshot_schema": None, "snapshot_sequence": None,
                                "snapshot_digest": None, "expected_identity_match": match,
                                "identity_basis": "direct_source_coordinates"})
                continue
            require(isinstance(directory, dict), "missing directory origin")
            distribution = directory.get("distribution_id")
            schema, sequence, snapshot_digest = (directory.get("snapshot_schema"),
                directory.get("snapshot_sequence"), directory.get("snapshot_digest"))
            require(isinstance(distribution, str) and type(schema) is int and schema in (1, 2)
                    and type(sequence) is int and 0 < sequence < 1 << 64
                    and isinstance(snapshot_digest, str)
                    and re.fullmatch(r"sha256:[0-9a-f]{64}", snapshot_digest), "invalid snapshot identity")
            known = distribution if distribution in {*children, "777genius/marketing-kit"} else "<other>"
            entries.append({"origin_mode": "directory", "distribution_id": known,
                            "snapshot_schema": schema, "snapshot_sequence": sequence,
                            "snapshot_digest": snapshot_digest,
                            "expected_identity_match": distribution in children
                                and record.get("declared_name") == children[distribution],
                            "identity_basis": "distribution_id_and_declared_name"})
        return {"status": "unresolved", "reason": "unverified_local_state", "declared_only": True,
                "total_installations": len(records), "truncated": len(records) > 8, "records": entries}
    except Exception:
        return unresolved


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
    locked = json.loads(bodies[1].decode("utf-8"), object_pairs_hook=live._unique_json_object)
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
    value = strict_native_yaml(body) if client == "hermes" else json.loads(body, object_pairs_hook=live._unique_json_object)
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


FOREIGN_NATIVE_MCP = "uap-unrelated-audit"


def seed_unrelated_native_mcp(selected_root, client, root):
    """Seed an inert foreign entry that must survive every native lifecycle step."""
    if client not in ("hermes", "pi"):
        return None
    config = in_case(selected_root / ("config.yaml" if client == "hermes" else "mcp.json"), root)
    require(not config.exists() and not config.is_symlink(), "foreign MCP seed would replace native configuration")
    if client == "hermes":
        entry = {"command": "echo", "args": ["unrelated-native-audit"]}
        body = "mcp_servers:\n  " + FOREIGN_NATIVE_MCP + ":\n    command: echo\n    args: [unrelated-native-audit]\n"
    else:
        entry = {"url": "https://foreign.invalid/mcp", "enabled": False}
        body = json.dumps({"mcpServers": {FOREIGN_NATIVE_MCP: entry}}) + "\n"
    config.write_text(body, encoding="utf-8")
    config.chmod(0o600)
    return client, config, entry


def check_unrelated_native_mcp(control, root):
    if control is None:
        return
    client, path, expected = control
    body, _ = read_saved_record(in_case(path, root))
    require(body is not None, "unrelated native MCP configuration was removed")
    document = (strict_native_yaml(body) if client == "hermes"
                else json.loads(body, object_pairs_hook=live._unique_json_object))
    collection = "mcp_servers" if client == "hermes" else "mcpServers"
    require(isinstance(document, dict) and isinstance(document.get(collection), dict)
            and document[collection].get(FOREIGN_NATIVE_MCP) == expected,
            "unrelated native MCP entry changed during lifecycle")



class PiProjectTrustOracle:
    """Frozen Project hook, admitted only after independent native owner proof."""

    def __init__(self, root, native_root, verified_owners=None):
        self.root, self.native = root, native_root
        self.project = root / "project"
        self.hook = native_root / "extensions/agentplugins-project-trust"
        self.phase, self.installed = "fresh", None
        self.owner_verifier = verified_owners
        self.directories = (native_root / "extensions", self.hook)
        self.files = tuple(self.hook / name for name in
                           ("index.ts", "projects.json", "index.ts.agentplugins.lock",
                            "projects.json.agentplugins.lock"))
        self.anchors = {}
        require(not any(path.exists() or path.is_symlink() for path in self.directories),
                "Pi trust fixture must start without a hook namespace")
        self.anchor(native_root)
        self.resource_audit = module("source_pi_skill_contract", "pi_project_skill_audit.py")

    def anchor(self, path):
        while path != self.root.parent:
            in_case(path, self.root)
            info = path.lstat()
            require(stat.S_ISDIR(info.st_mode) and not path.is_symlink(),
                    "Pi trust ancestor is not a real directory")
            stamp = (info.st_mode, info.st_dev, info.st_ino)
            require(self.anchors.setdefault(path, stamp) == stamp, "Pi trust ancestor replaced")
            path = path.parent

    def read(self, path):
        self.anchor(path.parent)
        body, before = read_saved_record(in_case(path, self.root))
        require(body is not None, "Pi trust file is absent")
        self.anchor(path.parent)
        # The same custody reader used for protected state holds ancestors and
        # checks final handle paths on Windows. Join its bytes to the file read.
        text, error = live.copilot_settings_read(path, self.root)
        require(error is None and isinstance(text, str) and text.encode("utf-8") == body,
                "Pi trust held read differs")
        after = path.lstat()
        require(os.path.samestat(before, after) and before.st_mode == after.st_mode
                and before.st_size == after.st_size and before.st_mtime_ns == after.st_mtime_ns,
                "Pi trust file replaced")
        return body

    def tree_digest(self, path):
        image = {}
        total_bytes = 0
        def walk(directory, prefix=""):
            nonlocal total_bytes
            self.anchor(directory)
            for child in sorted(directory.iterdir(), key=lambda p: p.name.encode("utf-8")):
                relative = prefix + child.name
                info = child.lstat()
                if stat.S_ISDIR(info.st_mode) and not child.is_symlink():
                    image[relative] = ("dir",)
                    walk(child, relative + "/")
                else:
                    body = self.read(child)
                    total_bytes += len(body)
                    require(total_bytes <= 64 << 20, "Pi trust tree exceeds byte bound")
                    image[relative] = ("file", hashlib.sha256(body).hexdigest())
                require(len(image) <= 16384, "Pi trust tree exceeds entry bound")
        walk(path)
        # Reuse the existing frozen Go/JS trust framing, not Skill digest framing.
        adapter = type("TrustReader", (), {"bytes": lambda _, p: self.read(p)})()
        return self.resource_audit.PiProjectSkillAudit.trust_digest(adapter, path, image)

    @staticmethod
    def value_digest(value):
        # Frozen framing for the independently checked string/list transport
        # objects below. Unused numeric and boolean values stay unsupported.
        def frame(item):
            if type(item) is str:
                body = item.encode("utf-8")
                return b"s" + str(len(body)).encode() + b":" + body
            if type(item) is list:
                return b"a" + str(len(item)).encode() + b":" + b"".join(frame(v) for v in item)
            require(type(item) is dict and all(type(k) is str for k in item), "unsupported Pi trust value")
            keys = sorted(item, key=lambda k: k.encode("utf-8"))
            return b"o" + str(len(keys)).encode() + b":" + b"".join(frame(k) + frame(item[k]) for k in keys)
        return digest(frame(value))

    def verified_owners(self, case):
        """Use authenticated four-owner closure proof, or the exact Notion proof."""
        if self.owner_verifier is not None:
            owners = self.owner_verifier(self, case)
            require(type(owners) is dict and len(owners) == 4, "Pi closure owners are incomplete")
            return owners
        _, binding, _ = checked_binding(state_file(self.root, "project"), case)
        locator = in_case(Path(binding["target_locator"]), self.root)
        checked_native_materialization(self.project / ".pi", locator, case, binding, self.root)
        physical = binding["physical_artifact_id"]
        require(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", physical) is not None,
                "Pi trust owner is not a portable artifact")
        require(locator == self.project / ".pi/extensions" / physical,
                "Pi trust owner is not the selected project package")
        return {physical: {
            "extensions": {physical: self.tree_digest(locator)}, "skills": {},
            "mcp_servers": {"notion": self.value_digest({"url": "https://mcp.notion.com/mcp"})}}}

    def validate_installed(self, case):
        owners = self.verified_owners(case)
        record = json.loads(self.read(self.hook / "projects.json"),
                            object_pairs_hook=live._unique_json_object,
                            parse_constant=lambda _: require(False, "non-finite Pi trust JSON"))
        expected = {"schema_version": 1, "projects": {str(self.project): owners}}
        require(type(record.get("schema_version")) is int and record == expected,
                "Pi trust record differs from verified project ownership")
        hook = self.read(self.hook / "index.ts")
        # Same exact frozen literal as pi_project_skill_audit.py trust_check().
        require(len(hook) == 9078 and hashlib.sha256(hook).hexdigest() ==
                "eb4fa44cb880e1df05e42356f7e4bef9efef4abad82495cdfc87867b76b90a73",
                "Pi trust hook differs from frozen release")

    def allowed_image(self, removed=False):
        paths = (*self.directories, *self.files)
        for directory in self.directories:
            self.anchor(directory)
        require(set(self.hook.iterdir()) == set(self.files[2:] if removed else self.files),
                "Pi trust namespace contains foreign entries")
        image = {}
        for path in paths:
            if removed and path in self.files[:2]:
                require(not path.exists() and not path.is_symlink(), "Pi trust record survived removal")
                continue
            info = path.lstat()
            directory = path in self.directories
            expected_mode = ((0o777 if directory else 0o666) if os.name == "nt"
                             else (0o700 if directory else 0o600))
            require((stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode))
                    and not path.is_symlink() and stat.S_IMODE(info.st_mode) == expected_mode,
                    "Pi trust path kind or mode changed")
            self.anchor(path if directory else path.parent)
            body = None if directory else self.read(path)
            if path in self.files[2:]:
                require(body == b"", "Pi trust writer lock is not inert")
            image[path] = (info.st_mode, info.st_dev, info.st_ino,
                           None if directory else info.st_mtime_ns, body)
        return image

    def project_snapshot(self, current):
        # Remove only exact validated owned keys; every sibling and all saved
        # User state remain in the original full protected snapshot.
        result = {"files": current["files"], "directories": dict(current["directories"])}
        for path in (*self.directories, *self.files):
            relative = path.relative_to(self.native)
            for key in (self.native.name + "/" + str(relative),
                        "directory:0:" + relative.as_posix(),
                        "metadata:0:" + relative.as_posix()):
                result["directories"].pop(key, None)
        return result

    def observe(self, operation, result, current, case):
        require(type(result.get("exit_code")) is int and result["exit_code"] == 0
                and result.get("timeout") is False, "Pi trust transition requires successful operation")
        if self.phase == "fresh":
            require(operation == "add", "Pi trust appeared outside confirmed add")
            self.validate_installed(case)
            image = self.allowed_image()
            self.installed, self.phase = image, "installed"
        elif self.phase == "installed":
            require(operation in ("info", "list", "update", "repair", "remove"),
                    "unexpected Pi trust phase")
            if operation == "remove":
                image = self.allowed_image(removed=True)
                require(all(value == self.installed[path] for path, value in image.items()),
                        "Pi trust cleanup changed retained identities")
                self.phase = "removed"
            else:
                self.validate_installed(case)
                require(self.allowed_image() == self.installed,
                        "Pi trust changed after confirmed add")
        else:
            require(operation == "list", "Pi trust operation after removal")
            image = self.allowed_image(removed=True)
            require(all(value == self.installed[path] for path, value in image.items()),
                    "Pi trust residue changed")
        return self.project_snapshot(current)


def checked_pi_closure_native(oracle, closure, case, selected_root, alias, owner):
    """One authenticated closure member, exact native object and transport."""
    binding, locator = owner["binding"], owner["locator"]
    physical = binding.get("physical_artifact_id")
    require(case["client"] == "pi" and type(physical) is str
            and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", physical)
            and locator == selected_root / "extensions" / physical,
            "Pi closure physical placement differs")
    require(not any((locator / name).exists() or (locator / name).is_symlink()
                    for name in ("index.ts", "index.js", ".agentplugins-pi.json", "package.json")),
            "Pi closure package acquired an unowned native entrypoint")
    objects = binding.get("native_objects")
    require(type(objects) is list and len(objects) == 2 and all(type(v) is dict for v in objects),
            "Pi closure native ownership inventory differs")
    packages = [v for v in objects if v.get("kind") == "managed_package_directory"]
    require(len(packages) == 1, "Pi closure managed package owner missing")
    package = packages[0]
    require(package.get("object_id") == "package:pi:" + physical
            and package.get("logical_name") == alias and Path(package.get("path", "")) == locator
            and package.get("protection_class") == "managed" and not package.get("user_modified")
            and re.fullmatch(r"sha256:[0-9a-f]{64}", package.get("managed_digest", "")),
            "Pi closure managed package owner differs")
    receipts = binding.get("receipts", [])
    require(type(receipts) is list and any(type(receipt) is dict and receipt.get("phase") == "committed"
            and receipt.get("active_path") == str(locator)
            and receipt.get("client_binding_id") == binding.get("client_binding_id")
            and receipt.get("after_digest") == package["managed_digest"] for receipt in receipts),
            "Pi closure package has no committed native receipt")
    member = next(v for v in objects if v is not package)
    trust = {"extensions": {physical: oracle.tree_digest(locator)}, "skills": {}, "mcp_servers": {}}
    if alias == "marketing-kit":
        kind = "pi_" + case["scope"] + "_skill_directory"
        path = selected_root / "skills/campaign-planning"
        require(member.get("kind") == kind and member.get("object_id") == kind + ":campaign-planning"
                and member.get("logical_name") == "campaign-planning"
                and member.get("source_relative") == "skills/campaign-planning"
                and Path(member.get("path", "")) == path, "Pi closure Skill ownership differs")
        require(set(path.iterdir()) == {path / "SKILL.md"}
                and set((locator / "skills/campaign-planning").iterdir()) == {locator / "skills/campaign-planning/SKILL.md"},
                "Pi closure Skill contains unreviewed resources")
        require(set((selected_root / "skills").iterdir()) == {path}
                and set((locator / "skills").iterdir()) == {locator / "skills/campaign-planning"},
                "Pi closure exposes an unreviewed Skill")
        for skill in (path / "SKILL.md", locator / "skills/campaign-planning/SKILL.md"):
            require(digest(oracle.read(skill)) == "sha256:42ffb5ea182abd0c710735a28e6f6989cc8a4e125642f1d954dabd2e10ca76b1",
                    "Pi closure Skill differs from immutable source")
        body = oracle.read(path / "SKILL.md")
        image = {"SKILL.md": ("file", (path / "SKILL.md").lstat().st_mode,
                              None, None, None, len(body), hashlib.sha256(body).hexdigest())}
        adapter = type("SkillReader", (), {"bytes": lambda _, p: oracle.read(p)})()
        require(member.get("managed_digest") == oracle.resource_audit.PiProjectSkillAudit.skill_digest(adapter, path, image),
                "Pi closure Skill receipt digest differs from native bytes")
        trust["skills"]["campaign-planning"] = oracle.tree_digest(path)
    else:
        kind = "pi_" + case["scope"] + "_mcp_server"
        path = selected_root / "mcp.json"
        require(member.get("kind") == kind and member.get("object_id") == kind + ":" + alias
                and member.get("logical_name") == alias and Path(member.get("path", "")) == path,
                "Pi closure MCP ownership differs")
        if alias in ("notion", "hubspot-crm"):
            entry = {"url": {"notion": "https://mcp.notion.com/mcp", "hubspot-crm": "https://mcp.hubspot.com"}[alias]}
        else:
            require(alias == "playwright", "unknown Pi closure member")
            closure.checked_playwright_runtime(SimpleNamespace(**globals()), locator, oracle.root)
            entry = {"command": "node", "args": [str(locator / "io.github.777genius.agentplugins/runtime/launcher.mjs")]}
        document = json.loads(oracle.read(path), object_pairs_hook=live._unique_json_object)
        require(type(document) is dict and type(document.get("mcpServers")) is dict
                and set(document) == {"mcpServers"}
                and set(document["mcpServers"]) == {FOREIGN_NATIVE_MCP, "notion", "hubspot-crm", "playwright"}
                and document["mcpServers"].get(alias) == entry, "Pi closure MCP transport differs")
        trust["mcp_servers"][alias] = oracle.value_digest(entry)
    require(member.get("protection_class") == "managed" and not member.get("user_modified")
            and re.fullmatch(r"sha256:[0-9a-f]{64}", member.get("managed_digest", "")),
            "Pi closure native member lacks ownership")
    return {"content_verified": True, "physical": physical, "trust_owner": trust,
            "path": path.relative_to(oracle.root).as_posix()}


def prepare_policy_context(directory, evidence_dir, target):
    """One frozen-source verifier build, separate from the released CLI."""
    closure = module("source_closure", "released_source_closure.py")
    context = closure.prepare(SimpleNamespace(**globals()), directory, evidence_dir)
    context["target"] = target
    return context


def run_case(binary, case, root, executable, policy_context=None, command_env=None):
    root.mkdir()
    home, project, data = (root / name for name in ("home", "project", "uap-data"))
    for path in (home, project, data):
        path.mkdir()
    env = live.isolated_client_env(home, root, data)
    if command_env:
        require(set(command_env) <= {"PATH", "NPM_CONFIG_CACHE", "NPM_CONFIG_PREFIX", "NPM_CONFIG_USERCONFIG",
                                     "NPM_CONFIG_GLOBALCONFIG", "NPM_CONFIG_REGISTRY", "NPM_CONFIG_IGNORE_SCRIPTS"}
                and all(type(value) is str for value in command_env.values()),
                "npm command environment would replace case isolation")
        env.update(command_env)
    require(not any("PROOF" in key for key in env), "proof-mode environment is forbidden for released CLI")
    command = [str(part) for part in binary] if isinstance(binary, (list, tuple)) else [str(binary)]
    require(bool(command) and (not isinstance(binary, (list, tuple))
            or len(command) == 2 and all(Path(part).is_absolute() for part in command)),
            "public npm command prefix is not absolute")
    env["HERMES_HOME"] = str(home / ".hermes")
    if executable:
        env["PATH"] = str(executable.parent) + os.pathsep + env.get("PATH", "")
    native_root = home / {"hermes": ".hermes", "antigravity": ".gemini/config", "pi": ".pi/agent"}[case["client"]]
    native_root.mkdir(parents=True)
    project_native = project / {"hermes": ".hermes", "antigravity": ".agents", "pi": ".pi"}[case["client"]]
    project_native.mkdir()
    sentinels = [home / "foreign.txt", project / "foreign.txt", native_root / "foreign.txt", project_native / "foreign.txt"]
    for path in sentinels:
        path.write_bytes(b"unmanaged source audit neighbor\n")
    foreign_mcp_control = seed_unrelated_native_mcp(
        native_root if case["scope"] == "user" else project_native, case["client"], root)
    live.initialize_project_skill_git_root(project, env, root)
    scope = ["--scope", case["scope"]]
    if case["scope"] == "project":
        scope += ["--project-root", str(project)]
    trace = []
    scope_changes = None
    saved_root = data if case["scope"] == "user" else project
    documents = [saved_root / "uap.toml", saved_root / "uap.lock"]
    closure = None
    closure_state = {"authorities": {}, "owners": None, "proofs": None, "identity": None,
                     "native_baseline": None, "removed": False}
    pi_native = None

    def refresh_closure(stage, result):
        require(policy_context is not None, "authenticated runtime policy context unavailable")
        api = SimpleNamespace(**globals())
        authority = closure.authorize(api, policy_context, root, case, stage)
        closure_state["authorities"][stage] = authority
        if result["exit_code"] != 0:
            require(stage == "add" and authority.get("policy_status") == "ineligible",
                    "failed closure operation is not an authenticated policy refusal")
            return
        owners = closure.checked_closure(api, case, root, documents, authority)
        verifier = (lambda alias, owner: checked_pi_closure_native(pi_native, closure, case, selected_root, alias, owner)
                    if case["client"] == "pi" else
                    closure.checked_member_native(api, case, root, selected_root, alias, owner))
        proofs = closure.checked_native(api, owners, verifier)
        closure.checked_namespace(api, case, root, selected_root, owners)
        identity = {alias: (owner["installation"]["installation_id"], owner["binding"]["client_binding_id"],
                            owner["binding"]["physical_artifact_id"], str(owner["locator"]))
                    for alias, owner in owners.items()}
        require(closure_state["identity"] in (None, identity), "closure changed saved installation identity")
        closure_state.update(owners=owners, proofs=proofs, identity=identity)
        if stage == "add":
            closure_state["native_baseline"] = live.snapshot([selected_root])

    def verified_pi_owners(oracle, selected_case):
        require(selected_case == case and closure_state["owners"] is not None
                and set(closure_state["proofs"]) == set(live.MARKETING_KIT_DISTRIBUTIONS),
                "Pi trust has no independently authenticated closure")
        return {proof["physical"]: proof["trust_owner"] for proof in closure_state["proofs"].values()}

    def run(operation, selector=None, success=True):
        nonlocal scope_changes
        argv = [*command, operation, *([selector] if selector else []), "--target", case["client"], *scope, "--format", "json"]
        if operation in ("add", "update", "repair", "remove"):
            argv += ["--yes"]
        for path in (root, home, project, data, native_root, project_native):
            in_case(path, root)
        result = live.call(argv, env, project, 180, capture_files=True,
                           capture_full=not success or case["expected"] == "dependency-closure")
        parsed = result.pop("_json", None)
        trace.append({"operation": operation, "argv": argv, **result})
        for path in sentinels:
            require(live.preserved_regular_text(path, "unmanaged source audit neighbor\n", root), "foreign state changed")
        check_unrelated_native_mcp(foreign_mcp_control, root)
        if case["expected"] == "dependency-closure" and operation in ("add", "update", "repair"):
            require(type(result.get("exit_code")) is int and result.get("timeout") is False,
                    "closure operation did not finish")
            refresh_closure(operation, result)
        if case["expected"] == "dependency-closure" and operation == "remove" and result.get("exit_code") == 0:
            closure_state["removed"] = True
        if (case["expected"] == "dependency-closure" and closure_state["native_baseline"] is not None
                and operation in ("info", "list", "update", "repair")
                and not closure_state["removed"]):
            require(live.snapshot([selected_root]) == closure_state["native_baseline"],
                    "closure operation changed native resources")
        current = protected_scope_snapshot(root, case["scope"], protected_root)
        if pi_trust is not None and result.get("exit_code") == 0:
            current = pi_trust.observe(operation, result, current, case)
        if current != protected:
            scope_changes = protected_scope_changes(protected, current, root, case["scope"], protected_root)
            raise ValueError(operation + " changed the other scope")
        if success:
            require(result["exit_code"] == 0 and not result.get("timeout") and isinstance(parsed, dict), operation + " did not complete")
        return parsed, result

    selected_root = native_root if case["scope"] == "user" else project_native
    protected_root = project_native if case["scope"] == "user" else native_root
    protected = protected_scope_snapshot(root, case["scope"], protected_root)
    pi_trust = None
    before = live.snapshot([selected_root])
    try:
        if case["expected"] == "dependency-closure":
            require(case["name"] == "marketing-kit" and policy_context is not None,
                    "authenticated closure prerequisites unavailable")
            closure = module("source_closure_case", "released_source_closure.py")
            if case["client"] == "pi":
                pi_native = PiProjectTrustOracle(root, native_root, verified_pi_owners)
                if case["scope"] == "project":
                    pi_trust = pi_native
        if (case["client"], case["scope"], case["name"], case["expected"]) == (
                "pi", "project", "notion", "materialized-lifecycle"):
            pi_trust = PiProjectTrustOracle(root, native_root)
        _, added = run("add", case["selector"], case["expected"] not in ("dependency-refusal", "dependency-closure"))
        if case["expected"] == "dependency-closure":
            authority = closure_state["authorities"]["add"]
            if added["exit_code"] != 0:
                refusal = checked_dependency_refusal(added, case)
                closure.checked_refusal_authority(SimpleNamespace(**globals()), case, authority, refusal)
                require(authority.get("policy_status") == "ineligible"
                        and refusal["distribution"] in authority.get("ineligible_distributions", []),
                        "refusal differs from authenticated ineligible policy")
                require(not state_file(root, case["scope"]).get("installations")
                        and not state_file(root, case["scope"]).get("environment_receipts"),
                        "refused closure left owned state")
                require(not any(path.exists() or path.is_symlink() for path in documents)
                        and live.snapshot([selected_root]) == before, "refused closure published native or saved effects")
                outcome = {"status": "expected-dependency-refusal", "refusal": refusal,
                           "authenticated_policy": closure_state["authorities"]}
            else:
                require(closure_state["owners"] is not None, "successful closure lacks authenticated owners")
                installed_owners = closure_state["owners"]
                initial_native = live.snapshot([selected_root])
                require(initial_native != before, "closure native materialization not observed")
                for owner in installed_owners.values():
                    run("info", owner["installation"]["installation_id"])
                run("list")
                for stage in ("update", "repair"):
                    run(stage, "marketing-kit")
                    require(live.snapshot([selected_root]) == initial_native,
                            "immutable closure operation changed native resources")
                proofs = closure_state["proofs"]
                run("remove", "marketing-kit")
                listed, _ = run("list")
                require(listed.get("data", {}).get("installations") == [], "removed closure is still listed")
                closure.checked_cleanup(SimpleNamespace(**globals()), installed_owners, selected_root, case, root, documents)
                outcome = {"status": "materialized-closure-lifecycle-passed", "native_content": proofs,
                           "authenticated_policy": closure_state["authorities"],
                           "owned_aliases": sorted(installed_owners), "update": "immutable-no-change",
                           "repair": "healthy-repair", "native_cleanup": "all-four-owners-absent"}
        elif case["expected"] == "dependency-refusal":
            if type(added.get("exit_code")) is int and added["exit_code"] == 0 and added.get("timeout") is False:
                raise ValueError("successful closure is inconclusive: runtime signed policy and full owned lifecycle are unverified")
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
        current = protected_scope_snapshot(root, case["scope"], protected_root)
        if pi_trust is not None and closure_state["authorities"].get("add", {}).get("policy_status") != "ineligible":
            require(pi_trust.phase == "removed", "Pi trust lifecycle is incomplete")
            current = pi_trust.observe("list", trace[-1], current, case)
        if current != protected:
            scope_changes = protected_scope_changes(protected, current, root, case["scope"], protected_root)
            raise ValueError("other scope changed")
        return {**case, **outcome, "trace": trace, "activation_qualified": False, "runtime_qualified": False}
    except Exception as error:
        outcome = {**case, "status": "failed", "reason": str(error), "trace": trace,
                   "activation_qualified": False, "runtime_qualified": False}
        if scope_changes is not None:
            outcome["protected_scope_changes"] = scope_changes
        if (case.get("expected") in ("dependency-refusal", "dependency-closure") and case.get("name") == "marketing-kit"
                and trace and trace[0].get("operation") == "add"
                and type(trace[0].get("exit_code")) is int and trace[0]["exit_code"] == 0
                and trace[0].get("timeout") is False):
            outcome["declared_runtime_lineage"] = declared_runtime_lineage(root, case)
        return outcome


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client", choices=CLIENTS, required=True)
    parser.add_argument("--target", choices=("linux-amd64", "darwin-arm64", "windows-amd64"), required=True)
    parser.add_argument("--native-bin", type=Path)
    parser.add_argument("--report-dir", type=Path, required=True)
    args = parser.parse_args()
    native.require_hosted(args.target)
    if args.client == "hermes":
        require(args.native_bin and args.native_bin.is_file(), "real native client executable required for detection")
    args.report_dir.mkdir(parents=True, exist_ok=False)
    report = {"schema_version": 1, "kind": "released-exact-source-lifecycle",
              "generated_at": datetime.now(timezone.utc).isoformat(),
              "workflow_sha": os.environ.get("AUDIT_WORKFLOW_SHA"), "harness_sha": os.environ.get("EXPECTED_HEAD"), "run_id": os.environ.get("GITHUB_RUN_ID"),
              "run_attempt": os.environ.get("GITHUB_RUN_ATTEMPT"), "platform": args.target,
              "source_snapshot": {"ledger_commit": LEDGER, "sequence": 47, "digest": "sha256:" + SNAPSHOT_DIGEST},
              "claims": {"signed_grants": False, "native_activation": False, "mcp_runtime": False, "oauth": False},
              "status": "failed", "cases": []}
    try:
        with tempfile.TemporaryDirectory(prefix="released-source-", dir=os.environ["RUNNER_TEMP"]) as temp:
            directory = Path(temp).resolve()
            snapshot = authenticated_snapshot(directory)
            binary, report["installer"] = native.provision_release(ROOT, directory, args.target, RELEASE_TAG, RELEASE_COMMIT, REPOSITORY)
            executable, report["native_client"] = provision_native(args.client, args.target, directory, args.native_bin)
            policy_context = prepare_policy_context(directory, args.report_dir / "verified-public-policy", args.target)
            npm = module("source_public_npm", "released_source_npm.py").provision(
                SimpleNamespace(**globals()), directory, args.target,
                release_context={"binary": binary, "proof": report["installer"]})
            report["npm"] = npm["proof"]
            scopes = ("user",) if args.client == "hermes" else ("user", "project")
            channels = (("binary", binary, None), ("npm", npm["command"], npm.get("env")))
            for channel, command, command_env in channels:
                for scope in scopes:
                    for distribution, refusal in (("777genius/notion", False), ("777genius/marketing-kit", True)):
                        case = source_case(snapshot, distribution, 1, args.client, scope, refusal)
                        case["channel"] = channel
                        if case["name"] == "marketing-kit":
                            case["expected"] = "dependency-closure"
                        report["cases"].append(run_case(command, case, directory / f'{channel}-{case["name"]}-{scope}', executable,
                                                        policy_context=policy_context, command_env=command_env))
            require(all(case["status"] != "failed" for case in report["cases"]), "some exact-source lifecycle cases failed")
            report["status"] = "passed"
    except Exception as error:
        report["reason"] = str(error)
    finally:
        (args.report_dir / "receipt.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"status": report["status"], "cases": [{"channel": row.get("channel"), "client": row["client"], "scope": row["scope"], "name": row["name"], "status": row["status"]} for row in report["cases"]]}))
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
