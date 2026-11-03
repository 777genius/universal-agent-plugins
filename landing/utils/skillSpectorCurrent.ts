import type { SkillListing } from '../types/skillsCatalog.ts';
import type { SkillSpectorDisplayReport } from '../types/skillSpector.ts';
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
  const repository = /^https:\/\/github\.com\/([a-z\d_.-]+\/[a-z\d_.-]+)(?:\/|$)/i.exec(source ?? '')?.[1];
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
export function hasCurrentPublishedScan(
  report: SkillSpectorDisplayReport,
  subject: PublishedScanSubject,
): boolean {
  if (report.schema_version !== 3) return false;
  const state = report.source_states.find((item) =>
    item.id === subject.sourceStateId &&
    item.repository === subject.repository &&
    item.current === true &&
    item.state === 'resolved',
  );
  if (!state || !Array.isArray(state.selected_ids) || state.selected_ids.length !== 1 ||
      typeof state.revision !== 'string' || typeof state.path !== 'string') return false;
  const selectedId: unknown = state.selected_ids[0];
  if (typeof selectedId !== 'string') return false;
  return report.records.some((record) =>
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
    ((record.coverage?.fully_inspected_files ?? 0) +
      (record.coverage?.partially_inspected_files ?? 0)) > 0,
  );
}
