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
import sys
import tempfile
import time

from harness import Fixture, Session, Screen, SELECT, CONFIRM, LIFECYCLE, check, clean, hashes

TARGETS = ('codex', 'cursor')
SETS = {'codex': ('codex',), 'cursor': ('cursor',), 'both': TARGETS}
CASES = tuple(f'{s}-{answer}' for s in SETS for answer in ('default-no', 'yes')) + (
    'neither', 'selection-escape', 'selection-ctrl-c',
    'confirmation-escape', 'confirmation-ctrl-c', 'queued-enters', 'queued-yes', 'queued-space',
    'native-ownership', 'selection-eof', 'confirmation-eof', 'confirmation-partial-eof')


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def source_digest(repo):
    """Hash tracked working-tree bytes, including local changes and deletions."""
    paths = subprocess.check_output(['git', 'ls-files', '-z'], cwd=repo).split(b'\0')
    state = hashlib.sha256()
    for raw in sorted(set(paths) - {b''}):
        path = repo / os.fsdecode(raw)
        state.update(raw + b'\0')
        state.update((digest(path) if path.is_file() else 'missing').encode() + b'\0')
    return state.hexdigest()


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
    if 'codex' not in selected: fixture.installed(selected)
    body = json.loads((fixture.data / 'state-v2.json').read_text())
    bindings = body['installations'][0]['clients']
    check(sorted(b['client_id'] for b in bindings.values()) == sorted(selected),
          'binding identities differ from selection')
    locators = []
    for binding_id, binding in bindings.items():
        client = binding['client_id']
        check(binding['materialization'] == 'materialized', 'binding not materialized')
        check(any(r.get('phase') == 'committed' for r in binding['receipts']), 'missing commit')
        check(binding.get('authentication') not in ('authenticated', 'not_required'), 'false auth')
        check(binding.get('activation') == 'active' if client == 'codex' else binding.get('activation') != 'active', 'wrong activation')
        check(binding['client_binding_id'] == binding_id, 'binding key mismatch')
        for receipt in binding['receipts']:
            check(receipt['client_binding_id'] == binding_id, 'receipt identity mismatch')
            check(receipt['active_path'] == binding['target_locator'], 'receipt path mismatch')
        target = Path(binding['target_locator']).resolve()
        expected_root = fixture.data / 'managed/clients/codex' if client == 'codex' else fixture.home / '.cursor'
        check(target.is_relative_to(expected_root.resolve()),
              f'identity/path mismatch: {client}: {target}')
        if client == 'codex':
            native = target / '.codex-plugin/plugin.json'
            check(native.is_file(), 'Codex native identity missing after Yes')
            check(json.loads(native.read_text())['name'] == 'pty-synthetic', 'wrong Codex identity')
            registry = json.loads((fixture.data / 'synthetic-codex-registry.json').read_text())
            check(len(registry['installed']) == 1 and registry['installed'][0]['enabled'], 'native registry mismatch')
        locators.append(str(target))
    for client in set(TARGETS) - set(selected):
        check(hashes(fixture.home / ('.' + client)) == {}, f'unselected {client} mutated')
    return {'bindings': bindings, 'package_paths': locators}


def prepare_codex_registry(fixture):
    """Synthetic controlled native registry protocol, not a real Codex executable."""
    stub = fixture.bin / 'codex'
    stub.write_text('#!' + sys.executable + '\n' + r'''import json, os, sys
from pathlib import Path
args = sys.argv[1:]
root = Path(os.environ['AGENTPLUGINS_HOME'])
registry = root / 'synthetic-codex-registry.json'
with open(os.environ['STUB_LOG'], 'a') as log:
    log.write(sys.argv[0] + ' ' + ' '.join(args) + '\n')
state = json.loads(registry.read_text()) if registry.exists() else {}
if args == ['--version']:
    print('synthetic 99.0.0')
elif args == ['plugin', 'list', '--json']:
    print(json.dumps({'installed': state.get('installed', [])}))
elif len(args) == 5 and args[:3] == ['plugin', 'marketplace', 'add'] and args[4] == '--json':
    path = Path(args[3]).resolve()
    if not path.is_relative_to((root / 'managed/clients/codex').resolve()): sys.exit(97)
    manifest = path / '.agents/plugins/marketplace.json'
    body = json.loads(manifest.read_text())
    state['marketplace'] = body['name']
    registry.write_text(json.dumps(state))
    print('{}')
elif len(args) == 4 and args[:2] == ['plugin', 'add'] and args[3] == '--json':
    market = state.get('marketplace')
    if not market or args[2] != 'pty-synthetic@' + market: sys.exit(97)
    state['installed'] = [dict(pluginId=args[2], name='pty-synthetic', marketplaceName=market,
                               installed=True, enabled=True)]
    registry.write_text(json.dumps(state))
    print('{}')
else:
    sys.exit(97)
''')


def run_case(name, binary, evidence, timeout):
    evidence.mkdir()
    with tempfile.TemporaryDirectory(prefix='selection-matrix-') as tmp:
        fixture = Fixture(tmp)
        native = fixture.package / '.codex-plugin/plugin.json'
        native.parent.mkdir()
        native.write_text(json.dumps({'name': 'pty-synthetic', 'version': '1.0.0'}))
        if name != 'native-ownership': prepare_codex_registry(fixture)
        eof = name.endswith('eof')
        argv = [str(binary), 'add', str(fixture.package)] + (['--plain'] if eof else [])
        session = Keyboard(argv, fixture, evidence, timeout)
        outcome = {'case': name, 'status': 'failed'}
        try:
            session.wait(SELECT, 'selection')
            if eof:
                fixture.unchanged()
                if name != 'selection-eof':
                    session.send(b'\n')
                    session.wait(CONFIRM, 'confirmation')
                    fixture.unchanged()
                session.send(b'y\x04\x04' if name == 'confirmation-partial-eof' else b'\x04')
                session.finish(1)
                fixture.unchanged()
                outcome['status'] = 'passed'
                return outcome
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
                        for index, client in enumerate(selected if len(selected) == 1 else ()):
                            marker = r'Have you completed required authentication[^\r\n]*\[y/N\]' if client == 'codex' else LIFECYCLE
                            offset = session.wait(marker, f'lifecycle-{index}', after=offset)
                            import termios
                            check(termios.tcgetattr(session.slave) == session.before, 'raw mode at activation')
                            session.send(b'n\n')
                        session.finish(0)
                        check(clean(session.raw).count('Have you completed required authentication') == int(selected == ('codex',)),
                              'unexpected authentication prompt')
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
                if (fixture.root / 'stub.log').exists():
                    log = (fixture.root / 'stub.log').read_text().splitlines()
                    check(log and all(line.endswith(' --version') or line.endswith('codex plugin list --json')
                                      or (name in ('codex-yes', 'both-yes') and ('/codex plugin marketplace add ' in line or '/codex plugin add pty-synthetic@' in line) and line.endswith(' --json'))
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
    parser.add_argument('--binary', type=Path)
    parser.add_argument('--build-go', type=Path, help='build exact current source with this Go executable')
    parser.add_argument('--binary-sha256')
    parser.add_argument('--artifacts', required=True, type=Path)
    parser.add_argument('--timeout', type=float, default=8)
    parser.add_argument('--case', action='append', choices=CASES)
    args = parser.parse_args()
    if os.name != 'posix': parser.error('Windows selection matrix NOT COVERED; Unix PTY required')
    if not 0 < args.timeout <= 30: parser.error('timeout must be in (0, 30]')
    repo = Path(__file__).resolve().parents[2]
    source_before = source_digest(repo)
    if args.build_go:
        check(args.binary is None and args.binary_sha256 is None, '--build-go excludes supplied binary')
        args.artifacts.mkdir(parents=True, exist_ok=False)
        args.binary = args.artifacts.resolve() / 'source-agentplugins'
        with (args.artifacts / 'build.log').open('wb') as log:
            subprocess.run([str(args.build_go.resolve()), 'build', '-o', str(args.binary), './cmd/agentplugins'],
                           cwd=repo / 'cli/plugin-kit-ai', stdout=log, stderr=subprocess.STDOUT, check=True, timeout=180)
        check(source_digest(repo) == source_before, 'tracked source changed during build')
        args.binary_sha256 = digest(args.binary)
    else:
        check(args.binary is not None and args.binary_sha256, 'supply --build-go or binary and hash')
        args.artifacts.mkdir(parents=True, exist_ok=False)
    check(digest(args.binary) == args.binary_sha256, 'binary hash mismatch; build needed if no verified binary')
    binary = args.artifacts.resolve() / 'frozen-agentplugins'
    shutil.copyfile(args.binary, binary); binary.chmod(0o700)
    check(digest(binary) == args.binary_sha256, 'frozen copy hash mismatch')
    repo = Path(__file__).resolve().parents[2]
    provenance = {'tracked_worktree_sha256': source_before, 'source_commit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=repo, text=True).strip(),
                  'source_files_sha256': {p.name: digest(p) for p in (
                      Path(__file__), Path(__file__).with_name('harness.py'),
                      Path(__file__).with_name('pty_owner.py'))},
                  'binary_sha256': digest(binary), 'platform': platform.platform(),
                  'binary_source_equivalence': 'built from current source in this run' if args.build_go else 'not established; supplied binary',
                  'client_registry': 'synthetic Codex registry and install protocol; version-only Cursor; no real client qualification',
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
    check(source_digest(repo) == source_before, 'tracked source changed during matrix')
    return int(any(r['status'] != 'passed' for r in results))


if __name__ == '__main__':
    raise SystemExit(main())
