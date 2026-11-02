import { skillSpectorSourceStateMatches } from './skillSpector.ts';
import type {
  SkillSpectorAssessment,
  SkillSpectorDisplayReport,
  SkillSpectorSubject,
} from '../types/skillSpector.ts';

export interface SkillSpectorDiagnosticInput {
  name: string;
  scope: 'plugin' | 'skill';
  subject?: SkillSpectorSubject;
  report?: SkillSpectorDisplayReport;
  reportStatus: 'loading' | 'ready' | 'unavailable';
  displayStatus: string;
  assessment: SkillSpectorAssessment;
}

/**
 * Explicitly user-triggered, read-only diagnostic export.
 * Include every published record and original context for this listing only.
 * A catalog name hint is NOT a source revision or proof of a successful scan.
 */
export function serializeSkillSpectorDiagnostics(input: SkillSpectorDiagnosticInput): string {
  const { report, subject, assessment } = input;
  // Keep original records once; the summary and the raw records are distinct data.
  const { records, ...summary } = assessment;
  const repository = subject?.repository.toLowerCase();
  const aliases = report?.schema_version === 3 && repository
    ? report.repository_aliases.filter((alias) =>
        String(alias.repository).toLowerCase() === repository)
    : [];
  const canonical = String(
    aliases[0]?.canonical_repository ?? subject?.repository ?? '',
  ).toLowerCase();
  const matchesRepository = (item: Record<string, unknown>) => {
    const candidate = String(item.repository ?? '').toLowerCase();
    return Boolean(canonical && (candidate === canonical || candidate === repository));
  };
  const relevantStates = report?.schema_version === 3 && subject
    ? subject.kind === 'plugin'
      ? report.plugin_states.filter((item) =>
          matchesRepository(item) &&
          item.plugin_path === subject.path &&
          (item.revision === null || item.revision === subject.revision))
      : report.source_states.filter((item) =>
          skillSpectorSourceStateMatches(item, subject.repository, canonical, subject.name))
    : [];
  const publication = report
    ? {
        schema_version: report.schema_version,
        generated_at: report.generated_at,
        source_commit: report.source_commit,
        scanner: report.scanner,
        profile: report.profile,
        scope: report.scope,
        selection_digest: report.selection_digest,
        ...(report.schema_version === 3
          ? {
              profiles: report.profiles,
              global_coverage: report.coverage,
              catalog_coverage: report.catalog_coverage ?? null,
            }
          : report.schema_version === 2
            ? { global_coverage: report.coverage }
            : {}),
      }
    : null;
  return JSON.stringify(
    {
      format: 'uap.skillspector.diagnostics.v1',
      guidance:
        'Published static analysis evidence only; not proof of safety or a runtime security audit. ' +
        'An unknown, partial, failed, or old result is not a completed current scan. ' +
        'This export contains all available published evidence for this listing; ' +
        'any evidence omitted by the publisher cannot be recovered from the export.',
      listing: {
        name: input.name,
        scope: input.scope,
        source: subject ?? null,
        report_load_status: input.reportStatus,
        displayed_status: input.displayStatus,
      },
      publication,
      // No successfully loaded report means no derived scan assessment is authoritative.
      assessment: input.reportStatus === 'ready' && report ? summary : null,
      source_states: relevantStates,
      repository_aliases: aliases,
      records,
    },
    null,
    2,
  );
}
