import assert from 'node:assert/strict';
import { ChildProcess, spawnSync } from 'node:child_process';
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

test('discovery preserves native profile locators while filtering credentials and launch options', async () => {
  const input = await options('environment');
  const profiles = {
    pi: join(directory, 'selected-pi-profile'),
    qoder: join(directory, 'selected-qoder-profile'),
  };
  const result = await discoverTargets({
    ...input,
    env: {
      ...input.env,
      PI_CODING_AGENT_DIR: profiles.pi,
      QODER_CONFIG_DIR: profiles.qoder,
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
  for (const [client, configRoot] of Object.entries(profiles)) {
    assert.deepEqual(result.targets.find((target) => target.client === client)?.profile, {
      id: client,
      client,
      configRoot,
      detectionStatus: 'detected',
    });
  }
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

test('repeated denied discovery signals stay handled until bounded cleanup refusal', async (t) => {
  const input = await options('hang');
  await rm(join(directory, 'pid'), { force: true });
  const controller = new AbortController();
  const originalKill = ChildProcess.prototype.kill;
  const owned: { child?: ChildProcess } = {};
  const attempts: Array<NodeJS.Signals | number | undefined> = [];
  const unhandled: unknown[] = [];
  let pid = 0;
  const denied = t.mock.method(
    ChildProcess.prototype,
    'kill',
    function (this: ChildProcess, signal?: NodeJS.Signals | number): boolean {
      if (
        pid === 0 ||
        this.pid !== pid ||
        (signal !== 'SIGTERM' && signal !== 'SIGKILL')
      ) {
        return originalKill.call(this, signal);
      }
      owned.child = this;
      attempts.push(signal);
      try {
        this.emit(
          'error',
          Object.assign(new Error('kill EPERM'), { code: 'EPERM' }),
        );
      } catch (error) {
        // Record an otherwise uncaught event without crashing the test worker.
        unhandled.push(error);
      }
      return false;
    },
  );
  const outcome = discoverTargets({
    ...input,
    timeoutMs: 5_000,
    signal: controller.signal,
  }).then(
    () => undefined,
    (error: unknown) => error,
  );
  try {
    const startupDeadline = Date.now() + 2_000;
    while (pid === 0 && Date.now() < startupDeadline) {
      try {
        pid = Number(await readFile(join(directory, 'pid'), 'utf8'));
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
      }
      if (pid === 0) await delay(10);
    }
    assert.ok(Number.isInteger(pid) && pid > 0, 'Owned discovery child did not start');
    controller.abort();
    const error = await outcome;
    assert.ok(error instanceof ManagementError);
    assert.equal(error.code, 'SHUTDOWN_TIMEOUT');
    assert.deepEqual(attempts, ['SIGTERM', 'SIGKILL']);
    assert.deepEqual(unhandled, []);
    assert.ok(owned.child);
    assert.equal(owned.child.exitCode, null);
    assert.equal(owned.child.signalCode, null);
  } finally {
    denied.mock.restore();
    controller.abort();
    const child = owned.child;
    if (child && child.exitCode === null && child.signalCode === null) {
      originalKill.call(child, 'SIGKILL');
    }
    await outcome;
    if (child) {
      const deadline = Date.now() + 2_000;
      while (
        child.exitCode === null &&
        child.signalCode === null &&
        Date.now() < deadline
      ) {
        await delay(10);
      }
      assert.ok(
        child.exitCode !== null || child.signalCode !== null,
        'Owned discovery cleanup did not observe exit',
      );
    }
  }
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
