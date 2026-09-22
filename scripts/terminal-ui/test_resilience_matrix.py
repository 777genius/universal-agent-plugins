"""Contract tests plus an opt-in execution of the black-box resilience lane."""

import json
import os
from pathlib import Path
import subprocess
import sys
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

    @unittest.skipUnless(os.name == 'posix', 'pipe select is POSIX only')
    def test_process_line_deadline_kills_and_reaps(self):
        holder = subprocess.Popen(
            [sys.executable, '-c', 'import time; time.sleep(60)'],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        try:
            with self.assertRaisesRegex(
                    AssertionError, 'before handshake deadline'):
                lane.expect_process_line(
                    holder, 'locked', 0.05, 'fixture did not start')
            self.assertIsNotNone(holder.poll())
        finally:
            if holder.poll() is None:
                holder.kill()
                holder.wait(timeout=2)
            if holder.stdout is not None:
                holder.stdout.close()
            if holder.stderr is not None:
                holder.stderr.close()


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
