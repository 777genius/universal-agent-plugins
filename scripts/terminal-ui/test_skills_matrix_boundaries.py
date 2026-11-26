"""Fault controls for preconsent effects and private executable provenance."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, call, patch

import skills_matrix as matrix


class NativeCallBoundaryTests(unittest.TestCase):
    def test_launch_mutation_is_rejected_even_when_it_writes_no_durable_bytes(self):
        for mutation in (['plugin', 'add', 'new@market'], ['plugin', 'remove', 'old@market'],
                         ['plugin', 'marketplace', 'add', 'root'],
                         ['plugin', 'marketplace', 'remove', 'market']):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                calls = [['plugin', 'add', 'seed@market']]
                fixture = SimpleNamespace(
                    home=root / 'home', project=root / 'project', data=root / 'data', root=root,
                    calls=lambda: [list(row) for row in calls], unchanged=Mock())
                def launch(*_args, **_kwargs):
                    calls.append(mutation)
                    return SimpleNamespace()
                with patch.object(matrix, 'Session', side_effect=launch), \
                        patch.object(matrix, 'WindowsSession', side_effect=launch):
                    runner = matrix.Matrix('cli', 'native')
                    session = runner.session(['cli'], fixture, root)
                with self.assertRaisesRegex(AssertionError, 'native mutation'):
                    runner.assert_unapplied(session, fixture)

    def test_seed_effects_and_new_readonly_probes_are_allowed_but_log_truncation_is_not(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            calls = [['plugin', 'add', 'seed@market'], ['plugin', 'list', '--json']]
            fixture = SimpleNamespace(
                root=root, home=root / 'home', project=root / 'project', data=root / 'data',
                calls=lambda: [list(row) for row in calls], unchanged=Mock())
            with patch.object(matrix, 'Session', return_value=SimpleNamespace()), \
                    patch.object(matrix, 'WindowsSession', return_value=SimpleNamespace()):
                runner = matrix.Matrix('cli', 'native')
                session = runner.session(['cli'], fixture, root)
            calls.extend([['--version'], ['plugin', 'list', '--json'], ['app-server']])
            runner.assert_unapplied(session, fixture)
            calls.pop(0)
            with self.assertRaisesRegex(AssertionError, 'native mutation'):
                runner.assert_unapplied(session, fixture)


class ProvenanceEnvironmentTests(unittest.TestCase):
    def test_git_cannot_be_redirected_but_existing_safety_and_network_policies_survive(self):
        overrides = {key: 'foreign-repository' for key in matrix.GIT_REPOSITORY_OVERRIDES}
        overrides.update(GIT_CONFIG_KEY_0='core.worktree', GIT_CONFIG_VALUE_0='foreign-root')
        preserved = dict(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
                         GIT_NO_LAZY_FETCH='1', GIT_TERMINAL_PROMPT='0',
                         GIT_SSH_COMMAND='ssh -F isolated-config', HTTPS_PROXY='http://policy.invalid')
        with patch.dict(matrix.os.environ, {**overrides, **preserved}, clear=True), \
                patch.object(matrix.subprocess, 'check_output', side_effect=['a' * 40, '']) as git:
            _checkout, head, dirty = matrix.harness_checkout()
        self.assertEqual((head, dirty), ('a' * 40, False))
        for call in git.call_args_list:
            self.assertEqual(call.kwargs['env'], preserved)
            self.assertEqual(call.args[0][1], '-C')


@unittest.skipUnless(shutil.which('git'), 'real Git is required')
class RealGitProvenanceTests(unittest.TestCase):
    """Use actual local repositories; no command/output mocks qualify these cases."""
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.checkout = self.root / 'checkout'
        self.checkout.mkdir()
        self.environment = matrix.harness_git_environment()
        self.environment.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
                                GIT_CONFIG_SYSTEM=os.devnull, GIT_TERMINAL_PROMPT='0')
        self.git('init', '-q', str(self.checkout))
        self.git('-C', str(self.checkout), 'config', 'user.name', 'Provenance Fixture')
        self.git('-C', str(self.checkout), 'config', 'user.email', 'fixture@example.invalid')
        self.script = self.checkout / 'scripts' / 'terminal-ui' / 'skills_matrix.py'
        self.script.parent.mkdir(parents=True)
        self.script.write_text('# tracked harness fixture\n', encoding='utf-8')
        (self.checkout / 'tracked.txt').write_text('committed bytes\n', encoding='utf-8')
        self.git('-C', str(self.checkout), 'add', '.')
        self.git('-C', str(self.checkout), 'commit', '-qm', 'local fixture')
        self.commit = self.git('-C', str(self.checkout), 'rev-parse', 'HEAD').strip()

    def git(self, *args):
        return subprocess.check_output(['git', *args], env=self.environment,
            text=True, stderr=subprocess.STDOUT, timeout=5)

    def observe(self, script=None):
        with patch.object(matrix, '__file__', str(script or self.script)), \
                patch.dict(matrix.os.environ, self.environment, clear=True):
            checkout, head, dirty = matrix.harness_checkout()
        self.assertEqual(head, self.commit)
        return checkout, dirty

    def test_config_cannot_hide_new_untracked_harness_files(self):
        self.git('-C', str(self.checkout), 'config', 'status.showUntrackedFiles', 'no')
        self.assertEqual(self.observe(), (self.checkout, False))
        (self.script.parent / 'untracked-fixture.py').write_text('# untracked evidence code\n')
        # Control: the inherited repository policy really conceals this file.
        self.assertEqual(self.git('-C', str(self.checkout), 'status', '--porcelain'), '')
        self.assertEqual(self.observe(), (self.checkout, True))

    def test_config_cannot_substitute_clean_alternate_tracked_tree(self):
        alternate = self.root / 'alternate'
        shutil.copytree(self.checkout, alternate, ignore=shutil.ignore_patterns('.git'))
        (self.checkout / 'tracked.txt').write_text('dirty actual harness tree\n')
        self.git('-C', str(self.checkout), 'config', 'core.worktree', str(alternate))
        # Control: -C alone observes the clean configured copy, not this harness.
        self.assertEqual(self.git('-C', str(self.checkout), 'status', '--porcelain'), '')
        self.assertEqual(self.observe(), (self.checkout, True))

    def test_linked_worktree_keeps_own_head_and_dirty_state(self):
        linked = self.root / 'linked'
        self.git('-C', str(self.checkout), 'worktree', 'add', '--detach', str(linked), self.commit)
        linked_script = linked / self.script.relative_to(self.checkout)
        self.assertEqual(self.observe(linked_script), (linked, False))
        (linked / 'tracked.txt').write_text('dirty linked worktree\n')
        self.assertEqual(self.observe(linked_script), (linked, True))
        self.assertEqual(self.observe(), (self.checkout, False))


class ExecutableBoundaryTests(unittest.TestCase):
    @unittest.skipUnless(os.name == 'posix', 'POSIX nonblocking FIFO contract')
    def test_fifo_before_copy_and_fifo_replacement_before_hash_fail_without_a_writer(self):
        # A regression must time out only this disposable child, never the test
        # runner. The child has no subprocesses and owns its temporary FIFO.
        program = """
import pathlib, sys, tempfile, os
sys.path.insert(0, sys.argv[1])
import skills_matrix as matrix
with tempfile.TemporaryDirectory() as temporary:
    root = pathlib.Path(temporary)
    source = root / 'source'
    if sys.argv[2] == 'copy':
        os.mkfifo(source)
        operation = lambda: matrix.copy_product_binary(source, root)
    else:
        source.write_bytes(b'original executable')
        copied = matrix.copy_product_binary(source, root)
        source.unlink()
        os.mkfifo(source)
        assert copied.read_bytes() == b'original executable'
        operation = lambda: matrix.executable_digest(source)
    try:
        operation()
    except AssertionError as error:
        assert 'not a regular file' in str(error)
    else:
        raise AssertionError('FIFO admitted as executable')
"""
        for mode in ('copy', 'hash-after-copy'):
            with self.subTest(mode=mode):
                result = subprocess.run([sys.executable, '-c', program,
                    str(Path(matrix.__file__).resolve().parent), mode],
                    capture_output=True, timeout=5, check=False)
                self.assertEqual(result.returncode, 0, result.stderr.decode('utf-8', 'replace'))

    @unittest.skipUnless(os.name == 'posix', 'POSIX opened-inode path replacement')
    def test_opened_regular_inode_is_copied_even_if_source_path_is_replaced(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            source = root / 'source'
            source.write_bytes(b'opened trusted inode')
            replacement = root / 'replacement'
            replacement.write_bytes(b'replacement pathname bytes')
            open_source = matrix.open_executable
            def replace_after_open(path):
                stream = open_source(path)
                replacement.replace(source)
                return stream
            with patch.object(matrix, 'open_executable', side_effect=replace_after_open):
                copied = matrix.copy_product_binary(source, root)
            self.assertEqual(copied.read_bytes(), b'opened trusted inode')
            self.assertNotEqual(matrix.executable_digest(source), matrix.executable_digest(copied))

    def test_supplied_responder_is_snapshotted_before_matrix_and_only_copy_is_launched(self):
        for tamper in (False, True):
            with self.subTest(tamper=tamper), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                binary, supplied = root / 'cli', root / 'shared-native'
                binary.write_bytes(b'CLI bytes')
                supplied.write_bytes(b'qualified native bytes')
                artifacts = root / 'artifacts'
                argv = ['skills_matrix.py', '--binary', str(binary), '--native-fixture', str(supplied),
                        '--product-sha', 'a' * 40, '--artifacts', str(artifacts), '--case', 'fresh']
                def scanner(args):
                    args.scanner_path = None
                    return {}
                def run(_name, evidence):
                    evidence.mkdir()
                    native = runner.call_args.args[1]
                    self.assertNotEqual(native, supplied)
                    self.assertFalse(os.path.samefile(native, supplied))
                    supplied.write_bytes(b'shared file replaced temporarily')
                    self.assertEqual(native.read_bytes(), b'qualified native bytes')
                    supplied.write_bytes(b'qualified native bytes')
                    if tamper:
                        native.write_bytes(b'mutated actual responder')
                with patch.object(matrix.sys, 'argv', argv), \
                        patch.object(matrix, 'prepare_scanner', side_effect=scanner), \
                        patch.object(matrix, 'harness_checkout', return_value=(root, 'b' * 40, False)), \
                        patch.object(matrix, 'Matrix') as runner, patch('builtins.print'):
                    runner.return_value.run_case.side_effect = run
                    self.assertEqual(matrix.main(), int(tamper))
                report = json.loads((artifacts / 'report.json').read_text(encoding='utf-8'))
                self.assertEqual(report['supplied_native_fixture_path'], str(supplied))
                self.assertEqual(report['native_fixture_path'], str(runner.call_args.args[1]))
                self.assertEqual(report['cases'][0]['status'], 'fail' if tamper else 'pass')


class CrashWindowTests(unittest.TestCase):
    def test_responder_exiting_during_evidence_write_refuses_the_crash_window(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            evidence = root / 'evidence'
            evidence.mkdir()
            marker = root / 'native-add-paused'
            marker.write_text(json.dumps(dict(nonce='nonce', pid=300, ppid=200)), encoding='utf-8')
            fixture = SimpleNamespace(root=root, bin=root / 'bin', project=root, env={},
                                      args=Mock(return_value=['cli']), control=Mock())
            process = SimpleNamespace(pid=100, returncode=None)
            events = []
            def kill():
                events.append('cli-kill')
                process.returncode = -9
            process.kill = kill
            process.poll = lambda: process.returncode
            process.communicate = Mock(return_value=(b'', b''))
            held = [dict(role='native', fd=30, identity={}),
                    dict(role='supervisor', fd=20, identity={})]
            real_persist = matrix.persist
            def persist(path, value):
                real_persist(path, value)
                if path.name == 'owned-native-identity.json':
                    events.append('identity-written')
            def exited(_fd):
                self.assertEqual(events, ['identity-written'])
                events.append('responder-exited')
                return True
            # Model the complete Linux kernel API used by this refusal path.
            # Windows select has neither poll nor POLLIN; no host constant or
            # actual process handle may leak into this portable fault control.
            poller = Mock()
            poller.poll.return_value = [(30, 1)]
            linux_select = SimpleNamespace(poll=Mock(return_value=poller), POLLIN=1)
            linux_os = SimpleNamespace(pidfd_open=Mock(), close=Mock())
            linux_signal = SimpleNamespace(SIGKILL=9, pidfd_send_signal=Mock())
            with patch.object(matrix.uuid, 'uuid4', return_value=SimpleNamespace(hex='nonce')), \
                    patch.object(matrix.sys, 'platform', 'linux'), \
                    patch.object(matrix, 'os', linux_os), \
                    patch.object(matrix, 'signal', linux_signal), \
                    patch.object(matrix, 'select', linux_select), \
                    patch.object(matrix.subprocess, 'Popen', return_value=process), \
                    patch.object(matrix, 'hold_native_lineage', return_value=held), \
                    patch.object(matrix, 'persist', side_effect=persist), \
                    patch.object(matrix, 'linux_pidfd_exited', side_effect=exited) as pidfd_exited:
                with self.assertRaisesRegex(AssertionError, 'responder exited before'):
                    matrix.Matrix('cli', 'native').case_recovery_refusal(fixture, evidence)
            # The only CLI kill is finally-owned cleanup after refusal, not the
            # requested crash which could otherwise produce a false qualification.
            self.assertEqual(events, ['identity-written', 'responder-exited', 'cli-kill'])
            pidfd_exited.assert_called_once_with(30)
            self.assertEqual(linux_signal.pidfd_send_signal.call_args_list,
                             [call(30, 9), call(20, 9)])
            self.assertEqual(poller.register.call_args_list, [call(30, 1), call(20, 1)])
            self.assertEqual(poller.poll.call_args_list, [call(3000), call(3000)])
            self.assertEqual(linux_os.close.call_args_list, [call(30), call(20)])
            cleanup = json.loads((evidence / 'crash-cleanup.json').read_text(encoding='utf-8'))
            self.assertEqual(cleanup, dict(cli_reaped=True, native_identity_confirmed=True,
                                          lineage_roles=['native', 'supervisor'], cleanup_errors=[]))


if __name__ == '__main__':
    unittest.main()
