import type {
  SkillSpectorAssessment,
  SkillSpectorFinding,
  SkillSpectorDisplayReport,
  SkillSpectorRecord,
  SkillSpectorReport,
  SkillSpectorSubject,
} from '../types/skillSpector.ts';

/** Published totals do not establish the review state of omitted finding groups. */
export function hasIncompleteSkillSpectorFindings(record: SkillSpectorRecord): boolean {
  return record.findings_truncated || record.coverage?.grouping_complete === false;
}

/** Presentation-only exact message grouping; every original context remains intact. */
export function groupSkillSpectorReasons<T extends { code: string; message: string }>(
  reasons: readonly T[],
) {
  const groups = new Map<string, { key: string; code: string; message: string; contexts: T[] }>();
  for (const reason of reasons) {
    const key = JSON.stringify([reason.code, reason.message]);
    let group = groups.get(key);
    if (!group) {
      group = { key, code: reason.code, message: reason.message, contexts: [] };
      groups.set(key, group);
    }
    group.contexts.push(reason);
  }
  return [...groups.values()];
}

const revisionPattern = /^[0-9a-f]{40}$/;
const digestPattern = /^sha256:[0-9a-f]{64}$/;
const repositoryPattern = /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/;
const namespace = 'SkillSpector report';
// Keep the original 256-file profile valid while the bounded 512-file profile rolls out.
const profileDigests = new Set([
  'sha256:ddd91212d95a717ab6c067d99e9f32a64b8dd55589bdf47a36695a85320187d5',
  'sha256:cf80ad7d737bc7eab6d282a48e63a0d94941e8fd9bde53dbace1222378780c44',
]);

/** Read only the bounded static display document; no credentials or scanner calls. */
export async function loadSkillSpectorReport(
  url: string,
  fetcher: typeof fetch = fetch,
): Promise<SkillSpectorReport> {
  const maximum = 8 << 20;
  const response = await fetcher(url, {
    headers: { Accept: 'application/json' },
    credentials: 'omit',
    signal: AbortSignal.timeout(15_000),
  });
  if (!response.ok || Number(response.headers.get('content-length')) > maximum || !response.body)
    invalid();
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > maximum) {
        await reader.cancel();
        invalid();
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return decodeSkillSpectorReport(
    JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes)),
  );
}

/** Reject malformed or misleading reports instead of treating missing data as clean. */
export function decodeSkillSpectorReport(value: unknown, now = new Date()): SkillSpectorReport {
  const report = object(value);
  const scanner = object(report.scanner);
  const profile = object(report.profile);
  if (
    !keys(report, [
      'schema_version',
      'generated_at',
      'source_commit',
      'scanner',
      'profile',
      'scope',
      'selection_digest',
      'records',
    ]) ||
    !keys(scanner, ['id', 'version', 'revision']) ||
    !keys(profile, ['id', 'digest']) ||
    report.schema_version !== 1 ||
    !timestamp(report.generated_at, now) ||
    !revision(report.source_commit) ||
    scanner.id !== 'skillspector' ||
    scanner.version !== '2.13.1' ||
    scanner.revision !== 'f3b25697d76256998ca9d42c198d256258d4c035' ||
    profile.id !== 'static-only-v1' ||
    typeof profile.digest !== 'string' ||
    !profileDigests.has(profile.digest) ||
    report.scope !== 'selected_sources' ||
    !digest(report.selection_digest) ||
    !Array.isArray(report.records) ||
    report.records.length > 128
  )
    invalid();
  decodeSkillSpectorRecords(report.records, String(report.generated_at), 128, now);
  return value as SkillSpectorReport;
}

/** Shared strict record validation for legacy reports and bounded repository shards. */
export function decodeSkillSpectorRecords(
  value: unknown,
  generatedAt: string,
  maximum: number,
  now = new Date(),
): SkillSpectorRecord[] {
  if (!Array.isArray(value) || value.length > maximum) invalid();
  const ids = new Set<string>();
  for (const item of value as unknown[]) {
    const record = object(item);
    const source = object(record.source);
    if (
      !keys(
        record,
        [
          'id',
          'kind',
          'plugin_path',
          'skill_name',
          'source',
          'scanned_at',
          'status',
          'finding_count',
          'findings_truncated',
          'findings',
          'errors',
        ],
        ['report_digest'],
      ) ||
      !keys(source, ['repository', 'revision', 'path', 'content_digest']) ||
      !text(record.id, 200) ||
      ids.has(record.id as string) ||
      !['plugin_skill', 'standalone_skill'].includes(String(record.kind)) ||
      !(record.plugin_path === null || path(record.plugin_path, true)) ||
      !(record.skill_name === null || text(record.skill_name, 200)) ||
      (record.kind === 'plugin_skill' && !pluginSkillRoot(record.plugin_path, source.path)) ||
      (record.kind === 'standalone_skill' && record.plugin_path !== null) ||
      !repository(source.repository) ||
      !revision(source.revision) ||
      !path(source.path, true) ||
      !(source.content_digest === null || digest(source.content_digest)) ||
      !(record.scanned_at === null || timestamp(record.scanned_at, now)) ||
      !['complete', 'partial', 'error'].includes(String(record.status)) ||
      !count(record.finding_count) ||
      typeof record.findings_truncated !== 'boolean' ||
      !Array.isArray(record.findings) ||
      record.findings.length > 32 ||
      record.findings.length > Number(record.finding_count) ||
      record.findings_truncated !== record.findings.length < Number(record.finding_count) ||
      !Array.isArray(record.errors) ||
      record.errors.length > 16
    )
      invalid();
    ids.add(record.id as string);
    if (record.report_digest !== undefined && !digest(record.report_digest)) invalid();
    if (
      record.status !== 'error' &&
      (source.content_digest === null ||
        record.scanned_at === null ||
        !digest(record.report_digest))
    )
      invalid();
    if (record.status === 'complete' && (record.errors as unknown[]).length) invalid();
    if (
      record.scanned_at !== null &&
      Date.parse(record.scanned_at as string) > Date.parse(generatedAt)
    )
      invalid();
    for (const item of record.findings as unknown[]) {
      const finding = object(item);
      if (
        !keys(finding, ['rule', 'severity', 'message', 'path'], ['line']) ||
        !text(finding.rule, 120) ||
        !['INFO', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL'].includes(String(finding.severity)) ||
        !text(finding.message, 2000) ||
        !text(finding.path, 1000) ||
        (finding.line !== undefined && (!count(finding.line) || finding.line === 0))
      )
        invalid();
    }
    for (const item of record.errors as unknown[]) {
      const error = object(item);
      if (!keys(error, ['code', 'message']) || !text(error.code, 120) || !text(error.message, 2000))
        invalid();
    }
  }
  return value as SkillSpectorRecord[];
}

/** Plugin joins are immutable; the Skills provider supplies historical name hints only. */
export function skillSpectorAssessment(
  report: SkillSpectorDisplayReport | undefined,
  subject: SkillSpectorSubject | undefined,
  now = new Date(),
): SkillSpectorAssessment {
  const canonical =
    report?.schema_version === 3
      ? String(
          report.repository_aliases.find(
            (alias) => String(alias.repository).toLowerCase() === subject?.repository.toLowerCase(),
          )?.canonical_repository ?? subject?.repository,
        ).toLowerCase()
      : subject?.repository.toLowerCase();
  const records = !subject
    ? []
    : (report?.records ?? []).filter((record) => {
        const repositoryMatches =
          record.source.repository.toLowerCase() === canonical ||
          (report?.schema_version === 3 &&
            record.freshness === 'historical' &&
            report.repository_aliases.some(
              (alias) =>
                String(alias.repository).toLowerCase() === record.source.repository.toLowerCase() &&
                String(alias.canonical_repository).toLowerCase() === canonical &&
                alias.revision === record.target_revision,
            ));
        if (!repositoryMatches) return false;
        return subject.kind === 'plugin'
          ? record.kind === 'plugin_skill' && record.plugin_path === subject.path
          : record.kind === 'standalone_skill' && record.skill_name === subject.name;
      });
  const coverage =
    (report?.schema_version === 2 || report?.schema_version === 3) && subject?.kind === 'plugin'
      ? report.plugin_coverage.find(
          (item) =>
            item.repository.toLowerCase() === canonical && item.plugin_path === subject.path,
        )
      : undefined;
  const findingCount = records.reduce((sum, record) => sum + record.finding_count, 0);
  if (report?.schema_version === 3) {
    const states = subject?.kind === 'plugin' ? report.plugin_states : report.source_states;
    const state = states.find(
      (item) =>
        (String(item.repository).toLowerCase() === canonical ||
          (subject?.kind === 'skill' &&
            String(item.repository).toLowerCase() === subject.repository.toLowerCase())) &&
        (subject?.kind === 'plugin'
          ? item.plugin_path === subject.path &&
            (item.revision === null || item.revision === subject.revision)
          : item.id === `${canonical}/${subject?.name}` ||
            String(item.id).toLowerCase() ===
              `${subject?.repository.toLowerCase()}/${subject?.name}`),
    );
    const currentRecords = records.filter(
      (record) =>
        record.freshness === 'current' &&
        (subject?.kind !== 'plugin' || record.source.revision === subject.revision),
    );
    const fullyChecked = currentRecords.filter(
      (record) =>
        record.status === 'complete' &&
        record.coverage?.status === 'complete' &&
        record.execution?.state === 'completed',
    ).length;
    const selected = Math.max(
      coverage?.selected ?? records.length,
      typeof state?.skills_total === 'number' ? state.skills_total : 0,
      subject?.kind === 'skill' && Array.isArray(state?.selected_ids)
        ? state.selected_ids.length
        : 0,
    );
    // Each known Skill belongs to exactly one outcome. Pending producer coverage
    // includes historical reports; it is not proof of a queued or running job.
    const failed = currentRecords.filter(
      (record) => record.status === 'error' || record.coverage?.status === 'not_started',
    ).length;
    const outcomes = {
      complete: fullyChecked,
      limited: currentRecords.length - fullyChecked - failed,
      failed,
      historical: records.length - currentRecords.length,
      // Historical-only results are still waiting for a current check.
      unscanned: Math.max(0, selected - records.length),
    };
    const pendingIds = subject?.kind === 'plugin' && Array.isArray(state?.selected_ids)
      ? (state.selected_ids as unknown[])
          .filter((id): id is string => typeof id === 'string')
          .filter((id) => !currentRecords.some((record) => record.id === id))
      : [];
    const outOfDate = outcomes.historical > 0;
    const retries = { scheduled: 0, blocked: 0, exhausted: 0 };
    for (const record of records) {
      const disposition = record.retry?.disposition;
      if (disposition && disposition in retries) retries[disposition as keyof typeof retries]++;
    }
    const reasons = records.flatMap((record) => record.coverage?.reasons ?? []);
    if (state?.reason) reasons.push({ code: String(state.reason), message: String(state.reason) });
    if (Array.isArray(state?.candidates) && state.candidates.length)
      reasons.push({ code: 'source_candidates', message: state.candidates.join(', ') });
    const retainedUnreviewedCount = records.reduce(
      (sum, record) =>
        sum +
        record.findings
          .filter((finding) => !finding.review_status || finding.review_status === 'unreviewed')
          .reduce((count, finding) => count + (finding.raw_count ?? 1), 0),
      0,
    );
    const unreviewedCount =
      retainedUnreviewedCount +
      records.reduce(
        (sum, record) =>
          sum +
          Math.max(
            0,
            record.finding_count -
              record.findings.reduce((count, finding) => count + (finding.raw_count ?? 1), 0),
          ),
        0,
      );
    const confirmed = records.some((record) =>
      record.findings.some(
        (finding) =>
          finding.review_status === 'confirmed' && ['HIGH', 'CRITICAL'].includes(finding.severity),
      ),
    );
    const status = !records.length
      ? ['unavailable', 'unresolved'].includes(String(state?.state))
        ? 'unavailable'
        : ['unsupported', 'not_applicable'].includes(String(state?.state))
          ? 'unsupported'
          : coverage?.pending || (Array.isArray(state?.selected_ids) && state.selected_ids.length)
            ? 'pending'
            : 'notChecked'
      : confirmed
        ? 'confirmedMaterialIssue'
        : outOfDate
          ? 'stale'
          : outcomes.failed === records.length
            ? 'unavailable'
            : fullyChecked < selected ||
                records.some(
                  (record) =>
                    record.coverage?.status !== 'complete' ||
                    record.execution?.state !== 'completed',
                )
              ? 'incomplete'
              : unreviewedCount || records.some(hasIncompleteSkillSpectorFindings)
                ? 'unreviewedFindings'
                : findingCount
                  ? 'reviewedFindings'
                  : 'noStaticFindings';
    return {
      status,
      findingCount,
      unreviewedCount,
      outOfDate,
      records,
      coverage,
      fullyChecked,
      inventoryKnown: subject?.kind === 'plugin' && typeof state?.skills_total === 'number',
      selected,
      pendingIds,
      reasons,
      groupCount: records.some((record) => record.coverage?.grouping_complete === false)
        ? undefined
        : records.reduce(
            (sum, record) => sum + (record.finding_group_count ?? record.findings.length),
            0,
          ),
      rawCount: records.reduce(
        (sum, record) => sum + (record.raw_finding_count ?? record.finding_count),
        0,
      ),
      outcomes,
      retries,
      legacy: false,
    };
  }
  if (!records.length)
    return { status: 'notChecked', findingCount, outOfDate: false, records, coverage };
  const outOfDate = records.some(
    (record) =>
      (record.scanned_at !== null &&
        now.getTime() - Date.parse(record.scanned_at) > 7 * 86_400_000) ||
      (subject?.kind === 'plugin' && record.source.revision !== subject.revision),
  );
  const status = records.every((record) => record.status === 'error')
    ? 'error'
    : records.some((record) => record.status !== 'complete' || record.errors.length) ||
        coverage?.pending
      ? 'partial'
      : outOfDate
        ? 'historical'
        : findingCount
          ? 'findingsCount'
          : 'noFindings';
  return { status, findingCount, outOfDate, records, coverage };
}

export function skillSpectorSourceUrl(record: SkillSpectorRecord): string {
  const suffix = record.source.path.split('/').map(encodeURIComponent).join('/');
  return `https://github.com/${record.source.repository}/tree/${record.source.revision}${suffix ? `/${suffix}` : ''}`;
}

/** Link only in-root relative artifacts at the exact scanned revision. */
export function skillSpectorFindingUrl(
  record: SkillSpectorRecord,
  finding: Pick<SkillSpectorFinding, 'path' | 'line'>,
): string | undefined {
  const parts = finding.path.split('/');
  if (
    !repository(record.source.repository) ||
    !revision(record.source.revision) ||
    !path(record.source.path, true) ||
    !text(finding.path, 1000) ||
    /[%\\:\p{C}]/u.test(finding.path) ||
    parts.some((part) => !part || part === '.' || part === '..') ||
    (finding.line !== undefined && (!count(finding.line) || finding.line === 0))
  )
    return undefined;
  const suffix = [...(record.source.path ? record.source.path.split('/') : []), ...parts]
    .map(encodeURIComponent)
    .join('/');
  return `https://github.com/${record.source.repository}/blob/${record.source.revision}/${suffix}${finding.line ? `#L${finding.line}` : ''}`;
}

export function skillSpectorFindingLocation(
  finding: Pick<SkillSpectorFinding, 'path' | 'line'>,
): string {
  return finding.line ? `${finding.path}:${finding.line}` : finding.path;
}

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) invalid();
  return value as Record<string, unknown>;
}
function keys(
  value: Record<string, unknown>,
  required: string[],
  optional: string[] = [],
): boolean {
  return (
    required.every((key) => Object.hasOwn(value, key)) &&
    Object.keys(value).every((key) => required.includes(key) || optional.includes(key))
  );
}
function text(value: unknown, maximum: number): value is string {
  return typeof value === 'string' && Boolean(value.trim()) && value.length <= maximum;
}
function repository(value: unknown): value is string {
  return (
    text(value, 200) &&
    repositoryPattern.test(value) &&
    !value.split('/').some((part) => part === '.' || part === '..')
  );
}
function revision(value: unknown): value is string {
  return typeof value === 'string' && revisionPattern.test(value);
}
function digest(value: unknown): value is string {
  return typeof value === 'string' && digestPattern.test(value);
}
function path(value: unknown, empty = false): value is string {
  return (
    typeof value === 'string' &&
    value.length <= 1000 &&
    (empty || Boolean(value)) &&
    /^(?:[A-Za-z0-9._-]+(?:\/[A-Za-z0-9._-]+)*)?$/.test(value) &&
    !value.includes('\\') &&
    !value.startsWith('/') &&
    !value.endsWith('/') &&
    (value === '' || !value.split('/').some((part) => !part || part === '.' || part === '..'))
  );
}
function pluginSkillRoot(pluginPath: unknown, skillPath: unknown): boolean {
  if (typeof pluginPath !== 'string' || typeof skillPath !== 'string') return false;
  const prefix = `${pluginPath ? `${pluginPath}/` : ''}skills/`;
  const child = skillPath.startsWith(prefix) ? skillPath.slice(prefix.length) : '';
  return Boolean(child) && !child.includes('/');
}

function count(value: unknown): value is number {
  return Number.isSafeInteger(value) && Number(value) >= 0;
}
function timestamp(value: unknown, now: Date): value is string {
  return (
    typeof value === 'string' &&
    /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$/.test(value) &&
    Number.isFinite(Date.parse(value)) &&
    new Date(value).toISOString() === value.replace(/Z$/, '.000Z') &&
    Date.parse(value) <= now.getTime()
  );
}
function invalid(): never {
  throw new Error(`${namespace} is invalid`);
}
