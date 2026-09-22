import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const repositoryRoot = path.resolve(packageRoot, '..')
const SOURCE_PATTERN = /\.(?:[cm]?[jt]sx?|vue)$/u
const PROTECTED_RULES = ['no-eval', 'no-implied-eval', 'no-new-func']
const DOCS_CHECK = 'pnpm run quality:check && pnpm run docs:test && pnpm run docs:gen && pnpm run docs:check-locale && pnpm run docs:check-model && pnpm run docs:check-public-boundary && pnpm run docs:check-drift && vitepress build && node ./tools/postbuild.mjs && pnpm run docs:check-output'

export function classifySourcePath(relativePath) {
  const normalized = relativePath.replaceAll('\\', '/').replace(/^website\//u, '')
  if (!SOURCE_PATTERN.test(normalized)) return 'non-source'
  if (normalized.endsWith('.test.mjs')) return 'test'
  if (normalized.startsWith('.vitepress/')) return 'production'
  if (normalized.startsWith('tools/')) return 'tooling'
  return null
}

export function assertRoutes(manifest) {
  if (manifest.packageManager !== 'pnpm@8.15.1') throw new Error('website packageManager changed without a reviewed migration')
  if (manifest.devDependencies?.oxlint !== '1.85.0') throw new Error('website Oxlint must stay on the reviewed exact version')
  if (manifest.scripts?.['quality:scope'] !== 'node ./tools/quality/check-source-scope.mjs') throw new Error('quality:scope must execute the exact source admission command')
  if (manifest.scripts?.['quality:lint'] !== 'oxlint --config ./oxlint.json --deny-warnings --disable-nested-config --vue-plugin ./.vitepress ./tools') throw new Error('quality lint source universe changed')
  if (manifest.scripts?.['quality:check'] !== 'pnpm run quality:scope && pnpm run quality:lint') throw new Error('quality:check must retain source admission and lint')
  if (manifest.scripts?.['docs:check'] !== DOCS_CHECK) throw new Error('docs:check must retain quality:check and every downstream docs gate')
}

export function assertLintPolicy(config) {
  assert.deepEqual(Object.keys(config).sort(), ['$schema', 'categories', 'rules'], 'Oxlint config cannot add overrides or ignore surfaces')
  if (config.categories?.correctness !== 'off' || config.categories?.suspicious !== 'off') throw new Error('broad scan must stay limited to protected rules')
  for (const name of PROTECTED_RULES) if (config.rules?.[name] !== 'error') throw new Error(`protected rule disabled: ${name}`)
}

function trackedSources() {
  const output = execFileSync('git', ['ls-files', '-z', '--', 'website'], { cwd: repositoryRoot, encoding: 'utf8' })
  return output.split('\0').filter(Boolean).filter((item) => SOURCE_PATTERN.test(item))
}

function verify() {
  const manifest = JSON.parse(readFileSync(path.join(packageRoot, 'package.json'), 'utf8'))
  const config = JSON.parse(readFileSync(path.join(packageRoot, 'oxlint.json'), 'utf8'))
  assertRoutes(manifest)
  assertLintPolicy(config)
  const sources = trackedSources()
  const unclassified = sources.filter((item) => classifySourcePath(item) === null)
  if (unclassified.length > 0) throw new Error(`unclassified website source:\n${unclassified.join('\n')}`)
  const counts = Object.create(null)
  for (const item of sources) counts[classifySourcePath(item)] = (counts[classifySourcePath(item)] ?? 0) + 1
  process.stdout.write(`website quality scope verified: ${JSON.stringify(counts)}\n`)
}

if (process.argv[1] === fileURLToPath(import.meta.url)) verify()
