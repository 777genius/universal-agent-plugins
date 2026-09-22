import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { ESLint } from 'eslint'

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const repositoryRoot = path.resolve(packageRoot, '..')
const SOURCE_PATTERN = /\.(?:[cm]?[jt]sx?|vue)$/u
const PROTECTED_RULES = ['no-eval', 'no-implied-eval', 'no-new-func']
const productionRoots = ['components/', 'composables/', 'data/', 'layouts/', 'pages/', 'plugins/', 'server/', 'stores/', 'types/', 'utils/']
const toolingRoots = ['build/', 'scripts/']
const toolingFiles = new Set(['.eslintrc.cjs', 'eslint.config.mjs', 'i18n.config.ts', 'nuxt.config.ts', 'playwright.config.ts'])

export function classifySourcePath(relativePath) {
  const normalized = relativePath.replaceAll('\\', '/').replace(/^landing\//u, '')
  if (!SOURCE_PATTERN.test(normalized)) return 'non-source'
  if (normalized.startsWith('tests/') || normalized.endsWith('.test.mjs')) return 'test'
  if (normalized === 'app.vue' || normalized === 'error.vue' || productionRoots.some((root) => normalized.startsWith(root))) return 'production'
  if (toolingFiles.has(normalized) || toolingRoots.some((root) => normalized.startsWith(root))) return 'tooling'
  return null
}

export function parseLintRoute(command) {
  if (typeof command !== 'string' || /(?:\|\||&&\s*true\b|;\s*true\b|--rule(?:=|\s))/u.test(command)) {
    throw new Error('lint:source must be a fail-closed ESLint command without rule overrides')
  }
  const tokens = [...command.matchAll(/"([^"]*)"|'([^']*)'|(\S+)/gu)].map((match) => match[1] ?? match[2] ?? match[3])
  if (tokens[0] !== 'eslint' || tokens.at(-1) !== '--no-error-on-unmatched-pattern') {
    throw new Error('lint:source must execute ESLint with the reviewed selection mode')
  }
  const targets = tokens.slice(1, -1)
  if (targets.length === 0 || targets.some((target) => target.startsWith('-'))) {
    throw new Error('lint:source must provide only source targets before the reviewed ESLint option')
  }
  return targets
}

export function assertRoutes(manifest) {
  if (manifest.packageManager !== 'pnpm@12.3.4') throw new Error('landing packageManager changed without a reviewed migration')
  if (manifest.scripts?.['quality:scope'] !== 'node ./scripts/check-quality-scope.mjs') throw new Error('quality:scope must execute the actual admission command')
  if (manifest.scripts?.lint !== 'pnpm run quality:scope && pnpm run lint:source') throw new Error('lint must retain source admission and the real ESLint route')
  return parseLintRoute(manifest.scripts?.['lint:source'])
}

function severityValue(setting) {
  const value = Array.isArray(setting) ? setting[0] : setting
  return value === 'error' ? 2 : value
}

export function assertEffectiveProtectedRules(config, relativePath = 'source') {
  for (const name of PROTECTED_RULES) {
    if (severityValue(config?.rules?.[name]) !== 2) throw new Error(`protected rule is not error for ${relativePath}: ${name}`)
  }
}

function trackedSources() {
  const output = execFileSync('git', ['ls-files', '-z', '--', 'landing'], { cwd: repositoryRoot, encoding: 'utf8' })
  return output.split('\0').filter(Boolean).filter((item) => SOURCE_PATTERN.test(item))
}

export async function verify(manifest = JSON.parse(readFileSync(path.join(packageRoot, 'package.json'), 'utf8'))) {
  const targets = assertRoutes(manifest)
  const sources = trackedSources()
  const unclassified = sources.filter((item) => classifySourcePath(item) === null)
  if (unclassified.length > 0) throw new Error(`unclassified landing source:\n${unclassified.join('\n')}`)

  const eslint = new ESLint({ cwd: packageRoot })
  const selectedResults = await eslint.lintFiles(targets)
  const selected = new Set(selectedResults.map((result) => path.relative(packageRoot, result.filePath).replaceAll('\\', '/')))
  const admitted = sources
    .map((item) => item.replace(/^landing\//u, ''))
    .filter((item) => ['production', 'tooling'].includes(classifySourcePath(item)))
  const omitted = admitted.filter((item) => !selected.has(item))
  if (omitted.length > 0) throw new Error(`admitted landing source omitted by ESLint selection:\n${omitted.join('\n')}`)

  for (const relativePath of admitted) {
    const config = await eslint.calculateConfigForFile(path.join(packageRoot, relativePath))
    if (config === undefined) throw new Error(`admitted landing source has no effective ESLint config: ${relativePath}`)
    assertEffectiveProtectedRules(config, relativePath)
  }

  const counts = Object.create(null)
  for (const item of sources) counts[classifySourcePath(item)] = (counts[classifySourcePath(item)] ?? 0) + 1
  process.stdout.write(`landing quality scope verified: ${JSON.stringify(counts)}\n`)
}

if (process.argv[1] === fileURLToPath(import.meta.url)) await verify()
