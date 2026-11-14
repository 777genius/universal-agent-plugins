import assert from 'node:assert/strict';
import test from 'node:test';
import { hasNpxUapInvocation, withNativeCliInvocation } from '../utils/cliInvocation.ts';

const projectSkill =
  'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design --scope project';

test('native CLI option changes only the executable and preserves the Skill arguments', () => {
  assert.equal(hasNpxUapInvocation(projectSkill), true);
  assert.equal(withNativeCliInvocation(projectSkill, false), projectSkill);
  assert.equal(withNativeCliInvocation(projectSkill, true),
    'agentplugins skills add https://github.com/anthropics/skills --skill frontend-design --scope project');
});

test('native CLI option handles the Directory CLI and a command inside a Skill prompt', () => {
  const plugin = 'npx -y universal-agent-plugins add context7 --target codex --scope project';
  assert.equal(withNativeCliInvocation(plugin, true),
    'agentplugins add context7 --target codex --scope project');
  assert.equal(withNativeCliInvocation('npx --yes universal-agent-plugins@0.1.108 repair context7', true),
    'agentplugins repair context7');

  const prompt = 'Check the source before running:\n' + projectSkill;
  assert.equal(hasNpxUapInvocation(prompt), true);
  assert.equal(withNativeCliInvocation(prompt, true),
    'Check the source before running:\nagentplugins skills add https://github.com/anthropics/skills --skill frontend-design --scope project');
});

test('does not rewrite unrelated npx calls, vendor installers or arbitrary text', () => {
  for (const command of [
    'npx other-tool@latest add context7',
    'npx universal-agent-plugins-fake skills add context7',
    'npm install -g agentplugins',
    'curl -fsSL https://example.com/install.sh | sh',
    'echo npx universal-agent-plugins@latest add context7',
    'agentplugins add context7',
  ]) {
    assert.equal(hasNpxUapInvocation(command), false, command);
    assert.equal(withNativeCliInvocation(command, true), command);
  }
});
