
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { projectCatalogCommands, projectCatalogSelection } from '../utils/projectCatalogSource.ts';
import type { ClientID, RegistryPlugin } from '../types/registry.ts';

function packageWith(components: Array<'mcp' | 'skills'>): RegistryPlugin {
  const fixture = JSON.parse(readFileSync(
    new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8',
  )) as { plugins: RegistryPlugin[] };
  const plugin = structuredClone(fixture.plugins[0]!);
  plugin.trust_state = 'reviewed';
  const release = plugin.distributions[0]!.releases[0]!;
  release.components = components;
  for (const client of ['grok', 'kimi', 'qoder'] as const) {
    if (!release.targets.some((target) => target.client === client)) {
      release.targets.push({
        client, delivery: 'managed', authentication: 'not_required', scopes: ['user'],
      });
    }
  }
  return plugin;
}

test('reviewed portable project matrix allows explicit prepared CLI source only', () => {
  const plugin = packageWith(['skills', 'mcp']);
  const qualified: ClientID[] = [
    'claude', 'codex', 'opencode', 'cursor', 'gemini', 'copilot', 'kimi', 'cline',
  ];
  for (const client of qualified) {
    const selected = projectCatalogSelection(plugin, [client]);
    assert.ok(selected.directSource, 'missing pinned source for ' + client);
    assert.equal(selected.resolution.distribution?.targets.find((target) => target.client === client)?.delivery, 'prepared');
    assert.ok(projectCatalogCommands(plugin, [client]).add.endsWith('--target ' + client + ' --scope project'));
  }
  for (const client of ['grok', 'qoder', 'windsurf'] as const) {
    assert.equal(projectCatalogSelection(plugin, [client]).resolution.distribution, undefined,
      client + ' cannot advertise project MCP without qualification');
    const skillsOnly = packageWith(['skills']);
    assert.ok(projectCatalogSelection(skillsOnly, [client]).directSource);
    assert.ok(projectCatalogCommands(skillsOnly, [client]).add.endsWith('--target ' + client + ' --scope project'));
  }
  assert.equal(projectCatalogSelection(plugin, ['codex', 'grok']).resolution.distribution, undefined);
});

test('unsupported or unsigned project targets never claim project authority', () => {
  const plugin = packageWith(['skills']);
  for (const client of ['chatgpt', 'pi', 'kiro', 'vscode'] as const) {
    assert.equal(projectCatalogSelection(plugin, [client]).resolution.distribution, undefined);
  }
  delete plugin.trust_state;
  assert.equal(projectCatalogSelection(plugin, ['codex']).resolution.distribution, undefined);
});
