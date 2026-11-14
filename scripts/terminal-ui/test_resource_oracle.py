"""Portable protocol fault tests; never native Windows evidence."""
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import resource_oracle as oracle


class OracleTests(unittest.TestCase):
    def logs(self, root):
        for case in oracle.CASES:
            (root / ('resource-oracle-' + case + '-123.log')).write_text(
                '--- PASS: TestQualificationNativeResourceOracle (0.01s)\nPASS\n')

    def test_complete_logs_and_missing_duplicate_failed_rejection(self):
        for fault in ('none', 'missing', 'duplicate', 'failed'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                self.logs(root)
                file = next(root.glob('*.log'))
                if fault == 'missing':
                    file.unlink()
                elif fault == 'duplicate':
                    file.with_name(file.stem + '-duplicate.log').write_bytes(file.read_bytes())
                elif fault == 'failed':
                    file.write_text('--- FAIL: TestQualificationNativeResourceOracle (0.1s)\nFAIL\n')
                if fault == 'none':
                    self.assertEqual(len(oracle.verify_logs(root)), 10)
                else:
                    with self.assertRaises(AssertionError):
                        oracle.verify_logs(root)

    def test_environment_does_not_inherit_credentials_or_child_selector(self):
        with tempfile.TemporaryDirectory() as temp, patch.dict(os.environ, {
                'SystemRoot': 'C:\\Windows', 'GITHUB_TOKEN': 'secret',
                'UAP_WINDOWS_RESOURCE_ORACLE_CHILD': 'branches/plain',
                'HOME': 'real-profile', 'PATH': 'real-path'}, clear=True):
            root = Path(temp)
            env = oracle.runtime_environment(root, root / 'logs')
            self.assertNotIn('GITHUB_TOKEN', env)
            self.assertNotIn('UAP_WINDOWS_RESOURCE_ORACLE_CHILD', env)
            self.assertEqual(env['UAP_WINDOWS_RESOURCE_ORACLE'], '1')
            for key in ('HOME', 'PATH', 'XDG_CONFIG_HOME', 'APPDATA', 'TEMP'):
                self.assertTrue(Path(env[key]).is_dir())
                self.assertEqual(Path(env[key]).parent, root)

    def test_gate_requires_every_proof_and_always_finishes_console(self):
        for fault in ('none', 'marker', 'exit', 'logs', 'restore', 'reuse', 'echo', 'owner', 'forced', 'reader', 'cleanup'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                fixture, evidence = root / 'fixture', root / 'evidence'
                fixture.mkdir()
                evidence.mkdir()
                driver = self

                class FakeConsole:
                    forced = False
                    error = None
                    raw = bytearray()

                    def __init__(self, argv, env, cwd, timeout, evidence):
                        self.config = json.loads(Path(argv[-1]).read_text())
                        self.nonce = self.config['nonce']
                        self.state = dict(phase='probe', exit=1 if fault == 'exit' else 0,
                                          owner_before={'modes': [1]},
                                          owner_after={'modes': [2 if fault == 'restore' else 1]})
                        self.save()
                        oracle_test.logs(evidence / 'child-logs')
                        if fault == 'logs':
                            next((evidence / 'child-logs').glob('*.log')).unlink()

                    def save(self):
                        oracle.write_json(Path(self.config['status']), self.state)

                    def wait(self, marker, after=0, **kwargs):
                        if marker == oracle.MARKER:
                            driver.assertEqual(kwargs, dict(child_nonce=self.nonce, child_final=True))
                            driver.assertEqual(self.timeout, 180)
                            if fault == 'marker':
                                raise AssertionError('missing marker')

                    def send(self, data):
                        self.raw.extend(b'wrong' if fault == 'echo' else data)
                        self.state.update(phase='done', line_read=fault != 'reuse',
                                          owner_probe=self.state['owner_before'])
                        self.save()

                    def poll(self):
                        return 1 if fault == 'owner' else 0

                oracle_test = self
                def finish(session, *args):
                    session.forced = fault == 'forced'
                    session.error = 'reader error' if fault == 'reader' else None
                    if fault == 'cleanup':
                        raise AssertionError('cleanup failure')

                with patch.object(oracle, 'ConPTY', FakeConsole), patch.object(
                        oracle, 'finish_console', side_effect=finish) as cleanup, patch.dict(
                        os.environ, {'SystemRoot': 'C:\\Windows'}):
                    result = {}
                    if fault == 'none':
                        oracle.run_native(root / 'test.exe', fixture, evidence, result)
                        self.assertTrue(result['non_forced_cleanup'])
                        self.assertTrue(result['owner_restoration_and_reuse'])
                    else:
                        with self.assertRaises(AssertionError):
                            oracle.run_native(root / 'test.exe', fixture, evidence, result)
                    cleanup.assert_called_once()


if __name__ == '__main__':
    unittest.main()
