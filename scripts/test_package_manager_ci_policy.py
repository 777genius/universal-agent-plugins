"""Guard the fast-draft / full-ready package-manager CI contract.

Structural checks only; they do not substitute for GitHub event evaluation.
Dependency-free so drafts can run without setting up Go or accounts.
"""

from pathlib import Path
import unittest

WORKFLOW = Path(__file__).resolve().parents[1] / ".github/workflows/package-manager-e2e.yml"


class PackageManagerCIPolicyTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.body = WORKFLOW.read_text(encoding="utf-8")

    def test_ready_pr_and_main_are_always_full(self):
        self.assertRegex(self.body, r"(?m)^  pull_request:\s*$")
        self.assertIn("types: [opened, synchronize, reopened, ready_for_review]", self.body)
        self.assertRegex(self.body, r"(?m)^  push:\n    branches: \[main\]")
        self.assertRegex(self.body, r"(?m)^  workflow_dispatch:\s*$")
        self.assertIn("github.event_name != 'pull_request' || github.event.pull_request.draft == false", self.body)
        self.assertIn("matrix:\n        os: [ubuntu-24.04, windows-2025, macos-15, macos-15-intel]", self.body)
        self.assertIn("fail-fast: false", self.body)

    def test_draft_only_runs_disposable_harness_preflight(self):
        self.assertIn("github.event_name == 'pull_request' && github.event.pull_request.draft", self.body)
        self.assertIn("name: draft evidence preflight", self.body)
        self.assertIn("test_package_manager_e2e.py", self.body)
        self.assertIn("test_package_manager_ci_policy.py", self.body)
        self.assertRegex(self.body, r"(?m)^    timeout-minutes: 5\s*$")
        self.assertNotIn("continue-on-error: true", self.body)

    def test_required_source_and_test_paths_are_watched(self):
        for path in (
            "'cli/**'", "'install/integrationctl/**'", "'sdk/**'", "'go.work*'",
            "'**/go.mod'", "'**/go.sum'", "'scripts/package-manager-e2e.py'",
            "'scripts/package_manager_binary.py'", "'scripts/test_package_manager_e2e.py'",
            "'scripts/test_package_manager_ci_policy.py'",
            "'.github/workflows/package-manager-e2e.yml'",
        ):
            self.assertEqual(self.body.count(path), 2, path)
        self.assertRegex(self.body, r"(?m)^  cancel-in-progress:.*pull_request.*push.*$")


if __name__ == "__main__":
    unittest.main()
