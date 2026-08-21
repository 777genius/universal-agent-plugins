#!/usr/bin/env python3
"""Build complete Agent Plugins packages from pinned copy-plus-overlay recipes.

The only subprocess invoked is Git. Upstream files are read as inert blobs from
an exact commit; they are never checked out, imported, or executed.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import stat
import subprocess
import sys
import tempfile
import unicodedata
from dataclasses import dataclass
from pathlib import Path, PurePosixPath

import jsonschema
import yaml

sys.path.insert(0, str(Path(__file__).resolve().parent))
from build_registry import (
    RegistryError,
    directory_tree_digest as package_tree_digest,
    read_object,
    validate_bridge_bindings,
)
from portable_paths import validate_segment, validate_tree
from validate_catalog import ValidationError, validate_plugin


ROOT = Path(__file__).resolve().parents[1]
SCHEMA = ROOT / "schemas" / "bridge-recipe.schema.json"
ID_RE = re.compile(r"^(?!.*(?:--|\.\.))[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$")
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
GIT_MODES = {"100644": 0o644, "100755": 0o755}
LFS_HEADER = b"version https://git-lfs.github.com/spec/v1\n"


class BridgeError(Exception):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise BridgeError(message)


class UniqueKeyLoader(yaml.SafeLoader):
    pass


def _unique_mapping(loader: UniqueKeyLoader, node: yaml.MappingNode, deep: bool = False):
    result = {}
    folded = set()
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        require(isinstance(key, str), "recipe mapping keys must be strings")
        normalized = unicodedata.normalize("NFC", key).casefold()
        require(normalized not in folded, f"duplicate or case-colliding recipe key: {key!r}")
        folded.add(normalized)
        result[key] = loader.construct_object(value_node, deep=deep)
    return result


UniqueKeyLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, _unique_mapping
)


def sha256(body: bytes) -> str:
    return "sha256:" + hashlib.sha256(body).hexdigest()


def portable_path(value: str, field: str) -> PurePosixPath:
    require(isinstance(value, str) and value and value.isascii(), f"{field}: path must be non-empty ASCII")
    require("\\" not in value and "%" not in value, f"{field}: ambiguous separator or escape")
    path = PurePosixPath(value)
    require(not path.is_absolute() and path.as_posix() == value, f"{field}: path must be normalized and relative")
    require(".git" not in path.parts, f"{field}: Git metadata is forbidden")
    for segment in path.parts:
        try:
            validate_segment(segment)
        except ValueError as error:
            raise BridgeError(f"{field}: {error}") from error
    return path


def git(directory: Path, *args: str, input_bytes: bytes | None = None) -> bytes:
    environment = {
        "PATH": os.environ.get("PATH", ""),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_CONFIG_GLOBAL": os.devnull,
        "GIT_TERMINAL_PROMPT": "0",
        "GIT_OPTIONAL_LOCKS": "0",
        "LANG": "C",
        "LC_ALL": "C",
    }
    try:
        result = subprocess.run(
            ["git", *args], cwd=directory, env=environment, input=input_bytes,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False, timeout=120,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        raise BridgeError(f"Git invocation failed: {error}") from error
    if result.returncode:
        detail = result.stderr.decode("utf-8", "replace").strip()
        raise BridgeError(f"Git {' '.join(args[:2])} failed: {detail}")
    return result.stdout


@dataclass(frozen=True)
class Blob:
    path: str
    mode: int
    body: bytes


class PinnedRepository:
    def __init__(self, repository: str, revision: str, mirror_root: Path | None):
        self.repository = repository
        self.revision = revision
        self.temporary = tempfile.TemporaryDirectory(prefix="bridge-git-")
        self.root = Path(self.temporary.name)
        git(self.root, "init", "--quiet", "--bare")
        if mirror_root is None:
            remote = f"https://github.com/{repository}.git"
            protocol = "protocol.file.allow=never"
        else:
            remote = str((mirror_root / f"{repository}.git").resolve())
            require(Path(remote).is_dir(), f"offline mirror does not exist: {remote}")
            protocol = "protocol.file.allow=always"
        git(self.root, "remote", "add", "origin", remote)
        git(
            self.root, "-c", protocol, "fetch", "--quiet", "--no-tags",
            "--depth=1", "--filter=blob:none", "origin", revision,
        )
        actual = git(self.root, "rev-parse", "FETCH_HEAD^{commit}").decode().strip()
        require(actual == revision, f"fetched commit {actual} does not match pinned revision {revision}")

    def close(self) -> None:
        self.temporary.cleanup()

    def _tree_records(self, source: str) -> list[tuple[str, str, str]]:
        raw = git(self.root, "ls-tree", "-rz", "-r", self.revision, "--", source)
        records = []
        for record in raw.split(b"\0"):
            if not record:
                continue
            try:
                metadata, path = record.split(b"\t", 1)
                mode, kind, _object_id = metadata.decode("ascii").split(" ")
                decoded = path.decode("utf-8")
            except (ValueError, UnicodeError) as error:
                raise BridgeError("upstream contains an invalid Git tree entry") from error
            require(kind != "commit" and mode != "160000", f"upstream submodule is forbidden: {decoded}")
            require(kind == "blob" and mode in GIT_MODES, f"unsupported upstream entry mode {mode}: {decoded}")
            records.append((decoded, mode, kind))
        return records

    def blobs(self, source: str) -> list[Blob]:
        source_path = portable_path(source, "copy.source")
        exact = git(self.root, "ls-tree", "-z", self.revision, "--", source)
        require(bool(exact), f"upstream path does not exist at pinned revision: {source}")
        header = exact.split(b"\0", 1)[0].split(b"\t", 1)[0].decode("ascii")
        mode, kind, _object_id = header.split(" ")
        require(kind != "commit" and mode != "160000", f"upstream submodule is forbidden: {source}")
        records = self._tree_records(source)
        if kind == "blob":
            records = [(source, mode, kind)]
        require(bool(records), f"upstream copy root is empty: {source}")
        result = []
        for path, file_mode, _ in records:
            portable_path(path, "upstream tree")
            body = git(self.root, "show", f"{self.revision}:{path}")
            require(not body.startswith(LFS_HEADER), f"Git LFS pointer is forbidden: {path}")
            result.append(Blob(path, GIT_MODES[file_mode], body))
        return result

    def evidence(self, path: str) -> bytes:
        portable_path(path, "evidence.path")
        exact = git(self.root, "ls-tree", "-z", self.revision, "--", path)
        require(bool(exact), f"pinned evidence path does not exist: {path}")
        header = exact.split(b"\0", 1)[0].split(b"\t", 1)[0].decode("ascii")
        mode, kind, _object_id = header.split(" ")
        require(kind == "blob" and mode in GIT_MODES, f"evidence must be a regular file: {path}")
        body = git(self.root, "show", f"{self.revision}:{path}")
        require(not body.startswith(LFS_HEADER), f"Git LFS pointer is forbidden: {path}")
        return body


def load_recipe(root: Path, bridge_id: str) -> tuple[Path, dict[str, object]]:
    require(ID_RE.fullmatch(bridge_id) is not None, "bridge id must be lowercase ASCII")
    path = root / "bridges" / bridge_id / "bridge.yaml"
    require(path.is_file() and not path.is_symlink(), f"bridge recipe not found: {path}")
    try:
        recipe = yaml.load(path.read_text(encoding="utf-8"), Loader=UniqueKeyLoader)
        schema = json.loads(SCHEMA.read_text(encoding="utf-8"))
        jsonschema.Draft202012Validator(schema).validate(recipe)
    except (OSError, UnicodeError, yaml.YAMLError, json.JSONDecodeError, jsonschema.ValidationError) as error:
        raise BridgeError(f"invalid bridge recipe {path}: {error}") from error
    require(isinstance(recipe, dict), "bridge recipe must be an object")
    require(recipe["product_id"] == bridge_id, "recipe product_id must match its directory")
    require(recipe["output"] == f"plugins/{bridge_id}", "recipe output must be plugins/<product_id>")
    return path, recipe


def put_file(root: Path, relative: str, body: bytes, mode: int, origins: dict[str, str], origin: str) -> None:
    portable_path(relative, "output path")
    folded = relative.casefold()
    collision = next((name for name in origins if name.casefold() == folded and name != relative), None)
    require(collision is None, f"portable path collision: {collision!r} and {relative!r}")
    target = root.joinpath(*PurePosixPath(relative).parts)
    require(not any(parent.is_file() for parent in target.parents if parent != root.parent), f"file/directory path conflict: {relative}")
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(body)
    target.chmod(mode)
    origins[relative] = origin


def overlay_files(path: Path) -> list[tuple[str, bytes]]:
    require(path.is_dir() and not path.is_symlink(), f"overlay directory does not exist: {path}")
    result = []
    for item in sorted(path.rglob("*")):
        relative = item.relative_to(path).as_posix()
        require(not item.is_symlink(), f"overlay symlink is forbidden: {relative}")
        require(item.is_dir() or item.is_file(), f"overlay special file is forbidden: {relative}")
        if item.is_file():
            require(not (item.stat().st_mode & 0o111), f"executable overlay is forbidden: {relative}")
            portable_path(relative, "overlay path")
            body = item.read_bytes()
            require(not body.startswith(LFS_HEADER), f"overlay LFS pointer is forbidden: {relative}")
            result.append((relative, body))
    require(bool(result), "overlay must contain at least one file")
    return result


def validate_evidence(repository: PinnedRepository, entries: list[dict[str, str]], kind: str) -> dict[str, str]:
    result = {}
    seen = set()
    for entry in entries:
        path = entry["path"]
        require(path not in seen, f"duplicate {kind} path: {path}")
        seen.add(path)
        actual = sha256(repository.evidence(path))
        require(actual == entry["sha256"], f"{kind} changed at pinned path {path}: expected {entry['sha256']}, got {actual}")
        result[path] = actual
    return result


def validate_components(output: Path, recipe: dict[str, object]) -> dict[str, list[str]]:
    try:
        mcp_count, skill_count = validate_plugin(output)
    except ValidationError as error:
        raise BridgeError(str(error)) from error
    mcp_names = []
    if (output / "mcp.json").is_file():
        document = json.loads((output / "mcp.json").read_text(encoding="utf-8"))
        mcp_names = sorted(document["mcpServers"])
    skill_names = []
    skills = output / "skills"
    if skills.is_dir():
        skill_names = sorted(path.name for path in skills.iterdir() if path.is_dir())
    expected = recipe["components"]
    executables = sorted(
        path.relative_to(output).as_posix()
        for path in output.rglob("*")
        if path.is_file() and path.stat().st_mode & stat.S_IXUSR
    )
    require(
        executables == sorted(recipe["executable_paths"]),
        f"executable path expectation mismatch: expected {sorted(recipe['executable_paths'])}, got {executables}",
    )
    require(mcp_names == sorted(expected["mcp_servers"]), f"MCP component expectation mismatch: expected {sorted(expected['mcp_servers'])}, got {mcp_names}")
    require(skill_names == sorted(expected["skills"]), f"skill component expectation mismatch: expected {sorted(expected['skills'])}, got {skill_names}")
    require(mcp_count == len(mcp_names) and skill_count == len(skill_names), "validated component inventory mismatch")
    manifest = json.loads((output / "plugin.json").read_text(encoding="utf-8"))
    require(manifest["name"] == recipe["product_id"], "generated manifest name must match product_id")
    require(str(manifest.get("description", "")).startswith("Community package for "), "bridge description must begin with 'Community package for '")
    if "expected_version" in recipe:
        require(manifest.get("version") == recipe["expected_version"], "manifest version does not match expected_version")
    return {
        "mcp_servers": mcp_names,
        "skills": skill_names,
        "executables": executables,
    }


def assemble(root: Path, bridge_id: str, destination: Path, mirror_root: Path | None) -> dict[str, object]:
    recipe_path, recipe = load_recipe(root, bridge_id)
    upstream = recipe["upstream"]
    revision = upstream["revision"]
    require(SHA_RE.fullmatch(revision) is not None, "upstream revision must be a full lowercase SHA")
    repository = PinnedRepository(upstream["repository"], revision, mirror_root)
    origins: dict[str, str] = {}
    copied_sources: dict[str, str] = {}
    try:
        license_evidence = validate_evidence(repository, upstream["license"]["attribution_paths"], "license/attribution")
        provenance_evidence = validate_evidence(repository, upstream["provenance"]["paths"], "provenance")
        require(recipe["copy"] or provenance_evidence, "zero-copy bridge requires pinned provenance evidence")
        for operation in recipe["copy"]:
            source = operation["source"]
            destination_root = operation["destination"]
            source_path = portable_path(source, "copy.source")
            destination_path = portable_path(destination_root, "copy.destination")
            for blob in repository.blobs(source):
                blob_path = PurePosixPath(blob.path)
                relative = PurePosixPath() if blob_path == source_path else blob_path.relative_to(source_path)
                output_path = destination_path if str(relative) == "." else destination_path / relative
                output_name = output_path.as_posix()
                require(output_name not in origins, f"copy destination conflict: {output_name}")
                put_file(destination, output_name, blob.body, blob.mode, origins, f"upstream:{blob.path}")
                copied_sources[blob.path] = output_name

        replacements = {item["path"]: item["upstream_sha256"] for item in recipe["overlay_replacements"]}
        require(len(replacements) == len(recipe["overlay_replacements"]), "duplicate overlay replacement path")
        seen_replacements = set()
        overlay_root = recipe_path.parent / recipe["overlay"]
        for relative, body in overlay_files(overlay_root):
            if relative in origins:
                require(relative in replacements, f"overlay/copy conflict requires reviewed overlay_replacements entry: {relative}")
                copied_body = destination.joinpath(*PurePosixPath(relative).parts).read_bytes()
                require(sha256(copied_body) == replacements[relative], f"overlaid upstream content changed: {relative}")
                seen_replacements.add(relative)
            else:
                require(relative not in replacements, f"overlay replacement has no copied upstream path: {relative}")
            put_file(destination, relative, body, 0o644, origins, f"overlay:{relative}")
        require(seen_replacements == set(replacements), "not every reviewed overlay replacement was applied")

        if recipe["copy"]:
            attribution_sources = set(license_evidence)
            require(attribution_sources <= set(copied_sources), "redistributing bridge must copy every required attribution path")
            require((destination / "LICENSE").is_file(), "redistributing bridge must contain LICENSE")
            if any(PurePosixPath(path).name.upper().startswith("NOTICE") for path in attribution_sources):
                require((destination / "NOTICE").is_file(), "upstream NOTICE must be copied into package NOTICE")
        else:
            require((destination / "NOTICE").is_file(), "zero-copy bridge must include reviewed NOTICE attribution")

        validate_tree(destination)
        inventory = validate_components(destination, recipe)
        manifest_body = (destination / "plugin.json").read_bytes()
        return {
            "bridge_id": bridge_id,
            "product_id": recipe["product_id"],
            "distribution_id": recipe["distribution_id"],
            "package_path": recipe["output"],
            "overlay_path": f"bridges/{bridge_id}/{recipe['overlay']}",
            "upstream_repository": upstream["repository"],
            "upstream_revision": revision,
            "license": upstream["license"]["spdx"],
            "license_evidence": license_evidence,
            "provenance_evidence": provenance_evidence,
            "manifest_digest": sha256(manifest_body),
            "tree_digest_algorithm": "agentplugins-tree-sha256-v1",
            "tree_digest": package_tree_digest(destination),
            "components": inventory,
        }
    finally:
        repository.close()


def compare_trees(expected: Path, actual: Path) -> None:
    require(expected.is_dir(), f"committed bridge output is missing: {expected}")
    validate_tree(expected)
    expected_entries = {path.relative_to(expected).as_posix(): path for path in expected.rglob("*")}
    actual_entries = {path.relative_to(actual).as_posix(): path for path in actual.rglob("*")}
    require(set(expected_entries) == set(actual_entries), f"output path set differs: expected {sorted(expected_entries)}, got {sorted(actual_entries)}")
    for relative in sorted(expected_entries):
        left, right = expected_entries[relative], actual_entries[relative]
        require(left.is_dir() == right.is_dir(), f"entry type differs: {relative}")
        if left.is_file():
            require(left.read_bytes() == right.read_bytes(), f"file bytes differ: {relative}")
            left_exec = bool(left.stat().st_mode & stat.S_IXUSR)
            right_exec = bool(right.stat().st_mode & stat.S_IXUSR)
            require(left_exec == right_exec, f"executable mode differs: {relative}")


def recipe_ids(root: Path) -> list[str]:
    bridges = root / "bridges"
    if not bridges.is_dir():
        return []
    return sorted(path.name for path in bridges.iterdir() if path.is_dir() and (path / "bridge.yaml").is_file())


def build_one(root: Path, bridge_id: str, mirror_root: Path | None) -> dict[str, object]:
    _path, recipe = load_recipe(root, bridge_id)
    output = root / recipe["output"]
    with tempfile.TemporaryDirectory(prefix=f"bridge-{bridge_id}-", dir=root) as temporary:
        generated = Path(temporary) / bridge_id
        generated.mkdir()
        report = assemble(root, bridge_id, generated, mirror_root)
        backup = None
        if output.exists():
            backup = Path(temporary) / "previous"
            output.rename(backup)
        try:
            output.parent.mkdir(parents=True, exist_ok=True)
            generated.rename(output)
        except Exception:
            if backup is not None and not output.exists():
                backup.rename(output)
            raise
    return report


def check_all(root: Path, mirror_root: Path | None) -> list[dict[str, object]]:
    ids = recipe_ids(root)
    reports = []
    with tempfile.TemporaryDirectory(prefix="bridge-check-") as temporary:
        temporary_root = Path(temporary)
        for bridge_id in ids:
            destination = temporary_root / bridge_id
            destination.mkdir()
            report = assemble(root, bridge_id, destination, mirror_root)
            _path, recipe = load_recipe(root, bridge_id)
            compare_trees(root / recipe["output"], destination)
            reports.append(report)
    directory_source = root / "registry" / "directory.json"
    if directory_source.is_file():
        validate_bridge_bindings(
            read_object(directory_source), repository_root=root,
            build_reports=reports,
        )
    return reports


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=ROOT, help=argparse.SUPPRESS)
    parser.add_argument("--upstream-mirror", type=Path, help="offline owner/repo.git mirror root")
    subparsers = parser.add_subparsers(dest="command", required=True)
    build_parser = subparsers.add_parser("build", help="build one bridge")
    build_parser.add_argument("id")
    subparsers.add_parser("check", help="rebuild and verify all committed bridges")
    args = parser.parse_args(argv)
    root = args.root.resolve()
    try:
        if args.command == "build":
            reports = [build_one(root, args.id, args.upstream_mirror)]
        else:
            reports = check_all(root, args.upstream_mirror)
    except (BridgeError, RegistryError, OSError, ValueError, json.JSONDecodeError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1
    print(json.dumps({"status": "ok", "command": args.command, "bridges": reports}, sort_keys=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
