"""Restart sequencing controls; strict prepared/native ownership has its own oracle tests."""
import json
from pathlib import Path
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import call, patch

import skills_matrix as matrix


class PreparedRestartTests(unittest.TestCase):
    def setUp(self):
        self.prepare_fixture()

    def prepare_fixture(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.evidence = self.root / 'evidence'
        self.evidence.mkdir()
        self.calls = []
        self.fixture = SimpleNamespace(root=self.root, home=self.root / 'home',
            project=self.root / 'project', data=self.root / 'data', env={},
            skill_name='selected-skill', calls=lambda: list(self.calls))
        for path in (self.fixture.home, self.fixture.project, self.fixture.data / 'operations-v2',
                     self.fixture.data / 'managed'):
            path.mkdir(parents=True)
        self.foreign = self.fixture.home / 'foreign.txt'
        self.foreign.write_bytes(b'foreign bytes')
        self.foreign_before = matrix.Matrix.recovery_foreign_entries((self.foreign,))
        (self.root / 'pause-add').write_text('owned crash nonce', encoding='utf-8')
        (self.root / 'native-registry.json').write_text(json.dumps(
            dict(marketplace='owned-market', spec='selected@owned-market')), encoding='utf-8')
        self.binding = dict(client_id='codex', client_binding_id='binding-id',
            activation='prepared', verification='package_validated', updated_at='before',
            package_revision=dict(tree_digest='tree-pin', manifest_digest='manifest-pin'),
            receipts=[dict(phase='committed', operation_id='committed-directory')])
        self.state = dict(installations=[dict(installation_id='installation-id',
            source=dict(canonical_source='original-source'), updated_at='before',
            clients={'binding-id': self.binding})], environment_receipts=[dict(receipt_id='owner')])
        self.write_state()
        self.commands = []
        self.fault = None

    def write_state(self):
        (self.fixture.data / 'state-v2.json').write_text(json.dumps(self.state), encoding='utf-8')

    def public_command(self, argv, **kwargs):
        self.commands.append(argv)
        self.assertEqual(kwargs['stdin'], subprocess.DEVNULL)
        self.assertEqual(kwargs['timeout'], 12)
        command = argv[1]
        if command == 'info':
            self.assertNotIn('--target', argv)
            client = dict(client_id='codex', materialization='materialized',
                activation=self.binding['activation'], verification=self.binding['verification'],
                package_revision=self.binding['package_revision'])
            if self.fault == 'info-promotes' and len(self.commands) == 1:
                client.update(activation='active', verification='installation_verified')
            if self.fault == 'info-writes' and len(self.commands) == 1:
                self.foreign.write_bytes(b'unexpected inspection effect')
            value = dict(command='info', data=dict(installation_id='installation-id', clients=[client]))
        else:
            self.assertEqual(command, 'repair')
            self.assertEqual(argv[-3:], ['--target', 'codex', '--yes'])
            self.binding.update(activation='active', verification='installation_verified',
                                updated_at='after')
            self.state['installations'][0]['updated_at'] = 'after'
            mutations = [['plugin', 'marketplace', 'update', 'owned-market', '--json'],
                         ['plugin', 'add', 'selected@owned-market', '--json']]
            if self.fault == 'native-missing':
                mutations.pop()
            if self.fault == 'native-extra':
                mutations.append(['plugin', 'remove', 'selected@owned-market', '--json'])
            if self.fault == 'native-duplicate':
                mutations.append(list(mutations[-1]))
            if self.fault == 'native-foreign':
                mutations[1][2] = 'foreign@foreign-market'
            self.calls.extend(mutations)
            if self.fault == 'source-rebound':
                self.state['installations'][0]['source']['canonical_source'] = 'foreign-source'
            if self.fault == 'receipt-replaced':
                self.binding['receipts'][0]['operation_id'] = 'replacement-operation'
            if self.fault == 'foreign-clobbered':
                self.foreign.write_bytes(b'clobbered bytes')
            if self.fault == 'foreign-symlink':
                self.replace_foreign_with_same_bytes_symlink()
            self.write_state()
            action = dict(action='repair', alias='selected-skill', client_id='codex',
                installation_id='installation-id', binding_id='binding-id', package_digest='tree-pin')
            result = dict(operation='repair', outcome='completed', client_id='codex',
                          installation_id='installation-id')
            if self.fault == 'foreign-result':
                result['installation_id'] = 'foreign-installation'
            value = dict(command='agentplugins repair', outcome='completed', scope='user',
                         actions=[action], results=[result])
        value.update(schema_version=1, result='success')
        return SimpleNamespace(returncode=0, stdout=json.dumps(value).encode(), stderr=b'')

    def complete(self):
        # This seam test cannot qualify native bytes. The independent installed
        # oracle must be invoked both before restart and after successful repair.
        with patch.object(matrix, 'installed') as installed, \
                patch.object(matrix.subprocess, 'run', side_effect=self.public_command):
            try:
                matrix.Matrix('private-cli', 'private-native', acquisition_timeout=12).\
                    complete_interrupted_activation_repair(self.fixture, self.evidence,
                                                           self.foreign_before)
            finally:
                self.oracle_calls = installed.call_args_list

    def test_recorded_info_then_explicit_repair_then_persisted_info(self):
        self.complete()
        self.assertEqual([argv[1] for argv in self.commands], ['info', 'repair', 'info'])
        self.assertEqual(self.oracle_calls, [
            call(self.fixture, (), prepared_targets=('codex',)), call(self.fixture, ('codex',))])
        facts = json.loads((self.evidence / 'interrupted-activation-repair-facts.json').read_text(encoding='utf-8'))
        self.assertEqual(facts['recorded_before'], 'prepared/package_validated')
        self.assertEqual(facts['recorded_after'], 'active/installation_verified')
        self.assertFalse(facts['physical_journal_qualification'])
        self.assertTrue(facts['native_replayed'])
        self.assertEqual(facts['authorized_native_mutations'], [
            ['plugin', 'marketplace', 'update', 'owned-market', '--json'],
            ['plugin', 'add', 'selected@owned-market', '--json']])

    def test_inspection_cannot_promote_or_mutate_before_explicit_repair(self):
        for fault in ('info-promotes', 'info-writes'):
            with self.subTest(fault=fault):
                self.fault = fault
                self.commands.clear()
                (self.root / 'pause-add').write_text('owned crash nonce', encoding='utf-8')
                self.foreign.write_bytes(b'foreign bytes')
                with self.assertRaises(AssertionError):
                    self.complete()
                self.assertEqual([argv[1] for argv in self.commands], ['info'])

    def test_completed_repair_preserves_authority_bytes_and_exact_native_registration(self):
        for fault in ('source-rebound', 'receipt-replaced', 'foreign-clobbered',
                      'native-missing', 'native-extra', 'native-foreign', 'foreign-result'):
            with self.subTest(fault=fault):
                # Each fault starts at its own prepared installation; a previous
                # negative must not contaminate the next boundary control.
                self.prepare_fixture()
                self.fault = fault
                with self.assertRaises(AssertionError):
                    self.complete()

    def replace_foreign_with_same_bytes_symlink(self):
        target = self.root / 'same-foreign-bytes.txt'
        target.write_bytes(self.foreign.read_bytes())
        self.foreign.unlink()
        try:
            self.foreign.symlink_to(target)
        except OSError as error:
            if isinstance(error, PermissionError) or getattr(error, 'winerror', None) == 1314:
                self.skipTest('symlink creation requires host permission: ' + str(error))
            raise

    def test_foreign_snapshot_is_persisted_before_cli_launch(self):
        self.fixture.control = lambda *_args: None
        self.fixture.args = lambda _binary: ['private-cli']

        def launch(*_args, **_kwargs):
            snapshot = json.loads((self.evidence / 'foreign-before-launch.json').read_text(
                encoding='utf-8'))
            paths = (self.fixture.home / 'foreign-user-é.txt',
                     self.fixture.project / 'foreign-project-é.txt')
            self.assertEqual(set(snapshot), {str(path) for path in paths})
            self.assertEqual(snapshot, matrix.Matrix.recovery_foreign_entries(paths))
            self.assertTrue(all(row['type'] == 'regular' for row in snapshot.values()))
            raise RuntimeError('stop before actual subprocess launch')

        with patch.object(matrix.sys, 'platform', 'linux'), \
                patch.object(matrix, 'os', SimpleNamespace(pidfd_open=object())), \
                patch.object(matrix, 'signal', SimpleNamespace(pidfd_send_signal=object())), \
                patch.object(matrix.subprocess, 'Popen', side_effect=launch):
            with self.assertRaisesRegex(RuntimeError, 'stop before actual subprocess launch'):
                matrix.Matrix('private-cli', 'private-native').case_interrupted_activation_repair(
                    self.fixture, self.evidence)

    def test_postcrash_foreign_corruption_is_rejected_before_restart(self):
        for fault in ('bytes', 'symlink', 'missing', 'directory'):
            with self.subTest(fault=fault):
                self.prepare_fixture()
                if fault == 'bytes':
                    self.foreign.write_bytes(b'foreign postcrash corruption')
                elif fault == 'symlink':
                    self.replace_foreign_with_same_bytes_symlink()
                else:
                    self.foreign.unlink()
                    if fault == 'directory':
                        self.foreign.mkdir()
                with self.assertRaises((AssertionError, FileNotFoundError)):
                    self.complete()
                self.assertEqual(self.commands, [])
                self.assertEqual(self.oracle_calls, [])

    def test_repair_cannot_replace_foreign_regular_entry_with_same_bytes_symlink(self):
        self.fault = 'foreign-symlink'
        with self.assertRaisesRegex(AssertionError, 'foreign entry type'):
            self.complete()
        self.assertEqual([argv[1] for argv in self.commands], ['info', 'repair'])

    def test_duplicate_allowed_native_effect_fails_exact_sequence(self):
        self.fault = 'native-duplicate'
        with self.assertRaisesRegex(AssertionError, 'exactly the recorded marketplace/plugin once'):
            self.complete()
        self.assertEqual(self.calls[-1], self.calls[-2])


if __name__ == '__main__':
    unittest.main()
