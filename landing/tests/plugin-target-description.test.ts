import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { installTargets, type InstallTargetID } from '../data/installTargets.ts';
import type { RegistryPlugin } from '../types/registry.ts';
import { pluginTargetDescription, type InstallTargetContext } from '../utils/pluginTargetDescription.ts';
import { pluginInstallAvailability } from '../utils/pluginInstallAvailability.ts';

const current: InstallTargetContext = { scope: 'user', published: true, expired: false, current: true };
function fixture(): RegistryPlugin {
  const data = JSON.parse(readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'));
  const plugin = structuredClone(data.plugins[0]) as RegistryPlugin;
  plugin.trust_state = 'reviewed';
  return plugin;
}
function hint(plugin: RegistryPlugin, id: InstallTargetID, context = current) {
  return pluginTargetDescription(plugin, installTargets.find(client => client.id === id)!, context);
}

test('unpublished target, native scope limitation and revoked policy have distinct explanations', () => {
  const plugin = fixture();
  for (const id of ['hermes', 'antigravity', 'pi'] as const) {
    assert.equal(hint(plugin, id).key, 'registryUi.install.catalogReleasePending');
    assert.equal(pluginInstallAvailability(plugin, [id]), undefined);
  }
  assert.equal(hint(plugin, 'hermes', { ...current, scope: 'project' }).key,
    'registryUi.install.unsupportedPluginComponents');
  const release = plugin.distributions[0]!.releases[0]!;
  plugin.distributions[0]!.kind = 'community';
  release.targets.push({ client: 'hermes', scopes: ['user'], delivery: 'managed', authentication: 'required' });
  assert.equal(hint(plugin, 'hermes', { ...current, scope: 'project' }).key,
    'registryUi.install.useComputerScope');
  release.release_status = 'revoked';
  assert.equal(hint(plugin, 'hermes').key, 'registryUi.install.noCompatibleSignedRelease');
  assert.equal(pluginInstallAvailability(plugin, ['hermes']), undefined);
  assert.equal(hint(plugin, 'hermes', { ...current, scope: 'project' }).key,
    'registryUi.install.unsupportedPluginComponents');
});

test('known negative evidence cannot be explained as merely an unpublished target', () => {
  const plugin = fixture();
  plugin.distributions[0]!.releases[0]!.blocking_clients.push('pi');
  assert.equal(hint(plugin, 'pi').key, 'registryUi.install.noCompatibleSignedRelease');
  assert.equal(pluginInstallAvailability(plugin, ['pi'], 'project'), undefined);
});

test('an eligible User route is explained when explicit policy excludes Project', () => {
  const plugin = fixture();
  const target = plugin.distributions[0]!.releases[0]!.targets.find(item => item.client === 'codex')!;
  target.canonical_closure = { eligible: true, closure_digest: 'sha256:' + 'a'.repeat(64) };
  assert.equal(pluginInstallAvailability(plugin, ['codex'], 'user')?.kind, 'signed');
  assert.equal(pluginInstallAvailability(plugin, ['codex'], 'project'), undefined);
  assert.equal(hint(plugin, 'codex', { ...current, scope: 'project' }).key,
    'registryUi.install.useComputerScope');
  target.canonical_closure.eligible = false;
  assert.equal(hint(plugin, 'codex', { ...current, scope: 'project' }).key,
    'registryUi.install.noCompatibleSignedRelease');
});

test('exact Pi Project source remains explicit and expires with its reviewed source', () => {
  const plugin = fixture();
  const project = { ...current, scope: 'project' as const };
  assert.equal(hint(plugin, 'pi', project).key, 'registryUi.install.piPinnedProjectSource');
  assert.equal(hint(plugin, 'pi', { ...project, expired: true, current: false }).key,
    'registryUi.install.unavailableSignedDirectorySnapshotExpired');
  assert.equal(hint(plugin, 'pi', { ...project, published: false, current: false }).key,
    'registryUi.install.unavailableReviewDataIsNotInstallationAuthority');
});

test('declared eligible delivery is shared without modifying signed input or authorizing other targets', () => {
  const plugin = fixture();
  const distribution = plugin.distributions[0]!;
  distribution.kind = 'community';
  distribution.releases[0]!.targets.push({
    client: 'antigravity', scopes: ['user'], delivery: 'prepared', authentication: 'required',
  });
  const before = structuredClone(plugin);
  assert.equal(hint(plugin, 'antigravity').key, 'registryUi.delivery.prepared');
  assert.equal(hint(plugin, 'hermes').key, 'registryUi.install.catalogReleasePending');
  assert.deepEqual(plugin, before);
});

test('Desktop retains its local scope and Web retains binding-specific instructions', () => {
  const plugin = fixture();
  assert.equal(hint(plugin, 'chatgpt').key, 'registryUi.desktopGuide.description');
  assert.equal(hint(plugin, 'chatgpt', { ...current, scope: 'project' }).key,
    'registryUi.install.useComputerScope');
  assert.equal(hint(plugin, 'chatgptweb').key, 'registryUi.install.notAvailableForChatgpt');
});

test('unsupported required component is not confused with missing catalog publication', () => {
  const plugin = fixture();
  plugin.components.push('extensions');
  assert.equal(hint(plugin, 'hermes').key, 'registryUi.install.unsupportedPluginComponents');
  assert.equal(hint(plugin, 'antigravity').key, 'registryUi.install.unsupportedPluginComponents');
});
