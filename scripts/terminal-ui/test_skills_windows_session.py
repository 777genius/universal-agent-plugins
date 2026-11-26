"""ConPTY owner readiness failures retain cleanup and the original failure."""
from pathlib import Path
import tempfile
from types import ModuleType, SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import skills_matrix as matrix


class WindowsSessionStartupTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.fixture = SimpleNamespace(project=self.root, env={'TERM': 'xterm', 'KEPT': 'value'})
        self.terminal = SimpleNamespace(wait=Mock(), close=Mock(), status_path=None)
        self.module = ModuleType('windows_conpty')
        self.module.ConPTY = Mock(return_value=self.terminal)
        self.module.finish_console = Mock()
        self.modules = patch.dict(matrix.sys.modules, {'windows_conpty': self.module})
        self.modules.start()
        self.addCleanup(self.modules.stop)

    def construct(self):
        with patch.object(matrix.uuid, 'uuid4', return_value=SimpleNamespace(hex='owned-nonce')):
            return matrix.WindowsSession(['private-cli', '--plain'], self.fixture, self.root, 0.5)

    def test_timeout_eof_early_exit_and_interrupt_close_the_returned_owner_once(self):
        for failure in (TimeoutError('OWNER_READY timeout'), EOFError('console output EOF'),
                        AssertionError('console owner exited before readiness'), KeyboardInterrupt()):
            with self.subTest(failure=type(failure).__name__):
                self.terminal.wait.reset_mock(side_effect=True)
                self.terminal.close.reset_mock()
                self.terminal.wait.side_effect = failure
                with self.assertRaises(type(failure)) as caught:
                    self.construct()
                self.assertIs(caught.exception, failure)
                self.terminal.wait.assert_called_once_with('OWNER_READY_owned-nonce')
                self.assertEqual(self.terminal.status_path, self.root / 'status.json')
                self.terminal.close.assert_called_once_with()
                self.module.finish_console.assert_not_called()

    def test_ready_owner_is_retained_without_premature_cleanup(self):
        def ready(marker):
            self.assertEqual(marker, 'OWNER_READY_owned-nonce')
            self.assertEqual(self.terminal.status_path, self.root / 'status.json')
        self.terminal.wait.side_effect = ready
        session = self.construct()
        self.assertIs(session.terminal, self.terminal)
        self.terminal.close.assert_not_called()
        self.module.finish_console.assert_not_called()
        argv, environment, project, timeout = self.module.ConPTY.call_args.args
        self.assertEqual(Path(argv[-1]), self.root / 'job.json')
        self.assertEqual(environment, {'KEPT': 'value'})
        self.assertEqual(self.fixture.env, {'TERM': 'xterm', 'KEPT': 'value'})
        self.assertEqual((project, timeout), (self.root, 0.5))

    def test_cleanup_failure_reports_both_failures_and_chains_original_readiness_error(self):
        for primary in (TimeoutError('readiness deadline'), KeyboardInterrupt()):
            with self.subTest(primary=type(primary).__name__):
                cleanup = OSError('job termination was not confirmed')
                self.terminal.close.reset_mock(side_effect=True)
                self.terminal.wait.side_effect = primary
                self.terminal.close.side_effect = cleanup
                with self.assertRaises(RuntimeError) as caught:
                    self.construct()
                self.assertIs(caught.exception.__cause__, primary)
                self.assertIn(repr(primary), str(caught.exception))
                self.assertIn(repr(cleanup), str(caught.exception))
                self.assertIn('cleanup', str(caught.exception))
                self.terminal.close.assert_called_once_with()
                self.module.finish_console.assert_not_called()

    def test_cleanup_interrupt_or_exit_remains_an_abort_with_original_failure_context(self):
        for abort in (KeyboardInterrupt(), SystemExit(23)):
            with self.subTest(abort=type(abort).__name__):
                primary = TimeoutError('readiness deadline before cleanup abort')
                self.terminal.close.reset_mock(side_effect=True)
                self.terminal.wait.side_effect = primary
                self.terminal.close.side_effect = abort
                with self.assertRaises(type(abort)) as caught:
                    self.construct()
                self.assertIs(caught.exception, abort)
                self.assertIs(caught.exception.__context__, primary)
                self.terminal.close.assert_called_once_with()
                self.module.finish_console.assert_not_called()

    def test_constructor_failure_keeps_conpty_own_startup_cleanup_authority(self):
        failure = OSError('ConPTY creation failed after its own cleanup')
        self.module.ConPTY.side_effect = failure
        with self.assertRaises(OSError) as caught:
            self.construct()
        self.assertIs(caught.exception, failure)
        self.terminal.wait.assert_not_called()
        self.terminal.close.assert_not_called()
        self.module.finish_console.assert_not_called()


if __name__ == '__main__':
    unittest.main()
