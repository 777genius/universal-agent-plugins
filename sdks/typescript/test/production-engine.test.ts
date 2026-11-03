import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { mkdir, mkdtemp, readFile, readdir, realpath, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { ManagementError, type LocalSkillRequest, type Operation } from '../dist/index.js';
import { startUap, type StartupConfiguration, type UapHost } from '../dist/node.js';

let directory: string;
let executable: string;

// Always build this checkout's production entrypoint. No fixture executable,
// installed release, opt-in environment variable or skipped scenario substitutes
// for the CLI + SDK boundary that npm test qualifies.
before(async () => {
  directory = await realpath(await mkdtemp(join(tmpdir(), 'uap-management-TEST-')));
  executable = join(directory, process.platform === 'win32' ? 'agentplugins.exe' : 'agentplugins');
  const cli = fileURLToPath(new URL('../../../cli/', import.meta.url));
  const build = spawnSync('go', ['build', '-trimpath', '-o', executable, './cmd/agentplugins'], {
    cwd: cli,
    env: { ...process.env, GOWORK: fileURLToPath(new URL('../../../go.work', import.meta.url)) },
    encoding: 'utf8',
    timeout: 180_000,
  });
  assert.equal(build.status, 0, build.stderr || build.error?.message);
}, { timeout: 190_000 });

after(async () => {
  if (directory) await rm(directory, { recursive: true, force: true });
});

function request() { return { signal: AbortSignal.timeout(10_000) }; }

async function absent(path: string) {
  await assert.rejects(stat(path), { code: 'ENOENT' });
}

function digest(bytes: Uint8Array) {
  return createHash('sha256').update(bytes).digest('hex');
}

function file(relativePath: string, bytes: Uint8Array, previous: Uint8Array | null = null) {
  return {
    relativePath,
    expectedSHA256: previous === null ? null : digest(previous),
    contentBase64: Buffer.from(bytes).toString('base64'),
  };
}

async function apply(host: UapHost, planId: string): Promise<Operation> {
  const requestId = randomUUID();
  const accepted = await host.client.apply(planId, requestId, request());
  assert.equal(accepted.planId, planId);
  assert.equal(accepted.requestId, requestId);
  const completed = await host.client.wait(accepted.operationId, {
    pollIntervalMs: 10,
    timeoutMs: 10_000,
  });
  assert.equal(completed.state, 'succeeded', JSON.stringify(completed.diagnostics));
  assert.equal(completed.result?.mutated, true);
  assert.equal(completed.action, 'local_skill');
  return completed;
}

async function isolatedEnvironment(root: string, stateRoot: string) {
  const home = join(root, 'home');
  const temporary = join(root, 'temporary');
  await Promise.all([mkdir(home), mkdir(temporary)]);
  const env: Record<string, string> = {
    HOME: home,
    USERPROFILE: home,
    APPDATA: join(home, 'appdata'),
    LOCALAPPDATA: join(home, 'localappdata'),
    XDG_CONFIG_HOME: join(home, 'config'),
    XDG_CACHE_HOME: join(home, 'cache'),
    XDG_DATA_HOME: join(home, 'data'),
    XDG_STATE_HOME: join(home, 'state'),
    AGENTPLUGINS_HOME: stateRoot,
    TMPDIR: temporary,
    TMP: temporary,
    TEMP: temporary,
    LANG: 'C.UTF-8',
    NO_COLOR: '1',
  };
  // Preserve only Windows OS locators, with one spelling per variable. Native
  // clients, credentials, proxies and developer policy overrides are not needed.
  for (const name of ['SystemRoot', 'WINDIR']) {
    const entry = Object.entries(process.env).find(([key]) => key.toUpperCase() === name.toUpperCase());
    if (entry?.[1]) env[name] = entry[1];
  }
  return env;
}

test('production CLI retains local Skill review, replay and exact restore across SDK host restart', { timeout: 60_000 }, async () => {
  const stateRoot = join(directory, 'state');
  const dependenciesRoot = join(directory, 'dependencies');
  const authoringRoot = join(directory, 'project', '.agents', 'skills');
  const skill = join(authoringRoot, 'draft');
  const config: StartupConfiguration = {
    scope: 'user', stateRoot, dependenciesRoot,
    targets: [{ id: 'captured-codex', client: 'codex', configRoot: join(directory, 'codex'), detectionStatus: 'not_detected' }],
    authoringRoots: [{ id: 'project-local', scope: 'project', path: authoringRoot }],
  };
  const env = await isolatedEnvironment(directory, stateRoot);
  const options = { executable, config, env, requiredCapabilities: ['local_skill'] };
  const original = Buffer.from('# Draft\n');
  const edited = Buffer.from('# Reviewed\n');
  const binary = Buffer.from([0, 255, 1, 128, 13, 10, 0]);
  const local = (change: Pick<LocalSkillRequest, 'action'> & Partial<LocalSkillRequest>): LocalSkillRequest => ({
    rootId: 'project-local', skillName: 'draft', ...change,
  });
  const unchangedIntent = async () => {
    await absent(join(dependenciesRoot, 'uap.toml'));
    await absent(join(dependenciesRoot, 'uap.lock'));
  };
  let host: UapHost | undefined;
  try {
    host = await startUap(options);
    const session = await host.client.session(request());
    assert.equal(session.contextId, host.session.contextId);
    assert.equal(session.environmentId, host.session.environmentId);
    assert.equal(session.authoringRoots?.[0]?.id, 'project-local');
    assert.equal(session.targets[0]?.available, false);
    await unchangedIntent();

    const create = await host.client.skills.local.prepare(local({
      action: 'upsert', files: [file('SKILL.md', original), file('assets/example.bin', binary)],
    }), request());
    assert.equal(create.confirmable, true);
    await absent(authoringRoot);
    const created = await apply(host, create.planId);
    assert.deepEqual(await readFile(join(skill, 'SKILL.md')), original);
    assert.deepEqual(await readFile(join(skill, 'assets', 'example.bin')), binary);

    const edit = await host.client.skills.local.prepare(local({
      action: 'upsert', files: [file('SKILL.md', edited, original)],
    }), request());
    assert.deepEqual(await readFile(join(skill, 'SKILL.md')), original, 'review must not edit files');
    const editResult = await apply(host, edit.planId);
    assert.equal(editResult.result?.localSkillBackup?.originalAction, 'upsert');
    assert.deepEqual(await readFile(join(skill, 'SKILL.md')), edited);
    assert.deepEqual(await readFile(join(skill, 'assets', 'example.bin')), binary);

    const remove = await host.client.skills.local.prepare(local({ action: 'delete' }), request());
    assert.deepEqual(await readFile(join(skill, 'SKILL.md')), edited, 'review must not delete files');
    const removed = await apply(host, remove.planId);
    const backup = removed.result?.localSkillBackup;
    assert.ok(backup);
    assert.equal(backup.originalAction, 'delete');
    assert.equal(backup.operationId, removed.operationId);
    assert.notEqual(backup.operationId, editResult.result?.localSkillBackup?.operationId);
    await absent(skill);
    assert.deepEqual(await readdir(authoringRoot), []);
    await unchangedIntent();

    const unaccepted = await host.client.skills.local.prepare(local({
      action: 'upsert', files: [file('SKILL.md', original)],
    }), request());
    await host.close();
    assert.equal(host.hasExited, true);
    host = await startUap(options);
    assert.equal(host.session.environmentId, session.environmentId);
    assert.deepEqual(await host.client.operation(removed.operationId, request()), removed);
    const history = await host.client.environment(request());
    assert.ok(history.localSkillBackups?.some((item) => item.operationId === backup.operationId && item.originalAction === 'delete'));
    assert.ok(history.localSkillBackups?.some((item) => item.operationId === editResult.operationId && item.originalAction === 'upsert'));

    // The original create is already accepted: replay must only observe it,
    // even though its review is gone and the Skill has subsequently been deleted.
    const replay = await host.client.apply(created.planId, created.requestId, request());
    assert.deepEqual(replay, created);
    await absent(skill);
    await assert.rejects(host.client.apply(removed.planId, created.requestId, request()),
      (error: unknown) => error instanceof ManagementError && error.code === 'REQUEST_ID_CONFLICT');
    await assert.rejects(host.client.apply(unaccepted.planId, randomUUID(), request()),
      (error: unknown) => error instanceof ManagementError && error.code === 'PLAN_STALE');
    await absent(skill);

    const restore = await host.client.skills.local.prepare(local({
      action: 'restore', restoreOperationId: backup.operationId,
    }), request());
    assert.equal(restore.restorePoint?.operationId, backup.operationId);
    assert.equal(restore.restorePoint?.originalAction, 'delete');
    await absent(skill);
    const restored = await apply(host, restore.planId);
    assert.deepEqual(await readFile(join(skill, 'SKILL.md')), edited);
    assert.deepEqual(await readFile(join(skill, 'assets', 'example.bin')), binary);
    const settled = await host.client.operation(removed.operationId, request());
    assert.equal(settled.resolvedBy, restored.operationId);
    assert.deepEqual(settled.result, removed.result, 'recovery links preserve the original outcome');
    assert.notEqual((await host.client.operation(editResult.operationId, request())).resolvedBy, restored.operationId);
    await unchangedIntent();
  } finally {
    await host?.close();
  }
});
