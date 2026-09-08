"""Synthetic controls only; no product execution and no packed acceptance claim."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest

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
        root, sha, put, get = TerminalControls.fixture(self)
        # Reuse only the synthetic planner transcripts; no native claim.
        private = get('run.json'); tools = private['tools']; claims = {k: False for k in p.CLAIMS}
        request = dict(intake='public-fixture/v1', expectedCommit=sha, nativeConfig=str(root / 'public-config.json'))
        put('public-config.json', dict(evidenceOutput=str(root / 'public-native')))
        put('public-native/invocations.json', [])
        native = dict(schema='dual-authoring-public-native/v1', status='completed', identity=private['identity'],
            qualification=None, tools=tools, invocations_sha256=p.digest(root / 'public-native/invocations.json'),
            signed_promotion=False, public_eligible=False, **claims)
        put('public-native/public-native-completion.json', native)
        request.update(nativeConfigSha256=p.digest(request['nativeConfig']),
            nativeCompletionSha256=p.digest(root / 'public-native/public-native-completion.json'))
        marker = json.dumps(dict(file=str(root / 'public-native/public-native-completion.json'), sha256=request['nativeCompletionSha256'], source=sha))
        put('public.tap', tap([p.PUBLIC_TEST]).replace('TAP version 13\n', 'TAP version 13\n# public native completion: ' + marker + '\n'))
        options = dict(request=request, nativeTap=str(root / 'public.tap'), nativeTapSha256=p.digest(root / 'public.tap'),
            go=tools['go']['path'], node=tools['node']['path'], modCache=str(root / 'unused-modules'))
        put('public-run.json', dict(schema='public-packed-run/v1', head=sha, options=options, tools=tools, **claims))
        sealed = get('bridge-config/sealed.json'); sealed['request'] = request
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


if __name__ == '__main__': unittest.main()
