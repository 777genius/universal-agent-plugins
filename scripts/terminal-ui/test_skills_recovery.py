"""Negative controls for the exact Linux CLI/supervisor/native crash ownership."""
import copy
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import skills_matrix as matrix


class NativeLineageTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        root = Path(self.temporary.name).resolve()
        self.binary, self.native = root / 'agentplugins', root / 'codex'
        self.binary.write_bytes(b'private CLI inode')
        self.native.write_bytes(b'private responder inode')
        self.process = SimpleNamespace(pid=100, poll=Mock(return_value=None))
        self.marker = dict(nonce='owned-nonce', pid=300, ppid=200, executable=str(self.native))
        self.identities = {
            100: self.identity(100, 50, self.binary),
            200: self.identity(200, 100, self.binary),
            300: self.identity(300, 200, self.native),
        }

    def identity(self, pid, parent, executable):
        info = executable.stat()
        return dict(pid=pid, ppid=parent, start_time=pid * 100,
                    executable=str(executable.resolve()), executable_identity=[info.st_dev, info.st_ino])

    def observe(self, pid):
        return copy.deepcopy(self.identities[pid])

    def acquire(self, observer=None, exited=False, protocol_error=None):
        with patch.object(matrix, 'linux_process_identity', side_effect=observer or self.observe), \
                patch.object(matrix.os, 'pidfd_open', side_effect=[30, 20], create=True) as opened, \
                patch.object(matrix.os, 'close') as closed, \
                patch.object(matrix, 'linux_pidfd_exited', return_value=exited), \
                patch.object(matrix, 'linux_supervisor_protocol', side_effect=protocol_error) as protocol:
            try:
                return matrix.hold_native_lineage(self.process, self.marker, 'owned-nonce',
                                                   self.native, self.binary)
            finally:
                self.opened, self.closed, self.protocol = opened, closed, protocol

    def test_exact_single_supervisor_hop_is_held_until_caller_cleanup(self):
        held = self.acquire()
        self.assertEqual([(row['role'], row['fd']) for row in held],
                         [('native', 30), ('supervisor', 20)])
        self.assertEqual([row['identity']['pid'] for row in held], [300, 200])
        self.protocol.assert_called_once_with(200)
        self.closed.assert_not_called()

    def test_wrong_nonce_invalid_pid_or_foreign_executable_never_open_handles(self):
        original = dict(self.marker)
        for field, value in (('nonce', 'foreign'), ('pid', True), ('pid', 1),
                             ('ppid', -1), ('executable', str(self.binary))):
            with self.subTest(field=field, value=value):
                self.marker = dict(original, **{field: value})
                with self.assertRaises(AssertionError):
                    self.acquire()
                self.opened.assert_not_called()
                self.closed.assert_not_called()

    def test_foreign_parent_or_native_process_is_rejected_without_signaling(self):
        original = copy.deepcopy(self.identities)
        variants = ((200, 'ppid', 999), (300, 'ppid', 999),
                    (200, 'executable', str(self.native)),
                    (300, 'executable_identity', [0, 0]),
                    (100, 'executable', str(self.native)))
        for pid, field, value in variants:
            with self.subTest(pid=pid, field=field):
                self.identities = copy.deepcopy(original)
                self.identities[pid][field] = value
                with self.assertRaises(AssertionError), \
                        patch.object(matrix.signal, 'pidfd_send_signal', create=True) as signal:
                    self.acquire()
                signal.assert_not_called()
                self.assertEqual(self.closed.call_count, self.opened.call_count)

    def test_pid_reuse_reparent_and_cli_exit_races_fail_closed(self):
        for changing_pid, field in ((300, 'start_time'), (200, 'start_time'),
                                    (300, 'ppid'), (100, 'start_time')):
            with self.subTest(pid=changing_pid, field=field):
                calls = {}
                def changed(pid):
                    value = self.observe(pid)
                    calls[pid] = calls.get(pid, 0) + 1
                    if pid == changing_pid and calls[pid] > 1:
                        value[field] += 1
                    return value
                with self.assertRaises(AssertionError):
                    self.acquire(observer=changed)
                self.assertEqual(self.closed.call_count, self.opened.call_count)
        self.process.poll.return_value = 1
        with self.assertRaisesRegex(AssertionError, 'CLI exited'):
            self.acquire()
        self.opened.assert_not_called()

    def test_exited_pidfd_missing_proc_and_wrong_supervisor_protocol_fail_closed(self):
        with self.assertRaisesRegex(AssertionError, 'stable handle'):
            self.acquire(exited=True)
        self.closed.assert_called_once_with(30)
        with self.assertRaisesRegex(AssertionError, 'protocol'):
            self.acquire(protocol_error=AssertionError('wrong supervisor protocol'))
        self.assertEqual(self.closed.call_count, 2)
        def missing(pid):
            if pid == 200:
                raise FileNotFoundError('supervisor disappeared')
            return self.observe(pid)
        with self.assertRaises(FileNotFoundError):
            self.acquire(observer=missing)
        self.closed.assert_called_once_with(30)

    def test_late_lineage_change_is_checked_after_protocol_observation(self):
        calls = {}
        def changed(pid):
            row = self.observe(pid)
            calls[pid] = calls.get(pid, 0) + 1
            if pid == 300 and calls[pid] == 3:
                row['start_time'] += 1
            return row
        with self.assertRaisesRegex(AssertionError, 'before crash control'):
            self.acquire(observer=changed)
        self.protocol.assert_called_once()
        self.assertEqual(self.closed.call_count, 2)


    def test_supervisor_protocol_requires_exact_argv_and_one_marker_without_logging_environment(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            process = root / '200'
            process.mkdir()
            command = process / 'cmdline'
            environment = process / 'environ'
            command.write_bytes(b'/proc/self/exe\0')
            flag = b'PLUGIN_KIT_AI_ORDINARY_PROCESS_SUPERVISOR=1'
            environment.write_bytes(b'PRIVATE_UNUSED_VALUE=do-not-log\0' + flag + b'\0')
            with patch.object(matrix, 'Path', return_value=root):
                matrix.linux_supervisor_protocol(200)
                for body in (b'', flag + b'\0' + flag + b'\0', b'x' * ((1 << 20) + 1)):
                    environment.write_bytes(body)
                    with self.assertRaises(AssertionError):
                        matrix.linux_supervisor_protocol(200)
                environment.write_bytes(flag + b'\0')
                command.write_bytes(b'/proc/self/exe\0unrelated-argument\0')
                with self.assertRaisesRegex(AssertionError, 'exact OSRunner reexec'):
                    matrix.linux_supervisor_protocol(200)

@unittest.skipUnless(sys.platform == 'linux', 'Linux proc identity contract')
class LinuxIdentityTests(unittest.TestCase):
    def test_current_process_identity_uses_kernel_pid_parent_executable_and_starttime(self):
        identity = matrix.linux_process_identity(os.getpid())
        self.assertEqual(identity['pid'], os.getpid())
        self.assertEqual(identity['ppid'], os.getppid())
        self.assertGreater(identity['start_time'], 0)
        self.assertEqual(Path(identity['executable']), Path(sys.executable).resolve())
        info = Path(sys.executable).stat()
        self.assertEqual(identity['executable_identity'], [info.st_dev, info.st_ino])


if __name__ == '__main__':
    unittest.main()
