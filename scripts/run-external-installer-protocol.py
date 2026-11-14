#!/usr/bin/env python3
"""One hosted Linux arm64 checkpoint of the documented external skills protocol.

No model session or credentials. This is maintainer protocol execution evidence,
not an independent tester report. Never execute on a user machine or profile.
"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

SPEC = importlib.util.spec_from_file_location('matrix', Path(__file__).with_name('run-native-client-matrix.py'))
matrix = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(matrix)
VERSION = '0.1.53'
RELEASE_COMMIT = '28cf05af0a1e4fea642825dd34b78f9c99094ab5'
PLUGIN = 'uap-external-check'


def require(ok, message):
    if not ok:
        raise ValueError(message)


def fixture(document):
    require("UAP_VERSION='" + VERSION + "'" in document, 'document release version differs')
    files = {}
    for path, marker in [('plugin.json', 'JSON'), ('skills/uap-external-check/SKILL.md', 'SKILL')]:
        pattern = r"cat > fixture/" + re.escape(path) + r" <<'" + marker + r"'\n(.*?)\n" + marker + r"\n"
        matches = re.findall(pattern, document, re.S)
        require(len(matches) == 1, 'document fixture missing or ambiguous: ' + path)
        files[path] = (matches[0] + '\n').encode()
    manifest = json.loads(files['plugin.json'])
    require(manifest.get('name') == PLUGIN and manifest.get('$schema') == 'https://agent-plugins.org/schemas/1.0.0/plugin.schema.json', 'unexpected fixture identity')
    return files


def native_identity(inventory, active):
    matches = [p for p in inventory.get('installed', []) if p.get('name') == PLUGIN]
    require(len(matches) == 1, 'native plugin name missing or ambiguous')
    entry = matches[0]
    marketplace = entry.get('marketplaceName', '')
    require(re.fullmatch('agentplugins-[0-9a-f]{12}', marketplace), 'unexpected managed marketplace')
    require(entry.get('pluginId') == PLUGIN + '@' + marketplace and entry.get('source', {}).get('path') == str(active) and entry.get('installed') is True and entry.get('enabled') is True, 'native identity/path/state mismatch')
    return entry['pluginId'], marketplace


def target_paths(value):
    if isinstance(value, dict):
        return {v for k, v in value.items() if k == 'target_locator' and isinstance(v, str)} | set().union(*(target_paths(v) for v in value.values()))
    if isinstance(value, list):
        return set().union(*(target_paths(v) for v in value))
    return set()



def run_command(label, command, project, env, output, evidence, structured=False):
    try:
        result = subprocess.run(command, cwd=project, env=env, capture_output=True, timeout=300)
    except subprocess.TimeoutExpired as error:
        # subprocess captures bytes even for partial output. Preserve them before
        # propagating failure so main's finally block can hash the diagnostics.
        (output / (label + '.log')).write_bytes(error.stdout or b'')
        (output / (label + '-stderr.log')).write_bytes(error.stderr or b'')
        evidence['commands'].append({'label': label, 'argv': command, 'exit_code': None,
                                     'timed_out': True, 'timeout_seconds': error.timeout})
        raise
    (output / (label + '.log')).write_bytes(result.stdout)
    (output / (label + '-stderr.log')).write_bytes(result.stderr)
    evidence['commands'].append({'label': label, 'argv': command, 'exit_code': result.returncode})
    require(result.returncode == 0, 'command failed: ' + label)
    text = result.stdout.decode('utf-8', errors='strict')
    return json.loads(text) if structured else text.strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    matrix.require_hosted('linux-arm64')
    source = Path(__file__).resolve().parents[1]
    commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=source, encoding='utf-8').strip()
    require(commit == os.environ.get('EXPECTED_COMMIT') and re.fullmatch('[0-9a-f]{40}', commit), 'unexpected workflow source')
    require(not subprocess.check_output(['git', 'status', '--porcelain'], cwd=source), 'dirty workflow checkout')
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    scratch = Path(tempfile.mkdtemp(prefix='uap-external-hosted-', dir=os.environ['RUNNER_TEMP'])).resolve()
    evidence = {'status': 'failed', 'harness_commit': commit, 'release_version': VERSION, 'release_commit': RELEASE_COMMIT,
                'boundary': 'Maintainer-run documented skills-only protocol; no model session, OAuth or independent adoption.', 'commands': []}
    try:
        home, project, binaries, cache = [scratch / n for n in ('home', 'project', 'bin', 'npm-cache')]
        for path in (home, project, binaries, cache): path.mkdir()
        client, evidence['client_asset'] = matrix.provision(matrix.PINS['linux-arm64']['codex'], binaries, 'codex')
        npx, npm, node = [shutil.which(n) for n in ('npx', 'npm', 'node')]
        require(all((npx, npm, node)), 'Node/npm/npx unavailable')
        env = {'HOME': str(home), 'USERPROFILE': str(home), 'PATH': str(binaries) + ':' + str(Path(node).parent) + ':/usr/bin:/bin',
               'TMPDIR': str(scratch), 'LANG': 'en_US.UTF-8', 'NPM_CONFIG_CACHE': str(cache)}
        document = (source / 'docs/external-installer-testing.md').read_text()
        evidence['document_sha256'] = hashlib.sha256(document.encode()).hexdigest()
        bodies = fixture(document)
        evidence['fixture_sha256'] = {p: hashlib.sha256(b).hexdigest() for p, b in bodies.items()}
        for relative, body in bodies.items():
            path = project / 'fixture' / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(body)
        sentinel = project / 'unrelated-sentinel.txt'
        sentinel.write_bytes(b'preserve this unrelated file\n')
        def run(label, command, structured=False):
            return run_command(label, command, project, env, output, evidence, structured)
        def uap(label, *arguments):
            return run(label, [npx, '--yes', 'universal-agent-plugins@' + VERSION, *arguments], '--format' in arguments)
        evidence['node_version'] = run('node-version', [node, '--version'])
        evidence['npm_version'] = run('npm-version', [npm, '--version'])
        require(run('client-version', [str(client), '--version']) == 'codex-cli 0.153.4', 'wrong Codex version')
        require(uap('version', 'version') == 'agentplugins ' + VERSION, 'wrong installer version')
        manifests = list(cache.glob('_npx/*/node_modules/universal-agent-plugins/assets.json'))
        require(len(manifests) == 1, 'npm installer identity missing/ambiguous')
        package = json.loads(manifests[0].read_text())
        require(package['version'] == VERSION and package['producer']['commit'] == RELEASE_COMMIT, 'wrong published npm producer')
        evidence['npm_assets_sha256'] = hashlib.sha256(manifests[0].read_bytes()).hexdigest()
        validation = uap('validate', 'validate', './fixture', '--format', 'json')
        require(validation['data']['conformant'] is True and validation['data']['components']['skills'] == 1 and validation['data']['components']['mcp_servers'] == validation['data']['components']['app_bindings'] == 0, 'wrong validated fixture')
        for step in ('add', 'repair'):
            result = uap(step, step, './fixture' if step == 'add' else PLUGIN, '--target', 'codex', '--format', 'json')
            require(result.get('result') == 'success' and result['data']['failed'] == 0 and result['data']['succeeded'] == 1, 'unsuccessful lifecycle: ' + step)
        state = json.loads((home / '.config/agentplugins/state-v2.json').read_text())
        paths = target_paths(state)
        require(len(paths) == 1, 'expected exactly one managed path')
        active = Path(next(iter(paths)))
        require(active.is_absolute() and active.resolve().is_relative_to(home) and active.is_dir(), 'managed path outside fresh profile')
        require((active / 'skills/uap-external-check/SKILL.md').read_bytes() == bodies['skills/uap-external-check/SKILL.md'], 'managed skill body differs')
        plugin_id, marketplace = native_identity(run('native-before-remove', [str(client), 'plugin', 'list', '--json'], True), active)
        evidence['native_plugin_id'] = plugin_id
        run('native-remove', [str(client), 'plugin', 'remove', plugin_id, '--json'])
        run('native-marketplace-remove', [str(client), 'plugin', 'marketplace', 'remove', marketplace, '--json'])
        remaining = run('native-after-remove', [str(client), 'plugin', 'list', '--json'], True)
        require(not any(p.get('pluginId') == plugin_id and (p.get('installed') or p.get('enabled')) for p in remaining['installed']), 'native plugin remains installed/enabled')
        removed = uap('remove', 'remove', PLUGIN, '--target', 'codex', '--external-uninstalled', '--format', 'json')
        require(removed.get('result') == 'success' and removed['data']['failed'] == 0 and not active.exists(), 'managed removal incomplete')
        require(sentinel.read_bytes() == b'preserve this unrelated file\n', 'unrelated project file modified')
        evidence['status'] = 'passed'
    finally:
        evidence['artifact_sha256'] = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in output.iterdir() if p.is_file()}
        (output / 'protocol-evidence.json').write_text(json.dumps(evidence, indent=2) + '\n')


if __name__ == '__main__':
    main()
