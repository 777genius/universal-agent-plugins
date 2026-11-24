"""Native-output contract controls with real held filesystem custody, no E2E claim."""

from contextlib import contextmanager
import copy
import json
from pathlib import Path
import runpy
import sys
import tempfile
import unittest

from scripts import test_marketing_kit_claude_retirement as custody_tests


ClaudeProjectDiscovery = runpy.run_path(str(Path(__file__).with_name(
    "marketing_kit_claude_discovery.py")))["ClaudeProjectDiscovery"]


@unittest.skipUnless(sys.platform == "linux", "Linux signed cache custody")
class ClaudeDiscoveryContracts(unittest.TestCase):
    @contextmanager
    def fixture(self, *, mutate_before=None, version="2.1.263 (Claude Code)\n"):
        support = custody_tests.SignedClosureRetirementContracts()
        with tempfile.TemporaryDirectory() as directory:
            f = support.fixture(Path(directory).resolve())
            subject = support.capture(f)
            binary = f['root'] / 'inert-native-contract'
            binary.write_text('This is never executed.\n')
            f['audit']['AGENT_BINARIES']['claude'] = str(binary)
            environment = {'HOME': str(f['home']), 'CLAUDE_CONFIG_DIR': str(f['profile'])}
            foreign = {'id': 'unmanaged-skill@skills-dir', 'scope': 'user', 'enabled': True}
            before = [foreign, *[{'id': owner['key'], 'scope': 'project', 'enabled': False,
                'version': owner['signed_release']['package_version'], 'installPath': str(owner['cache']),
                'projectPath': str(f['project']), 'errors': []} for owner in subject.owners]]
            if mutate_before:
                mutate_before(before)
            selected = {'rows': before, 'exit_code': 0, 'timeout': False, 'stderr': '', 'calls': []}

            def invoke(argv, env, cwd, timeout, *, capture_full):
                self.assertEqual(argv[0], str(binary))
                self.assertEqual(env, environment)
                self.assertEqual(cwd, f['project'])
                self.assertTrue(capture_full)
                selected['calls'].append(argv[1:])
                body = version if argv[1:] == ['--version'] else json.dumps(selected['rows'])
                if selected.get('body') is not None and argv[1:] != ['--version']:
                    body = selected['body']
                if selected.get('mutation') and argv[1:] != ['--version']:
                    selected['mutation']()
                return {'exit_code': selected['exit_code'], 'timeout': selected['timeout'],
                        'stdout_full': body, 'stderr_full': selected['stderr']}

            try:
                native = ClaudeProjectDiscovery(subject, environment, invoke)
                selected['rows'] = [foreign]
                yield f, support, subject, native, selected
            finally:
                subject.close()

    def retire(self, f, support, subject):
        support.retire(f)
        subject.finish(f['cleanup'], support.scan(f), lambda: support.scan(f), keep_open=True)

    def test_fresh_listing_is_separate_from_retirement_and_never_promotes_activation(self):
        with self.fixture() as (f, support, subject, native, selected):
            self.retire(f, support, subject)
            proof = native.removed()
            self.assertEqual(proof['registered_before'], 4)
            self.assertEqual(proof['owned_registrations_after'], 0)
            self.assertTrue(proof['foreign_rows_unchanged'])
            self.assertFalse(proof['native_activation_claimed'])
            self.assertFalse(proof['mcp_execution_claimed'])
            self.assertFalse(proof['authentication_claimed'])
            self.assertFalse(proof['writer_closure_claimed'])
            self.assertEqual(selected['calls'], [['--version'], ['plugin', 'list', '--json'],
                                                ['plugin', 'list', '--json']])
            self.assertTrue(subject.fds)
            native.removed()
            self.assertEqual(selected['calls'][-1], ['plugin', 'list', '--json'])

    def test_registered_native_identity_must_match_each_held_signed_owner(self):
        changes = {
            'id': lambda rows: rows[1].update(id='foreign@market'),
            'scope': lambda rows: rows[1].update(scope='user'),
            'version': lambda rows: rows[1].update(version='99'),
            'path': lambda rows: rows[1].update(installPath='/foreign'),
            'project': lambda rows: rows[1].update(projectPath='/other'),
            'enabled-type': lambda rows: rows[1].update(enabled='false'),
            'errors': lambda rows: rows[1].update(errors=['unavailable']),
            'duplicate': lambda rows: rows.append(copy.deepcopy(rows[1])),
        }
        for name, mutate in changes.items():
            with self.subTest(name=name), self.assertRaises(AssertionError), self.fixture(mutate_before=mutate):
                pass
        with self.assertRaisesRegex(AssertionError, 'fixed package pin'), self.fixture(version='2.1.264 (Claude Code)\n'):
            pass

    def test_retired_cache_does_not_excuse_a_remaining_native_row_or_foreign_mutation(self):
        for mutation in ('owned', 'foreign', 'extra'):
            with self.subTest(mutation=mutation), self.fixture() as (f, support, subject, native, selected):
                self.retire(f, support, subject)
                if mutation == 'owned': selected['rows'].append({'id': next(iter(native.owned_ids))})
                elif mutation == 'foreign': selected['rows'][0]['enabled'] = False
                else: selected['rows'].append({'id': 'unexpected@market'})
                with self.assertRaisesRegex(AssertionError, 'retained an owned registration or changed a foreign row'):
                    native.removed()

    def test_process_failures_truncated_duplicate_or_non_array_output_never_qualify(self):
        failures = [dict(exit_code=1), dict(exit_code=True), dict(timeout=True), dict(stderr='native failure'),
                    dict(body='[] trailing'), dict(body='{}'), dict(body='x' * ((1 << 20) + 1)),
                    dict(body='[{"id":"one","id":"two"}]'), dict(body='[{"id":"one"},{"id":"one"}]')]
        for failure in failures:
            with self.subTest(failure=tuple(failure)), self.fixture() as (f, support, subject, native, selected):
                self.retire(f, support, subject)
                selected.update(failure)
                with self.assertRaises((AssertionError, ValueError)):
                    native.removed()

    def test_native_query_cannot_mutate_retired_owned_bytes_and_still_qualify(self):
        with self.fixture() as (f, support, subject, native, selected):
            self.retire(f, support, subject)
            moved = subject.owners[0]['moved'] / '.agentplugins-runtime/plugin.json'
            selected['mutation'] = lambda: moved.write_text('changed by simulated native query')
            with self.assertRaises(ValueError):
                native.removed()
            self.assertTrue(subject.finished)

    def test_unfinished_or_released_custody_cannot_authorize_native_absence(self):
        for close in (False, True):
            with self.subTest(close=close), self.fixture() as (_f, _support, subject, native, _selected):
                if close:
                    subject.close()
                with self.assertRaises(ValueError):
                    native.removed()


if __name__ == '__main__':
    unittest.main()
