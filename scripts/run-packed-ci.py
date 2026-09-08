#!/usr/bin/env python3
"""Bounded Linux packed proof. Setup/warmup precedes the offline command contract."""
import importlib.util
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import time

SPEC = importlib.util.spec_from_file_location('packed', Path(__file__).with_name('check-packed-ci.py'))
proof = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(proof)


def write(path, value):
    with path.open('x', encoding='utf-8') as stream:
        stream.write(json.dumps(value, indent=2, ensure_ascii=False) + '\n')


def private_env(root, go, node, repo, modules):
    for name in ('home', 'tmp', 'config', 'cache', 'data', 'state', 'appdata', 'localappdata', 'hooks'):
        (root / name).mkdir(parents=True, exist_ok=True)
    env = {'PATH': os.pathsep.join([str(go.parent), str(node.parent), '/usr/bin', '/bin']),
           'LANG': 'C.UTF-8', 'LC_ALL': 'C.UTF-8', 'TZ': 'UTC', 'GOENV': 'off',
           'GOTOOLCHAIN': 'local', 'GOMAXPROCS': '2', 'GOWORK': str(repo / 'go.work'),
           'GOCACHE': str(root / 'cache/go-build'), 'GOMODCACHE': str(modules),
           'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': str(root / 'config/gitconfig'),
           'GIT_CONFIG_COUNT': '1', 'GIT_CONFIG_KEY_0': 'core.hooksPath',
           'GIT_CONFIG_VALUE_0': str(root / 'hooks'), 'GIT_TERMINAL_PROMPT': '0'}
    for key, directory in {'HOME': 'home', 'USERPROFILE': 'home', 'TMPDIR': 'tmp', 'TMP': 'tmp',
        'TEMP': 'tmp', 'XDG_CONFIG_HOME': 'config', 'XDG_CACHE_HOME': 'cache',
        'XDG_DATA_HOME': 'data', 'XDG_STATE_HOME': 'state', 'APPDATA': 'appdata',
        'LOCALAPPDATA': 'localappdata'}.items():
        env[key] = str(root / directory)
    return env


def planner(root, sha, go, node, sealed, printed, run):
    bridge = Path(__file__).resolve().parent.parent / "npm/agentplugins/scripts/packed-installer-bridge.js"
    results = root / 'results'; results.mkdir()
    extra = dict(UAP_PACKED_INSTALLER_NODE=str(node), UAP_PACKED_INSTALLER_CONFIG=str(sealed),
        UAP_PACKED_INSTALLER_CONFIG_SHA256=printed, UAP_PACKED_INSTALLER_COMMIT=sha,
        UAP_PACKED_INSTALLER_OUTPUT=str(results / 'completion.json'))
    discovery = run('discovery', [go, 'test', '-p=2', '-tags=packedci', '-json', '-list', proof.REGEX, proof.PACKAGE_PATH], extra)
    proof.go_discovery(discovery)
    output = run('planner', [go, 'test', '-p=2', '-tags=packedci', '-json', '-count=1', '-timeout=20m', '-run', proof.REGEX, proof.PACKAGE_PATH], extra)
    proof.go_results(output)
    run('post-verify', [node, bridge, 'verify', sealed, printed, sha])


def main(root, sha):
    repo = Path(__file__).resolve().parent.parent
    proof.require(re.fullmatch('[0-9a-f]{40}', sha), 'exact SHA required')
    proof.require(root.is_absolute() and root.resolve() == root and not root.is_relative_to(repo), 'external canonical output required')
    root.mkdir(mode=0o700)  # stale output is fatal
    (root / 'logs').mkdir()
    proof.require(platform.system() == 'Linux' and platform.machine() == 'x86_64', 'native Linux amd64 required')
    go, node, npm = [Path(shutil.which(tool) or '/missing-tool').resolve(strict=True) for tool in ('go', 'node', 'npm')]
    proof.require(npm.name == 'npm-cli.js', 'resolve npm JavaScript CLI, not shell shim')
    modules = root / 'modules'; modules.mkdir()
    env = private_env(root / 'orchestrator', go, node, repo, modules)
    phases = {}

    def run(name, argv, extra=None, timeout=1200):
        argv = list(map(str, argv)); started = time.monotonic()
        record = {'argv': argv, 'cwd': str(repo), 'env': dict(env, **(extra or {})), 'exit': None}
        try:
            with (root / 'logs' / (name + '.stdout')).open('x') as out, (root / 'logs' / (name + '.stderr')).open('x') as err:
                result = subprocess.run(argv, cwd=repo, env=record['env'], stdout=out, stderr=err, timeout=timeout)
            record['exit'] = result.returncode
        finally:
            record['seconds'] = round(time.monotonic() - started, 3)
            phases[name] = record
            write(root / 'logs' / (name + '.json'), record)
        proof.require(record['exit'] == 0, name + ' failed')
        return (root / 'logs' / (name + '.stdout')).read_text()

    proof.require(run('head', ['/usr/bin/git', 'rev-parse', 'HEAD']).strip() == sha, 'wrong checkout')
    proof.require(run('clean', ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all']) == '', 'dirty checkout')
    versions = {'go': run('go-version', [go, 'env', 'GOVERSION']).strip(),
                'node': run('node-version', [node, '--version']).strip(),
                'npm': run('npm-version', [node, npm, '--version']).strip()}
    proof.require(versions == proof.VERSIONS, 'CI requires exact bundled Node/npm and Go pins')
    machine = json.loads(run('go-host', [go, 'env', '-json', 'GOOS', 'GOARCH', 'GOHOSTOS', 'GOHOSTARCH']))
    proof.require(machine == dict(GOOS='linux', GOARCH='amd64', GOHOSTOS='linux', GOHOSTARCH='amd64'), 'Go host mismatch')
    # Private candidate versions are the accepted release-contract fixture identity,
    # not npm publication versions (package.json deliberately says development).
    identity = {'repository': '777genius/universal-agent-plugins', 'commit': sha, 'engine_revision': sha,
                'versions': {'agentplugins': '0.1.91', 'plugin-kit-ai': '2.0.0'}}
    write(root / 'run.json', {'head': sha, 'run_id': os.environ['GITHUB_RUN_ID'],
        'attempt': os.environ['GITHUB_RUN_ATTEMPT'], 'versions': versions,
        'tools': {k: {'path': str(p), 'sha256': proof.digest(p)} for k, p in dict(go=go, node=node, npm=npm).items()},
        'identity': identity, 'release_eligible': False, 'platform_acceptance': False, 'attested': False})
    run('warmup', [go, 'list', '-p=2', '-deps', '-test', proof.PACKAGE_PATH,
        './cli/plugin-kit-ai/cmd/agentplugins', './cli/plugin-kit-ai/cmd/plugin-kit-ai'])
    env.update(GOPROXY='off', GOSUMDB='off', GOVCS='*:off')
    work = root / 'work'; work.mkdir()
    stage = dict(candidate=True, repo=str(repo), output=str(root / 'candidate'), workParent=str(work),
        go=str(go), modCache=str(modules), identity=identity, assetScope='linux-amd64-pair', authoringMode=proof.MODE)
    write(root / 'stage.json', stage)
    scripts = repo / 'npm/agentplugins/scripts'
    producer = scripts / 'stage-dual-authoring-candidate.js'
    staged = json.loads(run('stage', [node, producer, 'stage', '--candidate', root / 'stage.json']))
    pin = proof.digest(root / 'candidate/candidate.json')
    proof.require(staged['manifest_sha256'] == pin, 'stage manifest pin')
    verify = {k: stage[k] for k in ('candidate', 'workParent', 'go', 'identity', 'assetScope', 'authoringMode')}
    verify.update(root=str(root / 'candidate'), manifestDigest=pin)
    write(root / 'verify.json', verify)
    run('verify', [node, producer, 'verify', '--candidate', root / 'verify.json'])
    tap = run('candidate-native', [node, '--test', '--test-reporter=tap', 'npm/agentplugins/test/dual-authoring-candidate-native.test.js'],
        {'UAP_CANDIDATE_NATIVE_CONFIG': str(root / 'verify.json'), 'UAP_CANDIDATE_NATIVE_SOURCE_REPO': str(repo)})
    proof.tap(tap, [proof.CANDIDATE_TEST])
    npm_stage = dict(verify, repo=str(repo), output=str(root / 'npm-pair'), node=str(node), npm=str(npm))
    write(root / 'npm-stage.json', npm_stage)
    run('npm-stage', [node, scripts / 'stage-dual-authoring-npm.js', '--candidate', root / 'npm-stage.json'])
    native = dict(stage=npm_stage, completionDigest=proof.digest(root / 'npm-pair/completion.json'), evidenceOutput=str(root / 'native'))
    write(root / 'npm-native.json', native)
    tap = run('npm-native', [node, '--test', '--test-reporter=tap', 'npm/agentplugins/test/private-npm-native.test.js'],
        {'UAP_PRIVATE_NPM_NATIVE_CONFIG': str(root / 'npm-native.json')})
    proof.tap(tap, proof.NATIVE_TESTS)
    completion = proof.read(root / 'native/native-completion.json')
    proof.require(completion['invocations'] == len(proof.read(root / 'native/invocations.json')) == 741, 'exactly 741 invocations required')
    parents = {str(Path(p).parent) for p in completion['projects'].values()}
    proof.require(set(completion['projects']) == set(proof.PRODUCTS) and len(parents) == 1, 'two project parents required')
    config = root / 'bridge-config'; config.mkdir()
    write(config / 'request.json', dict(expectedCommit=sha, nativeConfig=str(root / 'npm-native.json'),
        nativeConfigSha256=proof.digest(root / 'npm-native.json'),
        nativeCompletionSha256=proof.digest(root / 'native/native-completion.json'),
        fixtureRoot=parents.pop(), disposableEvidence=True))
    bridge = scripts / 'packed-installer-bridge.js'
    sealed = config / 'sealed.json'
    printed = run('seal', [node, bridge, 'seal', config / 'request.json', sealed]).strip()
    proof.require(printed == proof.digest(sealed), 'seal pin mismatch')
    env = private_env(root / 'planner', go, node, repo, modules)
    env.update(GOPROXY='off', GOSUMDB='off', GOVCS='*:off')
    planner(root, sha, go, node, sealed, printed, run)
    proof.require(run('terminal-clean', ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all']) == '', 'checkout changed')
    proof.check(root, sha)
    write(root / 'artifact-index.json', {str(p.relative_to(root)): proof.digest(p) for p in sorted(root.rglob('*'))
        if p.is_file() and not p.is_symlink() and not p.is_relative_to(modules) and not p.is_relative_to(root / 'orchestrator') and not p.is_relative_to(root / 'planner')})
    write(root / 'summary.json', dict(status='passed', head=sha, invocations=741, projects=10, plans=30,
        release_eligible=False, platform_acceptance=False, attested=False))


def public_main(root, sha, options_path):
    """Consume an owner-terminal public run; never rebuild or enrich old proof."""
    repo = Path(__file__).resolve().parent.parent
    options = proof.read(options_path)
    proof.require(set(options) == {'request', 'nativeTap', 'nativeTapSha256', 'go', 'node', 'modCache'}, 'public runner options')
    request = options['request']
    proof.require(request.get('intake') == 'public-fixture/v1' and request['expectedCommit'] == sha, 'explicit public intake SHA')
    proof.require(proof.digest(options['nativeTap']) == options['nativeTapSha256'], 'public transcript pin')
    proof.public_tap(proof.data(options['nativeTap']).decode(), request)
    proof.require(re.fullmatch('[0-9a-f]{40}', sha), 'exact SHA required')
    proof.require(root.is_absolute() and root.resolve() == root and not root.is_relative_to(repo), 'external canonical output required')
    # Output must be disjoint before creating logs or planner homes.
    cfg = proof.read(request['nativeConfig']); candidate = cfg['prepare']['candidate']
    protected = [repo, Path(options_path), Path(options['nativeTap']), Path(request['nativeConfig']),
        Path(request['fixtureRoot']), Path(cfg['evidenceOutput']), Path(cfg['prepare']['output']),
        Path(candidate['root']), Path(candidate['pairMarker']), Path(candidate['workParent']),
        *[Path(options[k]) for k in ('go', 'node', 'modCache')], *map(Path, candidate['outputs'].values())]
    for other in protected:
        proof.require(not root.is_relative_to(other) and not other.is_relative_to(root), 'public output overlaps input')
    go, node, modules = [Path(options[k]).resolve(strict=True) for k in ('go', 'node', 'modCache')]
    proof.require(str(go) == candidate['go'] and str(node) == cfg['prepare']['node'], 'public tool paths differ')
    proof.require(platform.system() == 'Linux' and platform.machine() == 'x86_64', 'native Linux amd64 required')
    root.mkdir(mode=0o700); (root / 'logs').mkdir(); (root / 'bridge-config').mkdir()
    env = private_env(root / 'planner', go, node, repo, modules)
    env.update(GOPROXY='off', GOSUMDB='off', GOVCS='*:off')
    def run(name, argv, extra=None):
        argv = list(map(str, argv)); record = dict(argv=argv, cwd=str(repo), env=dict(env, **(extra or {})), exit=None)
        started = time.monotonic()
        try:
            with (root / 'logs' / (name + '.stdout')).open('x') as out, (root / 'logs' / (name + '.stderr')).open('x') as err:
                record['exit'] = subprocess.run(argv, cwd=repo, env=record['env'], stdout=out, stderr=err, timeout=1200).returncode
        finally:
            record['seconds'] = round(time.monotonic() - started, 3); write(root / 'logs' / (name + '.json'), record)
        proof.require(record['exit'] == 0, name + ' failed')
        return (root / 'logs' / (name + '.stdout')).read_text()
    proof.require(run('head', ['/usr/bin/git', 'rev-parse', 'HEAD']).strip() == sha, 'wrong checkout')
    proof.require(run('clean', ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all']) == '', 'dirty checkout')
    write(root / 'public-run.json', dict(schema='public-packed-run/v1', head=sha, options=options,
        tools={k: dict(path=str(p), sha256=proof.digest(p)) for k, p in dict(go=go, node=node).items()},
        release_eligible=False, platform_acceptance=False, attested=False))
    request_path = root / 'bridge-config/request.json'; write(request_path, request)
    sealed = root / 'bridge-config/sealed.json'
    bridge = repo / 'npm/agentplugins/scripts/packed-installer-bridge.js'
    printed = run('seal', [node, bridge, 'seal', request_path, sealed]).strip()
    proof.require(printed == proof.digest(sealed), 'public seal pin')
    planner(root, sha, go, node, sealed, printed, run)
    proof.require(run('terminal-clean', ['/usr/bin/git', 'status', '--porcelain=v1', '--untracked-files=all']) == '', 'checkout changed')
    proof.check_public(root, sha)
    write(root / 'summary.json', dict(status='passed', intake='public-fixture/v1', head=sha, projects=10, plans=30,
        release_eligible=False, platform_acceptance=False, attested=False, signed_promotion=False, public_eligible=False))


if __name__ == '__main__':
    if len(sys.argv) == 5 and sys.argv[1] == '--public':
        public_main(Path(sys.argv[2]), sys.argv[3], Path(sys.argv[4]))
    elif len(sys.argv) == 3:
        main(Path(sys.argv[1]), sys.argv[2])
    else:
        raise SystemExit('usage: run-packed-ci.py ROOT SHA | --public ROOT SHA OPTIONS')
