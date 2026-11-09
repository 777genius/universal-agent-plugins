import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { join, resolve, sep } from 'node:path';
import { tmpdir } from 'node:os';
import { parseDirectoryData } from '../utils/registry.ts';
import { catalogVisiblePlugins, groupCatalogPlugins } from '../utils/filter.ts';

// Site grouping and classification are the authoritative *public display*
// projection. Input is the signed Directory mirror verified by the Go mirror
// job, never an arbitrary unsigned JSON downloaded by this test.
const shard = Number(process.env.UAP_REVIEWED_SHARD);
const shards = 4;
assert.ok(Number.isInteger(shard) && shard >= 0 && shard < shards);
const reportDir = process.env.UAP_REVIEWED_REPORT_DIR;
assert.ok(reportDir, 'Report dir required');
mkdirSync(reportDir, { recursive: true });
const snapshotRoot = resolve('landing/public/registry/schemas/1');
const pointer = JSON.parse(readFileSync(join(snapshotRoot, 'latest.json'), 'utf8'));
assert.equal(typeof pointer.snapshot_path, 'string');
const snapshotPath = resolve(snapshotRoot, pointer.snapshot_path);
assert.ok(snapshotPath.startsWith(snapshotRoot + sep), 'No snapshot path traversal');
const original = readFileSync(snapshotPath);
const index = parseDirectoryData(JSON.parse(original.toString('utf8')), 'published_snapshot');
const reviewed = groupCatalogPlugins(catalogVisiblePlugins(index.plugins))
  .map((group) => group.primary)
  .filter((plugin) => plugin.trust_state !== 'conformant_unreviewed')
  .sort((a, b) => a.name.localeCompare(b.name, 'en'));
assert.ok(reviewed.length > 0, 'Verified website reviewed catalog must not be empty');
const selected = reviewed.filter((_, i) => i % shards === shard);
assert.ok(selected.length, 'Reviewed catalog smaller than number of shards');
const parent = mkdtempSync(join(tmpdir(), 'uap-reviewed-install-'));
const errors: string[] = [];
const outcomes: Array<Record<string, unknown>> = [];
const native = ['claude', 'codex', 'opencode'] as const;
function execute(args: string[], env: NodeJS.ProcessEnv, cwd: string) {
  const child = spawnSync('agentplugins', args, {
    encoding: 'utf8', timeout: 140_000, maxBuffer: 2 * 1024 * 1024,
    cwd, env,
  });
  return { exit_code: child.status, timeout: child.error?.name === 'ETIMEDOUT',
    error: child.error?.message ?? null,
    stdout_tail: (child.stdout || '').slice(-1400), stderr_tail: (child.stderr || '').slice(-1400) };
}
const version = spawnSync('agentplugins', ['version'], { encoding: 'utf8', timeout: 15_000 });
const report = {
  schema_version: 1, run_sha: process.env.GITHUB_SHA ?? '', cli_version: (version.stdout || '').trim(),
  reviewed_total: reviewed.length, selected_total: selected.length, shard, shards,
  signed_snapshot_sha256: createHash('sha256').update(original).digest('hex'),
  scope: 'all live signed reviewed website cards; installed where compatible with a genuine native client',
  actual_model_calls: false, native_activation_qualified_by_this_job: false, outcomes,
};
try {
  assert.equal(version.status, 0, 'Published CLI must run');
  for (const [i, plugin] of selected.entries()) {
    const source = plugin.install_source;
    const selectedClient = native.find((client) =>
      plugin.client_support.clients.includes(client) &&
      plugin.client_support.scopes[client]?.includes('user'));
    const row: Record<string, unknown> = {
      name: plugin.name, source, reviewed: plugin.trust_state, installable: plugin.installable,
      selectedClient: selectedClient ?? null,
      source_revision: plugin.source.revision,
      distribution: plugin.default_distribution,
    };
    outcomes.push(row);
    if (!plugin.installable || !selectedClient) {
      row.status = plugin.installable ? 'genuine-native-client-unavailable' : 'not-installable-on-site';
      continue; // An explicit unqualified case, NEVER a native PASS.
    }
    if (typeof source !== 'string' || !source || source.length > 1024) {
      row.status = 'invalid-signed-source';
      errors.push(plugin.name + ': missing valid installation source');
      continue;
    }
    const home = join(parent, String(i), 'home');
    const project = join(parent, String(i), 'project');
    const config = join(home, 'config');
    mkdirSync(project, { recursive: true });
    mkdirSync(config, { recursive: true });
    mkdirSync(join(home, '.codex'), { recursive: true });
    const sentinel = join(home, 'not-owned-by-uap.txt');
    writeFileSync(sentinel, 'must be preserved\n');
    const env: NodeJS.ProcessEnv = {
      ...process.env, HOME: home, USERPROFILE: home, CODEX_HOME: join(home, '.codex'),
      XDG_CONFIG_HOME: config, XDG_DATA_HOME: join(home, 'data'), XDG_CACHE_HOME: join(home, 'cache'),
      GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: join(config, 'gitconfig'),
      GIT_TERMINAL_PROMPT: '0', GCM_INTERACTIVE: 'never', CI: 'true', DO_NOT_TRACK: '1',
    };
    for (const key of ['GITHUB_TOKEN', 'GH_TOKEN', 'OPENAI_API_KEY', 'ANTHROPIC_API_KEY',
      'NPM_TOKEN', 'GIT_ASKPASS']) delete env[key];
    const addArgs = ['add', source, '--target', selectedClient, '--scope', 'user', '--format', 'json'];
    row.install = execute(addArgs, env, project);
    if ((row.install as { exit_code: number | null }).exit_code !== 0) {
      row.status = 'install-not-qualified';
      errors.push(plugin.name + ': real installation failed; inspect receipt');
      continue;
    }
    row.list = execute(['list', '--target', selectedClient, '--format', 'json'], env, project);
    row.repeat = execute(addArgs, env, project);
    row.remove = execute(['remove', plugin.name, '--target', selectedClient,
      '--scope', 'user', '--format', 'json'], env, project);
    row.sentinel_preserved = existsSync(sentinel) &&
      readFileSync(sentinel, 'utf8') === 'must be preserved\n';
    if ((row.list as { exit_code: number | null }).exit_code !== 0 ||
        (row.repeat as { exit_code: number | null }).exit_code !== 0 ||
        (row.remove as { exit_code: number | null }).exit_code !== 0 ||
        !row.sentinel_preserved) {
      row.status = 'lifecycle-not-qualified';
      errors.push(plugin.name + ': real list/repeat/remove/ownership failed');
    } else {
      // Qualified installer lifecycle only. The existing 30-source matrix
      // separately verifies identity-attributed native registration and
      // native clean removal for its immutable coverage set.
      row.status = 'installer-lifecycle-passed-native-unqualified';
    }
  }
  assert.equal(outcomes.length, selected.length, 'Do not silently omit a reviewed card');
  assert.deepEqual(errors, [], 'Website reviewed source failures: inspect original per-card receipts');
} finally {
  writeFileSync(join(reportDir, 'reviewed-' + shard + '.json'),
    JSON.stringify({ ...report, failures: errors }, null, 2) + '\n');
  rmSync(parent, { recursive: true, force: true });
}
