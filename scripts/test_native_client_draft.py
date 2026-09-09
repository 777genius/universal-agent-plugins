"""Offline draft acquisition/qualification tests: no gh, npm, or client execution."""
import copy
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tarfile
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import zipfile

SPEC = importlib.util.spec_from_file_location('draft', Path(__file__).with_name('native_client_draft.py'))
draft = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(draft)


def args(**changes):
    values = dict(release_state='draft', release_repo=draft.REPOSITORY, target='linux-amd64',
                  release_tag='agentplugins-v1.2.3', release_commit='a' * 40,
                  producer_run_id='12', producer_run_attempt='2', release_id='34',
                  producer_artifact_id='56', producer_artifact_digest='b' * 64,
                  expected_asset_set_digest='c' * 64, producer_source=Path('/producer'))
    return SimpleNamespace(**(values | changes))


def responses():
    run = dict(id=12, run_attempt=2, repository={'full_name': draft.REPOSITORY},
               head_repository={'full_name': draft.REPOSITORY}, head_sha='a' * 40,
               path=draft.WORKFLOW, workflow_id=7, status='completed', conclusion='success',
               event='workflow_dispatch', run_started_at='2026-09-09T01:00:00Z', updated_at='2026-09-09T02:00:00Z')
    names = ['platform-proof / native runtime E2E (' + t + ')' for t in draft.TARGETS]
    names += ['platform-proof / Aggregate all six native platform proofs', 'verified-draft']
    jobs = [{'name': n, 'run_id': 12, 'status': 'completed', 'conclusion': 'success',
             'steps': [{'conclusion': 'success'}]} for n in names]
    artifact = dict(id=56, name='agentplugins-npm-1.2.3', expired=False, size_in_bytes=123,
                    digest='sha256:' + 'b' * 64, workflow_run={'id': 12, 'head_sha': 'a' * 40},
                    created_at='2026-09-09T01:30:00Z')
    return [run, {'id': 7, 'path': draft.WORKFLOW}, [{'jobs': jobs}], [{'artifacts': [artifact]}]]


def tarball(root, verified, changes=None):
    pkg = {'name': 'universal-agent-plugins', 'version': '1.2.3', 'bin': {'agentplugins': 'bin/agentplugins.js'}}
    pins = {'schema_version': 2, 'version': '1.2.3', 'npm_package': pkg['name'],
            'repository': verified['repository'], 'tag': verified['tag'], 'assets': verified['assets'],
            'producer': {'repository': verified['repository'], 'tag': verified['tag'], 'commit': verified['commit'],
                         'release_manifest': {'schema_version': 2, 'sha256': verified['manifest_sha256'], 'version': '1.2.3'}}}
    if changes:
        changes(pkg, pins)
    path = root / 'package.tgz'
    with tarfile.open(path, 'w:gz') as archive:
        for name, body in {'package.json': json.dumps(pkg).encode(), 'assets.json': json.dumps(pins).encode(),
                           'THIRD_PARTY_NOTICES.txt': b'notices', 'bin/agentplugins.js': b'// mock launcher; never execute'}.items():
            member = tarfile.TarInfo('package/' + name)
            member.size = len(body)
            archive.addfile(member, io.BytesIO(body))
    return path


def verified_release():
    return {'version': '1.2.3', 'repository': draft.REPOSITORY, 'tag': 'agentplugins-v1.2.3',
            'commit': 'a' * 40, 'manifest_sha256': 'd' * 64, 'gate_eligible': True,
            'assets': {'linux-amd64': {'file': 'agentplugins_1.2.3_linux_amd64', 'size': 6,
                                     'sha256': hashlib.sha256(b'binary').hexdigest()}}}


def attestation(name, sha):
    return [{'verificationResult': {'signature': {'certificate': {
        'sourceRepositoryURI': 'https://github.com/' + draft.REPOSITORY, 'sourceRepositoryDigest': 'a' * 40,
        'buildSignerURI': 'https://github.com/' + draft.REPOSITORY + '/' + draft.WORKFLOW + '@refs/tags/agentplugins-v1.2.3',
        'buildSignerDigest': 'a' * 40, 'runnerEnvironment': 'github-hosted', 'issuer': 'https://token.actions.githubusercontent.com'}},
        'statement': {'predicateType': 'https://slsa.dev/provenance/v1', 'subject': [{'name': name, 'digest': {'sha256': sha}}]},
        'verifiedTimestamps': ['fixture']}}]


class DraftTests(unittest.TestCase):
    def test_mode_contract_no_network(self):
        draft.validate(args())
        public = args(release_state='public', **{k: '' for k in draft.FIELDS})
        draft.validate(public)
        for field in draft.FIELDS:
            with self.subTest(field=field), patch.object(draft, 'api') as network:
                with self.assertRaises(ValueError):
                    draft.validate(args(**{field: ''}))
                with self.assertRaises(ValueError):
                    draft.validate(args(release_state='public'))
                network.assert_not_called()
        for change in ({'release_commit': 'main'}, {'release_tag': 'latest'}, {'release_repo': 'other/repo'},
                       {'producer_run_id': '-1'}, {'producer_artifact_digest': 'B' * 64}, {'release_id': '0'}):
            with self.assertRaises(ValueError):
                draft.validate(args(**change))

    def test_authenticates_exact_attempt_and_id(self):
        with patch.object(draft, 'api', side_effect=responses()) as api:
            proof = draft.authenticate(args())
        self.assertEqual(proof['artifact_id'], 56)
        self.assertIn('/attempts/2/jobs?', api.call_args_list[2].args[0])
        self.assertEqual(proof['run_attempt'], 2)

    def test_negative_producer_provenance(self):
        for key, bad in [('id', 13), ('run_attempt', 1), ('repository', {'full_name': 'other/repo'}),
                         ('head_repository', {'full_name': 'other/repo'}), ('head_sha', 'e' * 40),
                         ('path', '.github/workflows/other.yml'), ('event', 'pull_request'),
                         ('status', 'in_progress'), ('conclusion', 'failure')]:
            data = responses()
            data[0][key] = bad
            with self.subTest(key=key), patch.object(draft, 'api', side_effect=data):
                with self.assertRaises(ValueError):
                    draft.authenticate(args())
        data = responses()
        data[1]['path'] = '.github/workflows/other.yml'
        with patch.object(draft, 'api', side_effect=data), self.assertRaises(ValueError):
            draft.authenticate(args())

    def test_missing_duplicate_failed_or_skipped_platform_proof(self):
        for mutate in (lambda j: j.pop(), lambda j: j.append(j[0]),
                       lambda j: j[0].update(conclusion='failure'),
                       lambda j: j[0]['steps'][0].update(conclusion='skipped')):
            data = responses()
            mutate(data[2][0]['jobs'])
            with patch.object(draft, 'api', side_effect=data), self.assertRaises(ValueError):
                draft.authenticate(args())

    def test_artifact_provenance_and_attempt(self):
        for change in ({'id': 99}, {'name': 'wrong'}, {'expired': True}, {'digest': 'sha256:' + 'e' * 64},
                       {'workflow_run': {'id': 13, 'head_sha': 'a' * 40}}, {'size_in_bytes': 0},
                       {'created_at': '2026-09-08T01:30:00Z'}):
            data = responses()
            data[3][0]['artifacts'][0].update(change)
            with self.subTest(change=change), patch.object(draft, 'api', side_effect=data), self.assertRaises(ValueError):
                draft.authenticate(args())
        for duplicate in (False, True):
            data = responses()
            records = data[3][0]['artifacts']
            records[:] = records * 2 if duplicate else []
            with patch.object(draft, 'api', side_effect=data), self.assertRaises(ValueError):
                draft.authenticate(args())

    def test_bundle_paths_and_symlinks(self):
        for names in [('../evil',), ('/evil',), ('a\\evil',), ('C:/evil',), ('a/./evil',), ('a//evil',), ('a', 'A')]:
            with tempfile.TemporaryDirectory() as temp:
                body = io.BytesIO()
                with zipfile.ZipFile(body, 'w') as archive:
                    for name in names:
                        archive.writestr(name, b'fixture')
                with self.subTest(names=names), self.assertRaises(ValueError):
                    draft.unpack_bundle(body.getvalue(), Path(temp))
        body = io.BytesIO()
        with zipfile.ZipFile(body, 'w') as archive:
            member = zipfile.ZipInfo('link')
            member.external_attr = 0o120777 << 16
            archive.writestr(member, b'/etc/passwd')
        with tempfile.TemporaryDirectory() as temp, self.assertRaises(ValueError):
            draft.unpack_bundle(body.getvalue(), Path(temp))

    def test_package_pins_and_dependencies(self):
        verified = verified_release()
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            draft.inspect_package(tarball(root, verified), verified)
            changes = [lambda p, a: p.update(version='9.9.9'), lambda p, a: p.update(dependencies={'evil': '*'}),
                       lambda p, a: p.update(scripts={'postinstall': 'evil'}),
                       lambda p, a: a.update(assets={}), lambda p, a: a['producer'].update(commit='e' * 40)]
            for change in changes:
                with self.assertRaises(ValueError):
                    draft.inspect_package(tarball(root, verified, change), verified)

    def test_package_bounds_before_tar_or_json_parser(self):
        import gzip
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            verified = verified_release()
            path = tarball(root, verified)
            original = path.read_bytes()
            raw = gzip.decompress(original)
            cases = [
                (original, 'TAR_MEMBER_COUNT', 3, 'too many npm headers'),
                (original, 'TAR_MEMBER_LIMIT', 16, 'oversized npm header body'),
                (gzip.compress(raw + b'\0' * 8192), 'TAR_EXPANDED_LIMIT',
                 len(raw) + 1024, 'oversized expanded npm archive'),
                # Concatenated gzip streams count toward the same budget.
                (original + gzip.compress(b'\0' * 8192), 'TAR_EXPANDED_LIMIT',
                 len(raw) + 1024, 'oversized expanded npm archive'),
            ]
            for kind, size, data, message in (
                (tarfile.XHDTYPE, draft.TAR_METADATA_LIMIT + 1, b'', 'oversized npm header'),
                (tarfile.GNUTYPE_LONGNAME, draft.TAR_METADATA_LIMIT + 1, b'', 'oversized npm header'),
                (tarfile.GNUTYPE_SPARSE, 0, b'', 'sparse forbidden'),
                (tarfile.XHDTYPE, 26, b'26 GNU.sparse.size=9999999\n', 'sparse npm metadata'),
                (tarfile.XGLTYPE, 18, b'18 size=9999999999\n', 'pax size override'),
            ):
                member = tarfile.TarInfo('package/metadata')
                member.type, member.size = kind, size
                body = gzip.compress(member.tobuf() + data.ljust(512, b'\0') + b'\0' * 1024)
                cases.append((body, 'TAR_MEMBER_COUNT', 128, message))
            for body, limit, value, message in cases:
                path.write_bytes(body)
                with self.subTest(message=message), patch.object(draft, limit, value), \
                     patch.object(tarfile, 'open', side_effect=AssertionError('tar parser entered')), \
                     patch.object(draft.json, 'loads', side_effect=AssertionError('JSON parser entered')), \
                     patch.object(draft, 'output', side_effect=AssertionError('command executed')):
                    with self.assertRaisesRegex(ValueError, message):
                        draft.inspect_package(path, verified)
                    with self.assertRaisesRegex(ValueError, message):
                        draft.bootstrap(path, root, verified, 'linux-amd64', root, {})

    def test_package_inventory_before_any_file_read_and_normal_extensions(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            verified = verified_release()
            path = tarball(root, verified)
            files = draft.inspect_package(path, verified)
            files['package/' + 'long-' * 25] = b'extra source'
            for format in (tarfile.PAX_FORMAT, tarfile.GNU_FORMAT):
                with self.subTest(format=format):
                    with tarfile.open(path, 'w:gz', format=format) as archive:
                        for name, body in files.items():
                            member = tarfile.TarInfo(name)
                            member.size = len(body)
                            if format == tarfile.PAX_FORMAT:
                                member.pax_headers = {'mtime': '1.25'}
                            archive.addfile(member, io.BytesIO(body))
                    self.assertEqual(draft.inspect_package(path, verified), files)
            # An unsafe final logical member must fail before even the first
            # package body is read (after the physical inventory passed).
            with tarfile.open(path, 'w:gz') as archive:
                for name, body in (files | {'package/../evil': b'x'}).items():
                    member = tarfile.TarInfo(name)
                    member.size = len(body)
                    archive.addfile(member, io.BytesIO(body))
            with patch.object(tarfile.TarFile, 'extractfile', side_effect=AssertionError('file read')):
                with self.assertRaisesRegex(ValueError, 'unsafe npm member'):
                    draft.inspect_package(path, verified)

    def test_attestation_source_signer_subject(self):
        draft.verify_attestation(attestation('asset', 'b' * 64), 'asset', 'b' * 64, 'a' * 40)
        for field in ('sourceRepositoryURI', 'sourceRepositoryDigest', 'buildSignerURI', 'buildSignerDigest', 'runnerEnvironment', 'issuer'):
            data = attestation('asset', 'b' * 64)
            data[0]['verificationResult']['signature']['certificate'][field] = 'wrong'
            with self.subTest(field=field), self.assertRaises(ValueError):
                draft.verify_attestation(data, 'asset', 'b' * 64, 'a' * 40)
        for name, sha in [('other', 'b' * 64), ('asset', 'c' * 64)]:
            with self.assertRaises(ValueError):
                draft.verify_attestation(attestation(name, sha), 'asset', 'b' * 64, 'a' * 40)

    def test_live_helper_contract_and_missing_draft(self):
        with tempfile.TemporaryDirectory() as temp:
            receipt = Path(temp) / 'receipt.json'
            with patch.object(draft, 'output', side_effect=RuntimeError('missing draft')) as child:
                with self.assertRaises(RuntimeError):
                    draft.live_verify(Path('/harness'), Path('/producer'), args(), receipt)
            self.assertFalse(receipt.exists())
            command = child.call_args.args[0]
            self.assertIn('/harness/scripts/verify-agentplugins-draft.py', command)
            self.assertIn('--run-attempt', command)
            self.assertNotIn('--release-state', command)

    def test_acquisition_artifact_tamper_before_extraction_or_execution(self):
        with tempfile.TemporaryDirectory() as temp:
            with patch.object(draft, 'authenticate', return_value={}), patch.object(draft, 'live_verify', return_value={}), \
                 patch.object(draft, 'output', side_effect=[b'a' * 40, b'', b'd' * 40, b'altered zip']), \
                 patch.object(draft, 'unpack_bundle') as unpack:
                with self.assertRaisesRegex(ValueError, 'download digest'):
                    draft.acquire(Path(temp), Path(temp), args(), Path(temp))
                unpack.assert_not_called()

    def test_bootstrap_uses_exact_tarball_and_fresh_allowlist(self):
        self.bootstrap_case()

    def test_bootstrap_rejects_cached_binary_tamper(self):
        with self.assertRaisesRegex(ValueError, 'cached binary mismatch'):
            self.bootstrap_case(tamper=True)

    def bootstrap_case(self, tamper=False):
        matrix = draft.load_script('run-native-client-matrix')
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            home = root / 'home'
            home.mkdir()
            node_dir = root / 'node'
            (node_dir / 'node_modules/npm/bin').mkdir(parents=True)
            node = node_dir / 'node'
            node.write_bytes(b'fixture')
            (node_dir / 'node_modules/npm/bin/npm-cli.js').write_bytes(b'fixture')
            verified = verified_release()
            package = tarball(root, verified)
            calls = []
            forbidden = {'GH_TOKEN': 'sentinel', 'GITHUB_TOKEN': 'sentinel', 'NPM_TOKEN': 'sentinel',
                         'OPENAI_API_KEY': 'sentinel', 'ANTHROPIC_API_KEY': 'sentinel', 'NODE_OPTIONS': '--require evil',
                         'HOME': '/ambient', 'APPDATA': '/ambient', 'XDG_CONFIG_HOME': '/ambient', 'PATH': '/ambient'}
            with patch.dict(os.environ, forbidden):
                env = matrix.profile_environment(home, 'linux-amd64') | {'PATH': '/usr/bin:/bin', 'TMPDIR': str(root)}
                def execute(command, **kwargs):
                    runtime = kwargs['env']
                    calls.append((command, dict(runtime)))
                    for key in forbidden:
                        self.assertNotEqual(runtime.get(key), forbidden[key], key)
                    if 'install' in command:
                        self.assertIn('--ignore-scripts', command)
                        self.assertIn('--offline', command)
                        self.assertEqual(command[-1], str(package))
                        installed = root / 'npm-project/node_modules/universal-agent-plugins'
                        with tarfile.open(package) as archive:
                            for m in archive.getmembers():
                                dest = installed.joinpath(*Path(m.name).parts[1:])
                                dest.parent.mkdir(parents=True, exist_ok=True)
                                dest.write_bytes(archive.extractfile(m).read())
                        return b''
                    binary = root / 'npm-binary-cache/1.2.3/linux-amd64/agentplugins'
                    if not binary.exists():
                        self.assertEqual(runtime['AGENTPLUGINS_INTERNAL_PROOF_MODE'], 'local-frozen-release-asset-v1')
                        binary.parent.mkdir(parents=True)
                        binary.write_bytes(b'tamper' if tamper else b'binary')
                    else:
                        self.assertNotIn('AGENTPLUGINS_INTERNAL_PROOF_MODE', runtime)
                        self.assertNotIn('AGENTPLUGINS_INTERNAL_PROOF_BINARY', runtime)
                    return b'agentplugins 1.2.3\n'
                with patch.object(draft.shutil, 'which', return_value=str(node)), patch.object(draft, 'output', side_effect=execute):
                    binary, proof = draft.bootstrap(package, root, verified, 'linux-amd64', root, env)
            self.assertEqual(binary.read_bytes(), b'binary')
            self.assertTrue(proof['warm_without_proof_source'])
            self.assertEqual(len(calls), 4)
            self.assertFalse(any(k.startswith('AGENTPLUGINS_INTERNAL_PROOF') for k in env))


class AggregateTests(unittest.TestCase):
    def fixture(self, root):
        matrix = draft.load_script('run-native-client-matrix')
        source = Path(__file__).resolve().parents[1]
        initial = {'release': {'id': 34, 'assets': []}, 'subjects': [{'name': 'agentplugins_1.2.3_linux_amd64', 'sha256': '2' * 64, 'size': 6}]}
        producer = {'repository': draft.REPOSITORY, 'workflow': draft.WORKFLOW, 'commit': 'a' * 40,
                    'run_id': 12, 'run_attempt': 2, 'artifact_id': 56, 'artifact_digest': 'b' * 64}
        paths = []
        for client, tests in matrix.REQUIRED_TESTS.items():
            lane = root / client
            lane.mkdir()
            log = lane / 'native-tests.log'
            log.write_text(''.join('--- PASS: ' + t + ' (0.1s)\n' for t in sorted(tests)))
            release = dict(release_state='draft', repository=draft.REPOSITORY, tag='agentplugins-v1.2.3',
                           version='1.2.3', commit='a' * 40, tree='f' * 40, release_id=34, checksums_sha256='c' * 64,
                           producer=producer, tarball_sha256='1' * 64, binary_sha256='2' * 64, size=6, file='agentplugins_1.2.3_linux_amd64',
                           initial_draft=initial)
            packaged = dict(bootstrap_source='local_frozen_asset', cold_bootstrap=True, warm_without_proof_source=True,
                            npm_ignore_scripts=True, tarball_sha256='1' * 64, binary_sha256='2' * 64,
                            size=6, version='agentplugins 1.2.3')
            record = dict(client=client, target='linux-amd64', status='passed', exit_code=0, skipped_tests=[],
                          commit='d' * 40, tree='e' * 40, installer_release=release,
                          packaged_acquisition=packaged, installer_sha256='2' * 64,
                          helper_sha256=draft.helper_hashes(source), passed_tests=sorted(tests),
                          artifact_sha256={'native-tests.log': draft.digest(log)})
            path = lane / 'runner-evidence.json'
            path.write_text(json.dumps(record))
            paths.append(path)
        return paths, initial

    def run_aggregate(self, root, initial, *, fixture_failure=False, live_failure=False, changed=False):
        source = Path(__file__).resolve().parents[1]
        final = copy.deepcopy(initial)
        if changed:
            final['release']['id'] = 35
        matrix = draft.load_script('run-native-client-matrix')
        from unittest.mock import Mock
        verifier = Mock()
        if fixture_failure:
            verifier.verify_fixtures.side_effect = ValueError('missing fixture stage')
        with patch.object(draft, 'load_script', side_effect=lambda n: matrix if n == 'run-native-client-matrix' else verifier), \
             patch.object(draft, 'authenticate') as auth, patch.object(draft, 'output', side_effect=[b'a' * 40, b'']), \
             patch.object(draft, 'live_verify', side_effect=RuntimeError('draft gone') if live_failure else None, return_value=final) as live:
            draft.aggregate(source, args(target_scope='linux-amd64', evidence=root, receipt=root / 'qualification.json', harness_commit='d' * 40))
        auth.assert_called_once()
        live.assert_called_once()
        self.assertEqual(verifier.verify_fixtures.call_count, 3)

    def test_complete_lanes_final_verification_then_receipt(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, initial = self.fixture(root)
            self.run_aggregate(root, initial)
            result = json.loads((root / 'qualification.json').read_bytes())
            self.assertEqual(result['status'], 'passed')
            self.assertEqual(len(result['lanes']), 3)
            self.assertEqual(result['producer']['artifact_id'], 56)

    def test_failures_never_issue_qualification(self):
        changes = [lambda r: r.update(status='failed'), lambda r: r.update(exit_code=1),
                   lambda r: r.update(skipped_tests=['stage']), lambda r: r.update(passed_tests=[]),
                   lambda r: r.update(commit='b' * 40), lambda r: r.update(tree='b' * 40),
                   lambda r: r.update(installer_sha256='b' * 64),
                   lambda r: r['installer_release']['producer'].update(run_attempt=1),
                   lambda r: r['installer_release'].update(tarball_sha256='b' * 64),
                   lambda r: r['packaged_acquisition'].update(cold_bootstrap=False),
                   lambda r: r.update(helper_sha256={})]
        for change in changes:
            with tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                paths, initial = self.fixture(root)
                record = json.loads(paths[0].read_bytes())
                change(record)
                paths[0].write_text(json.dumps(record))
                with self.assertRaises(ValueError):
                    self.run_aggregate(root, initial)
                self.assertFalse((root / 'qualification.json').exists())
        for options in ({'fixture_failure': True}, {'live_failure': True}, {'changed': True}):
            with tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                _, initial = self.fixture(root)
                with self.assertRaises((ValueError, RuntimeError)):
                    self.run_aggregate(root, initial, **options)
                self.assertFalse((root / 'qualification.json').exists())

    def test_missing_lanes_and_skipped_substage_transcript(self):
        for missing in (True, False):
            with tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                paths, initial = self.fixture(root)
                if missing:
                    paths[0].unlink()
                else:
                    log = paths[0].parent / 'native-tests.log'
                    log.write_text(log.read_text() + '    --- SKIP: test/substage (0.0s)\n')
                    record = json.loads(paths[0].read_bytes())
                    record['artifact_sha256']['native-tests.log'] = draft.digest(log)
                    paths[0].write_text(json.dumps(record))
                with self.assertRaises(ValueError):
                    self.run_aggregate(root, initial)
                self.assertFalse((root / 'qualification.json').exists())


class AcquisitionIntegrationTests(unittest.TestCase):
    def test_mocked_frozen_bundle_acquisition_and_asset_tamper(self):
        for tamper in ('none', 'missing', 'extra', 'binary', 'tarball', 'checksums'):
            with self.subTest(tamper=tamper), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                verified = verified_release()
                frozen = {'agentplugins_1.2.3_linux_amd64': b'binary', 'checksums.txt': b'checksums',
                          'release-manifest.json': b'manifest', 'THIRD_PARTY_NOTICES.txt': b'notices'}
                live = {'subjects': [{'name': n, 'size': len(b), 'sha256': hashlib.sha256(b).hexdigest()} for n, b in frozen.items()]}
                package = tarball(root, verified, (lambda p, a: a.update(assets={})) if tamper == 'tarball' else None)
                content = io.BytesIO()
                with zipfile.ZipFile(content, 'w') as archive:
                    archive.writestr('package.tgz', package.read_bytes())
                    archive.writestr('verified-release.json', json.dumps(verified))
                    for name, body in frozen.items():
                        if tamper == 'missing' and name == 'THIRD_PARTY_NOTICES.txt':
                            continue
                        if tamper == 'binary' and name.startswith('agentplugins_'):
                            body = b'tamper'
                        if tamper == 'checksums' and name == 'checksums.txt':
                            body = b'tamper'
                        archive.writestr('release-assets/' + name, body)
                    if tamper == 'extra':
                        archive.writestr('release-assets/extra', b'bad')
                body = content.getvalue()
                arguments = args(producer_artifact_digest=hashlib.sha256(body).hexdigest(),
                                 expected_asset_set_digest=hashlib.sha256(b'checksums').hexdigest())
                def child(command, **kwargs):
                    if command[0] == 'git':
                        if command[-1] == 'HEAD': return b'a' * 40
                        if command[-1] == 'HEAD^{tree}': return b'd' * 40
                        return b''
                    if command[1] == 'api': return body
                    if command[0] == 'node': return json.dumps(verified).encode()
                    if command[1:3] == ['attestation', 'verify']:
                        name = Path(command[3]).name
                        return json.dumps(attestation(name, hashlib.sha256(frozen[name]).hexdigest())).encode()
                    self.fail('unexpected acquisition command: ' + repr(command))
                directory = root / 'acquired'
                directory.mkdir()
                with patch.object(draft, 'authenticate', return_value={'artifact_id': 56}), \
                     patch.object(draft, 'live_verify', return_value=live), patch.object(draft, 'output', side_effect=child):
                    if tamper != 'none':
                        with self.assertRaises(ValueError):
                            draft.acquire(root, directory, arguments, root)
                    else:
                        exact, assets, result, record = draft.acquire(root, directory, arguments, root)
                        self.assertEqual(exact.read_bytes(), package.read_bytes())
                        self.assertEqual(record['tarball_sha256'], draft.digest(package))
                        self.assertEqual(result, verified)
                        self.assertEqual(set(record['attestations']), set(frozen))


if __name__ == '__main__':
    unittest.main()
