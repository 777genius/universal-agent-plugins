import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import type { RegistryPlugin } from '../types/registry.ts';
import { pluginCommands } from '../utils/commands.ts';

const reviewed: RegistryPlugin = JSON.parse(
  readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'),
).plugins[0];
const revision = 'a'.repeat(40);
function community(path = ''): RegistryPlugin {
  return {
    ...structuredClone(reviewed),
    trust_state: 'conformant_unreviewed',
    install_source: 'discovery:example/plugin',
    source: { ...reviewed.source, repository: 'example/plugin', revision, path },
  };
}

test('community ADD pins the trusted source commit for root and nested packages without changing identity or maintenance commands', () => {
  for (const path of ['', 'plugins/nested']) {
    const plugin = community(path);
    const identity = plugin.install_source;
    const commands = pluginCommands(plugin, ['codex', 'cursor']);
    assert.equal(
      commands.add,
      `npx universal-agent-plugins add github:example/plugin@${revision}${path ? `//${path}` : ''} --target codex,cursor`,
    );
    assert.equal(plugin.install_source, identity);
    for (const action of ['update', 'repair', 'remove'] as const)
      assert.equal(
        commands[action],
        `npx universal-agent-plugins ${action} ${plugin.name} --target codex,cursor`,
      );
    assert.equal(
      commands.switch,
      `npx universal-agent-plugins switch ${plugin.name} --to <distribution-id>`,
    );
  }
  assert.equal(
    pluginCommands(reviewed).add,
    `npx universal-agent-plugins add ${reviewed.install_source}`,
  );
});

test('community ADD rejects mutable, absent, abbreviated, or uppercase commit revisions', () => {
  for (const invalid of [null, 'main', 'latest', 'a'.repeat(39), 'A'.repeat(40)]) {
    const plugin = community();
    plugin.source.revision = invalid;
    assert.throws(() => pluginCommands(plugin), /commit|revision/i);
  }
});

test('community ADD fails closed for missing or malformed source fields', () => {
  const valid = community().source;
  for (const source of [
    undefined,
    {},
    { ...valid, repository: 'not-a-repository' },
    { ...valid, path: null },
    { ...valid, path: '/absolute' },
    { ...valid, path: '../escape' },
    { ...valid, path: 'plugin\0hidden' },
    { ...valid, path: 'plugin\nextra' },
  ]) {
    const plugin = community();
    Reflect.set(plugin, 'source', source);
    assert.throws(() => pluginCommands(plugin), /Community source/);
  }
});

test('community selector remains one literal shell argument with spaces, quotes, dollars, and command substitution', () => {
  const path = "skills/team's notes/$UAP_INJECTION_SENTINEL/$(printf INJECTED); printf ESCAPED";
  const command = pluginCommands(community(path), ['codex']).add;
  // Replace npx with an argument printer. No CLI, installer, project mutation,
  // or provider runs; shell parsing itself is the independent contract.
  const shell = spawnSync('/bin/sh', ['-c', `npx() { printf '%s\\0' "$@"; }\n${command}`], {
    encoding: 'utf8',
    env: { ...process.env, UAP_INJECTION_SENTINEL: 'EXPANDED' },
  });
  assert.equal(shell.status, 0, shell.stderr);
  assert.deepEqual(shell.stdout.split('\0').slice(0, -1), [
    'universal-agent-plugins',
    'add',
    `github:example/plugin@${revision}//${path}`,
    '--target',
    'codex',
  ]);
});
