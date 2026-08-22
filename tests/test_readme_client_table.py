import json
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[1]
README_PATH = REPO_ROOT / "README.md"
ICON_ROOT = REPO_ROOT / "assets" / "client-icons"
CLIENTS_PATH = REPO_ROOT / "docs" / "CLIENTS.md"
COMPATIBILITY_PATH = REPO_ROOT / "docs" / "COMPATIBILITY.md"
HERO_PLUGINS_PATH = REPO_ROOT / "docs" / "HERO_PLUGINS.md"
TEST_MATRIX_PATH = REPO_ROOT / "docs" / "TEST_MATRIX.md"
VERIFICATION_PATH = REPO_ROOT / "docs" / "VERIFICATION.md"
CHATGPT_EVIDENCE_PATH = (
    REPO_ROOT
    / "tests"
    / "e2e"
    / "results"
    / "chatgpt-cloudflare-docs-desktop-package-2026-08-10.json"
)


class ReadmeClientTableTests(unittest.TestCase):
    def test_external_agent_plugin_install_is_documented(self) -> None:
        readme = README_PATH.read_text()
        quickstart = (REPO_ROOT / "docs" / "QUICKSTART.md").read_text()
        contributing = (REPO_ROOT / "CONTRIBUTING.md").read_text()

        for document in (readme, quickstart, contributing):
            with self.subTest(document=document[:40]):
                self.assertRegex(
                    document,
                    r"(?:owner/repo@FULL_COMMIT_SHA|[a-z0-9-]+/[a-z0-9._-]+@[a-f0-9]{40})//[^\s\\]+",
                )
                self.assertIn("--target cursor", document)

        self.assertIn("add ./my-plugin --target cursor", readme)
        self.assertIn("not limited to this Directory", readme)
        self.assertIn("do not need to be copied into it", readme)
        self.assertIn("Directory membership is needed only for a reviewed short name", quickstart)
        self.assertIn("add cloudflare-docs --target codex,cursor,kiro", readme)
        self.assertIn(
            "switch cloudflare-docs --to 777genius/cloudflare-docs", readme
        )
        self.assertNotIn("--yes", readme)

    def test_every_supported_client_has_a_sourced_logo(self) -> None:
        readme = README_PATH.read_text()
        provenance = (ICON_ROOT / "README.md").read_text()
        expected_client_icons = {
            "Codex": "openai.svg",
            "ChatGPT": "openai.svg",
            "Cursor": "cursor.svg",
            "GitHub Copilot CLI": "github-copilot.svg",
            "VS Code": "vscode.svg",
            "Kiro": "kiro.svg",
        }
        expected_icons = {
            "openai.svg",
            "cursor.svg",
            "github-copilot.svg",
            "vscode.svg",
            "kiro.svg",
        }

        self.assertEqual(
            {path.name for path in ICON_ROOT.glob("*.svg")}, expected_icons
        )
        for client, icon in expected_client_icons.items():
            with self.subTest(client=client):
                self.assertIn(
                    f'<img src="assets/client-icons/{icon}" width="20" '
                    f'height="20" alt=""> {client} |',
                    readme,
                )

        expected_source_tokens = {
            "openai.svg": ("github.com/openai/openai-cookbook/blob/4a85c301",),
            "cursor.svg": (
                "cursor.com/marketing-static/favicon-light.svg",
                "78f169abca311d70",
            ),
            "github-copilot.svg": ("github.com/primer/octicons/blob/d1e0051",),
            "vscode.svg": (
                "code.visualstudio.com/brand",
                "74ad401c6487a0dc",
            ),
            "kiro.svg": ("kiro.dev/icon.svg", "774cbc1c7ecec8c9"),
        }
        for icon, tokens in expected_source_tokens.items():
            with self.subTest(provenance=icon):
                source_line = next(
                    line for line in provenance.splitlines() if f"`{icon}`" in line
                )
                for token in tokens:
                    self.assertIn(token, source_line)

        self.assertEqual(readme.count("assets/client-icons/openai.svg"), 2)

    def test_chatgpt_claim_matches_recorded_evidence_boundary(self) -> None:
        readme = README_PATH.read_text()
        normalized_readme = " ".join(readme.split())
        evidence = json.loads(CHATGPT_EVIDENCE_PATH.read_text())
        proved = set(evidence["scope"]["proved"])
        not_proved = set(evidence["scope"]["not_proved"])
        self.assertTrue(proved.isdisjoint(not_proved))

        self.assertTrue(
            {
                "repository_marketplace_registration",
                "local_codex_plugin_package_ingestion",
                "official_manager_install",
                "exact_app_id_linkage",
            }.issubset(proved)
        )
        self.assertTrue(
            {
                "chatgpt_work_ui_discovery",
                "chatgpt_work_package_activation",
                "package_routed_runtime",
            }.issubset(not_proved)
        )
        self.assertIn("All 26 packages pass standard schema validation", readme)
        self.assertIn("15/15 runtime checks", normalized_readme)
        self.assertIn(
            "Installation coverage is broader than runtime coverage",
            normalized_readme,
        )
        self.assertIn("docs/TEST_MATRIX.md", readme)
        self.assertIn("docs/VERIFICATION.md", readme)
        self.assertNotIn(
            "local `.codex-plugin` ingestion and manager\nlifecycle are still separate, unproved steps",
            readme,
        )
        for internal_term in (
            "State v3",
            "projection",
            "materialization",
            "31363316668",
            "d3941c0",
        ):
            with self.subTest(internal_term=internal_term):
                self.assertNotIn(internal_term, readme)

        for path in (TEST_MATRIX_PATH, VERIFICATION_PATH):
            with self.subTest(lifecycle_document=path.name):
                document = path.read_text()
                self.assertIn("31363316668", document)
                self.assertIn("d3941c0", document)
                self.assertNotIn("31350094295", document)

        clients = CLIENTS_PATH.read_text()
        self.assertNotIn(
            "repository marketplace ingestion and manager lifecycle are not", clients
        )
        self.assertNotIn("local `.codex-plugin` ingestion remains unproved", clients)
        self.assertIn("package-routed runtime remain unproved", clients)

        compatibility = COMPATIBILITY_PATH.read_text()
        self.assertNotIn("not local package\ningestion or manager lifecycle", compatibility)
        self.assertIn("package-routed runtime", compatibility)

        hero_plugins = HERO_PLUGINS_PATH.read_text()
        self.assertNotIn("local package ingestion\nis not claimed", hero_plugins)
        self.assertIn("repository package separately\npassed marketplace ingestion", hero_plugins)
        self.assertIn("package-routed\nChatGPT Work runtime remains unproved", hero_plugins)


if __name__ == "__main__":
    unittest.main()
