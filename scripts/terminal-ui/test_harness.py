"""Harness self-tests use synthetic subprocesses; never count as CLI acceptance."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('terminal_harness', Path(__file__).with_name('harness.py'))
h = importlib.util.module_from_spec(spec)
spec.loader.exec_module(h)


class FixtureTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.fixture = h.Fixture(Path(self.tmp.name) / 'fixture')

    def test_environment_is_allowlist(self):
        env = self.fixture.env
        self.assertNotIn('OPENAI_API_KEY', env)
        self.assertNotIn('SSH_AUTH_SOCK', env)
        self.assertEqual(env['PATH'], str(self.fixture.bin))
        for key in ('HOME', 'USERPROFILE', 'CODEX_HOME', 'XDG_CONFIG_HOME', 'AGENTPLUGINS_HOME'):
            self.assertTrue(Path(env[key]).is_relative_to(self.fixture.root))
        self.assertEqual(json.loads((self.fixture.package / 'plugin.json').read_text())['name'], 'pty-synthetic')

    def test_mutation_fence_catches_client_directory_and_state(self):
        self.fixture.unchanged()
        (self.fixture.home / '.cursor' / 'unexpected').mkdir()
        with self.assertRaisesRegex(AssertionError, 'mutated'): self.fixture.unchanged()
        (self.fixture.home / '.cursor' / 'unexpected').rmdir()
        (self.fixture.data / 'state-v2.json').write_text('{}')
        with self.assertRaisesRegex(AssertionError, 'mutated'): self.fixture.unchanged()

    def test_cache_is_allowed_but_journal_is_not(self):
        (self.fixture.data / 'cache').mkdir()
        (self.fixture.data / 'cache' / 'entry').write_text('synthetic')
        self.fixture.unchanged()
        (self.fixture.data / 'operations-v2').mkdir()
        (self.fixture.data / 'operations-v2' / 'entry').write_text('{}')
        with self.assertRaises(AssertionError): self.fixture.unchanged()

    def test_symlink_snapshot_does_not_read_target(self):
        (self.fixture.home / 'link').symlink_to('/nonexistent-synthetic-target')
        self.assertEqual(h.hashes(self.fixture.home)['link'], 'link:/nonexistent-synthetic-target')

    def test_runtime_stub_invocation_rejected(self):
        (self.fixture.root / 'stub.log').write_text('/synthetic/codex --version\n')
        self.fixture.validate_stubs()
        (self.fixture.root / 'stub.log').write_text('/synthetic/codex exec hello\n')
        with self.assertRaises(AssertionError): self.fixture.validate_stubs()


class EmptySelectionTests(unittest.TestCase):
    def run_empty(self, early=b'', late=b''):
        from unittest.mock import Mock
        session = Mock()
        # An earlier transcript marker must not contaminate this branch boundary.
        session.raw = bytearray(b'Apply this plan? old transcript\n')
        def validation(*args, **kwargs):
            session.raw.extend(b'must select at least one\n' + early)
        def finish(expected):
            self.assertEqual(expected, 1)
            session.raw.extend(late)
        session.wait.side_effect = validation
        session.finish.side_effect = finish
        fixture = Mock()
        h.cancel_empty_selection(session, fixture, h.CONFIRM)
        self.assertEqual(fixture.unchanged.call_count, 2)
        session.finish.assert_called_once_with(1)

    def test_valid_cancellation_passes(self):
        self.run_empty()

    def test_early_confirmation_fails(self):
        with self.assertRaisesRegex(AssertionError, 'empty advanced'):
            self.run_empty(early=b'Apply this plan?')

    def test_confirmation_drained_during_cancellation_fails(self):
        with self.assertRaisesRegex(AssertionError, 'empty advanced'):
            self.run_empty(late=b'Apply this plan?')


class ScannerTests(unittest.TestCase):
    def test_verified_binary_is_frozen_before_fixture_copy(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            source = root / 'scanner'
            source.write_bytes(b'synthetic scanner bytes')
            args = argparse.Namespace(scanner_archive=None, scanner_binary=source,
                                      scanner_sha256=h.hashlib.sha256(source.read_bytes()).hexdigest(),
                                      artifacts=root)
            receipt = h.prepare_scanner(args)
            source.write_bytes(b'changed after verification')
            self.assertEqual(args.scanner_path.read_bytes(), b'synthetic scanner bytes')
            self.assertEqual(receipt['executable_sha256'], args.scanner_sha256)
            fixture = h.Fixture(root / 'fixture', args.scanner_path)
            installed, = [p for p in fixture.data.rglob('lintai') if p.is_file()]
            self.assertEqual(installed.read_bytes(), b'synthetic scanner bytes')

    def test_unverified_archive_and_binary_are_rejected(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            archive = root / 'lintai-v0.1.3-x86_64-unknown-linux-gnu.tar.gz'
            archive.write_bytes(b'not the pinned archive')
            args = argparse.Namespace(scanner_archive=archive, scanner_binary=None,
                                      scanner_sha256=None, artifacts=root)
            with self.assertRaisesRegex(AssertionError, 'SHA256'): h.prepare_scanner(args)
            args.scanner_archive = None; args.scanner_binary = archive
            with self.assertRaisesRegex(AssertionError, 'SHA256'): h.prepare_scanner(args)


class ScreenTests(unittest.TestCase):
    def test_fragmented_ansi_and_utf8(self):
        screen = h.Screen(3, 30)
        for chunk in (b'old\r\x1b[', b'2Knew \xc3', b'\xa9\n\rsecond'):
            screen.feed(chunk)
        self.assertEqual(screen.snapshot(), 'new é\nsecond\n')

    def test_cursor_repaint(self):
        screen = h.Screen(3, 30)
        screen.feed(b'first\r\nold\x1b[1A\r\x1b[2Kupdated')
        self.assertEqual(screen.snapshot(), 'updated\nold\n')


@unittest.skipUnless(os.name == 'posix', 'Unix PTY only')
class SessionTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.fixture = h.Fixture(Path(self.tmp.name) / 'fixture')
        self.evidence = Path(self.tmp.name) / 'evidence'
        self.evidence.mkdir()

    def session(self, code, **kwargs):
        s = h.Session([sys.executable, '-I', '-c', code], self.fixture, self.evidence, timeout=1, **kwargs)
        self.addCleanup(s.close)
        return s

    def test_real_pty_semantic_wait_and_restoration(self):
        s = self.session("import sys; print('SELECT_READY',flush=True); "
                         "line=sys.stdin.readline(); print('ANSWER:'+line,flush=True)")
        s.wait('SELECT_READY', 'ready')
        s.send(b'n\n')
        s.finish()
        self.assertTrue(all(s.restoration.values()))
        self.assertTrue((self.evidence / 'ready.frame.txt').exists())

    def test_timeout_is_bounded_and_cleanup_kills_child(self):
        s = self.session("import sys; sys.stdin.readline()")
        with self.assertRaisesRegex(AssertionError, 'timeout'): s.wait('NEVER', 'missing')
        self.assertIsNone(s.process.poll())

    def test_detects_raw_mode_leak(self):
        s = self.session("import tty; tty.setraw(0)")
        # Check before spawning restoration probe; its read can legitimately hang
        # in the broken terminal. finish itself must reject within its bound.
        with self.assertRaisesRegex(AssertionError, 'restoration failed'): s.finish()
        self.assertFalse(s.restoration['termios_equal'])

    def test_held_open_stdin_does_not_force_eof(self):
        s = self.session("print('automation',flush=True)", stdin_pipe=True)
        s.finish()
        self.assertFalse(s.process.stdin.closed)

    def test_owner_survives_job_and_terminal_can_be_reused_twice(self):
        s = self.session("import os; print('SID:'+str(os.getsid(0)),flush=True)")
        s.wait('SID:', 'session-id')
        s.finish()
        self.assertIn('SID:' + str(s.owner.pid), h.clean(s.raw))
        self.assertIsNone(s.owner.poll())
        self.assertEqual(h.termios.tcgetattr(s.slave), s.before)
        s.restore_probe()
        self.assertTrue(all(s.restoration.values()))
        s.close()
        self.assertEqual(s.owner.returncode, 0)
        self.assertTrue(s.channel.closed)
        self.assertEqual(s.control.fileno(), -1)
        with self.assertRaises(OSError): os.fstat(s.slave)
        s.close()  # cleanup is idempotent

    def test_forced_cleanup_reaps_job_owner_and_records_failure(self):
        s = self.session("import sys; print('READY',flush=True); sys.stdin.readline()")
        s.wait('READY', 'ready')
        pid = s.process.pid
        s.close()
        self.assertIsNotNone(s.process.returncode)
        self.assertEqual(s.owner.returncode, 0)
        with self.assertRaises(ProcessLookupError): os.kill(pid, 0)
        events = json.loads((self.evidence / 'events.json').read_text())
        self.assertTrue(events['forced_cleanup'])
        self.assertIsNone(events['restoration'])

    def test_raw_to_plain_lifecycle_and_nonzero_status(self):
        s = self.session("import sys,tty,termios; before=termios.tcgetattr(0); "
                         "tty.setraw(0); print('RAW_READY',flush=True); sys.stdin.read(1); "
                         "termios.tcsetattr(0,termios.TCSANOW,before); "
                         "print('LIFECYCLE_READY',flush=True); "
                         "line=sys.stdin.readline(); print('DECLINED:'+line,flush=True); sys.exit(7)")
        s.wait('RAW_READY', 'raw'); s.send(b'x')
        s.wait('LIFECYCLE_READY', 'lifecycle')
        self.assertEqual(h.termios.tcgetattr(s.slave), s.before)
        s.send(b'n\n'); s.finish(expected=7)
        self.assertEqual(s.process.poll(), 7)
        s.restore_probe()
        self.assertEqual(s.process.poll(), 7)  # probe status cannot replace CLI status
        self.assertIn('DECLINED:n', h.clean(s.raw))

    def test_paste_advancement_then_cancellation_is_rejected(self):
        s = self.session("import os,tty; tty.setraw(0); print('SELECT_READY',flush=True); "
                         "os.read(0,4096); print('Target: cursor\\r\\nApply this plan?',flush=True); "
                         "os.read(0,4096)")
        s.wait('SELECT_READY', 'selection')
        offset = len(s.raw)
        s.send(b'\x1b[200~\ny\n\x1b[201~')
        s.wait('Apply this plan', 'bad-advance', after=offset)
        s.send(b'\x1b')
        with self.assertRaisesRegex(AssertionError, 'paste advanced'):
            h.assert_paste_stayed_in_selection(s.raw[offset:], r'Apply this plan')

    def test_resize_reaches_slave(self):
        s = self.session("import os,sys; print('READY',flush=True); sys.stdin.readline(); "
                         "print('SIZE:'+str(os.get_terminal_size(0).columns),flush=True)")
        s.wait('READY', 'ready'); s.resize(8, 40); s.send(b'\n')
        s.wait('SIZE:40', 'resized'); s.finish()


if __name__ == '__main__': unittest.main()
