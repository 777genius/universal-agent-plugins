import assert from 'node:assert/strict';
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { loadModernRegistryIndex, loadRegistryIndex } from '../../build/load-registry.ts';
import { clientIds } from '../../data/clientIds.generated.ts';
import { eligibleTargetPolicies } from '../../utils/registry.ts';

// Invoked by the Go mirror integration after actual Ed25519 verification and
// normal mirror staging. No fixture parser replaces the production build loader.
const stage = process.argv[2];
assert.ok(stage);
const markerPath = join(stage, 'MIRROR_METADATA.json');
const snapshotPath = join(stage, 'registry/modern/schemas/1/snapshots/00000000000000000001.json');
const original = readFileSync(snapshotPath);
const index = loadModernRegistryIndex(snapshotPath, markerPath, 'mirror-modern-test-only');
const plugin = index.plugins[0];
const distribution = plugin.distributions[0];
const release = distribution.releases[0];
assert.deepEqual(plugin.client_support.clients, ['codex']);
assert.deepEqual(distribution.compatible_clients, ['codex']);
assert.deepEqual(release.targets.map((target) => target.client), ['codex', 'cursor']);
assert.deepEqual(release.blocking_clients, ['cursor']);
assert.deepEqual(release.materialized_clients, ['codex']);
assert.deepEqual(eligibleTargetPolicies(distribution.kind, release).map((target) => target.client), ['codex']);
assert.ok(release.evidence.every((item) => clientIds.includes(item.client)));
assert.ok(!JSON.stringify(index).includes('future-test-agent'));
assert.ok(readFileSync(snapshotPath).equals(original), 'projection must not rewrite signed bytes');
assert.throws(() => loadRegistryIndex(snapshotPath, 'published_snapshot'), /invalid or duplicate client/);
assert.throws(() => loadRegistryIndex(snapshotPath, 'review_preview'), /invalid or duplicate client/);
assert.throws(() => loadModernRegistryIndex(snapshotPath, markerPath, 'other-modern'), /lineage or bytes/);
writeFileSync(snapshotPath, Buffer.concat([original, Buffer.from('\n')]));
assert.throws(() => loadModernRegistryIndex(snapshotPath, markerPath, 'mirror-modern-test-only'), /lineage or bytes/);
writeFileSync(snapshotPath, original);
const marker = JSON.parse(readFileSync(markerPath, 'utf8'));
delete marker.directory_feed_id;
writeFileSync(markerPath, JSON.stringify(marker));
assert.throws(() => loadModernRegistryIndex(snapshotPath, markerPath, 'mirror-modern-test-only'), /lineage or bytes/);
console.log('PASS: authenticated modern mirror → actual build loader; known selections and denials preserved');
