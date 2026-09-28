"""Offline promotion authentication tests; no release or runtime operations."""
import argparse
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

SPEC = importlib.util.spec_from_file_location(
    "reuse", Path(__file__).with_name("reuse-agentplugins-verified-draft.py"))
reuse = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(reuse)


class ReuseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.args = argparse.Namespace(repository=reuse.REPOSITORY, tag="agentplugins-v1.2.3",
                                       commit="a" * 40, run_id=42, run_attempt=2,
                                       source=str(Path(self.temp.name).resolve()))
        self.release = {"id": 123, "tag": self.args.tag, "commit": self.args.commit,
                        "draft": True, "prerelease": False, "updated_at": "2026-09-28T00:00:00Z",
                        "assets": [{"id": n, "name": f"asset-{n}", "sha256": "b" * 64}
                                   for n in range(1, 10)]}
        self.receipt = {"schema_version": 1, "release_state": "verified-draft",
                        "repository": reuse.REPOSITORY, "producer_commit": self.args.commit,
                        "producer_run_id": 42, "producer_run_attempt": 2,
                        "signer_workflow": f"github.com/{reuse.REPOSITORY}/{reuse.WORKFLOW}",
                        "release": self.release, "asset_set_digest": "c" * 64,
                        "subjects": [{"name": f"asset-{n}", "sha256": "b" * 64}
                                     for n in range(1, 10)]}
        self.run = {"id": 42, "run_attempt": 2, "repository": {"full_name": reuse.REPOSITORY},
                    "head_repository": {"full_name": reuse.REPOSITORY},
                    "head_sha": self.args.commit, "head_branch": "main", "path": reuse.WORKFLOW,
                    "workflow_id": 7, "event": "workflow_dispatch", "status": "completed",
                    "conclusion": "success", "run_started_at": "2026-09-28T01:00:00Z",
                    "updated_at": "2026-09-28T03:00:00Z"}
        names = ("validate", "build (darwin, amd64)", "build (darwin, arm64)",
                 "build (linux, amd64)", "build (linux, arm64)",
                 "build (windows, amd64, .exe)", "build (windows, arm64, .exe)",
                 "stage-draft", "platform-proof / Verify release and pack exact npm tarball",
                 *(f"platform-proof / native runtime E2E ({target})" for target in reuse.TARGETS),
                 "platform-proof / Aggregate all six native platform proofs", "verified-draft")
        self.jobs = [{"name": name, "run_id": 42, "status": "completed",
                      "conclusion": "success", "steps": [{"conclusion": "success"}]}
                     for name in names]
        self.jobs.append({"name": "promote-release", "run_id": 42,
                          "status": "completed", "conclusion": "skipped", "steps": []})
        self.current_main = self.args.commit
        self.set_zip()

    def set_zip(self):
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, "w") as archive:
            archive.writestr("verified-draft.json", json.dumps(self.receipt))
        self.body = buffer.getvalue()
        self.artifact = {"id": 70, "name": "agentplugins-verified-draft-42-2",
                         "expired": False, "size_in_bytes": len(self.body),
                         "digest": "sha256:" + hashlib.sha256(self.body).hexdigest(),
                         "workflow_run": {"id": 42, "head_sha": self.args.commit},
                         "created_at": "2026-09-28T02:00:00Z"}

    def api(self, endpoint, *options):
        suffix = endpoint.removeprefix(f"repos/{reuse.REPOSITORY}/actions/")
        if suffix == "runs/42":
            return self.run
        if suffix == "workflows/7":
            return {"id": 7, "path": reuse.WORKFLOW}
        if suffix == "runs/42/attempts/2/jobs?per_page=100":
            self.assertEqual(options, ("--paginate", "--slurp"))
            return [{"jobs": self.jobs}]
        if suffix == "runs/42/artifacts?per_page=100":
            self.assertEqual(options, ("--paginate", "--slurp"))
            return [{"artifacts": [self.artifact]}]
        self.fail(f"unexpected API read: {endpoint}")

    def output(self, command):
        if command[:2] == ["gh", "api"]:
            self.assertEqual(command[2], f"repos/{reuse.REPOSITORY}/actions/artifacts/70/zip")
            return self.body
        if command[:3] == ["git", "-C", self.args.source]:
            if command[3] == "fetch":
                return b""
            if command[3] == "rev-parse":
                return (self.current_main if command[4] == "refs/remotes/origin/main"
                        else self.args.commit).encode() + b"\n"
            if command[3] == "rev-list":
                return (self.args.commit + "\n").encode()
            if command[3] == "status":
                return b""
        self.fail(f"unexpected command: {command}")

    def verify(self):
        with patch.object(reuse, "api", side_effect=self.api), \
             patch.object(reuse, "output", side_effect=self.output), \
             patch.object(reuse.draft, "verify", return_value=self.receipt) as live:
            result = reuse.verify(self.args)
        self.assertEqual(result, self.receipt)
        live.assert_called_once()

    def reject(self):
        with patch.object(reuse, "api", side_effect=self.api), \
             patch.object(reuse, "output", side_effect=self.output), \
             patch.object(reuse.draft, "verify", return_value=self.receipt):
            with self.assertRaises((ValueError, KeyError, TypeError, zipfile.BadZipFile)):
                reuse.verify(self.args)

    def test_exact_qualified_draft_reused(self):
        self.verify()

    def test_run_and_workflow_mismatch(self):
        for key, bad in (("head_sha", "b" * 40), ("head_branch", "feature"),
                         ("path", ".github/workflows/other.yml"),
                         ("run_attempt", 3), ("conclusion", "failure")):
            with self.subTest(key=key):
                original = self.run[key]
                self.run[key] = bad
                self.reject()
                self.run[key] = original

    def test_incomplete_six_platform_proofs_and_promotion_not_skipped(self):
        original = copy.deepcopy(self.jobs)
        self.jobs.pop(-2)
        self.reject()
        self.jobs = copy.deepcopy(original)
        self.jobs[-1]["conclusion"] = "success"
        self.reject()

    def test_receipt_artifact_and_identity_tampering(self):
        original = self.artifact["digest"]
        self.artifact["digest"] = "sha256:" + "f" * 64
        self.reject()
        self.artifact["digest"] = original
        self.receipt["producer_run_attempt"] = 1
        self.set_zip()
        self.reject()

    def test_live_draft_mutation(self):
        changed = copy.deepcopy(self.receipt)
        changed["release"]["assets"][0]["id"] = 999
        with patch.object(reuse, "api", side_effect=self.api), \
             patch.object(reuse, "output", side_effect=self.output), \
             patch.object(reuse.draft, "verify", return_value=changed):
            with self.assertRaisesRegex(ValueError, "draft changed since qualification"):
                reuse.verify(self.args)

    def test_current_main_moved(self):
        self.current_main = "d" * 40
        self.reject()


if __name__ == "__main__":
    unittest.main()
