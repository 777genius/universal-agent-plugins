"""Synthetic stream controls for the genuine keyboard driver; no agent runs."""
from types import SimpleNamespace
import unittest

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


if __name__ == "__main__":
    unittest.main()
