import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';
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

for (const [mode, code] of [['version','UNSUPPORTED_API_VERSION'], ['remote','INVALID_BOOTSTRAP'], ['mismatch','SESSION_MISMATCH'], ['malformed','INVALID_BOOTSTRAP'], ['oversized','BOOTSTRAP_TOO_LARGE'], ['exit','PROCESS_EXITED'], ['timeout','STARTUP_TIMEOUT']] as const) {
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
