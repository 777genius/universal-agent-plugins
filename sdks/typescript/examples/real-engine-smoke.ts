// Run after `npm run build`; uses a real pinned engine and public pinned fixture.
// No native installation is simulated. Only one accepted HTTP response is dropped.
import assert from 'node:assert/strict';
import { randomUUID, createHash } from 'node:crypto';
import { mkdir, mkdtemp, readFile, realpath, rename, rm, stat, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { isAbsolute, join, relative, resolve, sep } from 'node:path';
import { parseArgs } from 'node:util';
import { startUap, type StartupConfiguration, type UapHost } from '../dist/node.js';
import type { ChangeRequest, LocalSkillRequest, Operation } from '../dist/index.js';
import { actualuninstallCodex, inspectNative, type NativeClient, type NativeObservation } from './native-client-observation.ts';
import { actualUninstallPackageNative } from './package-client-observation.ts';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';

const { values } = parseArgs({ options: { client: { type: 'string', default: 'opencode' }, executable: { type: 'string' }, 'client-executable': { type: 'string' }, source: { type: 'string' }, output: { type: 'string' } } });
if (!values.executable || !values['client-executable'] || !values.source) throw new Error('--executable, --client-executable and --source are required; use a real pinned client and the immutable public commands fixture.');
assert.ok(['opencode', 'claude', 'codex', 'cline', 'gemini', 'copilot', 'grok', 'qoder'].includes(values.client));
const nativeClient = values.client as NativeClient;
const targetId = `${nativeClient}-test`;
const archiveSource = /^https:\/\/[^#]+#sha256=[a-f0-9]{64}$/.test(values.source);
if (!archiveSource) assert.match(values.source, /^[\w-]+\/[\w.-]+@[a-f0-9]{40}\/\/repotests\/testdata\/agentplugins_commands_fixture$/);
const executable = resolve(values.executable);
const nativeExecutable = resolve(values['client-executable']);
const root = await mkdtemp(join(tmpdir(), 'uap-real-sdk-'));
const outputPath = values.output ? resolve(values.output) : join(root, 'evidence.json');
const separateSkills = ['opencode', 'cline', 'gemini'].includes(nativeClient);
const nativeRoot = ['cline', 'gemini', 'copilot', 'grok', 'qoder'].includes(nativeClient) ? join(root, 'home', `.${nativeClient}`) : join(root, 'config', nativeClient);
const skillsRoot = join(nativeRoot, 'skills');
const project = join(root, 'project');
const config: StartupConfiguration = {
  stateRoot: join(root, 'state'), dependenciesRoot: join(root, 'dependencies'), scope: 'user',
  targets: [{ id: targetId, client: nativeClient, configRoot: nativeRoot, executable: nativeExecutable }],
  authoringRoots: [{ id: 'local-skills', path: skillsRoot, scope: 'user', targetId }],
};
const isolated = { HOME: join(root, 'home'), USERPROFILE: join(root, 'home'), APPDATA: join(root, 'home', 'AppData', 'Roaming'), LOCALAPPDATA: join(root, 'home', 'AppData', 'Local'), XDG_CONFIG_HOME: join(root, 'config'), XDG_DATA_HOME: join(root, 'data'), XDG_CACHE_HOME: join(root, 'cache'), XDG_STATE_HOME: join(root, 'xdg-state'), TMPDIR: join(root, 'tmp'), TEMP: join(root, 'tmp'), TMP: join(root, 'tmp') };
await Promise.all([...Object.values(isolated), project, nativeRoot].map((directory) => mkdir(directory, { recursive: true })));
const smokeEnvironment: NodeJS.ProcessEnv = { ...isolated, GIT_CONFIG_NOSYSTEM: '1', GIT_TERMINAL_PROMPT: '0', NO_COLOR: '1' };
if (nativeClient === 'opencode') Object.assign(smokeEnvironment, { OPENCODE_PURE: '1', OPENCODE_DISABLE_MODELS_FETCH: '1', OPENCODE_DISABLE_AUTOUPDATE: '1' });
if (nativeClient === 'claude') Object.assign(smokeEnvironment, { CLAUDE_CONFIG_DIR: nativeRoot, DISABLE_AUTOUPDATER: '1', CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: '1' });
if (nativeClient === 'cline') smokeEnvironment.CLINE_NO_AUTO_UPDATE = '1';
if (nativeClient === 'gemini') smokeEnvironment.GEMINI_CLI_HOME = isolated.HOME;
if (nativeClient === 'qoder') smokeEnvironment.QODER_CONFIG_DIR = nativeRoot;
if (nativeClient === 'codex') {
  smokeEnvironment.CODEX_HOME = nativeRoot;
  await assert.rejects(stat('/etc/codex'), { code: 'ENOENT' }, 'Codex native qualification requires an isolated host without /etc/codex configuration');
  await writeFile(join(nativeRoot, 'config.toml'), 'cli_auth_credentials_store = "file"\nmcp_oauth_credentials_store = "file"\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n');
}
const inherited = new Set<string>();
for (const name of ['PATH', 'Path', 'SystemRoot', 'WINDIR', 'LANG', 'LC_ALL', 'HTTP_PROXY', 'HTTPS_PROXY', 'NO_PROXY', 'http_proxy', 'https_proxy', 'no_proxy', 'SSL_CERT_FILE', 'SSL_CERT_DIR']) {
  if (process.env[name] !== undefined && !inherited.has(name.toUpperCase())) { smokeEnvironment[name] = process.env[name]; inherited.add(name.toUpperCase()); }
}
const sha256 = (bytes: Uint8Array) => createHash('sha256').update(bytes).digest('hex');
const evidence = {
  qualification: 'sdk-native-lifecycle', nativeVersion: '', sourceKind: archiveSource ? 'pinned-https-archive' : 'pinned-git',
  root, client: nativeClient, platform: process.platform, architecture: process.arch, source: values.source, engineVersion: '', environmentId: '',
  engineSHA256: sha256(await readFile(executable)), nativeSHA256: sha256(await readFile(nativeExecutable)),
  status: 'running', stage: 'startup', error: '', checks: [] as string[],
  failureEnvironment: undefined as unknown,
  operations: [] as { action: string; operationId: string; state: string }[],
  nativeObservations: [] as { stage: string; pluginId?: string; skills: { name: string; path: string; sha256: string }[] }[],
  lostResponse: undefined as undefined | { planId: string; requestId: string; observedState?: string; operationId?: string },
};
async function checkpoint(stage: string, check?: string) {
  evidence.stage = stage;
  if (check) evidence.checks.push(check);
  await writeFile(outputPath, JSON.stringify(evidence, null, 2) + '\n');
}
let host: UapHost | undefined;
let pluginId: string | undefined;
let managedArtifact: string | undefined;
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
  await checkpoint(`prepare ${local ? 'local_' : ''}${change.action}`);
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
function observationOptions() {
  return { client: nativeClient, executable: nativeExecutable, root, nativeRoot, project, env: smokeEnvironment, ...(managedArtifact ? { managedArtifact } : {}), ...(pluginId ? { expectedPluginId: pluginId } : {}) };
}
async function nativeObservation(): Promise<NativeObservation> {
  const observed = await inspectNative(observationOptions());
  pluginId ??= observed.pluginId;
  const skills = await Promise.all([...observed.skills].map(async ([name, path]) => ({ name, path, sha256: sha256(await readFile(path)) })));
  evidence.nativeObservations.push({ stage: evidence.stage, ...(observed.pluginId ? { pluginId: observed.pluginId } : {}), skills });
  return observed;
}
async function verifyNativeGuide() {
  const { skills: found } = await nativeObservation();
  const guide = found.get('fixture-guide');
  assert.ok(guide, `Real ${nativeClient} did not load the captured fixture Skill`);
  assert.ok(found.has('uap-commands-fixture-echo'), `Real ${nativeClient} did not load the portable command Skill`);
  assert.equal(found.has('uap-commands-fixture-wizard'), false, 'Explicit-only command leaked into native automatic Skills');
  const fixture = await readFile(new URL('../../../repotests/testdata/agentplugins_commands_fixture/skills/fixture-guide/SKILL.md', import.meta.url));
  assert.deepEqual(await readFile(guide), fixture, 'Native consumer location must contain the pinned fixture bytes');
  const command = await readFile(new URL('../../../repotests/testdata/agentplugins_commands_fixture/io.github.777genius.agentplugins/commands/echo.md', import.meta.url), 'utf8');
  const header = /^---\n([\s\S]*?)\n---\n/.exec(command);
  assert.ok(header);
  const description = /^description: (.+)$/m.exec(header[1]!);
  assert.ok(description);
  const echo = `---\nname: "uap-commands-fixture-echo"\ndescription: ${JSON.stringify(description[1])}\n---\n${command.slice(header[0].length)}`;
  assert.equal(await readFile(found.get('uap-commands-fixture-echo')!, 'utf8'), echo, 'Native portable command Skill must contain the exact pinned rendered bytes');
}
async function ownedArtifact(paths: string[]): Promise<string> {
  const prefix = (nativeClient === 'claude' ? skillsRoot : nativeClient === 'grok' ? join(nativeRoot, 'plugins') : join(root, 'state', 'managed', 'clients', nativeClient)) + sep;
  const candidates = [...new Set(paths.filter((path) => resolve(path).startsWith(prefix)))];
  assert.equal(candidates.length, 1, `Expected one owned managed plugin artifact: ${JSON.stringify(paths)}`);
  const artifact = await realpath(candidates[0]!);
  const inside = relative(root, artifact);
  assert.ok(inside !== '' && inside !== '..' && !inside.startsWith(`..${sep}`) && !isAbsolute(inside));
  assert.equal((await stat(artifact)).isDirectory(), true);
  return artifact;
}
async function preserveEditedNativeSkill() {
  assert.ok(separateSkills);
  const guideDirectory = join(skillsRoot, 'fixture-guide');
  const guidePath = join(guideDirectory, 'SKILL.md');
  const original = await readFile(guidePath);
  const edited = Buffer.concat([original, Buffer.from('\nUnreviewed local fixture edit.\n')]);
  await writeFile(guidePath, edited);
  const refused = await apply({ action: 'repair', aliases: ['sdk-fixture'] }, false, 'partial');
  assert.deepEqual(await readFile(guidePath), edited, 'Refused repair must preserve the edited native bytes');
  if (nativeClient === 'opencode') assert.ok(refused.operation.diagnostics.some((diagnostic) => diagnostic.code === 'OWNED_SKILL_CHANGED'));
  assert.ok(refused.operation.result?.nextActions.some((action) => action.kind === (nativeClient === 'opencode' ? 'preserve_changes' : 'inspect')));
  assert.ok((await client().environment()).pendingOperations?.some((operation) => operation.operationId === refused.operation.operationId));
  assert.equal((await client().operation(refused.operation.operationId)).resolvedBy, undefined);
  // Explicit manual remediation belongs to this disposable test, not automatic
  // SDK or installer behavior. Preserve the conflicting tree outside discovery.
  const preservedRoot = join(root, 'manually-preserved-native-edits');
  await mkdir(preservedRoot);
  const preserved = join(preservedRoot, 'owned-artifact');
  await rename(guideDirectory, preserved);
  await checkpoint('manual preservation of conflicting native edits', 'edited native projection is refused as partial, keeps pending history and preserves its exact bytes');
  const recovered = await apply({ action: 'recover', operationId: refused.operation.operationId });
  await verifyNativeGuide();
  assert.deepEqual(await readFile(join(preserved, relative(guideDirectory, guidePath))), edited);
  assert.equal((await client().operation(refused.operation.operationId)).resolvedBy, recovered.operation.operationId);
  assert.equal((await client().environment()).pendingOperations?.length ?? 0, 0);
  await checkpoint('fresh recovery after explicit manual preservation', 'after manual preservation, a fresh review restores exact native fixture bytes and settles only proved recovery');
}
async function repairModifiedArtifact() {
  assert.ok(managedArtifact);
  // Claude/Codex managed packages support reviewed replacement of a changed
  // artifact. OpenCode's separately owned Skill preservation policy is different.
  const driftPath = join(managedArtifact, 'sdk-fixture-drift.txt');
  await writeFile(driftPath, 'Disposable managed package drift.\n');
  await apply({ action: 'repair', aliases: ['sdk-fixture'] });
  await assert.rejects(stat(driftPath), { code: 'ENOENT' });
  await verifyNativeGuide();
  assert.equal((await client().environment()).pendingOperations?.length ?? 0, 0);
  await checkpoint('modified package repair verified', 'reviewed repair replaces managed package drift and native discovery retains exact pinned fixture bytes');
}
async function repairAbsentArtifact() {
  assert.ok(managedArtifact);
  const guide = nativeClient === 'claude'
    ? join(managedArtifact, '.agentplugins-runtime', 'skills', 'fixture-guide', 'SKILL.md')
    : join(managedArtifact, 'skills', 'fixture-guide', 'SKILL.md');
  const original = await readFile(guide);
  await rm(managedArtifact, { recursive: true });
  await assert.rejects(stat(managedArtifact), { code: 'ENOENT' });
  await apply({ action: 'repair', aliases: ['sdk-fixture'] });
  assert.deepEqual(await readFile(guide), original, 'Explicit repair must reconstruct the missing managed package');
  await verifyNativeGuide();
  await checkpoint('absent package repair verified', 'explicit reviewed repair reconstructs the missing managed package and exact native fixture bytes');
}
async function interruptedRepair() {
  // OpenCode's separate native Skill can be reconstructed during convergence.
  // Missing Claude/Codex managed packages require explicit Repair (tested above),
  // rather than the Sync used by generic recovery after durable acceptance.
  if (separateSkills) await rm(join(skillsRoot, 'fixture-guide'), { recursive: true });
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
  await checkpoint('crash recovery verified', 'lost accepted response plus owned-process SIGKILL preserves retry identity; fresh recovery verifies exact real native consumer bytes without duplicate dependency');
}
try {
  await checkpoint('startup');
  const versionOutput = (await promisify(execFile)(nativeExecutable, nativeClient === 'qoder' ? ['--config-dir', nativeRoot, '--version'] : ['--version'], { cwd: project, env: smokeEnvironment, timeout: 30_000, maxBuffer: 4 * 1024 * 1024 })).stdout;
  const versions = [...versionOutput.matchAll(/\b\d+\.\d+\.\d+\b/g)].map((match) => match[0]);
  assert.equal(new Set(versions).size, 1, `Ambiguous native version: ${versionOutput}`);
  evidence.nativeVersion = versions[0]!;
  const requiredVersions: Partial<Record<NativeClient, string>> = { cline: '3.0.73', gemini: '0.65.0', copilot: '1.0.103', grok: '1.0.67', qoder: '1.1.78' };
  const requiredVersion = requiredVersions[nativeClient];
  if (requiredVersion) assert.equal(evidence.nativeVersion, requiredVersion);
  host = await launch();
  evidence.engineVersion = host.session.engineVersion;
  evidence.environmentId = host.session.environmentId;
  const initial = await client().environment();
  assert.deepEqual(initial.entries, []);
  assert.equal(initial.pendingOperations?.length ?? 0, 0);
  const installed = await apply({ action: 'install', source: values.source, alias: 'sdk-fixture', targetIds: [targetId] });
  const environment = await client().environment();
  const entry = environment.entries.find((item) => item.alias === 'sdk-fixture');
  assert.ok(entry, JSON.stringify(environment));
  assert.equal(entry.declared, true);
  const ownedPaths = entry.bindings.flatMap((binding) => binding.managedPaths);
  assert.ok(ownedPaths.length > 0);
  for (const filename of ownedPaths) await stat(filename);
  if (!separateSkills) managedArtifact = await ownedArtifact(ownedPaths);
  await verifyNativeGuide();
  await checkpoint('native consumer verified', `real ${nativeClient} native discovery loads exact pinned Skill bytes and portable command; explicit-only command remains withheld`);
  const manifestPath = join(root, 'dependencies', 'uap.toml');
  const lockPath = join(root, 'dependencies', 'uap.lock');
  const [manifestBefore, lockBefore] = await Promise.all([readFile(manifestPath), readFile(lockPath)]);
  assert.ok(manifestBefore.length > 0 && lockBefore.length > 0);
  await apply({ action: 'update', aliases: ['sdk-fixture'] });
  assert.deepEqual(await readFile(lockPath), lockBefore);
  await checkpoint('pinned update verified', 'targeted immutable-source update preserves the lock');
  await apply({ action: 'repair', aliases: ['sdk-fixture'] });
  if (separateSkills) await preserveEditedNativeSkill();
  else {
    await repairModifiedArtifact();
    await repairAbsentArtifact();
  }
  await interruptedRepair();
  await host.close();
  host = await launch();
  const recovered = await client().apply(installed.planId, installed.requestId);
  assert.equal(recovered.operationId, installed.operation.operationId);
  assert.equal(recovered.state, 'succeeded');
  assert.equal(host.session.environmentId, evidence.environmentId);
  await checkpoint('retained history verified', 'accepted successful operation survives engine restart without replay');
  const removal: ChangeRequest = { action: 'remove', aliases: ['sdk-fixture'], targetIds: [targetId] };
  {
    const review = await client().prepare(removal);
    if (!review.confirmable) {
      assert.ok(review.conflicts.some((conflict) => conflict.code === 'EXTERNAL_UNINSTALL_REQUIRED'));
      await client().plans.discard(review.planId);
      assert.ok(pluginId);
      if (nativeClient === 'codex') await actualuninstallCodex({ ...observationOptions(), expectedPluginId: pluginId });
      else await actualUninstallPackageNative({ ...observationOptions(), expectedPluginId: pluginId });
      removal.externalUninstalled = true;
      await checkpoint('native external uninstall verified', 'real native plugin uninstall settles the exact native identity before an explicit fresh SDK removal attestation');
    } else await client().plans.discard(review.planId);
  }
  await apply(removal);
  assert.equal((await client().environment()).entries.length, 0);
  const { skills: removed } = await nativeObservation();
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
  if (host && !host.hasExited) {
    evidence.failureEnvironment = await client().environment({ signal: AbortSignal.timeout(5_000) })
      .catch((failure: unknown) => ({ unavailable: failure instanceof Error ? failure.message : 'Environment unavailable' }));
  }
  await checkpoint(evidence.stage);
  throw error;
} finally {
  await host?.close();
}
