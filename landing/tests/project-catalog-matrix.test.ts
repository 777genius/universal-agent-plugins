import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { projectCatalogCommands, projectCatalogSelection, projectComponentSupported } from '../utils/projectCatalogSource.ts';
import type { ClientID, ComponentID, RegistryPlugin } from '../types/registry.ts';

function packageWith(components: ComponentID[]): RegistryPlugin {
  const fixture = JSON.parse(readFileSync(
    new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8',
  )) as { plugins: RegistryPlugin[] };
  const plugin = structuredClone(fixture.plugins[0]!);
  plugin.trust_state = 'reviewed';
  const release = plugin.distributions[0]!.releases[0]!;
  release.components = components;
  for (const client of ['grok', 'kimi', 'qoder', 'kiro', 'vscode', 'pi', 'windsurf'] as const) {
    if (!release.targets.some((target) => target.client === client)) {
      release.targets.push({ client, delivery: 'managed', authentication: 'not_required', scopes: ['user'] });
    }
  }
  return plugin;
}

test('portable mixed Project package admits each target with one supported component', () => {
  const plugin = packageWith(['skills', 'mcp']);
  const targets: ClientID[] = [
    'claude', 'codex', 'opencode', 'cursor', 'gemini', 'copilot',
    'kimi', 'cline', 'kiro', 'vscode', 'pi', 'grok', 'qoder', 'windsurf',
  ];
  for (const client of targets) {
    const selected = projectCatalogSelection(plugin, [client]);
    assert.ok(selected.resolution.distribution, 'Project source missing for ' + client);
    assert.equal(selected.resolution.distribution?.targets.find(x => x.client === client)?.delivery, 'prepared');
    assert.equal(projectCatalogCommands(plugin, [client]).add,
      'npx -y universal-agent-plugins add gitlab --target ' + client + ' --scope project' +
      (client === 'vscode' ? ' --no-save' : ''));
  }
  assert.equal(projectCatalogCommands(plugin).add,
    'npx -y universal-agent-plugins add gitlab --scope project');
  assert.ok(projectCatalogSelection(plugin, ['codex', 'grok']).resolution.distribution,
    'a mixed package must not be refused merely because Grok cannot deliver its MCP part');
  assert.equal(projectComponentSupported('grok', 'skills'), true);
  assert.equal(projectComponentSupported('grok', 'mcp'), false);
  for (const client of ['qoder', 'windsurf'] as const) {
    assert.equal(projectComponentSupported(client, 'mcp'), true,
      client + ': Project MCP must match the published CLI, not the old site table');
  }
});

test('Project capability selection refuses unsupported-only packages, not mixed siblings', () => {
  const onlyMCP = packageWith(['mcp']);
  const onlySkills = packageWith(['skills']);
  const onlyExtensions = packageWith(['extensions']);
  assert.equal(projectCatalogSelection(onlyMCP, ['grok']).resolution.distribution, undefined);
  assert.ok(projectCatalogSelection(onlyMCP, ['codex']).resolution.distribution);
  assert.ok(projectCatalogSelection(onlyMCP, ['qoder']).resolution.distribution);
  assert.ok(projectCatalogSelection(onlySkills, ['grok']).resolution.distribution);
  assert.ok(projectCatalogSelection(onlyExtensions, ['pi']).resolution.distribution);
  assert.equal(projectCatalogSelection(onlyExtensions, ['codex']).resolution.distribution, undefined);
  assert.equal(projectCatalogSelection(onlyExtensions, ['grok']).resolution.distribution, undefined);
  const mixedExtensions = packageWith(['skills', 'extensions']);
  assert.ok(projectCatalogSelection(mixedExtensions, ['codex', 'grok']).resolution.distribution);
  assert.equal(projectComponentSupported('codex', 'extensions'), false);
  assert.equal(projectComponentSupported('pi', 'extensions'), true);
});

test('unsigned, no-component, and ChatGPT Project routes never become copyable', () => {
  const plugin = packageWith(['skills']);
  assert.equal(projectCatalogSelection(plugin, ['chatgpt']).resolution.distribution, undefined);
  assert.equal(projectCatalogSelection(packageWith([]), ['codex']).resolution.distribution, undefined);
  delete plugin.trust_state;
  assert.equal(projectCatalogSelection(plugin, ['codex']).resolution.distribution, undefined);
});
