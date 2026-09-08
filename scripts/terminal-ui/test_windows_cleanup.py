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

    def test_job_termination_waits_without_killing_owner_again(self):
        c = ConPTY.__new__(ConPTY)
        c.closed = False; c.forced = False; c.error = None
        c.pi = SimpleNamespace(hProcess=10)
        c.job = 70; c.job_assigned = True
        c.hpc = None; c.handles = [70, 10]; c.reader = None
        c.poll = lambda: None
        c.ok = lambda value: self.assertTrue(value)
        counts = iter([2, 1, 0])
        killed, waited = [], []
        def accounting(job, info, value, size, result):
            value._obj.active_processes = next(counts)
            return True
        c.k = SimpleNamespace(QueryInformationJobObject=accounting,
            TerminateJobObject=lambda job, code: killed.append(job) or True,
            TerminateProcess=lambda *args: self.fail('must not race job termination'),
            WaitForSingleObject=lambda handle, timeout: waited.append((handle, timeout)) or 0,
            CloseHandle=lambda handle: True)
        c.close()
        self.assertTrue(c.forced)
        self.assertEqual(killed, [70])
        self.assertEqual(waited, [(10, 2000)])

    def test_failed_job_assignment_still_terminates_suspended_owner(self):
        c = ConPTY.__new__(ConPTY)
        c.closed = False; c.forced = False; c.error = None
        c.pi = SimpleNamespace(hProcess=10)
        c.job = 70; c.job_assigned = False
        c.hpc = None; c.handles = [70, 10]; c.reader = None
        c.poll = lambda: None
        c.ok = lambda value: self.assertTrue(value)
        killed = []
        c.k = SimpleNamespace(TerminateProcess=lambda handle, code: killed.append(handle) or True,
            WaitForSingleObject=lambda *args: 0, CloseHandle=lambda handle: True)
        c.close()
        self.assertTrue(c.forced)
        self.assertEqual(killed, [10])

class OwnerTests(unittest.TestCase):
    def test_rejects_shared_or_unattached_console_before_open(self):
        from windows_job import ConsoleOwner
        ConsoleOwner.require_private_console([123], 1, 123)
        for pids, count in (([], 0), ([456], 1), ([123, 456], 2), ([123], 65)):
            with self.subTest(pids=pids, count=count):
                with self.assertRaisesRegex(AssertionError, 'alone'):
                    ConsoleOwner.require_private_console(pids, count, 123)

    def test_snapshot_keeps_full_width_handles_and_exact_cursor(self):
        from windows_job import ConsoleOwner
        owner = ConsoleOwner.__new__(ConsoleOwner)
        handles = {(-10) & 0xffffffff: 0x123456789, (-11) & 0xffffffff: 0x23456789a,
                   (-12) & 0xffffffff: 0x3456789ab}
        queried = []
        def mode(handle, value):
            queried.append(handle)
            value._obj.value = 7 if handle == handles[(-10) & 0xffffffff] else 3
            return True
        def cursor(handle, value):
            self.assertEqual(handle, handles[(-11) & 0xffffffff])
            value._obj.size, value._obj.visible = 25, True
            return True
        owner.k = SimpleNamespace(GetStdHandle=handles.__getitem__, GetConsoleMode=mode,
                                  GetConsoleCursorInfo=cursor)
        state = owner.snapshot()
        self.assertEqual(state['handles'], list(handles.values()))
        self.assertEqual(queried, list(handles.values()))
        self.assertEqual(state['modes'], [7, 3, 3])
        self.assertEqual(state['cursor'], {'size': 25, 'visible': True})

    def test_native_probe_failure_prevents_cli_launch_and_saves_error(self):
        import json
        import tempfile
        from pathlib import Path
        from unittest.mock import patch
        import windows_job
        with tempfile.TemporaryDirectory() as root:
            status = Path(root) / 'status.json'
            config = Path(root) / 'job.json'
            config.write_text(json.dumps(dict(status=str(status), argv=['never.exe'], cwd=root, nonce='test')))
            owner = SimpleNamespace(snapshot=lambda: (_ for _ in ()).throw(OSError('GetConsoleMode: WinError 6')))
            with patch.object(windows_job.sys, 'argv', ['windows_job.py', str(config)]), \
                    patch.object(windows_job, 'ConsoleOwner', return_value=owner), \
                    patch.object(windows_job.signal, 'signal'), \
                    patch.object(windows_job.subprocess, 'Popen') as launch:
                with self.assertRaisesRegex(OSError, 'WinError 6'):
                    windows_job.main()
                launch.assert_not_called()
            self.assertIn('WinError 6', json.loads(status.read_text())['error'])

    def test_powershell_no_profile_literal_unicode_arguments_and_exit(self):
        import base64
        from windows_conpty import powershell_argv
        argv = powershell_argv('pwsh.exe', ["C:\\test é's\\cli.exe", 'add', 'C:\\fixture $x'], 'nonce')
        self.assertEqual(argv[:5], ['pwsh.exe', '-NoLogo', '-NoProfile', '-NonInteractive', '-EncodedCommand'])
        command = base64.b64decode(argv[-1]).decode('utf-16-le')
        self.assertEqual(command, "$ErrorActionPreference = 'Stop'; $global:LASTEXITCODE = $null; Write-Output 'POWERSHELL_LAUNCH_nonce'; & 'C:\\test é''s\\cli.exe' 'add' 'C:\\fixture $x'; if ($null -eq $LASTEXITCODE) { throw 'native CLI did not return an exit code' }; exit $LASTEXITCODE")

    def test_owner_error_is_reported_before_prompt_timeout(self):
        import json
        import tempfile
        from pathlib import Path
        with tempfile.TemporaryDirectory() as root:
            c = ConPTY.__new__(ConPTY)
            c.timeout, c.error, c.raw = 1, None, bytearray()
            c.poll = lambda: 1
            c.status_path = Path(root) / 'status.json'
            c.status_path.write_text(json.dumps({'error': 'GetConsoleMode(std=-10): WinError 6'}))
            with self.assertRaisesRegex(AssertionError, r'GetConsoleMode\(std=-10\): WinError 6'):
                c.wait('prompt')

    def test_surviving_job_descendant_is_forced_even_after_owner_exits(self):
        c = ConPTY.__new__(ConPTY)
        c.closed = False; c.forced = False; c.error = None
        c.pi = SimpleNamespace(hProcess=10)
        c.job = 70; c.hpc = None; c.handles = [70, 10]; c.reader = None
        c.poll = lambda: 0
        c.ok = lambda value: self.assertTrue(value)
        killed, released = [], []
        counts = iter([1, 1, 0])
        def accounting(job, info, value, size, result):
            value._obj.active_processes = next(counts)
            return True
        c.k = SimpleNamespace(QueryInformationJobObject=accounting,
            TerminateJobObject=lambda job, code: killed.append(job) or True,
            CloseHandle=lambda handle: released.append(handle) or True)
        c.close()
        self.assertTrue(c.forced)
        self.assertEqual(killed, [70])
        self.assertEqual(released, [70, 10])

    @unittest.skipUnless(__import__('os').name == 'nt', 'Windows ABI sizes require native ctypes')
    def test_native_abi_structure_sizes(self):
        import ctypes
        from windows_conpty import STARTUPINFO, STARTUPINFOEX, PROCESS_INFORMATION, JOB_BASIC_LIMIT, JOB_EXTENDED_LIMIT, JOB_ACCOUNTING
        if ctypes.sizeof(ctypes.c_void_p) != 8:
            self.skipTest('workflow qualification targets Windows amd64')
        for structure, expected in ((STARTUPINFO, 104), (STARTUPINFOEX, 112), (PROCESS_INFORMATION, 24),
                                    (JOB_BASIC_LIMIT, 64), (JOB_EXTENDED_LIMIT, 144), (JOB_ACCOUNTING, 48)):
            self.assertEqual(ctypes.sizeof(structure), expected, structure.__name__)

    def test_invalid_read_handle_is_only_expected_during_cleanup(self):
        import ctypes
        from unittest.mock import patch
        for closed in (False, True):
            c = ConPTY.__new__(ConPTY)
            c.closed, c.error, c.output = closed, None, 50
            c.k = SimpleNamespace(ReadFile=lambda *args: False)
            with patch.object(ctypes, 'get_last_error', return_value=6, create=True):
                c.read()
            self.assertEqual(c.error, None if closed else 'ReadFile WinError 6')


class PowerShellFixtureTests(unittest.TestCase):
    def make_fixture(self, root):
        from pathlib import Path
        from harness import Fixture
        # Opaque synthetic scanner bytes suffice: these tests never execute it.
        scanner = Path(root) / 'scanner.exe'
        scanner.write_bytes(b'synthetic, not executable')
        return Fixture(Path(root) / 'fixture', scanner)

    def test_startup_baseline_adds_only_four_empty_directories(self):
        import tempfile
        from pathlib import Path
        from windows_conpty import prepare_powershell_fixture
        with tempfile.TemporaryDirectory() as root:
            fixture = self.make_fixture(root)
            before = fixture.mutations()
            env = dict(fixture.env)
            prepare_powershell_fixture(fixture)
            expected = dict(before, home=dict(before['home']))
            path = Path()
            for part in ('AppData', 'Local', 'Microsoft', 'PowerShell'):
                path /= part
                expected['home'][str(path)] = 'directory'
            self.assertEqual(fixture.before, expected)
            self.assertEqual(fixture.env, dict(env, PATHEXT='.EXE'))
            fixture.unchanged()

    def test_all_cli_mutations_remain_detected(self):
        import tempfile
        from windows_conpty import prepare_powershell_fixture
        targets = ('home/.codex/config.toml', 'home/.cursor/config.json',
                   'home/unexpected', 'home/AppData/Local/Microsoft/PowerShell/cache',
                   'project/config', 'managed/package', 'operations/journal', 'state')
        for target in targets:
            with self.subTest(target=target), tempfile.TemporaryDirectory() as root:
                fixture = self.make_fixture(root)
                prepare_powershell_fixture(fixture)
                roots = dict(home=fixture.home, project=fixture.project,
                             managed=fixture.data / 'managed', operations=fixture.data / 'operations-v2',
                             state=fixture.data / 'state-v2.json')
                key, _, relative = target.partition('/')
                path = roots[key] / relative if relative else roots[key]
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text('unauthorized mutation')
                with self.assertRaisesRegex(AssertionError, 'mutated before consent'):
                    fixture.unchanged()

    def test_removed_startup_directory_is_detected(self):
        import tempfile
        from windows_conpty import prepare_powershell_fixture
        with tempfile.TemporaryDirectory() as root:
            fixture = self.make_fixture(root)
            prepare_powershell_fixture(fixture)
            (fixture.home / 'AppData' / 'Local' / 'Microsoft' / 'PowerShell').rmdir()
            with self.assertRaisesRegex(AssertionError, 'mutated before consent'):
                fixture.unchanged()

    def test_setup_cannot_rebaseline_existing_mutation(self):
        import tempfile
        from windows_conpty import prepare_powershell_fixture
        with tempfile.TemporaryDirectory() as root:
            fixture = self.make_fixture(root)
            before = fixture.before
            (fixture.project / 'unauthorized').write_text('mutation')
            with self.assertRaisesRegex(AssertionError, 'mutated before consent'):
                prepare_powershell_fixture(fixture)
            self.assertIs(fixture.before, before)
            self.assertFalse((fixture.home / 'AppData').exists())


class FailureEvidenceTests(unittest.TestCase):
    def test_cleanup_preserves_prompt_failure_and_separate_cleanup_error(self):
        import json
        import tempfile
        from pathlib import Path
        from windows_conpty import finish_console
        for cleanup_fails in (False, True):
            with self.subTest(cleanup_fails=cleanup_fails), tempfile.TemporaryDirectory() as root:
                evidence = Path(root)
                def close(pid):
                    if cleanup_fails:
                        raise OSError('injected cleanup error')
                terminal = SimpleNamespace(close=close, raw=b'original transcript', forced=True, error=None)
                with self.assertRaisesRegex(Exception, 'original prompt timeout'):
                    try:
                        raise AssertionError('original prompt timeout')
                    finally:
                        finish_console(terminal, evidence / 'status.json', evidence)
                saved = json.loads((evidence / 'cleanup.json').read_text())
                self.assertIn('original prompt timeout', saved['primary_error'])
                self.assertEqual(bool(saved['cleanup_errors']), cleanup_fails)
                self.assertTrue(saved['forced'])
                self.assertEqual((evidence / 'terminal.ansi').read_bytes(), terminal.raw)

    def test_cleanup_failure_without_primary_still_fails(self):
        import tempfile
        from pathlib import Path
        from windows_conpty import finish_console
        with tempfile.TemporaryDirectory() as root:
            terminal = SimpleNamespace(close=lambda pid: (_ for _ in ()).throw(OSError('kill failed')),
                                       raw=b'', forced=True, error=None)
            with self.assertRaisesRegex(AssertionError, 'kill failed'):
                finish_console(terminal, Path(root) / 'status.json', Path(root))
