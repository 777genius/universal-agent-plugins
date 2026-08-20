import type {
  ClientEvidence,
  ClientID,
  ComponentID,
  DistributionKind,
  DistributionView,
  PluginAuthor,
  PluginIcon,
  PluginSource,
  RegistryIndex,
  RegistryPlugin,
} from '../types/registry'

const REPOSITORY = /^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?\/[a-z0-9](?:[a-z0-9._-]{0,98}[a-z0-9])?$/
const REVISION = /^[a-f0-9]{40}$/
const DIGEST = /^sha256:[a-f0-9]{64}$/
const PLUGIN_NAME = /^(?!.*(?:--|\.\.))[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/
const COMPONENTS = new Set<ComponentID>(['extensions', 'mcp', 'skills'])
const CLIENTS = new Set<ClientID>(['codex', 'chatgpt', 'cursor', 'copilot', 'vscode', 'kiro'])
const KINDS = new Set<DistributionKind>(['upstream', 'community_bridge', 'community'])

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function requiredString(item: Record<string, unknown>, field: string, context: string): string {
  const value = item[field]
  if (typeof value !== 'string' || value.length === 0) throw new Error(`${context}: ${field} must be a non-empty string`)
  return value
}

function optionalString(item: Record<string, unknown>, field: string): string | undefined {
  return typeof item[field] === 'string' && item[field] ? item[field] : undefined
}

function stringArray(value: unknown, field: string, context: string): string[] {
  if (!Array.isArray(value) || value.some(entry => typeof entry !== 'string' || entry.length === 0)) {
    throw new Error(`${context}: ${field} must be an array of non-empty strings`)
  }
  const values = value as string[]
  if (new Set(values).size !== values.length) throw new Error(`${context}: ${field} must be unique`)
  return values
}

function digestValue(value: unknown, context: string): string {
  if (typeof value !== 'string' || !DIGEST.test(value)) throw new Error(`${context} must be a sha256 digest`)
  return value
}

function author(value: unknown, context: string): PluginAuthor {
  if (!record(value)) throw new Error(`${context}: author must be an object`)
  const result: PluginAuthor = { name: requiredString(value, 'name', `${context} author`) }
  for (const field of ['email', 'url'] as const) {
    const parsed = optionalString(value, field)
    if (parsed) result[field] = parsed
  }
  return result
}

function source(value: unknown, context: string): PluginSource {
  if (!record(value)) throw new Error(`${context}: source must be an object`)
  const repository = requiredString(value, 'repository', `${context} source`)
  const revision = requiredString(value, 'revision', `${context} source`)
  if (!REPOSITORY.test(repository)) throw new Error(`${context}: source repository is invalid`)
  if (!REVISION.test(revision)) throw new Error(`${context}: source revision must be a full commit SHA`)
  return {
    repository,
    revision,
    path: requiredString(value, 'path', `${context} source`),
    manifest_sha256: digestValue(value.manifest_sha256 ?? value.manifest_digest, `${context} source manifest digest`),
    tree_sha256: digestValue(value.tree_sha256 ?? value.tree_digest, `${context} source tree digest`),
    ...(value.icon_sha256 === undefined ? {} : { icon_sha256: digestValue(value.icon_sha256, `${context} source icon digest`) }),
  }
}

function icon(value: unknown, context: string): PluginIcon {
  if (!record(value)) throw new Error(`${context}: icon must be an object`)
  return { path: requiredString(value, 'path', `${context} icon`), sha256: digestValue(value.sha256, `${context} icon digest`) }
}

function clientIDs(value: unknown, context: string): ClientID[] {
  const values = stringArray(value, 'clients', context)
  if (values.some(client => !CLIENTS.has(client as ClientID))) throw new Error(`${context}: contains an invalid client`)
  return values as ClientID[]
}

const legacyDelivery: RegistryPlugin['client_support']['delivery'] = {
  codex: 'prepared',
  chatgpt: 'manual_activation_required',
  cursor: 'installed',
  copilot: 'installed',
  vscode: 'prepared',
  kiro: 'manual_activation_required',
}

function legacyAuthentication(name: string): RegistryPlugin['authentication'] {
  if (['agent-code-navigator', 'chrome-devtools', 'cloudflare-docs', 'context7'].includes(name)) return 'none'
  if (['docker-hub', 'firebase', 'hubspot-developer'].includes(name)) return 'client_managed'
  if (['atlassian', 'cloudflare', 'cloudflare-bindings', 'cloudflare-observability', 'cloudflare-radar', 'figma', 'github', 'gitlab', 'greptile', 'heroku', 'hubspot-crm', 'linear', 'neon', 'notion', 'sentry', 'statsig', 'stripe', 'supabase', 'vercel'].includes(name)) return 'oauth'
  return 'unknown'
}

function legacyEvidence(value: unknown, context: string): ClientEvidence[] {
  if (!record(value)) throw new Error(`${context}: validation must be an object`)
  if (value.schema !== 'agent-plugins-1.0') throw new Error(`${context}: validation schema is invalid`)
  const runtime = clientIDs(value.runtime_evidence, `${context} runtime evidence`)
  return runtime.map(client => ({
    client,
    level: 'runtime',
    outcome: 'passed',
    evidence_url: 'docs/VERIFICATION.md',
  }))
}

function parseLegacyIndex(input: Record<string, unknown>): RegistryIndex {
  if (input.schema_version !== 1 || !Array.isArray(input.plugins)) {
    throw new Error('registry index must have schema_version 1 and a plugins array')
  }
  const names = new Set<string>()
  const plugins = input.plugins.map((raw, index): RegistryPlugin => {
    const context = `registry plugin ${index}`
    if (!record(raw)) throw new Error(`${context}: item must be an object`)
    const name = requiredString(raw, 'name', context)
    if (!PLUGIN_NAME.test(name)) throw new Error(`${context}: invalid name ${name}`)
    if (names.has(name)) throw new Error(`${context}: duplicate name ${name}`)
    names.add(name)
    if (typeof raw.built_in !== 'boolean') throw new Error(`${context}: built_in must be a boolean`)
    const parsedSource = source(raw.source, context)
    const installSource = requiredString(raw, 'install_source', context)
    const expectedSource = `${parsedSource.repository}@${parsedSource.revision}//${parsedSource.path}`
    if (raw.built_in ? installSource !== name : installSource !== expectedSource) {
      throw new Error(`${context}: ${raw.built_in ? 'built-in install_source must equal its name' : 'external install_source must use source repository@40-char-sha//path'}`)
    }
    if (!record(raw.client_support)) throw new Error(`${context}: client_support must be an object`)
    const resolution = raw.client_support.resolution
    if (resolution !== 'catalog' && resolution !== 'install_time') throw new Error(`${context}: client support resolution is invalid`)
    if ((raw.built_in && resolution !== 'catalog') || (!raw.built_in && resolution !== 'install_time')) {
      throw new Error(`${context}: client support resolution does not match source type`)
    }
    const compatibleClients = clientIDs(raw.client_support.clients, `${context} client support`)
    if (!compatibleClients.length) throw new Error(`${context}: at least one compatible client is required`)
    const components = stringArray(raw.components, 'components', context) as ComponentID[]
    if (components.some(component => !COMPONENTS.has(component))) throw new Error(`${context}: unsupported component`)
    const evidence = legacyEvidence(raw.validation, context)
    const distributionID = raw.built_in ? `777genius/${name}` : parsedSource.repository
    const kind: DistributionKind = raw.built_in ? 'community' : 'direct'
    const version = requiredString(raw, 'version', context)
    const distribution: DistributionView = {
      id: distributionID,
      kind,
      label: kind === 'community' ? 'Community package' : 'Direct source',
      publisher: author(raw.author, context).name,
      source: parsedSource,
      version,
      compatible_clients: compatibleClients,
      evidence,
    }
    return {
      name,
      display_name: name,
      version,
      description: requiredString(raw, 'description', context),
      author: author(raw.author, context),
      license: requiredString(raw, 'license', context),
      categories: stringArray(raw.categories, 'categories', context),
      keywords: stringArray(raw.keywords, 'keywords', context),
      source: parsedSource,
      install_source: installSource,
      built_in: raw.built_in,
      components,
      ...(raw.icon === undefined ? {} : { icon: icon(raw.icon, context) }),
      default_distribution: distributionID,
      distributions: [distribution],
      evidence,
      authentication: legacyAuthentication(name),
      client_support: {
        resolution: raw.built_in ? 'directory' : 'install_time',
        clients: compatibleClients,
        delivery: legacyDelivery,
        chatgpt_binding: compatibleClients.includes('chatgpt'),
      },
    }
  })
  return { schema_version: 1, data_source: 'legacy_compatibility', plugins }
}

function evidenceFromSnapshot(input: unknown, distributionID: string, releaseSequence: number): ClientEvidence[] {
  if (!Array.isArray(input)) return []
  return input.flatMap((item): ClientEvidence[] => {
    if (!record(item)) return []
    if (item.distribution_id !== distributionID || item.release_sequence !== releaseSequence) return []
    if (!CLIENTS.has(item.client as ClientID)) return []
    const level = item.level
    const outcome = item.outcome
    if (!['schema', 'materialization', 'discovery', 'runtime', 'oauth'].includes(String(level))) return []
    if (!['passed', 'failed', 'inconclusive', 'not_tested', 'not_applicable'].includes(String(outcome))) return []
    return [{
      client: item.client as ClientID,
      level: level as ClientEvidence['level'],
      outcome: outcome as ClientEvidence['outcome'],
      client_version: optionalString(item, 'client_version'),
      os: optionalString(item, 'os'),
      architecture: optionalString(item, 'architecture'),
      tested_at: optionalString(item, 'tested_at') ?? optionalString(item, 'timestamp'),
      evidence_url: optionalString(item, 'evidence_url'),
    }]
  })
}

function parseSnapshot(input: Record<string, unknown>, mode: 'published_snapshot' | 'review_preview'): RegistryIndex {
  if (input.schema_version !== 1 || !Array.isArray(input.products) || !Array.isArray(input.distributions)) {
    throw new Error('Directory data must have schema_version 1, products, and distributions')
  }
  if (mode === 'published_snapshot' && (!Number.isInteger(input.snapshot_sequence) || typeof input.generated_at !== 'string')) {
    throw new Error('published snapshot requires snapshot_sequence and generated_at')
  }
  const distributionRecords = new Map<string, Record<string, unknown>>()
  for (const raw of input.distributions) {
    if (!record(raw)) throw new Error('Directory distribution must be an object')
    const id = requiredString(raw, 'id', 'Directory distribution')
    if (distributionRecords.has(id)) throw new Error(`duplicate distribution ${id}`)
    distributionRecords.set(id, raw)
  }
  const seen = new Set<string>()
  const plugins = input.products.map((raw, index): RegistryPlugin => {
    const context = `Directory product ${index}`
    if (!record(raw)) throw new Error(`${context} must be an object`)
    const name = requiredString(raw, 'id', context)
    if (!PLUGIN_NAME.test(name) || seen.has(name)) throw new Error(`${context}: invalid or duplicate id ${name}`)
    seen.add(name)
    const defaultID = requiredString(raw, 'default_distribution', context)
    const listed = stringArray(raw.distributions, 'distributions', context)
    if (!listed.includes(defaultID)) throw new Error(`${context}: default distribution is not listed`)
    const distributions = listed.map((id): DistributionView => {
      const item = distributionRecords.get(id)
      if (!item || item.product_id !== name) throw new Error(`${context}: missing or mismatched distribution ${id}`)
      const kind = requiredString(item, 'kind', `distribution ${id}`) as DistributionKind
      if (!KINDS.has(kind)) throw new Error(`distribution ${id}: unsupported kind`)
      if (!Array.isArray(item.releases) || !item.releases.length) throw new Error(`distribution ${id}: releases are required`)
      const activeReleases = item.releases.filter(record).filter(release => release.status === undefined || release.status === 'active')
      const release = activeReleases.sort((a, b) => Number(b.sequence) - Number(a.sequence))[0]
      if (!release || !Number.isInteger(release.sequence)) throw new Error(`distribution ${id}: active release sequence is required`)
      const packageSource = source({
        ...(record(release.package_source) ? release.package_source : {}),
        manifest_digest: release.manifest_digest,
        tree_digest: release.tree_digest,
      }, `distribution ${id}`)
      const policies = Array.isArray(item.release_policies) ? item.release_policies.filter(record) : []
      const policy = policies.find(candidate => candidate.release_sequence === release.sequence) ?? release
      const compatible = clientIDs(policy.compatible_clients ?? [], `distribution ${id} compatible clients`)
      if (!compatible.length) throw new Error(`distribution ${id}: at least one compatible client is required`)
      const evidence = evidenceFromSnapshot(input.verification_summaries ?? input.current_verification, id, release.sequence as number)
      return {
        id,
        kind,
        label: kind === 'upstream' ? 'Upstream package' : 'Community package',
        publisher: optionalString(item, 'publisher') ?? optionalString(item, 'packager') ?? id.split('/')[0]!,
        source: packageSource,
        release_sequence: release.sequence as number,
        version: optionalString(release, 'version') ?? optionalString(release, 'package_version') ?? 'unversioned',
        compatible_clients: compatible,
        evidence,
      }
    })
    const selected = distributions.find(item => item.id === defaultID)!
    const components = stringArray(raw.components ?? raw.component_inventory ?? [], 'components', context) as ComponentID[]
    if (components.some(component => !COMPONENTS.has(component))) throw new Error(`${context}: unsupported component`)
    const productAuthor = record(raw.author) ? author(raw.author, context) : { name: selected.publisher }
    const auth = raw.authentication
    return {
      name,
      display_name: optionalString(raw, 'display_name') ?? name,
      version: selected.version,
      description: requiredString(raw, 'description', context),
      author: productAuthor,
      license: optionalString(raw, 'license') ?? 'See source',
      categories: stringArray(raw.categories ?? [], 'categories', context),
      keywords: stringArray(raw.keywords ?? [], 'keywords', context),
      source: selected.source,
      install_source: name,
      built_in: true,
      components,
      ...(raw.icon === undefined ? {} : { icon: icon(raw.icon, context) }),
      default_distribution: defaultID,
      distributions,
      evidence: selected.evidence,
      authentication: ['none', 'client_managed', 'oauth', 'unknown'].includes(String(auth)) ? auth as RegistryPlugin['authentication'] : 'unknown',
      client_support: {
        resolution: 'directory',
        clients: [...new Set(distributions.flatMap(item => item.compatible_clients))],
        delivery: record(raw.delivery) ? raw.delivery as RegistryPlugin['client_support']['delivery'] : legacyDelivery,
        chatgpt_binding: raw.chatgpt_binding === true,
      },
    }
  })
  return {
    schema_version: 1,
    data_source: mode,
    ...(typeof input.snapshot_sequence === 'number' ? { snapshot_sequence: input.snapshot_sequence } : {}),
    ...(typeof input.generated_at === 'string' ? { generated_at: input.generated_at } : {}),
    plugins,
  }
}

export function parseDirectoryData(input: unknown, mode?: 'published_snapshot' | 'review_preview'): RegistryIndex {
  if (!record(input)) throw new Error('Directory data must be an object')
  if ('plugins' in input) return parseLegacyIndex(input)
  return parseSnapshot(input, mode ?? 'review_preview')
}

export const parseRegistryIndex = parseDirectoryData

export function isPinnedExternalSource(value: string): boolean {
  const match = /^([^@]+)@([a-f0-9]{40})\/\/(.+)$/.exec(value)
  return Boolean(match && REPOSITORY.test(match[1]!) && match[3]!.length > 0)
}

export function evidenceLabel(value: ClientEvidence): string {
  const level = value.level === 'oauth' ? 'OAuth' : value.level[0]!.toUpperCase() + value.level.slice(1)
  const outcome = value.outcome === 'not_tested' ? 'not tested' : value.outcome.replace('_', ' ')
  return `${level} ${outcome}`
}

export function validationLabel(plugin: RegistryPlugin): string {
  const passed = plugin.evidence.filter(item => item.outcome === 'passed')
  const hasEnvironment = (item: ClientEvidence) => Boolean(item.client_version && item.os && item.architecture && item.tested_at)
  if (passed.some(item => item.level === 'oauth' && hasEnvironment(item))) return 'OAuth tested'
  if (passed.some(item => item.level === 'runtime' && hasEnvironment(item))) return 'Runtime tested'
  if (passed.some(item => item.level === 'discovery' && hasEnvironment(item))) return 'Discovery tested'
  if (passed.some(item => item.level === 'materialization' && hasEnvironment(item))) return 'Materialization tested'
  return 'Schema validated'
}

export function defaultDistribution(plugin: RegistryPlugin): DistributionView {
  const distribution = plugin.distributions.find(item => item.id === plugin.default_distribution)
  if (!distribution) throw new Error(`${plugin.name}: default distribution is unavailable`)
  return distribution
}

export function expectedDistribution(plugin: RegistryPlugin, targets: readonly ClientID[]): DistributionView | undefined {
  const supportsAll = (distribution: DistributionView) => targets.every(target => distribution.compatible_clients.includes(target))
  const declared = defaultDistribution(plugin)
  if (supportsAll(declared)) return declared
  const priority: Record<DistributionKind, number> = { upstream: 0, community_bridge: 1, community: 2, direct: 3 }
  return plugin.distributions.filter(supportsAll).sort((a, b) => priority[a.kind] - priority[b.kind])[0]
}

export function githubSourceUrl(plugin: RegistryPlugin, distribution = defaultDistribution(plugin)): string {
  const path = distribution.source.path.split('/').map(encodeURIComponent).join('/')
  return `https://github.com/${distribution.source.repository}/tree/${distribution.source.revision}/${path}`
}

export function mirroredIconPath(plugin: RegistryPlugin): string | undefined {
  if (!plugin.built_in || !plugin.icon || !plugin.icon.path.startsWith('assets/plugin-icons/')) return undefined
  const filename = plugin.icon.path.split('/').at(-1)
  return filename ? `plugin-icons/${filename}` : undefined
}
