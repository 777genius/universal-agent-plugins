#!/usr/bin/env python3
"""Generate the current OpenAI host-package compatibility layer."""

from __future__ import annotations

import argparse
import filecmp
import json
import shutil
import tempfile
from pathlib import Path

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


def build(output_root: Path, marketplace_path: Path) -> None:
    """Generate all OpenAI packages and their marketplace catalog."""
    # Lazy to avoid the legacy catalog builder's import of OPENAI_MCP_AUTH
    # forming a module cycle while keeping Directory resolution authoritative.
    from build_registry import eligible_product_targets, load_directory_source

    bindings = load_app_bindings(APP_BINDINGS)
    portable_roots = sorted(path for path in PORTABLE_ROOT.iterdir() if path.is_dir())
    unknown_bindings = set(bindings) - {path.name for path in portable_roots}
    if unknown_bindings:
        raise ValueError(f"app bindings reference unknown plugins: {sorted(unknown_bindings)}")
    directory = load_directory_source()
    eligible_roots = [
        path for path in portable_roots
        if eligible_product_targets(directory, path.name)
    ]
    entries = []
    for portable_root in eligible_roots:
        portable = load(portable_root / "plugin.json")
        name = str(portable["name"])
        if name != portable_root.name:
            raise ValueError(f"{portable_root}: plugin name does not match directory")
        output = output_root / name
        output.mkdir(parents=True, exist_ok=True)
        has_skills = (portable_root / "skills").is_dir()
        has_mcp = (portable_root / "mcp.json").is_file()
        binding = bindings.get(name)
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
            shutil.copytree(portable_root / "skills", output / "skills", dirs_exist_ok=True)
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
