"""Inert process-capture regressions; no native client or hosted guard spoofing."""
import errno
import importlib.util
import io
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from types import SimpleNamespace
from unittest.mock import Mock, call, patch

research = None
if os.name == "posix":
    SPEC = importlib.util.spec_from_file_location("bounded_research", Path(__file__).with_name("native_client_research.py"))
    research = importlib.util.module_from_spec(SPEC)
    SPEC.loader.exec_module(research)


class PortableReadTests(unittest.TestCase):
    def load(self):
        spec = importlib.util.spec_from_file_location("portable_research", Path(__file__).with_name("native_client_research.py"))
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_import_and_bounded_reads_do_not_require_posix_resource(self):
        with patch.dict(sys.modules, {"resource": None}):
            module = self.load()
            self.assertEqual(module.bounded_read(io.BytesIO(b"bounded"), 7), b"bounded")
            with self.assertRaises(ModuleNotFoundError):
                module.child_limits()

    def test_unsupported_probe_refuses_before_files_or_processes(self):
        module = self.load()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with patch.object(module, "os", SimpleNamespace(name="unsupported")), patch.object(module.subprocess, "Popen") as spawn:
                with self.assertRaisesRegex(ValueError, "requires POSIX"):
                    module.run_probe(["never-executed"], root, {}, root, "refused")
                spawn.assert_not_called()
            self.assertEqual(list(root.iterdir()), [])


@unittest.skipUnless(os.name == "posix", "requires POSIX process-group signaling")
class GroupCleanupTests(unittest.TestCase):
    def process(self, returncode):
        process = Mock(spec=["pid", "poll", "wait"], pid=43210)
        process.poll.return_value = returncode
        return process

    def test_initial_success_or_missing_group_does_not_poll(self):
        for outcome in (None, ProcessLookupError(errno.ESRCH, "missing group")):
            with self.subTest(outcome=outcome):
                process = self.process(None)
                with patch.object(research.os, "killpg", side_effect=[outcome]) as kill:
                    research.kill_group(process)
                kill.assert_called_once_with(43210, research.signal.SIGKILL)
                process.poll.assert_not_called()
                process.wait.assert_not_called()

    def test_permission_error_with_live_child_preserves_original_failure(self):
        process = self.process(None)
        process.wait.side_effect = subprocess.TimeoutExpired(["owned-child"], 5)
        denied = PermissionError(errno.EPERM, "original group refusal")
        with patch.object(research.os, "killpg", side_effect=denied) as kill:
            with self.assertRaises(PermissionError) as raised:
                research.kill_group(process)
        self.assertIs(raised.exception, denied)
        process.poll.assert_called_once_with()
        process.wait.assert_called_once_with(timeout=5)
        kill.assert_called_once_with(43210, research.signal.SIGKILL)

    def test_permission_error_waits_for_exiting_child_then_requires_original_group_absence(self):
        process = self.process(None)
        denied = PermissionError(errno.EPERM, "exiting group")

        def reap(*, timeout):
            self.assertEqual(timeout, 5)
            process.pid = 54321
            return 0

        process.wait.side_effect = reap
        with patch.object(research.os, "killpg", side_effect=[denied, ProcessLookupError(errno.ESRCH, "missing group")]) as kill:
            research.kill_group(process)
        process.poll.assert_called_once_with()
        process.wait.assert_called_once_with(timeout=5)
        self.assertEqual(kill.call_args_list, [call(43210, research.signal.SIGKILL), call(43210, 0)])

    def test_reaped_late_child_does_not_admit_present_or_ambiguous_group(self):
        for outcome in (None, PermissionError(errno.EPERM, "probe refusal"), OSError(errno.EIO, "probe error")):
            with self.subTest(outcome=outcome):
                process = self.process(None)
                process.wait.return_value = 0
                denied = PermissionError(errno.EPERM, "original group refusal")
                with patch.object(research.os, "killpg", side_effect=[denied, outcome]) as kill:
                    with self.assertRaises(PermissionError) as raised:
                        research.kill_group(process)
                self.assertIs(raised.exception, denied)
                process.wait.assert_called_once_with(timeout=5)
                self.assertEqual(kill.call_args_list, [call(43210, research.signal.SIGKILL), call(43210, 0)])

    def test_wait_failure_or_interruption_is_not_hidden_by_absence_probe(self):
        for failure in (OSError(errno.EIO, "wait failure"), RuntimeError("unexpected wait failure"), KeyboardInterrupt("interrupted wait")):
            with self.subTest(failure=type(failure).__name__):
                process = self.process(None)
                process.wait.side_effect = failure
                denied = PermissionError(errno.EPERM, "original group refusal")
                with patch.object(research.os, "killpg", side_effect=denied) as kill:
                    with self.assertRaises(type(failure)) as raised:
                        research.kill_group(process)
                self.assertIs(raised.exception, failure)
                process.wait.assert_called_once_with(timeout=5)
                kill.assert_called_once_with(43210, research.signal.SIGKILL)

    def test_exited_child_accepts_only_absence_of_original_group(self):
        process = self.process(0)
        denied = PermissionError(errno.EPERM, "original group refusal")

        def reap():
            process.pid = 54321
            return 0

        process.poll.side_effect = reap
        with patch.object(research.os, "killpg", side_effect=[denied, ProcessLookupError(errno.ESRCH, "missing group")]) as kill:
            research.kill_group(process)
        process.poll.assert_called_once_with()
        process.wait.assert_not_called()
        self.assertEqual(kill.call_args_list, [call(43210, research.signal.SIGKILL), call(43210, 0)])

    def test_present_or_ambiguous_group_preserves_original_permission_failure(self):
        for outcome in (None, PermissionError(errno.EPERM, "probe refusal"), OSError(errno.EIO, "probe error")):
            with self.subTest(outcome=outcome):
                process = self.process(-9)
                denied = PermissionError(errno.EPERM, "original group refusal")
                with patch.object(research.os, "killpg", side_effect=[denied, outcome]) as kill:
                    with self.assertRaises(PermissionError) as raised:
                        research.kill_group(process)
                self.assertIs(raised.exception, denied)
                process.poll.assert_called_once_with()
                self.assertEqual(kill.call_args_list, [call(43210, research.signal.SIGKILL), call(43210, 0)])

    def test_absence_probe_does_not_suppress_unexpected_failure_or_interruption(self):
        for failure in (RuntimeError("unexpected probe failure"), KeyboardInterrupt("interrupted probe")):
            with self.subTest(failure=type(failure).__name__):
                process = self.process(0)
                denied = PermissionError(errno.EPERM, "original group refusal")
                with patch.object(research.os, "killpg", side_effect=[denied, failure]) as kill:
                    with self.assertRaises(type(failure)) as raised:
                        research.kill_group(process)
                self.assertIs(raised.exception, failure)
                process.poll.assert_called_once_with()
                self.assertEqual(kill.call_args_list, [call(43210, research.signal.SIGKILL), call(43210, 0)])


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

    def test_group_permission_error_waits_for_owned_child_exiting_after_poll(self):
        signal_group = os.killpg
        denied = PermissionError(errno.EPERM, "leader exit transition")
        with subprocess.Popen([sys.executable, "-c", "import sys; sys.stdin.buffer.read()"],
                              stdin=subprocess.PIPE, start_new_session=True) as process:
            poll = process.poll

            def begin_exit():
                result = poll()
                self.assertIsNone(result, "owned child must still wait for EOF")
                process.stdin.close()
                return result

            def deny_kill(pid, signum):
                if signum == research.signal.SIGKILL:
                    raise denied
                return signal_group(pid, signum)

            with patch.object(process, "poll", side_effect=begin_exit), patch.object(research.os, "killpg", side_effect=deny_kill) as kill:
                research.kill_group(process)
            self.assertEqual(process.returncode, 0)
            self.assertEqual(kill.call_args_list, [
                call(process.pid, research.signal.SIGKILL), call(process.pid, 0)])

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

    def test_group_permission_error_refuses_surviving_descendant_without_retry(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            body, ready, survived = self.descendant_body(root, True)
            signal_group = os.killpg
            with subprocess.Popen([sys.executable, "-c", body], cwd=root, start_new_session=True) as process:
                try:
                    self.wait_for_unreaped_exit(process)
                    self.assertTrue(ready.exists(), "descendant must start before leader exit")
                    first_signal = True
                    denied = PermissionError(errno.EPERM, "inert group denial")

                    def deny_first_signal(pid, signum):
                        nonlocal first_signal
                        if first_signal:
                            first_signal = False
                            raise denied
                        return signal_group(pid, signum)

                    with patch.object(research.os, "killpg", side_effect=deny_first_signal) as kill:
                        with self.assertRaises(PermissionError) as raised:
                            research.kill_group(process)
                    self.assertIs(raised.exception, denied)
                    self.assertEqual(process.returncode, 0)
                    self.assertEqual(kill.call_args_list, [
                        call(process.pid, research.signal.SIGKILL), call(process.pid, 0)])
                finally:
                    research.kill_group(process)
                    process.wait(timeout=2)
                time.sleep(.6)
                self.assertFalse(survived.exists())

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


@unittest.skipUnless(os.name == "posix", "requires POSIX process capture")
class FiniteInputProbeTests(unittest.TestCase):
    def test_exact_input_is_closed_at_eof_and_capture_is_complete(self):
        import hashlib
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            payload = b'{"id":"one","type":"get_commands"}\n'
            command = [sys.executable, "-c", "import sys; data=sys.stdin.buffer.read(); sys.stdout.buffer.write(data)"]
            record, stdout, stderr = research.run_probe(command, root, {}, root, "stdin", memory_only=True, input_bytes=payload)
            self.assertEqual((record["exit"], record["error"], stdout, stderr), (0, None, payload, b""))
            self.assertTrue(record["stdin_written"])
            self.assertEqual(record["stdin_bytes"], len(payload))
            self.assertEqual(record["stdin_sha256"], hashlib.sha256(payload).hexdigest())
            self.assertEqual(list(root.iterdir()), [])

    def test_input_refuses_before_files_or_spawn(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for payload in (b"", b"a" * 257, "text", bytearray(b"a"), 1):
                with self.subTest(payload=type(payload).__name__), patch.object(research.subprocess, "Popen") as spawn:
                    with self.assertRaises(ValueError):
                        research.run_probe(["never"], root, {}, root, "invalid", input_bytes=payload)
                    spawn.assert_not_called()
                self.assertEqual(list(root.iterdir()), [])

    def test_partial_and_nonblocking_refusal_never_claim_written(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for effect in (lambda *_: 0, BlockingIOError(errno.EAGAIN, "full"), BrokenPipeError(errno.EPIPE, "closed")):
                with self.subTest(effect=str(effect)), patch.object(research.os, "write", side_effect=effect):
                    record, _, _ = research.run_probe([sys.executable, "-c", "import sys; sys.stdin.buffer.read()"], root, {}, root, "short", memory_only=True, input_bytes=b"a")
                self.assertIsNotNone(record["error"])
                self.assertFalse(record["stdin_written"])

    def test_timeout_and_output_bound_still_apply_with_input(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for body, kwargs, error in (("import sys,time; sys.stdin.buffer.read(); time.sleep(5)", {"timeout": .1}, "probe deadline exceeded"),
                                       ("import sys; sys.stdin.buffer.read(); sys.stdout.buffer.write(b'x'*100000); sys.stdout.flush()", {"limit": 16}, "output limit reached")):
                record, stdout, _ = research.run_probe([sys.executable, "-c", body], root, {}, root, "bounded", memory_only=True, input_bytes=b"a", **kwargs)
                self.assertEqual(record["error"], error)
                self.assertTrue(record["stdin_written"])
                self.assertLess(record["elapsed_seconds"], 3)
                self.assertLessEqual(len(stdout), kwargs.get("limit", 1 << 20))

    def test_response_is_confirmed_before_eof_even_for_delayed_handler(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            payload = b'{"id":"one","type":"get_commands"}\n'
            body = "import sys,time; from pathlib import Path; line=sys.stdin.buffer.readline(); time.sleep(.05); sys.stdout.buffer.write(line); sys.stdout.flush(); assert sys.stdin.buffer.read()==b''; Path('eof-after-response').write_text('yes')"
            seen = []
            def complete(line):
                self.assertFalse((root / "eof-after-response").exists())
                seen.append(line)
                return line == payload
            record, stdout, stderr = research.run_probe([sys.executable, "-c", body], root, {}, root, "delayed", memory_only=True, input_bytes=payload, input_complete=complete)
            self.assertEqual((record["exit"], record["error"], stdout, stderr, seen), (0, None, payload, b"", [payload]))
            self.assertTrue(record["stdin_response_confirmed"])
            self.assertEqual((root / "eof-after-response").read_text(), "yes")

    def test_response_refusal_exception_missing_frame_and_deadline_fail_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            def refused(_): raise RuntimeError("exact frame refused")
            for body, predicate, error in (
                    ("import sys; sys.stdin.buffer.readline(); print('wrong', flush=True); sys.stdin.buffer.read()", lambda _: False, "probe input response refused"),
                    ("import sys; sys.stdin.buffer.readline(); print('wrong', flush=True); sys.stdin.buffer.read()", refused, "probe input response predicate raised RuntimeError"),
                    ("import sys; sys.stdin.buffer.readline()", lambda _: True, "probe input response missing"),
                    ("import sys,time; sys.stdin.buffer.readline(); time.sleep(5)", lambda _: True, "probe deadline exceeded")):
                record, _, _ = research.run_probe([sys.executable, "-c", body], root, {}, root, "response", memory_only=True, timeout=.2, input_bytes=b"one\n", input_complete=predicate)
                self.assertEqual(record["error"], error)
                self.assertFalse(record["stdin_response_confirmed"])
                if predicate is refused:
                    self.assertEqual(record["stdin_response_exception"], {"type": "RuntimeError", "message": "exact frame refused"})

    def test_none_keeps_legacy_devnull_and_result_schema(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            record, stdout, stderr = research.run_probe([sys.executable, "-c", "import sys; assert sys.stdin.buffer.read()==b''; print('legacy')"], root, {}, root, "legacy", memory_only=True)
            self.assertEqual((record["exit"], record["error"], stdout, stderr), (0, None, b"legacy\n", b""))
            self.assertFalse(any(key.startswith("stdin_") for key in record))


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
