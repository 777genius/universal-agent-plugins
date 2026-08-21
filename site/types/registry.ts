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
export type DeliveryMode = 'managed' | 'prepared' | 'manual_activation'
export type DistributionStatus = 'candidate' | 'active' | 'suspended'
export type ReleaseStatus = 'active' | 'superseded' | 'revoked'

export interface AppBinding {
  app_key: string
  id: string
  mcp_server: string
}

export interface ReleaseTarget {
  client: ClientID
  delivery: DeliveryMode
  scopes: string[]
  app_binding?: AppBinding
}

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
  package_tree_digest?: string
  dependency_identity?: string
  installer_version?: string
  artifact_digest?: string
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
  status: DistributionStatus
  release_status: ReleaseStatus
  selectable: boolean
  targets: ReleaseTarget[]
  components: ComponentID[]
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
  installable: boolean
  components: ComponentID[]
  icon?: PluginIcon
  default_distribution: string
  declared_default_distribution: string
  default_fallback_reason?: string
  distributions: DistributionView[]
  evidence: ClientEvidence[]
  authentication: 'none' | 'client_managed' | 'oauth' | 'unknown'
  client_support: {
    resolution: 'directory' | 'install_time'
    clients: ClientID[]
    delivery: Partial<Record<ClientID, DeliveryMode>>
    scopes: Partial<Record<ClientID, string[]>>
    app_bindings: Partial<Record<ClientID, AppBinding>>
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
