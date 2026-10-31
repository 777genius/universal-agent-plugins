"""Fail closed when the comparison harness could misclassify results."""
import importlib.util
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
            sha = "a" * 40
            (project / "uap.toml").write_text("schema_version = 1\n")
            (project / "uap.lock").write_text("revision = '" + sha + "'\n")
            first = comparison.captured_portable(project, "uap")
            self.assertTrue(comparison.portable_identity(first, sha)["exact_source_commit_recorded_in_lock"])
            (project / "uap.lock").write_text("revision = '" + "b" * 40 + "'\n")
            self.assertNotEqual(first, comparison.captured_portable(project, "uap"))
            self.assertFalse(comparison.portable_identity(
                comparison.captured_portable(project, "uap"), sha
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


if __name__ == "__main__":
    unittest.main()
