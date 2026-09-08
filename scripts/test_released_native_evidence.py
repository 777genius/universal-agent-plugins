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


if __name__ == '__main__':
    unittest.main()
