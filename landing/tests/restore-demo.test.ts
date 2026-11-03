import assert from 'node:assert/strict';
import { test } from 'node:test';
import { restoreDemoCommands } from '../data/restoreDemo.ts';
import { tomlExampleTokens } from '../utils/tomlExampleTokens.ts';
import { marketingKitUserManifest } from '../data/marketingKitDemo.ts';

test('Project and Global keep distinct real CLI verbs and explicit project creation', () => {
  assert.equal(restoreDemoCommands.project.install,
    'npx -y universal-agent-plugins install marketing-kit --scope project');
  assert.equal(restoreDemoCommands.project.restore, 'npx -y universal-agent-plugins install');
  assert.equal(restoreDemoCommands.project.export, '');
  assert.equal(restoreDemoCommands.global.install, 'npx -y universal-agent-plugins install marketing-kit');
  assert.equal(restoreDemoCommands.global.export, 'npx -y universal-agent-plugins deps export');
  assert.equal(restoreDemoCommands.global.restore, 'npx -y universal-agent-plugins deps import --install');
  for (const mode of Object.values(restoreDemoCommands)) {
    for (const command of Object.values(mode)) assert.doesNotMatch(command, /marketing-kit@|--scope user|--yes/);
  }
});

test('TOML highlighting is lossless and distinguishes keys, sections, strings and versions', () => {
  const tokens = tomlExampleTokens(marketingKitUserManifest);
  assert.equal(tokens.map((token) => token.text).join(''), marketingKitUserManifest);
  assert.equal(tokens.find((token) => token.text === '[dependencies]')?.kind, 'section');
  assert.equal(tokens.find((token) => token.text === 'schema_version')?.kind, 'key');
  assert.equal(tokens.find((token) => token.text === '"0.1.0"')?.kind, 'number');
  assert.equal(tokens.find((token) => token.text === '"777genius/marketing-kit"')?.kind, 'string');
  for (const source of ['', '# comment\n', 'key = "<img src=x onerror=alert(1)>"', 'key = "a\\"b"']) {
    assert.equal(tomlExampleTokens(source).map((token) => token.text).join(''), source);
  }
});
