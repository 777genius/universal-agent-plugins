"""Fault controls for consent state, fixture cleanup and executed-byte evidence."""
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import skills_matrix as matrix
from skills_oracles import durable_tree


class ConsentStateTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.fixture = SimpleNamespace(
            root=self.root, home=self.root / 'home', project=self.root / 'project',
            data=self.root / 'data', skill_name='guide', args=Mock(return_value=['cli']),
            unchanged=Mock(), calls=Mock(return_value=[]))
        for path in (self.fixture.home, self.fixture.project, self.fixture.data):
            path.mkdir()
        for name in ('uap.toml', 'uap.lock', 'state-v2.json'):
            (self.fixture.data / name).write_text('original saved authority')
        self.evidence = self.root / 'evidence'
        self.evidence.mkdir()
        self.runner = matrix.Matrix('cli', 'native')
        self.session = SimpleNamespace(
            raw=('guide ' + str(self.root)).encode(), wait=Mock(), send=Mock(),
            finish=Mock(), close=Mock())

    def launch(self, *_args, **_kwargs):
        return self.session

    def test_mutation_during_launch_is_detected_before_consent(self):
        def launch(*args, **kwargs):
            (self.fixture.data / 'uap.toml').write_text('prematurely saved intent')
            return self.launch(*args, **kwargs)
        with patch.object(matrix, 'Session', side_effect=launch), \
                patch.object(matrix, 'WindowsSession', side_effect=launch):
            with self.assertRaisesRegex(AssertionError, 'durable saved intent/state'):
                self.runner.canceled_review(self.fixture, self.evidence, b'n\n')
        self.session.send.assert_not_called()
        self.session.close.assert_called_once()

    def test_decline_control_c_and_eof_cannot_hide_saved_state_writes(self):
        for action, filename in ((b'n\n', 'uap.toml'), (b'\x03', 'uap.lock'),
                                 (b'\x04', 'state-v2.json')):
            with self.subTest(action=action):
                path = self.fixture.data / filename
                original = path.read_bytes()
                self.session.finish.side_effect = lambda _code: path.write_text('unconsented state')
                with patch.object(matrix, 'Session', side_effect=self.launch), \
                        patch.object(matrix, 'WindowsSession', side_effect=self.launch):
                    with self.assertRaisesRegex(AssertionError, 'durable saved intent/state'):
                        self.runner.canceled_review(self.fixture, self.evidence, action)
                path.write_bytes(original)

    def test_acquisition_cache_and_logs_do_not_make_cancellation_fail(self):
        before = durable_tree(self.fixture)
        def finish(_code):
            for name in ('logs', 'source-cache'):
                directory = self.fixture.data / name
                directory.mkdir()
                (directory / 'acquired').write_text('allowed evidence/cache')
        self.session.finish.side_effect = finish
        with patch.object(matrix, 'Session', side_effect=self.launch), \
                patch.object(matrix, 'WindowsSession', side_effect=self.launch):
            self.runner.canceled_review(self.fixture, self.evidence, b'n\n')
        self.assertEqual(durable_tree(self.fixture), before)


class CleanupAndResultTests(unittest.TestCase):
    def test_source_evidence_and_cleanup_evidence_failures_still_close_publisher(self):
        for all_writes_fail in (False, True):
            with self.subTest(all_writes_fail=all_writes_fail), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                fixture = SimpleNamespace(
                    source_kind='publisher', source='https://fixture.invalid',
                    publisher=SimpleNamespace(files={'SKILL.md': b'Skill'}, requests=[]),
                    root=root, data=root / 'data', home=root / 'home', project=root / 'project',
                    calls=Mock(return_value=[]), close=Mock())
                def persist(path, _value):
                    if all_writes_fail or path.name == 'source.json':
                        raise OSError('evidence output failed')
                with patch.object(matrix, 'SkillsFixture', return_value=fixture), \
                        patch.object(matrix, 'persist', side_effect=persist):
                    with self.assertRaisesRegex(OSError, 'evidence output failed'):
                        matrix.Matrix('cli', 'native').run_case('fresh', root / 'evidence')
                fixture.close.assert_called_once()

    def test_fresh_refuses_failed_skipped_or_unchanged_agent_status(self):
        for status in ('failed', 'skipped', 'unchanged', None):
            with self.subTest(status=status):
                result = {'items': [{'targetId': 'codex', 'status': status},
                                    {'targetId': 'cursor', 'status': 'succeeded'}]}
                runner = matrix.Matrix('cli', 'native')
                with patch.object(runner, 'command', return_value=result), \
                        patch.object(matrix, 'installed') as installed:
                    with self.assertRaisesRegex(AssertionError, 'every selected agent'):
                        runner.case_fresh(SimpleNamespace(), Path('unused'))
                installed.assert_not_called()


class ExecutedProvenanceTests(unittest.TestCase):
    def test_git_provenance_uses_script_checkout_from_unrelated_cwd(self):
        with tempfile.TemporaryDirectory() as temporary:
            previous = Path.cwd()
            try:
                os.chdir(temporary)
                with patch.object(matrix.subprocess, 'check_output',
                                  side_effect=['a' * 40 + '\n', ' M tracked.py\n']) as output:
                    checkout, head, dirty = matrix.harness_checkout()
            finally:
                os.chdir(previous)
        self.assertEqual(checkout, Path(matrix.__file__).resolve().parents[2])
        self.assertEqual(head, 'a' * 40)
        self.assertTrue(dirty)
        for call in output.call_args_list:
            self.assertEqual(call.args[0][:3], ['git', '-C', str(checkout)])

    def test_shared_binary_replace_and_restore_cannot_change_executed_copy(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            supplied = root / 'shared-cli'
            original = b'qualified immutable CLI bytes'
            supplied.write_bytes(original)
            artifacts = root / 'evidence'
            artifacts.mkdir()
            private = matrix.copy_product_binary(supplied, artifacts)
            self.assertFalse(os.path.samefile(supplied, private))
            self.assertNotEqual(private.resolve(), supplied.resolve())
            replacement = root / 'replacement'
            replacement.write_bytes(b'unqualified replacement executable')
            replacement.replace(supplied)
            self.assertEqual(private.read_bytes(), original)
            supplied.write_bytes(original)
            self.assertEqual(hashlib.sha256(private.read_bytes()).digest(),
                             hashlib.sha256(original).digest())

    def test_main_launches_copy_and_rejects_copy_mutation(self):
        for mutate, alias in ((False, False), (False, True), (True, False), (True, True)):
            with self.subTest(mutate=mutate, alias=alias), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                if alias:
                    (root / 'path alias Ω').mkdir()
                    root = root / 'path alias Ω' / '..'
                supplied, native = root / 'cli', root / 'native'
                supplied.write_bytes(b'CLI bytes')
                native.write_bytes(b'responder bytes')
                canonical_supplied = supplied.resolve(strict=True)
                if alias:
                    self.assertNotEqual(str(supplied), str(canonical_supplied))
                artifacts = root / 'evidence'
                argv = ['skills_matrix.py', '--binary', str(supplied), '--product-sha', 'a' * 40,
                        '--native-fixture', str(native), '--artifacts', str(artifacts), '--case', 'fresh']
                def scanner(args):
                    args.scanner_path = None
                    return {}
                def run(_name, evidence):
                    evidence.mkdir()
                    launched = Path(runner.call_args.args[0])
                    self.assertNotEqual(launched.resolve(strict=True), canonical_supplied)
                    self.assertFalse(os.path.samefile(launched, supplied))
                    self.assertEqual(launched.read_bytes(), b'CLI bytes')
                    if mutate:
                        launched.write_bytes(b'changed execution copy')
                with patch.object(matrix.sys, 'argv', argv), \
                        patch.object(matrix, 'prepare_scanner', side_effect=scanner), \
                        patch.object(matrix, 'harness_checkout', return_value=(root, 'b' * 40, False)), \
                        patch.object(matrix, 'Matrix') as runner, patch('builtins.print'):
                    runner.return_value.run_case.side_effect = run
                    self.assertEqual(matrix.main(), int(mutate))
                report = json.loads((artifacts / 'report.json').read_text(encoding='utf-8'))
                expected_copy = artifacts.resolve(strict=True) / 'product' / (
                    'agentplugins.exe' if os.name == 'nt' else 'agentplugins')
                self.assertEqual(report['supplied_binary_path'], str(canonical_supplied))
                self.assertEqual(report['binary_path'], str(expected_copy))
                self.assertEqual(Path(runner.call_args.args[0]), expected_copy)
                self.assertEqual(report['native_fixture_path'], str(native.resolve(strict=True)))
                self.assertEqual(report['binary_sha256'], hashlib.sha256(b'CLI bytes').hexdigest())
                self.assertEqual(report['cases'][0]['status'], 'fail' if mutate else 'pass')
                if mutate:
                    self.assertIn('input CLI binary changed during qualification',
                                  report['cases'][0]['error'])


if __name__ == '__main__':
    unittest.main()
