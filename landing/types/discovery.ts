import type { ClientID } from './registry';

export interface DiscoveryPointer {
  pointer_schema_version: 1;
  snapshot_schema_version: 1;
  sequence: number;
  snapshot_path: string;
  envelope_path: string;
  search_path: string;
  fetch_contract: {
    max_redirects: number;
    latest_max_bytes: number;
    snapshot_max_bytes: number;
    envelope_max_bytes: number;
    search_max_bytes: number;
    retry_attempts: number;
  };
}

export interface DiscoveryEnvelope {
  envelope_schema_version: 1;
  snapshot_schema_version: 1;
  sequence: number;
  key_id: string;
  algorithm: 'Ed25519';
  signature_domain: 'UAP-DISCOVERY-INDEX-ED25519-V1';
  snapshot_digest: string;
  signature: string;
}

export interface DiscoveryRecord {
  slug: string;
  name: string;
  description: string;
  owner: string;
  repository: string;
  package_path: string;
  revision: string;
  version: string | null;
  license: string | null;
  schema_version: '1.0.0';
  components: { extensions: number; mcp: number; skills: number };
  mcp_transports: string[];
  compatible_clients: ClientID[];
  authentication: 'not_required' | 'required' | 'unknown';
  status: 'conformant_unreviewed';
  runtime_reviewed: false;
  tree_digest: string;
  manifest_digest: string;
  stars: number;
  repository_updated_at: string;
  reviewed_distribution_id: string | null;
  availability: 'available' | 'unavailable';
  author?: { name: string; email?: string; url?: string } | null;
  first_seen?: string;
  last_seen?: string;
}

export interface DiscoverySnapshot {
  discovery_schema_version: 1;
  sequence: number;
  publication_id: string;
  source_commit: string;
  generated_at: string;
  expires_at: string;
  complete: true;
  query_manifest_digest: string;
  partitions: Array<{ query: string; size_min: number; size_max: number; total_count: number }>;
  search_projection: { path: string; digest: string; record_count: number };
  records: DiscoveryRecord[];
}

export interface DiscoverySearch {
  search_schema_version: 1;
  sequence: number;
  generated_at: string;
  records: DiscoveryRecord[];
}

export interface DiscoveryBundle {
  pointer: DiscoveryPointer;
  envelope: DiscoveryEnvelope;
  snapshot: DiscoverySnapshot;
  search: DiscoverySearch;
  bytes: Record<'pointer' | 'envelope' | 'snapshot' | 'search', Uint8Array>;
  etags: Partial<Record<'pointer' | 'envelope' | 'snapshot' | 'search', string>>;
  source: 'remote' | 'cache';
}
