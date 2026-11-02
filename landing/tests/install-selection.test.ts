import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { effectScope, nextTick, reactive, toRaw, unref } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { usePluginInstallSelection } from '../composables/usePluginInstallSelection.ts';
import { useInstallPreferencesStore } from '../stores/installPreferences.ts';
import { pluginCommands } from '../utils/commands.ts';
import type { RegistryPlugin } from '../types/registry.ts';

const fixture: RegistryPlugin = JSON.parse(
  readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'),
).plugins[0];
function data() {
  const plugin = structuredClone(fixture);
  const distribution = plugin.distributions[0]!;
  distribution.kind = 'community';
  distribution.status = 'active';
  const release = distribution.releases[0]!;
  release.release_status = 'active';
  release.blocking_clients = [];
  release.meets_minimum_capabilities = true;
  release.targets = [
    {
      client: 'codex',
      scopes: ['user', 'project'],
      delivery: 'prepared',
      authentication: 'required',
    },
    { client: 'cursor', scopes: ['user'], delivery: 'managed', authentication: 'not_required' },
  ];
  distribution.releases = [release];
  plugin.distributions = [distribution];
  return reactive(plugin);
}
function mount(plugin: RegistryPlugin) {
  const lifetime = effectScope();
  const selection = lifetime.run(() => usePluginInstallSelection(() => plugin))!;
  return { ...selection, stop: () => lifetime.stop() };
}

test('project requires explicit agents without replacing the user default or silently dropping targets', () => {
  setActivePinia(createPinia());
  const state = mount(data());
  assert.equal(state.scope.value, 'user');
  assert.equal(state.autoDetect.value, true);
  assert.deepEqual(state.targets.value, ['cursor']);
  state.updateScope('project');
  assert.equal(state.autoDetect.value, false);
  assert.deepEqual(state.targets.value, ['cursor']);
  assert.equal(state.resolution.value.distribution, undefined);
  state.updateAutoDetect(true);
  assert.equal(state.autoDetect.value, false);
  state.updateTargets(['codex', 'cursor']);
  assert.equal(state.resolution.value.distribution, undefined);
  state.updateTargets(['codex']);
  assert.equal(unref(state.resolution).distribution?.targets[0]?.authentication, 'required');
  assert.match(
    pluginCommands(data(), state.targets.value, true, state.scope.value).add,
    /--target codex --scope project$/,
  );
  state.stop();
});

test('project invalidation and remount preserve requested scope and invalid agents until explicit recovery', async () => {
  setActivePinia(createPinia());
  const plugin = data();
  let state = mount(plugin);
  state.updateScope('project');
  state.updateTargets(['codex']);
  state.toggleInstall();
  plugin.distributions[0]!.releases[0]!.targets =
    plugin.distributions[0]!.releases[0]!.targets.filter((target) => target.client !== 'codex');
  await nextTick();
  assert.equal(state.scope.value, 'project');
  assert.equal(state.autoDetect.value, false);
  assert.deepEqual(state.targets.value, ['codex']);
  assert.equal(state.resolution.value.distribution, undefined);
  assert.equal(state.projectAvailable.value, false);
  const preferences = useInstallPreferencesStore();
  assert.equal(preferences.package.scope, 'project');
  state.stop();
  state = mount(plugin);
  assert.equal(state.scope.value, 'project');
  assert.deepEqual(state.targets.value, ['codex']);
  assert.equal(state.resolution.value.distribution, undefined);
  assert.equal(state.installExpanded.value, true);
  state.updateScope('user');
  state.updateTargets(['cursor']);
  assert.equal(state.autoDetect.value, false);
  assert.ok(state.resolution.value.distribution);
  assert.doesNotMatch(
    pluginCommands(plugin, state.targets.value, true, state.scope.value).add,
    /--scope project/,
  );
  state.stop();
});

test('target and project availability include fallback and older releases rather than default aggregates', () => {
  setActivePinia(createPinia());
  const plugin = data();
  const primary = plugin.distributions[0]!;
  const fallback = structuredClone(toRaw(primary));
  fallback.id = 'project-fallback';
  fallback.releases[0]!.components = ['skills'];
  primary.releases[0]!.targets = primary.releases[0]!.targets.filter(
    (target) => target.client === 'cursor',
  );
  plugin.distributions.push(fallback);
  plugin.client_support.clients = ['cursor'];
  const state = mount(plugin);
  assert.ok(state.availableClients.value.some((client) => client.id === 'codex'));
  assert.equal(state.projectAvailable.value, true);
  state.updateScope('project');
  state.updateTargets(['codex']);
  assert.equal(state.resolution.value.distribution?.id, 'project-fallback');
  assert.deepEqual(state.resolution.value.distribution?.components, ['skills']);
  state.stop();
});

test('missing scope and ChatGPT user-only facts never create project availability', () => {
  setActivePinia(createPinia());
  const plugin = data();
  plugin.distributions[0]!.releases[0]!.targets = [
    {
      client: 'chatgpt',
      scopes: ['user'],
      delivery: 'manual_activation',
      authentication: 'unknown',
    },
    { client: 'codex', scopes: [], delivery: 'managed', authentication: 'unknown' },
  ];
  const state = mount(plugin);
  assert.equal(state.projectAvailable.value, false);
  state.updateScope('project');
  assert.equal(state.scope.value, 'user');
  assert.equal(state.autoDetect.value, true);
  state.stop();
});
