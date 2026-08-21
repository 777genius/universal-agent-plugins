import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, test } from 'node:test'
import {
  finalizeCspDirectory,
  finalizeHtmlCsp,
  verifyHtmlCsp,
} from '../scripts/csp-html.mjs'

const temporaryDirectories: string[] = []
afterEach(async () => {
  await Promise.all(temporaryDirectories.splice(0).map(path => rm(path, { force: true, recursive: true })))
})

function hash(source: string) {
  return `'sha256-${createHash('sha256').update(source, 'utf8').digest('base64')}'`
}

function page(scripts: string, scriptSources = "'self'") {
  return `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'self'; object-src 'none'; script-src ${scriptSources}; style-src 'self'; style-src-attr 'unsafe-inline'; base-uri 'none'"></head><body>${scripts}</body></html>`
}

test('finalizes a multi-script page from the exact inline bytes and preserves other directives', () => {
  const first = ' globalThis.one = 1\n'
  const second = '{"app":{"baseURL":"/docs/"}}'
  const html = page(`<script>${first}</script><script src="/docs/_nuxt/app.js"></script><script type="application/json">${second}</script>`)

  const finalized = finalizeHtmlCsp(html, 'index.html')

  assert.match(finalized, new RegExp(`script-src 'self' ${hash(first).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')} ${hash(second).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`))
  assert.match(finalized, /object-src 'none'/)
  assert.match(finalized, /base-uri 'none'/)
  verifyHtmlCsp(finalized, 'index.html')
})

test('finalization is idempotent', () => {
  const once = finalizeHtmlCsp(page('<style>body { color: CanvasText }</style><script>console.log("stable")</script>'))
  assert.equal(finalizeHtmlCsp(once), once)
  assert.match(once, new RegExp(`style-src 'self' ${hash('body { color: CanvasText }').replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`))
  assert.ok(once.includes("'sha256-0sLsI2a+NIcumVvBF9zD/ArGqlZR2xfnxsALPmK7nj8='"))
  assert.ok(once.includes("'sha256-60LHlRjW/B3CtzIoE/Lf1/NEDvko9efWMFaGVhHu/cs='"))
})

test('verification rejects an unreviewed runtime style hash', () => {
  const authorized = finalizeHtmlCsp(page('<script>allowed()</script>'))
  assert.throws(
    () => verifyHtmlCsp(authorized.replace(
      "'sha256-60LHlRjW/B3CtzIoE/Lf1/NEDvko9efWMFaGVhHu/cs='",
      "'sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA='",
    ), 'runtime-style.html'),
    /style-src does not exactly authorize the inline styles/,
  )
})

test('moves the CSP meta policy ahead of executable and styled content', () => {
  const latePolicy = page('<script>early()</script>').replace('<head><meta', '<head><script>head()</script><meta')
  const finalized = finalizeHtmlCsp(latePolicy)
  assert.ok(finalized.indexOf('Content-Security-Policy') < finalized.indexOf('<script'))
  verifyHtmlCsp(finalized)
})

test('fails closed on missing or malformed CSP meta policies', () => {
  assert.throws(
    () => finalizeHtmlCsp('<html><script>one()</script></html>', 'missing.html'),
    /expected exactly one Content-Security-Policy meta element, found 0/,
  )
  assert.throws(
    () => finalizeHtmlCsp(page('<script>one()</script>').replace("; base-uri 'none'", "; script-src 'self'"), 'duplicate.html'),
    /duplicate CSP directive: script-src/,
  )
  assert.throws(
    () => finalizeHtmlCsp(page('<script>one()</script>').replace("; script-src 'self'", ''), 'missing-script-src.html'),
    /missing script-src/,
  )
  assert.throws(
    () => finalizeHtmlCsp(page('<script>one()</script>').replace("; base-uri 'none'", "; frame-ancestors 'none'"), 'unsupported.html'),
    /not supported in a meta policy: frame-ancestors/,
  )
  assert.throws(
    () => finalizeHtmlCsp(page('<script>one()</script>').replace("; base-uri 'none'", "; script-src-elem *"), 'override.html'),
    /CSP override directive is not supported: script-src-elem/,
  )
})

test('rejects unsafe or external script sources', () => {
  assert.throws(
    () => finalizeHtmlCsp(page('<script>one()</script>', "'self' 'unsafe-inline'"), 'unsafe.html'),
    /unsafe script-src source: 'unsafe-inline'/,
  )
  assert.throws(
    () => finalizeHtmlCsp(page('<script>one()</script>').replace("style-src 'self'", "style-src 'self' 'unsafe-inline'"), 'unsafe-style.html'),
    /unsafe style-src source: 'unsafe-inline'/,
  )
  assert.throws(
    () => finalizeHtmlCsp(page('<script src="https://cdn.example/app.js"></script>'), 'external.html'),
    /external script source is not a same-origin path/,
  )
  assert.throws(
    () => finalizeHtmlCsp(page('<script>one()</script>').replace("style-src-attr 'unsafe-inline'", "style-src-attr 'unsafe-inline' 'self'"), 'wide-style-attr.html'),
    /style-src-attr must contain only/,
  )
})

test('verification rejects a hash mismatch and an unauthorized inline script', () => {
  const authorized = finalizeHtmlCsp(page('<script>allowed()</script>'))
  assert.throws(
    () => verifyHtmlCsp(authorized.replace('allowed()', 'changed()'), 'mismatch.html'),
    /does not exactly authorize the inline scripts/,
  )
  assert.throws(
    () => verifyHtmlCsp(authorized.replace('</body>', '<script>extra()</script></body>'), 'extra.html'),
    /does not exactly authorize the inline scripts/,
  )
})

test('recursively processes only HTML files in deterministic path order', async () => {
  const root = await mkdtemp(join(tmpdir(), 'uap-csp-test-'))
  temporaryDirectories.push(root)
  await mkdir(join(root, 'a', 'nested'), { recursive: true })
  await mkdir(join(root, 'z'), { recursive: true })
  await writeFile(join(root, 'z', 'last.html'), page('<script>last()</script>'))
  await writeFile(join(root, 'a', 'nested', 'middle.html'), page('<script>middle()</script>'))
  await writeFile(join(root, 'a', 'first.html'), page('<script>first()</script>'))
  await writeFile(join(root, 'asset.js'), 'doNotTouch()')

  assert.deepEqual(await finalizeCspDirectory(root), [
    'a/first.html',
    'a/nested/middle.html',
    'z/last.html',
  ])
  assert.equal(await readFile(join(root, 'asset.js'), 'utf8'), 'doNotTouch()')
  verifyHtmlCsp(await readFile(join(root, 'a', 'first.html'), 'utf8'))
})
