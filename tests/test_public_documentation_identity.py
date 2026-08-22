from __future__ import annotations

import importlib.util
import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DOCUMENTS = (
    ROOT / "README.md",
    ROOT / "docs" / "QUICKSTART.md",
    ROOT / "docs" / "VERIFICATION.md",
    ROOT / "docs" / "TEST_MATRIX.md",
)
INSTALL_DOCUMENTS = (
    ROOT / "README.md",
    ROOT / "docs" / "QUICKSTART.md",
    ROOT / "docs" / "HERO_PLUGINS.md",
)
MODULE_PATH = ROOT / "scripts" / "build_registry.py"
SPEC = importlib.util.spec_from_file_location("build_registry_for_docs", MODULE_PATH)
assert SPEC and SPEC.loader
registry = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(registry)

SWITCH_RE = re.compile(
    r"\bswitch\s+(?P<product>[a-z0-9-]+)\s+--to\s+"
    r"(?P<distribution>[a-z0-9-]+/[a-z0-9._-]+)\b"
)
ADD_RE = re.compile(
    r"\bnpx\s+universal-agent-plugins\s+add\s+(?P<product>[a-z0-9-]+)"
    r"(?:\s+--target\s+(?P<targets>[a-z]+(?:,[a-z]+)*))?\b"
)
SHA256_RE = re.compile(r"sha256:[0-9a-f]{64}")
CURRENT_TUPLE_RE = re.compile(
    r"Current Directory release:\s*`(?P<distribution>[a-z0-9-]+/[a-z0-9._-]+)`"
    r"\s+release\s+`(?P<release>[1-9][0-9]*)`,\s+tree\s+`(?P<tree>sha256:[0-9a-f]{64})`,"
    r"\s+manifest\s+`(?P<manifest>sha256:[0-9a-f]{64})`\."
)
HISTORICAL_TUPLE_RE = re.compile(
    r"Historical Directory release:\s*`(?P<distribution>[a-z0-9-]+/[a-z0-9._-]+)`"
    r"\s+release\s+`[1-9][0-9]*`.*?"
    r"https://github\.com/[^\s)]+/blob/[0-9a-f]{40}/[^\s)]+",
    re.DOTALL,
)


class PublicDocumentationIdentityTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.directory = registry.load_directory_source()
        cls.distributions = {
            item["id"]: item for item in cls.directory["distributions"]
        }

    def test_every_qualified_switch_example_resolves_for_supported_targets(self) -> None:
        examples = []
        for path in DOCUMENTS:
            examples.extend((path, match) for match in SWITCH_RE.finditer(path.read_text()))
        self.assertTrue(examples, "public docs must retain a qualified switch example")

        for path, match in examples:
            distribution_id = match.group("distribution")
            self.assertIn(distribution_id, self.distributions, f"{path}: unknown distribution")
            distribution = self.distributions[distribution_id]
            self.assertEqual(match.group("product"), distribution["product_id"])
            release_sequence = distribution["releases"][-1]["sequence"]
            policy = next(
                item
                for item in distribution["release_policies"]
                if item["release_sequence"] == release_sequence
            )
            for target in policy["targets"]:
                with self.subTest(path=path, distribution=distribution_id, target=target["client"]):
                    resolved = registry.resolve_directory(
                        self.directory, distribution_id, [target["client"]]
                    )
                    self.assertEqual(resolved["distribution_id"], distribution_id)
                    self.assertEqual(resolved["release_sequence"], release_sequence)

    def test_every_starter_short_name_has_an_eligible_install_candidate(self) -> None:
        examples = []
        for path in INSTALL_DOCUMENTS:
            examples.extend((path, match) for match in ADD_RE.finditer(path.read_text()))
        self.assertTrue(examples, "public starter docs must retain a short-name add example")

        for path, match in examples:
            product = match.group("product")
            target_value = match.group("targets")
            if target_value:
                with self.subTest(path=path, product=product, targets=target_value):
                    resolved = registry.resolve_directory(
                        self.directory, product, target_value.split(",")
                    )
                    self.assertEqual(resolved["product_id"], product)
                continue

            resolved_target = None
            for target in registry.CLIENT_IDS:
                try:
                    registry.resolve_directory(self.directory, product, [target])
                except registry.RegistryError:
                    continue
                resolved_target = target
                break
            self.assertIsNotNone(
                resolved_target,
                f"{path}: {product} has no eligible install candidate for any client",
            )

    def test_readme_preserves_public_cli_and_source_contracts(self) -> None:
        readme = (ROOT / "README.md").read_text()
        quickstart = (ROOT / "docs" / "QUICKSTART.md").read_text()

        self.assertIn("`universal-agent-plugins` is the npm package", readme)
        self.assertIn("`agentplugins` is the installed\ncommand", readme)
        for action in ("add", "update", "remove", "repair", "switch"):
            with self.subTest(action=action):
                self.assertRegex(readme, rf"npx universal-agent-plugins {action}\b")

        for document in (readme, quickstart):
            with self.subTest(document=document[:40]):
                self.assertIn("--target codex,cursor", document)
                self.assertNotIn("--target all", document)
                self.assertRegex(
                    document,
                    r"[a-z0-9-]+/[a-z0-9._-]+@[0-9a-f]{40}//[^\s\\]+",
                )
                self.assertIn("add ./my-plugin --target cursor", document)
                for source_label in (
                    "upstream",
                    "community bridge",
                    "community",
                    "direct source",
                ):
                    self.assertIn(source_label, document.lower())
                self.assertIn("not official vendor", document)

        for stale_readme_copy in (
            "catalog digest",
            "historical repository revision",
            "State v3",
            "Registry v3",
        ):
            with self.subTest(stale_readme_copy=stale_readme_copy):
                self.assertNotIn(stale_readme_copy, readme)

    def test_readme_keeps_evidence_claims_narrow(self) -> None:
        readme = (ROOT / "README.md").read_text()
        normalized = " ".join(readme.split())
        self.assertIn("All 26 packages pass standard schema validation", normalized)
        self.assertIn(
            "materialized or installed package does not by itself prove", normalized
        )
        self.assertIn("Figma OAuth was tested separately in Codex only", normalized)
        self.assertIn("ChatGPT and Copilot claims are narrower", normalized)
        self.assertIn(
            "runtime-tested, OAuth-tested, read-only, and not-proven", normalized
        )

    def test_current_and_historical_identity_prose_is_verifiable(self) -> None:
        for path in DOCUMENTS:
            text = path.read_text()
            for paragraph in re.split(r"\n\s*\n", text):
                current_positions = [
                    match.start()
                    for match in re.finditer(r"\bcurrent\b", paragraph, re.IGNORECASE)
                ]
                hash_matches = list(SHA256_RE.finditer(paragraph))
                current_digest_claim = any(
                    0 < digest.start() - current < 500
                    for current in current_positions
                    for digest in hash_matches
                )
                if current_digest_claim:
                    matches = list(CURRENT_TUPLE_RE.finditer(paragraph))
                    self.assertTrue(matches, f"{path}: current digest prose must use a checked Directory tuple")
                    self.assertEqual(
                        sorted(match.group() for match in hash_matches),
                        sorted(value for match in matches for value in (match.group("tree"), match.group("manifest"))),
                        f"{path}: unverified digest in current prose",
                    )
                    for match in matches:
                        distribution = self.distributions[match.group("distribution")]
                        release = next(
                            item
                            for item in distribution["releases"]
                            if item["sequence"] == int(match.group("release"))
                        )
                        self.assertEqual(match.group("tree"), release["tree_digest"])
                        self.assertEqual(match.group("manifest"), release["manifest_digest"])

                qualified_ids = [item for item in self.distributions if item in paragraph]
                has_release_identity = qualified_ids and "release" in paragraph.lower() and SHA256_RE.search(paragraph)
                if has_release_identity and not CURRENT_TUPLE_RE.search(paragraph):
                    self.assertRegex(paragraph, HISTORICAL_TUPLE_RE, f"{path}: historical tuple needs an immutable evidence link")


if __name__ == "__main__":
    unittest.main()
