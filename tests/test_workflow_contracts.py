import re
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
LAUNCH = ROOT / ".github/workflows/launch-evidence-e2e.yml"
LIVE = ROOT / ".github/workflows/live-e2e.yml"
PAGES = ROOT / ".github/workflows/pages.yml"
VALIDATE = ROOT / ".github/workflows/validate.yml"


def load(path: Path):
    return yaml.load(path.read_text(), Loader=yaml.BaseLoader)


def commands(job):
    return "\n".join(step.get("run", "") for step in job["steps"] if isinstance(step, dict))


class WorkflowContractTests(unittest.TestCase):
    def test_pages_concurrency_isolates_prs_from_production(self) -> None:
        workflow = load(PAGES)
        self.assertEqual(workflow["concurrency"]["group"], "${{ github.event_name == 'pull_request' && format('pages-pr-{0}', github.event.pull_request.number) || 'pages-production' }}")
        self.assertEqual(workflow["concurrency"]["cancel-in-progress"], "true")

    def test_launch_pr_is_fixture_only_and_has_no_secrets_or_runtime_claim(self) -> None:
        workflow = load(LAUNCH)
        job = workflow["jobs"]["fixture-only-non-runtime"]
        body = yaml.safe_dump(job)
        self.assertIn("pull_request", workflow["on"])
        self.assertIn("--mode fixture-only", commands(job))
        self.assertNotIn("secrets.", body)
        self.assertEqual(job["permissions"], {"contents": "read"})
        self.assertLessEqual(int(job["timeout-minutes"]), 10)

    def test_manual_gate_is_enforced_and_checksum_bound(self) -> None:
        workflow = load(LAUNCH)
        job = workflow["jobs"]["enforced-stable-gate"]
        text = commands(job)
        inputs = workflow["on"]["workflow_dispatch"]["inputs"]
        for name in ("agentplugins_version", "binary_url", "binary_sha256", "directory_origin", "directory_bundle_url", "directory_bundle_sha256", "live_inputs_url", "live_inputs_sha256", "consent"):
            self.assertEqual(inputs[name]["required"], "true")
        self.assertIn("--binary-digest", text)
        self.assertIn("--expected-version", text)
        self.assertIn("--directory-snapshot", text)
        self.assertIn("--notion-oauth-attestation", text)
        self.assertIn("--chatgpt-attestation", text)
        self.assertIn("sha256sum --check --strict", text)
        self.assertNotIn("npm install", text)
        self.assertNotIn("catalog/v1", text)
        self.assertNotIn("catalog/v2", text)
        self.assertNotIn("0.1.6", text)
        self.assertEqual(job["permissions"], {"contents": "read"})
        self.assertLessEqual(int(job["timeout-minutes"]), 90)

    def test_owned_workflows_pin_actions_and_upload_checksums_immutably(self) -> None:
        for path in (LAUNCH, LIVE):
            text = path.read_text()
            with self.subTest(path=path.name):
                uses = re.findall(r"uses:\s+([^\s#]+)", text)
                self.assertTrue(uses)
                self.assertTrue(all(re.search(r"@[0-9a-f]{40}$", item) for item in uses))
                self.assertIn("SHA256SUMS", text)
                self.assertIn("overwrite: false", text)
                self.assertNotIn("universal-agent-plugins@", text)
                self.assertNotIn("AGENTPLUGINS_VERSION: \"0.1.6\"", text)

    def test_live_workflow_is_read_only_and_does_not_publish(self) -> None:
        workflow = load(LIVE)
        text = LIVE.read_text()
        self.assertEqual(workflow["permissions"], {"contents": "read"})
        self.assertNotIn("publish-release", text)
        self.assertNotIn("contents: write", text)
        self.assertNotIn("catalog/v1", text)
        self.assertNotIn("catalog/v2", text)

    def test_untrusted_pull_request_bridge_reproduction_remains_secretless(self) -> None:
        workflow = load(VALIDATE)
        job = workflow["jobs"]["bridge-reproduction"]
        text = commands(job)
        self.assertEqual(job["permissions"], {"contents": "read"})
        self.assertNotIn("secrets.", yaml.safe_dump(job))
        self.assertIn("scripts/build-bridges check", text)
        self.assertNotIn("curl ", text)


if __name__ == "__main__":
    unittest.main()
