import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { assertLintPolicy, assertRoutes, classifySourcePath } from './check-source-scope.mjs'

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const manifest = JSON.parse(readFileSync(path.join(packageRoot, 'package.json'), 'utf8'))
const config = JSON.parse(readFileSync(path.join(packageRoot, 'oxlint.json'), 'utf8'))

test('website source classification covers theme, tooling, and tests', () => {
  assert.equal(classifySourcePath('website/.vitepress/config/index.ts'), 'production')
  assert.equal(classifySourcePath('website/tools/generate.mjs'), 'tooling')
  assert.equal(classifySourcePath('website/tools/lib/frontmatter.test.mjs'), 'test')
  assert.equal(classifySourcePath('website/new-owner/source.tsx'), null)
})

test('actual website route rejects missing, no-op, and truncated composition', () => {
  assert.doesNotThrow(() => assertRoutes(manifest))
  assert.throws(() => assertRoutes({ ...manifest, scripts: { ...manifest.scripts, 'quality:scope': 'node -e "process.exit(0)"' } }), /exact source admission/u)
  assert.throws(() => assertRoutes({ ...manifest, scripts: { ...manifest.scripts, 'quality:check': 'node -e "process.exit(0)"' } }), /source admission/u)
  assert.throws(() => assertRoutes({ ...manifest, scripts: { ...manifest.scripts, 'docs:check': 'pnpm run docs:test' } }), /downstream docs gate/u)
  assert.throws(() => assertRoutes({ ...manifest, scripts: { ...manifest.scripts, 'docs:check': `if test -n "$CI"; then ${manifest.scripts['docs:check']}; fi` } }), /downstream docs gate/u)
  assert.throws(() => assertRoutes({ ...manifest, scripts: { ...manifest.scripts, 'docs:check': `${manifest.scripts['docs:check']} # disabled` } }), /downstream docs gate/u)
  assert.throws(() => assertRoutes({ ...manifest, scripts: { ...manifest.scripts, 'docs:check': 'pnpm run quality:check && true' } }), /downstream docs gate/u)
  assert.throws(() => assertRoutes({ ...manifest, scripts: { ...manifest.scripts, 'docs:check': manifest.scripts['docs:check'].replace(' && pnpm run docs:check-output', '') } }), /downstream docs gate/u)
  assert.throws(() => assertRoutes({ ...manifest, scripts: { ...manifest.scripts, 'quality:lint': manifest.scripts['quality:lint'].replace(' ./.vitepress', '') } }), /source universe/u)
})

test('website policy rejects a disabled protected rule', () => {
  assert.doesNotThrow(() => assertLintPolicy(config))
  assert.throws(() => assertLintPolicy({ ...config, rules: { ...config.rules, 'no-new-func': 'off' } }), /protected rule/u)
  assert.throws(() => assertLintPolicy({ ...config, ignorePatterns: ['tools/**'] }), /cannot add overrides/u)
  assert.throws(() => assertLintPolicy({ ...config, overrides: [] }), /cannot add overrides/u)
})
