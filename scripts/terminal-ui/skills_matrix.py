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
import subprocess
import sys
import tempfile
import time
import uuid

from harness import Session, check, clean, prepare_scanner, scanner_options
from native_skills import compact
from skills_fixture import GITHUB_SOURCE, SOURCE_SHA, SkillsFixture, build_native_fixture
from skills_oracles import (durable_tree, envelope, failure_log, installed,
                            items_by_agent, native_mutations, persist)

CASES = ('source-feasibility', 'fresh', 'unchanged', 'unchanged-failure',
         'partial-json', 'partial-decline', 'partial-retry', 'no-targets',
         'review-decline', 'review-ctrl-c', 'review-eof', 'review-redirect',
         'catalog-redirect', 'plain-browser', 'explicit-ui')
BROWSER_CASES = frozenset(('catalog-redirect', 'plain-browser', 'explicit-ui'))
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
        source = self.source
        if source == 'auto':
            source = 'publisher' if sys.platform == 'linux' else 'github'
        if name in BROWSER_CASES:
            source = 'github'
        with tempfile.TemporaryDirectory(prefix='skills-console-é-') as temporary:
            fixture = SkillsFixture(temporary, self.native, self.scanner, source)
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
            manifest.write_bytes(reviewed_bytes + b'\n# changed after review\n')
            session.send(b'y\n')
            session.wait(RETRY, 'retry-ready')
            check('Needs attention' in clean(session.raw),
                  'unchanged failure has no per-agent attention')
            session.send(b'n\n')
            session.finish(1)
        finally:
            session.close()
        manifest.write_bytes(reviewed_bytes)
        check(durable_tree(fixture) == before, 'failed no-op changed durable installation')
        check(not native_mutations(fixture.calls()[calls:]),
              'failed no-op performed native mutation')
        logs = list((fixture.data / 'logs').glob('install-*.jsonl'))
        check(logs and any('codex' in log.read_text() for log in logs),
              'unchanged pre-apply failure was not logged')

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
        installed(fixture, ('cursor', 'codex') if retry else ('cursor',))

    def case_partial_decline(self, fixture, evidence):
        self.partial_interactive(fixture, evidence, False)

    def case_partial_retry(self, fixture, evidence):
        self.partial_interactive(fixture, evidence, True)

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


def selected_cases(cases, suite, platform):
    if cases:
        return cases
    selected = [name for name in CASES if suite == 'all' or
                (name in BROWSER_CASES) == (suite == 'browser')]
    if platform == 'win32':
        selected = [name for name in selected if name not in ('review-redirect', 'catalog-redirect')]
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
    if native is None:
        go = args.build_go or shutil.which('go')
        check(go is not None, '--native-fixture or Go toolchain is required')
        native = build_native_fixture(
            go, args.artifacts / ('native.exe' if os.name == 'nt' else 'native'))
    scanner = prepare_scanner(args)
    matrix = Matrix(binary, native.resolve(strict=True), args.timeout,
                    args.acquisition_timeout, args.source, args.scanner_path)
    try:
        head = subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip()
        dirty = bool(subprocess.check_output(['git', 'status', '--porcelain'], text=True))
    except subprocess.CalledProcessError:
        head, dirty = None, None
    report = dict(schema_version=1, product_sha=args.product_sha,
                  harness_sha=head, harness_dirty=dirty,
                  source_fixture_sha=SOURCE_SHA, source_mode=args.source,
                  binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(),
                  native_protocol='synthetic; no vendor/model/auth qualification',
                  scanner=scanner, platform=sys.platform, cases=[])
    for name in selected_cases(args.case, args.suite, sys.platform):
        row = dict(case=name, status='fail')
        started = time.monotonic()
        try:
            matrix.run_case(name, args.artifacts / name)
            check(hashlib.sha256(binary.read_bytes()).hexdigest() == report['binary_sha256'],
                  'input CLI binary changed during qualification')
            row['status'] = 'pass'
        except Exception as error:
            row['error'] = repr(error)
        row['elapsed_seconds'] = round(time.monotonic() - started, 3)
        report['cases'].append(row)
        persist(args.artifacts / 'report.json', report)
        print(json.dumps(row), flush=True)
    return int(any(row['status'] != 'pass' for row in report['cases']))


if __name__ == '__main__':
    raise SystemExit(main())
