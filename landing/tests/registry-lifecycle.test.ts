import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import { withAppBase } from '../utils/localizedRoutes.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}
const settle = () => new Promise<void>((resolve) => setImmediate(resolve));
const source = readFileSync(new URL('../composables/useRegistry.ts', import.meta.url), 'utf8')
  .replace(/^import[\s\S]*?from '[^']+';\n/gm, '')
  .replaceAll('import.meta.client', 'true')
  .replaceAll('export ', '');
const script = stripTypeScriptTypes(source) + '\n({ useRegistryPage });';

function harness() {
  const states = new Map<string, { value: any }>();
  const status = { value: { state: 'idle', count: 0 } };
  const discovery = deferred<any>();
  const security = deferred<any>();
  const loads: { resolve: (value: any) => void; promise: Promise<any> }[] = [];
  const endpoints: string[] = [];
  const cacheKeys: string[] = [];
  let scope: { mount?: () => void; dispose?: () => void };
  let discoveryCalls = 0;
  let securityCalls = 0;
  let decorated = 0;
  class FocusElement {
    closest() {
      return true;
    }
  }
  const events = new Map<string, Set<() => void>>();
  const document = {
    activeElement: null as null | FocusElement,
    addEventListener(name: string, callback: () => void) {
      if (!events.has(name)) events.set(name, new Set());
      events.get(name)!.add(callback);
    },
    removeEventListener(name: string, callback: () => void) {
      events.get(name)?.delete(callback);
    },
  };
  const api = runInNewContext(script, {
    URL,
    Error,
    structuredClone,
    queueMicrotask,
    Element: FocusElement,
    document,
    location: { origin: 'https://example.test' },
    withAppBase,
    useI18n: () => ({ t: (key: string) => key }),
    useRuntimeConfig: () => ({
      public: {
        baseURL: '/universal-agent-plugins/',
        discoveryKeyID: 'key',
        discoveryPublicKey: 'public',
      },
    }),
    useState: (key: string, init?: () => unknown) => {
      if (!states.has(key)) states.set(key, { value: init?.() });
      return states.get(key);
    },
    shallowRef: () => ({ value: undefined }),
    useDiscoveryStatus: () => status,
    onMounted: (callback: () => void) => {
      scope.mount = callback;
    },
    onScopeDispose: (callback: () => void) => {
      scope.dispose = callback;
    },
    useAsyncData: async (key: string, load: () => Promise<any>) => {
      cacheKeys.push(key);
      return { data: { value: await load() }, error: { value: undefined } };
    },
    $fetch: (endpoint: string) => {
      endpoints.push(endpoint);
      const request = deferred<any>();
      loads.push(request);
      return request.promise;
    },
    createError: (options: object) => Object.assign(new Error(), options),
    BrowserDiscoveryCache: class {},
    loadDiscovery: () => {
      discoveryCalls++;
      return discovery.promise;
    },
    loadSecurity: () => {
      securityCalls++;
      return security.promise;
    },
    discoveryPlugin: (record: object) => record,
    applySecurityAssessment: (plugin: object) => {
      decorated++;
      return { ...plugin, checked: true };
    },
  });
  function start(kind = 'catalog', value?: string) {
    const ownScope = {};
    scope = ownScope;
    const ready = api.useRegistryPage({ projection: { kind, value }, discovery: true });
    const request = loads.at(-1)!;
    return {
      async mount(name: string) {
        request.resolve({
          schema_version: 1,
          data_source: 'published_snapshot',
          plugins: [{ name }],
        });
        await ready;
        (ownScope as typeof scope).mount?.();
        await settle();
      },
      dispose() {
        (ownScope as typeof scope).dispose?.();
      },
      ready,
    };
  }
  return {
    start,
    states,
    status,
    discovery,
    security,
    endpoints,
    cacheKeys,
    focus() {
      document.activeElement = new FocusElement();
    },
    blur() {
      document.activeElement = null;
      for (const callback of events.get('focusout') ?? []) callback();
    },
    get plugins() {
      return states.get('registry-index')!.value.plugins;
    },
    get counts() {
      return { discoveryCalls, securityCalls, decorated };
    },
  };
}
const bundle = {
  source: 'remote',
  search: { records: [{ name: 'community' }] },
  snapshot: { sequence: 2, generated_at: '2026-09-08' },
};

test('same-endpoint remount and home to catalog retain shared loads while old generations cannot apply', async () => {
  const h = harness();
  const home = h.start();
  await home.mount('home');
  const catalog = h.start();
  await catalog.mount('catalog');
  home.dispose();
  h.discovery.resolve(bundle);
  h.security.resolve({ snapshot: {} });
  await settle();
  assert.deepEqual(
    Array.from(h.plugins, (p: any) => p.name),
    ['catalog', 'community'],
  );
  assert.equal(h.counts.decorated, 2);
  assert.deepEqual(h.counts, { discoveryCalls: 1, securityCalls: 1, decorated: 2 });
  assert.equal(h.cacheKeys[0], h.cacheKeys[1]);
  assert.ok(
    h.endpoints.every((endpoint) => endpoint === '/universal-agent-plugins/api/registry/catalog'),
  );
});

test('A to B to A prevents stale seed success and stale rejection from restoring an old page', async () => {
  const h = harness();
  const a = h.start('client', 'codex');
  await a.mount('old-A');
  const b = h.start('client', 'cursor');
  const nextA = h.start('client', 'codex');
  await nextA.mount('new-A');
  await b.mount('late-B');
  a.dispose();
  b.dispose();
  h.discovery.reject(new Error('expired'));
  await settle();
  assert.equal(h.plugins[0].name, 'new-A');
  assert.equal(h.status.value.state, 'stale');
});

for (const phase of [
  'discovery',
  'error',
  'security',
  'discovery-focus',
  'security-focus',
] as const) {
  test(`leaving to download prevents late ${phase} writes`, async () => {
    const h = harness();
    const page = h.start();
    await page.mount('catalog');
    if (phase === 'discovery-focus') h.focus();
    if (phase.startsWith('security') || phase === 'discovery-focus') {
      h.discovery.resolve(bundle);
      await settle();
    }
    if (phase === 'security-focus') {
      h.focus();
      h.security.resolve({ snapshot: {} });
      await settle();
    }
    page.dispose();
    const prior = JSON.stringify({ plugins: h.plugins, status: h.status.value });
    if (phase === 'error') h.discovery.reject(new Error('unavailable'));
    else h.discovery.resolve(bundle);
    h.security.resolve({ snapshot: {} });
    h.blur();
    await settle();
    assert.equal(JSON.stringify({ plugins: h.plugins, status: h.status.value }), prior);
    assert.equal(h.counts.decorated, 0);
  });
}

test('a remount after the focus await only decorates the new seed', async () => {
  const h = harness();
  const old = h.start();
  await old.mount('old');
  h.focus();
  h.discovery.resolve(bundle);
  await settle();
  const next = h.start();
  await next.mount('new');
  old.dispose();
  h.security.resolve({ snapshot: {} });
  h.blur();
  await settle();
  assert.deepEqual(
    Array.from(h.plugins, (p: any) => p.name),
    ['new', 'community'],
  );
  assert.equal(h.counts.decorated, 2);
});
