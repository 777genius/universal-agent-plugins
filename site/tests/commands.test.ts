import { readFileSync } from 'node:fs'
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'
import { pluginCommands } from '../utils/commands.ts'
import { parseRegistryIndex } from '../utils/registry.ts'

const fixture = JSON.parse(readFileSync(fileURLToPath(new URL('./fixtures/registry.valid.json', import.meta.url)), 'utf8')) as unknown
const plugins = parseRegistryIndex(fixture).plugins

describe('command generation', () => {
  it('generates the exact built-in lifecycle commands', () => {
    assert.deepEqual(pluginCommands(plugins[0]!, 'cursor'), {
      add: 'npx universal-agent-plugins add context7 --target cursor',
      update: 'npx universal-agent-plugins update context7 --target cursor',
      repair: 'npx universal-agent-plugins repair context7 --target cursor',
      switch: 'npx universal-agent-plugins switch context7 --to <distribution-id>',
      remove: 'npx universal-agent-plugins remove context7 --target cursor',
    })
  })

  it('keeps the exact pinned source for external add and the installed name thereafter', () => {
    const source = 'example/plugins@0123456789abcdef0123456789abcdef01234567//plugins/example'
    assert.deepEqual(pluginCommands(plugins[1]!, 'copilot'), {
      add: `npx universal-agent-plugins add ${source} --target copilot`,
      update: 'npx universal-agent-plugins update example-external --target copilot',
      repair: 'npx universal-agent-plugins repair example-external --target copilot',
      switch: 'npx universal-agent-plugins switch example-external --to <distribution-id>',
      remove: 'npx universal-agent-plugins remove example-external --target copilot',
    })
  })

  it('generates one command for multiple selected agents', () => {
    assert.deepEqual(pluginCommands(plugins[0]!, ['codex', 'cursor', 'codex']), {
      add: 'npx universal-agent-plugins add context7 --target codex,cursor',
      update: 'npx universal-agent-plugins update context7 --target codex,cursor',
      repair: 'npx universal-agent-plugins repair context7 --target codex,cursor',
      switch: 'npx universal-agent-plugins switch context7 --to <distribution-id>',
      remove: 'npx universal-agent-plugins remove context7 --target codex,cursor',
    })
  })

  it('requires at least one selected agent', () => {
    assert.throws(() => pluginCommands(plugins[0]!, []), /At least one target/)
  })
})
