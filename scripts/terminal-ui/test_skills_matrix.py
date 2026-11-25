"""Failure-sensitive controls for Skills qualification source/state oracles."""
import json
import os
from pathlib import Path
import ssl
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import urllib.error
import urllib.request

import skills_fixture as source
import skills_matrix as matrix
import skills_oracles as oracle


class EnvelopeTests(unittest.TestCase):
    def valid(self, result='success'):
        return dict(schema_version=1, command='skills add', result=result)

    def test_one_envelope_and_exit_must_agree(self):
        self.assertEqual(oracle.envelope(json.dumps(self.valid()), 0)['result'], 'success')
        self.assertEqual(oracle.envelope(json.dumps(self.valid('failure')), 1)['result'], 'failure')
        for body, code in (
                (json.dumps(self.valid()), 1),
                (json.dumps(self.valid('failure')), 0),
                (json.dumps(self.valid()) + '\n' + json.dumps(self.valid()), 0),
                ('plain output\n' + json.dumps(self.valid()), 0)):
            with self.subTest(body=body, code=code), self.assertRaises(AssertionError):
                oracle.envelope(body, code)

    def test_schema_and_command_are_not_inferred(self):
        for changed in ({'schema_version': 2}, {'command': 'add'}, {'result': 'partial'}):
            with self.subTest(value=changed), self.assertRaises(AssertionError):
                oracle.envelope(json.dumps(dict(self.valid(), **changed)), 0)

    def test_missing_and_duplicate_agent_results_are_refused(self):
        for rows in (None, [], [{'status': 'failed'}],
                     [{'targetId': 'codex'}, {'targetId': 'codex'}]):
            with self.subTest(rows=rows), self.assertRaises(AssertionError):
                oracle.items_by_agent({'items': rows})

    def test_partial_agent_identity_is_retained(self):
        result = {'items': [{'targetId': 'codex', 'status': 'failed'},
                            {'targetId': 'cursor', 'status': 'succeeded'}]}
        rows = oracle.items_by_agent(result)
        self.assertEqual(rows['codex']['status'], 'failed')
        self.assertEqual(rows['cursor']['status'], 'succeeded')


class DurableStateTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.fixture = SimpleNamespace(root=self.root, home=self.root / 'home',
                                       project=self.root / 'project', data=self.root / 'data')
        for path in (self.fixture.home, self.fixture.project, self.fixture.data):
            path.mkdir()

    def test_noop_ignores_logs_but_catches_pins_registry_and_empty_directories(self):
        before = oracle.durable_tree(self.fixture)
        logs = self.fixture.data / 'logs'
        logs.mkdir()
        (logs / 'install.jsonl').write_text('new diagnostic\n')
        self.assertEqual(oracle.durable_tree(self.fixture), before)
        for path in (self.fixture.data / 'uap.lock', self.root / 'native-registry.json'):
            path.write_text('changed')
            self.assertNotEqual(oracle.durable_tree(self.fixture), before)
            path.unlink()
        (self.fixture.home / 'empty-new-directory').mkdir()
        self.assertNotEqual(oracle.durable_tree(self.fixture), before)

    def test_log_must_exist_and_stay_inside_disposable_fixture(self):
        path = self.root / 'diagnostic.jsonl'
        path.write_text('{"client":"codex","status":"failed"}\n')
        self.assertEqual(oracle.failure_log({'log_file': str(path)}, self.fixture)[0]['client'],
                         'codex')
        path.unlink()
        with self.assertRaises(AssertionError):
            oracle.failure_log({'log_file': str(path)}, self.fixture)
        with self.assertRaises(AssertionError):
            oracle.failure_log({'log_file': str(self.root.parent / 'outside')}, self.fixture)

    def test_readonly_commands_do_not_hide_any_mutation(self):
        reads = [['--version'], ['plugin', 'list', '--json'], ['app-server']]
        writes = [['plugin', 'add', 'demo@market', '--json'],
                  ['plugin', 'remove', 'demo@market', '--json'],
                  ['plugin', 'marketplace', 'add', '/owned', '--json'],
                  ['plugin', 'marketplace', 'remove', 'market', '--json'],
                  ['plugin', 'marketplace', 'update', 'market', '--json']]
        self.assertEqual(oracle.native_mutations(reads + writes), writes)


@unittest.skipUnless(sys.platform == 'linux', 'Linux-only SSL_CERT_FILE source fixture')
class PublisherTrustTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.publisher = source.Publisher(Path(self.temporary.name) / 'publisher')
        self.addCleanup(self.publisher.close)

    def opener(self, context):
        return urllib.request.build_opener(urllib.request.ProxyHandler({}),
                                            urllib.request.HTTPSHandler(context=context))

    def test_complete_original_bytes_require_explicit_valid_trust(self):
        context = ssl.create_default_context(cafile=str(self.publisher.ca))
        opener = self.opener(context)
        index_url = self.publisher.url + '/.well-known/agent-skills/index.json'
        with opener.open(index_url, timeout=5) as response:
            index = json.load(response)
        entry = index['skills'][0]
        self.assertEqual(set(entry['files']), {'SKILL.md', 'reference.txt'})
        for name in entry['files']:
            url = self.publisher.url + '/.well-known/agent-skills/console-proof/' + name
            with opener.open(url, timeout=5) as response:
                self.assertEqual(response.read(), self.publisher.files[name])
        self.assertEqual(len(self.publisher.requests), 3)
        untrusted = self.opener(ssl.create_default_context())
        with self.assertRaises(urllib.error.URLError):
            untrusted.open(index_url, timeout=5)

    def test_publisher_failure_remains_a_real_http_failure(self):
        self.publisher.status = 503
        context = ssl.create_default_context(cafile=str(self.publisher.ca))
        opener = self.opener(context)
        with self.assertRaises(urllib.error.HTTPError) as failure:
            opener.open(self.publisher.url + '/.well-known/agent-skills/index.json', timeout=5)
        self.assertEqual(failure.exception.code, 503)


class FixtureIsolationTests(unittest.TestCase):
    def test_public_source_exposes_git_without_ambient_credentials(self):
        fixture = SimpleNamespace(env={
            'PATH': 'private-stubs', 'HOME': 'private-home',
            'HTTPS_PROXY': 'http://127.0.0.1:9', 'HTTP_PROXY': 'http://127.0.0.1:9',
            'ALL_PROXY': 'http://127.0.0.1:9', 'NO_PROXY': ''})
        with tempfile.TemporaryDirectory() as temporary:
            git = Path(temporary).resolve() / 'git-bin' / 'git'
            with patch.object(source.shutil, 'which', return_value=str(git)):
                source.public_git_environment(fixture)
            self.assertEqual(fixture.env['PATH'], 'private-stubs' + os.pathsep + str(git.parent))
        self.assertEqual(fixture.env['HOME'], 'private-home')
        self.assertNotIn('HTTPS_PROXY', fixture.env)
        self.assertNotIn('GITHUB_TOKEN', fixture.env)

    def test_missing_git_fails_instead_of_swapping_source(self):
        with patch.object(source.shutil, 'which', return_value=None):
            with self.assertRaises(AssertionError):
                source.public_git_environment(SimpleNamespace(env={'PATH': 'stubs'}))

    def test_explicit_unsupported_case_remains_requested(self):
        self.assertEqual(matrix.selected_cases(['review-redirect'], 'all', 'win32'),
                         ['review-redirect'])

    def test_windows_default_keeps_actual_skills_lifecycle_cases(self):
        selected = matrix.selected_cases(None, 'all', 'win32')
        for required in ('fresh', 'unchanged', 'unchanged-failure', 'partial-json',
                         'partial-decline', 'partial-retry', 'review-ctrl-c',
                         'review-eof', 'plain-browser', 'explicit-ui'):
            self.assertIn(required, selected)
        self.assertNotIn('review-redirect', selected)

    def test_website_cannot_stand_in_for_catalog_browser_coverage(self):
        self.assertEqual(matrix.BROWSER_CASES,
                         {'catalog-redirect', 'plain-browser', 'explicit-ui'})
        self.assertTrue(source.GITHUB_SOURCE.endswith('@' + source.SOURCE_SHA))
        self.assertNotIn('--ref', source.GITHUB_SOURCE)


if __name__ == '__main__':
    unittest.main()
