export interface PluginAuthor {
  name: string
  email?: string
  url?: string
}

export interface PluginSource {
  repository: string
  revision: string | null
  path: string
  manifest_sha256: string
  tree_sha256: string
  icon_sha256?: string
}

export type ClientID = 'codex' | 'chatgpt' | 'cursor' | 'copilot' | 'vscode' | 'kiro'
export type ComponentID = 'extensions' | 'mcp' | 'skills'
export type DistributionKind = 'upstream' | 'community_bridge' | 'community' | 'direct'
export type EvidenceLevel = 'schema' | 'materialization' | 'discovery' | 'runtime' | 'oauth'
export type EvidenceOutcome = 'passed' | 'failed' | 'inconclusive' | 'not_tested' | 'not_applicable'
export type DeliveryState = 'installed' | 'prepared' | 'manual_activation_required'

export interface PluginIcon {
  path: string
  sha256: string
}

export interface ClientEvidence {
  client: ClientID
  level: EvidenceLevel
  outcome: EvidenceOutcome
  client_version?: string
  os?: string
  architecture?: string
  tested_at?: string
  evidence_url?: string
}

export interface DistributionView {
  id: string
  kind: DistributionKind
  label: string
  publisher: string
  source: PluginSource
  release_sequence?: number
  version: string
  compatible_clients: ClientID[]
  evidence: ClientEvidence[]
}

/**
 * Stable site view model. Both the temporary flat catalog and the published
 * signed Directory snapshot are normalized into this shape at build time.
 */
export interface RegistryPlugin {
  name: string
  display_name: string
  version: string
  description: string
  author: PluginAuthor
  license: string
  categories: string[]
  keywords: string[]
  source: PluginSource
  install_source: string
  built_in: boolean
  components: ComponentID[]
  icon?: PluginIcon
  default_distribution: string
  distributions: DistributionView[]
  evidence: ClientEvidence[]
  authentication: 'none' | 'client_managed' | 'oauth' | 'unknown'
  client_support: {
    resolution: 'directory' | 'install_time'
    clients: ClientID[]
    delivery: Partial<Record<ClientID, DeliveryState>>
    chatgpt_binding: boolean
  }
}

export interface RegistryIndex {
  schema_version: 1
  data_source: 'published_snapshot' | 'review_preview' | 'legacy_compatibility'
  snapshot_sequence?: number
  generated_at?: string
  plugins: RegistryPlugin[]
}

export interface ClientTarget {
  id: ClientID
  name: string
  icon: string
  note: string
}
