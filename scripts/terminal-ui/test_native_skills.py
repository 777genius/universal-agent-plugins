"""Stream and inert owned-process controls for the keyboard driver; no agent runs."""
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import sys
from tempfile import TemporaryDirectory
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import native_skills as n


def board(left, right, redraw=True):
    prefix = b"\r\x1b[2A" if redraw else b""
    return prefix + (
        "\r\x1b[2K  Claude Code   " + left + "\r\n"
        "\r\x1b[2K  OpenAI Codex  " + right + "\r\n\r").encode()


QUEUED = board("· 0/2 queued", "· 0/2 queued", redraw=False)
PROGRESS = board("● 0/2 preparing", "· 0/2 queued")
FINISHED = board("✓ 2/2 installed", "✓ 2/2 installed")
VERIFIED = b"\r\nSkills installation succeeded\r\nSkills installation verified.\r\n"


class StreamSession:
    def __init__(self, before, chunks):
        self.raw = bytearray(before)
        self.screen = n.Screen(35, 120)
        self.screen.feed(before)
        self.chunks = list(chunks)
        self.timeout = 5
        self.events, self.frames = [], {}
        self.process = SimpleNamespace(poll=lambda: None if self.chunks else 0)

    def pump(self, _timeout=0.1):
        if not self.chunks:
            return False
        chunk = self.chunks.pop(0)
        self.raw.extend(chunk)
        self.screen.feed(chunk)
        return bool(chunk)

    def frame(self, label):
        self.frames[label] = self.screen.snapshot()
        self.events.append({"event": label, "offset": len(self.raw)})


class NativeSkillsProgressTests(unittest.TestCase):
    def test_combined_separate_and_fragmented_reads_preserve_the_progress_frame(self):
        before = b"review confirmed\r\n"
        transcript = QUEUED + PROGRESS + FINISHED + VERIFIED
        layouts = ([transcript], [QUEUED, PROGRESS, FINISHED, VERIFIED],
                   [bytes([value]) for value in transcript])
        frames, offsets = [], []
        for chunks in layouts:
            with self.subTest(reads=len(chunks)):
                session = StreamSession(before, chunks)
                progress_at = n.wait_install_progress(session, len(before))
                frames.append(session.frames["install-progress"])
                offsets.append(progress_at)
                self.assertEqual(session.events[-1]["offset"], progress_at)
                self.assertIn("Claude Code", frames[-1])
                self.assertIn("OpenAI Codex", frames[-1])
                self.assertIn("0/2 preparing", frames[-1])
                self.assertNotIn("Skills installation verified.", frames[-1])
                while session.chunks:
                    session.pump()
                self.assertEqual(bytes(session.raw), before + transcript)
                self.assertIn("Skills installation verified.", session.screen.snapshot())
                self.assertNotIn("preparing", session.screen.snapshot())
        self.assertEqual(frames, [frames[0]] * len(layouts))
        self.assertEqual(offsets, [len(before + QUEUED + PROGRESS) - 1] * len(layouts))

    def test_completion_before_progress_or_without_progress_is_rejected(self):
        for transcript in (VERIFIED + PROGRESS, QUEUED + FINISHED + VERIFIED):
            with self.subTest(transcript=transcript):
                session = StreamSession(b"", [transcript])
                with self.assertRaisesRegex(AssertionError, "verified before"):
                    n.wait_install_progress(session, 0)
                self.assertNotIn("install-progress", session.frames)

    def test_preconfirmation_progress_cannot_satisfy_the_install(self):
        before = QUEUED + PROGRESS
        session = StreamSession(before, [FINISHED + VERIFIED])
        with self.assertRaisesRegex(AssertionError, "verified before"):
            n.wait_install_progress(session, len(before))
        self.assertNotIn("install-progress", session.frames)

    def test_both_rows_must_belong_to_one_postconfirmation_frame(self):
        first_row = PROGRESS[:PROGRESS.index(b"\n") + 1]
        old_second_row = "\r\x1b[2K  OpenAI Codex  · 0/2 queued\r\n".encode()
        for transcript in (first_row + VERIFIED, first_row + first_row + VERIFIED,
                           old_second_row + first_row + VERIFIED):
            with self.subTest(transcript=transcript):
                session = StreamSession(b"", [transcript])
                with self.assertRaisesRegex(AssertionError, "verified before"):
                    n.wait_install_progress(session, 0)
                self.assertNotIn("install-progress", session.frames)

    def test_exited_stream_without_a_complete_progress_frame_is_rejected(self):
        session = StreamSession(b"", [QUEUED + FINISHED])
        with self.assertRaisesRegex(AssertionError, "exited before"):
            n.wait_install_progress(session, 0)

    def test_transcript_limit_and_confirmation_boundary_remain_strict(self):
        session = StreamSession(b"", [b"x" * (n.LIMIT + 1)])
        with self.assertRaisesRegex(AssertionError, "exceeded 1 MiB"):
            n.wait_install_progress(session, 0)
        for offset in (-1, 1):
            with self.subTest(offset=offset):
                with self.assertRaisesRegex(AssertionError, "invalid confirmation"):
                    n.wait_install_progress(StreamSession(b"", []), offset)

    def test_failed_frame_capture_restores_the_latest_screen(self):
        session = StreamSession(b"", [QUEUED + PROGRESS + FINISHED + VERIFIED])

        def fail_frame(_label):
            raise RuntimeError("capture failed")

        session.frame = fail_frame
        with self.assertRaisesRegex(RuntimeError, "capture failed"):
            n.wait_install_progress(session, 0)
        self.assertIn("Skills installation verified.", session.screen.snapshot())



class NativeSkillsFailureControls(unittest.TestCase):
    def test_exit_visible_after_empty_read_drains_queued_progress_with_same_deadline(self):
        transcript = QUEUED + PROGRESS + FINISHED + VERIFIED
        session = StreamSession(b"", [b"", transcript])
        session.process.poll = lambda: 0
        reads = []
        pump = session.pump

        def recorded(timeout=0.1):
            reads.append(timeout)
            return pump(timeout)

        session.pump = recorded
        n.wait_install_progress(session, 0)
        self.assertIn("0/2 preparing", session.frames["install-progress"])
        self.assertIn(0, reads)
        self.assertEqual(bytes(session.raw), transcript)

        expired = StreamSession(b"", [b"", transcript])
        expired.process.poll = lambda: 0
        with patch.object(n.time, "monotonic", side_effect=[0, 0, 5]):
            with self.assertRaisesRegex(AssertionError, "exited before"):
                n.wait_install_progress(expired, 0)
        self.assertEqual(expired.chunks, [transcript])
        self.assertNotIn("install-progress", expired.frames)

    def test_missing_events_or_failed_close_always_retains_a_failed_report(self):
        import hashlib
        for closed, contents, expected in (
                (True, None, False), (True, "invalid-json", False),
                (True, "[]", False), (False, "valid", False), (True, "valid", True)):
            with self.subTest(closed=closed, contents=contents), TemporaryDirectory() as temporary:
                root = Path(temporary)
                evidence = root / "evidence"
                evidence.mkdir()
                events = {"exit": 0, "forced_cleanup": False, "restoration": {
                    key: True for key in ("termios_equal", "cursor_restored", "next_line_read", "echo")}}
                if contents is not None:
                    (evidence / "events.json").write_text(json.dumps(events) if contents == "valid" else contents)
                raw = b"actual retained terminal bytes\n"
                report = {"status": "passed", "verified_result": True}
                n.preserve(SimpleNamespace(raw=raw), report, root, evidence, closed=closed)
                saved = json.loads((root / "native-terminal-skills.json").read_text())
                self.assertIs(saved["closure_verified"], expected)
                self.assertEqual(saved["status"], "passed" if expected else "failed")
                self.assertEqual((root / "native-terminal-skills.log").read_bytes(), raw)
                self.assertEqual(saved["transcript"]["sha256"], hashlib.sha256(raw).hexdigest())
                if contents != "valid":
                    self.assertIn("terminal_evidence_error", saved)

    def test_failure_screen_redacts_exact_and_wrapped_private_prefixes(self):
        project = Path("/private/fixture[1]/project")
        home = "/private/home[2]"
        wrapped = str(project)[:12] + "\r\n  │" + str(project)[12:]
        raw = ("Choose Skills to install\r\n" + str(project) + "\r\n" +
               str(project.parent) + "\r\n" + home + "\r\n" + wrapped +
               "\r\n[ ] 1. docx · skills/docx\r\nenter submit\r\n").encode()
        session = StreamSession(raw, [])
        session.fixture = SimpleNamespace(project=project, env={"HOME": home})
        view = n.failure_screen(session)
        self.assertNotIn("/private/", view)
        self.assertNotIn("fixture[1]", view)
        self.assertNotIn("home[2]", view)
        self.assertEqual(view.count("<project>"), 2)
        self.assertIn("<fixture>", view)
        self.assertIn("<home>", view)
        self.assertIn("docx · skills/docx", view)
        self.assertIn("enter submit", view)
        self.assertEqual(bytes(session.raw), raw)


class NativeSkillsOwnerControls(unittest.TestCase):
    @unittest.skipUnless(hasattr(signal, "pthread_sigmask"), "POSIX signal masking is required")
    def test_interrupt_does_not_split_a_status_response_or_the_next_rpc(self):
        code = r"""
import json, os, signal, socket, sys
sys.path.insert(0, sys.argv[1])
import native_skills as n
number = getattr(signal, sys.argv[2])
left, right = socket.socketpair()
left.settimeout(2)
right.settimeout(2)
peer = os.fork()
if peer == 0:
    left.close()
    wire = right.makefile("rwb", buffering=0)
    status = 1
    try:
        assert json.loads(wire.readline())["op"] == "first"
        wire.write(b'{"result":')
        os.kill(os.getppid(), number)
        wire.write(b'1}\n')
        assert json.loads(wire.readline())["op"] == "second"
        wire.write(b'{"result":2}\n')
        status = 0
    finally:
        wire.close()
        right.close()
        os._exit(status)
right.close()
session = n.NativeSession.__new__(n.NativeSession)
session.channel = left.makefile("rwb", buffering=0)
def interrupt(_number, _frame):
    raise KeyboardInterrupt("owned interrupt")
signal.signal(number, interrupt)
try:
    interrupted = False
    try:
        session.rpc(op="first")
    except KeyboardInterrupt:
        interrupted = True
    assert interrupted
    assert session.rpc(op="second") == 2
finally:
    session.channel.close()
    left.close()
    _, status = os.waitpid(peer, 0)
assert status == 0, status
print("complete response consumed; next RPC aligned")
"""
        for number in ("SIGINT", "SIGTERM"):
            with self.subTest(signal=number), TemporaryDirectory() as temporary:
                completed = subprocess.run(
                    [sys.executable, "-I", "-c", code, str(Path(n.__file__).parent), number],
                    env={"PATH": os.defpath, "HOME": temporary}, capture_output=True,
                    text=True, timeout=10, check=False)
                self.assertEqual(completed.returncode, 0, completed.stderr)
                self.assertEqual(completed.stdout, "complete response consumed; next RPC aligned\n")

    @unittest.skipUnless(sys.platform == "linux", "native keyboard EOF fallback is a Linux contract")
    def test_owner_eof_closes_an_already_live_owned_descendant(self):
        # The assertion precedes the fallback kill below: cleanup cannot supply
        # the observed EOF. No installer, agent, model, or paid service runs.
        descendant = ("import socket,sys,time; "
                      "connection=socket.socket(socket.AF_UNIX); connection.connect(sys.argv[1]); "
                      "connection.sendall(b'live\\n'); time.sleep(30)")
        job = ("import subprocess,sys,time; "
               "subprocess.Popen([sys.executable,'-I','-c',sys.argv[1],sys.argv[2]]); time.sleep(30)")
        with TemporaryDirectory() as temporary, socket.socket(socket.AF_UNIX) as listener:
            root = Path(temporary)
            project, evidence = root / "project", root / "evidence"
            project.mkdir()
            evidence.mkdir()
            address = str(root / "liveness.sock")
            listener.bind(address)
            listener.listen(1)
            listener.settimeout(3)
            session = n.NativeSession(
                [sys.executable, "-I", "-c", job, descendant, address],
                SimpleNamespace(project=project, env={"PATH": os.defpath, "HOME": str(root)}),
                evidence, timeout=3)
            try:
                connection, _ = listener.accept()
                with connection:
                    connection.settimeout(3)
                    with connection.makefile("rb", buffering=0) as wire:
                        self.assertEqual(wire.readline(64), b"live\n")
                    self.assertIsNone(session.process.poll())
                    connection.settimeout(0.05)
                    with self.assertRaises(socket.timeout):
                        connection.recv(1)
                    # This is the same status-channel EOF left by a hard-killed
                    # driver. The persistent owner must close its live jobs.
                    session.channel.close()
                    session.control.close()
                    self.assertEqual(session.owner.wait(timeout=3), 0)
                    connection.settimeout(3)
                    self.assertEqual(connection.recv(1), b"")
            finally:
                try:
                    os.killpg(session.process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                session.channel.close()
                session.control.close()
                try:
                    session.owner.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    session.owner.kill()
                    session.owner.wait(timeout=2)
                finally:
                    session.owner.stderr.close()
                    os.close(session.master)
                    os.close(session.slave)


if __name__ == "__main__":
    unittest.main()
