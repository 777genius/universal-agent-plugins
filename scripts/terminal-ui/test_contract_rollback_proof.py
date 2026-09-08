"""Regression checks for immutable rollback inputs and missing contracts."""
import importlib.util
import io
import json
from pathlib import Path
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location(
    "rollback", Path(__file__).with_name("contract_rollback_proof.py"))
rollback = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rollback)


class CandidateTests(unittest.TestCase):
    def test_default_pins_head_before_archive(self):
        sha = "a" * 40
        with patch.object(rollback.subprocess, "check_output", side_effect=[sha + "\n", b"archive"]) as git:
            self.assertEqual(rollback.candidate_archive(Path("/source"), None), (sha, b"archive"))
        self.assertEqual(git.call_args_list[0].args[0],
                         ["git", "rev-parse", "--verify", "HEAD^{commit}"])
        self.assertEqual(git.call_args_list[1].args[0],
                         ["git", "archive", sha, "cli", "sdk", "install", ".github", "scripts"])

    def test_explicit_sha_and_mutable_ref_rejection(self):
        sha = "b" * 40
        with patch.object(rollback.subprocess, "check_output", side_effect=[sha + "\n", b"archive"]) as git:
            self.assertEqual(rollback.candidate_archive(Path("/source"), sha), (sha, b"archive"))
            self.assertEqual(git.call_args_list[0].args[0][-1], sha + "^{commit}")
        for value in ("HEAD", "main", "af6b7a2a", "--all"):
            with self.subTest(value=value), patch.object(rollback.subprocess, "check_output") as git:
                with self.assertRaises(ValueError):
                    rollback.candidate_archive(Path("/source"), value)
                git.assert_not_called()

    def test_missing_archived_contract_cannot_use_worktree_copy(self):
        # The real source has a contract; this candidate deliberately does not.
        data = io.BytesIO()
        with tarfile.open(fileobj=data, mode="w") as archive:
            for name in ("internal/terminalprompts/mode.go", "go.mod", "go.sum"):
                entry = tarfile.TarInfo("cli/plugin-kit-ai/" + name)
                archive.addfile(entry, io.BytesIO())
        with tempfile.TemporaryDirectory() as directory:
            with patch.object(sys, "argv", ["proof", "--artifacts", directory]), \
                 patch.object(rollback, "candidate_archive", return_value=("c" * 40, data.getvalue())), \
                 patch.object(rollback.subprocess, "run") as run:
                with self.assertRaises(FileNotFoundError):
                    rollback.main()
                run.assert_not_called()
            manifest = json.loads((Path(directory) / "rollback.json").read_text())
            self.assertEqual(manifest["status"], "failed")
            self.assertEqual(manifest["candidate_sha"], "c" * 40)
            self.assertIn("adapter_success_contract_test.go", manifest["error"])


if __name__ == "__main__":
    unittest.main()
