import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { projectCatalogCommands, projectCatalogSelection } from '../utils/projectCatalogSource.ts';
import type { RegistryPlugin } from '../types/registry.ts';

function reviewed(): RegistryPlugin {
  const fixture = JSON.parse(readFileSync(
    new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8',
  )) as { plugins: RegistryPlugin[] };
  return { ...structuredClone(fixture.plugins[0]!), trust_state: 'reviewed' };
}

test('reviewed user-only v1 release can prepare a project from the exact pinned Git source', () => {
  const plugin = reviewed();
  const selection = projectCatalogSelection(plugin, ['codex']);
  const source = plugin.distributions[0]!.releases[0]!.source;
  const exact = `github:${source.repository}@${source.revision}//${source.path}`;
  assert.equal(selection.directSource, exact);
  assert.equal(selection.resolution.distribution?.id, plugin.default_distribution);
  const commands = projectCatalogCommands(plugin, ['codex']);
  assert.equal(commands.add, `npx -y universal-agent-plugins add ${exact} --target codex --scope project`);
  assert.equal(commands.update, commands.add);
  assert.equal(commands.repair, 'npx -y universal-agent-plugins repair gitlab --target codex --scope project');
  assert.equal(commands.remove, 'npx -y universal-agent-plugins remove gitlab --scope project');
});

test('a signed project policy takes precedence over direct source fallback', () => {
  const plugin = reviewed();
  plugin.distributions[0]!.releases[0]!.targets.find(x => x.client === 'codex')!.scopes = ['user', 'project'];
  const selection = projectCatalogSelection(plugin, ['codex']);
  assert.equal(selection.directSource, undefined);
  assert.equal(projectCatalogCommands(plugin, ['codex']).add,
    'npx -y universal-agent-plugins add gitlab --target codex --scope project');
});

test('direct project eligibility is fail-closed for unreviewed, unpinned and unsupported input', () => {
  const variants: RegistryPlugin[] = [];
  const unreviewed = reviewed();
  delete unreviewed.trust_state;
  variants.push(unreviewed);
  const unpinned = reviewed();
  unpinned.distributions[0]!.releases[0]!.source.revision = null;
  variants.push(unpinned);
  const extension = reviewed();
  extension.distributions[0]!.releases[0]!.components.push('extensions');
  variants.push(extension);
  const revoked = reviewed();
  revoked.distributions[0]!.releases[0]!.release_status = 'revoked';
  variants.push(revoked);
  const unsafePath = reviewed();
  unsafePath.distributions[0]!.releases[0]!.source.path = '../outside';
  variants.push(unsafePath);
  for (const candidate of variants) {
    assert.equal(projectCatalogSelection(candidate, ['codex']).resolution.distribution, undefined);
    assert.throws(() => projectCatalogCommands(candidate, ['codex']), /No eligible/);
  }
  assert.equal(projectCatalogSelection(reviewed(), ['cursor']).resolution.distribution, undefined);
  assert.equal(projectCatalogSelection(reviewed(), ['codex', 'cursor']).resolution.distribution, undefined);
  assert.equal(projectCatalogSelection(reviewed(), []).resolution.distribution, undefined);
});

test('the normal user-scope policy is unchanged when direct project preparation is offered', () => {
  const plugin = reviewed();
  assert.deepEqual(plugin.distributions[0]!.releases[0]!.targets.find(x => x.client === 'codex')!.scopes, ['user']);
  const { resolution } = projectCatalogSelection(plugin, ['codex']);
  assert.equal(resolution.distribution?.source.revision, plugin.source.revision);
});
