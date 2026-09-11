#!/usr/bin/env python3
"""Opt-in native Windows resource oracle; portable checks are not native proof."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import tempfile
import time
import uuid

from harness import check, clean
from windows_conpty import ConPTY, finish_console

CASES = ('runtime-only-growth', 'balanced-plain', 'balanced-growth',
         'console-retained-plain', 'console-retained-growth',
         'thread-retained-plain', 'thread-retained-growth',
         'close-failure-plain', 'close-failure-growth', 'branches-plain')
MARKER = r'\bQUALIFICATION_NATIVE_RESOURCE_ORACLE_OK\b'
# Ten children each bounded by 15s, plus parent overhead. Separate from the
# original 30-cycle cancellation driver and its unchanged timeout.
SUITE_SECONDS = 180
PROBE_SECONDS = 15
TEST_ARGS = ['-test.run=^TestQualificationNativeResourceOracle$',
             '-test.v', '-test.timeout=180s']


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n', encoding='utf-8')


def runtime_environment(root, logs):
    env = {}
    for key in ('HOME', 'USERPROFILE', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME',
                'XDG_CACHE_HOME', 'XDG_STATE_HOME', 'XDG_RUNTIME_DIR',
                'APPDATA', 'LOCALAPPDATA', 'TMP', 'TEMP', 'PATH'):
        directory = root / key.lower()
        directory.mkdir()
        env[key] = str(directory)
    env.update(SystemRoot=os.environ['SystemRoot'], WINDIR=os.environ['SystemRoot'],
               UAP_WINDOWS_RESOURCE_ORACLE='1',
               UAP_WINDOWS_RESOURCE_ORACLE_LOG_DIR=str(logs))
    return env


def verify_logs(logs):
    files = sorted(logs.glob('*.log'))
    check(len(files) == len(CASES), 'expected exactly ten fresh child logs')
    result = {}
    for case in CASES:
        matches = list(logs.glob('resource-oracle-' + case + '-*.log'))
        check(len(matches) == 1, 'missing or duplicate child log: ' + case)
        path = matches[0]
        content = path.read_text(encoding='utf-8')
        check(re.search(r'(?m)^--- PASS: TestQualificationNativeResourceOracle \(', content)
              and re.search(r'(?m)^PASS\r?$', content)
              and not re.search(r'(?m)^(?:--- FAIL:|FAIL\b|panic:)', content),
              'child log lacks successful test completion: ' + case)
        result[case] = {'path': str(path), 'sha256': digest(path)}
    return result


def run_native(binary, root, evidence, result):
    logs = evidence / 'child-logs'
    logs.mkdir()
    env = runtime_environment(root, logs)
    nonce = uuid.uuid4().hex
    status_path, config_path = evidence / 'status.json', evidence / 'job.json'
    argv = [str(binary), *TEST_ARGS]
    result.update(test_command=argv, runtime_environment=env, fixture=str(root), nonce=nonce)
    write_json(config_path, dict(argv=argv, cwd=str(root), nonce=nonce, status=str(status_path)))
    owner = [sys.executable, '-I', str(Path(__file__).with_name('windows_job.py').resolve()),
             str(config_path)]
    result['owner_command'] = owner
    session = ConPTY(owner, env, str(root), PROBE_SECONDS, evidence=evidence)
    session.status_path = status_path
    try:
        session.wait('OWNER_READY_' + nonce)
        session.timeout = SUITE_SECONDS
        # The existing nonce-bound completion check handles final success and
        # RESTORE_READY arriving in one capture, and fails promptly on child exit.
        session.wait(MARKER, child_nonce=nonce, child_final=True)
        result['final_marker'] = True
        session.timeout = PROBE_SECONDS
        session.wait('RESTORE_READY_' + nonce)
        status = json.loads(status_path.read_text(encoding='utf-8'))
        check('error' not in status and status['phase'] == 'probe'
              and type(status['exit']) is int and status['exit'] == 0,
              'oracle parent did not exit successfully')
        result['child_exit'] = status['exit']
        result['child_logs'] = verify_logs(logs)
        check(status['owner_before'] == status['owner_after'], 'owner console not restored')
        offset = len(session.raw)
        session.send(('line_' + nonce + '\r').encode())
        session.wait('RESTORE_OK_' + nonce, after=offset)
        deadline = time.monotonic() + PROBE_SECONDS
        while session.poll() is None and time.monotonic() < deadline:
            time.sleep(0.02)
        check(session.poll() == 0, 'owner failed or hung')
        status = json.loads(status_path.read_text(encoding='utf-8'))
        check('error' not in status and status['phase'] == 'done'
              and status['line_read'] is True
              and status['owner_probe'] == status['owner_before'], 'owner console reuse failed')
        check('line_' + nonce in clean(session.raw[offset:]), 'owner kernel echo missing')
        result['owner_restoration_and_reuse'] = True
    finally:
        finish_console(session, status_path, evidence)
    check(not session.forced and session.error is None, 'cleanup forced or reader failed')
    result['non_forced_cleanup'] = True


def source_identity(repo):
    command = ['git', 'rev-parse', 'HEAD']
    commit = subprocess.check_output(command, cwd=repo, text=True, timeout=15).strip()
    paths = subprocess.check_output(['git', 'ls-files', '--', 'cli/plugin-kit-ai'],
                                    cwd=repo, text=True, timeout=15).splitlines()
    paths += ['scripts/terminal-ui/' + name for name in
              ('resource_oracle.py', 'windows_conpty.py', 'windows_job.py', 'harness.py')]
    paths += ['.github/workflows/terminal-ui.yml']
    return {'commit': commit, 'files_sha256': {p: digest(repo / p) for p in paths
                                             if (repo / p).is_file()}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default='go')
    parser.add_argument('--artifacts', type=Path, required=True)
    args = parser.parse_args()
    # Never reuse evidence from an earlier run.
    args.artifacts.mkdir(parents=True, exist_ok=False)
    evidence = args.artifacts.resolve()
    repo = Path(__file__).resolve().parents[2]
    result = dict(passed=False, native_windows=os.name == 'nt', platform=platform.platform(),
                  python=sys.version, suite_timeout_seconds=SUITE_SECONDS, test_only=True)
    started = time.monotonic()
    try:
        check(os.name == 'nt', 'native Windows required; no portable execution claim')
        result['source'] = source_identity(repo)
        binary = evidence / 'promptio.test.exe'
        command = [args.go, 'test', '-c', '-o', str(binary), './internal/promptio']
        result['build_command'] = command
        # Build against source; the runtime receives none of this environment.
        build_env = dict(os.environ, GOWORK='off', GOTOOLCHAIN='local', CGO_ENABLED='0',
                         GOPROXY='off', GOSUMDB='off')
        with (evidence / 'build.log').open('wb') as log:
            build = subprocess.run(command, cwd=repo / 'cli/plugin-kit-ai', env=build_env,
                                   stdout=log, stderr=subprocess.STDOUT, timeout=180)
        check(build.returncode == 0, 'test executable build failed; see build.log')
        result['test_binary_sha256'] = digest(binary)
        check(source_identity(repo) == result['source'], 'source changed during build')
        with tempfile.TemporaryDirectory(prefix='uap-resource-oracle-') as temp:
            run_native(binary, Path(temp).resolve(), evidence, result)
        result['fixture_removed'] = True
        result['passed'] = True
    except Exception as exc:
        result['error'] = repr(exc)
    finally:
        result['elapsed_seconds'] = round(time.monotonic() - started, 3)
        write_json(evidence / 'results.json', result)
    print(json.dumps({key: result[key] for key in ('passed', 'native_windows', 'elapsed_seconds')}
                     | {'results': str(evidence / 'results.json'), 'error': result.get('error')}))
    return 0 if result['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
