from __future__ import annotations

import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).resolve().parents[1] / "scripts" / "validate_catalog.py"
SPEC = importlib.util.spec_from_file_location("validate_catalog", MODULE_PATH)
assert SPEC and SPEC.loader
validator = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(validator)
sys.path.insert(0, str(MODULE_PATH.parent))
import build_registry as registry  # noqa: E402


class CatalogValidatorTests(unittest.TestCase):
    def make_plugin(self, root: Path) -> Path:
        plugin = root / "plugins" / "demo"
        plugin.mkdir(parents=True)
        (plugin / "plugin.json").write_text(
            json.dumps(
                {
                    "$schema": validator.PLUGIN_SCHEMA,
                    "name": "demo",
                    "version": "0.1.0",
                    "description": "Demo plugin",
                    "author": {"name": "Test"},
                    "keywords": ["demo"],
                }
            )
        )
        (plugin / "README.md").write_text("# Demo\n")
        (plugin / "mcp.json").write_text(
            json.dumps(
                {
                    "$schema": validator.MCP_SCHEMA,
                    "mcpServers": {
                        "demo": {
                            "type": "streamable-http",
                            "url": "https://example.com/mcp",
                        }
                    },
                }
            )
        )
        return plugin

    def test_valid_catalog(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            self.make_plugin(Path(tmp))
            self.assertEqual(validator.validate_catalog(Path(tmp)), (1, 1, 0))

    def test_unknown_manifest_field_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            plugin = self.make_plugin(Path(tmp))
            manifest = json.loads((plugin / "plugin.json").read_text())
            manifest["mcpServers"] = "./mcp.json"
            (plugin / "plugin.json").write_text(json.dumps(manifest))
            with self.assertRaises(validator.ValidationError):
                validator.validate_catalog(Path(tmp))

    def test_secret_header_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            plugin = self.make_plugin(Path(tmp))
            mcp = json.loads((plugin / "mcp.json").read_text())
            mcp["mcpServers"]["demo"]["headers"] = {"Authorization": "Bearer token"}
            (plugin / "mcp.json").write_text(json.dumps(mcp))
            with self.assertRaises(validator.ValidationError):
                validator.validate_catalog(Path(tmp))

    def test_unpinned_npx_launcher_aliases_fail(self) -> None:
        for command in (
            "npx",
            "npx.cmd",
            "NPX",
            r"C:\\tools\\npx.exe",
            "/usr/local/bin/npx",
            "npx.ps1",
            "NPX.PS1",
            r"C:\\tools\\npx.ps1",
            "/path/npx.ps1",
        ):
            with self.subTest(command=command), tempfile.TemporaryDirectory() as tmp:
                plugin = self.make_plugin(Path(tmp))
                mcp = {
                    "$schema": validator.MCP_SCHEMA,
                    "mcpServers": {
                        "demo": {
                            "type": "stdio",
                            "command": command,
                            "args": ["-y", "demo@latest"],
                        }
                    },
                }
                (plugin / "mcp.json").write_text(json.dumps(mcp))
                with self.assertRaisesRegex(validator.ValidationError, "npx package must be pinned"):
                    validator.validate_catalog(Path(tmp))

    def test_non_npx_launcher_is_not_subject_to_npx_pinning(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            plugin = self.make_plugin(Path(tmp))
            mcp = {
                "$schema": validator.MCP_SCHEMA,
                "mcpServers": {
                    "demo": {
                        "type": "stdio",
                        "command": "npx-wrapper.cmd",
                        "args": ["demo@latest"],
                    }
                },
            }
            (plugin / "mcp.json").write_text(json.dumps(mcp))
            self.assertEqual(validator.validate_catalog(Path(tmp)), (1, 1, 0))

    def test_powershell_npx_shims_inherit_runtime_closure_rejection(self) -> None:
        commands = ("npx.ps1", "NPX.PS1", r"C:\\tools\\npx.ps1", "/path/npx.ps1")
        for command in commands:
            with self.subTest(command=command), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                plugin = self.make_plugin(root)
                mcp = {
                    "$schema": validator.MCP_SCHEMA,
                    "mcpServers": {
                        "demo": {
                            "type": "stdio",
                            "command": command,
                            "args": ["-y", "demo@1.0.0"],
                        }
                    },
                }
                (plugin / "mcp.json").write_text(json.dumps(mcp))
                self.assertEqual(validator.normalized_executable_basename(command), "npx")
                self.assertEqual(validator.validate_catalog(root), (1, 1, 0))
                source = {
                    "distributions": [{
                        "id": "test/demo",
                        "status": "active",
                        "releases": [{
                            "sequence": 1,
                            "package_source": {
                                "repository": "test/repository",
                                "revision": None,
                                "path": "plugins/demo",
                            },
                        }],
                        "release_policies": [{
                            "release_sequence": 1,
                            "status": "active",
                        }],
                    }],
                }
                with self.assertRaisesRegex(
                    registry.RegistryError, "content-addressed runtime closure"
                ):
                    registry.validate_active_local_runtime_closures(
                        source,
                        repository_root=root,
                        repository="test/repository",
                    )

    def test_invalid_skill_field_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            plugin = self.make_plugin(Path(tmp))
            (plugin / "mcp.json").unlink()
            skill = plugin / "skills" / "demo-skill"
            skill.mkdir(parents=True)
            (skill / "SKILL.md").write_text(
                "---\nname: demo-skill\ndescription: Demo\ncommand: ./run.sh\n---\n"
            )
            with self.assertRaises(validator.ValidationError):
                validator.validate_catalog(Path(tmp))

    def test_symlink_escape_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            plugin = self.make_plugin(root)
            outside = root / "outside.json"
            outside.write_text((plugin / "mcp.json").read_text())
            (plugin / "mcp.json").unlink()
            (plugin / "mcp.json").symlink_to(outside)
            with self.assertRaises(validator.ValidationError):
                validator.validate_catalog(root)

    def test_plugin_root_parent_traversal_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            plugin = self.make_plugin(root)
            mcp = {
                "$schema": validator.MCP_SCHEMA,
                "mcpServers": {
                    "demo": {
                        "type": "stdio",
                        "command": "demo",
                        "cwd": "${PLUGIN_ROOT}/../outside",
                    }
                },
            }
            (plugin / "mcp.json").write_text(json.dumps(mcp))
            with self.assertRaises(validator.ValidationError):
                validator.validate_catalog(root)

    def test_windows_incompatible_package_path_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            plugin = self.make_plugin(root)
            (plugin / "CON").write_text("not portable")
            with self.assertRaises(validator.ValidationError):
                validator.validate_catalog(root)


if __name__ == "__main__":
    unittest.main()
