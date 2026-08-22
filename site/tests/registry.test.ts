import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'
import { authenticationLabel, deliveryLabel, directoryIsExpired, expectedDistribution, githubSourceUrl, isPinnedExternalSource, mirroredIconPath, parseDirectoryData, parseRegistryIndex, resolveDistribution, targetAuthenticationLabel, validationLabel } from '../utils/registry.ts'
import { pluginCommands } from '../utils/commands.ts'

const fixture = JSON.parse(readFileSync(fileURLToPath(new URL('./fixtures/registry.valid.json', import.meta.url)), 'utf8')) as unknown
const snapshotFixture = JSON.parse(readFileSync(fileURLToPath(new URL('./fixtures/directory.snapshot.json', import.meta.url)), 'utf8')) as unknown
const blockedNewestFixture = JSON.parse(readFileSync(fileURLToPath(new URL('./fixtures/directory.blocked-newest.snapshot.json', import.meta.url)), 'utf8')) as unknown
const resolverGolden = JSON.parse(readFileSync(fileURLToPath(new URL('./fixtures/resolver-golden.json', import.meta.url)), 'utf8')) as {
  vectors: Array<{ id: string, changes: string[], targets: Array<'codex' | 'cursor' | 'kiro'>, expected: { distribution_id: string | null, release_sequence: number | null, fallback_reason: string | null, unavailable_reason?: string } }>
}
const stylesheet = readFileSync(fileURLToPath(new URL('../assets/css/main.css', import.meta.url)), 'utf8')

interface SnapshotFixture {
  products: Array<{ default_distribution: string }>
  distributions: Array<{
    id: string
    status: string
    release_policies: Array<{
      release_sequence: number
      status: string
      current_evidence: string[]
      targets: Array<{ client: string, authentication: string, delivery: string, scopes: string[], app_binding?: { app_key: string, id: string, mcp_server: string } }>
    }>
    releases: Array<Record<string, unknown>>
  }>
  evidence: Array<{ id: string, distribution_id: string, release_sequence: number, client?: string, level: string, outcome: string, package_tree_digest: string, installer_version?: string, artifact: { digest: string } }>
  revocations: Array<{ distribution_id: string, release_sequence: number }>
}

function signedFixture(): SnapshotFixture & Record<string, unknown> {
  return structuredClone(snapshotFixture) as SnapshotFixture & Record<string, unknown>
}

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
    assert.equal(directory.generated_at, '2026-08-20T00:00:00Z')
    assert.equal(directory.expires_at, '2026-09-19T00:00:00Z')
    assert.equal(directory.plugins.length, 1)
    assert.equal(directory.plugins[0]?.display_name, 'Demo')
    assert.equal(directory.plugins[0]?.distributions.length, 2)
    assert.equal(directory.plugins[0]?.distributions.filter(item => item.kind === 'community_bridge').length, 1)
    assert.equal(directory.plugins[0]?.distributions.find(item => item.kind === 'community_bridge')?.label, 'Community bridge')
    assert.equal(directory.plugins[0]?.distributions.find(item => item.kind === 'upstream')?.label, 'Upstream package')
    assert.equal(directory.plugins[0]?.default_distribution, 'example/demo')
    assert.equal(directory.plugins[0]?.declared_default_distribution, 'example/demo')
    assert.deepEqual(directory.plugins[0]?.client_support.clients, ['codex', 'cursor', 'kiro'])
    assert.equal(expectedDistribution(directory.plugins[0]!, ['codex', 'cursor'])?.id, 'example/demo')
    assert.equal(expectedDistribution(directory.plugins[0]!, ['codex', 'kiro'])?.id, 'example/demo-bridge')
    assert.deepEqual(directory.plugins[0]?.evidence[0], {
      id: 'runtime-demo-codex', client: 'codex', level: 'runtime', outcome: 'passed', client_version: '0.200.0', os: 'linux', architecture: 'amd64', tested_at: '2026-08-19T00:00:00Z',
      package_tree_digest: `sha256:${'1'.repeat(64)}`, installer_version: '0.1.6', artifact: { repository: 'example/evidence', revision: 'e'.repeat(40), path: 'evidence/demo.json', digest: `sha256:${'3'.repeat(64)}`, url: `https://github.com/example/evidence/blob/${'e'.repeat(40)}/evidence/demo.json` },
    })
    assert.deepEqual(directory.plugins[0]?.package_evidence[0], {
      id: 'schema-demo', level: 'schema', outcome: 'passed', package_tree_digest: `sha256:${'1'.repeat(64)}`,
      artifact: { repository: 'example/evidence', revision: 'f'.repeat(40), path: 'evidence/demo-schema.json', digest: `sha256:${'6'.repeat(64)}`, url: `https://github.com/example/evidence/blob/${'f'.repeat(40)}/evidence/demo-schema.json` },
    })
    assert.equal('client' in directory.plugins[0]!.package_evidence[0]!, false)
    assert.equal(directory.plugins[0]!.evidence.some(item => item.id === 'schema-demo'), false)
    assert.equal(validationLabel(directory.plugins[0]!), 'Runtime tested')
  })

  it('selects an active bridge and falls back when the declared default is suspended', () => {
    const raw = signedFixture()
    raw.distributions[0]!.status = 'suspended'
    const directory = parseDirectoryData(raw, 'published_snapshot')
    const plugin = directory.plugins[0]!
    assert.equal(plugin.declared_default_distribution, 'example/demo')
    assert.equal(plugin.default_distribution, 'example/demo-bridge')
    assert.equal(plugin.default_fallback_reason, 'Declared default is suspended')
    assert.equal(expectedDistribution(plugin, ['codex', 'kiro'])?.id, 'example/demo-bridge')
    assert.equal(plugin.distributions.find(item => item.id === 'example/demo')?.selectable, false)
    assert.equal(plugin.distributions.filter(item => item.selectable).length, 1)
  })

  it('never selects revoked or superseded releases for a new install', () => {
    const revoked = signedFixture()
    revoked.revocations.push({ distribution_id: 'example/demo', release_sequence: 2 })
    const revokedPlugin = parseDirectoryData(revoked, 'published_snapshot').plugins[0]!
    assert.equal(revokedPlugin.default_distribution, 'example/demo-bridge')
    assert.equal(revokedPlugin.distributions[0]?.release_status, 'revoked')
    assert.equal(revokedPlugin.distributions[0]?.selectable, false)

    const superseded = signedFixture()
    superseded.distributions[0]!.release_policies[0]!.status = 'superseded'
    const supersededPlugin = parseDirectoryData(superseded, 'published_snapshot').plugins[0]!
    assert.equal(supersededPlugin.default_distribution, 'example/demo-bridge')
    assert.equal(supersededPlugin.distributions[0]?.release_status, 'superseded')
    assert.equal(expectedDistribution(supersededPlugin, ['codex'])?.id, 'example/demo-bridge')
  })

  it('matches authoritative blocking-evidence fallback decisions', () => {
    for (const level of ['materialization', 'discovery', 'runtime']) {
      const raw = signedFixture()
      const runtime = raw.evidence.find(item => item.id === 'runtime-demo-codex')!
      runtime.level = level
      runtime.outcome = 'failed'
      const plugin = parseDirectoryData(raw, 'published_snapshot').plugins[0]!
      assert.equal(expectedDistribution(plugin, ['codex'])?.id, 'example/demo-bridge', `${level} failure must fall back`)
      assert.equal(expectedDistribution(plugin, ['cursor'])?.id, 'example/demo', `${level} failure must block only its client`)
    }
  })

  it('does not let unsigned, stale, or non-blocking evidence affect eligibility', () => {
    const baseline = signedFixture()
    const unsigned = structuredClone(baseline.evidence.find(item => item.id === 'runtime-demo-codex')!)
    unsigned.id = 'unsigned-failure'
    unsigned.outcome = 'failed'
    baseline.evidence.push(unsigned)
    assert.equal(expectedDistribution(parseDirectoryData(baseline, 'published_snapshot').plugins[0]!, ['codex'])?.id, 'example/demo')

    for (const outcome of ['inconclusive', 'not_tested', 'not_applicable']) {
      const raw = signedFixture()
      raw.evidence.find(item => item.id === 'runtime-demo-codex')!.outcome = outcome
      assert.equal(expectedDistribution(parseDirectoryData(raw, 'published_snapshot').plugins[0]!, ['codex'])?.id, 'example/demo')
    }

    const oauth = signedFixture()
    const oauthEvidence = oauth.evidence.find(item => item.id === 'runtime-demo-codex')!
    oauthEvidence.level = 'oauth'
    oauthEvidence.outcome = 'failed'
    assert.equal(expectedDistribution(parseDirectoryData(oauth, 'published_snapshot').plugins[0]!, ['codex'])?.id, 'example/demo')

    const stale = signedFixture()
    const staleEvidence = structuredClone(stale.evidence.find(item => item.id === 'runtime-demo-codex')!)
    staleEvidence.id = 'stale-failure'
    staleEvidence.outcome = 'failed'
    staleEvidence.package_tree_digest = `sha256:${'9'.repeat(64)}`
    stale.evidence.push(staleEvidence)
    stale.distributions[0]!.release_policies[0]!.current_evidence.push(staleEvidence.id)
    assert.equal(expectedDistribution(parseDirectoryData(stale, 'published_snapshot').plugins[0]!, ['codex'])?.id, 'example/demo')
  })

  it('falls back to an older eligible release before another distribution', () => {
    const raw = signedFixture()
    const upstream = raw.distributions[0]!
    const older = structuredClone(upstream.releases[0]!)
    older.sequence = 1
    older.package_version = '0.8.0'
    older.tree_digest = `sha256:${'7'.repeat(64)}`
    ;(older.package_source as Record<string, unknown>).revision = '9'.repeat(40)
    upstream.releases.push(older)
    const olderMaterialization = structuredClone(raw.evidence.find(item => item.id === 'materialization-demo-codex')!)
    olderMaterialization.id = 'materialization-demo-codex-release-1'
    olderMaterialization.release_sequence = 1
    olderMaterialization.package_tree_digest = `sha256:${'7'.repeat(64)}`
    raw.evidence.push(olderMaterialization)
    upstream.release_policies.push({
      ...structuredClone(upstream.release_policies[0]!),
      current_evidence: [olderMaterialization.id],
      release_sequence: 1,
    } as typeof upstream.release_policies[number])
    raw.evidence.find(item => item.id === 'runtime-demo-codex')!.outcome = 'failed'

    const selected = expectedDistribution(parseDirectoryData(raw, 'published_snapshot').plugins[0]!, ['codex'])
    assert.equal(selected?.id, 'example/demo')
    assert.equal(selected?.release_sequence, 1)
    assert.equal(selected?.version, '0.8.0')
  })

  it('uses the exact older Codex release for the command and every install-candidate field', () => {
    const plugin = parseDirectoryData(blockedNewestFixture, 'published_snapshot').plugins[0]!
    const candidate = expectedDistribution(plugin, ['codex'])!

    assert.equal(plugin.distributions[0]!.releases[0]!.version, '2.0.0', 'fixture history keeps blocked release 2 newest')
    assert.equal(candidate.release_sequence, 1)
    assert.equal(candidate.version, '1.0.0')
    assert.deepEqual(candidate.components, ['mcp', 'skills'])
    assert.equal(authenticationLabel(candidate, ['codex'], plugin.authentication), 'No account required')
    assert.equal(targetAuthenticationLabel(plugin.distributions[0]!.releases[0]!.targets[0]!.authentication), 'Authentication required')
    assert.equal(candidate.source.revision, '1'.repeat(40))
    assert.equal(candidate.source.path, 'plugins/release-fallback-v1')
    assert.deepEqual(candidate.package_evidence.map(item => item.id), ['schema-release-1'])
    assert.deepEqual(candidate.evidence.map(item => item.id), ['runtime-release-1-codex'])
    assert.equal(validationLabel(candidate), 'Materialization tested')
    assert.equal(githubSourceUrl(plugin, candidate), `https://github.com/example/plugins/tree/${'1'.repeat(40)}/plugins/release-fallback-v1`)
    assert.equal(pluginCommands(plugin, ['codex']).add, 'npx universal-agent-plugins add release-fallback --target codex')

    assert.notEqual(candidate.version, plugin.distributions[0]!.releases[0]!.version)
    assert.equal(candidate.evidence.some(item => item.id === 'runtime-release-2-codex'), false)
    assert.equal(candidate.package_evidence.some(item => item.id === 'schema-release-2'), false)
  })

  it('preserves signed delivery, scopes, and the exact Cloudflare ChatGPT app binding', () => {
    const raw = signedFixture()
    const bridge = raw.distributions[1]!
    raw.products[0]!.default_distribution = bridge.id
    bridge.release_policies[0]!.targets = [
      { client: 'codex', authentication: 'not_required', delivery: 'managed', scopes: ['user'] },
      { client: 'vscode', authentication: 'unknown', delivery: 'prepared', scopes: ['user'] },
      { client: 'chatgpt', authentication: 'required', delivery: 'manual_activation', scopes: ['user'], app_binding: { app_key: 'cloudflare-docs', id: 'plugin_asdk_app_6a78e90cf73481918ef10cdb87cd4bb4', mcp_server: 'cloudflare-docs' } },
    ]
    const plugin = parseDirectoryData(raw, 'published_snapshot').plugins[0]!
    const selected = expectedDistribution(plugin, ['chatgpt'])!
    assert.equal(selected.id, bridge.id)
    assert.deepEqual(selected.targets, bridge.release_policies[0]!.targets)
    assert.equal(deliveryLabel(selected.targets[0]!.delivery), 'Managed install')
    assert.equal(deliveryLabel(selected.targets[1]!.delivery), 'Prepared; client import remains')
    assert.equal(deliveryLabel(selected.targets[2]!.delivery), 'Manual activation required')
    assert.deepEqual(plugin.client_support.app_bindings.chatgpt, bridge.release_policies[0]!.targets[2]!.app_binding)
  })

  it('retains signed target authentication and summarizes the exact selected target set honestly', () => {
    const plugin = parseDirectoryData(snapshotFixture, 'published_snapshot').plugins[0]!
    const selected = expectedDistribution(plugin, ['codex', 'cursor'])!

    assert.equal(selected.targets.find(target => target.client === 'codex')?.authentication, 'not_required')
    assert.equal(selected.targets.find(target => target.client === 'cursor')?.authentication, 'required')
    assert.equal(authenticationLabel(selected, ['codex'], plugin.authentication), 'No account required')
    assert.equal(authenticationLabel(selected, ['cursor'], plugin.authentication), 'Authentication required')
    assert.equal(authenticationLabel(selected, ['codex', 'cursor'], plugin.authentication), 'Authentication varies')
    assert.equal(targetAuthenticationLabel('unknown'), 'Check package requirements')
  })

  it('retains and distinguishes Agent Code Navigator and Atlassian signed authentication', () => {
    const raw = JSON.parse(readFileSync(fileURLToPath(new URL('../../registry/directory.json', import.meta.url)), 'utf8')) as Record<string, unknown>
    raw.products = (raw.products as Array<{ id: string }>).filter(product => ['agent-code-navigator', 'atlassian'].includes(product.id))
    raw.distributions = (raw.distributions as Array<{ product_id: string }>).filter(distribution => ['agent-code-navigator', 'atlassian'].includes(distribution.product_id))
    raw.snapshot_schema_version = 1
    raw.sequence = 44
    raw.generated_at = '2026-08-22T00:00:00Z'
    raw.expires_at = '2026-09-22T00:00:00Z'
    const directory = parseDirectoryData(raw, 'published_snapshot')
    const navigator = directory.plugins.find(plugin => plugin.name === 'agent-code-navigator')!
    const atlassian = directory.plugins.find(plugin => plugin.name === 'atlassian')!
    const navigatorRelease = expectedDistribution(navigator, ['codex', 'cursor'])!
    const atlassianRelease = expectedDistribution(atlassian, ['codex', 'cursor'])!

    assert.deepEqual(navigatorRelease.targets.filter(target => ['codex', 'cursor'].includes(target.client)).map(target => target.authentication), ['not_required', 'not_required'])
    assert.deepEqual(atlassianRelease.targets.filter(target => ['codex', 'cursor'].includes(target.client)).map(target => target.authentication), ['required', 'required'])
    assert.equal(authenticationLabel(navigatorRelease, ['codex', 'cursor'], navigator.authentication), 'No account required')
    assert.equal(authenticationLabel(atlassianRelease, ['codex', 'cursor'], atlassian.authentication), 'Authentication required')
  })

  it('rejects missing or invalid signed target authentication', () => {
    const missing = signedFixture()
    delete (missing.distributions[0]!.release_policies[0]!.targets[0] as Partial<typeof missing.distributions[0]['release_policies'][0]['targets'][0]>).authentication
    assert.throws(() => parseDirectoryData(missing, 'published_snapshot'), /authentication must be a non-empty string/)

    const invalid = signedFixture()
    invalid.distributions[0]!.release_policies[0]!.targets[0]!.authentication = 'oauth'
    assert.throws(() => parseDirectoryData(invalid, 'published_snapshot'), /invalid authentication/)
  })

  it('accepts evidence only for the selected release exact package tuple', () => {
    const raw = signedFixture()
    raw.evidence.find(item => item.id === 'runtime-demo-codex')!.package_tree_digest = `sha256:${'9'.repeat(64)}`
    const plugin = parseDirectoryData(raw, 'published_snapshot').plugins[0]!
    assert.equal(plugin.evidence.some(item => item.id === 'runtime-demo-codex'), false)
    assert.equal(validationLabel(plugin), 'Materialization tested')

    const incomplete = signedFixture()
    delete incomplete.evidence.find(item => item.id === 'runtime-demo-codex')!.installer_version
    assert.equal(parseDirectoryData(incomplete, 'published_snapshot').plugins[0]!.evidence.some(item => item.id === 'runtime-demo-codex'), false)
  })

  it('matches every shared authoritative resolver golden vector', () => {
    for (const vector of resolverGolden.vectors) {
      const raw = signedFixture()
      const upstream = raw.distributions.find(item => item.id === 'example/demo')!
      const bridge = raw.distributions.find(item => item.id === 'example/demo-bridge')!
      for (const change of vector.changes) {
        if (change === 'suspend-default') upstream.status = 'suspended'
        else if (change === 'add-newer-eligible-upstream-release') {
          const newer = structuredClone(upstream.releases[0]!)
          newer.sequence = 3
          newer.package_version = '3.0.0'
          newer.tree_digest = `sha256:${'a'.repeat(64)}`
          ;(newer.package_source as Record<string, unknown>).revision = 'a'.repeat(40)
          upstream.releases.push(newer)
          const policy = structuredClone(upstream.release_policies[0]!)
          policy.release_sequence = 3
          policy.current_evidence = policy.current_evidence.map((id) => {
            const observation = structuredClone(raw.evidence.find(item => item.id === id)!)
            observation.id = `${id}-release-3`
            observation.release_sequence = 3
            observation.package_tree_digest = newer.tree_digest as string
            raw.evidence.push(observation)
            return observation.id
          })
          upstream.release_policies.push(policy)
        }
        else if (change === 'bridge-is-default') raw.products[0]!.default_distribution = bridge.id
        else if (change === 'suspend-bridge') bridge.status = 'suspended'
        else if (change === 'remove-upstream-materialization') upstream.release_policies[0]!.current_evidence = upstream.release_policies[0]!.current_evidence.filter(id => !id.startsWith('materialization-'))
        else if (change === 'fail-current-runtime-codex') raw.evidence.find(item => item.id === 'runtime-demo-codex')!.outcome = 'failed'
        else if (change === 'remove-bridge-kiro') bridge.release_policies[0]!.targets = bridge.release_policies[0]!.targets.filter(target => target.client !== 'kiro')
        else if (change === 'add-stale-runtime-failure-codex') {
          const stale = structuredClone(raw.evidence.find(item => item.id === 'runtime-demo-codex')!)
          stale.id = 'stale-runtime-failure-codex'
          stale.outcome = 'failed'
          stale.package_tree_digest = `sha256:${'9'.repeat(64)}`
          raw.evidence.push(stale)
          upstream.release_policies[0]!.current_evidence.push(stale.id)
        } else assert.fail(`unknown golden change ${change}`)
      }
      const plugin = parseDirectoryData(raw, 'published_snapshot').plugins[0]!
      const resolution = resolveDistribution(plugin, vector.targets)
      assert.equal(resolution.distribution?.id ?? null, vector.expected.distribution_id, vector.id)
      assert.equal(resolution.distribution?.release_sequence ?? null, vector.expected.release_sequence, vector.id)
      assert.equal(resolution.fallback_reason ?? null, vector.expected.fallback_reason, vector.id)
      assert.equal(resolution.unavailable_reason, vector.expected.unavailable_reason, vector.id)
    }
  })

  it('uses expiry only to gate current claims, without changing resolver evidence or selection', () => {
    const directory = parseDirectoryData(snapshotFixture, 'published_snapshot')
    const before = resolveDistribution(directory.plugins[0]!, ['codex'])
    assert.equal(directoryIsExpired(directory, Date.parse('2026-09-18T23:59:59Z')), false)
    assert.equal(directoryIsExpired(directory, Date.parse('2026-09-19T00:00:00Z')), true)
    const after = resolveDistribution(directory.plugins[0]!, ['codex'])
    assert.deepEqual(after, before)
  })

  it('fails closed on malformed or non-increasing publication instants', () => {
    for (const [field, value] of [
      ['generated_at', '2026-02-30T00:00:00Z'],
      ['generated_at', '2026-08-20 00:00:00Z'],
      ['generated_at', '2026-08-20T00:00:00'],
      ['expires_at', '2026-09-19T00:00:00+24:00'],
      ['expires_at', 'not-a-date'],
    ] as const) {
      const raw = signedFixture()
      raw[field] = value
      assert.throws(() => parseDirectoryData(raw, 'published_snapshot'), /RFC3339/)
    }
    for (const expiresAt of ['2026-08-20T00:00:00Z', '2026-08-19T23:59:59.999999999Z']) {
      const raw = signedFixture()
      raw.expires_at = expiresAt
      assert.throws(() => parseDirectoryData(raw, 'published_snapshot'), /must be after generated_at/)
    }
    const offset = signedFixture()
    offset.generated_at = '2026-08-20T01:00:00+01:00'
    offset.expires_at = '2026-08-20T00:00:00.000000001Z'
    assert.doesNotThrow(() => parseDirectoryData(offset, 'published_snapshot'))
  })

  it('keeps one product card when signed provenance has alternatives', () => {
    const directory = parseDirectoryData(signedFixture(), 'published_snapshot')
    assert.equal(directory.plugins.length, 1)
    assert.equal(directory.plugins[0]?.distributions.length, 2)
    assert.deepEqual(directory.plugins.map(plugin => plugin.name), ['demo'])
  })

  it('requires publication identity only at the signed production boundary', () => {
    const raw = snapshotFixture as Record<string, unknown>
    const unresolved = structuredClone({ ...raw, snapshot_schema_version: undefined, sequence: undefined, generated_at: undefined, expires_at: undefined, schema_version: 1 }) as Record<string, unknown>
    ;(unresolved.distributions as Array<{ releases: Array<{ package_source: { revision: string | null } }> }>)[0]!.releases[0]!.package_source.revision = null
    const preview = parseDirectoryData(unresolved, 'review_preview')
    assert.equal(preview.data_source, 'review_preview')
    assert.equal(preview.plugins[0]?.source.revision, null)
    assert.throws(() => parseDirectoryData(unresolved, 'published_snapshot'), /signed sequence, generated_at, and expires_at/)
    assert.throws(() => parseDirectoryData(fixture, 'published_snapshot'), /requires signed snapshot products and distributions/)
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
    assert.equal(validationLabel(registry.plugins[0]!), 'No current evidence')
    assert.equal(authenticationLabel(expectedDistribution(registry.plugins[0]!, ['cursor']), ['cursor'], registry.plugins[0]!.authentication), 'No account required')
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
