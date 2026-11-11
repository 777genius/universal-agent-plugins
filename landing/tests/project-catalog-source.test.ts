import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { pinnedPiProjectCommands, pinnedPiProjectSource, projectCatalogAutomaticSelection, projectCatalogCommands, projectCatalogSelection } from '../utils/projectCatalogSource.ts';
import type { RegistryPlugin } from '../types/registry.ts';

function reviewed(): RegistryPlugin {
  const fixture = JSON.parse(readFileSync(
    new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8',
  )) as { plugins: RegistryPlugin[] };
  return { ...structuredClone(fixture.plugins[0]!), trust_state: 'reviewed' };
}

test('published signed-v1 short name installs into explicit Project without v2 grants', () => {
  const plugin = reviewed();
  const original = structuredClone(plugin.distributions[0]!.releases[0]!.targets);
  const choice = projectCatalogSelection(plugin, ['codex']);
  assert.equal(choice.resolution.distribution?.id, plugin.default_distribution);
  assert.equal(choice.resolution.distribution?.targets.find(x => x.client === 'codex')?.delivery, 'prepared');
  assert.deepEqual(plugin.distributions[0]!.releases[0]!.targets, original,
    'Project preview must not alter signed User evidence');
  const commands = projectCatalogCommands(plugin, ['codex']);
  assert.equal(commands.add, 'npx -y universal-agent-plugins add gitlab --target codex --scope project');
  assert.equal(commands.update, 'npx -y universal-agent-plugins update gitlab --scope project');
  assert.equal(commands.repair, 'npx -y universal-agent-plugins repair gitlab --target codex --scope project');
  assert.equal(commands.remove, 'npx -y universal-agent-plugins remove gitlab --scope project');
});

test('automatic Project command defers compatible installed-agent selection to CLI', () => {
  const plugin = reviewed();
  assert.ok(projectCatalogAutomaticSelection(plugin)?.resolution.distribution);
  assert.equal(projectCatalogCommands(plugin).add,
    'npx -y universal-agent-plugins add gitlab --scope project');
});

test('Marketing Kit with signed-v1 user grants and null Git revision can use Project', () => {
  const plugin = reviewed();
  plugin.name = 'marketing-kit';
  plugin.install_source = 'marketing-kit';
  plugin.installable = true;
  const release = plugin.distributions[0]!.releases[0]!;
  release.source = {
    ...release.source, repository: '777genius/universal-agent-plugins-registry',
    revision: null, path: 'plugins/marketing-kit',
  };
  assert.ok(projectCatalogSelection(plugin, ['codex']).resolution.distribution);
  assert.equal(projectCatalogCommands(plugin, ['codex']).add,
    'npx -y universal-agent-plugins add marketing-kit --target codex --scope project');
  assert.equal(projectCatalogCommands(plugin).add,
    'npx -y universal-agent-plugins add marketing-kit --scope project');
});

test('an explicit project grant is compatible but never claims native execution', () => {
  const plugin = reviewed();
  plugin.distributions[0]!.releases[0]!.targets.find(x => x.client === 'codex')!.scopes = ['user', 'project'];
  const choice = projectCatalogSelection(plugin, ['codex']);
  assert.equal(choice.resolution.distribution?.targets.find(x => x.client === 'codex')?.delivery, 'prepared');
  assert.equal(projectCatalogCommands(plugin, ['codex']).add,
    'npx -y universal-agent-plugins add gitlab --target codex --scope project');
});

test('revoked, unsigned, malformed and unsupported sources still refuse Project', () => {
  const variants: RegistryPlugin[] = [];
  const unsigned = reviewed(); delete unsigned.trust_state; variants.push(unsigned);
  const untrusted = reviewed(); untrusted.installable = false; variants.push(untrusted);
  const badCommit = reviewed(); badCommit.distributions[0]!.releases[0]!.source.revision = 'wrong'; variants.push(badCommit);
  // Project admits partial components; only an entirely unsupported package is refused.
  const extension = reviewed(); extension.distributions[0]!.releases[0]!.components = ['extensions']; variants.push(extension);
  const revoked = reviewed(); revoked.distributions[0]!.releases[0]!.release_status = 'revoked'; variants.push(revoked);
  const unsafe = reviewed(); unsafe.distributions[0]!.releases[0]!.source.path = '../escape'; variants.push(unsafe);
  for (const item of variants) {
    assert.equal(projectCatalogSelection(item, ['codex']).resolution.distribution, undefined);
    assert.equal(projectCatalogAutomaticSelection(item), undefined);
    assert.throws(() => projectCatalogCommands(item, ['codex']), /No eligible/);
  }
  assert.equal(projectCatalogSelection(reviewed(), ['chatgpt']).resolution.distribution, undefined);
  assert.equal(projectCatalogSelection(reviewed(), []).resolution.distribution, undefined);
});

test('extra unsupported sibling does not revoke signed-v1 Project selection', () => {
  const plugin = reviewed();
  plugin.distributions[0]!.releases[0]!.components = ['skills', 'extensions'];
  const chosen = projectCatalogSelection(plugin, ['codex']);
  assert.ok(chosen.resolution.distribution);
  assert.equal(projectCatalogCommands(plugin, ['codex']).add,
    'npx -y universal-agent-plugins add gitlab --target codex --scope project');
});

test('signed-v2 canonical closure cannot fall back from User to ungranted Project', () => {
  const plugin = reviewed();
  const target = plugin.distributions[0]!.releases[0]!.targets.find(x => x.client === 'codex')!;
  target.scopes = ['user'];
  target.canonical_closure = { eligible: true, closure_digest: 'sha256:' + 'a'.repeat(64) };
  assert.equal(projectCatalogSelection(plugin, ['codex']).resolution.distribution, undefined);
  target.scopes = ['user', 'project'];
  assert.ok(projectCatalogSelection(plugin, ['codex']).resolution.distribution);
});

test('signed user source and original target scope are unchanged by selection', () => {
  const plugin = reviewed();
  assert.deepEqual(plugin.distributions[0]!.releases[0]!.targets.find(x => x.client === 'codex')!.scopes, ['user']);
  const result = projectCatalogSelection(plugin, ['codex']).resolution;
  assert.equal(result.distribution?.source.revision, plugin.source.revision);
});


test('Pi project uses an explicit immutable Git source instead of inventing a signed target', () => {
  const plugin = reviewed();
  const previous = structuredClone(plugin.distributions);
  assert.equal(projectCatalogSelection(plugin, ['pi']).resolution.distribution, undefined);
  const source = 'github:777genius/universal-agent-plugins@2ddbb99dd190c1792b79904f9875e6322bccd243//plugins/gitlab';
  assert.equal(pinnedPiProjectSource(plugin), source);
  const commands = pinnedPiProjectCommands(plugin);
  assert.equal(commands.add, `npx -y universal-agent-plugins add ${source} --target pi --scope project`);
  assert.equal(commands.update, `npx -y universal-agent-plugins add ${source} --target pi --scope project`);
  assert.equal(commands.repair, 'npx -y universal-agent-plugins repair gitlab --target pi --scope project');
  assert.equal(commands.remove, 'npx -y universal-agent-plugins remove gitlab --scope project');
  assert.deepEqual(plugin.distributions, previous, 'The signed target list must not be rewritten');
});

test('Pi exact-source route refuses unsound or unsupported signed-source projections', () => {
  const variants = [
    (p: RegistryPlugin) => { p.trust_state = undefined; },
    (p: RegistryPlugin) => { p.trust_state = 'conformant_unreviewed'; },
    (p: RegistryPlugin) => { p.installable = false; },
    (p: RegistryPlugin) => { p.distributions[0]!.releases[0]!.source.revision = null; },
    (p: RegistryPlugin) => { p.distributions[0]!.releases[0]!.source.path = '../escape'; },
    (p: RegistryPlugin) => { p.distributions[0]!.releases[0]!.source.tree_sha256 = 'sha256:bad'; },
    (p: RegistryPlugin) => { p.distributions[0]!.releases[0]!.source.manifest_sha256 = 'bad'; },
    (p: RegistryPlugin) => { p.distributions[0]!.releases[0]!.components = []; },
    (p: RegistryPlugin) => { p.distributions[0]!.releases[0]!.components = ['hooks' as never]; },
    (p: RegistryPlugin) => { p.distributions[0]!.releases[0]!.selectable = false; },
    (p: RegistryPlugin) => { p.distributions[0]!.releases[0]!.release_status = 'revoked'; },
    (p: RegistryPlugin) => { p.distributions[0]!.status = 'suspended'; },
    (p: RegistryPlugin) => { p.distributions[0]!.kind = 'upstream'; },
    (p: RegistryPlugin) => {
      p.distributions[0]!.releases[0]!.targets[0]!.canonical_closure = { eligible: true, closure_digest: 'sha256:' + 'a'.repeat(64) };
    },
  ];
  for (const change of variants) {
    const plugin = reviewed();
    change(plugin);
    assert.equal(pinnedPiProjectSource(plugin), undefined, String(change));
    assert.throws(() => pinnedPiProjectCommands(plugin), /No eligible Pi Project/);
  }
});

test('signed Pi Project grant keeps the original signed short-name route', () => {
  const plugin = reviewed();
  plugin.distributions[0]!.releases[0]!.targets.push({
    client: 'pi', authentication: 'required', delivery: 'managed', scopes: ['user', 'project'],
  });
  assert.ok(projectCatalogSelection(plugin, ['pi']).resolution.distribution);
  assert.equal(pinnedPiProjectSource(plugin), undefined);
  assert.equal(projectCatalogCommands(plugin, ['pi']).add,
    'npx -y universal-agent-plugins add gitlab --target pi --scope project');
});
