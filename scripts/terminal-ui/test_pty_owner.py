"""Finite process-group cleanup controls; no CLI or agent runtime is invoked."""
import errno
import importlib.util
import os
from pathlib import Path
import signal
import subprocess
import unittest
from unittest.mock import Mock, call, patch

if os.name == 'posix':
    spec = importlib.util.spec_from_file_location('terminal_owner', Path(__file__).with_name('pty_owner.py'))
    owner = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(owner)
else:
    owner = None


@unittest.skipUnless(os.name == 'posix', 'Unix process groups only')
class KillJobTests(unittest.TestCase):
    def child(self, status=0):
        child = Mock(pid=18721)
        child.poll.return_value = status
        child.wait.return_value = status
        return child

    def test_zombie_group_eperm_requires_reap_and_exact_group_absence(self):
        child = self.child(0)
        errors = [PermissionError(errno.EPERM, 'zombie group'),
                  ProcessLookupError(errno.ESRCH, 'group absent after reap')]
        with patch.object(owner.os, 'killpg', side_effect=errors) as kill:
            self.assertEqual(owner.kill_job(child), 0)
        self.assertEqual(kill.call_args_list, [call(child.pid, signal.SIGKILL), call(child.pid, 0)])
        child.poll.assert_called_once_with()
        child.wait.assert_called_once_with(timeout=2)

    def test_live_leader_permission_error_is_not_ignored(self):
        child = self.child(None)
        denied = PermissionError(errno.EPERM, 'live leader')
        with patch.object(owner.os, 'killpg', side_effect=denied) as kill:
            with self.assertRaises(PermissionError) as caught:
                owner.kill_job(child)
        self.assertIs(caught.exception, denied)
        kill.assert_called_once_with(child.pid, signal.SIGKILL)
        child.poll.assert_called_once_with()
        child.wait.assert_not_called()

    def test_exited_leader_with_live_group_is_not_qualified_closed(self):
        child = self.child(0)
        denied = PermissionError(errno.EPERM, 'group not signalable')
        with patch.object(owner.os, 'killpg', side_effect=[denied, None]) as kill:
            with self.assertRaises(PermissionError) as caught:
                owner.kill_job(child)
        self.assertIs(caught.exception, denied)
        self.assertEqual(kill.call_args_list, [call(child.pid, signal.SIGKILL), call(child.pid, 0)])
        child.wait.assert_not_called()

    def test_inaccessible_group_after_reap_preserves_both_errors(self):
        child = self.child(0)
        denied = PermissionError(errno.EPERM, 'initial signal denied')
        inaccessible = PermissionError(errno.EPERM, 'group still inaccessible')
        with patch.object(owner.os, 'killpg', side_effect=[denied, inaccessible]) as kill:
            with self.assertRaises(PermissionError) as caught:
                owner.kill_job(child)
        self.assertIs(caught.exception, inaccessible)
        self.assertIs(caught.exception.__context__, denied)
        self.assertEqual(kill.call_args_list, [call(child.pid, signal.SIGKILL), call(child.pid, 0)])
        child.wait.assert_not_called()

    def test_other_signal_errors_do_not_enter_exit_qualification(self):
        for error in [PermissionError(errno.EACCES, 'different refusal'),
                      OSError(errno.EINVAL, 'invalid signal')]:
            with self.subTest(errno=error.errno):
                child = self.child(0)
                with patch.object(owner.os, 'killpg', side_effect=error) as kill:
                    with self.assertRaises(OSError) as caught:
                        owner.kill_job(child)
                self.assertIs(caught.exception, error)
                kill.assert_called_once_with(child.pid, signal.SIGKILL)
                child.poll.assert_not_called()
                child.wait.assert_not_called()

    def test_only_esrch_after_reap_proves_group_absence(self):
        for error in [ProcessLookupError(errno.ECHILD, 'different lookup failure'),
                      OSError(errno.EINVAL, 'invalid group probe')]:
            with self.subTest(errno=error.errno):
                child = self.child(0)
                denied = PermissionError(errno.EPERM, 'initial signal denied')
                with patch.object(owner.os, 'killpg', side_effect=[denied, error]) as kill:
                    with self.assertRaises(OSError) as caught:
                        owner.kill_job(child)
                self.assertIs(caught.exception, error)
                self.assertIs(caught.exception.__context__, denied)
                self.assertEqual(kill.call_args_list, [call(child.pid, signal.SIGKILL), call(child.pid, 0)])
                child.wait.assert_not_called()

    def test_successful_signal_still_waits_with_the_original_bound(self):
        child = self.child(-signal.SIGKILL)
        with patch.object(owner.os, 'killpg') as kill:
            self.assertEqual(owner.kill_job(child), -signal.SIGKILL)
        kill.assert_called_once_with(child.pid, signal.SIGKILL)
        child.poll.assert_not_called()
        child.wait.assert_called_once_with(timeout=2)

    def test_already_missing_group_still_reaps_the_owned_child(self):
        child = self.child(0)
        with patch.object(owner.os, 'killpg', side_effect=ProcessLookupError(errno.ESRCH, 'absent')) as kill:
            self.assertEqual(owner.kill_job(child), 0)
        kill.assert_called_once_with(child.pid, signal.SIGKILL)
        child.poll.assert_not_called()
        child.wait.assert_called_once_with(timeout=2)

    def test_reap_timeout_is_not_qualified_closed(self):
        child = self.child(None)
        timeout = subprocess.TimeoutExpired('owned test process', 2)
        child.wait.side_effect = timeout
        with patch.object(owner.os, 'killpg') as kill:
            with self.assertRaises(subprocess.TimeoutExpired) as caught:
                owner.kill_job(child)
        self.assertIs(caught.exception, timeout)
        kill.assert_called_once_with(child.pid, signal.SIGKILL)
        child.wait.assert_called_once_with(timeout=2)


if __name__ == '__main__':
    unittest.main()
