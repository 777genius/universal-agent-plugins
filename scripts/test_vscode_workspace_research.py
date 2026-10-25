"""Inert contracts only: no vendor, sudo, acquisition or hosted guard spoofing."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock
import zipfile

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("vscode_workspace_test", ROOT / "research-vscode-workspace.py")
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)


class WorkspaceCollectorTests(unittest.TestCase):
    def test_actual_host_guard_refuses_before_output(self):
        with tempfile.TemporaryDirectory() as root:
            output = Path(root) / "never-created"
            with mock.patch.dict(os.environ, {}, clear=True), mock.patch.object(sys, "argv", ["collector", "--output", str(output)]):
                self.assertEqual(collector.main(), 2)
            self.assertFalse(output.exists())

    def test_publisher_bytes_and_two_executable_contract_remain_fixed(self):
        self.assertEqual(collector.publisher.ARCHIVE_BYTES, 350844741)
        self.assertEqual(collector.publisher.ARCHIVE_SHA256, "48f658052e54e020a1c1be7b37ac0f9ae39f07e8ccda805c63f053cfda72b2be")
        self.assertEqual(set(collector.PINNED_FILES), {"code", "resources/app/out/cli.js", "resources/app/product.json"})
        source = (ROOT.parent / "install/integrationctl/research/vscodeworkspace/model_linux.go").read_text()
        self.assertIn("node_modules.asar.unpacked/@vscode/ripgrep-universal/bin/linux-x64/rg", source)

    def test_control_capture_has_no_privileged_kill_fallback(self):
        errors = []
        with mock.patch.object(collector.os, "killpg", side_effect=PermissionError):
            collector.signal_controller(mock.Mock(pid=123), 15, errors)
        self.assertEqual(len(errors), 1)
        self.assertIn("cleanup not proved", errors[0])

    def test_empty_flat_envelope_admits_only_ordinary_zip(self):
        with tempfile.TemporaryDirectory() as root:
            output = Path(root)
            summary = {"source_commit": "a" * 40, "collection_complete": False, "native_project_qualified": False}
            (output / "vscode-workspace-research.json").write_text(json.dumps(summary))
            self.assertTrue(collector.publish_from_memory(output, summary)["complete"])
            with zipfile.ZipFile(output / "upload/research.zip") as archive:
                self.assertEqual(archive.namelist(), ["vscode-workspace-research.json"])
                self.assertFalse(json.loads(archive.read(archive.namelist()[0]))["native_project_qualified"])

    def test_nonregular_raw_member_yields_only_false_failure_envelope(self):
        with tempfile.TemporaryDirectory() as root:
            output = Path(root)
            summary = {"source_commit": "b" * 40}
            (output / "vscode-workspace-research.json").write_text(json.dumps(summary))
            (output / "native-cache").mkdir()
            self.assertFalse(collector.publish_from_memory(output, summary)["complete"])
            with zipfile.ZipFile(output / "upload/research.zip") as archive:
                self.assertEqual(archive.namelist(), ["collection-failure.json"])
                result = json.loads(archive.read(archive.namelist()[0]))
                self.assertFalse(result["collection_complete"])
                self.assertFalse(result["native_project_qualified"])

    def test_build_and_watchdog_bounds_are_explicit(self):
        source = (ROOT / "research-vscode-workspace.py").read_text()
        self.assertIn('timeout=min(60, budget.check()), limit=1 << 20', source)
        self.assertIn("work_seconds=4.0, reap_seconds=1.0", source)
        self.assertNotIn("--publish-only", source)
        self.assertEqual((collector.ROOT_TIMEOUT, collector.ROOT_KILL), (460, 470))
        self.assertEqual(collector.MAX_CONTROL, 64 << 10)

    def test_unrecorded_flat_member_is_never_uploaded(self):
        with tempfile.TemporaryDirectory() as root:
            output = Path(root)
            summary = {"source_commit": "c" * 40, "collection_complete": False}
            (output / "vscode-workspace-research.json").write_text(json.dumps(summary))
            (output / "unexpected-native-cache.json").write_text("private native data")
            self.assertFalse(collector.publish_from_memory(output, summary)["complete"])
            with zipfile.ZipFile(output / "upload/research.zip") as archive:
                self.assertEqual(archive.namelist(), ["collection-failure.json"])

    def test_fork_uses_preloaded_code_after_source_file_mutation(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            module_path, marker = root / "private_admission.py", root / "mutated-source-executed"
            module_path.write_text("def publish_artifacts(output,summary,allowed_names=None):\n    (output/'preloaded.txt').write_text('preloaded memory')\n    return True\n")
            module_spec = importlib.util.spec_from_file_location("private_preloaded_admission", module_path)
            module = importlib.util.module_from_spec(module_spec)
            module_spec.loader.exec_module(module)
            module_path.write_text("raise AssertionError('mutated source must never execute')\n")
            with mock.patch.object(collector, "publisher", module):
                result = collector.publish_from_memory(root, {})
            self.assertTrue(result["complete"])
            self.assertFalse(marker.exists())
            self.assertEqual((root / "preloaded.txt").read_text(), "preloaded memory")

    def test_new_post_native_import_refuses_without_executing_source(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            marker = root / "executed"
            (root / "new_post_native.py").write_text("raise AssertionError('new import must never execute')\n")
            def lazy_publish(*_args, **_kwargs):
                __import__("new_post_native")
                return True
            with mock.patch.object(collector.publisher, "publish_artifacts", lazy_publish), mock.patch.object(collector.sys, "path", [str(root)] + list(collector.sys.path)):
                self.assertFalse(collector.publish_from_memory(root, {})["complete"])
            self.assertFalse(marker.exists())

    def test_fork_admission_deadline_reaps_only_owned_child(self):
        def blocked(*_args, **_kwargs):
            while True:
                pass
        with tempfile.TemporaryDirectory() as root, mock.patch.object(collector.publisher, "publish_artifacts", blocked):
            result = collector.publish_from_memory(Path(root), {}, work_seconds=0.03, reap_seconds=0.5)
        self.assertFalse(result["complete"])
        self.assertTrue(result["child_reaped"])

    def test_unproved_late_publisher_cannot_recreate_canonical_upload(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            output = root / "output"
            output.mkdir()
            expected = output.lstat()
            release_read, release_write = os.pipe()
            child_pids = []
            original_fork = os.fork
            def fork():
                pid = original_fork()
                if pid > 0:
                    child_pids.append(pid)
                return pid
            def late_publish(selected, _summary, allowed_names=None):
                os.read(release_read, 1)
                try:
                    (selected / "upload").mkdir()
                    (selected / "upload/research.zip").write_bytes(b"late positive bytes")
                except FileNotFoundError:
                    (root / "late-refused").write_bytes(b"canonical parent withheld")
                return True
            try:
                with mock.patch.object(collector.os, "fork", fork), mock.patch.object(collector.os, "kill"), mock.patch.object(collector.publisher, "publish_artifacts", late_publish):
                    result = collector.publish_from_memory(output, {}, work_seconds=0.02, reap_seconds=0.02)
                self.assertFalse(result["child_reaped"])
                self.assertFalse(result["complete"])
                quarantine = collector.quarantine_output(output, expected)
                self.assertTrue(quarantine.is_dir())
                self.assertFalse(output.exists())
                os.write(release_write, b"x")
                deadline = time.monotonic() + 2
                joined = False
                while time.monotonic() < deadline:
                    pid, _status = os.waitpid(child_pids[0], os.WNOHANG)
                    if pid:
                        joined = True
                        break
                    time.sleep(0.01)
                self.assertTrue(joined, "inert late publisher did not join")
                self.assertTrue((root / "late-refused").is_file())
                self.assertFalse((output / "upload/research.zip").exists())
            finally:
                os.close(release_read)
                os.close(release_write)
                for pid in child_pids:
                    try:
                        child, _status = os.waitpid(pid, os.WNOHANG)
                        if not child:
                            os.kill(pid, 9)
                            os.waitpid(pid, 0)
                    except ChildProcessError:
                        pass

    def test_unexpected_publisher_failure_withholds_existing_zip_and_foreign_root_refuses(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            output = root / "output"
            output.mkdir()
            expected = output.lstat()
            def publish_then_fail(selected, _summary, allowed_names=None):
                (selected / "upload").mkdir()
                (selected / "upload/research.zip").write_bytes(b"positive bytes before error")
                raise RuntimeError("inert late publication error")
            with mock.patch.object(collector.publisher, "publish_artifacts", publish_then_fail):
                result = collector.publish_from_memory(output, {})
            self.assertFalse(result["complete"])
            self.assertFalse(result.get("archive_failure_allowed"))
            foreign = root / "foreign"
            foreign.mkdir()
            with self.assertRaises(ValueError):
                collector.quarantine_output(output, foreign.lstat())
            quarantine = collector.quarantine_output(output, expected)
            self.assertEqual((quarantine / "upload/research.zip").read_bytes(), b"positive bytes before error")
            self.assertFalse(output.exists())

    def test_workspace_cleanup_uses_isolated_fixed_python_and_trusted_cwd(self):
        record = {"exit": 0, "error": None}
        with tempfile.TemporaryDirectory() as root:
            path = Path(root)
            with mock.patch.object(collector, "run_probe", return_value=(record, b"", b"")) as run:
                self.assertTrue(collector.cleanup_frontend(path, path.stat(), path)["complete"])
            argv, cwd = run.call_args.args[:2]
            self.assertEqual(argv[:3], ["/usr/bin/python3", "-I", "-c"])
            self.assertEqual(cwd, Path("/"))

    def cleanup_fixture(self):
        task = Path(tempfile.mkdtemp(prefix="vscode-workspace-input-", dir="/tmp"))
        self.addCleanup(lambda: shutil.rmtree(task, ignore_errors=True))
        uid, gid = (65534, 65534) if os.getuid() == 0 else (os.getuid(), os.getgid())
        if os.getuid() == 0:
            os.chown(task, uid, gid)
        return task, uid, gid

    def cleanup_child(self, task, identity, uid, gid):
        argv = ["/usr/bin/python3", "-I", "-c", collector.FRONTEND_CLEANUP_CODE,
                str(task), str(identity.st_dev), str(identity.st_ino), str(uid)]
        if os.getuid() == 0:
            argv = ["/usr/bin/setpriv", "--reuid=" + str(uid), "--regid=" + str(gid), "--clear-groups",
                    "--bounding-set=-all", "--inh-caps=-all", "--ambient-caps=-all", "--no-new-privs", *argv]
        return subprocess.run(argv, cwd="/", env={"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"},
                              stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=7)

    def test_ordinary_uid_cleanup_removes_readonly_cache_and_preserves_external_link_target(self):
        task, uid, gid = self.cleanup_fixture()
        cache = task / "go-cache"
        cache.mkdir()
        script = cache / "mkall.sh"
        script.write_text("inert cache asset")
        with tempfile.TemporaryDirectory() as outside:
            external = Path(outside) / "untouched"
            external.write_text("outside bytes")
            external.chmod(0o440)
            before = external.stat()
            link = task / "outside-link"
            link.symlink_to(external)
            if os.getuid() == 0:
                os.chown(script, uid, gid)
                os.chown(cache, uid, gid)
                os.chown(link, uid, gid, follow_symlinks=False)
            script.chmod(0o444)
            cache.chmod(0o555)
            result = self.cleanup_child(task, task.stat(), uid, gid)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse(task.exists())
            self.assertEqual(external.read_text(), "outside bytes")
            self.assertEqual(external.stat().st_mode, before.st_mode)
            self.assertEqual(external.stat().st_ino, before.st_ino)

    def test_ordinary_uid_cleanup_refuses_replaced_root_without_mutating_replacement(self):
        task, uid, gid = self.cleanup_fixture()
        identity = task.stat()
        saved = task.with_name(task.name + "-saved")
        task.rename(saved)
        self.addCleanup(lambda: shutil.rmtree(saved, ignore_errors=True))
        task.mkdir()
        sentinel = task / "keep"
        sentinel.write_text("replacement bytes")
        if os.getuid() == 0:
            os.chown(sentinel, uid, gid)
            os.chown(task, uid, gid)
        task.chmod(0o555)
        before = task.stat()
        result = self.cleanup_child(task, identity, uid, gid)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(sentinel.read_text(), "replacement bytes")
        self.assertEqual(task.stat().st_mode, before.st_mode)
        task.chmod(0o700)

    @unittest.skipUnless(os.getuid() == 0, "creating a genuinely foreign-owned directory requires the root test fixture")
    def test_ordinary_uid_cleanup_refuses_foreign_directory_without_chmod(self):
        task, uid, gid = self.cleanup_fixture()
        foreign = task / "foreign-owned"
        foreign.mkdir(mode=0o555)
        sentinel = foreign / "keep"
        sentinel.write_text("foreign bytes")
        before = foreign.stat()
        result = self.cleanup_child(task, task.stat(), uid, gid)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(sentinel.read_text(), "foreign bytes")
        self.assertEqual(foreign.stat().st_mode, before.st_mode)

    def test_workflow_uploads_only_independently_admitted_zip(self):
        source = (ROOT.parent / ".github/workflows/vscode-workspace-research.yml").read_text()
        self.assertIn("steps.upload_admission.outputs.valid == 'true'", source)
        self.assertIn("vscode-workspace-research/upload/research.zip", source)
        self.assertIn("persist-credentials: false", source)
        self.assertIn("/usr/bin/python3 -I -", source)
        self.assertIn("github.event.pull_request.head.sha || github.sha", source)


if __name__ == "__main__":
    unittest.main()

