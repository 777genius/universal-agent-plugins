"""Inert admission tests. No vendor acquisition or native process."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

SPEC = importlib.util.spec_from_file_location("project_artifacts", Path(__file__).with_name("native_client_project_artifacts.py"))
artifacts = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(artifacts)


class ProjectEnvelopeTests(unittest.TestCase):
    def fixture(self, root):
        source = root / "source"
        source.mkdir()
        body = b'{"inert":true}'
        (source / "one.stdout.log").write_bytes(body)
        (source / "one.stderr.log").write_bytes(b"")
        command = {"label": "one"}
        for stream, value in (("stdout", body), ("stderr", b"")):
            command.update({stream + "_file": "one." + stream + ".log", stream + "_bytes": len(value),
                            stream + "_sha256": hashlib.sha256(value).hexdigest()})
        (source / "opencode-project-evidence.json").write_text(json.dumps({"success": True, "commands": [command]}))
        return source

    def test_actual_archive_only_flat_admitted_bytes(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            source = self.fixture(root)
            destination = root / "upload.zip"
            admitted = artifacts.write_archive(source, destination)
            if not admitted:
                with zipfile.ZipFile(destination) as archive:
                    self.fail(archive.read("opencode-project-evidence.json").decode("utf-8"))
            with zipfile.ZipFile(destination) as archive:
                self.assertEqual(set(archive.namelist()), {p.name for p in source.iterdir()})
                for member in archive.infolist():
                    self.assertEqual(member.compress_type, zipfile.ZIP_STORED)
                    self.assertEqual(archive.read(member), (source / member.filename).read_bytes())
            self.assertLess(destination.stat().st_size, artifacts.LIMIT)

    def test_refused_source_yields_only_false_failure_record(self):
        for kind in ("directory", "symlink", "hardlink", "oversize", "added-name", "added-log", "changed-stream"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temp:
                root = Path(temp).resolve()
                source = self.fixture(root)
                if kind == "directory": (source / "directory.json").mkdir()
                elif kind == "symlink":
                    try: (source / "alias.log").symlink_to(source / "one.stdout.log")
                    except OSError: continue
                elif kind == "hardlink": os.link(source / "one.stdout.log", source / "linked.log")
                elif kind == "oversize": (source / "large.log").write_bytes(b"x" * ((1 << 20) + 1))
                elif kind == "added-name": (source / "not-evidence.bin").write_bytes(b"refused")
                elif kind == "added-log": (source / "unrecorded.log").write_bytes(b"refused")
                else: (source / "one.stdout.log").write_bytes(b"changed")
                destination = root / "upload.zip"
                self.assertFalse(artifacts.write_archive(source, destination))
                with zipfile.ZipFile(destination) as archive:
                    self.assertEqual(archive.namelist(), ["opencode-project-evidence.json"])
                    summary = json.loads(archive.read(archive.namelist()[0]))
                    self.assertFalse(summary["success"])
                    self.assertFalse(summary["native_discovery_qualified"])

    def test_live_count_and_aggregate_refusal(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            source = self.fixture(root)
            for i in range(129): (source / (str(i) + ".log")).write_bytes(b"")
            with self.assertRaises(ValueError): artifacts.read_envelope(source)
        with tempfile.TemporaryDirectory() as temp:
            source = self.fixture(Path(temp).resolve())
            with patch.object(artifacts, "LIMIT", 65536):
                with self.assertRaises(ValueError): artifacts.read_envelope(source)

    def test_deadline_and_replacement_refuse(self):
        with tempfile.TemporaryDirectory() as temp:
            source = self.fixture(Path(temp).resolve())
            with patch.object(artifacts.time, "monotonic", side_effect=[0, 5]):
                with self.assertRaises(TimeoutError): artifacts.read_envelope(source)
            real = artifacts.os.fstat
            def replaced(fd):
                info = real(fd)
                return os.stat_result((info.st_mode, info.st_ino + 1, info.st_dev, info.st_nlink,
                    info.st_uid, info.st_gid, info.st_size, info.st_atime, info.st_mtime, info.st_ctime))
            with patch.object(artifacts.os, "fstat", side_effect=replaced):
                with self.assertRaises(ValueError): artifacts.read_envelope(source)


if __name__ == "__main__":
    unittest.main()

class ProjectMatrixAdmissionTests(unittest.TestCase):
    def test_child_timeout_cannot_leave_success_archive_or_follow_fallback_link(self):
        spec = importlib.util.spec_from_file_location("project_matrix", Path(__file__).with_name("run-native-client-matrix.py"))
        matrix = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(matrix)
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            source = root / "uap-opencode-project-admitted-test"
            source.mkdir()
            output = root / "output"
            output.mkdir()
            foreign = root / "foreign.txt"
            foreign.write_bytes(b"foreign bytes")
            destination = output / (source.name + ".zip")
            try:
                destination.symlink_to(foreign)
            except OSError:
                destination.write_bytes(b"incomplete archive")
            with patch.object(matrix.subprocess, "run", side_effect=matrix.subprocess.TimeoutExpired("inert", 5)):
                error = matrix.collect_opencode_project_artifact(source, output)
            self.assertIn("TimeoutExpired", error)
            self.assertEqual(foreign.read_bytes(), b"foreign bytes")
            self.assertFalse(destination.is_symlink())
            with zipfile.ZipFile(destination) as archive:
                self.assertEqual(archive.namelist(), ["opencode-project-evidence.json"])
                self.assertFalse(json.loads(archive.read(archive.namelist()[0]))["success"])
