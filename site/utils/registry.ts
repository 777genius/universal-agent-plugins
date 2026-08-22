import type {
  ClientEvidence,
  ClientID,
  ComponentID,
  DistributionReleaseView,
  DistributionResolution,
  DistributionKind,
  DistributionView,
  PluginAuthor,
  PluginIcon,
  PackageEvidence,
  PluginSource,
  ReleaseTarget,
  TargetAuthentication,
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
const RFC3339_INSTANT = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:\d{2})$/

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

interface ParsedInstant {
  epochSeconds: number
  fraction: string
}

function parseRFC3339Instant(value: unknown, context: string): ParsedInstant {
  if (typeof value !== 'string') throw new Error(`${context} must be a strict RFC3339 instant`)
  const match = RFC3339_INSTANT.exec(value)
  if (!match) throw new Error(`${context} must be a strict RFC3339 instant`)
  const [, yearText, monthText, dayText, hourText, minuteText, secondText, fraction = '', zone] = match
  const [year, month, day, hour, minute, second] = [yearText, monthText, dayText, hourText, minuteText, secondText].map(Number)
  const leapYear = year! % 4 === 0 && (year! % 100 !== 0 || year! % 400 === 0)
  const monthDays = [31, leapYear ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
  if (month! < 1 || month! > 12 || day! < 1 || day! > monthDays[month! - 1]!
    || hour! > 23 || minute! > 59 || second! > 59) {
    throw new Error(`${context} must be a real RFC3339 calendar instant`)
  }
  let offsetSeconds = 0
  if (zone !== 'Z') {
    const offsetHour = Number(zone!.slice(1, 3))
    const offsetMinute = Number(zone!.slice(4, 6))
    if (offsetHour > 23 || offsetMinute > 59) throw new Error(`${context} has an invalid RFC3339 offset`)
    offsetSeconds = (offsetHour * 60 + offsetMinute) * 60 * (zone![0] === '+' ? 1 : -1)
  }
  const calendar = new Date(0)
  calendar.setUTCFullYear(year!, month! - 1, day!)
  calendar.setUTCHours(hour!, minute!, second!, 0)
  return { epochSeconds: calendar.getTime() / 1000 - offsetSeconds, fraction }
}

function compareInstants(left: ParsedInstant, right: ParsedInstant): number {
  if (left.epochSeconds !== right.epochSeconds) return left.epochSeconds - right.epochSeconds
  const width = Math.max(left.fraction.length, right.fraction.length)
  return left.fraction.padEnd(width, '0').localeCompare(right.fraction.padEnd(width, '0'))
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

function source(value: unknown, context: string, allowUnresolved = false): PluginSource {
  if (!record(value)) throw new Error(`${context}: source must be an object`)
  const repository = requiredString(value, 'repository', `${context} source`)
  const revision = value.revision === null && allowUnresolved ? null : requiredString(value, 'revision', `${context} source`)
  if (!REPOSITORY.test(repository)) throw new Error(`${context}: source repository is invalid`)
  if (revision !== null && !REVISION.test(revision)) throw new Error(`${context}: source revision must be a full commit SHA`)
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
  chatgpt: 'manual_activation',
  cursor: 'managed',
  copilot: 'managed',
  vscode: 'prepared',
  kiro: 'manual_activation',
}

function legacyAuthentication(name: string): RegistryPlugin['authentication'] {
  if (['agent-code-navigator', 'chrome-devtools', 'cloudflare-docs', 'context7'].includes(name)) return 'none'
  if (['docker-hub', 'firebase', 'hubspot-developer'].includes(name)) return 'client_managed'
  if (['atlassian', 'cloudflare', 'cloudflare-bindings', 'cloudflare-observability', 'cloudflare-radar', 'figma', 'github', 'gitlab', 'greptile', 'heroku', 'hubspot-crm', 'linear', 'neon', 'notion', 'sentry', 'statsig', 'stripe', 'supabase', 'vercel'].includes(name)) return 'oauth'
  return 'unknown'
}

function legacyTargetAuthentication(authentication: RegistryPlugin['authentication']): TargetAuthentication {
  if (authentication === 'none') return 'not_required'
  if (authentication === 'oauth' || authentication === 'client_managed') return 'required'
  return 'unknown'
}

function legacyEvidence(value: unknown, context: string): ClientEvidence[] {
  if (!record(value)) throw new Error(`${context}: validation must be an object`)
  if (value.schema !== 'agent-plugins-1.0') throw new Error(`${context}: validation schema is invalid`)
  const runtime = clientIDs(value.runtime_evidence, `${context} runtime evidence`)
  return runtime.map(client => ({
    id: `legacy-${client}-runtime`,
    client,
    level: 'runtime',
    outcome: 'passed',
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
    const authentication = legacyAuthentication(name)
    const distribution: DistributionView = {
      id: distributionID,
      kind,
      label: kind === 'community' ? 'Community package' : 'Direct source',
      publisher: author(raw.author, context).name,
      source: parsedSource,
      version,
      compatible_clients: compatibleClients,
      evidence,
      package_evidence: [],
      status: 'active',
      release_status: 'active',
      selectable: true,
      targets: compatibleClients.map(client => ({ client, authentication: legacyTargetAuthentication(authentication), delivery: legacyDelivery[client]!, scopes: ['user'] })),
      components,
      releases: [],
    }
    distribution.releases = [{
      release_sequence: 1,
      source: parsedSource,
      version,
      targets: distribution.targets,
      components,
      evidence,
      package_evidence: [],
      release_status: 'active',
      selectable: true,
      blocking_clients: [],
      materialized_clients: compatibleClients,
      meets_minimum_capabilities: true,
    }]
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
      installable: true,
      components,
      ...(raw.icon === undefined ? {} : { icon: icon(raw.icon, context) }),
      default_distribution: distributionID,
      declared_default_distribution: distributionID,
      distributions: [distribution],
      evidence,
      package_evidence: [],
      authentication,
      client_support: {
        resolution: raw.built_in ? 'directory' : 'install_time',
        clients: compatibleClients,
        delivery: legacyDelivery,
        scopes: Object.fromEntries(compatibleClients.map(client => [client, ['user']])),
        app_bindings: {},
      },
    }
  })
  return { schema_version: 1, data_source: 'legacy_compatibility', plugins }
}

function releaseTargets(value: unknown, context: string): ReleaseTarget[] {
  if (!Array.isArray(value) || !value.length) throw new Error(`${context}: targets are required`)
  const seen = new Set<ClientID>()
  return value.map((raw, index): ReleaseTarget => {
    if (!record(raw)) throw new Error(`${context}: target ${index} must be an object`)
    const client = requiredString(raw, 'client', `${context} target ${index}`) as ClientID
    if (!CLIENTS.has(client) || seen.has(client)) throw new Error(`${context}: target ${index} has an invalid or duplicate client`)
    seen.add(client)
    const delivery = requiredString(raw, 'delivery', `${context} target ${client}`)
    if (!['managed', 'prepared', 'manual_activation'].includes(delivery)) throw new Error(`${context}: target ${client} has invalid delivery`)
    const authentication = requiredString(raw, 'authentication', `${context} target ${client}`) as TargetAuthentication
    if (!['not_required', 'required', 'unknown'].includes(authentication)) throw new Error(`${context}: target ${client} has invalid authentication`)
    const scopes = stringArray(raw.scopes, 'scopes', `${context} target ${client}`)
    let appBinding: ReleaseTarget['app_binding']
    if (record(raw.app_binding)) {
      appBinding = {
        app_key: requiredString(raw.app_binding, 'app_key', `${context} target ${client} app binding`),
        id: requiredString(raw.app_binding, 'id', `${context} target ${client} app binding`),
        mcp_server: requiredString(raw.app_binding, 'mcp_server', `${context} target ${client} app binding`),
      }
    }
    if (client === 'chatgpt' && !appBinding) throw new Error(`${context}: ChatGPT target requires app_binding`)
    if (client !== 'chatgpt' && appBinding) throw new Error(`${context}: app_binding is valid only for ChatGPT`)
    return { client, authentication, delivery: delivery as ReleaseTarget['delivery'], scopes, ...(appBinding ? { app_binding: appBinding } : {}) }
  })
}

function evidenceFromSnapshot(input: unknown, distributionID: string, releaseSequence: number, treeDigest: string, selectedIDs: readonly string[]): { client: ClientEvidence[], package: PackageEvidence[] } {
  const client: ClientEvidence[] = []
  const packageEvidence: PackageEvidence[] = []
  if (!Array.isArray(input)) return { client, package: packageEvidence }
  for (const item of input) {
    if (!record(item)) continue
    if (item.distribution_id !== distributionID || item.release_sequence !== releaseSequence) continue
    if (item.package_tree_digest !== treeDigest) continue
    if (!selectedIDs.includes(String(item.id))) continue
    const level = item.level
    const outcome = item.outcome
    if (!['schema', 'materialization', 'discovery', 'runtime', 'oauth'].includes(String(level))) continue
    if (!['passed', 'failed', 'inconclusive', 'not_tested', 'not_applicable'].includes(String(outcome))) continue
    if (!record(item.artifact)
      || typeof item.artifact.repository !== 'string' || !REPOSITORY.test(item.artifact.repository)
      || typeof item.artifact.revision !== 'string' || !REVISION.test(item.artifact.revision)
      || typeof item.artifact.path !== 'string' || !item.artifact.path
      || typeof item.artifact.digest !== 'string' || !DIGEST.test(item.artifact.digest)) continue
    const artifact = {
      repository: item.artifact.repository,
      revision: item.artifact.revision,
      path: item.artifact.path,
      digest: item.artifact.digest,
      url: `https://github.com/${item.artifact.repository}/blob/${item.artifact.revision}/${item.artifact.path}`,
    }
    const common = {
      id: String(item.id),
      outcome: outcome as ClientEvidence['outcome'],
      package_tree_digest: treeDigest,
      ...(optionalString(item, 'observed_at') ? { tested_at: optionalString(item, 'observed_at') } : {}),
      artifact,
    }
    if (level === 'schema') {
      if (item.client !== undefined) continue
      packageEvidence.push({ ...common, level: 'schema' })
      continue
    }
    const evidenceClient = item.client as ClientID
    const clientVersion = optionalString(item, 'client_version')
    const installerVersion = optionalString(item, 'installer_version')
    const os = optionalString(item, 'os')
    const architecture = optionalString(item, 'architecture')
    const testedAt = optionalString(item, 'observed_at')
    if (!CLIENTS.has(evidenceClient) || !clientVersion || !installerVersion || !os || !architecture || !testedAt) continue
    client.push({
      ...common,
      client: evidenceClient,
      level: level as ClientEvidence['level'],
      client_version: clientVersion,
      installer_version: installerVersion,
      os,
      architecture,
      tested_at: testedAt,
      ...(optionalString(item, 'dependency_identity') ? { dependency_identity: optionalString(item, 'dependency_identity') } : {}),
    })
  }
  return { client, package: packageEvidence }
}

function parseSnapshot(input: Record<string, unknown>, mode: 'published_snapshot' | 'review_preview'): RegistryIndex {
  const isSigned = input.snapshot_schema_version === 1
  if ((!isSigned && input.schema_version !== 1) || !Array.isArray(input.products) || !Array.isArray(input.distributions)) {
    throw new Error('Directory data must have schema version 1, products, and distributions')
  }
  const snapshotSequence = isSigned ? input.sequence : input.snapshot_sequence
  if (mode === 'published_snapshot' && (!isSigned || !Number.isInteger(snapshotSequence) || typeof input.generated_at !== 'string' || typeof input.expires_at !== 'string')) {
    throw new Error('published snapshot requires one signed sequence, generated_at, and expires_at')
  }
  let generatedAt: string | undefined
  let expiresAt: string | undefined
  if (input.generated_at !== undefined || input.expires_at !== undefined || isSigned) {
    const generated = parseRFC3339Instant(input.generated_at, 'Directory generated_at')
    const expires = parseRFC3339Instant(input.expires_at, 'Directory expires_at')
    if (compareInstants(expires, generated) <= 0) throw new Error('Directory expires_at must be after generated_at')
    generatedAt = input.generated_at as string
    expiresAt = input.expires_at as string
  }
  const distributionRecords = new Map<string, Record<string, unknown>>()
  for (const raw of input.distributions) {
    if (!record(raw)) throw new Error('Directory distribution must be an object')
    const id = requiredString(raw, 'id', 'Directory distribution')
    if (distributionRecords.has(id)) throw new Error(`duplicate distribution ${id}`)
    distributionRecords.set(id, raw)
  }
  const revoked = new Set((Array.isArray(input.revocations) ? input.revocations : []).filter(record).map(item => `${String(item.distribution_id)}:${String(item.release_sequence)}`))
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
    if (!record(raw.minimum_capabilities)) throw new Error(`${context}: minimum_capabilities must be an object`)
    const minimumCapabilities = raw.minimum_capabilities
    const requiredComponents = new Set<ComponentID>([...COMPONENTS].filter(component => minimumCapabilities[component] === 'required'))
    const distributions = listed.map((id): DistributionView => {
      const item = distributionRecords.get(id)
      if (!item || item.product_id !== name) throw new Error(`${context}: missing or mismatched distribution ${id}`)
      const kind = requiredString(item, 'kind', `distribution ${id}`) as DistributionKind
      if (!KINDS.has(kind)) throw new Error(`distribution ${id}: unsupported kind`)
      if (!Array.isArray(item.releases) || !item.releases.length) throw new Error(`distribution ${id}: releases are required`)
      const status = requiredString(item, 'status', `distribution ${id}`)
      if (!['candidate', 'active', 'suspended'].includes(status)) throw new Error(`distribution ${id}: unsupported status`)
      const policies = Array.isArray(item.release_policies) ? item.release_policies.filter(record) : []
      const releases = item.releases.filter(record).sort((a, b) => Number(b.sequence) - Number(a.sequence))
      const releaseSequences = releases.map(release => release.sequence)
      if (new Set(releaseSequences).size !== releaseSequences.length) throw new Error(`distribution ${id}: release sequences must be unique`)
      const releaseViews = releases.map((release): DistributionReleaseView => {
        if (!Number.isInteger(release.sequence) || Number(release.sequence) < 1) throw new Error(`distribution ${id}: release sequence must be a positive integer`)
        const releaseSequence = release.sequence as number
        const packageSource = source({
          ...(record(release.package_source) ? release.package_source : {}),
          manifest_digest: release.manifest_digest,
          tree_digest: release.tree_digest,
        }, `distribution ${id}`, mode === 'review_preview')
        const policy = policies.find(candidate => candidate.release_sequence === releaseSequence)
        if (!policy) throw new Error(`distribution ${id}: release ${String(releaseSequence)} has no signed policy`)
        const targets = releaseTargets(policy.targets, `distribution ${id} release ${String(releaseSequence)}`)
        const policyStatus = requiredString(policy, 'status', `distribution ${id} release policy`)
        if (!['active', 'superseded', 'revoked'].includes(policyStatus)) throw new Error(`distribution ${id}: unsupported release status`)
        const releaseStatus = revoked.has(`${id}:${String(releaseSequence)}`) ? 'revoked' : policyStatus as DistributionView['release_status']
        if (!Array.isArray(policy.current_evidence) || policy.current_evidence.some(value => typeof value !== 'string')) throw new Error(`distribution ${id}: current_evidence must be an array of evidence IDs`)
        const treeDigest = digestValue(release.tree_digest, `distribution ${id} tree digest`)
        const evidence = evidenceFromSnapshot(input.evidence ?? input.verification_summaries ?? input.current_verification, id, releaseSequence, treeDigest, policy.current_evidence as string[])
        const components = stringArray(release.components ?? [], 'components', `distribution ${id}`) as ComponentID[]
        if (components.some(component => !COMPONENTS.has(component))) throw new Error(`distribution ${id}: unsupported component`)
        const blockingClients = [...new Set(evidence.client.filter(observation => observation.outcome === 'failed' && ['materialization', 'discovery', 'runtime'].includes(observation.level)).map(observation => observation.client))]
        const materializedClients = [...new Set(evidence.client.filter(observation => observation.level === 'materialization' && observation.outcome === 'passed').map(observation => observation.client))]
        const meetsMinimumCapabilities = [...requiredComponents].every(component => components.includes(component))
        return {
          release_sequence: releaseSequence,
          source: packageSource,
          version: optionalString(release, 'version') ?? optionalString(release, 'package_version') ?? 'unversioned',
          targets,
          components,
          evidence: evidence.client,
          package_evidence: evidence.package,
          release_status: releaseStatus as DistributionView['release_status'],
          selectable: status === 'active' && releaseStatus === 'active' && meetsMinimumCapabilities,
          blocking_clients: blockingClients,
          materialized_clients: materializedClients,
          meets_minimum_capabilities: meetsMinimumCapabilities,
        }
      })
      const eligibleTargets = (release: DistributionReleaseView) => release.selectable
        ? release.targets.filter(target => !release.blocking_clients.includes(target.client) && (kind !== 'upstream' || release.materialized_clients.includes(target.client)))
        : []
      const selectedRelease = releaseViews.find(release => eligibleTargets(release).length > 0) ?? releaseViews[0]!
      const compatible = [...new Set(releaseViews.flatMap(release => eligibleTargets(release).map(target => target.client)))]
      return {
        id,
        kind,
        label: kind === 'upstream'
          ? 'Upstream package'
          : kind === 'community_bridge'
            ? 'Community bridge'
            : 'Community package',
        publisher: optionalString(item, 'publisher') ?? optionalString(item, 'packager') ?? id.split('/')[0]!,
        source: selectedRelease.source,
        release_sequence: selectedRelease.release_sequence,
        version: selectedRelease.version,
        compatible_clients: compatible,
        evidence: selectedRelease.evidence,
        package_evidence: selectedRelease.package_evidence,
        status: status as DistributionView['status'],
        release_status: selectedRelease.release_status,
        selectable: compatible.length > 0,
        targets: selectedRelease.targets,
        components: selectedRelease.components,
        releases: releaseViews,
      }
    })
    const declared = distributions.find(item => item.id === defaultID)!
    const priority: Record<DistributionKind, number> = { upstream: 0, community_bridge: 1, community: 2, direct: 3 }
    const selected = declared.selectable ? declared : distributions.filter(item => item.selectable).sort((a, b) => priority[a.kind] - priority[b.kind] || a.id.localeCompare(b.id))[0] ?? declared
    const components = selected.components
    const productAuthor = record(raw.author) ? author(raw.author, context) : { name: selected.publisher }
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
      installable: selected.selectable,
      components,
      ...(raw.icon === undefined ? {} : { icon: icon(record(raw.icon) && raw.icon.sha256 === undefined ? { path: raw.icon.path, sha256: raw.icon.digest } : raw.icon, context) }),
      default_distribution: selected.id,
      declared_default_distribution: defaultID,
      ...(selected.id === defaultID ? {} : { default_fallback_reason: declared.status !== 'active' ? `Declared default is ${declared.status}` : `Declared default release is ${declared.release_status}` }),
      distributions,
      evidence: selected.evidence,
      package_evidence: selected.package_evidence,
      authentication: 'unknown',
      client_support: {
        resolution: 'directory',
        clients: [...new Set(distributions.filter(item => item.selectable).flatMap(item => item.compatible_clients))],
        delivery: Object.fromEntries(selected.targets.map(target => [target.client, target.delivery])),
        scopes: Object.fromEntries(selected.targets.map(target => [target.client, target.scopes])),
        app_bindings: Object.fromEntries(selected.targets.filter(target => target.app_binding).map(target => [target.client, target.app_binding!])),
      },
    }
  })
  return {
    schema_version: 1,
    data_source: mode,
    ...(typeof snapshotSequence === 'number' ? { snapshot_sequence: snapshotSequence } : {}),
    ...(generatedAt ? { generated_at: generatedAt } : {}),
    ...(expiresAt ? { expires_at: expiresAt } : {}),
    plugins,
  }
}

export function parseDirectoryData(input: unknown, mode?: 'published_snapshot' | 'review_preview'): RegistryIndex {
  if (!record(input)) throw new Error('Directory data must be an object')
  if ('plugins' in input) {
    if (mode === 'published_snapshot') throw new Error('published snapshot mode requires signed snapshot products and distributions')
    return parseLegacyIndex(input)
  }
  return parseSnapshot(input, mode ?? 'review_preview')
}

export const parseRegistryIndex = parseDirectoryData

export function isPinnedExternalSource(value: string): boolean {
  const match = /^([^@]+)@([a-f0-9]{40})\/\/(.+)$/.exec(value)
  return Boolean(match && REPOSITORY.test(match[1]!) && match[3]!.length > 0)
}

export function evidenceLabel(value: ClientEvidence | PackageEvidence): string {
  const level = value.level === 'oauth' ? 'OAuth' : value.level[0]!.toUpperCase() + value.level.slice(1)
  const outcome = value.outcome === 'not_tested' ? 'not tested' : value.outcome.replace('_', ' ')
  return `${level} ${outcome}`
}

export function validationLabel(view: Pick<RegistryPlugin, 'evidence' | 'package_evidence'>): string {
  const passed = view.evidence.filter(item => item.outcome === 'passed')
  const hasEnvironment = (item: ClientEvidence) => Boolean(item.client_version && item.os && item.architecture && item.tested_at)
  if (passed.some(item => item.level === 'oauth' && hasEnvironment(item))) return 'OAuth tested'
  if (passed.some(item => item.level === 'runtime' && hasEnvironment(item))) return 'Runtime tested'
  if (passed.some(item => item.level === 'discovery' && hasEnvironment(item))) return 'Discovery tested'
  if (passed.some(item => item.level === 'materialization' && hasEnvironment(item))) return 'Materialization tested'
  const schema = view.package_evidence[0]
  return schema ? evidenceLabel(schema) : 'No current evidence'
}

export function defaultDistribution(plugin: RegistryPlugin): DistributionView {
  const distribution = plugin.distributions.find(item => item.id === plugin.default_distribution)
  if (!distribution) throw new Error(`${plugin.name}: default distribution is unavailable`)
  return distribution
}

function eligibleRelease(distribution: DistributionView, targets: ReadonlySet<ClientID>): { release?: DistributionReleaseView, reason?: string } {
  if (distribution.status !== 'active') return { reason: `distribution is ${distribution.status}` }
  const reasons: string[] = []
  for (const release of [...distribution.releases].sort((a, b) => b.release_sequence - a.release_sequence)) {
    const supported = new Set(release.targets.map(target => target.client))
    if (release.release_status !== 'active') {
      reasons.push(`release ${release.release_sequence} is ${release.release_status}`)
    } else if (!release.meets_minimum_capabilities) {
      reasons.push(`release ${release.release_sequence} misses required components`)
    } else if ([...targets].some(target => !supported.has(target))) {
      const missing = [...targets].filter(target => !supported.has(target)).sort()
      reasons.push(`release ${release.release_sequence} does not support ${missing.join(',')}`)
    } else {
      const failures = [...targets].filter(target => release.blocking_clients.includes(target)).sort()
      if (failures.length) {
        reasons.push(`release ${release.release_sequence} has blocking trusted failure for ${failures.join(',')}`)
        continue
      }
      if (distribution.kind === 'upstream') {
        const missing = [...targets].filter(target => !release.materialized_clients.includes(target)).sort()
        if (missing.length) {
          reasons.push(`release ${release.release_sequence} lacks current positive package compatibility evidence (passed materialization) for ${missing.join(',')}`)
          continue
        }
      }
      return { release }
    }
  }
  return { reason: reasons.join('; ') || 'no releases' }
}

export function resolveDistribution(plugin: RegistryPlugin, targets: readonly ClientID[]): DistributionResolution {
  if (!targets.length || new Set(targets).size !== targets.length) return { unavailable_reason: 'targets must be unique supported client IDs' }
  const selectedTargets = new Set(targets)
  const resolved = (distribution: DistributionView): DistributionView | undefined => {
    const release = eligibleRelease(distribution, selectedTargets).release
    return release ? {
      ...distribution,
      source: release.source,
      release_sequence: release.release_sequence,
      version: release.version,
      compatible_clients: release.targets.filter(target => !release.blocking_clients.includes(target.client)).map(target => target.client),
      evidence: release.evidence,
      package_evidence: release.package_evidence,
      release_status: release.release_status,
      selectable: true,
      targets: release.targets,
      components: release.components,
    } : undefined
  }
  const declared = plugin.distributions.find(distribution => distribution.id === plugin.declared_default_distribution)
  if (!declared) throw new Error(`${plugin.name}: declared default distribution is unavailable`)
  const selectedDefault = resolved(declared)
  if (selectedDefault) return { distribution: selectedDefault }
  const defaultReason = eligibleRelease(declared, selectedTargets).reason ?? 'no releases'
  const priority: Record<DistributionKind, number> = { upstream: 0, community_bridge: 1, community: 2, direct: 3 }
  const alternatives = plugin.distributions.filter(item => item.id !== declared.id).sort((a, b) => priority[a.kind] - priority[b.kind] || a.id.localeCompare(b.id))
  const ineligibleReasons = [{ distribution_id: declared.id, reason: defaultReason }]
  for (const distribution of alternatives) {
    const release = resolved(distribution)
    if (release) {
      const fallbackReason = `declared default ${declared.id} was ineligible: ${defaultReason}`
      return { distribution: { ...release, fallback_reason: fallbackReason }, fallback_reason: fallbackReason }
    }
    ineligibleReasons.push({ distribution_id: distribution.id, reason: eligibleRelease(distribution, selectedTargets).reason ?? 'no releases' })
  }
  const targetList = [...selectedTargets].sort().join(',')
  return {
    unavailable_reason: `${plugin.name}: no eligible distribution supports the complete target set ${targetList}; ${ineligibleReasons.map(item => `${item.distribution_id}: ${item.reason}`).join('; ')}`,
    ineligible_reasons: ineligibleReasons,
  }
}

export function expectedDistribution(plugin: RegistryPlugin, targets: readonly ClientID[]): DistributionView | undefined {
  return resolveDistribution(plugin, targets).distribution
}

export function directoryIsExpired(registry: Pick<RegistryIndex, 'data_source' | 'expires_at'>, now = Date.now()): boolean {
  if (registry.data_source !== 'published_snapshot' || !registry.expires_at) return false
  const expiry = parseRFC3339Instant(registry.expires_at, 'Directory expires_at')
  return now >= expiry.epochSeconds * 1000 + Number(`0.${expiry.fraction || '0'}`) * 1000
}

export function deliveryLabel(delivery: ReleaseTarget['delivery']): string {
  if (delivery === 'managed') return 'Managed install'
  if (delivery === 'prepared') return 'Prepared; client import remains'
  return 'Manual activation required'
}

export function targetAuthenticationLabel(authentication: TargetAuthentication): string {
  if (authentication === 'not_required') return 'No account required'
  if (authentication === 'required') return 'Authentication required'
  return 'Check package requirements'
}

export function authenticationLabel(
  distribution: Pick<DistributionView, 'targets'> | undefined,
  selectedClients: readonly ClientID[],
  legacyAuthentication?: RegistryPlugin['authentication'],
): string {
  // The flat legacy catalog has only a product-wide value. Keep its established
  // labels, but never infer OAuth from a signed target's generic `required`.
  if (legacyAuthentication === 'none') return 'No account required'
  if (legacyAuthentication === 'oauth') return 'OAuth required'
  if (legacyAuthentication === 'client_managed') return 'Client-managed authentication'

  if (!distribution || !selectedClients.length) return 'Check package requirements'
  const selected = distribution.targets.filter(target => selectedClients.includes(target.client))
  if (selected.length !== selectedClients.length) return 'Check package requirements'
  const values = new Set(selected.map(target => target.authentication))
  if (values.size > 1) return 'Authentication varies'
  return targetAuthenticationLabel(selected[0]!.authentication)
}

export function githubSourceUrl(plugin: RegistryPlugin, distribution = defaultDistribution(plugin)): string {
  if (distribution.source.revision === null) return `https://github.com/${distribution.source.repository}`
  const path = distribution.source.path.split('/').map(encodeURIComponent).join('/')
  return `https://github.com/${distribution.source.repository}/tree/${distribution.source.revision}/${path}`
}

export function mirroredIconPath(plugin: RegistryPlugin): string | undefined {
  if (!plugin.built_in || !plugin.icon || !plugin.icon.path.startsWith('assets/plugin-icons/')) return undefined
  const filename = plugin.icon.path.split('/').at(-1)
  return filename ? `plugin-icons/${filename}` : undefined
}
