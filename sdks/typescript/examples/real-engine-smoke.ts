// Run after `npm run build`; uses a real pinned engine and public pinned fixture.
// No native installation is simulated. Only one accepted HTTP response is dropped.
import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { randomUUID, createHash } from 'node:crypto';
import { mkdir, mkdtemp, readFile, rename, rm, stat, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { parseArgs, promisify } from 'node:util';
import { startUap, type StartupConfiguration, type UapHost } from '../dist/node.js';
import type { ChangeRequest, LocalSkillRequest, Operation } from '../dist/index.js';

const { values } = parseArgs({ options: { executable: { type: 'string' }, 'client-executable': { type: 'string' }, source: { type: 'string' }, output: { type: 'string' } } });
if (!values.executable || !values['client-executable'] || !values.source) throw new Error('--executable, --client-executable and --source are required; use a real native OpenCode executable and the immutable public commands fixture.');
assert.match(values.source, /^[\w-]+\/[\w.-]+@[a-f0-9]{40}\/\/repotests\/testdata\/agentplugins_commands_fixture$/);
const executable = resolve(values.executable);
const nativeExecutable = resolve(values['client-executable']);
const root = await mkdtemp(join(tmpdir(), 'uap-real-sdk-'));
const outputPath = values.output ? resolve(values.output) : join(root, 'evidence.json');
const nativeRoot = join(root, 'config', 'opencode');
const skillsRoot = join(nativeRoot, 'skills');
const config: StartupConfiguration = {
  stateRoot: join(root, 'state'), dependenciesRoot: join(root, 'dependencies'), scope: 'user',
  targets: [{ id: 'opencode-test', client: 'opencode', configRoot: nativeRoot, executable: nativeExecutable }],
  authoringRoots: [{ id: 'local-skills', path: skillsRoot, scope: 'user', targetId: 'opencode-test' }],
};
const isolated = { HOME: join(root, 'home'), USERPROFILE: join(root, 'home'), APPDATA: join(root, 'home', 'AppData', 'Roaming'), LOCALAPPDATA: join(root, 'home', 'AppData', 'Local'), XDG_CONFIG_HOME: join(root, 'config'), XDG_DATA_HOME: join(root, 'data'), XDG_CACHE_HOME: join(root, 'cache'), XDG_STATE_HOME: join(root, 'xdg-state'), TMPDIR: join(root, 'tmp'), TEMP: join(root, 'tmp'), TMP: join(root, 'tmp') };
await Promise.all([...Object.values(isolated), join(root, 'project')].map((directory) => mkdir(directory, { recursive: true })));
const smokeEnvironment: NodeJS.ProcessEnv = { ...isolated, OPENCODE_PURE: '1', OPENCODE_DISABLE_MODELS_FETCH: '1', OPENCODE_DISABLE_AUTOUPDATE: '1', NO_COLOR: '1' };
const inherited = new Set<string>();
for (const name of ['PATH', 'Path', 'SystemRoot', 'WINDIR', 'LANG', 'LC_ALL', 'HTTP_PROXY', 'HTTPS_PROXY', 'NO_PROXY', 'http_proxy', 'https_proxy', 'no_proxy', 'SSL_CERT_FILE', 'SSL_CERT_DIR']) {
  if (process.env[name] !== undefined && !inherited.has(name.toUpperCase())) { smokeEnvironment[name] = process.env[name]; inherited.add(name.toUpperCase()); }
}
const sha256 = (bytes: Uint8Array) => createHash('sha256').update(bytes).digest('hex');
const evidence = {
  root, source: values.source, engineVersion: '', environmentId: '',
  engineSHA256: sha256(await readFile(executable)), nativeSHA256: sha256(await readFile(nativeExecutable)),
  status: 'running', stage: 'startup', error: '', checks: [] as string[],
  operations: [] as { action: string; operationId: string; state: string }[],
  lostResponse: undefined as undefined | { planId: string; requestId: string; observedState?: string; operationId?: string },
};
async function checkpoint(stage: string, check?: string) {
  evidence.stage = stage;
  if (check) evidence.checks.push(check);
  await writeFile(outputPath, JSON.stringify(evidence, null, 2) + '\n');
}
let host: UapHost | undefined;
let dropNextApply = false;
const networkFetch = globalThis.fetch;
const lossFetch: typeof globalThis.fetch = async (input, init) => {
  const response = await networkFetch(input, init);
  const request = input instanceof Request ? input : new Request(input, init);
  if (dropNextApply && request.method === 'POST' && /\/api\/v1\/plans\/[^/]+\/apply$/.test(new URL(request.url).pathname) && response.status === 202) {
    dropNextApply = false;
    // The server really accepted the request. Discard its response and kill only
    // this example's owned engine before exposing an operation to the caller.
    assert.ok(host);
    process.kill(host.pid, 'SIGKILL');
    await response.body?.cancel().catch(() => undefined);
    throw new Error('Deliberately lost the accepted apply response');
  }
  return response;
};
async function launch(): Promise<UapHost> {
  // openapi-fetch captures fetch when constructing the client. This isolated
  // executable example injects transport loss without a production test hook.
  const previous = globalThis.fetch;
  globalThis.fetch = lossFetch;
  try { return await startUap({ executable, config, env: smokeEnvironment }); }
  finally { globalThis.fetch = previous; }
}
function client() { assert.ok(host); return host.client; }
async function apply(change: ChangeRequest | LocalSkillRequest, local = false, expectedState: Operation['state'] = 'succeeded'): Promise<{ operation: Operation; planId: string; requestId: string }> {
  const api = client();
  const plan = local ? await api.skills.local.prepare(change as LocalSkillRequest) : await api.prepare(change as ChangeRequest);
  assert.ok(plan.confirmable, JSON.stringify(plan.conflicts));
  const requestId = randomUUID();
  const accepted = await api.apply(plan.planId, requestId);
  const replay = await api.apply(plan.planId, requestId);
  assert.equal(replay.operationId, accepted.operationId);
  const operation = await api.wait(accepted.operationId, { timeoutMs: 120_000, pollIntervalMs: 100 });
  evidence.operations.push({ action: local ? `local_${change.action}` : change.action, operationId: operation.operationId, state: operation.state });
  await checkpoint(`${local ? 'local_' : ''}${change.action} ${operation.state}`);
  assert.equal(operation.state, expectedState, JSON.stringify(operation));
  return { operation, planId: plan.planId, requestId };
}
async function nativeSkills(): Promise<Map<string, string>> {
  const { stdout } = await promisify(execFile)(nativeExecutable, ['debug', 'skill'], { cwd: join(root, 'project'), env: smokeEnvironment, timeout: 30_000, maxBuffer: 4 * 1024 * 1024 });
  const listing: { name: string; location: string }[] = JSON.parse(stdout);
  assert.ok(Array.isArray(listing));
  const found = new Map<string, string>();
  for (const item of listing) {
    if (resolve(item.location) !== join(skillsRoot, item.name, 'SKILL.md')) continue;
    assert.equal(found.has(item.name), false, 'Native consumer listed a duplicate owned Skill');
    found.set(item.name, item.location);
  }
  return found;
}
async function verifyNativeGuide() {
  const found = await nativeSkills();
  const guide = found.get('fixture-guide');
  assert.ok(guide, 'Real OpenCode did not load the captured fixture Skill');
  assert.ok(found.has('uap-commands-fixture-echo'), 'Real OpenCode did not load the portable command Skill');
  assert.equal(found.has('uap-commands-fixture-wizard'), false, 'Explicit-only command leaked into native automatic Skills');
  const fixture = await readFile(new URL('../../../repotests/testdata/agentplugins_commands_fixture/skills/fixture-guide/SKILL.md', import.meta.url));
  assert.deepEqual(await readFile(guide), fixture, 'Native consumer location must contain the pinned fixture bytes');
}
async function preserveEditedNativeSkill() {
  const guideDirectory = join(skillsRoot, 'fixture-guide');
  const guidePath = join(guideDirectory, 'SKILL.md');
  const original = await readFile(guidePath);
  const edited = Buffer.concat([original, Buffer.from('\nUnreviewed local fixture edit.\n')]);
  await writeFile(guidePath, edited);
  const refused = await apply({ action: 'repair', aliases: ['sdk-fixture'] }, false, 'partial');
  assert.deepEqual(await readFile(guidePath), edited, 'Refused repair must preserve the edited native bytes');
  assert.ok(refused.operation.diagnostics.some((diagnostic) => diagnostic.code === 'OWNED_SKILL_CHANGED'));
  assert.ok(refused.operation.result?.nextActions.some((action) => action.kind === 'preserve_changes'));
  assert.ok((await client().environment()).pendingOperations?.some((operation) => operation.operationId === refused.operation.operationId));
  assert.equal((await client().operation(refused.operation.operationId)).resolvedBy, undefined);
  // Explicit manual remediation belongs to this disposable test, not automatic
  // SDK or installer behavior. Preserve the conflicting tree outside discovery.
  const preservedRoot = join(root, 'manually-preserved-native-edits');
  await mkdir(preservedRoot);
  const preserved = join(preservedRoot, 'fixture-guide');
  await rename(guideDirectory, preserved);
  await checkpoint('manual preservation of conflicting native edits', 'edited native projection is refused as partial, keeps pending history and preserves its exact bytes');
  const recovered = await apply({ action: 'recover', operationId: refused.operation.operationId });
  await verifyNativeGuide();
  assert.deepEqual(await readFile(join(preserved, 'SKILL.md')), edited);
  assert.equal((await client().operation(refused.operation.operationId)).resolvedBy, recovered.operation.operationId);
  assert.equal((await client().environment()).pendingOperations?.length ?? 0, 0);
  await checkpoint('fresh recovery after explicit manual preservation', 'after manual preservation, a fresh review restores exact native fixture bytes and settles only proved recovery');
}
async function interruptedRepair() {
  // A wholly absent owned directory can be safely reconstructed. An existing
  // edited directory is a different policy case, qualified separately above.
  await rm(join(skillsRoot, 'fixture-guide'), { recursive: true });
  const plan = await client().prepare({ action: 'repair', aliases: ['sdk-fixture'] });
  assert.ok(plan.confirmable);
  const requestId = randomUUID();
  evidence.lostResponse = { planId: plan.planId, requestId };
  await checkpoint('persisted retry identity before deliberately lost response');
  dropNextApply = true;
  await assert.rejects(client().apply(plan.planId, requestId), /Deliberately lost/);
  assert.equal(dropNextApply, false);
  await host!.close();
  host = await launch();
  const replay = await client().apply(plan.planId, requestId);
  assert.equal((await client().apply(plan.planId, requestId)).operationId, replay.operationId);
  evidence.lostResponse.observedState = replay.state;
  evidence.lostResponse.operationId = replay.operationId;
  assert.ok(['interrupted', 'succeeded'].includes(replay.state), JSON.stringify(replay));
  let pending = (await client().environment()).pendingOperations ?? [];
  if (replay.state === 'interrupted') assert.ok(pending.some((item) => item.operationId === replay.operationId));
  // Native journal reconciliation may deliberately require a second fresh
  // desired-state review. Each pass is independently prepared and observed.
  for (let attempt = 0; pending.length > 0 && attempt < 3; attempt++) {
    await apply({ action: 'recover', operationId: replay.operationId });
    pending = (await client().environment()).pendingOperations ?? [];
  }
  assert.equal(pending.length, 0, 'Recovery left accepted work unresolved');
  await verifyNativeGuide();
  const entries = (await client().environment()).entries;
  assert.equal(entries.filter((entry) => entry.alias === 'sdk-fixture').length, 1);
  await checkpoint('crash recovery verified', 'lost accepted response plus owned-process SIGKILL preserves retry identity; fresh recovery restores real native consumer bytes without duplicate dependency');
}
try {
  await checkpoint('startup');
  host = await launch();
  evidence.engineVersion = host.session.engineVersion;
  evidence.environmentId = host.session.environmentId;
  const initial = await client().environment();
  assert.deepEqual(initial.entries, []);
  assert.equal(initial.pendingOperations?.length ?? 0, 0);
  const installed = await apply({ action: 'install', source: values.source, alias: 'sdk-fixture', targetIds: ['opencode-test'] });
  const environment = await client().environment();
  const entry = environment.entries.find((item) => item.alias === 'sdk-fixture');
  assert.ok(entry, JSON.stringify(environment));
  assert.equal(entry.declared, true);
  const ownedPaths = entry.bindings.flatMap((binding) => binding.managedPaths);
  assert.ok(ownedPaths.length > 0);
  for (const filename of ownedPaths) await stat(filename);
  await verifyNativeGuide();
  await checkpoint('native consumer verified', 'real OpenCode debug skill loads exact pinned Skill bytes and portable command; explicit-only command remains withheld');
  const manifestPath = join(root, 'dependencies', 'uap.toml');
  const lockPath = join(root, 'dependencies', 'uap.lock');
  const [manifestBefore, lockBefore] = await Promise.all([readFile(manifestPath), readFile(lockPath)]);
  assert.ok(manifestBefore.length > 0 && lockBefore.length > 0);
  await apply({ action: 'update', aliases: ['sdk-fixture'] });
  assert.deepEqual(await readFile(lockPath), lockBefore);
  await checkpoint('pinned update verified', 'targeted exact-commit update preserves the lock');
  await apply({ action: 'repair', aliases: ['sdk-fixture'] });
  await preserveEditedNativeSkill();
  await interruptedRepair();
  await host.close();
  host = await launch();
  const recovered = await client().apply(installed.planId, installed.requestId);
  assert.equal(recovered.operationId, installed.operation.operationId);
  assert.equal(recovered.state, 'succeeded');
  assert.equal(host.session.environmentId, evidence.environmentId);
  await checkpoint('retained history verified', 'accepted successful operation survives engine restart without replay');
  await apply({ action: 'remove', aliases: ['sdk-fixture'], targetIds: ['opencode-test'] });
  assert.equal((await client().environment()).entries.length, 0);
  const removed = await nativeSkills();
  assert.equal(removed.has('fixture-guide'), false);
  assert.equal(removed.has('uap-commands-fixture-echo'), false);
  await checkpoint('removal verified', 'managed removal clears desired environment and real native Skill discovery');
  const markdown = '# SDK local Skill\n\nReviewed local bytes.\n';
  await apply({ action: 'upsert', rootId: 'local-skills', skillName: 'sdk-local', files: [{ relativePath: 'SKILL.md', expectedSHA256: null, contentBase64: Buffer.from(markdown).toString('base64') }] }, true);
  const localPath = join(skillsRoot, 'sdk-local', 'SKILL.md');
  assert.equal(await readFile(localPath, 'utf8'), markdown);
  const revised = markdown + '\nUpdated once.\n';
  await apply({ action: 'upsert', rootId: 'local-skills', skillName: 'sdk-local', files: [{ relativePath: 'SKILL.md', expectedSHA256: sha256(Buffer.from(markdown)), contentBase64: Buffer.from(revised).toString('base64') }] }, true);
  const deleted = await apply({ action: 'delete', rootId: 'local-skills', skillName: 'sdk-local' }, true);
  await assert.rejects(stat(localPath), { code: 'ENOENT' });
  assert.ok(deleted.operation.result?.localSkillBackup);
  await host.close();
  host = await launch();
  assert.ok((await client().environment()).localSkillBackups?.some((backup) => backup.operationId === deleted.operation.operationId), 'Undo must survive process restart');
  await apply({ action: 'restore', rootId: 'local-skills', skillName: 'sdk-local', restoreOperationId: deleted.operation.operationId }, true);
  assert.equal(await readFile(localPath, 'utf8'), revised);
  assert.equal((await client().environment()).pendingOperations?.length ?? 0, 0);
  evidence.status = 'passed';
  await checkpoint('complete', 'local create/edit/delete and restart-visible exact backup restoration use the same real Go lifecycle');
  process.stdout.write(JSON.stringify(evidence, null, 2) + '\n');
} catch (error) {
  evidence.status = 'failed';
  evidence.error = error instanceof Error ? error.message : 'Unknown qualification failure';
  await checkpoint(evidence.stage);
  throw error;
} finally {
  await host?.close();
}
