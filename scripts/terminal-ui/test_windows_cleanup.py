"""Resource ownership fault injection; not native ConPTY qualification."""
import unittest
import threading
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


class CaptureWaitTests(unittest.TestCase):
    def console(self):
        c = ConPTY.__new__(ConPTY)
        c.timeout, c.error, c.raw = 1, None, bytearray()
        c.output_changed, c.reader_done = threading.Condition(), False
        c.closed, c.output, c.status_path = False, 50, None
        c.poll = lambda: 0
        return c

    def test_poll_can_publish_marker_and_signal_exit(self):
        c = self.console()
        def poll():
            with c.output_changed:
                c.raw.extend(b'RESTORE_OK')
            return 0
        c.poll = poll
        self.assertEqual(c.wait('RESTORE_OK'), len(b'RESTORE_OK'))

    def test_delayed_reader_after_process_signal_and_multiple_scans(self):
        import ctypes
        from unittest.mock import patch
        for ending in ('marker', 'eof', 'error'):
            with self.subTest(ending=ending):
                c = self.console()
                release, entered = threading.Event(), threading.Event()
                calls = []
                def read(handle, buf, length, size, overlapped):
                    if calls:
                        size._obj.value = 0
                        return True
                    calls.append(True)
                    entered.set()
                    if not release.wait(2):
                        raise RuntimeError('test did not release reader')
                    if ending == 'error': return False
                    data = b'RESTORE_OK' if ending == 'marker' else b'other output'
                    ctypes.memmove(buf, data, len(data))
                    size._obj.value = len(data)
                    return True
                c.k = SimpleNamespace(ReadFile=read)
                polls = []
                def poll():
                    polls.append(0)
                    if len(polls) == 3: release.set()
                    return 0
                c.poll = poll
                with patch.object(ctypes, 'get_last_error', return_value=5, create=True):
                    reader = threading.Thread(target=c.read)
                    reader.start()
                    try:
                        self.assertTrue(entered.wait(1))
                        if ending == 'marker':
                            self.assertEqual(c.wait('RESTORE_OK'), len(b'RESTORE_OK'))
                        else:
                            message = 'EOF.*before marker' if ending == 'eof' else 'ReadFile WinError 5'
                            with self.assertRaisesRegex(AssertionError, message):
                                c.wait('RESTORE_OK')
                        self.assertGreaterEqual(len(polls), 3)
                    finally:
                        release.set()
                        reader.join(2)
                    self.assertFalse(reader.is_alive())

    def test_original_deadline_including_late_marker_and_eof(self):
        from unittest.mock import patch
        for ending in ('open', 'marker', 'eof'):
            with self.subTest(ending=ending):
                c = self.console()
                now = [0.0]
                waits = []
                class ScheduledCondition(threading.Condition):
                    def wait(self, timeout):
                        waits.append(timeout)
                        now[0] += timeout
                        if now[0] >= 1:
                            if ending == 'marker': c.raw.extend(b'RESTORE_OK')
                            if ending == 'eof': c.reader_done = True
                c.output_changed = ScheduledCondition()
                with patch('windows_conpty.time.monotonic', side_effect=lambda: now[0]):
                    with self.assertRaisesRegex(AssertionError, 'timeout waiting for RESTORE_OK'):
                        c.wait('RESTORE_OK')
                self.assertAlmostEqual(sum(waits), 1)
                self.assertEqual(now[0], 1)
                self.assertTrue(all(0 < wait <= .02 for wait in waits))

    def test_marker_published_during_poll_after_deadline_is_rejected(self):
        from unittest.mock import patch
        c = self.console()
        now = [0.0]
        def poll():
            now[0] = 1.01
            c.raw.extend(b'RESTORE_OK')
            return 0
        c.poll = poll
        with patch('windows_conpty.time.monotonic', side_effect=lambda: now[0]):
            with self.assertRaisesRegex(AssertionError, 'timeout waiting for RESTORE_OK'):
                c.wait('RESTORE_OK')

    def test_marker_split_across_notifications_and_after_offset(self):
        from unittest.mock import patch
        c = self.console()
        c.raw.extend(b'RESTORE_OK old output')
        offset = len(c.raw)
        now = [0.0]
        chunks = iter((b'REST', b'ORE_', b'OK'))
        class ScheduledCondition(threading.Condition):
            def wait(self, timeout):
                now[0] += timeout
                c.raw.extend(next(chunks))
        c.output_changed = ScheduledCondition()
        with patch('windows_conpty.time.monotonic', side_effect=lambda: now[0]):
            self.assertEqual(c.wait('RESTORE_OK', after=offset), offset + len(b'RESTORE_OK'))
        self.assertAlmostEqual(now[0], .06)

    def test_reader_eof_errors_and_capture_limit(self):
        import ctypes
        from unittest.mock import patch
        for kind in ('zero', 'broken', 'invalid', 'aborted', 'exception', 'limit'):
            for closed in (False, True):
                with self.subTest(kind=kind, closed=closed):
                    c = self.console()
                    c.closed = closed
                    def read(handle, buf, length, size, overlapped):
                        if kind == 'exception': raise OSError('injected reader error')
                        if kind == 'limit':
                            c.raw.extend(b'x' * (4 * 1024 * 1024))
                            ctypes.memmove(buf, b'RESTORE_OK', 10)
                            size._obj.value = 10
                            return True
                        size._obj.value = 0
                        return kind == 'zero'
                    code = {'broken': 109, 'invalid': 6, 'aborted': 995}.get(kind, 5)
                    c.k = SimpleNamespace(ReadFile=read)
                    with patch.object(ctypes, 'get_last_error', return_value=code, create=True):
                        c.read()
                    self.assertTrue(c.reader_done)
                    expected = {'exception': 'injected reader error', 'limit': 'capture exceeded 4 MiB'}
                    message = expected.get(kind)
                    if kind in ('invalid', 'aborted') and not closed:
                        message = 'ReadFile WinError ' + str(code)
                    if message:
                        self.assertIn(message, c.error)
                    else:
                        self.assertIsNone(c.error)
                        message = 'EOF.*before marker'
                    with self.assertRaisesRegex(AssertionError, message):
                        c.wait('RESTORE_OK')

    def test_child_exit_during_exchange_reports_status_without_timeout(self):
        import json
        import tempfile
        from pathlib import Path
        from unittest.mock import patch
        for code in (0, 1):
            with self.subTest(code=code), tempfile.TemporaryDirectory() as root:
                c = self.console()
                c.poll = lambda: None  # Persistent owner awaits its probe input.
                c.status_path = Path(root) / 'status.json'
                c.status_path.write_text(json.dumps({'phase': 'probe', 'exit': code}))
                c.raw.extend(b'console resources did not settle: baseline=[118 2] actual=[124 2]\n' + b'x' * 7000 + b'\nRESTORE_READY_nonce')
                with patch.object(c.output_changed, 'wait', side_effect=AssertionError('must not wait')):
                    with self.assertRaisesRegex(AssertionError, "helper exited.*REUSE_READY 24") as caught:
                        c.wait('REUSE_READY 24', child_nonce='nonce')
                self.assertIn("'exit': " + str(code), str(caught.exception))
                self.assertIn('actual=[124 2]', str(caught.exception))

    def test_child_restore_marker_nonce_fragmentation_and_success(self):
        import json
        import tempfile
        from pathlib import Path
        with tempfile.TemporaryDirectory() as root:
            c = self.console()
            c.status_path = Path(root) / 'status.json'
            c.status_path.write_text(json.dumps({'exit': 0}))
            for raw in (b'RESTORE_READY_other\nOK', b'RESTORE_READY_non\nOK',
                        b'OK\nRESTORE_READY_nonce'):
                c.raw = bytearray(raw)
                self.assertEqual(c.wait('OK', child_nonce='nonce'), len(raw))

    def test_owner_native_error_wins_over_captured_marker(self):
        import json
        import tempfile
        from pathlib import Path
        c = self.console()
        c.raw.extend(b'RESTORE_OK')
        with tempfile.TemporaryDirectory() as root:
            c.status_path = Path(root) / 'status.json'
            c.status_path.write_text(json.dumps({'error': 'native failure WinError 6'}))
            with self.assertRaisesRegex(AssertionError, 'native failure WinError 6'):
                c.wait('RESTORE_OK')


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
        import subprocess
        from windows_conpty import powershell_argv
        args = ["C:/test é's/cli.exe", 'add', 'C:/fixture $x', 'embedded"quote', '']
        env = {'HOME': "C:/CLI é's/home", 'PATH': 'C:/empty', 'EMPTY': ''}
        argv = powershell_argv('pwsh.exe', args, 'nonce', env)
        self.assertEqual(argv[:5], ['pwsh.exe', '-NoLogo', '-NoProfile', '-NonInteractive', '-EncodedCommand'])
        command = base64.b64decode(argv[-1]).decode('utf-16-le')
        quote = lambda value: "'" + value.replace("'", "''") + "'"
        self.assertIn('$start.FileName = ' + quote(args[0]), command)
        self.assertIn('$start.Arguments = ' + quote(subprocess.list2cmdline(args[1:])), command)
        self.assertIn('$start.EnvironmentVariables.Clear();', command)
        for key, value in env.items():
            encoded = base64.b64encode(value.encode('utf-16-le')).decode('ascii')
            self.assertIn('$start.EnvironmentVariables[' + quote(key) + '] = '
                          "[System.Text.Encoding]::Unicode.GetString([System.Convert]::FromBase64String('"
                          + encoded + "'));", command)
        self.assertIn('$start.UseShellExecute = $false', command)
        self.assertNotIn('RedirectStandard', command)
        self.assertNotIn('$env:', command)
        self.assertIn("Write-Output 'POWERSHELL_LAUNCH_nonce'", command)
        self.assertTrue(command.endswith('$child.WaitForExit(); exit $child.ExitCode'))

    def test_powershell_environment_values_round_trip_without_literal_interpolation(self):
        import base64
        import re
        from windows_conpty import powershell_argv
        values = ["O’Brien", '\u2018left\u2019right\u201alow\u201breversed',
                  'Unicode é 中文 😀', "ASCII O'Brien", '`backtick`n',
                  '$HOME $(throw "must not execute")', 'line1\r\nline2\nline3',
                  '', "’; throw 'must not execute'; #", '“double” „quotes‟']
        env = {'VALUE_' + str(i): value for i, value in enumerate(values)}
        before = dict(env)
        command = base64.b64decode(
            powershell_argv('pwsh.exe', ['cli.exe'], 'nonce', env)[-1]
        ).decode('utf-16-le')
        assignments = re.findall(
            r"\$start\.EnvironmentVariables\['(VALUE_\d+)'\] = "
            r"\[System.Text.Encoding\]::Unicode.GetString\("
            r"\[System.Convert\]::FromBase64String\('([A-Za-z0-9+/=]*)'\)\);",
            command)
        self.assertEqual(len(assignments), len(env))
        self.assertEqual({key: base64.b64decode(value, validate=True).decode('utf-16-le')
                          for key, value in assignments}, env)
        self.assertEqual(env, before)

    def test_owner_error_is_reported_before_prompt_timeout(self):
        import json
        import tempfile
        from pathlib import Path
        with tempfile.TemporaryDirectory() as root:
            c = ConPTY.__new__(ConPTY)
            c.timeout, c.error, c.raw = 1, None, bytearray()
            c.output_changed, c.reader_done = threading.Condition(), False
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
            c.output_changed, c.reader_done = threading.Condition(), False
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

    def test_shell_profile_is_separate_without_changing_cli_baseline(self):
        import tempfile
        from pathlib import Path
        from windows_conpty import prepare_powershell_fixture
        with tempfile.TemporaryDirectory() as root:
            fixture = self.make_fixture(root)
            before, env = fixture.before, dict(fixture.env)
            shell_env = prepare_powershell_fixture(fixture)
            self.assertIs(fixture.before, before)
            self.assertEqual(fixture.env, env)
            self.assertEqual(shell_env['PATHEXT'], '.EXE')
            shell_home = fixture.root / 'powershell-home'
            for key, value in env.items():
                if Path(value).is_relative_to(fixture.home):
                    self.assertEqual(Path(shell_env[key]), shell_home / Path(value).relative_to(fixture.home))
                elif key in ('TMP', 'TEMP', 'TMPDIR'):
                    self.assertEqual(Path(shell_env[key]), fixture.root / 'powershell-tmp')
                else:
                    self.assertEqual(shell_env[key], value)
            # Reproduce the exact native startup artifact outside CLI HOME.
            cache = shell_home / 'AppData/Local/Microsoft/PowerShell/StartupProfileData-NonInteractive'
            cache.parent.mkdir(parents=True)
            cache.write_bytes(b'synthetic shell startup cache')
            fixture.unchanged()
            self.assertFalse((fixture.home / 'AppData').exists())

    def test_run_case_routes_shell_and_cli_environments_separately(self):
        import base64
        import json
        import tempfile
        from pathlib import Path
        from unittest.mock import patch
        import windows_conpty
        with tempfile.TemporaryDirectory() as root:
            fixture = self.make_fixture(root)
            args = SimpleNamespace(artifacts=Path(root), scanner_path=None,
                                   binary=Path(root) / 'never.exe',
                                   powershell=Path(root) / 'never-pwsh.exe', timeout=15)
            captured = {}
            def capture(argv, env, cwd, timeout, evidence):
                captured.update(env=env, cwd=cwd, timeout=timeout,
                                config=json.loads(Path(argv[-1]).read_text()))
                raise RuntimeError('stop before any process starts')
            with patch.object(windows_conpty, 'Fixture', return_value=fixture), \
                    patch.object(windows_conpty, 'ConPTY', side_effect=capture):
                with self.assertRaisesRegex(RuntimeError, 'stop before any process starts'):
                    windows_conpty.run_case('default-no', args)
            self.assertEqual(captured['env']['HOME'], str(fixture.root / 'powershell-home'))
            self.assertEqual(captured['cwd'], fixture.project)
            self.assertEqual(captured['timeout'], 15)
            command = base64.b64decode(captured['config']['argv'][-1]).decode('utf-16-le')
            for key, value in fixture.env.items():
                encoded = base64.b64encode(value.encode('utf-16-le')).decode('ascii')
                self.assertIn("$start.EnvironmentVariables['" + key + "'] = "
                              "[System.Text.Encoding]::Unicode.GetString([System.Convert]::FromBase64String('"
                              + encoded + "'));", command)
            self.assertNotIn('powershell-home', command)
            self.assertNotIn('TERM', fixture.env)
            fixture.unchanged()

    def test_all_cli_mutations_remain_detected(self):
        import tempfile
        from windows_conpty import prepare_powershell_fixture
        targets = ('home/.codex/config.toml', 'home/.cursor/config.json',
                   'home/unexpected', 'home/AppData/Local/Microsoft/PowerShell/cache',
                   'home/AppData/Local/Microsoft/PowerShell/StartupProfileData-NonInteractive',
                   'home/localappdata/cache', 'home/appdata/config',
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

    def test_removed_cli_directory_is_detected(self):
        import tempfile
        from windows_conpty import prepare_powershell_fixture
        with tempfile.TemporaryDirectory() as root:
            fixture = self.make_fixture(root)
            prepare_powershell_fixture(fixture)
            (fixture.home / '.cursor').rmdir()
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
