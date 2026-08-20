import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'
import { expectedDistribution, githubSourceUrl, isPinnedExternalSource, mirroredIconPath, parseDirectoryData, parseRegistryIndex, validationLabel } from '../utils/registry.ts'

const fixture = JSON.parse(readFileSync(fileURLToPath(new URL('./fixtures/registry.valid.json', import.meta.url)), 'utf8')) as unknown
const snapshotFixture = JSON.parse(readFileSync(fileURLToPath(new URL('./fixtures/directory.snapshot.json', import.meta.url)), 'utf8')) as unknown
const stylesheet = readFileSync(fileURLToPath(new URL('../assets/css/main.css', import.meta.url)), 'utf8')

type RGB = [number, number, number]

function hexToRgb(value: string): RGB {
  return [1, 3, 5].map(offset => Number.parseInt(value.slice(offset, offset + 2), 16)) as RGB
}

function relativeLuminance(color: RGB): number {
  const [red, green, blue] = color.map((channel) => {
    const value = channel / 255
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * red! + 0.7152 * green! + 0.0722 * blue!
}

function contrastRatio(first: RGB, second: RGB): number {
  const [lighter, darker] = [relativeLuminance(first), relativeLuminance(second)].sort((a, b) => b - a)
  return (lighter! + 0.05) / (darker! + 0.05)
}

function mix(first: RGB, second: RGB, firstWeight: number): RGB {
  return first.map((channel, index) => channel * firstWeight + second[index]! * (1 - firstWeight)) as RGB
}

describe('registry parsing', () => {
  it('normalizes a signed snapshot into one product with source alternatives and exact evidence', () => {
    const directory = parseDirectoryData(snapshotFixture, 'published_snapshot')
    assert.equal(directory.data_source, 'published_snapshot')
    assert.equal(directory.snapshot_sequence, 42)
    assert.equal(directory.plugins.length, 1)
    assert.equal(directory.plugins[0]?.display_name, 'Demo')
    assert.equal(directory.plugins[0]?.distributions.length, 2)
    assert.equal(directory.plugins[0]?.default_distribution, 'example/demo')
    assert.deepEqual(directory.plugins[0]?.client_support.clients, ['codex', 'cursor', 'kiro'])
    assert.equal(expectedDistribution(directory.plugins[0]!, ['codex', 'cursor'])?.id, 'example/demo')
    assert.equal(expectedDistribution(directory.plugins[0]!, ['codex', 'kiro'])?.id, 'example/demo-bridge')
    assert.deepEqual(directory.plugins[0]?.evidence[0], {
      client: 'codex', level: 'runtime', outcome: 'passed', client_version: '0.200.0', os: 'linux', architecture: 'amd64', tested_at: '2026-08-19T00:00:00Z', evidence_url: `https://github.com/example/evidence/blob/${'e'.repeat(40)}/evidence/demo.json`,
    })
    assert.equal(validationLabel(directory.plugins[0]!), 'Runtime tested')
  })

  it('requires publication identity only at the signed production boundary', () => {
    const raw = snapshotFixture as Record<string, unknown>
    const unresolved = structuredClone({ ...raw, snapshot_schema_version: undefined, sequence: undefined, generated_at: undefined, expires_at: undefined, schema_version: 1 }) as Record<string, unknown>
    ;(unresolved.distributions as Array<{ releases: Array<{ package_source: { revision: string | null } }> }>)[0]!.releases[0]!.package_source.revision = null
    const preview = parseDirectoryData(unresolved, 'review_preview')
    assert.equal(preview.data_source, 'review_preview')
    assert.equal(preview.plugins[0]?.source.revision, null)
    assert.throws(() => parseDirectoryData(unresolved, 'published_snapshot'), /signed sequence, generated_at, and expires_at/)
  })

  it('normalizes valid built-in and external entries', () => {
    const registry = parseRegistryIndex(fixture)
    assert.equal(registry.data_source, 'legacy_compatibility')
    assert.equal(registry.plugins.length, 2)
    assert.equal(registry.plugins[0]?.author.name, 'Community package for Upstash')
    assert.equal(registry.plugins[0]?.source.path, 'plugins/context7')
    assert.deepEqual(registry.plugins[0]?.client_support.clients, ['codex', 'cursor', 'copilot', 'vscode', 'kiro'])
    assert.equal(registry.plugins[1]?.client_support.resolution, 'install_time')
    assert.deepEqual(registry.plugins[0]?.evidence.map(item => item.client), ['codex', 'cursor'])
    assert.equal(validationLabel(registry.plugins[0]!), 'Schema validated')
    assert.deepEqual(registry.plugins[1]?.components, ['skills'])
  })

  it('fails on invalid essential fields', () => {
    assert.throws(() => parseRegistryIndex({ schema_version: 1, plugins: [{ name: 'missing-fields' }] }), /built_in must be a boolean/)
    assert.throws(() => parseRegistryIndex({ schema_version: 2, plugins: [] }), /schema_version 1/)
  })

  it('rejects invalid or source-mismatched client support', () => {
    const registry = parseRegistryIndex(fixture)
    const builtIn = registry.plugins[0]!
    assert.throws(() => parseRegistryIndex({
      schema_version: 1,
      plugins: [{ ...builtIn, client_support: { resolution: 'install_time', clients: ['cursor'] } }],
    }), /does not match source type/)
    assert.throws(() => parseRegistryIndex({
      schema_version: 1,
      plugins: [{ ...builtIn, client_support: { resolution: 'catalog', clients: ['unknown'] } }],
    }), /invalid client/)
  })

  it('rejects duplicate names', () => {
    const raw = fixture as { plugins: unknown[] }
    assert.throws(() => parseRegistryIndex({ schema_version: 1, plugins: [raw.plugins[0], raw.plugins[0]] }), /duplicate name/)
  })

  it('parses the authoritative production index with exactly 26 built-ins', () => {
    const real = JSON.parse(readFileSync(fileURLToPath(new URL('../../registry/index.json', import.meta.url)), 'utf8')) as unknown
    const registry = parseRegistryIndex(real)
    assert.ok(registry.plugins.length >= 26)
    assert.equal(registry.plugins.filter(plugin => plugin.built_in).length, 26)
    for (const plugin of registry.plugins) {
      if (plugin.built_in) {
        assert.equal(plugin.install_source, plugin.name)
        assert.equal(plugin.client_support.resolution, 'directory')
      } else {
        assert.equal(plugin.install_source, `${plugin.source.repository}@${plugin.source.revision}//${plugin.source.path}`)
        assert.equal(plugin.client_support.resolution, 'install_time')
      }
    }
    for (const plugin of registry.plugins.filter(plugin => plugin.built_in && plugin.icon)) {
      const filename = plugin.icon!.path.split('/').at(-1)!
      const body = readFileSync(fileURLToPath(new URL(`../public/plugin-icons/${filename}`, import.meta.url)))
      assert.equal(`sha256:${createHash('sha256').update(body).digest('hex')}`, plugin.icon!.sha256)
    }
  })

  it('accepts a generated index with 26 built-ins plus a valid external entry', () => {
    const real = JSON.parse(readFileSync(fileURLToPath(new URL('../../registry/index.json', import.meta.url)), 'utf8')) as { plugins: unknown[] }
    const fixtureIndex = fixture as { plugins: unknown[] }
    const builtIns = real.plugins.filter((plugin) => (plugin as { built_in?: boolean }).built_in)
    const registry = parseRegistryIndex({ schema_version: 1, plugins: [...builtIns, fixtureIndex.plugins[1]] })

    assert.equal(registry.plugins.length, 27)
    assert.equal(registry.plugins.filter(plugin => plugin.built_in).length, 26)
    const external = registry.plugins.find(plugin => !plugin.built_in)!
    assert.equal(external.client_support.resolution, 'install_time')
    assert.equal(external.install_source, `${external.source.repository}@${external.source.revision}//${external.source.path}`)
  })

  it('builds immutable GitHub source links and never mirrors external icons', () => {
    const registry = parseRegistryIndex(fixture)
    assert.equal(githubSourceUrl(registry.plugins[1]!), 'https://github.com/example/plugins/tree/0123456789abcdef0123456789abcdef01234567/plugins/example')
    assert.equal(mirroredIconPath(registry.plugins[0]!), 'plugin-icons/context7.png')
    assert.equal(mirroredIconPath({ ...registry.plugins[1]!, icon: registry.plugins[0]!.icon }), undefined)
  })
})

describe('external pinned-source behavior', () => {
  const valid = 'owner/repo@0123456789abcdef0123456789abcdef01234567//plugins/example'

  it('recognizes only full 40-character GitHub pins', () => {
    assert.equal(isPinnedExternalSource(valid), true)
    assert.equal(isPinnedExternalSource('owner/repo@main//plugins/example'), false)
    assert.equal(isPinnedExternalSource('example-external'), false)
  })

  it('fails closed instead of allowing external short-name resolution', () => {
    const registry = parseRegistryIndex(fixture)
    const external = registry.plugins[1]!
    assert.throws(() => parseRegistryIndex({
      schema_version: 1,
      plugins: [{ ...external, install_source: external.name }],
    }), /external install_source must use source repository@40-char-sha\/\/path/)
  })
})

describe('site contrast', () => {
  it('keeps primary button text above 4.5:1 across the gradient in both themes', () => {
    const gradient = stylesheet.match(/\.button--primary \{[^}]*linear-gradient\(135deg, (#[0-9a-f]{6}), (#[0-9a-f]{6})\)/i)
    assert.ok(gradient)
    const start = hexToRgb(gradient[1]!)
    const end = hexToRgb(gradient[2]!)
    const white: RGB = [255, 255, 255]

    for (const theme of ['dark', 'light']) {
      for (const [position, background] of [['start', start], ['midpoint', mix(start, end, 0.5)], ['end', end]] as const) {
        const ratio = contrastRatio(white, background)
        assert.ok(ratio >= 4.5, `${theme} ${position} contrast is ${ratio.toFixed(4)}:1`)
      }
    }
  })

  it('keeps the light-theme badge-list cyan above 4.5:1', () => {
    const lightTheme = stylesheet.match(/:root\[data-theme="light"\] \{([^}]+)\}/)
    assert.ok(lightTheme)
    const cyan = lightTheme[1]!.match(/--cyan: (#[0-9a-f]{6});/i)
    const surface = lightTheme[1]!.match(/--surface-raised: (#[0-9a-f]{6});/i)
    const badgeMix = stylesheet.match(/\.badge-list li \{[^}]*background: color-mix\(in srgb, var\(--cyan\) ([0-9]+)%, var\(--surface-raised\)\)/)
    assert.ok(cyan && surface && badgeMix)
    const foreground = hexToRgb(cyan[1]!)
    const background = mix(foreground, hexToRgb(surface[1]!), Number(badgeMix[1]) / 100)
    const ratio = contrastRatio(foreground, background)

    assert.ok(ratio >= 4.5, `light badge-list contrast is ${ratio.toFixed(4)}:1`)
  })
})
