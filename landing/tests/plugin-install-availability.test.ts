import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import type { RegistryPlugin } from '../types/registry.ts';
import { hasPluginInstallTarget, pluginInstallAvailability, reconcilePluginSourceTargets } from '../utils/pluginInstallAvailability.ts';

function fixture(): RegistryPlugin {
  const data = JSON.parse(readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'));
  const plugin = structuredClone(data.plugins[0]) as RegistryPlugin;
  plugin.trust_state = 'reviewed';
  return plugin;
}

function context7(): RegistryPlugin {
  const plugin = fixture();
  plugin.name = plugin.install_source = 'context7';
  const distribution = plugin.distributions[0]!;
  distribution.id = plugin.default_distribution = plugin.declared_default_distribution = 'upstash/context7';
  distribution.kind = 'upstream';
  distribution.releases[0]!.materialized_clients = ['codex'];
  for (const source of [plugin.source, distribution.source, ...distribution.releases.map(release => release.source)]) {
    source.repository = 'upstash/context7';
    source.path = 'plugins/agent-plugins/context7';
  }
  return plugin;
}

test('Desktop uses the signed Codex user policy while Web requires its own route', () => {
  const plugin = fixture();
  assert.equal(pluginInstallAvailability(plugin, ['chatgpt'])?.kind, 'signed');
  assert.equal(pluginInstallAvailability(plugin, ['chatgpt'], 'project'), undefined);
  assert.equal(pluginInstallAvailability(plugin, ['chatgptweb']), undefined);
  assert.equal(hasPluginInstallTarget(plugin, 'chatgpt'), true);
  assert.equal(hasPluginInstallTarget(plugin, 'chatgptweb'), false);
});

test('guarded Context7 Web preparation is shared without synthesizing signed grants', () => {
  const plugin = context7();
  const before = structuredClone(plugin);
  const offered = pluginInstallAvailability(plugin, ['chatgptweb']);
  assert.equal(offered?.kind, 'guided_web');
  assert.equal(offered && 'resolution' in offered ? offered.resolution.distribution : undefined, undefined);
  assert.equal(pluginInstallAvailability(plugin, ['chatgptweb', 'codex'])?.kind, 'guided_web');
  assert.equal(pluginInstallAvailability(plugin, ['chatgptweb', 'hermes']), undefined);
  assert.equal(pluginInstallAvailability(plugin, ['chatgptweb'], 'project'), undefined);
  assert.equal(hasPluginInstallTarget(plugin, 'chatgptweb'), true);
  assert.deepEqual(plugin, before);
});

test('unreviewed, wrong-source, inactive and unavailable Context7 never gain guided setup', () => {
  const variants = [context7(), context7(), context7(), context7(), context7()];
  variants[0]!.trust_state = 'conformant_unreviewed';
  variants[1]!.distributions[0]!.releases[0]!.source.repository = 'other/context7';
  variants[2]!.distributions[0]!.releases[0]!.release_status = 'revoked';
  variants[3]!.installable = false;
  variants[4]!.install_source = 'other';
  for (const plugin of variants) assert.equal(pluginInstallAvailability(plugin, ['chatgptweb']), undefined);
});

test('bound Web stays a signed route and can combine with Desktop', () => {
  const plugin = fixture();
  plugin.distributions[0]!.releases[0]!.targets.push({
    client: 'chatgpt', scopes: ['user'], delivery: 'manual_activation', authentication: 'unknown',
    app_binding: { app_key: 'gitlab', id: 'asdk_app_verified_test', mcp_server: 'gitlab' },
  });
  assert.equal(pluginInstallAvailability(plugin, ['chatgptweb'])?.kind, 'signed');
  assert.equal(pluginInstallAvailability(plugin, ['chatgpt', 'chatgptweb'])?.kind, 'signed');
});

test('explicit saved source routes use generated agent and scope capabilities', () => {
  const plugin = fixture();
  const before = structuredClone(plugin);
  for (const client of ['pi', 'antigravity', 'hermes'] as const) {
    assert.equal(pluginInstallAvailability(plugin, [client])?.kind, 'pinned_git');
    assert.equal(hasPluginInstallTarget(plugin, client), true);
    assert.equal(pluginInstallAvailability(plugin, [client], 'project')?.kind,
      client === 'hermes' ? undefined : 'pinned_git');
  }
  assert.equal(pluginInstallAvailability(plugin, []), undefined);
  assert.equal(pluginInstallAvailability(plugin, ['pi', 'codex'], 'project'), undefined);
  assert.equal(pluginInstallAvailability(plugin, ['pi', 'hermes']), undefined);
  assert.deepEqual(plugin, before, 'Source availability must not synthesize signed target records');
  plugin.distributions[0]!.releases[0]!.blocking_clients.push('pi');
  assert.equal(hasPluginInstallTarget(plugin, 'pi'), false);
});

test('source requests stay exclusive when adding another source or a signed peer', () => {
  const plugin = fixture();
  assert.deepEqual(reconcilePluginSourceTargets(plugin, ['codex', 'pi'], ['codex']), ['pi']);
  assert.deepEqual(reconcilePluginSourceTargets(plugin, ['pi', 'hermes'], ['pi']), ['hermes']);
  assert.deepEqual(reconcilePluginSourceTargets(plugin, ['hermes', 'codex'], ['hermes']), ['codex']);
  assert.deepEqual(reconcilePluginSourceTargets(plugin, ['pi', 'antigravity'], ['pi'], 'project'), ['antigravity']);
});

test('canonical aliases and non-saved clients never gain a source fallback', () => {
  const plugin = fixture();
  plugin.distributions[0]!.releases[0]!.targets = [{
    client: 'claude', scopes: ['user'], delivery: 'managed', authentication: 'unknown',
  }];
  assert.equal(pluginInstallAvailability(plugin, ['codex'])?.kind, 'pinned_git');
  for (const client of ['chatgpt', 'chatgptweb', 'vscode'] as const)
    assert.equal(pluginInstallAvailability(plugin, [client]), undefined);
});
