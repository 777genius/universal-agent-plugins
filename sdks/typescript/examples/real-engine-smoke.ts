// Run after `npm run build`; uses a real pinned engine and a public pinned source.
// This is installation evidence, unlike the protocol-only owned-child fixture.
import assert from 'node:assert/strict';
import { randomUUID, createHash } from 'node:crypto';
import { mkdtemp, readFile, stat, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { parseArgs } from 'node:util';
import { startUap, type StartupConfiguration } from '../dist/node.js';
import type { ChangeRequest, LocalSkillRequest, Operation } from '../dist/index.js';

const { values } = parseArgs({ options: { executable: { type: 'string' }, source: { type: 'string' }, output: { type: 'string' } } });
if (!values.executable || !values.source) throw new Error('--executable and --source are required; use an immutable public standard fixture source.');
const root = await mkdtemp(join(tmpdir(), 'uap-real-sdk-'));
const config: StartupConfiguration = {
  stateRoot: join(root, 'state'), dependenciesRoot: join(root, 'dependencies'), scope: 'user',
  targets: [{ id: 'opencode-test', client: 'opencode', configRoot: join(root, 'opencode') }],
  authoringRoots: [{ id: 'local-skills', path: join(root, 'opencode', 'skills'), scope: 'user', targetId: 'opencode-test' }],
};
const smokeEnvironment = { ...process.env };
// The test runner can provide duplicated proxy casing; native child policy is
// intentionally case-insensitive. Preserve the explicitly supplied uppercase form.
for (const key of ['http_proxy', 'https_proxy', 'no_proxy']) if (smokeEnvironment[key.toUpperCase()] !== undefined) delete smokeEnvironment[key];
let host = await startUap({ executable: resolve(values.executable), config, env: smokeEnvironment, startupTimeoutMs: 30_000 });
const evidence: { root: string; engineVersion: string; environmentId: string; source: string; checks: string[]; operations: { action: string; operationId: string; state: string }[] } = {
  root, engineVersion: host.session.engineVersion, environmentId: host.session.environmentId, source: values.source, checks: [], operations: [],
};
async function apply(change: ChangeRequest | LocalSkillRequest, local = false): Promise<{ operation: Operation; planId: string; requestId: string }> {
  const plan = local ? await host.client.skills.local.prepare(change as LocalSkillRequest) : await host.client.prepare(change as ChangeRequest);
  assert.ok(plan.confirmable, JSON.stringify(plan.conflicts));
  const requestId = randomUUID();
  const accepted = await host.client.apply(plan.planId, requestId);
  const replay = await host.client.apply(plan.planId, requestId);
  assert.equal(replay.operationId, accepted.operationId);
  const operation = await host.client.wait(accepted.operationId, { timeoutMs: 120_000, pollIntervalMs: 100 });
  assert.equal(operation.state, 'succeeded', JSON.stringify(operation));
  evidence.operations.push({ action: local ? `local_${change.action}` : change.action, operationId: operation.operationId, state: operation.state });
  return { operation, planId: plan.planId, requestId };
}
try {
  const initial = await host.client.environment();
  assert.deepEqual(initial.entries, []);
  assert.equal(initial.pendingOperations?.length ?? 0, 0);
  const installed = await apply({ action: 'install', source: values.source, alias: 'sdk-fixture', targetIds: ['opencode-test'] });
  const environment = await host.client.environment();
  const entry = environment.entries.find((item) => item.alias === 'sdk-fixture');
  assert.ok(entry, JSON.stringify(environment));
  const ownedPaths = entry.bindings.flatMap((binding) => binding.managedPaths);
  assert.ok(ownedPaths.length > 0);
  for (const filename of ownedPaths) await stat(filename);
  evidence.checks.push('real native projections exist', 'duplicate apply returns the accepted operation');
  const manifestPath = join(root, 'dependencies', 'uap.toml');
  const lockPath = join(root, 'dependencies', 'uap.lock');
  const [manifestBefore, lockBefore] = await Promise.all([readFile(manifestPath), readFile(lockPath)]);
  assert.ok(manifestBefore.length > 0 && lockBefore.length > 0);
  await apply({ action: 'update', aliases: ['sdk-fixture'] });
  assert.deepEqual(await readFile(lockPath), lockBefore);
  evidence.checks.push('explicit commit remains pinned through update');
  await apply({ action: 'repair', aliases: ['sdk-fixture'] });
  await host.close();
  host = await startUap({ executable: resolve(values.executable), config, env: smokeEnvironment });
  const recovered = await host.client.apply(installed.planId, installed.requestId);
  assert.equal(recovered.operationId, installed.operation.operationId);
  assert.equal(recovered.state, 'succeeded');
  assert.equal(host.session.environmentId, evidence.environmentId);
  evidence.checks.push('accepted operation survives engine restart without replay');
  await apply({ action: 'remove', aliases: ['sdk-fixture'], targetIds: ['opencode-test'] });
  assert.equal((await host.client.environment()).entries.length, 0);
  evidence.checks.push('managed removal clears the desired environment');
  const markdown = '# SDK local Skill\n\nReviewed local bytes.\n';
  await apply({ action: 'upsert', rootId: 'local-skills', skillName: 'sdk-local', files: [{ relativePath: 'SKILL.md', expectedSHA256: null, contentBase64: Buffer.from(markdown).toString('base64') }] }, true);
  const localPath = join(root, 'opencode', 'skills', 'sdk-local', 'SKILL.md');
  assert.equal(await readFile(localPath, 'utf8'), markdown);
  const revised = markdown + '\nUpdated once.\n';
  await apply({ action: 'upsert', rootId: 'local-skills', skillName: 'sdk-local', files: [{ relativePath: 'SKILL.md', expectedSHA256: createHash('sha256').update(markdown).digest('hex'), contentBase64: Buffer.from(revised).toString('base64') }] }, true);
  const deleted = await apply({ action: 'delete', rootId: 'local-skills', skillName: 'sdk-local' }, true);
  await assert.rejects(stat(localPath), { code: 'ENOENT' });
  assert.ok(deleted.operation.result?.localSkillBackup);
  await apply({ action: 'restore', rootId: 'local-skills', skillName: 'sdk-local', restoreOperationId: deleted.operation.operationId }, true);
  assert.equal(await readFile(localPath, 'utf8'), revised);
  evidence.checks.push('local create/edit/delete and exact backup restoration use the same Go lifecycle');
  assert.equal((await host.client.environment()).pendingOperations?.length ?? 0, 0);
  const output = JSON.stringify(evidence, null, 2) + '\n';
  if (values.output) await writeFile(resolve(values.output), output);
  process.stdout.write(output);
} finally {
  await host.close();
}
