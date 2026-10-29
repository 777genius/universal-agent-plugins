"""Fail-closed controls for captured baseline evidence, without running a PTY."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('baseline', ROOT / 'check-darwin-consent-baseline.py')
baseline = importlib.util.module_from_spec(spec)
spec.loader.exec_module(baseline)


class ConsentBaselineEvidence(unittest.TestCase):
    def setUp(self):
        # PR534 run40383065030, job121784094400, artifact12680070047:
        # real Go JSON output from pinned f4b11718830d98d9f75adbf8ed6b87eab25ce820.
        self.rows = [json.loads(line) for line in
            (ROOT / 'testdata/darwin-consent-baseline/macos-26-reap-deadlock.jsonl').read_text().splitlines()]

    def test_real_timeout_has_known_cancellation_reaping_signature(self):
        self.assertEqual(baseline.classify(self.rows), 'cancelled-virtual-tty-child-reap-deadlock')
        # The previous inline checker rejects this known outcome before a named
        # failure is emitted, even though the target ran and the old child hung.
        self.assertFalse(any(r.get('Test') == baseline.TARGET and r.get('Action') == 'fail' for r in self.rows))

    def test_original_suffix_loss_proof_still_required(self):
        rows = [dict(Package=baseline.PACKAGE, Action='run', Test=test)
                for test in (baseline.PARENT, baseline.TARGET)]
        rows += [dict(Package=baseline.PACKAGE, Action='output', Test=baseline.TARGET,
                      Output='next owner: "" context deadline exceeded\n'),
                 dict(Package=baseline.PACKAGE, Action='fail', Test=baseline.TARGET),
                 dict(Package=baseline.PACKAGE, Action='fail')]
        self.assertEqual(baseline.classify(rows), 'next-owner-input-lost')
        rows[2]['Output'] = 'unrelated failure\n'
        with self.assertRaises(ValueError): baseline.classify(rows)

    def test_every_target_timeout_and_stack_component_is_required(self):
        for term in ['panic: test timed out after 25s', baseline.TARGET + ' (25s)',
                     '[syscall]', 'syscall.Wait4(', 'os.(*Process).pidWait(',
                     'os/exec.(*Cmd).Wait(', 'terminalprompts.TestDarwinVirtualTTYConsent.func1(',
                     '/consent_virtual_darwin_test.go:', '[chan send]',
                     'os/exec.(*Cmd).watchCtx(', 'created by os/exec.(*Cmd).Start in goroutine ']:
            rows = copy.deepcopy(self.rows)
            self.assertTrue(any(term in row.get('Output', '') for row in rows), term)
            for row in rows:
                if 'Output' in row: row['Output'] = row['Output'].replace(term, 'unrelated')
            with self.subTest(term=term), self.assertRaises(ValueError): baseline.classify(rows)

    def test_wrong_test_package_pass_skip_and_generic_fail_are_rejected(self):
        variants = [[], [dict(Package=baseline.PACKAGE, Action='fail')]]
        for field, old, new in [('Test', baseline.TARGET, 'TestOther'),
                                ('Package', baseline.PACKAGE, 'unrelated/package')]:
            rows = copy.deepcopy(self.rows)
            for row in rows:
                if row.get(field) == old: row[field] = new
            variants.append(rows)
        for action in ['pass', 'skip']:
            rows = copy.deepcopy(self.rows)
            rows.insert(-1, dict(Package=baseline.PACKAGE, Action=action, Test=baseline.TARGET))
            variants.append(rows)
        rows = copy.deepcopy(self.rows); rows[-1]['Action'] = 'pass'; variants.append(rows)
        for i, rows in enumerate(variants):
            with self.subTest(case=i), self.assertRaises(ValueError): baseline.classify(rows)


if __name__ == '__main__':
    unittest.main()
