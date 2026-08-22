import re
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
LAUNCH = ROOT / ".github/workflows/launch-evidence-e2e.yml"
LIVE = ROOT / ".github/workflows/live-e2e.yml"
PAGES = ROOT / ".github/workflows/pages.yml"
VALIDATE = ROOT / ".github/workflows/validate.yml"
DIRECTORY_PUBLICATION = ROOT / ".github/workflows/directory-publication.yml"
PUBLICATION_INPUTS = {
    "publication_id": "string",
    "publication_sequence": "number",
    "publication_snapshot_digest": "string",
    "publication_source_commit": "string",
    "publication_ledger_commit": "string",
}


def load(path: Path):
    return yaml.load(path.read_text(), Loader=yaml.BaseLoader)


def commands(job):
    return "\n".join(step.get("run", "") for step in job["steps"] if isinstance(step, dict))


class WorkflowContractTests(unittest.TestCase):
    def test_directory_publication_prepare_installs_bridge_runtime_dependencies(self) -> None:
        workflow = load(DIRECTORY_PUBLICATION)
        prepare_commands = commands(workflow["jobs"]["prepare"])
        self.assertIn("PyYAML==6.0.3", prepare_commands)

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
        self.assertEqual(set(workflow["on"]["workflow_call"]["inputs"]), {"consent", *PUBLICATION_INPUTS})
        self.assertTrue(all(workflow["on"]["workflow_call"]["inputs"][name]["required"] == "true" for name in PUBLICATION_INPUTS))
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
        self.assertNotIn("fetch_production_directory", prepare)
        self.assertIn("fetch_staged_directory", prepare)
        self.assertIn("--publication-ledger-commit", commands(native))

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
        self.assertEqual(required["permissions"], {
            "actions": "read",
            "attestations": "read",
            "contents": "read",
            "id-token": "write",
        })

    def test_nested_launch_workflow_permissions_never_escalate(self) -> None:
        publication = load(DIRECTORY_PUBLICATION)
        live = load(LIVE)
        launch = load(LAUNCH)
        expected = {
            "actions": "read",
            "attestations": "read",
            "contents": "read",
            "id-token": "write",
        }
        publication_call = publication["jobs"]["required_stable_launch_evidence"]
        live_call = live["jobs"]["required-stable-launch-evidence"]
        self.assertEqual(publication_call["permissions"], expected)
        self.assertEqual(live_call["permissions"], expected)

        permission_levels = {"none": 0, "read": 1, "write": 2}
        for job_name, job in launch["jobs"].items():
            for permission, level in job.get("permissions", {}).items():
                with self.subTest(job=job_name, permission=permission):
                    self.assertIn(permission, expected)
                    self.assertLessEqual(
                        permission_levels[level],
                        permission_levels[expected[permission]],
                    )

    def test_scheduled_live_workflow_never_calls_staged_publication_gate(self) -> None:
        workflow = load(LIVE)
        required = workflow["jobs"]["required-stable-launch-evidence"]
        self.assertEqual(
            required["if"],
            "github.event_name == 'workflow_call' || github.event_name == 'workflow_dispatch'",
        )
        scheduled = {
            name: job
            for name, job in workflow["jobs"].items()
            if job.get("if") == "github.event_name == 'schedule'"
        }
        self.assertEqual(
            set(scheduled),
            {"scheduled-fixture-contract", "scheduled-production-directory-observation"},
        )
        scheduled_body = yaml.safe_dump(scheduled)
        self.assertNotIn("inputs.publication_", scheduled_body)
        self.assertNotIn("launch-evidence-e2e.yml", scheduled_body)
        observation = commands(scheduled["scheduled-production-directory-observation"])
        self.assertIn("fetch_production_directory", observation)
        self.assertIn('"runtime_claims": False', observation)
        self.assertIn('"oauth_claims": False', observation)
        self.assertIn("SHA256SUMS", observation)
        public_reads = workflow["jobs"]["public-read-flows"]
        self.assertEqual(
            public_reads["if"], "github.event_name == 'schedule' || inputs.consent"
        )

    def test_publication_identity_contract_matches_across_both_reusable_edges(self) -> None:
        launch = load(LAUNCH)
        live = load(LIVE)
        publication = load(DIRECTORY_PUBLICATION)
        launch_inputs = launch["on"]["workflow_call"]["inputs"]
        live_inputs = live["on"]["workflow_call"]["inputs"]
        live_call = live["jobs"]["required-stable-launch-evidence"]["with"]
        publication_call = publication["jobs"]["required_stable_launch_evidence"]["with"]
        expected = {"consent", *PUBLICATION_INPUTS}
        self.assertEqual(set(launch_inputs), expected)
        self.assertEqual(set(live_inputs), expected)
        self.assertEqual(set(live_call), expected)
        self.assertEqual(set(publication_call), expected)
        for name, expected_type in PUBLICATION_INPUTS.items():
            with self.subTest(input=name):
                self.assertEqual(launch_inputs[name]["required"], "true")
                self.assertEqual(live_inputs[name]["required"], "true")
                self.assertEqual(launch_inputs[name]["type"], expected_type)
                self.assertEqual(live_inputs[name]["type"], expected_type)
                self.assertEqual(live_call[name], "${{ inputs." + name + " }}")
        self.assertEqual(publication_call["publication_id"], "${{ needs.sign.outputs.publication_id }}")
        self.assertEqual(publication_call["publication_sequence"], "${{ fromJSON(needs.sign.outputs.sequence) }}")
        self.assertEqual(publication_call["publication_snapshot_digest"], "${{ needs.sign.outputs.snapshot_digest }}")
        self.assertEqual(publication_call["publication_source_commit"], "${{ needs.sign.outputs.marker_commit }}")
        self.assertEqual(publication_call["publication_ledger_commit"], "${{ needs.materialize_site.outputs.ledger_commit }}")

    def test_directory_release_stages_exact_identity_before_reusable_gate_and_promotion(self) -> None:
        workflow = load(DIRECTORY_PUBLICATION)
        required = workflow["jobs"]["required_stable_launch_evidence"]
        self.assertEqual(set(required["needs"]), {"sign", "materialize_site", "gate_exact_staged_publication"})
        self.assertEqual(required["uses"], "./.github/workflows/live-e2e.yml")
        self.assertEqual(required["with"]["consent"], "true")
        self.assertEqual(required["permissions"], {
            "actions": "read",
            "attestations": "read",
            "contents": "read",
            "id-token": "write",
        })
        exact = workflow["jobs"]["gate_exact_staged_publication"]
        verify = next(step for step in exact["steps"] if step.get("name") == "Verify staged bytes and immutable identity before promotion")
        self.assertEqual(exact["needs"], ["sign", "materialize_site"])
        for field in (
            "EXPECTED_SEQUENCE", "EXPECTED_SNAPSHOT_DIGEST", "EXPECTED_PUBLICATION_ID",
            "EXPECTED_SOURCE_COMMIT", "EXPECTED_SIGNED_LEDGER_COMMIT",
            "EXPECTED_MATERIALIZED_LEDGER_COMMIT",
        ):
            self.assertIn(field, verify["env"])
        self.assertIn("raw.githubusercontent.com", verify["run"])
        deploy_needs = workflow["jobs"]["deploy"]["needs"]
        self.assertIn("sign", deploy_needs)
        self.assertIn("gate_exact_staged_publication", deploy_needs)
        self.assertIn("gate_launch_approval", deploy_needs)
        production = workflow["jobs"]["observe_production_latest"]
        self.assertIn("deploy", production["needs"])
        self.assertNotIn("observe_production_latest", required["needs"])
        self.assertIn("observe_production_latest.py", commands(production))
        self.assertEqual(production["permissions"], {"contents": "read"})

    def test_sequence_one_cannot_promote_without_launch_ceremony(self) -> None:
        workflow = load(DIRECTORY_PUBLICATION)
        launch_if = workflow["jobs"]["required_stable_launch_evidence"]["if"]
        record_if = workflow["jobs"]["record_launch_approval"]["if"]
        marker_if = workflow["jobs"]["gate_launch_approval"]["if"]
        deploy_if = workflow["jobs"]["deploy"]["if"]
        self.assertIn("needs.sign.outputs.sequence == '1'", launch_if)
        self.assertIn("needs.sign.outputs.sequence == '1'", record_if)
        self.assertIn("needs.required_stable_launch_evidence.result == 'success'", record_if)
        self.assertIn("needs.sign.outputs.sequence == '1'", marker_if)
        self.assertIn("needs.record_launch_approval.result == 'success'", marker_if)
        self.assertIn("needs.gate_launch_approval.result == 'success'", deploy_if)

    def test_normal_refresh_skips_launch_but_keeps_exact_publication_gates(self) -> None:
        workflow = load(DIRECTORY_PUBLICATION)
        launch_if = workflow["jobs"]["required_stable_launch_evidence"]["if"]
        marker_if = workflow["jobs"]["gate_launch_approval"]["if"]
        deploy_if = workflow["jobs"]["deploy"]["if"]
        self.assertNotIn("needs.sign.outputs.sequence > 1", launch_if)
        self.assertIn("needs.sign.outputs.sequence > 1", marker_if)
        self.assertIn("needs.required_stable_launch_evidence.result == 'skipped'", marker_if)
        self.assertIn("needs.record_launch_approval.result == 'skipped'", marker_if)
        self.assertIn("always()", deploy_if)
        self.assertIn("needs.gate_launch_approval.result == 'success'", deploy_if)
        for required_result in (
            "needs.sign.result == 'success'",
            "needs.materialize_site.result == 'success'",
            "needs.gate_exact_staged_publication.result == 'success'",
        ):
            self.assertIn(required_result, deploy_if)

    def test_emergency_revocation_uses_the_same_higher_sequence_promotion_contract(self) -> None:
        workflow = load(DIRECTORY_PUBLICATION)
        deploy = workflow["jobs"]["deploy"]
        self.assertEqual(
            set(deploy["needs"]),
            {"sign", "materialize_site", "gate_exact_staged_publication", "gate_launch_approval"},
        )
        self.assertIn("needs.gate_launch_approval.result == 'success'", deploy["if"])
        self.assertIn("gate_exact_staged_publication", deploy["needs"])

    def test_untrusted_pull_request_bridge_reproduction_remains_secretless(self) -> None:
        workflow = load(VALIDATE)
        job = workflow["jobs"]["bridge-reproduction"]
        text = commands(job)
        self.assertEqual(job["permissions"], {"contents": "read"})
        self.assertNotIn("secrets.", yaml.safe_dump(job))
        self.assertIn("scripts/build-bridges check", text)
        self.assertNotIn("curl ", text)

    def test_pull_request_ci_reacquires_only_changed_external_releases(self) -> None:
        workflow = load(VALIDATE)
        job = workflow["jobs"]["portable-catalog"]
        step = next(item for item in job["steps"] if item.get("name") == "Reacquire and validate changed external releases")
        self.assertEqual(step["if"], "github.event_name == 'pull_request'")
        self.assertEqual(step["env"]["BASE_REVISION"], "${{ github.event.pull_request.base.sha }}")
        self.assertIn("--external-release-check changed", step["run"])
        self.assertIn('--base-revision "${BASE_REVISION}"', step["run"])
        self.assertEqual(next(item for item in job["steps"] if item.get("uses", "").startswith("actions/checkout"))["with"]["fetch-depth"], "0")


if __name__ == "__main__":
    unittest.main()
