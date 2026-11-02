import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import { computed, ref } from 'vue';

const source = readFileSync(
  new URL('../composables/useDiscoveryStatus.ts', import.meta.url),
  'utf8',
);
const script =
  stripTypeScriptTypes(source).replace(/^export /gm, '') + '\n({ useDiscoveryIsStale });';

test('stale status never blocks a different fresh plugin and expiry changes invalidate guidance', () => {
  let now = Date.parse('2026-10-04T07:00:00Z');
  const status = ref({ state: 'stale', expiresAt: '2026-10-04T06:49:08Z' });
  const plugin = ref({ discovery: { expires_at: '2026-10-05T07:00:00Z' } });
  const helper = runInNewContext(script, {
    computed,
    useState: () => status,
    Date: { now: () => now, parse: Date.parse },
  });
  const stale = helper.useDiscoveryIsStale(() => plugin.value);
  assert.equal(stale.value, false);
  plugin.value = { discovery: { expires_at: '2026-10-04T06:49:08Z' } };
  assert.equal(stale.value, true);
  plugin.value = { discovery: { expires_at: '2026-10-05T07:00:00Z' } };
  status.value = { state: 'current', expiresAt: plugin.value.discovery.expires_at };
  assert.equal(stale.value, false);
  now = Date.parse(plugin.value.discovery.expires_at);
  status.value = { ...status.value, state: 'stale' };
  assert.equal(stale.value, true);
});
