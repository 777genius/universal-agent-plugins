"""Synthetic controls only; no product execution and no packed acceptance claim."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parent.parent

def module(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / 'scripts' / (name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result)
    return result

p = module('check-packed-ci')
w = module('check-packed-workflow')
r = module('run-packed-ci')


def tap(names):
    return 'TAP version 13\n' + ''.join(f'# Subtest: {n}\nok {i} - {n}\n' for i, n in enumerate(names, 1)) + \
        f'1..{len(names)}\n# tests {len(names)}\n# suites 0\n# pass {len(names)}\n# fail 0\n# cancelled 0\n# skipped 0\n# todo 0\n# duration_ms 1\n'


def go(rows):
    return ''.join(json.dumps(dict(Package=p.PACKAGE, **row)) + '\n' for row in rows)


def plan_fixture():
    rows = []
    for product in p.PRODUCTS:
        for lane in p.LANES:
            components = [dict(kind='skill', name='extra-skill', support='native')]
            if lane == 'skill' or lane.startswith('hybrid-'):
                components.append(dict(kind='skill', name=lane, support='native'))
            if lane != 'skill': components.append(dict(kind='mcp_server', name=lane, support='native'))
            for target in p.TARGETS:
                plan = dict(client_id=target, scope='user', status='ready' if target == 'claude' else 'manual_activation_required', components=copy.deepcopy(components))
                rows.append(dict(product=product, lane=lane, target=target,
                    report=dict(result='success', data=dict(dry_run=True, plan=plan))))
    return dict(plans=rows)


def invocation_fixture():
    rows = [dict(product='plugin-kit-ai', argv=[str(i)], status=i % 3, signal=None, stdout='', stderr='') for i in range(739)]
    index = 0
    for command in ('publish', 'publication', 'publication doctor'):
        for tail in (['--format=credential-fixture'], ['--format', 'credential-fixture']):
            argv = ['--format=json', *command.split(), *tail]
            for args in (argv, argv + ['--format=json']):
                rows[index].update(argv=args, status=2); index += 1
    return rows + copy.deepcopy(rows[-2:])


class EvidenceControls(unittest.TestCase):
    def test_tap_named_and_terminal(self):
        for names in ([p.CANDIDATE_TEST], p.NATIVE_TESTS):
            valid = tap(names); p.tap(valid, names)
            bad = [valid[:-1], valid.replace('ok 1 -', 'not ok 1 -'), valid.replace(names[0], 'renamed'),
                valid.replace('# skipped 0', '# skipped 1'), valid.replace('# cancelled 0', '# cancelled 1'),
                valid.replace('1..' + str(len(names)), ''), valid.replace('# pass ' + str(len(names)), ''),
                valid.replace('ok 1 - ' + names[0], 'ok 1 - ' + names[0] + ' # SKIP'), tap(names[:0])]
            for text in bad:
                with self.subTest(text=text), self.assertRaises(ValueError): p.tap(text, names)

    def test_exact_go_discovery(self):
        rows = [dict(Action='output', Output=p.NAME + '\n'), dict(Action='pass')]
        p.go_discovery(go(rows))
        for bad in (rows[1:], rows + rows[:1], [dict(Action='output', Output='TestRenamed\n'), rows[1]],
                    [rows[0], dict(Action='skip')]):
            with self.assertRaises(ValueError): p.go_discovery(go(bad))
        with self.assertRaises(ValueError): p.go_discovery(go(rows).replace(p.PACKAGE, 'wrong/package'))
        with self.assertRaises(ValueError): p.go_discovery(go(rows)[:-1])

    def test_exact_ten_leaves_with_group_events(self):
        names = [p.NAME] + [p.NAME + '/' + x for x in p.PRODUCTS] + [p.NAME + '/' + x + '/' + l for x in p.PRODUCTS for l in p.LANES]
        rows = [dict(Action=a, Test=n) for n in names for a in ('run', 'pass')] + [dict(Action='pass')]
        p.go_results(go(rows))
        p.go_results(go([e for e in rows if e.get("Test") not in {p.NAME + "/" + x for x in p.PRODUCTS}]))
        for bad in (rows[-1:], rows[1:], rows + rows[:2], [e for e in rows if e.get('Test') != names[-1]],
                    rows + [dict(Action='skip', Test=names[-1])], rows + [dict(Action='fail')]):
            with self.assertRaises(ValueError): p.go_results(go(bad))

    def test_exact_plan_tuple_and_components(self):
        valid = plan_fixture(); p.plans(valid)
        for mutate in (lambda x: x['plans'].pop(), lambda x: x['plans'].append(x['plans'][0]),
            lambda x: x['plans'].__setitem__(1, x['plans'][0]),
            lambda x: x['plans'][0].update(target='wrong'),
            lambda x: x['plans'][0]['report']['data'].update(dry_run=False),
            lambda x: x['plans'][0]['report']['data']['plan'].update(status='ready'),
            lambda x: x['plans'][0]['report']['data']['plan']['components'].pop(0),
            lambda x: x['plans'][0]['report']['data']['plan']['components'][0].update(support='unsupported')):
            bad = copy.deepcopy(valid); mutate(bad)
            with self.assertRaises(ValueError): p.plans(bad)

    def test_invocation_counts_duplicates_signals_and_negative_statuses(self):
        rows = invocation_fixture()
        p.invocation_rows(rows, 741)
        for bad, count in ((rows[:-1], 740), (rows + rows[:1], 742), (rows, 740), ([rows[0]] * 741, 741)):
            with self.assertRaises(ValueError): p.invocation_rows(bad, count)
        for key, value in (('argv', ['removed-original-format']), ('signal', 'SIGTERM'), ('status', -9), ('status', True), ('stderr', 'failure')):
            bad = copy.deepcopy(rows); bad[0][key] = value
            with self.assertRaises(ValueError): p.invocation_rows(bad, 741)

    def test_claims_missing_or_true(self):
        good = dict.fromkeys(p.CLAIMS, False); p.false_claims(good)
        for key in p.CLAIMS:
            for value in (True, None, 0, ''):
                with self.assertRaises(ValueError): p.false_claims(dict(good, **{key: value}))
            bad = dict(good); del bad[key]
            with self.assertRaises(ValueError): p.false_claims(bad)

    def test_missing_malformed_linked_and_stale_files(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp); file = root / 'record.json'
            with self.assertRaises(OSError): p.read(file)
            file.write_text('{"x":1,"x":2}')
            with self.assertRaises(ValueError): p.read(file)
            file.write_text('{}'); p.read(file)
            with self.assertRaises(FileExistsError): r.write(file, {})
            os.symlink(file, root / 'symbolic')
            with self.assertRaises(ValueError): p.read(root / 'symbolic')
            os.link(file, root / 'hard')
            with self.assertRaises(ValueError): p.read(file)
            with self.assertRaises((ValueError, OSError)): p.check(root, 'a' * 40)

    def test_private_environment_allowlist(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            env = r.private_env(root, Path('/tools/go'), Path('/tools/node'), ROOT, root / 'modules')
            for key in ('GOFLAGS', 'NODE_OPTIONS', 'NODE_PATH', 'AGENTPLUGINS_STAGED_TEST_CHILD', 'GITHUB_TOKEN', 'NPM_TOKEN'):
                self.assertNotIn(key, env)
            for key in ('HOME', 'USERPROFILE', 'APPDATA', 'TMPDIR', 'XDG_CONFIG_HOME'):
                self.assertTrue(Path(env[key]).is_relative_to(root))


class TerminalControls(unittest.TestCase):
    def fixture(self):
        temp = tempfile.TemporaryDirectory(); self.addCleanup(temp.cleanup)
        root = Path(temp.name); sha = 'a' * 40
        def put(name, value):
            path = root / name; path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(json.dumps(value) if not isinstance(value, str) else value)
        def get(name): return json.loads((root / name).read_text())
        claims = dict.fromkeys(p.CLAIMS, False)
        identity = dict(repository='777genius/universal-agent-plugins', commit=sha, engine_revision=sha,
                        versions={'agentplugins': '0.1.91', 'plugin-kit-ai': '2.0.0'})
        tools = {}
        for key in p.VERSIONS:
            put('tools/' + key, 'synthetic inert tool ' + key)
            tools[key] = dict(path=str(root / 'tools' / key), sha256=p.digest(root / 'tools' / key))
        put('run.json', dict(head=sha, identity=identity, versions=p.VERSIONS, tools=tools, run_id='1', attempt='1', **claims))
        candidate = dict(schema='dual-authoring-candidate/v1', identity=identity, status='CANDIDATE', asset_scope='linux-amd64-pair', release_eligible=False,
            build=dict(go_version=p.VERSIONS['go'], authoring_mode=p.MODE, go_sha256=tools['go']['sha256']))
        put('candidate/candidate.json', candidate); candidate_pin = p.digest(root / 'candidate/candidate.json')
        packs = {}
        for product in p.PRODUCTS:
            file = product + '.tgz'; put('npm-pair/' + file, 'synthetic inert pack')
            body = (root / 'npm-pair' / file).read_bytes()
            packs[product] = dict(file=file, size=len(body), sha256=p.digest(root / 'npm-pair' / file),
                integrity='sha512-' + p.base64.b64encode(p.hashlib.sha512(body).digest()).decode())
        pack_tools = {key: dict(value, version='go version go1.25.13 linux/amd64' if key == 'go' else p.VERSIONS[key]) for key, value in tools.items()}
        pack_tools['stager_node'] = pack_tools['node']
        put('npm-pair/completion.json', dict(schema='dual-authoring-npm-completion/v1', identity=identity, status='CANDIDATE', authoring_mode=p.MODE,
            asset_scope='linux-amd64-pair', candidate_sha256=candidate_pin, packs=packs, tools=pack_tools, **claims))
        completion_pin = p.digest(root / 'npm-pair/completion.json')
        put('npm-native.json', dict(stage=dict(repo=str(ROOT)), completionDigest=completion_pin))
        native = dict(kind='actual-linux-private-npm-pair', identity=identity, candidate_sha256=candidate_pin, completion_sha256=completion_pin,
            packs=packs, tools=pack_tools, invocations=741,
            inventory_sha256=p.digest(ROOT / 'cli/plugin-kit-ai/cmd/plugin-kit-ai/release_compat.go'), **claims)
        put('native/native-completion.json', native)
        put('native/invocations.json', invocation_fixture())
        request = dict(expectedCommit=sha, nativeConfigSha256=p.digest(root / 'npm-native.json'),
                       nativeCompletionSha256=p.digest(root / 'native/native-completion.json'))
        inputs = dict(projects=[dict(product=x, lane=l, source=str(root / x / l)) for x in p.PRODUCTS for l in p.LANES])
        scripts = ROOT / 'npm/agentplugins/scripts'
        sealed = dict(schema='packed-installer-bridge/v1', request=request, inputs=inputs, verifier_sha256=p.digest(scripts / 'packed-installer-bridge.js'),
                      helper_sha256=p.digest(scripts / 'dual-authoring-candidate.js'), **claims)
        put('bridge-config/request.json', request); put('bridge-config/sealed.json', sealed)
        seal_pin = p.digest(root / 'bridge-config/sealed.json')
        put('results/completion.json', dict(plan_fixture(), kind='packed-generated-existing-injected-installer-planner', inputs=inputs, config_sha256=seal_pin, commit=sha, **claims))
        put('journey.json', dict(identity=identity, manifest_sha256=candidate_pin, trees=[[{}]*5]*2, platform_acceptance=False, attested=False))
        names = [p.NAME] + [p.NAME + '/' + x + '/' + l for x in p.PRODUCTS for l in p.LANES]
        transcripts = {'head': sha + '\n', 'clean': '', 'terminal-clean': '',
            'candidate-native': tap([p.CANDIDATE_TEST]).replace('TAP version 13\n', 'TAP version 13\n# native journey evidence: ' + str(root / 'journey.json') + '\n'),
            'npm-native': tap(p.NATIVE_TESTS), 'seal': seal_pin + '\n', 'post-verify': json.dumps(inputs),
            'stage': json.dumps(dict(manifest_sha256=candidate_pin, **claims)),
            'verify': json.dumps(dict(manifest_sha256=candidate_pin, consistency_verified=True, **claims)),
            'discovery': go([dict(Action='output', Output=p.NAME+'\n'), dict(Action='pass')]),
            'planner': go([dict(Action=a, Test=n) for n in names for a in ('run','pass')] + [dict(Action='pass')])}
        for name in ('head', 'clean', 'go-version', 'node-version', 'npm-version', 'go-host', 'warmup', 'stage', 'verify',
                     'candidate-native', 'npm-stage', 'npm-native', 'seal', 'discovery', 'planner', 'post-verify', 'terminal-clean'):
            argv = []; env = dict(GOPROXY='off', GOSUMDB='off', GOENV='off', GOTOOLCHAIN='local', GOVCS='*:off')
            if name in ('discovery', 'planner'):
                argv = [tools['go']['path'], 'test', '-p=2', '-tags=packedci', '-json',
                    *([] if name == 'discovery' else ['-count=1','-timeout=20m']), '-list' if name == 'discovery' else '-run', p.REGEX, p.PACKAGE_PATH]
            if name in ('candidate-native', 'npm-native'):
                argv = [tools['node']['path'], '--test', '--test-reporter=tap', 'npm/agentplugins/test/' + ('dual-authoring-candidate-native.test.js' if name == 'candidate-native' else 'private-npm-native.test.js')]
                env.update(dict(UAP_CANDIDATE_NATIVE_CONFIG=str(root / 'verify.json'), UAP_CANDIDATE_NATIVE_SOURCE_REPO=str(ROOT)) if name == 'candidate-native' else dict(UAP_PRIVATE_NPM_NATIVE_CONFIG=str(root / 'npm-native.json')))
            commands = {
                'stage': ('stage-dual-authoring-candidate.js', ['stage', '--candidate', str(root / 'stage.json')]),
                'verify': ('stage-dual-authoring-candidate.js', ['verify', '--candidate', str(root / 'verify.json')]),
                'npm-stage': ('stage-dual-authoring-npm.js', ['--candidate', str(root / 'npm-stage.json')]),
                'seal': ('packed-installer-bridge.js', ['seal', str(root / 'bridge-config/request.json'), str(root / 'bridge-config/sealed.json')])}
            if name in commands:
                script, args = commands[name]; argv = [tools['node']['path'], str(scripts / script), *args]
            if name == 'post-verify': argv = [tools['node']['path'], str(scripts / 'packed-installer-bridge.js'), 'verify', str(root / 'bridge-config/sealed.json'), seal_pin, sha]
            put('logs/' + name + '.json', dict(exit=0, argv=argv, env=env, cwd=str(ROOT)))
            put('logs/' + name + '.stdout', transcripts.get(name, ''))
            put('logs/' + name + '.stderr', '')
        return root, sha, put, get

    def test_terminal_chain_and_negative_controls(self):
        root, sha, put, get = self.fixture(); p.check(root, sha)
        changes = [('results/completion.json', 'kind', 'candidate-only'), ('run.json', 'head', 'b'*40), ('run.json', 'versions', dict(p.VERSIONS, npm='12.0.2')),
            ('logs/npm-native.json', 'exit', 1), ('logs/npm-native.json', 'env', {}),
            ('logs/planner.json', 'argv', []), ('native/native-completion.json', 'inventory_sha256', '0'*64),
            ('native/native-completion.json', 'invocations', 740), ('results/completion.json', 'config_sha256', '0'*64),
            ('bridge-config/sealed.json', 'helper_sha256', '0'*64), ('candidate/candidate.json', 'identity', {}),
            ('npm-pair/completion.json', 'candidate_sha256', '0'*64), ('results/completion.json', 'attested', True)]
        for name, key, value in changes:
            with self.subTest(name=name, key=key):
                original = get(name); bad = dict(original); bad[key] = value; put(name, bad)
                with self.assertRaises((ValueError, KeyError)): p.check(root, sha)
                put(name, original)
        (root / 'logs/npm-native.stdout').unlink()
        with self.assertRaises(OSError): p.check(root, sha)


class PublicControls(unittest.TestCase):
    def test_public_terminal_and_matrix_controls(self):
        self.public_terminal_controls('v1')

    def test_public_v2_terminal_and_matrix_controls(self):
        self.public_terminal_controls('v2')

    def public_terminal_controls(self, version):
        root, sha, put, get = TerminalControls.fixture(self)
        # Reuse only the synthetic planner transcripts; no native claim.
        private = get('run.json'); tools = private['tools']; claims = {k: False for k in p.CLAIMS}
        request = dict(intake='public-fixture/' + version, expectedCommit=sha, nativeConfig=str(root / 'public-config.json'))
        put('public-config.json', dict(evidenceOutput=str(root / 'public-native')))
        put('public-native/invocations.json', [])
        native = dict(schema='dual-authoring-public-native/' + version, status='completed', identity=private['identity'],
            qualification=None, tools=tools, invocations_sha256=p.digest(root / 'public-native/invocations.json'),
            signed_promotion=False, public_eligible=False, **claims)
        if version == 'v2':
            native['projects'] = {'agentplugins': '/fixture/agentplugins projects ü'}
            native['installer_boundary'] = dict(executable_observation='help-and-preflight-rejection', valid_add_dry_run='not_evaluated',
                argv=['add', '/fixture/agentplugins projects ü/skill', '--target=codex', '--dry-run', '--format=json'],
                reason='production-security-inputs-not-offline')
            put('summary.json', dict(status='passed', intake=request['intake'], head=sha, projects=10, plans=30,
                scope='public-authoring-help-preflight-and-injected-planner', installer_boundary=native['installer_boundary'],
                signed_promotion=False, public_eligible=False, **claims))
        put('public-native/public-native-completion.json', native)
        request.update(nativeConfigSha256=p.digest(request['nativeConfig']),
            nativeCompletionSha256=p.digest(root / 'public-native/public-native-completion.json'))
        marker = json.dumps(dict(file=str(root / 'public-native/public-native-completion.json'), sha256=request['nativeCompletionSha256'], source=sha))
        put('public.tap', tap([p.PUBLIC_TEST]).replace('TAP version 13\n', 'TAP version 13\n# public native completion: ' + marker + '\n'))
        options = dict(request=request, nativeTap=str(root / 'public.tap'), nativeTapSha256=p.digest(root / 'public.tap'),
            go=tools['go']['path'], node=tools['node']['path'], modCache=str(root / 'unused-modules'))
        put('public-run.json', dict(schema='public-packed-run/v1', head=sha, options=options, tools=tools, **claims))
        sealed = get('bridge-config/sealed.json'); sealed['request'] = request
        if version == 'v2':
            sealed['inputs']['public_evidence'] = dict(schema=native['schema'], installer_boundary=native['installer_boundary'])
        inventory_root = root / 'sealed-tree'; inventory_root.mkdir()
        entries = [dict(path='.', mode=inventory_root.stat().st_mode & 0o777, kind='directory')]
        sealed['inputs']['snapshots'] = [dict(root=str(inventory_root), entries=entries,
            sha256=p.hashlib.sha256((json.dumps(entries, indent=2) + '\n').encode()).hexdigest())]
        put('bridge-config/request.json', request); put('bridge-config/sealed.json', sealed)
        pin = p.digest(root / 'bridge-config/sealed.json')
        result = get('results/completion.json'); result.update(config_sha256=pin, inputs=sealed['inputs']); put('results/completion.json', result)
        put('logs/post-verify.stdout', json.dumps(sealed['inputs']))
        put('logs/seal.stdout', pin + '\n')
        for name in ('head', 'clean', 'terminal-clean', 'post-verify', 'discovery', 'planner'):
            phase = get('logs/' + name + '.json')
            if name == 'head': phase['argv'] = ['/usr/bin/git', 'rev-parse', 'HEAD']
            if name in ('clean', 'terminal-clean'): phase['argv'] = ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all']
            if name == 'post-verify': phase['argv'][-2] = pin
            if name in ('discovery', 'planner'):
                phase['env'].update(UAP_PACKED_INSTALLER_NODE=tools['node']['path'],
                    UAP_PACKED_INSTALLER_CONFIG=str(root / 'bridge-config/sealed.json'), UAP_PACKED_INSTALLER_CONFIG_SHA256=pin,
                    UAP_PACKED_INSTALLER_COMMIT=sha, UAP_PACKED_INSTALLER_OUTPUT=str(root / 'results/completion.json'))
            put('logs/' + name + '.json', phase)
        p.check_public(root, sha)
        if version == 'v2':
            with self.assertRaisesRegex(ValueError, 'insufficient evidence'): p.check_public(root, sha, require_valid_add=True)
            for key, value in [('installer_boundary', None), ('scope', 'successful-production-add')]:
                original = get('summary.json'); put('summary.json', dict(original, **{key: value}))
                with self.assertRaisesRegex(ValueError, 'summary scope/gap'): p.check_public(root, sha)
                put('summary.json', original)
            original = get('bridge-config/sealed.json')
            bad = get('bridge-config/sealed.json'); del bad['inputs']['public_evidence']['installer_boundary']
            put('bridge-config/sealed.json', bad)
            with self.assertRaisesRegex(ValueError, 'sealed installer boundary'): p.check_public(root, sha)
            put('bridge-config/sealed.json', original)
        valid_tap = (root / 'public.tap').read_text()
        for bad in [tap([p.PUBLIC_TEST]), valid_tap.replace(sha, 'b'*40), valid_tap + '# public native completion: ' + marker + '\n']:
            with self.assertRaises(ValueError): p.public_tap(bad, request)
        for file, key, value in [('public-run.json', 'head', 'b'*40), ('logs/planner.json', 'argv', []),
            ('logs/discovery.json', 'env', {}), ('logs/post-verify.json', 'exit', 1),
            ('results/completion.json', 'plans', result['plans'][:-1]),
            ('results/completion.json', 'plans', [result['plans'][0]] + result['plans'][:-1]),
            ('results/completion.json', 'attested', True), ('bridge-config/sealed.json', 'helper_sha256', '0'*64)]:
            original = get(file); put(file, dict(original, **{key: value}))
            with self.subTest(file=file, key=key), self.assertRaises((ValueError, KeyError)): p.check_public(root, sha)
            put(file, original)
        (inventory_root / 'unexpected').mkdir()
        with self.assertRaisesRegex(ValueError, 'sealed tree changed'): p.check_public(root, sha)
        (inventory_root / 'unexpected').rmdir()
        for text in [tap([p.PUBLIC_TEST])[:-1], tap([p.PUBLIC_TEST]).replace('# skipped 0', '# skipped 1'), tap(p.NATIVE_TESTS)]:
            put('public.tap', text)
            with self.assertRaises(ValueError): p.check_public(root, sha)

    def test_public_v2_boundary_and_success_consumer(self):
        native = dict(schema='dual-authoring-public-native/v2', projects={'agentplugins': '/fixture/agentplugins projects ü'},
            installer_boundary=dict(executable_observation='help-and-preflight-rejection', valid_add_dry_run='not_evaluated',
                argv=['add', '/fixture/agentplugins projects ü/skill', '--target=codex', '--dry-run', '--format=json'],
                reason='production-security-inputs-not-offline'))
        self.assertEqual(p.public_boundary(native, 'public-fixture/v2'), native['installer_boundary'])
        with self.assertRaisesRegex(ValueError, 'insufficient evidence'):
            p.public_boundary(native, 'public-fixture/v2', require_valid_add=True)
        for intake in ('public-fixture/v1', 'private', None):
            with self.assertRaises(ValueError): p.public_boundary(native, intake)
        boundary = native['installer_boundary']
        for bad in (None, {}, True, dict(boundary, valid_add_dry_run=True), dict(boundary, reason=''),
            dict(boundary, argv=[]), dict(boundary, accepted=True)):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                p.public_boundary(dict(native, installer_boundary=bad), 'public-fixture/v2')
        for key in boundary:
            bad = dict(boundary); del bad[key]
            with self.assertRaises(ValueError): p.public_boundary(dict(native, installer_boundary=bad), 'public-fixture/v2')
        with self.assertRaises(ValueError):
            p.public_boundary(dict(native, schema='dual-authoring-public-native/v1'), 'public-fixture/v2')

    def test_public_runner_rejects_missing_or_wrong_contract_before_output(self):
        root = Path(tempfile.mkdtemp(prefix='public-packed-runner-SYNTHETIC-'))
        options = root / 'options.json'; output = root / 'must-not-exist'
        for value in ({}, dict(request={'intake': 'private'}, nativeTap='/unused', nativeTapSha256='0'*64, go='/unused', node='/unused', modCache='/unused')):
            options.write_text(json.dumps(value))
            with self.assertRaises(ValueError): r.public_main(output, 'a'*40, options)
            self.assertFalse(output.exists())


class WorkflowControls(unittest.TestCase):
    def test_runner_context_rejected_only_at_job_env_scope(self):
        text = (ROOT / '.github/workflows/authoring-native.yml').read_text()
        runner = (ROOT / 'scripts/run-packed-ci.py').read_text()
        for expression in ('${{ runner.temp }}', "${{ runner['temp'] }}"):
            bad = text.replace("      PYTHONDONTWRITEBYTECODE: '1'",
                '      PACKED_ROOT: ' + expression + '/authoring-packed-${{ github.run_id }}-${{ github.run_attempt }}\n' +
                "      PYTHONDONTWRITEBYTECODE: '1'", 1)
            with self.subTest(expression=expression), self.assertRaisesRegex(ValueError, 'runner context is unavailable in job env'):
                w.check(bad, runner)
        # The same context is supported at step scope.
        good = text.replace('      - name: Build, pack, execute, seal and plan at the exact checkout SHA\n',
            '      - name: Build, pack, execute, seal and plan at the exact checkout SHA\n'
            '        env:\n          STEP_TEMP: ${{ runner.temp }}\n')
        w.check(good, runner)

    def test_committed_graph_and_mutations(self):
        text = (ROOT / '.github/workflows/authoring-native.yml').read_text()
        runner = (ROOT / 'scripts/run-packed-ci.py').read_text()
        w.check(text, runner)
        mutations = [('  packed:\n', '  omitted:\n'), ('needs: [native, packed]', 'needs: [native]'),
            ('    if: always()\n', '    if: success()\n'), ('  packed:\n', '  packed:\n    if: false\n'),
            ('  native:\n', '  native:\n    if: false\n'), ('arch: arm64}', 'arch: amd64}'),
            ('  pull_request:\n', '  pull_request:\n    paths: [scripts/**]\n'),
            ('    timeout-minutes: 45', '    continue-on-error: true\n    timeout-minutes: 45'),
            ('scripts/check-packed-ci.py "$PACKED_ROOT"', 'scripts/missing.py "$PACKED_ROOT"'),
            ('scripts/check-packed-workflow.py --results', 'scripts/omitted.py --results'),
            ('        run: python3 -B scripts/run-packed-ci.py', '        if: false\n        run: python3 -B scripts/run-packed-ci.py')]
        for before, after in mutations:
            with self.subTest(before=before), self.assertRaises(ValueError): w.check(text.replace(before, after), runner)
        for before, after in [("'-tags=packedci', ", ''), ('proof.REGEX', "'^TestOther$'"), ('proof.PACKAGE_PATH', "'./wrong'"), ('proof.check(root, sha)', '')]:
            with self.assertRaises(ValueError): w.check(text, runner.replace(before, after))

    def test_dependencies_must_all_succeed(self):
        good = dict(native=dict(result='success'), packed=dict(result='success')); w.results(good)
        for job in good:
            for status in ('failure', 'skipped', 'cancelled', None):
                bad = copy.deepcopy(good); bad[job]['result'] = status
                with self.assertRaises(ValueError): w.results(bad)
            bad = copy.deepcopy(good); del bad[job]
            with self.assertRaises(ValueError): w.results(bad)




class C3AuthenticatedControls(unittest.TestCase):
    # SYNTHETIC gate mock exercises retained prepared intake validations only.
    @patch.object(r.proof, 'require_authenticated_execution', return_value=None)
    @patch.object(r.proof, 'require_authenticated_controller')
    def test_closed_intake_before_output(self, synthetic_gate, synthetic_execution):
        from unittest.mock import patch
        root = Path(tempfile.mkdtemp(prefix='C3-runner-SYNTHETIC-'))
        options = root / 'options.json'; output = root / 'must-not-exist'
        synthetic_gate.return_value = '/unused'
        for value in ({}, dict(request={'intake': 'public-fixture/v2'}, go='/unused', node='/unused', modCache='/unused')):
            options.write_text(json.dumps(value, indent=2) + '\n')
            with self.assertRaises(ValueError): r.authenticated_main(output, 'a' * 40, options)
            self.assertFalse(output.exists())
        # The external authenticated reader fails deterministically; no process,
        # real provider, planner or output creation may follow its failure.
        tool = root / 'tool'; tool.write_text('SYNTHETIC')
        modules = root / 'modules'; modules.mkdir()
        admission = root / 'admission.json'; journey = root / 'journey.json'; journey.write_text('{}\n')
        dirs = {}
        for name in ('repo', 'work_parent', 'stage_root', 'input_root', 'journey_root', 'fixture_root'):
            directory = root / name; directory.mkdir(); dirs[name] = str(directory)
        admission.write_text(json.dumps(dirs, indent=2) + '\n')
        request = dict(intake=p.AUTHENTIC, expectedCommit='a' * 40, journey=str(journey), journeySha256=p.digest(journey),
            admission=str(admission), admissionSha256=p.digest(admission), fixtureRoot=dirs['fixture_root'])
        synthetic_gate.return_value = str(tool)
        value = dict(request=request, go=str(tool), node=str(tool), modCache=str(modules))
        options.write_text(json.dumps(value, indent=2) + '\n')
        with patch.object(r.proof, 'authenticated_verify', side_effect=ValueError('missing reviewed installer/observer')) as reader, \
                patch.object(r, 'planner', side_effect=AssertionError('planner effect')) as planner:
            with self.assertRaisesRegex(ValueError, 'missing reviewed'): r.authenticated_main(output, 'a' * 40, options)
            reader.assert_called_once(); planner.assert_not_called(); self.assertFalse(output.exists())

    # SYNTHETIC gate mock exercises retained prepared terminal validations only.
    @patch.object(p, 'require_authenticated_execution', return_value=None)
    @patch.object(p, 'require_authenticated_controller')
    def test_exact_thirty_plans_and_post_seal(self, synthetic_gate, synthetic_execution):
        root = Path(tempfile.mkdtemp(prefix='C3-plans-SYNTHETIC-')); (root / 'results').mkdir(); (root / 'logs').mkdir()
        fixture = root / 'original-projects'; fixture.mkdir()
        entries = [dict(path='.', mode=fixture.stat().st_mode & 0o777, kind='directory')]
        inputs = dict(projects=[dict(product=product, lane=lane, source=str(fixture / (product + ' projects ü') / lane))
            for product in p.PRODUCTS for lane in p.LANES], snapshots=[dict(root=str(fixture), entries=entries,
                sha256=p.hashlib.sha256((json.dumps(entries, indent=2) + '\n').encode()).hexdigest())])
        record = dict(plan_fixture(), kind='packed-generated-existing-injected-installer-planner', commit='a' * 40,
            config_sha256='b' * 64, inputs=inputs, release_eligible=False, platform_acceptance=False, attested=False)
        terminal = root / 'results/completion.json'; post = root / 'logs/post-verify.stdout'
        put = lambda file, value: file.write_text(json.dumps(value, indent=2) + '\n')
        put(terminal, record); put(post, inputs)
        # Exercise the entire distinct terminal branch with a SYNTHETIC opaque
        # readback, then corrupt real retained logs. No subprocess is launched.
        from unittest.mock import patch
        tool = root / 'tool'; tool.write_text('SYNTHETIC TOOL')
        synthetic_gate.return_value = str(tool)
        modules = root / 'modules'; modules.mkdir()
        journey = root / 'J.json'; admission = root / 'admission.json'
        put(journey, {}); put(admission, {})
        request = dict(intake=p.AUTHENTIC, expectedCommit='a' * 40, journey=str(journey), journeySha256=p.digest(journey),
            admission=str(admission), admissionSha256=p.digest(admission), fixtureRoot=str(fixture))
        options = dict(request=request, go=str(tool), node=str(tool), modCache=str(modules))
        claims = dict(release_eligible=False, platform_acceptance=False, attested=False)
        pin_tool = dict(path=str(tool), sha256=p.digest(tool))
        inputs['public_inputs'] = dict(schema='authoring-public-local-inputs/v1', cell='linux-amd64/pair-node22',
            journey_sha256=request['journeySha256'], admission_sha256=request['admissionSha256'],
            tools=dict(go=pin_tool, orchestrator_node=pin_tool), signed_promotion=False, public_eligible=False, qualification=None)
        put(root / 'authenticated-run.json', dict(schema='public-authenticated-packed-run/v1', head='a' * 40,
            options=options, tools=dict(go=pin_tool, node=pin_tool), **claims))
        bridge = ROOT / 'npm/agentplugins/scripts/packed-installer-bridge.js'
        config = root / 'bridge-config'; config.mkdir(); sealed = config / 'sealed.json'
        put(config / 'request.json', request)
        put(sealed, dict(schema=p.AUTHENTIC_SEAL, request=request, inputs=inputs,
            verifier_sha256=p.digest(bridge), helper_sha256=p.digest(bridge.with_name('dual-authoring-candidate.js')),
            reader_sha256=p.digest(bridge.with_name('public-authoring-acceptance.js')), **claims))
        pin = p.digest(sealed); record['config_sha256'] = pin
        put(terminal, record); put(post, inputs)
        commands = {'head': ['/usr/bin/git', 'rev-parse', 'HEAD'],
            'clean': ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all'],
            'terminal-clean': ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all'],
            'seal': [str(tool), str(bridge), 'authenticated-seal', str(config / 'request.json'), str(sealed)],
            'post-verify': [str(tool), str(bridge), 'verify', str(sealed), pin, 'a' * 40]}
        for name, flag in [('discovery', '-list'), ('planner', '-run')]:
            commands[name] = [str(tool), 'test', '-p=2', '-tags=packedci', '-json',
                *([] if name == 'discovery' else ['-count=1', '-timeout=20m']), flag, p.REGEX, p.PACKAGE_PATH]
        for name, argv in commands.items():
            env = dict(GOPROXY='off', GOSUMDB='off', GOVCS='*:off', GOENV='off', GOTOOLCHAIN='local')
            if name in ('discovery', 'planner'):
                env.update(UAP_PACKED_INSTALLER_NODE=str(tool), UAP_PACKED_INSTALLER_CONFIG=str(sealed),
                    UAP_PACKED_INSTALLER_CONFIG_SHA256=pin, UAP_PACKED_INSTALLER_COMMIT='a' * 40,
                    UAP_PACKED_INSTALLER_OUTPUT=str(terminal))
            put(root / 'logs' / (name + '.json'), dict(argv=argv, cwd=str(ROOT), env=env, exit=0))
            (root / 'logs' / (name + '.stderr')).write_text('')
        logs = {'head': 'a' * 40 + '\n', 'clean': '', 'terminal-clean': '', 'seal': pin + '\n',
            'discovery': go([dict(Action='output', Output=p.NAME + '\n'), dict(Action='pass')]),
            'planner': go([dict(Action=action, Test=name) for name in [p.NAME] +
                [p.NAME + '/' + product + '/' + lane for product in p.PRODUCTS for lane in p.LANES]
                for action in ('run', 'pass')] + [dict(Action='pass')])}
        for name, text in logs.items(): (root / 'logs' / (name + '.stdout')).write_text(text)
        put(root / 'summary.json', dict(status='passed', scope='local-authenticated-inputs-and-injected-planner',
            intake=p.AUTHENTIC, head='a' * 40, projects=10, plans=30, **claims,
            signed_promotion=False, public_eligible=False, qualification=None))
        with patch.object(p, 'authenticated_verify', return_value=copy.deepcopy(inputs)) as reader:
            p.check_authenticated(root, 'a' * 40); reader.assert_called_once()
            for file, key, value in [('logs/planner.json', 'argv', []), ('logs/post-verify.json', 'exit', 1),
                ('summary.json', 'scope', 'completed-authenticated-E'), ('bridge-config/sealed.json', 'reader_sha256', '0' * 64)]:
                target = root / file; old = p.read(target); put(target, dict(old, **{key: value}))
                with self.subTest(file=file), self.assertRaises(ValueError): p.check_authenticated(root, 'a' * 40)
                put(target, old)
        with patch.object(p, 'authenticated_verify', side_effect=ValueError('late custody cancellation')):
            with self.assertRaisesRegex(ValueError, 'late custody cancellation'): p.check_authenticated(root, 'a' * 40)
        record['config_sha256'] = 'b' * 64; put(terminal, record)
        check = lambda: p.authenticated_plans(root, 'a' * 40, inputs, 'b' * 64, str(fixture))
        check()
        for name, mutate in [('29 plans', lambda v: v['plans'].pop()),
            ('duplicate plans', lambda v: v['plans'].__setitem__(1, v['plans'][0])),
            ('wrong seal', lambda v: v.update(config_sha256='c' * 64)),
            ('changed original project', lambda v: v['inputs']['projects'][0].update(source='/different/project'))]:
            bad = copy.deepcopy(record); mutate(bad); put(terminal, bad)
            with self.subTest(case=name), self.assertRaises(ValueError): check()
        put(terminal, record); put(post, {})
        with self.assertRaisesRegex(ValueError, 'post-plan seal'): check()
        put(post, inputs); (fixture / 'extra-empty').mkdir()
        with self.assertRaisesRegex(ValueError, 'sealed tree changed'): check()

    def test_substituted_node_rejected_before_authenticated_effects(self):
        import subprocess
        root = Path(tempfile.mkdtemp(prefix='C3-controller-SYNTHETIC-'))
        tool = root / 'substitute-node'; tool.write_text('SYNTHETIC TOOL')
        options = dict(request=dict(intake=p.AUTHENTIC, expectedCommit='a' * 40),
            go=str(tool), node=str(tool), modCache=str(root))
        options_path = root / 'options.json'
        options_path.write_text(json.dumps(options, indent=2) + '\n')
        receipt = dict(schema='public-authenticated-packed-run/v1', head='a' * 40,
            options=options, tools=dict(node=dict(path=str(tool), sha256=p.digest(tool))))
        (root / 'authenticated-run.json').write_text(json.dumps(receipt, indent=2) + '\n')
        output = root / 'must-not-exist'
        before = {str(f): f.read_bytes() for f in root.iterdir()}
        # Actual entrypoints and rejecting gate: no test-only gate mock here.
        with patch.object(subprocess, 'run', side_effect=AssertionError('subprocess effect')) as child, \
                patch.object(r, 'planner', side_effect=AssertionError('planner effect')) as planner, \
                patch.object(r, 'write', side_effect=AssertionError('output effect')) as write:
            for name, call in (
                ('runner', lambda: r.authenticated_main(output, 'a' * 40, options_path)),
                ('checker', lambda: p.check_authenticated(root, 'a' * 40)),
                ('checker-before-summary', lambda: p.check_authenticated(root, 'a' * 40, require_summary=False)),
                ('direct-reader', lambda: p.authenticated_verify(options['node'], ['authenticated-options', options_path])),
            ):
                with self.subTest(entrypoint=name), self.assertRaisesRegex(ValueError,
                        'PUBLIC_PROVISIONING_REQUIRED:linux-amd64:node'):
                    call()
            child.assert_not_called(); planner.assert_not_called(); write.assert_not_called()
        self.assertFalse(output.exists())
        self.assertEqual(before, {str(f): f.read_bytes() for f in root.iterdir()})

    def test_completed_e_cannot_use_fixture_success(self):
        root = Path(tempfile.mkdtemp(prefix='C3-E-closed-SYNTHETIC-'))
        for schema in ('public-fixture/v1', 'public-fixture/v2', p.AUTHENTIC):
            (root / 'summary.json').write_text(json.dumps(dict(status='passed', intake=schema, plans=30, projects=10)))
            with self.subTest(intake=schema), self.assertRaisesRegex(ValueError, 'completed E cannot use'):
                p.check_authenticated(root, 'a' * 40, require_completed_e=True)
            with self.assertRaisesRegex(ValueError, 'PUBLIC_PROVISIONING_REQUIRED:linux-amd64:node'):
                p.check_authenticated(root, 'a' * 40)


class C3bProvisionControls(unittest.TestCase):
    def fixture(self):
        root = Path(tempfile.mkdtemp(prefix='C3b-provision-SYNTHETIC-'))
        (root / '.github').mkdir(); (root / 'scripts').mkdir()
        source = root / 'scripts/check-packed-ci.py'; source.write_text('SYNTHETIC SOURCE\n')
        (root / 'scripts/run-packed-ci.py').write_text('SYNTHETIC RUNNER\n')
        for folder in ('npm/agentplugins/scripts', 'npm/agentplugins/lib', 'npm/plugin-kit-ai/lib'):
            (root / folder).mkdir(parents=True); (root / folder / 'source.js').write_text('SYNTHETIC SOURCE\n')
        value = p.read_provisioning(); file = root / '.github/authoring-public-tools.json'
        tool = root / 'node'; tool.write_text('SYNTHETIC TOOL\n')
        for name in p.PROVISION_TOOLS:
            value['controllers']['linux-amd64'][name] = dict(path=str(tool), version='v22.21.1', sha256=p.digest(tool))
        self.put(file, value)
        return root, source, file, tool, value

    def put(self, file, value): file.write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n')

    def test_literal_canonical_and_closed_manifest_table(self):
        root, source, file, tool, value = self.fixture()
        literal = '{\n  "path": "/synthetic/node",\n  "version": "v22.21.1",\n  "sha256": "' + p.hashlib.sha256(b'SYNTHETIC TOOL\n').hexdigest() + '"\n}'
        value['controllers']['linux-amd64']['node'] = json.loads(literal); self.put(file, value)
        with patch.object(p, '__file__', str(source)):
            self.assertEqual(json.dumps(p.read_provisioning()['controllers']['linux-amd64']['node'], indent=2), literal)
            missing = copy.deepcopy(value); del missing['cells']['linux-arm64/kit-node18']; self.put(file, missing)
            with self.assertRaisesRegex(ValueError, 'PUBLIC_PROVISIONING_REQUIRED:linux-arm64/kit-node18:entry'): p.read_provisioning()
            mutations = [lambda v: v['cells'].pop('linux-arm64/kit-node18'), lambda v: v['controllers'].update(extra={}),
                lambda v: v['controllers']['linux-amd64'].pop('tar'), lambda v: v['cells']['linux-amd64/kit-node18'].update(extra=None),
                lambda v: v.update(reader='windows-amd64'), lambda v: v['cells']['linux-amd64/kit-node18'].update(controller='linux-arm64'),
                lambda v: v['controllers']['linux-amd64']['node'].update(sha256='0' * 64),
                lambda v: v['controllers']['linux-amd64']['node'].update(path='/synthetic/../node'),
                lambda v: v['controllers']['linux-amd64']['node'].update(version='x' * 257),
                lambda v: v['controllers']['linux-amd64']['node'].update(extra=True)]
            for index, mutate in enumerate(mutations):
                bad = copy.deepcopy(value); mutate(bad); self.put(file, bad)
                with self.subTest(case=index), self.assertRaises(ValueError): p.read_provisioning()
            body = (json.dumps(value, indent=2) + '\n').encode()
            for bad in (json.dumps(value).encode(), body.replace(b'  "schema":', b'  "reader": "linux-amd64",\n  "schema":'),
                        b'[[' * 10, b' ' * (1024 * 1024 + 1), b'\xff', b'null\n'):
                file.write_bytes(bad)
                with self.subTest(bytes=len(bad)), self.assertRaises(ValueError): p.read_provisioning()

    def test_source_binding_tools_missing_pins_links_and_aliases(self):
        root, source, file, tool, value = self.fixture()
        with patch.object(p, '__file__', str(source)):
            self.assertEqual(p.require_authenticated_controller(), str(tool))
            tool.write_text('CHANGED TOOL\n')
            with self.assertRaisesRegex(ValueError, 'pin mismatch'): p.require_authenticated_controller()
            tool.write_text('SYNTHETIC TOOL\n')
            value['controllers']['linux-amd64']['gh'] = None; self.put(file, value)
            with self.assertRaisesRegex(ValueError, 'PUBLIC_PROVISIONING_REQUIRED:linux-amd64:gh'): p.require_authenticated_controller()
            value['controllers']['linux-amd64']['node']['path'] = str(root / 'missing'); self.put(file, value)
            with self.assertRaisesRegex(ValueError, 'PUBLIC_PROVISIONING_REQUIRED:linux-amd64:node'): p.require_authenticated_controller()
            link = root / 'link'; link.symlink_to(tool); value['controllers']['linux-amd64']['node']['path'] = str(link); self.put(file, value)
            with self.assertRaisesRegex(ValueError, 'canonical provision path'): p.require_authenticated_controller()
            with self.assertRaises(TypeError): p.require_authenticated_controller(root)
        # A receipt with its own complete matching tool manifest cannot select root.
        with self.assertRaisesRegex(ValueError, 'PUBLIC_PROVISIONING_REQUIRED:linux-amd64:node'): p.require_authenticated_controller()
        with patch.object(p, '__file__', str(root / 'absent/scripts/check-packed-ci.py')):
            with self.assertRaisesRegex(ValueError, 'PUBLIC_PROVISIONING_REQUIRED:linux-amd64:manifest'): p.require_authenticated_controller()

    def test_prepared_controller_rechecks_and_minimal_environment(self):
        import subprocess
        from types import SimpleNamespace
        root, source, file, tool, value = self.fixture()
        # Real tool-byte binding; only later unfinished capability is mocked.
        # All subprocesses are mocked; SYNTHETIC TOOL is never executed.
        with patch.object(p, '__file__', str(source)), patch.object(p, 'require_authenticated_execution'), \
                patch.object(subprocess, 'run', return_value=SimpleNamespace(returncode=0, stderr=b'', stdout=b'{}')) as child:
            self.assertEqual(p.authenticated_verify(str(tool), ['authenticated-options', '/synthetic/options']), {})
            argv = child.call_args.args[0]; env = child.call_args.kwargs['env']
            self.assertEqual(argv, [str(tool), str(root / 'npm/agentplugins/scripts/packed-installer-bridge.js'), 'authenticated-options', '/synthetic/options'])
            self.assertEqual(set(env), {'PATH', 'LANG', 'LC_ALL'})
            for name in ('NODE_OPTIONS', 'NODE_PATH', 'PYTHONPATH', 'PYTHONHOME', 'LD_PRELOAD', 'LD_LIBRARY_PATH'):
                self.assertNotIn(name, env)
            child.reset_mock()
            with self.assertRaisesRegex(ValueError, 'comparison mismatch'): p.authenticated_verify('/receipt/node', [])
            child.assert_not_called()
            def changed(*args, **kwargs):
                tool.write_text('LATE CHANGE\n'); return SimpleNamespace(returncode=0, stderr=b'', stdout=b'{}')
            child.side_effect = changed
            with self.assertRaisesRegex(ValueError, 'pin mismatch'): p.authenticated_verify(str(tool), [])
            tool.write_text('SYNTHETIC TOOL\n'); child.side_effect = None
            def changed_source(*args, **kwargs):
                source.write_text('LATE SOURCE CHANGE\n'); return SimpleNamespace(returncode=0, stderr=b'', stdout=b'{}')
            child.side_effect = changed_source
            with self.assertRaisesRegex(ValueError, 'trusted source/controller changed'): p.authenticated_verify(str(tool), [])
            child.reset_mock(); child.side_effect = None
            controller = p.require_authenticated_controller; calls = []
            def before_launch():
                calls.append(True)
                if len(calls) == 2: source.write_text('CHANGED BEFORE LAUNCH\n')
                return controller()
            with patch.object(p, 'require_authenticated_controller', side_effect=before_launch):
                with self.assertRaisesRegex(ValueError, 'trusted source/controller changed'): p.authenticated_verify(str(tool), [])
            child.assert_not_called()

    def test_verified_controller_still_cannot_open_execution(self):
        import subprocess
        root, source, file, tool, value = self.fixture(); output = root / 'must-not-exist'
        with patch.object(p, '__file__', str(source)), patch.object(r.proof, '__file__', str(source)), \
                patch.object(subprocess, 'run') as child, patch.object(r, 'write') as write, patch.object(r, 'planner') as planner:
            for call in (lambda: p.authenticated_verify(str(tool), []), lambda: p.check_authenticated(root, 'a' * 40),
                         lambda: p.check_authenticated(root, 'a' * 40, require_summary=False),
                         lambda: r.authenticated_main(output, 'a' * 40, root / 'unread-receipt')):
                with self.assertRaisesRegex(ValueError, 'C3b execution incomplete'): call()
            child.assert_not_called(); write.assert_not_called(); planner.assert_not_called()
            self.assertFalse(output.exists())


if __name__ == '__main__': unittest.main()
