"""Stream and inert owned-process controls for the keyboard driver; no agent runs."""
from contextlib import ExitStack
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
ACKNOWLEDGED = b"\r\nSkills installation confirmed.\r\n"


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
    def test_queued_progress_before_cli_acknowledgment_is_not_postconfirmation(self):
        for chunks in ([QUEUED + PROGRESS + ACKNOWLEDGED + FINISHED + VERIFIED],
                       [QUEUED + PROGRESS, ACKNOWLEDGED + FINISHED + VERIFIED]):
            with self.subTest(reads=len(chunks)):
                session = StreamSession(b"review\r\n", chunks)
                with self.assertRaisesRegex(AssertionError, "before CLI confirmation"):
                    n.wait_install_progress(session, len(session.raw))
                self.assertNotIn("install-progress", session.frames)

    def test_cli_acknowledgment_must_be_fresh_exact_and_once(self):
        for before, after, error in (
                (ACKNOWLEDGED, QUEUED + PROGRESS, "before CLI confirmation"),
                (b"", b"not " + ACKNOWLEDGED.lstrip() + QUEUED + PROGRESS, "before CLI confirmation"),
                (b"Skills installation ", b"confirmed.\r\n" + QUEUED + PROGRESS, "before CLI confirmation"),
                (b"", ACKNOWLEDGED + ACKNOWLEDGED + QUEUED + PROGRESS, "duplicate CLI confirmation")):
            with self.subTest(before=before, after=after):
                session = StreamSession(before, [after])
                with self.assertRaisesRegex(AssertionError, error):
                    n.wait_install_progress(session, len(before))
                self.assertNotIn("install-progress", session.frames)

    def test_combined_separate_and_fragmented_reads_preserve_the_progress_frame(self):
        before = b"review confirmed\r\n"
        transcript = ACKNOWLEDGED + QUEUED + PROGRESS + FINISHED + VERIFIED
        layouts = ([transcript], [ACKNOWLEDGED, QUEUED, PROGRESS, FINISHED, VERIFIED],
                   [bytes([value]) for value in transcript])
        frames, offsets = [], []
        for chunks in layouts:
            with self.subTest(reads=len(chunks)):
                session = StreamSession(before, chunks)
                progress_at = n.wait_install_progress(session, len(before))
                frames.append(session.frames["install-progress"])
                offsets.append(progress_at)
                self.assertEqual(session.events[-1]["offset"], progress_at)
                self.assertEqual(session.events[0], {"event": "confirmation-accepted",
                                                     "offset": len(before + ACKNOWLEDGED)})
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
        self.assertEqual(offsets, [len(before + ACKNOWLEDGED + QUEUED + PROGRESS) - 1] * len(layouts))

    def test_completion_before_progress_or_without_progress_is_rejected(self):
        for transcript in (VERIFIED + PROGRESS, QUEUED + FINISHED + VERIFIED):
            with self.subTest(transcript=transcript):
                session = StreamSession(b"", [ACKNOWLEDGED + transcript])
                with self.assertRaisesRegex(AssertionError, "verified before"):
                    n.wait_install_progress(session, 0)
                self.assertNotIn("install-progress", session.frames)

    def test_preconfirmation_progress_cannot_satisfy_the_install(self):
        before = QUEUED + PROGRESS
        session = StreamSession(before, [ACKNOWLEDGED + FINISHED + VERIFIED])
        with self.assertRaisesRegex(AssertionError, "verified before"):
            n.wait_install_progress(session, len(before))
        self.assertNotIn("install-progress", session.frames)

    def test_both_rows_must_belong_to_one_postconfirmation_frame(self):
        first_row = PROGRESS[:PROGRESS.index(b"\n") + 1]
        old_second_row = "\r\x1b[2K  OpenAI Codex  · 0/2 queued\r\n".encode()
        for transcript in (first_row + VERIFIED, first_row + first_row + VERIFIED,
                           old_second_row + first_row + VERIFIED):
            with self.subTest(transcript=transcript):
                session = StreamSession(b"", [ACKNOWLEDGED + transcript])
                with self.assertRaisesRegex(AssertionError, "verified before"):
                    n.wait_install_progress(session, 0)
                self.assertNotIn("install-progress", session.frames)

    def test_exited_stream_without_a_complete_progress_frame_is_rejected(self):
        session = StreamSession(b"", [ACKNOWLEDGED + QUEUED + FINISHED])
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
        session = StreamSession(b"", [ACKNOWLEDGED + QUEUED + PROGRESS + FINISHED + VERIFIED])

        def fail_frame(_label):
            raise RuntimeError("capture failed")

        session.frame = fail_frame
        with self.assertRaisesRegex(RuntimeError, "capture failed"):
            n.wait_install_progress(session, 0)
        self.assertIn("Skills installation verified.", session.screen.snapshot())




class NativeSkillsScreenControls(unittest.TestCase):
    def test_native_root_snapshot_detects_same_content_and_dangling_symlinks(self):
        for populated in (False, True):
            with self.subTest(populated=populated), TemporaryDirectory() as temporary:
                root = Path(temporary) / "skills"
                root.mkdir()
                if populated:
                    (root / "foreign.md").write_text("unchanged foreign bytes")
                before = n.hashes(root)
                foreign = root.with_name("foreign")
                root.rename(foreign)
                root.symlink_to(foreign, target_is_directory=True)
                self.assertNotEqual(n.hashes(root), before)
                with patch.object(n, "directory_hashes", side_effect=AssertionError("followed root link")):
                    self.assertEqual(n.hashes(root), {".": "link:" + str(foreign)})
                self.assertEqual(sorted(p.name for p in foreign.iterdir()),
                                 ["foreign.md"] if populated else [])
                root.unlink()
                absent = n.hashes(root)
                root.symlink_to(root.with_name("missing"), target_is_directory=True)
                self.assertNotEqual(n.hashes(root), absent)
                root.unlink()
                root.mkdir()
                self.assertNotEqual(n.hashes(root), absent)
                root.rmdir()
                root.write_text("not a Skill directory")
                with self.assertRaisesRegex(AssertionError, "not a directory"):
                    n.hashes(root)

    def test_expanded_review_requires_a_destination_inside_the_selected_project(self):
        project = Path("/private/fixture/project")
        summary = "Install these Skills? d show paths\n2 Skills · 2 agents · 4 bindings\n" \
                  "install docx\ninstall frontend-design\n"
        warning = 'Installation may trust the selected project "' + str(project) + \
                  '" in Claude Code. This preference remains after removal.'
        for destination, accepted in ((str(project), True), (str(project) + "/.claude/skills/a", True),
                                      (str(project) + "-old/.claude/skills/a", False),
                                      (str(project) + "2/.agents/skills/docx", False),
                                      ("/other" + str(project) + "/.claude/skills/a", False)):
            with self.subTest(destination=destination):
                # The details layout wraps paths across bordered physical rows.
                wrapped = destination[:19] + "\n│ " + destination[19:]
                views = iter((summary, warning, "d hide paths\ninstall docx → Claude Code (" +
                              wrapped + ")\n" + warning))

                def wait(_session, _label, predicate):
                    view = next(views)
                    n.check(predicate(view), "review predicate refused destination")
                    return view

                with patch.object(n, "wait_screen", side_effect=wait), patch.object(n, "key"):
                    if accepted:
                        n.confirm_review(object(), project)
                    else:
                        with self.assertRaisesRegex(AssertionError, "refused destination"):
                            n.confirm_review(object(), project)

    def test_tab_redraw_preserves_strict_skill_and_target_selection_markers(self):
        # Pinned Bubble Tea/Ultraviolet skips unchanged prefixes with HT/CBT.
        # These are parser controls, not an invented native-agent transcript.
        for label, parse, expected in (
                ("7. docx · skills/docx", n.skill_rows, "docx · skills/docx"),
                ("8. frontend-design · skills/frontend-design", n.skill_rows,
                 "frontend-design · skills/frontend-design"),
                ("Claude Code (claude)", n.target_rows, "claude"),
                ("OpenAI Codex (codex)", n.target_rows, "codex")):
            with self.subTest(label=label):
                prefix = "┃ > [ ] "
                raw = (prefix + "old option\r\t" + label + "\x1b[K").encode()
                session = StreamSession(raw, [])
                self.assertEqual(n.skill_rows(label), [])
                self.assertEqual(n.target_rows(label), [])
                self.assertEqual(parse(n.screen(session)), [(" ", expected)])
                # Move back through the prefix to the checkbox cell, then
                # repaint the label by tabbing over the unchanged marker.
                session.raw.extend(("\r\t\x1b[Z\x1b[5C•\r\t" + label + "\x1b[K").encode())
                self.assertEqual(parse(n.screen(session)), [("•", expected)])
                session.raw.extend(("\r\t\x1b[Z\x1b[5C \r\t" + label + "\x1b[K").encode())
                self.assertEqual(parse(n.screen(session)), [(" ", expected)])

    def test_tab_stops_keep_exact_eight_column_and_edge_semantics(self):
        for start, sequence, expected in (
                (0, b"\t", 8), (7, b"\t", 8), (8, b"\t", 16),
                (16, b"\t", 19), (19, b"\t", 19), (20, b"\t", 19),
                (0, b"\x1b[Z", 0), (8, b"\x1b[Z", 0),
                (9, b"\x1b[Z", 8), (19, b"\x1b[0Z", 16), (19, b"\x1b[2Z", 8),
                (19, b"\x1b[9Z", 0)):
            with self.subTest(start=start, sequence=sequence):
                screen = n.Screen(2, 20)
                screen.col = start
                for value in sequence:
                    screen.feed(bytes([value]))
                self.assertEqual((screen.row, screen.col), (0, expected))
                self.assertEqual(screen.snapshot(), "\n")
                self.assertEqual(screen.unknown, set())
        for sequence, expected in ((b"\t", 16), (b"\x1b[Z", 8)):
            with self.subTest(pending_wrap=sequence):
                screen = n.Screen(2, 17)
                screen.feed(b"x" * 17 + sequence + b"!")
                self.assertEqual((screen.row, screen.col), (0, expected + 1))
                self.assertEqual(screen.grid[0][expected], "!")
                self.assertEqual(screen.grid[1], [" "] * 17)


class NativeSkillsFailureControls(unittest.TestCase):
    def test_oversized_structured_evidence_keeps_a_compact_failed_report(self):
        for oversized, closed in ((kind, closed) for kind in ("frame", "events", "preconfirmation", "error")
                                  for closed in (False, True)):
            with self.subTest(oversized=oversized, closed=closed), TemporaryDirectory() as temporary:
                root = Path(temporary)
                evidence = root / "evidence"
                evidence.mkdir()
                events = {"exit": 0, "forced_cleanup": False, "restoration": {
                    key: True for key in ("termios_equal", "cursor_restored", "next_line_read", "echo")}}
                report = {"schema_version": 1, "status": "passed", "phase": "review",
                          "error": "original review failure", "verified_result": False}
                if not closed:
                    report["closure_error"] = "original owner failure"
                if oversized == "frame":
                    (evidence / "large.frame.txt").write_text("x" * n.LIMIT)
                    (evidence / "large.frame.svg").write_text("<svg/>")
                elif oversized == "events":
                    events["events"] = ["x" * n.LIMIT]
                else:
                    report[oversized] = "x" * n.LIMIT
                (evidence / "events.json").write_text(json.dumps(events))
                n.preserve(SimpleNamespace(raw=b"retained transcript"), report, root, evidence, closed=closed)
                body = (root / "native-terminal-skills.json").read_bytes()
                saved = json.loads(body)
                self.assertLessEqual(len(body), n.LIMIT)
                self.assertEqual(saved["status"], "failed")
                self.assertEqual(report["status"], "failed")
                self.assertEqual(saved["phase"], "review")
                self.assertIs(saved["closure_verified"], closed)
                if not closed:
                    self.assertEqual(saved["closure_error"], "original owner failure")
                self.assertTrue(saved["structured_evidence_truncated"])
                self.assertGreater(saved["structured_evidence_bytes"], n.LIMIT)
                self.assertFalse(saved["transcript"]["truncated"])
                self.assertEqual(saved["terminal"]["restoration"], events["restoration"])
                if oversized == "error":
                    self.assertTrue(saved["error_truncated"])
                    self.assertTrue(saved["error"].startswith("x" * 100))
                else:
                    self.assertEqual(saved["error"], "original review failure")
                self.assertEqual((root / "native-terminal-skills.log").read_bytes(), b"retained transcript")

    def test_exit_visible_after_empty_read_drains_queued_progress_with_same_deadline(self):
        transcript = ACKNOWLEDGED + QUEUED + PROGRESS + FINISHED + VERIFIED
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


    def test_accepted_final_drain_is_parsed_without_another_clock_or_read(self):
        transcript = ACKNOWLEDGED + QUEUED + PROGRESS + FINISHED + VERIFIED
        session = StreamSession(b"", [b"", transcript])
        session.process.poll = lambda: 0
        with patch.object(session, "pump", wraps=session.pump) as pump, \
                patch.object(n.time, "monotonic", side_effect=[0, 0, 4, 5]) as clock:
            n.wait_install_progress(session, 0)
        self.assertEqual([call.args for call in pump.call_args_list], [(0.1,), (0,)])
        self.assertEqual(clock.call_count, 3)
        self.assertIn("0/2 preparing", session.frames["install-progress"])
        self.assertEqual(bytes(session.raw), transcript)

        # An accepted but incomplete read cannot extend the same deadline.
        incomplete = StreamSession(b"", [b"", ACKNOWLEDGED + QUEUED])
        incomplete.process.poll = lambda: 0
        with patch.object(n.time, "monotonic", side_effect=[0, 0, 4, 5]):
            with self.assertRaisesRegex(AssertionError, "timeout waiting"):
                n.wait_install_progress(incomplete, 0)
        self.assertNotIn("install-progress", incomplete.frames)

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


    def test_partial_frame_keeps_structured_failure_and_complete_frames(self):
        for partial in (False, True):
            with self.subTest(partial=partial), TemporaryDirectory() as temporary:
                root = Path(temporary)
                evidence = root / "evidence"
                evidence.mkdir()
                events = {"exit": 0, "forced_cleanup": False, "restoration": {
                    key: True for key in ("termios_equal", "cursor_restored", "next_line_read", "echo")}}
                (evidence / "events.json").write_text(json.dumps(events))
                (evidence / "complete.frame.txt").write_text("complete text")
                (evidence / "complete.frame.svg").write_text("<svg>complete</svg>")
                if partial:
                    (evidence / "partial.frame.txt").write_text("retained partial text")
                report = {"status": "passed", "verified_result": True}
                n.preserve(SimpleNamespace(raw=b"retained"), report, root, evidence, closed=True)
                saved = json.loads((root / "native-terminal-skills.json").read_text())
                self.assertTrue(saved["closure_verified"])
                self.assertEqual(saved["status"], "failed" if partial else "passed")
                self.assertEqual(saved["frames"][0], {"name": "complete.frame",
                                 "text": "complete text", "svg": "<svg>complete</svg>"})
                self.assertEqual(len(saved["frames"]), 2 if partial else 1)
                if partial:
                    frame = saved["frames"][1]
                    self.assertEqual(frame["text"], "retained partial text")
                    self.assertNotIn("svg", frame)
                    self.assertIn("FileNotFoundError", frame["svg_error"])
                    self.assertLessEqual(len(frame["svg_error"]), 512)

    def test_close_capture_overflow_cannot_exit_success_with_verified_closure(self):
        for overflow in (False, True):
            with self.subTest(overflow=overflow), TemporaryDirectory() as temporary:
                root = Path(temporary)
                project, installer = root / "project", root / "installer"
                project.mkdir()
                installer.write_text("inert fixture; never executed")
                evidence = root / "terminal-ui"
                session = SimpleNamespace(
                    raw=bytearray(b"verified before close"), events=[],
                    owner=SimpleNamespace(returncode=0),
                    wait=lambda *a, **k: None, finish=lambda *a: None,
                    frame=lambda *a: None)

                def close():
                    if overflow:
                        session.raw.extend(b"x" * n.LIMIT)
                    events = {"exit": 0, "forced_cleanup": False, "restoration": {
                        key: True for key in ("termios_equal", "cursor_restored", "next_line_read", "echo")}}
                    (evidence / "events.json").write_text(json.dumps(events))

                session.close = close
                args = SimpleNamespace(installer=str(installer), root=str(root),
                                       project=str(project), source="inert-fixture")
                with ExitStack() as stack:
                    stack.enter_context(patch.object(n.sys, "platform", "linux"))
                    stack.enter_context(patch.dict(n.os.environ, {
                        "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1",
                        "GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted"}, clear=True))
                    stack.enter_context(patch.object(n.argparse.ArgumentParser, "parse_args", return_value=args))
                    stack.enter_context(patch.object(n, "NativeSession", return_value=session))
                    stack.enter_context(patch.object(n.signal, "signal"))
                    for name in ("choose_skills", "choose_targets", "confirm_review", "key",
                                 "verified_result", "screen"):
                        stack.enter_context(patch.object(n, name, return_value=None))
                    stack.enter_context(patch.object(n, "unchanged_before_confirmation", return_value={}))
                    stack.enter_context(patch.object(n, "wait_install_progress", return_value=0))
                    if overflow:
                        with self.assertRaisesRegex(AssertionError, "evidence failed"):
                            n.main()
                    else:
                        n.main()
                saved = json.loads((root / "native-terminal-skills.json").read_text())
                self.assertTrue(saved["closure_verified"])
                self.assertEqual(saved["status"], "failed" if overflow else "passed")
                self.assertIs(saved["transcript"]["truncated"], overflow)
                self.assertLessEqual((root / "native-terminal-skills.log").stat().st_size, n.LIMIT)

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
        with (TemporaryDirectory() as temporary,
              TemporaryDirectory(prefix="uap-sock-", dir="/tmp") as socket_directory,
              socket.socket(socket.AF_UNIX) as listener):
            root = Path(temporary) / ("long-fixture-" * 10)
            root.mkdir()
            project, evidence = root / "project", root / "evidence"
            project.mkdir()
            evidence.mkdir()
            address = str(Path(socket_directory) / "liveness.sock")
            self.assertGreater(len(os.fsencode(root / "liveness.sock")), 108)
            self.assertLess(len(os.fsencode(address)), 108)
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
