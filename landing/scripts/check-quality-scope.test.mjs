import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { ESLint } from 'eslint'
import { assertEffectiveProtectedRules, assertRoutes, classifySourcePath } from './check-quality-scope.mjs'

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const manifest = JSON.parse(readFileSync(path.join(packageRoot, 'package.json'), 'utf8'))

test('landing source classification rejects an unknown owner', () => {
  assert.equal(classifySourcePath('landing/components/App.vue'), 'production')
  assert.equal(classifySourcePath('landing/.eslintrc.cjs'), 'tooling')
  assert.equal(classifySourcePath('landing/tests/registry.test.ts'), 'test')
  assert.equal(classifySourcePath('landing/new-owner/source.tsx'), null)
})

test('actual landing route rejects bypasses and missing commands', () => {
  assert.doesNotThrow(() => assertRoutes(manifest))
  const mutate = (name, value) => ({ ...manifest, scripts: { ...manifest.scripts, [name]: value } })
  assert.throws(() => assertRoutes(mutate('lint', 'pnpm run lint:source')), /source admission/u)
  assert.throws(() => assertRoutes(mutate('quality:scope', 'node -e "process.exit(0)"')), /actual admission/u)
  assert.throws(() => assertRoutes(mutate('lint', 'true && pnpm run quality:scope && pnpm run lint:source')), /source admission/u)
  assert.throws(() => assertRoutes(mutate('lint', 'pnpm run quality:scope && pnpm run lint:source # disabled')), /real ESLint route/u)
  assert.throws(() => assertRoutes(mutate('lint:source', `${manifest.scripts['lint:source']} || true`)), /fail-closed/u)
  assert.throws(() => assertRoutes(mutate('lint:source', `${manifest.scripts['lint:source']}; true`)), /fail-closed/u)
  assert.throws(() => assertRoutes(mutate('lint:source', `if test -n "$CI"; then ${manifest.scripts['lint:source']}; fi`)), /selection mode/u)
  assert.throws(() => assertRoutes(mutate('lint:source', `${manifest.scripts['lint:source']} --rule no-eval:off`)), /rule overrides/u)
  assert.throws(() => assertRoutes(mutate('lint:source', manifest.scripts['lint:source'].replace('eslint ', 'eslint --no-config-lookup '))), /only source targets/u)
})

test('effective policy rejects a later override fixture', async () => {
  const fixture = new ESLint({
    cwd: packageRoot,
    overrideConfigFile: true,
    overrideConfig: [
      { files: ['**/*.ts'], rules: { 'no-eval': 'error', 'no-implied-eval': 'error', 'no-new-func': 'error' } },
      { files: ['**/*.ts'], rules: { 'no-eval': 'off' } },
    ],
  })
  const effective = await fixture.calculateConfigForFile(path.join(packageRoot, 'build/load-registry.ts'))
  assert.throws(() => assertEffectiveProtectedRules(effective, 'build/load-registry.ts'), /not error/u)
})
