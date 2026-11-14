#!/usr/bin/env python3
"""Disposable Unix PTY regressions for consent, JSON remove, and group progress.

Uses the production CLI, config-only OpenCode discovery, and the shared synthetic
scanner fixture. This does not qualify native clients, authentication, or release.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import tempfile

from harness import Fixture, Session, SELECT, CONFIRM, LIFECYCLE, check, clean, selection_choices

CASES = ('plain-queued-enter', 'plain-queued-yes', 'plain-default-yes',
         'plain-queued-lifecycle', 'plain-no', 'plain-cancel',
         'json-remove-tty', 'json-remove-pipe', 'json-remove-dry-run',
         'group-narrow', 'group-plain')


def run_case(name, binary, evidence, timeout):
    evidence.mkdir()
    with tempfile.TemporaryDirectory(prefix='ui-regression-') as tmp:
        grouped = name.startswith(('json-', 'group-'))
        fixture = Fixture(tmp, include_opencode=grouped)
        if name.startswith('group-'):
            skill = fixture.package / 'skills' / 'ui-regression' / 'SKILL.md'
            skill.parent.mkdir(parents=True)
            skill.write_text('---\nname: ui-regression\ndescription: Synthetic text-only skill\n---\nRead fixture text.\n')
        argv = [str(binary), 'add', str(fixture.package)]
        if name.startswith('json-'):
            seed = subprocess.run(argv + ['--target=cursor,opencode', '--format=json'],
                                  cwd=fixture.project, env=fixture.env, stdin=subprocess.DEVNULL,
                                  capture_output=True, timeout=timeout)
            (evidence / 'seed.stdout').write_bytes(seed.stdout)
            (evidence / 'seed.stderr').write_bytes(seed.stderr)
            check(seed.returncode == 0, 'remove fixture installation failed')
            fixture.installed(['cursor', 'opencode'])
            fixture.before = fixture.mutations()
            argv = [str(binary), 'remove', 'pty-synthetic', '--format=json']
            if name == 'json-remove-dry-run': argv += ['--dry-run']
        elif grouped:
            argv += ['--target=cursor,opencode', '--color=never']
            if name == 'group-plain': argv += ['--plain']
        else:
            argv += ['--plain', '--color=never']
        session = Session(argv, fixture, evidence, timeout,
                          stdin_pipe=name == 'json-remove-pipe',
                          redirect='stdout' if name.startswith('json-') else None,
                          cols=40 if name == 'group-narrow' else 100)
        try:
            if name.startswith('json-'):
                session.finish(1)
                check((evidence / 'stdout.raw').read_bytes() == b'', 'JSON error polluted stdout')
                check('requires --target' in clean(session.raw), 'missing target diagnostic absent')
                check('Choose targets' not in clean(session.raw), 'JSON remove prompted')
                fixture.unchanged()
            elif grouped:
                session.finish()
                fixture.installed(['cursor', 'opencode'], active=('opencode',))
                body = clean(session.raw).lower()
                check('setup required' in body, 'manual/auth-unchecked result missing')
                check('3/3  installed' not in body and '✓ installed' not in body,
                      'unverified authentication was labeled installed')
                check(not re.search(rb'\x1b\[[0-9;]*[AK]', session.raw), 'fallback moved cursor')
                check('cursor' in body and 'opencode' in body, 'group omitted a selected client')
            else:
                session.wait(SELECT, 'selection')
                session.wait(r'(?i)cursor', 'choices-ready')
                offered = selection_choices(session.raw)
                selection = str(offered.index('cursor') + 1).encode() + b'\n'
                suffix = {'plain-queued-enter': b'\n', 'plain-queued-yes': b'y\n'}.get(name, b'')
                session.send(selection + suffix)
                if suffix:
                    session.finish()
                    fixture.unchanged()
                    check('Installation not applied' in clean(session.raw), 'queued consent did not decline')
                else:
                    session.wait(r'Apply this plan\? \[Y/n\]', 'confirmation')
                    fixture.unchanged()
                    if name in ('plain-default-yes', 'plain-queued-lifecycle'):
                        session.send(b'\nn\n' if name == 'plain-queued-lifecycle' else b'\n')
                        session.wait(LIFECYCLE, 'activation')
                        fixture.installed(['cursor'])
                        if name != 'plain-queued-lifecycle': session.send(b'n\n')
                        session.finish()
                        check('Activation remains unconfirmed' in clean(session.raw), 'activation No lost')
                        fixture.installed(['cursor'])
                    else:
                        session.send(b'cancel\n' if name == 'plain-cancel' else b'n\n')
                        session.finish(1 if name == 'plain-cancel' else 0)
                        fixture.unchanged()
        finally:
            (evidence / 'mutations.json').write_text(json.dumps({'before': fixture.before, 'after': fixture.mutations()}, indent=2))
            session.close()
            fixture.validate_stubs()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--artifacts', required=True, type=Path)
    parser.add_argument('--case', choices=CASES, action='append')
    parser.add_argument('--timeout', type=float, default=12)
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    args.artifacts.mkdir(parents=True, exist_ok=False)
    results = []
    for name in args.case or CASES:
        try:
            run_case(name, binary, args.artifacts / name, args.timeout)
            result = {'case': name, 'status': 'passed'}
        except Exception as exc:
            result = {'case': name, 'status': 'failed', 'error': str(exc)}
        results.append(result)
        print(json.dumps(result), flush=True)
        (args.artifacts / 'results.json').write_text(json.dumps({
            'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
            'scanner': 'synthetic cached protocol stub; no security proof',
            'results': results}, indent=2))
    return int(any(r['status'] != 'passed' for r in results))


if __name__ == '__main__':
    raise SystemExit(main())
