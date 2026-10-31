// Run after npm run build against a real Go engine and immutable HTTPS fixture.
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';
import { spawn } from 'node:child_process';
import { mkdir, mkdtemp, readFile, writeFile, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { parseArgs } from 'node:util';
import { startUap, type StartupConfiguration, type UapHost } from '../dist/node.js';
import { type ChangeRequest } from '../dist/index.js';
const { values } = parseArgs({ options: { client: { type: 'string', default: 'kimi' }, executable: { type: 'string' }, 'client-executable': { type: 'string' }, source: { type: 'string' }, output: { type: 'string' } } });
const client = values.client!;
assert.ok(['kimi', 'kiro', 'windsurf'].includes(client));
assert.ok(values.executable && values.source && (client === 'windsurf' || values['client-executable']), '--executable, --source and --client-executable (except Windsurf) required');
const source = new URL(values.source!);
assert.equal(source.protocol, 'https:'); assert.match(source.hash, /^#sha256=[a-f0-9]{64}$/); assert.equal(source.username + source.password, '');
const executable = resolve(values.executable!);
const nativeExecutable = values['client-executable'] ? resolve(values['client-executable']) : undefined;
const root = await mkdtemp(join(tmpdir(), `uap-policy-${client}-`));
const home = join(root, 'home');
const nativeRoot = join(home, client === 'windsurf' ? '.codeium/windsurf' : `.${client}`);
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
const evidence = { qualification: 'sdk-lifecycle-only', client, nativeLoaded: false, root, source: values.source, platform: process.platform, architecture: process.arch, engineSHA256: hash(await readFile(executable)), nativeSHA256: nativeExecutable ? hash(await readFile(nativeExecutable)) : null, nativeVersion: null as string | null, engineVersion: '', environmentId: '', status: 'running', stage: 'startup', error: '', operations: [] as { action: string; operationId: string; state: string }[], checks: [] as string[] };
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
async function native(args: string[]) {
  assert.ok(nativeExecutable);
  return new Promise<string>((accept, reject) => {
    const child = spawn(nativeExecutable, args, { cwd: root, env, stdio: ['pipe', 'pipe', 'pipe'] });
    let stdout = '', stderr = '', size = 0, failure: Error | undefined;
    const stop = (error: Error) => { failure ??= error; child.kill('SIGKILL'); };
    const deadline = setTimeout(() => stop(new Error('Native process exceeded 30 seconds')), 30_000);
    child.stdout.on('data', (chunk: Buffer) => {
      size += chunk.length; if (size > 1_048_576) return stop(new Error('Native output exceeded 1 MiB'));
      stdout += chunk.toString();
    });
    child.stderr.on('data', (chunk: Buffer) => { size += chunk.length; if (size > 1_048_576) return stop(new Error('Native output exceeded 1 MiB')); stderr += chunk.toString(); });
    child.on('error', error => { failure = error; });
    child.on('close', code => { clearTimeout(deadline); if (failure) reject(failure); else if (code !== 0) reject(new Error(`Native exited ${code}: ${stderr}`)); else accept(stdout); });
    child.stdin.end();
  });
}
try {
  if (nativeExecutable) evidence.nativeVersion = (await native(['--version'])).trim();
  host = await launch(); evidence.engineVersion = host.session.engineVersion; evidence.environmentId = host.session.environmentId;
  assert.deepEqual((await api().environment()).entries, []);
  const installed = await apply({ action: 'install', source: values.source, alias: 'sdk-fixture', targetIds: [targetId] });
  const environment = await api().environment(); assert.deepEqual(environment.diagnostics, []);
  const entry = environment.entries.find(item => item.alias === 'sdk-fixture'); assert.ok(entry?.declared);
  assert.equal(entry.bindings.length, 1); const binding = entry.bindings[0]!; assert.equal(binding.targetId, targetId); assert.equal(binding.client, client); assert.equal(binding.scope, 'user'); assert.equal(binding.status, 'installed');
  const paths = binding.managedPaths; assert.ok(paths.length); for (const path of paths) await stat(path);
  function guidance(operation: typeof installed.operation, repair = false) {
    const actions = operation.result!.nextActions;
    if (client === 'kimi') assert.ok(actions.some(action => /reload|new/i.test(action.reason)));
    else if (client === 'windsurf' && !repair) assert.ok(actions.some(action => /refresh|restart|reload/i.test(action.reason)));
    else assert.deepEqual(actions, []);
  }
  guidance(installed.operation);
  const lock = await readFile(join(root, 'dependencies', 'uap.lock'));
  await apply({ action: 'update', aliases: ['sdk-fixture'] }); assert.deepEqual(await readFile(join(root, 'dependencies', 'uap.lock')), lock);
  const repaired = await apply({ action: 'repair', aliases: ['sdk-fixture'] }); guidance(repaired.operation, true);
  await host.close(); host = await launch(); assert.equal(host.session.environmentId, evidence.environmentId);
  const replay = await api().apply(installed.planId, installed.requestId); assert.equal(replay.operationId, installed.operation.operationId); assert.equal(replay.state, 'succeeded');
  await apply({ action: 'remove', aliases: ['sdk-fixture'], targetIds: [targetId] });
  assert.deepEqual((await api().environment()).entries, []);
  for (const path of paths) {
    if (client === 'windsurf' && path.endsWith('mcp_config.json')) { const config = JSON.parse(await readFile(path, 'utf8')); assert.deepEqual(config.mcpServers ?? {}, {}); }
    else await assert.rejects(stat(path), { code: 'ENOENT' });
  }
  evidence.status = 'passed'; await checkpoint('complete', 'idempotent apply; owned profile; pinned lock; intact repair; restart history; removal');
  process.stdout.write(JSON.stringify(evidence, null, 2) + '\n');
} catch (error) { evidence.status = 'failed'; evidence.error = error instanceof Error ? error.message : String(error); await checkpoint(evidence.stage); throw error; }
finally { await host?.close(); }
