"""Draft acquisition for the native harness. All remote operations are read-only.

The artifact is selected by ID AND name and authenticated against an exact
producer attempt. No build/pack fallback exists. Runtime environments are
constructed from scratch; acquisition authentication never enters npm/launchers.
"""
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile
import zipfile

REPOSITORY = '777genius/universal-agent-plugins'
WORKFLOW = '.github/workflows/agentplugins-release.yml'
TARGETS = {'darwin-amd64', 'darwin-arm64', 'linux-amd64', 'linux-arm64', 'windows-amd64', 'windows-arm64'}
FIELDS = ('producer_run_id', 'producer_run_attempt', 'expected_asset_set_digest', 'producer_artifact_id', 'producer_artifact_digest', 'release_id')


def require(ok, message):
    if not ok:
        raise ValueError(message)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def output(command, **kwargs):
    return subprocess.check_output(command, timeout=300, **kwargs)


def api(endpoint, *args):
    return json.loads(output(['gh', 'api', endpoint, *args]))


def validate(args):
    require(args.release_state in ('public', 'draft'), 'invalid release state')
    supplied = [getattr(args, key, '') for key in FIELDS]
    require(bool(args.release_tag) == bool(args.release_commit), 'release tag and commit must be paired')
    if args.release_tag:
        require(re.fullmatch(r'agentplugins-v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)', args.release_tag), 'invalid stable tag')
        require(re.fullmatch(r'[0-9a-f]{40}', args.release_commit), 'invalid producer commit')
    if args.release_state == 'public':
        require(not any(supplied), 'public mode must not receive draft identities')
        return
    require(all(supplied), 'draft requires all frozen producer, release and artifact identities')
    require(args.release_repo == REPOSITORY, 'wrong producer repository')
    require(re.fullmatch(r'agentplugins-v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)', args.release_tag or ''), 'invalid stable tag')
    require(re.fullmatch(r'[0-9a-f]{40}', args.release_commit or ''), 'invalid producer commit')
    for key in ('expected_asset_set_digest', 'producer_artifact_digest'):
        require(re.fullmatch(r'[0-9a-f]{64}', getattr(args, key)), 'invalid digest')
    for key in ('producer_run_id', 'producer_run_attempt', 'producer_artifact_id', 'release_id'):
        require(re.fullmatch(r'[1-9]\d*', str(getattr(args, key))), 'invalid numeric identity')


def authenticate(args):
    validate(args)
    prefix = f'repos/{REPOSITORY}/actions'
    run_id, attempt = int(args.producer_run_id), int(args.producer_run_attempt)
    run = api(f'{prefix}/runs/{run_id}')
    require(run['id'] == run_id and run['run_attempt'] == attempt
            and run['repository']['full_name'] == REPOSITORY
            and run['head_repository']['full_name'] == REPOSITORY
            and run['head_sha'] == args.release_commit and run['path'] == WORKFLOW
            and run['event'] == 'workflow_dispatch' and run['status'] == 'completed'
            and run['conclusion'] == 'success', 'wrong or unsuccessful producer invocation')
    workflow = api(f'{prefix}/workflows/{run["workflow_id"]}')
    require(workflow['path'] == WORKFLOW and workflow['id'] == run['workflow_id'], 'wrong producer workflow')
    pages = api(f'{prefix}/runs/{run_id}/attempts/{attempt}/jobs?per_page=100', '--paginate', '--slurp')
    jobs = [job for page in pages for job in page['jobs']]
    for name in ['platform-proof / native runtime E2E (' + target + ')' for target in sorted(TARGETS)] + ['platform-proof / Aggregate all six native platform proofs', 'verified-draft']:
        matches = [job for job in jobs if job['name'] == name]
        require(len(matches) == 1 and matches[0]['conclusion'] == 'success'
                and matches[0]['status'] == 'completed' and matches[0]['run_id'] == run_id,
                'missing or unsuccessful producer proof: ' + name)
        steps = matches[0].get('steps', [])
        require(steps and all(s['conclusion'] == 'success' for s in steps), 'skipped or failed producer proof step')
    pages = api(f'{prefix}/runs/{run_id}/artifacts?per_page=100', '--paginate', '--slurp')
    name = 'agentplugins-npm-' + args.release_tag.removeprefix('agentplugins-v')
    matches = [a for p in pages for a in p['artifacts'] if a['name'] == name or a['id'] == int(args.producer_artifact_id)]
    require(len(matches) == 1, 'missing or duplicate npm artifact')
    artifact = matches[0]
    require(artifact['id'] == int(args.producer_artifact_id) and artifact['name'] == name
            and artifact['expired'] is False and artifact['size_in_bytes'] > 0
            and artifact['digest'] == 'sha256:' + args.producer_artifact_digest
            and artifact['workflow_run']['id'] == run_id
            and artifact['workflow_run']['head_sha'] == args.release_commit,
            'artifact identity mismatch or expired artifact')
    # Run artifact listing spans attempts. Bind its creation to this attempt too.
    require(run['run_started_at'] <= artifact['created_at'] <= run['updated_at'], 'artifact belongs to another attempt')
    return {'repository': REPOSITORY, 'workflow': WORKFLOW, 'commit': args.release_commit,
            'run_id': run_id, 'run_attempt': attempt, 'artifact_id': artifact['id'],
            'artifact_digest': args.producer_artifact_digest}


def unpack_bundle(body, root):
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        names = set()
        for member in archive.infolist():
            name = member.filename
            parts = PurePosixPath(name).parts
            require(name and '\\' not in name and not name.startswith('/')
                    and all(p not in ('..', '.') and ':' not in p for p in parts)
                    and name.rstrip('/') == '/'.join(parts), 'unsafe artifact path')
            folded = name.rstrip('/').casefold()
            require(folded not in names, 'duplicate or aliased artifact path')
            names.add(folded)
            mode = (member.external_attr >> 16) & 0o170000
            require(mode in (0, 0o100000, 0o040000) and member.file_size < 400 << 20, 'nonregular or oversized artifact member')
            path = root.joinpath(*parts)
            if member.is_dir():
                path.mkdir(parents=True, exist_ok=True)
            else:
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(archive.read(member))


def live_verify(source, producer_source, args, receipt):
    # Keep the scope-1 CLI contract unchanged. Repository disagreements fail closed.
    output([sys.executable, str(source / 'scripts/verify-agentplugins-draft.py'),
            '--repository', args.release_repo, '--tag', args.release_tag,
            '--commit', args.release_commit, '--release-id', str(args.release_id),
            '--asset-set-digest', args.expected_asset_set_digest,
            '--run-id', str(args.producer_run_id), '--run-attempt', str(args.producer_run_attempt),
            '--source', str(producer_source), '--receipt', str(receipt)])
    record = json.loads(receipt.read_bytes())
    require(record['release_state'] == 'verified-draft'
            and record['release']['id'] == int(args.release_id)
            and record['producer_commit'] == args.release_commit
            and record['asset_set_digest'] == args.expected_asset_set_digest,
            'draft receipt identity mismatch')
    return record


def acquire(source, directory, args, evidence):
    provenance = authenticate(args)
    producer_source = Path(args.producer_source).resolve()
    require(output(['git', '-C', str(producer_source), 'rev-parse', 'HEAD']).decode().strip() == args.release_commit, 'wrong producer checkout')
    require(not output(['git', '-C', str(producer_source), 'status', '--porcelain', '--untracked-files=normal']), 'dirty producer checkout')
    tree = output(['git', '-C', str(producer_source), 'rev-parse', 'HEAD^{tree}']).decode().strip()
    require(re.fullmatch(r'[0-9a-f]{40}', tree), 'invalid producer tree')
    live = live_verify(source, producer_source, args, evidence / 'initial-draft.json')
    body = output(['gh', 'api', f'repos/{REPOSITORY}/actions/artifacts/{args.producer_artifact_id}/zip'])
    require(hashlib.sha256(body).hexdigest() == args.producer_artifact_digest, 'artifact download digest mismatch')
    bundle = directory / 'draft-bundle'
    bundle.mkdir()
    unpack_bundle(body, bundle)
    tarballs = list(bundle.glob('*.tgz'))
    require(len(tarballs) == 1 and {p.name for p in bundle.iterdir()} == {tarballs[0].name, 'verified-release.json', 'release-assets'}, 'unexpected or missing bundle contents')
    assets = bundle / 'release-assets'
    verified = json.loads(output(['node', str(source / 'npm/agentplugins/scripts/release-assets.js'), 'verify', str(assets), args.release_tag, args.release_commit]))
    require(verified == json.loads((bundle / 'verified-release.json').read_bytes()) and verified['gate_eligible'] is True, 'bundle verification record mismatch')
    require(digest(assets / 'checksums.txt') == args.expected_asset_set_digest, 'frozen asset digest mismatch')
    subjects = {s['name']: s for s in live['subjects']}
    require({p.name for p in assets.iterdir()} == set(subjects), 'incomplete asset set')
    attestations = {}
    for path in assets.iterdir():
        require(path.is_file() and path.stat().st_size == subjects[path.name]['size'] and digest(path) == subjects[path.name]['sha256'], 'bundle differs from live attested asset')
        attestations[path.name] = json.loads(output(['gh', 'attestation', 'verify', str(path), '--repo', REPOSITORY,
            '--signer-workflow', f'github.com/{REPOSITORY}/{WORKFLOW}', '--source-digest', args.release_commit,
            '--deny-self-hosted-runners', '--format', 'json']))
        verify_attestation(attestations[path.name], path.name, digest(path), args.release_commit)
    selected = verified['assets'][args.target]
    release = {'acquisition': 'authenticated producer npm artifact', 'release_state': 'draft',
               'repository': REPOSITORY, 'tag': args.release_tag, 'version': verified['version'],
               'commit': args.release_commit, 'tree': tree, 'file': selected['file'],
               'binary_sha256': selected['sha256'], 'size': selected['size'],
               'manifest_sha256': verified['manifest_sha256'], 'checksums_sha256': args.expected_asset_set_digest,
               'release_id': int(args.release_id), 'producer': provenance, 'attestations': attestations,
               'tarball_sha256': digest(tarballs[0]), 'initial_draft': live}
    inspect_package(tarballs[0], verified)
    with tarfile.open(tarballs[0], 'r:gz') as archive:
        require(archive.extractfile('package/THIRD_PARTY_NOTICES.txt').read() ==
                (assets / 'THIRD_PARTY_NOTICES.txt').read_bytes(), 'packaged notices mismatch')
    return tarballs[0], assets, verified, release


def verify_attestation(records, name, sha, commit):
    require(isinstance(records, list) and records, 'empty attestation verification')
    for record in records:
        result = record['verificationResult']
        certificate = result['signature']['certificate']
        statement = result['statement']
        require(certificate['sourceRepositoryURI'] == 'https://github.com/' + REPOSITORY
                and certificate['sourceRepositoryDigest'] == commit
                and certificate['buildSignerDigest'] == commit
                and certificate['buildSignerURI'].startswith('https://github.com/' + REPOSITORY + '/' + WORKFLOW + '@refs/')
                and certificate['runnerEnvironment'] == 'github-hosted'
                and certificate['issuer'] == 'https://token.actions.githubusercontent.com'
                and statement['predicateType'] == 'https://slsa.dev/provenance/v1'
                and any(s.get('name') == name and s.get('digest', {}).get('sha256') == sha for s in statement['subject'])
                and result.get('verifiedTimestamps'), 'attestation source, signer or subject mismatch')


def inspect_package(tarball, verified):
    # Validate all paths before npm extracts anything; reject links and dependencies.
    with tarfile.open(tarball, 'r:gz') as archive:
        names = set()
        for member in archive.getmembers():
            parts = PurePosixPath(member.name).parts
            require(parts and parts[0] == 'package' and member.name.rstrip('/') == '/'.join(parts)
                    and all(p not in ('.', '..') and ':' not in p for p in parts)
                    and '\\' not in member.name and (member.isfile() or member.isdir()), 'unsafe npm member')
            require(member.name.rstrip('/').casefold() not in names, 'duplicate npm member')
            names.add(member.name.rstrip('/').casefold())
        pkg = json.load(archive.extractfile('package/package.json'))
        pins = json.load(archive.extractfile('package/assets.json'))
    require(pkg['name'] == 'universal-agent-plugins' and pkg['version'] == verified['version']
            and pkg.get('bin') == {'agentplugins': 'bin/agentplugins.js'}, 'wrong npm package')
    require(not any(pkg.get(k) for k in ('dependencies', 'optionalDependencies', 'peerDependencies', 'bundledDependencies', 'bundleDependencies'))
            and not any(pkg.get('scripts', {}).get(k) for k in ('preinstall', 'install', 'postinstall')), 'npm dependency or install scripts forbidden')
    require(pins['schema_version'] == 2 and pins['version'] == verified['version']
            and pins['npm_package'] == pkg['name'] and pins['assets'] == verified['assets']
            and pins['tag'] == verified['tag'] and pins['repository'] == verified['repository']
            and pins['producer'] == {'repository': verified['repository'], 'tag': verified['tag'],
                'commit': verified['commit'], 'release_manifest': {'schema_version': 2,
                    'sha256': verified['manifest_sha256'], 'version': verified['version']}}, 'package pins differ from frozen assets')


def bootstrap(tarball, assets, verified, target, scratch, env):
    project = scratch / 'npm-project'
    project.mkdir()
    (project / 'package.json').write_text('{"private":true}')
    node = shutil.which('node')
    require(node is not None, 'Node missing')
    node = str(Path(node).resolve())
    npm = Path(node).parent / 'node_modules/npm/bin/npm-cli.js'
    if not npm.is_file():
        npm_path = shutil.which('npm')
        require(npm_path is not None and os.name != 'nt', 'npm CLI missing next to Node')
        npm = Path(npm_path).resolve()
    runtime = dict(env)
    runtime['PATH'] = str(Path(node).parent) + os.pathsep + runtime['PATH']
    cache = scratch / 'npm-binary-cache'
    require(not cache.exists(), 'cache must be cold')
    runtime['AGENTPLUGINS_CACHE_DIR'] = str(cache)
    output([node, str(npm), 'install', '--ignore-scripts', '--no-audit', '--no-fund', '--offline', '--save-exact', str(tarball)], cwd=project, env=runtime)
    installed = project / 'node_modules/universal-agent-plugins'
    # npm must have installed exactly the frozen regular file bytes.
    with tarfile.open(tarball, 'r:gz') as archive:
        for member in archive.getmembers():
            if member.isfile():
                path = installed.joinpath(*PurePosixPath(member.name).parts[1:])
                require(path.is_file() and not path.is_symlink() and path.read_bytes() == archive.extractfile(member).read(), 'installed package differs from tarball')
    launcher = installed / 'bin/agentplugins.js'
    selected = verified['assets'][target]
    runtime['AGENTPLUGINS_INTERNAL_PROOF_MODE'] = 'local-frozen-release-asset-v1'
    runtime['AGENTPLUGINS_INTERNAL_PROOF_BINARY'] = str(assets / selected['file'])
    command = [node, str(launcher), 'version']
    measured = output(command, cwd=project, env=runtime).decode().strip()
    require(measured == 'agentplugins ' + verified['version'], 'launcher version mismatch')
    del runtime['AGENTPLUGINS_INTERNAL_PROOF_MODE']
    del runtime['AGENTPLUGINS_INTERNAL_PROOF_BINARY']
    binary = cache / verified['version'] / target / ('agentplugins.exe' if target.startswith('windows') else 'agentplugins')
    require(binary.is_file() and not binary.is_symlink() and binary.stat().st_size == selected['size'] and digest(binary) == selected['sha256'], 'cached binary mismatch')
    stamp = binary.stat().st_mtime_ns
    require(output(command, cwd=project, env=runtime).decode().strip() == measured
            and binary.stat().st_mtime_ns == stamp and digest(binary) == selected['sha256'], 'warm launcher changed cache')
    require(output([str(binary), 'version'], cwd=project, env=runtime).decode().strip() == measured, 'cached binary version mismatch')
    return binary, {'bootstrap_source': 'local_frozen_asset', 'tarball_sha256': digest(tarball),
                    'launcher_sha256': digest(launcher), 'binary_sha256': digest(binary),
                    'size': binary.stat().st_size, 'version': measured, 'cold_bootstrap': True,
                    'warm_without_proof_source': True, 'npm_ignore_scripts': True}


def load_script(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), Path(__file__).with_name(name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def helper_hashes(source):
    return {name: digest(source / name) for name in (
        'scripts/run-native-client-matrix.py', 'scripts/native_client_draft.py',
        'scripts/verify-agentplugins-draft.py', 'scripts/verify-released-native-evidence.py',
        'npm/agentplugins/scripts/release-assets.js')}


def aggregate(source, args):
    validate(args)
    require(args.release_state == 'draft', 'aggregate receipt is draft-only')
    root, destination = Path(args.evidence), Path(args.receipt)
    require(not destination.exists(), 'qualification receipt already exists')
    matrix = load_script('run-native-client-matrix')
    targets = ['linux-amd64'] if args.target_scope == 'linux-amd64' else ['linux-arm64', 'darwin-arm64', 'windows-amd64']
    expected = {(client, target) for target in targets for client in matrix.REQUIRED_TESTS}
    paths = list(root.rglob('runner-evidence.json'))
    require(len(paths) == len(expected), 'missing or extra lanes')
    seen, common, hashes, lanes = set(), None, {}, []
    for path in paths:
        record = json.loads(path.read_bytes())
        client, target = record['client'], record['target']
        require((client, target) in expected - seen, 'duplicate or unexpected lane')
        seen.add((client, target))
        require(record['status'] == 'passed' and type(record['exit_code']) is int and record['exit_code'] == 0
                and record['skipped_tests'] == [], 'failed or skipped lane')
        require(record['commit'] == args.harness_commit and re.fullmatch(r'[0-9a-f]{40}', record['tree']), 'mixed harness checkout')
        require(record['helper_sha256'] == helper_hashes(source), 'mixed harness helper bytes')
        release = record['installer_release']
        producer = release['producer']
        require(release['release_state'] == 'draft' and release['repository'] == REPOSITORY
                and release['tag'] == args.release_tag and release['commit'] == args.release_commit
                and release['release_id'] == int(args.release_id)
                and release['checksums_sha256'] == args.expected_asset_set_digest
                and producer == {'repository': REPOSITORY, 'workflow': WORKFLOW, 'commit': args.release_commit,
                    'run_id': int(args.producer_run_id), 'run_attempt': int(args.producer_run_attempt),
                    'artifact_id': int(args.producer_artifact_id), 'artifact_digest': args.producer_artifact_digest}, 'mixed producer identities')
        identity = (record['tree'], release['tree'], release['tarball_sha256'], release['initial_draft'])
        # Verification timestamps differ by lane; compare replacement-sensitive release/subjects.
        identity = identity[:3] + (identity[3]['release'], identity[3]['subjects'])
        require(common is None or identity == common, 'mixed bundle, source tree or live draft')
        common = identity
        subjects = {s['name']: s for s in release['initial_draft']['subjects']}
        require(release['file'] in subjects
                and subjects[release['file']]['sha256'] == release['binary_sha256']
                and subjects[release['file']]['size'] == release['size'], 'installer differs from live draft subject')
        packaged = record['packaged_acquisition']
        require(packaged['bootstrap_source'] == 'local_frozen_asset' and packaged['cold_bootstrap'] is True
                and packaged['warm_without_proof_source'] is True and packaged['npm_ignore_scripts'] is True
                and packaged['tarball_sha256'] == release['tarball_sha256']
                and packaged['binary_sha256'] == record['installer_sha256'] == release['binary_sha256']
                and packaged['size'] == release['size'] and packaged['version'] == 'agentplugins ' + release['version'], 'missing packaged bootstrap proof')
        require(target not in hashes or hashes[target] == record['installer_sha256'], 'differing installer binary hashes')
        hashes[target] = record['installer_sha256']
        bodies = {}
        for name, sha in record['artifact_sha256'].items():
            parts = PurePosixPath(name).parts
            require(name == '/'.join(parts) and parts and not name.startswith('/')
                    and all(p not in ('.', '..') and ':' not in p for p in parts) and '\\' not in name, 'unsafe evidence path')
            item = path.parent.joinpath(*parts)
            require(item.is_file() and not item.is_symlink() and digest(item) == sha, 'evidence digest mismatch')
            bodies[name] = item.read_bytes()
        log = bodies['native-tests.log'].decode('utf-8', errors='strict')
        passed = set(re.findall(r'^--- PASS: (\w+)', log, re.M))
        require(passed == set(record['passed_tests']) and matrix.REQUIRED_TESTS[client].issubset(passed)
                and not re.search(r'^\s*--- (SKIP|FAIL):', log, re.M), 'missing tests or skipped/failed transcript')
        verifier = load_script('verify-released-native-evidence')
        verifier.verify_tools(record, target, client)
        verifier.verify_fixtures(bodies, record, client, target)
        lanes.append({'client': client, 'target': target, 'evidence_sha256': digest(path)})
    require(seen == expected, 'missing lanes')
    # Reauthenticate after all runtime lanes, then call the unchanged read-only helper.
    authenticate(args)
    producer_source = Path(args.producer_source).resolve()
    require(output(['git', '-C', str(producer_source), 'rev-parse', 'HEAD']).decode().strip() == args.release_commit, 'wrong final producer checkout')
    require(not output(['git', '-C', str(producer_source), 'status', '--porcelain', '--untracked-files=normal']), 'dirty final producer checkout')
    final = live_verify(source, producer_source, args, destination.with_name('final-draft.json'))
    require(final['release'] == common[3] and final['subjects'] == common[4], 'draft changed during native qualification')
    result = {'schema_version': 1, 'status': 'passed', 'release_state': 'verified-draft',
              'target_scope': args.target_scope, 'producer_commit': args.release_commit,
              'harness_commit': args.harness_commit, 'harness_tree': common[0],
              'tarball_sha256': common[2], 'producer': producer, 'helper_sha256': helper_hashes(source),
              'lanes': lanes, 'final_draft': final,
              'limitation': 'genuine pinned clients; scripted loopback providers; no real-model or OAuth qualification'}
    with destination.open('x', encoding='utf-8') as stream:
        stream.write(json.dumps(result, indent=2) + '\n')


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--release-state', choices=('public', 'draft'), default='public')
    parser.add_argument('--release-repo', default=REPOSITORY)
    for field in ('release_tag', 'release_commit', *FIELDS):
        parser.add_argument('--' + field.replace('_', '-'), default='')
    parser.add_argument('--validate-only', action='store_true')
    parser.add_argument('--producer-source', type=Path)
    parser.add_argument('--harness-commit')
    parser.add_argument('--target-scope', choices=('historical-nine', 'linux-amd64'), default='historical-nine')
    parser.add_argument('--evidence', type=Path)
    parser.add_argument('--receipt', type=Path)
    args = parser.parse_args()
    validate(args)
    if not args.validate_only:
        aggregate(Path(__file__).resolve().parents[1], args)


if __name__ == '__main__':
    main()
