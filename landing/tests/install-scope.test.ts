import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { installCommands, pluginCommands } from '../utils/commands.ts';
import { resolveDistribution } from '../utils/registry.ts';
import type { DistributionView, RegistryPlugin } from '../types/registry.ts';

const fixture: RegistryPlugin = JSON.parse(
  readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'),
).plugins[0];
function distribution(
  id: string,
  scopes: string[],
  targets = ['codex', 'cursor'],
): DistributionView {
  const result = structuredClone(fixture.distributions[0]!);
  result.id = id;
  result.kind = 'community';
  result.status = 'active';
  const release = result.releases[0]!;
  release.release_sequence = 1;
  release.release_status = 'active';
  release.meets_minimum_capabilities = true;
  release.blocking_clients = [];
  release.targets = targets.map((client) => ({
    client: client as 'codex' | 'cursor',
    scopes,
    authentication: 'not_required',
    delivery: 'prepared',
  }));
  result.releases = [release];
  return result;
}
function plugin(...distributions: DistributionView[]): RegistryPlugin {
  return {
    ...structuredClone(fixture),
    distributions,
    declared_default_distribution: distributions[0]!.id,
  };
}

test('every project lifecycle command carries scope and owned management retains whole-alias semantics', () => {
  for (const targets of [['codex'], ['claude', 'cursor']]) {
    const commands = pluginCommands(fixture, targets, true, 'project');
    assert.equal(
      commands.add,
      `npx -y universal-agent-plugins add gitlab --target ${targets.join(',')} --scope project`,
    );
    assert.equal(commands.update, 'npx -y universal-agent-plugins update gitlab --scope project');
    assert.equal(
      commands.repair,
      `npx -y universal-agent-plugins repair gitlab --target ${targets.join(',')} --scope project`,
    );
    assert.equal(commands.remove, 'npx -y universal-agent-plugins remove gitlab --scope project');
    assert.equal(
      commands.switch,
      'npx -y universal-agent-plugins switch gitlab --to <distribution-id> --scope project',
    );
    const imperative = pluginCommands(fixture, targets, false, 'project');
    for (const action of ['update', 'repair', 'remove'] as const) {
      assert.equal(
        imperative[action],
        `npx -y universal-agent-plugins ${action} gitlab --target ${targets.join(',')} --scope project`,
      );
    }
  }
  const source = `github:example/plugin@${'a'.repeat(40)}//plugins/demo`;
  const git = installCommands(source, 'demo', ['codex'], true, 'project', 'agentplugins');
  assert.equal(git.add, `agentplugins add ${source} --target codex --scope project`);
  assert.equal(git.update, git.add);
  assert.equal(git.remove, 'agentplugins remove demo --scope project');
  assert.equal(installCommands('./local', 'local', ['codex'], true, 'project').saveMode, 'local');
  assert.match(
    installCommands('gitlab', 'gitlab', ['unknown'], true, 'project').add,
    / --scope project --no-save$/,
  );
  assert.throws(() => pluginCommands(fixture, undefined, true, 'project'), /explicit targets/);
  assert.throws(() => installCommands('gitlab', 'gitlab', [], true, 'project'), /target/);
});

test('project community re-add retains exact validated source and every scope suffix', () => {
  const community: RegistryPlugin = {
    ...structuredClone(fixture),
    trust_state: 'conformant_unreviewed',
    source: {
      ...fixture.source,
      repository: 'example/demo',
      revision: 'b'.repeat(40),
      path: 'plugin with spaces',
    },
  };
  const commands = pluginCommands(community, ['codex'], true, 'project');
  assert.equal(
    commands.add,
    `npx -y universal-agent-plugins add 'github:example/demo@${'b'.repeat(40)}//plugin with spaces' --target codex --scope project`,
  );
  assert.equal(commands.update, commands.add);
  assert.equal(community.install_source, fixture.install_source);
  assert.doesNotMatch(pluginCommands(fixture, ['codex'], false).add, /--scope|--no-save/);
});

test('scope chooses one eligible release and can fall back without borrowing default delivery or components', () => {
  const user = distribution('default', ['user']);
  const project = distribution('project', ['project']);
  project.releases[0]!.components = ['skills'];
  const selected = plugin(user, project);
  assert.equal(
    resolveDistribution(selected, ['codex', 'cursor'], 'user').distribution?.id,
    'default',
  );
  const resolved = resolveDistribution(selected, ['codex', 'cursor'], 'project');
  assert.equal(resolved.distribution?.id, 'project');
  assert.deepEqual(resolved.distribution?.components, ['skills']);
  assert.equal(resolved.distribution?.targets[0]?.delivery, 'prepared');
  assert.match(resolved.fallback_reason!, /does not support project scope/);
  const missing = resolveDistribution(plugin(user), ['codex'], 'project');
  assert.equal(missing.distribution, undefined);
  assert.match(missing.unavailable_reason!, /project scope for codex/);
  // Existing target-only callers retain their original contract.
  assert.equal(resolveDistribution(plugin(project), ['codex']).distribution?.id, 'project');
});

test('multiple clients and scope must coexist in the same active release', () => {
  const first = distribution('default', ['project'], ['codex']);
  const older = structuredClone(first.releases[0]!);
  older.targets = [{ ...older.targets[0]!, client: 'cursor' }];
  older.release_sequence = 0;
  first.releases.push(older);
  assert.equal(
    resolveDistribution(plugin(first), ['codex', 'cursor'], 'project').distribution,
    undefined,
  );
  assert.equal(
    resolveDistribution(plugin(first), ['cursor'], 'project').distribution?.release_sequence,
    0,
  );
  const fallback = distribution('fallback', ['project'], ['cursor']);
  assert.equal(
    resolveDistribution(plugin(first, fallback), ['codex', 'cursor'], 'project').distribution,
    undefined,
  );
  for (const mutate of [
    (d: DistributionView) => {
      d.status = 'suspended';
    },
    (d: DistributionView) => {
      d.releases[0]!.release_status = 'revoked';
    },
    (d: DistributionView) => {
      d.releases[0]!.blocking_clients = ['codex'];
    },
    (d: DistributionView) => {
      d.kind = 'upstream';
      d.releases[0]!.materialized_clients = [];
    },
    (d: DistributionView) => {
      d.releases[0]!.targets[0]!.scopes = [];
    },
  ]) {
    const d = distribution('default', ['project']);
    mutate(d);
    assert.equal(resolveDistribution(plugin(d), ['codex'], 'project').distribution, undefined);
  }
  const chatgpt = distribution('default', ['user'], ['chatgpt']);
  assert.equal(
    resolveDistribution(plugin(chatgpt), ['chatgpt'], 'project').distribution,
    undefined,
  );
});
