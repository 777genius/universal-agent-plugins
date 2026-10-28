"""Inert process-capture regressions; no native client or hosted guard spoofing."""
import errno
import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

research = None
if os.name == "posix":
    SPEC = importlib.util.spec_from_file_location("bounded_research", Path(__file__).with_name("native_client_research.py"))
    research = importlib.util.module_from_spec(SPEC)
    SPEC.loader.exec_module(research)



@unittest.skipUnless(os.name == "posix", "requires POSIX resource limits, selectable pipes, sessions and killpg")
class CaptureTests(unittest.TestCase):
    def run_child(self, root, body, label="probe", **kwargs):
        return research.run_probe([sys.executable, "-c", body], root, {}, root, label, **kwargs)

    def test_exact_cap_eof_and_each_stream_overflow(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for stream in ("stdout", "stderr"):
                for size in (127, 128, 129):
                    with self.subTest(stream=stream, size=size):
                        record, stdout, stderr = self.run_child(
                            root, f"import sys; sys.{stream}.buffer.write(b'x'*{size}); sys.{stream}.flush()",
                            label=f"{stream}-{size}", limit=128)
                        body = stdout if stream == "stdout" else stderr
                        self.assertEqual(body, b"x" * min(size, 128))
                        self.assertEqual(stderr if stream == "stdout" else stdout, b"")
                        self.assertEqual(record["error"], "output limit reached" if size > 128 else None)
                        if size <= 128:
                            self.assertEqual(record["exit"], 0)
                        self.assertLessEqual((root / record[stream + "_file"]).stat().st_size, 128)

    def test_live_caps_do_not_limit_unrelated_native_cache_files(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            record, stdout, stderr = self.run_child(
                root, "from pathlib import Path; import sys; "
                "Path('native-cache.bin').write_bytes(b'c'*(2*1024*1024)); "
                "print(repr(sys.stdin.read()))", limit=64)
            self.assertIsNone(record["error"])
            self.assertEqual(record["exit"], 0)
            self.assertEqual(stdout, b"''\n")
            self.assertEqual(stderr, b"")
            self.assertEqual((root / "native-cache.bin").stat().st_size, 2 * 1024 * 1024)

    def test_simultaneous_separate_streams_larger_than_pipe_capacity(self):
        with tempfile.TemporaryDirectory() as temporary:
            body = ("import sys,threading; "
                    "t=threading.Thread(target=lambda:sys.stderr.buffer.write(b'e'*262144)); "
                    "t.start(); sys.stdout.buffer.write(b'o'*262144); t.join()")
            record, stdout, stderr = self.run_child(Path(temporary), body, limit=262144)
            self.assertIsNone(record["error"])
            self.assertEqual(record["exit"], 0)
            self.assertEqual(stdout, b"o" * 262144)
            self.assertEqual(stderr, b"e" * 262144)

    def wait_for_unreaped_exit(self, process):
        deadline = time.monotonic() + 2
        while time.monotonic() < deadline:
            if os.waitid(os.P_PID, process.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT):
                return
            time.sleep(.005)
        self.fail("child must exit before the simulated macOS group signal")

    def test_group_permission_error_reaps_exited_leader_before_retry(self):
        signal_group = os.killpg
        with subprocess.Popen([sys.executable, "-c", "pass"], start_new_session=True) as process:
            self.wait_for_unreaped_exit(process)

            def signal_unreaped_group(pid, signum):
                if process.returncode is None:
                    raise PermissionError(errno.EPERM, "unreaped group")
                return signal_group(pid, signum)

            with patch.object(research.os, "killpg", side_effect=signal_unreaped_group):
                research.kill_group(process)
            self.assertEqual(process.returncode, 0)

    def test_group_permission_error_remains_an_error_for_live_or_denied_group(self):
        for exited in (False, True):
            with self.subTest(exited=exited):
                body = "pass" if exited else "import time; time.sleep(10)"
                with subprocess.Popen([sys.executable, "-c", body], start_new_session=True) as process:
                    try:
                        if exited:
                            self.wait_for_unreaped_exit(process)
                        with patch.object(research.os, "killpg", side_effect=PermissionError(errno.EPERM, "denied")):
                            with self.assertRaises(PermissionError):
                                research.kill_group(process)
                        self.assertEqual(process.returncode, 0 if exited else None)
                    finally:
                        research.kill_group(process)
                        process.wait(timeout=2)

    def descendant_body(self, root, exit_leader):
        ready, survived = root / "descendant-ready", root / "descendant-survived"
        child = ("from pathlib import Path; import time; "
                 f"Path({str(ready)!r}).write_text('ready'); time.sleep(.5); "
                 f"Path({str(survived)!r}).write_text('escaped')")
        parent = ("import subprocess,time; from pathlib import Path; "
                  f"subprocess.Popen([{sys.executable!r},'-c',{child!r}]); "
                  f"ready=Path({str(ready)!r}); "
                  "\nwhile not ready.exists(): time.sleep(.005)\n")
        if not exit_leader:
            parent += "time.sleep(10)\n"
        return parent, ready, survived

    def test_leader_exit_closes_descendant_inherited_pipes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            body, ready, survived = self.descendant_body(root, True)
            started = time.monotonic()
            record, _, _ = self.run_child(root, body, timeout=2)
            self.assertTrue(ready.exists(), "descendant must start before leader exit")
            self.assertIsNone(record["error"])
            self.assertEqual(record["exit"], 0)
            self.assertLess(time.monotonic() - started, 1)
            time.sleep(.6)
            self.assertFalse(survived.exists())

    def test_group_permission_retry_still_terminates_surviving_descendant(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            body, ready, survived = self.descendant_body(root, True)
            signal_group = os.killpg
            with subprocess.Popen([sys.executable, "-c", body], cwd=root, start_new_session=True) as process:
                try:
                    self.wait_for_unreaped_exit(process)
                    self.assertTrue(ready.exists(), "descendant must start before leader exit")
                    first_signal = True

                    def deny_first_signal(pid, signum):
                        nonlocal first_signal
                        if first_signal:
                            first_signal = False
                            raise PermissionError(errno.EPERM, "inert group denial")
                        return signal_group(pid, signum)

                    with patch.object(research.os, "killpg", side_effect=deny_first_signal):
                        research.kill_group(process)
                    self.assertEqual(process.returncode, 0)
                    time.sleep(.6)
                    self.assertFalse(survived.exists())
                finally:
                    research.kill_group(process)
                    process.wait(timeout=2)

    def test_capture_error_and_parent_interruption_reap_started_child(self):
        for failure in (OSError("inert capture failure"), KeyboardInterrupt("inert interruption")):
            with self.subTest(failure=type(failure).__name__), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                body, ready, survived = self.descendant_body(root, False)
                processes = []

                def interrupt(process, *_args):
                    processes.append(process)
                    deadline = time.monotonic() + 2
                    while not ready.exists() and time.monotonic() < deadline:
                        time.sleep(.005)
                    self.assertTrue(ready.exists(), "actual startup required before injected failure")
                    raise failure

                with patch.object(research, "capture_output", side_effect=interrupt):
                    if isinstance(failure, OSError):
                        record, _, _ = self.run_child(root, body)
                        self.assertEqual(record["error"], "inert capture failure")
                    else:
                        with self.assertRaises(KeyboardInterrupt):
                            self.run_child(root, body)
                self.assertIsNotNone(processes[0].poll())
                self.assertTrue(processes[0].stdout.closed)
                self.assertTrue(processes[0].stderr.closed)
                time.sleep(.6)
                self.assertFalse(survived.exists())

    def test_failed_startup_preserves_bounded_empty_logs(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            record, stdout, stderr = research.run_probe([str(root / "missing-inert-command")], root, {}, root, "start")
            self.assertIsNone(record["exit"])
            self.assertTrue(record["error"])
            self.assertLessEqual(len(record["error"]), 512)
            self.assertEqual((stdout, stderr), (b"", b""))


if __name__ == "__main__":
    unittest.main()


class NonPosixImportTests(unittest.TestCase):
    def test_nonposix_discovery_does_not_import_posix_capture(self):
        source = Path(__file__)
        spec = importlib.util.spec_from_file_location("inert_nonposix_capture_tests", source)
        module = importlib.util.module_from_spec(spec)
        with patch.object(os, "name", "nt"), patch.object(importlib.util, "spec_from_file_location", side_effect=AssertionError("POSIX helper import on non-POSIX")):
            spec.loader.exec_module(module)
        self.assertIsNone(module.research)
        self.assertTrue(module.CaptureTests.__unittest_skip__)
