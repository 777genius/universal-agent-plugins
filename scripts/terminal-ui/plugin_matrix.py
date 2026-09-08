#!/usr/bin/env python3
"""Bounded real-CLI plugin fixture lane; no build, client launch, or external MCP.

Run with --binary PATH --artifacts NEW_DIRECTORY. Failures stay failures. The
synthetic scanner inherited from harness is UI evidence, not security evidence.
Schema/semantics: specregistry/schemas/1.0.0, loader/loader_test.go, and
providers/native_identity_test.go under install/integrationctl/agentplugins.
"""
import argparse
from contextlib import contextmanager
import hashlib
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
import platform
from pathlib import Path
import re
import shutil
import shlex
import subprocess
import tempfile
import threading
import time

from harness import Fixture, Session, SELECT, CONFIRM, LIFECYCLE, check, clean, hashes

SCHEMA = 'https://agent-plugins.org/schemas/1.0.0/'
KINDS = ('empty', 'skill', 'stdio-missing', 'http-auth', 'mixed', 'malformed',
         'unsupported', 'collision')
CASES = tuple(f'{kind}:{action}' for kind in KINDS for action in
              (('reject',) if kind == 'malformed' else
               ('cancel',) if kind in ('stdio-missing', 'collision') else ('cancel', 'default-no'))
              ) + ('skill:install', 'stdio-missing:reject', 'mixed:partial-plan',
                   'collision:reject', 'http-auth:auth-unknown', 'empty:all-ten')

# Expectations follow the existing planner contract, not a stricter invented
# pre-consent policy. Keep validation, delivery, activation, auth and runtime
# separate. Original failed case names remain in the audit for traceability.
AUDIT_MATRIX = {
    'mixed:reject': {
        'case': 'mixed:partial-plan', 'classification': 'test assumption',
        'reason': 'Healthy skills remain deliverable when missing stdio is skipped.',
        'source': 'install/integrationctl/agentplugins/usecase/component_readiness_test.go:TestMixedAddAndGroupKeepHealthySiblings'},
    'http-auth:reject': {
        'case': 'http-auth:auth-unknown', 'classification': 'test assumption',
        'reason': 'Installation does not authenticate remote MCP; preserve not_checked and make no HTTP request.',
        'source': 'install/integrationctl/agentplugins/planner/planner_test.go:TestPlannerAuthenticationRequiresAffirmativePerClientCatalogEvidence'},
    'collision:reject': {
        'case': 'collision:reject', 'classification': 'human diagnostic bug',
        'reason': 'Ownership rejection must retain remediation and selected-target context.',
        'source': 'cli/plugin-kit-ai/internal/agentpluginscli/add_multi.go:runAddManyLoaded'},
    'empty:all-ten': {
        'case': 'empty:all-ten', 'classification': 'human display ambiguity',
        'reason': 'Outer JSON targets are logical IDs; inner plans may share the Copilot physical owner. Show selected identities and explain shared delivery.',
        'source': 'cli/plugin-kit-ai/internal/agentpluginscli/cli_test.go:TestCopilotAndVSCodeShareOneGroupedPhysicalMutation'},
}


def put(root, relative, body):
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(body, encoding='utf-8')


def package(fixture, kind, endpoint=None):
    check(kind in KINDS, 'unknown fixture kind')
    manifest = {'$schema': SCHEMA + 'plugin.schema.json', 'name': 'pty-synthetic',
                'version': '1.0.0'}
    put(fixture.package, 'plugin.json', json.dumps(manifest))
    if kind in ('skill', 'mixed', 'collision'):
        put(fixture.package, 'skills/guide/SKILL.md',
            '---\nname: guide\ndescription: Explain the isolated fixture\n---\n'
            'Read reference.txt and report its fixture marker. Do not run commands.\n')
        put(fixture.package, 'skills/guide/reference.txt', 'plugin-matrix-fixture-v1\n')
    if kind in ('stdio-missing', 'mixed', 'http-auth'):
        server = {'type': 'stdio', 'command': 'uap-fixture-missing-runtime'}
        if kind == 'http-auth':
            check(endpoint and endpoint.startswith('http://127.0.0.1:'), 'loopback endpoint required')
            server = {'type': 'streamable-http', 'url': endpoint}
        put(fixture.package, 'mcp.json', json.dumps({
            '$schema': SCHEMA + 'mcp.schema.json', 'mcpServers': {'fixture': server}}))
    if kind == 'malformed':
        put(fixture.package, 'plugin.json', '{"name":')
    if kind == 'unsupported':
        # Portable loader deliberately ignores these; do not invent hook support.
        put(fixture.package, 'hooks/hooks.json', '{}\n')
        put(fixture.package, 'commands/run.md', 'Inert unsupported component.\n')
    if kind == 'collision':
        put(fixture.home, '.cursor/plugins/local/foreign/.cursor-plugin/plugin.json',
            '{"name":"pty-synthetic"}')
        put(fixture.home, '.cursor/plugins/local/foreign/keep.txt', 'unowned\n')
    fixture.before = fixture.mutations()


@contextmanager
def local_endpoint(enabled):
    requests = []
    if not enabled:
        yield None, requests
        return
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            requests.append({'method': self.command, 'path': self.path})
            self.send_response(401)
            self.send_header('WWW-Authenticate', 'Bearer realm="isolated-fixture"')
            self.send_header('Content-Length', '0')
            self.end_headers()
        do_GET = do_POST
        def log_message(self, *args):
            pass
    server = HTTPServer(('127.0.0.1', 0), Handler)
    server.timeout = 1
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f'http://127.0.0.1:{server.server_port}/mcp', requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


def ten_client_paths(fixture, system):
    """Synthetic discovery surfaces from clientdetect/detector.go.

    The explicit system argument permits path-contract tests on either host;
    real runs always use the native system, including the native scanner cache.
    """
    check(system in ('Linux', 'Darwin'), 'ten-profile fixture supports Linux and Darwin only')
    config = Path(fixture.env['XDG_CONFIG_HOME'])
    editor = (fixture.home / 'Library/Application Support' if system == 'Darwin' else config)
    paths = (fixture.home / '.copilot', fixture.home / '.kiro',
             Path(fixture.env['CLAUDE_CONFIG_DIR']),
             Path(fixture.env['GEMINI_CLI_HOME']) / '.gemini',
             editor / 'Code/User/globalStorage/saoudrizwan.claude-dev',
             config / 'opencode', fixture.home / '.codeium/windsurf')
    for path in paths:
        check(path.resolve().is_relative_to(fixture.home.resolve()),
              'discovery path escapes synthetic HOME')
    return paths


def seed_ten_clients(fixture):
    for path in ten_client_paths(fixture, platform.system()):
        path.mkdir(parents=True, exist_ok=True)
    # Claude selection requires its CLI surface, not only its config directory.
    for name in ('copilot', 'code', 'kiro-cli', 'claude', 'gemini', 'opencode', 'windsurf'):
        shutil.copy2(fixture.bin / 'cursor', fixture.bin / name)
    fixture.before = fixture.mutations()


def choices(raw):
    # Only actual checkbox rows, not arbitrary client words in diagnostics.
    return list(dict.fromkeys(re.findall(r'\[[^\]]*\][^\r\n]*\(([a-z0-9_-]+)\)', clean(raw))))


def send(session, keys):
    session.events.append({'event': 'key', 'hex': keys.hex(), 'offset': len(session.raw),
                           'elapsed': round(time.monotonic() - session.start, 3)})
    session.send(keys)


def exact_skill(fixture):
    fixture.installed(['cursor'])
    state = json.loads((fixture.data / 'state-v2.json').read_text())
    bindings = list(state['installations'][0]['clients'].values())
    target = Path(bindings[0]['target_locator'])
    expected = hashes(fixture.package / 'skills')
    check(hashes(target / 'skills') == expected, 'installed skill files/bytes differ from source')
    native = hashes(target)
    files = {name for name, digest in native.items() if digest != 'directory'}
    expected_files = {'plugin.json', '.cursor-plugin/plugin.json',
                      'skills/guide/SKILL.md', 'skills/guide/reference.txt'}
    check(files == expected_files, f'unexpected native files: {files ^ expected_files}')
    check(json.loads((target / '.cursor-plugin/plugin.json').read_text()) == {
        'name': 'pty-synthetic', 'version': '1.0.0', 'skills': './skills/'},
        'Cursor projected manifest differs from adapter contract')
    check(json.loads((target / 'plugin.json').read_text()) ==
          json.loads((fixture.package / 'plugin.json').read_text()), 'portable manifest differs')
    check(hashes(fixture.home / '.codex') == {}, 'unselected Codex changed')
    return target


def json_plan(fixture, binary, evidence, targets, timeout):
    # Separate read-only inspection after the actual keyboard consent decision;
    # this does not bypass selection or authorize installation/activation.
    argv = [str(binary), 'add', str(fixture.package), '--target=' + ','.join(targets),
            '--dry-run', '--format=json']
    (evidence / 'plan-command.json').write_text(json.dumps(argv))
    result = subprocess.run(argv, cwd=fixture.project, env=fixture.env,
                            capture_output=True, timeout=timeout)
    (evidence / 'plan.json').write_bytes(result.stdout)
    (evidence / 'plan.stderr').write_bytes(result.stderr)
    check(result.returncode == 0, 'JSON plan failed')
    fixture.unchanged()
    data = json.loads(result.stdout)['data']
    return data.get('targets', [{'target': targets[0], 'output': data}])


def check_auth_unknown(plan):
    check(plan['authentication'] == 'not_checked', 'plan overstated authentication')
    check(plan['verification'] == 'package_validated', 'plan overstated runtime/installation verification')


def run_case(case, binary, evidence, timeout=8):
    kind, action = case.split(':')
    check(case in CASES, 'unknown case')
    evidence.mkdir(parents=True)
    with tempfile.TemporaryDirectory(prefix='uap-plugin-matrix-') as tmp, local_endpoint(kind == 'http-auth') as (url, requests):
        fixture = Fixture(tmp)
        package(fixture, kind, url)
        if action == 'all-ten':
            seed_ten_clients(fixture)
        if url:
            fixture.env['NO_PROXY'] = '127.0.0.1'
        shutil.copytree(fixture.package, evidence / 'package')
        if kind == 'collision':
            shutil.copytree(fixture.home / '.cursor/plugins/local/foreign', evidence / 'unowned-native')
        (evidence / 'fixture.json').write_text(json.dumps({
            'kind': kind, 'action': action, 'package_hashes': hashes(fixture.package),
            'collision': kind == 'collision', 'scanner': 'synthetic protocol; not security proof',
            'endpoint': url}, indent=2))
        session = Session([str(binary), 'add', str(fixture.package)], fixture, evidence, timeout)
        try:
            if kind == 'malformed':
                session.finish(1)
                check(re.search(r'(?i)(manifest|plugin.json|json)', clean(session.raw)), 'missing manifest diagnostic')
                check(not re.search(CONFIRM, clean(session.raw)), 'invalid manifest reached consent')
                fixture.unchanged()
                return
            session.wait(SELECT, 'selection')
            session.wait(r'enter submit', 'choices-ready')
            skipped = re.findall(r'Skipped installed clients[^\r\n]*', clean(session.raw))
            (evidence / 'selection.json').write_text(json.dumps({
                'compatible': choices(session.raw), 'skipped': skipped}, indent=2))
            check(not skipped, f'unexpected skipped client/reason: {skipped}')
            if action == 'all-ten':
                expected = {'codex', 'cursor', 'copilot', 'vscode', 'kiro', 'claude',
                            'gemini', 'opencode', 'cline', 'windsurf'}
                offered = set(choices(session.raw))
                (evidence / 'compatible-list.json').write_text(json.dumps(sorted(offered)))
                if offered != expected:
                    send(session, b'\x1b')
                    session.finish(1)
                    fixture.unchanged()
                    raise AssertionError(f'empty package ten-profile compatibility mismatch: missing={sorted(expected - offered)}, extra={sorted(offered - expected)}')
                offset = len(session.raw)
                send(session, b'\r')
                session.wait(CONFIRM, 'all-ten-confirmation', after=offset)
                session.wait(r'(?s)Yes.*?No.*?enter submit', 'all-ten-controls', after=offset)
                text = clean(session.raw[offset:])
                planned = re.findall(r'Target: ([a-z]+)', text)
                check('pty-synthetic' in text and '1.0.0' in text, 'ten-target plan identity missing')
                fixture.unchanged()
                send(session, b'\r')
                session.finish()
                fixture.unchanged()
                check(set(planned) == expected and len(planned) == len(expected),
                      f'ten-target plan differs from selected identities: {planned}')
                check('Uses shared physical binding owned by copilot' in text,
                      'shared physical owner not explained')
                records = json_plan(fixture, binary, evidence, planned, timeout)
                check({r['target'] for r in records} == expected and len(records) == 10,
                      'JSON lost selected logical identities')
                plans = {r['target']: r['output']['result']['plan'] for r in records}
                for target, plan in plans.items():
                    check_auth_unknown(plan)
                    check(plan['client_id'] == ('copilot' if target == 'vscode' else target),
                          'unexpected physical plan owner')
                check(plans['copilot'] == plans['vscode'],
                      'shared targets have different physical bindings')
                return
            check(choices(session.raw) == ['codex', 'cursor'],
                  f'compatible choices differ: {choices(session.raw)}')
            fixture.unchanged()
            if action == 'cancel':
                send(session, b'\x1b')
                session.finish(1)
                fixture.unchanged()
                return
            offset = len(session.raw)
            # Actual Space / Down / Enter: deselect Codex, select only Cursor.
            send(session, b' \x1b[B\r')
            if action == 'reject':
                deadline = time.monotonic() + timeout
                while session.process.poll() is None and not re.search(CONFIRM, clean(session.raw[offset:])):
                    check(time.monotonic() < deadline, 'no rejection or consent within bound')
                    session.pump()
                if re.search(CONFIRM, clean(session.raw[offset:])):
                    session.wait(r'(?s)Yes.*?No.*?enter submit', 'unexpected-consent', after=offset)
                    fixture.unchanged()
                    send(session, b'\r')
                    session.finish()
                    fixture.unchanged()
                    raise AssertionError(f'{kind}: reached consent instead of fail-closed rejection; default No preserved zero changes')
                session.finish(1)
                fixture.unchanged()
                text = clean(session.raw[offset:]).lower()
                check('cursor' in text, 'failure omitted selected client identity (Cursor)')
                pattern = (r'(unmanaged|unowned|collision|ownership)' if kind == 'collision' else
                           r'(runtime|executable|command).*?(missing|not found|unavailable)|missing.*?(runtime|executable|command)')
                check(re.search(pattern, text, re.S), 'missing fail-closed diagnostic')
                check(re.search(r'(install|remove|choose|verify|authenticate)', text), 'missing remediation')
                check(not re.search(CONFIRM, text), 'unsafe plan reached consent')
                return
            session.wait(CONFIRM, 'confirmation', after=offset)
            session.wait(r'(?s)Yes.*?No.*?enter submit', 'confirmation-ready', after=offset)
            text = clean(session.raw[offset:])
            check(all(value in text for value in ('pty-synthetic', '1.0.0', 'Target: cursor', 'Planned action:')),
                  'full client-specific plan missing before consent')
            check('Target: codex' not in text, 'unselected target entered plan')
            fixture.unchanged()
            if kind in ('skill', 'mixed'):
                check('skill guide: native' in text, 'skill support decision missing')
            if kind == 'mixed':
                check(all(value in text for value in ('mcp_server fixture: unsupported',
                      'stdio_runtime_unavailable', 'uap-fixture-missing-runtime', 'install it explicitly',
                      'components_skipped_local_readiness')), 'MCP skipped reason/remediation missing')
            if kind in ('mixed', 'http-auth'):
                check('Authentication: not_checked' in text, 'auth uncertainty label lost')
                check('Verification: package_validated' in text, 'verification overstated')
                check('Installed and verified' not in text and 'Ready' not in text, 'plan falsely claimed usability')
            if kind == 'http-auth':
                check('mcp_server fixture: native' in text, 'HTTP support decision missing')
                check('Authentication: not_checked' in text, 'HTTP auth status overstated')
                check(re.search(r'(?i)verify.*authentication', text), 'missing actionable auth guidance')
            if kind in ('empty', 'unsupported'):
                check(not re.search(r'  - (skill|mcp_server|hook|command) ', text),
                      'loader invented portable components')
            if action in ('default-no', 'partial-plan', 'auth-unknown'):
                send(session, b'\r')
                session.finish()
                fixture.unchanged()
                if action in ('partial-plan', 'auth-unknown'):
                    records = json_plan(fixture, binary, evidence, ['cursor'], timeout)
                    check(len(records) == 1 and records[0]['target'] == 'cursor', 'JSON target mismatch')
                    plan = records[0]['output']['result']['plan']
                    check_auth_unknown(plan)
                    components = {(c['kind'], c['name']): c for c in plan['components']}
                    mcp = components[('mcp_server', 'fixture')]
                    if action == 'partial-plan':
                        check(components[('skill', 'guide')]['support'] == 'native', 'healthy skill lost')
                        check(mcp['support'] == 'unsupported' and mcp['reason'] == 'stdio_runtime_unavailable',
                              'missing runtime skip differs from contract')
                    else:
                        check(mcp['support'] == 'native', 'HTTP component lost')
                        check(requests == [], 'installation unexpectedly contacted HTTP endpoint')
                return
            send(session, b' \r')
            session.wait(LIFECYCLE, 'activation')
            target = exact_skill(fixture)
            (evidence / 'installed-files.json').write_text(json.dumps(hashes(target), indent=2))
            send(session, b'n\n')
            session.finish()
            check('Have you completed required authentication' not in clean(session.raw), 'No advanced into auth')
            exact_skill(fixture)
            # Noninteractive removal is explicitly authorized synthetic cleanup;
            # add/selection never bypasses the keyboard with --target.
            remove_argv = [str(binary), 'remove', 'pty-synthetic', '--target=cursor', '--external-uninstalled']
            (evidence / 'remove-command.json').write_text(json.dumps(remove_argv))
            result = subprocess.run(remove_argv, cwd=fixture.project, env=fixture.env,
                                    capture_output=True, timeout=timeout)
            (evidence / 'remove.stdout').write_bytes(result.stdout)
            (evidence / 'remove.stderr').write_bytes(result.stderr)
            check(result.returncode == 0, 'synthetic CLI removal failed')
            check(not target.exists(), 'CLI removal retained native package')
            state = json.loads((fixture.data / 'state-v2.json').read_text())
            check(all(binding.get('materialization') != 'materialized'
                      for install in state.get('installations', [])
                      for binding in install.get('clients', {}).values()), 'removal left materialized binding')
            check(hashes(fixture.home / '.codex') == {}, 'removal changed unselected Codex')
        finally:
            session.close()
            (evidence / 'http-requests.json').write_text(json.dumps(requests))
            (evidence / 'mutations.json').write_text(json.dumps({
                'before': fixture.before, 'after': fixture.mutations()}, indent=2))
            fixture.validate_stubs()
            if kind == 'http-auth':
                check(requests == [], 'installation unexpectedly contacted HTTP endpoint')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--artifacts', type=Path, required=True)
    parser.add_argument('--case', action='append', choices=CASES)
    parser.add_argument('--timeout', type=float, default=8)
    args = parser.parse_args()
    check(os.name == 'posix', 'coverage gap: rich keyboard ConPTY lane not qualified; use native Windows harness separately')
    binary = args.binary.resolve(strict=True)
    args.artifacts.mkdir(parents=True, exist_ok=False)
    source = Path(__file__).resolve().parents[2]
    report = {'source_sha': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=source, text=True).strip(),
              'binary': str(binary), 'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
              'binary_source_link': 'Supplied binary SHA256 recorded; consult external build receipt for source parity.',
              'platform': platform.system(),
              'audit_matrix': AUDIT_MATRIX,
              'fixture_contracts': {
                  'manifest': 'install/integrationctl/agentplugins/adapters/specregistry/schemas/1.0.0/plugin.schema.json',
                  'mcp': 'install/integrationctl/agentplugins/adapters/specregistry/schemas/1.0.0/mcp.schema.json',
                  'loader': 'install/integrationctl/agentplugins/adapters/loader/loader_test.go',
                  'projection': 'install/integrationctl/agentplugins/providers/stager_test.go',
                  'ownership': 'install/integrationctl/agentplugins/providers/native_identity_test.go',
                  'discovery': 'install/integrationctl/agentplugins/adapters/clientdetect/detector.go'},
              'gaps': ['Ten-profile empty run uses synthetic native Linux/Darwin discovery paths. ChatGPT is an eleventh client with no config-only surface; omitted.',
                       'This report covers only the recorded native platform and listed cases; Windows ConPTY is a separate lane.',
                       'Scanner is synthetic; no security/runtime/client activation qualification.'], 'cases': []}
    for case in args.case or CASES:
        entry = {'case': case, 'status': 'pass',
                 'repro': shlex.join(['python3', str(Path(__file__).resolve()), '--binary', str(binary),
                                     '--case', case, '--artifacts', '/tmp/uap-plugin-matrix-repro-' + case.replace(':', '-')])}
        try:
            run_case(case, binary, args.artifacts / case.replace(':', '-'), args.timeout)
        except Exception as error:
            entry.update(status='fail', error=str(error), fix_owner='Unassigned; coordinating owner to triage. Production files outside lane scope.')
        report['cases'].append(entry)
        print(f"{entry['status']}: {case}: {entry.get('error', '')}", flush=True)
    report['evidence_hashes'] = hashes(args.artifacts)
    report['source_files'] = {name: hashlib.sha256((Path(__file__).parent / name).read_bytes()).hexdigest()
                              for name in ('plugin_matrix.py', 'test_plugin_matrix.py', 'harness.py', 'pty_owner.py')}
    (args.artifacts / 'report.json').write_text(json.dumps(report, indent=2))
    return int(any(case['status'] == 'fail' for case in report['cases']))


if __name__ == '__main__':
    raise SystemExit(main())
