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

    def test_live_gate_resolves_official_release_and_native_matrix(self) -> None:
        workflow = load(LAUNCH)
        native = workflow["jobs"]["native-release"]
        npm = workflow["jobs"]["node22-npm-facade"]
        aggregate = workflow["jobs"]["aggregate-one-release"]
        enforced = workflow["jobs"]["enforced-stable-gate"]
        inputs = workflow["on"]["workflow_dispatch"]["inputs"]
        self.assertEqual(inputs["consent"]["required"], "true")
        self.assertNotIn("release_tag", inputs)
        self.assertEqual(set(workflow["on"]["workflow_call"]["inputs"]), {"consent", "publication_id", "publication_sequence", "publication_snapshot_digest", "publication_source_commit"})
        self.assertTrue(all(workflow["on"]["workflow_call"]["inputs"][name]["required"] == "true" for name in ("publication_id", "publication_sequence", "publication_snapshot_digest", "publication_source_commit")))
        self.assertIn("workflow_call", workflow["on"])
        slots = native["strategy"]["matrix"]["include"]
        self.assertEqual({(slot["os"], slot["architecture"]) for slot in slots}, {
            ("macos", "arm64"), ("macos", "amd64"), ("linux", "arm64"),
            ("linux", "amd64"), ("windows", "amd64"), ("windows", "arm64"),
        })
        self.assertEqual({slot["asset"] for slot in slots}, {
            "agentplugins_0.1.8_darwin_arm64", "agentplugins_0.1.8_darwin_amd64",
            "agentplugins_0.1.8_linux_arm64", "agentplugins_0.1.8_linux_amd64",
            "agentplugins_0.1.8_windows_amd64.exe", "agentplugins_0.1.8_windows_arm64.exe",
        })
        self.assertIn("prepare_launch_evidence.py", commands(native))
        self.assertIn("node-version: '22'", yaml.safe_dump(npm))
        self.assertIn("npm install --global", commands(npm))
        self.assertIn("npm audit signatures", commands(npm))
        self.assertIn("--npm-facade", commands(npm))
        self.assertIn("universal-agent-plugins-0.1.8.tgz", commands(npm))
        self.assertIn("--asset-name agentplugins_0.1.8_linux_amd64", commands(npm))
        self.assertNotIn("universal-agent-plugins.tgz", commands(npm))
        self.assertNotRegex(commands(npm), r"github\.com/.*\.tgz")
        self.assertIn("inputs.consent", aggregate["if"])
        self.assertIn("release_manifest_digest", commands(aggregate))
        self.assertEqual(set(enforced["needs"]), {"native-release", "node22-npm-facade", "aggregate-one-release"})
        self.assertEqual(enforced["environment"], "stable-launch-e2e")
        self.assertIn("--prepared-context", commands(enforced))
        self.assertIn("--native-observations", commands(enforced))
        self.assertIn("request_launch_runtime_observations.py", commands(enforced))
        self.assertIn("--observer-bundle", commands(enforced))
        self.assertIn("observer-bundle.schema.json", commands(enforced))
        self.assertIn("STABLE_LAUNCH_OBSERVER_ED25519_PUBLIC_KEY", yaml.safe_dump(enforced))
        self.assertIn("STABLE_LAUNCH_OBSERVER_KEY_ID", yaml.safe_dump(enforced))
        body = LAUNCH.read_text()
        for forbidden in ("binary_url", "binary_sha256", "directory_bundle_url", "directory_bundle_sha256", "live_inputs_url", "scenario-driver"):
            self.assertNotIn(forbidden, body)
        self.assertNotIn("inputs.release_tag", body)
        self.assertNotIn("--release-tag", body)
        production = (ROOT / "tests/e2e/production-launch.json").read_text()
        self.assertIn('"cli_release_repository": "777genius/plugin-kit-ai"', production)
        self.assertIn('"cli_release_tag": "agentplugins-v0.1.8"', production)
        prepare = (ROOT / "scripts/prepare_launch_evidence.py").read_text()
        self.assertNotIn('os.environ.get("GITHUB_TOKEN")', prepare)
        self.assertIn("token=None", prepare)

    def test_false_consent_skips_every_live_and_aggregate_job(self) -> None:
        workflow = load(LAUNCH)
        for name in ("native-release", "node22-npm-facade", "aggregate-one-release", "enforced-stable-gate"):
            with self.subTest(job=name):
                condition = workflow["jobs"][name]["if"]
                self.assertIn("inputs.consent", condition)
        self.assertEqual(workflow["jobs"]["fixture-only-non-runtime"]["if"], "github.event_name == 'pull_request'")

    def test_owned_workflows_pin_actions_and_upload_checksums_immutably(self) -> None:
        for path in (LAUNCH, LIVE):
            text = path.read_text()
            with self.subTest(path=path.name):
                uses = re.findall(r"uses:\s+([^\s#]+)", text)
                self.assertTrue(uses)
                self.assertTrue(all(item.startswith("./") or re.search(r"@[0-9a-f]{40}$", item) for item in uses))
                self.assertIn("SHA256SUMS", text)
                self.assertIn("overwrite: false", text)
                if path == LAUNCH:
                    self.assertIn("agentplugins_0.1.8_linux_amd64", text)
                self.assertNotIn("AGENTPLUGINS_VERSION: \"0.1.6\"", text)

    def test_live_workflow_is_read_only_and_does_not_publish(self) -> None:
        workflow = load(LIVE)
        text = LIVE.read_text()
        self.assertEqual(workflow["permissions"], {"contents": "read"})
        self.assertNotIn("publish-release", text)
        self.assertNotIn("contents: write", text)
        self.assertNotIn("catalog/v1", text)
        self.assertNotIn("catalog/v2", text)
        self.assertIn("workflow_call", workflow["on"])
        required = workflow["jobs"]["required-stable-launch-evidence"]
        self.assertEqual(required["uses"], "./.github/workflows/launch-evidence-e2e.yml")
        self.assertNotIn("release_tag", required.get("with", {}))
        self.assertEqual(required["permissions"], {"actions": "read", "contents": "read", "id-token": "write"})

    def test_directory_release_requires_reusable_live_evidence(self) -> None:
        workflow = load(ROOT / ".github/workflows/directory-publication.yml")
        required = workflow["jobs"]["required_stable_launch_evidence"]
        self.assertEqual(required["needs"], "deploy")
        self.assertEqual(required["uses"], "./.github/workflows/live-e2e.yml")
        self.assertEqual(required["with"]["consent"], "true")
        self.assertEqual(required["permissions"], {"actions": "read", "contents": "read", "id-token": "write"})

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
