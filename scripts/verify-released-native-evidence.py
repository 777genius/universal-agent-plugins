#!/usr/bin/env python3
"""Verify scoped downloaded released-native artifacts and optionally freeze a ZIP.

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

def verify(root, version, release_commit, harness_commit, harness_tree, *, scope='historical-nine'):
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
            require(release.get('repository') == REPOSITORY and release.get('acquisition') == 'public GitHub release download', 'not a public released installer: ' + name)
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
    args = parser.parse_args()
    if args.archive:
        require(not args.archive.resolve().is_relative_to(args.input.resolve()), 'archive must be outside input')
    summary, files = verify(args.input, args.release_version, args.release_commit, args.harness_commit, args.harness_tree, scope=args.scope)
    if args.archive:
        archive(args.archive, summary, files)
    print(json.dumps(summary, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
