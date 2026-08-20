import { readFileSync } from 'node:fs'
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'
import { availableFilters, filterPlugins } from '../utils/filter.ts'
import { parseRegistryIndex } from '../utils/registry.ts'

const fixture = JSON.parse(readFileSync(fileURLToPath(new URL('./fixtures/registry.valid.json', import.meta.url)), 'utf8')) as unknown
const plugins = parseRegistryIndex(fixture).plugins

describe('catalog filtering', () => {
  it('searches names, descriptions, authors, keywords, and components case-insensitively', () => {
    assert.deepEqual(filterPlugins(plugins, { query: 'UPSTASH' }).map(plugin => plugin.name), ['context7'])
    assert.deepEqual(filterPlugins(plugins, { query: 'skills' }).map(plugin => plugin.name), ['example-external'])
    assert.deepEqual(filterPlugins(plugins, { query: 'version-specific' }).map(plugin => plugin.name), ['context7'])
  })

  it('combines category, component, and source filters', () => {
    assert.deepEqual(filterPlugins(plugins, { category: 'documentation', component: 'mcp', source: 'community' }), [plugins[0]])
    assert.deepEqual(filterPlugins(plugins, { source: 'direct' }), [plugins[1]])
  })

  it('derives stable filter options from registry data', () => {
    assert.deepEqual(availableFilters(plugins), {
      categories: ['development', 'documentation'],
      components: ['mcp', 'skills'],
    })
  })
})
