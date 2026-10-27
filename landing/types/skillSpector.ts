/** Display-only historical reports. These records never authorize installation. */
export interface SkillSpectorFinding {
  rule: string;
  severity: string;
  message: string;
  path: string;
  line?: number;
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
  | (Omit<SkillSpectorManifest, 'coverage' | 'shards'> & { records: SkillSpectorRecord[] });

export type SkillSpectorSubject =
  | { kind: 'plugin'; repository: string; path: string; revision: string | null }
  | { kind: 'skill'; repository: string; name: string };
export type SkillSpectorStatus =
  'notChecked' | 'noFindings' | 'findingsCount' | 'partial' | 'error' | 'historical';
export interface SkillSpectorAssessment {
  status: SkillSpectorStatus;
  findingCount: number;
  outOfDate: boolean;
  coverage?: SkillSpectorPluginCoverage;
  records: SkillSpectorRecord[];
}
