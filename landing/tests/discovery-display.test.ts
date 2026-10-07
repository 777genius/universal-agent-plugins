import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  loadDiscovery,
  loadDiscoveryForDisplay,
  verifyDiscovery,
  type CachedDiscovery,
  type DiscoveryCache,
} from '../utils/discovery.ts';
import { canonicalJSON } from '../utils/signedFeed.ts';
import {
  discoveryFetcher,
  discoveryTrust,
  signedDiscoveryFixture,
} from './fixtures/signed-discovery.ts';

const origin = new URL('https://catalog.example/discovery/');
const now = new Date('2026-10-04T07:00:00Z');
const options = { origin, trust: discoveryTrust, now };
function memoryCache(initial?: CachedDiscovery): DiscoveryCache & { value?: CachedDiscovery } {
  return {
    value: initial,
    async load() {
      return this.value;
    },
    async store(value) {
      this.value = value;
    },
  };
}

test('first visit displays all 3779 authenticated expired listings while authority stays strict', async () => {
  const data = signedDiscoveryFixture({ count: 3779 });
  const cache = memoryCache();
  const bundle = await loadDiscoveryForDisplay({
    ...options,
    cache,
    fetcher: discoveryFetcher(data),
  });
  assert.equal(bundle.snapshot.sequence, 80);
  assert.equal(bundle.search.records.length, 3779);
  assert.equal(bundle.freshness, 'stale');
  assert.equal(bundle.source, 'remote');
  assert.deepEqual(cache.value?.bytes, data.bytes);
  await assert.rejects(verifyDiscovery(data.bytes, discoveryTrust, {}, now), /stale/);
  await assert.rejects(loadDiscovery({ ...options, fetcher: discoveryFetcher(data) }), /stale/);
});

test('expired verified cache survives offline and 304; fresh higher sequence replaces it', async () => {
  const old = signedDiscoveryFixture();
  const cache = memoryCache({ bytes: old.bytes, etags: { pointer: '"80"' } });
  for (const fetcher of [
    async () => {
      throw new Error('offline');
    },
    async () => new Response(null, { status: 304 }),
  ]) {
    const bundle = await loadDiscoveryForDisplay({ ...options, cache, fetcher });
    assert.equal(bundle.source, 'cache');
    assert.equal(bundle.freshness, 'stale');
    assert.equal(bundle.snapshot.sequence, 80);
  }
  const fresh = signedDiscoveryFixture({
    sequence: 81,
    generated: '2026-10-04T06:55:00Z',
    expires: '2026-10-05T06:55:00Z',
  });
  const bundle = await loadDiscoveryForDisplay({
    ...options,
    cache,
    fetcher: discoveryFetcher(fresh),
  });
  assert.equal(bundle.source, 'remote');
  assert.equal(bundle.freshness, 'current');
  assert.equal(bundle.snapshot.sequence, 81);
  assert.deepEqual(cache.value?.bytes, fresh.bytes);
});

test('display rejects tampered signatures/schema/search and future signed snapshots', async () => {
  const invalid = [
    signedDiscoveryFixture(),
    signedDiscoveryFixture(),
    signedDiscoveryFixture(),
    signedDiscoveryFixture({ generated: '2026-10-05T00:00:00Z', expires: '2026-10-06T00:00:00Z' }),
  ];
  const encode = (value: unknown) => new TextEncoder().encode(canonicalJSON(value, 'Discovery'));
  const envelope = JSON.parse(new TextDecoder().decode(invalid[0]!.bytes.envelope));
  envelope.signature = Buffer.alloc(64).toString('base64');
  invalid[0]!.bytes.envelope = encode(envelope);
  const snapshot = JSON.parse(new TextDecoder().decode(invalid[1]!.bytes.snapshot));
  snapshot.untrusted = true;
  invalid[1]!.bytes.snapshot = encode(snapshot);
  const search = JSON.parse(new TextDecoder().decode(invalid[2]!.bytes.search));
  search.records[0].description = 'tampered';
  invalid[2]!.bytes.search = encode(search);
  for (const data of invalid) {
    const cache = memoryCache();
    await assert.rejects(
      loadDiscoveryForDisplay({ ...options, cache, fetcher: discoveryFetcher(data) }),
    );
    assert.equal(cache.value, undefined);
  }
});

test('invalid network replacement falls back only to reverified cache; corrupt cache is unusable', async () => {
  const old = signedDiscoveryFixture();
  const invalid = signedDiscoveryFixture({ sequence: 81 });
  invalid.bytes.envelope[20] ^= 1;
  const cache = memoryCache({ bytes: old.bytes, etags: {} });
  const bundle = await loadDiscoveryForDisplay({
    ...options,
    cache,
    fetcher: discoveryFetcher(invalid),
  });
  assert.equal(bundle.snapshot.sequence, 80);
  assert.equal(bundle.source, 'cache');
  assert.deepEqual(cache.value?.bytes, old.bytes);
  cache.value = { bytes: invalid.bytes, etags: {} };
  await assert.rejects(
    loadDiscoveryForDisplay({
      ...options,
      cache,
      fetcher: async () => new Response(null, { status: 304 }),
    }),
  );
});

test('rollback and equal-sequence equivocation never replace verified display cache', async () => {
  const old = signedDiscoveryFixture();
  const cache = memoryCache({ bytes: old.bytes, etags: {} });
  const lower = signedDiscoveryFixture({ sequence: 79 });
  assert.equal(
    (await loadDiscoveryForDisplay({ ...options, cache, fetcher: discoveryFetcher(lower) }))
      .snapshot.sequence,
    80,
  );
  const fork = signedDiscoveryFixture();
  const pointer = JSON.parse(new TextDecoder().decode(fork.bytes.pointer));
  pointer.fetch_contract.retry_attempts = 2;
  fork.bytes.pointer = new TextEncoder().encode(canonicalJSON(pointer, 'Discovery'));
  const retained = await loadDiscoveryForDisplay({
    ...options,
    cache,
    fetcher: discoveryFetcher(fork),
  });
  assert.equal(retained.snapshot.sequence, 80);
  assert.equal(retained.source, 'cache');
  await assert.rejects(
    loadDiscovery({
      ...options,
      now: new Date('2026-10-04T06:00:00Z'),
      cache,
      fetcher: discoveryFetcher(fork),
    }),
    /equivocation/,
  );
  assert.deepEqual(cache.value?.bytes, old.bytes);
});
