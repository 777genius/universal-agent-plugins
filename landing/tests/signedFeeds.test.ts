import assert from 'node:assert/strict';
import { test } from 'node:test';
import { loadFirstAvailable, resolveSignedFeedOrigins } from '../utils/signedFeeds.ts';

test('uses the baked same-origin feed when no live registry origin is configured', () => {
  assert.deepEqual(
    resolveSignedFeedOrigins({
      kind: 'discovery',
      baseURL: '/universal-agent-plugins/',
      pageOrigin: 'https://777genius.github.io',
    }).map((origin) => origin.href),
    ['https://777genius.github.io/universal-agent-plugins/discovery/'],
  );
});

test('tries the baked copy first and the live registry feed second', () => {
  assert.deepEqual(
    resolveSignedFeedOrigins({
      kind: 'discovery',
      baseURL: '/universal-agent-plugins/',
      pageOrigin: 'https://777genius.github.io',
      registryPagesOrigin: 'https://777genius.github.io/universal-agent-plugins-registry',
    }).map((origin) => origin.href),
    [
      'https://777genius.github.io/universal-agent-plugins/discovery/',
      'https://777genius.github.io/universal-agent-plugins-registry/discovery/',
    ],
  );
});

test('does not duplicate origins when the site is the registry itself', () => {
  assert.deepEqual(
    resolveSignedFeedOrigins({
      kind: 'security',
      baseURL: '/universal-agent-plugins-registry/',
      pageOrigin: 'https://777genius.github.io',
      registryPagesOrigin: 'https://777genius.github.io/universal-agent-plugins-registry/',
    }).map((origin) => origin.href),
    ['https://777genius.github.io/universal-agent-plugins-registry/security/'],
  );
});

test('ignores an invalid live registry origin', () => {
  assert.deepEqual(
    resolveSignedFeedOrigins({
      kind: 'discovery',
      baseURL: '/',
      pageOrigin: 'http://127.0.0.1:3000',
      registryPagesOrigin: 'not a url',
    }).map((origin) => origin.href),
    ['http://127.0.0.1:3000/discovery/'],
  );
});

test('loadFirstAvailable returns the first successful origin', async () => {
  const seen: string[] = [];
  const value = await loadFirstAvailable(
    [new URL('https://baked.example/discovery/'), new URL('https://live.example/discovery/')],
    async (origin) => {
      seen.push(origin.href);
      return origin.href;
    },
  );
  assert.equal(value, 'https://baked.example/discovery/');
  assert.deepEqual(seen, ['https://baked.example/discovery/']);
});

test('loadFirstAvailable falls back after a stale baked copy', async () => {
  const seen: string[] = [];
  const value = await loadFirstAvailable(
    [new URL('https://baked.example/discovery/'), new URL('https://live.example/discovery/')],
    async (origin) => {
      seen.push(origin.href);
      if (origin.hostname === 'baked.example') throw new Error('Discovery snapshot is stale');
      return 'live';
    },
  );
  assert.equal(value, 'live');
  assert.deepEqual(seen, ['https://baked.example/discovery/', 'https://live.example/discovery/']);
});

test('loadFirstAvailable throws the last error when every origin fails', async () => {
  await assert.rejects(
    () =>
      loadFirstAvailable(
        [new URL('https://baked.example/discovery/'), new URL('https://live.example/discovery/')],
        async (origin) => {
          throw new Error(`${origin.hostname} unavailable`);
        },
      ),
    { message: 'live.example unavailable' },
  );
});
