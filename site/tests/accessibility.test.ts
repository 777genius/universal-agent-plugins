import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'

function source(relative: string): string {
  return readFileSync(fileURLToPath(new URL(relative, import.meta.url)), 'utf8')
}

describe('focused catalog accessibility contract', () => {
  it('uses a named checkbox multiselect with keyboard-native controls', () => {
    const component = source('../components/AppMultiSelect.vue')
    assert.match(component, /role="group" :aria-label="label"/)
    assert.match(component, /role="checkbox"/)
    assert.match(component, /:aria-checked=/)
    assert.match(component, /type="button"/)
    assert.match(component, /:disabled="option\.disabled/)
  })

  it('keeps local client logos paired with accessible text and explains disabled ChatGPT', () => {
    const panel = source('../components/InstallPanel.vue')
    const multiselect = source('../components/AppMultiSelect.vue')
    assert.match(panel, /client-icons\/\$\{client\.icon\}/)
    assert.match(panel, /Unavailable: no registered app binding/)
    assert.match(multiselect, /<span>\{\{ option\.label \}\}<\/span>/)
    assert.match(multiselect, /<img v-if="option\.icon"[^>]+alt=""/)
  })

  it('places the target selector beside the exact generated command', () => {
    const home = source('../pages/index.vue')
    const card = source('../components/PluginCard.vue')
    const layout = source('../layouts/default.vue')
    assert.match(home, /class="hero-command-row"[\s\S]*AppMultiSelect[\s\S]*CommandSnippet/)
    assert.match(card, /class="plugin-card__install"[\s\S]*AppMultiSelect[\s\S]*CommandSnippet/)
    assert.match(layout, /Pull request preview/)
  })

  it('exposes both Add a plugin pull-request actions', () => {
    const catalog = source('../components/PluginCatalog.vue')
    assert.equal([...catalog.matchAll(/registry\/README\.md#submit-an-external-package/g)].length, 2)
    assert.match(catalog, /Add a plugin by pull request/)
  })

  it('renders contributor copy as text under a restrictive static CSP', () => {
    const config = source('../nuxt.config.ts')
    const components = [source('../components/PluginCard.vue'), source('../pages/plugins/[slug].vue')].join('\n')
    assert.match(config, /contentSecurityPolicy = "default-src 'self'/)
    assert.match(config, /object-src 'none'/)
    assert.doesNotMatch(components, /v-html/)
  })
})
