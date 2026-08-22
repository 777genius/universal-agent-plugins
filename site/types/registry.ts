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
export type ClientEvidenceLevel = Exclude<EvidenceLevel, 'schema'>
export type EvidenceOutcome = 'passed' | 'failed' | 'inconclusive' | 'not_tested' | 'not_applicable'
export type DeliveryMode = 'managed' | 'prepared' | 'manual_activation'
export type DistributionStatus = 'candidate' | 'active' | 'suspended'
export type ReleaseStatus = 'active' | 'superseded' | 'revoked'
export type TargetAuthentication = 'not_required' | 'required' | 'unknown'

export interface AppBinding {
  app_key: string
  id: string
  mcp_server: string
}

export interface ReleaseTarget {
  client: ClientID
  authentication: TargetAuthentication
  delivery: DeliveryMode
  scopes: string[]
  app_binding?: AppBinding
}

export interface PluginIcon {
  path: string
  sha256: string
}

export interface EvidenceArtifact {
  repository: string
  revision: string
  path: string
  digest: string
  url: string
}

interface EvidenceDetails {
  id: string
  outcome: EvidenceOutcome
  tested_at?: string
}

export interface PackageEvidence extends EvidenceDetails {
  level: 'schema'
  package_tree_digest: string
  artifact: EvidenceArtifact
}

export interface ClientEvidence extends EvidenceDetails {
  client: ClientID
  level: ClientEvidenceLevel
  client_version?: string
  os?: string
  architecture?: string
  dependency_identity?: string
  installer_version?: string
  package_tree_digest?: string
  artifact?: EvidenceArtifact
}

export interface DistributionReleaseView {
  release_sequence: number
  source: PluginSource
  version: string
  targets: ReleaseTarget[]
  components: ComponentID[]
  evidence: ClientEvidence[]
  package_evidence: PackageEvidence[]
  release_status: ReleaseStatus
  selectable: boolean
  blocking_clients: ClientID[]
  materialized_clients: ClientID[]
  meets_minimum_capabilities: boolean
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
  package_evidence: PackageEvidence[]
  status: DistributionStatus
  release_status: ReleaseStatus
  selectable: boolean
  targets: ReleaseTarget[]
  components: ComponentID[]
  releases: DistributionReleaseView[]
  fallback_reason?: string
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
  package_evidence: PackageEvidence[]
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
  expires_at?: string
  plugins: RegistryPlugin[]
}

export interface DistributionResolution {
  distribution?: DistributionView
  fallback_reason?: string
  unavailable_reason?: string
  ineligible_reasons?: Array<{ distribution_id: string, reason: string }>
}

export interface ClientTarget {
  id: ClientID
  name: string
  icon: string
  note: string
}
