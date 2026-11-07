"""Guard the fast-draft / full-ready package-manager CI contract.

Structural checks only; they do not substitute for GitHub event evaluation.
Dependency-free so drafts can run without setting up Go or accounts.
"""

from pathlib import Path
import re
import unittest

WORKFLOW = Path(__file__).resolve().parents[1] / ".github/workflows/package-manager-e2e.yml"
NATIVE_WORKFLOW = WORKFLOW.with_name("agentplugins-native-clients.yml")
RESILIENCE_WORKFLOW = WORKFLOW.with_name("package-manager-resilience.yml")


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
        # Ordinary PRs stay on the four already-qualified source-build OSs.
        # Weekly/manual deep E2E expands to both genuine ARM64 hosted runners.
        self.assertIn("os: ${{ fromJSON(", self.body)
        self.assertIn('["ubuntu-24.04","windows-2025","macos-15","macos-15-intel"]', self.body)
        self.assertIn('["ubuntu-24.04","ubuntu-24.04-arm","windows-2025","windows-11-arm","macos-15","macos-15-intel"]', self.body)
        self.assertIn("schedule:", self.body)
        self.assertIn("fail-fast: false", self.body)

    def test_release_qualification_reuses_workflows_and_fail_closed_profiles(self):
        root = WORKFLOW.parent
        version = (root / "agentplugins-published-version.yml").read_text(encoding="utf-8")
        installer = (root / "agentplugins-native-install.yml").read_text(encoding="utf-8")
        self.assertIn("workflow_call:", version)
        self.assertIn("releases/latest", version)
        self.assertIn("npm view universal-agent-plugins dist-tags.latest", version)
        self.assertIn("npm view \"universal-agent-plugins@$version\" version", version)
        for target in ("darwin-amd64", "darwin-arm64", "linux-amd64",
                       "linux-arm64", "windows-amd64", "windows-arm64"):
            self.assertIn("target: " + target, installer)
        self.assertIn("needs: release-version", installer)
        self.assertIn("schedule:", installer)
        for workflow in ("agentplugins-published-windows-audit.yml",
                         "agentplugins-published-macos-audit.yml"):
            body = (root / workflow).read_text(encoding="utf-8")
            with self.subTest(workflow=workflow):
                self.assertIn("uses: ./.github/workflows/agentplugins-published-version.yml", body)
                self.assertIn("Agentplugins NPM Publish", body)
                self.assertIn("schedule:", body)
                self.assertIn("AUDIT_LIMIT:", body)
                self.assertIn('["codex","claude","opencode","gemini","copilot"]', body)
                self.assertIn('["codex","claude","opencode","gemini","copilot","cline","kimi","qoder","pi"]', body)
                self.assertIn("kind: plugin", body)
                self.assertIn("kind: [plugin, skill]", body)
                self.assertIn("scripts/live-install-audit.py", body)
                self.assertIn("AUDIT_VENDOR_OUTCOME:", body)
                self.assertIn("if: always()", body)
                self.assertNotIn("continue-on-error: true", body)
        for workflow in ("agentplugins-published-catalog-audit.yml",
                         "agentplugins-published-other-clients-audit.yml"):
            body = (root / workflow).read_text(encoding="utf-8")
            with self.subTest(workflow=workflow):
                self.assertIn("uses: ./.github/workflows/agentplugins-published-version.yml", body)
                self.assertIn("Agentplugins NPM Publish", body)
                self.assertIn("schedule:", body)
                self.assertIn("scripts/live-install-audit.py", body)
                self.assertNotIn("universal-agent-plugins@0.1.101", body)

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
            "'.github/workflows/agentplugins-native-clients.yml'",
            "'.github/workflows/package-manager-resilience.yml'",
            "'.github/workflows/package-manager-e2e.yml'",
        ):
            event_paths = re.findall(r"(?m)^      - " + re.escape(path) + r"$", self.body)
            self.assertEqual(len(event_paths), 2, path)
        self.assertRegex(self.body, r"(?m)^  cancel-in-progress:.*pull_request.*push.*$")

    def test_each_preflight_rejection_is_an_independent_fail_closed_step(self):
        # One native command per Actions step: Windows PowerShell must never
        # return the second test's exit code after the first test fails.
        for test in ("test_package_manager_e2e.py", "test_package_manager_ci_policy.py"):
            run = "        run: python -B -m unittest discover -s scripts -p " + test
            self.assertEqual(self.body.count(run + "\n"), 2, test)

    def test_native_matrix_not_lost_when_draft_becomes_ready(self):
        native = NATIVE_WORKFLOW.read_text(encoding="utf-8")
        self.assertIn("types: [opened, synchronize, reopened, ready_for_review]", native)
        self.assertIn("github.event.pull_request.draft == false", native)
        self.assertIn("client: [codex, claude, opencode]", native)
        self.assertIn("--project-components", native)
        self.assertIn("schedule:", native)
        self.assertIn("workflow_dispatch:", native)

    def test_resilience_runs_at_open_or_ready_and_targets_only_risk_changes(self):
        resilience = RESILIENCE_WORKFLOW.read_text(encoding="utf-8")
        self.assertIn("types: [opened, synchronize, reopened, ready_for_review]", resilience)
        self.assertIn("github.event.pull_request.draft == false", resilience)
        self.assertIn("schedule:", resilience)
        self.assertIn("workflow_dispatch:", resilience)
        self.assertIn("-fuzz '^FuzzVersionSelectionOrderAndPinReplay$'", resilience)
        self.assertIn("go test -race", resilience)
        for path in ("environment/resolver_reliability_test.go", "processlock/reliability_test.go"):
            self.assertIn(path, resilience)


if __name__ == "__main__":
    unittest.main()
