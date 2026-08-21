import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { verifyCspDirectory } from './csp-html.mjs'

const output = resolve(process.cwd(), '.output/public')
const sourcePath = process.env.UAP_SIGNED_SNAPSHOT_PATH
  ?? process.env.UAP_DIRECTORY_PREVIEW_PATH
  ?? process.env.UAP_REGISTRY_PATH
  ?? '../registry/index.json'
const registry = JSON.parse(readFileSync(resolve(process.cwd(), sourcePath), 'utf8'))
const productIDs = Array.isArray(registry.products)
  ? registry.products.map(product => product.id)
  : registry.plugins.map(plugin => plugin.name)
const base = (process.env.NUXT_APP_BASE_URL ?? '/').replace(/\/?$/, '/')
const failures = []

const expected = ['index.html', 'plugins/index.html', 'robots.txt', 'sitemap.xml', ...productIDs.map(id => `plugins/${id}/index.html`)]
for (const file of expected) {
  const target = resolve(output, file)
  if (!existsSync(target) || !statSync(target).isFile()) failures.push(`missing prerendered file: ${file}`)
}

function files(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name)
    return entry.isDirectory() ? files(path) : [path]
  })
}

function targetExists(pathname) {
  let relative = decodeURIComponent(pathname.slice(base.length)).replace(/^\//, '')
  if (!relative || relative.endsWith('/')) relative += 'index.html'
  const direct = resolve(output, relative)
  return existsSync(direct) || existsSync(`${direct}.html`) || existsSync(resolve(direct, 'index.html'))
}

const htmlFiles = files(output).filter(path => path.endsWith('.html')).sort()
for (const file of htmlFiles) {
  const html = readFileSync(file, 'utf8')
  for (const match of html.matchAll(/(?:href|src)="([^"]+)"/g)) {
    const value = match[1]
    if (!value || value.startsWith('#') || /^(?:https?:|mailto:|data:)/.test(value)) continue
    const pathname = value.split(/[?#]/, 1)[0]
    if (!pathname.startsWith(base)) {
      failures.push(`${file.slice(output.length + 1)}: internal URL escapes Pages base: ${value}`)
    } else if (!targetExists(pathname)) {
      failures.push(`${file.slice(output.length + 1)}: broken internal URL: ${value}`)
    }
  }
}

try {
  const verified = await verifyCspDirectory(output)
  if (verified !== htmlFiles.length) failures.push('CSP verifier did not inspect every generated HTML file')
} catch (error) {
  failures.push(error instanceof Error ? error.message : String(error))
}

if (failures.length) {
  throw new Error(`generated-site checks failed:\n${failures.join('\n')}`)
}
console.log(`generated-site checks passed (${expected.length} required routes, base ${base})`)
