#!/usr/bin/env python3
"""Verify public evidence or explicit offline draft archives; optionally freeze a ZIP.

Input is a fresh directory populated by gh run download. No archives are
extracted, clients executed, or release assets modified. Recorded attestation
results are retained, not cryptographically reverified by this offline tool.
Archive paths/file types are restricted; original log content is not redacted.
Review disclosure separately before publishing under a non-agentplugins tag.
"""
import argparse
import hashlib
from functools import lru_cache
import json
from pathlib import Path
import re
import stat
import zipfile

CLIENT_TESTS = {
    'codex': {'TestAgentpluginsCodexNativeLifecycle'},
    'claude': {'TestAgentpluginsClaudeNativeLifecycle', 'TestAgentpluginsClaudeNativeRuntimeLifecycle'},
    'opencode': {'TestAgentpluginsOpenCodeNativeLifecycle', 'TestAgentpluginsOpenCodeNativeToolCollision', 'TestAgentpluginsOpenCodeNativeRuntimeExtended'},
}
TARGETS = ('darwin-arm64', 'windows-amd64', 'linux-arm64')
SCOPES = {'historical-nine': TARGETS, 'linux-amd64': ('linux-amd64',)}
REPOSITORY = '777genius/universal-agent-plugins'


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(body):
    return hashlib.sha256(body).hexdigest()


def exact_hex(value, length):
    return isinstance(value, str) and re.fullmatch('[0-9a-f]{' + str(length) + '}', value) is not None


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, 'duplicate JSON key: ' + key)
        result[key] = value
    return result


def safe_name(name):
    require(isinstance(name, str) and name and not any(ord(c) < 32 or ord(c) == 127 or c in '\\:' for c in name) and all(part not in ('', '.', '..') for part in name.split('/')), 'unsafe artifact path: ' + str(name))


def collect(root):
    require(root.is_dir() and not root.is_symlink(), 'input must be a real directory')
    files = {}
    for entry in sorted(root.iterdir()):
        safe_name(entry.name)
        mode = entry.lstat()
        require(not stat.S_ISLNK(mode.st_mode), 'symlink: ' + str(entry))
        if stat.S_ISDIR(mode.st_mode):
            for name, body in collect(entry).items():
                files[entry.name + '/' + name] = body
        else:
            require(stat.S_ISREG(mode.st_mode) and mode.st_nlink == 1, 'nonregular or aliased file: ' + str(entry))
            require(entry.suffix in ('.json', '.jsonl', '.log'), 'unexpected artifact file: ' + str(entry))
            files[entry.name] = entry.read_bytes()
    return files


# Stage contracts are the assertions emitted by the six native test shapes.
STAGES = {
 'codex': 'install local_security_scan scanner_prerequisite native_inventory A_native_tool A_skill_discovery A_cwd_default A_cwd_explicit B_native_tool B_skill_discovery B_cwd_default B_cwd_explicit B_data_identity unchanged owned_artifact_guard owned_repair native_cache_repair partial_failure all_unsupported collision_drift foreign_preservation remove repeat_remove skill_context http http_request_contract generated_protocol_header_priority immutable_git'.split(),
 'claude-lifecycle': 'install skill_discovery stdio_discovery mcp_transport_discovery version_update same_version_refresh unchanged owned_repair foreign_content_inside_managed_directory_blocked remove repeat_remove'.split(),
 'claude-runtime': 'install runtime_A runtime_B same_version_refresh repair remove'.split(),
 'opencode-lifecycle': 'install version_update same_version_refresh unchanged owned_repair foreign_key_collision foreign_config_preservation sse_unsupported remove repeat_remove'.split(),
 'opencode-extended': 'install refresh update repaired removed'.split(),
 'opencode-collision': ['install'],
}
for phase in ('same_version', 'owned_repair', 'native_cache_repair', 'partial_failure'):
    STAGES['codex'] += [phase + '_' + detail for detail in ('native_tool', 'skill_discovery', 'cwd_default', 'cwd_explicit', 'http')]


@lru_cache(maxsize=1)
def pins():
    # Read literal pins without importing/executing the runtime wrapper.
    import ast
    tree = ast.parse(Path(__file__).with_name('run-native-client-matrix.py').read_text())
    return next(ast.literal_eval(n.value) for n in tree.body if isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'PINS' for t in n.targets))


def verify_tools(record, target, client):
    for field, key in (('client_asset', client), ('scanner_asset', 'lintai'), ('ripgrep_asset', 'rg')):
        kind, repo, version, archive_name, integrity, _ = pins()[target][key]
        require(isinstance(integrity, str) and re.fullmatch(r'(sha256:[0-9a-f]{64}|sha512-[A-Za-z0-9+/]{86}==)', integrity), 'unfilled or invalid archive pin')
        source = f'https://github.com/{repo}/releases/tag/{version}' if kind == 'github' else f'https://registry.npmjs.org/{repo}/-/{archive_name}'
        asset = record.get(field, {})
        require(all(asset.get(k) == v for k, v in {'source': source, 'version': version, 'archive': archive_name, 'archive_integrity': integrity}.items()) and exact_hex(asset.get('binary_sha256'), 64), 'wrong pinned tool: ' + field)


def verify_attestations(release):
    expected = {release['file']: release['binary_sha256'], 'checksums.txt': release['checksums_sha256'], 'release-manifest.json': release['manifest_sha256']}
    records = release.get('attestations', {})
    require(set(records) == set(expected), 'missing recorded attestations')
    for name, sha in expected.items():
        require(isinstance(records[name], list) and records[name], 'missing recorded attestations')
        for item in records[name]:
            result = item.get('verificationResult', {})
            cert = result.get('signature', {}).get('certificate', {})
            statement = result.get('statement', {})
            require(cert.get('sourceRepositoryURI') == 'https://github.com/' + REPOSITORY and cert.get('sourceRepositoryDigest') == release['commit'] and cert.get('runnerEnvironment') == 'github-hosted' and cert.get('issuer') == 'https://token.actions.githubusercontent.com', 'recorded provenance source mismatch')
            require(cert.get('buildSignerURI', '').startswith('https://github.com/' + REPOSITORY + '/.github/workflows/agentplugins-release.yml@refs/') and cert.get('buildSignerDigest') == release['commit'], 'recorded provenance signer mismatch')
            require(statement.get('predicateType') == 'https://slsa.dev/provenance/v1' and any(s.get('name') == name and s.get('digest', {}).get('sha256') == sha for s in statement.get('subject', [])), 'recorded provenance subject mismatch')
            require(isinstance(result.get('verifiedTimestamps'), list) and result['verifiedTimestamps'], 'missing recorded provenance timestamps')


def verify_fixtures(bodies, record, client, target):
    found = []
    for path, body in bodies.items():
        base = Path(path).name
        if base not in ('evidence.json', 'tool-collision-evidence.json', 'extended-runtime-evidence.json'):
            continue
        data = json.loads(body, object_pairs_hook=unique_object)
        if client == 'codex': kind, key = 'codex', 'codex'
        elif client == 'claude':
            kind = 'claude-runtime' if 'runtime_scope' in data else 'claude-lifecycle'
            key = kind
        elif base == 'extended-runtime-evidence.json': kind, key = 'opencode-extended', 'extended'
        elif base == 'tool-collision-evidence.json':
            kind, key = 'opencode-collision', tuple(data.get('logical_servers', []))
            require(data.get('status') == 'passed' and data.get('model') == 'scripted_loopback_no_real_model' and data.get('oauth') == 'not_evaluated', 'collision boundary/status mismatch')
        else: kind, key = 'opencode-lifecycle', data.get('config_route_exercised')
        require(key not in found, 'duplicate fixture identity')
        found.append(key)
        stages = data.get('stages', {})
        required = list(STAGES[kind])
        windows_claude = client == 'claude' and target == 'windows-amd64'
        if kind == 'claude-lifecycle' and windows_claude:
            required.remove('stdio_discovery')
            require(stages.get('stdio_discovery') == {'status': 'observed_unsupported', 'reason': 'managed_stdio_platform_unsupported'}, 'Windows Claude stdio boundary mismatch')
        if kind == 'opencode-collision' and key == ('api/server',):
            required += 'version-update same-version-refresh repair remove foreign_config_preservation'.split()
        require(all((stages.get(k, {}).get('status') if isinstance(stages.get(k), dict) else stages.get(k)) == 'passed' for k in required), 'missing/failed fixture stage: ' + path)
        for stage_name, stage in stages.items():
            status = stage.get('status') if isinstance(stage, dict) else stage
            require(status in ('passed', 'not_evaluated', 'not_proven', 'not_applicable') or (windows_claude and stage_name == 'stdio_discovery' and status == 'observed_unsupported'), 'failed fixture stage: ' + path)
            for ref in stage.get('artifacts', []) if isinstance(stage, dict) else []:
                safe_name(ref)
                require(str(Path(path).parent / ref) in bodies, 'missing stage artifact')
        release = record['installer_release']
        identity = data.get('source_identity', data)
        require(identity.get('installer_base_commit') == release['commit'] and identity.get('installer_tree') == release['tree'] and identity.get('installer_patch_sha256') in ([], ''), 'fixture source mismatch')
        require(data.get('installer_sha256', data.get('installer_binary_sha256')) == record['installer_sha256'] and data.get('client_sha256', data.get('client_binary_sha256')) == record['client_asset']['binary_sha256'], 'fixture binary mismatch')
        versions = {'codex': '0.153.4', 'claude': '2.1.263 (Claude Code)', 'opencode': '1.18.29'}
        version_field = 'client_version_measured' if kind in ('claude-lifecycle', 'opencode-lifecycle') else 'client_version'
        if kind != 'opencode-extended': require(data.get(version_field) == versions[client], 'missing fixture client version')
        if 'installer_source_state' in identity: require(identity['installer_source_state'] == 'committed', 'uncommitted fixture source')
        for field in ('client_version', 'client_version_measured', 'client_version_pinned'):
            if field in data: require(data[field] == versions[client], 'fixture client version mismatch')
        if kind != 'claude-lifecycle':
            scanner = data.get('scanner', data.get('security_scanner', {}))
            require(scanner.get('binary_sha256') == record['scanner_asset']['binary_sha256'] and scanner.get('archive_sha256') == record['scanner_asset']['archive_integrity'].removeprefix('sha256:') and scanner.get('platform') == target and scanner.get('source_tag') == record['scanner_asset']['version'], 'fixture scanner mismatch')
        if kind == 'opencode-extended':
            suffix = '.exe' if target.startswith('windows') else ''
            require(data.get('probe_sha256') == record['harness_build_sha256']['native-probe' + suffix] and data.get('status') == 'passed' and data.get('provider') == 'scripted_loopback_no_real_model', 'extended fixture identity mismatch')
        if kind == 'claude-runtime':
            require(data.get('provider') == 'scripted_loopback' and data.get('real_model') == data.get('oauth') == 'not_evaluated' and data.get('installer_data_retention') == 'passed', 'Claude runtime boundary mismatch')
            expected_stdio = ('observed_unsupported', 'not_evaluated', 'HTTP+installed-skill') if windows_claude else ('passed', 'passed', 'stdio_default+stdio_explicit+HTTP+skill')
            require((data.get('stdio_runtime'), data.get('stdio_cwd_argv_env_data'), data.get('runtime_scope')) == expected_stdio, 'Claude stdio scope mismatch')
        hashes = data.get('transcript_sha256', data.get('artifact_sha256', {}))
        require(isinstance(hashes, dict) and hashes, 'missing fixture transcripts')
        for ref, sha in hashes.items():
            safe_name(ref)
            full = str(Path(path).parent / ref)
            require(full in bodies and exact_hex(sha, 64) and digest(bodies[full]) == sha, 'fixture transcript mismatch')
    expected = {'codex': {'codex'}, 'claude': {'claude-lifecycle', 'claude-runtime'}, 'opencode': {'opencode.json', 'opencode.jsonc', 'extended', ('api/server',), ('api server',), ('api/server', 'api server')}}
    require(set(found) == expected[client], 'missing or unexpected structured fixtures')

def verify(root, version, release_commit, harness_commit, harness_tree, *, scope='historical-nine', _draft=None):
    require(scope in SCOPES, 'unknown evidence scope')
    targets = SCOPES[scope]
    require(re.fullmatch(r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)', version), 'exact stable version required')
    for value in (release_commit, harness_commit, harness_tree):
        require(exact_hex(value, 40), 'exact source SHA required')
    names = {f'released-native-client-{client}-{target}' for client in CLIENT_TESTS for target in targets}
    require(root.is_dir() and not root.is_symlink(), 'input must be a real directory')
    require({p.name for p in root.iterdir()} == names, f'exactly {len(names)} expected artifact directories required for {scope}')
    files, jobs, release_trees, manifests, checksums = {}, [], set(), set(), set()
    by_target = {}
    for client, required in CLIENT_TESTS.items():
        for target in targets:
            name = f'released-native-client-{client}-{target}'
            bodies = collect(root / name)
            require('runner-evidence.json' in bodies, 'missing runner evidence: ' + name)
            record = json.loads(bodies['runner-evidence.json'], object_pairs_hook=unique_object)
            require(record.get('schema_version') == 1 and record.get('client') == client and record.get('target') == target, 'wrong job identity: ' + name)
            require(record.get('commit') == harness_commit and record.get('tree') == harness_tree, 'wrong harness identity: ' + name)
            require(record.get('status') == 'passed' and type(record.get('exit_code')) is int and record['exit_code'] == 0 and record.get('skipped_tests') == [], 'failed/skipped job: ' + name)
            passed = record.get('passed_tests')
            require(isinstance(passed, list) and all(isinstance(t, str) for t in passed) and required.issubset(passed), 'missing required tests: ' + name)
            hashes = record.get('artifact_sha256')
            require(isinstance(hashes, dict) and set(hashes) == set(bodies) - {'runner-evidence.json'}, 'artifact inventory mismatch: ' + name)
            for path, expected in hashes.items():
                safe_name(path)
                require(exact_hex(expected, 64) and digest(bodies[path]) == expected, 'artifact hash mismatch: ' + name + '/' + path)
            require('native-tests.log' in bodies, 'missing test transcript: ' + name)
            log = bodies['native-tests.log'].decode('utf-8')
            actual = set(re.findall(r'^--- PASS: (\w+)', log, re.M))
            require(set(passed) == actual and required.issubset(actual) and not re.search(r'^\s*--- (SKIP|FAIL):', log, re.M), 'test transcript disagrees: ' + name)
            release = record.get('installer_release', {})
            require(release.get('repository') == REPOSITORY, 'wrong release repository: ' + name)
            if _draft:
                _draft(record, bodies, client, target)
            else:
                require(release.get('acquisition') == 'public GitHub release download', 'not a public released installer: ' + name)
            require(release.get('version') == version and release.get('tag') == 'agentplugins-v' + version and release.get('commit') == release_commit, 'wrong release identity: ' + name)
            require(record.get('installer_version_measured') == 'agentplugins ' + version, 'wrong measured version: ' + name)
            asset = 'agentplugins_' + version + '_' + target.replace('-', '_') + ('.exe' if target.startswith('windows-') else '')
            require(release.get('file') == asset and type(release.get('size')) is int and release['size'] > 0, 'wrong binary asset: ' + name)
            binary_hash = release.get('binary_sha256')
            require(exact_hex(binary_hash, 64) and record.get('installer_sha256') == binary_hash, 'installer hash disagreement: ' + name)
            require(exact_hex(release.get('tree'), 40), 'missing release tree: ' + name)
            for field in ('manifest_sha256', 'checksums_sha256'):
                require(exact_hex(release.get(field), 64), 'missing release digest: ' + name)
            verify_tools(record, target, client)
            if not _draft:
                verify_attestations(release)
            suffix = '.exe' if target.startswith('windows-') else ''
            helpers = record.get('harness_build_sha256', {})
            require(set(helpers) == {'native-probe' + suffix, 'repotests' + suffix} and all(exact_hex(v, 64) for v in helpers.values()), 'missing harness helper hashes: ' + name)
            verify_fixtures(bodies, record, client, target)
            require(target not in by_target or by_target[target] == (binary_hash, release['size']), 'cross-job binary mismatch: ' + name)
            by_target[target] = (binary_hash, release['size'])
            release_trees.add(release['tree'])
            manifests.add(release['manifest_sha256'])
            checksums.add(release['checksums_sha256'])
            files.update({name + '/' + p: b for p, b in bodies.items()})
            jobs.append({'client': client, 'target': target, 'binary_sha256': binary_hash, 'passed_tests': sorted(actual)})
    require(len(release_trees) == len(manifests) == len(checksums) == 1, 'cross-job release metadata mismatch')
    summary = {'schema_version': 1, 'status': 'passed', 'release_version': version, 'release_commit': release_commit,
               'release_tree': next(iter(release_trees)), 'harness_commit': harness_commit, 'harness_tree': harness_tree,
               'scope': scope, 'jobs': jobs, 'files_sha256': {p: digest(b) for p, b in sorted(files.items())},
               'boundary': 'Fixture assertions only; no real-model quality or OAuth claim. Hash and record consistency verification only. Embedded provenance was verified by the runner, not cryptographically reverified here. Log content is unredacted; publication needs disclosure review.'}
    return summary, files


# Draft schemas are deliberately separate from the historical public contract.
def object_keys(value, keys, label):
    require(isinstance(value, dict) and set(value) == set(keys.split()), 'unsupported ' + label + ' schema')


def read_json(body):
    return json.loads(body, object_pairs_hook=unique_object)


def original_file(path):
    mode = path.lstat()
    require(stat.S_ISREG(mode.st_mode) and mode.st_nlink == 1, 'nonregular or aliased original: ' + str(path))
    return path.read_bytes()


# Keep the existing 400 MiB ceiling for each of the six native binaries.
# The npm package contains launcher sources, not copies of those binaries.
ZIP_MEMBER_LIMIT = 400 << 20
ZIP_COMPRESSED_LIMIT = 401 << 20
ZIP_TOTAL_LIMIT = (6 * 400 + 64) << 20

def draft_bundle(body, args, native):
    """Inspect preserved ZIP/tar members as bytes; never extract or execute them."""
    import io
    require(digest(body) == args.producer_artifact_digest, 'producer artifact digest mismatch')
    files, seen = {}, set()
    version, tag = args.release_version, args.release_tag
    binary_names = {target: f'agentplugins_{version}_{target.replace("-", "_")}' +
                    ('.exe' if target.startswith('windows') else '') for target in native.TARGETS}
    names = set(binary_names.values()) | {'release-manifest.json', 'checksums.txt', 'THIRD_PARTY_NOTICES.txt'}
    expected = {'verified-release.json'} | {'release-assets/' + n for n in names}
    with zipfile.ZipFile(io.BytesIO(body)) as zipped:
        inventory = zipped.infolist()
        require(len(inventory) <= len(expected) + 2, 'too many ZIP members')
        total = compressed_total = 0
        for item in inventory:
            require(item.orig_filename == item.filename, 'aliased ZIP member name')
            name = item.filename.rstrip('/') if item.is_dir() else item.filename
            safe_name(name)
            require(name.casefold() not in seen, 'duplicate or aliased ZIP member')
            seen.add(name.casefold())
            mode = stat.S_IFMT(item.external_attr >> 16)
            require(mode in ((0, stat.S_IFDIR) if item.is_dir() else (0, stat.S_IFREG)), 'nonregular ZIP member')
            require(item.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                    and not item.flag_bits & 1, 'unsupported ZIP compression/encryption')
            require(0 <= item.file_size < ZIP_MEMBER_LIMIT and
                    0 <= item.compress_size <= ZIP_COMPRESSED_LIMIT, 'oversized ZIP member')
            total += item.file_size
            compressed_total += item.compress_size
            require(total <= ZIP_TOTAL_LIMIT and compressed_total <= ZIP_TOTAL_LIMIT,
                    'oversized ZIP aggregate')
            if item.is_dir():
                require(item.filename == 'release-assets/' and item.file_size == 0, 'unexpected bundle directory')
            else:
                files[name] = item
        tarballs = [n for n in files if '/' not in n and n.endswith('.tgz')]
        require(len(tarballs) == 1, 'exactly one original npm tarball required')
        tarball = tarballs[0]
        require(set(files) == expected | {tarball}, 'unexpected or missing bundle assets/notices')
        # Complete the inventory validation before opening any member body.
        files = {name: zipped.read(item) for name, item in files.items()}
    assets = {n.removeprefix('release-assets/'): b for n, b in files.items() if n.startswith('release-assets/')}
    computed = {}
    for target in sorted(native.TARGETS):
        name = binary_names[target]
        require(name in assets and assets[name], 'missing binary asset')
        computed[target] = {'file': name, 'size': len(assets[name]), 'sha256': digest(assets[name])}
    require(set(assets) == names and set(files) == {tarball, 'verified-release.json'} | {'release-assets/' + n for n in names}, 'unexpected or missing bundle assets/notices')
    require(all(assets.values()), 'empty release asset/notices')
    manifest = read_json(assets['release-manifest.json'])
    require(type(manifest.get('schema_version')) is int and manifest == {
        'schema_version': 2, 'version': version, 'tag': tag, 'commit': args.release_commit, 'assets': computed}, 'unsupported or inconsistent manifest schema')
    checks = {}
    for line in assets['checksums.txt'].decode('utf-8').splitlines():
        match = re.fullmatch(r'([0-9a-f]{64})  ([^/\\]+)', line)
        require(match is not None and match[2] not in checks, 'invalid or duplicate checksum')
        checks[match[2]] = match[1]
    require(checks == {n: digest(b) for n, b in assets.items() if n != 'checksums.txt'}
            and digest(assets['checksums.txt']) == args.expected_asset_set_digest, 'frozen checksums mismatch')
    verified = read_json(files['verified-release.json'])
    require(verified == {'repository': '777genius/plugin-kit-ai', 'version': version, 'tag': tag,
        'commit': args.release_commit, 'assets': computed, 'manifest_schema': 2,
        'manifest_sha256': digest(assets['release-manifest.json']),
        'notices': [{'file': 'THIRD_PARTY_NOTICES.txt', 'sha256': digest(assets['THIRD_PARTY_NOTICES.txt'])}],
        'gate_eligible': True} and type(verified['manifest_schema']) is int and verified['gate_eligible'] is True,
        'unsupported or inconsistent verified-release schema')
    package = native.inspect_package(files[tarball], verified)
    for name in ('package/package.json', 'package/assets.json',
                 'package/THIRD_PARTY_NOTICES.txt', 'package/bin/agentplugins.js'):
        require(name in package, 'missing npm file: ' + name)
    for name in ('package/package.json', 'package/assets.json'):
        read_json(package[name])  # Reject duplicate JSON keys too.
    require(package['package/THIRD_PARTY_NOTICES.txt'] == assets['THIRD_PARTY_NOTICES.txt'], 'packaged notices mismatch')
    launcher = digest(package['package/bin/agentplugins.js'])
    return assets, verified, digest(files[tarball]), launcher


def verify_draft(root, args):
    import native_client_draft as native
    native.validate(args)
    require(args.scope in SCOPES, 'unknown evidence scope')
    originals = {name: original_file(args.qualification / name) for name in ('qualification.json', 'final-draft.json')}
    require(not args.qualification.is_symlink() and {p.name for p in args.qualification.iterdir()} == set(originals), 'unexpected qualification files')
    q = read_json(originals['qualification.json'])
    object_keys(q, 'schema_version status release_state target_scope producer_commit harness_commit harness_tree tarball_sha256 producer helper_sha256 lanes final_draft limitation', 'qualification')
    require(type(q['schema_version']) is int and q['schema_version'] == 1 and q['status'] == 'passed'
            and q['release_state'] == 'verified-draft' and q['target_scope'] == args.scope
            and q['producer_commit'] == args.release_commit and q['harness_commit'] == args.harness_commit
            and q['harness_tree'] == args.harness_tree, 'qualification identity mismatch')
    producer = {'repository': REPOSITORY, 'workflow': native.WORKFLOW, 'commit': args.release_commit,
                'run_id': int(args.producer_run_id), 'run_attempt': int(args.producer_run_attempt),
                'artifact_id': int(args.producer_artifact_id), 'artifact_digest': args.producer_artifact_digest}
    require(q['producer'] == producer and all(type(q['producer'][k]) is int for k in ('run_id', 'run_attempt', 'artifact_id')), 'producer identity mismatch')
    helpers = q['helper_sha256']
    require(isinstance(helpers, dict) and set(helpers) == {
        'scripts/run-native-client-matrix.py', 'scripts/native_client_draft.py',
        'scripts/verify-agentplugins-draft.py', 'scripts/verify-released-native-evidence.py',
        'npm/agentplugins/scripts/release-assets.js'} and all(exact_hex(v, 64) for v in helpers.values()), 'unsupported helper inventory')
    bundle = original_file(args.producer_bundle)
    assets, verified, tarball, launcher = draft_bundle(bundle, args, native)
    require(q['tarball_sha256'] == tarball, 'qualification tarball mismatch')
    final = read_json(originals['final-draft.json'])
    require(final == q['final_draft'], 'final receipt differs from qualification')

    def receipt(record):
        object_keys(record, 'schema_version release_state verified_at repository release asset_set_digest subjects signer_workflow producer_commit producer_run_id producer_run_attempt', 'draft receipt')
        require(type(record['schema_version']) is int and record['schema_version'] == 1
                and record['release_state'] == 'verified-draft' and record['repository'] == REPOSITORY
                and record['asset_set_digest'] == args.expected_asset_set_digest
                and record['signer_workflow'] == f'github.com/{REPOSITORY}/{native.WORKFLOW}'
                and record['producer_commit'] == args.release_commit
                and all(type(record[k]) is int and record[k] == producer[v] for k, v in
                        (('producer_run_id', 'run_id'), ('producer_run_attempt', 'run_attempt'))), 'draft receipt identity mismatch')
        from datetime import datetime
        require(isinstance(record['verified_at'], str) and datetime.fromisoformat(record['verified_at']).utcoffset() is not None, 'missing receipt timestamp')
        release = record['release']
        object_keys(release, 'id tag commit draft prerelease updated_at assets', 'release snapshot')
        require(type(release['id']) is int and release['id'] == int(args.release_id)
                and release['tag'] == args.release_tag and release['commit'] == args.release_commit
                and release['draft'] is True and release['prerelease'] is False
                and isinstance(release['updated_at'], str) and release['updated_at'], 'release snapshot identity mismatch')
        subjects, ids = {}, set()
        for subject in record['subjects']:
            object_keys(subject, 'name id size sha256', 'subject')
            name = subject['name']
            require(name in assets and name not in subjects and type(subject['id']) is int and subject['id'] > 0
                    and subject['id'] not in ids and type(subject['size']) is int
                    and subject['size'] == len(assets[name]) and subject['sha256'] == digest(assets[name]), 'duplicate or inconsistent subject')
            subjects[name] = subject
            ids.add(subject['id'])
        require(set(subjects) == set(assets), 'incomplete receipt subjects')
        seen = set()
        for asset in release['assets']:
            object_keys(asset, 'id name size state created_at updated_at digest', 'release asset')
            name = asset['name']
            require(name in subjects and name not in seen and asset['state'] == 'uploaded'
                    and type(asset['id']) is int and type(asset['size']) is int
                    and all(asset[k] == subjects[name][k] for k in ('id', 'size'))
                    and asset['digest'] in (None, 'sha256:' + subjects[name]['sha256'])
                    and all(isinstance(asset[k], str) and asset[k] for k in ('created_at', 'updated_at')), 'release asset identity mismatch')
            seen.add(name)
        require(seen == set(assets), 'incomplete release snapshot')
        return subjects

    subjects = receipt(final)
    expected = {(c, t) for c in CLIENT_TESTS for t in SCOPES[args.scope]}
    indexed = {}
    for lane in q['lanes']:
        object_keys(lane, 'client target evidence_sha256', 'qualification lane')
        key = (lane['client'], lane['target'])
        require(key in expected and key not in indexed and exact_hex(lane['evidence_sha256'], 64), 'duplicate or unknown qualification lane')
        indexed[key] = lane['evidence_sha256']
    require(set(indexed) == expected, 'missing qualification lanes')

    def lane_check(record, bodies, client, target):
        require(type(record['schema_version']) is int and digest(bodies['runner-evidence.json']) == indexed[client, target], 'qualification lane digest mismatch')
        require(len(record['passed_tests']) == len(set(record['passed_tests'])), 'duplicate passed tests')
        require(record.get('helper_sha256') == helpers, 'mixed helper hashes')
        require(len({p.casefold() for p in bodies}) == len(bodies), 'aliased lane paths')
        release = record['installer_release']
        require(release.get('acquisition') == 'authenticated producer npm artifact'
                and release.get('release_state') == 'draft' and release.get('producer') == producer
                and type(release.get('release_id')) is int and release['release_id'] == int(args.release_id)
                and release.get('tarball_sha256') == tarball
                and all(type(release['producer'][k]) is int for k in ('run_id', 'run_attempt', 'artifact_id')), 'draft lane producer/package mismatch')
        initial = release['initial_draft']
        receipt(initial)
        from datetime import datetime
        require(datetime.fromisoformat(initial['verified_at']) <= datetime.fromisoformat(final['verified_at']), 'final receipt predates initial verification')
        require(initial['release'] == final['release'] and initial['subjects'] == final['subjects']
                and read_json(bodies['initial-draft.json']) == initial, 'mixed initial/final draft receipts')
        require(all(release[k] == v for k, v in {
            'manifest_sha256': verified['manifest_sha256'], 'checksums_sha256': args.expected_asset_set_digest,
            'file': verified['assets'][target]['file'], 'size': verified['assets'][target]['size'],
            'binary_sha256': verified['assets'][target]['sha256']}.items()), 'lane differs from frozen assets')
        packaged = record['packaged_acquisition']
        object_keys(packaged, 'bootstrap_source tarball_sha256 launcher_sha256 binary_sha256 size version cold_bootstrap warm_without_proof_source npm_ignore_scripts', 'packaged acquisition')
        require(packaged == {'bootstrap_source': 'local_frozen_asset', 'tarball_sha256': tarball,
            'launcher_sha256': launcher, 'binary_sha256': release['binary_sha256'], 'size': release['size'],
            'version': 'agentplugins ' + args.release_version, 'cold_bootstrap': True,
            'warm_without_proof_source': True, 'npm_ignore_scripts': True}
            and all(packaged[k] is True for k in ('cold_bootstrap', 'warm_without_proof_source', 'npm_ignore_scripts'))
            and type(packaged['size']) is int, 'packaged acquisition mismatch')
        require(set(release['attestations']) == set(subjects), 'missing or extra recorded draft attestations')
        for name, subject in subjects.items():
            native.verify_attestation(release['attestations'][name], name, subject['sha256'], args.release_commit)

    summary, files = verify(root, args.release_version, args.release_commit, args.harness_commit,
                            args.harness_tree, scope=args.scope, _draft=lane_check)
    files.update({'draft-qualification/' + n: b for n, b in originals.items()})
    files['producer-artifact.zip'] = bundle
    summary.update(release_state='draft', verification_mode='offline-draft-archive', producer=producer,
                   release_id=int(args.release_id), tarball_sha256=tarball, helper_sha256=helpers,
                   files_sha256={p: digest(b) for p, b in sorted(files.items())},
                   boundary='Offline recorded consistency only; no fresh attestation or current draft-state verification. Genuine-client claims require the original successful run; no real-model, OAuth or authoring D5 qualification. Logs are unredacted; retain privately pending disclosure review.')
    return summary, files


def archive(output, summary, files):
    """Freeze already verified bytes, not mutable source paths; refuse overwrite."""
    payload = dict(files)
    payload['summary.json'] = (json.dumps(summary, indent=2, sort_keys=True) + '\n').encode()
    with output.open('xb') as stream:
        with zipfile.ZipFile(stream, 'w', compression=zipfile.ZIP_DEFLATED) as zipped:
            for name, body in sorted(payload.items()):
                safe_name(name)
                info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
                info.external_attr = (stat.S_IFREG | 0o644) << 16
                zipped.writestr(info, body, compress_type=zipfile.ZIP_DEFLATED)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('input', type=Path)
    for name in ('release-version', 'release-commit', 'harness-commit', 'harness-tree'):
        parser.add_argument('--' + name, required=True)
    parser.add_argument('--scope', choices=SCOPES, default='historical-nine', help='Linux amd64 is supplemental proof, never a replacement for historical nine')
    parser.add_argument('--archive', type=Path, help='new durable ZIP path outside input')
    parser.add_argument('--release-state', choices=('public', 'draft'), default='public')
    parser.add_argument('--qualification', type=Path, help='original scope-2 qualification artifact directory')
    parser.add_argument('--producer-bundle', type=Path, help='original producer Actions artifact ZIP')
    for field in ('producer-run-id', 'producer-run-attempt', 'producer-artifact-id', 'producer-artifact-digest', 'release-id', 'expected-asset-set-digest'):
        parser.add_argument('--' + field, default='')
    args = parser.parse_args()
    if args.archive:
        require(not args.archive.resolve().is_relative_to(args.input.resolve()), 'archive must be outside input')
    if args.release_state == 'draft':
        require(args.qualification is not None and args.producer_bundle is not None, 'draft requires qualification and original producer bundle')
        args.release_repo, args.release_tag = REPOSITORY, 'agentplugins-v' + args.release_version
        summary, files = verify_draft(args.input, args)
    else:
        require(not any((args.qualification, args.producer_bundle, args.producer_run_id, args.producer_run_attempt, args.producer_artifact_id, args.producer_artifact_digest, args.release_id, args.expected_asset_set_digest)), 'public mode rejects draft inputs')
        summary, files = verify(args.input, args.release_version, args.release_commit, args.harness_commit, args.harness_tree, scope=args.scope)
    if args.archive:
        archive(args.archive, summary, files)
    print(json.dumps(summary, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
