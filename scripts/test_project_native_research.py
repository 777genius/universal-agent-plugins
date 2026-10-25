"""Inert admission/phase tests; no downloaded or substituted vendor executable."""
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import tempfile
import sys
import tarfile
import unittest
from unittest.mock import patch
import zipfile

import project_native_research as research


def tar_bytes(members):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w:gz") as archive:
        for name, data, kind in members:
            info = tarfile.TarInfo(name)
            info.type = kind
            info.size = len(data) if kind == tarfile.REGTYPE else 0
            info.linkname = "outside" if kind == tarfile.SYMTYPE else ""
            archive.addfile(info, io.BytesIO(data))
    return output.getvalue()


class ProjectResearchTests(unittest.TestCase):
    def test_archive_traversal_duplicates_and_expansion_are_rejected(self):
        spec = {"format": "tar.gz", "max_expanded_bytes": 32}
        for members in ([("../escape", b"x", tarfile.REGTYPE)],
                        [("/absolute", b"x", tarfile.REGTYPE)],
                        [("x", b"a", tarfile.REGTYPE), ("./x", b"b", tarfile.REGTYPE)],
                        [("large", b"x" * 33, tarfile.REGTYPE)]):
            with self.subTest(members=members):
                with self.assertRaises(ValueError):
                    research.inventory_archive(tar_bytes(members), spec, research.Budget())
        with self.assertRaisesRegex(ValueError, "byte limit"):
            research.inventory_archive(gzip.compress(b"x" * 33),
                                       {"format": "gzip", "entrypoint": "grok", "max_expanded_bytes": 32}, research.Budget())

    def test_inventory_never_materializes_links_and_execution_needs_pin(self):
        spec = {"format": "tar.gz", "entrypoint": "bin/client"}
        members, contents = research.inventory_archive(tar_bytes([
            ("bin/client", b"inert bytes", tarfile.REGTYPE),
            ("profile", b"", tarfile.SYMTYPE)]), spec, research.Budget())
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with self.assertRaisesRegex(ValueError, "integrity"):
                research.materialize(root / "one", spec, {"pin_verified": False, "members": members}, contents)
            with self.assertRaisesRegex(ValueError, "links"):
                research.materialize(root / "two", spec, {"pin_verified": True, "members": members}, contents)
            self.assertEqual(list(root.iterdir()), [])
            path, identities = research.materialize(root / "node", spec, {"pin_verified": True, "members": members}, contents, selected_only=True)
            self.assertEqual(path.read_bytes(), b"inert bytes")
            research.verify_files(identities, research.Budget())
            path.write_bytes(b"mutated")
            with self.assertRaisesRegex(ValueError, "bytes changed"):
                research.verify_files(identities, research.Budget())

    def test_zip_symlink_and_unsafe_name_are_not_executable_files(self):
        output = io.BytesIO()
        with zipfile.ZipFile(output, "w") as archive:
            archive.writestr("client", b"inert")
            link = zipfile.ZipInfo("linked")
            link.external_attr = 0o120777 << 16
            archive.writestr(link, b"client")
        members, contents = research.inventory_archive(output.getvalue(), {"format": "zip"}, research.Budget())
        self.assertEqual(contents, {"client": b"inert"})
        self.assertEqual(members[1]["kind"], "link")
        for name in ("a\\b", "C:/x", "a//b", "a/../b"):
            with self.assertRaises(ValueError):
                research.member_name(name)

    def test_help_or_missing_skipped_cases_never_qualify_scope(self):
        cases = [{"id": "kimi.acquisition", "status": "passed", "reason": "bytes"},
                 {"id": "kimi.help", "status": "passed", "reason": "help"}]
        research.validate_cases("kimi", "help", cases)
        with self.assertRaises(ValueError):
            research.validate_cases("kimi", "observe", cases)
        for status in ("skipped", "observed_boundary", "failed", "passed-by-default"):
            cases[1]["status"] = status
            with self.assertRaises(ValueError):
                research.validate_cases("kimi", "help", cases)
        cases[1] = dict(cases[0])
        with self.assertRaises(ValueError):
            research.validate_cases("kimi", "help", cases)

    def test_private_token_redacted_and_canary_rejected_in_all_evidence(self):
        secret = b"native-private-token-canary"
        stdout, stderr = b"token=" + secret, b""
        record = {"probe": "help", "argv": ["inert"], "cwd": "/owned", "exit": 0, "error": None}
        captures = {}
        for stream, body in (("stdout", stdout), ("stderr", stderr)):
            name = "help." + stream + ".log"
            captures[name] = body
            record.update({stream + "_file": name, stream + "_bytes": len(body),
                           stream + "_sha256": hashlib.sha256(body).hexdigest()})
        summary = {"commands": [record], "error": secret.decode()}
        admitted = research.evidence(summary, captures, [secret])
        self.assertTrue(all(secret not in body for body in admitted.values()))
        self.assertEqual(admitted["help.stdout.log"], b"token=[redacted]")
        for name, body in (("research.json", json.dumps({"token": secret.decode()}).encode()),
                           ("transcript.log", secret), (secret.decode() + ".log", b"innocent")):
            with self.subTest(name=name):
                with self.assertRaisesRegex(ValueError, "private token"):
                    research.admit_bodies({"research.json": b"{}", name: body}, [secret])
        captures["unrecorded.log"] = b"unexpected"
        with self.assertRaisesRegex(ValueError, "extra private"):
            research.evidence(summary, captures, [secret])

    def test_native_entrypoint_refuses_local_host_before_creating_files(self):
        with tempfile.TemporaryDirectory() as temporary, patch.dict("os.environ", {}, clear=True):
            output = Path(temporary) / "not-created"
            with self.assertRaisesRegex(RuntimeError, "GitHub-hosted"):
                research.run("kimi", "help", output)
            self.assertFalse(output.exists())

    def test_oversized_evidence_retains_only_closed_failure_receipt(self):
        summary = {"commands": [], "collection_complete": True, "observations": "x" * (research.STREAM + 1)}
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            research.publish(summary, None, root)
            with zipfile.ZipFile(root / "upload/research.zip") as archive:
                self.assertEqual(archive.namelist(), ["research.json"])
                body = json.loads(archive.read("research.json"))
                self.assertIs(body["collection_complete"], False)
                self.assertIs(body["artifact_admission_failed"], True)
                self.assertTrue(all(value is False for value in body["qualification"].values()))
            self.assertIs(summary["collection_complete"], False)

    def test_private_capture_never_opens_or_reads_native_writable_log_paths(self):
        # An inert Python child exercises the real selector/capture, not a vendor.
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            foreign = root / "foreign"
            foreign.write_bytes(b"unchanged foreign authority")
            (root / "capture.stdout.log").symlink_to(foreign)
            (root / "capture.stderr.log").write_bytes(b"x" * 4096)
            child = "import pathlib,sys; pathlib.Path('capture.stderr.log').unlink(); pathlib.Path('capture.stderr.log').symlink_to('foreign'); sys.stdout.write('exact observed stdout'); sys.stderr.write('e'*65)"
            record, stdout, stderr = research.run_probe(
                [sys.executable, "-c", child], root, {}, root, "capture", timeout=5, limit=64, memory_only=True)
            self.assertEqual(stdout, b"exact observed stdout")
            self.assertEqual(stderr, b"e" * 64)
            self.assertEqual(record["error"], "output limit reached")
            self.assertEqual(foreign.read_bytes(), b"unchanged foreign authority")
            self.assertTrue((root / "capture.stdout.log").is_symlink())
            self.assertTrue((root / "capture.stderr.log").is_symlink())


if __name__ == "__main__":
    unittest.main()
