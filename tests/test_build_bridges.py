from __future__ import annotations

import hashlib
import importlib.util
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import yaml


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("build_bridges", ROOT / "scripts" / "build_bridges.py")
assert SPEC and SPEC.loader
bridges = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = bridges
SPEC.loader.exec_module(bridges)


FIXTURE_SHA = "9ec238505ab95b2e07222e69a893f0bbac201ae6"


def run_git(root: Path, *args: str, env: dict[str, str] | None = None) -> str:
    result = subprocess.run(
        ["git", *args], cwd=root, env=env, check=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
    )
    return result.stdout.strip()


def commit_environment() -> dict[str, str]:
    result = dict(os.environ)
    result.update(
        GIT_AUTHOR_NAME="Fixture",
        GIT_AUTHOR_EMAIL="fixture@example.invalid",
        GIT_AUTHOR_DATE="2026-01-01T00:00:00Z",
        GIT_COMMITTER_NAME="Fixture",
        GIT_COMMITTER_EMAIL="fixture@example.invalid",
        GIT_COMMITTER_DATE="2026-01-01T00:00:00Z",
    )
    return result


class BridgeBuilderTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.temp = Path(self.temporary.name)
        self.root = self.temp / "root"
        self.root.mkdir()
        shutil.copytree(ROOT / "tests" / "fixtures" / "bridges", self.root / "bridges")
        shutil.copytree(ROOT / "tests" / "fixtures" / "plugins", self.root / "plugins")
        self.work = self.temp / "work"
        shutil.copytree(ROOT / "tests" / "fixtures" / "bridge_upstream", self.work)
        run_git(self.work, "init", "-q")
        run_git(self.work, "add", ".")
        run_git(self.work, "commit", "-q", "-m", "fixture", env=commit_environment())
        self.assertEqual(run_git(self.work, "rev-parse", "HEAD"), FIXTURE_SHA)
        self.mirror = self.temp / "mirror"
        (self.mirror / "fixture").mkdir(parents=True)
        run_git(self.temp, "clone", "-q", "--bare", str(self.work), str(self.mirror / "fixture" / "upstream.git"))

    def tearDown(self) -> None:
        self.temporary.cleanup()

    @property
    def recipe_path(self) -> Path:
        return self.root / "bridges" / "fixture-bridge" / "bridge.yaml"

    def recipe(self) -> dict[str, object]:
        return yaml.safe_load(self.recipe_path.read_text())

    def write_recipe(self, recipe: dict[str, object]) -> None:
        self.recipe_path.write_text(yaml.safe_dump(recipe, sort_keys=False))

    def commit_upstream_change(self, message: str) -> str:
        run_git(self.work, "add", ".")
        run_git(self.work, "commit", "-q", "-m", message, env=commit_environment())
        revision = run_git(self.work, "rev-parse", "HEAD")
        run_git(
            self.mirror / "fixture" / "upstream.git",
            "fetch", "-q", str(self.work), revision,
        )
        return revision

    def assemble(self) -> tuple[Path, dict[str, object]]:
        output = self.temp / "assembled" / "fixture-bridge"
        output.mkdir(parents=True)
        return output, bridges.assemble(self.root, "fixture-bridge", output, self.mirror)

    def test_fixed_recipe_is_reproducible_and_preserves_executable_mode(self) -> None:
        first, report = self.assemble()
        second = self.temp / "second" / "fixture-bridge"
        second.mkdir(parents=True)
        second_report = bridges.assemble(self.root, "fixture-bridge", second, self.mirror)
        bridges.compare_trees(first, second)
        bridges.compare_trees(self.root / "plugins" / "fixture-bridge", second)
        self.assertEqual(report, second_report)
        self.assertEqual(report["upstream_revision"], FIXTURE_SHA)
        self.assertRegex(report["tree_digest"], r"^sha256:[0-9a-f]{64}$")
        mode = (second / "skills" / "fixture-skill" / "tool.sh").stat().st_mode
        self.assertTrue(mode & stat.S_IXUSR)

    def test_reproduction_reports_are_bound_to_directory_bridge_releases(self) -> None:
        directory = self.root / "registry"
        directory.mkdir()
        (directory / "directory.json").write_text("{}")
        with mock.patch.object(bridges, "validate_bridge_bindings") as validate:
            reports = bridges.check_all(self.root, self.mirror)
        report = next(item for item in reports if item["product_id"] == "fixture-bridge")
        self.assertEqual(report["upstream_revision"], FIXTURE_SHA)
        self.assertEqual(report["tree_digest"], "sha256:d56b21a056a2e268a0641f09e7b00f6ad1468ac4a6eb10d845bd31068cb569c5")
        validate.assert_called_once_with(
            {}, repository_root=self.root, build_reports=reports,
        )

    def test_builder_invokes_only_git_and_never_upstream_executable(self) -> None:
        marker = self.temp / "executed"
        tool = self.work / "skills" / "fixture-skill" / "tool.sh"
        self.assertNotIn(str(marker), tool.read_text())
        calls: list[list[str]] = []
        original = bridges.subprocess.run

        def recording_run(argv, *args, **kwargs):  # type: ignore[no-untyped-def]
            calls.append(list(argv))
            return original(argv, *args, **kwargs)

        with mock.patch.object(bridges.subprocess, "run", side_effect=recording_run):
            self.assemble()
        self.assertTrue(calls)
        self.assertTrue(all(call[0] == "git" for call in calls))
        self.assertFalse(marker.exists())

    def test_license_digest_change_fails_closed(self) -> None:
        recipe = self.recipe()
        recipe["upstream"]["license"]["attribution_paths"][0]["sha256"] = "sha256:" + "0" * 64
        self.write_recipe(recipe)
        with self.assertRaisesRegex(bridges.BridgeError, "license/attribution changed"):
            self.assemble()

    def test_undeclared_overlay_copy_conflict_fails_closed(self) -> None:
        recipe = self.recipe()
        recipe["copy"].append({"source": "package-metadata.json", "destination": "plugin.json"})
        self.write_recipe(recipe)
        with self.assertRaisesRegex(bridges.BridgeError, "overlay/copy conflict"):
            self.assemble()

    def test_changed_overlaid_upstream_content_fails_closed(self) -> None:
        skill = self.work / "skills" / "fixture-skill" / "SKILL.md"
        original_digest = "sha256:" + hashlib.sha256(skill.read_bytes()).hexdigest()
        skill.write_text(skill.read_text() + "\nChanged upstream.\n")
        revision = self.commit_upstream_change("change overlaid content")
        recipe = self.recipe()
        recipe["upstream"]["revision"] = revision
        recipe["copy"].append(
            {
                "source": "skills/fixture-skill/SKILL.md",
                "destination": "README.md",
            }
        )
        recipe["overlay_replacements"] = [
            {"path": "README.md", "upstream_sha256": original_digest}
        ]
        self.write_recipe(recipe)
        with self.assertRaisesRegex(bridges.BridgeError, "overlaid upstream content changed"):
            self.assemble()

    def test_new_upstream_sha_produces_deterministic_package_diff(self) -> None:
        skill = self.work / "skills" / "fixture-skill" / "SKILL.md"
        skill.write_text(skill.read_text() + "\nA reviewed upstream update.\n")
        revision = self.commit_upstream_change("update skill")
        recipe = self.recipe()
        recipe["upstream"]["revision"] = revision
        self.write_recipe(recipe)
        output, report = self.assemble()
        self.assertEqual(report["upstream_revision"], revision)
        with self.assertRaisesRegex(bridges.BridgeError, "file bytes differ"):
            bridges.compare_trees(
                self.root / "plugins" / "fixture-bridge", output
            )

    def test_new_upstream_executable_requires_recipe_review(self) -> None:
        (self.work / "LICENSE").chmod(0o755)
        revision = self.commit_upstream_change("make license executable")
        recipe = self.recipe()
        recipe["upstream"]["revision"] = revision
        self.write_recipe(recipe)
        with self.assertRaisesRegex(bridges.BridgeError, "executable path expectation mismatch"):
            self.assemble()

    def test_zero_copy_recipe_requires_pinned_provenance(self) -> None:
        recipe = self.recipe()
        recipe["copy"] = []
        recipe["upstream"]["provenance"]["paths"] = []
        self.write_recipe(recipe)
        with self.assertRaisesRegex(bridges.BridgeError, "zero-copy bridge requires"):
            self.assemble()

    def test_zero_copy_mcp_bridge_records_exact_provenance_evidence(self) -> None:
        output = self.temp / "zero-copy" / "fixture-mcp-bridge"
        output.mkdir(parents=True)
        report = bridges.assemble(
            self.root, "fixture-mcp-bridge", output, self.mirror
        )
        self.assertEqual(
            report["provenance_evidence"],
            {
                "mcp-package.json":
                    "sha256:039d3c96ba64fd40e2d0b11c52a0365f39e73430326c8a50b80aaac8536ec85e"
            },
        )
        self.assertEqual(report["components"]["mcp_servers"], ["fixture-mcp"])
        self.assertEqual(
            sorted(path.name for path in output.iterdir()),
            ["NOTICE", "README.md", "mcp.json", "plugin.json"],
        )

    def test_path_traversal_is_rejected_by_recipe_schema(self) -> None:
        recipe = self.recipe()
        recipe["copy"][0]["destination"] = "../LICENSE"
        self.write_recipe(recipe)
        with self.assertRaisesRegex(bridges.BridgeError, "invalid bridge recipe"):
            self.assemble()

    def test_lfs_pointer_is_rejected(self) -> None:
        pointer = self.work / "large.bin"
        pointer.write_text(
            "version https://git-lfs.github.com/spec/v1\n"
            "oid sha256:" + "1" * 64 + "\nsize 123\n"
        )
        run_git(self.work, "add", "large.bin")
        run_git(self.work, "commit", "-q", "-m", "lfs", env=commit_environment())
        revision = run_git(self.work, "rev-parse", "HEAD")
        run_git(self.mirror / "fixture" / "upstream.git", "fetch", "-q", str(self.work), revision)
        repository = bridges.PinnedRepository("fixture/upstream", revision, self.mirror)
        try:
            with self.assertRaisesRegex(bridges.BridgeError, "Git LFS pointer"):
                repository.blobs("large.bin")
        finally:
            repository.close()

    def test_submodule_gitlink_is_rejected(self) -> None:
        run_git(
            self.work, "update-index", "--add", "--cacheinfo",
            f"160000,{FIXTURE_SHA},vendor",
        )
        run_git(self.work, "commit", "-q", "-m", "gitlink", env=commit_environment())
        revision = run_git(self.work, "rev-parse", "HEAD")
        run_git(self.mirror / "fixture" / "upstream.git", "fetch", "-q", str(self.work), revision)
        repository = bridges.PinnedRepository("fixture/upstream", revision, self.mirror)
        try:
            with self.assertRaisesRegex(bridges.BridgeError, "submodule"):
                repository.blobs("vendor")
        finally:
            repository.close()

    def test_only_build_and_check_commands_are_accepted(self) -> None:
        with self.assertRaises(SystemExit):
            bridges.main(["list"])


class RealBridgeCohortTests(unittest.TestCase):
    def test_recipes_are_zero_copy_exactly_pinned_and_outputs_are_complete(self) -> None:
        expected = {
            "chrome-devtools": ("777genius/chrome-devtools-bridge", "ChromeDevTools/chrome-devtools-mcp", "774d78f5eef5e610407a0c92fa6ec5ed74b027e8", "Apache-2.0"),
            "cloudflare-docs": ("777genius/cloudflare-docs-bridge", "cloudflare/mcp-server-cloudflare", "0c51a6fbcf9a2fae80120287e8238fb947cdc2df", "Apache-2.0"),
            "github": ("777genius/github-bridge", "github/github-mcp-server", "fcdd664099f957c4a7dc183d9381cef191e8c8a9", "MIT"),
        }
        for bridge_id, values in expected.items():
            with self.subTest(bridge=bridge_id):
                _path, recipe = bridges.load_recipe(ROOT, bridge_id)
                distribution, repository, revision, license_id = values
                self.assertEqual(recipe["distribution_id"], distribution)
                self.assertEqual(recipe["copy"], [])
                self.assertEqual(recipe["upstream"]["repository"], repository)
                self.assertEqual(recipe["upstream"]["revision"], revision)
                self.assertEqual(recipe["upstream"]["license"]["spdx"], license_id)
                self.assertTrue(recipe["upstream"]["license"]["attribution_paths"])
                self.assertTrue(recipe["upstream"]["provenance"]["paths"])
                output = ROOT / "plugins" / bridge_id
                self.assertEqual(sorted(path.name for path in output.iterdir()), ["NOTICE", "README.md", "mcp.json", "plugin.json"])
                manifest = json.loads((output / "plugin.json").read_text())
                self.assertTrue(manifest["description"].startswith("Community package for "))

    def test_runtime_identity_is_exact_and_non_floating(self) -> None:
        chrome = json.loads((ROOT / "plugins/chrome-devtools/mcp.json").read_text())["mcpServers"]["chrome-devtools"]
        cloudflare = json.loads((ROOT / "plugins/cloudflare-docs/mcp.json").read_text())["mcpServers"]["cloudflare-docs"]
        github = json.loads((ROOT / "plugins/github/mcp.json").read_text())["mcpServers"]["github"]
        self.assertEqual(chrome["args"], ["-y", "chrome-devtools-mcp@1.7.0"])
        directory = json.loads((ROOT / "registry/directory.json").read_text())
        chrome = {
            item["id"]: item for item in directory["distributions"]
            if item["id"] in {"777genius/chrome-devtools", "777genius/chrome-devtools-bridge"}
        }
        self.assertEqual(set(chrome), {"777genius/chrome-devtools", "777genius/chrome-devtools-bridge"})
        for distribution in chrome.values():
            self.assertEqual(distribution["status"], "suspended")
            self.assertEqual({policy["status"] for policy in distribution["release_policies"]}, {"revoked"})
        self.assertEqual(cloudflare["url"], "https://docs.mcp.cloudflare.com/mcp")
        self.assertEqual(github["url"], "https://api.githubcopilot.com/mcp/")

    def test_claimed_targets_materialize_complete_packages_in_disposable_roots(self) -> None:
        directory = json.loads((ROOT / "registry/directory.json").read_text())
        distributions = {item["id"]: item for item in directory["distributions"]}
        bridge_ids = [
            "777genius/chrome-devtools-bridge",
            "777genius/cloudflare-docs-bridge",
            "777genius/github-bridge",
        ]
        with tempfile.TemporaryDirectory(prefix="bridge-materialization-") as temporary:
            sandbox = Path(temporary)
            for distribution_id in bridge_ids:
                distribution = distributions[distribution_id]
                product_id = distribution["product_id"]
                targets = distribution["release_policies"][0]["targets"]
                for target in targets:
                    with self.subTest(distribution=distribution_id, target=target["client"]):
                        materialized = sandbox / target["client"] / product_id
                        shutil.copytree(ROOT / "plugins" / product_id, materialized)
                        self.assertEqual(bridges.validate_plugin(materialized), (1, 0))
                        bridges.compare_trees(ROOT / "plugins" / product_id, materialized)

    def test_upstream_context7_identity_and_pinned_bridge_checks_are_mandatory(self) -> None:
        directory = json.loads((ROOT / "registry/directory.json").read_text())
        distribution = next(item for item in directory["distributions"] if item["id"] == "upstash/context7")
        release = distribution["releases"][0]
        self.assertEqual(distribution["kind"], "upstream")
        self.assertEqual(release["package_source"], {
            "repository": "upstash/context7",
            "revision": "769c6cd22c3d95462d1f55d789e9532cabefa5a9",
            "path": "plugins/agent-plugins/context7",
        })
        self.assertEqual(release["tree_digest"], "sha256:08eed3b67f2e71a11b68baa594380c2f69ec1bc97584d701deaf7942ac34c0d8")
        workflow = (ROOT / ".github/workflows/validate.yml").read_text()
        self.assertIn('scripts/build-bridges --root tests/fixtures --upstream-mirror "${RUNNER_TEMP}/upstream-mirror" check', workflow)
        self.assertIn("scripts/build-bridges check", workflow)


if __name__ == "__main__":
    unittest.main()
