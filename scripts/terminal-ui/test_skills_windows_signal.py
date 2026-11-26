"""Private Windows owner CTRL+C setup and bounded real-input probe contracts."""
import copy
import io
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import skills_matrix as matrix
import windows_job as job


class CtrlCProcessingTests(unittest.TestCase):
    def owner(self, admitted=True):
        owner = job.ConsoleOwner.__new__(job.ConsoleOwner)
        owner.private_console_pid = os.getpid() if admitted else os.getpid() + 1
        owner.k = SimpleNamespace(SetConsoleCtrlHandler=Mock(return_value=True))
        return owner

    def test_reset_is_checked_process_local_and_does_not_claim_the_prior_ignore_state(self):
        owner = self.owner()
        self.assertEqual(owner.restore_ctrl_c_processing(), dict(
            owner_pid=os.getpid(), reset_checked=True, prior_ignore_state='unknown'))
        owner.k.SetConsoleCtrlHandler.assert_called_once_with(None, False)

    def test_unadmitted_owner_cannot_reset_ctrl_c_processing(self):
        owner = self.owner(admitted=False)
        with self.assertRaisesRegex(AssertionError, 'admitted private console'):
            owner.restore_ctrl_c_processing()
        owner.k.SetConsoleCtrlHandler.assert_not_called()

    def test_foreign_console_is_refused_before_opening_handles_or_resetting(self):
        kernel = Mock()
        def processes(pids, _capacity):
            pids[0], pids[1] = os.getpid(), os.getpid() + 1
            return 2
        kernel.GetConsoleProcessList.side_effect = processes
        with patch.dict(job.sys.modules, {'msvcrt': SimpleNamespace()}), \
                patch.object(job.ctypes, 'WinDLL', return_value=kernel, create=True):
            with self.assertRaisesRegex(AssertionError, 'alone in its new ConPTY'):
                job.ConsoleOwner(Mock())
        kernel.CreateFileW.assert_not_called()
        kernel.SetConsoleCtrlHandler.assert_not_called()

    def test_reset_failure_is_not_a_successful_normalization_receipt(self):
        owner = self.owner()
        owner.k.SetConsoleCtrlHandler.return_value = False
        failure = OSError('SetConsoleCtrlHandler failed')
        failure.winerror = 5
        with patch.object(job.ctypes, 'WinError', return_value=failure, create=True), \
                patch.object(job.ctypes, 'get_last_error', return_value=5, create=True):
            with self.assertRaisesRegex(OSError, 'SetConsoleCtrlHandler'):
                owner.restore_ctrl_c_processing()


class CtrlCInputProbeTests(unittest.TestCase):
    def setUp(self):
        self.snapshot = dict(handles=[10, 11, 12], modes=[503, 7, 7],
                             cursor=dict(size=25, visible=True))
        self.state = dict(owner_before=copy.deepcopy(self.snapshot), owner_ctrl_c_events=0)
        self.console = SimpleNamespace(snapshot=Mock(return_value=self.snapshot), stdin=Mock())
        self.markers = []

    def test_one_observed_event_keeps_modes_and_never_reads_input(self):
        def sleep(_seconds):
            self.state['owner_ctrl_c_events'] += 1
        with patch.object(job.time, 'monotonic', side_effect=[0, 0, 0.02]), \
                patch.object(job.time, 'sleep', side_effect=sleep):
            job.probe_ctrl_c_input(self.console, self.state, self.markers.append)
        self.assertEqual(self.markers, ['CTRL_C_INPUT_READY_', 'CTRL_C_INPUT_OK_'])
        self.assertEqual(self.state['ctrl_c_probe'],
                         dict(observed_events=1, owner_console_unchanged=True))
        self.console.stdin.assert_not_called()
        self.console.stdin.readline.assert_not_called()

    def test_no_control_event_fails_within_five_seconds_without_ok(self):
        with patch.object(job.time, 'monotonic', side_effect=[10, 10, 15]), \
                patch.object(job.time, 'sleep') as sleep:
            with self.assertRaisesRegex(AssertionError, 'actual ETX did not reach'):
                job.probe_ctrl_c_input(self.console, self.state, self.markers.append)
        self.assertEqual(self.markers, ['CTRL_C_INPUT_READY_'])
        sleep.assert_called_once_with(0.02)
        self.assertNotIn('ctrl_c_probe', self.state)

    def test_prior_duplicate_or_mode_changing_events_cannot_qualify(self):
        self.state['owner_ctrl_c_events'] = 1
        with self.assertRaisesRegex(AssertionError, 'unrelated control event'):
            job.probe_ctrl_c_input(self.console, self.state, self.markers.append)
        self.assertEqual(self.markers, [])
        for count, changed in ((2, False), (True, False), (1, True)):
            with self.subTest(count=count, changed=changed):
                self.state['owner_ctrl_c_events'] = 0
                self.markers.clear()
                snapshot = copy.deepcopy(self.state['owner_before'])
                snapshot['modes'][0] += int(changed)
                self.console.snapshot.return_value = snapshot
                def sleep(_seconds):
                    self.state['owner_ctrl_c_events'] = count
                with patch.object(job.time, 'monotonic', side_effect=[0, 0, 0.02]), \
                        patch.object(job.time, 'sleep', side_effect=sleep):
                    with self.assertRaises(AssertionError):
                        job.probe_ctrl_c_input(self.console, self.state, self.markers.append)
                self.assertEqual(self.markers, ['CTRL_C_INPUT_READY_'])
                self.assertNotIn('ctrl_c_probe', self.state)

    def test_browser_requires_a_new_event_beyond_the_owner_input_probe(self):
        with tempfile.TemporaryDirectory() as temporary:
            session = matrix.WindowsSession.__new__(matrix.WindowsSession)
            session.status_path = Path(temporary) / 'status.json'
            for count in (0, 1, True, 2):
                session.status_path.write_text(json.dumps(dict(
                    owner_ctrl_c_events=count, ctrl_c_probe=dict(observed_events=1))), encoding='utf-8')
                if type(count) is int and count > 1:
                    session.assert_ctrl_c_delivery()
                else:
                    with self.assertRaisesRegex(AssertionError, 'new actual console CTRL'):
                        session.assert_ctrl_c_delivery()


class OwnerStartupOrderTests(unittest.TestCase):
    def test_owner_proves_input_before_child_and_retains_later_interrupt_and_restoration(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            status, config = root / 'status.json', root / 'job.json'
            config.write_text(json.dumps(dict(argv=['private-cli.exe'], cwd=temporary, nonce='owned',
                                             status=str(status), ctrl_c_probe=True)), encoding='utf-8')
            events, handlers = [], []
            class Output(io.StringIO):
                def write(self, text):
                    if text == 'CTRL_C_INPUT_READY_owned':
                        events.append('etx-input-probe')
                        handlers[0](job.signal.SIGINT, None)
                    return super().write(text)
            owner = SimpleNamespace(initial=[1, 2, 3], initial_modes=[], stdout=Output(),
                stdin=SimpleNamespace(readline=Mock(return_value='line_owned\n')), stderr=io.StringIO(),
                snapshot=lambda: dict(handles=[10, 11, 12], modes=[503, 7, 7],
                                      cursor=dict(size=25, visible=True)))
            def register(_number, callback):
                events.append('callable-handler')
                handlers.append(callback)
            def reset():
                self.assertEqual(events, ['callable-handler'])
                events.append('checked-reset')
                return dict(owner_pid=os.getpid(), reset_checked=True, prior_ignore_state='unknown')
            owner.restore_ctrl_c_processing = reset
            def launch(*_args, **_kwargs):
                self.assertEqual(events, ['callable-handler', 'checked-reset', 'etx-input-probe'])
                saved = json.loads(status.read_text(encoding='utf-8'))
                self.assertEqual(saved['ctrl_c_probe'],
                                 dict(observed_events=1, owner_console_unchanged=True))
                self.assertEqual(saved['phase'], 'owner-ready')
                events.append('child-spawn')
                def wait():
                    handlers[0](job.signal.SIGINT, None)
                    return 1
                return SimpleNamespace(pid=321, wait=wait)
            with patch.object(job.sys, 'argv', ['windows_job.py', str(config)]), \
                    patch.object(job, 'ConsoleOwner', return_value=owner), \
                    patch.object(job.signal, 'signal', side_effect=register), \
                    patch.object(job.subprocess, 'Popen', side_effect=launch):
                job.main()
            saved = json.loads(status.read_text(encoding='utf-8'))
            self.assertEqual((saved['phase'], saved['exit'], saved['owner_ctrl_c_events']), ('done', 1, 2))
            self.assertEqual(saved['owner_before'], saved['owner_after'])
            self.assertEqual(saved['owner_before'], saved['owner_probe'])
            self.assertTrue(saved['line_read'])

    def test_callable_handler_precedes_reset_and_probe_failure_prevents_child_spawn(self):
        for reset_fails in (False, True):
            with self.subTest(reset_fails=reset_fails), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                status, config = root / 'status.json', root / 'job.json'
                config.write_text(json.dumps(dict(argv=['never.exe'], cwd=temporary, nonce='owned',
                                                 status=str(status), ctrl_c_probe=True)), encoding='utf-8')
                events = []
                owner = SimpleNamespace(initial=[1, 2, 3], initial_modes=[], stdout=io.StringIO(),
                    snapshot=lambda: dict(handles=[10, 11, 12], modes=[503, 7, 7],
                                          cursor=dict(size=25, visible=True)))
                def handler(_number, callback):
                    self.assertTrue(callable(callback))
                    events.append('callable-handler')
                def reset():
                    self.assertEqual(events, ['callable-handler'])
                    events.append('checked-reset')
                    if reset_fails:
                        raise OSError('CTRL+C reset denied')
                    return dict(owner_pid=os.getpid(), reset_checked=True, prior_ignore_state='unknown')
                owner.restore_ctrl_c_processing = reset
                with patch.object(job.sys, 'argv', ['windows_job.py', str(config)]), \
                        patch.object(job, 'ConsoleOwner', return_value=owner), \
                        patch.object(job.signal, 'signal', side_effect=handler), \
                        patch.object(job.time, 'monotonic', side_effect=[0, 5]), \
                        patch.object(job.subprocess, 'Popen') as launch:
                    with self.assertRaises(OSError if reset_fails else AssertionError):
                        job.main()
                launch.assert_not_called()
                saved = json.loads(status.read_text(encoding='utf-8'))
                self.assertIn('reset denied' if reset_fails else 'actual ETX did not reach', saved['error'])
                self.assertNotIn('OWNER_READY_', owner.stdout.getvalue())
                self.assertNotIn('CTRL_C_INPUT_OK_', owner.stdout.getvalue())


if __name__ == '__main__':
    unittest.main()
