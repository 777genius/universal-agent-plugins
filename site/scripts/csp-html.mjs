import { createHash } from 'node:crypto'
import { readFile, readdir, writeFile } from 'node:fs/promises'
import { relative, resolve, sep } from 'node:path'

const META_UNSUPPORTED_DIRECTIVES = new Set([
  'frame-ancestors',
  'report-to',
  'report-uri',
  'sandbox',
])
const SCRIPT_UNSAFE_SOURCES = new Set(["'unsafe-eval'", "'unsafe-inline'", "'wasm-unsafe-eval'"])
// Reka UI 2.10.3 emits these two deterministic viewport rules when its
// ComboboxViewport and SelectViewport mount. Keep this allowlist byte-exact:
// a dependency change that alters either rule must fail browser E2E and receive
// a fresh source review before its replacement hash is accepted.
const REVIEWED_RUNTIME_STYLE_HASHES = Object.freeze([
  "'sha256-0sLsI2a+NIcumVvBF9zD/ArGqlZR2xfnxsALPmK7nj8='",
  "'sha256-60LHlRjW/B3CtzIoE/Lf1/NEDvko9efWMFaGVhHu/cs='",
])
const META_TAG = /<meta\b(?:[^>"']|"[^"]*"|'[^']*')*>/giu
const SCRIPT_ELEMENT = /<script\b((?:[^>"']|"[^"]*"|'[^']*')*)>([\s\S]*?)<\/script\s*>/giu
const STYLE_ELEMENT = /<style\b((?:[^>"']|"[^"]*"|'[^']*')*)>([\s\S]*?)<\/style\s*>/giu

function fail(message, filename) {
  throw new Error(`${filename}: ${message}`)
}

function decodeAttribute(value) {
  return value.replace(/&(?:amp|quot|#0*39|#x0*27);/giu, entity => ({
    '&amp;': '&',
    '&quot;': '"',
    '&#39;': "'",
    '&#x27;': "'",
  })[entity.toLowerCase()])
}

function encodeAttribute(value) {
  return value.replace(/&/gu, '&amp;').replace(/"/gu, '&quot;').replace(/</gu, '&lt;')
}

function parseAttributes(source, filename, element) {
  const attributes = new Map()
  let offset = 0
  while (offset < source.length) {
    const whitespace = /^\s+/u.exec(source.slice(offset))
    if (whitespace) offset += whitespace[0].length
    if (offset === source.length) break

    const nameMatch = /^[^\s=/>]+/u.exec(source.slice(offset))
    if (!nameMatch) fail(`malformed ${element} attributes`, filename)
    const name = nameMatch[0].toLowerCase()
    if (attributes.has(name)) fail(`duplicate ${element} attribute: ${name}`, filename)
    offset += nameMatch[0].length

    const afterName = /^\s*/u.exec(source.slice(offset))[0]
    offset += afterName.length
    let value = null
    if (source[offset] === '=') {
      offset += 1
      offset += /^\s*/u.exec(source.slice(offset))[0].length
      const quote = source[offset]
      if (quote === '"' || quote === "'") {
        const end = source.indexOf(quote, offset + 1)
        if (end === -1) fail(`unterminated ${element} attribute: ${name}`, filename)
        value = decodeAttribute(source.slice(offset + 1, end))
        offset = end + 1
      } else {
        const unquoted = /^[^\s>]+/u.exec(source.slice(offset))
        if (!unquoted) fail(`missing ${element} attribute value: ${name}`, filename)
        value = decodeAttribute(unquoted[0])
        offset += unquoted[0].length
      }
    }
    attributes.set(name, value)
  }
  return attributes
}

function findCspMeta(html, filename) {
  const matches = []
  for (const match of html.matchAll(META_TAG)) {
    const tag = match[0]
    const attributes = parseAttributes(tag.slice(5, -1).replace(/\/\s*$/u, ''), filename, 'meta')
    if (attributes.get('http-equiv')?.toLowerCase() === 'content-security-policy') {
      matches.push({ attributes, end: match.index + tag.length, start: match.index, tag })
    }
  }
  if (matches.length !== 1) {
    fail(`expected exactly one Content-Security-Policy meta element, found ${matches.length}`, filename)
  }
  if (matches[0].attributes.get('content') === null || !matches[0].attributes.has('content')) {
    fail('Content-Security-Policy meta element is missing its content attribute', filename)
  }
  return matches[0]
}

function placeCspFirst(html, meta, replacement, filename) {
  const withoutMeta = html.slice(0, meta.start) + html.slice(meta.end)
  const heads = [...withoutMeta.matchAll(/<head\b(?:[^>"']|"[^"]*"|'[^']*')*>/giu)]
  if (heads.length !== 1) fail(`expected exactly one head element, found ${heads.length}`, filename)
  const insertion = heads[0].index + heads[0][0].length
  return withoutMeta.slice(0, insertion) + replacement + withoutMeta.slice(insertion)
}

function parsePolicy(value, filename) {
  const directives = []
  const names = new Set()
  for (const rawDirective of value.split(';')) {
    const trimmed = rawDirective.trim()
    if (!trimmed) continue
    const parts = trimmed.split(/\s+/u)
    const name = parts.shift().toLowerCase()
    if (!/^[a-z][a-z0-9-]*$/u.test(name)) fail(`malformed CSP directive: ${name}`, filename)
    if (names.has(name)) fail(`duplicate CSP directive: ${name}`, filename)
    names.add(name)
    directives.push({ name, sources: parts })
  }
  if (!directives.length) fail('empty Content-Security-Policy', filename)
  if (!names.has('script-src')) fail('Content-Security-Policy is missing script-src', filename)
  for (const override of ['script-src-elem', 'style-src-elem']) {
    if (names.has(override)) fail(`CSP override directive is not supported: ${override}`, filename)
  }
  for (const directive of directives) {
    if (META_UNSUPPORTED_DIRECTIVES.has(directive.name)) {
      fail(`CSP directive is not supported in a meta policy: ${directive.name}`, filename)
    }
  }
  return directives
}

function scriptHash(source) {
  return `'sha256-${createHash('sha256').update(source, 'utf8').digest('base64')}'`
}

function inlineScriptHashes(html, filename) {
  const hashes = []
  let withoutScripts = html
  for (const match of html.matchAll(SCRIPT_ELEMENT)) {
    const attributes = parseAttributes(match[1], filename, 'script')
    const source = attributes.get('src')
    if (source !== undefined) {
      if (source === null || source === '' || source.startsWith('//')
        || /^[a-z][a-z0-9+.-]*:/iu.test(source) || source.includes('\\')) {
        fail(`external script source is not a same-origin path: ${source ?? '(missing)'}`, filename)
      }
      if (match[2].trim()) fail('script element cannot have both src and inline content', filename)
    } else {
      hashes.push(scriptHash(match[2]))
    }
  }
  withoutScripts = withoutScripts.replace(SCRIPT_ELEMENT, '')
  if (/<\/?script\b/iu.test(withoutScripts)) fail('malformed script element', filename)
  return [...new Set(hashes)]
}

function expectedScriptSources(hashes) {
  return ["'self'", ...hashes]
}

function validateHashSources(sources, directive, filename) {
  for (const source of sources) {
    const lower = source.toLowerCase()
    if (SCRIPT_UNSAFE_SOURCES.has(lower)) fail(`unsafe ${directive} source: ${source}`, filename)
    if (lower !== "'self'" && !/^'sha256-[a-z0-9+/]+={0,2}'$/iu.test(source)) {
      fail(`unsupported ${directive} source: ${source}`, filename)
    }
  }
}

function styleHashes(html, filename) {
  const hashes = []
  for (const match of html.matchAll(STYLE_ELEMENT)) {
    parseAttributes(match[1], filename, 'style')
    hashes.push(scriptHash(match[2]))
  }
  if (/<\/?style\b/iu.test(html.replace(STYLE_ELEMENT, ''))) fail('malformed style element', filename)
  return [...new Set(hashes)]
}

function expectedStyleSources(hashes) {
  return ["'self'", ...hashes, ...REVIEWED_RUNTIME_STYLE_HASHES]
}

function validateNoUnsafeSources(directives, filename) {
  const styleAttributes = directives.find(({ name }) => name === 'style-src-attr')
  if (!styleAttributes
    || styleAttributes.sources.length !== 1
    || styleAttributes.sources[0].toLowerCase() !== "'unsafe-inline'") {
    fail("style-src-attr must contain only the reviewed 'unsafe-inline' positioning exception", filename)
  }
  for (const { name, sources } of directives) {
    for (const source of sources) {
      if (SCRIPT_UNSAFE_SOURCES.has(source.toLowerCase()) && name !== 'style-src-attr') {
        fail(`unsafe ${name} source: ${source}`, filename)
      }
    }
  }
}

function serializePolicy(directives) {
  return directives.map(({ name, sources }) => [name, ...sources].join(' ')).join('; ')
}

export function finalizeHtmlCsp(html, filename = '<html>') {
  const meta = findCspMeta(html, filename)
  const directives = parsePolicy(meta.attributes.get('content'), filename)
  validateNoUnsafeSources(directives, filename)
  const scriptDirective = directives.find(directive => directive.name === 'script-src')
  validateHashSources(scriptDirective.sources, 'script-src', filename)
  scriptDirective.sources = expectedScriptSources(inlineScriptHashes(html, filename))
  const styleDirective = directives.find(directive => directive.name === 'style-src')
  const hashes = styleHashes(html, filename)
  if (hashes.length && !styleDirective) fail('Content-Security-Policy is missing style-src', filename)
  if (styleDirective) {
    validateHashSources(styleDirective.sources, 'style-src', filename)
    styleDirective.sources = expectedStyleSources(hashes)
  }

  const content = encodeAttribute(serializePolicy(directives))
  const replacement = meta.tag.replace(
    /(\bcontent\s*=\s*)(?:"[^"]*"|'[^']*'|[^\s>]+)/iu,
    `$1"${content}"`,
  )
  return placeCspFirst(html, meta, replacement, filename)
}

export function verifyHtmlCsp(html, filename = '<html>') {
  const meta = findCspMeta(html, filename)
  const firstInlineResource = html.search(/<(?:script|style)\b/iu)
  if (firstInlineResource !== -1 && meta.start > firstInlineResource) {
    fail('Content-Security-Policy meta element must precede scripts and styles', filename)
  }
  const directives = parsePolicy(meta.attributes.get('content'), filename)
  validateNoUnsafeSources(directives, filename)
  const scriptDirective = directives.find(directive => directive.name === 'script-src')
  validateHashSources(scriptDirective.sources, 'script-src', filename)
  const expected = expectedScriptSources(inlineScriptHashes(html, filename))
  if (scriptDirective.sources.length !== expected.length
    || scriptDirective.sources.some((source, index) => source !== expected[index])) {
    fail(`script-src does not exactly authorize the inline scripts (expected ${expected.join(' ')})`, filename)
  }
  const styleDirective = directives.find(directive => directive.name === 'style-src')
  const expectedStyles = expectedStyleSources(styleHashes(html, filename))
  if (expectedStyles.length > 1 && !styleDirective) fail('Content-Security-Policy is missing style-src', filename)
  if (styleDirective) {
    validateHashSources(styleDirective.sources, 'style-src', filename)
    if (styleDirective.sources.length !== expectedStyles.length
      || styleDirective.sources.some((source, index) => source !== expectedStyles[index])) {
      fail(`style-src does not exactly authorize the inline styles (expected ${expectedStyles.join(' ')})`, filename)
    }
  }
}

async function htmlFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true })
  entries.sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0)
  const results = []
  for (const entry of entries) {
    const path = resolve(directory, entry.name)
    if (entry.isDirectory()) results.push(...await htmlFiles(path))
    else if (entry.isFile() && entry.name.endsWith('.html')) results.push(path)
  }
  return results
}

export async function finalizeCspDirectory(directory) {
  const paths = await htmlFiles(directory)
  const processed = []
  const finalizedFiles = []
  for (const path of paths) {
    const filename = relative(directory, path).split(sep).join('/')
    const html = await readFile(path, 'utf8')
    const finalized = finalizeHtmlCsp(html, filename)
    finalizedFiles.push({ finalized, html, path })
    processed.push(filename)
  }
  for (const { finalized, html, path } of finalizedFiles) {
    if (finalized !== html) await writeFile(path, finalized, 'utf8')
  }
  return processed
}

export async function verifyCspDirectory(directory) {
  const paths = await htmlFiles(directory)
  for (const path of paths) {
    const filename = relative(directory, path).split(sep).join('/')
    verifyHtmlCsp(await readFile(path, 'utf8'), filename)
  }
  return paths.length
}
