"""Inert controls for the genuine Directory keyboard driver; no agent runs."""
from contextlib import ExitStack
import json
from pathlib import Path
from tempfile import TemporaryDirectory
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import native_plugin as n
from test_native_skills import StreamSession


ACK = b"\r\nChanges confirmed.\r\n"
ACTIVITY = "\r\x1b[2K⠋ Applying changes · 0s".encode()
STOP = b"\r\x1b[2K"
RESULT = b"install selected-id/claude: completed \r\ninstall selected-id/codex: completed \r\n"
REVIEW = "install context7/claude: not installed\ninstall context7/codex: not installed\n" + n.QUESTION


def session(before, chunks):
    value = StreamSession(before, chunks)
    value.fixture = SimpleNamespace(project=Path("/inert/fixture/project"), env={})
    return value


def profile(root):
    env = {"CODEX_HOME": str(root / "home" / ".codex"),
           "CLAUDE_CONFIG_DIR": str(root / "claude-config"),
           "AGENTPLUGINS_HOME": str(root / "installer-state")}
    for name, value in env.items():
        path = Path(value)
        path.mkdir(parents=True)
        if name != "AGENTPLUGINS_HOME":
            skill = path / "skills" / "foreign-site-plugin"
            skill.mkdir(parents=True)
            (skill / "SKILL.md").write_text("foreign bytes must remain unchanged")
    (Path(env["CODEX_HOME"]) / "config.toml").write_text("[analytics]\nenabled = false\n")
    return env


def closure(evidence):
    (evidence / "events.json").write_text(json.dumps({
        "exit": 0, "forced_cleanup": False, "restoration": {
            key: True for key in ("termios_equal", "cursor_restored", "next_line_read", "echo")}}))


class NativePluginProgressTests(unittest.TestCase):
    def test_combined_separate_and_fragmented_reads_retain_visible_activity(self):
        before = b"review\r\n"
        transcript = ACK + ACTIVITY + STOP + RESULT
        frames, offsets = [], []
        for chunks in ([transcript], [ACK, ACTIVITY, STOP, RESULT],
                       [bytes([value]) for value in transcript]):
            with self.subTest(reads=len(chunks)):
                value = session(before, chunks)
                end = n.wait_progress(value, len(before))
                frames.append(value.frames["plugin-progress"])
                offsets.append(end)
                self.assertIn("⠋ Applying changes · 0s", frames[-1])
                self.assertNotIn("completed", frames[-1])
                self.assertEqual(value.events[0], {"event": "confirmation-accepted", "offset": len(before + ACK)})
                self.assertEqual(value.events[-1]["offset"], end)
                while value.chunks:
                    value.pump()
                self.assertEqual(n.completed_result(value.raw, end)["installation_id"], "selected-id")
                self.assertEqual(bytes(value.raw), before + transcript)
        self.assertEqual(offsets, [len(before + ACK + ACTIVITY)] * 3)
        self.assertEqual(frames, [frames[0]] * 3)

    def test_colored_activity_uses_the_actual_carriage_return_frame_boundary(self):
        activity = b"\r\x1b[2K\x1b[35m" + "⠙".encode() + \
                   b"\x1b[0m \x1b[1mApplying changes\x1b[0m " + "· 1s".encode()
        value = session(b"", [ACK + activity + STOP + RESULT])
        self.assertEqual(n.wait_progress(value, 0), len(ACK + activity))
        self.assertIn("⠙ Applying changes · 1s", value.frames["plugin-progress"])

    def test_acknowledgment_must_be_exact_fresh_once_and_before_activity(self):
        for before, transcript, error in (
                (ACK, ACTIVITY + STOP + RESULT, "before CLI confirmation"),
                (b"", ACTIVITY + STOP + ACK + RESULT, "before CLI confirmation"),
                (b"", b"not Changes confirmed.\r\n" + ACTIVITY + STOP + RESULT, "before CLI confirmation"),
                (b"Changes ", b"confirmed.\r\n" + ACTIVITY + STOP + RESULT, "before CLI confirmation"),
                (b"", ACK + ACK + ACTIVITY + STOP + RESULT, "duplicate Directory")):
            with self.subTest(before=before, transcript=transcript):
                value = session(before, [transcript])
                with self.assertRaisesRegex(AssertionError, error):
                    n.wait_progress(value, len(before))
                self.assertNotIn("plugin-progress", value.frames)

    def test_completion_without_prior_visible_activity_is_not_progress(self):
        for transcript in (RESULT + ACTIVITY + STOP, RESULT,
                           "Applying changes…\r\n".encode() + RESULT,
                           b"Applying changes\r\n" + RESULT):
            with self.subTest(transcript=transcript):
                value = session(b"", [ACK + transcript])
                with self.assertRaisesRegex(AssertionError, "(completed|before CLI confirmation)"):
                    n.wait_progress(value, 0)
                self.assertNotIn("plugin-progress", value.frames)

    def test_old_activity_and_an_incomplete_new_frame_do_not_satisfy_progress(self):
        before = ACK + ACTIVITY + STOP
        value = session(before, [ACK + RESULT])
        with self.assertRaisesRegex(AssertionError, "completed without"):
            n.wait_progress(value, len(before))
        value = session(b"", [ACK + ACTIVITY])
        with self.assertRaisesRegex(AssertionError, "exited before"):
            n.wait_progress(value, 0)

    def test_accepted_final_drain_is_parsed_with_the_same_deadline(self):
        transcript = ACK + ACTIVITY + STOP + RESULT
        value = session(b"", [b"", transcript])
        value.process.poll = lambda: 0
        with patch.object(value, "pump", wraps=value.pump) as pump, \
                patch.object(n.time, "monotonic", side_effect=[0, 0, 4, 5]) as clock:
            n.wait_progress(value, 0)
        self.assertEqual([call.args for call in pump.call_args_list], [(0.1,), (0,)])
        self.assertEqual(clock.call_count, 3)
        self.assertIn("Applying changes · 0s", value.frames["plugin-progress"])
        expired = session(b"", [b"", transcript])
        expired.process.poll = lambda: 0
        with patch.object(n.time, "monotonic", side_effect=[0, 0, 5]):
            with self.assertRaisesRegex(AssertionError, "exited before"):
                n.wait_progress(expired, 0)
        self.assertEqual(expired.chunks, [transcript])

    def test_transcript_limit_and_confirmation_boundary_remain_strict(self):
        with self.assertRaisesRegex(AssertionError, "exceeded 1 MiB"):
            n.wait_progress(session(b"", [b"x" * (n.LIMIT + 1)]), 0)
        for offset in (-1, 1):
            with self.subTest(offset=offset), self.assertRaisesRegex(AssertionError, "invalid plugin"):
                n.wait_progress(session(b"", []), offset)

    def test_frame_capture_failure_restores_the_latest_screen(self):
        value = session(b"", [ACK + ACTIVITY + STOP + RESULT])
        with patch.object(value, "frame", side_effect=RuntimeError("capture failed")):
            with self.assertRaisesRegex(RuntimeError, "capture failed"):
                n.wait_progress(value, 0)
        self.assertIn("selected-id/codex: completed", value.screen.snapshot())


class NativePluginContractTests(unittest.TestCase):
    def test_arguments_and_review_use_the_ordinary_saved_directory_route(self):
        self.assertEqual(n.arguments(Path("/fixture/installer")), ["/fixture/installer", "add", "context7"])
        for view, accepted in (
                (REVIEW, True), (REVIEW.replace(": not installed", ""), True),
                (REVIEW.replace("[y/N]", "[Y/n]"), False),
                (REVIEW.replace("context7/claude", "other/claude"), False),
                (REVIEW.replace("context7/codex", "context7/claude"), False),
                ("install context7/other: not installed\n" + REVIEW, False)):
            with self.subTest(view=view):
                def wait(_session, _label, predicate):
                    n.check(predicate(view), "review refused")
                    return view
                with patch.object(n, "wait_screen", side_effect=wait):
                    if accepted:
                        self.assertEqual(n.review(session(b"", [])), view)
                    else:
                        with self.assertRaisesRegex(AssertionError, "review refused"):
                            n.review(session(b"", []))
        with patch.object(n, "wait_screen", return_value=REVIEW):
            with self.assertRaisesRegex(AssertionError, "before confirmation"):
                n.review(session(ACK, []))

    def test_result_binds_two_clients_and_one_identity_without_claiming_mcp_ready(self):
        manual = b"  Manual: Claude MCP is not connected\r\n  Next: inspect existing policy\r\n"
        for prefix in (b"", b"\r", STOP, b"\r\n"):
            with self.subTest(prefix=prefix):
                self.assertEqual(n.completed_result(prefix + RESULT + manual, 0), {
                    "installation_id": "selected-id", "manual_or_next_actions": [
                        "Manual: Claude MCP is not connected", "Next: inspect existing policy"]})
        for raw in (b"", RESULT.split(b"\n", 1)[0],
                    RESULT.replace(b"/codex:", b"/claude:"),
                    RESULT.replace(b"selected-id/codex", b"other-id/codex"),
                    RESULT.replace(b"/codex:", b"/other:"),
                    RESULT.replace(b"completed", b"failed", 1), RESULT + RESULT):
            with self.subTest(raw=raw), self.assertRaises(AssertionError):
                n.completed_result(raw, 0)
        for offset in (-1, len(RESULT) + 1):
            with self.subTest(offset=offset), self.assertRaisesRegex(AssertionError, "invalid plugin completion"):
                n.completed_result(RESULT, offset)

    def test_preconfirmation_state_observes_foreign_bytes_config_and_intent(self):
        for mutation in ("skill", "config", "root-link", "plugin-registration", "claude-settings",
                         "claude-trust", "state-v2.json", "uap.toml", "uap.lock"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                env = profile(root)
                before = n.protected_state(root, env)
                self.assertEqual(n.protected_state(root, env), before)
                codex = Path(env["CODEX_HOME"])
                if mutation == "skill":
                    (codex / "skills" / "foreign-site-plugin" / "SKILL.md").write_text("changed")
                elif mutation == "config":
                    (codex / "config.toml").write_text("changed")
                elif mutation == "root-link":
                    (codex / "skills").rename(codex / "foreign-skills")
                    (codex / "skills").symlink_to(codex / "foreign-skills", target_is_directory=True)
                elif mutation == "plugin-registration":
                    (codex / "plugins").mkdir()
                    (codex / "plugins" / "installed_plugins.json").write_text("early registration")
                elif mutation in ("claude-settings", "claude-trust"):
                    name = "settings.json" if mutation == "claude-settings" else ".claude.json"
                    (Path(env["CLAUDE_CONFIG_DIR"]) / name).write_text("early profile write")
                else:
                    (Path(env["AGENTPLUGINS_HOME"]) / mutation).write_text("changed")
                    with self.assertRaisesRegex(AssertionError, "before confirmation"):
                        n.protected_state(root, env)
                    continue
                self.assertNotEqual(n.protected_state(root, env), before)

    def test_shared_preserve_keeps_default_and_plugin_names_separate_and_bounded(self):
        for name in (None, "plugin"):
            with self.subTest(name=name), TemporaryDirectory() as temporary:
                root = Path(temporary)
                evidence = root / "evidence"
                evidence.mkdir()
                closure(evidence)
                (evidence / "large.frame.txt").write_text("x" * n.LIMIT)
                (evidence / "large.frame.svg").write_text("<svg/>")
                selected = name or "skills"
                other = "skills" if name else "plugin"
                sentinel = root / ("native-terminal-" + other + ".json")
                sentinel.write_text("unchanged other report")
                report = {"status": "failed", "phase": "review", "error": "original cause"}
                keywords = {} if name is None else {"name": name}
                n.preserve(SimpleNamespace(raw=b"retained"), report, root, evidence, closed=True, **keywords)
                target = root / ("native-terminal-" + selected + ".json")
                saved = json.loads(target.read_text())
                self.assertLessEqual(target.stat().st_size, n.LIMIT)
                self.assertEqual((saved["status"], saved["phase"], saved["error"]), ("failed", "review", "original cause"))
                self.assertTrue(saved["closure_verified"])
                self.assertTrue(saved["structured_evidence_truncated"])
                self.assertEqual((root / ("native-terminal-" + selected + ".log")).read_bytes(), b"retained")
                self.assertEqual(sentinel.read_text(), "unchanged other report")

    def test_main_requires_successful_close_and_final_nonoverflow_report(self):
        for outcome in ("success", "overflow", "owner-failed"):
            with self.subTest(outcome=outcome), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                project, installer = root / "project", root / "installer"
                project.mkdir()
                installer.write_text("inert fixture; never executed")
                env = profile(root)
                evidence = root / "terminal-plugin-ui"
                value = SimpleNamespace(raw=bytearray(RESULT), events=[],
                                        owner=SimpleNamespace(returncode=1 if outcome == "owner-failed" else 0),
                                        finish=lambda *_: None, frame=lambda *_: None)
                def close():
                    if outcome == "overflow":
                        value.raw.extend(b"x" * n.LIMIT)
                    closure(evidence)
                value.close = close
                args = SimpleNamespace(root=str(root), project=str(project), installer=str(installer))
                with ExitStack() as stack:
                    stack.enter_context(patch.object(n.sys, "platform", "linux"))
                    stack.enter_context(patch.dict(n.os.environ, {**env,
                        "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1", "GITHUB_ACTIONS": "true",
                        "RUNNER_ENVIRONMENT": "github-hosted"}, clear=True))
                    stack.enter_context(patch.object(n.argparse.ArgumentParser, "parse_args", return_value=args))
                    stack.enter_context(patch.object(n, "NativeSession", return_value=value))
                    stack.enter_context(patch.object(n.signal, "signal"))
                    for name in ("choose_targets", "review", "key", "screen"):
                        stack.enter_context(patch.object(n, name, return_value=None))
                    stack.enter_context(patch.object(n, "wait_progress", return_value=0))
                    if outcome == "success":
                        n.main()
                    else:
                        with self.assertRaises(AssertionError):
                            n.main()
                saved = json.loads((root / "native-terminal-plugin.json").read_text())
                self.assertEqual(saved["status"], "passed" if outcome == "success" else "failed")
                self.assertIs(saved["closure_verified"], outcome != "owner-failed")
                self.assertIs(saved["transcript"]["truncated"], outcome == "overflow")
                self.assertLessEqual((root / "native-terminal-plugin.log").stat().st_size, n.LIMIT)
                for key in ("model_request", "authentication_qualified", "native_readiness_qualified"):
                    self.assertIs(saved[key], False)


if __name__ == "__main__":
    unittest.main()
