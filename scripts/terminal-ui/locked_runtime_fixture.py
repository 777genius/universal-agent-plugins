#!/usr/bin/env python3
"""Locked npm runtime fixture: synthetic package, stub npm seam, cold probes.

Issue 346 contract under test: installation must materialize the locked npm
runtime BEFORE a client's first MCP probe, so a short diagnostic timeout
(Gemini `mcp list` hardcodes 5s) can never race a lazy `npm ci`. The fixture
launcher never runs npm: it fails fast when the runtime was not prepared. The
stub npm boundary records every invocation, checks the reviewed safe argument
and environment contract, and can fail or sleep on demand.

Contract sources: install/integrationctl/agentplugins/providers/
locked_npm_runtime.go (runtime.json/package.json/package-lock.json validation),
locked_npm_command.go (npm arguments and environment scrub), and
clients/shared/stdio.go (projected command/args/env the probes execute).
"""

import base64
import json
import os
from pathlib import Path
import select
import shutil
import subprocess
import threading
import time

from harness import check

SCHEMA = 'https://agent-plugins.org/schemas/1.0.0/'
RUNTIME_DIR = 'io.github.777genius.agentplugins/runtime'
LAUNCHER_ARG = '${PLUGIN_ROOT}/' + RUNTIME_DIR + '/launcher.mjs'
PACKAGE = 'fixture-mcp'
ENTRYPOINT = 'node_modules/' + PACKAGE + '/server.mjs'
SERVER_NAME = 'locked-fixture'
SAFE_CI_ARGS = ['ci', '--ignore-scripts', '--omit=dev', '--no-audit', '--no-fund']
PROBE_BUDGET = 5.0


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2) + '\n', encoding='utf-8')


LAUNCHER = '''// Fail-fast fixture launcher. It NEVER runs npm or package lifecycle code:
// an unprepared runtime is an installation failure, not a first-probe task.
import { appendFileSync, existsSync, readFileSync } from 'node:fs';
import { spawn } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const runtimeDir = path.dirname(fileURLToPath(import.meta.url));
const config = JSON.parse(readFileSync(path.join(runtimeDir, 'runtime.json'), 'utf8'));
const data = process.env.PLUGIN_DATA;
if (!data) {
  console.error('locked runtime fixture: PLUGIN_DATA is not set');
  process.exit(126);
}
appendFileSync(path.join(data, 'launcher-executed.log'), new Date().toISOString() + '\\n');
const target = path.join(data, 'npm-runtime',
  config.package_lock_sha256.replace(/^sha256:/, ''));
const markerPath = path.join(target, '.agentplugins-runtime.json');
if (!existsSync(markerPath)) {
  console.error('locked runtime fixture: runtime is not prepared; '
    + 'installation must prewarm before the first client probe');
  process.exit(126);
}
const marker = JSON.parse(readFileSync(markerPath, 'utf8'));
if (marker.package !== config.package || marker.version !== config.version ||
    marker.entrypoint !== config.entrypoint) {
  console.error('locked runtime fixture: prepared marker does not match runtime.json');
  process.exit(126);
}
const entry = path.join(target, ...config.entrypoint.split('/'));
if (!existsSync(entry)) {
  console.error('locked runtime fixture: prepared entrypoint is missing');
  process.exit(126);
}
const child = spawn(process.execPath, [entry, ...process.argv.slice(2)],
  { stdio: 'inherit' });
child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code === null ? 1 : code);
});
'''

SERVER = '''let buffer = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
  buffer += chunk;
  for (;;) {
    const index = buffer.indexOf('\\n');
    if (index < 0) break;
    const line = buffer.slice(0, index).trim();
    buffer = buffer.slice(index + 1);
    if (!line) continue;
    let message;
    try { message = JSON.parse(line); } catch { continue; }
    if (message.id === undefined) continue;
    const result = message.method === 'initialize'
      ? { protocolVersion: '2024-11-05', capabilities: {},
          serverInfo: { name: 'fixture-mcp', version: '__VERSION__' } }
      : {};
    process.stdout.write(JSON.stringify(
      { jsonrpc: '2.0', id: message.id, result }) + '\\n');
  }
});
process.stdin.on('end', () => process.exit(0));
'''

STUB_NPM = '''#!/__NODE__
'use strict';
const fs = require('fs');
const path = require('path');
const root = __ROOT__;
const log = path.join(root, 'npm-stub.log');
const args = process.argv.slice(2);
fs.appendFileSync(log, 'CALL ' + JSON.stringify({
  args,
  env: {
    ignore_scripts: process.env.npm_config_ignore_scripts || null,
    registry: process.env.npm_config_registry || null,
    cache: process.env.npm_config_cache || null,
    audit: process.env.npm_config_audit || null,
    home: process.env.HOME || null,
  },
}) + '\\n');
if (args.length === 1 && args[0] === '--version') {
  process.stdout.write('10.9.9\\n');
  process.exit(0);
}
if (args[0] !== 'ci') {
  fs.appendFileSync(log, 'REJECT unexpected-args\\n');
  process.stderr.write('stub npm: unexpected invocation\\n');
  process.exit(97);
}
if (fs.existsSync(path.join(root, 'npm-fail'))) {
  fs.appendFileSync(log, 'FAIL\\n');
  process.stderr.write('stub npm: synthetic preparation failure\\n');
  process.exit(1);
}
fs.appendFileSync(log, 'START\\n');
let delay = 0;
const delayFile = path.join(root, 'npm-delay');
if (fs.existsSync(delayFile)) {
  delay = parseFloat(fs.readFileSync(delayFile, 'utf8').trim() || '0');
}
fs.appendFileSync(log, 'SLEEP ' + delay + '\\n');
setTimeout(() => {
  const manifest = JSON.parse(
    fs.readFileSync(path.join(process.cwd(), 'package.json'), 'utf8'));
  const name = Object.keys(manifest.dependencies || {})[0];
  const dest = path.join(process.cwd(), 'node_modules', name);
  fs.mkdirSync(dest, { recursive: true });
  const payload = path.join(root, 'npm-payload');
  for (const entry of fs.readdirSync(payload)) {
    fs.copyFileSync(path.join(payload, entry), path.join(dest, entry));
  }
  fs.mkdirSync(path.join(process.cwd(), 'node_modules', '.bin'), { recursive: true });
  fs.appendFileSync(log, 'DONE\\n');
  process.exit(0);
}, Math.max(0, delay) * 1000);
'''


def integrity(seed):
    return 'sha512-' + base64.b64encode(
        (seed.encode() * 64)[:64]).decode()


class LockedRuntimeFixture:
    """Owns the synthetic locked-runtime package and its instrumented npm seam."""

    def __init__(self, fixture, version='1.0.0'):
        self.fixture = fixture
        self.version = version
        self.digest = ''
        self.data_path = None
        self._write_package(version)
        self._write_stub_npm()
        self._link_node()

    def rewrite_version(self, version):
        self.version = version
        self._write_package(version)

    def _write_package(self, version):
        package = self.fixture.package
        write_json(package / 'plugin.json', {
            '$schema': SCHEMA + 'plugin.schema.json',
            'name': 'pty-synthetic', 'version': version})
        write_json(package / 'mcp.json', {
            '$schema': SCHEMA + 'mcp.schema.json',
            'mcpServers': {SERVER_NAME: {
                'type': 'stdio', 'command': 'node', 'args': [LAUNCHER_ARG]}}})
        lock = {
            'lockfileVersion': 3, 'requires': True,
            'packages': {
                '': {'dependencies': {PACKAGE: version}},
                'node_modules/' + PACKAGE: {
                    'version': version,
                    'resolved': ('https://registry.npmjs.org/' + PACKAGE +
                                 '/-/' + PACKAGE + '-' + version + '.tgz'),
                    'integrity': integrity(version),
                },
            },
        }
        lock_body = json.dumps(lock, indent=2) + '\n'
        import hashlib
        self.digest = hashlib.sha256(lock_body.encode()).hexdigest()
        runtime = package / RUNTIME_DIR
        runtime.mkdir(parents=True, exist_ok=True)
        (runtime / 'runtime.json').write_text(json.dumps({
            'schema_version': 1, 'package': PACKAGE, 'version': version,
            'entrypoint': ENTRYPOINT,
            'package_lock_sha256': 'sha256:' + self.digest,
            'omit_optional': False}, indent=2) + '\n', encoding='utf-8')
        (runtime / 'package.json').write_text(json.dumps({
            'name': 'locked-runtime-fixture', 'private': True,
            'dependencies': {PACKAGE: version}}, indent=2) + '\n',
            encoding='utf-8')
        (runtime / 'package-lock.json').write_text(lock_body, encoding='utf-8')
        (runtime / 'launcher.mjs').write_text(LAUNCHER, encoding='utf-8')
        payload = self.fixture.root / 'npm-payload'
        payload.mkdir(parents=True, exist_ok=True)
        (payload / 'server.mjs').write_text(
            SERVER.replace('__VERSION__', version), encoding='utf-8')

    def _write_stub_npm(self):
        node = shutil.which('node')
        check(node is not None, 'node is required for the locked runtime fixture')
        body = (STUB_NPM.replace('#!/__NODE__', '#!' + node)
                .replace('__ROOT__', json.dumps(str(self.fixture.root))))
        stub = self.fixture.bin / 'npm'
        stub.write_text(body, encoding='utf-8')
        stub.chmod(0o700)

    def _link_node(self):
        node = shutil.which('node')
        check(node is not None, 'node is required for the locked runtime fixture')
        link = self.fixture.bin / 'node'
        if not link.exists():
            link.symlink_to(node)

    # --- npm seam -----------------------------------------------------------

    def npm_log(self):
        path = self.fixture.root / 'npm-stub.log'
        return path.read_text(encoding='utf-8') if path.exists() else ''

    def npm_calls(self):
        calls = []
        for line in self.npm_log().splitlines():
            if line.startswith('CALL '):
                calls.append(json.loads(line[len('CALL '):]))
        return calls

    def check_npm_contract(self):
        calls = self.npm_calls()
        check(calls, 'the locked npm seam was never invoked')
        for call in calls:
            check(call['args'] in (SAFE_CI_ARGS, ['--version']),
                  f'npm seam saw unsafe arguments: {call["args"]}')
            if call['args'] == SAFE_CI_ARGS:
                env = call['env']
                check(env['ignore_scripts'] == 'true',
                      'npm ci ran without npm_config_ignore_scripts=true')
                check(env['registry'] == 'https://registry.npmjs.org/',
                      'npm ci left the pinned registry')
                check(env['audit'] == 'false',
                      'npm ci ran with the audit surface enabled')

    def ci_runs(self):
        return sum(1 for line in self.npm_log().splitlines()
                   if line == 'START')

    def set_delay(self, seconds):
        (self.fixture.root / 'npm-delay').write_text(
            f'{seconds}\n', encoding='utf-8')

    def fail_next(self):
        (self.fixture.root / 'npm-fail').write_text('1\n', encoding='utf-8')

    def clear_controls(self):
        for name in ('npm-delay', 'npm-fail'):
            path = self.fixture.root / name
            if path.exists():
                path.unlink()

    # --- prepared runtime store ---------------------------------------------

    def npm_stores(self):
        return [path for path in self.fixture.data.rglob('npm-runtime')
                if path.is_dir()]

    def capture_data_path(self):
        servers = self.projected_servers()
        check(servers, 'no projected locked-runtime server to read PLUGIN_DATA from')
        values = {server['env'].get('PLUGIN_DATA') for server in servers}
        check(len(values) == 1 and None not in values,
              f'projected PLUGIN_DATA is not one owned locator: {values}')
        captured = Path(values.pop())
        check(captured.resolve().is_relative_to(self.fixture.root.resolve()),
              f'captured PLUGIN_DATA escapes the fixture root: {captured}')
        self.data_path = captured
        return self.data_path

    def store(self):
        data = self.capture_data_path()
        return data / 'npm-runtime' / self.digest

    def assert_store_ready(self):
        target = self.store()
        marker = json.loads(
            (target / '.agentplugins-runtime.json').read_text(encoding='utf-8'))
        check(marker['package'] == PACKAGE and marker['version'] == self.version,
              f'prepared runtime marker does not bind {PACKAGE} '
              f'{self.version}: {marker}')
        entry = target / ENTRYPOINT
        check(entry.is_file(), 'prepared runtime lost its entrypoint')
        check((target / 'node_modules' / '.bin').is_dir(),
              'prepared runtime lost node_modules/.bin')

    def assert_store_absent(self):
        for target in self.npm_stores():
            for child in target.iterdir():
                if child.name == self.digest:
                    check(not (child / '.agentplugins-runtime.json').exists(),
                          'interrupted preparation left a committed runtime')

    def assert_store_purged(self):
        if self.data_path is None:
            self.capture_data_path()
        check(not self.data_path.exists(),
              f'purge-data kept the plugin data locator: {self.data_path}')

    def delete_entrypoint(self):
        (self.store() / ENTRYPOINT).unlink()

    # --- launcher execution evidence ---------------------------------------

    def launcher_runs(self):
        if self.data_path is not None:
            path = self.data_path / 'launcher-executed.log'
            return len(path.read_text(encoding='utf-8').splitlines()) \
                if path.exists() else 0
        # Without a committed PLUGIN_DATA locator, count any execution the
        # installer could have written anywhere in its data root; returning
        # zero here would make the failure-lane negatives vacuous.
        total = 0
        for path in (self.fixture.data / 'plugin-data').glob(
                '**/launcher-executed.log'):
            total += len(path.read_text(encoding='utf-8').splitlines())
        return total

    def assert_launcher_runs(self, expected):
        check(self.launcher_runs() == expected,
              f'fixture launcher executions = {self.launcher_runs()}, '
              f'want {expected}')

    # --- client probes ------------------------------------------------------

    def projected_servers(self):
        config = Path(self.fixture.env['XDG_CONFIG_HOME']) / 'opencode'
        gemini = Path(self.fixture.env['GEMINI_CLI_HOME']) / '.gemini'
        candidates = [
            ('gemini', gemini / 'settings.json'),
            ('opencode', config / 'opencode.json'),
            ('opencode', config / 'opencode.jsonc'),
        ]
        found = []
        for label, path in candidates:
            if not path.is_file():
                continue
            body = path.read_text(encoding='utf-8')
            try:
                document = json.loads(body)
            except json.JSONDecodeError:
                document = json.loads('\n'.join(
                    line for line in body.splitlines()
                    if not line.lstrip().startswith('//')))
            for name, entry in walk_servers(document):
                argv = server_argv(entry)
                if any(value.endswith('runtime/launcher.mjs') for value in argv):
                    found.append({
                        'label': label, 'name': name, 'argv': argv,
                        'env': dict(entry.get('env') or
                                    entry.get('environment') or {}),
                        'source': str(path),
                    })
        labels = [row['label'] for row in found]
        check(sorted(labels) == ['gemini', 'opencode'],
              f'projected locked-runtime servers differ from the selected '
              f'targets: {found}')
        return found

    def probe(self, evidence, bound=PROBE_BUDGET):
        servers = self.projected_servers()
        results = [None] * len(servers)

        def run(index):
            # A thread that dies before recording leaves a silent None; keep the
            # failure reason visible in the probe evidence instead.
            try:
                results[index] = probe_server(
                    servers[index],
                    dict(self.fixture.env, **servers[index]['env']), bound)
            except Exception as error:
                results[index] = {
                    'label': servers[index]['label'], 'ok': False,
                    'probe_error': f'{type(error).__name__}: {error}'}

        started = time.monotonic()
        threads = [threading.Thread(target=run, args=(index,))
                   for index in range(len(servers))]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(timeout=bound + 3)
        elapsed = time.monotonic() - started
        evidence.mkdir(parents=True, exist_ok=True)
        (evidence / 'probe-results.json').write_text(json.dumps({
            'bound_seconds': bound, 'batch_elapsed_seconds':
                round(elapsed, 3), 'results': results}, indent=2) + '\n',
            encoding='utf-8')
        check(not any(thread.is_alive() for thread in threads),
              'cold probe batch outlived its deadline')
        # Each probe enforces the real per-server deadline; the batch check
        # only guards against serialization and allows thread-spawn overhead.
        check(elapsed <= bound + 1.0,
              f'cold probe batch took {elapsed:.2f}s, beyond the '
              f'{bound}s diagnostic budget')
        for result in results:
            check(result and result['ok'],
                  f'cold probe failed within the diagnostic budget: {result}')
        return results


def walk_servers(node):
    if isinstance(node, dict):
        if 'command' in node and isinstance(node.get('command'),
                                            (str, list)):
            yield 'server', node
        for key, value in node.items():
            for name, entry in walk_servers(value):
                yield f'{key}/{name}', entry
    elif isinstance(node, list):
        for value in node:
            for name, entry in walk_servers(value):
                yield name, entry


def server_argv(entry):
    command = entry.get('command')
    args = entry.get('args') or []
    if isinstance(command, list):
        return [str(value) for value in command]
    return [str(command)] + [str(value) for value in args]


INIT_REQUEST = json.dumps({
    'jsonrpc': '2.0', 'id': 1, 'method': 'initialize',
    'params': {
        'protocolVersion': '2024-11-05', 'capabilities': {},
        'clientInfo': {'name': 'cold-probe', 'version': '1.0'}}})


def read_line(stream, deadline):
    while time.monotonic() < deadline:
        remaining = max(0, deadline - time.monotonic())
        ready, _, _ = select.select([stream], [], [], min(0.1, remaining))
        if not ready:
            continue
        line = stream.readline()
        if line:
            return line
        return None
    return None


def probe_server(server, env, bound):
    started = time.monotonic()
    deadline = started + bound
    process = subprocess.Popen(
        server['argv'], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
        stderr=subprocess.PIPE, env=env, text=True)
    result = {'label': server['label'], 'name': server['name'],
              'argv': server['argv'], 'ok': False}
    try:
        process.stdin.write(INIT_REQUEST + '\n')
        process.stdin.flush()
        line = read_line(process.stdout, deadline)
        if line:
            message = json.loads(line)
            info = message.get('result', {}).get('serverInfo', {})
            result['ok'] = (message.get('id') == 1 and
                            info.get('name') == PACKAGE)
            result['serverInfo'] = info
        result['elapsed_seconds'] = round(time.monotonic() - started, 3)
        # communicate() after an explicit stdin close raises ValueError on
        # newer CPython; close the request stream, wait, then drain stderr.
        try:
            process.stdin.close()
        except OSError:
            pass
        try:
            process.wait(timeout=2)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=2)
        result['stderr'] = process.stderr.read()
        result['exit'] = process.returncode
    finally:
        if process.poll() is None:
            process.kill()
            process.wait(timeout=2)
    return result
