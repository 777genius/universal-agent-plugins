import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import type { RegistryIndex, RegistryPlugin } from '../types/registry.ts';
import { filterPlugins } from '../utils/filter.ts';
import { agentInstallExample } from '../utils/agentInstallExample.ts';
import { projectRegistry } from '../utils/registryProjection.ts';

const fixture = JSON.parse(
  readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'),
) as RegistryIndex;
function onlyTarget(client: 'codex' | 'chatgpt' | 'grok' | 'kimi') {
  const plugin = structuredClone(fixture.plugins[0]!) as RegistryPlugin;
  plugin.name = client;
  plugin.trust_state = 'reviewed';
  for (const distribution of plugin.distributions) {
    for (const release of distribution.releases) {
      release.targets = [
        {
          client,
          delivery: client === 'chatgpt' ? 'manual_activation' : 'managed',
          authentication: 'required',
          scopes: ['user'],
          ...(client === 'chatgpt'
            ? { app_binding: { app_key: 'demo', id: 'asdk_app_demo_123', mcp_server: 'demo' } }
            : {}),
        },
      ];
    }
  }
  return plugin;
}

test('legacy Web query and explicit Desktop query select independent install surfaces', () => {
  const desktop = onlyTarget('codex');
  const web = onlyTarget('chatgpt');
  assert.deepEqual(
    filterPlugins([desktop, web], { client: 'chatgpt' }).map((p) => p.name),
    ['chatgpt'],
  );
  assert.deepEqual(
    filterPlugins([desktop, web], { client: 'chatgpt-desktop' }).map((p) => p.name),
    ['codex'],
  );
  const index = { ...fixture, plugins: [desktop, web] };
  assert.deepEqual(
    projectRegistry(index, { kind: 'client', value: 'chatgpt' }).plugins.map((p) => p.name),
    ['codex', 'chatgpt'],
  );
});

test('Pi projection includes only the existing guarded exact Git Project alternative', () => {
  const plugin = onlyTarget('codex');
  const index = { ...fixture, plugins: [plugin] };
  assert.equal(filterPlugins(index.plugins, { client: 'pi' }).length, 1);
  assert.equal(projectRegistry(index, { kind: 'client', value: 'pi' }).plugins.length, 1);
  plugin.distributions[0]!.releases[0]!.blocking_clients.push('pi');
  assert.equal(filterPlugins(index.plugins, { client: 'pi' }).length, 0);
  assert.equal(projectRegistry(index, { kind: 'client', value: 'pi' }).plugins.length, 0);
});

test('agent examples prefer eligible Context7 and retain Web-only aggregate targets', () => {
  const desktop = onlyTarget('codex');
  const context7 = onlyTarget('codex');
  context7.name = 'context7';
  assert.equal(agentInstallExample([desktop, context7], 'codex')?.plugin, context7);
  const web = onlyTarget('chatgpt');
  assert.equal(agentInstallExample([web], 'chatgpt')?.target, 'chatgptweb');
  assert.equal(agentInstallExample([web], 'hermes'), undefined);
});

for (const target of ['grok', 'kimi'] as const) {
  test(`${target} examples use eligible Context7 and skip it when only another package is eligible`, () => {
    const context7 = onlyTarget(target);
    context7.name = 'context7';
    const alternative = onlyTarget(target);
    alternative.name = 'another-package';
    assert.deepEqual(agentInstallExample([alternative, context7], target), { plugin: context7, target });
    context7.distributions[0]!.releases[0]!.blocking_clients.push(target);
    assert.deepEqual(agentInstallExample([context7, alternative], target), { plugin: alternative, target });
  });
}
