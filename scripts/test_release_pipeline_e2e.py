"""Offline release rehearsal: real helpers, Git, npm pack and asset byte policy.

Only gh's read/attestation boundary and npm's final publish boundary are faked.
All source repositories and package data are disposable synthetic fixtures.
"""
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
import zipfile

SOURCE = Path(__file__).resolve().parents[1]
REPOSITORY = '777genius/universal-agent-plugins'
WORKFLOW = '.github/workflows/agentplugins-release.yml'
TAG = 'agentplugins-v1.2.3'
TARGETS = ('darwin-amd64', 'darwin-arm64', 'linux-amd64', 'linux-arm64',
           'windows-amd64', 'windows-arm64')


def shell_step(workflow, name):
    text = (SOURCE / '.github/workflows' / workflow).read_text()
    step = text.split('      - name: ' + name + '\n', 1)[1].split('\n      -', 1)[0]
    body = step.split('        run: |\n', 1)[1]
    lines = []
    for line in body.splitlines():
        if line.strip() and not line.startswith('          '):
            break
        lines.append(line[10:])
    return '\n'.join(lines) + '\n'


# Explicit read allowlist: unknown endpoints, writes and mutations fail closed.
FAKE_GH = r'''#!/usr/bin/env python3
import hashlib, json, os, pathlib, sys
state = json.loads(pathlib.Path(os.environ['FIXTURE_STATE']).read_text())
a = sys.argv[1:]
with open(os.environ['GH_LOG'], 'a') as log:
    log.write(json.dumps(a) + '\n')
if a[:2] == ['attestation', 'verify']:
    path = pathlib.Path(a[2])
    expected = ['--repo', state['repository'], '--signer-workflow',
                'github.com/' + state['repository'] + '/' + state['workflow'],
                '--source-digest', state['commit']]
    assert a[3:] == expected
    assert hashlib.sha256(path.read_bytes()).hexdigest() == state['subjects'][path.name]
    print('verified synthetic attestation')
elif a[:1] == ['api']:
    endpoint = a[1]
    assert a[2:] == state['options'][endpoint]
    if endpoint in state['bytes']:
        sys.stdout.buffer.write(pathlib.Path(state['bytes'][endpoint]).read_bytes())
    else:
        print(json.dumps(state['responses'][endpoint]))
else:
    raise SystemExit('forbidden gh operation: ' + repr(a))
'''


class ReleasePipelineE2E(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='TEST-release-pipeline-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / 'source'
        self.source.mkdir()
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self.env = {**os.environ, 'PYTHONDONTWRITEBYTECODE': '1',
                    'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': os.devnull,
                    'GIT_AUTHOR_NAME': 'iliya', 'GIT_AUTHOR_EMAIL': 'iliyazelenkog@gmail.com',
                    'GIT_COMMITTER_NAME': 'iliya', 'GIT_COMMITTER_EMAIL': 'iliyazelenkog@gmail.com',
                    'NPM_CONFIG_CACHE': str(self.root / 'npm-cache'),
                    'NPM_CONFIG_USERCONFIG': str(self.root / 'npmrc'),
                    'FIXTURE_STATE': str(self.root / 'state.json'),
                    'GH_LOG': str(self.root / 'gh.log'),
                    'PATH': str(self.bin) + os.pathsep + os.environ['PATH']}
        self.git('init', '--initial-branch=main')
        policy = self.source / 'npm/agentplugins/scripts/release-assets.js'
        policy.parent.mkdir(parents=True)
        shutil.copyfile(SOURCE / 'npm/agentplugins/scripts/release-assets.js', policy)
        (policy.parent.parent / 'THIRD_PARTY_NOTICES.txt').write_text('synthetic notices\n')
        # The only source commit is in this newly created fixture repository.
        self.git('add', '.')
        for role in ('AUTHOR', 'COMMITTER'):
            self.assertTrue(self.git('var', 'GIT_' + role + '_IDENT').stdout.startswith(
                b'iliya <iliyazelenkog@gmail.com> '))
        self.git('commit', '-m', 'test: synthetic release source')
        self.commit = self.git('rev-parse', 'HEAD').stdout.decode().strip()
        self.git('tag', TAG)
        self.remote = self.root / 'origin.git'
        self.run_command(['git', 'init', '--bare', '--initial-branch=main', str(self.remote)])
        self.git('remote', 'add', 'origin', str(self.remote))
        self.git('push', 'origin', 'main', TAG)
        self.assets = self.root / 'assets'
        self.assets.mkdir()
        for target in TARGETS:
            suffix = target.replace('-', '_') + ('.exe' if target.startswith('windows') else '')
            (self.assets / ('agentplugins_1.2.3_' + suffix)).write_text('synthetic ' + target)
        self.run_command(['node', str(policy), 'prepare', str(self.assets), TAG, self.commit])
        self.package = self.root / 'package'
        self.package.mkdir()
        (self.package / 'package.json').write_text(json.dumps({
            'name': 'universal-agent-plugins', 'version': '1.2.3', 'files': ['index.txt']}))
        (self.package / 'index.txt').write_text('synthetic packed exactly once\n')
        packed = self.run_command(['npm', 'pack', '--ignore-scripts', '--json'], cwd=self.package)
        self.tarball_file = json.loads(packed.stdout)[0]['filename']
        self.tarball = (self.package / self.tarball_file).read_bytes()
        names = ['validate', *(f'build ({os_}, {arch}' + (', .exe)' if os_ == 'windows' else ')')
                  for os_ in ('darwin', 'linux', 'windows') for arch in ('amd64', 'arm64')),
                 'stage-draft', 'platform-proof / Verify release and pack exact npm tarball',
                 *(f'platform-proof / native runtime E2E ({target})' for target in TARGETS),
                 'platform-proof / Aggregate all six native platform proofs',
                 'qualify-go', 'qualify-npm', 'verified-draft']
        self.jobs = [{'name': name, 'run_id': 42, 'status': 'completed', 'conclusion': 'success',
                      'steps': [{'conclusion': 'success'}]} for name in names]
        next(job for job in self.jobs if job['name'] == 'stage-draft')['steps'] = [
            {'name': name, 'conclusion': 'success'} for name in (
                'Prepare immutable stable assets', 'Run actions/attest',
                'Create non-public immutable draft', 'Capture staged release identity',
                'Share frozen draft assets with the read-only proof workflow')]
        self.jobs.append({'name': 'promote-release', 'run_id': 42, 'status': 'completed',
                          'conclusion': 'skipped', 'steps': []})
        self.state = {'repository': REPOSITORY, 'workflow': WORKFLOW, 'commit': self.commit,
                      'responses': {}, 'options': {}, 'bytes': {}, 'subjects': {}}
        prefix = 'repos/' + REPOSITORY
        self.prefix = prefix
        self.run = {'id': 42, 'run_attempt': 2, 'head_sha': self.commit, 'head_branch': TAG,
                    'repository': {'full_name': REPOSITORY}, 'head_repository': {'full_name': REPOSITORY},
                    'path': WORKFLOW, 'workflow_id': 7, 'event': 'workflow_dispatch',
                    'status': 'completed', 'conclusion': 'success',
                    'run_started_at': '2026-10-26T00:00:00Z', 'updated_at': '2099-01-01T00:00:00Z'}
        self.add(prefix + '/actions/runs/42', self.run)
        self.add(prefix + '/actions/workflows/7', {'id': 7, 'path': WORKFLOW})
        self.add(prefix + '/actions/runs/42/attempts/2/jobs?per_page=100', [{'jobs': self.jobs}],
                 ['--paginate', '--slurp'])
        self.add('graphql', {'data': {'repository': {'release': {'databaseId': 123}}}},
                 ['-f', 'query=query($owner:String!,$name:String!,$tag:String!){repository(owner:$owner,name:$name){release(tagName:$tag){databaseId}}}',
                  '-F', 'owner=777genius', '-F', 'name=universal-agent-plugins', '-F', 'tag=' + TAG])
        self.release = {'id': 123, 'tag_name': TAG, 'draft': True, 'prerelease': False,
                        'updated_at': '2026-10-26T01:00:00Z'}
        self.add(prefix + '/releases/123', self.release)
        self.add(prefix + '/releases/tags/' + TAG, self.release)
        self.add(prefix + '/commits/' + TAG, {'sha': self.commit})
        assets = []
        for index, path in enumerate(sorted(self.assets.iterdir()), 1):
            digest = hashlib.sha256(path.read_bytes()).hexdigest()
            assets.append({'id': index, 'name': path.name, 'size': path.stat().st_size,
                           'state': 'uploaded', 'digest': 'sha256:' + digest})
            self.state['subjects'][path.name] = digest
            self.add(prefix + '/releases/assets/' + str(index), None,
                     ['-H', 'Accept: application/octet-stream'], path)
        self.add(prefix + '/releases/123/assets?per_page=100', [assets], ['--paginate', '--slurp'])
        self.artifacts = []
        self.add(prefix + '/actions/runs/42/artifacts?per_page=100', [{'artifacts': self.artifacts}],
                 ['--paginate', '--slurp'])
        self.archive(70, 'agentplugins-npm-1.2.3', {
            self.tarball_file: self.tarball, 'verified-release.json': b'{}'})
        (self.bin / 'gh').write_text(FAKE_GH)
        (self.bin / 'gh').chmod(0o755)
        self.receipt = self.root / 'verified-draft.json'
        self.common = ['--repository', REPOSITORY, '--tag', TAG, '--commit', self.commit,
                       '--run-id', '42', '--run-attempt', '2', '--source', str(self.source)]

    def run_command(self, command, cwd=None, check=True, env=None):
        result = subprocess.run(command, cwd=cwd or self.source, env=env or self.env,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if check:
            self.assertEqual(result.returncode, 0, result.stderr.decode())
        return result

    def git(self, *args):
        return self.run_command(['git', *args])

    def add(self, endpoint, response, options=None, path=None):
        self.state['options'][endpoint] = options or []
        if path:
            self.state['bytes'][endpoint] = str(path)
        else:
            self.state['responses'][endpoint] = response

    def save(self):
        Path(self.env['FIXTURE_STATE']).write_text(json.dumps(self.state))

    def archive(self, identity, name, members):
        body = io.BytesIO()
        with zipfile.ZipFile(body, 'w') as archive:
            for filename, data in members.items():
                archive.writestr(filename, data)
        path = self.root / (str(identity) + '.zip')
        path.write_bytes(body.getvalue())
        self.artifacts.append({'id': identity, 'name': name, 'expired': False,
                               'size_in_bytes': path.stat().st_size,
                               'digest': 'sha256:' + hashlib.sha256(path.read_bytes()).hexdigest(),
                               'created_at': '2026-10-26T02:00:00Z',
                               'workflow_run': {'id': 42, 'head_sha': self.commit}})
        self.add(self.prefix + '/actions/artifacts/' + str(identity) + '/zip', None, path=path)

    def helper(self, name, extra=(), check=True):
        self.save()
        return self.run_command([sys.executable, str(SOURCE / 'scripts' / name),
                                 *self.common, *extra], check=check)

    def qualify(self):
        self.run.update(status='in_progress', conclusion=None)
        self.helper('verify-agentplugins-draft.py', [
            '--release-id', '123', '--asset-set-digest',
            hashlib.sha256((self.assets / 'checksums.txt').read_bytes()).hexdigest(),
            '--receipt', str(self.receipt), '--npm-tarball-file', self.tarball_file])
        receipt = json.loads(self.receipt.read_text())
        self.assertEqual(receipt['schema_version'], 2)
        self.run.update(status='completed', conclusion='success')
        self.archive(71, 'agentplugins-verified-draft-42-2', {'verified-draft.json': self.receipt.read_bytes()})
        return receipt

    def test_receipt_authentication_restoration_and_exact_publish(self):
        receipt = self.qualify()
        self.assertEqual(self.helper('reuse-agentplugins-verified-draft.py').stdout.strip(), b'123')
        self.release['draft'] = False  # fixture-only promotion, no API writes
        output = self.root / 'npm-tarball'
        output.mkdir(mode=0o700)
        self.helper('reuse-agentplugins-npm-artifact.py', ['--output-dir', str(output)])
        self.assertEqual((output / self.tarball_file).read_bytes(), self.tarball)
        metadata = json.loads((output / 'npm-package.json').read_text())
        self.assertEqual(metadata['integrity'], receipt['npm_artifact']['integrity'])
        calls = [json.loads(line) for line in Path(self.env['GH_LOG']).read_text().splitlines()]
        self.assertEqual(sum(call[:2] == ['attestation', 'verify'] for call in calls), 18)
        (self.bin / 'npm').write_text('''#!/usr/bin/env python3
import hashlib, json, os, pathlib, sys
a = sys.argv[1:]
with open(os.environ['NPM_LOG'], 'a') as log: log.write(json.dumps(a) + '\\n')
if a == ['view', 'universal-agent-plugins@1.2.3', 'version']:
    sys.exit(0 if os.environ.get('VERSION_EXISTS') else 1)
assert a == ['publish', os.environ['EXPECTED_TARBALL'], '--registry=https://registry.npmjs.org/', '--access', 'public', '--provenance', '--tag', 'latest']
pathlib.Path(os.environ['PUBLISH_HASH']).write_text(hashlib.sha256(pathlib.Path(a[1]).read_bytes()).hexdigest())
''')
        (self.bin / 'npm').chmod(0o755)
        config = self.root / 'npmrc'
        config.write_text('registry=https://registry.npmjs.org/\n')
        env = {**self.env, 'RUNNER_TEMP': str(self.root), 'PACKAGE_NAME': 'universal-agent-plugins',
               'VERSION': '1.2.3', 'TARBALL': self.tarball_file, 'NPM_TOKEN': '', 'NODE_AUTH_TOKEN': '',
               'NPM_LOG': str(self.root / 'npm.log'), 'PUBLISH_HASH': str(self.root / 'published'),
               'EXPECTED_TARBALL': str(output / self.tarball_file)}
        step = shell_step('agentplugins-npm-publish.yml', 'Refuse overwrite and publish the exact staged bytes')
        self.run_command(['bash', '-e', '-o', 'pipefail', '-c', step], env=env)
        self.assertEqual((self.root / 'published').read_text(), hashlib.sha256(self.tarball).hexdigest())
        result = self.run_command(['bash', '-e', '-o', 'pipefail', '-c', step],
                                  env={**env, 'VERSION_EXISTS': '1'}, check=False)
        self.assertNotEqual(result.returncode, 0)
        npm_calls = [json.loads(line) for line in (self.root / 'npm.log').read_text().splitlines()]
        self.assertEqual([call[0] for call in npm_calls], ['view', 'publish', 'view'])
        self.assertEqual(self.git('status', '--porcelain').stdout, b'')

    def test_missing_failed_qualifications_and_changed_archives_rejected(self):
        self.qualify()
        self.release['draft'] = False
        output = self.root / 'rejected-output'
        output.mkdir(mode=0o700)
        original = copy.deepcopy(self.jobs)
        for name in ('qualify-go', 'qualify-npm'):
            for failure in ('missing', 'failure'):
                with self.subTest(name=name, failure=failure):
                    self.jobs[:] = copy.deepcopy(original)
                    job = next(job for job in self.jobs if job['name'] == name)
                    if failure == 'missing':
                        self.jobs.remove(job)
                    else:
                        job['conclusion'] = 'failure'
                    self.reject_reuse(output, 'missing or wrong qualification job: ' + name)
        self.jobs[:] = original
        for identity in (70, 71):
            path = self.root / (str(identity) + '.zip')
            original_bytes = path.read_bytes()
            for replacement in (original_bytes + b'corruption', b'replacement zip'):
                path.write_bytes(replacement)
                self.reject_reuse(output, 'ZIP digest mismatch')
            path.write_bytes(original_bytes)

    def reject_reuse(self, output, reason):
        result = self.helper('reuse-agentplugins-npm-artifact.py', ['--output-dir', str(output)], check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b'')
        self.assertIn(reason, result.stderr.decode())
        self.assertEqual(list(output.iterdir()), [])

    def test_workflow_source_guards_before_external_effects(self):
        step = shell_step('agentplugins-release.yml', 'Verify frozen stable tag and main ancestry')
        env = {**self.env, 'TAG': TAG, 'WORKFLOW_REF': 'refs/tags/' + TAG,
               'WORKFLOW_COMMIT': self.commit, 'GITHUB_OUTPUT': str(self.root / 'outputs')}
        self.git('commit', '--allow-empty', '-m', 'test: descendant main')
        descendant = self.git('rev-parse', 'HEAD').stdout.decode().strip()
        self.git('push', 'origin', 'main')
        self.git('checkout', '--detach', self.commit)
        self.run_command(['bash', '-e', '-o', 'pipefail', '-c', step], env=env)
        for changed in ({'WORKFLOW_REF': 'refs/heads/feature'}, {'WORKFLOW_COMMIT': 'f' * 40}):
            self.assertNotEqual(self.run_command(['bash', '-e', '-o', 'pipefail', '-c', step],
                                                env={**env, **changed}, check=False).returncode, 0)
        self.git('tag', '--force', TAG, descendant)
        self.git('push', '--force', 'origin', TAG)
        self.assertNotEqual(self.run_command(['bash', '-e', '-o', 'pipefail', '-c', step],
                                            env=env, check=False).returncode, 0)
        self.git('tag', '--force', TAG, self.commit)
        self.git('push', '--force', 'origin', TAG)
        self.git('checkout', '--orphan', 'unrelated')
        self.git('commit', '-m', 'test: unrelated main')
        self.git('push', '--force', 'origin', 'HEAD:main')
        self.git('checkout', '--detach', self.commit)
        self.assertNotEqual(self.run_command(['bash', '-e', '-o', 'pipefail', '-c', step],
                                            env=env, check=False).returncode, 0)
        # A mismatched workflow SHA must stop before fetch, gh or package effects.
        npm_step = shell_step('agentplugins-npm-publish.yml', 'Verify exact public release identity and attestations')
        (self.bin / 'git').write_text('#!/bin/sh\ncase "$*" in "rev-parse HEAD") echo "' + self.commit + '";; *) exit 99;; esac\n')
        (self.bin / 'git').chmod(0o755)
        result = self.run_command(['bash', '-e', '-o', 'pipefail', '-c', npm_step],
                                  env={**env, 'GITHUB_REF': 'refs/tags/' + TAG, 'GITHUB_SHA': 'f' * 40},
                                  check=False)
        self.assertEqual(result.returncode, 1)
        self.assertFalse(Path(self.env['GH_LOG']).exists())


if __name__ == '__main__':
    unittest.main()
