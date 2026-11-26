#!/usr/bin/env python3
"""Public Skills process qualification using existing PTY/ConPTY ownership.

Missing source access or platform prerequisites fail the requested case. Native
responders are synthetic protocols, not vendor/model/authentication evidence.
The lifecycle suite is cross-platform. Browser cases require production assets.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import select
import signal
import subprocess
import sys
import tempfile
import time
import uuid

from harness import Session, check, clean, prepare_scanner, scanner_options
from native_skills import compact
from skills_fixture import (GITHUB_SOURCE, NATIVE_SOURCE, SOURCE_SHA, SkillsFixture,
                            build_native_fixture)
from skills_oracles import (durable_tree, envelope, failure_log, installed,
                            items_by_agent, native_mutations, persist)

CASES = ('source-feasibility', 'fresh', 'unchanged', 'unchanged-failure',
         'partial-json', 'partial-decline', 'partial-retry', 'log-unavailable', 'recovery-refusal', 'no-targets',
         'review-decline', 'review-ctrl-c', 'review-eof', 'review-redirect',
         'catalog-redirect', 'catalog-both-redirect', 'plain-browser', 'explicit-ui')
BROWSER_CASES = frozenset(('catalog-redirect', 'catalog-both-redirect', 'plain-browser', 'explicit-ui'))
QUESTION = r'Install these Skills\? \[Y/n\] '
RETRY = r'Retry [^\r\n]*\? \[Y/n\] '


class WindowsSession:
    """Reuse native ConPTY owner and its same-console restoration oracle."""
    def __init__(self, argv, fixture, evidence, timeout):
        from windows_conpty import ConPTY
        self.fixture, self.evidence, self.timeout = fixture, evidence, timeout
        self.nonce = uuid.uuid4().hex
        self.status_path = evidence / 'status.json'
        config = dict(argv=argv, cwd=str(fixture.project), nonce=self.nonce,
                      status=str(self.status_path))
        config_path = evidence / 'job.json'
        persist(config_path, config)
        env = dict(fixture.env)
        env.pop('TERM', None)
        self.terminal = ConPTY(
            [sys.executable, '-I', str(Path(__file__).with_name('windows_job.py').resolve()),
             str(config_path)], env, fixture.project, timeout, evidence=evidence)
        self.terminal.status_path = self.status_path
        self.terminal.wait('OWNER_READY_' + self.nonce)

    @property
    def raw(self):
        return self.terminal.raw

    def wait(self, pattern, _label, after=0):
        self.terminal.timeout = self.timeout
        return self.terminal.wait(pattern, after=after)

    def send(self, keys):
        self.terminal.send(keys.replace(b'\n', b'\r'))

    def finish(self, expected=0):
        terminal = self.terminal
        terminal.timeout = self.timeout
        terminal.wait('RESTORE_READY_' + self.nonce)
        status = json.loads(self.status_path.read_text(encoding='utf-8'))
        check(status['exit'] == expected, f'CLI exit {status["exit"]}, expected {expected}')
        check(status['owner_before'] == status['owner_after'], 'owner console changed')
        check(status['before'] == status['after'], 'child console modes changed')
        check(status['cursor_before'] and status['cursor_after'], 'cursor not restored')
        offset = len(terminal.raw)
        terminal.send(('line_' + self.nonce + '\r').encode())
        terminal.wait('RESTORE_OK_' + self.nonce, after=offset)
        deadline = time.monotonic() + self.timeout
        while terminal.poll() is None and time.monotonic() < deadline:
            time.sleep(0.02)
        check(terminal.poll() == 0, 'native console owner did not exit')
        status = json.loads(self.status_path.read_text(encoding='utf-8'))
        check(status['line_read'] and status['probe_modes'] == status['before'],
              'same-console canonical input reuse failed')
        check(status['owner_probe'] == status['owner_before'],
              'owner handles/modes changed on reuse')
        check('line_' + self.nonce in clean(terminal.raw[offset:]), 'kernel echo missing')

    def close(self):
        from windows_conpty import finish_console
        finish_console(self.terminal, self.status_path, self.evidence, self.fixture)
        check(not self.terminal.forced, 'forced native cleanup cannot qualify a case')


class Matrix:
    def __init__(self, binary, native, timeout=20, acquisition_timeout=180,
                 source='auto', scanner=None):
        self.binary, self.native = Path(binary), Path(native)
        self.timeout, self.acquisition_timeout = timeout, acquisition_timeout
        self.source, self.scanner = source, scanner

    def command(self, fixture, evidence, label, targets='codex', expect=0):
        argv = fixture.args(self.binary, targets=targets)
        started = time.monotonic()
        result = subprocess.run(argv, cwd=fixture.project, env=fixture.env,
                                stdin=subprocess.DEVNULL, capture_output=True,
                                timeout=self.acquisition_timeout)
        (evidence / (label + '.stdout')).write_bytes(result.stdout)
        (evidence / (label + '.stderr')).write_bytes(result.stderr)
        persist(evidence / (label + '.command.json'), dict(
            argv=argv, exit=result.returncode,
            elapsed_seconds=round(time.monotonic() - started, 3)))
        check(result.returncode == expect,
              f'{label}: exit {result.returncode}, expected {expect}; see captured stdout/stderr')
        return envelope(result.stdout, result.returncode)

    def session(self, argv, fixture, evidence, redirect=None):
        persist(evidence / 'argv.json', argv)
        if os.name == 'nt':
            check(redirect is None, 'redirect case requires Unix descriptors')
            return WindowsSession(argv, fixture, evidence, self.acquisition_timeout)
        return Session(argv, fixture, evidence, self.acquisition_timeout,
                       redirect=redirect, rows=35, cols=120)

    def review(self, session, fixture):
        session.wait(QUESTION, 'review-ready')
        session.timeout = self.timeout
        fixture.unchanged()
        text = compact(clean(session.raw))
        check(fixture.skill_name in text, 'review omitted selected Skill identity')
        check(compact(str(fixture.root)) in text, 'review omitted complete destination authority')

    def run_case(self, name, evidence):
        evidence.mkdir()
        source = source_for_case(name, self.source, sys.platform)
        if name == 'recovery-refusal':
            check(sys.platform == 'linux', 'recovery process identity fixture requires Linux pidfd')
        with tempfile.TemporaryDirectory(prefix='skills-console-é-') as temporary:
            fixture = SkillsFixture(temporary, self.native, self.scanner, source)
            source_info = dict(source_kind=fixture.source_kind,
                               requested_source=fixture.source)
            if fixture.publisher:
                source_info['publisher_files_sha256'] = {
                    name: hashlib.sha256(body).hexdigest()
                    for name, body in fixture.publisher.files.items()}
            else:
                source_info['github_fixture_sha'] = SOURCE_SHA
            persist(evidence / 'source.json', source_info)
            try:
                method = getattr(self, 'case_' + name.replace('-', '_'))
                method(fixture, evidence)
            finally:
                try:
                    persist(evidence / 'native-calls.json', fixture.calls())
                    persist(evidence / 'durable-state.json', durable_tree(fixture))
                    for filename in ('state-v2.json', 'uap.toml', 'uap.lock'):
                        path = fixture.data / filename
                        if path.is_file():
                            shutil.copyfile(path, evidence / filename)
                    logs = fixture.data / 'logs'
                    if logs.is_dir():
                        saved_logs = evidence / 'diagnostics'
                        saved_logs.mkdir()
                        for log in logs.glob('install-*.jsonl'):
                            check(not log.is_symlink(), 'diagnostic file unexpectedly became a symlink')
                            shutil.copyfile(log, saved_logs / log.name)
                    if fixture.publisher:
                        persist(evidence / 'publisher-requests.json', fixture.publisher.requests)
                finally:
                    fixture.close()

    def case_source_feasibility(self, fixture, evidence):
        result = self.command(fixture, evidence, 'acquire-and-install', targets='cursor')
        check(result['readiness'] == 'needs_action',
              'synthetic editor fixture overclaimed readiness')
        installed(fixture, ('cursor',))
        if fixture.publisher:
            check(any(path.endswith('/SKILL.md') for path in fixture.publisher.requests),
                  'public binary did not acquire original Skill bytes')

    def case_fresh(self, fixture, evidence):
        result = self.command(fixture, evidence, 'fresh', targets='codex,cursor')
        check(set(items_by_agent(result)) == {'codex', 'cursor'}, 'lost a selected agent')
        installed(fixture, ('codex', 'cursor'))
        check(any(args[:2] == ['plugin', 'add'] for args in fixture.calls()),
              'native installation protocol was not exercised')

    def case_unchanged(self, fixture, evidence):
        self.command(fixture, evidence, 'seed')
        installed(fixture, ('codex',))
        before, calls = durable_tree(fixture), len(fixture.calls())
        result = self.command(fixture, evidence, 'repeat')
        check(items_by_agent(result)['codex']['status'] == 'unchanged',
              'repeat lost no-op status')
        check(result.get('readiness') == 'ready', 'repeat lacks current native readiness evidence')
        check(['app-server'] in fixture.calls()[calls:], 'repeat did not observe native Skills')
        check(durable_tree(fixture) == before, 'unchanged repeat rewrote durable bytes')
        check(not native_mutations(fixture.calls()[calls:]),
              'unchanged repeat mutated native registry')

    def case_unchanged_failure(self, fixture, evidence):
        self.command(fixture, evidence, 'seed')
        installed(fixture, ('codex',))
        before, calls = durable_tree(fixture), len(fixture.calls())
        fixture.before = fixture.mutations()
        argv = fixture.args(self.binary, yes=False, json_output=False)
        session = self.session(argv + ['--plain', '--color=never'], fixture, evidence)
        try:
            self.review(session, fixture)
            # A changed input must refuse before publication, including no-op
            # actions. Native observation unavailability alone is intentionally
            # a successful unchanged result with unverified readiness.
            manifest = fixture.data / 'uap.toml'
            reviewed_bytes = manifest.read_bytes()
            tampered_bytes = reviewed_bytes + b'\n# changed after review\n'
            manifest.write_bytes(tampered_bytes)
            tampered_state = durable_tree(fixture)
            session.send(b'y\n')
            session.wait(RETRY, 'retry-ready')
            check('Needs attention' in clean(session.raw),
                  'unchanged failure has no per-agent attention')
            session.send(b'n\n')
            session.finish(1)
        finally:
            session.close()
        check(manifest.read_bytes() == tampered_bytes, 'refused command clobbered changed input')
        check(durable_tree(fixture) == tampered_state, 'refused command changed durable bytes')
        manifest.write_bytes(reviewed_bytes)
        check(durable_tree(fixture) == before, 'failed no-op changed durable installation')
        check(not native_mutations(fixture.calls()[calls:]),
              'failed no-op performed native mutation')
        logs = list((fixture.data / 'logs').glob('install-*.jsonl'))
        check(logs, 'unchanged pre-apply failure was not logged')
        failure_log({'log_file': str(logs[0]), 'items': [
            {'targetId': 'codex', 'alias': fixture.skill_name, 'status': 'failed'}]}, fixture)

    def case_partial_json(self, fixture, evidence):
        fixture.control('fail-add')
        result = self.command(fixture, evidence, 'partial', targets='codex,cursor', expect=1)
        rows = items_by_agent(result)
        check(rows['codex']['status'] == 'failed', 'native failure labeled successful')
        check(rows['cursor']['status'] == 'succeeded', 'healthy independent binding lost')
        installed(fixture, ('cursor',))
        failure_log(result, fixture)

    def partial_interactive(self, fixture, evidence, retry):
        fixture.control('fail-add')
        argv = fixture.args(self.binary, targets='codex,cursor', yes=False, json_output=False)
        session = self.session(argv + ['--plain', '--color=never'], fixture, evidence)
        try:
            self.review(session, fixture)
            session.send(b'y\n')
            session.wait(RETRY, 'retry-ready')
            check('Needs attention' in clean(session.raw), 'partial result is not explained')
            at_retry = len(fixture.calls())
            healthy = installed(fixture, ('cursor',))['cursor']
            session.send(b'y\n' if retry else b'n\n')
            session.finish(0 if retry else 1)
            if retry:
                after = fixture.calls()[at_retry:]
                check(sum(args[:2] == ['plugin', 'add'] for args in after) == 1,
                      'retry did not install exactly the failed native binding')
                check('Retrying 1 agent(s)' in clean(session.raw), 'retry targeting not announced')
            else:
                check(not native_mutations(fixture.calls()[at_retry:]),
                      'declined retry performed effects')
        finally:
            session.close()
        final = installed(fixture, ('cursor', 'codex') if retry else ('cursor',))
        check(final['cursor'] == healthy, 'retry rewrote the already healthy binding or receipts')

    def case_partial_decline(self, fixture, evidence):
        self.partial_interactive(fixture, evidence, False)

    def case_partial_retry(self, fixture, evidence):
        self.partial_interactive(fixture, evidence, True)

    def case_log_unavailable(self, fixture, evidence):
        # A regular file is portable ENOTDIR evidence, including as root. Do not
        # rely on chmod permissions, which differ on Windows and privileged hosts.
        blocked = fixture.data / 'logs'
        blocked.write_bytes(b'diagnostic-directory-obstructed-by-owned-fixture\n')
        fixture.control('fail-add')
        argv = fixture.args(self.binary, targets='codex,cursor', yes=False, json_output=False)
        session = self.session(argv + ['--plain', '--color=never'], fixture, evidence)
        try:
            self.review(session, fixture)
            session.send(b'y\n')
            session.wait(RETRY, 'retry-after-log-failure')
            text = clean(session.raw)
            check('Needs attention' in text, 'log failure removed per-agent explanation')
            check('Technical details could not be saved.' in text,
                  'diagnostic save failure was not disclosed')
            check('Failure codes:' in text, 'missing-log fallback lost classified failure codes')
            check('Technical details: ' not in text, 'failure pointed to a nonexistent log')
            check('synthetic native protocol refused operation' not in text,
                  'missing-log fallback exposed raw provider output')
            at_retry = len(fixture.calls())
            session.send(b'n\n')
            session.finish(1)
            check(not native_mutations(fixture.calls()[at_retry:]),
                  'declining retry after log failure performed another mutation')
        finally:
            session.close()
        check(blocked.read_bytes() == b'diagnostic-directory-obstructed-by-owned-fixture\n',
              'diagnostic reporting overwrote the obstructing file')
        installed(fixture, ('cursor',))

    def case_recovery_refusal(self, fixture, evidence):
        # Hold exact native child identity before crashing the CLI. The responder
        # does not spawn children, so a pidfd covers the sole active native effect.
        # No process-group ancestry assumption or fabricated journal is used.
        check(sys.platform == 'linux' and hasattr(os, 'pidfd_open') and
              hasattr(signal, 'pidfd_send_signal'), 'Linux pidfd support is required')
        nonce = uuid.uuid4().hex
        fixture.control('pause-add', nonce)
        argv = fixture.args(self.binary)
        persist(evidence / 'crash-command.json', argv)
        process = subprocess.Popen(argv, cwd=fixture.project, env=fixture.env,
                                   stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE)
        child_fd, child_owned = None, False
        primary = None
        try:
            marker = fixture.root / 'native-add-paused'
            deadline = time.monotonic() + self.acquisition_timeout
            while not marker.exists() and process.poll() is None:
                check(time.monotonic() < deadline, 'native effect marker was not reached')
                time.sleep(0.02)
            check(process.poll() is None, 'CLI exited before native effect pause')
            identity = json.loads(marker.read_text(encoding='utf-8'))
            check(identity.get('nonce') == nonce and
                  identity.get('ppid') == process.pid and
                  isinstance(identity.get('pid'), int) and identity['pid'] > 1,
                  'native marker does not identify this owned CLI child')
            child_fd = os.pidfd_open(identity['pid'])
            actual_executable = Path(os.readlink(f'/proc/{identity["pid"]}/exe')).resolve()
            expected_executable = (fixture.bin / 'codex').resolve()
            check(actual_executable == expected_executable and
                  Path(identity['executable']).resolve() == expected_executable,
                  'native marker PID is not this fixture executable')
            child_owned = True
            poller = select.poll()
            poller.register(child_fd, select.POLLIN)
            check(not poller.poll(0), 'native fixture exited before crash control')
            persist(evidence / 'owned-native-identity.json', identity)
            process.kill()  # Popen owns and reaps this exact CLI process.
            try:
                signal.pidfd_send_signal(child_fd, signal.SIGKILL)
            except ProcessLookupError:
                pass  # Parent-death cleanup can have already ended this child.
            check(poller.poll(3000), 'owned native process did not exit')
            stdout, stderr = process.communicate(timeout=3)
            (evidence / 'crash.stdout').write_bytes(stdout)
            (evidence / 'crash.stderr').write_bytes(stderr)
            check(process.returncode == -signal.SIGKILL, 'CLI crash control did not execute')
        except BaseException as error:
            primary = error
            raise
        finally:
            cleanup_errors = []
            if child_fd is not None:
                try:
                    if child_owned:
                        try:
                            signal.pidfd_send_signal(child_fd, signal.SIGKILL)
                        except ProcessLookupError:
                            pass
                        poller = select.poll()
                        poller.register(child_fd, select.POLLIN)
                        check(poller.poll(3000), 'native child cleanup could not be confirmed')
                except Exception as error:
                    cleanup_errors.append(repr(error))
                finally:
                    os.close(child_fd)
            try:
                if process.poll() is None:
                    process.kill()
                process.communicate(timeout=3)
            except Exception as error:
                cleanup_errors.append(repr(error))
            persist(evidence / 'crash-cleanup.json', dict(
                cli_reaped=process.poll() is not None, native_identity_confirmed=child_owned,
                cleanup_errors=cleanup_errors))
            if cleanup_errors:
                raise RuntimeError(f'{primary!r}; uncertain fixture cleanup: {cleanup_errors}')
        operations = fixture.data / 'operations-v2'
        journals = list(operations.glob('*.json'))
        check(journals, 'crash did not produce an actual installer operation journal')
        for journal in journals:
            # Retain the exact product-generated journal, without fabricating or
            # rewriting its state schema to make refusal occur.
            shutil.copyfile(journal, evidence / ('interrupted-' + journal.name))
        before, calls = durable_tree(fixture), len(fixture.calls())
        result = self.command(fixture, evidence, 'recovery-refused', expect=1)
        check(result.get('code') == 'RECOVERY_REQUIRED' or
              result.get('error', {}).get('code') == 'RECOVERY_REQUIRED',
              'unfinished native operation did not produce a typed recovery refusal')
        check(not result.get('error', {}).get('retryable', False),
              'recovery refusal offered an ordinary retry')
        check(durable_tree(fixture) == before, 'recovery refusal changed interrupted state')
        check(not native_mutations(fixture.calls()[calls:]),
              'unfinished recovery started another native mutation')

    def canceled_review(self, fixture, evidence, action, redirect=None):
        argv = fixture.args(self.binary, targets='cursor', yes=False, json_output=False)
        session = self.session(argv + ['--plain', '--color=never'],
                               fixture, evidence, redirect=redirect)
        try:
            self.review(session, fixture)
            session.send(action)
            session.finish(0 if action == b'n\n' else 1)
            fixture.unchanged()
            check(not native_mutations(fixture.calls()),
                  'canceled review invoked native mutation')
        finally:
            session.close()

    def case_review_decline(self, fixture, evidence):
        self.canceled_review(fixture, evidence, b'n\n')

    def case_review_ctrl_c(self, fixture, evidence):
        self.canceled_review(fixture, evidence, b'\x03')

    def case_review_eof(self, fixture, evidence):
        self.canceled_review(fixture, evidence, b'\x1a\r' if os.name == 'nt' else b'\x04')

    def case_review_redirect(self, fixture, evidence):
        self.canceled_review(fixture, evidence, b'n\n', redirect='stdout')

    def case_no_targets(self, fixture, evidence):
        for name in ('.codex', '.cursor'):
            shutil.rmtree(fixture.home / name)
        for path in fixture.bin.iterdir():
            path.unlink()
        if os.name == 'nt':
            editor = Path(fixture.env['LOCALAPPDATA']) / 'Programs/cursor'
            shutil.rmtree(editor)
        fixture.before = fixture.mutations()
        argv = [str(self.binary), 'skills', 'add', fixture.source,
                *fixture.source_flags, '--plain', '--no-ui', '--color=never']
        session = self.session(argv, fixture, evidence)
        try:
            session.finish(1)
            text = clean(session.raw).lower()
            check('no eligible detected agent' in text, 'no-target refusal lost clear guidance')
            check('not detected' in text or 'not installed' in text,
                  'no-target refusal lost target reasons')
            check('install these skills?' not in text, 'no eligible target reached consent')
            fixture.unchanged()
        finally:
            session.close()

    def catalog(self, fixture, evidence, redirect=None):
        argv = [str(self.binary), 'skills', 'add', GITHUB_SOURCE, '--plain', '--color=never']
        if redirect:
            argv += ['--target', 'cursor']
        session = self.session(argv, fixture, evidence, redirect=redirect)
        try:
            session.wait(r'q or Enter cancels', 'visible-catalog')
            session.timeout = self.timeout
            fixture.unchanged()
            check('Available Skills' in clean(session.raw), 'numeric catalog not visible before input')
            check(not (fixture.root / 'browser-opened').exists(), '--plain opened automatic browser')
            session.send(b'q\n')
            session.finish(1)
            fixture.unchanged()
        finally:
            session.close()

    def case_catalog_redirect(self, fixture, evidence):
        self.catalog(fixture, evidence, redirect='stdout')

    def case_catalog_both_redirect(self, fixture, evidence):
        argv = [str(self.binary), 'skills', 'add', GITHUB_SOURCE,
                '--plain', '--no-ui', '--color=never']
        session = self.session(argv, fixture, evidence, redirect='both')
        try:
            # No input is ever sent to the CLI. The existing restoration probe
            # runs only after actual process exit and reuses the same descriptor.
            session.finish(1)
            fixture.unchanged()
            output = ((evidence / 'stdout.raw').read_bytes() +
                      (evidence / 'stderr.raw').read_bytes()).decode('utf-8', 'replace')
            check('Available Skills' not in output and 'Select: ' not in output,
                  'unavailable visible terminal still entered the hidden catalog')
            check(any(word in output.lower() for word in
                      ('terminal', 'non-interactive', '--skill', '--path', 'prompt')),
                  'unavailable terminal refusal lost actionable guidance')
            check(not native_mutations(fixture.calls()), 'hidden catalog refusal changed native state')
        finally:
            session.close()

    def case_plain_browser(self, fixture, evidence):
        install_opener(fixture, self.native)
        self.catalog(fixture, evidence)

    def case_explicit_ui(self, fixture, evidence):
        install_opener(fixture, self.native)
        argv = [str(self.binary), 'skills', 'add', GITHUB_SOURCE, '--plain', '--ui']
        session = self.session(argv, fixture, evidence)
        try:
            session.wait(r'Skills picker opened', 'bundled-explicit-ui')
            session.timeout = self.timeout
            check((fixture.root / 'browser-opened').is_file(), 'explicit --ui did not call opener')
            fixture.unchanged()
            session.send(b'\x03')
            session.finish(1)
            fixture.unchanged()
        finally:
            session.close()


def install_opener(fixture, native):
    name = {'darwin': 'open', 'win32': 'rundll32.exe'}.get(sys.platform, 'xdg-open')
    shutil.copy2(native, fixture.bin / name)
    (fixture.bin / name).chmod(0o700)


def source_for_case(name, requested, platform):
    if name in BROWSER_CASES:
        return 'github'
    if requested != 'auto':
        return requested
    return 'publisher' if platform == 'linux' else 'github'


def selected_cases(cases, suite, platform):
    if cases:
        return cases
    selected = [name for name in CASES if suite == 'all' or
                (name in BROWSER_CASES) == (suite == 'browser')]
    if platform != 'linux':
        selected = [name for name in selected if name != 'recovery-refusal']
    if platform == 'win32':
        selected = [name for name in selected if name not in ('review-redirect', 'catalog-redirect', 'catalog-both-redirect')]
    return selected


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--product-sha', required=True,
                        help='exact source revision used to build the supplied CLI')
    parser.add_argument('--artifacts', required=True, type=Path)
    parser.add_argument('--native-fixture', type=Path)
    parser.add_argument('--build-go', type=Path)
    parser.add_argument('--source', choices=('auto', 'publisher', 'github'), default='auto')
    parser.add_argument('--case', action='append', choices=CASES)
    parser.add_argument('--suite', choices=('lifecycle', 'browser', 'all'), default='lifecycle')
    parser.add_argument('--timeout', type=float, default=20)
    parser.add_argument('--acquisition-timeout', type=float, default=180)
    scanner_options(parser)
    args = parser.parse_args()
    check(len(args.product_sha) == 40 and all(c in '0123456789abcdef' for c in args.product_sha),
          '--product-sha must be the full lowercase build revision')
    check(args.timeout > 0 and args.acquisition_timeout > 0, 'deadlines must be positive')
    args.artifacts = args.artifacts.resolve()
    args.artifacts.mkdir(parents=True, exist_ok=False)
    binary = args.binary.resolve(strict=True)
    native = args.native_fixture
    native_origin = 'supplied' if native is not None else 'built'
    if native is None:
        go = args.build_go or shutil.which('go')
        check(go is not None, '--native-fixture or Go toolchain is required')
        native = build_native_fixture(
            go, args.artifacts / ('native.exe' if os.name == 'nt' else 'native'))
    native = native.resolve(strict=True)
    scanner = prepare_scanner(args)
    matrix = Matrix(binary, native, args.timeout,
                    args.acquisition_timeout, args.source, args.scanner_path)
    try:
        head = subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip()
        dirty = bool(subprocess.check_output(['git', 'status', '--porcelain'], text=True))
    except subprocess.CalledProcessError:
        head, dirty = None, None
    report = dict(schema_version=1, product_sha=args.product_sha,
                  harness_sha=head, harness_dirty=dirty,
                  github_fixture_sha=SOURCE_SHA, source_mode=args.source,
                  binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(),
                  native_fixture_path=str(native), native_fixture_origin=native_origin,
                  native_fixture_sha256=hashlib.sha256(native.read_bytes()).hexdigest(),
                  native_fixture_embedded_source_sha256=hashlib.sha256(
                      NATIVE_SOURCE.encode('utf-8')).hexdigest(),
                  native_protocol='synthetic; no vendor/model/auth qualification',
                  scanner=scanner, platform=sys.platform, cases=[])
    for name in selected_cases(args.case, args.suite, sys.platform):
        row = dict(case=name, status='fail',
                   source_kind=source_for_case(name, args.source, sys.platform))
        started = time.monotonic()
        try:
            matrix.run_case(name, args.artifacts / name)
            check(hashlib.sha256(binary.read_bytes()).hexdigest() == report['binary_sha256'],
                  'input CLI binary changed during qualification')
            check(hashlib.sha256(native.read_bytes()).hexdigest() == report['native_fixture_sha256'],
                  'input native fixture binary changed during qualification')
            row['status'] = 'pass'
        except Exception as error:
            row['error'] = repr(error)
        source_path = args.artifacts / name / 'source.json'
        if source_path.is_file():
            row.update(json.loads(source_path.read_text(encoding='utf-8')))
        row['elapsed_seconds'] = round(time.monotonic() - started, 3)
        report['cases'].append(row)
        persist(args.artifacts / 'report.json', report)
        print(json.dumps(row), flush=True)
    return int(any(row['status'] != 'pass' for row in report['cases']))


if __name__ == '__main__':
    raise SystemExit(main())
