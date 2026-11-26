"""Failure-sensitive controls for Skills qualification source/state oracles."""
import copy
import hashlib
import json
import os
from pathlib import Path
import ssl
import socket
import shutil
import sys
import tempfile
import time
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

    def test_declared_install_still_requires_one_complete_document(self):
        output = dict(self.valid(), command='agentplugins install', outcome='completed',
                      actions=[{'action': 'repair'}, {'action': 'unchanged'}],
                      results=[{'operation': 'repair', 'outcome': 'completed'}])
        body = json.dumps(output)
        self.assertEqual(oracle.envelope(body, 0, command='agentplugins install'), output)
        with self.assertRaisesRegex(AssertionError, 'exactly one JSON envelope'):
            oracle.envelope(body + '\n' + body, 0, command='agentplugins install')

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

    def diagnostic(self, path):
        return {'log_file': str(path), 'items': [
            {'targetId': 'codex', 'alias': 'demo', 'status': 'failed'}]}

    def log_rows(self):
        return [{'type': 'run', 'schema_version': 1, 'command': 'skills add', 'attempt': 1},
                {'type': 'binding', 'client': 'codex', 'alias': 'demo', 'status': 'failed',
                 'code': 'native_command_failed', 'summary': 'Native operation did not complete.'}]

    def test_log_must_exist_and_stay_inside_disposable_fixture(self):
        path = self.root / 'diagnostic.jsonl'
        rows = self.log_rows()
        path.write_text('\n'.join(json.dumps(row) for row in rows) + '\n')
        self.assertEqual(oracle.failure_log(self.diagnostic(path), self.fixture)[1]['client'], 'codex')
        path.unlink()
        with self.assertRaisesRegex(AssertionError, 'nonexistent log'):
            oracle.failure_log(self.diagnostic(path), self.fixture)
        outside = self.root.parent / (self.root.name + '-outside.jsonl')
        self.addCleanup(outside.unlink, missing_ok=True)
        outside.write_text('\n'.join(json.dumps(row) for row in rows) + '\n', encoding='utf-8')
        self.assertTrue(outside.is_file())
        with self.assertRaisesRegex(AssertionError, 'log escaped fixture'):
            oracle.failure_log(self.diagnostic(outside), self.fixture)

    def test_header_only_success_only_wrong_client_and_unclassified_logs_cannot_pass(self):
        path = self.root / 'diagnostic.jsonl'
        header, failure = self.log_rows()
        alternatives = [
            [header],
            [header, dict(failure, status='completed')],
            [header, dict(failure, client='cursor')],
            [header, dict(failure, alias='other')],
            [header, dict(failure, code='')],
            [header, dict(failure, summary='')],
            [dict(header, schema_version=2), failure],
            [dict(header, command='add'), failure],
        ]
        for rows in alternatives:
            with self.subTest(rows=rows):
                path.write_text('\n'.join(json.dumps(row) for row in rows) + '\n')
                with self.assertRaises(AssertionError):
                    oracle.failure_log(self.diagnostic(path), self.fixture)
        path.write_text('{"truncated":')
        with self.assertRaisesRegex(AssertionError, 'JSON Lines'):
            oracle.failure_log(self.diagnostic(path), self.fixture)

    def test_missing_alias_on_both_result_and_log_is_not_a_matching_binding(self):
        path = self.root / 'diagnostic.jsonl'
        rows = self.log_rows()
        rows[1].pop('alias')
        path.write_text('\n'.join(json.dumps(row) for row in rows) + '\n')
        result = self.diagnostic(path)
        result['items'][0].pop('alias')
        with self.assertRaisesRegex(AssertionError, 'expected package alias'):
            oracle.failure_log(result, self.fixture)

    def test_readonly_commands_do_not_hide_any_mutation(self):
        reads = [['--version'], ['plugin', 'list', '--json'], ['app-server']]
        writes = [['plugin', 'add', 'demo@market', '--json'],
                  ['plugin', 'remove', 'demo@market', '--json'],
                  ['plugin', 'marketplace', 'add', '/owned', '--json'],
                  ['plugin', 'marketplace', 'remove', 'market', '--json'],
                  ['plugin', 'marketplace', 'update', 'market', '--json']]
        self.assertEqual(oracle.native_mutations(reads + writes), writes)


class InstallationEvidenceTests(unittest.TestCase):
    """These test oracle sensitivity, not a simulated installer success."""
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.fixture = SimpleNamespace(
            root=self.root, home=self.root / 'home', project=self.root / 'project',
            data=self.root / 'data', skill_name='guide', skill_bytes=b'original Skill bytes\n',
            skill_blob=None, publisher=None)
        self.fixture.data.mkdir()
        self.target = self.root / 'managed' / 'demo'
        self.skill = self.target / 'skills' / 'guide' / 'SKILL.md'
        self.skill.parent.mkdir(parents=True)
        self.skill.write_bytes(self.fixture.skill_bytes)
        cache = self.root / 'native-cache'
        cached = cache / 'skills' / 'guide' / 'SKILL.md'
        cached.parent.mkdir(parents=True)
        cached.write_bytes(self.fixture.skill_bytes)
        self.digest = 'sha256:' + 'a' * 64
        authority = {'source_key': 'fixture-source', 'client_id': 'codex', 'alias': 'demo',
                     'environment_id': 'user-default'}
        self.binding = dict(
            client_id='codex', client_binding_id='binding', target_locator=str(self.target),
            materialization='materialized', activation='active', authentication='not_required',
            verification='installation_verified', package_revision={'tree_digest': self.digest},
            receipts=[{'phase': 'committed', 'environment_authority': authority,
                       'client_binding_id': 'binding', 'active_path': str(self.target)}])
        self.owner = dict(alias='demo', client_binding_id='binding', client_id='codex',
                          source_key='fixture-source', package_digest=self.digest,
                          installation_id='installation', environment_id='user-default',
                          phase='committed')
        self.state = dict(installations=[{'installation_id': 'installation',
                                         'clients': {'binding': self.binding}}],
                          environment_receipts=[self.owner])
        self.save()
        (self.fixture.data / 'uap.toml').write_text('fixture agreement')
        self.lock = {'lock_version': 1, 'packages': [{
            'alias': 'demo', 'source_key': 'fixture-source', 'targets': ['codex'],
            'tree_digest': self.digest}]}
        self.save_lock()
        (self.target / 'plugin.json').write_text(json.dumps({'name': 'demo', 'version': '1.0.0'}),
                                               encoding='utf-8')
        self.registry = dict(cache=str(cache), name='demo', marketplace='fixture-market',
                             spec='demo@fixture-market', path=str(self.target), installed=[{
                                 'pluginId': 'demo@fixture-market', 'name': 'demo',
                                 'marketplaceName': 'fixture-market', 'installed': True,
                                 'enabled': True, 'version': '1.0.0',
                                 'source': {'source': 'local', 'path': str(self.target)}}])
        self.save_registry()

    def save(self):
        (self.fixture.data / 'state-v2.json').write_text(json.dumps(self.state), encoding='utf-8')

    def save_lock(self):
        (self.fixture.data / 'uap.lock').write_text(json.dumps(self.lock), encoding='utf-8')

    def save_registry(self):
        (self.root / 'native-registry.json').write_text(json.dumps(self.registry), encoding='utf-8')

    def test_complete_matching_native_evidence_is_accepted(self):
        self.assertEqual(oracle.installed(self.fixture, ('codex',))['codex'], self.binding)

    def test_prepared_native_binding_does_not_qualify_completion(self):
        self.binding['activation'] = 'prepared'
        self.save()
        with self.assertRaisesRegex(AssertionError, 'native Skill completion'):
            oracle.installed(self.fixture, ('codex',))

    def test_wrong_receipt_digest_and_authority_are_rejected(self):
        self.owner['package_digest'] = 'sha256:' + 'b' * 64
        self.save()
        with self.assertRaisesRegex(AssertionError, 'digest disagree'):
            oracle.installed(self.fixture, ('codex',))
        self.owner['package_digest'] = self.digest
        self.binding['receipts'][0]['environment_authority']['source_key'] = 'foreign-source'
        self.save()
        with self.assertRaisesRegex(AssertionError, 'source/client authority'):
            oracle.installed(self.fixture, ('codex',))

    def test_native_cache_and_pinned_source_cannot_disagree(self):
        self.skill.write_bytes(b'changed projection\n')
        with self.assertRaisesRegex(AssertionError, 'Skill bytes changed'):
            oracle.installed(self.fixture, ('codex',))
        self.skill.write_bytes(self.fixture.skill_bytes)
        registry = json.loads((self.root / 'native-registry.json').read_text(encoding='utf-8'))
        cached = Path(registry['cache']) / 'skills' / 'guide' / 'SKILL.md'
        cached.write_bytes(b'changed native cache\n')
        with self.assertRaisesRegex(AssertionError, 'cache has different Skill bytes'):
            oracle.installed(self.fixture, ('codex',))

    def test_lock_digest_in_comment_or_other_entry_cannot_qualify_binding(self):
        package = self.lock['packages'][0]
        package['tree_digest'] = 'sha256:' + 'b' * 64
        self.lock['comment'] = 'retained old digest: ' + self.digest
        self.lock['packages'].append(dict(package, alias='unrelated', tree_digest=self.digest))
        self.save_lock()
        with self.assertRaisesRegex(AssertionError, 'matching locked binding.*digest disagree'):
            oracle.installed(self.fixture, ('codex',))

    def test_lock_must_match_alias_source_target_and_unambiguous_package(self):
        original = dict(self.lock['packages'][0])
        for field, wrong in (('alias', 'other'), ('source_key', 'foreign-source'),
                             ('targets', ['cursor']), ('targets', 'codex')):
            with self.subTest(field=field, wrong=wrong):
                self.lock['packages'] = [dict(original, **{field: wrong})]
                self.save_lock()
                with self.assertRaisesRegex(AssertionError, 'exact binding alias/source/target'):
                    oracle.installed(self.fixture, ('codex',))
        self.lock['packages'] = [original, dict(original)]
        self.save_lock()
        with self.assertRaisesRegex(AssertionError, 'exact binding alias/source/target'):
            oracle.installed(self.fixture, ('codex',))

    def test_lock_is_parsed_instead_of_accepting_a_raw_digest_substring(self):
        path = self.fixture.data / 'uap.lock'
        for body in ('# previous digest ' + self.digest, '{"lock_version": 2, "packages": []}'):
            with self.subTest(body=body):
                path.write_text(body, encoding='utf-8')
                with self.assertRaisesRegex(AssertionError, 'saved immutable lock'):
                    oracle.installed(self.fixture, ('codex',))

    def test_committed_receipt_alias_must_match_lock_and_owner(self):
        self.binding['receipts'][0]['environment_authority']['alias'] = 'foreign-alias'
        self.save()
        with self.assertRaisesRegex(AssertionError, 'authority or alias'):
            oracle.installed(self.fixture, ('codex',))

    def test_removed_or_disabled_native_registration_rejects_intact_cache(self):
        cached = Path(self.registry['cache']) / 'skills' / 'guide' / 'SKILL.md'
        original = dict(self.registry['installed'][0])
        for entries in ([], [dict(original, installed=False)], [dict(original, enabled=False)],
                        [dict(original, pluginId='other@fixture-market')],
                        [dict(original, version='0.0.0')],
                        [dict(original, source={'path': str(self.root / 'other-package')})]):
            with self.subTest(installed=entries):
                self.registry['installed'] = entries
                self.save_registry()
                self.assertEqual(cached.read_bytes(), self.fixture.skill_bytes)
                with self.assertRaisesRegex(AssertionError, 'not registered as installed and enabled'):
                    oracle.installed(self.fixture, ('codex',))

    def test_native_registry_read_explicitly_uses_utf8(self):
        read_text = Path.read_text
        observed = []

        def check_encoding(path, *args, **kwargs):
            if path.name == 'native-registry.json':
                observed.append(kwargs.get('encoding'))
            return read_text(path, *args, **kwargs)

        with patch.object(Path, 'read_text', check_encoding):
            oracle.installed(self.fixture, ('codex',))
        self.assertEqual(observed, ['utf-8'])

    def test_registry_metadata_must_identify_the_current_native_package(self):
        self.registry['spec'] = 'other@fixture-market'
        self.save_registry()
        with self.assertRaisesRegex(AssertionError, 'source does not match'):
            oracle.installed(self.fixture, ('codex',))

    def test_pinned_github_object_is_checked_independently_of_fixture_body(self):
        self.fixture.skill_bytes = None
        self.fixture.skill_blob = '0' * 40
        with self.assertRaisesRegex(AssertionError, 'immutable pinned source blob'):
            oracle.installed(self.fixture, ('codex',))


class RetainedFailureEvidenceTests(unittest.TestCase):
    """Managed commit survives native failure without qualifying active success."""
    save = InstallationEvidenceTests.save
    save_lock = InstallationEvidenceTests.save_lock
    save_registry = InstallationEvidenceTests.save_registry

    def setUp(self):
        InstallationEvidenceTests.setUp(self)
        self.cursor = copy.deepcopy(self.binding)
        cursor_path = self.root / 'cursor' / 'demo'
        shutil.copytree(self.target, cursor_path)
        self.cursor.update(client_id='cursor', client_binding_id='cursor-binding',
                           target_locator=str(cursor_path), activation='manual_activation_required',
                           authentication='not_checked', verification='package_validated')
        cursor_receipt = self.cursor['receipts'][0]
        cursor_receipt.update(client_binding_id='cursor-binding', active_path=str(cursor_path))
        cursor_receipt['environment_authority']['client_id'] = 'cursor'
        self.state['installations'][0]['clients']['cursor-binding'] = self.cursor
        self.state['environment_receipts'].append(dict(
            self.owner, client_id='cursor', client_binding_id='cursor-binding'))
        self.lock['packages'][0]['targets'] = ['codex', 'cursor']
        self.binding.update(activation='failed', verification='failed', policy='allowed')
        self.registry = {'path': str(self.target)}
        self.result = {'result': 'failure', 'status': 'partial', 'items': [
            {'targetId': 'codex', 'alias': 'demo', 'destinationPath': str(self.target),
             'status': 'failed'},
            {'targetId': 'cursor', 'alias': 'demo', 'destinationPath': str(cursor_path),
             'status': 'succeeded'}]}
        self.save()
        self.save_lock()
        self.save_registry()

    def prove(self):
        return oracle.installed(self.fixture, ('cursor',), retained_failed=('codex',),
                                result=self.result)

    def test_partial_commit_is_owned_but_only_healthy_delivery_is_returned(self):
        self.assertEqual(self.prove(), {'cursor': self.cursor})
        self.assertEqual(oracle.installed(self.fixture, ('cursor',), retained_failed=('codex',)),
                         {'cursor': self.cursor})

    def test_tls_publisher_resources_do_not_replace_expected_result_target_identity(self):
        resources = {'SKILL.md': self.fixture.skill_bytes, 'reference.txt': b'original reference\n'}
        self.fixture.publisher = SimpleNamespace(files=resources)
        for binding in (self.binding, self.cursor):
            skill = Path(binding['target_locator']) / 'skills' / self.fixture.skill_name
            (skill / 'reference.txt').write_bytes(resources['reference.txt'])
        self.assertEqual(self.prove(), {'cursor': self.cursor})
        (self.skill.parent / 'reference.txt').write_bytes(b'changed reference\n')
        with self.assertRaisesRegex(AssertionError, 'complete source'):
            self.prove()

    def test_failed_ownership_is_never_implicitly_accepted_or_counted_successful(self):
        with self.assertRaisesRegex(AssertionError, 'durable targets'):
            oracle.installed(self.fixture, ('cursor',))
        with self.assertRaisesRegex(AssertionError, 'native Skill completion'):
            oracle.installed(self.fixture, ('cursor', 'codex'))

    def test_expected_partitions_must_be_disjoint_unique_and_supported(self):
        for healthy, failed in ((('cursor', 'codex'), ('codex',)),
                                (('cursor', 'cursor'), ('codex',)),
                                (('cursor',), ('codex', 'codex')),
                                (('codex',), ('cursor',))):
            with self.subTest(healthy=healthy, failed=failed), self.assertRaises(AssertionError):
                oracle.installed(self.fixture, healthy, retained_failed=failed)

    def test_retained_failure_requires_exact_failed_lifecycle_facts(self):
        original = dict(self.binding)
        for field, wrong in (('activation', 'active'), ('activation', 'prepared'),
                             ('verification', 'installation_verified'),
                             ('authentication', 'not_checked'), ('policy', 'blocked'),
                             ('materialization', 'absent')):
            with self.subTest(field=field, wrong=wrong):
                self.binding.update(original)
                self.binding[field] = wrong
                self.save()
                with self.assertRaises(AssertionError):
                    self.prove()

    def test_registered_disabled_or_stale_cache_metadata_cannot_qualify_failure(self):
        for key, value in (('installed', []), ('installed', [{'installed': True, 'enabled': True}]),
                           ('installed', [{'installed': False, 'enabled': False}]),
                           ('cache', str(self.root / 'native-cache')), ('marketplace', 'stale'),
                           ('name', 'demo'), ('spec', 'demo@stale')):
            with self.subTest(key=key):
                self.registry = {'path': str(self.target), key: value}
                self.save_registry()
                with self.assertRaisesRegex(AssertionError, 'retained registration'):
                    self.prove()

    def test_native_cache_effect_is_rejected_even_without_registry_metadata(self):
        cache = self.fixture.home / '.codex' / 'plugins' / 'cache'
        cache.mkdir(parents=True)
        with self.assertRaisesRegex(AssertionError, 'cache effect'):
            self.prove()

    def test_missing_or_foreign_native_source_is_rejected(self):
        self.registry['path'] = str(self.root / 'other-managed-package')
        self.save_registry()
        with self.assertRaisesRegex(AssertionError, 'unrelated source'):
            self.prove()
        (self.root / 'native-registry.json').unlink()
        with self.assertRaisesRegex(AssertionError, 'registry observation'):
            self.prove()

    def test_failed_binding_still_requires_original_skill_bytes(self):
        self.skill.write_bytes(b'foreign bytes')
        with self.assertRaisesRegex(AssertionError, 'Skill bytes changed'):
            self.prove()

    def test_retained_commit_and_environment_receipts_must_match_exact_binding(self):
        original = copy.deepcopy(self.binding['receipts'])
        for field, wrong in (('phase', 'rolled_back'), ('client_binding_id', 'cursor-binding'),
                             ('active_path', str(self.root / 'other'))):
            with self.subTest(field=field):
                self.binding['receipts'] = copy.deepcopy(original)
                self.binding['receipts'][0][field] = wrong
                self.save()
                with self.assertRaises(AssertionError):
                    self.prove()

    def test_wrong_or_duplicate_environment_owner_is_rejected(self):
        original = copy.deepcopy(self.owner)
        for field, wrong in (('phase', 'prepared'), ('installation_id', 'other-installation'),
                             ('environment_id', 'other-environment'), ('client_id', 'cursor'),
                             ('package_digest', 'sha256:' + 'b' * 64)):
            with self.subTest(field=field):
                self.owner.update(original)
                self.owner[field] = wrong
                self.save()
                with self.assertRaises(AssertionError):
                    self.prove()
        self.owner.update(original)
        self.state['environment_receipts'].append(dict(self.owner))
        self.save()
        with self.assertRaisesRegex(AssertionError, 'ownership receipt'):
            self.prove()

    def test_failed_binding_lock_pin_cannot_be_borrowed_from_an_unrelated_entry(self):
        package = self.lock['packages'][0]
        package['targets'] = ['cursor']
        self.lock['packages'].append(dict(package, alias='other', targets=['codex']))
        self.save_lock()
        with self.assertRaisesRegex(AssertionError, 'exact binding alias/source/target'):
            self.prove()

    def test_extra_and_missing_durable_bindings_are_rejected(self):
        clients = self.state['installations'][0]['clients']
        clients['foreign-binding'] = dict(self.cursor, client_id='claude',
                                           client_binding_id='foreign-binding')
        self.save()
        with self.assertRaisesRegex(AssertionError, 'durable targets'):
            self.prove()
        del clients['foreign-binding']
        del clients['binding']
        self.save()
        with self.assertRaisesRegex(AssertionError, 'durable targets'):
            self.prove()

    def test_result_cannot_relabel_failure_or_a_successful_peer(self):
        for index, wrong in ((0, 'succeeded'), (0, 'unchanged'), (0, 'skipped'), (1, 'failed')):
            with self.subTest(index=index, wrong=wrong):
                original = self.result['items'][index]['status']
                self.result['items'][index]['status'] = wrong
                with self.assertRaisesRegex(AssertionError, 'reported binding outcome'):
                    self.prove()
                self.result['items'][index]['status'] = original

    def test_result_requires_matching_alias_destination_and_partial_failure(self):
        row = self.result['items'][0]
        for field, wrong in (('alias', 'other'), ('destinationPath', self.cursor['target_locator'])):
            with self.subTest(field=field):
                original = row[field]
                row[field] = wrong
                with self.assertRaisesRegex(AssertionError, 'owned alias or destination'):
                    self.prove()
                row[field] = original
        self.result['result'] = 'success'
        with self.assertRaisesRegex(AssertionError, 'partial failure'):
            self.prove()

    def test_result_requires_exact_reviewed_targets(self):
        self.result['items'].append(dict(self.result['items'][0], targetId='claude'))
        with self.assertRaisesRegex(AssertionError, 'reported targets'):
            self.prove()

    def test_healthy_cursor_cannot_hide_a_failed_lifecycle(self):
        self.cursor.update(activation='failed', verification='failed')
        self.save()
        with self.assertRaisesRegex(AssertionError, 'successful prepared boundary'):
            self.prove()


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

    def stalled_peer(self):
        peer = socket.create_connection(('127.0.0.1', self.publisher.server.server_port), timeout=2)
        self.addCleanup(peer.close)
        deadline = time.monotonic() + 2
        while time.monotonic() < deadline:
            with self.publisher.server.connection_lock:
                if self.publisher.server.connections:
                    return peer
            time.sleep(0.005)
        self.fail('raw stalled peer was never dispatched')

    def test_stalled_tls_peer_does_not_block_verified_requests(self):
        self.publisher.server.handshake_timeout = 5
        peer = self.stalled_peer()
        context = ssl.create_default_context(cafile=str(self.publisher.ca))
        opener = self.opener(context)
        with opener.open(self.publisher.url + '/.well-known/agent-skills/index.json', timeout=2) as response:
            self.assertEqual(json.load(response)['skills'][0]['name'], 'console-proof')
        self.assertEqual(len(self.publisher.requests), 1)
        # The healthy request completed while the first connection still had
        # not sent a TLS ClientHello; waiting for its timeout cannot qualify.
        peer.settimeout(0.05)
        with self.assertRaises(socket.timeout):
            peer.recv(1)

    def test_stalled_handshake_times_out_and_request_threads_are_cleaned_up(self):
        peer = self.stalled_peer()
        self.assertEqual(peer.recv(1), b'')
        deadline = time.monotonic() + 2
        while time.monotonic() < deadline:
            with self.publisher.server.connection_lock:
                if not self.publisher.server.connections:
                    break
            time.sleep(0.005)
        else:
            self.fail('timed-out handshake retained an active peer')
        self.assertEqual(self.publisher.requests, [])

    def test_close_cleans_stalled_peer_and_joins_dispatch_threads(self):
        self.publisher.server.handshake_timeout = 5
        peer = self.stalled_peer()
        failures = []

        def close_publisher():
            try:
                self.publisher.close()
            except Exception as error:
                failures.append(error)

        closer = source.threading.Thread(target=close_publisher, daemon=True)
        closer.start()
        closer.join(timeout=3)
        self.assertFalse(closer.is_alive(), 'publisher close exceeded the cleanup deadline')
        self.assertEqual(failures, [])
        self.assertFalse(self.publisher.thread.is_alive())
        with self.publisher.server.connection_lock:
            self.assertFalse(self.publisher.server.connections)
        self.assertEqual(peer.recv(1), b'')


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

    def test_native_call_log_is_read_as_utf8(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            calls = [['plugin', 'add', 'документы@источник', '--json']]
            (root / 'native-calls.jsonl').write_text(
                '\n'.join(json.dumps(row, ensure_ascii=False) for row in calls) + '\n',
                encoding='utf-8')
            read_text = Path.read_text
            encodings = []

            def observed_read(path, *args, **kwargs):
                encodings.append(kwargs.get('encoding'))
                return read_text(path, *args, **kwargs)

            with patch.object(Path, 'read_text', observed_read):
                self.assertEqual(source.SkillsFixture.calls(SimpleNamespace(root=root)), calls)
            self.assertEqual(encodings, ['utf-8'])

    def test_missing_git_fails_instead_of_swapping_source(self):
        with patch.object(source.shutil, 'which', return_value=None):
            with self.assertRaises(AssertionError):
                source.public_git_environment(SimpleNamespace(env={'PATH': 'stubs'}))

    def test_explicit_unsupported_case_remains_requested(self):
        self.assertEqual(matrix.selected_cases(['review-redirect'], 'all', 'win32'),
                         ['review-redirect'])

    def test_windows_default_keeps_actual_skills_lifecycle_cases(self):
        selected = matrix.selected_cases(None, 'all', 'win32')
        for required in ('fresh', 'declared-repair-json', 'unchanged', 'unchanged-failure', 'partial-json',
                         'partial-decline', 'partial-retry', 'log-unavailable', 'review-ctrl-c',
                         'review-eof', 'plain-browser', 'explicit-ui'):
            self.assertIn(required, selected)
        self.assertNotIn('review-redirect', selected)

    def test_real_journal_pidfd_control_is_selected_only_on_linux_by_default(self):
        self.assertIn('recovery-refusal', matrix.selected_cases(None, 'all', 'linux'))
        self.assertNotIn('recovery-refusal', matrix.selected_cases(None, 'all', 'darwin'))
        self.assertNotIn('recovery-refusal', matrix.selected_cases(None, 'all', 'win32'))
        self.assertEqual(matrix.selected_cases(['recovery-refusal'], 'all', 'darwin'),
                         ['recovery-refusal'])

    def test_source_kind_is_explicit_for_every_platform_and_browser_control(self):
        self.assertEqual(matrix.source_for_case('fresh', 'auto', 'linux'), 'publisher')
        self.assertEqual(matrix.source_for_case('fresh', 'auto', 'darwin'), 'github')
        self.assertEqual(matrix.source_for_case('fresh', 'auto', 'win32'), 'github')
        self.assertEqual(matrix.source_for_case('fresh', 'github', 'linux'), 'github')
        self.assertEqual(matrix.source_for_case('plain-browser', 'publisher', 'linux'), 'github')

    def test_website_cannot_stand_in_for_catalog_browser_coverage(self):
        self.assertEqual(matrix.BROWSER_CASES,
                         {'catalog-redirect', 'catalog-both-redirect', 'plain-browser', 'explicit-ui'})
        self.assertTrue(source.GITHUB_SOURCE.endswith('@' + source.SOURCE_SHA))
        self.assertNotIn('--ref', source.GITHUB_SOURCE)


class ReportProvenanceTests(unittest.TestCase):
    def run_report(self, root, supplied=True, mutate=None):
        binary = root / 'agentplugins'
        binary.write_bytes(b'unit-test CLI identity')
        native = root / 'supplied-native'
        native.write_bytes(b'unit-test responder identity')
        artifacts = root / 'evidence'
        argv = ['skills_matrix.py', '--binary', str(binary), '--product-sha', 'a' * 40,
                '--artifacts', str(artifacts), '--case', 'fresh']
        if supplied:
            argv += ['--native-fixture', str(native)]
        else:
            argv += ['--build-go', sys.executable]

        def build_stub(_go, output):
            nonlocal native
            native = output
            native.write_bytes(b'unit-test responder identity')
            return native

        def scanner_stub(args):
            args.scanner_path = None
            return 'unit-test scanner metadata'

        def case_stub(_name, evidence):
            evidence.mkdir()
            if mutate:
                (native if mutate == 'native' else binary).write_bytes(b'changed during case')

        with patch.object(matrix.sys, 'argv', argv), \
                patch.object(matrix, 'prepare_scanner', side_effect=scanner_stub), \
                patch.object(matrix, 'build_native_fixture', side_effect=build_stub) as build, \
                patch.object(matrix, 'Matrix') as runner, \
                patch.object(matrix.subprocess, 'check_output', side_effect=['a' * 40 + '\n', '']), \
                patch('builtins.print'):
            runner.return_value.run_case.side_effect = case_stub
            code = matrix.main()
            self.assertEqual(build.call_count, 0 if supplied else 1)
        return code, json.loads((artifacts / 'report.json').read_text()), native

    def test_supplied_and_built_responder_provenance_are_distinct(self):
        for supplied in (True, False):
            with self.subTest(supplied=supplied), tempfile.TemporaryDirectory() as temporary:
                code, report, native = self.run_report(Path(temporary), supplied=supplied)
                self.assertEqual(code, 0)
                self.assertEqual(report['native_fixture_path'], str(native.resolve()))
                self.assertEqual(report['native_fixture_origin'], 'supplied' if supplied else 'built')
                self.assertEqual(report['native_fixture_sha256'],
                                 hashlib.sha256(b'unit-test responder identity').hexdigest())
                self.assertEqual(report['native_fixture_embedded_source_sha256'],
                                 hashlib.sha256(source.NATIVE_SOURCE.encode('utf-8')).hexdigest())

    def test_case_cannot_pass_after_cli_or_responder_bytes_change(self):
        for changed in ('cli', 'native'):
            with self.subTest(changed=changed), tempfile.TemporaryDirectory() as temporary:
                code, report, _native = self.run_report(Path(temporary), mutate=changed)
                self.assertEqual(code, 1)
                self.assertEqual(report['cases'][0]['status'], 'fail')
                self.assertIn('binary changed during qualification', report['cases'][0]['error'])


if __name__ == '__main__':
    unittest.main()
