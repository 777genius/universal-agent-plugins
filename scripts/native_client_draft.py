"""Draft acquisition for the native harness. All remote operations are read-only.

The artifact is selected by ID AND name and authenticated against an exact
producer attempt. No build/pack fallback exists. Runtime environments are
constructed from scratch; acquisition authentication never enters npm/launchers.
"""
import gzip
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
import tempfile
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
            '--source', str(producer_source), '--policy-source', str(source), '--receipt', str(receipt)])
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
    snapshot = receive(source, args, 'snapshot')
    require(snapshot['producer_tree'] == tree, 'mixed producer tree')
    live = snapshot['live']
    (evidence / 'initial-draft.json').write_text(json.dumps(live))
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
    package = inspect_package(tarballs[0], verified)
    require(package['package/THIRD_PARTY_NOTICES.txt'] ==
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


TAR_EXPANDED_LIMIT = 64 << 20
TAR_MEMBER_LIMIT = 16 << 20
TAR_METADATA_LIMIT = 64 << 10
TAR_MEMBER_COUNT = 128


def read_package(source):
    """Read npm files after bounded gzip and full physical/logical preflight.

    Accept a path or compressed bytes. Native binaries remain outside this
    source package, with their separate 400 MiB ZIP member ceiling.
    """
    selected = {}
    with tempfile.TemporaryFile() as expanded:
        # Include padding, extension records and concatenated gzip streams in the
        # budget. Never ask the decompressor to allocate an archive-sized buffer.
        total = 0
        compressed = (gzip.GzipFile(fileobj=io.BytesIO(source)) if isinstance(source, bytes)
                      else gzip.open(source, 'rb'))
        with compressed:
            while True:
                chunk = compressed.read(min(64 << 10, TAR_EXPANDED_LIMIT - total + 1))
                if not chunk:
                    break
                total += len(chunk)
                require(total <= TAR_EXPANDED_LIMIT, 'oversized expanded npm archive')
                expanded.write(chunk)
        expanded.seek(0)
        count = 0
        extensions = (tarfile.XHDTYPE, tarfile.XGLTYPE, tarfile.GNUTYPE_LONGNAME, tarfile.GNUTYPE_LONGLINK)
        while True:
            header = expanded.read(512)
            if not header or header == b'\0' * 512:
                break
            count += 1
            require(count <= TAR_MEMBER_COUNT, 'too many npm headers')
            # frombuf parses only this header, without consuming pax/longname or
            # sparse extensions. tarfile.open would process them before yielding.
            member = tarfile.TarInfo.frombuf(header, 'utf-8', 'surrogateescape')
            require(member.type in (tarfile.REGTYPE, tarfile.AREGTYPE, tarfile.DIRTYPE) + extensions,
                    'unsupported npm header (links/sparse forbidden)')
            limit = TAR_METADATA_LIMIT if member.type in extensions else TAR_MEMBER_LIMIT
            require(0 <= member.size <= limit, 'oversized npm header body')
            require(not member.isdir() or member.size == 0, 'nonempty npm directory')
            end = expanded.tell() + ((member.size + 511) // 512) * 512
            require(end <= total, 'truncated npm header body')
            if member.type in (tarfile.XHDTYPE, tarfile.XGLTYPE):
                metadata = expanded.read(member.size)
                require(b'GNU.sparse' not in metadata, 'sparse npm metadata forbidden')
                # A pax size override would change the physical record offsets
                # on the later tarfile pass. npm sources need no such encoding.
                require(b' size=' not in metadata, 'npm pax size override forbidden')
            expanded.seek(end)
        expanded.seek(0)
        with tarfile.open(fileobj=expanded, mode='r:') as package:
            members, size = {}, 0
            for member in package:
                require(len(members) < TAR_MEMBER_COUNT, 'too many npm members')
                require(member.isfile() or member.isdir(), 'nonregular npm member')
                require(not member.issparse() and 0 <= member.size <= TAR_MEMBER_LIMIT,
                        'oversized or sparse npm member')
                size += member.size
                require(size <= TAR_EXPANDED_LIMIT, 'oversized npm member total')
                name = member.name.rstrip('/') if member.isdir() else member.name
                parts = PurePosixPath(name).parts
                require(parts and name == '/'.join(parts)
                        and not any(ord(c) < 32 or ord(c) == 127 or c in '\\:' for c in name)
                        and all(p not in ('.', '..') for p in parts), 'unsafe npm member')
                require(name == 'package' or name.startswith('package/'), 'unsafe npm member')
                require(name.casefold() not in members, 'duplicate npm member')
                members[name.casefold()] = member
            # Complete both inventories before reading any file body or JSON.
            # Expanded bytes also bound the aggregate retained file contents.
            for member in members.values():
                if member.isfile():
                    selected[member.name] = package.extractfile(member).read(member.size + 1)
                    require(len(selected[member.name]) == member.size, 'truncated npm file')
    return selected


def inspect_package(tarball, verified):
    package = read_package(tarball)
    pkg = json.loads(package['package/package.json'])
    pins = json.loads(package['package/assets.json'])
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

    return package


def bootstrap(tarball, assets, verified, target, scratch, env):
    package = inspect_package(tarball, verified)
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
    for name, body in package.items():
        path = installed.joinpath(*PurePosixPath(name).parts[1:])
        require(path.is_file() and not path.is_symlink() and path.stat().st_size == len(body)
                and path.read_bytes() == body, 'installed package differs from tarball')
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


NATIVE_WORKFLOW = '.github/workflows/agentplugins-released-native-clients.yml'
RECEIPT_LIMIT = 64 << 10


def strict_json(body):
    require(len(body) <= RECEIPT_LIMIT, 'oversized receipt')
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, 'duplicate receipt key')
            result[key] = value
        return result
    return json.loads(body, object_pairs_hook=pairs, parse_constant=lambda _: require(False, 'invalid JSON number'))


def keys(record, names):
    require(type(record) is dict and set(record) == set(names.split()), 'unexpected receipt fields')


def trusted_binding(source, args):
    env = os.environ
    require(env.get('GITHUB_REPOSITORY') == REPOSITORY and env.get('GITHUB_EVENT_NAME') == 'workflow_dispatch'
            and env.get('GITHUB_REF') == 'refs/heads/main'
            and env.get('GITHUB_WORKFLOW_REF') == f'{REPOSITORY}/{NATIVE_WORKFLOW}@refs/heads/main',
            'draft requires canonical main workflow dispatch')
    sha = env.get('GITHUB_SHA', '')
    require(re.fullmatch(r'[0-9a-f]{40}', sha) and
            output(['git', '-C', str(source), 'rev-parse', 'HEAD']).decode().strip() == sha,
            'wrong immutable harness checkout')
    require(not output(['git', '-C', str(source), 'status', '--porcelain', '--untracked-files=normal']),
            'dirty harness checkout')
    tree = output(['git', '-C', str(source), 'rev-parse', 'HEAD^{tree}']).decode().strip()
    require(re.fullmatch(r'[0-9a-f]{40}', tree), 'invalid harness tree')
    for key in ('GITHUB_RUN_ID', 'GITHUB_RUN_ATTEMPT'):
        require(re.fullmatch(r'[1-9]\d*', env.get(key, '')), 'invalid native invocation')
    return dict(repository=REPOSITORY, workflow=NATIVE_WORKFLOW, harness_commit=sha, harness_tree=tree,
                run_id=int(env['GITHUB_RUN_ID']), run_attempt=int(env['GITHUB_RUN_ATTEMPT']),
                target_scope=args.target_scope, release_tag=args.release_tag, release_commit=args.release_commit,
                **{key: str(getattr(args, key)) for key in FIELDS}, helper_sha256=helper_hashes(source))


def emit(path, record):
    body = (json.dumps(record, sort_keys=True) + '\n').encode()
    require(len(body) <= RECEIPT_LIMIT, 'oversized receipt')
    with Path(path).open('xb') as stream:
        stream.write(body)
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'], 'a') as stream:
            stream.write('receipt-sha256=' + hashlib.sha256(body).hexdigest() + '\n')


def receive(source, args, kind):
    # Only trusted job outputs select transport. No dispatch URLs or names.
    prefix = 'SNAPSHOT' if kind == 'snapshot' else 'LANES'
    artifact_id, sha, archive_sha = [os.environ.get(prefix + suffix, '') for suffix in
                                   ('_ARTIFACT_ID', '_SHA256', '_ARTIFACT_DIGEST')]
    require(re.fullmatch(r'[1-9]\d*', artifact_id) and re.fullmatch(r'[0-9a-f]{64}', sha)
            and re.fullmatch(r'[0-9a-f]{64}', archive_sha), 'invalid receipt transport identity')
    binding = trusted_binding(source, args)
    artifact = api(f'repos/{REPOSITORY}/actions/artifacts/{artifact_id}')
    require(artifact['id'] == int(artifact_id) and artifact['expired'] is False
            and artifact['name'] == 'draft-' + kind
            and artifact['digest'] == 'sha256:' + archive_sha
            and 0 < artifact['size_in_bytes'] <= RECEIPT_LIMIT * 2
            and artifact['workflow_run']['id'] == binding['run_id']
            and artifact['workflow_run']['head_sha'] == binding['harness_commit'], 'wrong receipt artifact')
    body = output(['gh', 'api', f'repos/{REPOSITORY}/actions/artifacts/{artifact_id}/zip'])
    require(len(body) <= RECEIPT_LIMIT * 2 and hashlib.sha256(body).hexdigest() == archive_sha,
            'receipt archive digest mismatch')
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        members = archive.infolist()
        require(len(members) == 1 and members[0].filename == 'receipt.json'
                and members[0].file_size <= RECEIPT_LIMIT
                and (members[0].external_attr >> 16) & 0o170000 in (0, 0o100000), 'unexpected receipt archive')
        raw = archive.read(members[0])
    require(hashlib.sha256(raw).hexdigest() == sha, 'receipt digest mismatch')
    record = strict_json(raw)
    keys(record, 'kind binding producer_tree live' if kind == 'snapshot' else
         'kind binding snapshot_sha256 tarball_sha256 lanes')
    require(record['kind'] == ('snapshot' if kind == 'snapshot' else 'lanes-verified')
            and json.dumps(record['binding'], sort_keys=True) == json.dumps(binding, sort_keys=True), 'receipt invocation mismatch')
    if kind == 'snapshot':
        require(re.fullmatch(r'[0-9a-f]{40}', record['producer_tree']), 'invalid producer tree')
        live = record['live']
        keys(live, 'schema_version release_state verified_at repository release asset_set_digest subjects signer_workflow producer_commit producer_run_id producer_run_attempt')
        keys(live['release'], 'id tag commit draft prerelease updated_at assets')
        require(len(live['subjects']) == len(live['release']['assets']) == 9, 'incomplete snapshot subjects')
        for item in live['subjects']:
            keys(item, 'name id size sha256')
        for item in live['release']['assets']:
            keys(item, 'id name size state created_at updated_at digest')
        require(live['producer_commit'] == args.release_commit and live['producer_run_id'] == int(args.producer_run_id)
                and live['producer_run_attempt'] == int(args.producer_run_attempt)
                and live['asset_set_digest'] == args.expected_asset_set_digest
                and live['release']['id'] == int(args.release_id), 'snapshot producer mismatch')
    else:
        require(record['snapshot_sha256'] == os.environ['SNAPSHOT_SHA256']
                and re.fullmatch(r'[0-9a-f]{64}', record['tarball_sha256']), 'mixed snapshot or package')
        targets = ['linux-amd64'] if args.target_scope == 'linux-amd64' else ['linux-arm64', 'darwin-arm64', 'windows-amd64']
        expected = {(c, t) for c in ('codex', 'claude', 'opencode') for t in targets}
        require(type(record['lanes']) is list and len(record['lanes']) == len(expected), 'missing lanes')
        for lane in record['lanes']:
            keys(lane, 'client target evidence_sha256')
            pair = (lane['client'], lane['target'])
            require(pair in expected and re.fullmatch(r'[0-9a-f]{64}', lane['evidence_sha256']), 'invalid lane')
            expected.remove(pair)
    return record


def metadata(source, args):
    binding = trusted_binding(source, args)
    producer = authenticate(args)
    producer_source = Path(args.producer_source).resolve()
    tree = output(['git', '-C', str(producer_source), 'rev-parse', 'HEAD^{tree}']).decode().strip()
    require(re.fullmatch(r'[0-9a-f]{40}', tree), 'invalid producer tree')
    destination = Path(args.receipt)
    require(not destination.exists(), 'receipt already exists')
    if args.metadata == 'recheck':
        initial = receive(source, args, 'snapshot')
        lanes = receive(source, args, 'lanes')
        require(tree == initial['producer_tree'], 'mixed producer tree')
    # Temporary live receipt cannot leave a final success artifact on comparison failure.
    with tempfile.TemporaryDirectory() as directory:
        final = live_verify(source, producer_source, args, Path(directory) / 'live.json')
    if args.metadata == 'snapshot':
        emit(destination, dict(kind='snapshot', binding=binding, producer_tree=tree, live=final))
        return
    require(final['release'] == initial['live']['release'] and final['subjects'] == initial['live']['subjects'],
            'draft changed during native qualification')
    from datetime import datetime
    require(datetime.fromisoformat(final['verified_at']) > datetime.fromisoformat(initial['live']['verified_at']),
            'final verification is not fresh')
    result = dict(schema_version=2, status='passed', release_state='verified-draft', target_scope=args.target_scope,
                  producer_commit=args.release_commit, harness_commit=binding['harness_commit'],
                  harness_tree=binding['harness_tree'], tarball_sha256=lanes['tarball_sha256'], producer=producer,
                  helper_sha256=binding['helper_sha256'], lanes=lanes['lanes'], final_draft=final,
                  invocation=dict(binding=binding, snapshot_sha256=os.environ['SNAPSHOT_SHA256'],
                                  lanes_sha256=os.environ['LANES_SHA256'],
                                  transport={k: os.environ[k] for k in ('SNAPSHOT_ARTIFACT_ID', 'SNAPSHOT_ARTIFACT_DIGEST',
                                             'LANES_ARTIFACT_ID', 'LANES_ARTIFACT_DIGEST')}),
                  limitation='genuine pinned clients; scripted loopback providers; no real-model or OAuth qualification')
    emit(destination.with_name('final-draft.json'), final)
    emit(destination, result)


def aggregate(source, args):
    validate(args)
    require(args.release_state == 'draft', 'aggregate receipt is draft-only')
    root, destination = Path(args.evidence), Path(args.receipt)
    require(not destination.exists(), 'qualification receipt already exists')
    snapshot = receive(source, args, 'snapshot')
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
        require(release['initial_draft'] == snapshot['live'], 'lane snapshot mismatch')
        for subject in snapshot['live']['subjects']:
            verify_attestation(release['attestations'][subject['name']], subject['name'], subject['sha256'], args.release_commit)
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
    require(common == (snapshot['binding']['harness_tree'], snapshot['producer_tree'], common[2],
                       snapshot['live']['release'], snapshot['live']['subjects']), 'lanes differ from snapshot')
    emit(destination, {'kind': 'lanes-verified', 'binding': snapshot['binding'],
                       'snapshot_sha256': os.environ['SNAPSHOT_SHA256'],
                       'tarball_sha256': common[2], 'lanes': lanes})


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--release-state', choices=('public', 'draft'), default='public')
    parser.add_argument('--release-repo', default=REPOSITORY)
    for field in ('release_tag', 'release_commit', *FIELDS):
        parser.add_argument('--' + field.replace('_', '-'), default='')
    parser.add_argument('--validate-only', action='store_true')
    parser.add_argument('--metadata', choices=('snapshot', 'recheck'))
    parser.add_argument('--producer-source', type=Path)
    parser.add_argument('--harness-commit')
    parser.add_argument('--target-scope', choices=('historical-nine', 'linux-amd64'), default='historical-nine')
    parser.add_argument('--evidence', type=Path)
    parser.add_argument('--receipt', type=Path)
    args = parser.parse_args()
    validate(args)
    if args.validate_only and args.release_state == 'draft':
        trusted_binding(Path(__file__).resolve().parents[1], args)
        authenticate(args)
    elif args.metadata:
        metadata(Path(__file__).resolve().parents[1], args)
    elif not args.validate_only:
        aggregate(Path(__file__).resolve().parents[1], args)


if __name__ == '__main__':
    main()
