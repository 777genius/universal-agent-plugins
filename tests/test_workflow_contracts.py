import unittest
from pathlib import Path

import yaml


REPO_ROOT = Path(__file__).resolve().parents[1]
WORKFLOW_PATH = REPO_ROOT / ".github/workflows/live-e2e.yml"
VALIDATE_WORKFLOW_PATH = REPO_ROOT / ".github/workflows/validate.yml"
PAGES_WORKFLOW_PATH = REPO_ROOT / ".github/workflows/pages.yml"


def load_workflow() -> dict[str, object]:
    return yaml.load(WORKFLOW_PATH.read_text(), Loader=yaml.BaseLoader)


def job_run_commands(job: dict[str, object]) -> str:
    return "\n".join(
        step.get("run", "")
        for step in job["steps"]
        if isinstance(step, dict)
    )


class WorkflowContractTests(unittest.TestCase):
    def test_pages_concurrency_isolates_prs_from_production(self) -> None:
        workflow = yaml.load(
            PAGES_WORKFLOW_PATH.read_text(), Loader=yaml.BaseLoader
        )

        self.assertEqual(
            workflow["concurrency"]["group"],
            "${{ github.event_name == 'pull_request' && format('pages-pr-{0}', github.event.pull_request.number) || 'pages-production' }}",
        )
        self.assertEqual(workflow["concurrency"]["cancel-in-progress"], "true")

    def test_release_publish_uses_tested_script(self) -> None:
        workflow = WORKFLOW_PATH.read_text()

        self.assertIn("run: scripts/publish_github_release.sh", workflow)

    def test_agentplugins_version_cannot_be_overridden_at_dispatch(self) -> None:
        workflow = load_workflow()
        inputs = workflow["on"]["workflow_dispatch"]["inputs"]
        workflow_text = WORKFLOW_PATH.read_text()

        self.assertNotIn("agentplugins_version", inputs)
        self.assertNotIn("github.event.inputs.agentplugins_version", workflow_text)

    def test_scheduled_marketplace_e2e_resolves_latest_immutable_release(self) -> None:
        workflow = load_workflow()
        inputs = workflow["on"]["workflow_dispatch"]["inputs"]
        job = workflow["jobs"]["codex-marketplace-install"]
        commands = job_run_commands(job)
        install_step = next(
            step
            for step in job["steps"]
            if isinstance(step, dict)
            and "scripts/run_codex_install_e2e.py" in step.get("run", "")
        )

        self.assertNotIn("default", inputs["marketplace_ref"])
        self.assertNotIn("v0.1.5", WORKFLOW_PATH.read_text())
        self.assertIn("releases/latest", commands)
        self.assertIn("^v[0-9]+\\.[0-9]+\\.[0-9]+$", commands)
        self.assertIn("git merge-base --is-ancestor", commands)
        self.assertEqual(
            install_step["env"]["MARKETPLACE_REF"],
            "${{ steps.marketplace.outputs.ref }}",
        )

    def test_agentplugins_lifecycle_and_projection_jobs_are_isolated(self) -> None:
        jobs = load_workflow()["jobs"]
        expected = {
            "agentplugins-package-lifecycle": (
                "scripts/run_agentplugins_lifecycle_e2e.py",
                "/tmp/agentplugins-package-lifecycle-e2e.json",
            ),
            "agentplugins-hero-projections": (
                "scripts/run_agentplugins_hero_matrix_e2e.py",
                "/tmp/agentplugins-hero-projections-e2e.json",
            ),
        }
        pinned_uses = {
            "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
            "actions/setup-python@5fda3b95a4ea91299a34e894583c3862153e4b97",
            "actions/setup-node@820762786026740c76f36085b0efc47a31fe5020",
            "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a",
        }

        for name, (runner, evidence) in expected.items():
            with self.subTest(job=name):
                job = jobs[name]
                commands = job_run_commands(job)
                self.assertEqual(job["env"]["AGENTPLUGINS_VERSION"], "0.1.6")
                self.assertIn(
                    'npm install --global "universal-agent-plugins@${AGENTPLUGINS_VERSION}"',
                    commands,
                )
                self.assertIn(runner, commands)
                self.assertIn(
                    '--expected-version "${AGENTPLUGINS_VERSION}"', commands
                )
                self.assertIn(
                    'catalog_digest="sha256:$(sha256sum catalog/v1/catalog.json',
                    commands,
                )
                self.assertIn('--catalog-digest "${catalog_digest}"', commands)
                self.assertIn(evidence, commands)
                self.assertIn("check-jsonschema --schemafile", commands)
                self.assertIn("scripts/validate_client_evidence.py --file", commands)
                uses = {
                    step["uses"]
                    for step in job["steps"]
                    if isinstance(step, dict) and "uses" in step
                }
                self.assertEqual(uses, pinned_uses)

    def test_release_gate_needs_native_lifecycle_and_projection_jobs(self) -> None:
        jobs = load_workflow()["jobs"]
        needs = set(jobs["publish-release"]["needs"])
        agentplugins_jobs = {
            "copilot-native-lifecycle",
            "agentplugins-package-lifecycle",
            "agentplugins-chatgpt-catalog-v2",
            "agentplugins-hero-projections",
        }

        self.assertTrue(
            {
                "codex-marketplace-install",
                *agentplugins_jobs,
            }.issubset(needs)
        )
        for name in agentplugins_jobs:
            with self.subTest(job=name):
                self.assertEqual(
                    jobs[name]["env"]["AGENTPLUGINS_VERSION"], "0.1.6"
                )

    def test_copilot_native_job_binds_evidence_to_local_catalog(self) -> None:
        job = load_workflow()["jobs"]["copilot-native-lifecycle"]
        commands = job_run_commands(job)

        self.assertIn(
            'catalog_digest="sha256:$(sha256sum catalog/v1/catalog.json', commands
        )
        self.assertIn('--catalog-digest "${catalog_digest}"', commands)

    def test_chatgpt_job_gates_released_cli_against_catalog_v2(self) -> None:
        job = load_workflow()["jobs"]["agentplugins-chatgpt-catalog-v2"]
        commands = job_run_commands(job)
        uses = {
            step["uses"]
            for step in job["steps"]
            if isinstance(step, dict) and "uses" in step
        }

        self.assertEqual(job["env"]["AGENTPLUGINS_VERSION"], "0.1.6")
        self.assertIn(
            'npm install --global "universal-agent-plugins@${AGENTPLUGINS_VERSION}"',
            commands,
        )
        self.assertIn("scripts/run_agentplugins_chatgpt_catalog_e2e.py", commands)
        self.assertIn(
            'catalog_digest="sha256:$(sha256sum catalog/v2/catalog.json', commands
        )
        run_step = next(
            step
            for step in job["steps"]
            if isinstance(step, dict)
            and "scripts/run_agentplugins_chatgpt_catalog_e2e.py" in step.get("run", "")
        )
        self.assertIn("/catalog/v2/catalog.json", run_step["env"]["CATALOG_URL"])
        self.assertIn('--expected-version "${AGENTPLUGINS_VERSION}"', commands)
        self.assertIn('--catalog-digest "${catalog_digest}"', commands)
        self.assertIn(
            "/tmp/agentplugins-chatgpt-catalog-v2-e2e.json", commands
        )
        self.assertIn("check-jsonschema --schemafile", commands)
        self.assertIn("scripts/validate_client_evidence.py --file", commands)
        self.assertEqual(
            uses,
            {
                "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
                "actions/setup-python@5fda3b95a4ea91299a34e894583c3862153e4b97",
                "actions/setup-node@820762786026740c76f36085b0efc47a31fe5020",
                "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a",
            },
        )

    def test_pull_request_gate_runs_released_consumer_e2e(self) -> None:
        workflow = yaml.load(VALIDATE_WORKFLOW_PATH.read_text(), Loader=yaml.BaseLoader)
        job = workflow["jobs"]["agentplugins-chatgpt-catalog-v2"]
        commands = job_run_commands(job)

        self.assertIn("pull_request", workflow["on"])
        self.assertEqual(job["env"]["AGENTPLUGINS_VERSION"], "0.1.6")
        self.assertIn("pull_request.head.repo.full_name", job["env"]["CATALOG_REPOSITORY"])
        self.assertIn("pull_request.head.sha", job["env"]["CATALOG_REVISION"])
        self.assertIn(
            'npm install --global "universal-agent-plugins@${AGENTPLUGINS_VERSION}"',
            commands,
        )
        self.assertIn("scripts/run_agentplugins_chatgpt_catalog_e2e.py", commands)
        self.assertIn("${CATALOG_REPOSITORY}/${CATALOG_REVISION}", commands)
        self.assertIn("catalog/v2/catalog.json", commands)
        self.assertIn("check-jsonschema --schemafile", commands)
        self.assertIn("scripts/validate_client_evidence.py --file", commands)

    def test_untrusted_pull_request_gate_reproduces_bridges_offline(self) -> None:
        workflow = yaml.load(VALIDATE_WORKFLOW_PATH.read_text(), Loader=yaml.BaseLoader)
        self.assertIn("pull_request", workflow["on"])
        job = workflow["jobs"]["bridge-reproduction"]
        commands = job_run_commands(job)

        self.assertEqual(job["permissions"], {"contents": "read"})
        self.assertNotIn("secrets.", yaml.safe_dump(job))
        self.assertIn("scripts/build-bridges", commands)
        self.assertIn("--root tests/fixtures", commands)
        self.assertIn("--upstream-mirror", commands)
        self.assertIn(" check", commands)
        self.assertNotIn("curl ", commands)
        self.assertNotIn("wget ", commands)
        self.assertEqual(
            {
                step["uses"]
                for step in job["steps"]
                if isinstance(step, dict) and "uses" in step
            },
            {
                "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
                "actions/setup-python@5fda3b95a4ea91299a34e894583c3862153e4b97",
            },
        )


if __name__ == "__main__":
    unittest.main()
