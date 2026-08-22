#!/usr/bin/env python3
"""Validate the repository's Agent Plugins 1.0 portable packages."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path, PurePosixPath
from urllib.parse import urlsplit

sys.path.insert(0, str(Path(__file__).resolve().parent))
from portable_paths import validate_tree


PLUGIN_SCHEMA = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
MCP_SCHEMA = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"
PLUGIN_FIELDS = {
    "$schema",
    "name",
    "version",
    "description",
    "author",
    "homepage",
    "repository",
    "license",
    "keywords",
    "extensions",
}
AUTHOR_FIELDS = {"name", "email", "url"}
SKILL_FIELDS = {
    "name",
    "description",
    "license",
    "compatibility",
    "metadata",
    "allowed-tools",
}
PLUGIN_NAME = re.compile(r"^(?!.*(?:--|\.\.))[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$")
SKILL_NAME = re.compile(r"^(?!.*--)[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$")
SEMVER = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$")


class ValidationError(Exception):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValidationError(message)


def load_json(path: Path) -> dict[str, object]:
    try:
        value = json.loads(path.read_text())
    except (OSError, json.JSONDecodeError) as exc:
        raise ValidationError(f"{path}: invalid JSON: {exc}") from exc
    require(isinstance(value, dict), f"{path}: top level must be an object")
    return value


def validate_url(value: object, field: str) -> None:
    require(isinstance(value, str) and value, f"{field}: expected a non-empty string")
    parsed = urlsplit(value)
    require(parsed.scheme in {"http", "https"}, f"{field}: expected an HTTP(S) URL")
    require(bool(parsed.hostname), f"{field}: URL must have a host")
    require(parsed.username is None and parsed.password is None, f"{field}: user info is forbidden")
    require(not parsed.fragment, f"{field}: fragments are forbidden")
    loopback = parsed.hostname == "localhost" or parsed.hostname in {"127.0.0.1", "::1"}
    require(parsed.scheme == "https" or loopback, f"{field}: non-loopback URL must use HTTPS")


def validate_plugin_manifest(plugin_root: Path) -> dict[str, object]:
    path = plugin_root / "plugin.json"
    require(path.is_file(), f"{plugin_root}: missing plugin.json")
    manifest = load_json(path)
    unknown = set(manifest) - PLUGIN_FIELDS
    require(not unknown, f"{path}: unknown fields: {sorted(unknown)}")
    require(manifest.get("$schema") == PLUGIN_SCHEMA, f"{path}: wrong $schema")
    name = manifest.get("name")
    require(isinstance(name, str) and 1 <= len(name) <= 64, f"{path}: invalid name length")
    require(bool(PLUGIN_NAME.fullmatch(name)), f"{path}: invalid plugin name {name!r}")
    require(name == plugin_root.name, f"{path}: name must match package directory")
    version = manifest.get("version")
    require(isinstance(version, str) and bool(SEMVER.fullmatch(version)), f"{path}: catalog requires SemVer")
    require(isinstance(manifest.get("description"), str) and manifest["description"], f"{path}: description required")
    author = manifest.get("author")
    require(isinstance(author, dict), f"{path}: author object required")
    require(not (set(author) - AUTHOR_FIELDS), f"{path}: unknown author fields")
    require(isinstance(author.get("name"), str) and author["name"], f"{path}: author.name required")
    keywords = manifest.get("keywords")
    require(isinstance(keywords, list) and all(isinstance(item, str) for item in keywords), f"{path}: keywords must be strings")
    extensions = manifest.get("extensions")
    if extensions is not None:
        require(isinstance(extensions, dict), f"{path}: extensions must be an object")
        require(all(isinstance(value, dict) for value in extensions.values()), f"{path}: extension values must be objects")
    return manifest


def validate_package_path(plugin_root: Path, value: str, field: str) -> None:
    require(value.startswith("./"), f"{field}: package path must start with ./")
    relative = PurePosixPath(value[2:].replace("\\", "/"))
    require(bool(relative.parts) and ".." not in relative.parts, f"{field}: parent traversal is forbidden")
    resolved = (plugin_root / value[2:]).resolve(strict=False)
    require(resolved == plugin_root.resolve() or plugin_root.resolve() in resolved.parents, f"{field}: path escapes package")


def validate_placeholder_path(value: str, placeholder: str, field: str) -> None:
    if value == placeholder:
        return
    require(value.startswith(f"{placeholder}/"), f"{field}: invalid placeholder path")
    relative = PurePosixPath(value[len(placeholder) + 1 :].replace("\\", "/"))
    require(bool(relative.parts) and ".." not in relative.parts, f"{field}: parent traversal is forbidden")


def normalized_executable_basename(command: str) -> str:
    """Return a platform-independent executable name for launcher policy."""
    basename = command.replace("\\", "/").rsplit("/", 1)[-1].casefold()
    for suffix in (".cmd", ".exe", ".bat", ".ps1"):
        if basename.endswith(suffix):
            return basename[:-len(suffix)]
    return basename


def validate_stdio(plugin_root: Path, name: str, config: dict[str, object]) -> None:
    allowed = {"type", "command", "args", "env", "cwd"}
    require(not (set(config) - allowed), f"{name}: unknown stdio fields")
    command = config.get("command")
    require(isinstance(command, str) and command and not any(ch.isspace() for ch in command), f"{name}: command must be one token")
    require("${" not in command, f"{name}: command does not support placeholders")
    if command.startswith("."):
        validate_package_path(plugin_root, command, f"{name}.command")
    args = config.get("args", [])
    require(isinstance(args, list) and all(isinstance(item, str) for item in args), f"{name}: args must be strings")
    env = config.get("env", {})
    require(isinstance(env, dict) and all(isinstance(k, str) and isinstance(v, str) for k, v in env.items()), f"{name}: env must map strings")
    require(not ({"PLUGIN_ROOT", "PLUGIN_DATA"} & set(env)), f"{name}: reserved env key")
    for value in env.values():
        placeholders = re.findall(r"\$\{[^}]+\}", value)
        require(all(item in {"${PLUGIN_ROOT}", "${PLUGIN_DATA}"} for item in placeholders), f"{name}: unsupported env placeholder")
    cwd = config.get("cwd")
    if cwd is not None:
        require(isinstance(cwd, str), f"{name}: cwd must be a string")
        valid = cwd.startswith("./") or cwd == "${PLUGIN_ROOT}" or cwd.startswith("${PLUGIN_ROOT}/") or cwd == "${PLUGIN_DATA}" or cwd.startswith("${PLUGIN_DATA}/")
        require(valid, f"{name}: invalid cwd")
        if cwd.startswith("./"):
            validate_package_path(plugin_root, cwd, f"{name}.cwd")
        elif cwd.startswith("${PLUGIN_ROOT}"):
            validate_placeholder_path(cwd, "${PLUGIN_ROOT}", f"{name}.cwd")
        else:
            validate_placeholder_path(cwd, "${PLUGIN_DATA}", f"{name}.cwd")
    if normalized_executable_basename(command) == "npx":
        packages = [arg for arg in args if not arg.startswith("-") and "@" in arg]
        require(packages and all(not arg.endswith("@latest") for arg in packages), f"{name}: npx package must be pinned")
    if command == "docker":
        images = [arg for arg in args if "/" in arg and not arg.startswith("-")]
        require(any("@sha256:" in image for image in images), f"{name}: Docker image must be pinned by digest")


def validate_remote(name: str, config: dict[str, object]) -> None:
    allowed = {"type", "url", "headers"}
    require(not (set(config) - allowed), f"{name}: unknown remote fields")
    validate_url(config.get("url"), f"{name}.url")
    require("${" not in str(config["url"]), f"{name}: URL placeholders are forbidden")
    headers = config.get("headers", {})
    require(isinstance(headers, dict) and all(isinstance(k, str) and isinstance(v, str) for k, v in headers.items()), f"{name}: headers must map strings")
    lowered = [key.lower() for key in headers]
    require(len(lowered) == len(set(lowered)), f"{name}: duplicate case-insensitive headers")
    require(all("${" not in value for value in headers.values()), f"{name}: credential placeholders are forbidden")
    require("authorization" not in lowered, f"{name}: credentials must be client-managed")


def validate_mcp(plugin_root: Path) -> int:
    path = plugin_root / "mcp.json"
    if not path.exists():
        return 0
    require(path.is_file(), f"{path}: must be a regular file")
    doc = load_json(path)
    require(set(doc) == {"$schema", "mcpServers"}, f"{path}: only $schema and mcpServers are allowed")
    require(doc["$schema"] == MCP_SCHEMA, f"{path}: wrong $schema")
    servers = doc["mcpServers"]
    require(isinstance(servers, dict), f"{path}: mcpServers must be an object")
    for name, config in servers.items():
        require(isinstance(name, str) and name, f"{path}: server name must be non-empty")
        require(isinstance(config, dict), f"{path}: server {name} must be an object")
        transport = config.get("type")
        require(transport in {"stdio", "streamable-http", "sse"}, f"{path}: invalid transport for {name}")
        if transport == "stdio":
            validate_stdio(plugin_root, name, config)
        else:
            validate_remote(name, config)
    return len(servers)


def parse_skill_frontmatter(path: Path) -> dict[str, str]:
    text = path.read_text()
    require(text.startswith("---\n"), f"{path}: missing YAML frontmatter")
    end = text.find("\n---\n", 4)
    require(end != -1, f"{path}: unterminated YAML frontmatter")
    values: dict[str, str] = {}
    for line in text[4:end].splitlines():
        if not line or line.startswith(" "):
            continue
        require(":" in line, f"{path}: invalid frontmatter line {line!r}")
        key, value = line.split(":", 1)
        values[key.strip()] = value.strip().strip('"\'')
    return values


def validate_skills(plugin_root: Path) -> int:
    skills_root = plugin_root / "skills"
    if not skills_root.exists():
        return 0
    require(skills_root.is_dir(), f"{skills_root}: must be a directory")
    count = 0
    for skill_root in sorted(skills_root.iterdir()):
        if not skill_root.is_dir():
            continue
        path = skill_root / "SKILL.md"
        require(path.is_file(), f"{skill_root}: missing SKILL.md")
        values = parse_skill_frontmatter(path)
        require(not (set(values) - SKILL_FIELDS), f"{path}: unsupported frontmatter fields: {sorted(set(values) - SKILL_FIELDS)}")
        name = values.get("name", "")
        description = values.get("description", "")
        require(name == skill_root.name and bool(SKILL_NAME.fullmatch(name)), f"{path}: invalid or mismatched skill name")
        require(1 <= len(description) <= 1024, f"{path}: invalid description")
        compatibility = values.get("compatibility")
        require(compatibility is None or 1 <= len(compatibility) <= 500, f"{path}: invalid compatibility")
        count += 1
    return count


def validate_plugin(plugin_root: Path) -> tuple[int, int]:
    require(not plugin_root.is_symlink(), f"{plugin_root}: plugin root cannot be a symlink")
    try:
        validate_tree(plugin_root)
    except ValueError as exc:
        raise ValidationError(f"{plugin_root}: {exc}") from exc
    for path in plugin_root.rglob("*"):
        require(not path.is_symlink(), f"{path}: symlinks are forbidden in portable packages")
    validate_plugin_manifest(plugin_root)
    require((plugin_root / "README.md").is_file(), f"{plugin_root}: package README required")
    forbidden = [plugin_root / ".mcp.json", plugin_root / ".codex-plugin"]
    require(not any(path.exists() for path in forbidden), f"{plugin_root}: client-specific files are forbidden in portable core")
    mcp_count = validate_mcp(plugin_root)
    skill_count = validate_skills(plugin_root)
    require(mcp_count + skill_count > 0, f"{plugin_root}: catalog packages must contain a component")
    return mcp_count, skill_count


def validate_catalog(root: Path) -> tuple[int, int, int]:
    plugins_root = root / "plugins"
    require(plugins_root.is_dir(), f"{plugins_root}: missing plugins directory")
    plugin_count = mcp_count = skill_count = 0
    for plugin_root in sorted(plugins_root.iterdir()):
        if not plugin_root.is_dir():
            continue
        mcp, skills = validate_plugin(plugin_root)
        plugin_count += 1
        mcp_count += mcp
        skill_count += skills
    require(plugin_count > 0, "catalog is empty")
    return plugin_count, mcp_count, skill_count


def main() -> int:
    root = Path(__file__).resolve().parents[1]
    try:
        plugins, servers, skills = validate_catalog(root)
    except ValidationError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    print(f"OK: {plugins} plugins, {servers} MCP servers, {skills} skills")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
