import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { effectScope, nextTick, reactive, toRaw, unref } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { usePluginInstallSelection } from '../composables/usePluginInstallSelection.ts';
import { useInstallPreferencesStore } from '../stores/installPreferences.ts';
import { pluginCommands } from '../utils/commands.ts';
import { reconcileLocalChatGPTTargets } from '../data/installTargets.ts';
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


test('Codex and Desktop picker choices reconcile to one native backend', () => {
  assert.deepEqual(reconcileLocalChatGPTTargets(['codex', 'chatgpt', 'chatgptweb'], ['codex']), ['chatgpt', 'chatgptweb']);
  assert.deepEqual(reconcileLocalChatGPTTargets(['codex', 'chatgpt', 'chatgptweb'], ['chatgpt']), ['codex', 'chatgptweb']);
  assert.deepEqual(reconcileLocalChatGPTTargets(['codex', 'chatgpt'], ['cursor']), ['codex']);
  assert.deepEqual(reconcileLocalChatGPTTargets(['chatgpt', 'chatgptweb'], ['chatgpt']), ['chatgpt', 'chatgptweb']);
});

test('restored Codex and Desktop preferences cannot emit duplicate local install targets', () => {
  setActivePinia(createPinia());
  const plugin = data();
  const preferences = useInstallPreferencesStore();
  preferences.selectPackage(
    plugin.install_source,
    { targetIds: ['codex', 'chatgpt'], autoDetect: false, expanded: true, scope: 'user' },
    ['codex', 'chatgpt', 'cursor'],
  );
  const state = mount(plugin);
  assert.deepEqual(state.targets.value, ['codex']);
  assert.match(pluginCommands(plugin, state.targets.value).add, /--target codex$/);
  assert.doesNotMatch(pluginCommands(plugin, state.targets.value).add, /codex,chatgpt/);
  state.stop();
});

test('mixed ChatGPT Web and Codex/Desktop commands use the imperative no-save path', () => {
  const plugin = data();
  const oldCommand = pluginCommands(plugin, ['codex', 'chatgptweb']).add;
  const splitCommand = pluginCommands(plugin, ['chatgpt', 'chatgptweb']).add;
  assert.match(oldCommand, /--target codex,chatgptweb --no-save$/);
  assert.match(splitCommand, /--target chatgpt,chatgptweb --no-save$/);
  assert.doesNotMatch(pluginCommands(plugin, ['codex']).add, /--no-save/);
});

test('unreviewed community app metadata cannot authorize a ChatGPT Web connection', () => {
  setActivePinia(createPinia());
  const plugin = data();
  plugin.trust_state = 'conformant_unreviewed';
  plugin.distributions[0]!.releases[0]!.targets.push({
    client: 'chatgpt',
    scopes: ['user'],
    delivery: 'manual_activation',
    authentication: 'unknown',
    app_binding: { app_key: 'spoofed', id: 'untrusted_app', mcp_server: 'untrusted' },
  });
  const state = mount(plugin);
  assert.ok(state.availableClients.value.some((client) => client.id === 'chatgpt'));
  assert.equal(state.availableClients.value.some((client) => client.id === 'chatgptweb'), false);
  // Exercise an invalid persisted choice as well as the ordinary picker.
  state.targets.value = ['chatgptweb'];
  assert.equal(state.resolution.value.distribution, undefined);
  assert.match(state.resolution.value.unavailable_reason ?? '', /reviewed signed Directory/);
  state.stop();
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
  assert.ok(state.availableClients.value.some((client) => client.id === 'chatgpt'));
  assert.equal(state.availableClients.value.some((client) => client.id === 'chatgptweb'), false);
  assert.equal(state.targetDistributions.value.get('chatgptweb'), undefined);
  assert.ok(state.targetDistributions.value.get('chatgpt'));
  release.targets[release.targets.length - 1]!.app_binding = {
    app_key: 'context7',
    id: 'asdk_app_verified_test',
    mcp_server: 'context7',
  };
  assert.ok(state.availableClients.value.some((client) => client.id === 'chatgptweb'));
  state.updateTargets(['chatgptweb']);
  state.updateAutoDetect(false);
  assert.ok(state.resolution.value.distribution?.targets.some((target) => target.client === 'chatgpt'));
  state.updateTargets(['chatgpt']);
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

test('Web picker uses the eligible signed release rather than a de-duplicated target policy', () => {
  setActivePinia(createPinia());
  const plugin = data();
  const older = plugin.distributions[0]!.releases[0]!;
  older.targets.push({
    client: 'chatgpt',
    scopes: ['user'],
    delivery: 'manual_activation',
    authentication: 'unknown',
  });
  // Reachable target summaries collapse identical client/authentication pairs.
  // An earlier unbound release must not hide the later signed Web binding.
  const newer = structuredClone(toRaw(older));
  newer.release_sequence = older.release_sequence + 1;
  newer.targets.find((target) => target.client === 'chatgpt')!.app_binding = {
    app_key: 'context7',
    id: 'asdk_app_verified_test',
    mcp_server: 'context7',
  };
  plugin.distributions[0]!.releases = [older, newer];
  const state = mount(plugin);
  assert.ok(state.availableClients.value.some((client) => client.id === 'chatgptweb'));
  state.updateTargets(['chatgptweb']);
  state.updateAutoDetect(false);
  assert.equal(state.resolution.value.distribution?.release_sequence, newer.release_sequence);
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
  assert.deepEqual(state.targets.value, ['codex'], 'Explicit eligible choice replaces the unsupported saved placeholder');
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
  assert.equal(state.autoDetect.value, false);
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

test('explicit source selection is exclusive, persists across navigation and never enters automatic commands', async () => {
  setActivePinia(createPinia());
  const plugin = data();
  plugin.trust_state = 'reviewed';
  let state = mount(plugin);
  assert.equal(unref(state.pinnedSourceCommands), undefined);
  state.updateAutoDetect(false);
  state.updateTargets(['cursor', 'hermes']);
  assert.deepEqual(state.targets.value, ['hermes']);
  assert.match(state.pinnedSourceCommands.value?.add ?? '', /^npx -y universal-agent-plugins@0\.1\.108 add github:/);
  assert.equal(state.resolution.value.distribution, undefined);
  state.toggleInstall();
  await nextTick();
  state.stop();
  state = mount(plugin);
  assert.equal(state.autoDetect.value, false);
  assert.deepEqual(state.targets.value, ['hermes']);
  assert.equal(state.pinnedSourceSelected.value, true);
  state.updateTargets(['hermes', 'pi']);
  assert.deepEqual(state.targets.value, ['pi']);
  state.updateTargets(['pi', 'codex']);
  assert.deepEqual(state.targets.value, ['codex']);
  assert.equal(unref(state.pinnedSourceCommands), undefined);
  assert.ok(state.resolution.value.distribution);
  state.updateTargets(['codex', 'antigravity']);
  state.updateAutoDetect(true);
  assert.equal(unref(state.pinnedSourceCommands), undefined);
  assert.equal(pluginCommands(plugin).add, 'npx -y universal-agent-plugins add gitlab');
  state.stop();
});

for (const [scope, client] of [['user', 'hermes'], ['project', 'pi']] as const) {
  test(`blocked explicit ${scope} source survives save and navigation until explicit recovery`, async () => {
    setActivePinia(createPinia());
    const plugin = data();
    plugin.trust_state = 'reviewed';
    let state = mount(plugin);
    state.updateScope(scope);
    state.updateAutoDetect(false);
    state.updateTargets([client]);
    state.toggleInstall();
    assert.equal(state.pinnedSourceSelected.value, true);
    plugin.distributions[0]!.releases[0]!.blocking_clients.push(client);
    await nextTick();
    assert.equal(state.autoDetect.value, false);
    assert.deepEqual(state.targets.value, [client]);
    assert.equal(state.pinnedSourceSelected.value, false);
    assert.equal(state.availability.value, undefined);
    assert.equal(unref(state.pinnedSourceCommands), undefined);
    // A subsequent UI edit saves the declined intent again through selectPackage.
    state.toggleInstall();
    state.toggleInstall();
    await nextTick();
    const preferences = useInstallPreferencesStore();
    assert.deepEqual(preferences.package.targetIds, [client]);
    assert.equal(preferences.package.autoDetect, false);
    assert.equal(preferences.package.scope, scope);
    state.stop();
    state = mount(plugin);
    assert.equal(state.scope.value, scope);
    assert.equal(state.autoDetect.value, false);
    assert.deepEqual(state.targets.value, [client]);
    assert.equal(state.installExpanded.value, true);
    assert.equal(state.availability.value, undefined);
    assert.equal(state.resolution.value.distribution, undefined);
    assert.equal(unref(state.pinnedSourceCommands), undefined);
    const replacement = scope === 'user' ? 'cursor' : 'codex';
    state.updateTargets([client, replacement]);
    assert.deepEqual(state.targets.value, [replacement], 'The picker includes the retained checked option');
    assert.equal(state.scope.value, scope);
    assert.equal(state.autoDetect.value, false);
    assert.equal(unref(state.availability)?.kind, 'signed');
    state.stop();
  });
}
