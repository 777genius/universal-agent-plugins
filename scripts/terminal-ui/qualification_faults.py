#!/usr/bin/env python3
"""Supplemental native fault tests; builds test-only helpers, never an installer.

Unix: real PTY Huh/Plain fault, cancellation and same-descriptor reuse checks.
Windows: real inherited ConPTY ReadConsole cancellation timing sweep and reuse.
Does not claim deterministic coverage of every Windows syscall-entry schedule.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile
import time
from types import SimpleNamespace

from harness import check

UNIX_CASES = ['huh-cancel', 'plain-cancel', 'huh-write-error',
              'huh-write-short', 'huh-render-panic', 'huh-init-error']


def unix_case(binary, case, root, evidence, timeout):
    import termios
    from harness import Session
    trigger = root / 'trigger'
    env = dict(os.environ, TERM='xterm-256color', HOME=str(root),
               UAP_QUALIFICATION_CASE=case, UAP_QUALIFICATION_TRIGGER=str(trigger))
    fixture = SimpleNamespace(project=root, env=env)
    session = Session([str(binary), '-test.run=^TestQualificationTerminalFault$',
                       '-test.v', '-test.timeout=15s'], fixture, evidence, timeout)
    active = False
    try:
        if case != 'huh-init-error':
            if case.startswith('huh-'):
                # The question is preprinted before Tea initializes. Require
                # the actual choice controls AND raw mode before injecting.
                if case == 'huh-cancel':
                    session.wait(r'Yes.*No', 'active-controls')
                else:
                    # A wrapped writer loses Fd at the adapter boundary. The
                    # renderer has no dimensions; prove actual renderer startup
                    # and raw input, without claiming visible form controls.
                    deadline = time.monotonic() + timeout
                    while b'\x1b[?25l' not in session.raw and time.monotonic() < deadline:
                        session.pump(0.02)
                    check(b'\x1b[?25l' in session.raw, 'renderer did not initialize')
                    session.pump(0.08)
                    session.frame('active-renderer')
                mode = termios.tcgetattr(session.slave)
                active = not (mode[3] & termios.ICANON) and not (mode[3] & termios.ECHO)
                check(active, 'fault must be injected into active raw terminal')
            else:
                session.wait(r'Apply qualification', 'active-plain')
                # No input is sent: hold the real terminal read blocked.
                session.pump(0.08)
                active = True
            trigger.write_text('inject\n')
            session.wait(r'QUALIFICATION_TRIGGERED', 'triggered')
            if case.startswith('huh-write-') or case == 'huh-render-panic':
                session.send(b' ')
        session.wait(r'QUALIFICATION_REUSE_READY', 'same-process-reuse')
        check(termios.tcgetattr(session.slave) == session.before,
              'terminal modes not restored before inherited input reuse')
        session.send(b'qualification-reuse\n')
        session.wait(r'QUALIFICATION_REUSE_OK', 'same-process-reused')
        session.finish()
        return {'active_terminal_observed': active, 'restoration': session.restoration}
    finally:
        try:
            # Failure evidence is captured before the harness cleanup reset.
            modes_equal = termios.tcgetattr(session.slave) == session.before
            transcript = bytes(session.raw)
            (evidence / 'fault-state.json').write_text(json.dumps({
                'active_terminal_observed': active, 'termios_equal_before_cleanup': modes_equal,
                'cursor_restored_before_cleanup': transcript.rfind(b'\x1b[?25h') > transcript.rfind(b'\x1b[?25l')
                    if b'\x1b[?25l' in transcript else True,
                'child_exit_before_cleanup': session.process.poll(),
            }, indent=2))
        finally:
            session.close()


def windows_case(binary, case, root, evidence, timeout):
    import sys
    import uuid
    from harness import clean
    from windows_conpty import ConPTY, finish_console
    nonce = uuid.uuid4().hex
    env = {key: str(root) for key in ('HOME', 'USERPROFILE', 'APPDATA', 'LOCALAPPDATA', 'TMP', 'TEMP')}
    env.update(SystemRoot=os.environ['SystemRoot'], WINDIR=os.environ['SystemRoot'],
               PATH=str(root), UAP_QUALIFICATION_CASE=case)
    status_path = evidence / 'status.json'
    config_path = evidence / 'job.json'
    config_path.write_text(json.dumps(dict(
        argv=[str(binary), '-test.run=^TestQualificationConsoleCancellation$',
              '-test.v', '-test.timeout=45s'], cwd=str(root), nonce=nonce,
        status=str(status_path))), encoding='utf-8')
    session = ConPTY([sys.executable, '-I', str(Path(__file__).with_name('windows_job.py')),
                      str(config_path)], env, str(root), timeout, evidence=evidence)
    session.status_path = status_path
    status = {}
    try:
        session.wait('OWNER_READY_' + nonce)
        offset = 0
        for i in range(30):
            offset = session.wait(r'QUALIFICATION_CONSOLE_REUSE_READY ' + str(i) + r'\b', after=offset)
            # One Enter only: CRLF input injection can enqueue a second empty
            # line and invalidate the next cancellation-entry sample.
            session.send(('qualification-reuse-' + str(i) + '\r').encode())
        session.wait(r'QUALIFICATION_CONSOLE_OK', after=offset)
        session.wait('RESTORE_READY_' + nonce)
        status = json.loads(status_path.read_text(encoding='utf-8'))
        check(status['exit'] == 0, 'native console helper failed: ' + repr(status))
        check(status['owner_before'] == status['owner_after'], 'console state changed after cancellation sweep')
        offset = len(session.raw)
        session.send(('line_' + nonce + '\r').encode())
        session.wait('RESTORE_OK_' + nonce, after=offset)
        deadline = time.monotonic() + timeout
        while session.poll() is None and time.monotonic() < deadline:
            time.sleep(0.02)
        check(session.poll() == 0, 'native console owner failed or hung')
        status = json.loads(status_path.read_text(encoding='utf-8'))
        check(status['line_read'] and status['owner_probe'] == status['owner_before'], 'owner console reuse failed')
        check('line_' + nonce in clean(session.raw[offset:]), 'owner kernel echo missing')
    finally:
        finish_console(session, status_path, evidence)
    check(not session.forced, 'forced cleanup cannot qualify as a pass')
    return {'native_console': True, 'cancellation_and_reuse_iterations': 30,
            'owner_restoration_and_reuse': True}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default='go')
    parser.add_argument('--artifacts', type=Path, required=True)
    parser.add_argument('--case', action='append')
    parser.add_argument('--timeout', type=float, default=10)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[2]
    args.artifacts.mkdir(parents=True, exist_ok=True)
    artifacts = args.artifacts.resolve()
    native_windows = os.name == 'nt'
    allowed = ['windows-console-cancel'] if native_windows else UNIX_CASES
    cases = args.case or allowed
    check(all(c in allowed for c in cases), 'case is not supported on this native host')
    binary = artifacts / ('qualification.test.exe' if native_windows else 'qualification.test')
    package = './internal/promptio' if native_windows else './internal/terminalprompts'
    command = [args.go, 'test', '-c', '-o', str(binary), package]
    build = subprocess.run(command, cwd=repo / 'cli/plugin-kit-ai', stdout=subprocess.PIPE,
                           stderr=subprocess.STDOUT, text=True, timeout=180)
    (artifacts / 'build.log').write_text(build.stdout)
    check(build.returncode == 0, 'test helper build failed; see build.log')
    results = {'platform': platform.platform(), 'test_binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
               'build_command': command, 'test_only': True, 'cases': []}
    for case in cases:
        evidence = artifacts / case
        evidence.mkdir(exist_ok=True)
        row = {'case': case, 'passed': False}
        started = time.monotonic()
        try:
            with tempfile.TemporaryDirectory(prefix='uap-qualification-fault-') as temp:
                runner = windows_case if native_windows else unix_case
                row.update(runner(binary, case, Path(temp), evidence, args.timeout))
            row['passed'] = True
        except Exception as exc:
            row['error'] = repr(exc)
        row['elapsed_seconds'] = round(time.monotonic() - started, 3)
        results['cases'].append(row)
        (artifacts / 'results.json').write_text(json.dumps(results, indent=2))
        print(json.dumps(row), flush=True)
    return 0 if all(row['passed'] for row in results['cases']) else 1


if __name__ == '__main__':
    raise SystemExit(main())
