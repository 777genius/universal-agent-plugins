import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import { pluginCommands } from '../utils/commands.ts';

// Exercise the actual setup expressions, with Nuxt/Vue state supplied by this
// isolated harness. No signed production data or installer is mutated.
const component = readFileSync(
  new URL('../components/registry/InstallPanel.vue', import.meta.url),
  'utf8',
);
const setup = component
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)![1]!
  .replace(/^import .*;\n/gm, '');
const script = stripTypeScriptTypes(setup) + '\n({ commands, unavailableDiscoveryReason });';

const targetlessFixture = {
  name: 'targetless-fixture',
  install_source: 'discovery:example/targetless-fixture',
  installable: true,
  components: [],
  trust_state: 'conformant_unreviewed',
  client_support: { resolution: 'install_time', clients: [] },
  discovery: { availability: 'available' },
};

function panelState({ installable = true, autoDetect = true, current = true } = {}) {
  return runInNewContext(script, {
    defineProps: () => ({ plugin: { ...targetlessFixture, installable } }),
    defineModel: (name: string) => ({ value: name === 'targets' ? [] : autoDetect }),
    computed: (read: () => unknown) => ({
      get value() {
        return read();
      },
    }),
    useSite: () => ({ asset: (path: string) => path, sourceUrl: () => '' }),
    useDirectoryStatus: () => ({
      current: { value: current },
      expired: { value: !current },
      published: { value: true },
    }),
    clients: [{ id: 'codex', name: 'Codex', icon: 'codex.svg' }],
    resolveDistribution: () => ({ unavailable_reason: 'No compatible source.' }),
    expectedDistribution: () => undefined,
    deliveryLabel: () => '',
    pluginCommands,
    watch: () => {},
  });
}

test('installable install-time fixture with no targets exposes all four automatic commands', () => {
  const state = panelState();
  for (const action of ['add', 'update', 'repair', 'remove']) {
    assert.equal(
      state.commands.value[action],
      `npx universal-agent-plugins ${action} ${action === 'add' ? targetlessFixture.install_source : targetlessFixture.name}`,
    );
    assert.equal(state.commands.value[action].includes('--target'), false);
  }
  assert.equal(state.unavailableDiscoveryReason.value, '');
});

test('targetless fixture cannot bypass explicit selection or snapshot authority', () => {
  assert.equal(panelState({ autoDetect: false }).commands.value, undefined);
  assert.equal(panelState({ current: false }).commands.value, undefined);
});

test('non-installable targetless fixture has an explicit reason and no commands', () => {
  const state = panelState({ installable: false });
  assert.equal(state.commands.value, undefined);
  assert.match(state.unavailableDiscoveryReason.value, /doesn't include any tools/);
});
