"""Inert Cline frontend recovery tests; no vendor or hosted happy-path."""
import importlib.util
import os
import select
from pathlib import Path
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

SPEC=importlib.util.spec_from_file_location("cline_project_gate",Path(__file__).with_name("research-cline-project.py"))
gate=importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(gate)

class ClineProjectFrontendTests(unittest.TestCase):
    def test_real_guard_refuses_before_any_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            output=Path(temporary)/"no-output"
            with patch.dict(os.environ,{},clear=True),patch.object(sys,"argv",[str(SPEC.origin),"--output",str(output)]),patch.object(gate.cline,"acquire_binary",side_effect=AssertionError("vendor acquisition")):
                self.assertEqual(gate.main(),2)
            self.assertFalse(output.exists())

    def test_environment_has_only_explicit_disposable_authority(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()
            with patch.dict(os.environ,{"GITHUB_TOKEN":"inert","CLINE_MCP_SETTINGS_PATH":"/foreign","PYTHONPATH":"/foreign"}):
                env=gate.clean_environment(root,Path("/known/go/bin/go"))
            self.assertEqual(env["CLINE_NO_AUTO_UPDATE"],"1")
            self.assertEqual(env["HOME"],str(root/"frontend-home"))
            for key in ("GITHUB_TOKEN","CLINE_MCP_SETTINGS_PATH","PYTHONPATH","CLINE_DIR","CLINE_DATA_DIR"):self.assertNotIn(key,env)
            self.assertEqual(env["GOTOOLCHAIN"],"local")

    def test_closed_capture_table_and_duplicate_refuse_before_child(self):
        summary={"commands":[]}
        budget=type("Budget",(),{"check":lambda _self:5})()
        result={"exit":0,"error":None}
        with patch.object(gate,"run_probe",return_value=(result,b"bounded",b"")) as probe:
            self.assertEqual(gate.capture("frontend-commit",["/usr/bin/git","rev-parse","HEAD"],"/",{},Path("/inert"),summary,budget,10),(b"bounded",b""))
            self.assertEqual(probe.call_args.kwargs["limit"],128<<10)
            self.assertEqual(probe.call_args.kwargs["timeout"],5)
            for label in ("frontend-commit","unknown"):
                with self.assertRaises(ValueError):gate.capture(label,["unreviewed"],"/",{},Path("/inert"),summary,budget,10)
            self.assertEqual(probe.call_count,1)

    def test_executable_hash_never_follows_alias_or_multilink(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();ordinary=root/"inert-binary";ordinary.write_bytes(b"inert bytes")
            self.assertEqual(gate.pinned_file(ordinary,64)["bytes"],11)
            alias=root/"alias";alias.symlink_to(ordinary)
            with self.assertRaises(ValueError):gate.pinned_file(alias,64)
            alias.unlink();os.link(ordinary,alias)
            with self.assertRaises(ValueError):gate.pinned_file(ordinary,64)
            alias.unlink()
            with self.assertRaises(ValueError):gate.pinned_file(ordinary,1)

    def test_cleanup_deletes_only_exact_owned_root_and_never_alias_target(self):
        with tempfile.TemporaryDirectory() as temporary:
            parent=Path(temporary).resolve();foreign=parent/"foreign";foreign.mkdir();(foreign/"keep").write_bytes(b"caller owned")
            root=parent/"scratch";root.mkdir();expected=root.lstat();(root/"alias").symlink_to(foreign,target_is_directory=True)
            sealed=root/"readonly";sealed.mkdir();(sealed/"file").write_bytes(b"inert");sealed.chmod(0o500)
            gate.bounded_cleanup(root,expected)
            self.assertFalse(root.exists());self.assertEqual((foreign/"keep").read_bytes(),b"caller owned")
            root.mkdir()
            with self.assertRaises(ValueError):gate.bounded_cleanup(root,foreign.lstat())
            self.assertTrue(root.exists())

    def test_cleanup_deadline_refuses_without_broad_fallback(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()/"scratch";root.mkdir();(root/"keep").write_bytes(b"inert");expected=root.lstat()
            with patch.object(gate.time,"monotonic",side_effect=[0,6]):
                with self.assertRaises(TimeoutError):gate.bounded_cleanup(root,expected)
            self.assertEqual((root/"keep").read_bytes(),b"inert")

    @unittest.skipUnless(hasattr(os,"fork"),"forked preloaded publisher is Linux-only")
    def test_publication_calls_loaded_function_and_denies_late_import(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();destination=root/"proof"
            def already_loaded(_raw,selected,*,client):
                if client!="cline":raise AssertionError("unreviewed client")
                fd=os.open(selected,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
                try:os.write(fd,b"loaded closure")
                finally:os.close(fd)
                return True
            with patch.object(gate.artifacts,"write_archive",already_loaded):self.assertTrue(gate.publish(root,destination)["complete"])
            self.assertEqual(destination.read_bytes(),b"loaded closure")
            def late_import(_raw,_selected,*,client):
                if client!="cline":raise AssertionError("unreviewed client")
                __import__("cline_gate_deliberately_absent_module")
                return True
            with patch.object(gate.artifacts,"write_archive",late_import):self.assertEqual(gate.publish(root,root/"refused")["exit"],3)
            self.assertFalse((root/"refused").exists())

    @unittest.skipUnless(hasattr(os,"fork"),"forked admission is Linux-only")
    def test_stuck_publisher_is_killed_and_reaped_within_reserved_window(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()
            def stuck(_raw,_selected,*,client):
                if client!="cline":raise AssertionError("unreviewed client")
                time.sleep(30);return True
            started=time.monotonic()
            with patch.object(gate.artifacts,"write_archive",stuck):result=gate.publish(root,root/"absent",work_seconds=0.01,reap_seconds=1)
            self.assertFalse(result["complete"]);self.assertTrue(result["child_reaped"])
            self.assertEqual(result["error"],"artifact admission deadline")
            self.assertLess(time.monotonic()-started,1.5);self.assertFalse((root/"absent").exists())

    def test_unreapable_child_reports_uncertainty_without_blocking_wait(self):
        with patch.object(gate.os,"fork",return_value=12345),patch.object(gate.os,"waitpid",return_value=(0,0)) as waited,patch.object(gate.os,"kill") as killed,patch.object(gate.time,"monotonic",side_effect=[0,0.1,4.1,4.1,4.2,5.2]):
            result=gate.publish(Path("/inert"),Path("/inert.zip"))
        self.assertFalse(result["complete"]);self.assertFalse(result["child_reaped"])
        self.assertEqual(result["error"],"artifact admission cleanup unproved")
        self.assertEqual(waited.call_count,2)
        for call in waited.call_args_list:self.assertEqual(call.args,(12345,os.WNOHANG))
        killed.assert_called_once_with(12345,gate.signal.SIGKILL)

class PublicationWithholdingTests(unittest.TestCase):
    def paths(self, temporary):
        root = Path(temporary).resolve()
        raw, upload = root / "raw", root / "upload"
        raw.mkdir()
        upload.mkdir()
        return root, raw, upload, upload.lstat()

    def withheld(self, root, upload):
        self.assertFalse(upload.exists())
        quarantines = list(root.glob("upload-unproved-*/withheld"))
        self.assertEqual(len(quarantines), 1)
        return quarantines[0]

    @unittest.skipUnless(hasattr(os, "fork"), "Linux-only preloaded publisher")
    def test_exception_after_zip_withholds_reaped_positive_archive(self):
        with tempfile.TemporaryDirectory() as temporary:
            root, raw, upload, expected = self.paths(temporary)
            def write_then_raise(_raw, selected, *, client):
                selected.write_bytes(b"positive bytes before unexpected exception")
                raise RuntimeError("inert post-publication failure")
            with patch.object(gate.artifacts, "write_archive", write_then_raise):
                result = gate.admit_publication(raw, upload, expected)
            self.assertFalse(result["complete"])
            self.assertTrue(result["child_reaped"])
            self.assertEqual(result["exit"], 3)
            self.assertFalse(result["archive_failure_allowed"])
            self.assertEqual((self.withheld(root, upload) / "research.zip").read_bytes(),
                             b"positive bytes before unexpected exception")

    @unittest.skipUnless(hasattr(os, "fork"), "Linux-only preloaded publisher")
    def test_reaped_deadline_withholds_already_written_archive(self):
        with tempfile.TemporaryDirectory() as temporary:
            root, raw, upload, expected = self.paths(temporary)
            ready_read, ready_write = os.pipe()
            original_fork = os.fork
            def fork_after_written_barrier():
                pid = original_fork()
                if pid > 0:
                    if not select.select([ready_read], [], [], 2)[0]:
                        os.kill(pid, 9)
                        deadline = time.monotonic() + 2
                        while time.monotonic() < deadline:
                            if os.waitpid(pid, os.WNOHANG)[0]:
                                break
                            time.sleep(0.01)
                        else:
                            raise AssertionError("inert barrier cleanup unproved")
                        raise AssertionError("inert writer did not reach archive barrier")
                    os.read(ready_read, 1)
                return pid
            def write_then_wait(_raw, selected, *, client):
                selected.write_bytes(b"positive bytes before deadline")
                os.write(ready_write, b"x")
                time.sleep(30)
                return True
            try:
                with patch.object(gate.os, "fork", fork_after_written_barrier), patch.object(gate.artifacts, "write_archive", write_then_wait):
                    result = gate.admit_publication(raw, upload, expected, work_seconds=0.02, reap_seconds=1)
            finally:
                os.close(ready_read)
                os.close(ready_write)
            self.assertFalse(result["complete"])
            self.assertTrue(result["child_reaped"])
            self.assertEqual(result["error"], "artifact admission deadline")
            self.assertFalse(result["archive_failure_allowed"])
            self.assertEqual((self.withheld(root, upload) / "research.zip").read_bytes(), b"positive bytes before deadline")

    @unittest.skipUnless(hasattr(os, "fork"), "Linux-only preloaded publisher")
    def test_closed_normal_failure_retains_only_its_failure_path(self):
        with tempfile.TemporaryDirectory() as temporary:
            _root, raw, upload, expected = self.paths(temporary)
            def normal_failure(_raw, selected, *, client):
                selected.write_bytes(b"closed synthetic false failure envelope")
                return False
            with patch.object(gate.artifacts, "write_archive", normal_failure):
                result = gate.admit_publication(raw, upload, expected)
            self.assertFalse(result["complete"])
            self.assertTrue(result["child_reaped"])
            self.assertEqual(result["exit"], 2)
            self.assertTrue(result["archive_failure_allowed"])
            self.assertEqual((upload / "research.zip").read_bytes(), b"closed synthetic false failure envelope")

    def test_deadline_exit_two_never_inherits_normal_failure_admission(self):
        # An exit2 observed only after the work deadline remains unqualified.
        with patch.object(gate.os, "fork", return_value=12345), patch.object(gate.os, "waitpid", side_effect=[(0, 0), (12345, 512)]), patch.object(gate.os, "kill"), patch.object(gate.time, "monotonic", side_effect=[0, 0.1, 4.1, 4.1, 4.2]):
            result = gate.publish(Path("/inert"), Path("/inert.zip"))
        self.assertFalse(result["complete"])
        self.assertTrue(result["child_reaped"])
        self.assertEqual(result["exit"], 2)
        self.assertFalse(result["archive_failure_allowed"])

    @unittest.skipUnless(hasattr(os, "fork"), "Linux-only preloaded publisher")
    def test_unreaped_late_writer_cannot_recreate_withheld_upload_parent(self):
        with tempfile.TemporaryDirectory() as temporary:
            root, raw, upload, expected = self.paths(temporary)
            release_read, release_write = os.pipe()
            child_pids = []
            original_fork, original_kill = os.fork, os.kill
            def fork():
                pid = original_fork()
                if pid > 0:
                    child_pids.append(pid)
                return pid
            def late_write(_raw, selected, *, client):
                os.read(release_read, 1)
                try:
                    selected.write_bytes(b"late positive bytes")
                except FileNotFoundError:
                    (root / "late-refused").write_bytes(b"parent remains withheld")
                return True
            joined = False
            try:
                with patch.object(gate.os, "fork", fork), patch.object(gate.os, "kill"), patch.object(gate.artifacts, "write_archive", late_write):
                    result = gate.admit_publication(raw, upload, expected, work_seconds=0.02, reap_seconds=0.02)
                self.assertFalse(result["child_reaped"])
                self.assertFalse(result["archive_failure_allowed"])
                self.withheld(root, upload)
                os.write(release_write, b"x")
                deadline = time.monotonic() + 2
                while time.monotonic() < deadline:
                    if os.waitpid(child_pids[0], os.WNOHANG)[0]:
                        joined = True
                        break
                    time.sleep(0.01)
                self.assertTrue(joined, "inert late publisher did not join")
                self.assertEqual((root / "late-refused").read_bytes(), b"parent remains withheld")
                self.assertFalse(upload.exists())
            finally:
                os.close(release_read)
                os.close(release_write)
                if child_pids and not joined:
                    original_kill(child_pids[0], 9)
                    deadline = time.monotonic() + 2
                    while time.monotonic() < deadline:
                        if os.waitpid(child_pids[0], os.WNOHANG)[0]:
                            joined = True
                            break
                        time.sleep(0.01)
                    if not joined:
                        self.fail("inert publisher cleanup remains unproved")

    def test_quarantine_refuses_foreign_or_alias_authority(self):
        with tempfile.TemporaryDirectory() as temporary:
            root, _raw, upload, expected = self.paths(temporary)
            foreign = root / "foreign"
            foreign.mkdir()
            with self.assertRaises(ValueError):
                gate.quarantine_upload(upload, foreign.lstat())
            self.assertTrue(upload.is_dir())
            alias = root / "alias"
            alias.symlink_to(upload, target_is_directory=True)
            with self.assertRaises(ValueError):
                gate.quarantine_upload(alias, expected)
            self.assertTrue(upload.is_dir())


if __name__=="__main__":unittest.main()

class LoadedRealClinePublisherTests(unittest.TestCase):
    @unittest.skipUnless(hasattr(os,'fork'),'Linux-only preloaded publisher')
    def test_actual_loaded_cline_archive_has_no_post_native_import_or_reexec(self):
        import json,zipfile
        # This deliberately records only a failed/partial synthetic stage. It
        # proves the loaded file-admission closure, not native observations.
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();source=root/'raw';source.mkdir();upload=root/'upload';upload.mkdir()
            summary={'schema_version':1,'client':'cline','version':'3.0.73','collection_complete':False,'commands':[]}
            summary.update({key:False for key in gate.cline_project_envelope.NATIVE_FALSE})
            (source/gate.SUMMARY).write_text(json.dumps(summary))
            for name in ('utf-8','cp437'):gate.codecs.lookup(name)
            # Publish uses the module objects loaded above; an exec/import of
            # any mutable original source is neither requested nor permitted.
            result=gate.publish(source,upload/'proof.zip')
            self.assertTrue(result['complete'],result)
            self.assertTrue(result['child_reaped'])
            with zipfile.ZipFile(upload/'proof.zip') as archive:
                self.assertEqual(archive.namelist(),[gate.SUMMARY])
                self.assertFalse(json.loads(archive.read(gate.SUMMARY))['collection_complete'])
