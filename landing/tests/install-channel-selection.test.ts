import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import * as Vue from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { useInstallPreferencesStore } from '../stores/installPreferences.ts';
import * as platform from '../utils/installPlatform.ts';

const source = readFileSync(
  new URL('../composables/useInstallChannelSelection.ts', import.meta.url),
  'utf8',
)
  .replace(/^import[\s\S]*?from '[^']+';\n/gm, '')
  .replace('export function', 'function')
  .replace("await import('bowser')", 'await loadBowser()');
function consumer(
  channels = Vue.ref([
    { id: 'npm', recommended: true },
    { id: 'script' },
    { id: 'brew' },
    { id: 'powershell' },
  ]),
  fail = false,
) {
  let mount!: () => Promise<void>;
  const scope = Vue.effectScope();
  const state = scope.run(() =>
    runInNewContext(stripTypeScriptTypes(source) + '\nuseInstallChannelSelection(channels)', {
      ...Vue,
      ...platform,
      useInstallPreferencesStore,
      channels,
      onMounted: (callback: () => Promise<void>) => {
        mount = callback;
      },
      window: { navigator: { userAgent: 'fixture' } },
      loadBowser: async () => {
        if (fail) throw new Error('detection unavailable');
        return { default: { getParser: () => ({ getOSName: () => 'Linux' }) } };
      },
    }),
  );
  return { state, channels, mount: () => mount(), stop: () => scope.stop() };
}

test('overlapping automatic locale consumers settle without tracking shared action reads', async () => {
  setActivePinia(createPinia());
  const store = useInstallPreferencesStore();
  // A bounded action guard turns the old infinite scheduler loop into a test failure.
  let selections = 0;
  store.$onAction(({ name }) => {
    if (name === 'selectChannel' && ++selections > 20)
      throw new Error('automatic consumers oscillated');
  });
  const old = consumer();
  assert.equal(store.channelId, 'npm'); // neutral SSR setup, no navigator detection
  await old.mount();
  await Vue.nextTick();
  assert.equal(store.channelId, 'script');
  const incoming = consumer();
  assert.equal(store.channelId, 'script'); // preserve hydrated state during async setup
  try {
    await incoming.mount();
    await Vue.nextTick();
    incoming.state.detectedInstallPlatform.value = 'windows';
    await Vue.nextTick();
    assert.equal(store.channelId, 'powershell');
    const settled = selections;
    await Vue.nextTick();
    assert.equal(selections, settled);
    assert.equal(store.channelUserSelected, false);
    old.stop();
    incoming.state.detectedInstallPlatform.value = 'macos';
    await Vue.nextTick();
    assert.equal(store.channelId, 'brew');
  } finally {
    old.stop();
    incoming.stop();
  }
});

test('manual priority, valid IDs, empty channels and detection fallback survive remount', async () => {
  setActivePinia(createPinia());
  const store = useInstallPreferencesStore();
  const first = consumer();
  first.state.selectInstallChannel('brew');
  const next = consumer(undefined, true);
  try {
    await first.mount();
    await next.mount();
    await Vue.nextTick();
    assert.equal(store.channelId, 'brew');
    assert.equal(store.channelUserSelected, true);
    next.state.selectInstallChannel('invalid');
    assert.equal(store.channelId, 'brew');
    first.stop();
    next.channels.value = [{ id: 'npm', recommended: true }, { id: 'script' }];
    await Vue.nextTick();
    assert.equal(store.channelId, 'npm');
    assert.equal(store.channelUserSelected, false);
    next.state.detectedInstallPlatform.value = 'linux';
    await Vue.nextTick();
    assert.equal(store.channelId, 'script');
    next.state.detectedInstallPlatform.value = 'mobile';
    await Vue.nextTick();
    assert.equal(store.channelId, 'npm');
    next.channels.value = [];
    await Vue.nextTick();
    assert.equal(store.channelId, null);
    next.channels.value = [{ id: 'script' }];
    await Vue.nextTick();
    assert.equal(store.channelId, 'script');
  } finally {
    first.stop();
    next.stop();
  }
});

test('failed mounted detection applies neutral fallback without resetting a pending route', async () => {
  setActivePinia(createPinia());
  const first = consumer();
  await first.mount();
  await Vue.nextTick();
  const next = consumer(undefined, true);
  try {
    assert.equal(useInstallPreferencesStore().channelId, 'script');
    first.stop();
    await next.mount();
    await Vue.nextTick();
    assert.equal(useInstallPreferencesStore().channelId, 'npm');
  } finally {
    first.stop();
    next.stop();
  }
});

test('mounted and still-pending consumers cannot recursively schedule each other', async () => {
  setActivePinia(createPinia());
  const store = useInstallPreferencesStore();
  let calls = 0;
  store.$onAction(({ name }) => {
    if (name === 'selectChannel' && ++calls > 20) throw new Error('automatic consumers oscillated');
  });
  const first = consumer();
  const pending = consumer();
  try {
    await first.mount();
    await Vue.nextTick();
    assert.equal(store.channelId, 'script');
    assert.ok(calls < 5, 'selection writes must be bounded while the new route awaits detection');
    await pending.mount();
    await Vue.nextTick();
    assert.equal(store.channelId, 'script');
  } finally {
    first.stop();
    pending.stop();
  }
});
