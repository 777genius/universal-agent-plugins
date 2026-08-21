from __future__ import annotations

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
import directory_publication_cas as cas


GIT = "/usr/bin/git"
TAG_ONE = "refs/tags/directory-publication-schema-1-sequence-00000000000000000001"


def git(repo: Path, *args: str, check: bool = True) -> str:
    completed = subprocess.run(
        [GIT, "-C", str(repo), *args], check=check, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    return completed.stdout.strip()


class BarePublicationCasTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        root = Path(self.temporary.name)
        self.remote = root / "remote.git"
        self.publisher = root / "publisher"
        git(root, "init", "--bare", "-q", str(self.remote))
        git(root, "init", "-q", str(self.publisher))
        git(self.publisher, "config", "user.name", "test")
        git(self.publisher, "config", "user.email", "test@example.com")
        (self.publisher / "source.txt").write_text("reviewed source\n")
        git(self.publisher, "add", "source.txt")
        git(self.publisher, "commit", "-qm", "source")
        self.source = git(self.publisher, "rev-parse", "HEAD")
        git(self.publisher, "remote", "add", "origin", str(self.remote))
        git(self.publisher, "push", "-q", "origin", f"{self.source}:refs/heads/main")
        git(self.publisher, "push", "-q", "origin", f"{self.source}:refs/heads/directory-publication-ledger")

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def commit_object(self, parent: str, message: str) -> str:
        tree = git(self.publisher, "show", "-s", "--format=%T", parent)
        return git(self.publisher, "commit-tree", tree, "-p", parent, "-m", message)

    def objects(self, publication_id: str = "run-1", message: str = "ledger Q") -> tuple[str, str]:
        marker = cas.create_marker(self.publisher, self.source, publication_id)
        ledger = self.commit_object(self.source, message)
        return marker, ledger

    def state(self, tag: str = TAG_ONE) -> cas.RefState:
        return cas.read_ref_state(
            self.publisher, "origin", "refs/heads/main",
            "refs/heads/directory-publication-ledger", tag,
        )

    def publish(self, marker: str, ledger: str, tag: str = TAG_ONE,
                push_runner=None) -> str:
        return cas.atomic_transition(
            self.publisher, "origin", source=self.source, marker=marker,
            ledger_old=self.source, ledger_new=ledger, sequence_tag=tag,
            push_runner=push_runner,
        )

    def test_marker_is_deterministic_single_parent_same_tree_and_different_oid(self) -> None:
        first = cas.create_marker(self.publisher, self.source, "run-123")
        second = cas.create_marker(self.publisher, self.source, "run-123")
        self.assertEqual(first, second)
        self.assertNotEqual(first, self.source)
        self.assertEqual(git(self.publisher, "show", "-s", "--format=%P", first), self.source)
        self.assertEqual(
            git(self.publisher, "show", "-s", "--format=%T", first),
            git(self.publisher, "show", "-s", "--format=%T", self.source),
        )
        self.assertEqual(git(self.publisher, "diff", "--name-only", self.source, first), "")

    def test_successful_sequence_one_atomic_transition(self) -> None:
        marker, ledger = self.objects()
        self.assertEqual(self.publish(marker, ledger), "published")
        self.assertEqual(self.state(), cas.RefState(marker, ledger, ledger))

    def test_each_conflicting_ref_leaves_all_other_refs_unchanged(self) -> None:
        for conflict in ("main", "ledger", "tag"):
            with self.subTest(conflict=conflict):
                self.tearDown()
                self.setUp()
                marker, ledger = self.objects()
                competing = self.commit_object(self.source, f"competing {conflict}")
                target = {
                    "main": "refs/heads/main",
                    "ledger": "refs/heads/directory-publication-ledger",
                    "tag": TAG_ONE,
                }[conflict]
                git(self.publisher, "push", "-q", "origin", f"{competing}:{target}")
                before = self.state()
                with self.assertRaisesRegex(cas.CasError, "conflict"):
                    self.publish(marker, ledger)
                self.assertEqual(self.state(), before)

    def test_two_publishers_have_exactly_one_success(self) -> None:
        marker_one, ledger_one = self.objects("run-1", "ledger one")
        marker_two, ledger_two = self.objects("run-2", "ledger two")
        outcomes = []
        for marker, ledger in ((marker_one, ledger_one), (marker_two, ledger_two)):
            try:
                outcomes.append(self.publish(marker, ledger))
            except cas.CasError:
                outcomes.append("conflict")
        self.assertEqual(outcomes, ["published", "conflict"])
        self.assertEqual(self.state(), cas.RefState(marker_one, ledger_one, ledger_one))

    def test_response_loss_exact_readback_and_rerun(self) -> None:
        marker, ledger = self.objects()

        def lose_response(arguments):
            git(self.publisher, *arguments)
            return False

        self.assertEqual(self.publish(marker, ledger, push_runner=lose_response), "published")
        self.assertEqual(self.publish(marker, ledger), "committed")
        self.assertEqual(self.state(), cas.RefState(marker, ledger, ledger))

    def test_exact_rerun_authenticates_materialized_descendant_and_reuses_it(self) -> None:
        marker, signed = self.objects()
        self.publish(marker, signed)
        materialized = self.commit_object(
            signed, "chore(directory): materialize signed production site"
        )
        git(self.publisher, "push", "-q", "origin", f"{materialized}:refs/heads/directory-publication-ledger")
        output = Path(self.temporary.name) / "materialized.commit"
        push_attempts = []
        self.assertEqual(
            cas.atomic_transition(
                self.publisher, "origin", source=self.source, marker=marker,
                ledger_old=self.source, ledger_new=signed, sequence_tag=TAG_ONE,
                materialized_output=output,
                push_runner=lambda arguments: push_attempts.append(arguments) is None,
            ),
            "materialized",
        )
        self.assertEqual(push_attempts, [])
        self.assertEqual(output.read_text(), materialized + "\n")
        self.assertEqual(self.state(), cas.RefState(marker, materialized, signed))

    def test_exact_rerun_rejects_unrelated_or_hostile_ledger_descendants(self) -> None:
        for message, second_child in (("unrelated", False), ("chore(directory): materialize signed production site", True)):
            with self.subTest(message=message, second_child=second_child):
                self.tearDown()
                self.setUp()
                marker, signed = self.objects()
                self.publish(marker, signed)
                moved = self.commit_object(signed, message)
                if second_child:
                    moved = self.commit_object(moved, message)
                git(self.publisher, "push", "-q", "origin", f"{moved}:refs/heads/directory-publication-ledger")
                with self.assertRaisesRegex(cas.CasError, "materialized ledger"):
                    self.publish(marker, signed)

        self.tearDown()
        self.setUp()
        marker, signed = self.objects()
        self.publish(marker, signed)
        (self.publisher / "registry").mkdir()
        (self.publisher / "registry" / "hostile.json").write_text("{}\n")
        git(self.publisher, "add", "registry/hostile.json")
        hostile_tree = git(self.publisher, "write-tree")
        hostile = git(
            self.publisher, "commit-tree", hostile_tree, "-p", signed, "-m",
            "chore(directory): materialize signed production site",
        )
        git(self.publisher, "push", "-q", "origin", f"{hostile}:refs/heads/directory-publication-ledger")
        with self.assertRaisesRegex(cas.CasError, "changed signed registry bytes"):
            self.publish(marker, signed)

    def test_materialization_exact_lease_conflict_and_idempotent_readback(self) -> None:
        marker, ledger = self.objects()
        self.publish(marker, ledger)
        materialized = self.commit_object(ledger, "materialized site")
        competing = self.commit_object(ledger, "competing materialization")
        git(self.publisher, "push", "-q", "origin", f"{competing}:refs/heads/directory-publication-ledger")
        with self.assertRaisesRegex(cas.CasError, "materialization ledger conflict"):
            cas.materialize_transition(
                self.publisher, "origin", ledger_old=ledger,
                ledger_new=materialized,
            )
        self.assertEqual(self.state().ledger, competing)

        git(self.publisher, "push", "-q", "--force", "origin", f"{ledger}:refs/heads/directory-publication-ledger")
        self.assertEqual(cas.materialize_transition(
            self.publisher, "origin", ledger_old=ledger, ledger_new=materialized,
        ), "published")
        self.assertEqual(cas.materialize_transition(
            self.publisher, "origin", ledger_old=ledger, ledger_new=materialized,
        ), "committed")


class MarkerBindingContractTests(unittest.TestCase):
    def test_candidate_release_site_and_materialization_bind_to_marker(self) -> None:
        workflow_path = ROOT / ".github" / "workflows" / "directory-publication.yml"
        workflow_text = workflow_path.read_text()
        workflow = yaml.load(workflow_text, Loader=yaml.BaseLoader)
        candidate = next(step for step in workflow["jobs"]["prepare"]["steps"] if step.get("id") == "candidate")
        self.assertEqual(candidate["env"]["SOURCE_COMMIT"], "${{ steps.marker.outputs.commit }}")
        self.assertIn('--source-tree-commit "${SOURCE_TREE_COMMIT}"', candidate["run"])
        build_artifact = next(step for step in workflow["jobs"]["build_site"]["steps"] if step.get("id") == "artifact")
        self.assertEqual(build_artifact["env"]["SOURCE_COMMIT"], "${{ needs.sign.outputs.marker_commit }}")
        materialize_verify = next(step for step in workflow["jobs"]["materialize_site"]["steps"] if step.get("name") == "Reject unsafe archive entries and verify the artifact")
        self.assertEqual(materialize_verify["env"]["EXPECTED_SOURCE_COMMIT"], "${{ needs.sign.outputs.marker_commit }}")
        self.assertIn("ref: ${{ needs.sign.outputs.marker_commit }}", workflow_text)
        preparer = (ROOT / "scripts" / "prepare_directory_publication.py").read_text()
        self.assertIn('package_source["revision"] = source_commit', preparer)


if __name__ == "__main__":
    unittest.main()
