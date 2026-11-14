import type {
  SecurityBundle,
  SecurityEnvelope,
  SecurityPointer,
  SecurityRecord,
  SecuritySnapshot,
  SecuritySubject,
  SecurityTrust,
} from '../types/security';
import type { RegistryPlugin } from '../types/registry';
import {
  assertExactKeys,
  decodeBase64,
  fetchSignedArtifact,
  parseCanonicalJSON,
  parseUTCTimestamp,
  sha256Digest,
  signedMessage,
} from './signedFeed.ts';

const namespace = 'Security Index';
const signatureDomain = 'UAP-SECURITY-INDEX-ED25519-V1';
const digestPattern = /^sha256:[0-9a-f]{64}$/;
const revisionPattern = /^[0-9a-f]{40}$/;
const identifierPattern = /^[a-z0-9][a-z0-9._-]{0,63}$/;
const publicationPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$/;
const versionPattern = /^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/;
const findingPattern = /^[A-Z]+\d+$/;

export interface SecurityLoadOptions {
  origin: URL;
  trust: SecurityTrust;
  fetcher?: typeof fetch;
  now?: Date;
}

export async function loadSecurity(options: SecurityLoadOptions): Promise<SecurityBundle> {
  const fetcher = options.fetcher ?? fetch;
  const pointerResponse = await fetchWithRetry(
    new URL('latest.json', options.origin),
    16 << 10,
    fetcher,
    3,
  );
  const pointer = parsePointer(pointerResponse.bytes);
  const [snapshotResponse, envelopeResponse] = await Promise.all([
    fetchWithRetry(
      new URL(pointer.snapshot_path, options.origin),
      pointer.fetch_contract.snapshot_max_bytes,
      fetcher,
      pointer.fetch_contract.retry_attempts,
    ),
    fetchWithRetry(
      new URL(pointer.envelope_path, options.origin),
      pointer.fetch_contract.envelope_max_bytes,
      fetcher,
      pointer.fetch_contract.retry_attempts,
    ),
  ]);
  return verifySecurity(
    pointerResponse.bytes,
    snapshotResponse.bytes,
    envelopeResponse.bytes,
    options.trust,
    options.now ?? new Date(),
  );
}

export async function verifySecurity(
  pointerBytes: Uint8Array,
  snapshotBytes: Uint8Array,
  envelopeBytes: Uint8Array,
  trust: SecurityTrust,
  now = new Date(),
): Promise<SecurityBundle> {
  const pointer = parsePointer(pointerBytes);
  const envelope = parseCanonicalJSON<SecurityEnvelope>(envelopeBytes, namespace, 'envelope');
  const snapshot = parseCanonicalJSON<SecuritySnapshot>(snapshotBytes, namespace, 'snapshot');
  assertEnvelope(envelope, trust);
  assertSnapshot(snapshot);
  if (pointer.sequence !== snapshot.sequence || envelope.sequence !== snapshot.sequence) {
    throw new Error(`${namespace} sequence mismatch`);
  }
  if ((await sha256Digest(snapshotBytes)) !== envelope.snapshot_digest) {
    throw new Error(`${namespace} digest mismatch`);
  }
  const publicKey = decodeBase64(trust.publicKeyBase64);
  const signature = decodeBase64(envelope.signature);
  if (publicKey.length !== 32 || signature.length !== 64) {
    throw new Error(`${namespace} key or signature has an invalid length`);
  }
  const key = await crypto.subtle.importKey('raw', publicKey, { name: 'Ed25519' }, false, [
    'verify',
  ]);
  if (
    !(await crypto.subtle.verify(
      'Ed25519',
      key,
      signature,
      signedMessage(signatureDomain, snapshotBytes),
    ))
  ) {
    throw new Error(`${namespace} signature is invalid`);
  }
  const generated = parseUTCTimestamp(snapshot.generated_at, namespace);
  const expires = parseUTCTimestamp(snapshot.expires_at, namespace);
  if (now.getTime() < generated.getTime()) {
    throw new Error(`${namespace} local clock is before generation time`);
  }
  if (now.getTime() >= expires.getTime()) throw new Error(`${namespace} is stale`);
  return { pointer, envelope, snapshot };
}

export function applySecurityAssessment(
  plugin: RegistryPlugin,
  snapshot: SecuritySnapshot,
): RegistryPlugin {
  const subject = plugin.discovery && {
    tree_digest: plugin.discovery.tree_digest,
    manifest_digest: plugin.discovery.manifest_digest,
  };
  if (!subject) return plugin;
  const record = lookupSecurity(snapshot.records, subject);
  if (!record || record.outcome === 'check_unavailable') return plugin;
  return {
    ...plugin,
    security: {
      generated_at: snapshot.generated_at,
      scanner: snapshot.scanner,
      policy: snapshot.policy,
      outcome: record.outcome,
      counts: record.counts,
      scanned_files: record.scanned_files,
      findings: record.findings,
    },
  };
}

export function lookupSecurity(
  records: SecurityRecord[],
  subject: SecuritySubject,
): SecurityRecord | undefined {
  let low = 0;
  let high = records.length;
  const wanted = subjectKey(subject);
  while (low < high) {
    const middle = Math.floor((low + high) / 2);
    if (subjectKey(records[middle]!.subject) < wanted) low = middle + 1;
    else high = middle;
  }
  return records[low]?.subject.tree_digest === subject.tree_digest &&
    records[low]?.subject.manifest_digest === subject.manifest_digest
    ? records[low]
    : undefined;
}

function parsePointer(bytes: Uint8Array): SecurityPointer {
  const value = parseCanonicalJSON<SecurityPointer>(bytes, namespace, 'pointer');
  assertExactKeys(
    value,
    [
      'pointer_schema_version',
      'snapshot_schema_version',
      'sequence',
      'snapshot_path',
      'envelope_path',
      'fetch_contract',
    ],
    namespace,
    'pointer',
  );
  assertExactKeys(
    value.fetch_contract,
    [
      'max_redirects',
      'latest_max_bytes',
      'snapshot_max_bytes',
      'envelope_max_bytes',
      'retry_attempts',
    ],
    namespace,
    'fetch contract',
  );
  const stem = String(value.sequence).padStart(20, '0');
  const contract = value.fetch_contract;
  if (
    !publicSequence(value.sequence) ||
    value.pointer_schema_version !== 1 ||
    value.snapshot_schema_version !== 1 ||
    value.snapshot_path !== `snapshots/${stem}.json` ||
    value.envelope_path !== `snapshots/${stem}.envelope.json` ||
    contract.max_redirects !== 0 ||
    contract.retry_attempts < 1 ||
    contract.retry_attempts > 3 ||
    contract.latest_max_bytes !== 16 << 10 ||
    contract.snapshot_max_bytes !== 8 << 20 ||
    contract.envelope_max_bytes !== 16 << 10
  ) {
    throw new Error(`${namespace} pointer is invalid`);
  }
  return value;
}

function assertEnvelope(value: SecurityEnvelope, trust: SecurityTrust) {
  assertExactKeys(
    value,
    [
      'envelope_schema_version',
      'snapshot_schema_version',
      'sequence',
      'key_id',
      'algorithm',
      'signature_domain',
      'snapshot_digest',
      'signature',
    ],
    namespace,
    'envelope',
  );
  if (
    value.envelope_schema_version !== 1 ||
    value.snapshot_schema_version !== 1 ||
    !publicSequence(value.sequence) ||
    value.key_id !== trust.keyID ||
    value.algorithm !== 'Ed25519' ||
    value.signature_domain !== signatureDomain ||
    !digestPattern.test(value.snapshot_digest) ||
    !/^[A-Za-z0-9+/]{86}==$/.test(value.signature)
  ) {
    throw new Error(`${namespace} envelope is invalid`);
  }
}

function assertSnapshot(value: SecuritySnapshot) {
  assertExactKeys(
    value,
    [
      'security_schema_version',
      'sequence',
      'publication_id',
      'source_commit',
      'generated_at',
      'expires_at',
      'complete',
      'discovery',
      'scanner',
      'policy',
      'coverage',
      'records',
    ],
    namespace,
    'snapshot',
  );
  assertExactKeys(value.discovery, ['sequence', 'snapshot_digest'], namespace, 'discovery identity');
  assertExactKeys(value.scanner, ['id', 'version'], namespace, 'scanner');
  assertExactKeys(value.policy, ['id', 'version', 'digest'], namespace, 'policy');
  assertExactKeys(value.coverage, ['subjects', 'checked', 'unavailable'], namespace, 'coverage');
  if (
    value.security_schema_version !== 1 ||
    !publicSequence(value.sequence) ||
    !value.complete ||
    !publicationPattern.test(value.publication_id) ||
    !revisionPattern.test(value.source_commit) ||
    !publicSequence(value.discovery.sequence) ||
    !digestPattern.test(value.discovery.snapshot_digest) ||
    !identifierPattern.test(value.scanner.id) ||
    !versionPattern.test(value.scanner.version) ||
    !identifierPattern.test(value.policy.id) ||
    !publicSequence(value.policy.version) ||
    !digestPattern.test(value.policy.digest) ||
    !Array.isArray(value.records) ||
    value.records.length > 10_000
  ) {
    throw new Error(`${namespace} snapshot identity is invalid`);
  }
  const generated = parseUTCTimestamp(value.generated_at, namespace);
  const expires = parseUTCTimestamp(value.expires_at, namespace);
  if (expires <= generated || expires.getTime() - generated.getTime() > 31 * 86_400_000) {
    throw new Error(`${namespace} lifetime is invalid`);
  }
  let checked = 0;
  let prior = '';
  value.records.forEach((record, index) => {
    assertRecord(record);
    const key = subjectKey(record.subject);
    if (index && key <= prior) {
      throw new Error(`${namespace} records are duplicated or not ordered`);
    }
    prior = key;
    if (record.outcome !== 'check_unavailable') checked += 1;
  });
  if (
    !nonNegativeInteger(value.coverage.subjects) ||
    !nonNegativeInteger(value.coverage.checked) ||
    !nonNegativeInteger(value.coverage.unavailable) ||
    value.coverage.subjects !== value.records.length ||
    value.coverage.checked !== checked ||
    value.coverage.unavailable !== value.records.length - checked
  ) {
    throw new Error(`${namespace} coverage is inconsistent`);
  }
}

function assertRecord(record: SecurityRecord) {
  const unavailable = record.outcome === 'check_unavailable';
  assertExactKeys(
    record,
    unavailable
      ? ['subject', 'outcome', 'counts', 'scanned_files', 'error_code', 'findings']
      : ['subject', 'outcome', 'counts', 'scanned_files', 'report_digest', 'findings'],
    namespace,
    'record',
  );
  assertExactKeys(record.subject, ['tree_digest', 'manifest_digest'], namespace, 'subject');
  assertExactKeys(record.counts, ['blocking', 'warnings', 'total'], namespace, 'counts');
  if (
    !digestPattern.test(record.subject.tree_digest) ||
    !digestPattern.test(record.subject.manifest_digest) ||
    !nonNegativeInteger(record.counts.blocking) ||
    !nonNegativeInteger(record.counts.warnings) ||
    !nonNegativeInteger(record.counts.total) ||
    record.counts.total !== record.counts.blocking + record.counts.warnings ||
    !nonNegativeInteger(record.scanned_files) ||
    !Array.isArray(record.findings) ||
    record.findings.length > 32
  ) {
    throw new Error(`${namespace} record is invalid`);
  }
  if (unavailable) {
    if (
      (record.error_code !== 'acquisition_failed' && record.error_code !== 'scan_failed') ||
      record.counts.total ||
      record.scanned_files ||
      record.report_digest ||
      record.findings.length
    ) {
      throw new Error(`${namespace} unavailable record is invalid`);
    }
    return;
  }
  if (
    !['no_blocking_findings', 'warnings', 'blocking_findings'].includes(record.outcome) ||
    !digestPattern.test(record.report_digest ?? '') ||
    record.error_code
  ) {
    throw new Error(`${namespace} checked record is invalid`);
  }
  const expected = record.counts.blocking
    ? 'blocking_findings'
    : record.counts.warnings
      ? 'warnings'
      : 'no_blocking_findings';
  if (record.outcome !== expected) throw new Error(`${namespace} outcome does not match counts`);
  let projectedBlocking = 0;
  let projectedWarnings = 0;
  let prior = '';
  record.findings.forEach((finding, index) => {
    const fields = ['code', 'disposition', 'severity', 'confidence', 'category', 'path', 'message'];
    if (finding.line !== undefined) fields.push('line');
    assertExactKeys(finding, fields, namespace, 'finding');
    if (
      !findingPattern.test(finding.code) ||
      !['blocking', 'warning'].includes(finding.disposition) ||
      !['allow', 'warn', 'deny'].includes(finding.severity) ||
      !['low', 'medium', 'high'].includes(finding.confidence) ||
      !finding.category.trim() ||
      [...finding.category].length > 64 ||
      [...finding.path].length > 1024 ||
      (finding.line !== undefined &&
        (!Number.isSafeInteger(finding.line) || finding.line < 1)) ||
      !finding.message.trim() ||
      [...finding.message].length > 2000
    ) {
      throw new Error(`${namespace} finding is invalid`);
    }
    const key = findingKey(finding);
    if (index && key < prior) throw new Error(`${namespace} findings are not ordered`);
    prior = key;
    if (finding.disposition === 'blocking') projectedBlocking += 1;
    else projectedWarnings += 1;
  });
  if (
    projectedBlocking > record.counts.blocking ||
    projectedWarnings > record.counts.warnings
  ) {
    throw new Error(`${namespace} finding projection exceeds counts`);
  }
}

async function fetchWithRetry(
  url: URL,
  maximum: number,
  fetcher: typeof fetch,
  attempts: number,
) {
  let last: unknown;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      return await fetchSignedArtifact(url, maximum, fetcher, namespace);
    } catch (error) {
      last = error;
      if (attempt + 1 < attempts) {
        await new Promise((resolve) => setTimeout(resolve, (attempt + 1) * 100));
      }
    }
  }
  throw last;
}

function publicSequence(value: number) {
  return Number.isSafeInteger(value) && value >= 1;
}

function nonNegativeInteger(value: number) {
  return Number.isSafeInteger(value) && value >= 0;
}

function subjectKey(subject: SecuritySubject) {
  return `${subject.tree_digest}\0${subject.manifest_digest}`;
}

function findingKey(finding: SecurityRecord['findings'][number]) {
  return `${finding.disposition}\0${finding.code}\0${finding.path}\0${String(
    finding.line ?? 0,
  ).padStart(10, '0')}\0${finding.message}`;
}
