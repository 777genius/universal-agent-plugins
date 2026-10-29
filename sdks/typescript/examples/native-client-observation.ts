// Native discovery only: no model, authentication, thread, or simulated client.
import assert from 'node:assert/strict';
import { execFile, spawn } from 'node:child_process';
import { realpath } from 'node:fs/promises';
import { isAbsolute, join, relative, resolve, sep } from 'node:path';
import { promisify } from 'node:util';

export type NativeClient = 'opencode' | 'claude' | 'codex';
export interface NativeObservation {
  skills: Map<string, string>;
  pluginId?: string;
}
interface Options {
  client: NativeClient;
  executable: string;
  root: string;
  nativeRoot: string;
  project: string;
  env: NodeJS.ProcessEnv;
  expectedPluginId?: string;
}
const pluginName = 'uap-commands-fixture';
const fixtureNames = ['fixture-guide', `${pluginName}-echo`, `${pluginName}-wizard`];
const limit = 4 * 1024 * 1024;

async function command(options: Options, args: string[]): Promise<string> {
  const { stdout } = await promisify(execFile)(options.executable, args, {
    cwd: options.project, env: options.env, timeout: 30_000, maxBuffer: limit,
  });
  return stdout;
}
async function contained(root: string, path: string): Promise<string> {
  assert.ok(isAbsolute(path), `Native path is not absolute: ${path}`);
  const [base, actual] = await Promise.all([realpath(root), realpath(path)]);
  const child = relative(base, actual);
  assert.ok(child !== '' && child !== '..' && !child.startsWith(`..${sep}`) && !isAbsolute(child), `Native path escapes disposable root: ${path}`);
  return actual;
}
function objects(value: unknown): Record<string, unknown>[] {
  if (Array.isArray(value)) return value.flatMap(objects);
  if (!value || typeof value !== 'object') return [];
  const node = value as Record<string, unknown>;
  return [node, ...Object.values(node).flatMap(objects)];
}

// The app-server protocol is newline-delimited JSON, with numeric request IDs.
// Keep a single bounded owned process and always observe its actual exit.
async function codexSkills(options: Options): Promise<unknown> {
  const child = spawn(options.executable, ['app-server'], { cwd: options.project, env: options.env, stdio: ['pipe', 'pipe', 'pipe'] });
  let stderr = '';
  let buffered = '';
  let bytes = 0;
  let rejectPending: ((error: Error) => void) | undefined;
  let pending: { id: number; resolve: (value: unknown) => void } | undefined;
  let failure: Error | undefined;
  let closed = false;
  let stopping = false;
  const recordFailure = (error: Error) => { failure ??= error; rejectPending?.(failure); };
  const fail = (error: Error) => { recordFailure(error); child.kill('SIGKILL'); };
  const exited = new Promise<void>((done) => {
    // A signal error does not prove exit. Only a spawn failure without a PID
    // can settle this promise before the actual close event.
    child.on('error', (error) => { recordFailure(error); if (child.pid === undefined) { closed = true; done(); } });
    child.once('close', () => {
      closed = true;
      if (!stopping) recordFailure(new Error(`Codex app-server exited unexpectedly: ${stderr}`));
      done();
    });
  });
  child.stdin.on('error', recordFailure);
  const timer = setTimeout(() => fail(new Error('Codex native observation timed out')), 30_000);
  child.stderr.on('data', (chunk: Buffer) => {
    bytes += chunk.length;
    if (bytes > limit) { fail(new Error('Codex native output exceeded limit')); return; }
    stderr += chunk.toString();
  });
  child.stdout.on('data', (chunk: Buffer) => {
    bytes += chunk.length;
    if (bytes > limit) { fail(new Error('Codex native output exceeded limit')); return; }
    buffered += chunk.toString();
    let newline: number;
    while ((newline = buffered.indexOf('\n')) >= 0) {
      const line = buffered.slice(0, newline); buffered = buffered.slice(newline + 1);
      if (!line.trim()) continue;
      try {
        const message = JSON.parse(line) as Record<string, unknown>;
        if (pending && message.id === pending.id) {
          if (message.error) { fail(new Error(`Codex RPC error: ${JSON.stringify(message.error)}`)); return; }
          const accept = pending.resolve; pending = undefined; rejectPending = undefined; accept(message.result);
        }
      } catch (error) { fail(error instanceof Error ? error : new Error(String(error))); return; }
    }
  });
  const request = (id: number, method: string, params: unknown) => new Promise<unknown>((accept, reject) => {
    if (failure) { reject(failure); return; }
    pending = { id, resolve: accept }; rejectPending = reject;
    child.stdin.write(JSON.stringify({ id, method, params }) + '\n', (error) => { if (error) fail(error); });
  });
  let result: unknown;
  try {
    await request(1, 'initialize', { clientInfo: { name: 'uap_sdk_native_fixture', version: '1.0.0' }, capabilities: { experimentalApi: true } });
    child.stdin.write(JSON.stringify({ method: 'initialized' }) + '\n');
    result = await request(2, 'skills/list', { cwds: [options.project], forceReload: true });
  } finally {
    stopping = true;
    child.stdin.end();
    if (!closed) child.kill('SIGKILL');
    let cleanupTimer: ReturnType<typeof setTimeout> | undefined;
    try {
      await Promise.race([
        exited,
        new Promise<never>((_, reject) => {
          cleanupTimer = setTimeout(() => reject(new Error(`Codex app-server cleanup could not observe actual exit (PID ${child.pid ?? 'unavailable'}): ${failure?.message ?? stderr}`)), 5_000);
        }),
      ]);
    } finally {
      clearTimeout(timer);
      if (cleanupTimer) clearTimeout(cleanupTimer);
    }
  }
  if (failure) throw failure;
  return result;
}

export async function inspectNative(options: Options): Promise<NativeObservation> {
  const skills = new Map<string, string>();
  if (options.client === 'opencode') {
    const listing: unknown = JSON.parse(await command(options, ['debug', 'skill']));
    assert.ok(Array.isArray(listing));
    for (const row of listing as { name: string; location: string }[]) {
      if (resolve(row.location) !== join(options.nativeRoot, 'skills', row.name, 'SKILL.md')) continue;
      assert.equal(skills.has(row.name), false, 'Duplicate owned native Skill');
      await contained(options.root, row.location);
      skills.set(row.name, row.location);
    }
    return { skills };
  }
  const listing: unknown = JSON.parse(await command(options, ['plugin', 'list', '--json']));
  if (options.client === 'claude') {
    assert.ok(Array.isArray(listing));
    const pluginId = options.expectedPluginId ?? `${pluginName}@skills-dir`;
    const entries = (listing as Record<string, unknown>[]).filter((row) => row.id === pluginId);
    assert.ok(entries.length <= 1, 'Duplicate Claude native plugin identity');
    const entry = entries[0];
    if (!entry) return { skills, pluginId };
    assert.equal(entry.enabled, true);
    assert.equal(typeof entry.installPath, 'string');
    const artifactRoot = entry.installPath as string;
    await contained(options.root, artifactRoot);
    const details = await command(options, ['plugin', 'details', pluginId]);
    for (const name of fixtureNames) {
      if (!details.includes(name)) continue;
      const path = join(artifactRoot, '.agentplugins-runtime', 'skills', name, 'SKILL.md');
      await contained(artifactRoot, path);
      skills.set(name, path);
    }
    assert.ok(details.includes(`Skills (${skills.size})`), `Unexpected Claude skill inventory: ${details}`);
    return { skills, pluginId };
  }
  // A disabled installation is still installed; it must never count as removal.
  const installed = objects(listing).filter((row) => row.installed === true && typeof row.pluginId === 'string' && row.pluginId.startsWith(`${pluginName}@`));
  for (const row of installed) assert.equal(row.enabled, true, 'Expected Codex plugin remains installed but disabled');
  const listedIds = [...new Set(installed.map((row) => row.pluginId as string))];
  assert.ok(listedIds.length <= 1, 'Multiple fixture plugin identities in Codex');
  const pluginId = options.expectedPluginId ?? listedIds[0];
  if (options.expectedPluginId && listedIds.length) assert.equal(listedIds[0], options.expectedPluginId);
  const result = await codexSkills(options);
  assert.ok(result && typeof result === 'object');
  const data = (result as { data?: unknown }).data;
  assert.ok(Array.isArray(data), 'Codex skills/list response has no data');
  for (const group of data as { skills?: unknown; errors?: unknown[] }[]) {
    assert.equal(group.errors?.length ?? 0, 0, 'Codex discovery errors');
    assert.ok(Array.isArray(group.skills));
    for (const row of group.skills as Record<string, unknown>[]) {
      if (!pluginId || row.pluginId !== pluginId) continue;
      assert.equal(row.enabled, true);
      assert.equal(typeof row.name, 'string');
      assert.equal(typeof row.path, 'string');
      const qualified = row.name as string;
      assert.ok(qualified.startsWith(`${pluginName}:`));
      const name = qualified.slice(pluginName.length + 1);
      assert.equal(skills.has(name), false, 'Duplicate Codex native skill');
      const path = row.path as string;
      const actual = await contained(options.root, path);
      assert.ok(actual.split(sep).join('/').includes('/plugins/cache/'), 'Codex skill is outside native plugin cache');
      skills.set(name, path);
    }
  }
  assert.equal(listedIds.includes(pluginId ?? ''), skills.size > 0, 'Codex plugin list disagrees with fresh skills/list');
  return { skills, ...(pluginId ? { pluginId } : {}) };
}

// Public pinned Codex CLI grammar: plugin_cmd.rs, rust-v0.153.4,
// RemovePluginArgs / run_plugin_remove. Attest only after native absence proof.
export async function actualuninstallCodex(options: Options & { expectedPluginId: string }): Promise<void> {
  assert.equal(options.client, 'codex');
  assert.ok(options.expectedPluginId.startsWith(`${pluginName}@`));
  const before = await inspectNative(options);
  assert.ok(before.skills.size > 0, 'Native Codex plugin must be present before uninstall');
  const removed = JSON.parse(await command(options, ['plugin', 'remove', options.expectedPluginId, '--json'])) as { pluginId?: string };
  assert.equal(removed.pluginId, options.expectedPluginId, 'Native remove returned another plugin identity');
  const after = await inspectNative(options);
  assert.equal(after.skills.size, 0, 'Fresh native Codex discovery still exposes removed plugin');
}
