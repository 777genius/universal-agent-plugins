// Run after npm run build against a real Go engine and immutable HTTPS fixture.
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';
import { spawn } from 'node:child_process';
import { mkdir, mkdtemp, readFile, writeFile, stat, rename } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve, relative, isAbsolute, sep } from 'node:path';
import { parseArgs } from 'node:util';
import { startUap, type StartupConfiguration, type UapHost } from '../dist/node.js';
import { ManagementError, type ChangeRequest } from '../dist/index.js';
const { values } = parseArgs({ options: { client: { type: 'string', default: 'pi' }, executable: { type: 'string' }, 'client-executable': { type: 'string' }, source: { type: 'string' }, output: { type: 'string' } } });
const pi = true;
const client = pi ? 'pi' : values.client!;
assert.ok(pi || ['kimi', 'kiro', 'windsurf'].includes(client));
assert.ok(values.executable && values.source && (client === 'windsurf' || values['client-executable']), '--executable, --source and --client-executable (except Windsurf) required');
const source = new URL(values.source!);
assert.equal(source.protocol, 'https:'); assert.match(source.hash, /^#sha256=[a-f0-9]{64}$/); assert.equal(source.username + source.password, '');
const executable = resolve(values.executable!);
const nativeExecutable = values['client-executable'] ? resolve(values['client-executable']) : undefined;
const root = await mkdtemp(join(tmpdir(), pi ? 'uap-pi-sdk-' : `uap-policy-${client}-`));
const home = join(root, 'home');
const nativeRoot = join(home, client === 'windsurf' ? '.codeium/windsurf' : client === 'pi' ? '.pi/agent' : `.${client}`);
const env: NodeJS.ProcessEnv = { HOME: home, USERPROFILE: home, XDG_CONFIG_HOME: join(root, 'config'), XDG_CACHE_HOME: join(root, 'cache'), XDG_DATA_HOME: join(root, 'data'), XDG_STATE_HOME: join(root, 'xdg-state'), TMPDIR: join(root, 'tmp'), NO_COLOR: '1', LANG: 'C.UTF-8', GIT_CONFIG_NOSYSTEM: '1', GIT_TERMINAL_PROMPT: '0' };
await Promise.all([nativeRoot, ...Object.values(env).filter((value): value is string => !!value && value.startsWith(root))].map(path => mkdir(path, { recursive: true })));
const inherited = new Set<string>();
for (const name of ['PATH', 'Path', 'SystemRoot', 'WINDIR', 'SSL_CERT_FILE', 'SSL_CERT_DIR', 'HTTPS_PROXY', 'HTTP_PROXY', 'NO_PROXY', 'https_proxy', 'http_proxy', 'no_proxy']) {
  if (process.env[name] !== undefined && !inherited.has(name.toUpperCase())) { env[name] = process.env[name]; inherited.add(name.toUpperCase()); }
}
const targetId = `${client}-test`;
const config: StartupConfiguration = { stateRoot: join(root, 'state'), dependenciesRoot: join(root, 'dependencies'), scope: 'user', targets: [{ id: targetId, client, configRoot: nativeRoot, ...(nativeExecutable ? { executable: nativeExecutable } : {}) }] };
const hash = (bytes: Uint8Array) => createHash('sha256').update(bytes).digest('hex');
const output = values.output ? resolve(values.output) : join(root, 'evidence.json');
const evidence = { qualification: pi ? 'sdk-native-lifecycle' : 'sdk-lifecycle-only', client, nativeLoaded: false, root, source: values.source, platform: process.platform, architecture: process.arch, engineSHA256: hash(await readFile(executable)), nativeSHA256: nativeExecutable ? hash(await readFile(nativeExecutable)) : null, nativeVersion: null as string | null, engineVersion: '', environmentId: '', status: 'running', stage: 'startup', error: '', operations: [] as { action: string; operationId: string; state: string }[], checks: [] as string[], nativeObservations: [] as unknown[], editedRefusal: undefined as undefined | { phase: string; code: string; status: number } };
async function checkpoint(stage: string, check?: string) { evidence.stage = stage; if (check) evidence.checks.push(check); await writeFile(output, JSON.stringify(evidence, null, 2) + '\n'); }
let host: UapHost | undefined;
function api() { assert.ok(host); return host.client; }
async function launch() { return startUap({ executable, config, env }); }
async function apply(change: ChangeRequest) {
  await checkpoint(`prepare ${change.action}`);
  const plan = await api().prepare(change); assert.ok(plan.confirmable, JSON.stringify(plan));
  const requestId = randomUUID(); const accepted = await api().apply(plan.planId, requestId);
  assert.equal((await api().apply(plan.planId, requestId)).operationId, accepted.operationId);
  const operation = await api().wait(accepted.operationId, { timeoutMs: 120_000, pollIntervalMs: 100 });
  evidence.operations.push({ action: change.action, operationId: operation.operationId, state: operation.state });
  assert.equal(operation.state, 'succeeded', JSON.stringify(operation)); assert.deepEqual(operation.diagnostics, []);
  await checkpoint(`${change.action} succeeded`); return { operation, planId: plan.planId, requestId };
}
// Every native subprocess has a byte cap, deadline and observed close after kill.
async function native(args: string[], input?: object) {
  assert.ok(nativeExecutable);
  return new Promise<string>((accept, reject) => {
    const child = spawn(nativeExecutable, args, { cwd: root, env, stdio: ['pipe', 'pipe', 'pipe'] });
    let stdout = '', stderr = '', size = 0, failure: Error | undefined;
    const stop = (error: Error) => { failure ??= error; child.kill('SIGKILL'); };
    const deadline = setTimeout(() => stop(new Error('Native process exceeded 30 seconds')), 30_000);
    let requestedStop = false;
    child.stdout.on('data', (chunk: Buffer) => {
      size += chunk.length; if (size > 1_048_576) return stop(new Error('Native output exceeded 1 MiB'));
      stdout += chunk.toString();
      if (input && stdout.split('\n').some(line => { try { return JSON.parse(line).id === 'sdk-witness'; } catch { return false; } })) { requestedStop = true; child.kill('SIGTERM'); }
    });
    child.stderr.on('data', (chunk: Buffer) => { size += chunk.length; if (size > 1_048_576) return stop(new Error('Native output exceeded 1 MiB')); stderr += chunk.toString(); });
    child.on('error', error => { failure = error; });
    child.on('close', code => { clearTimeout(deadline); if (failure) reject(failure); else if (!requestedStop && code !== 0) reject(new Error(`Native exited ${code}: ${stderr}`)); else accept(stdout); });
    if (input) child.stdin.write(JSON.stringify(input) + '\n'); else child.stdin.end();
  });
}
let artifact = '';
let descriptor: Record<string, unknown> | undefined;
const recordsPath = join(root, 'native-records.jsonl'); env.UAP_PI_SDK_RECORDS = recordsPath;
const fixture = await readFile(new URL('../../../repotests/testdata/agentplugins_pi_sdk_fixture/com.universal-agent-plugins.pi/index.ts', import.meta.url));
async function observe(present = true) {
  await writeFile(recordsPath, '');
  const text = await native(['--offline', '--no-approve', '--no-session', '--no-mcp', '--no-tools', '--mode', 'rpc'], present ? { id: 'sdk-witness', type: 'prompt', message: '/uap-sdk-witness' } : { id: 'sdk-witness', type: 'get_commands' });
  const response = text.split('\n').flatMap(line => { try { return [JSON.parse(line) as Record<string, any>]; } catch { return []; } });
  const answer = response.find(item => item.id === 'sdk-witness'); assert.ok(answer?.success, text);
  const records = (await readFile(recordsPath, 'utf8')).split('\n').filter(Boolean).map(line => JSON.parse(line) as Record<string, any>);
  if (present) {
    assert.equal(answer.data.disposition, 'handled');
    assert.deepEqual(records.map(item => item.type), ['top-level', 'factory', 'command']);
    const modulePath = join(artifact, 'com.universal-agent-plugins.pi', 'index.ts');
    for (const record of records) assert.equal(record.module, modulePath);
    assert.deepEqual(await readFile(modulePath), fixture);
    const current = records[1]!.descriptor as Record<string, unknown>;
    assert.equal(current.active_path, artifact); assert.equal(current.package_root, artifact);
    assert.equal(current.schema_version, 1); assert.equal(current.bootstrap_version, 1);
    const dataPath = relative(root, String(current.data_root)); assert.ok(dataPath && !isAbsolute(dataPath) && dataPath !== '..' && !dataPath.startsWith(`..${sep}`));
    const lock = JSON.parse(await readFile(join(root, 'dependencies', 'uap.lock'), 'utf8'));
    assert.equal(lock.packages.length, 1);
    assert.equal(current.canonical_tree_digest, lock.packages[0].tree_digest, 'Descriptor canonical digest must equal the SDK lock digest');
    if (descriptor) assert.deepEqual(current, descriptor); else descriptor = current;
    evidence.nativeLoaded = true;
  } else {
    assert.deepEqual(records, []); assert.ok(!answer.data.commands.some((command: { name: string }) => command.name === 'uap-sdk-witness'));
  }
  evidence.nativeObservations.push({ stage: evidence.stage, present, records, response });
}
try {
  evidence.nativeVersion = (await native(['--version'])).trim();
  host = await launch(); evidence.engineVersion = host.session.engineVersion; evidence.environmentId = host.session.environmentId;
  assert.deepEqual((await api().environment()).entries, []);
  const installed = await apply({ action: 'install', source: values.source, alias: 'sdk-fixture', targetIds: [targetId] });
  const environment = await api().environment(); assert.deepEqual(environment.diagnostics, []);
  const entry = environment.entries.find(item => item.alias === 'sdk-fixture'); assert.ok(entry?.declared);
  const paths = [...new Set(entry.bindings.flatMap(binding => binding.managedPaths))];
  const candidates = paths.filter(path => path.startsWith(join(nativeRoot, 'extensions') + sep)); assert.equal(candidates.length, 1); artifact = candidates[0]!;
  await observe();
  const lock = await readFile(join(root, 'dependencies', 'uap.lock'));
  await apply({ action: 'update', aliases: ['sdk-fixture'] }); assert.deepEqual(await readFile(join(root, 'dependencies', 'uap.lock')), lock); await observe();
  await checkpoint('pinned update verified', 'immutable-source update retains exact lock and native extension bytes');
  await rename(artifact, join(root, 'missing-package-preserved'));
  await apply({ action: 'repair', aliases: ['sdk-fixture'] }); await observe();
  await checkpoint('missing package repaired', 'explicit repair reconstructs the missing package and real Pi loads its exact factory');
  const editedPath = join(artifact, 'user-edit.txt'); const edited = 'Disposable user edit; must be preserved.\n'; await writeFile(editedPath, edited);
  await assert.rejects(api().prepare({ action: 'repair', aliases: ['sdk-fixture'] }), error => {
    assert.ok(error instanceof ManagementError); assert.equal(error.code, 'OPERATION_REFUSED'); assert.equal(error.status, 422);
    evidence.editedRefusal = { phase: 'prepare', code: error.code, status: error.status }; return true;
  });
  assert.equal(await readFile(editedPath, 'utf8'), edited);
  await checkpoint('edited package refusal', 'Pi repair refuses modified package and preserves exact edited bytes');
  const preserved = join(root, 'manually-preserved-edits'); await rename(artifact, preserved);
  await apply({ action: 'repair', aliases: ['sdk-fixture'] }); await observe(); assert.equal(await readFile(join(preserved, 'user-edit.txt'), 'utf8'), edited);
  await host.close(); host = await launch(); assert.equal(host.session.environmentId, evidence.environmentId);
  const replay = await api().apply(installed.planId, installed.requestId); assert.equal(replay.operationId, installed.operation.operationId); assert.equal(replay.state, 'succeeded');
  assert.equal((await api().environment()).entries.length, 1);
  await checkpoint('restart history verified', 'restart observes original accepted request identity without duplicate installation');
  await apply({ action: 'remove', aliases: ['sdk-fixture'], targetIds: [targetId] }); assert.deepEqual((await api().environment()).entries, []); await assert.rejects(stat(artifact), { code: 'ENOENT' }); await observe(false);
  evidence.status = 'passed'; await checkpoint('complete', 'fresh real Pi process no longer loads removed extension or exposes its command');
  process.stdout.write(JSON.stringify(evidence, null, 2) + '\n');
} catch (error) { evidence.status = 'failed'; evidence.error = error instanceof Error ? error.message : String(error); await checkpoint(evidence.stage); throw error; }
finally { await host?.close(); }
