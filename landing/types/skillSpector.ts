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

export type SkillSpectorSubject =
  | { kind: 'plugin'; repository: string; path: string; revision: string | null }
  | { kind: 'skill'; repository: string; name: string };
export type SkillSpectorStatus =
  'notChecked' | 'noFindings' | 'findingsCount' | 'partial' | 'error' | 'historical';
export interface SkillSpectorAssessment {
  status: SkillSpectorStatus;
  findingCount: number;
  outOfDate: boolean;
  records: SkillSpectorRecord[];
}
