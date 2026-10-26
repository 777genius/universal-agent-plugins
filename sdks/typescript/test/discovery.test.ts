import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFile, mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { discoverTargets } from '../dist/node.js';
import { ManagementError } from '../dist/index.js';

let directory: string;
let fixture: string;
before(async () => {
  directory = await mkdtemp(join(tmpdir(), 'uap-sdk-discovery-test-'));
  fixture = join(directory, process.platform === 'win32' ? 'fixture.exe' : 'fixture');
  const build = spawnSync(
    'go',
    ['build', '-o', fixture, fileURLToPath(new URL('./fixtures/discovery/main.go', import.meta.url))],
    { env: { ...process.env, GOWORK: 'off' }, encoding: 'utf8' },
  );
  assert.equal(build.status, 0, build.stderr || build.error?.message);
});
after(async () => {
  if (directory) await rm(directory, { recursive: true, force: true });
});
async function options(mode = 'valid') {
  const executable = join(directory, mode + (process.platform === 'win32' ? '.exe' : ''));
  await copyFile(fixture, executable);
  return {
    executable,
    timeoutMs: 2_000,
    env: {
      HOME: directory,
      USERPROFILE: directory,
      GEMINI_CLI_HOME: join(directory, 'gemini-profile'),
      ...(process.env.SystemRoot ? { SystemRoot: process.env.SystemRoot } : {}),
    },
  };
}

test('real owned discovery child preserves private canonical facts without starting a server', async () => {
  const input = await options();
  const result = await discoverTargets(input);
  assert.deepEqual(result, {
    apiVersion: 1,
    engineVersion: 'fixture',
    targets: [
      {
        client: 'gemini',
        label: 'Gemini CLI',
        detectionStatus: 'detected',
        available: true,
        profile: {
          id: 'gemini',
          client: 'gemini',
          configRoot: input.env.GEMINI_CLI_HOME,
          detectionStatus: 'detected',
        },
      },
    ],
  });
  assert.equal('client' in result, false);
  assert.equal('token' in result, false);
});

for (const [mode, code] of [
  ['malformed', 'INVALID_DISCOVERY'],
  ['version', 'UNSUPPORTED_API_VERSION'],
  ['relative', 'INVALID_DISCOVERY'],
  ['absent-profile', 'INVALID_DISCOVERY'],
  ['status-mismatch', 'INVALID_DISCOVERY'],
  ['duplicate', 'INVALID_DISCOVERY'],
  ['trailing', 'INVALID_DISCOVERY'],
  ['oversized', 'DISCOVERY_TOO_LARGE'],
  ['exit', 'PROCESS_EXITED'],
] as const) {
  test(`rejects ${mode} discovery without forwarding private output`, async () => {
    await assert.rejects(discoverTargets(await options(mode)), (error: unknown) => {
      assert.ok(error instanceof ManagementError);
      assert.equal(error.code, code);
      assert.equal(error.message.includes('private native'), false);
      assert.equal(error.message.includes('private trailing'), false);
      return true;
    });
  });
}

test('discovery filters credentials and launch options even from an explicit environment', async () => {
  const input = await options('environment');
  const result = await discoverTargets({
    ...input,
    env: {
      ...input.env,
      OPENAI_API_KEY: 'fixture-secret',
      ANTHROPIC_AUTH_TOKEN: 'fixture-secret',
      NODE_OPTIONS: '--unexpected',
      HTTP_PROXY: 'http://fixture-secret@invalid',
      HTTPS_PROXY: 'http://fixture-secret@invalid',
      UAP_SDK_TEST_MODE: 'unexpected',
    },
  });
  assert.equal(result.targets[0]?.label, 'Gemini CLI');
  assert.equal(result.targets[0]?.profile?.configRoot, input.env.GEMINI_CLI_HOME);
});

test('deadline reaps only the owned discovery process even when it ignores SIGTERM', async () => {
  const input = await options('hang');
  const started = Date.now();
  await assert.rejects(
    discoverTargets({ ...input, timeoutMs: 200 }),
    (error: unknown) => error instanceof ManagementError && error.code === 'DISCOVERY_TIMEOUT',
  );
  assert.ok(Date.now() - started < 3_000);
  const pid = Number(await readFile(join(directory, 'pid'), 'utf8'));
  assert.throws(
    () => process.kill(pid, 0),
    (error: unknown) => (error as NodeJS.ErrnoException).code === 'ESRCH',
  );
});

test('abort after child start waits for owned-process cleanup', async () => {
  const input = await options('hang');
  await rm(join(directory, 'pid'), { force: true });
  const controller = new AbortController();
  const reason = new Error('caller canceled discovery');
  const pending = discoverTargets({ ...input, signal: controller.signal });
  let pid = 0;
  for (let attempt = 0; attempt < 50 && !pid; attempt++) {
    try {
      pid = Number(await readFile(join(directory, 'pid'), 'utf8'));
    } catch {
      await delay(10);
    }
  }
  assert.ok(pid > 0, 'owned child did not start');
  controller.abort(reason);
  await assert.rejects(pending, (error: unknown) => error === reason);
  assert.throws(
    () => process.kill(pid, 0),
    (error: unknown) => (error as NodeJS.ErrnoException).code === 'ESRCH',
  );
});

test('rejects relative or missing engine paths without shell or PATH fallback', async () => {
  await assert.rejects(
    discoverTargets({ executable: 'agentplugins' }),
    (error: unknown) => error instanceof ManagementError && error.code === 'INVALID_EXECUTABLE',
  );
  await assert.rejects(
    discoverTargets({ executable: join(directory, 'missing') }),
    (error: unknown) => error instanceof ManagementError && error.code === 'PROCESS_START_FAILED',
  );
});
