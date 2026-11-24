"""Owned process controls; portable fixtures do not qualify native Firebase."""
from contextlib import ExitStack
import ctypes
import hashlib
import io
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
from types import SimpleNamespace
import unittest
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import windows_firebase_capture_probe as probe


class PortableOwnedGroup:
    """Linux-only test substitute; the Windows test uses the real Job object."""

    def __init__(self, process, ready):
        self.process, self.ready = process, ready
        if os.getpgid(process.pid) != process.pid:
            raise RuntimeError("inert controller lacks its private process group")

    def counts(self):
        pids = {self.process.pid}
        if self.ready.exists():
            pids.update(json.loads(self.ready.read_text()).values())
        active = 0
        for pid in pids:
            try:
                fields = (Path("/proc") / str(pid) / "stat").read_text().rsplit(")", 1)[1].split()
            except FileNotFoundError:
                continue
            active += fields[0] != "Z" and int(fields[2]) == self.process.pid
        return {"total": len(pids), "active": active, "terminated": len(pids) - active}

    def close(self):
        # This controlled fixture keeps its leader alive until this exact close.
        # No detached or guessed PID is used to acquire cleanup authority.
        if self.process.poll() is not None:
            raise RuntimeError("inert live-leader precondition changed")
        os.killpg(self.process.pid, signal.SIGKILL)
        self.process.wait(timeout=2)
        deadline = time.monotonic() + 2
        while self.counts()["active"]:
            if time.monotonic() >= deadline:
                raise RuntimeError("inert owned group did not settle")
            time.sleep(.02)


class FirebaseFamilyControls(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="firebase-family-control-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    @unittest.skipUnless(os.name == "nt" or sys.platform == "linux",
                         "requires Windows Job or the Linux owned-group control")
    def test_real_inert_descendant_waits_for_admission_and_settles_on_timeout(self):
        ready, held = self.root / "ready.json", self.root / "held.bin"
        descendant = (
            "import json,os,sys,time; from pathlib import Path; "
            "held=open(sys.argv[2],'wb'); held.write(b'held'); held.flush(); "
            "Path(sys.argv[1]).write_text(json.dumps(dict(leader=os.getppid(),descendant=os.getpid()))); "
            "time.sleep(60)"
        )
        leader = (
            "import subprocess,sys,time; "
            "subprocess.Popen([sys.executable,'-I','-S','-B','-c',sys.argv[1],sys.argv[2],sys.argv[3]]); "
            "time.sleep(60)"
        )
        real_popen, real_job, real_counts = subprocess.Popen, probe.WindowsJob, probe.job_counts
        admitted = []

        def start(*args, **kwargs):
            if os.name != "nt":
                kwargs["start_new_session"] = True
            return real_popen(*args, **kwargs)

        def admit(process):
            # Give an ungated command an opportunity to write its marker.
            time.sleep(.2)
            self.assertFalse(ready.exists(), "command ran before family admission")
            job = real_job(process) if os.name == "nt" else PortableOwnedGroup(process, ready)
            admitted.append(job)
            return job

        def counts(job):
            return real_counts(job) if os.name == "nt" else job.counts()

        with ExitStack() as stack:
            stdout = stack.enter_context((self.root / "stdout").open("xb"))
            stderr = stack.enter_context((self.root / "stderr").open("xb"))
            stack.enter_context(mock.patch.object(probe.subprocess, "Popen", side_effect=start))
            stack.enter_context(mock.patch.object(probe, "WindowsJob", side_effect=admit))
            stack.enter_context(mock.patch.object(probe, "job_counts", side_effect=counts))
            proof = probe.supervise(
                [sys.executable, "-I", "-S", "-B", "-c", leader, descendant, str(ready), str(held)],
                cwd=self.root, env=dict(os.environ), stdout=stdout, stderr=stderr, timeout=5)
        self.assertEqual(len(admitted), 1)
        self.assertTrue(ready.is_file(), (self.root / "stderr").read_text())
        self.assertTrue(proof["gate_released"])
        self.assertTrue(proof["timeout"])
        self.assertIsNone(proof["child_exit"])
        self.assertGreaterEqual(proof["counts_before"]["active"], 3)
        self.assertTrue(proof["forced"])
        self.assertTrue(proof["settled"], proof)
        self.assertIsNone(proof["primary_error"])
        self.assertIsNone(proof["cleanup_error"])
        if os.name != "nt":
            self.assertEqual(admitted[0].counts()["active"], 0)
        held.unlink()  # The Windows fixture's open file must no longer be held.

    def test_temp_cleanup_requires_settlement_and_never_promotes_failure(self):
        real_mkdtemp = tempfile.mkdtemp
        for name, settled, timeout, child_exit, forced, expected in (
                ("unsettled", False, True, None, True, 1),
                ("settled-timeout", True, True, None, True, 1),
                ("clean", True, False, 0, False, 0)):
            with self.subTest(case=name):
                evidence = self.root / name
                evidence.mkdir()
                sentinel = self.root / (name + "-foreign.txt")
                sentinel.write_bytes(b"unrelated sibling must remain\n")
                original = json.dumps({"status": "passed" if expected == 0 else "failed",
                                       "inert_control": True}).encode() + b"\n"
                roots = []

                def make_root(**kwargs):
                    result = real_mkdtemp(dir=self.root, **kwargs)
                    roots.append(Path(result))
                    return result

                def observe(*_args, **_kwargs):
                    (evidence / "published-firebase-claude.json").write_bytes(original)
                    return {"child_exit": child_exit, "timeout": timeout, "gate_released": True,
                            "counts_before": {"active": int(forced), "total": 3, "terminated": 0},
                            "forced": forced, "settled": settled, "primary_error": None,
                            "cleanup_error": None if settled else "RuntimeError"}

                with mock.patch.object(probe.tempfile, "mkdtemp", side_effect=make_root), \
                        mock.patch.object(probe, "supervise", side_effect=observe):
                    self.assertEqual(probe.run_probe("inert-never-executed.cmd", evidence), expected)
                self.assertEqual(len(roots), 1)
                self.assertEqual(roots[0].exists(), not settled)
                self.assertEqual((evidence / "published-firebase-claude.json").read_bytes(), original)
                self.assertEqual(sentinel.read_bytes(), b"unrelated sibling must remain\n")
                receipt = json.loads((evidence / "published-firebase-process-family.json").read_text())
                self.assertEqual(receipt["family"]["timeout"], timeout)
                self.assertEqual(receipt["temp_removed"], settled)
                self.assertEqual(receipt["retained_owned_temp"], not settled)
                self.assertEqual(receipt["primary_report"],
                                 {"bytes": len(original), "sha256": hashlib.sha256(original).hexdigest()})


class DiagnosticKernel:
    """Inert Win32 responses; membership must precede any executable lookup."""

    def __init__(self, pids=(41, 42), foreign=(42,), name="child.exe", failure=None):
        self.pids, self.foreign, self.name = pids, foreign, name
        self.failure, self.events = failure, []
        self.C, self.handle = ctypes, 700
        self.api = SimpleNamespace(**{
            name: mock.Mock(side_effect=method) for name, method in (
                ("QueryInformationJobObject", self.query),
                ("OpenProcess", self.open), ("IsProcessInJob", self.member),
                ("QueryFullProcessImageNameW", self.image), ("CloseHandle", self.close_handle))})

    def query(self, handle, kind, pointer, length, unused):
        if handle != self.handle or kind != 3 or length != ctypes.sizeof(pointer._obj):
            raise AssertionError("wrong held Job query")
        self.events.append("query")
        if self.failure == "query":
            return 0
        out = pointer._obj
        out.assigned = len(self.pids)
        out.count = 17 if self.failure == "count" else len(self.pids)
        for i, pid in enumerate(self.pids[:16]):
            out.pids[i] = pid
        return 1

    def open(self, access, inherit, pid):
        if access != 0x1000 or inherit:
            raise AssertionError("excess process authority")
        self.events.append(("open", pid))
        return 0 if self.failure == "open" else pid + 1000

    def member(self, handle, job, output):
        if job != self.handle:
            raise AssertionError("foreign Job")
        pid = handle - 1000
        self.events.append(("member", pid))
        output._obj.value = pid not in self.foreign
        return 0 if self.failure == "membership" else 1

    def image(self, handle, flags, output, size):
        pid = handle - 1000
        if (pid in self.foreign or ("member", pid) not in self.events
                or flags != 0 or size._obj.value != 4096):
            raise AssertionError("unverified image read")
        self.events.append(("image", pid))
        if self.failure == "image":
            return 0
        value = "C:\\private-must-not-leak\\" + self.name
        output.value = value
        size._obj.value = len(value)
        return 1

    def close_handle(self, handle):
        self.events.append(("close", handle - 1000))
        return 1


class FirebaseFailureDiagnosticControls(unittest.TestCase):
    setUp = FirebaseFamilyControls.setUp

    def test_cleanup_failure_keeps_primary_negative_and_only_owned_relative_metadata(self):
        evidence = self.root / "evidence"
        evidence.mkdir()
        owned = self.root / "owned"
        owned.mkdir()
        member = owned / "staging" / "locked.txt"
        member.parent.mkdir()
        member.write_bytes(b"owned residue")
        foreign = self.root / "foreign.txt"
        foreign.write_bytes(b"unchanged")
        primary = b'{"status":"failed-install-residue","inert_control":true}\n'
        error = PermissionError(13, "private raw exception must not escape", str(member))
        error.winerror = 32

        def observe(*_args, **_kwargs):
            (evidence / "published-firebase-claude.json").write_bytes(primary)
            return {"child_exit": 1, "timeout": False, "gate_released": True,
                    "counts_before": {"active": 4, "total": 60, "terminated": 0},
                    "forced": True, "settled": True, "primary_error": None,
                    "cleanup_error": None}

        with mock.patch.object(probe.tempfile, "mkdtemp", return_value=str(owned)), \
                mock.patch.object(probe, "supervise", side_effect=observe), \
                mock.patch.object(probe.shutil, "rmtree", side_effect=error) as remove, \
                mock.patch.object(probe.os, "chmod", side_effect=AssertionError("no chmod")):
            self.assertEqual(probe.run_probe("inert-never-executed.cmd", evidence), 1)
        remove.assert_called_once_with(owned)
        doc = json.loads((evidence / "published-firebase-process-family.json").read_text())
        self.assertIn("failure_diagnostics", doc)
        self.assertEqual(doc["failure_diagnostics"]["temp_cleanup"],
                         {"errno": 13, "winerror": 32, "relative_member": "staging/locked.txt"})
        self.assertFalse(doc["temp_removed"])
        self.assertTrue(doc["retained_owned_temp"])
        self.assertEqual(doc["temp_cleanup_error"], "PermissionError")
        self.assertTrue(doc["family"]["settled"])
        self.assertTrue(doc["family"]["forced"])
        self.assertEqual((evidence / "published-firebase-claude.json").read_bytes(), primary)
        self.assertEqual(doc["primary_report"],
                         {"bytes": len(primary), "sha256": hashlib.sha256(primary).hexdigest()})
        self.assertEqual(foreign.read_bytes(), b"unchanged")
        self.assertTrue(member.exists())
        self.assertNotIn(str(owned), json.dumps(doc))
        self.assertNotIn("private raw exception", json.dumps(doc))

    def test_cleanup_attribution_refuses_foreign_alias_replacement_and_unbounded_members(self):
        owned = self.root / "owned"
        owned.mkdir()
        member = owned / "member"
        member.write_bytes(b"owned")
        foreign = self.root / "foreign"
        foreign.write_bytes(b"foreign")
        original = owned.lstat()
        for name, path in (("foreign", foreign), ("escape", owned / ".." / "foreign"),
                           ("long", owned / ("x" * 513)),
                           ("deep", owned.joinpath(*(["x"] * 17))),
                           ("missing", owned / "absent")):
            with self.subTest(name=name):
                error = PermissionError(13, "hidden " + str(foreign), str(path))
                error.winerror = 5
                self.assertEqual(probe.temp_failure_diagnostic(error, owned, original),
                                 {"errno": 13, "winerror": 5, "relative_member": None})
        error = PermissionError(13, "hidden", str(member))
        error.winerror = True
        changed = SimpleNamespace(st_dev=original.st_dev, st_ino=original.st_ino + 1,
                                  st_mode=original.st_mode)
        self.assertIsNone(probe.temp_failure_diagnostic(error, owned, changed)["relative_member"])
        real_lstat = Path.lstat

        def reparse(path, *args, **kwargs):
            value = real_lstat(path, *args, **kwargs)
            if path == member:
                return SimpleNamespace(st_dev=value.st_dev, st_ino=value.st_ino,
                                       st_mode=value.st_mode, st_nlink=1, st_file_attributes=0x400)
            return value

        with mock.patch.object(Path, "lstat", reparse):
            detail = probe.temp_failure_diagnostic(error, owned, original)
        self.assertEqual(detail, {"errno": 13, "winerror": None, "relative_member": None})
        self.assertEqual(foreign.read_bytes(), b"foreign")

    def test_process_snapshot_requires_held_membership_and_bounds_every_query(self):
        for failure in (None, "query", "count", "open", "membership", "image"):
            with self.subTest(failure=failure):
                job = DiagnosticKernel(failure=failure)
                with mock.patch.object(probe.time, "monotonic", return_value=10.0):
                    detail = probe.process_failure_diagnostic(job, 900.0)
                self.assertNotIn("private-must-not-leak", json.dumps(detail))
                self.assertNotIn(42, [p["pid"] for p in detail["processes"]])
                self.assertNotIn(("image", 42), job.events)
                if failure is None:
                    self.assertEqual(detail["processes"], [{"pid": 41, "executable": "child.exe"}])
                if failure in ("query", "count"):
                    self.assertFalse(job.api.OpenProcess.called)
                if failure == "membership":
                    self.assertFalse(job.api.QueryFullProcessImageNameW.called)
                opened = [e[1] for e in job.events if isinstance(e, tuple) and e[0] == "open"]
                closed = [e[1] for e in job.events if isinstance(e, tuple) and e[0] == "close"]
                self.assertEqual(closed, [] if failure == "open" else opened)
        job = DiagnosticKernel(pids=tuple(range(1, 17)), foreign=(), name="x" * 128)
        with mock.patch.object(probe.time, "monotonic", return_value=10.0):
            detail = probe.process_failure_diagnostic(job, 900.0)
        self.assertEqual(len(detail["processes"]), 16)
        self.assertEqual(detail["status"], "complete")
        job = DiagnosticKernel(pids=(41,), foreign=(), name="x" * 129)
        with mock.patch.object(probe.time, "monotonic", return_value=10.0):
            detail = probe.process_failure_diagnostic(job, 900.0)
        self.assertEqual(detail["processes"], [{"pid": 41, "executable": None}])
        with mock.patch.object(probe.time, "monotonic", return_value=10.0):
            expired = DiagnosticKernel()
            self.assertEqual(probe.process_failure_diagnostic(expired, 9.0)["status"], "deadline")
        self.assertFalse(expired.api.QueryInformationJobObject.called)

    def test_diagnostic_errors_and_budget_cannot_prevent_mandatory_family_close(self):
        for scenario in ("success", "raises", "base-exception", "not-forced"):
            with self.subTest(scenario=scenario):
                events = []
                process = SimpleNamespace(pid=111, stdin=io.BytesIO(),
                                          poll=mock.Mock(return_value=1),
                                          wait=mock.Mock(side_effect=lambda **kw: events.append("wait")))
                job = SimpleNamespace(close=mock.Mock(side_effect=lambda: events.append("close")))

                def diagnostic(_job, deadline):
                    self.assertEqual(deadline, 910.0)
                    events.append("diagnostic")
                    if scenario == "raises":
                        raise RuntimeError("hidden diagnostic failure")
                    if scenario == "base-exception":
                        raise KeyboardInterrupt("hidden interruption")
                    return {"status": "complete", "processes": [{"pid": 55, "executable": "child.exe"}]}

                with mock.patch.object(probe.subprocess, "Popen", return_value=process), \
                        mock.patch.object(probe, "WindowsJob", return_value=job), \
                        mock.patch.object(probe, "job_counts", return_value={"total": 4,
                                          "active": int(scenario != "not-forced"), "terminated": 0}), \
                        mock.patch.object(probe, "process_failure_diagnostic", side_effect=diagnostic), \
                        mock.patch.object(probe.time, "monotonic", return_value=10.0):
                    proof = probe.supervise(["inert"], cwd=self.root, env={},
                                            stdout=io.BytesIO(), stderr=io.BytesIO(), timeout=900)
                self.assertEqual(events, (["diagnostic"] if scenario != "not-forced" else []) + ["close", "wait"])
                self.assertTrue(proof["settled"])
                self.assertEqual(proof["child_exit"], 1)
                self.assertIsNone(proof["primary_error"])
                self.assertIsNone(proof["cleanup_error"])
        job = DiagnosticKernel(pids=(41,), foreign=())
        clock = [10.0]
        original_open = job.api.OpenProcess.side_effect

        def late_open(*args):
            result = original_open(*args)
            clock[0] = 11.01
            return result

        job.api.OpenProcess.side_effect = late_open
        with mock.patch.object(probe.time, "monotonic", side_effect=lambda: clock[0]):
            detail = probe.process_failure_diagnostic(job, 900.0)
        self.assertEqual(detail["status"], "deadline")
        self.assertFalse(job.api.IsProcessInJob.called)
        self.assertFalse(job.api.QueryFullProcessImageNameW.called)
        job.api.CloseHandle.assert_called_once_with(1041)
        receipt = {"family": {"child_exit": 1, "settled": True}, "temp_removed": False}
        before = json.dumps(receipt, indent=2).encode()
        oversized = {"processes": [{"pid": i, "executable": "\U0001f600" * 128} for i in range(16)]}
        probe.add_failure_diagnostics(receipt, oversized)
        self.assertLessEqual(len(json.dumps(receipt, indent=2).encode()) - len(before), 8192)
        self.assertEqual(receipt.pop("failure_diagnostics"), {"status": "diagnostic-bound-exceeded"})
        self.assertEqual(json.dumps(receipt, indent=2).encode(), before)



class FirebaseConsoleSummaryControls(unittest.TestCase):
    def proof(self, **changes):
        return {"child_exit": 0, "timeout": False, "gate_released": True,
                "forced": False, "settled": True, "primary_error": None,
                "cleanup_error": None, **changes}

    def primary(self, **changes):
        row = {"status": "passed", "repeat_verified": True, "removal_verified": True,
               "sentinels_preserved": True, "native_residuals": []}
        for label in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
            row[label] = {"exit_code": 0, "timeout": False}
        row.update(changes)
        return json.dumps({"case": row}).encode()

    def summary(self, primary, proof=None, removed=True, error=None):
        line = probe.selected_probe_summary(self.proof() if proof is None else proof,
                                            removed, error, primary)
        self.assertLessEqual(len(line.encode("utf-8")), 1800)
        return json.loads(line)

    def test_lifecycle_negative_and_family_temp_negative_are_distinct(self):
        lifecycle = self.summary(self.primary(status="cleanup-failed", removal_verified=False,
                                              cleanup={"exit_code": 1, "timeout": False}),
                                 self.proof(child_exit=1))
        self.assertEqual(lifecycle["status"], "cleanup-failed")
        self.assertIn("removal_verified", lifecycle["failed_gates"])
        self.assertEqual(lifecycle["commands"]["cleanup"]["exit_code"], 1)
        custody = self.summary(self.primary(), self.proof(forced=True), False, "private error")
        self.assertEqual(custody["status"], "passed")
        self.assertEqual(custody["failed_gates"], [])
        self.assertTrue(custody["family"]["forced"])
        self.assertFalse(custody["temp_removed"])
        self.assertTrue(custody["temp_cleanup_error_present"])

    def test_unsettled_missing_oversized_and_malformed_have_fixed_codes(self):
        with mock.patch.object(probe.json, "loads", side_effect=AssertionError("must not parse")):
            line = probe.selected_probe_summary(self.proof(settled=False), False, None, b"secret")
            self.assertIn('"primary":"unsettled"', line)
            self.assertNotIn("secret", line)
            self.assertIn('"primary":"missing"',
                          probe.selected_probe_summary(self.proof(), True, None, None))
            self.assertIn('"primary":"oversized"', probe.selected_probe_summary(
                self.proof(), True, None, b"x" * (probe.OUTPUT_LIMIT + 1)))
        for data in (b"private malformed", b"null", b'{"case":[]}', b"\xff"):
            self.assertEqual(self.summary(data)["primary"], "malformed")

    def test_unrestricted_strings_containers_and_exit_integers_never_escape(self):
        secret = "DO-NOT-PRINT/private/path/token"
        primary = self.primary(status=secret, repeat_verified=secret,
            native_residuals=[secret], install={"exit_code": True, "timeout": secret},
            cleanup={"exit_code": 10 ** 300, "timeout": [secret]},
            post_remove_list={"exit_code": -(1 << 31) - 1, "timeout": False},
            stdout_tail=secret, stderr_tail=secret, environment={"secret": secret})
        line = probe.selected_probe_summary(self.proof(child_exit=10 ** 300,
            primary_error=secret, cleanup_error=secret), True, secret, primary)
        self.assertNotIn(secret, line)
        self.assertLessEqual(len(line.encode("utf-8")), 1800)
        value = json.loads(line)
        self.assertEqual(value["status"], "unrecognized")
        self.assertIsNone(value["family"]["child_exit"])
        for label in ("install", "cleanup", "post_remove_list"):
            self.assertIsNone(value["commands"][label]["exit_code"])
        for status in probe.SUMMARY_STATUSES:
            self.summary(self.primary(status=status, repeat_verified=False,
                                      removal_verified=False, sentinels_preserved=False,
                                      native_residuals=["private"]))

    def test_diagnostic_byte_cap_has_fixed_fallback(self):
        with mock.patch.object(probe, "CONSOLE_LIMIT", 1):
            line = probe.selected_probe_summary(self.proof(), True, None, self.primary())
        self.assertEqual(json.loads(line)["primary"], "diagnostic-bound")
        self.assertLessEqual(len(line.encode("utf-8")), 1800)


    def test_run_probe_reports_skipped_oversized_body_distinct_from_missing(self):
        for oversized in (False, True):
            with self.subTest(oversized=oversized), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                evidence, owned = root / "evidence", root / "owned"
                evidence.mkdir()
                owned.mkdir()
                report = evidence / "published-firebase-claude.json"
                def observe(*args, **kwargs):
                    if oversized:
                        with report.open("wb") as stream:
                            stream.truncate(probe.OUTPUT_LIMIT + 1)
                    return self.proof()
                original_read = Path.read_bytes
                def guarded_read(path):
                    if path == report:
                        self.fail("unavailable primary must not be read")
                    return original_read(path)
                output = io.StringIO()
                with mock.patch.object(probe.tempfile, "mkdtemp", return_value=str(owned)), \
                        mock.patch.object(probe, "supervise", side_effect=observe), \
                        mock.patch.object(Path, "read_bytes", guarded_read), \
                        mock.patch("sys.stdout", output):
                    self.assertEqual(probe.run_probe("inert-not-executed.cmd", evidence), 1)
                summary = json.loads(output.getvalue())
                self.assertEqual(summary["primary"], "oversized" if oversized else "missing")
                receipt = json.loads((evidence / "published-firebase-process-family.json").read_text())
                self.assertIsNone(receipt["primary_report"])
                self.assertTrue(receipt["temp_removed"])
                self.assertFalse(owned.exists())

    def test_formatter_failure_preserves_run_probe_verdict_and_receipt(self):
        for child_exit, expected in ((0, 0), (1, 1)):
            with self.subTest(child_exit=child_exit), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                evidence, owned = root / "evidence", root / "owned"
                evidence.mkdir()
                owned.mkdir()
                primary = self.primary()
                proof = self.proof(child_exit=child_exit)
                def observe(*args, **kwargs):
                    (evidence / "published-firebase-claude.json").write_bytes(primary)
                    return dict(proof)
                output = io.StringIO()
                with mock.patch.object(probe.tempfile, "mkdtemp", return_value=str(owned)), \
                        mock.patch.object(probe, "supervise", side_effect=observe), \
                        mock.patch.object(probe, "selected_probe_summary", side_effect=ValueError("private")), \
                        mock.patch("sys.stdout", output):
                    verdict = probe.run_probe("inert-not-executed.cmd", evidence)
                self.assertEqual(verdict, expected)
                self.assertFalse(owned.exists())
                receipt = json.loads((evidence / "published-firebase-process-family.json").read_text())
                self.assertEqual(receipt["family"], proof)
                self.assertEqual(receipt["primary_report"], {
                    "bytes": len(primary), "sha256": hashlib.sha256(primary).hexdigest()})
                self.assertTrue(receipt["temp_removed"])
                self.assertEqual((evidence / "published-firebase-claude.json").read_bytes(), primary)
                self.assertEqual(json.loads(output.getvalue())["primary"], "diagnostic-unavailable")
                self.assertNotIn("private", output.getvalue())


if __name__ == "__main__":
    unittest.main()
