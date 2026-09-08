import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire, registerHooks } from 'node:module';
import { test } from 'node:test';
import { pathToFileURL } from 'node:url';
import type { RegistryIndex } from '../types/registry';

// Resolve the same pinned decoder Nuxt uses, without adding a direct dependency.
const nuxtRequire = createRequire(import.meta.resolve('nuxt'));
const { createFetch } = await import(pathToFileURL(nuxtRequire.resolve('ofetch')).href);
const aliases = registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith('~/')) {
      return nextResolve(new URL(`../${specifier.slice(2)}.ts`, import.meta.url).href, context);
    }
    return nextResolve(specifier, context);
  },
});
const { useRegistryPage, useRegistry } = await import('../composables/useRegistry.ts');
aliases.deregister();

// Exact captured Pages response bytes; catalog and Codex were byte-identical.
// Trace resource IDs and SHA-256 provenance are recorded in the task handoff.
const bytes = (name: string) =>
  readFileSync(new URL(`./fixtures/registry-responses/${name}.json`, import.meta.url), 'utf8');
const catalog = bytes('catalog');
const gitlab = bytes('gitlab');
const empty = bytes('empty');

test('production registry loader decodes octet-stream and fails closed before state assignment', async (t) => {
  const states = new Map<string, { value: unknown }>();
  const ref = (value?: unknown) => ({ value });
  let body = catalog;
  let httpStatus = 200;
  const requests: string[] = [];
  const decoder = createFetch({
    fetch: async (request: string) => {
      requests.push(String(request));
      return new Response(body, {
        status: httpStatus,
        headers: { 'content-type': 'application/octet-stream' },
      });
    },
    defaults: { retry: 0 },
  });
  const globals = {
    $fetch: decoder,
    useRuntimeConfig: () => ({ public: {} }),
    useState: (key: string, init?: () => unknown) => {
      if (!states.has(key)) states.set(key, ref(init?.()));
      return states.get(key);
    },
    useDiscoveryStatus: () => ref({ state: 'idle', count: 0 }),
    shallowRef: ref,
    // Only Nuxt's async-data/ref shell is substituted; the production composable,
    // endpoint selection, state writes, and actual ofetch decoder execute intact.
    useAsyncData: async (_key: string, load: () => Promise<RegistryIndex>) => {
      try {
        return { data: ref(await load()), error: ref() };
      } catch (error) {
        return { data: ref(), error: ref(error) };
      }
    },
    createError: (options: object) => Object.assign(new Error(), options),
  };
  const previous = new Map(
    Object.keys(globals).map((key) => [key, Object.getOwnPropertyDescriptor(globalThis, key)]),
  );
  Object.assign(globalThis, globals);
  t.after(() => {
    for (const [key, descriptor] of previous) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  });

  await t.test('default pinned decoder reproduces the inferred Blob locally', async () => {
    const result = await decoder('/api/registry/catalog');
    assert.ok(result instanceof Blob);
    assert.equal(result.plugins, undefined);
    assert.equal(await result.text(), catalog);
  });

  for (const [projection, response, endpoint, count] of [
    [{ kind: 'catalog' }, catalog, '/api/registry/catalog', 28],
    [{ kind: 'client', value: 'codex' }, catalog, '/api/registry/client/codex', 28],
    [{ kind: 'plugin', value: 'gitlab' }, gitlab, '/api/registry/plugin/gitlab', 1],
    [{ kind: 'empty' }, empty, '/api/registry/empty', 0],
  ] as const) {
    await t.test(`${projection.kind} projection preserves genuine contents`, async () => {
      body = response;
      const result = await useRegistryPage({ projection });
      assert.equal(requests.at(-1), endpoint);
      assert.deepEqual(result, JSON.parse(response));
      assert.equal(result.plugins.length, count);
      if (projection.kind === 'client') {
        assert.equal(
          result.plugins.filter((p) => p.client_support.clients.includes('codex')).length,
          28,
        );
      }
      if (projection.kind === 'plugin') assert.ok(result.plugins.find((p) => p.name === 'gitlab'));
      assert.equal(useRegistry(), result);
      assert.equal(states.get('registry-page-key')?.value, `registry-page:${endpoint}`);
    });
  }

  for (const invalid of ['{"plugins":', 'null', '{}', '{"plugins":{}}']) {
    await t.test(
      `invalid response ${invalid} leaves the previous catalog and key intact`,
      async () => {
        body = catalog;
        const prior = await useRegistryPage();
        const key = states.get('registry-page-key')?.value;
        body = invalid;
        await assert.rejects(useRegistryPage({ projection: { kind: 'empty' } }), {
          statusCode: 500,
          statusMessage: 'Plugin directory is unavailable',
        });
        assert.equal(useRegistry(), prior);
        assert.deepEqual(prior, JSON.parse(catalog));
        assert.equal(states.get('registry-page-key')?.value, key);
        body = gitlab;
        assert.equal(
          (await useRegistryPage({ projection: { kind: 'plugin', value: 'gitlab' } })).plugins[0]
            ?.name,
          'gitlab',
        );
      },
    );
  }

  await t.test('HTTP failures retain the existing unavailable error and cause', async () => {
    const prior = useRegistry();
    const key = states.get('registry-page-key')?.value;
    httpStatus = 500;
    await assert.rejects(useRegistryPage(), (error: any) => {
      assert.equal(error.statusCode, 500);
      assert.equal(error.statusMessage, 'Plugin directory is unavailable');
      assert.equal(error.cause.statusCode, 500);
      return true;
    });
    assert.equal(useRegistry(), prior);
    assert.equal(states.get('registry-page-key')?.value, key);
  });
});
