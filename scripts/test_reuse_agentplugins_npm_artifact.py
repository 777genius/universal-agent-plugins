"""Offline exact-tarball custody tests, using real ZIP bytes and hashes."""
import argparse
import base64
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import stat
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import warnings
import zipfile

SPEC = importlib.util.spec_from_file_location(
    "npm_reuse", Path(__file__).with_name("reuse-agentplugins-npm-artifact.py"))
npm = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(npm)


class NpmArtifactTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.out = self.root / "output"
        self.out.mkdir(mode=0o700)
        self.args = argparse.Namespace(repository=npm.reuse.REPOSITORY, tag="agentplugins-v1.2.3",
                                       commit="a" * 40, run_id=42, run_attempt=2,
                                       source=str(self.root), output_dir=str(self.out))
        self.tarball_file = "universal-agent-plugins-1.2.3.tgz"
        packed = io.BytesIO()
        with tarfile.open(fileobj=packed, mode="w:gz") as archive:
            data = b'{"name":"universal-agent-plugins","version":"1.2.3"}\n'
            member = tarfile.TarInfo("package/package.json")
            member.size = len(data)
            member.mode = 0o644
            archive.addfile(member, io.BytesIO(data))
        self.tarball = packed.getvalue()
        self.run = {"id": 42, "run_attempt": 2, "repository": {"full_name": self.args.repository},
                    "head_repository": {"full_name": self.args.repository},
                    "head_sha": self.args.commit, "head_branch": "main", "path": npm.reuse.WORKFLOW,
                    "event": "workflow_dispatch", "status": "completed", "conclusion": "success",
                    "run_started_at": "2026-09-28T01:00:00Z", "updated_at": "2026-09-28T03:00:00Z"}
        self.assets = [{"id": 100 + n, "name": name, "size": n + 1, "state": "uploaded",
                        "digest": "sha256:" + "b" * 64} for n, name in enumerate((
                            "checksums.txt", "release-manifest.json", "THIRD_PARTY_NOTICES.txt",
                            "agentplugins_1.2.3_linux_amd64", "agentplugins_1.2.3_linux_arm64",
                            "agentplugins_1.2.3_darwin_amd64", "agentplugins_1.2.3_darwin_arm64",
                            "agentplugins_1.2.3_windows_amd64.exe", "agentplugins_1.2.3_windows_arm64.exe"))]
        self.live = {"id": 123, "tag_name": self.args.tag, "draft": False, "prerelease": False}
        self.receipt = {"schema_version": 2,
                        "release": {"id": 123, "tag": self.args.tag, "commit": self.args.commit,
                                    "draft": True, "prerelease": False, "assets": copy.deepcopy(self.assets)},
                        "subjects": [{"id": asset["id"], "name": asset["name"], "size": asset["size"],
                                      "sha256": "b" * 64} for asset in self.assets]}
        self.set_zip()
        self.receipt["npm_artifact"] = self.binding()
        self.dirty = False
        self.api_reads = []

    def set_zip(self, extra=(), tarball=None):
        buffer = io.BytesIO()
        with warnings.catch_warnings():
            warnings.simplefilter("ignore", UserWarning)
            with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED) as archive:
                archive.writestr(self.tarball_file, self.tarball if tarball is None else tarball)
                archive.writestr("verified-release.json", b"{}")
                archive.writestr("release-assets/", b"")
                for asset in self.assets:
                    archive.writestr("release-assets/" + asset["name"], b"binary proof bundle")
                for name, data in extra:
                    archive.writestr(name, data)
        self.body = buffer.getvalue()
        self.artifact = {"id": 70, "name": "agentplugins-npm-1.2.3", "expired": False,
                         "size_in_bytes": len(self.body), "digest": "sha256:" + hashlib.sha256(self.body).hexdigest(),
                         "created_at": "2026-09-28T02:00:00Z",
                         "workflow_run": {"id": 42, "head_sha": self.args.commit}}

    def binding(self):
        return {**{key: copy.deepcopy(self.artifact[key]) for key in npm.ARTIFACT_KEYS},
                "tarball_file": self.tarball_file,
                "integrity": "sha512-" + base64.b64encode(hashlib.sha512(self.tarball).digest()).decode(),
                "shasum": hashlib.sha1(self.tarball).hexdigest()}

    def api(self, endpoint, *options):
        self.api_reads.append(endpoint)
        prefix = f"repos/{self.args.repository}"
        if endpoint == f"{prefix}/actions/runs/42":
            return self.run
        if endpoint == f"{prefix}/actions/runs/42/artifacts?per_page=100":
            self.assertEqual(options, ("--paginate", "--slurp"))
            return [{"artifacts": [self.artifact]}]
        if endpoint == f"{prefix}/releases/tags/{self.args.tag}":
            return self.live
        if endpoint == f"{prefix}/commits/{self.args.tag}":
            return {"sha": self.args.commit}
        if endpoint == f"{prefix}/releases/123/assets?per_page=100":
            return [self.assets]
        self.fail(f"unexpected API read: {endpoint}")

    def output(self, command):
        if command == ["gh", "api", f"repos/{self.args.repository}/actions/artifacts/70/zip"]:
            return self.body
        if command[:3] == ["git", "-C", str(self.root)]:
            if command[3] in ("rev-parse", "rev-list"):
                return (self.args.commit + "\n").encode()
            if command[3] == "status":
                return b" M changed\n" if self.dirty else b""
        self.fail(f"unexpected command: {command}")

    def verify(self):
        with patch.object(npm.reuse, "authenticate", return_value=self.receipt) as auth, \
             patch.object(npm, "api", side_effect=self.api), \
             patch.object(npm, "output", side_effect=self.output):
            result = npm.verify(self.args)
        auth.assert_called_once_with(self.args)
        return result

    def reject(self):
        with self.assertRaises((ValueError, KeyError, TypeError, zipfile.BadZipFile)):
            self.verify()
        self.assertEqual(list(self.out.iterdir()), [])

    def test_exact_qualified_bytes_reused_without_extracting_binary_assets(self):
        result = self.verify()
        self.assertEqual({file.name for file in self.out.iterdir()}, {self.tarball_file, "npm-package.json"})
        self.assertEqual((self.out / self.tarball_file).read_bytes(), self.tarball)
        self.assertEqual(json.loads((self.out / "npm-package.json").read_text()), result)
        self.assertEqual(result["integrity"], self.receipt["npm_artifact"]["integrity"])
        self.assertEqual(result["shasum"], hashlib.sha1(self.tarball).hexdigest())
        self.assertEqual(result["package_name"], "universal-agent-plugins")
        self.assertEqual(result["version"], "1.2.3")
        self.assertEqual(result["commit"], self.args.commit)

    def test_capture_during_qualification_records_exact_api_identity_and_hashes(self):
        self.run["status"], self.run["conclusion"] = "in_progress", None
        self.run["head_branch"] = self.args.tag
        with patch.object(npm, "api", side_effect=self.api), patch.object(npm, "output", side_effect=self.output):
            captured = npm.capture_npm_artifact(self.args.repository, self.args.tag, self.args.commit,
                                                42, 2, self.tarball_file)
        self.assertEqual(captured, self.receipt["npm_artifact"])

    def test_exact_tag_dispatch_allowed_and_other_branch_refused(self):
        self.run["head_branch"] = "feature"
        self.reject()
        self.run["head_branch"] = self.args.tag
        self.verify()

    def test_missing_or_modified_receipt_binding(self):
        original = copy.deepcopy(self.receipt)
        for field in (None, "id", "name", "digest", "size_in_bytes", "created_at", "workflow_run",
                      "tarball_file", "integrity", "shasum"):
            with self.subTest(field=field):
                self.receipt = copy.deepcopy(original)
                if field is None:
                    self.receipt.pop("npm_artifact")
                else:
                    self.receipt["npm_artifact"].pop(field)
                self.reject()
        for key, bad in (("integrity", "sha512-incorrect"), ("shasum", "c" * 40)):
            self.receipt = copy.deepcopy(original)
            self.receipt["npm_artifact"][key] = bad
            self.reject()
        self.receipt = original
        self.receipt["schema_version"] = 1
        self.reject()

    def test_artifact_identity_run_attempt_and_timestamp_are_bound(self):
        original = copy.deepcopy(self.artifact)
        for key, bad in (("id", 71), ("expired", True), ("name", "agentplugins-npm-9.9.9"),
                         ("workflow_run", {"id": 41, "head_sha": self.args.commit}),
                         ("workflow_run", {"id": 42, "head_sha": "f" * 40}),
                         ("created_at", "2026-09-28T00:59:59Z"),
                         ("created_at", "2026-09-28T03:00:01Z"),
                         ("size_in_bytes", npm.MAX_ARCHIVE + 1)):
            with self.subTest(key=key, bad=bad):
                self.artifact = {**copy.deepcopy(original), key: bad}
                self.reject()
        self.artifact = original
        self.run["run_attempt"] = 3
        self.reject()

    def test_corrupt_zip_or_replaced_tarball_cannot_reuse_old_proof(self):
        self.body += b"corruption"
        self.reject()
        self.set_zip(tarball=b"substituted unqualified npm tarball")
        self.receipt["npm_artifact"].update({key: self.artifact[key] for key in npm.ARTIFACT_KEYS})
        self.reject()

    def test_zip_rejects_traversal_duplicates_links_and_unexpected_members(self):
        for name in ("../escape", "/escape", "release-assets/../escape", "release-assets\\escape",
                     "nested/extra", self.tarball_file):
            with self.subTest(name=name):
                self.set_zip([(name, b"bad")])
                self.receipt["npm_artifact"] = self.binding()
                self.reject()
        link = zipfile.ZipInfo("npm-package.json")
        link.create_system = 3
        link.external_attr = (stat.S_IFLNK | 0o777) << 16
        self.set_zip([(link, b"../escape")])
        self.receipt["npm_artifact"] = self.binding()
        self.reject()

    def test_public_release_and_assets_must_retain_qualified_identities(self):
        for key, bad in (("id", 124), ("tag_name", "agentplugins-v9.9.9"), ("draft", True),
                         ("prerelease", True)):
            with self.subTest(key=key):
                original = self.live[key]
                self.live[key] = bad
                self.reject()
                self.live[key] = original
        for key, bad in (("id", 900), ("size", 9999), ("digest", "sha256:" + "c" * 64)):
            with self.subTest(key=key):
                original = self.assets[0][key]
                self.assets[0][key] = bad
                self.reject()
                self.assets[0][key] = original

    def test_exclusive_output_collision_preserves_foreign_file(self):
        original_open = Path.open
        def competing_open(path, mode="r", *args, **kwargs):
            if path == self.out / "npm-package.json" and mode == "xb":
                with original_open(path, "wb") as stream:
                    stream.write(b"foreign output")
                raise FileExistsError(str(path))
            return original_open(path, mode, *args, **kwargs)
        with patch.object(Path, "open", competing_open):
            with self.assertRaises(FileExistsError):
                self.verify()
        self.assertEqual((self.out / "npm-package.json").read_bytes(), b"foreign output")
        self.assertFalse((self.out / self.tarball_file).exists())

    def test_dirty_source_or_nonprivate_destination_rejected_before_write(self):
        self.dirty = True
        self.reject()
        self.dirty = False
        self.out.chmod(0o755)
        self.reject()


if __name__ == "__main__":
    unittest.main()
