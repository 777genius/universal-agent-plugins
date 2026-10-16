import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import { pluginCommands } from '../utils/commands.ts';
import { applyCliInvocation } from '../utils/cliInvocation.ts';
import {
  dependencyAddFlag,
  isPinnedGitSource,
  dependencySaveMode,
  SAVED_DEPENDENCIES_CLI_RELEASED,
} from '../utils/savedDependencies.ts';
import type { RegistryPlugin } from '../types/registry.ts';
import type { PluginInstallTargetId } from '../types/content.ts';
import clientCapabilities from '../data/clientCapabilities.generated.json' with { type: 'json' };

const plugin: RegistryPlugin = JSON.parse(
  readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'),
).plugins[0];

test('production saves supported selections and retains explicit legacy command bytes', () => {
  assert.equal(SAVED_DEPENDENCIES_CLI_RELEASED, true);
  const published = pluginCommands(plugin, ['codex', 'cursor']);
  assert.equal(published.saveMode, 'saved');
  assert.equal(published.add, 'npx universal-agent-plugins add gitlab --target codex,cursor');
  assert.equal(published.update, 'npx universal-agent-plugins update gitlab');
  assert.equal(published.remove, 'npx universal-agent-plugins remove gitlab');
  for (const targets of [['gemini'], ['opencode'], ['cursor'], ['chatgpt'], ['chatgpt', 'codex']]) {
    const commands = pluginCommands(plugin, targets, false);
    const targetFlag = ` --target ${targets.join(',')}`;
    assert.equal(commands.add, `npx universal-agent-plugins add gitlab${targetFlag}`);
    assert.equal(commands.update, `npx universal-agent-plugins update gitlab${targetFlag}`);
    assert.equal(commands.remove, `npx universal-agent-plugins remove gitlab${targetFlag}`);
    assert.equal(commands.saveMode, undefined);
  }
  const commands = pluginCommands(plugin, ['codex', 'cursor'], false);
  assert.equal(commands.add, 'npx universal-agent-plugins add gitlab --target codex,cursor');
  assert.equal(commands.update, 'npx universal-agent-plugins update gitlab --target codex,cursor');
  assert.equal(commands.remove, 'npx universal-agent-plugins remove gitlab --target codex,cursor');
  assert.equal(commands.saveMode, undefined);
});

test('released generator saves supported remote selections and manages all owned targets', () => {
  for (const source of ['gitlab', `github:example/plugin@${'a'.repeat(40)}//plugins/demo`]) {
    const commands = pluginCommands(
      { ...plugin, install_source: source },
      ['claude', 'codex'],
      true,
    );
    assert.equal(commands.add, `npx universal-agent-plugins add ${source} --target claude,codex`);
    assert.equal(commands.saveMode, 'saved');
    assert.equal(
      commands.update,
      source === 'gitlab'
        ? 'npx universal-agent-plugins update gitlab'
        : `npx universal-agent-plugins add ${source} --target claude,codex`,
    );
    assert.equal(commands.remove, 'npx universal-agent-plugins remove gitlab');
  }
  assert.equal(pluginCommands(plugin, undefined, true).saveMode, 'saved');
  for (const targets of [['gemini'], ['opencode'], ['cursor'], ['codex', 'cursor']]) {
    const commands = pluginCommands(plugin, targets, true);
    assert.equal(commands.add, `npx universal-agent-plugins add gitlab --target ${targets.join(',')}`);
    assert.equal(commands.saveMode, 'saved');
    assert.equal(commands.update, 'npx universal-agent-plugins update gitlab');
    assert.equal(commands.remove, 'npx universal-agent-plugins remove gitlab');
  }
});

test('released generator explicitly opts unsupported and mixed remote selections out', () => {
  for (const targets of [['unknown'], ['vscode'], ['chatgpt', 'codex'], ['chatgpt', 'claude']]) {
    const commands = pluginCommands(plugin, targets, true);
    assert.equal(
      commands.add,
      `npx universal-agent-plugins add gitlab --target ${targets.join(',')} --no-save`,
    );
    assert.equal(commands.saveMode, 'noSave');
    assert.equal(
      commands.remove,
      `npx universal-agent-plugins remove gitlab --target ${targets.join(',')}`,
    );
  }
});

test('generated capabilities describe every landing install target', () => {
  // Exhaustiveness is checked by TypeScript when a new install lane is added.
  const targets: Record<PluginInstallTargetId, true> = {
    claude: true,
    codex: true,
    gemini: true,
    opencode: true,
    cursor: true,
  };
  for (const target of Object.keys(targets)) {
    const metadata = clientCapabilities.filter((client) => client.id === target);
    assert.equal(metadata.length, 1, `metadata for ${target}`);
    assert.equal(metadata[0].saved_dependency, true);
  }
  const chatgpt = clientCapabilities.find((client) => client.id === 'chatgpt');
  assert.equal(chatgpt?.saved_dependency, false);
  assert.equal(chatgpt?.excluded_reason, 'manual_preparation_only');
  const vscode = clientCapabilities.find((client) => client.id === 'vscode');
  assert.equal(vscode?.saved_dependency, false);
  assert.equal(vscode?.excluded_reason, 'dependency_management_deferred');
});

test('local, literal discovery, and ChatGPT-only keep imperative commands and truthful notices', () => {
  for (const [source, targets, mode] of [
    ['./plugin', ['codex'], 'local'],
    ['/tmp/plugin', ['codex'], 'local'],
    ['C:\\plugin', ['codex'], 'local'],
    ['discovery:example/demo', ['codex'], 'discovery'],
    ['gitlab', ['chatgpt'], 'chatgpt'],
  ] as const) {
    const commands = pluginCommands({ ...plugin, install_source: source }, targets, true);
    assert.equal(commands.saveMode, mode);
    assert.equal(
      commands.add,
      `npx universal-agent-plugins add ${source} --target ${targets.join(',')}`,
    );
  }
  assert.equal(dependencySaveMode('github:example/plugin@main', ['codex']), 'noSave');
});

test('catalog selection generator preserves released bytes and saves eligible user-scoped lanes', () => {
  const source = readFileSync(new URL('../data/pluginInstall.ts', import.meta.url), 'utf8')
    .replace(/^import[\s\S]*?;\n/gm, '')
    .replace(/^export /gm, '');
  const api = runInNewContext(
    stripTypeScriptTypes(source) +
      '\n({ getResolvedPluginInstallSpec, getPluginInstallSpec, buildInstallCommandForSelection, buildManageCommandsForSelection });',
    {
      contentByLocale: { en: { plugins: [{ slug: 'gitlab' }] } },
      SAVED_DEPENDENCIES_CLI_RELEASED,
      dependencySaveMode,
      dependencyAddFlag,
      isPinnedGitSource,
    },
  );
  const spec = api.getResolvedPluginInstallSpec('gitlab');
  assert.equal(spec.manageCommands.update, 'universal-agent-plugins update gitlab');
  // The alias may already own Claude and Codex while the UI selects only Codex.
  // Copying Update must advance the whole Directory alias, without a subset filter.
  assert.equal(
    api.buildManageCommandsForSelection(spec, ['codex'], true).update,
    'universal-agent-plugins update gitlab',
  );
  assert.equal(
    api.buildManageCommandsForSelection(spec, ['codex'], false).update,
    'universal-agent-plugins update gitlab --target codex',
  );

  assert.equal(
    api.getResolvedPluginInstallSpec('gitlab', true).manageCommands.update,
    'universal-agent-plugins update gitlab',
  );
  const component = readFileSync(
    new URL('../components/plugins/PluginInstallSection.vue', import.meta.url),
    'utf8',
  );
  const managementExpression = component.slice(
    component.indexOf('const renderedManageCommands ='),
    component.indexOf('const selectionScope ='),
  );
  const displayed = (released: boolean, installSpec = spec, selectedTargetIds = ['codex']) =>
    runInNewContext(managementExpression + '\nrenderedManageCommands', {
      computed: (read: () => unknown) => read(),
      props: { installSpec },
      selectedTargetIds: { value: selectedTargetIds },
      selectedCliInvocation: { value: 'npx universal-agent-plugins' },
      applyCliInvocation,
      buildManageCommandsForSelection: (current: unknown, targets: string[]) =>
        api.buildManageCommandsForSelection(current, targets, released),
    });
  for (const released of [false, true]) {
    const all = displayed(released, spec, []);
    for (const action of ['update', 'repair', 'remove'] as const) {
      assert.equal(
        all[action],
        released && action === 'update'
          ? 'npx universal-agent-plugins update gitlab'
          : `npx universal-agent-plugins ${action} gitlab --target claude,codex,gemini,opencode,cursor`,
      );
      assert.doesNotMatch(all[action], /--target(?:\s*$|\s+--)/);
    }
  }
  assert.equal(displayed(true).update, 'npx universal-agent-plugins update gitlab');
  assert.equal(displayed(false).update, 'npx universal-agent-plugins update gitlab --target codex');
  const exactSource = `github:example/demo@${'c'.repeat(40)}//plugins/demo`;
  api.getPluginInstallSpec('gitlab').cliSource = exactSource;
  const gitSpec = api.getResolvedPluginInstallSpec('gitlab', true);
  assert.equal(
    displayed(true, gitSpec, []).update,
    `npx universal-agent-plugins add ${exactSource} --target claude,codex,gemini,opencode,cursor`,
  );

  assert.equal(
    displayed(true, gitSpec).update,
    `npx universal-agent-plugins add ${exactSource} --target codex`,
  );
  assert.equal(
    api.buildManageCommandsForSelection(gitSpec, ['codex'], true).update,
    `universal-agent-plugins add ${exactSource} --target codex`,
  );

  // Resolved specs can outlive the gate/selection that produced their base commands.
  // Derive future commands from the current selection rather than those stale bases.
  assert.equal(
    displayed(true, gitSpec, ['codex', 'cursor']).update,
    `npx universal-agent-plugins add ${exactSource} --target codex,cursor`,
  );
  const legacyGitSpec = api.getResolvedPluginInstallSpec('gitlab', false);
  assert.equal(
    displayed(true, legacyGitSpec, ['codex']).update,
    `npx universal-agent-plugins add ${exactSource} --target codex`,
  );
  assert.equal(
    displayed(false, legacyGitSpec, []).update,
    'npx universal-agent-plugins update gitlab --target claude,codex,gemini,opencode,cursor',
  );
  assert.equal(gitSpec.manageCommands.update, `universal-agent-plugins add ${exactSource}`);
  assert.equal(
    gitSpec.supportedTargets.find((lane: { targetId: string }) => lane.targetId === 'codex')
      .commands.update,
    `universal-agent-plugins add ${exactSource} --target codex`,
  );
  assert.equal(
    api.getResolvedPluginInstallSpec('gitlab', false).manageCommands.update,
    'universal-agent-plugins update gitlab',
  );
  api.getPluginInstallSpec('gitlab').cliSource = 'gitlab';

  for (const target of ['opencode', 'cursor']) {
    const lane = spec.supportedTargets.find(
      (lane: { targetId: string }) => lane.targetId === target,
    );
    assert.equal(lane.scope, 'user');
    assert.equal(lane.commands.install, `universal-agent-plugins add gitlab --target ${target}`);
  }
  assert.equal(
    api.buildInstallCommandForSelection(spec, ['claude', 'codex'], false),
    'universal-agent-plugins add gitlab --target claude,codex',
  );
  assert.equal(
    api.buildInstallCommandForSelection(spec, ['claude', 'codex'], true),
    'universal-agent-plugins add gitlab --target claude,codex',
  );
  assert.equal(
    api.buildInstallCommandForSelection(spec, ['codex', 'cursor'], true),
    'universal-agent-plugins add gitlab --target codex,cursor',
  );
  assert.equal(
    api.buildInstallCommandForSelection(spec, [], true),
    'universal-agent-plugins add gitlab --target claude,codex,gemini,opencode,cursor',
  );
});

test('saved community Git update uses the currently displayed exact selector and selected target', () => {
  const current = {
    ...plugin,
    trust_state: 'conformant_unreviewed' as const,
    source: {
      ...plugin.source,
      repository: 'example/demo',
      revision: 'b'.repeat(40),
      path: 'plugins/demo',
    },
  };
  const command = pluginCommands(current, ['codex'], true).update;
  assert.equal(
    command,
    `npx universal-agent-plugins add github:example/demo@${'b'.repeat(40)}//plugins/demo --target codex`,
  );
  assert.equal(
    pluginCommands(current, ['codex'], false).update,
    'npx universal-agent-plugins update gitlab --target codex',
  );
});
