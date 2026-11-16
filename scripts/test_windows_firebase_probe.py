"""Owned process controls; portable fixtures do not qualify native Firebase."""
from contextlib import ExitStack
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
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


if __name__ == "__main__":
    unittest.main()
