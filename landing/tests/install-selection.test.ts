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


test('mixed ChatGPT Web and Codex/Desktop commands use the imperative no-save path', () => {
  const plugin = data();
  const oldCommand = pluginCommands(plugin, ['codex', 'chatgpt-web']).add;
  const splitCommand = pluginCommands(plugin, ['chatgpt-desktop', 'chatgpt-web']).add;
  assert.match(oldCommand, /--target codex,chatgpt-web --no-save$/);
  assert.match(splitCommand, /--target chatgpt-desktop,chatgpt-web --no-save$/);
  assert.doesNotMatch(pluginCommands(plugin, ['codex']).add, /--no-save/);
});

test('ChatGPT Web needs signed app binding; Desktop shares Codex without project fallback', () => {
  setActivePinia(createPinia());
  const plugin = data();
  const release = plugin.distributions[0]!.releases[0]!;
  release.targets.push({
    client: 'chatgpt',
    scopes: ['user'],
    delivery: 'manual_activation',
    authentication: 'unknown',
  });
  const state = mount(plugin);
  assert.ok(state.availableClients.value.some((client) => client.id === 'chatgpt-desktop'));
  assert.equal(state.availableClients.value.some((client) => client.id === 'chatgpt'), false);
  assert.equal(state.targetDistributions.value.get('chatgpt'), undefined);
  assert.ok(state.targetDistributions.value.get('chatgpt-desktop'));
  release.targets[release.targets.length - 1]!.app_binding = {
    app_key: 'context7',
    id: 'asdk_app_verified_test',
    mcp_server: 'context7',
  };
  assert.ok(state.availableClients.value.some((client) => client.id === 'chatgpt'));
  state.updateTargets(['chatgpt']);
  state.updateAutoDetect(false);
  assert.ok(state.resolution.value.distribution?.targets.some((target) => target.client === 'chatgpt'));
  state.updateTargets(['chatgpt-desktop']);
  assert.equal(state.resolution.value.distribution?.id, plugin.distributions[0]!.id);
  state.updateScope('project');
  assert.equal(state.scope.value, 'project');
  state.updateAutoDetect(false);
  assert.equal(state.resolution.value.distribution, undefined);
  state.updateScope('user');
  assert.equal(state.scope.value, 'user');
  assert.ok(state.resolution.value.distribution);
  state.stop();
});

test('project defaults to CLI detection and preserves manual project targets', () => {
  setActivePinia(createPinia());
  const state = mount(data());
  assert.equal(state.scope.value, 'user');
  assert.equal(state.autoDetect.value, true);
  assert.deepEqual(state.targets.value, ['cursor']);
  state.updateScope('project');
  assert.equal(state.autoDetect.value, true);
  assert.equal(state.projectAvailable.value, true);
  assert.ok(state.projectAutomatic.value?.resolution.distribution);
  assert.deepEqual(state.targets.value, ['cursor'], 'legacy manual placeholder must not choose an agent');
  assert.equal(state.resolution.value.distribution, undefined);
  state.updateAutoDetect(false);
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

test('switching user manual Cursor to project re-enables terminal detection', () => {
  setActivePinia(createPinia());
  const state = mount(data());
  state.updateAutoDetect(false);
  state.updateTargets(['cursor']);
  assert.equal(state.autoDetect.value, false);
  state.updateScope('project');
  assert.equal(state.autoDetect.value, true);
  assert.equal(state.projectAvailable.value, true);
  assert.equal(state.scope.value, 'project');
  // Selecting a project target explicitly still turns detection off.
  state.updateAutoDetect(false);
  state.updateTargets(['codex']);
  assert.equal(state.autoDetect.value, false);
  state.updateScope('project');
  assert.equal(state.autoDetect.value, false, 'reselecting the same project must not erase manual intent');
  state.stop();
});

test('project invalidation and remount preserve requested scope and invalid agents until explicit recovery', async () => {
  setActivePinia(createPinia());
  const plugin = data();
  let state = mount(plugin);
  state.updateScope('project');
  state.updateAutoDetect(false);
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
  assert.equal(state.autoDetect.value, true);
  state.updateAutoDetect(false);
  state.updateTargets(['codex']);
  assert.equal(state.resolution.value.distribution?.id, 'project-fallback');
  assert.deepEqual(state.resolution.value.distribution?.components, ['skills']);
  state.stop();
});

test('project auto selection survives remount and remains separate from user scope', async () => {
  setActivePinia(createPinia());
  const plugin = data();
  let state = mount(plugin);
  state.updateScope('project');
  state.toggleInstall();
  await nextTick();
  const store = useInstallPreferencesStore();
  assert.equal(store.package.scope, 'project');
  assert.equal(store.package.autoDetect, true);
  state.stop();
  state = mount(plugin);
  assert.equal(state.scope.value, 'project');
  assert.equal(state.autoDetect.value, true);
  assert.equal(state.projectAvailable.value, true);
  assert.equal(state.installExpanded.value, true);
  state.updateAutoDetect(false);
  state.updateTargets(['codex']);
  await nextTick();
  assert.equal(store.package.autoDetect, false);
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
