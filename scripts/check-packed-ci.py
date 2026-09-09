#!/usr/bin/env python3
"""Read-only terminal checks; successful same-SHA suites supply execution proof."""
import base64
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
import stat
import sys

VERSIONS = dict(go='go1.25.13', node='v22.23.2', npm='10.9.8')
MODE = 'release-cli-contract-v1'
PRODUCTS = ('agentplugins', 'plugin-kit-ai')
LANES = ('skill', 'mcp-remote', 'mcp-stdio', 'hybrid-remote', 'hybrid-stdio')
TARGETS = ('cursor', 'codex', 'claude')
NAME = 'TestPackedGeneratedPackagesReachExistingInstallerPlanner'
REGEX = '^' + NAME + '$'
PACKAGE_PATH = './cli/plugin-kit-ai/internal/authoring/commands'
PACKAGE = 'github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands'
CANDIDATE_TEST = 'actual controlled Linux pair: offline verification, engine reports, frozen journeys and negative proof'
NATIVE_TESTS = ['native retirement oracle preserves original format tails and rejects output mismatches',
    'NATIVE opt-in: exact two Linux tarballs, five accepted template lanes and complete release journeys']
PUBLIC_TEST = 'PUBLIC NATIVE: exact integrated packs, both actual bins, static parity and lifecycle'
CLAIMS = ('release_eligible', 'platform_acceptance', 'attested')


def require(condition, message):
    if not condition:
        raise ValueError(message)


def data(path):
    path = Path(path)
    require(path.resolve() == path.absolute(), 'symlink path: ' + str(path))
    st = path.lstat()
    require(stat.S_ISREG(st.st_mode) and st.st_nlink == 1, 'nonregular/hardlinked evidence: ' + str(path))
    return path.read_bytes()


def digest(path):
    return hashlib.sha256(data(path)).hexdigest()


def read(path):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, 'duplicate JSON key')
            result[key] = value
        return result
    return json.loads(data(path), object_pairs_hook=unique)


def false_claims(record, fields=CLAIMS):
    require(all(record.get(k) is False for k in fields), 'missing/true claim')


def tap(text, names):
    require(text.startswith('TAP version 13\n') and text.endswith('\n'), 'truncated TAP')
    require(not re.search(r'(?im)^\s*not ok\b|^ok .*#\s*(?:SKIP|TODO)\b|^Bail out!', text), 'failed/skipped TAP')
    require(re.findall(r'^ok \d+ - (.+)$', text, re.M) == names, 'missing/extra named native test')
    require(re.findall(r'^# Subtest: (.+)$', text, re.M) == names, 'missing native starts')
    for key, number in dict(tests=len(names), suites=0, pass_=len(names), fail=0, cancelled=0, skipped=0, todo=0).items():
        require(re.findall(r'^# ' + key.rstrip('_') + r' (\d+)$', text, re.M) == [str(number)], 'TAP terminal ' + key)
    require(re.search(r'^# duration_ms [0-9.]+\n\Z', text, re.M), 'truncated TAP terminal')
    require(re.findall(r'^1\.\.(\d+)$', text, re.M) == [str(len(names))], 'missing TAP plan')


def events(text):
    require(text.endswith('\n'), 'truncated Go JSON')
    rows = [json.loads(line) for line in text.splitlines()]
    require(rows and all(e.get('Package') == PACKAGE and e.get('Action') not in ('fail', 'skip') for e in rows), 'wrong package/failed/skipped Go event')
    require([e['Action'] for e in rows if not e.get('Test') and e['Action'] in ('pass', 'fail', 'skip')] == ['pass'], 'missing Go package success')
    return rows


def go_discovery(text):
    rows = events(text)
    require([e.get('Output') for e in rows if e.get('Output', '').startswith('Test')] == [NAME + '\n'], 'exact named discovery required')
    require(not any(e.get('Test') for e in rows), 'execution substituted for discovery')


def go_results(text):
    rows = events(text)
    leaves = {NAME, *(NAME + '/' + p + '/' + l for p in PRODUCTS for l in LANES)}
    groups = {NAME + '/' + p for p in PRODUCTS}
    ran = Counter(e['Test'] for e in rows if e.get('Test') and e['Action'] == 'run')
    passed = Counter(e['Test'] for e in rows if e.get('Test') and e['Action'] == 'pass')
    require(ran == passed and leaves <= set(ran) <= leaves | groups and all(v == 1 for v in ran.values()),
            'exact root/ten leaves must run/pass; optional product groups are not leaves')
    require(all(not e.get('Test') or e['Test'] in ran for e in rows), 'unexpected subtest')


def plans(record):
    rows = record['plans']
    require(len(rows) == 30 and Counter((r['product'], r['lane'], r['target']) for r in rows) ==
            Counter({(p, l, t): 1 for p in PRODUCTS for l in LANES for t in TARGETS}), 'exact 30 unique plan tuples required')
    for row in rows:
        report = row['report']
        require(report['result'] == 'success' and report['data']['dry_run'] is True, 'successful dry run required')
        found = []
        def walk(value):
            if isinstance(value, dict):
                if 'client_id' in value and 'components' in value and 'status' in value:
                    found.append(value)
                for child in value.values(): walk(child)
            elif isinstance(value, list):
                for child in value: walk(child)
        walk(report)
        require(len(found) == 1, 'one target plan required')
        plan = found[0]
        require(plan['client_id'] == row['target'] and plan['scope'] == 'user' and
                plan['status'] == ('ready' if row['target'] == 'claude' else 'manual_activation_required'), 'wrong plan target/status')
        components = plan['components']
        expected = {('skill', 'extra-skill')}
        if row['lane'] == 'skill' or row['lane'].startswith('hybrid-'): expected.add(('skill', row['lane']))
        if row['lane'] != 'skill': expected.add(('mcp_server', row['lane']))
        require({(c['kind'], c['name']) for c in components} == expected, 'exact lane component names')
        kinds = Counter(c['kind'] for c in components)
        require(kinds == Counter(skill=2 if row['lane'] == 'skill' or row['lane'].startswith('hybrid-') else 1,
                                 **({} if row['lane'] == 'skill' else {'mcp_server': 1})), 'lane components')
        require(len({(c['kind'], c['name']) for c in components}) == len(components) and
                any(c['kind'] == 'skill' and c['name'] == 'extra-skill' for c in components) and
                all(c.get('support') and c['support'] != 'unsupported' for c in components), 'missing/duplicate/unsupported components')


def invocation_rows(invocations, count):
    require(count == len(invocations) == 741, 'exactly 741 invocations')
    require(len({json.dumps(r, sort_keys=True) for r in invocations}) == 739, 'repeated invocation evidence')
    counts = Counter((row['product'], tuple(row['argv']), row['status']) for row in invocations)
    for command in ('publish', 'publication', 'publication doctor'):
        for tail in (('--format=credential-fixture',), ('--format', 'credential-fixture')):
            original = ('--format=json', *command.split(), *tail)
            for argv in (original, (*original, '--format=json')):
                require(counts[('plugin-kit-ai', argv, 2)] == 1, 'original format tail/JSON companion missing')
    for row in invocations:
        require(row['product'] in PRODUCTS and type(row['status']) is int and row['status'] in (0, 1, 2) and row['signal'] is None and row['stderr'] == '', 'invalid invocation')

def check(root, sha):
    run = read(root / 'run.json'); false_claims(run)
    require(run['head'] == sha and re.fullmatch('[0-9a-f]{40}', sha) and run['versions'] == VERSIONS, 'run identity/tools')
    require(str(run['run_id']).isdigit() and str(run['attempt']).isdigit(), 'run identity missing')
    identity = run['identity']
    require(identity == dict(repository='777genius/universal-agent-plugins', commit=sha, engine_revision=sha,
        versions={'agentplugins': '0.1.91', 'plugin-kit-ai': '2.0.0'}), 'candidate identity')
    for tool in ('go', 'node', 'npm'):
        require(digest(Path(run['tools'][tool]['path'])) == run['tools'][tool]['sha256'], 'tool changed')
    phases = ('head', 'clean', 'go-version', 'node-version', 'npm-version', 'go-host', 'warmup', 'stage', 'verify',
              'candidate-native', 'npm-stage', 'npm-native', 'seal', 'discovery', 'planner', 'post-verify', 'terminal-clean')
    for name in phases:
        phase = read(root / 'logs' / (name + '.json'))
        require(type(phase['exit']) is int and phase['exit'] == 0, 'phase failed: ' + name)
        data(root / 'logs' / (name + '.stdout')); data(root / 'logs' / (name + '.stderr'))
        require('AGENTPLUGINS_STAGED_TEST_CHILD' not in phase['env'] and 'GOFLAGS' not in phase['env'] and 'NODE_OPTIONS' not in phase['env'], 'forbidden inherited control')
        if name in ('discovery', 'planner'):
            flag = '-list' if name == 'discovery' else '-run'
            require(phase['argv'] == [run['tools']['go']['path'], 'test', '-p=2', '-tags=packedci', '-json',
                *([] if name == 'discovery' else ['-count=1', '-timeout=20m']), flag, REGEX, PACKAGE_PATH], 'tag/selection command mismatch')
    repo = Path(__file__).resolve().parent.parent
    scripts = repo / 'npm/agentplugins/scripts'
    node = run['tools']['node']['path']
    expected_commands = {
        'candidate-native': [node, '--test', '--test-reporter=tap', 'npm/agentplugins/test/dual-authoring-candidate-native.test.js'],
        'npm-native': [node, '--test', '--test-reporter=tap', 'npm/agentplugins/test/private-npm-native.test.js'],
        'post-verify': [node, str(scripts / 'packed-installer-bridge.js'), 'verify', str(root / 'bridge-config/sealed.json'),
                        digest(root / 'bridge-config/sealed.json'), sha]}
    for name, script, args in (
        ('stage', 'stage-dual-authoring-candidate.js', ['stage', '--candidate', str(root / 'stage.json')]),
        ('verify', 'stage-dual-authoring-candidate.js', ['verify', '--candidate', str(root / 'verify.json')]),
        ('npm-stage', 'stage-dual-authoring-npm.js', ['--candidate', str(root / 'npm-stage.json')]),
        ('seal', 'packed-installer-bridge.js', ['seal', str(root / 'bridge-config/request.json'), str(root / 'bridge-config/sealed.json')])):
        expected_commands[name] = [node, str(scripts / script), *args]
    for name, argv in expected_commands.items():
        phase = read(root / 'logs' / (name + '.json'))
        require(all(phase['env'].get(k) == v for k, v in dict(GOPROXY='off', GOSUMDB='off', GOENV='off', GOTOOLCHAIN='local', GOVCS='*:off').items()), 'offline phase environment')
        require(phase['argv'] == argv and phase['cwd'] == str(repo), 'wrong execution command: ' + name)
    for name, key, value in [('candidate-native', 'UAP_CANDIDATE_NATIVE_CONFIG', root / 'verify.json'),
        ('candidate-native', 'UAP_CANDIDATE_NATIVE_SOURCE_REPO', repo),
        ('npm-native', 'UAP_PRIVATE_NPM_NATIVE_CONFIG', root / 'npm-native.json')]:
        require(read(root / 'logs' / (name + '.json'))['env'].get(key) == str(value), 'missing native opt-in')
    log = lambda name: data(root / 'logs' / (name + '.stdout')).decode()
    tap(log('candidate-native'), [CANDIDATE_TEST]); tap(log('npm-native'), NATIVE_TESTS)
    go_discovery(log('discovery')); go_results(log('planner'))
    candidate = read(root / 'candidate/candidate.json')
    pack = read(root / 'npm-pair/completion.json')
    native = read(root / 'native/native-completion.json')
    cfg = read(root / 'npm-native.json')
    sealed = read(root / 'bridge-config/sealed.json')
    result = read(root / 'results/completion.json')
    require(candidate['schema'] == 'dual-authoring-candidate/v1' and pack['schema'] == 'dual-authoring-npm-completion/v1' and
        native['kind'] == 'actual-linux-private-npm-pair' and sealed['schema'] == 'packed-installer-bridge/v1' and
        result['kind'] == 'packed-generated-existing-injected-installer-planner', 'wrong terminal schema/kind')
    false_claims(candidate, ('release_eligible',))
    for record in (pack, native, sealed, result): false_claims(record)
    for record in (candidate, pack, native): require(record['identity'] == identity, 'terminal identity mismatch')
    candidate_pin = digest(root / 'candidate/candidate.json')
    require(json.loads(log('stage'))['manifest_sha256'] == candidate_pin, 'stage pin')
    false_claims(json.loads(log('stage')))
    verified = json.loads(log('verify')); false_claims(verified)
    require(verified['manifest_sha256'] == candidate_pin and verified['consistency_verified'] is True, 'build verification')
    require(candidate['asset_scope'] == 'linux-amd64-pair', 'candidate scope')
    require(pack['status'] == candidate['status'] == 'CANDIDATE' and pack['authoring_mode'] == MODE and pack['asset_scope'] == 'linux-amd64-pair', 'candidate mode/scope')
    require(pack['candidate_sha256'] == native['candidate_sha256'] == candidate_pin, 'candidate chain')
    require(native['completion_sha256'] == cfg['completionDigest'] == digest(root / 'npm-pair/completion.json'), 'pack completion chain')
    require(native['packs'] == pack['packs'] and set(pack['packs']) == set(PRODUCTS), 'pack identity')
    for p in pack['packs'].values():
        body = data(root / 'npm-pair' / p['file'])
        require(hashlib.sha256(body).hexdigest() == p['sha256'] and len(body) == p['size'] and
            'sha512-' + base64.b64encode(hashlib.sha512(body).digest()).decode() == p['integrity'], 'tarball pin')
    require(native['tools'] == pack['tools'], 'native tool identity')
    for name in ('node', 'npm', 'go', 'stager_node'):
        tool = pack['tools'][name]; key = 'node' if name == 'stager_node' else name
        require(tool['path'] == run['tools'][key]['path'] and tool['sha256'] == run['tools'][key]['sha256'], 'stager tool hash')
        require(tool['version'] == ('go version go1.25.13 linux/amd64' if key == 'go' else VERSIONS[key]), 'stager tool version')
    require(candidate['build']['go_version'] == VERSIONS['go'] and candidate['build']['authoring_mode'] == MODE and
        candidate['build']['go_sha256'] == run['tools']['go']['sha256'], 'candidate build identity')
    invocations = read(root / 'native/invocations.json')
    invocation_rows(invocations, native['invocations'])
    require(cfg['stage']['repo'] == str(repo), 'wrong terminal checkout')
    require(native['inventory_sha256'] == digest(repo / 'cli/plugin-kit-ai/cmd/plugin-kit-ai/release_compat.go'), 'same-SHA inventory')
    request = sealed['request']
    require(request == read(root / 'bridge-config/request.json') and request['expectedCommit'] == sha and
        request['nativeConfigSha256'] == digest(root / 'npm-native.json') and
        request['nativeCompletionSha256'] == digest(root / 'native/native-completion.json'), 'native seal pins')
    require(sealed['verifier_sha256'] == digest(repo / 'npm/agentplugins/scripts/packed-installer-bridge.js') and
        sealed['helper_sha256'] == digest(repo / 'npm/agentplugins/scripts/dual-authoring-candidate.js'), 'verifier pins')
    require(log('seal').strip() == result['config_sha256'] == digest(root / 'bridge-config/sealed.json') and result['commit'] == sha, 'planner pin')
    require(result['inputs'] == sealed['inputs'] == json.loads(log('post-verify')), 'post-planner seal verification')
    require(Counter((p['product'], p['lane']) for p in result['inputs']['projects']) == Counter({(p, l): 1 for p in PRODUCTS for l in LANES}), 'ten project identities')
    require(len({p['source'] for p in result['inputs']['projects']}) == 10, 'duplicate project')
    journeys = re.findall(r'^# native journey evidence: (.+)$', log('candidate-native'), re.M)
    require(len(journeys) == 1, 'candidate terminal journey missing')
    journey = read(Path(journeys[0])); false_claims(journey, ('platform_acceptance', 'attested'))
    require(journey['identity'] == identity and journey['manifest_sha256'] == candidate_pin and len(journey['trees']) == 2 and all(len(t) == 5 for t in journey['trees']), 'candidate journey identity/inventory')
    plans(result)
    require(log('head').strip() == sha and log('clean') == log('terminal-clean') == '', 'clean checkout proof')


def public_tap(text, request):
    tap(text, [PUBLIC_TEST])
    markers = re.findall(r'^# public native completion: (.+)$', text, re.M)
    require(len(markers) == 1, 'one public terminal TAP binding required')
    cfg = read(request['nativeConfig'])
    require(json.loads(markers[0]) == dict(file=str(Path(cfg['evidenceOutput']) / 'public-native-completion.json'),
        sha256=request['nativeCompletionSha256'], source=request['expectedCommit']), 'public TAP terminal identity')


def unchanged_snapshots(inputs):
    require(inputs['snapshots'], 'missing sealed inventories')
    for snapshot in inputs['snapshots']:
        root = Path(snapshot['root']); expected = snapshot['entries']
        encoded = (json.dumps(expected, indent=2, ensure_ascii=False) + '\n').encode()
        require(hashlib.sha256(encoded).hexdigest() == snapshot['sha256'], 'inventory digest')
        actual = []
        def walk(file, relative):
            st = file.lstat(); require(not st.st_mode & 0o7000, 'special inventory mode')
            row = dict(path=relative, mode=stat.S_IMODE(st.st_mode), kind='directory' if stat.S_ISDIR(st.st_mode) else 'file')
            if stat.S_ISLNK(st.st_mode):
                require(file.resolve(strict=True).is_relative_to(root) and file.resolve() != root, 'inventory link escape')
                row.update(kind='symlink', target=str(file.readlink()))
            elif not stat.S_ISDIR(st.st_mode):
                body = data(file); row.update(size=len(body), sha256=hashlib.sha256(body).hexdigest())
            actual.append(row)
            if stat.S_ISDIR(st.st_mode):
                for child in sorted(file.iterdir()): walk(child, child.name if relative == '.' else relative + '/' + child.name)
        require(root.resolve() == root, 'aliased inventory root')
        walk(root, '.')
        require(actual == expected, 'sealed tree changed')


def public_boundary(native, intake, require_valid_add=False):
    require(intake in ('public-fixture/v1', 'public-fixture/v2'), 'explicit public intake')
    v2 = intake == 'public-fixture/v2'
    require(native['schema'] == 'dual-authoring-public-native/' + ('v2' if v2 else 'v1'), 'public schema substitution')
    if v2:
        expected = dict(executable_observation='help-and-preflight-rejection', valid_add_dry_run='not_evaluated',
            argv=['add', str(Path(native['projects']['agentplugins']) / 'skill'), '--target=codex', '--dry-run', '--format=json'],
            reason='production-security-inputs-not-offline')
        require(native.get('installer_boundary') == expected, 'outstanding production add requirement')
    else:
        require('installer_boundary' not in native, 'v2 boundary in v1')
    require(not require_valid_add, 'insufficient evidence for successful production add: separate installer acceptance required')
    return native.get('installer_boundary')


def check_public(root, sha, require_valid_add=False, require_summary=True):
    run = read(root / 'public-run.json'); false_claims(run)
    require(run['schema'] == 'public-packed-run/v1' and run['head'] == sha and re.fullmatch('[0-9a-f]{40}', sha), 'public run identity')
    options = run['options']; request = options['request']
    require(request['intake'] in ('public-fixture/v1', 'public-fixture/v2') and request['expectedCommit'] == sha, 'public request identity')
    require(digest(options['nativeTap']) == options['nativeTapSha256'], 'changed public TAP')
    public_tap(data(options['nativeTap']).decode(), request)
    require(digest(request['nativeConfig']) == request['nativeConfigSha256'], 'changed public config')
    cfg = read(request['nativeConfig'])
    terminal_path = Path(cfg['evidenceOutput']) / 'public-native-completion.json'
    require(digest(terminal_path) == request['nativeCompletionSha256'], 'changed public terminal')
    native = read(terminal_path); false_claims(native, (*CLAIMS, 'signed_promotion', 'public_eligible'))
    boundary = public_boundary(native, request['intake'], require_valid_add)
    require(native['status'] == 'completed' and
        native['qualification'] is None and native['identity']['commit'] == native['identity']['engine_revision'] == sha, 'public terminal contract')
    require(digest(Path(cfg['evidenceOutput']) / 'invocations.json') == native['invocations_sha256'], 'public invocation pin')
    sealed_path = root / 'bridge-config/sealed.json'; sealed = read(sealed_path); false_claims(sealed)
    require(sealed['schema'] == 'packed-installer-bridge/v1' and sealed['request'] == request == read(root / 'bridge-config/request.json'), 'public sealed request')
    evidence = sealed['inputs'].get('public_evidence', {})
    if boundary is not None:
        require(evidence.get('schema') == native['schema'] and evidence.get('installer_boundary') == boundary, 'sealed installer boundary')
    else:
        require('installer_boundary' not in evidence, 'v2 sealed boundary in v1')
    repo = Path(__file__).resolve().parent.parent; bridge = repo / 'npm/agentplugins/scripts/packed-installer-bridge.js'
    require(sealed['verifier_sha256'] == digest(bridge) and sealed['helper_sha256'] == digest(bridge.with_name('dual-authoring-candidate.js')), 'public verifier changed')
    for key in ('go', 'node'):
        tool = run['tools'][key]
        require(tool['path'] == options[key] and tool['sha256'] == digest(tool['path']) == native['tools'][key]['sha256'], 'public tool changed')
    node, go = [run['tools'][k]['path'] for k in ('node', 'go')]
    commands = {'head': ['/usr/bin/git', 'rev-parse', 'HEAD'],
        'clean': ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all'],
        'terminal-clean': ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all'],
        'seal': [node, str(bridge), 'seal', str(root / 'bridge-config/request.json'), str(sealed_path)],
        'post-verify': [node, str(bridge), 'verify', str(sealed_path), digest(sealed_path), sha]}
    for name, flag in [('discovery', '-list'), ('planner', '-run')]:
        commands[name] = [go, 'test', '-p=2', '-tags=packedci', '-json',
            *([] if name == 'discovery' else ['-count=1', '-timeout=20m']), flag, REGEX, PACKAGE_PATH]
    for name, command in commands.items():
        phase = read(root / 'logs' / (name + '.json'))
        require(type(phase['exit']) is int and phase['exit'] == 0 and phase['argv'] == command and phase['cwd'] == str(repo), 'public phase: ' + name)
        require(all(phase['env'].get(k) == v for k, v in dict(GOPROXY='off', GOSUMDB='off', GOVCS='*:off', GOENV='off', GOTOOLCHAIN='local').items()), 'public offline environment')
        require(not any(k in phase['env'] for k in ('GOFLAGS', 'NODE_OPTIONS', 'AGENTPLUGINS_STAGED_TEST_CHILD')), 'inherited public control')
        data(root / 'logs' / (name + '.stdout')); data(root / 'logs' / (name + '.stderr'))
        if name in ('discovery', 'planner'):
            require(all(phase['env'].get(k) == v for k, v in dict(UAP_PACKED_INSTALLER_NODE=node,
                UAP_PACKED_INSTALLER_CONFIG=str(sealed_path), UAP_PACKED_INSTALLER_CONFIG_SHA256=digest(sealed_path),
                UAP_PACKED_INSTALLER_COMMIT=sha, UAP_PACKED_INSTALLER_OUTPUT=str(root / 'results/completion.json')).items()), 'public planner opt-in')
    log = lambda name: data(root / 'logs' / (name + '.stdout')).decode()
    go_discovery(log('discovery')); go_results(log('planner'))
    result = read(root / 'results/completion.json'); false_claims(result)
    require(result['kind'] == 'packed-generated-existing-injected-installer-planner' and result['commit'] == sha, 'public planner completion')
    require(result['config_sha256'] == digest(sealed_path) == log('seal').strip(), 'public planner seal pin')
    require(result['inputs'] == sealed['inputs'] == json.loads(log('post-verify')), 'public preservation')
    unchanged_snapshots(result['inputs'])
    projects = result['inputs']['projects']
    require(Counter((p['product'], p['lane']) for p in projects) == Counter({(p, l): 1 for p in PRODUCTS for l in LANES}) and
        len({p['source'] for p in projects}) == 10, 'public ten distinct projects')
    plans(result)
    require(log('head').strip() == sha and log('clean') == log('terminal-clean') == '', 'public exact clean checkout')
    if boundary is not None and require_summary:
        summary = read(root / 'summary.json'); false_claims(summary, (*CLAIMS, 'signed_promotion', 'public_eligible'))
        require(summary == dict(status='passed', intake=request['intake'], head=sha, projects=10, plans=30,
            scope='public-authoring-help-preflight-and-injected-planner', installer_boundary=boundary,
            release_eligible=False, platform_acceptance=False, attested=False, signed_promotion=False, public_eligible=False), 'public summary scope/gap')


AUTHENTIC = 'public-authenticated/v1'
AUTHENTIC_SEAL = 'packed-installer-bridge/public-authenticated/v1'


def authenticated_options(options, sha):
    require(type(options) is dict and set(options) == {'request', 'go', 'node', 'modCache'}, 'closed authenticated options')
    request = options['request']
    require(type(request) is dict and set(request) == {'intake', 'expectedCommit', 'journey', 'journeySha256',
        'admission', 'admissionSha256', 'fixtureRoot'}, 'closed authenticated request')
    require(request['intake'] == AUTHENTIC and request['expectedCommit'] == sha and
        re.fullmatch('[0-9a-f]{40}', sha) and sha != '0' * 40, 'authentic source identity')
    for key in ('journeySha256', 'admissionSha256'):
        require(type(request[key]) is str and re.fullmatch('[0-9a-f]{64}', request[key]) and request[key] != '0' * 64, 'authentic pin')
    for name in ('go', 'node', 'modCache'):
        file = Path(options[name])
        require(file.is_absolute() and file.resolve(strict=True) == file, 'canonical authenticated tool/cache')
    for key in ('journey', 'admission'):
        require(Path(request[key]).lstat().st_size <= 1024 * 1024, 'bounded authentic input')
        require(digest(request[key]) == request[key + 'Sha256'], 'changed authentic input')
    return request


def authentic_read(path):
    require(Path(path).lstat().st_size <= 1024 * 1024, 'bounded authentic JSON')
    value = read(path)
    require(data(path) == (json.dumps(value, indent=2, ensure_ascii=False) + '\n').encode(), 'canonical authentic JSON')
    return value


PROVISION_TARGETS = ('linux-amd64', 'linux-arm64', 'darwin-amd64', 'darwin-arm64', 'windows-amd64', 'windows-arm64')
PROVISION_CELLS = tuple(t + '/' + lane for t in PROVISION_TARGETS for lane in ('kit-node18', 'pair-node22', 'pair-node24'))
PROVISION_TOOLS = ('node', 'python', 'git', 'gh', 'tar')
PROVISION_FIELDS = ('runner', 'image', 'controller', 'npm_node', 'shim_node', 'npm', 'go', 'mod_cache', 'observer', 'installer_policy')


def provision_fields(value, names, label='manifest'):
    if type(value) is dict:
        for name in names:
            missing = name + ':entry' if label in ('controllers', 'cells') else label + ':' + name
            require(name in value, 'PUBLIC_PROVISIONING_REQUIRED:' + missing)
    require(type(value) is dict and list(value) == list(names), 'closed ordered provision fields')


def provision_bytes(file, maximum=1024 * 1024):
    import os
    file = Path(file)
    require(str(file) == os.path.abspath(file) and file.resolve() == file, 'canonical provision path')
    before = file.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and 0 < before.st_size <= maximum,
            'bounded regular unaliased provision file')
    with open(file, 'rb') as stream:
        opened = os.fstat(stream.fileno())
        require((opened.st_dev, opened.st_ino) == (before.st_dev, before.st_ino), 'provision file changed')
        body = stream.read(maximum + 1)
        after = os.fstat(stream.fileno())
    # Reading may update atime (e.g. relatime on a fresh checkout). Compare
    # identity and mutation metadata explicitly, retaining nanosecond precision.
    def identity(st):
        return (st.st_dev, st.st_ino, st.st_mode, st.st_nlink, st.st_uid, st.st_gid,
                st.st_size, st.st_mtime_ns, st.st_ctime_ns)
    require(identity(before) == identity(opened) == identity(after) == identity(file.lstat()) and
            len(body) == before.st_size, 'provision file changed')
    return body


def read_provisioning():
    # No caller root, expectedCommit, receipt, PATH or environment selection.
    file = Path(__file__).absolute().parent.parent / '.github/authoring-public-tools.json'
    try: body = provision_bytes(file)
    except FileNotFoundError: raise ValueError('PUBLIC_PROVISIONING_REQUIRED:linux-amd64:manifest') from None
    text = body.decode('utf-8'); depth = 0; quoted = escaped = False
    for ch in text:
        if quoted:
            if escaped: escaped = False
            elif ch == '\\': escaped = True
            elif ch == '"': quoted = False
        elif ch == '"': quoted = True
        elif ch in '{[':
            depth += 1; require(depth <= 8, 'provision depth')
        elif ch in '}]': depth -= 1
    value = json.loads(text)
    require(body == (json.dumps(value, indent=2, ensure_ascii=False) + '\n').encode(), 'canonical provision JSON')
    def pin(v): require(type(v) is str and re.fullmatch('[0-9a-f]{64}', v) and v != '0' * 64, 'provision pin')
    def label(v): require(type(v) is str and re.fullmatch('[!-~]{1,256}', v), 'bounded provision identity')
    # Absolute path limit: 4096 Unicode code points, shared with JS.
    def absolute(v, target):
        import ntpath, posixpath
        p = ntpath if target.startswith('windows-') else posixpath
        require(type(v) is str and 0 < len(v) <= 4096 and not re.search('[\x00-\x1f\x7f]', v) and
                p.isabs(v) and p.normpath(v) == v and v not in ('/', p.splitdrive(v)[0] + '\\') and
                not v.startswith(('\\\\', '//')), 'canonical provision path')
    def closure(v, target):
        provision_fields(v, ('root', 'files')); absolute(v['root'], target)
        require(type(v['files']) is list and 0 < len(v['files']) <= 4096, 'bounded provision closure')
        last = ''
        for row in v['files']:
            provision_fields(row, ('path', 'sha256')); pin(row['sha256']); name = row['path']
            require(type(name) is str and len(name) <= 4096 and re.fullmatch(r'[A-Za-z0-9_.@+-]+(?:/[A-Za-z0-9_.@+-]+)*', name) and
                    not set(name.split('/')) & {'.', '..'} and name > last, 'ordered unique relative closure files')
            last = name
    def tool(v, target, npm=False):
        provision_fields(v, ('path', 'version', 'sha256', 'closure') if npm else ('path', 'version', 'sha256'))
        absolute(v['path'], target); label(v['version']); pin(v['sha256'])
        if npm:
            import ntpath, posixpath
            closure(v['closure'], target); p = ntpath if target.startswith('windows-') else posixpath
            require(any(p.join(v['closure']['root'], *f['path'].split('/')) == v['path'] and f['sha256'] == v['sha256']
                        for f in v['closure']['files']), 'npm CLI in complete closure')
    provision_fields(value, ('schema', 'controllers', 'cells', 'reader'))
    require(value['schema'] == 'authoring-public-tools/v1' and value['reader'] == 'linux-amd64', 'fixed provision reader/schema')
    provision_fields(value['controllers'], PROVISION_TARGETS, 'controllers'); provision_fields(value['cells'], PROVISION_CELLS, 'cells')
    for target, row in value['controllers'].items():
        provision_fields(row, PROVISION_TOOLS, target)
        for v in row.values():
            if v is not None: tool(v, target)
    for key, row in value['cells'].items():
        target = key.split('/')[0]; provision_fields(row, PROVISION_FIELDS, key)
        require(row['controller'] == target, 'fixed cell controller')
        for name in ('runner', 'image', 'observer', 'installer_policy'):
            if row[name] is not None:
                provision_fields(row[name], ('id', 'sha256')); label(row[name]['id']); pin(row[name]['sha256'])
        for name in ('npm_node', 'shim_node', 'npm', 'go'):
            if row[name] is not None:
                tool(row[name], target, name == 'npm')
                if name.endswith('_node'):
                    require(re.fullmatch('v' + key.split('node')[1] + r'\.[0-9]+\.[0-9]+', row[name]['version']), 'selected Node major')
        if row['mod_cache'] is not None: closure(row['mod_cache'], target)
    return value


def require_authenticated_controller():
    value = read_provisioning()
    for name, tool in value['controllers']['linux-amd64'].items():
        message = 'PUBLIC_PROVISIONING_REQUIRED:linux-amd64:' + name
        require(tool is not None, message)
        try: body = provision_bytes(tool['path'], 256 * 1024 * 1024)
        except FileNotFoundError: raise ValueError(message) from None
        require(hashlib.sha256(body).hexdigest() == tool['sha256'], 'source-frozen provision pin mismatch:' + name)
    return value['controllers']['linux-amd64']['node']['path']


def require_authenticated_execution():
    raise ValueError('C3b execution incomplete: result/installer/observer validators and independent invocation authority required')


def authenticated_source():
    # Snapshot trusted source, never receipt-selected source. External provisioning
    # keeps this namespace immutable; before/after hashing is not same-UID isolation.
    repo = Path(__file__).absolute().parent.parent
    files = []
    for directory in ('.github', 'scripts', 'npm/agentplugins/scripts', 'npm/agentplugins/lib', 'npm/plugin-kit-ai/lib'):
        def walk(folder):
            require(folder.resolve() == folder and stat.S_ISDIR(folder.lstat().st_mode), 'trusted source directory')
            for file in sorted(folder.iterdir()):
                require(not file.is_symlink(), 'trusted source link')
                if file.is_dir(): walk(file)
                else: files.append(file)
                require(len(files) <= 4096, 'trusted source closure bound')
        walk(repo / directory)
    return {str(f): hashlib.sha256(provision_bytes(f, 16 * 1024 * 1024)).hexdigest() for f in files}


def authenticated_verify(node, argv):
    controller = require_authenticated_controller()
    require(str(node) == controller, 'source-frozen controller comparison mismatch')
    require_authenticated_execution()
    import subprocess
    repo = Path(__file__).absolute().parent.parent
    bridge = repo / 'npm/agentplugins/scripts/packed-installer-bridge.js'
    source = authenticated_source()
    require(require_authenticated_controller() == controller and authenticated_source() == source, 'trusted source/controller changed')
    try:
        result = subprocess.run([controller, str(bridge), *map(str, argv)], cwd=repo,
            env={'PATH': '/usr/local/bin:/usr/bin:/bin', 'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8'},
            capture_output=True, timeout=1200)
    finally:
        require(require_authenticated_controller() == controller and authenticated_source() == source, 'trusted source/controller changed')
    require(result.returncode == 0 and result.stderr == b'', 'authenticated reader failed: ' + result.stderr.decode(errors='replace')[:4096])
    require(len(result.stdout) <= 32 * 1024 * 1024, 'bounded authenticated reader output')
    return json.loads(result.stdout)

def authenticated_plans(root, sha, inputs, sealed_pin, fixture_root):
    """Complementary injected plans only; does not authenticate J or remote E."""
    result = read(root / 'results/completion.json'); false_claims(result)
    require(result['kind'] == 'packed-generated-existing-injected-installer-planner' and result['commit'] == sha and
        result['config_sha256'] == sealed_pin, 'authentic planner identity')
    require(result['inputs'] == inputs == json.loads(data(root / 'logs/post-verify.stdout')), 'authentic post-plan seal')
    projects = inputs['projects']
    require(Counter((x['product'], x['lane']) for x in projects) == Counter({(p, l): 1 for p in PRODUCTS for l in LANES}) and
        len({x['source'] for x in projects}) == 10, 'same ten authentic projects')
    for row in projects:
        require(row['source'] == str(Path(fixture_root) / (row['product'] + ' projects ü') / row['lane']), 'original project path')
    unchanged_snapshots(inputs); plans(result)


def check_authenticated(root, sha, require_summary=True, require_completed_e=False):
    require(not require_completed_e, 'completed E cannot use local J or fixture success; C3b E reader required')
    controller = require_authenticated_controller()
    require_authenticated_execution()
    run = authentic_read(root / 'authenticated-run.json'); false_claims(run)
    require(set(run) == {'schema', 'head', 'options', 'tools', *CLAIMS} and
        run['schema'] == 'public-authenticated-packed-run/v1' and run['head'] == sha, 'authentic run schema')
    options = run['options']; require(options['node'] == controller, 'source-frozen controller comparison mismatch')
    request = authenticated_options(options, sha)
    sealed_path = root / 'bridge-config/sealed.json'; sealed = read(sealed_path); false_claims(sealed)
    require(set(sealed) == {'schema', 'request', 'verifier_sha256', 'helper_sha256', 'reader_sha256', 'inputs', *CLAIMS} and
        sealed['schema'] == AUTHENTIC_SEAL and sealed['request'] == request == read(root / 'bridge-config/request.json'), 'authentic seal schema')
    repo = Path(__file__).resolve().parent.parent; bridge = repo / 'npm/agentplugins/scripts/packed-installer-bridge.js'
    for key, file in [('verifier_sha256', bridge), ('helper_sha256', bridge.with_name('dual-authoring-candidate.js')),
        ('reader_sha256', bridge.with_name('public-authoring-acceptance.js'))]:
        require(sealed[key] == digest(file), 'authentic verifier pin')
    for key in ('go', 'node'):
        require(run['tools'][key] == dict(path=options[key], sha256=digest(options[key])), 'authentic tool pin')
    node, go = options['node'], options['go']; sealed_pin = digest(sealed_path)
    fresh = authenticated_verify(node, ['verify', sealed_path, sealed_pin, sha])
    require(fresh == sealed['inputs'], 'fresh authenticated seal')
    public = fresh['public_inputs']
    require(public['schema'] == 'authoring-public-local-inputs/v1' and public['cell'] == 'linux-amd64/pair-node22' and
        public['journey_sha256'] == request['journeySha256'] and public['admission_sha256'] == request['admissionSha256'], 'local custody boundary')
    false_claims(public, ('signed_promotion', 'public_eligible')); require(public['qualification'] is None, 'no local qualification')
    for key, tool in [('node', 'orchestrator_node'), ('go', 'go')]:
        require(public['tools'][tool]['path'] == options[key] and public['tools'][tool]['sha256'] == digest(options[key]), 'journey planner tools')
    commands = {'head': ['/usr/bin/git', 'rev-parse', 'HEAD'],
        'clean': ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all'],
        'terminal-clean': ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all'],
        'seal': [node, str(bridge), 'authenticated-seal', str(root / 'bridge-config/request.json'), str(sealed_path)],
        'post-verify': [node, str(bridge), 'verify', str(sealed_path), sealed_pin, sha]}
    for name, flag in [('discovery', '-list'), ('planner', '-run')]:
        commands[name] = [go, 'test', '-p=2', '-tags=packedci', '-json',
            *([] if name == 'discovery' else ['-count=1', '-timeout=20m']), flag, REGEX, PACKAGE_PATH]
    for name, command in commands.items():
        phase = read(root / 'logs' / (name + '.json'))
        require(type(phase['exit']) is int and phase['exit'] == 0 and phase['argv'] == command and phase['cwd'] == str(repo), 'authentic phase ' + name)
        require(all(phase['env'].get(k) == v for k, v in dict(GOPROXY='off', GOSUMDB='off', GOVCS='*:off', GOENV='off', GOTOOLCHAIN='local').items()), 'authentic offline planner environment')
        require(not any(k in phase['env'] for k in ('GOFLAGS', 'NODE_OPTIONS', 'AGENTPLUGINS_STAGED_TEST_CHILD')), 'authentic inherited override')
        if name in ('discovery', 'planner'):
            expected = dict(UAP_PACKED_INSTALLER_NODE=node, UAP_PACKED_INSTALLER_CONFIG=str(sealed_path),
                UAP_PACKED_INSTALLER_CONFIG_SHA256=sealed_pin, UAP_PACKED_INSTALLER_COMMIT=sha,
                UAP_PACKED_INSTALLER_OUTPUT=str(root / 'results/completion.json'))
            require({k: v for k, v in phase['env'].items() if k.startswith('UAP_PACKED_INSTALLER_')} == expected, 'five exact planner variables')
        require(data(root / 'logs' / (name + '.stderr')) == b'', 'authentic phase stderr')
    log = lambda name: data(root / 'logs' / (name + '.stdout')).decode()
    go_discovery(log('discovery')); go_results(log('planner'))
    require(log('seal').strip() == sealed_pin and log('head').strip() == sha and log('clean') == log('terminal-clean') == '', 'authentic source/seal logs')
    authenticated_plans(root, sha, fresh, sealed_pin, request['fixtureRoot'])
    if require_summary:
        require(authentic_read(root / 'summary.json') == dict(status='passed', scope='local-authenticated-inputs-and-injected-planner',
            intake=AUTHENTIC, head=sha, projects=10, plans=30, release_eligible=False, platform_acceptance=False,
            attested=False, signed_promotion=False, public_eligible=False, qualification=None), 'authentic summary scope')


if __name__ == '__main__':
    if len(sys.argv) == 4 and sys.argv[1] == '--public-authenticated':
        check_authenticated(Path(sys.argv[2]), sys.argv[3])
    elif len(sys.argv) == 4 and sys.argv[1] == '--public':
        check_public(Path(sys.argv[2]), sys.argv[3])
    elif len(sys.argv) == 3:
        check(Path(sys.argv[1]), sys.argv[2])
    else:
        raise SystemExit('usage: check-packed-ci.py ROOT SHA | --public ROOT SHA')
