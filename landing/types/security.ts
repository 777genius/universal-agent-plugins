export type SecurityOutcome =
  | 'no_blocking_findings'
  | 'warnings'
  | 'blocking_findings'
  | 'check_unavailable';

export interface SecuritySubject {
  tree_digest: string;
  manifest_digest: string;
}

export interface SecurityFinding {
  code: string;
  disposition: 'blocking' | 'warning';
  severity: 'allow' | 'warn' | 'deny';
  confidence: 'low' | 'medium' | 'high';
  category: string;
  path: string;
  line?: number;
  message: string;
}

export interface SecurityRecord {
  subject: SecuritySubject;
  outcome: SecurityOutcome;
  counts: { blocking: number; warnings: number; total: number };
  scanned_files: number;
  report_digest?: string;
  error_code?: 'acquisition_failed' | 'scan_failed';
  findings: SecurityFinding[];
}

export interface SecurityPointer {
  pointer_schema_version: 1;
  snapshot_schema_version: 1;
  sequence: number;
  snapshot_path: string;
  envelope_path: string;
  fetch_contract: {
    max_redirects: number;
    latest_max_bytes: number;
    snapshot_max_bytes: number;
    envelope_max_bytes: number;
    retry_attempts: number;
  };
}

export interface SecurityEnvelope {
  envelope_schema_version: 1;
  snapshot_schema_version: 1;
  sequence: number;
  key_id: string;
  algorithm: 'Ed25519';
  signature_domain: 'UAP-SECURITY-INDEX-ED25519-V1';
  snapshot_digest: string;
  signature: string;
}

export interface SecuritySnapshot {
  security_schema_version: 1;
  sequence: number;
  publication_id: string;
  source_commit: string;
  generated_at: string;
  expires_at: string;
  complete: true;
  discovery: { sequence: number; snapshot_digest: string };
  scanner: { id: string; version: string };
  policy: { id: string; version: number; digest: string };
  coverage: { subjects: number; checked: number; unavailable: number };
  records: SecurityRecord[];
}

export interface SecurityBundle {
  pointer: SecurityPointer;
  envelope: SecurityEnvelope;
  snapshot: SecuritySnapshot;
}

export interface SecurityTrust {
  keyID: string;
  publicKeyBase64: string;
}
