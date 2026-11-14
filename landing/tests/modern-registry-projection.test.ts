import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { parseDirectoryData, directoryIsExpired } from '../utils/registry.ts';
import { parseModernSnapshot } from '../utils/registry-codecs/signed.ts';
import { modernReleaseTargets } from '../utils/registry-codecs/evidence.ts';
import { clientIds } from '../data/clientIds.generated.ts';

const fixture = () => JSON.parse(readFileSync(
  new URL('../../cmd/agentplugins-registry-mirror/testdata/modern-snapshot.json', import.meta.url),
  'utf8',
));

test('modern projection keeps known denials and all emitted client IDs are generated', () => {
  const index = parseModernSnapshot(fixture());
  const plugin = index.plugins[0]!;
  const distribution = plugin.distributions[0]!;
  const release = distribution.releases[0]!;
  assert.deepEqual(plugin.client_support.clients, ['codex']);
  assert.deepEqual(distribution.compatible_clients, ['codex']);
  assert.deepEqual(release.blocking_clients, ['cursor']);
  assert.deepEqual(release.materialized_clients, ['codex']);
  assert.ok(release.targets.every((target) => clientIds.includes(target.client)));
  assert.ok(release.evidence.every((item) => clientIds.includes(item.client)));
  assert.ok(!JSON.stringify(index).includes('future-test-agent'));
  assert.equal(directoryIsExpired(index, Date.parse('2026-12-15T00:00:00Z')), true);
  assert.throws(() => parseDirectoryData(fixture(), 'published_snapshot'), /invalid or duplicate client/);
  assert.throws(() => parseDirectoryData(fixture(), 'review_preview'), /invalid or duplicate client/);
  assert.throws(() => parseModernSnapshot({ ...fixture(), snapshot_schema_version: 2 }), /schema 1/);
});

test('modern unknown target fields and duplicates are validated before omission', () => {
  const cases: [string, (target: Record<string, unknown>, targets: unknown[]) => void][] = [
    ['unsafe identity', (t) => { t.client = '../future'; }],
    ['empty identity', (t) => { t.client = ''; }],
    ['noncanonical identity', (t) => { t.client = 'Future-Agent'; }],
    ['duplicate future', (t, all) => { all.push(t); }],
    ['delivery', (t) => { t.delivery = 'new-protocol'; }],
    ['authentication', (t) => { t.authentication = 'automatic'; }],
    ['Project scope', (t) => { t.scopes = ['project']; }],
    ['multiple scopes', (t) => { t.scopes = ['user', 'project']; }],
    ['missing scope', (t) => { delete t.scopes; }],
    ['future binding', (t) => { t.app_binding = { app_key: 'tool', id: 'app_tool', mcp_server: 'tool' }; }],
    ['null binding', (t) => { t.app_binding = null; }],
    ['unknown field', (t) => { t.permissions = 'all'; }],
    ['scoped grant field', (t) => { t.scope = 'user'; }],
  ];
  for (const [name, edit] of cases) {
    const targets = fixture().distributions[0].release_policies[0].targets;
    edit(targets[2], targets);
    assert.throws(() => modernReleaseTargets(targets, name), Error, name);
  }
});

test('future positive evidence cannot restore revoked or unsupported known releases', () => {
  for (const mutation of ['revoked', 'suspended', 'missing components', 'missing known positive', 'future only']) {
    const snapshot = fixture();
    const distribution = snapshot.distributions[0];
    const policy = distribution.release_policies[0];
    if (mutation === 'revoked') policy.status = 'revoked';
    if (mutation === 'suspended') distribution.status = 'suspended';
    if (mutation === 'missing components') distribution.releases[0].components = [];
    if (mutation === 'missing known positive' || mutation === 'future only') {
      snapshot.evidence = snapshot.evidence.filter((item: { id: string }) => item.id !== 'codex-pass');
      policy.current_evidence = policy.current_evidence.filter((id: string) => id !== 'codex-pass');
    }
    if (mutation === 'future only') {
      policy.targets = [policy.targets[2]];
      snapshot.evidence = snapshot.evidence.filter((item: { id: string }) => item.id !== 'cursor-fail');
      policy.current_evidence = policy.current_evidence.filter((id: string) => id !== 'cursor-fail');
    }
    const plugin = parseModernSnapshot(snapshot).plugins[0]!;
    assert.equal(plugin.installable, false, mutation);
    assert.deepEqual(plugin.client_support.clients, [], mutation);
    assert.deepEqual(plugin.distributions[0]!.compatible_clients, [], mutation);
  }
});
