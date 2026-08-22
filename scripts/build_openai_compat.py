#!/usr/bin/env python3
"""Generate the current OpenAI host-package compatibility layer."""

from __future__ import annotations

import argparse
import filecmp
import io
import json
import os
import shutil
import subprocess
import tarfile
import tempfile
from pathlib import Path, PurePosixPath

from openai_app_bindings import (
    APP_BINDINGS,
    app_document,
    load_app_bindings,
    validate_binding_target,
)


ROOT = Path(__file__).resolve().parents[1]
PORTABLE_ROOT = ROOT / "plugins"
OPENAI_ROOT = ROOT / "compat" / "openai" / "plugins"
MARKETPLACE = ROOT / ".agents" / "plugins" / "marketplace.json"
BRAND_ASSETS = ROOT / "assets"
LOCAL_REPOSITORY = "777genius/universal-agent-plugins"
OPENAI_PACKAGE_TARGET = "codex"

CATEGORIES = {
    "atlassian": "Productivity",
    "figma": "Creativity",
    "linear": "Productivity",
    "notion": "Productivity",
    "stripe": "Finance",
}

# Host-specific auth metadata published by OpenAI's own plugin packages. These
# fields are intentionally absent from the portable Agent Plugins 1.0 schema.
OPENAI_MCP_AUTH = {
    "github": {"bearer_token_env_var": "GITHUB_PAT_TOKEN"},
    "figma": {"oauth_resource": "https://mcp.figma.com/mcp"},
    "linear": {"oauth_resource": "https://mcp.linear.app/mcp"},
    "notion": {"oauth_resource": "https://mcp.notion.com"},
}

READ_ONLY_PLUGINS = {
    "agent-code-navigator",
    "cloudflare-docs",
    "cloudflare-radar",
    "context7",
    "docker-hub",
    "greptile",
    "hubspot-crm",
}

# Provenance-backed exceptions from OpenAI's published plugin metadata. Unknown
# integrations default to Read + Write so the generated UI never understates risk.
CAPABILITY_OVERRIDES = {
    "figma": ["Interactive", "Read", "Write"],
    "sentry": ["Interactive", "Write"],
}

SHORT_DESCRIPTIONS = {
    "agent-code-navigator": "Route code intelligence",
    "atlassian": "Jira and Confluence MCP",
    "chrome-devtools": "Browser debugging MCP",
    "cloudflare": "Cloudflare API MCP",
    "cloudflare-bindings": "Workers bindings MCP",
    "cloudflare-docs": "Cloudflare docs MCP",
    "cloudflare-observability": "Cloudflare logs MCP",
    "cloudflare-radar": "Internet telemetry MCP",
    "context7": "Current library docs",
    "docker-hub": "Docker Hub discovery",
    "figma": "Figma design context",
    "firebase": "Firebase development MCP",
    "github": "GitHub workflows MCP",
    "gitlab": "GitLab workflows MCP",
    "greptile": "Repository intelligence",
    "heroku": "Heroku operations MCP",
    "hubspot-crm": "HubSpot CRM access",
    "hubspot-developer": "HubSpot developer tools",
    "linear": "Linear planning MCP",
    "neon": "Neon database MCP",
    "notion": "Notion workspace MCP",
    "sentry": "Sentry debugging MCP",
    "statsig": "Statsig experiments MCP",
    "stripe": "Stripe billing MCP",
    "supabase": "Supabase backend MCP",
    "vercel": "Vercel deployment MCP",
}


def load(path: Path) -> dict[str, object]:
    """Load one JSON object from disk."""
    value = json.loads(path.read_text())
    if not isinstance(value, dict):
        raise ValueError(f"{path} must contain an object")
    return value


def dump(path: Path, value: object) -> None:
    """Write deterministic, human-readable JSON."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def display_name(name: str) -> str:
    """Return the user-facing name for a portable package."""
    special = {
        "agent-code-navigator": "Agent Code Navigator",
        "chrome-devtools": "Chrome DevTools",
        "cloudflare-api": "Cloudflare API",
        "context7": "Context7",
        "docker-hub": "Docker Hub",
        "firebase": "Firebase",
        "github": "GitHub",
        "gitlab": "GitLab",
        "hubspot-crm": "HubSpot CRM",
        "hubspot-developer": "HubSpot Developer",
        "neon": "Neon",
        "notion": "Notion",
        "sentry": "Sentry",
        "statsig": "Statsig",
        "stripe": "Stripe",
        "supabase": "Supabase",
        "vercel": "Vercel",
    }
    return special.get(name, name.replace("-", " ").title())


def openai_manifest(
    portable: dict[str, object],
    has_skills: bool,
    has_mcp: bool,
    has_app: bool,
) -> dict[str, object]:
    """Translate a portable manifest into the current OpenAI host manifest."""
    name = str(portable["name"])
    description = str(portable["description"])
    homepage = str(portable.get("homepage", portable.get("repository", "")))
    manifest: dict[str, object] = {
        "name": name,
        "version": portable["version"],
        "description": description,
        "author": portable["author"],
        "homepage": homepage,
        "repository": portable["repository"],
        "license": portable["license"],
        "keywords": portable["keywords"],
    }
    if has_skills:
        manifest["skills"] = "./skills/"
    if has_mcp:
        manifest["mcpServers"] = "./.mcp.json"
    if has_app:
        manifest["apps"] = "./.app.json"
    capabilities = CAPABILITY_OVERRIDES.get(
        name,
        ["Read"] if name in READ_ONLY_PLUGINS else ["Read", "Write"],
    )
    manifest["interface"] = {
        "displayName": display_name(name),
        "shortDescription": SHORT_DESCRIPTIONS[name],
        "longDescription": description,
        "developerName": "777genius",
        "category": CATEGORIES.get(name, "Developer Tools"),
        "capabilities": capabilities,
        "websiteURL": homepage,
        "defaultPrompt": [f"Use {display_name(name)} for this task."],
        "brandColor": "#111827",
        "composerIcon": "./assets/icon.png",
        "logo": "./assets/logo.png",
        "logoDark": "./assets/logo.png",
        "screenshots": [],
    }
    return manifest


def openai_mcp(portable: dict[str, object], plugin_name: str) -> dict[str, object]:
    """Translate portable MCP transports and approved host auth metadata."""
    result: dict[str, object] = {}
    servers = portable["mcpServers"]
    assert isinstance(servers, dict)
    for name, raw in servers.items():
        assert isinstance(raw, dict)
        config = dict(raw)
        transport = config.pop("type")
        if transport == "streamable-http":
            config["type"] = "http"
        elif transport == "sse":
            config["type"] = "sse"
        config.update(OPENAI_MCP_AUTH.get(plugin_name, {}))
        result[name] = config
    return {"mcpServers": result}


def selected_release(
    directory: dict[str, object], selection: dict[str, object]
) -> tuple[dict[str, object], dict[str, object]]:
    """Return the exact distribution and release named by a resolver result."""
    distribution = next(
        item
        for item in directory["distributions"]
        if item["id"] == selection["distribution_id"]
    )
    release = next(
        item
        for item in distribution["releases"]
        if item["sequence"] == selection["release_sequence"]
    )
    return distribution, release


def selection_identity(selection: dict[str, object]) -> tuple[object, object, object]:
    """Return the immutable identity fields from an authoritative selection."""
    return tuple(
        selection[key]
        for key in ("product_id", "distribution_id", "release_sequence")
    )


def stale_app_binding(name: str, reason: str) -> ValueError:
    """Return an actionable error for a sidecar that no longer matches policy."""
    return ValueError(
        f"{name}: configured app sidecar is stale or mismatched: {reason}; "
        "update or remove its entry in compat/openai/app-bindings.json"
    )


def selected_target(
    distribution: dict[str, object], release: dict[str, object], client: str
) -> dict[str, object] | None:
    """Return one selected release's explicit client policy target."""
    policy = next(
        item
        for item in distribution["release_policies"]
        if item["release_sequence"] == release["sequence"]
    )
    return next(
        (item for item in policy["targets"] if item["client"] == client),
        None,
    )


def extract_git_package(revision: str, package_path: str, destination: Path) -> bool:
    """Materialize one exact package already present in the reviewed Git clone."""
    git = shutil.which("git")
    if git is None:
        return False
    environment = {
        "PATH": os.environ.get("PATH", ""),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_CONFIG_GLOBAL": os.devnull,
        "GIT_OPTIONAL_LOCKS": "0",
        "LC_ALL": "C",
    }
    try:
        result = subprocess.run(
            [git, "-C", str(ROOT), "archive", "--format=tar", revision, package_path],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=environment,
        )
        if result.returncode != 0:
            return False
        prefix = PurePosixPath(package_path)
        wrote_file = False
        with tarfile.open(fileobj=io.BytesIO(result.stdout), mode="r:") as archive:
            for member in archive:
                path = PurePosixPath(member.name.rstrip("/"))
                if path == prefix or prefix not in path.parents:
                    continue
                relative = path.relative_to(prefix)
                target = destination.joinpath(*relative.parts)
                if member.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                elif member.isfile():
                    source = archive.extractfile(member)
                    if source is None:
                        return False
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(source.read())
                    target.chmod(0o755 if member.mode & 0o111 else 0o644)
                    wrote_file = True
                elif member.issym():
                    link = PurePosixPath(member.linkname)
                    resolved = PurePosixPath(*relative.parent.parts, *link.parts)
                    if link.is_absolute() or ".." in resolved.parts:
                        return False
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.symlink_to(member.linkname)
                else:
                    return False
        return wrote_file
    except (OSError, tarfile.TarError, ValueError):
        return False


def exact_selected_package(
    directory: dict[str, object],
    selection: dict[str, object],
    extracted_root: Path,
) -> Path | None:
    """Return verified exact selected bytes, or None when offline bytes are absent."""
    from build_registry import RegistryError, validate_registry_path, validate_release_package

    distribution, release = selected_release(directory, selection)
    package_source = release["package_source"]
    if package_source["repository"] != LOCAL_REPOSITORY:
        return None
    try:
        package_path = validate_registry_path(package_source["path"])
    except RegistryError:
        return None
    revision = package_source["revision"]
    if revision is None:
        package_root = ROOT / package_path
        allow_unresolved = True
    elif isinstance(revision, str):
        package_root = extracted_root / str(selection["product_id"])
        if not extract_git_package(revision, package_path, package_root):
            return None
        allow_unresolved = False
    else:
        return None
    try:
        validate_release_package(
            package_root,
            release,
            label=(
                f"{selection['distribution_id']}@{selection['release_sequence']}"
            ),
            allow_unresolved_revision=allow_unresolved,
        )
    except (RegistryError, OSError):
        return None
    return package_root


def build(output_root: Path, marketplace_path: Path) -> None:
    """Generate all OpenAI packages and their marketplace catalog."""
    # Lazy to avoid the legacy catalog builder's import of OPENAI_MCP_AUTH
    # forming a module cycle while keeping Directory resolution authoritative.
    from build_registry import RegistryError, load_directory_source, resolve_directory

    bindings = load_app_bindings(APP_BINDINGS)
    directory = load_directory_source()
    products = sorted(directory["products"], key=lambda item: item["id"])
    unknown_bindings = set(bindings) - {str(product["id"]) for product in products}
    if unknown_bindings:
        raise ValueError(f"app bindings reference unknown plugins: {sorted(unknown_bindings)}")
    entries = []
    with tempfile.TemporaryDirectory() as tmp:
        extracted_root = Path(tmp)
        for product in products:
            name = str(product["id"])
            try:
                selection = resolve_directory(
                    directory, name, [OPENAI_PACKAGE_TARGET]
                )
            except RegistryError:
                continue
            binding = bindings.get(name)
            if binding is not None:
                try:
                    app_selection = resolve_directory(directory, name, ["chatgpt"])
                except RegistryError as error:
                    raise stale_app_binding(
                        name, f"ChatGPT target cannot resolve ({error})"
                    ) from error
                if selection_identity(app_selection) != selection_identity(selection):
                    raise stale_app_binding(
                        name,
                        "ChatGPT selects "
                        f"{selection_identity(app_selection)!r}, but Codex selects "
                        f"{selection_identity(selection)!r}",
                    )
                distribution, release = selected_release(directory, app_selection)
                app_target = selected_target(distribution, release, "chatgpt")
                expected_binding = {
                    key: binding[key] for key in ("app_key", "id", "mcp_server")
                }
                if app_target is None or app_target.get("app_binding") != expected_binding:
                    actual_binding = (
                        None if app_target is None else app_target.get("app_binding")
                    )
                    raise stale_app_binding(
                        name,
                        "signed ChatGPT target app_binding "
                        f"{actual_binding!r} does not equal sidecar binding "
                        f"{expected_binding!r}",
                    )
            portable_root = exact_selected_package(
                directory, selection, extracted_root
            )
            if portable_root is None:
                continue
            portable = load(portable_root / "plugin.json")
            if portable.get("name") != product["manifest_name"] or portable["name"] != name:
                raise ValueError(f"{portable_root}: plugin name does not match directory")
            output = output_root / name
            output.mkdir(parents=True, exist_ok=True)
            has_skills = (portable_root / "skills").is_dir()
            has_mcp = (portable_root / "mcp.json").is_file()
            portable_mcp = load(portable_root / "mcp.json") if has_mcp else None
            if binding is not None:
                if portable_mcp is None:
                    raise ValueError(f"{name}: app binding requires portable mcp.json")
                validate_binding_target(name, binding, portable_mcp)
            dump(
                output / ".codex-plugin" / "plugin.json",
                openai_manifest(portable, has_skills, has_mcp, binding is not None),
            )
            if has_mcp:
                assert portable_mcp is not None
                dump(output / ".mcp.json", openai_mcp(portable_mcp, name))
            if binding is not None:
                dump(output / ".app.json", app_document(binding))
            if has_skills:
                shutil.copytree(
                    portable_root / "skills", output / "skills",
                    dirs_exist_ok=True, symlinks=True,
                )
            assets = output / "assets"
            assets.mkdir(parents=True, exist_ok=True)
            shutil.copy2(BRAND_ASSETS / "icon.png", assets / "icon.png")
            shutil.copy2(BRAND_ASSETS / "logo.png", assets / "logo.png")
            shutil.copy2(portable_root / "README.md", output / "README.md")
            entries.append(
                {
                    "name": name,
                    "source": {
                        "source": "local",
                        "path": f"./compat/openai/plugins/{name}",
                    },
                    "policy": {
                        "installation": "AVAILABLE",
                        "authentication": "ON_INSTALL",
                    },
                    "category": CATEGORIES.get(name, "Developer Tools"),
                }
            )
    dump(
        marketplace_path,
        {
            "name": "universal-agent-plugins",
            "interface": {"displayName": "Universal Agent Plugins"},
            "plugins": entries,
        },
    )


def tree_files(root: Path) -> dict[str, bytes]:
    """Return a byte-exact snapshot of a generated tree."""
    if not root.exists():
        return {}
    return {
        str(path.relative_to(root)): path.read_bytes()
        for path in root.rglob("*")
        if path.is_file()
    }


def check() -> int:
    """Compare committed OpenAI adapters with a fresh generation."""
    with tempfile.TemporaryDirectory() as tmp:
        temp = Path(tmp)
        expected_plugins = temp / "plugins"
        expected_marketplace = temp / "marketplace.json"
        build(expected_plugins, expected_marketplace)
        if tree_files(expected_plugins) != tree_files(OPENAI_ROOT):
            print("ERROR: compat/openai/plugins is out of date")
            return 1
        if not MARKETPLACE.is_file() or not filecmp.cmp(expected_marketplace, MARKETPLACE, shallow=False):
            print("ERROR: .agents/plugins/marketplace.json is out of date")
            return 1
    print("OK: OpenAI compatibility layer is up to date")
    return 0


def main() -> int:
    """Generate or verify the OpenAI compatibility layer."""
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    if args.check:
        return check()
    if OPENAI_ROOT.exists():
        shutil.rmtree(OPENAI_ROOT)
    build(OPENAI_ROOT, MARKETPLACE)
    print(f"Generated {len(list(OPENAI_ROOT.iterdir()))} OpenAI compatibility packages")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
