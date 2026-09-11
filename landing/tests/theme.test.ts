import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import { test } from 'node:test';
import { createPinia } from 'pinia';
import { computed, watch } from 'vue';
import { useThemeStore } from '../stores/theme.ts';

function evaluate(path: string, globals: object, result: string) {
  const source = readFileSync(new URL(path, import.meta.url), 'utf8')
    .replace(/^import .*;\n/gm, '')
    .replace(/export default /g, 'const plugin = ')
    .replace(/export const /g, 'const ')
    .replace(/import.meta.hot/g, 'false')
    .replace(/import.meta.server/g, 'false');
  return runInNewContext(stripTypeScriptTypes(source) + `\n${result}`, globals);
}
const adapter = evaluate(
  '../composables/useBrowserTheme.ts',
  { computed, watch, useThemeStore },
  'createBrowserThemeAdapter',
);
function fixture(saved: string | null = null, denied = false) {
  const store = useThemeStore(createPinia());
  const changes: string[] = [];
  const writes: string[] = [];
  const listeners = new Set<(event: { matches: boolean }) => void>();
  const browser = {
    get localStorage() {
      if (denied) throw Error('denied');
      return {
        getItem: () => saved,
        setItem: (_: string, value: string) => {
          writes.push(value);
        },
      };
    },
    matchMedia: () => ({
      matches: false,
      addEventListener: (_: string, fn: any) => listeners.add(fn),
      removeEventListener: (_: string, fn: any) => listeners.delete(fn),
    }),
  };
  const start = () => adapter(store, { change: (name: string) => changes.push(name) }, browser);
  return { store, changes, writes, listeners, browser, start };
}
test('saved light restores Pinia and Vuetify; toggle persists and cleanup stops effects', () => {
  const f = fixture('light');
  const stop = f.start();
  assert.equal(f.store.current, 'light');
  assert.equal(f.store.userSelected, true);
  assert.deepEqual(f.changes, ['light']);
  f.store.toggleTheme();
  assert.equal(f.changes.at(-1), 'dark');
  assert.equal(f.writes.at(-1), 'dark');
  stop();
  f.store.toggleTheme();
  assert.equal(f.changes.at(-1), 'dark');
});
test('invalid storage and system-light start dark; system changes stop overriding manual choice', () => {
  const f = fixture('invalid');
  const stop = f.start();
  assert.equal(f.store.current, 'dark');
  assert.equal(f.listeners.size, 1);
  for (const listener of f.listeners) listener({ matches: false });
  assert.equal(f.store.current, 'light');
  assert.deepEqual(f.writes, []);
  f.store.setTheme('light', true);
  for (const listener of f.listeners) listener({ matches: true });
  assert.equal(f.store.current, 'light');
  assert.equal(f.writes.at(-1), 'light');
  stop();
  assert.equal(f.listeners.size, 0);
});
test('denied storage still updates Vuetify and manual preference', () => {
  const f = fixture(null, true);
  const stop = f.start();
  f.store.toggleTheme();
  assert.equal(f.store.current, 'light');
  assert.equal(f.store.userSelected, true);
  assert.equal(f.changes.at(-1), 'light');
  stop();
});
test('read/write method failures and a choice before readiness remain usable', () => {
  const f = fixture('dark');
  Object.defineProperty(f.browser, 'localStorage', {
    value: {
      getItem() {
        throw Error('read denied');
      },
      setItem() {
        throw Error('write denied');
      },
    },
  });
  f.store.setTheme('light', true);
  const stop = f.start();
  assert.equal(f.changes.at(-1), 'light');
  f.store.toggleTheme();
  assert.equal(f.changes.at(-1), 'dark');
  stop();
  const g = fixture('dark');
  g.store.setTheme('light', true);
  g.start()();
  assert.equal(g.store.current, 'light');
});
test('plugin waits for installed Nuxt readiness, installs once, and cleans up on unmount', () => {
  const f = fixture('light');
  const hooks = new Map<string, () => void>();
  const idle: (() => void)[] = [];
  let unmount = () => {};
  let localeCalls = 0;
  const app = {
    isHydrating: true,
    hooks: { hookOnce: (name: string, callback: () => void) => hooks.set(name, callback) },
    vueApp: {
      onUnmount: (fn: () => void) => {
        unmount = fn;
      },
    },
    $vuetifyTheme: { change: (name: string) => f.changes.push(name) },
  };
  const onNuxtReady = evaluate(
    '../node_modules/nuxt/dist/app/composables/ready.js',
    { useNuxtApp: () => app, requestIdleCallback: (fn: () => void) => idle.push(fn) },
    'onNuxtReady',
  );
  const plugin = evaluate(
    '../plugins/init-theme-locale.client.ts',
    {
      defineNuxtPlugin: (p: unknown) => p,
      useThemeStore: () => f.store,
      useLocation: () => ({ initLocale: () => localeCalls++ }),
      onNuxtReady,
      createBrowserThemeAdapter: adapter,
      window: f.browser,
    },
    'plugin',
  );
  plugin.setup(app);
  // Root mount can finish while the async layout still holds deferHydration.
  assert.equal(f.store.current, 'dark');
  assert.deepEqual(f.changes, []);
  assert.equal(localeCalls, 0);
  app.isHydrating = false;
  hooks.get('app:suspense:resolve')!();
  assert.equal(f.store.current, 'dark');
  idle.shift()!();
  assert.equal(f.store.current, 'light');
  assert.deepEqual(f.changes, ['light']);
  assert.equal(localeCalls, 1);
  unmount();
  f.store.toggleTheme();
  assert.deepEqual(f.changes, ['light']);
});
test('component composable has no browser effects or adapter installation', () => {
  const f = fixture();
  let watchers = 0;
  const useBrowserTheme = evaluate(
    '../composables/useBrowserTheme.ts',
    { computed, watch: () => watchers++, useThemeStore: () => f.store },
    'useBrowserTheme',
  );
  const a = useBrowserTheme();
  const b = useBrowserTheme();
  a.toggleTheme();
  assert.equal(b.currentTheme.value, 'light');
  assert.equal(watchers, 0);
});
test('plugin initializes locale with denied storage and cancels deferred work after unmount', () => {
  for (const cancel of [false, true]) {
    const f = fixture(null, true);
    let ready = () => {};
    let unmount = () => {};
    let localeCalls = 0;
    const plugin = evaluate(
      '../plugins/init-theme-locale.client.ts',
      {
        defineNuxtPlugin: (p: unknown) => p,
        useThemeStore: () => f.store,
        useLocation: () => ({ initLocale: () => localeCalls++ }),
        onNuxtReady: (fn: () => void) => {
          ready = fn;
        },
        createBrowserThemeAdapter: adapter,
        window: f.browser,
      },
      'plugin',
    );
    plugin.setup({
      vueApp: {
        onUnmount: (fn: () => void) => {
          unmount = fn;
        },
      },
      $vuetifyTheme: { change: (name: string) => f.changes.push(name) },
    });
    if (cancel) unmount();
    ready();
    assert.equal(localeCalls, cancel ? 0 : 1);
    assert.equal(f.listeners.size, cancel ? 0 : 1);
    unmount();
    assert.equal(f.listeners.size, 0);
  }
});
