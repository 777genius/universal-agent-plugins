from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
PUBLICATION = ROOT / "registry" / "publication"
sys.path.insert(0, str(PUBLICATION))
import launch_approval


GIT = "/usr/bin/git"
CONTRACT = PUBLICATION / "launch-approved-marker.json"
MARKER_REF = "refs/tags/directory-publication-schema-1-launch-approved"
SEQUENCE_PREFIX = "directory-publication-schema-1-sequence-"


def git(repo: Path, *args: str) -> str:
    return subprocess.run(
        [GIT, "-C", str(repo), *args], check=True, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    ).stdout.strip()


class LaunchApprovalTransitionTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.repo = Path(self.temporary.name)
        git(self.repo, "init", "-q")
        git(self.repo, "config", "user.name", "test")
        git(self.repo, "config", "user.email", "test@example.invalid")
        (self.repo / "seed").write_text("seed\n")
        git(self.repo, "add", "seed")
        git(self.repo, "commit", "-qm", "seed")
        self.seed = git(self.repo, "rev-parse", "HEAD")
        self.q1 = self._commit_sequence(1, parent_materialization=None)
        git(self.repo, "tag", f"{SEQUENCE_PREFIX}{1:020d}", self.q1)
        self.m1 = self._materialize("site one")
        self.q2 = self._commit_sequence(2, parent_materialization=self.m1)
        git(self.repo, "tag", f"{SEQUENCE_PREFIX}{2:020d}", self.q2)
        self.m2 = self._materialize("site two")

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def _write_json(self, relative: str, value: dict) -> None:
        path = self.repo / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n")

    def _commit_sequence(self, sequence: int, parent_materialization: str | None) -> str:
        if parent_materialization:
            git(self.repo, "checkout", "-q", parent_materialization)
        feed = "registry/schemas/1"
        if sequence == 1:
            self._write_json(f"{feed}/ledger-contract.json", {
                "contract_version": 1,
                "initial_sequence": 1,
                "schema_version": 1,
                "seed_commit": self.seed,
                "sequence_tag_prefix": SEQUENCE_PREFIX,
            })
        name = f"{sequence:020d}"
        snapshot = {
            "publication_id": f"run-{sequence}",
            "sequence": sequence,
            "source_commit": f"{sequence:040x}",
        }
        self._write_json(f"{feed}/snapshots/{name}.json", snapshot)
        self._write_json(f"{feed}/snapshots/{name}.envelope.json", {
            "key_id": "uap-directory-2026-01",
            "sequence": sequence,
            "snapshot_digest": f"sha256:{sequence:064x}",
        })
        self._write_json(f"{feed}/latest.json", {
            "envelope_path": f"snapshots/{name}.envelope.json",
            "sequence": sequence,
            "snapshot_path": f"snapshots/{name}.json",
        })
        git(self.repo, "add", "registry")
        git(self.repo, "commit", "-qm", f"sequence {sequence}")
        return git(self.repo, "rev-parse", "HEAD")

    def _materialize(self, value: str) -> str:
        (self.repo / "index.html").write_text(value)
        git(self.repo, "add", "index.html")
        git(self.repo, "commit", "-qm", "materialize")
        return git(self.repo, "rev-parse", "HEAD")

    def _validate(
        self, current: str, marker: str | None = None,
        repository: str = "777genius/universal-agent-plugins",
        environment: str = "directory-publication",
    ) -> str:
        return launch_approval.validate(
            repo=self.repo, contract_path=CONTRACT, repository=repository,
            environment=environment, current_commit=current, marker_commit=marker,
        )

    def test_failed_sequence_one_then_sequence_two_is_blocked_without_marker(self) -> None:
        with self.assertRaises(launch_approval.InvalidLaunchApproval):
            self._validate(self.m2)

    def test_success_marker_allows_later_refresh_and_exact_rerun(self) -> None:
        git(self.repo, "tag", MARKER_REF.removeprefix("refs/tags/"), self.m1)
        self.assertEqual(self._validate(self.m2), self.m1)
        self.assertEqual(self._validate(self.m2), self.m1)

    def test_tampered_stale_and_cross_environment_markers_are_rejected(self) -> None:
        with self.subTest("tampered target"):
            with self.assertRaisesRegex(launch_approval.InvalidLaunchApproval, "materialization child"):
                self._validate(self.m2, self.q1)
        git(self.repo, "checkout", "-q", self.q1)
        stale_materialization = self._materialize("unapproved sibling")
        with self.subTest("stale sibling"):
            with self.assertRaises(launch_approval.InvalidLaunchApproval):
                self._validate(self.m2, stale_materialization)
        with self.subTest("cross repository"):
            with self.assertRaisesRegex(launch_approval.InvalidLaunchApproval, "repository differs"):
                self._validate(self.m2, self.m1, repository="attacker/fork")
        with self.subTest("cross environment"):
            with self.assertRaisesRegex(launch_approval.InvalidLaunchApproval, "environment differs"):
                self._validate(self.m2, self.m1, environment="staging")

    def test_workflow_has_no_higher_sequence_skip_around_marker_gate(self) -> None:
        workflow = yaml.load(
            (ROOT / ".github/workflows/directory-publication.yml").read_text(),
            Loader=yaml.BaseLoader,
        )
        jobs = workflow["jobs"]
        self.assertEqual(jobs["record_launch_approval"]["environment"], "directory-publication")
        self.assertIn("needs.sign.outputs.sequence == '1'", jobs["record_launch_approval"]["if"])
        self.assertIn("needs.sign.outputs.sequence > 1", jobs["gate_launch_approval"]["if"])
        marker_jobs = yaml.safe_dump({
            "record": jobs["record_launch_approval"],
            "gate": jobs["gate_launch_approval"],
        })
        marker_commands = "\n".join(
            step.get("run", "")
            for name in ("record_launch_approval", "gate_launch_approval")
            for step in jobs[name]["steps"]
        )
        self.assertNotIn("actions/download-artifact", marker_jobs)
        self.assertNotIn("pull_request", marker_jobs)
        self.assertIn("--force-with-lease=\"${marker_ref}:\"", marker_commands)
        self.assertEqual(
            set(jobs["deploy"]["needs"]),
            {"sign", "materialize_site", "gate_exact_staged_publication", "gate_launch_approval"},
        )
        self.assertIn("needs.gate_launch_approval.result == 'success'", jobs["deploy"]["if"])
        self.assertNotIn("required_stable_launch_evidence", jobs["deploy"]["if"])


if __name__ == "__main__":
    unittest.main()
