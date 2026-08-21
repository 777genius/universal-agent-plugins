import importlib.util
import json
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "sync_agentplugins_install_commands",
    ROOT / "scripts" / "sync_agentplugins_install_commands.py",
)
assert SPEC and SPEC.loader
sync = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(sync)


class InstallCommandConsistencyTests(unittest.TestCase):
    def test_every_package_has_the_exact_copy_ready_codex_command(self) -> None:
        source = json.loads((ROOT / "registry" / "directory.json").read_text())
        distributions = {item["id"]: item for item in source["distributions"]}
        for product in source["products"]:
            plugin_root = ROOT / "plugins" / product["id"]
            self.assertEqual(sync.updated_readme(plugin_root), (plugin_root / "README.md").read_text())
            self.assertIn(
                f"npx universal-agent-plugins add {product['id']} --target codex",
                (plugin_root / "README.md").read_text(),
            )
            self.assertTrue(any(
                target["client"] == "codex" and target["scopes"] == ["user"]
                for distribution_id in product["distributions"]
                for policy in distributions[distribution_id]["release_policies"]
                for target in policy["targets"]
            ))

    def test_bridge_sources_and_generated_packages_share_the_install_block(self) -> None:
        for bridge_root in sorted(path for path in (ROOT / "bridges").iterdir() if path.is_dir()):
            overlay = bridge_root / "overlay"
            if not (overlay / "README.md").is_file():
                continue
            self.assertEqual(sync.updated_readme(overlay), (overlay / "README.md").read_text())
            name = json.loads((overlay / "plugin.json").read_text())["name"]
            self.assertEqual(
                (overlay / "README.md").read_text().split(sync.START, 1)[1].split(sync.END, 1)[0],
                (ROOT / "plugins" / name / "README.md").read_text().split(sync.START, 1)[1].split(sync.END, 1)[0],
            )


if __name__ == "__main__":
    unittest.main()
