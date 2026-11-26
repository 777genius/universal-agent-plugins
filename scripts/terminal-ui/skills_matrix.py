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
import stat
import subprocess
import sys
import tempfile
import time
import uuid

from harness import Session, check, clean, hashes, prepare_scanner, scanner_options
from native_skills import compact
from skills_fixture import (GITHUB_SOURCE, NATIVE_SOURCE, SOURCE_SHA, SkillsFixture,
                            build_native_fixture)
from skills_oracles import (durable_tree, envelope, failure_log, installed,
                            items_by_agent, native_mutations, persist)

CASES = ('source-feasibility', 'fresh', 'declared-repair-json', 'unchanged', 'unchanged-failure',
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


def linux_process_identity(pid):
    """Observe the same PID/start-time boundary used by the Linux OSRunner."""
    root = Path('/proc') / str(pid)
    check(root.stat().st_uid == os.geteuid(), 'native lineage has a foreign process owner')
    body = (root / 'stat').read_bytes()
    check(len(body) <= 4096, 'native lineage proc identity exceeded its bound')
    opening, closing = body.find(b'('), body.rfind(b')')
    check(opening > 0 and closing > opening and body[:opening].strip() == str(pid).encode(),
          'native lineage has a malformed proc identity')
    fields = body[closing + 1:].split()
    check(len(fields) >= 20 and fields[0] in (b'R', b'S', b'D', b'T', b't', b'W', b'K', b'P', b'I'),
          'native lineage process is not live')
    parent, started = int(fields[1]), int(fields[19])
    check(parent > 0 and started > 0, 'native lineage has no stable parent/start time')
    executable = (root / 'exe').resolve(strict=True)
    info = (root / 'exe').stat()
    return dict(pid=pid, ppid=parent, start_time=started, executable=str(executable),
                executable_identity=[info.st_dev, info.st_ino])


def linux_pidfd_exited(fd):
    poller = select.poll()
    poller.register(fd, select.POLLIN)
    return bool(poller.poll(0))


def linux_supervisor_protocol(pid):
    root = Path('/proc') / str(pid)
    for name, limit in (('cmdline', 4096), ('environ', 1 << 20)):
        with (root / name).open('rb') as stream:
            body = stream.read(limit + 1)
        check(len(body) <= limit, 'native supervisor protocol evidence exceeded its bound')
        if name == 'cmdline':
            check(body == b'/proc/self/exe\0', 'native parent is not the exact OSRunner reexec')
        else:
            check(body.split(b'\0').count(b'PLUGIN_KIT_AI_ORDINARY_PROCESS_SUPERVISOR=1') == 1,
                  'native parent lacks the exact OSRunner supervisor protocol')


def hold_native_lineage(process, identity, nonce, native, binary):
    """Pin only the known CLI -> OSRunner supervisor -> responder topology."""
    check(identity.get('nonce') == nonce and
          type(identity.get('pid')) is int and identity['pid'] > 1 and
          type(identity.get('ppid')) is int and identity['ppid'] > 1,
          'native marker does not identify this owned CLI child')
    check(Path(identity.get('executable', '')).resolve() == native.resolve(),
          'native marker PID is not this fixture executable')
    held = []
    try:
        check(process.poll() is None, 'CLI exited before native lineage acquisition')
        anchor = linux_process_identity(process.pid)
        binary_info = binary.stat()
        check(anchor['executable'] == str(binary.resolve()) and
              anchor['executable_identity'] == [binary_info.st_dev, binary_info.st_ino],
              'owned CLI anchor is not the qualification executable')
        for role, pid, expected in (('native', identity['pid'], native),
                                    ('supervisor', identity['ppid'], binary)):
            before = linux_process_identity(pid)
            fd = os.pidfd_open(pid)
            held.append(dict(role=role, fd=fd, identity=before))
            after = linux_process_identity(pid)
            check(before == after and not linux_pidfd_exited(fd),
                  'native lineage changed while its stable handle was acquired')
            expected_info = expected.stat()
            check(after['executable'] == str(expected.resolve()) and
                  after['executable_identity'] == [expected_info.st_dev, expected_info.st_ino],
                  'native lineage executable is not the exact fixture/CLI inode')
        native_row, supervisor_row = [row['identity'] for row in held]
        check(native_row['ppid'] == identity['ppid'] and
              supervisor_row['ppid'] == process.pid and
              len({process.pid, native_row['pid'], supervisor_row['pid']}) == 3,
              'native lineage does not reach the owned CLI through one supervisor')
        linux_supervisor_protocol(supervisor_row['pid'])
        check(process.poll() is None and linux_process_identity(process.pid) == anchor,
              'owned CLI anchor changed during lineage acquisition')
        for row in held:
            check(linux_process_identity(row['identity']['pid']) == row['identity'] and
                  not linux_pidfd_exited(row['fd']),
                  'native lineage changed before crash control')
        return held
    except BaseException:
        for row in held:
            os.close(row['fd'])
        raise


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
        before = durable_tree(fixture)
        if os.name == 'nt':
            check(redirect is None, 'redirect case requires Unix descriptors')
            session = WindowsSession(argv, fixture, evidence, self.acquisition_timeout)
        else:
            session = Session(argv, fixture, evidence, self.acquisition_timeout,
                              redirect=redirect, rows=35, cols=120)
        session.durable_before = before
        return session

    def assert_unapplied(self, session, fixture):
        fixture.unchanged()
        check(durable_tree(fixture) == session.durable_before,
              'unconfirmed or canceled interaction changed durable saved intent/state')

    def review(self, session, fixture):
        session.wait(QUESTION, 'review-ready')
        session.timeout = self.timeout
        self.assert_unapplied(session, fixture)
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
            try:
                source_info = dict(source_kind=fixture.source_kind,
                                   requested_source=fixture.source)
                if fixture.publisher:
                    source_info['publisher_files_sha256'] = {
                        name: hashlib.sha256(body).hexdigest()
                        for name, body in fixture.publisher.files.items()}
                else:
                    source_info['github_fixture_sha'] = SOURCE_SHA
                persist(evidence / 'source.json', source_info)
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
        rows = items_by_agent(result)
        check(set(rows) == {'codex', 'cursor'}, 'lost a selected agent')
        check(all(row.get('status') == 'succeeded' for row in rows.values()),
              'fresh installation did not report success for every selected agent')
        installed(fixture, ('codex', 'cursor'))
        check(any(args[:2] == ['plugin', 'add'] for args in fixture.calls()),
              'native installation protocol was not exercised')

    def case_declared_repair_json(self, fixture, evidence):
        # Skills add saves a real normalized standard plugin and agreement. Its
        # missing projection takes the same declared-install repair path as an
        # authored plugin, without adding source infrastructure or native calls.
        self.command(fixture, evidence, 'seed-saved-plugin', targets='cursor')
        before_binding = installed(fixture, ('cursor',))['cursor']
        target = Path(before_binding['target_locator'])
        check(target.is_dir() and not target.is_symlink() and
              target.resolve().is_relative_to((fixture.home / '.cursor/plugins/local').resolve()),
              'repair fixture does not own the selected Cursor projection')
        projection = hashes(target)
        agreement = {name: (fixture.data / name).read_bytes() for name in ('uap.toml', 'uap.lock')}
        state = json.loads((fixture.data / 'state-v2.json').read_text(encoding='utf-8'))
        owner = next(receipt for receipt in state['environment_receipts']
                     if receipt['client_binding_id'] == before_binding['client_binding_id'])
        shutil.rmtree(target)
        argv = [str(self.binary), 'install', '--scope', 'user', '--yes', '--format', 'json']
        result = subprocess.run(argv, cwd=fixture.project, env=fixture.env,
                                stdin=subprocess.DEVNULL, capture_output=True,
                                timeout=self.acquisition_timeout)
        persist(evidence / 'declared-install.command.json', dict(argv=argv, exit=result.returncode))
        (evidence / 'declared-install.stdout').write_bytes(result.stdout)
        (evidence / 'declared-install.stderr').write_bytes(result.stderr)
        check(result.returncode == 0, 'declared install did not restore the missing projection')
        # json.loads rejects a repair document followed by a convergence document.
        output = envelope(result.stdout, result.returncode, command='agentplugins install')
        check(output.get('outcome') == 'completed' and output.get('scope') == 'user',
              'declared install lost its final successful convergence facts')
        actions = output.get('actions', [])
        check([(row.get('action'), row.get('client_id'), row.get('alias')) for row in actions] ==
              [('repair', 'cursor', owner['alias']), ('unchanged', 'cursor', owner['alias'])],
              'single result did not retain both repair and final unchanged convergence')
        native = output.get('results', [])
        check(len(native) == 1 and native[0].get('operation') == 'repair' and
              native[0].get('outcome') == 'completed' and native[0].get('client_id') == 'cursor',
              'single result lost the completed repair or invented another native effect')
        after_binding = installed(fixture, ('cursor',))['cursor']
        check(after_binding.get('package_revision') == before_binding.get('package_revision'),
              'repair changed the recorded immutable package pin')
        check(projection and hashes(target) == projection,
              'declared repair did not restore exact owned projection bytes')
        check(all((fixture.data / name).read_bytes() == body for name, body in agreement.items()),
              'declared repair rewrote saved manifest or lock bytes')
        final_state = json.loads((fixture.data / 'state-v2.json').read_text(encoding='utf-8'))
        after_owner = next(receipt for receipt in final_state['environment_receipts']
                           if receipt['client_binding_id'] == after_binding['client_binding_id'])
        authority_keys = ('environment_id', 'alias', 'client_id', 'source_key', 'package_digest')
        check(all(after_owner.get(key) == owner.get(key) for key in authority_keys),
              'repair lost the original saved environment/source authority')
        check(any(receipt.get('phase') == 'committed' and
                  receipt.get('environment_authority', {}).get('expected_action') == 'repair' and
                  receipt['environment_authority'].get('source_key') == owner['source_key']
                  for receipt in after_binding.get('receipts', [])),
              'restored projection has no committed repair authority')
        persist(evidence / 'declared-repair-facts.json', dict(
            fixture='normalized Skill represented as a saved standard plugin',
            source_authority=after_owner, restored_projection=projection,
            manifest_lock_unchanged=True))

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
        installed(fixture, ('cursor',), retained_failed=('codex',), result=result)
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
            healthy = installed(fixture, ('cursor',), retained_failed=('codex',))['cursor']
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
        final = installed(fixture, ('cursor', 'codex') if retry else ('cursor',),
                          retained_failed=() if retry else ('codex',))
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
        installed(fixture, ('cursor',), retained_failed=('codex',))

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
        held, child_owned = [], False
        primary = None
        try:
            marker = fixture.root / 'native-add-paused'
            deadline = time.monotonic() + self.acquisition_timeout
            while not marker.exists() and process.poll() is None:
                check(time.monotonic() < deadline, 'native effect marker was not reached')
                time.sleep(0.02)
            check(process.poll() is None, 'CLI exited before native effect pause')
            identity = json.loads(marker.read_text(encoding='utf-8'))
            # Preserve the rejected marker too; cleanup evidence must not hide
            # which PID/PPID topology failed ownership admission.
            persist(evidence / 'native-marker.json', identity)
            held = hold_native_lineage(process, identity, nonce, fixture.bin / 'codex', self.binary)
            child_owned = True
            persist(evidence / 'owned-native-identity.json', dict(
                marker=identity, lineage=[dict(role=row['role'], **row['identity']) for row in held]))
            process.kill()  # Popen owns and reaps this exact CLI process.
            try:
                signal.pidfd_send_signal(held[0]['fd'], signal.SIGKILL)
            except ProcessLookupError:
                pass  # Parent-death cleanup can have already ended this child.
            for row in held:
                poller = select.poll()
                poller.register(row['fd'], select.POLLIN)
                check(poller.poll(3000), 'owned ' + row['role'] + ' process did not exit')
            stdout, stderr = process.communicate(timeout=3)
            (evidence / 'crash.stdout').write_bytes(stdout)
            (evidence / 'crash.stderr').write_bytes(stderr)
            check(process.returncode == -signal.SIGKILL, 'CLI crash control did not execute')
        except BaseException as error:
            primary = error
            raise
        finally:
            cleanup_errors = []
            for row in held:
                try:
                    # Both handles passed exact lineage/inode/start-time checks.
                    # Never signal a marker PID or an inferred process group.
                    try:
                        signal.pidfd_send_signal(row['fd'], signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    poller = select.poll()
                    poller.register(row['fd'], select.POLLIN)
                    check(poller.poll(3000), 'owned ' + row['role'] + ' cleanup could not be confirmed')
                except Exception as error:
                    cleanup_errors.append(repr(error))
                finally:
                    os.close(row['fd'])
            try:
                if process.poll() is None:
                    process.kill()
                process.communicate(timeout=3)
            except Exception as error:
                cleanup_errors.append(repr(error))
            persist(evidence / 'crash-cleanup.json', dict(
                cli_reaped=process.poll() is not None, native_identity_confirmed=child_owned,
                lineage_roles=[row['role'] for row in held],
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
            self.assert_unapplied(session, fixture)
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
            self.assert_unapplied(session, fixture)
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
            self.assert_unapplied(session, fixture)
            check('Available Skills' in clean(session.raw), 'numeric catalog not visible before input')
            check(not (fixture.root / 'browser-opened').exists(), '--plain opened automatic browser')
            session.send(b'q\n')
            session.finish(1)
            self.assert_unapplied(session, fixture)
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
            self.assert_unapplied(session, fixture)
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
            self.assert_unapplied(session, fixture)
            session.send(b'\x03')
            session.finish(1)
            self.assert_unapplied(session, fixture)
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


def harness_checkout():
    checkout = Path(__file__).resolve().parents[2]
    try:
        head = subprocess.check_output(
            ['git', '-C', str(checkout), 'rev-parse', 'HEAD'], text=True).strip()
        dirty = bool(subprocess.check_output(
            ['git', '-C', str(checkout), 'status', '--porcelain'], text=True))
    except (OSError, subprocess.CalledProcessError):
        head, dirty = None, None
    return checkout, head, dirty


def copy_product_binary(source, artifacts):
    """Launch only independent bytes held in a new private evidence directory."""
    directory = artifacts / 'product'
    directory.mkdir(mode=0o700)
    binary = directory / ('agentplugins.exe' if os.name == 'nt' else 'agentplugins')
    with source.open('rb') as original, binary.open('xb') as private:
        check(stat.S_ISREG(os.fstat(original.fileno()).st_mode), 'CLI input is not a regular file')
        shutil.copyfileobj(original, private)
    binary.chmod(0o700)
    check(binary.resolve() != source.resolve() and not os.path.samefile(source, binary),
          'qualification CLI must be independent of the supplied executable')
    return binary


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
    supplied_binary = args.binary.resolve(strict=True)
    binary = copy_product_binary(supplied_binary, args.artifacts)
    supplied_digest = hashlib.sha256(supplied_binary.read_bytes()).hexdigest()
    check(hashlib.sha256(binary.read_bytes()).hexdigest() == supplied_digest,
          'supplied CLI changed while its qualification copy was prepared')
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
    checkout, head, dirty = harness_checkout()
    report = dict(schema_version=1, product_sha=args.product_sha,
                  harness_sha=head, harness_dirty=dirty, harness_checkout=str(checkout),
                  supplied_binary_path=str(supplied_binary), binary_path=str(binary),
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
            check(hashlib.sha256(supplied_binary.read_bytes()).hexdigest() == supplied_digest,
                  'input CLI binary changed during qualification')
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
