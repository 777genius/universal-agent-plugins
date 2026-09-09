#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path
import tempfile
import shutil
from unittest.mock import patch
import unittest
import zipfile

SPEC = importlib.util.spec_from_file_location('proof', Path(__file__).with_name('verify-released-native-evidence.py'))
proof = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(proof)


class EvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'input'
        self.root.mkdir()
        for client, tests in proof.CLIENT_TESTS.items():
            for target in proof.TARGETS:
                folder = self.root / f'released-native-client-{client}-{target}'
                folder.mkdir()
                transcript = ''.join(f'--- PASS: {test} (0.1s)\n' for test in sorted(tests)).encode()
                (folder / 'native-tests.log').write_bytes(transcript)
                suffix = '.exe' if target.startswith('windows-') else ''
                asset = 'agentplugins_1.2.3_' + target.replace('-', '_') + suffix
                record = {'schema_version': 1, 'client': client, 'target': target, 'commit': 'b'*40, 'tree': 'c'*40,
                          'status': 'passed', 'exit_code': 0, 'skipped_tests': [], 'passed_tests': sorted(tests),
                          'artifact_sha256': {'native-tests.log': proof.digest(transcript)}, 'installer_sha256': 'd'*64,
                          'installer_version_measured': 'agentplugins 1.2.3',
                          'harness_build_sha256': {'native-probe'+suffix: 'e'*64, 'repotests'+suffix: 'f'*64},
                          'installer_release': {'repository': proof.REPOSITORY, 'acquisition': 'public GitHub release download',
                              'version': '1.2.3', 'tag': 'agentplugins-v1.2.3', 'commit': 'a'*40, 'tree': 'd'*40,
                              'file': asset, 'size': 10, 'binary_sha256': 'd'*64, 'manifest_sha256': 'a'*64, 'checksums_sha256': 'b'*64,
                              'attestations': {n: [{'verificationResult': {}}] for n in (asset, 'checksums.txt', 'release-manifest.json')}}}
                self.enrich(folder, record, client, target)
                (folder / 'runner-evidence.json').write_text(json.dumps(record))
        self.record_path = next(self.root.rglob('runner-evidence.json'))

    def enrich(self, folder, record, client, target):
        for field, key in (('client_asset', client), ('scanner_asset', 'lintai'), ('ripgrep_asset', 'rg')):
            kind, repo, version, archive, integrity, _ = proof.pins()[target][key]
            record[field] = dict(source=f'https://github.com/{repo}/releases/tag/{version}' if kind == 'github' else f'https://registry.npmjs.org/{repo}/-/{archive}', version=version, archive=archive, archive_integrity=integrity, binary_sha256='1'*64)
        release = record['installer_release']
        for name in release['attestations']:
            sha = {'checksums.txt':'b'*64, 'release-manifest.json':'a'*64}.get(name,'d'*64)
            release['attestations'][name] = [{'verificationResult': {'signature': {'certificate': {
                'sourceRepositoryURI':'https://github.com/'+proof.REPOSITORY, 'sourceRepositoryDigest':'a'*40,
                'runnerEnvironment':'github-hosted', 'issuer':'https://token.actions.githubusercontent.com',
                'buildSignerURI':'https://github.com/'+proof.REPOSITORY+'/.github/workflows/agentplugins-release.yml@refs/heads/main', 'buildSignerDigest':'a'*40}},
                'statement': {'predicateType':'https://slsa.dev/provenance/v1', 'subject':[{'name':name,'digest':{'sha256':sha}}]}, 'verifiedTimestamps':[{'type':'Tlog','timestamp':'2026-09-08T00:00:00Z'}]}}]
        shapes = {'codex':[('codex','evidence.json',{})], 'claude':[('claude-lifecycle','evidence.json',{}),('claude-runtime','evidence.json',{'runtime_scope':'stdio_default+stdio_explicit+HTTP+skill','provider':'scripted_loopback','real_model':'not_evaluated','oauth':'not_evaluated','stdio_runtime':'passed','stdio_cwd_argv_env_data':'passed','installer_data_retention':'passed'})], 'opencode':[
            ('opencode-lifecycle','evidence.json',{'config_route_exercised':route}) for route in ('opencode.json','opencode.jsonc')] + [
            ('opencode-extended','extended-runtime-evidence.json',{'probe_sha256':'e'*64,'status':'passed','provider':'scripted_loopback_no_real_model'})] + [
            ('opencode-collision','tool-collision-evidence.json',{'logical_servers':names,'model':'scripted_loopback_no_real_model','oauth':'not_evaluated','status':'passed'}) for names in (['api/server'],['api server'],['api/server','api server'])]}
        for index,(kind,name,extra) in enumerate(shapes[client]):
            directory = folder / (f'fixture-{index}' + ('-${PLUGIN_DATA}' if kind == 'opencode-extended' else ''))
            directory.mkdir()
            transcript = b'fixture command transcript\n'
            (directory/'command.log').write_bytes(transcript)
            stages = {key:{'status':'passed'} for key in proof.STAGES[kind]}
            if kind == 'opencode-collision' and extra['logical_servers'] == ['api/server']:
                stages.update({key:{'status':'passed'} for key in 'version-update same-version-refresh repair remove foreign_config_preservation'.split()})
            stages['oauth'] = {'status':'not_evaluated'}
            data = dict(installer_base_commit='a'*40,installer_tree='d'*40,installer_patch_sha256=[], installer_sha256='d'*64,client_sha256='1'*64,
                scanner={'binary_sha256':'1'*64,'archive_sha256':record['scanner_asset']['archive_integrity'][7:], 'platform':target,'source_tag':record['scanner_asset']['version']},
                stages=stages,transcript_sha256={'command.log':proof.digest(transcript)},**extra)
            data['client_version_measured' if kind in ('claude-lifecycle','opencode-lifecycle') else 'client_version'] = {'codex':'0.153.4','claude':'2.1.263 (Claude Code)','opencode':'1.18.29'}[client]
            if kind == 'claude-runtime': data['stages'] = {k:'passed' for k in proof.STAGES[kind]}
            if target == 'windows-amd64' and kind == 'claude-lifecycle':
                data['stages']['stdio_discovery'] = {'status':'observed_unsupported','reason':'managed_stdio_platform_unsupported'}
            if target == 'windows-amd64' and kind == 'claude-runtime':
                data.update(stdio_runtime='observed_unsupported',stdio_cwd_argv_env_data='not_evaluated',runtime_scope='HTTP+installed-skill')
            (directory/name).write_text(json.dumps(data))
        for path in folder.rglob('*'):
            if path.is_file(): record['artifact_sha256'][path.relative_to(folder).as_posix()] = proof.digest(path.read_bytes())

    def refresh_hashes(self):
        folder = self.record_path.parent
        record = json.loads(self.record_path.read_text())
        record['artifact_sha256'] = {p.relative_to(folder).as_posix():proof.digest(p.read_bytes()) for p in folder.rglob('*') if p.is_file() and p != self.record_path}
        self.record_path.write_text(json.dumps(record))

    def test_structured_evidence_and_pin_failures(self):
        fixture = next(p for p in self.record_path.parent.rglob('evidence.json'))
        original = fixture.read_bytes()
        for change in (lambda r:r['stages'].update(install={'status':'failed'}),lambda r:r.update(installer_sha256='0'*64),lambda r:r['transcript_sha256'].update({'absent.log':'0'*64})):
            fixture.write_bytes(original)
            data=json.loads(fixture.read_text());change(data);fixture.write_text(json.dumps(data));self.refresh_hashes()
            with self.assertRaises(ValueError):self.verify()
        fixture.unlink(); self.refresh_hashes()
        with self.assertRaisesRegex(ValueError,'structured fixtures'): self.verify()

    def test_pin_and_empty_provenance_rejected(self):
        original = self.record_path.read_bytes()
        for change in (lambda r:r['client_asset'].update(archive_integrity='sha256:'+'0'*64),lambda r:r['scanner_asset'].update(version='bad'),lambda r:r['installer_release']['attestations'].update({'checksums.txt':[{'verificationResult':{}}]})):
            self.record_path.write_bytes(original);self.mutate(change)
            with self.assertRaises(ValueError):self.verify()

    def test_literal_token_safe_but_traversal_not_safe(self):
        proof.safe_name('fixture-${PLUGIN_DATA}/extended-runtime-evidence.json')
        for name in ('../outside.log','/root.log','a//b.log','a/../b.log','C:/b.log','a\\b.log','a\x00b.log'):
            with self.assertRaises(ValueError):proof.safe_name(name)

    def verify(self):
        return proof.verify(self.root, '1.2.3', 'a'*40, 'b'*40, 'c'*40)

    def mutate(self, change):
        record = json.loads(self.record_path.read_text())
        change(record)
        self.record_path.write_text(json.dumps(record))

    def linux_supplement(self):
        # Synthetic fixture conversion only; these are never lifecycle receipts.
        for folder in list(self.root.iterdir()):
            if not folder.name.endswith('linux-arm64'):
                shutil.rmtree(folder)
                continue
            for path in folder.rglob('*.json'):
                path.write_text(path.read_text().replace('linux-arm64', 'linux-amd64').replace('linux_arm64', 'linux_amd64'))
            folder = folder.rename(folder.with_name(folder.name.replace('linux-arm64', 'linux-amd64')))
            self.record_path = folder / 'runner-evidence.json'
            self.refresh_hashes()
        # Reuse synthetic tool metadata with the target name translated, without
        # pretending the unfilled production pins have been verified.
        return {**proof.pins(), 'linux-amd64': {key: tuple(value.replace('linux-arm64', 'linux-amd64') if isinstance(value, str) else value for value in pin) for key, pin in proof.pins()['linux-arm64'].items()}}

    def test_linux_supplement_requires_all_three_and_keeps_historical_default(self):
        synthetic_pins = self.linux_supplement()
        with patch.object(proof, 'pins', return_value=synthetic_pins):
            with self.assertRaisesRegex(ValueError, 'exactly 9'):
                self.verify()
            summary, _ = proof.verify(self.root, '1.2.3', 'a'*40, 'b'*40, 'c'*40, scope='linux-amd64')
            self.assertEqual(summary['scope'], 'linux-amd64')
            self.assertEqual(len(summary['jobs']), 3)
            self.assertEqual({job['target'] for job in summary['jobs']}, {'linux-amd64'})
            self.mutate(lambda r: r.update(passed_tests=[]))
            with self.assertRaisesRegex(ValueError, 'missing required tests'):
                proof.verify(self.root, '1.2.3', 'a'*40, 'b'*40, 'c'*40, scope='linux-amd64')
            shutil.rmtree(self.record_path.parent)
            with self.assertRaisesRegex(ValueError, 'exactly 3'):
                proof.verify(self.root, '1.2.3', 'a'*40, 'b'*40, 'c'*40, scope='linux-amd64')

    def test_linux_supplement_cannot_accept_unfilled_production_pins(self):
        synthetic_pins = self.linux_supplement()
        pin = list(synthetic_pins['linux-amd64']['codex'])
        pin[4] = None
        synthetic_pins['linux-amd64']['codex'] = tuple(pin)
        with patch.object(proof, 'pins', return_value=synthetic_pins):
            with self.assertRaisesRegex(ValueError, 'unfilled'):
                proof.verify(self.root, '1.2.3', 'a'*40, 'b'*40, 'c'*40, scope='linux-amd64')

    def test_exact_matrix_and_deterministic_archive(self):
        summary, files = self.verify()
        self.assertEqual(len(summary['jobs']), 9)
        a, b = Path(self.temp.name)/'a.zip', Path(self.temp.name)/'b.zip'
        proof.archive(a, summary, files)
        proof.archive(b, summary, files)
        self.assertEqual(a.read_bytes(), b.read_bytes())
        with zipfile.ZipFile(a) as z:
            self.assertEqual(set(z.namelist()), set(files) | {'summary.json'})
        with self.assertRaises(FileExistsError):
            proof.archive(a, summary, files)

    def test_identity_failures(self):
        original = self.record_path.read_bytes()
        changes = [lambda r: r.update(commit='0'*40), lambda r: r.update(status='failed'),
                   lambda r: r.update(skipped_tests=['test']), lambda r: r.update(passed_tests=[]),
                   lambda r: r.pop('installer_release'), lambda r: r['installer_release'].update(commit='0'*40),
                   lambda r: r['installer_release'].update(attestations={}),
                   lambda r: r['installer_release'].update(manifest_sha256='0'*64),
                   lambda r: r.update(installer_sha256='0'*64)]
        for change in changes:
            with self.subTest(change=change):
                self.record_path.write_bytes(original)
                self.mutate(change)
                with self.assertRaises(ValueError): self.verify()

    def test_transcript_hash_and_unrecorded_content_fail(self):
        log = self.record_path.parent / 'native-tests.log'
        log.write_text('forged')
        with self.assertRaisesRegex(ValueError, 'hash mismatch'): self.verify()
        self.mutate(lambda r: r['artifact_sha256'].update({'native-tests.log': proof.digest(log.read_bytes())}))
        with self.assertRaisesRegex(ValueError, 'transcript disagrees'): self.verify()

    def test_missing_extra_and_symlink_fail(self):
        extra = self.root / 'extra'
        extra.mkdir()
        with self.assertRaises(ValueError): self.verify()
        extra.rmdir()
        self.record_path.unlink()
        self.record_path.symlink_to(self.root / 'not-present')
        with self.assertRaisesRegex(ValueError, 'symlink'): self.verify()

    def test_unsafe_path_and_duplicate_json_fail(self):
        self.mutate(lambda r: r['artifact_sha256'].update({'../outside.log': 'a'*64}))
        with self.assertRaises(ValueError): self.verify()
        self.record_path.write_text('{"client":"codex","client":"claude"}')
        with self.assertRaisesRegex(ValueError, 'duplicate JSON'): self.verify()


class DraftArchiveTests(unittest.TestCase):
    enrich = EvidenceTests.enrich
    mutate = EvidenceTests.mutate
    """Synthesized fixture bytes only: never published proof or runtime receipts."""
    def setUp(self):
        EvidenceTests.setUp(self)
        import copy
        import io
        import tarfile
        from types import SimpleNamespace
        import native_client_draft as native
        self.native = native
        self.args = SimpleNamespace(release_state='draft', release_repo=proof.REPOSITORY,
            release_version='1.2.3', release_tag='agentplugins-v1.2.3', release_commit='a'*40,
            harness_commit='b'*40, harness_tree='c'*40, scope='historical-nine',
            producer_run_id='12', producer_run_attempt='2', producer_artifact_id='56', release_id='34')
        self.assets, computed = {}, {}
        for target in sorted(native.TARGETS):
            name = 'agentplugins_1.2.3_' + target.replace('-', '_') + ('.exe' if target.startswith('windows') else '')
            body = ('SYNTHESIZED FIXTURE ONLY ' + target).encode()
            self.assets[name] = body
            computed[target] = dict(file=name, size=len(body), sha256=proof.digest(body))
        manifest = dict(schema_version=2, version='1.2.3', tag=self.args.release_tag, commit='a'*40, assets=computed)
        self.assets['release-manifest.json'] = json.dumps(manifest).encode()
        self.assets['THIRD_PARTY_NOTICES.txt'] = b'SYNTHESIZED NOTICES FIXTURE ONLY'
        self.assets['checksums.txt'] = ''.join(proof.digest(b) + '  ' + n + '\n' for n, b in sorted(self.assets.items())).encode()
        self.args.expected_asset_set_digest = proof.digest(self.assets['checksums.txt'])
        verified = dict(repository='777genius/plugin-kit-ai', version='1.2.3', tag=self.args.release_tag,
            commit='a'*40, assets=computed, manifest_schema=2, gate_eligible=True,
            manifest_sha256=proof.digest(self.assets['release-manifest.json']),
            notices=[dict(file='THIRD_PARTY_NOTICES.txt', sha256=proof.digest(self.assets['THIRD_PARTY_NOTICES.txt']))])
        pkg = dict(name='universal-agent-plugins', version='1.2.3', bin={'agentplugins': 'bin/agentplugins.js'})
        pins = dict(schema_version=2, version='1.2.3', npm_package=pkg['name'], repository=verified['repository'],
            tag=self.args.release_tag, assets=computed, producer=dict(repository=verified['repository'],
            tag=self.args.release_tag, commit='a'*40, release_manifest=dict(schema_version=2,
            sha256=verified['manifest_sha256'], version='1.2.3')))
        self.package_files = {'package/package.json': json.dumps(pkg).encode(), 'package/assets.json': json.dumps(pins).encode(),
            'package/THIRD_PARTY_NOTICES.txt': self.assets['THIRD_PARTY_NOTICES.txt'],
            'package/bin/agentplugins.js': b'// SYNTHESIZED FIXTURE ONLY; NEVER EXECUTE'}
        self.bundle_files = {'verified-release.json': json.dumps(verified).encode(),
                             **{'release-assets/'+n: b for n, b in self.assets.items()}}
        self.repack_tarball()
        self.args.producer_bundle = Path(self.temp.name) / 'producer.zip'
        self.write_bundle()
        producer = dict(repository=proof.REPOSITORY, workflow=native.WORKFLOW, commit='a'*40,
            run_id=12, run_attempt=2, artifact_id=56, artifact_digest=self.args.producer_artifact_digest)
        subjects = [dict(name=n, id=100+i, size=len(b), sha256=proof.digest(b)) for i, (n,b) in enumerate(sorted(self.assets.items()))]
        snapshot = dict(id=34, tag=self.args.release_tag, commit='a'*40, draft=True, prerelease=False,
            updated_at='2026-09-09T00:00:00Z', assets=[dict(id=s['id'], name=s['name'], size=s['size'], state='uploaded',
            created_at='2026-09-09T00:00:00Z', updated_at='2026-09-09T00:00:00Z', digest='sha256:'+s['sha256']) for s in subjects])
        final = dict(schema_version=1, release_state='verified-draft', verified_at='2026-09-09T00:00:00+00:00',
            repository=proof.REPOSITORY, release=snapshot, subjects=subjects, asset_set_digest=self.args.expected_asset_set_digest,
            signer_workflow=f'github.com/{proof.REPOSITORY}/{native.WORKFLOW}', producer_commit='a'*40,
            producer_run_id=12, producer_run_attempt=2)
        helpers = {n: proof.digest(('SYNTHESIZED HELPER '+n).encode()) for n in native.helper_hashes(Path(__file__).resolve().parents[1])}
        tarhash = proof.digest(self.bundle_files['fixture.tgz'])
        self.q = dict(schema_version=1, status='passed', release_state='verified-draft', target_scope='historical-nine',
            producer_commit='a'*40, harness_commit='b'*40, harness_tree='c'*40, tarball_sha256=tarhash,
            producer=producer, helper_sha256=helpers, lanes=[], final_draft=final,
            limitation='genuine pinned clients; scripted loopback providers; no real-model or OAuth qualification')
        for path in self.root.rglob('runner-evidence.json'):
            record = json.loads(path.read_bytes())
            release = record['installer_release']
            binary = computed[record['target']]
            release.update(acquisition='authenticated producer npm artifact', release_state='draft',
                producer=producer, release_id=34, tarball_sha256=tarhash, initial_draft=final,
                binary_sha256=binary['sha256'], size=binary['size'], checksums_sha256=self.args.expected_asset_set_digest,
                manifest_sha256=verified['manifest_sha256'])
            template = next(iter(release['attestations'].values()))
            release['attestations'] = {}
            for subject in subjects:
                records = copy.deepcopy(template)
                records[0]['verificationResult']['statement']['subject'] = [{'name':subject['name'], 'digest':{'sha256':subject['sha256']}}]
                release['attestations'][subject['name']] = records
            record.update(helper_sha256=helpers, installer_sha256=binary['sha256'], packaged_acquisition=dict(
                bootstrap_source='local_frozen_asset', tarball_sha256=tarhash,
                launcher_sha256=proof.digest(self.package_files['package/bin/agentplugins.js']), binary_sha256=binary['sha256'],
                size=binary['size'], version='agentplugins 1.2.3', cold_bootstrap=True, warm_without_proof_source=True, npm_ignore_scripts=True))
            for fixture in path.parent.rglob('*.json'):
                if fixture == path: continue
                data = json.loads(fixture.read_bytes())
                data['installer_sha256'] = binary['sha256']
                fixture.write_text(json.dumps(data))
            (path.parent/'initial-draft.json').write_text(json.dumps(final))
            path.write_text(json.dumps(record))
        self.args.qualification = Path(self.temp.name)/'qualification'
        self.args.qualification.mkdir()
        self.sync_records()

    def repack_tarball(self):
        import io
        import tarfile
        stream = io.BytesIO()
        with tarfile.open(fileobj=stream, mode='w:gz') as tar:
            for name, body in self.package_files.items():
                member = tarfile.TarInfo(name); member.size = len(body)
                tar.addfile(member, io.BytesIO(body))
        self.bundle_files['fixture.tgz'] = stream.getvalue()

    def write_bundle(self):
        import io
        stream = io.BytesIO()
        with zipfile.ZipFile(stream, 'w') as zipped:
            for n,b in self.bundle_files.items(): zipped.writestr(n,b)
        self.args.producer_bundle.write_bytes(stream.getvalue())
        self.args.producer_artifact_digest = proof.digest(stream.getvalue())

    def sync_records(self):
        self.q['lanes'] = []
        for path in self.root.rglob('runner-evidence.json'):
            record = json.loads(path.read_bytes())
            record['artifact_sha256'] = {p.relative_to(path.parent).as_posix():proof.digest(p.read_bytes())
                for p in path.parent.rglob('*') if p.is_file() and p != path}
            path.write_text(json.dumps(record))
            self.q['lanes'].append(dict(client=record['client'], target=record['target'], evidence_sha256=proof.digest(path.read_bytes())))
        self.write_qualification()

    def write_qualification(self):
        (self.args.qualification/'qualification.json').write_text(json.dumps(self.q))
        (self.args.qualification/'final-draft.json').write_text(json.dumps(self.q['final_draft']))

    def test_explicit_cli_mode_rejects_mixed_or_missing_inputs(self):
        import sys
        common = ['verify-released-native-evidence.py', str(self.root), '--release-version', '1.2.3',
                  '--release-commit', 'a'*40, '--harness-commit', 'b'*40, '--harness-tree', 'c'*40]
        for options in (['--producer-run-id', '12'], ['--qualification', str(self.args.qualification)],
                        ['--release-state', 'draft'], ['--release-state', 'draft', '--qualification', str(self.args.qualification),
                         '--producer-bundle', str(self.args.producer_bundle)]):
            with self.subTest(options=options), patch.object(sys, 'argv', common + options), self.assertRaises(ValueError):
                proof.main()

    def test_draft_roundtrip_original_bytes_and_no_execution(self):
        with patch('subprocess.Popen', side_effect=AssertionError('offline verifier executed a command')):
            summary, files = proof.verify_draft(self.root, self.args)
        self.assertIn('no fresh attestation or current draft-state', summary['boundary'])
        self.assertEqual(len(summary['jobs']), 9)
        output = Path(self.temp.name)/'draft.zip'
        proof.archive(output, summary, files)
        with zipfile.ZipFile(output) as zipped:
            self.assertEqual(zipped.read('producer-artifact.zip'), self.args.producer_bundle.read_bytes())
            for path in self.root.rglob('*'):
                if path.is_file(): self.assertEqual(zipped.read(path.relative_to(self.root).as_posix()), path.read_bytes())
            self.assertEqual(zipped.read('draft-qualification/qualification.json'), (self.args.qualification/'qualification.json').read_bytes())
        with self.assertRaises(ValueError): EvidenceTests.verify(self)  # Public default rejects draft lanes.

    def test_consumes_actual_scope2_aggregate_schema_offline(self):
        from types import SimpleNamespace
        source = Path(__file__).resolve().parents[1]
        for path in self.root.rglob('runner-evidence.json'):
            record = json.loads(path.read_bytes())
            record['helper_sha256'] = self.native.helper_hashes(source)
            path.write_text(json.dumps(record))
        self.sync_records()
        destination = self.args.qualification/'qualification.json'
        destination.unlink()
        args = SimpleNamespace(**vars(self.args), target_scope=self.args.scope, evidence=self.root,
                               receipt=destination, producer_source=source)
        import os
        initial = self.q['final_draft']
        binding = dict(repository=self.native.REPOSITORY, workflow=self.native.NATIVE_WORKFLOW,
                       harness_commit=self.args.harness_commit, harness_tree=self.args.harness_tree, run_id=99, run_attempt=2,
                       target_scope=self.args.scope, release_tag=self.args.release_tag, release_commit=self.args.release_commit,
                       helper_sha256=self.native.helper_hashes(source), **{k:str(getattr(args,k)) for k in self.native.FIELDS})
        record = json.loads(next(self.root.rglob('runner-evidence.json')).read_bytes())
        snapshot = dict(kind='snapshot', binding=binding, producer_tree=record['installer_release']['tree'], live=initial)
        sha = proof.digest((json.dumps(snapshot, sort_keys=True)+'\n').encode())
        with patch.dict(os.environ, SNAPSHOT_SHA256=sha, LANES_SHA256='b'*64, SNAPSHOT_ARTIFACT_ID='71',
                        SNAPSHOT_ARTIFACT_DIGEST='b'*64, LANES_ARTIFACT_ID='72', LANES_ARTIFACT_DIGEST='b'*64), \
             patch.object(self.native, 'receive', return_value=snapshot), \
             patch('subprocess.Popen', side_effect=AssertionError('unexpected process')):
            self.native.aggregate(source, args)
            lanes = json.loads(destination.read_bytes())
            os.environ['LANES_SHA256'] = self.native.digest(destination)
            destination.unlink()
            destination.with_name('final-draft.json').unlink()
            import copy
            final = copy.deepcopy(initial)
            final['verified_at'] = '2099-01-01T00:00:00+00:00'
            args.metadata = 'recheck'
            with patch.object(self.native, 'receive', side_effect=[snapshot, lanes]), \
                 patch.object(self.native, 'trusted_binding', return_value=binding), \
                 patch.object(self.native, 'authenticate', return_value=self.q['producer']), \
                 patch.object(self.native, 'live_verify', return_value=final), \
                 patch.object(self.native, 'output', return_value=snapshot['producer_tree'].encode()):
                self.native.metadata(source, args)
            summary, _ = proof.verify_draft(self.root, self.args)
            current = json.loads(destination.read_bytes())
            for change in (lambda q:q['invocation']['binding'].update(run_attempt=3),
                           lambda q:q['invocation'].update(lanes_sha256='0'*64), lambda q:q.pop('invocation')):
                modified = copy.deepcopy(current); change(modified)
                destination.write_text(json.dumps(modified))
                with self.assertRaises(ValueError): proof.verify_draft(self.root, self.args)
            destination.write_text(json.dumps(current))
        self.assertEqual(len(summary['jobs']), 9)

    def test_draft_qualification_schema_identities_and_lanes(self):
        import copy
        original = copy.deepcopy(self.q)
        changes = [lambda q:q.update(schema_version=2), lambda q:q.update(schema_version=True),
            lambda q:q.update(unknown=True), lambda q:q.update(status='failed'), lambda q:q.update(release_state='public'),
            lambda q:q.update(harness_tree='0'*40), lambda q:q.update(tarball_sha256='0'*64),
            lambda q:q['producer'].update(run_attempt=1), lambda q:q['producer'].update(artifact_id=57),
            lambda q:q['producer'].update(repository='other/repo'), lambda q:q['producer'].update(workflow='other'),
            lambda q:q.update(helper_sha256={}), lambda q:q['lanes'].pop(), lambda q:q['lanes'].append(q['lanes'][0]),
            lambda q:q['lanes'][0].update(target='unknown'), lambda q:q['lanes'][0].update(evidence_sha256='0'*64),
            lambda q:q['final_draft']['release'].update(id=35), lambda q:q['final_draft']['release'].update(draft=False),
            lambda q:q['final_draft']['release'].update(prerelease=True), lambda q:q['final_draft'].update(asset_set_digest='0'*64),
            lambda q:q['final_draft']['subjects'].append(q['final_draft']['subjects'][0]),
            lambda q:q['final_draft']['release']['assets'][0].update(id=999),
            lambda q:q['final_draft'].update(verified_at='invalid')]
        for change in changes:
            with self.subTest(change=change):
                self.q = copy.deepcopy(original); change(self.q); self.write_qualification()
                with self.assertRaises(ValueError): proof.verify_draft(self.root, self.args)

    def test_draft_lane_tamper_even_when_reindexed(self):
        original = self.record_path.read_bytes()
        changes = [lambda r:r.update(skipped_tests=['skip']), lambda r:r.update(exit_code=True),
            lambda r:r.update(helper_sha256={}), lambda r:r['installer_release']['producer'].update(run_id=99),
            lambda r:r['installer_release'].update(tree='0'*40), lambda r:r['installer_release'].update(release_id=True),
            lambda r:r['installer_release']['initial_draft']['release'].update(id=35),
            lambda r:r['installer_release']['attestations'].pop('THIRD_PARTY_NOTICES.txt'),
            lambda r:r['installer_release']['attestations'].update(extra=[]),
            lambda r:r['packaged_acquisition'].update(warm_without_proof_source=False),
            lambda r:r['packaged_acquisition'].update(launcher_sha256='0'*64), lambda r:r['client_asset'].update(version='bad')]
        for change in changes:
            with self.subTest(change=change):
                self.record_path.write_bytes(original); self.mutate(change); self.sync_records()
                with self.assertRaises(ValueError): proof.verify_draft(self.root, self.args)
        self.record_path.write_bytes(original)
        log = self.record_path.parent/'native-tests.log'
        log.write_bytes(log.read_bytes()+b'    --- SKIP: nested/substage (0.0s)\n')
        self.sync_records()
        with self.assertRaises(ValueError): proof.verify_draft(self.root, self.args)

    def test_draft_bundle_rejects_tamper_paths_assets_and_package_pins(self):
        original = dict(self.bundle_files)
        for name in ('release-assets/extra', '../escape', 'release-assets/CHECKSUMS.TXT'):
            self.bundle_files = original | {name:b'fixture'}; self.write_bundle()
            with self.assertRaises(ValueError): proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)
        for name in ('release-assets/THIRD_PARTY_NOTICES.txt', 'release-assets/checksums.txt', 'verified-release.json'):
            self.bundle_files = dict(original); self.bundle_files[name] = b'{}'; self.write_bundle()
            with self.assertRaises(ValueError): proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)
        self.bundle_files = dict(original)
        pins = json.loads(self.package_files['package/assets.json']); pins['assets'] = {}
        self.package_files['package/assets.json'] = json.dumps(pins).encode()
        self.repack_tarball(); self.write_bundle()
        with self.assertRaises(ValueError): proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)
        with self.assertRaises(ValueError): proof.draft_bundle(b'tampered', self.args, self.native)

    def test_draft_linux_scope_requires_exact_three(self):
        for folder in list(self.root.iterdir()):
            if not folder.name.endswith('linux-arm64'):
                shutil.rmtree(folder)
                continue
            folder = folder.rename(folder.with_name(folder.name.replace('linux-arm64', 'linux-amd64')))
            path = folder/'runner-evidence.json'
            record = json.loads(path.read_bytes())
            record['target'] = 'linux-amd64'
            release = record['installer_release']
            release['file'] = release['file'].replace('linux_arm64', 'linux_amd64')
            binary_hash = proof.digest(self.assets[release['file']])
            release.update(binary_sha256=binary_hash, size=len(self.assets[release['file']]))
            record['installer_sha256'] = binary_hash
            record['packaged_acquisition'].update(binary_sha256=binary_hash, size=release['size'])
            for field, key in (('client_asset', record['client']), ('scanner_asset', 'lintai'), ('ripgrep_asset', 'rg')):
                kind, repo, version, archive, integrity, _ = proof.pins()['linux-amd64'][key]
                record[field].update(source=f'https://github.com/{repo}/releases/tag/{version}' if kind == 'github' else f'https://registry.npmjs.org/{repo}/-/{archive}',
                                     version=version, archive=archive, archive_integrity=integrity)
            for fixture in folder.rglob('*.json'):
                if fixture.name in ('runner-evidence.json', 'initial-draft.json'): continue
                data = json.loads(fixture.read_bytes())
                data['installer_sha256'] = binary_hash
                data['scanner'].update(platform='linux-amd64', archive_sha256=record['scanner_asset']['archive_integrity'][7:])
                fixture.write_text(json.dumps(data))
            path.write_text(json.dumps(record))
        self.args.scope = self.q['target_scope'] = 'linux-amd64'
        self.sync_records()
        summary, _ = proof.verify_draft(self.root, self.args)
        self.assertEqual(len(summary['jobs']), 3)
        self.q['lanes'].pop(); self.write_qualification()
        with self.assertRaisesRegex(ValueError, 'missing qualification lanes'): proof.verify_draft(self.root, self.args)

    def test_draft_archive_member_links_aliases_and_duplicate_json(self):
        import io
        import stat
        for name, mode in (('link', stat.S_IFLNK), ('release-assets/', stat.S_IFREG)):
            stream = io.BytesIO()
            with zipfile.ZipFile(stream, 'w') as zipped:
                member = zipfile.ZipInfo(name); member.external_attr = (mode | 0o644) << 16
                zipped.writestr(member, b'fixture')
            body = stream.getvalue(); self.args.producer_artifact_digest = proof.digest(body)
            with self.assertRaisesRegex(ValueError, 'nonregular ZIP'): proof.draft_bundle(body, self.args, self.native)
        self.bundle_files['VERIFIED-release.json'] = b'{}'; self.write_bundle()
        with self.assertRaisesRegex(ValueError, 'aliased ZIP'): proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)
        del self.bundle_files['VERIFIED-release.json']
        self.bundle_files['verified-release.json'] = self.bundle_files['verified-release.json'].replace(b'"version": "1.2.3"', b'"version": "1.2.3", "version": "1.2.3"')
        self.write_bundle()
        with self.assertRaisesRegex(ValueError, 'duplicate JSON'): proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)

    def test_draft_duplicate_json_and_unknown_directory(self):
        path = self.args.qualification/'qualification.json'
        path.write_bytes(path.read_bytes().replace(b'"schema_version": 1', b'"schema_version": 1, "schema_version": 1'))
        with self.assertRaisesRegex(ValueError, 'duplicate JSON'): proof.verify_draft(self.root, self.args)
        self.write_qualification()
        (self.root/'released-native-client-unknown-linux-arm64').mkdir()
        with self.assertRaisesRegex(ValueError, 'exactly 9'): proof.verify_draft(self.root, self.args)

    def test_original_container_and_qualification_limits_before_read_all(self):
        for name, constant in (('qualification.json', 'QUALIFICATION_JSON_LIMIT'),
                               ('final-draft.json', 'FINAL_DRAFT_JSON_LIMIT'),
                               (None, 'ZIP_CONTAINER_LIMIT')):
            path = self.args.qualification/name if name else self.args.producer_bundle
            with self.subTest(name=name), patch.object(proof, constant, path.stat().st_size - 1), \
                 patch.object(Path, 'read_bytes', side_effect=AssertionError('read-all')), \
                 patch.object(proof.zipfile, 'ZipFile', side_effect=AssertionError('ZIP allocation')):
                with self.assertRaisesRegex(ValueError, 'oversized original'):
                    proof.verify_draft(self.root, self.args)

    def reject_bundle_before_read(self, message):
        with patch.object(zipfile.ZipFile, 'read', side_effect=AssertionError('ZIP body read')) as read:
            with self.assertRaisesRegex(ValueError, message):
                proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)
            read.assert_not_called()

    def test_zip_unexpected_inventory_rejected_before_any_read(self):
        original = dict(self.bundle_files)
        for count in (1, 40):
            with self.subTest(count=count):
                self.bundle_files = original | {f'unexpected-{i}': b'x' for i in range(count)}
                self.write_bundle()
                self.reject_bundle_before_read('unexpected or missing|too many ZIP')

    def test_zip_aggregate_rejected_before_any_read(self):
        total = sum(map(len, self.bundle_files.values()))
        with patch.object(proof, 'ZIP_TOTAL_LIMIT', total - 1):
            self.reject_bundle_before_read('oversized ZIP aggregate')

    def test_zip_compressed_aggregate_and_unsupported_encodings_before_read(self):
        with zipfile.ZipFile(self.args.producer_bundle) as zipped:
            inventory = zipped.infolist()
        for item in inventory:
            item.compress_size = 1000
        with patch.object(zipfile.ZipFile, 'infolist', return_value=inventory), \
             patch.object(proof, 'ZIP_TOTAL_LIMIT', len(inventory) * 1000 - 1):
            self.reject_bundle_before_read('oversized ZIP aggregate')
        for field, value, message in (('orig_filename', 'verified-release.json\0alias', 'aliased ZIP'),
                                      ('compress_type', zipfile.ZIP_LZMA, 'unsupported ZIP'),
                                      ('flag_bits', 1, 'unsupported ZIP')):
            with self.subTest(field=field):
                old = getattr(inventory[0], field)
                setattr(inventory[0], field, value)
                with patch.object(zipfile.ZipFile, 'infolist', return_value=inventory):
                    self.reject_bundle_before_read(message)
                setattr(inventory[0], field, old)

    def test_zip_individual_compressed_and_expanded_bounds_before_read(self):
        for limit in ('ZIP_COMPRESSED_LIMIT', 'ZIP_MEMBER_LIMIT'):
            with self.subTest(limit=limit), patch.object(proof, limit, 1):
                self.reject_bundle_before_read('oversized ZIP member')

    def test_zip_duplicates_and_aliases_before_read(self):
        import warnings
        for name in ('verified-release.json', 'VERIFIED-release.json'):
            with self.subTest(name=name):
                self.write_bundle()
                with warnings.catch_warnings():
                    warnings.simplefilter('ignore', UserWarning)
                    with zipfile.ZipFile(self.args.producer_bundle, 'a') as zipped:
                        zipped.writestr(name, b'{}')
                self.args.producer_artifact_digest = proof.digest(self.args.producer_bundle.read_bytes())
                self.reject_bundle_before_read('duplicate or aliased ZIP')

    def test_zip_budget_preserves_six_large_platform_binaries(self):
        # Exercise inventory only: represent six near-ceiling binaries without
        # allocating them or treating synthetic bodies as valid binary evidence.
        with zipfile.ZipFile(self.args.producer_bundle) as zipped:
            inventory = zipped.infolist()
        binaries = [item for item in inventory if item.filename.startswith('release-assets/agentplugins_')]
        self.assertEqual(len(binaries), 6)
        for item in binaries:
            item.file_size = item.compress_size = (400 << 20) - 1
        with patch.object(zipfile.ZipFile, 'infolist', return_value=inventory), \
             patch.object(zipfile.ZipFile, 'read', side_effect=RuntimeError('inventory accepted')):
            with self.assertRaisesRegex(RuntimeError, 'inventory accepted'):
                proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)

    def test_bundle_uses_one_shared_package_parse(self):
        with patch.object(self.native, 'inspect_package', wraps=self.native.inspect_package) as inspect, \
             patch.object(self.native.tarfile, 'open', wraps=self.native.tarfile.open) as parse:
            proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)
        inspect.assert_called_once()
        parse.assert_called_once()

    def test_tar_logical_inventory_before_any_file_read(self):
        self.package_files['package/../evil'] = b'x'
        self.repack_tarball()
        self.write_bundle()
        with patch.object(self.native.tarfile.TarFile, 'extractfile', side_effect=AssertionError('file read')):
            with self.assertRaisesRegex(ValueError, 'unsafe npm member'):
                proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)

    def reject_tar_before_parser(self, message):
        self.write_bundle()
        with patch.object(self.native.tarfile, 'open', side_effect=AssertionError('tar parser entered')) as inspect:
            with self.assertRaisesRegex(ValueError, message):
                proof.draft_bundle(self.args.producer_bundle.read_bytes(), self.args, self.native)
            inspect.assert_not_called()

    def test_tar_member_count_bomb_before_parser(self):
        self.package_files.update({f'package/extra-{i}': b'' for i in range(8)})
        self.repack_tarball()
        with patch.object(self.native, 'TAR_MEMBER_COUNT', 6):
            self.reject_tar_before_parser('too many npm headers')

    def test_tar_expansion_bomb_including_trailing_padding(self):
        import gzip
        raw = gzip.decompress(self.bundle_files['fixture.tgz'])
        self.bundle_files['fixture.tgz'] = gzip.compress(raw + b'\0' * 8192)
        with patch.object(self.native, 'TAR_EXPANDED_LIMIT', len(raw) + 1024):
            self.reject_tar_before_parser('oversized expanded npm archive')

    def test_tar_body_limit_before_any_selected_read(self):
        import tarfile
        with patch.object(self.native, 'TAR_MEMBER_LIMIT', 16), \
             patch.object(tarfile.TarFile, 'extractfile', side_effect=AssertionError('selected body read')):
            self.reject_tar_before_parser('oversized npm header body')

    def test_tar_metadata_and_sparse_headers_before_parser_allocations(self):
        import gzip
        import tarfile
        cases = [(tarfile.XHDTYPE, self.native.TAR_METADATA_LIMIT + 1, b'', 'oversized npm header'),
                 (tarfile.GNUTYPE_LONGNAME, self.native.TAR_METADATA_LIMIT + 1, b'', 'oversized npm header'),
                 (tarfile.GNUTYPE_SPARSE, 0, b'', 'sparse forbidden'),
                 (tarfile.XHDTYPE, 26, b'26 GNU.sparse.size=9999999\n', 'sparse npm metadata'),
                 (tarfile.XGLTYPE, 18, b'18 size=9999999999\n', 'pax size override')]
        for kind, size, data, message in cases:
            with self.subTest(kind=kind, message=message):
                member = tarfile.TarInfo('package/metadata'); member.type = kind; member.size = size
                raw = member.tobuf() + data.ljust(512, b'\0') + b'\0' * 1024
                self.bundle_files['fixture.tgz'] = gzip.compress(raw)
                with patch.object(tarfile, 'open', side_effect=AssertionError('extension parser entered')):
                    self.reject_tar_before_parser(message)

    def test_tar_extension_headers_count_and_normal_pax_support(self):
        import io
        import tarfile
        stream = io.BytesIO()
        with tarfile.open(fileobj=stream, mode='w:gz', format=tarfile.PAX_FORMAT) as archive:
            for name, body in self.package_files.items():
                member = tarfile.TarInfo(name); member.size = len(body)
                member.pax_headers = {'mtime': '1.25'}
                archive.addfile(member, io.BytesIO(body))
        self.bundle_files['fixture.tgz'] = stream.getvalue()
        self.assertEqual(self.native.read_package(stream.getvalue()), self.package_files)
        # Four logical files have eight physical headers including pax records.
        with patch.object(self.native, 'TAR_MEMBER_COUNT', 6):
            self.reject_tar_before_parser('too many npm headers')


class ContainerPreflightTests(unittest.TestCase):
    def archive_bytes(self, count=1, streaming=False, zip64=False):
        import io
        class Stream(io.BytesIO):
            def seekable(self): return False
            def seek(self, *args): raise OSError('stream')
        stream = Stream() if streaming else io.BytesIO()
        with zipfile.ZipFile(stream, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
            for i in range(count):
                with archive.open(str(i), 'w', force_zip64=zip64) as member:
                    member.write(b'fixture')
            archive.comment = b'producer comment'
        return stream.getvalue()

    def reject(self, body, message):
        with patch.object(proof.zipfile, 'ZipFile', side_effect=AssertionError('ZipFile allocated')) as constructor:
            with self.assertRaisesRegex(ValueError, message):
                # Enter through the production caller with an exact immutable digest.
                from types import SimpleNamespace
                import native_client_draft as native
                args = SimpleNamespace(producer_artifact_digest=proof.digest(body),
                                       release_version='1.2.3', release_tag='agentplugins-v1.2.3')
                proof.draft_bundle(body, args, native)
            constructor.assert_not_called()

    def test_directory_size_before_constructor(self):
        body = self.archive_bytes()
        with patch.object(proof, 'ZIP_DIRECTORY_LIMIT', 45):
            self.reject(body, 'oversized ZIP central directory')

    def test_actual_count_and_declared_mismatch_before_constructor(self):
        import struct
        for count, declared, message in ((13, 13, 'too many ZIP'), (13, 1, 'too many ZIP'),
                                          (1, 2, 'declared-count mismatch'), (1, 0, 'declared-count mismatch')):
            with self.subTest(count=count, declared=declared):
                body = bytearray(self.archive_bytes(count))
                end = body.rfind(b'PK\x05\x06')
                struct.pack_into('<2H', body, end + 8, declared, declared)
                self.reject(bytes(body), message)

    def test_malformed_truncated_directory_and_end_before_constructor(self):
        import struct
        original = self.archive_bytes()
        cd = original.index(b'PK\x01\x02')
        end = original.rfind(b'PK\x05\x06')
        cases = [b'', original[:-1], original[:end], original + b'trailing']
        body = bytearray(original); body[cd:cd+4] = b'bad!'; cases.append(bytes(body))
        body = bytearray(original); struct.pack_into('<H', body, cd+28, 65535); cases.append(bytes(body))
        body = bytearray(original); struct.pack_into('<I', body, end+12, len(body)+1); cases.append(bytes(body))
        body = bytearray(original); struct.pack_into('<I', body, end+16, len(body)+1); cases.append(bytes(body))
        for body in cases:
            with self.subTest(size=len(body)):
                self.reject(body, 'ZIP')

    def test_streaming_zip64_members_comments_and_fixed_zip64_end(self):
        import io
        import struct
        for streaming in (False, True):
            for zip64 in (False, True):
                body = self.archive_bytes(streaming=streaming, zip64=zip64)
                end = body.rfind(b'PK\x05\x06')
                size, offset = struct.unpack_from('<2I', body, end+12)
                record = struct.pack('<4sQ2H2I4Q', b'PK\x06\x06', 44, 45, 45, 0, 0, 1, 1, size, offset)
                locator = struct.pack('<4sIQI', b'PK\x06\x07', 0, end, 1)
                extended = bytearray(record)
                struct.pack_into('<Q', extended, 4, 60)
                extended_body = body[:end]+extended+b'\0'*16+locator+body[end:]
                proof.preflight_zip(extended_body, 12)
                # The local stdlib supports extensible sectors; older versions
                # can still reject them after this bounded preflight.
                with zipfile.ZipFile(io.BytesIO(extended_body)) as archive:
                    self.assertEqual(archive.read('0'), b'fixture')
                sentinel_end = bytearray(body[end:])
                struct.pack_into('<2H2I', sentinel_end, 8, 65535, 65535, 0xffffffff, 0xffffffff)
                for candidate in (body, body[:end]+record+locator+body[end:],
                                  body[:end]+record+locator+sentinel_end):
                    for prefix in (b'', b'prefix'):
                        proof.preflight_zip(prefix+candidate, 12)
                        with zipfile.ZipFile(io.BytesIO(prefix+candidate)) as archive:
                            self.assertEqual(archive.read('0'), b'fixture')

    def test_bad_zip64_before_constructor(self):
        import struct
        body = self.archive_bytes()
        end = body.rfind(b'PK\x05\x06')
        locator = struct.pack('<4sIQI', b'PK\x06\x07', 0, end, 1)
        self.reject(body[:end]+locator+body[end:], 'ZIP64')
        body = bytearray(body)
        struct.pack_into('<2H', body, end+8, 65535, 65535)
        self.reject(bytes(body), 'missing ZIP64')

    def test_container_overhead_before_constructor(self):
        body = self.archive_bytes()
        # Prefix bytes count even though they are outside all member sizes.
        with patch.object(proof, 'ZIP_CONTAINER_LIMIT', len(body)):
            self.reject(b'padding'+body, 'oversized ZIP container')

    def test_original_read_is_bounded_even_after_small_stat(self):
        import io
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp)/'original'
            path.write_bytes(b'x')
            class BoundedStream(io.BytesIO):
                def read(self, size=-1):
                    self.assert_bound(size)
                    return super().read(size)
            stream = BoundedStream(b'x'*18)
            stream.assert_bound = lambda size: self.assertTrue(0 < size <= 17)
            with patch.object(Path, 'open', return_value=stream), \
                 patch.object(Path, 'read_bytes', side_effect=AssertionError('read-all')):
                with self.assertRaisesRegex(ValueError, 'oversized original'):
                    proof.original_file(path, 16)
            path.write_bytes(b'x'*16)
            self.assertEqual(proof.original_file(path, 16), b'x'*16)
            path.write_bytes(b'')
            self.assertEqual(proof.original_file(path, 16), b'')


if __name__ == '__main__':
    unittest.main()
