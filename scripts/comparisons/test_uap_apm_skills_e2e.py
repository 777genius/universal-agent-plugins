"""Fail closed when the comparison harness could misclassify results."""
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("uap_apm_skills_e2e.py")
SPEC = importlib.util.spec_from_file_location("uap_apm_comparison", SCRIPT)
assert SPEC and SPEC.loader
comparison = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(comparison)


def command(code: int = 0, *, timeout: bool = False, cleanup: str | None = None) -> dict:
    return {"exit_code": code, "timed_out": timeout, "cleanup_error": cleanup}


class ComparisonSafetyTests(unittest.TestCase):
    def test_cache_only_cannot_qualify_codex(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = Path(tmp)
            expected = {"SKILL.md": b"exact skill", "references/fixture.txt": b"exact ref"}
            cache = project / "apm_modules" / "benchmark-note"
            cache.mkdir(parents=True)
            (cache / "SKILL.md").write_bytes(expected["SKILL.md"])
            (cache / "references").mkdir()
            (cache / "references/fixture.txt").write_bytes(expected["references/fixture.txt"])
            self.assertEqual(comparison.payloads(project, expected),
                             {"skills": [], "references": []})
            results = []
            comparison.add_result(results, "apm", "S1_install", command(),
                                  comparison.payloads(project, expected), True)
            self.assertEqual(results[-1]["status"], "FAIL")

    def test_exact_project_file_and_symlinked_parent(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = Path(tmp)
            expected = {"SKILL.md": b"skill", "references/fixture.txt": b"reference"}
            real = project / ".agents" / "skills" / "benchmark-note"
            real.mkdir(parents=True)
            (real / "SKILL.md").write_bytes(expected["SKILL.md"])
            (real / "references").mkdir()
            (real / "references/fixture.txt").write_bytes(expected["references/fixture.txt"])
            self.assertEqual(len(comparison.payloads(project, expected)["references"]), 1)
            (real / "SKILL.md").unlink()
            (real / "SKILL.md").symlink_to("references/fixture.txt")
            self.assertEqual(comparison.payloads(project, expected)["skills"], [])
            # A symlinked ancestor must not be treated as owned projected data.
            (real / "SKILL.md").unlink()
            (real / "SKILL.md").write_bytes(expected["SKILL.md"])
            renamed = project / "real_agents"
            (project / ".agents").rename(renamed)
            (project / ".agents").symlink_to(renamed, target_is_directory=True)
            self.assertEqual(comparison.payloads(project, expected)["skills"], [])

    def test_foreign_loss_and_refusal_cannot_produce_false_pass(self):
        files = {"skills": ["project/.agents/skills/benchmark-note/SKILL.md"],
                 "references": ["project/.agents/skills/benchmark-note/references/fixture.txt"]}
        for scenario in ("S1_install", "S1_foreign_collision"):
            for bad in (command(), command(1), command(cleanup="child still alive")):
                observed = []
                comparison.add_result(observed, "uap", scenario, bad, files, False)
                self.assertEqual(observed[-1]["status"], "FAIL")
        observed = []
        comparison.add_result(observed, "uap", "S1_foreign_collision", command(1),
                              {"skills": [], "references": []}, True)
        self.assertEqual(observed[-1]["status"], "BLOCKED")
        self.assertNotEqual(observed[-1]["status"], "PASS")
        observed = []
        comparison.add_result(observed, "uap", "S1_foreign_collision", command(timeout=True),
                              {"skills": [], "references": []}, True)
        self.assertEqual(observed[-1]["status"], "FAIL")
        observed = []
        comparison.add_result(observed, "uap", "S1_install", command(cleanup="unverified"),
                              files, True)
        self.assertEqual(observed[-1]["status"], "FAIL")

    def test_deleted_or_modified_sentinel_records_nonpreservation(self):
        with tempfile.TemporaryDirectory() as tmp:
            project, data = comparison.make_project(Path(tmp))
            sentinel = project / ".agents/skills/foreign-neighbor/SKILL.md"
            self.assertTrue(comparison.safe_same_bytes(sentinel, data, project))
            sentinel.unlink()
            self.assertFalse(comparison.safe_same_bytes(sentinel, data, project))
            sentinel.write_bytes(b"modified")
            self.assertFalse(comparison.safe_same_bytes(sentinel, data, project))

    def test_portable_bytes_and_lock_identity(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = Path(tmp)
            commit = "a" * 40
            manifest = (
                'schema_version = 1\n'
                '[dependencies.benchmark]\n'
                f'git = "{comparison.REPOSITORY}"\n'
                f'path = "{comparison.SKILL_PATH}"\n'
                f'commit = "{commit}"\n'
            )
            lock = {
                "lock_version": 1,
                "packages": [{
                    "alias": "benchmark", "source": {
                        "git": {
                            "repository": comparison.REPOSITORY,
                            "resolved_skill_path": comparison.SKILL_PATH,
                            "commit": commit,
                        },
                    },
                }],
            }
            (project / "uap.toml").write_text(manifest)
            (project / "uap.lock").write_text(json.dumps(lock))
            original, capture_error = comparison.captured_portable(project, "uap")
            self.assertIsNone(capture_error)
            identity = comparison.portable_identity(original, commit, "uap")
            self.assertTrue(identity["requested_source_matches"])
            self.assertTrue(identity["resolved_source_matches"])
            self.assertTrue(identity["exact_source_commit_recorded_in_lock"])
            # Requested path does not override an explicit conflicting resolved path.
            git = lock["packages"][0]["source"]["git"]
            git["path"] = comparison.SKILL_PATH
            git["resolved_skill_path"] = "moved/not-the-selected-skill"
            (project / "uap.lock").write_text(json.dumps(lock))
            conflicting, capture_error = comparison.captured_portable(project, "uap")
            self.assertIsNone(capture_error)
            self.assertFalse(comparison.portable_identity(
                conflicting, commit, "uap"
            )["resolved_source_matches"])
            git["resolved_skill_path"] = comparison.SKILL_PATH
            lock["packages"][0]["source"]["git"]["commit"] = "b" * 40
            # A SHA that merely occurs in an unrelated field must not count.
            lock["comment"] = commit
            (project / "uap.lock").write_text(json.dumps(lock))
            changed, capture_error = comparison.captured_portable(project, "uap")
            self.assertIsNone(capture_error)
            self.assertNotEqual(original, changed)
            self.assertFalse(comparison.portable_identity(
                changed, commit, "uap"
            )["exact_source_commit_recorded_in_lock"])

    def test_portable_capture_rejects_symlinks_nonfiles_and_large_inputs(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = root / "project"
            project.mkdir()
            (project / "uap.toml").write_bytes(b"schema_version = 1\\n")
            lock = project / "uap.lock"
            data, issue = comparison.captured_portable(project, "uap")
            self.assertEqual(data, {})
            self.assertEqual(issue, {"file": "uap.lock", "code": "missing"})

            lock.write_bytes(b"x" * (2 * 1024 * 1024 + 1))
            data, issue = comparison.captured_portable(project, "uap")
            self.assertEqual(data, {})
            self.assertEqual(issue, {"file": "uap.lock", "code": "too_large"})

            lock.unlink()
            lock.symlink_to("uap.toml")
            _, issue = comparison.captured_portable(project, "uap")
            self.assertEqual(issue, {"file": "uap.lock", "code": "symlink_path"})

            lock.unlink()
            lock.mkdir()
            _, issue = comparison.captured_portable(project, "uap")
            self.assertEqual(issue, {"file": "uap.lock", "code": "not_regular"})

            lock.rmdir()
            lock.write_bytes(b"{}")
            alias = root / "project-link"
            alias.symlink_to(project, target_is_directory=True)
            _, issue = comparison.captured_portable(alias, "uap")
            self.assertEqual(issue, {"file": "uap.toml", "code": "symlink_path"})

    def test_apm_lock_resolved_identity_not_requested_comment(self):
        import yaml

        commit = "a" * 40
        manifest = {
            "dependencies": {"apm": [
                f"{comparison.REPOSITORY}/{comparison.SKILL_PATH}#{commit}"
            ]},
        }
        lock = {
            "dependencies": [{
                "repo_url": comparison.REPOSITORY,
                "virtual_path": comparison.SKILL_PATH,
                "resolved_commit": "b" * 40,
                "comment": commit,
            }],
        }
        portable = {
            "apm.yml": yaml.safe_dump(manifest).encode(),
            "apm.lock.yaml": yaml.safe_dump(lock).encode(),
        }
        self.assertFalse(comparison.portable_identity(
            portable, commit, "apm"
        )["exact_source_commit_recorded_in_lock"])
        lock["dependencies"][0]["resolved_commit"] = commit
        portable["apm.lock.yaml"] = yaml.safe_dump(lock).encode()
        self.assertTrue(comparison.portable_identity(
            portable, commit, "apm"
        )["exact_source_commit_recorded_in_lock"])

    def test_launch_failure_is_structured(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            root.mkdir(exist_ok=True)
            outcome = comparison.run_process(
                ["/nonexistent-uap-apm-benchmark-executable"], root,
                {"PATH": "/usr/bin"}, root,
            )
            self.assertIsNone(outcome["exit_code"])
            self.assertIsNotNone(outcome["cleanup_error"])

    def test_uap_full_git_sha_source_is_not_ref_flag(self):
        commit = "a" * 40
        argv = comparison.tool_args("uap", "/bin/agentplugins", Path("/scratch/project"), commit)
        self.assertNotIn("--ref", argv)
        self.assertNotIn("--path", argv)
        self.assertEqual(
            argv[3],
            f"777genius/universal-agent-plugins@{commit}//{comparison.SKILL_PATH}",
        )
        self.assertEqual(argv[argv.index("--scope") + 1], "project")
        self.assertEqual(argv[argv.index("--target") + 1], "codex")


if __name__ == "__main__":
    unittest.main()
