"""Contract tests plus an opt-in execution of the black-box resilience lane."""

import json
import os
from pathlib import Path
import tempfile
import unittest

import resilience_matrix as lane


class ResilienceMatrixContract(unittest.TestCase):
    def test_case_contract(self):
        self.assertEqual(len(lane.CASES), 26)
        self.assertEqual(len(set(lane.CASES)), len(lane.CASES))
        for required in (
                'corrupt-state', 'future-state', 'duplicate-installation',
                'tampered-receipt', 'corrupt-journal', 'held-process-lock',
                'migrate-state-v2', 'migrate-state-v3',
                'killed-lock-owner', 'concurrent-add',
                'doctor-during-add', 'add-remove-race', 'scanner-failure',
                'permission-denied', 'closed-stdout', 'closed-stderr',
                'file-size-limit', 'foreign-collision', 'dangling-symlink',
                'source-path-traversal', 'fresh-multi-client-install',
                'per-client-lifecycle', 'unicode-normalization',
                'sigint-during-activation', 'unicode-long-path'):
            self.assertIn(required, lane.CASES)

    def test_json_envelope_contract(self):
        valid = json.dumps({
            'schema_version': 1,
            'command': 'doctor',
            'result': 'success',
            'data': {},
        })
        self.assertEqual(lane.parse_envelope(valid)['command'], 'doctor')
        self.assertIsNone(lane.parse_envelope('   '))
        for invalid in (
                '{}',
                '{"schema_version":2,"command":"doctor","result":"success"}',
                '{"schema_version":1,"command":"","result":"success"}',
                '{"schema_version":1,"command":"doctor","result":"maybe"}',
                'not-json'):
            with self.subTest(invalid=invalid), self.assertRaises(AssertionError):
                lane.parse_envelope(invalid)

    def test_write_json_is_deterministic_and_terminated(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / 'nested' / 'value.json'
            lane.write_json(path, {'b': 2, 'a': 1})
            self.assertEqual(json.loads(path.read_text()), {'a': 1, 'b': 2})
            self.assertTrue(path.read_bytes().endswith(b'\n'))


@unittest.skipUnless(os.environ.get('RESILIENCE_MATRIX_BINARY'),
                     'set RESILIENCE_MATRIX_BINARY for black-box execution')
class ResilienceMatrixExecution(unittest.TestCase):
    def test_state_and_lock_smoke(self):
        binary = Path(os.environ['RESILIENCE_MATRIX_BINARY']).resolve(strict=True)
        with tempfile.TemporaryDirectory() as temp:
            matrix = lane.Matrix(binary, timeout=20)
            for name in ('corrupt-state', 'held-process-lock', 'concurrent-add'):
                with self.subTest(name=name):
                    matrix.run_case(name, Path(temp) / name)


if __name__ == '__main__':
    unittest.main()
