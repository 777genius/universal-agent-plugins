#!/usr/bin/env python3
"""Build the deterministic, Git-native public plugin registry.

External packages are treated strictly as data. This module downloads a pinned
GitHub archive, bounds and validates it, and never invokes package content.
"""

from __future__ import annotations

import argparse
import gzip
import hashlib
import json
import os
import re
import stat
import sys
import tarfile
import tempfile
import time
import unicodedata
import urllib.error
import urllib.request
from pathlib import Path, PurePosixPath
from urllib.parse import quote, urlsplit

import jsonschema

sys.path.insert(0, str(Path(__file__).resolve().parent))
from build_agentplugins_catalog import package_tree_digest
from portable_paths import (
    MAX_DEPTH as PORTABLE_MAX_DEPTH,
    MAX_FILES as PORTABLE_MAX_FILES,
    MAX_FILE_BYTES as PORTABLE_MAX_FILE_BYTES,
    MAX_TREE_BYTES as PORTABLE_MAX_TREE_BYTES,
    validate_segment,
)
from validate_catalog import ValidationError, validate_plugin


ROOT = Path(__file__).resolve().parents[1]
ENTRIES = ROOT / "registry" / "entries"
OUTPUT = ROOT / "registry" / "index.json"
DIRECTORY_SOURCE = ROOT / "registry" / "directory.json"
REVIEW_PREVIEW = ROOT / "registry" / "review-preview.json"
REVIEW_SEARCH = ROOT / "registry" / "review-search.json"
LEGACY_CATALOG_DIGESTS = {
    ROOT / "catalog" / "v1" / "catalog.json": "sha256:9ed64038a8a1b1eab6956008f94b3ffa16f1b6ddf01e8b2809b202656423f183",
    ROOT / "catalog" / "v2" / "catalog.json": "sha256:66199c87bd68c65e39d15aa2c5c6e6c7830c9b116d8ed3590123031b32357050",
    ROOT / "registry" / "index.json": "sha256:c38141953857be29383813e56e58383457c8b14ac8e2bdfcbcdec31bcd4b7207",
}
REPOSITORY_RE = re.compile(r"^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?/[a-z0-9](?:[a-z0-9._-]{0,98}[a-z0-9])?$")
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
CATEGORY_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
DESCRIPTOR_FIELDS = {"schema_version", "repository", "revision", "path", "categories"}
APPROVED_ARCHIVE_HOSTS = {"codeload.github.com"}
APPROVED_API_HOSTS = {"api.github.com"}
CONNECT_TIMEOUT_SECONDS = 15
API_TOTAL_SECONDS = 15
TOTAL_DOWNLOAD_SECONDS = 30
ARCHIVE_PROCESS_SECONDS = 30
MAX_API_RESPONSE_BYTES = 1 << 20
MAX_DOWNLOAD_BYTES = 25 << 20
MAX_ARCHIVE_BYTES = 300 << 20
MAX_EXTRACTED_BYTES = 128 << 20
MAX_FILES = 5_000
MAX_MEMBERS = 6_000
MAX_FILE_BYTES = 16 << 20
MAX_PATH_DEPTH = 32
MAX_CATEGORIES = 8
ICON_NAMES = {"chrome-devtools": "googlechrome.svg", "docker-hub": "docker.svg", "hubspot-crm": "hubspot.svg", "hubspot-developer": "hubspot.svg"}
CLIENT_IDS = ("codex", "chatgpt", "cursor", "copilot", "vscode", "kiro")
KIND_PRIORITY = {"upstream": 0, "community_bridge": 1, "community": 2}
DIRECTORY_TREE_DIGEST_ALGORITHM = "agentplugins-tree-sha256-v1"
DIRECTORY_TREE_DIGEST_DOMAIN = b"agentplugins.package-tree\x00sha256\x00v1"
DIRECTORY_MINIMUM_INSTALLER_VERSION = "0.1.8"


class RegistryError(Exception):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RegistryError(message)


def digest_bytes(body: bytes) -> str:
    return "sha256:" + hashlib.sha256(body).hexdigest()


def _directory_tree_digest_entries(entries: list[tuple[bytes, bytes, bytes, bytes, bytes]]) -> str:
    """Hash already-normalized entries with the Go v1 byte framing."""
    ordered = sorted(entries, key=lambda entry: entry[0])
    digest = hashlib.sha256()
    digest.update(len(DIRECTORY_TREE_DIGEST_DOMAIN).to_bytes(8, "big"))
    digest.update(DIRECTORY_TREE_DIGEST_DOMAIN)
    for relative, kind, mode, target, content in ordered:
        for field in (b"entry", relative, kind, mode, target):
            digest.update(len(field).to_bytes(8, "big"))
            digest.update(field)
        digest.update(len(content).to_bytes(8, "big"))
        if kind == b"file":
            digest.update(content)
    return "sha256:" + digest.hexdigest()


def directory_tree_digest(root: Path) -> str:
    """Reproduce the Go agentplugins-tree-sha256-v1 package digest exactly.

    Directory publishing remains stricter than the Go snapshotter by rejecting
    every symlink. The byte framing below is still the Go contract for every
    file, directory, and executable mode that Directory publishing accepts.
    """
    entries: list[tuple[bytes, bytes, bytes, bytes, bytes]] = []
    seen: dict[str, str] = {}
    file_count = 0
    total_bytes = 0
    if root.is_symlink() or not root.is_dir():
        raise ValueError("portable package root must be a real directory")
    def raise_walk_error(error: OSError) -> None:
        raise error

    for current, directory_names, file_names in os.walk(root, topdown=True, onerror=raise_walk_error, followlinks=False):
        current_path = Path(current)
        if current_path == root:
            # filepath.WalkDir excludes only the root metadata entries. A
            # nested or differently-cased spelling remains invalid below.
            directory_names[:] = [
                name for name in directory_names
                if name != ".git" and not (name == ".plugin-kit-ai.lock" and (root / name).is_symlink())
            ]
            file_names = [name for name in file_names if name not in {".git", ".plugin-kit-ai.lock"}]
        for name in [*directory_names, *file_names]:
            path = current_path / name
            relative_path = path.relative_to(root)
            relative = relative_path.as_posix()
            if path.is_symlink():
                raise ValueError(f"portable package contains a symlink: {relative!r}")
            path_mode = path.stat().st_mode
            is_directory = stat.S_ISDIR(path_mode)
            is_file = stat.S_ISREG(path_mode)
            if not (is_directory or is_file):
                raise ValueError(f"portable package contains a special file: {relative!r}")
            if len(relative_path.parts) > PORTABLE_MAX_DEPTH:
                raise ValueError(f"portable package path exceeds depth {PORTABLE_MAX_DEPTH}: {relative!r}")
            for part in relative_path.parts:
                validate_segment(part)
                if part.casefold() == ".git":
                    raise ValueError(f"portable package contains reserved Git metadata path: {relative!r}")
            if relative.casefold() == ".plugin-kit-ai.lock":
                raise ValueError(f"portable package contains reserved ownership-marker path: {relative!r}")
            folded = relative.casefold()
            previous = seen.get(folded)
            if previous is not None and previous != relative:
                raise ValueError(f"portable path collision: {previous!r} and {relative!r}")
            seen[folded] = relative
            relative_bytes = relative.encode("utf-8")
            if is_directory:
                entries.append((relative_bytes, b"directory", b"040000", b"", b""))
                continue
            file_count += 1
            if file_count > PORTABLE_MAX_FILES:
                raise ValueError(f"portable package exceeds {PORTABLE_MAX_FILES} files")
            size = path.stat().st_size
            if size > PORTABLE_MAX_FILE_BYTES:
                raise ValueError(f"portable package file exceeds {PORTABLE_MAX_FILE_BYTES} bytes: {relative!r}")
            total_bytes += size
            if total_bytes > PORTABLE_MAX_TREE_BYTES:
                raise ValueError(f"portable package exceeds {PORTABLE_MAX_TREE_BYTES} total bytes")
            mode = b"100755" if path_mode & 0o111 else b"100644"
            entries.append((relative_bytes, b"file", mode, b"", path.read_bytes()))
    return _directory_tree_digest_entries(entries)


def parse_json_bytes(body: bytes, source: str) -> object:
    def unique_object(pairs):  # type: ignore[no-untyped-def]
        result = {}
        normalized_keys = set()
        for key, item in pairs:
            require(key not in result, f"{source}: duplicate JSON key {key!r}")
            normalized = unicodedata.normalize("NFC", key).casefold()
            require(normalized not in normalized_keys, f"{source}: case/Unicode-colliding JSON key {key!r}")
            normalized_keys.add(normalized)
            result[key] = item
        return result

    def reject_constant(value: str) -> None:
        raise RegistryError(
            f"{source}: non-finite JSON number {value!r} is forbidden"
        )

    try:
        return json.loads(
            body.decode("utf-8"),
            object_pairs_hook=unique_object,
            parse_constant=reject_constant,
        )
    except (UnicodeError, json.JSONDecodeError) as error:
        raise RegistryError(f"{source}: invalid UTF-8 JSON: {error}") from error


def read_json(path: Path) -> object:
    try:
        body = path.read_bytes()
    except OSError as error:
        raise RegistryError(f"{path}: cannot read JSON: {error}") from error
    return parse_json_bytes(body, str(path))


def read_object(path: Path) -> dict[str, object]:
    value = read_json(path)
    require(isinstance(value, dict), f"{path}: top level must be an object")
    return value


def validate_repository(value: object) -> str:
    require(isinstance(value, str) and REPOSITORY_RE.fullmatch(value) is not None, "repository must be canonical lowercase GitHub owner/repo")
    require(not value.endswith(".git"), "repository must not use a .git suffix")
    return value


def validate_registry_path(value: object) -> str:
    require(isinstance(value, str) and value.isascii(), "path must be non-empty ASCII")
    require(len(value) <= 512, "path exceeds 512 characters")
    require(value == unicodedata.normalize("NFC", value), "path must be NFC normalized")
    require("\\" not in value and "%" not in value, "path contains an ambiguous separator or escape")
    path = PurePosixPath(value)
    require(value and not path.is_absolute() and path.as_posix() == value, "path must be a normalized relative POSIX path")
    require(len(path.parts) <= MAX_PATH_DEPTH, f"path exceeds depth {MAX_PATH_DEPTH}")
    for segment in path.parts:
        try:
            validate_segment(segment)
        except ValueError as error:
            raise RegistryError(str(error)) from error
    require(".git" not in path.parts, "path must not address Git metadata")
    return value


def validate_descriptor(path: Path) -> dict[str, object]:
    descriptor = read_object(path)
    require(set(descriptor) == DESCRIPTOR_FIELDS, f"{path}: descriptor must contain only {sorted(DESCRIPTOR_FIELDS)}")
    require(descriptor["schema_version"] == 1, f"{path}: schema_version must be 1")
    repository = validate_repository(descriptor["repository"])
    revision = descriptor["revision"]
    require(isinstance(revision, str) and SHA_RE.fullmatch(revision) is not None, f"{path}: revision must be a full lowercase commit SHA")
    plugin_path = validate_registry_path(descriptor["path"])
    categories = descriptor["categories"]
    require(isinstance(categories, list) and 1 <= len(categories) <= MAX_CATEGORIES, f"{path}: categories must contain 1-{MAX_CATEGORIES} values")
    require(all(isinstance(item, str) and len(item) <= 40 and CATEGORY_RE.fullmatch(item) for item in categories), f"{path}: invalid category")
    require(categories == sorted(set(categories)), f"{path}: categories must be unique and sorted")
    name = path.stem
    require(path.name == f"{name}.json" and name.isascii() and re.fullmatch(r"(?!.*(?:--|\.\.))[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?", name) is not None, f"{path}: filename must be a normalized plugin name")
    require(PurePosixPath(plugin_path).name == name, f"{path}: path directory must match descriptor filename")
    return {"name": name, "repository": repository, "revision": revision, "path": plugin_path, "categories": categories}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):  # type: ignore[no-untyped-def]
        raise RegistryError(f"network redirect is forbidden ({code})")


def commit_api_url(repository: str, revision: str) -> str:
    url = f"https://api.github.com/repos/{quote(repository, safe='/')}/git/commits/{revision}"
    parsed = urlsplit(url)
    require(parsed.scheme == "https" and parsed.hostname in APPROVED_API_HOSTS and parsed.username is None and parsed.password is None and not parsed.query and not parsed.fragment, "unsafe GitHub API URL")
    return url


def resolve_commit(repository: str, revision: str, opener=None) -> None:  # type: ignore[no-untyped-def]
    url = commit_api_url(repository, revision)
    opener = opener or urllib.request.build_opener(NoRedirect())
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "uap-registry-builder/1",
        "X-GitHub-Api-Version": "2022-11-28",
    }
    token = os.environ.get("GITHUB_TOKEN")
    if token:
        headers["Authorization"] = f"Bearer {token}"
    request = urllib.request.Request(url, headers=headers)
    started = time.monotonic()
    try:
        response = opener.open(request, timeout=CONNECT_TIMEOUT_SECONDS)
        with response:
            require(response.status == 200, f"GitHub commit lookup returned HTTP {response.status}")
            require(response.geturl() == url, "GitHub commit response URL mismatch")
            final = urlsplit(response.geturl())
            require(final.scheme == "https" and final.hostname in APPROVED_API_HOSTS and final.username is None and final.password is None and not final.query and not final.fragment, "GitHub commit response URL is not approved")
            length = response.headers.get("Content-Length")
            if length is not None:
                require(length.isascii() and length.isdigit() and int(length) <= MAX_API_RESPONSE_BYTES, "GitHub commit response Content-Length exceeds limit")
            chunks = []
            total = 0
            while True:
                require(time.monotonic() - started <= API_TOTAL_SECONDS, "GitHub commit lookup exceeded total time limit")
                chunk = response.read(min(64 << 10, MAX_API_RESPONSE_BYTES - total + 1))
                if not chunk:
                    break
                total += len(chunk)
                require(total <= MAX_API_RESPONSE_BYTES, "GitHub commit response exceeds size limit")
                chunks.append(chunk)
        value = parse_json_bytes(b"".join(chunks), "GitHub commit response")
        require(isinstance(value, dict), "GitHub commit response must be an object")
        require(value.get("sha") == revision, "GitHub commit response SHA does not exactly match revision")
        require(value.get("url") == url, "GitHub commit object URL does not exactly match lookup URL")
        tree = value.get("tree")
        require(isinstance(tree, dict) and isinstance(tree.get("sha"), str) and SHA_RE.fullmatch(tree["sha"]) is not None, "GitHub response is not a Git commit object")
        parents = value.get("parents")
        require(isinstance(parents, list) and all(isinstance(parent, dict) and isinstance(parent.get("sha"), str) and SHA_RE.fullmatch(parent["sha"]) is not None for parent in parents), "GitHub response is not a Git commit object")
    except RegistryError:
        raise
    except (OSError, urllib.error.URLError) as error:
        raise RegistryError(f"GitHub commit lookup failed closed: {error}") from error


def archive_url(repository: str, revision: str) -> str:
    url = f"https://codeload.github.com/{quote(repository, safe='/')}/tar.gz/{revision}"
    parsed = urlsplit(url)
    require(parsed.scheme == "https" and parsed.hostname in APPROVED_ARCHIVE_HOSTS and parsed.username is None and parsed.password is None and not parsed.query and not parsed.fragment, "unsafe archive URL")
    return url


def download_archive(repository: str, revision: str, destination: Path, opener=None) -> None:  # type: ignore[no-untyped-def]
    url = archive_url(repository, revision)
    opener = opener or urllib.request.build_opener(NoRedirect())
    request = urllib.request.Request(url, headers={"Accept": "application/x-gzip", "User-Agent": "uap-registry-builder/1"})
    started = time.monotonic()
    try:
        response = opener.open(request, timeout=CONNECT_TIMEOUT_SECONDS)
        with response, destination.open("wb") as output:
            final = urlsplit(response.geturl())
            require(response.status == 200, f"archive download returned HTTP {response.status}")
            require(final.scheme == "https" and final.hostname in APPROVED_ARCHIVE_HOSTS and response.geturl() == url, "archive response URL is not approved")
            length = response.headers.get("Content-Length")
            if length is not None:
                require(length.isascii() and length.isdigit() and int(length) <= MAX_DOWNLOAD_BYTES, "archive Content-Length exceeds limit")
            total = 0
            while True:
                require(time.monotonic() - started <= TOTAL_DOWNLOAD_SECONDS, "archive download exceeded total time limit")
                chunk = response.read(min(64 << 10, MAX_DOWNLOAD_BYTES - total + 1))
                if not chunk:
                    break
                total += len(chunk)
                require(total <= MAX_DOWNLOAD_BYTES, "archive download exceeds compressed size limit")
                output.write(chunk)
    except RegistryError:
        raise
    except (OSError, urllib.error.URLError) as error:
        raise RegistryError(f"archive download failed closed: {error}") from error


def decompress_archive(compressed: Path, expanded: Path) -> None:
    total = 0
    started = time.monotonic()
    try:
        with gzip.open(compressed, "rb") as source, expanded.open("wb") as output:
            while True:
                require(time.monotonic() - started <= ARCHIVE_PROCESS_SECONDS, "archive decompression exceeded time limit")
                chunk = source.read(min(1 << 20, MAX_ARCHIVE_BYTES - total + 1))
                if not chunk:
                    break
                total += len(chunk)
                require(total <= MAX_ARCHIVE_BYTES, "expanded archive exceeds limit")
                output.write(chunk)
    except (OSError, EOFError) as error:
        raise RegistryError(f"invalid gzip archive: {error}") from error


def safe_member_path(name: str) -> PurePosixPath:
    require(name.isascii() and name == unicodedata.normalize("NFC", name), "archive path must be normalized ASCII")
    require("\\" not in name and "%" not in name and not name.startswith("/"), "archive contains an ambiguous or absolute path")
    path = PurePosixPath(name)
    require(path.as_posix() == name.rstrip("/") and path.parts and ".." not in path.parts, "archive contains a non-normalized path")
    require(len(path.parts) <= MAX_PATH_DEPTH + 1, "archive path exceeds depth limit")
    for segment in path.parts:
        try:
            validate_segment(segment)
        except ValueError as error:
            raise RegistryError(str(error)) from error
    return path


def extract_package(expanded: Path, plugin_path: str, destination: Path) -> None:
    prefix_parts: tuple[str, ...] | None = None
    selected: list[tuple[tarfile.TarInfo, tuple[str, ...]]] = []
    seen: set[str] = set()
    total = archive_files = archive_members = 0
    started = time.monotonic()
    try:
        with tarfile.open(expanded, mode="r:") as archive:
            for member in archive:
                require(time.monotonic() - started <= ARCHIVE_PROCESS_SECONDS, "archive validation exceeded time limit")
                archive_members += 1
                require(archive_members <= MAX_MEMBERS, "archive exceeds member-count limit")
                path = safe_member_path(member.name)
                require(not (member.issym() or member.islnk()) and (member.isdir() or member.isfile()), f"archive contains link or special file: {member.name!r}")
                require(not member.sparse and not any("sparse" in key.casefold() for key in member.pax_headers), f"archive contains a sparse file: {member.name!r}")
                folded = path.as_posix().casefold()
                require(folded not in seen, "archive contains duplicate or case-colliding paths")
                seen.add(folded)
                if member.isfile():
                    archive_files += 1
                    require(archive_files <= MAX_FILES, "archive exceeds file-count limit")
                    require(0 <= member.size <= MAX_FILE_BYTES, "archive file exceeds size limit")
                if prefix_parts is None:
                    prefix_parts = (path.parts[0],)
                require(path.parts[:1] == prefix_parts, "archive has multiple top-level roots")
                relative = path.parts[1:]
                target_prefix = PurePosixPath(plugin_path).parts
                if relative[:len(target_prefix)] != target_prefix:
                    continue
                package_relative = relative[len(target_prefix):]
                if not package_relative:
                    require(member.isdir(), "plugin path is not a directory")
                    continue
                require(len(package_relative) <= MAX_PATH_DEPTH, "package path exceeds depth limit")
                if member.isfile():
                    total += member.size
                    require(total <= MAX_EXTRACTED_BYTES, "package exceeds extracted-size limit")
                selected.append((member, package_relative))
            require(selected, "descriptor path does not exist in pinned archive")
            for member, relative in selected:
                target = destination.joinpath(*relative)
                if member.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                    continue
                target.parent.mkdir(parents=True, exist_ok=True)
                source = archive.extractfile(member)
                require(source is not None, f"cannot read archive member {member.name!r}")
                remaining = member.size
                with target.open("wb") as output:
                    while remaining:
                        require(time.monotonic() - started <= ARCHIVE_PROCESS_SECONDS, "archive extraction exceeded time limit")
                        chunk = source.read(min(64 << 10, remaining))
                        require(bool(chunk), f"truncated archive member {member.name!r}")
                        remaining -= len(chunk)
                        output.write(chunk)
                os.chmod(target, 0o755 if member.mode & 0o111 else 0o644)
    except (tarfile.TarError, OSError) as error:
        raise RegistryError(f"invalid tar archive: {error}") from error


def component_names(root: Path, manifest: dict[str, object]) -> list[str]:
    result = []
    if manifest.get("extensions"):
        result.append("extensions")
    if (root / "mcp.json").is_file():
        result.append("mcp")
    if (root / "skills").is_dir():
        result.append("skills")
    return sorted(result)


def validate_schema(document: object, document_path: Path, schema_name: str) -> None:
    schema_path = ROOT / "schemas" / "1.0.0" / f"{schema_name}.schema.json"
    schema = read_object(schema_path)
    try:
        validator = jsonschema.Draft202012Validator(schema)
        error = next(validator.iter_errors(document), None)
    except jsonschema.SchemaError as schema_error:
        raise RegistryError(f"{schema_path}: invalid vendored schema: {schema_error.message}") from schema_error
    if error is not None:
        location = "/".join(str(part) for part in error.absolute_path) or "<root>"
        raise RegistryError(f"{document_path}: Agent Plugins 1.0 schema error at {location}: {error.message}")


def package_fields(root: Path, categories: list[str]) -> dict[str, object]:
    # json.load silently accepts duplicate object keys. Parse every submitted
    # JSON file with the registry's fail-closed reader before schema validation.
    for json_path in sorted(root.rglob("*.json")):
        read_json(json_path)
    manifest_path = root / "plugin.json"
    manifest = read_object(manifest_path)
    validate_schema(manifest, manifest_path, "plugin")
    mcp_path = root / "mcp.json"
    if mcp_path.is_file():
        validate_schema(read_object(mcp_path), mcp_path, "mcp")
    try:
        validate_plugin(root)
    except (ValidationError, ValueError) as error:
        raise RegistryError(str(error)) from error
    license_value = manifest.get("license")
    require(isinstance(license_value, str) and license_value.strip(), f"{manifest_path}: license required")
    author = manifest.get("author")
    require(isinstance(author, dict) and isinstance(author.get("name"), str) and author["name"], f"{manifest_path}: author metadata required")
    return {
        "name": manifest["name"], "version": manifest["version"], "description": manifest["description"],
        "author": author, "license": license_value, "categories": sorted(set(categories)),
        "keywords": sorted(set(manifest.get("keywords", []))), "components": component_names(root, manifest),
        "manifest_sha256": digest_bytes(manifest_path.read_bytes()), "tree_sha256": package_tree_digest(root),
    }


def canonical_manifest_repository(value: object) -> str | None:
    if not isinstance(value, str):
        return None
    parsed = urlsplit(value)
    if parsed.scheme != "https" or parsed.hostname != "github.com" or parsed.username or parsed.password or parsed.query or parsed.fragment:
        return None
    candidate = parsed.path.strip("/")
    return candidate if REPOSITORY_RE.fullmatch(candidate) and not candidate.endswith(".git") else None


def external_entry(descriptor: dict[str, object], opener=None) -> dict[str, object]:  # type: ignore[no-untyped-def]
    with tempfile.TemporaryDirectory(prefix="uap-registry-") as temporary:
        temp = Path(temporary)
        compressed, expanded, package = temp / "source.tar.gz", temp / "source.tar", temp / str(descriptor["name"])
        package.mkdir()
        resolve_commit(str(descriptor["repository"]), str(descriptor["revision"]), opener)
        download_archive(str(descriptor["repository"]), str(descriptor["revision"]), compressed, opener)
        decompress_archive(compressed, expanded)
        extract_package(expanded, str(descriptor["path"]), package)
        fields = package_fields(package, list(descriptor["categories"]))
        require(fields["name"] == descriptor["name"], "manifest name must match descriptor filename")
        manifest = read_object(package / "plugin.json")
        require(canonical_manifest_repository(manifest.get("repository")) == descriptor["repository"], "manifest repository must exactly match the pinned descriptor repository")
        source = {"repository": descriptor["repository"], "revision": descriptor["revision"], "path": descriptor["path"], "manifest_sha256": fields.pop("manifest_sha256"), "tree_sha256": fields.pop("tree_sha256")}
        result = {
            **fields,
            "source": source,
            "install_source": f"{descriptor['repository']}@{descriptor['revision']}//{descriptor['path']}",
            "built_in": False,
            "client_support": {"resolution": "install_time", "clients": list(CLIENT_IDS)},
            "validation": {"level": "schema_only", "schema": "agent-plugins-1.0", "runtime_evidence": []},
        }
        return result


def builtin_entries() -> list[dict[str, object]]:
    catalog = read_object(ROOT / "catalog" / "v2" / "catalog.json")
    repository, revision = validate_repository(catalog.get("repository")), catalog.get("revision")
    require(isinstance(revision, str) and SHA_RE.fullmatch(revision) is not None, "catalog revision is not immutable")
    catalog_by_name = {item["name"]: item for item in catalog.get("plugins", []) if isinstance(item, dict) and isinstance(item.get("name"), str)}
    result = []
    for root in sorted(path for path in (ROOT / "plugins").iterdir() if path.is_dir()):
        fields = package_fields(root, [])
        name = str(fields["name"])
        require(name in catalog_by_name, f"{name}: missing from catalog/v2")
        catalog_item = catalog_by_name[name]
        require(catalog_item.get("source_path") == f"plugins/{name}", f"{name}: catalog source mismatch")
        require(catalog_item.get("manifest_digest") == fields["manifest_sha256"], f"{name}: local manifest differs from the pinned catalog revision")
        require(catalog_item.get("tree_digest") == fields["tree_sha256"], f"{name}: local tree differs from the pinned catalog revision")
        compatibility = catalog_item.get("compatibility")
        require(isinstance(compatibility, dict) and compatibility, f"{name}: catalog compatibility is missing")
        require(set(compatibility).issubset(CLIENT_IDS), f"{name}: catalog compatibility contains an unknown client")
        supported_clients = [client for client in CLIENT_IDS if client in compatibility]
        evidence = sorted(client for client, value in compatibility.items() if isinstance(value, dict) and value.get("verification") == "tested")
        fields["categories"] = sorted(set(fields["keywords"]))
        source = {"repository": repository, "revision": revision, "path": f"plugins/{name}", "manifest_sha256": fields.pop("manifest_sha256"), "tree_sha256": fields.pop("tree_sha256")}
        item = {
            **fields,
            "source": source,
            "install_source": name,
            "built_in": True,
            "client_support": {"resolution": "catalog", "clients": supported_clients},
            "validation": {"level": "runtime_evidence" if evidence else "schema_only", "schema": "agent-plugins-1.0", "runtime_evidence": evidence},
        }
        icon_name = ICON_NAMES.get(name, name + ".svg")
        icon_path = ROOT / "assets" / "plugin-icons" / icon_name
        if not icon_path.is_file():
            png = icon_path.with_suffix(".png")
            icon_path = png if png.is_file() else icon_path
        if icon_path.is_file():
            icon_digest = digest_bytes(icon_path.read_bytes())
            source["icon_sha256"] = icon_digest
            item["icon"] = {"path": icon_path.relative_to(ROOT).as_posix(), "sha256": icon_digest}
        result.append(item)
    require(len(result) == 26, f"expected 26 built-ins, found {len(result)}")
    return result


def build(opener=None) -> dict[str, object]:  # type: ignore[no-untyped-def]
    plugins = builtin_entries()
    seen = {str(item["name"]).casefold() for item in plugins}
    if ENTRIES.exists():
        descriptor_paths = []
        for candidate in sorted(ENTRIES.iterdir()):
            require(candidate.name == ".gitkeep" or (candidate.suffix == ".json" and candidate.is_file() and not candidate.is_symlink()), f"{candidate}: registry entries may contain only regular JSON descriptors")
            if candidate.suffix == ".json":
                descriptor_paths.append(candidate)
        for descriptor_path in descriptor_paths:
            descriptor = validate_descriptor(descriptor_path)
            normalized = str(descriptor["name"]).casefold()
            require(normalized not in seen, f"duplicate normalized plugin name: {descriptor['name']}")
            item = external_entry(descriptor, opener)
            require(str(item["name"]).casefold() not in seen, f"duplicate normalized manifest name: {item['name']}")
            seen.add(normalized)
            plugins.append(item)
    plugins.sort(key=lambda item: str(item["name"]))
    return {"schema_version": 1, "plugins": plugins}


def _schema(name: str) -> dict[str, object]:
    return read_object(ROOT / "schemas" / name)


def _validate_document(document: object, schema_name: str, label: str) -> None:
    schema = _schema(schema_name)
    try:
        jsonschema.Draft202012Validator.check_schema(schema)
        error = next(jsonschema.Draft202012Validator(schema).iter_errors(document), None)
    except jsonschema.SchemaError as schema_error:
        raise RegistryError(f"schemas/{schema_name}: invalid schema: {schema_error.message}") from schema_error
    if error is not None:
        location = "/".join(str(part) for part in error.absolute_path) or "<root>"
        raise RegistryError(f"{label}: schema error at {location}: {error.message}")


def _validate_source_schema(source: object) -> None:
    schema_names = (
        "directory-source.schema.json", "directory-product.schema.json",
        "directory-distribution.schema.json", "directory-evidence.schema.json",
    )
    schemas = [_schema(name) for name in schema_names]
    store = {schema["$id"]: schema for schema in schemas}
    try:
        resolver = jsonschema.RefResolver.from_schema(schemas[0], store=store)
        error = next(jsonschema.Draft202012Validator(schemas[0], resolver=resolver).iter_errors(source), None)
    except (jsonschema.SchemaError, jsonschema.exceptions.RefResolutionError) as schema_error:
        raise RegistryError(f"Directory source schema cannot be resolved locally: {schema_error}") from schema_error
    if error is not None:
        location = "/".join(str(part) for part in error.absolute_path) or "<root>"
        raise RegistryError(f"Directory source: schema error at {location}: {error.message}")


def _display_name(name: str) -> str:
    special = {"api": "API", "crm": "CRM", "github": "GitHub", "gitlab": "GitLab"}
    return " ".join(special.get(part, part.capitalize()) for part in name.split("-"))


def migrated_directory_source() -> dict[str, object]:
    """Create the one-time, reviewable migration from the byte-frozen catalog."""
    catalog = read_object(ROOT / "catalog" / "v2" / "catalog.json")
    published_at = catalog.get("published_at")
    require(isinstance(published_at, str), "catalog/v2 published_at is missing")
    products: list[dict[str, object]] = []
    distributions: list[dict[str, object]] = []
    for item in builtin_entries():
        name = str(item["name"])
        distribution_id = f"777genius/{name}"
        components = list(item["components"])
        source = dict(item["source"])
        compatibility = next(
            value["compatibility"]
            for value in catalog["plugins"]
            if isinstance(value, dict) and value.get("name") == name
        )
        targets = []
        for client in CLIENT_IDS:
            if client not in compatibility:
                continue
            package = compatibility[client]["package"]
            delivery = "manual_activation" if client == "chatgpt" else ("prepared" if package == "prepared" else "managed")
            target = {"client": client, "scopes": ["user"], "delivery": delivery}
            if client == "chatgpt":
                binding = compatibility[client]["app_binding"]
                target["app_binding"] = {key: binding[key] for key in ("app_key", "id", "mcp_server")}
            targets.append(target)
        minimum = {
            "skills": "required" if "skills" in components else "optional",
            "mcp": "required" if "mcp" in components else "optional",
        }
        product: dict[str, object] = {
            "schema_version": 1,
            "id": name,
            "display_name": _display_name(name),
            "description": item["description"],
            "manifest_name": name,
            "aliases": [name],
            "reserved_aliases": [name],
            "categories": item["categories"] or ["agent-plugins"],
            "minimum_capabilities": minimum,
            "default_distribution": distribution_id,
            "distributions": [distribution_id],
        }
        if "icon" in item:
            product["icon"] = {"path": item["icon"]["path"], "digest": item["icon"]["sha256"]}
        products.append(product)
        distributions.append({
            "schema_version": 1,
            "id": distribution_id,
            "product_id": name,
            "kind": "community",
            "status": "active",
            "packager": "777genius",
            "releases": [{
                "sequence": 1,
                "package_version": item["version"],
                "manifest_name": name,
                "agent_plugins_schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                "package_source": {"repository": source["repository"], "revision": source["revision"], "path": source["path"]},
                "tree_digest_algorithm": DIRECTORY_TREE_DIGEST_ALGORITHM,
                "tree_digest": directory_tree_digest(ROOT / source["path"]),
                "manifest_digest": source["manifest_sha256"],
                "components": components,
                "published_at": published_at,
            }],
            "release_policies": [{
                "release_sequence": 1,
                "status": "active",
                "minimum_installer_version": DIRECTORY_MINIMUM_INSTALLER_VERSION,
                "targets": targets,
                "current_evidence": [],
            }],
        })
    require(len(products) == 26, f"migration must contain exactly 26 products, found {len(products)}")
    return {"schema_version": 1, "products": products, "distributions": distributions, "evidence": []}


def load_directory_source(path: Path = DIRECTORY_SOURCE) -> dict[str, object]:
    source = read_object(path)
    require(set(source) == {"schema_version", "products", "distributions", "evidence"}, f"{path}: unexpected top-level fields")
    require(source.get("schema_version") == 1, f"{path}: schema_version must be 1")
    for key in ("products", "distributions", "evidence"):
        require(isinstance(source.get(key), list), f"{path}: {key} must be an array")
    return source


def _policy_for(distribution: dict[str, object], sequence: int) -> dict[str, object]:
    policies = [policy for policy in distribution["release_policies"] if policy["release_sequence"] == sequence]
    require(len(policies) == 1, f"{distribution['id']}: release {sequence} must have exactly one mutable policy")
    return policies[0]


def validate_directory(source: dict[str, object], *, verify_packages: bool = True) -> None:
    _validate_source_schema(source)
    products = source["products"]
    distributions = source["distributions"]
    evidence = source["evidence"]
    for index, product in enumerate(products):
        _validate_document(product, "directory-product.schema.json", f"product[{index}]")
    for index, distribution in enumerate(distributions):
        _validate_document(distribution, "directory-distribution.schema.json", f"distribution[{index}]")
    for index, observation in enumerate(evidence):
        _validate_document(observation, "directory-evidence.schema.json", f"evidence[{index}]")
    product_ids = [product["id"] for product in products]
    distribution_ids = [distribution["id"] for distribution in distributions]
    evidence_ids = [observation["id"] for observation in evidence]
    require(product_ids == sorted(product_ids) and len(set(product_ids)) == len(product_ids), "products must have unique sorted IDs")
    require(distribution_ids == sorted(distribution_ids) and len(set(distribution_ids)) == len(distribution_ids), "distributions must have unique sorted IDs")
    require(evidence_ids == sorted(evidence_ids) and len(set(evidence_ids)) == len(evidence_ids), "evidence must have unique sorted IDs")
    products_by_id = {product["id"]: product for product in products}
    distributions_by_id = {distribution["id"]: distribution for distribution in distributions}
    evidence_by_id = {observation["id"]: observation for observation in evidence}
    alias_owner: dict[str, str] = {}
    for product in products:
        require(product["aliases"] == sorted(product["aliases"]), f"{product['id']}: aliases must be sorted")
        require(product["reserved_aliases"] == sorted(product["reserved_aliases"]), f"{product['id']}: reserved_aliases must be sorted")
        require(set(product["aliases"]).issubset(product["reserved_aliases"]), f"{product['id']}: active aliases must remain reserved")
        require(product["categories"] == sorted(product["categories"]), f"{product['id']}: categories must be sorted")
        for alias in product["reserved_aliases"]:
            require(alias not in alias_owner, f"reserved alias {alias!r} is owned by both {alias_owner.get(alias)} and {product['id']}")
            alias_owner[alias] = product["id"]
        listed = product["distributions"]
        require(listed == sorted(listed), f"{product['id']}: distributions must be sorted")
        require(product["default_distribution"] in listed, f"{product['id']}: default distribution is not listed")
        for distribution_id in listed:
            require(distribution_id in distributions_by_id, f"{product['id']}: unknown distribution {distribution_id}")
            require(distributions_by_id[distribution_id]["product_id"] == product["id"], f"{distribution_id}: product ownership mismatch")
    for distribution in distributions:
        product_id = distribution["product_id"]
        require(product_id in products_by_id and distribution["id"] in products_by_id[product_id]["distributions"], f"{distribution['id']}: distribution is not owned by its product")
        releases = distribution["releases"]
        sequences = [release["sequence"] for release in releases]
        require(sequences == sorted(sequences) and len(set(sequences)) == len(sequences), f"{distribution['id']}: release sequences must be unique and increasing")
        require([policy["release_sequence"] for policy in distribution["release_policies"]] == sequences, f"{distribution['id']}: policies must be sorted one-for-one with releases")
        for release in releases:
            sequence = release["sequence"]
            require(release["manifest_name"] == products_by_id[product_id]["manifest_name"], f"{distribution['id']}@{sequence}: manifest identity mismatch")
            require(release["tree_digest_algorithm"] == DIRECTORY_TREE_DIGEST_ALGORITHM, f"{distribution['id']}@{sequence}: unsupported tree digest algorithm")
            require(release["components"] == sorted(release["components"]), f"{distribution['id']}@{sequence}: components must be sorted")
            policy = _policy_for(distribution, sequence)
            target_ids = [target["client"] for target in policy["targets"]]
            require(target_ids == [client for client in CLIENT_IDS if client in target_ids] and len(set(target_ids)) == len(target_ids), f"{distribution['id']}@{sequence}: targets must be unique and in canonical order")
            require(policy["current_evidence"] == sorted(policy["current_evidence"]), f"{distribution['id']}@{sequence}: evidence pointers must be sorted")
            current_tuples: set[tuple[object, ...]] = set()
            for evidence_id in policy["current_evidence"]:
                require(evidence_id in evidence_by_id, f"{distribution['id']}@{sequence}: unknown evidence {evidence_id}")
                observation = evidence_by_id[evidence_id]
                require(observation["distribution_id"] == distribution["id"] and observation["release_sequence"] == sequence and observation["package_tree_digest"] == release["tree_digest"], f"{evidence_id}: evidence identity does not match release")
                evidence_tuple = tuple(observation.get(field) for field in ("level", "client", "dependency_identity", "client_version", "installer_version", "os", "architecture"))
                require(evidence_tuple not in current_tuples, f"{distribution['id']}@{sequence}: multiple current evidence pointers for one applicability tuple")
                current_tuples.add(evidence_tuple)
            package_source = release["package_source"]
            if package_source["revision"] is None:
                require(package_source["repository"] == "777genius/universal-agent-plugins", f"{distribution['id']}@{sequence}: only an in-repository release may await post-merge revision binding")
                require(package_source["path"] == f"plugins/{product_id}", f"{distribution['id']}@{sequence}: unresolved in-repository release must use the canonical product package path")
                require("published_at" not in release, f"{distribution['id']}@{sequence}: unresolved release cannot claim a publication time")
            if distribution["kind"] == "community_bridge":
                require("build_provenance" in release, f"{distribution['id']}@{sequence}: community bridge release requires pinned upstream build provenance")
            else:
                require("build_provenance" not in release, f"{distribution['id']}@{sequence}: build provenance is reserved for community bridge releases")
            if distribution["kind"] == "upstream":
                publisher = str(distribution["id"]).split("/", 1)[0]
                require(str(package_source["repository"]).split("/", 1)[0] == publisher, f"{distribution['id']}@{sequence}: upstream package must be sourced from the upstream publisher namespace")
            # Only an unresolved in-repository release represents the package
            # bytes in this checkout. Bound historical releases are immutable
            # at their recorded commit and may intentionally differ after the
            # canonical product path moves to a newer distribution.
            if verify_packages and package_source["repository"] == "777genius/universal-agent-plugins" and package_source["revision"] is None:
                package_root = ROOT / package_source["path"]
                require(package_root.is_dir(), f"{distribution['id']}@{sequence}: package path is missing")
                fields = package_fields(package_root, [])
                require(directory_tree_digest(package_root) == release["tree_digest"], f"{distribution['id']}@{sequence}: package tree digest drift")
                require(fields["manifest_sha256"] == release["manifest_digest"], f"{distribution['id']}@{sequence}: manifest digest drift")
    for product in products:
        default = distributions_by_id[product["default_distribution"]]
        require(default["status"] == "active", f"{product['id']}: default distribution is not active")
        eligible = []
        for release in default["releases"]:
            policy = _policy_for(default, release["sequence"])
            required = {component for component, state in product["minimum_capabilities"].items() if state == "required"}
            if policy["status"] == "active" and required.issubset(release["components"]):
                eligible.append(release)
        require(eligible, f"{product['id']}: default has no publishable active release satisfying minimum capabilities")
        if default["kind"] == "upstream":
            candidate = eligible[-1]
            policy = _policy_for(default, candidate["sequence"])
            passed_targets = {
                evidence_by_id[evidence_id].get("client")
                for evidence_id in policy["current_evidence"]
                if evidence_by_id[evidence_id]["level"] == "materialization"
                and evidence_by_id[evidence_id]["outcome"] == "passed"
            }
            missing_targets = sorted(
                target["client"] for target in policy["targets"]
                if target["client"] not in passed_targets
            )
            require(
                not missing_targets,
                f"{product['id']}: upstream default {default['id']}@{candidate['sequence']} "
                "lacks current positive package compatibility evidence "
                f"(passed materialization) for targets: {','.join(missing_targets)}",
            )


def _eligible_release(distribution: dict[str, object], product: dict[str, object], targets: set[str], evidence: dict[str, dict[str, object]] | None = None) -> tuple[dict[str, object] | None, str | None]:
    if distribution["status"] != "active":
        return None, f"distribution is {distribution['status']}"
    required = {component for component, state in product["minimum_capabilities"].items() if state == "required"}
    reasons = []
    for release in reversed(distribution["releases"]):
        policy = _policy_for(distribution, release["sequence"])
        supported = {target["client"] for target in policy["targets"]}
        if policy["status"] != "active":
            reasons.append(f"release {release['sequence']} is {policy['status']}")
        elif not required.issubset(release["components"]):
            reasons.append(f"release {release['sequence']} misses required components")
        elif not targets.issubset(supported):
            reasons.append(f"release {release['sequence']} does not support {','.join(sorted(targets - supported))}")
        elif evidence is not None:
            failures = sorted({
                observation["client"]
                for evidence_id in policy["current_evidence"]
                for observation in [evidence[evidence_id]]
                if observation.get("client") in targets
                and observation["level"] in {"materialization", "discovery", "runtime"}
                and observation["outcome"] == "failed"
            })
            if failures:
                reasons.append(f"release {release['sequence']} has blocking trusted failure for {','.join(failures)}")
                continue
            return release, None
        else:
            return release, None
    return None, "; ".join(reasons) or "no releases"


def resolve_directory(source: dict[str, object], selector: str, targets: list[str]) -> dict[str, object]:
    """Resolve one release for the complete target set; never mix distributions."""
    require(targets and len(targets) == len(set(targets)) and set(targets).issubset(CLIENT_IDS), "targets must be unique supported client IDs")
    products = {product["id"]: product for product in source["products"]}
    distributions = {distribution["id"]: distribution for distribution in source["distributions"]}
    aliases = {alias: product for product in source["products"] for alias in product["aliases"]}
    evidence = {observation["id"]: observation for observation in source["evidence"]}
    if selector in distributions:
        distribution = distributions[selector]
        product = products[distribution["product_id"]]
        release, reason = _eligible_release(distribution, product, set(targets), evidence)
        require(release is not None, f"{selector}: {reason}")
        return {"product_id": product["id"], "distribution_id": selector, "release_sequence": release["sequence"], "fallback_reason": None}
    require(selector in aliases, f"unknown Directory selector: {selector}")
    product = aliases[selector]
    default_id = product["default_distribution"]
    default = distributions[default_id]
    release, reason = _eligible_release(default, product, set(targets), evidence)
    if release is not None:
        return {"product_id": product["id"], "distribution_id": default_id, "release_sequence": release["sequence"], "fallback_reason": None}
    candidates = [distributions[item] for item in product["distributions"] if item != default_id]
    candidates.sort(key=lambda item: (KIND_PRIORITY[item["kind"]], item["id"]))
    for distribution in candidates:
        fallback_release, _ = _eligible_release(distribution, product, set(targets), evidence)
        if fallback_release is not None:
            return {"product_id": product["id"], "distribution_id": distribution["id"], "release_sequence": fallback_release["sequence"], "fallback_reason": f"declared default {default_id} was ineligible: {reason}"}
    raise RegistryError(f"{selector}: no distribution supports the complete target set ({reason})")


def is_direct_source(selector: str) -> bool:
    if selector.startswith("./") or selector.startswith("../") or selector.startswith("/"):
        return True
    prefix, separator, path = selector.partition("//")
    repository, marker, revision = prefix.partition("@")
    return bool(separator and path and marker and REPOSITORY_RE.fullmatch(repository) and SHA_RE.fullmatch(revision))


def directory_preview(source: dict[str, object]) -> dict[str, object]:
    distributions = {distribution["id"]: distribution for distribution in source["distributions"]}
    evidence = {observation["id"]: observation for observation in source["evidence"]}
    products = []
    for product in source["products"]:
        choices = []
        for distribution_id in product["distributions"]:
            distribution = distributions[distribution_id]
            release = distribution["releases"][-1]
            policy = _policy_for(distribution, release["sequence"])
            blocking_clients = {
                evidence[evidence_id]["client"]
                for evidence_id in policy["current_evidence"]
                if evidence[evidence_id].get("client")
                and evidence[evidence_id]["level"] in {"materialization", "discovery", "runtime"}
                and evidence[evidence_id]["outcome"] == "failed"
            }
            choices.append({
                "id": distribution_id,
                "kind": distribution["kind"],
                "status": distribution["status"],
                "release_sequence": release["sequence"],
                "package_version": release["package_version"],
                "components": release["components"],
                "eligible_targets": [target["client"] for target in policy["targets"] if target["client"] not in blocking_clients] if policy["status"] == "active" and distribution["status"] == "active" else [],
                "current_evidence": policy["current_evidence"],
                "source": release["package_source"],
                "tree_digest_algorithm": release["tree_digest_algorithm"],
                "tree_digest": release["tree_digest"],
                "manifest_digest": release["manifest_digest"],
            })
        products.append({
            "id": product["id"], "display_name": product["display_name"], "description": product["description"],
            "aliases": product["aliases"], "categories": product["categories"], "default_distribution": product["default_distribution"],
            "fallback_order": [item["id"] for item in sorted((distributions[value] for value in product["distributions"]), key=lambda item: (item["id"] != product["default_distribution"], KIND_PRIORITY[item["kind"]], item["id"]))],
            "distributions": choices,
        })
    return {"schema_version": 1, "product_count": len(products), "products": products}


def directory_search(source: dict[str, object]) -> dict[str, object]:
    return {"schema_version": 1, "entries": [{"product_id": product["id"], "text": " ".join([product["display_name"], product["description"], *product["aliases"], *product["categories"]]).casefold()} for product in source["products"]]}


def validate_readme_blocks(source: dict[str, object]) -> None:
    for product in source["products"]:
        package_root = ROOT / "plugins" / product["id"]
        if not package_root.is_dir():
            continue
        readme = package_root / "README.md"
        require(readme.is_file(), f"{readme}: missing package README")
        body = readme.read_text(encoding="utf-8")
        start, end = "<!-- agentplugins-install:start -->", "<!-- agentplugins-install:end -->"
        require(body.count(start) == body.count(end) == 1, f"{readme}: expected one delimited install block")
        block = body.split(start, 1)[1].split(end, 1)[0]
        expected = f"npx universal-agent-plugins add {product['id']} --target codex"
        require(expected in block, f"{readme}: install block must contain {expected!r}")


def validate_legacy_catalog_freeze() -> None:
    for path, expected in LEGACY_CATALOG_DIGESTS.items():
        require(path.is_file() and digest_bytes(path.read_bytes()) == expected, f"{path}: byte-frozen legacy catalog changed")


def validate_no_flat_directory_entries() -> None:
    if not ENTRIES.exists():
        return
    descriptors = sorted(path.name for path in ENTRIES.iterdir() if path.suffix == ".json")
    require(not descriptors, "flat registry entries are frozen; submit products and distributions in registry/directory.json")


def encoded(index: dict[str, object]) -> bytes:
    return (json.dumps(index, indent=2, ensure_ascii=False, sort_keys=False) + "\n").encode("utf-8")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="validate source and fail if deterministic outputs are stale")
    parser.add_argument("--migrate-legacy", action="store_true", help="write the initial 26-product Directory source from the frozen catalog")
    args = parser.parse_args()
    try:
        if args.migrate_legacy:
            require(not DIRECTORY_SOURCE.exists(), f"{DIRECTORY_SOURCE}: refusing to overwrite review source")
            DIRECTORY_SOURCE.write_bytes(encoded(migrated_directory_source()))
        source = load_directory_source()
        validate_directory(source)
        validate_readme_blocks(source)
        validate_legacy_catalog_freeze()
        validate_no_flat_directory_entries()
        preview = encoded(directory_preview(source))
        search = encoded(directory_search(source))
        _validate_document(json.loads(preview), "directory-preview.schema.json", "review preview")
        _validate_document(json.loads(search), "directory-search.schema.json", "review search")
        if args.check:
            require(REVIEW_PREVIEW.is_file() and REVIEW_PREVIEW.read_bytes() == preview, f"{REVIEW_PREVIEW}: deterministic review preview is stale")
            require(REVIEW_SEARCH.is_file() and REVIEW_SEARCH.read_bytes() == search, f"{REVIEW_SEARCH}: deterministic preview search data is stale")
        else:
            # The legacy flat index is byte-frozen. Directory evolution writes
            # only the review outputs; old clients keep their exact feed.
            REVIEW_PREVIEW.parent.mkdir(parents=True, exist_ok=True)
            REVIEW_PREVIEW.write_bytes(preview)
            REVIEW_SEARCH.write_bytes(search)
    except RegistryError as error:
        print(f"Directory build failed: {error}", file=sys.stderr)
        return 1
    print(f"Universal Agent Plugins Directory valid ({len(source['products'])} products)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
