import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { startUap, type StartupConfiguration } from '../dist/node.js';
import { ManagementError } from '../dist/index.js';

let directory: string;
let executable: string;
let config: StartupConfiguration;
before(async () => {
  directory = await mkdtemp(join(tmpdir(), 'uap-sdk-host-test-'));
  executable = join(directory, process.platform === 'win32' ? 'fixture.exe' : 'fixture');
  const build = spawnSync('go', ['build', '-o', executable, fileURLToPath(new URL('./fixtures/host.go', import.meta.url))], { env: { ...process.env, GOWORK: 'off' }, encoding: 'utf8' });
  assert.equal(build.status, 0, build.stderr || build.error?.message);
  config = { scope: 'user', stateRoot: join(directory, 'state'), dependenciesRoot: join(directory, 'dependencies'), targets: [{ id: 'claude-test', client: 'claude', configRoot: join(directory, 'claude') }] };
});
after(async () => { if (directory) await rm(directory, { recursive: true, force: true }); });
function options(mode = '') { return { executable, config, startupTimeoutMs: 2_000, shutdownTimeoutMs: 100, env: { ...process.env, UAP_SDK_TEST_MODE: mode } }; }

test('real owned child performs private authenticated bootstrap and repeat close', async () => {
  const host = await startUap({ ...options(), requiredCapabilities: ['install'] });
  assert.equal(host.session.contextId, 'captured-fixture-context');
  assert.ok(host.pid > 0);
  assert.equal('token' in host, false);
  assert.equal((await host.client.session()).apiVersion, 1);
  await Promise.all([host.close(), host.close()]);
  await assert.rejects(host.client.session());
});

test('exposes confirmed child exit without treating a failed HTTP request as death', async () => {
  const host = await startUap(options());
  try {
    assert.equal(host.hasExited, false);
    await assert.rejects(host.client.session({ signal: AbortSignal.abort() }));
    assert.equal(host.hasExited, false);
    assert.equal((await host.client.session()).apiVersion, 1);
    await host.close();
    assert.equal(host.hasExited, true);
    await host.close();
    assert.equal(host.hasExited, true);
  } finally {
    await host.close();
  }
});

test('reports external termination of the exact owned child after readiness', async () => {
  const host = await startUap(options());
  try {
    assert.equal(host.hasExited, false);
    process.kill(host.pid, 'SIGKILL');
    const deadline = Date.now() + 2_000;
    while (!host.hasExited && Date.now() < deadline) await delay(10);
    assert.equal(host.hasExited, true);
    await assert.rejects(host.client.session());
  } finally {
    await host.close();
  }
});

for (const [mode, code] of [['version','UNSUPPORTED_API_VERSION'], ['remote','INVALID_BOOTSTRAP'], ['mismatch','SESSION_MISMATCH'], ['malformed','INVALID_BOOTSTRAP'], ['oversized','BOOTSTRAP_TOO_LARGE'], ['exit','PROCESS_EXITED'], ['timeout','STARTUP_TIMEOUT'], ['environment-missing','SESSION_MISMATCH'], ['environment-empty','SESSION_MISMATCH'], ['environment-number','SESSION_MISMATCH']] as const) {
  test(`rejects ${mode} startup without exposing private output`, async () => {
    await assert.rejects(startUap({ ...options(mode), startupTimeoutMs: 300 }), (error: unknown) => {
      assert.ok(error instanceof ManagementError);
      assert.equal(error.code, code);
      assert.equal(error.message.includes('a'.repeat(64)), false);
      return true;
    });
  });
}

test('required missing capability closes the newly owned child', async () => {
  await assert.rejects(startUap({ ...options(), requiredCapabilities: ['unsupported'] }), (error: unknown) => error instanceof ManagementError && error.code === 'UNSUPPORTED_CAPABILITY');
});

test('shutdown is bounded when the owned process ignores EOF and SIGTERM', async () => {
  const host = await startUap(options('stubborn'));
  const started = Date.now();
  await host.close();
  assert.ok(Date.now() - started < 3_000);
});

test('invalid executable paths fail without invoking a shell or PATH lookup', async () => {
  await assert.rejects(startUap({ ...options(), executable: 'agentplugins' }), (error: unknown) => error instanceof ManagementError && error.code === 'INVALID_EXECUTABLE');
  await assert.rejects(startUap({ ...options(), executable: join(directory, 'missing') }), (error: unknown) => error instanceof ManagementError && error.code === 'PROCESS_START_FAILED');
});


test('default process environment preserves the CLI home and removes duplicate proxy casing', async () => {
  const changes: Record<string, string> = { AGENTPLUGINS_HOME: join(directory, 'custom-cli-home'), HTTP_PROXY: 'http://upper.invalid', http_proxy: 'http://lower.invalid', HTTPS_PROXY: 'http://upper.invalid', https_proxy: 'http://lower.invalid', NO_PROXY: '127.0.0.1', no_proxy: 'ignored.invalid' };
  const previous = Object.fromEntries(Object.keys(changes).map((key) => [key, process.env[key]]));
  let host: Awaited<ReturnType<typeof startUap>> | undefined;
  try {
    Object.assign(process.env, changes);
    host = await startUap({ executable, config, startupTimeoutMs: 2_000, shutdownTimeoutMs: 100 });
    assert.equal(host.session.targets[0]?.displayPath, changes.AGENTPLUGINS_HOME);
    for (const name of ['HTTP_PROXY', 'HTTPS_PROXY', 'NO_PROXY']) {
      assert.equal(host.session.capabilities.find((item) => item.action === `environment:${name}`)?.supported, true);
      // Windows environment lookup itself folds names; the fixture counts the
      // inherited spellings instead of relying on case-insensitive Getenv.
      assert.equal(host.session.capabilities.find((item) => item.action === `environment:${name.toLowerCase()}`)?.supported, false);
    }
  } finally {
    await host?.close();
    for (const [key, value] of Object.entries(previous)) { if (value === undefined) delete process.env[key]; else process.env[key] = value; }
  }
});
