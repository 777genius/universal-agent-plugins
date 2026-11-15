"""Inert regressions for the builder-only native acceptance boundary."""
import base64
from contextlib import ExitStack
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import hermes_install_acceptance as acceptance


class BuilderCleanupAcceptance(unittest.TestCase):
    def test_only_explicit_exact_builder_may_continue_with_original_failure(self):
        original_error = 'original forced cleanup diagnostic'
        base = {'result': {'ExitCode': 0, 'CleanupComplete': True, 'NoChildStarted': False,
                           'Stdout': base64.b64encode(b'original stdout').decode(),
                           'Stderr': base64.b64encode(b'original stderr').decode()},
                'error': original_error, 'forced_cleanup_only': True}
        cases = [('native default', False, True, 1, {}, False),
                 ('reviewed builder', True, True, 1, {}, True),
                 ('different command', True, False, 1, {}, False),
                 ('owner status mismatch', True, True, 0, {}, False),
                 ('joined or unknown cause', True, True, 1, {'forced_cleanup_only': False}, False),
                 ('unconfirmed cleanup', True, True, 1, {'CleanupComplete': False}, False),
                 ('failed native leader', True, True, 1, {'ExitCode': 9}, False),
                 ('unstarted native child', True, True, 1, {'NoChildStarted': True}, False)]
        for label, opt_in, exact, owner_code, changes, allowed in cases:
            with self.subTest(label=label), tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
                packet = json.loads(json.dumps(base))
                for key, value in changes.items():
                    (packet['result'] if key in packet['result'] else packet)[key] = value
                owner = subprocess.CompletedProcess(['supervisor'], owner_code, json.dumps(packet).encode(), b'owner stderr')
                stack.enter_context(patch.dict(os.environ, {'RUNNER_TEMP': directory, 'HERMES_INSTALL_RUNNER': 'supervisor'}))
                stack.enter_context(patch.object(acceptance.subprocess, 'run', return_value=owner))
                stack.enter_context(patch.object(acceptance, 'reviewed_windows_builder', return_value=exact))
                if allowed:
                    record, stdout, stderr = acceptance.supervised(['exact builder'], Path(directory), os.environ,
                                                                  completed_builder_cleanup=opt_in)
                    self.assertEqual(record['error'], original_error)
                    self.assertTrue(record['builder_preparation_continued_after_forced_cleanup'])
                    self.assertEqual((stdout, stderr), (b'original stdout', b'original stderr'))
                else:
                    with self.assertRaises(RuntimeError):
                        acceptance.supervised(['command'], Path(directory), os.environ, completed_builder_cleanup=opt_in)
                evidence = list(Path(directory).glob('hermes-project-supervision-*'))
                self.assertEqual(len(evidence), 1)
                self.assertEqual((evidence[0] / 'owner.stdout').read_bytes(), owner.stdout)
                self.assertEqual((evidence[0] / 'native.stderr').read_bytes(), b'original stderr')
                self.assertEqual(json.loads((evidence[0] / 'record.json').read_text())['error'], original_error)

    def test_reviewed_builder_requires_exact_pinned_argument_vector(self):
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {'RUNNER_TEMP': directory}):
            root = Path(directory).resolve()
            source = root / 'hermes-agent'
            argv = [str(root / 'python.exe'), '-S', '-B', str(source / 'scripts/bundles/native_build.py'),
                    '--source', str(source), '--work', str(root / 'hermes-project-work'),
                    '--cache', str(root / 'hermes-project-cache'), '--out', str(root / 'hermes-project-payload'),
                    '--commit', '2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19']
            self.assertEqual(acceptance.reviewed_windows_builder(argv), os.name == 'nt')
            for index in range(len(argv)):
                changed = list(argv)
                changed[index] = 'unreviewed'
                self.assertFalse(acceptance.reviewed_windows_builder(changed))
            self.assertFalse(acceptance.reviewed_windows_builder(argv + ['extra']))


if __name__ == '__main__':
    unittest.main()
