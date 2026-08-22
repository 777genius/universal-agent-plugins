#!/usr/bin/env python3
"""Add or verify one copy-ready agentplugins command in every package README."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
from build_registry import eligible_product_targets, load_directory_source  # noqa: E402

PLUGINS = ROOT / "plugins"
BRIDGES = ROOT / "bridges"
START = "<!-- agentplugins-install:start -->"
END = "<!-- agentplugins-install:end -->"
NPX_COMMAND = "npx universal-agent-plugins"


def block(name: str, eligible_targets: list[str]) -> str:
    if eligible_targets:
        target = "codex" if "codex" in eligible_targets else eligible_targets[0]
        content = f"""## Install

```bash
{NPX_COMMAND} add {name} --target {target}
```"""
    else:
        content = """## Installation unavailable

> Installation is currently unavailable because the Directory has no eligible release target."""
    return f"""{START}
{content}
{END}"""


def updated_readme(plugin_root: Path, source: dict[str, object] | None = None) -> str:
    if source is None:
        source = load_directory_source()
    readme = plugin_root / "README.md"
    body = readme.read_text()
    name = json.loads((plugin_root / "plugin.json").read_text())["name"]
    install = block(name, eligible_product_targets(source, name))
    if START in body:
        before, remainder = body.split(START, 1)
        _, after = remainder.split(END, 1)
        return before + install + after
    sections = body.split("\n\n")
    if len(sections) < 3 or not sections[0].startswith("# "):
        raise ValueError(f"unexpected README structure: {readme}")
    sections.insert(2, install)
    return "\n\n".join(sections)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    changed = []
    source = load_directory_source()
    package_roots = sorted(path for path in PLUGINS.iterdir() if path.is_dir())
    overlay_roots = sorted(path / "overlay" for path in BRIDGES.iterdir() if (path / "overlay" / "README.md").is_file())
    for plugin_root in package_roots + overlay_roots:
        readme = plugin_root / "README.md"
        expected = updated_readme(plugin_root, source)
        if readme.read_text() != expected:
            changed.append(readme)
            if not args.check:
                readme.write_text(expected)
    if args.check and changed:
        raise SystemExit("ERROR: package install commands are out of date: " + ", ".join(str(path.relative_to(ROOT)) for path in changed))
    pinned_examples = [
        path.relative_to(ROOT)
        for path in ROOT.rglob("*.md")
        if f"{NPX_COMMAND}@" in path.read_text()
    ]
    if pinned_examples:
        raise SystemExit(
            "ERROR: public npx examples must not pin the installer version: "
            + ", ".join(str(path) for path in pinned_examples)
        )
    print(f"OK: {len(package_roots)} package and {len(overlay_roots)} bridge-source install commands")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
