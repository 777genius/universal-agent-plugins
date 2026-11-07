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
  for (const client of ['grok', 'kimi', 'qoder', 'kiro', 'vscode', 'pi'] as const) {
    if (!release.targets.some((target) => target.client === client)) {
      release.targets.push({ client, delivery: 'managed', authentication: 'not_required', scopes: ['user'] });
    }
  }
  return plugin;
}

test('portable Project Skills and MCP use signed names for eligible agents', () => {
  const plugin = packageWith(['skills', 'mcp']);
  const qualified: ClientID[] = [
    'claude', 'codex', 'opencode', 'cursor', 'gemini', 'copilot',
    'kimi', 'cline', 'qoder', 'kiro', 'vscode', 'pi',
  ];
  for (const client of qualified) {
    const selected = projectCatalogSelection(plugin, [client]);
    assert.ok(selected.resolution.distribution, 'Project source missing for ' + client);
    assert.equal(selected.resolution.distribution?.targets.find(x => x.client === client)?.delivery, 'prepared');
    assert.equal(projectCatalogCommands(plugin, [client]).add,
      'npx -y universal-agent-plugins add gitlab --target ' + client + ' --scope project' +
      (client === 'vscode' ? ' --no-save' : ''));
  }
  assert.equal(projectCatalogCommands(plugin).add,
    'npx -y universal-agent-plugins add gitlab --scope project');
  for (const client of ['grok', 'windsurf'] as const) {
    assert.equal(projectCatalogSelection(plugin, [client]).resolution.distribution, undefined,
      client + ' cannot advertise Project MCP without qualification');
    assert.ok(projectCatalogSelection(packageWith(['skills']), [client]).resolution.distribution);
  }
  assert.equal(projectCatalogSelection(plugin, ['codex', 'grok']).resolution.distribution, undefined);
});

test('unsigned and ChatGPT project routes never become copyable', () => {
  const plugin = packageWith(['skills']);
  assert.equal(projectCatalogSelection(plugin, ['chatgpt']).resolution.distribution, undefined);
  delete plugin.trust_state;
  assert.equal(projectCatalogSelection(plugin, ['codex']).resolution.distribution, undefined);
});
