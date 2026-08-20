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


if __name__ == "__main__":
    unittest.main()
