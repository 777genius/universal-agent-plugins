"""Resource ownership fault injection; not native ConPTY qualification."""
import unittest
from types import SimpleNamespace
from windows_conpty import ConPTY

class CleanupTests(unittest.TestCase):
    def test_termination_failure_and_exit_race_release_all_resources(self):
        for exited in (False, True):
            with self.subTest(exited=exited):
                released = []
                joined = []
                c = ConPTY.__new__(ConPTY)
                c.closed = False; c.forced = False; c.error = None
                c.pi = SimpleNamespace(hProcess=10)
                c.hpc = 20; c.handles = [30, 40, 10, 50]
                c.poll = lambda: None
                c.ok = lambda value: self.assertTrue(value)
                c.reader = SimpleNamespace(join=lambda timeout: joined.append(True), is_alive=lambda: False)
                c.k = SimpleNamespace(TerminateProcess=lambda *args: False,
                    WaitForSingleObject=lambda *args: 0 if exited else 258,
                    ClosePseudoConsole=lambda handle: released.append(handle),
                    CloseHandle=lambda handle: released.append(handle) or True)
                if exited: c.close()
                else:
                    with self.assertRaisesRegex(AssertionError, 'TerminateProcess failed'): c.close()
                self.assertCountEqual(released, [20, 30, 40, 10, 50])
                self.assertEqual(joined, [True])
                c.close()
                self.assertEqual(len(released), 5)
