"""Synthetic archives and inert Python children; no publisher acquisition/execution."""
import gzip
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
import zipfile
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("vscode_research", Path(__file__).with_name("research-vscode-native.py"))
research = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(research)


def archive_bytes(entries, tail=b""):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w", format=tarfile.GNU_FORMAT) as archive:
        for name, kind, body in entries:
            entry = tarfile.TarInfo(name)
            entry.type = kind
            if kind in (tarfile.REGTYPE, tarfile.AREGTYPE):
                entry.size = len(body)
            if kind == tarfile.SYMTYPE:
                entry.linkname = body.decode()
            archive.addfile(entry, io.BytesIO(body) if entry.isreg() else None)
    return gzip.compress(output.getvalue() + tail)


def regular(name, body=b"inert"):
    return (research.PREFIX + "/" + name, tarfile.REGTYPE, body)


def extract(root, body):
    archive, destination = root / "archive.gz", root / "publisher"
    archive.write_bytes(body)
    destination.mkdir()
    return research.materialize_archive(archive, destination, research.bounds.Budget())


class VSCodeResearchTests(unittest.TestCase):
    def test_actual_guard_refuses_before_output_acquisition_or_native(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "never-created" / "research"
            env = {k: v for k, v in os.environ.items() if k not in (
                "GITHUB_ACTIONS", "RUNNER_ENVIRONMENT", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED")}
            result = subprocess.run([sys.executable, str(Path(research.__file__)), "--output", str(output)],
                                    env=env, capture_output=True, timeout=5, check=False)
            self.assertEqual(result.returncode, 2)
            self.assertIn(b"disposable GitHub-hosted", result.stderr)
            self.assertFalse(output.parent.exists())

    def test_exact_archive_digest_and_size_precede_materialization(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for body in (b"short", b"x" * 10, b"x" * 11):
                with patch.object(research, "ARCHIVE_BYTES", 10), \
                        patch.object(research.urllib.request, "build_opener") as factory:
                    factory.return_value.open.return_value = io.BytesIO(body)
                    with self.assertRaises(ValueError):
                        research.acquire_archive(root, research.bounds.Budget())
                self.assertFalse((root / "publisher").exists())
                (root / "publisher.tar.gz").unlink(missing_ok=True)
            body = b"measured inert archive"
            with patch.object(research, "ARCHIVE_BYTES", len(body)), \
                    patch.object(research, "ARCHIVE_SHA256", hashlib.sha256(body).hexdigest()), \
                    patch.object(research.urllib.request, "build_opener") as factory:
                factory.return_value.open.return_value = io.BytesIO(body)
                self.assertEqual(research.acquire_archive(root, research.bounds.Budget()).read_bytes(), body)

    def test_redirects_never_leave_exact_https_publisher(self):
        for url in ("http://vscode.download.prss.microsoft.com/x", "https://foreign.test/x",
                    "https://user:secret@vscode.download.prss.microsoft.com/x",
                    "https://vscode.download.prss.microsoft.com:444/x"):
            with self.subTest(url=url), self.assertRaises(ValueError):
                research.PublisherRedirects().redirect_request(None, None, 302, "", {}, url)
        redirects = research.PublisherRedirects()
        redirects.count = 3
        with self.assertRaises(ValueError):
            redirects.redirect_request(None, None, 302, "", {}, research.URL)

    def test_regular_files_and_omitted_links_have_no_alias_graph(self):
        entries = [regular("code", b"\x7fELFinert"),
                   (research.PREFIX + "/omitted", tarfile.SYMTYPE, b"../../foreign"),
                   regular("omitted/child", b"contained ordinary bytes")]
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            result = extract(root, archive_bytes(entries))
            self.assertEqual(result["omitted_symlinks"], 1)
            path = root / "publisher" / research.PREFIX / "omitted"
            self.assertTrue(path.is_dir())
            self.assertFalse(path.is_symlink())
            self.assertEqual((path / "child").read_bytes(), b"contained ordinary bytes")

    def test_archive_names_aliases_types_and_counts_are_refused(self):
        cases = [
            [("/" + research.PREFIX + "/code", tarfile.REGTYPE, b"x")],
            [(research.PREFIX + "/../code", tarfile.REGTYPE, b"x")],
            [(research.PREFIX + "/./code", tarfile.REGTYPE, b"x")],
            [(research.PREFIX + "/x\\code", tarfile.REGTYPE, b"x")],
            [(research.PREFIX + "/x:code", tarfile.REGTYPE, b"x")],
            [(research.PREFIX + "/control\n", tarfile.REGTYPE, b"x")],
            [regular("code"), regular("code")],
            [regular("Foo/one"), regular("foo/two")],
            [regular("parent"), regular("parent/child")],
            [regular("parent/child"), regular("parent")],
        ]
        for kind in (tarfile.LNKTYPE, tarfile.FIFOTYPE, tarfile.CHRTYPE, tarfile.GNUTYPE_SPARSE):
            cases.append([(research.PREFIX + "/code", kind, b"")])
        for entries in cases:
            with self.subTest(entries=entries), tempfile.TemporaryDirectory() as directory, self.assertRaises((ValueError, tarfile.TarError)):
                extract(Path(directory), archive_bytes(entries))
        for field, limit in (("MAX_MEMBERS", 1), ("MAX_FILE", 1), ("MAX_EXPANDED", 600), ("MAX_DEPTH", 1)):
            with self.subTest(limit=field), tempfile.TemporaryDirectory() as directory, \
                    patch.object(research, field, limit), self.assertRaises(ValueError):
                extract(Path(directory), archive_bytes([regular("one"), regular("two")]))

    def test_tar_metadata_crc_and_trailing_data_are_bounded(self):
        good = archive_bytes([regular("code")])
        damaged_crc = good[:-8] + bytes([good[-8] ^ 1]) + good[-7:]
        for body in (good[:-5], damaged_crc, archive_bytes([regular("code")], b"not padding")):
            with tempfile.TemporaryDirectory() as directory, self.assertRaises((ValueError, EOFError, OSError)):
                extract(Path(directory), body)
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode="w", format=tarfile.PAX_FORMAT) as archive:
            entry = tarfile.TarInfo(research.PREFIX + "/code")
            entry.size = 1
            entry.pax_headers = {"GNU.sparse.size": "1"}
            archive.addfile(entry, io.BytesIO(b"x"))
        with tempfile.TemporaryDirectory() as directory, self.assertRaisesRegex(ValueError, "PAX"):
            extract(Path(directory), gzip.compress(output.getvalue()))
        with tempfile.TemporaryDirectory() as directory, patch.object(research, "MAX_METADATA", 1), self.assertRaises(ValueError):
            extract(Path(directory), gzip.compress(output.getvalue()))
        for body in (b"999999999999 x=y\n", b"3 a", b"9 x=bad\n", b"12 path=one\n12 path=two\n"):
            with self.assertRaises(ValueError):
                research.pax_values(body)

    def test_descriptor_writes_refuse_existing_symlink(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / "archive.gz"
            archive.write_bytes(archive_bytes([regular("child/file")]))
            destination = root / "publisher"
            (destination / research.PREFIX).mkdir(parents=True)
            foreign = root / "foreign"
            foreign.mkdir()
            (destination / research.PREFIX / "child").symlink_to(foreign, target_is_directory=True)
            with self.assertRaises(OSError):
                research.materialize_archive(archive, destination, research.bounds.Budget())
            self.assertEqual(list(foreign.iterdir()), [])

    def test_required_files_are_regular_hashed_and_tamper_visible(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in research.REQUIRED:
                path = root / research.PREFIX / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(b"\x7fELFinert" if name == "code" else b"inert")
            before = research.required_files(root)
            path = root / research.PREFIX / research.REQUIRED[1]
            path.write_bytes(b"changed")
            self.assertNotEqual(before, research.required_files(root))
            path.unlink()
            path.symlink_to(root / research.PREFIX / "code")
            with self.assertRaises(ValueError):
                research.required_files(root)

    def test_finite_native_chain_has_same_process_isolation_and_clean_env(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            probe = root / "version"
            probe.mkdir()
            with patch.dict(os.environ, {"GH_TOKEN": "inert", "HTTP_PROXY": "foreign", "VSCODE_IPC_HOOK_CLI": "foreign"}):
                env = research.probe_environment(probe)
            self.assertEqual(env["ELECTRON_RUN_AS_NODE"], "1")
            self.assertNotIn("GH_TOKEN", env)
            self.assertNotIn("HTTP_PROXY", env)
            self.assertNotIn("VSCODE_IPC_HOOK_CLI", env)
            with patch.object(research.os, "getuid", return_value=1000), patch.object(research.os, "getgid", return_value=1001), \
                    patch.object(research.os, "geteuid", return_value=1000), patch.object(research.os, "getegid", return_value=1001):
                chain, argv, _ = research.isolated_command(root, "version", env, "--version", 12345)
            self.assertEqual(argv[:3], [str(root / "publisher" / research.PREFIX / "code"), str(root / "publisher" / research.PREFIX / research.REQUIRED[1]), "--version"])
            self.assertEqual(chain[:5], ["/usr/bin/sudo", "-n", "/usr/bin/unshare", "--net", "/usr/bin/setpriv"])
            self.assertIn("--clear-groups", chain)
            self.assertIn("--bounding-set=-all", chain)
            self.assertIn("--no-new-privs", chain)
            self.assertIn("/usr/bin/env", chain)
            self.assertIn("-i", chain)
            self.assertIn("os.execve", research.ISOLATION_VERIFIER)
            self.assertIn("socket.if_nameindex()", research.ISOLATION_VERIFIER)
            self.assertNotIn("--no-sandbox", argv)
            with patch.object(research.os, "getuid", return_value=0), self.assertRaises(ValueError):
                research.isolated_command(root, "version", env, "--version", 12345)

    def test_cleanup_is_anchored_no_follow_and_refuses_replaced_root(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            output, root, foreign = parent / "output", parent / "task", parent / "foreign"
            output.mkdir(); root.mkdir(); foreign.mkdir()
            (foreign / "keep").write_text("foreign")
            (root / "nested").mkdir()
            (root / "nested" / "inert").write_text("inert")
            (root / "alias").symlink_to(foreign, target_is_directory=True)
            identity = root.lstat()
            self.assertTrue(research.cleanup(root, identity, output)["complete"])
            self.assertFalse(root.exists())
            self.assertEqual((foreign / "keep").read_text(), "foreign")
            root.mkdir()
            identity = root.lstat()
            root.rename(parent / "original")
            root.mkdir()
            result = research.cleanup(root, identity, output)
            self.assertFalse(result["complete"])
            self.assertTrue(root.exists())
            self.assertTrue((parent / "original").exists())

    def test_output_admission_cannot_qualify_workspace_or_truncation(self):
        good = {"error": None, "exit": 0}
        self.assertTrue(research.validate_probe(good, (research.VERSION + "\n" + research.COMMIT).encode(), b"", "version"))
        self.assertFalse(research.validate_probe(good, (research.VERSION + "0\n" + research.COMMIT).encode(), b"", "version"))
        self.assertFalse(research.validate_probe({"error": "overflow", "exit": 0}, b"help", b"", "help"))
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)
            (output / "inert").write_bytes(b"x" * 9)
            with patch.object(research, "MAX_ARTIFACT", 8), self.assertRaises(ValueError):
                research.artifact_bound(output)
        source = Path(research.__file__).read_text()
        self.assertIn('"native_project_qualified": False', source)
        workflow = Path(research.__file__).parent.parent / ".github/workflows/vscode-native-research.yml"
        text = workflow.read_text()
        self.assertIn("persist-credentials: false", text)
        self.assertIn("timeout-minutes: 10", text)
        self.assertIn("github.event.pull_request.head.sha || github.sha", text)
        self.assertNotIn("pull_request_target", text)
        self.assertIn("if: ${{ always() }}", text)
        self.assertIn("vscode-native-research/upload/research.zip", text)
        self.assertNotIn("path: ${{ runner.temp }}/vscode-native-research/\n", text)
        self.assertIn("steps.upload_admission.outputs.valid", text)

    def test_only_validated_bounded_envelope_is_uploaded_even_on_admission_failure(self):
        for condition in ("valid", "overflow", "symlink", "directory"):
            with self.subTest(condition=condition), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                raw = root / "raw"
                raw.mkdir()
                (raw / "version.stdout.log").write_bytes(b"observed finite output")
                if condition == "overflow":
                    (raw / "large.log").write_bytes(b"x" * 100)
                elif condition == "symlink":
                    foreign = root / "foreign"
                    foreign.write_text("must never upload foreign data")
                    (raw / "alias").symlink_to(foreign)
                elif condition == "directory":
                    (raw / "unexpected").mkdir()
                limit = (64 << 10) + 64 if condition == "overflow" else research.MAX_ARTIFACT
                with patch.object(research, "MAX_ARTIFACT", limit):
                    accepted = research.publish_artifacts(raw, {"source_commit": "inert", "error": "original failure"})
                self.assertEqual(accepted, condition == "valid")
                upload = raw / "upload"
                self.assertEqual([path.name for path in upload.iterdir()], ["research.zip"])
                envelope = upload / "research.zip"
                self.assertLessEqual(envelope.stat().st_size, limit)
                with zipfile.ZipFile(envelope) as archive:
                    if accepted:
                        self.assertEqual(archive.namelist(), ["version.stdout.log"])
                        self.assertEqual(archive.read("version.stdout.log"), b"observed finite output")
                    else:
                        self.assertEqual(archive.namelist(), ["collection-failure.json"])
                        failure = json.loads(archive.read("collection-failure.json"))
                        self.assertFalse(failure["collection_complete"])
                        self.assertFalse(failure["native_project_qualified"])
                        self.assertEqual(failure["collection_error"], "original failure")

    def test_root_group_or_effective_identity_mismatch_refuses_before_exec(self):
        for uid, gid, euid, egid in ((1000, 0, 1000, 0), (1000, 1001, 0, 1001), (1000, 1001, 1000, 0)):
            with self.subTest(ids=(uid, gid, euid, egid)), patch.object(research.os, "getuid", return_value=uid), \
                    patch.object(research.os, "getgid", return_value=gid), patch.object(research.os, "geteuid", return_value=euid), \
                    patch.object(research.os, "getegid", return_value=egid), self.assertRaises(ValueError):
                research.isolated_command(Path("/inert"), "version", {}, "--version", 12345)
        for required in ('observed["gid"]==0', 'observed["euid"]!=int(uid)', 'observed["egid"]!=int(gid)'):
            self.assertIn(required, research.ISOLATION_VERIFIER)


if __name__ == "__main__":
    unittest.main()
