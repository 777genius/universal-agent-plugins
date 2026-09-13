"""Install and execute the plugin-kit-ai CLI binary from GitHub Releases."""

from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
from pathlib import Path
from typing import Dict, Iterable, Optional
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from . import __version__
from .platform import asset_name_for_version, detect_platform

DEFAULT_REPOSITORY = "777genius/plugin-kit-ai"
DEFAULT_API_BASE = "https://api.github.com"
PLACEHOLDER_VERSION = "0.0.0.dev0"


def normalize_tag(raw: str) -> str:
    value = str(raw or "")
    if not value or value == "latest":
        return ""
    match = re.fullmatch(r"(plugin-kit-ai-v|v)?((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*))", value)
    if not match:
        raise ValueError(f"invalid plugin-kit-ai release tag: {value}")
    prefix, version, major = match.group(1), match.group(2), int(match.group(3))
    if major >= 2:
        if prefix == "v":
            raise ValueError("plugin-kit-ai v2+ requires a product-prefixed release tag")
        return f"plugin-kit-ai-v{version}"
    return f"{prefix or 'v'}{version}"


def version_from_tag(tag: str) -> str:
    normalized = normalize_tag(tag)
    if not normalized:
        raise ValueError("an exact plugin-kit-ai release tag is required")
    if normalized.startswith("plugin-kit-ai-v"):
        return normalized[len("plugin-kit-ai-v") :]
    return normalized[1:]


def derive_release_base(api_base: str, override: str) -> str:
    if override and override.strip():
        return override.strip().rstrip("/")
    trimmed = str(api_base or DEFAULT_API_BASE).strip().rstrip("/")
    if trimmed in {"https://api.github.com", "http://api.github.com"}:
        return "https://github.com"
    if trimmed.endswith("/api/v3"):
        return trimmed[: -len("/api/v3")]
    if trimmed.endswith("/api"):
        return trimmed[: -len("/api")]
    return trimmed


def resolve_requested_tag() -> str:
    env_version = normalize_tag(os.environ.get("PLUGIN_KIT_AI_VERSION", ""))
    if env_version:
        return env_version
    if __version__ and __version__ != PLACEHOLDER_VERSION:
        return normalize_tag(__version__)
    return ""


def request_headers(accept_json: bool) -> Dict[str, str]:
    headers: Dict[str, str] = {}
    token = os.environ.get("GITHUB_TOKEN", "").strip()
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if accept_json:
        headers["Accept"] = "application/vnd.github+json"
    return headers


def fetch_bytes(url: str, accept_json: bool = False) -> bytes:
    req = Request(url, headers=request_headers(accept_json))
    try:
        with urlopen(req) as resp:
            return resp.read()
    except (HTTPError, URLError) as exc:
        raise RuntimeError(f"request failed for {url}: {exc}") from exc


def fetch_text(url: str, accept_json: bool = False) -> str:
    return fetch_bytes(url, accept_json=accept_json).decode("utf-8")


def latest_tag(api_base: str, repository: str) -> str:
    clean_base = str(api_base or DEFAULT_API_BASE).strip().rstrip("/")
    payload = json.loads(fetch_text(f"{clean_base}/repos/{repository}/releases/latest", accept_json=True))
    tag_name = str(payload.get("tag_name", ""))
    if not tag_name:
        raise RuntimeError(f"could not resolve latest release tag from {clean_base}")
    return normalize_tag(tag_name)


def parse_checksums(text: str) -> Dict[str, str]:
    out: Dict[str, str] = {}
    for raw_line in str(text or "").splitlines():
        line = raw_line.strip()
        if not line:
            continue
        fields = line.split()
        if len(fields) < 2:
            raise RuntimeError(f'invalid checksums.txt line "{line}"')
        checksum = fields[0].strip()
        name = fields[-1].lstrip("*").strip()
        out[name] = checksum
    return out


def sha256_bytes(body: bytes) -> str:
    return hashlib.sha256(body).hexdigest()


def default_cache_root() -> Path:
    override = os.environ.get("PLUGIN_KIT_AI_CACHE_DIR", "").strip()
    if override:
        return Path(override)
    if sys.platform == "darwin":
        return Path.home() / "Library" / "Caches" / "plugin-kit-ai"
    if sys.platform == "win32":
        base = os.environ.get("LOCALAPPDATA") or str(Path.home() / "AppData" / "Local")
        return Path(base) / "plugin-kit-ai"
    return Path(os.environ.get("XDG_CACHE_HOME", str(Path.home() / ".cache"))) / "plugin-kit-ai"


def extract_binary(archive_path: Path, wanted_name: str, target_path: Path) -> None:
    with tarfile.open(archive_path, mode="r:gz") as archive:
        for member in archive.getmembers():
            if not member.isfile():
                continue
            if Path(member.name).name != wanted_name:
                continue
            extracted = archive.extractfile(member)
            if extracted is None:
                continue
            target_path.parent.mkdir(parents=True, exist_ok=True)
            with extracted, target_path.open("wb") as out:
                shutil.copyfileobj(extracted, out)
            return
    raise RuntimeError(f"archive does not contain {wanted_name} at archive root")


def ensure_installed(*, quiet: bool = False) -> Dict[str, str]:
    repository = os.environ.get("PLUGIN_KIT_AI_REPOSITORY", DEFAULT_REPOSITORY)
    api_base = os.environ.get("GITHUB_API_BASE", DEFAULT_API_BASE)
    release_base = derive_release_base(api_base, os.environ.get("PLUGIN_KIT_AI_RELEASE_BASE_URL", ""))
    platform_info = detect_platform()

    tag = resolve_requested_tag()
    if not tag:
        tag = latest_tag(api_base, repository)
    version = version_from_tag(tag)
    asset_name = asset_name_for_version(version, platform_info)

    cache_root = default_cache_root()
    installed_binary = cache_root / tag / platform_info.binary_name
    if installed_binary.exists():
        return {
            "tag": tag,
            "version": version,
            "asset_name": asset_name,
            "installed_binary": str(installed_binary),
            "repository": repository,
        }

    download_base = f"{release_base}/{repository}/releases/download/{tag}"
    checksums = parse_checksums(fetch_text(f"{download_base}/checksums.txt"))
    if asset_name not in checksums:
        raise RuntimeError(f"checksums.txt missing asset {asset_name}")

    archive = fetch_bytes(f"{download_base}/{asset_name}")
    expected_sum = checksums[asset_name]
    actual_sum = sha256_bytes(archive)
    if actual_sum != expected_sum:
        raise RuntimeError(f"checksum mismatch for {asset_name}")

    with tempfile.TemporaryDirectory(prefix="plugin-kit-ai-pypi-") as tmpdir:
        archive_path = Path(tmpdir) / asset_name
        archive_path.write_bytes(archive)
        extract_binary(archive_path, platform_info.binary_name, installed_binary)

    if platform_info.os_name != "windows":
        current_mode = installed_binary.stat().st_mode
        installed_binary.chmod(current_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

    if not quiet:
        lines = [
            "Installed plugin-kit-ai PyPI wrapper binary",
            f"Version: {tag}",
            f"Repository: {repository}",
            f"Asset: {asset_name}",
            f"Installed path: {installed_binary}",
            "Checksum: verified via checksums.txt",
        ]
        sys.stdout.write(os.linesep.join(lines) + os.linesep)

    return {
        "tag": tag,
        "version": version,
        "asset_name": asset_name,
        "installed_binary": str(installed_binary),
        "repository": repository,
    }


def format_install_error(err: Exception) -> str:
    return os.linesep.join(
        [
            f"plugin-kit-ai PyPI bootstrap: {err}",
            "Fallbacks:",
            "- Homebrew: brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai",
            "- npm: npm i -g plugin-kit-ai",
            "- Verified script: curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh",
        ]
    )


def run_binary(argv: Optional[Iterable[str]] = None) -> int:
    install = ensure_installed(quiet=True)
    args = [install["installed_binary"], *(list(argv) if argv is not None else sys.argv[1:])]
    completed = subprocess.run(args, check=False)
    return completed.returncode
