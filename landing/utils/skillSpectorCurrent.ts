import type { SkillListing } from '../types/skillsCatalog.ts';
import type { SkillSpectorDisplayReport, SkillSpectorRecord } from '../types/skillSpector.ts';
import { skillRepositoryUrl } from './skillsCatalog.ts';

export interface PublishedScanSubject {
  repository: string;
  sourceStateId: string;
  name: string;
}

/**
 * A provider listing is not scanner evidence. Join only a canonical GitHub
 * source and the exact provider listing ID; ambiguous/malformed IDs fail closed.
 */
export function publishedScanSubject(skill: SkillListing): PublishedScanSubject | undefined {
  const source = skillRepositoryUrl(skill);
  const repository = /^https:\/\/github\.com\/([a-z\d_.-]+\/[a-z\d_.-]+)(?:\/|$)/i.exec(
    source ?? '',
  )?.[1];
  const listing = /^skills-sh:([^:]+):([^:]+)$/.exec(skill.id);
  if (!repository || !listing || listing[1]?.toLowerCase() !== repository.toLowerCase())
    return undefined;
  const name = skill.skillNameHint;
  if (!name || listing[2] !== name) return undefined;
  return {
    repository: repository.toLowerCase(),
    sourceStateId: repository.toLowerCase() + '/' + listing[2],
    name,
  };
}

/**
 * A checked source, an old report, a queued job and a valid install command
 * are all insufficient. Only a matching executed v3 record is eligible.
 * Partial analysis is eligible as *scanned*, but never represented as complete.
 */
export function currentPublishedScan(
  report: SkillSpectorDisplayReport,
  subject: PublishedScanSubject,
): SkillSpectorRecord | undefined {
  if (report.schema_version !== 3) return undefined;
  // Contradictory resolution rows must not be bypassed by finding one positive row.
  const states = report.source_states.filter((item) => item.id === subject.sourceStateId);
  if (states.length !== 1) return undefined;
  const state = states[0];
  if (
    !state ||
    state.repository !== subject.repository ||
    state.current !== true ||
    state.state !== 'resolved' ||
    state.reason !== null ||
    !Array.isArray(state.selected_ids) ||
    state.selected_ids.length !== 1 ||
    typeof state.revision !== 'string' ||
    typeof state.path !== 'string'
  )
    return undefined;
  const selectedId: unknown = state.selected_ids[0];
  if (typeof selectedId !== 'string') return undefined;
  return report.records.find(
    (record) =>
      record.id === selectedId &&
      record.kind === 'standalone_skill' &&
      record.skill_name === subject.name &&
      record.source.repository.toLowerCase() === subject.repository &&
      record.source.revision === state.revision &&
      record.source.path === state.path &&
      record.target_revision === state.revision &&
      record.freshness === 'current' &&
      record.execution?.state === 'completed' &&
      (record.status === 'complete' || record.status === 'partial') &&
      record.coverage?.status === record.status &&
      // A partial result that inspected no Skill files is not a scan.
      (record.coverage?.fully_inspected_files ?? 0) +
        (record.coverage?.partially_inspected_files ?? 0) >
        0,
  );
}

export function hasCurrentPublishedScan(
  report: SkillSpectorDisplayReport,
  subject: PublishedScanSubject,
): boolean {
  return Boolean(currentPublishedScan(report, subject));
}

/** Zero findings is meaningful only for complete current evidence of this exact root. */
export function hasNoPublishedRemarks(
  report: SkillSpectorDisplayReport,
  subject: PublishedScanSubject,
): boolean {
  const record = currentPublishedScan(report, subject);
  if (!record || report.schema_version !== 3) return false;
  const coverage = record.coverage;
  return Boolean(
    record.status === 'complete' &&
    record.execution?.exit_code === 0 &&
    coverage?.status === 'complete' &&
    record.legacy_format === undefined &&
    record.scanner?.id === report.scanner.id &&
    record.scanner?.revision === report.scanner.revision &&
    record.scanner?.version === report.scanner.version &&
    report.profiles.some(
      (profile) => profile.id === record.profile?.id && profile.digest === record.profile?.digest,
    ) &&
    /^sha256:[0-9a-f]{64}$/.test(record.source.content_digest ?? '') &&
    coverage.fully_inspected_files > 0 &&
    coverage.partially_inspected_files === 0 &&
    coverage.entirely_uninspected_files === 0 &&
    coverage.reasons.length === 0 &&
    coverage.grouping_complete === true &&
    coverage.original_findings.length === 0 &&
    coverage.reference_evidence_truncated !== true &&
    (coverage.ledger_exceptions?.length ?? 0) === 0 &&
    (coverage.ledger_exceptions_total ?? 0) === 0 &&
    record.errors.length === 0 &&
    record.findings_truncated === false &&
    record.finding_count === 0 &&
    record.raw_finding_count === 0 &&
    record.finding_group_count === 0 &&
    record.findings.length === 0 &&
    record.retry?.disposition === 'none' &&
    record.retry.next_attempt_at === null &&
    record.retry.next_profile === null,
  );
}
