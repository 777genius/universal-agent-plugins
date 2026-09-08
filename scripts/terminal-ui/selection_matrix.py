#!/usr/bin/env python3
"""Bounded real-keyboard selection matrix; native Unix only, stdlib only."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import tempfile
import time

from harness import Fixture, Session, Screen, SELECT, CONFIRM, LIFECYCLE, check, clean, hashes

TARGETS = ('codex', 'cursor')
SETS = {'codex': ('codex',), 'cursor': ('cursor',), 'both': TARGETS}
CASES = tuple(f'{s}-{answer}' for s in SETS for answer in ('default-no', 'yes')) + (
    'neither', 'selection-escape', 'selection-ctrl-c',
    'confirmation-escape', 'confirmation-ctrl-c', 'queued-enters', 'queued-yes', 'queued-space',
    'native-ownership')


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def choices(text):
    return re.findall(r'\[([• ])\][^\n]*?\(([^()]+)\)', text)


def plan_ids(text):
    return re.findall(r'^Target: ([a-z0-9-]+)\s*$', text, re.M)


class Keyboard(Session):
    def frame(self, label):
        super().frame(label)
        state = self.fixture.mutations()
        self.events[-1]['protected_state_sha256'] = hashlib.sha256(
            json.dumps(state, sort_keys=True).encode()).hexdigest()
        self.events[-1]['unchanged'] = state == self.fixture.before

    def pump(self, timeout=0.1):
        super().pump(timeout)
        # This lane exercises reverse-index away from the top margin. Normalize
        # only the evidence renderer; PTY input/output bytes stay untouched.
        screen = Screen(self.screen.rows, self.screen.cols)
        screen.feed(bytes(self.raw).replace(b'\x1bM', b'\x1b[1A'))
        self.screen = screen

    def send(self, keys):
        self.events.append({'event': 'raw-key', 'hex': keys.hex(),
                            'offset': len(self.raw),
                            'elapsed': round(time.monotonic() - self.start, 3)})
        super().send(keys)

    def key(self, keys, label):
        offset = len(self.raw)
        self.send(keys)
        deadline = time.monotonic() + self.timeout
        while len(self.raw) == offset and time.monotonic() < deadline:
            self.pump()
        check(len(self.raw) > offset, f'no keyboard repaint: {label}')
        self.pump(0.1)
        self.frame(label)


def selected_frame(session, expected):
    rows = choices(session.screen.snapshot())
    check([r[1] for r in rows] == list(TARGETS), f'choice order/identities: {rows}')
    check([r[1] for r in rows if r[0] == '•'] == list(expected),
          f'displayed selection differs from identities {expected}: {rows}')


def installed(fixture, selected):
    fixture.installed(selected)
    body = json.loads((fixture.data / 'state-v2.json').read_text())
    bindings = body['installations'][0]['clients']
    check(sorted(b['client_id'] for b in bindings.values()) == sorted(selected),
          'binding identities differ from selection')
    locators = []
    for binding_id, binding in bindings.items():
        client = binding['client_id']
        check(binding['client_binding_id'] == binding_id, 'binding key mismatch')
        for receipt in binding['receipts']:
            check(receipt['client_binding_id'] == binding_id, 'receipt identity mismatch')
            check(receipt['active_path'] == binding['target_locator'], 'receipt path mismatch')
        target = Path(binding['target_locator']).resolve()
        check(target.is_relative_to((fixture.home / ('.' + client)).resolve()),
              f'identity/path mismatch: {client}: {target}')
        if client == 'codex':
            native = target / '.codex-plugin/plugin.json'
            check(native.is_file(), 'Codex native identity missing after Yes')
            check(json.loads(native.read_text())['name'] == 'pty-synthetic', 'wrong Codex identity')
        locators.append(str(target))
    for client in set(TARGETS) - set(selected):
        check(hashes(fixture.home / ('.' + client)) == {}, f'unselected {client} mutated')
    return {'bindings': bindings, 'package_paths': locators}


def run_case(name, binary, evidence, timeout):
    evidence.mkdir()
    with tempfile.TemporaryDirectory(prefix='selection-matrix-') as tmp:
        fixture = Fixture(tmp)
        native = fixture.package / '.codex-plugin/plugin.json'
        native.parent.mkdir()
        native.write_text(json.dumps({'name': 'pty-synthetic', 'version': '1.0.0'}))
        session = Keyboard([str(binary), 'add', str(fixture.package)], fixture, evidence, timeout)
        outcome = {'case': name, 'status': 'failed'}
        try:
            session.wait(SELECT, 'selection')
            session.wait(r'enter submit', 'selection-ready')
            selected_frame(session, TARGETS)
            fixture.unchanged()
            if name.startswith('selection-'):
                session.send(b'\x1b' if name.endswith('escape') else b'\x03')
                session.finish(1)
                fixture.unchanged()
            else:
                selected = SETS.get(name.split('-')[0], TARGETS)
                if name == 'neither': selected = ()
                if name == 'native-ownership': selected = ('codex',)
                # Every path exercises down/up and a reversible Space toggle.
                session.key(b' ', 'toggle-off')
                selected_frame(session, ('cursor',))
                session.key(b' ', 'toggle-back')
                selected_frame(session, TARGETS)
                session.key(b'\x1b[B', 'down')
                session.key(b'\x1b[A', 'up')
                if 'codex' not in selected: session.key(b' ', 'codex-off')
                if 'cursor' not in selected:
                    session.key(b'\x1b[B', 'cursor-focus')
                    session.key(b' ', 'cursor-off')
                selected_frame(session, selected)
                outcome['selected_ids'] = list(selected)
                fixture.unchanged()
                offset = len(session.raw)
                session.send(b'\r\r' if name == 'queued-enters' else
                             b'\ry\r' if name == 'queued-yes' else
                             b'\r \r' if name == 'queued-space' else b'\r')
                if name == 'neither':
                    session.wait(r'(?i)(at least one|select one|cannot be empty|must select)',
                                 'empty-validation')
                    selected_frame(session, ())
                    fixture.unchanged()
                    session.send(b'\x1b'); session.finish(1)
                    check(not re.search(CONFIRM, clean(session.raw[offset:])), 'empty reached confirmation')
                    fixture.unchanged()
                else:
                    session.wait(CONFIRM, 'confirmation', after=offset)
                    if name not in ('queued-enters', 'queued-yes', 'queued-space'):
                        session.wait(r'(?s)Yes.*?No.*?enter submit', 'confirmation-ready', after=offset)
                    fixture.unchanged()
                    plan = plan_ids(clean(session.raw[offset:]))
                    check(plan == list(selected), f'plan targets {plan}, expected {list(selected)}')
                    check('Plugin: pty-synthetic 1.0.0' in clean(session.raw[offset:]), 'plan identity missing')
                    session.frame('plan-verified')
                    if name.startswith('confirmation-'):
                        session.send(b'\x1b' if name.endswith('escape') else b'\x03')
                        session.finish(1); fixture.unchanged()
                    elif name == 'native-ownership':
                        session.send(b' \r')
                        session.finish(1)
                        check('observe native identity for codex' in clean(session.raw),
                              'native ownership failure diagnostic missing')
                        fixture.unchanged()
                    elif name.endswith('-yes') and name != 'queued-yes':
                        session.send(b' \r')
                        for index in range(len(selected)):
                            offset = session.wait(LIFECYCLE, f'activation-{index}', after=offset)
                            import termios
                            check(termios.tcgetattr(session.slave) == session.before, 'raw mode at activation')
                            session.send(b'n\n')
                        session.finish(0)
                        check('Have you completed required authentication' not in clean(session.raw),
                              'activation No advanced to authentication')
                        outcome['installation'] = installed(fixture, selected)
                    else:
                        if name not in ('queued-enters', 'queued-yes', 'queued-space'): session.send(b'\r')
                        session.finish(0); fixture.unchanged()
            outcome['status'] = 'passed'
        except Exception as exc:
            outcome['error'] = str(exc)
            # Preserve actual status and restoration even when a semantic assertion fails.
            if session.process.poll() is not None and session.restoration is None:
                try: session.restore_probe()
                except Exception as restore: outcome['restoration_error'] = str(restore)
        finally:
            outcome['unchanged'] = fixture.mutations() == fixture.before
            (evidence / 'mutations.json').write_text(json.dumps({
                'before': fixture.before, 'after': fixture.mutations()}, indent=2))
            session.close()
            try:
                if name == 'native-ownership':
                    log = (fixture.root / 'stub.log').read_text().splitlines()
                    check(log and all(line.endswith(' --version') or line.endswith('codex plugin list --json')
                                      for line in log), 'unexpected native subprocess')
                else:
                    fixture.validate_stubs()
            except Exception as exc:
                outcome['stub_error'] = str(exc)
                outcome['status'] = 'failed'
            log = fixture.root / 'stub.log'
            if log.exists(): (evidence / 'stub.log').write_bytes(log.read_bytes())
        return outcome


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--binary-sha256', required=True)
    parser.add_argument('--artifacts', required=True, type=Path)
    parser.add_argument('--timeout', type=float, default=8)
    parser.add_argument('--case', action='append', choices=CASES)
    args = parser.parse_args()
    if os.name != 'posix': parser.error('Windows selection matrix NOT COVERED; Unix PTY required')
    if not 0 < args.timeout <= 30: parser.error('timeout must be in (0, 30]')
    check(digest(args.binary) == args.binary_sha256, 'binary hash mismatch; build needed if no verified binary')
    args.artifacts.mkdir(parents=True, exist_ok=False)
    binary = args.artifacts.resolve() / 'frozen-agentplugins'
    shutil.copyfile(args.binary, binary); binary.chmod(0o700)
    check(digest(binary) == args.binary_sha256, 'frozen copy hash mismatch')
    repo = Path(__file__).resolve().parents[2]
    provenance = {'source_commit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=repo, text=True).strip(),
                  'source_files_sha256': {p.name: digest(p) for p in (
                      Path(__file__), Path(__file__).with_name('harness.py'),
                      Path(__file__).with_name('pty_owner.py'))},
                  'binary_sha256': digest(binary), 'platform': platform.platform(),
                  'binary_source_equivalence': 'not established; historical frozen binary',
                  'scanner': 'synthetic protocol stub; UI evidence only',
                  'windows': 'not covered', 'macos': 'not run' if platform.system() != 'Darwin' else 'native'}
    results = []
    for name in args.case or CASES:
        result = run_case(name, binary, args.artifacts / name, args.timeout)
        result.update(provenance)
        (args.artifacts / name / 'outcome.json').write_text(json.dumps(result, indent=2))
        results.append(result)
        print(json.dumps({k: result[k] for k in ('case', 'status', 'error') if k in result}), flush=True)
        (args.artifacts / 'results.json').write_text(json.dumps(results, indent=2))
    return int(any(r['status'] != 'passed' for r in results))


if __name__ == '__main__':
    raise SystemExit(main())
