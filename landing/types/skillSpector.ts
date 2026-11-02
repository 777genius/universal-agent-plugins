/** Display-only historical reports. These records never authorize installation. */
export interface SkillSpectorFinding extends SkillSpectorOccurrence {
  rule: string;
  severity: string;
  message: string;
  path: string;
  line?: number;
  id?: string;
  finding_ids?: string[];
  evidence?: Record<string, unknown>;
  confidence?: number | null;
  review_status?: 'unreviewed' | 'confirmed' | 'false_positive' | 'accepted';
  occurrences?: SkillSpectorOccurrence[];
  raw_count?: number;
  context?: unknown;
  code_snippet?: unknown;
  remediation?: unknown;
  match_fingerprint?: string | null;
  exception?: {
    id: string;
    reason: string;
    source_content_digest: string;
    scanner_revision: string;
    match_fingerprint: string;
    snippet_digest: string;
    profile_digest: string;
  };
}

export interface SkillSpectorOccurrence {
  path: string;
  line?: number;
  end_line?: number;
  start_column?: number;
  end_column?: number;
  finding_id?: string;
}
export interface SkillSpectorReason {
  code: string;
  message: string;
  analyzer_id?: string;
  path?: string;
  retryable?: boolean;
  start_line?: number | null;
  end_line?: number | null;
}
export interface SkillSpectorReference {
  source_path: string;
  line: number;
  column: number;
  evidence: string;
  target_path: string | null;
  status: 'resolved' | 'missing' | 'ambiguous' | 'rejected' | 'uninspected';
  disposition: 'analyzed' | 'partial' | 'failed' | 'out_of_scope';
  reference_kind:
    | 'markdown_image'
    | 'documentation_image'
    | 'markdown_link'
    | 'inline_command'
    | 'quoted_or_code'
    | 'plain_path';
  source_sha256?: string;
}
export interface SkillSpectorLedgerException {
  reason_code: string;
  message: string;
  path?: string;
  start_line?: number | null;
  end_line?: number | null;
  outcome?: 'completed' | 'partial' | 'failed' | 'skipped' | 'out_of_scope';
  phase?: string;
  error_class?: string;
  analyzers?: string[];
  fatal?: boolean;
  scope?: string;
}
export interface SkillSpectorInspection {
  status: 'complete' | 'partial' | 'not_started';
  fully_inspected_files: number;
  partially_inspected_files: number;
  entirely_uninspected_files: number;
  reasons: SkillSpectorReason[];
  analyzers: Record<string, unknown>[];
  original_findings: Record<string, unknown>[];
  grouping_complete?: boolean;
  references?: SkillSpectorReference[];
  ledger_exceptions?: SkillSpectorLedgerException[];
  references_total?: number;
  ledger_exceptions_total?: number;
  reference_evidence_truncated?: boolean;
}

export interface SkillSpectorRecord {
  id: string;
  kind: 'plugin_skill' | 'standalone_skill';
  plugin_path: string | null;
  skill_name: string | null;
  source: {
    repository: string;
    revision: string;
    path: string;
    content_digest: string | null;
  };
  scanned_at: string | null;
  status: 'complete' | 'partial' | 'error';
  report_digest?: string;
  finding_count: number;
  findings_truncated: boolean;
  findings: SkillSpectorFinding[];
  errors: { code: string; message: string }[];
  scanner?: SkillSpectorReport['scanner'];
  profile?: { id: string; digest: string };
  execution?: {
    state: 'queued' | 'running' | 'completed' | 'failed';
    exit_code: number | null;
    acquisition_seconds: number | null;
    analysis_seconds: number | null;
    started_at: string | null;
    finished_at: string | null;
  };
  coverage?: SkillSpectorInspection;
  raw_finding_count?: number;
  finding_group_count?: number;
  freshness?: 'current' | 'historical';
  target_revision?: string;
  retry?: {
    attempt_count: number;
    next_attempt_at: string | null;
    next_profile: string | null;
    reason: string;
    disposition: 'none' | 'scheduled' | 'blocked' | 'exhausted';
    latest_status?: 'complete' | 'partial' | 'error';
  };
  attempt_history?: Record<string, unknown>[];
  legacy_format?: 1 | 2;
}

export interface SkillSpectorReport {
  schema_version: 1;
  generated_at: string;
  source_commit: string;
  scanner: { id: 'skillspector'; version: string; revision: string };
  profile: { id: 'static-only-v1'; digest: string };
  scope: 'selected_sources';
  selection_digest: string;
  records: SkillSpectorRecord[];
}

export interface SkillSpectorManifest {
  schema_version: 2;
  generated_at: string;
  source_commit: string;
  scanner: SkillSpectorReport['scanner'];
  profile: { id: 'static-only-v2'; digest: string };
  scope: 'selected_sources';
  selection_digest: string;
  coverage: { selected: number; reported: number; pending: number };
  shards: SkillSpectorShardDescriptor[];
  plugin_coverage: SkillSpectorPluginCoverage[];
}
export interface SkillSpectorPluginCoverage {
  repository: string;
  plugin_path: string;
  selected: number;
  reported: number;
  pending: number;
  complete?: number;
  partial?: number;
  error?: number;
  stale?: number;
}
export interface SkillSpectorManifestV3 extends Omit<
  SkillSpectorManifest,
  'schema_version' | 'profile' | 'coverage'
> {
  schema_version: 3;
  profile: { id: 'static-policy-v3'; digest: string };
  profiles: { id: string; digest: string }[];
  coverage: {
    selected: number;
    reported: number;
    pending: number;
    complete: number;
    partial: number;
    error: number;
    stale: number;
  };
  source_states: Record<string, unknown>[];
  plugin_states: Record<string, unknown>[];
  repository_aliases: Record<string, unknown>[];
  catalog_coverage?: {
    observed: number;
    resolved: number;
    historical: number;
    unresolved: number;
    unavailable: number;
    unsupported: number;
  };
}
export interface SkillSpectorShardDescriptor {
  repository: string;
  path: string;
  digest: string;
  bytes: number;
  record_count: number;
}
export type SkillSpectorDisplayReport =
  | SkillSpectorReport
  | (Omit<SkillSpectorManifest, 'coverage' | 'shards'> & { records: SkillSpectorRecord[] })
  | (SkillSpectorManifestV3 & { records: SkillSpectorRecord[] });

export type SkillSpectorSubject =
  | { kind: 'plugin'; repository: string; path: string; revision: string | null }
  | { kind: 'skill'; repository: string; name: string };
export type SkillSpectorStatus =
  | 'notChecked'
  | 'noFindings'
  | 'findingsCount'
  | 'partial'
  | 'error'
  | 'historical'
  | 'pending'
  | 'incomplete'
  | 'unavailable'
  | 'noStaticFindings'
  | 'unreviewedFindings'
  | 'confirmedMaterialIssue'
  | 'stale'
  | 'unsupported'
  | 'reviewedFindings';
export interface SkillSpectorAssessment {
  status: SkillSpectorStatus;
  findingCount: number;
  outOfDate: boolean;
  coverage?: SkillSpectorPluginCoverage;
  records: SkillSpectorRecord[];
  fullyChecked?: number;
  inventoryKnown?: boolean;
  selected?: number;
  pendingIds?: string[];
  legacy?: boolean;
  reasons?: SkillSpectorReason[];
  groupCount?: number;
  rawCount?: number;
  unreviewedCount?: number;
  outcomes?: {
    complete: number;
    limited: number;
    failed: number;
    historical: number;
    unscanned: number;
  };
  retries?: { scheduled: number; blocked: number; exhausted: number };
}
