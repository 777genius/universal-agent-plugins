import type {
  SkillSpectorManifestV3,
  SkillSpectorRecord,
  SkillSpectorShardDescriptor,
} from '../types/skillSpector.ts';
import exceptions from '../features/skillspector/exceptions.json' with { type: 'json' };

const digest = /^sha256:[0-9a-f]{64}$/;
const revision = /^[0-9a-f]{40}$/;
const repository = /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/;
function sameHistoricalRepository(
  left: string,
  right: string,
  targetRevision: string,
  aliases: SkillSpectorManifestV3['repository_aliases'],
) {
  const canonical = (name: string) => {
    const lower = name.toLowerCase();
    // One verified step on each side, matching the producer's same_repository contract.
    return String(
      aliases.find(
        (alias) =>
          alias.revision === targetRevision && String(alias.repository).toLowerCase() === lower,
      )?.canonical_repository ?? lower,
    ).toLowerCase();
  };
  return canonical(left) === canonical(right);
}
const fail = (): never => {
  throw new Error('SkillSpector v3 report is invalid');
};
// The decoder narrows fields at each use below.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const obj = (value: unknown): Record<string, any> =>
  value && typeof value === 'object' && !Array.isArray(value)
    ? // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (value as Record<string, any>)
    : fail();
const count = (value: unknown, max = 10_000_000) =>
  Number.isSafeInteger(value) && Number(value) >= 0 && Number(value) <= max;
const text = (value: unknown, max = 4000) =>
  typeof value === 'string' && value.length > 0 && value.length <= max;
const positivePosition = (value: unknown) => Number.isSafeInteger(value) && Number(value) > 0;
const containedReferencePath = (value: unknown) =>
  text(value, 1000) &&
  !String(value).startsWith('/') &&
  !/[\\\p{C}]/u.test(String(value)) &&
  String(value)
    .replace(/\/$/, '')
    .split('/')
    .every((part) => part && part !== '.' && part !== '..');
function shape(value: unknown, required: string[], optional: string[] = []) {
  const result = obj(value);
  if (
    !required.every((key) => Object.hasOwn(result, key)) ||
    Object.keys(result).some((key) => !required.includes(key) && !optional.includes(key))
  )
    fail();
  return result;
}
function lineRange(value: unknown) {
  const position = obj(value);
  for (const key of ['start_line', 'end_line'])
    if (position[key] !== undefined && position[key] !== null && !positivePosition(position[key]))
      fail();
  if (
    position.start_line !== undefined &&
    position.start_line !== null &&
    position.end_line !== undefined &&
    position.end_line !== null &&
    position.end_line < position.start_line
  )
    fail();
}
function referenceContext(value: unknown, status: string, legacy: boolean) {
  const c = obj(value);
  const fields = [
    'references',
    'ledger_exceptions',
    'references_total',
    'ledger_exceptions_total',
    'reference_evidence_truncated',
  ];
  if (!fields.some((key) => Object.hasOwn(c, key))) return;
  if (
    !fields.every((key) => Object.hasOwn(c, key)) ||
    !Array.isArray(c.references) ||
    c.references.length > 1024 ||
    !Array.isArray(c.ledger_exceptions) ||
    c.ledger_exceptions.length > 100_000 ||
    !count(c.references_total) ||
    !count(c.ledger_exceptions_total) ||
    c.references_total < c.references.length ||
    c.ledger_exceptions_total < c.ledger_exceptions.length ||
    typeof c.reference_evidence_truncated !== 'boolean'
  )
    fail();
  const omitted =
    c.references_total > c.references.length ||
    c.ledger_exceptions_total > c.ledger_exceptions.length;
  if (
    c.reference_evidence_truncated !== omitted ||
    (omitted &&
      (status === 'complete' ||
        c.status !== 'partial' ||
        !c.reasons.some(
          (reason: Record<string, unknown>) => reason.code === 'normalization_truncated',
        )))
  )
    fail();
  if (
    new TextEncoder().encode(JSON.stringify(c.references)).byteLength +
      new TextEncoder().encode(JSON.stringify(c.ledger_exceptions)).byteLength -
      4 >
    1 << 20
  )
    fail();
  for (const item of c.references) {
    const reference = shape(
      item,
      [
        'source_path',
        'line',
        'column',
        'evidence',
        'target_path',
        'status',
        'disposition',
        'reference_kind',
      ],
      ['source_sha256'],
    );
    if (
      !containedReferencePath(reference.source_path) ||
      !positivePosition(reference.line) ||
      !positivePosition(reference.column) ||
      typeof reference.evidence !== 'string' ||
      reference.evidence.length > 8 << 20 ||
      !(reference.target_path === null || containedReferencePath(reference.target_path)) ||
      !['resolved', 'missing', 'ambiguous', 'rejected', 'uninspected'].includes(reference.status) ||
      !['analyzed', 'partial', 'failed', 'out_of_scope'].includes(reference.disposition) ||
      ![
        'markdown_image',
        'documentation_image',
        'markdown_link',
        'inline_command',
        'quoted_or_code',
        'plain_path',
      ].includes(reference.reference_kind) ||
      (reference.source_sha256 !== undefined &&
        (typeof reference.source_sha256 !== 'string' ||
          !/^[0-9a-f]{64}$/.test(reference.source_sha256)))
    )
      fail();
    if ((reference.target_path !== null) !== ['resolved', 'uninspected'].includes(reference.status)) fail();
    if (reference.status === 'uninspected' && reference.disposition !== 'partial') fail();
    if (
      (['missing', 'ambiguous', 'uninspected'].includes(reference.status) ||
        (reference.status === 'resolved' && reference.disposition !== 'analyzed')) &&
      (status === 'complete' || c.status === 'complete')
    )
      fail();
  }
  for (const item of c.ledger_exceptions) {
    const diagnostic = shape(
      item,
      ['reason_code', 'message'],
      [
        'path',
        'start_line',
        'end_line',
        'outcome',
        'phase',
        'error_class',
        'analyzers',
        'fatal',
        'scope',
      ],
    );
    if (
      !text(diagnostic.reason_code, 120) ||
      !text(diagnostic.message, 8 << 20) ||
      (diagnostic.path !== undefined && !containedReferencePath(diagnostic.path)) ||
      (diagnostic.outcome !== undefined &&
        !['completed', 'partial', 'failed', 'skipped', 'out_of_scope'].includes(
          diagnostic.outcome,
        )) ||
      (diagnostic.fatal !== undefined && typeof diagnostic.fatal !== 'boolean')
    )
      fail();
    for (const key of ['phase', 'error_class', 'scope'])
      if (diagnostic[key] !== undefined && !text(diagnostic[key], 200)) fail();
    if (
      diagnostic.analyzers !== undefined &&
      (!Array.isArray(diagnostic.analyzers) ||
        diagnostic.analyzers.length > 1000 ||
        !diagnostic.analyzers.every((analyzer: unknown) => text(analyzer, 200)))
    )
      fail();
    lineRange(diagnostic);
  }
  if (status === 'complete' && !legacy && c.ledger_exceptions.length) fail();
}
const safePath = (value: unknown, empty = false) =>
  typeof value === 'string' &&
  value.length <= 1000 &&
  (empty || value.length > 0) &&
  !value.startsWith('/') &&
  !/[\\%:\p{C}]/u.test(value) &&
  (value === '' || value.split('/').every((part) => part && !['.', '..', '.git'].includes(part)));
function jsonValue(value: unknown, depth = 0): void {
  if (depth > 64 || (typeof value === 'number' && !Number.isFinite(value))) fail();
  if (value && typeof value === 'object') {
    const keys = new Set<string>();
    for (const [key, child] of Object.entries(value)) {
      const normalized = key.normalize('NFC').toLowerCase();
      if (keys.has(normalized)) fail();
      keys.add(normalized);
      jsonValue(child, depth + 1);
    }
  }
}
const time = (value: unknown, now: Date) =>
  typeof value === 'string' &&
  /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$/.test(value) &&
  Number.isFinite(Date.parse(value)) &&
  new Date(value).toISOString() === value.replace(/Z$/, '.000Z') &&
  Date.parse(value) <= now.getTime();
const profile = (value: unknown) => {
  const p = obj(value);
  if (!text(p.id, 100) || !digest.test(p.digest)) fail();
  return p;
};
const scanner = (value: unknown) => {
  const s = obj(value);
  if (s.id !== 'skillspector' || !text(s.version, 100) || !revision.test(s.revision)) fail();
  return s;
};
// Full v3 inventory includes signed community package roots; transport and per-package
// limits below remain independent of this global coverage budget.
const maximumInventoryRoots = 20_000;
// Listing observations may coexist with inventory roots not present in the listing.
const maximumSourceStates = maximumInventoryRoots + 10_000;
function coverage(value: unknown) {
  const c = obj(value);
  for (const key of ['selected', 'reported', 'pending', 'complete', 'partial', 'error', 'stale'])
    if (!count(c[key], maximumInventoryRoots)) fail();
  if (
    c.selected !== c.complete + c.partial + c.error + c.pending ||
    c.reported !== c.complete + c.partial + c.error + c.stale ||
    c.stale > c.pending
  )
    fail();
  return c;
}

/** v3 distinguishes a published report from a fully inspected source. */
export function decodeSkillSpectorManifestV3(
  value: unknown,
  now = new Date(),
): SkillSpectorManifestV3 {
  const m = obj(value);
  jsonValue(m);
  if (
    m.schema_version !== 3 ||
    m.scope !== 'selected_sources' ||
    !time(m.generated_at, now) ||
    !revision.test(m.source_commit) ||
    !digest.test(m.selection_digest)
  )
    fail();
  scanner(m.scanner);
  if (
    m.scanner.version !== '2.13.1' ||
    m.scanner.revision !== 'f3b25697d76256998ca9d42c198d256258d4c035'
  )
    fail();
  if (profile(m.profile).id !== 'static-policy-v3') fail();
  if (!Array.isArray(m.profiles) || !m.profiles.length || m.profiles.length > 8) fail();
  const profiles = new Set<string>();
  for (const p of m.profiles) {
    profile(p);
    if (profiles.has(p.id) || !['static-standard-v3', 'static-extended-v3'].includes(p.id)) fail();
    profiles.add(p.id);
  }
  const c = coverage(m.coverage);
  if (
    !Array.isArray(m.shards) ||
    m.shards.length > 10_000 ||
    !Array.isArray(m.plugin_coverage) ||
    m.plugin_coverage.length > maximumInventoryRoots
  )
    fail();
  let records = 0,
    bytes = 0;
  const paths = new Set<string>();
  for (const value of m.shards) {
    const s = obj(value);
    if (
      !repository.test(s.repository) ||
      !/^shards\/[0-9a-f]{64}\.json$/.test(s.path) ||
      s.digest !== `sha256:${s.path.slice(7, -5)}` ||
      paths.has(s.path) ||
      !count(s.bytes, 8 << 20) ||
      !s.bytes ||
      !count(s.record_count, 256) ||
      !s.record_count
    )
      fail();
    paths.add(s.path);
    records += s.record_count;
    bytes += s.bytes;
  }
  if (records !== c.reported || bytes > 256 << 20) fail();
  const plugins = new Set<string>();
  for (const value of m.plugin_coverage) {
    const p = obj(value);
    coverage(p);
    if (
      !repository.test(p.repository) ||
      typeof p.plugin_path !== 'string' ||
      !/^(?:[A-Za-z0-9._-]+(?:\/[A-Za-z0-9._-]+)*)?$/.test(p.plugin_path) ||
      p.plugin_path.split('/').some((s: string) => ['.', '..', '.git'].includes(s))
    )
      fail();
    const key = JSON.stringify([p.repository.toLowerCase(), p.plugin_path]);
    if (plugins.has(key)) fail();
    plugins.add(key);
  }
  for (const key of ['source_states', 'plugin_states', 'repository_aliases'])
    if (
      !Array.isArray(m[key]) ||
      m[key].length >
        (key === 'source_states'
          ? maximumSourceStates
          : key === 'plugin_states'
            ? maximumInventoryRoots
            : 10_000)
    )
      fail();
  const aliases = new Map<string, string>();
  for (const alias of m.repository_aliases) {
    const a = obj(alias);
    if (
      !repository.test(a.repository) ||
      !repository.test(a.canonical_repository) ||
      !revision.test(a.revision)
    )
      fail();
    const original = a.repository.toLowerCase(),
      canonical = a.canonical_repository.toLowerCase();
    if (aliases.has(original)) fail();
    aliases.set(original, canonical);
  }
  for (const original of aliases.keys()) {
    const visited = new Set<string>();
    let current = original;
    while (aliases.has(current) && aliases.get(current) !== current) {
      if (visited.has(current)) fail();
      visited.add(current);
      current = aliases.get(current)!;
    }
  }
  for (const state of m.source_states) {
    const s = obj(state);
    if (
      !text(s.id, 1025) ||
      typeof s.repository !== 'string' ||
      s.repository.length > 1000 ||
      !['resolved', 'historical', 'unresolved', 'unavailable', 'unsupported'].includes(s.state) ||
      typeof s.current !== 'boolean' ||
      !(s.reason === null || text(s.reason, 200)) ||
      !(s.revision === null || revision.test(s.revision)) ||
      !(s.path === null || safePath(s.path, true)) ||
      !Array.isArray(s.selected_ids) ||
      !s.selected_ids.every((id: unknown) => text(id, 200)) ||
      !Array.isArray(s.candidates) ||
      !s.candidates.every((path: unknown) => safePath(path, true))
    )
      fail();
  }
  for (const state of m.plugin_states) {
    const s = obj(state);
    if (
      !text(s.id, 200) ||
      !repository.test(s.repository) ||
      !safePath(s.plugin_path, true) ||
      !['resolved', 'not_applicable', 'unresolved', 'unavailable'].includes(s.state) ||
      !(s.reason === null || text(s.reason, 200)) ||
      !(s.revision === null || revision.test(s.revision)) ||
      !(s.skills_total === null || count(s.skills_total, 10_000)) ||
      !Array.isArray(s.selected_ids) ||
      !s.selected_ids.every((id: unknown) => text(id, 200))
    )
      fail();
  }
  for (const key of ['selected', 'reported', 'pending', 'complete', 'partial', 'error', 'stale'])
    if (
      m.plugin_coverage.reduce((sum: number, item: Record<string, number>) => sum + item[key]!, 0) >
      c[key]
    )
      fail();
  return value as SkillSpectorManifestV3;
}

export function decodeSkillSpectorShardV3(
  value: unknown,
  manifest: SkillSpectorManifestV3,
  descriptor: SkillSpectorShardDescriptor,
  now = new Date(),
): SkillSpectorRecord[] {
  const s = obj(value);
  jsonValue(s);
  if (
    s.schema_version !== 3 ||
    s.repository !== descriptor.repository ||
    JSON.stringify(s.scanner) !== JSON.stringify(manifest.scanner) ||
    JSON.stringify(s.profile) !== JSON.stringify(manifest.profile) ||
    !Array.isArray(s.records) ||
    s.records.length !== descriptor.record_count
  )
    fail();
  const ids = new Set<string>();
  for (const value of s.records) {
    const r = obj(value),
      source = obj(r.source),
      c = obj(r.coverage),
      e = obj(r.execution);
    if (
      !text(r.id, 200) ||
      ids.has(r.id) ||
      !['plugin_skill', 'standalone_skill'].includes(r.kind) ||
      !repository.test(source.repository) ||
      !(
        source.repository.toLowerCase() === descriptor.repository.toLowerCase() ||
        (r.freshness === 'historical' &&
          sameHistoricalRepository(
            source.repository,
            descriptor.repository,
            r.target_revision,
            manifest.repository_aliases,
          ))
      ) ||
      !revision.test(source.revision) ||
      !revision.test(r.target_revision) ||
      !['current', 'historical'].includes(r.freshness) ||
      !['complete', 'partial', 'error'].includes(r.status)
    )
      fail();
    ids.add(r.id);
    if (
      typeof source.path !== 'string' ||
      source.path.split('/').some((part: string) => ['.', '..', '.git'].includes(part)) ||
      /[\\%:\p{C}]/u.test(source.path) ||
      source.path.startsWith('/') ||
      !/^(?:[A-Za-z0-9._-]+(?:\/[A-Za-z0-9._-]+)*)?$/.test(source.path) ||
      !(source.content_digest === null || digest.test(source.content_digest)) ||
      !(r.skill_name === null || text(r.skill_name, 200)) ||
      (r.kind === 'standalone_skill' && r.plugin_path !== null) ||
      (r.kind === 'plugin_skill' &&
        (typeof r.plugin_path !== 'string' ||
          !safePath(r.plugin_path, true) ||
          !/^(?:[A-Za-z0-9._-]+(?:\/[A-Za-z0-9._-]+)*)?$/.test(r.plugin_path)))
    )
      fail();
    if (r.kind === 'plugin_skill') {
      const prefix = `${r.plugin_path ? `${r.plugin_path}/` : ''}skills/`;
      const child = source.path.startsWith(prefix) ? source.path.slice(prefix.length) : '';
      if (!child || child.includes('/')) fail();
    }
    scanner(r.scanner);
    profile(r.profile);
    if (
      r.legacy_format !== undefined &&
      (r.freshness !== 'historical' || !['static-only-v1', 'static-only-v2'].includes(r.profile.id))
    )
      fail();
    if (
      r.freshness === 'current' &&
      (r.source.revision !== r.target_revision ||
        r.scanner.id !== manifest.scanner.id ||
        r.scanner.version !== manifest.scanner.version ||
        r.scanner.revision !== manifest.scanner.revision)
    )
      fail();
    if (
      !manifest.profiles.some((p) => p.id === r.profile.id && p.digest === r.profile.digest) &&
      !(r.freshness === 'historical' && ['static-only-v1', 'static-only-v2'].includes(r.profile.id))
    )
      fail();
    if (!['complete', 'partial', 'not_started'].includes(c.status)) fail();
    for (const key of [
      'fully_inspected_files',
      'partially_inspected_files',
      'entirely_uninspected_files',
    ])
      if (!count(c[key])) fail();
    if (
      !Array.isArray(c.reasons) ||
      !Array.isArray(c.analyzers) ||
      !Array.isArray(c.original_findings)
    )
      fail();
    for (const reason of c.reasons) {
      if (!text(obj(reason).code, 120) || !text(reason.message)) fail();
      lineRange(reason);
    }
    referenceContext(c, r.status, Boolean(r.legacy_format));
    if (c.grouping_complete !== undefined && typeof c.grouping_complete !== 'boolean') fail();
    if (
      !r.legacy_format &&
      c.grouping_complete === false &&
      (!r.findings_truncated ||
        r.status !== 'partial' ||
        c.status !== 'partial' ||
        !c.reasons.some((reason: Record<string, unknown>) =>
          ['output_limit', 'normalization_truncated'].includes(String(reason.code)),
        ))
    )
      fail();
    if (
      !['queued', 'running', 'completed', 'failed'].includes(e.state) ||
      !(e.exit_code === null || Number.isSafeInteger(e.exit_code)) ||
      !(e.acquisition_seconds === null || Number.isFinite(e.acquisition_seconds)) ||
      e.acquisition_seconds < 0 ||
      !(e.analysis_seconds === null || Number.isFinite(e.analysis_seconds)) ||
      e.analysis_seconds < 0
    )
      fail();
    for (const t of [e.started_at, e.finished_at, r.scanned_at])
      if (
        !(t === null || time(t, now)) ||
        (t !== null && Date.parse(t) > Date.parse(manifest.generated_at))
      )
        fail();
    if (
      r.status !== 'error' &&
      (!digest.test(source.content_digest) ||
        !digest.test(r.report_digest) ||
        r.scanned_at === null)
    )
      fail();
    if (
      c.status === 'complete' &&
      (c.reasons.length ||
        c.partially_inspected_files ||
        c.entirely_uninspected_files ||
        e.state !== 'completed' ||
        r.status !== 'complete')
    )
      fail();
    if (r.status === 'complete' && !r.legacy_format) {
      const complete = new Set(
        c.analyzers
          .filter((item: Record<string, unknown>) => item.status === 'completed')
          .map((item: Record<string, unknown>) => item.analyzer_id),
      );
      if (
        ![0, 1].includes(e.exit_code) ||
        !c.fully_inspected_files ||
        r.errors.length ||
        !['artifact_integrity', 'static_patterns_prompt_injection', 'static_yara'].every((id) =>
          complete.has(id),
        )
      )
        fail();
      for (const analyzer of c.analyzers) {
        if (
          !['completed', 'not_applicable', 'disabled'].includes(analyzer.status) ||
          (analyzer.status === 'disabled' &&
            ![
              'semantic_developer_intent',
              'semantic_quality_policy',
              'semantic_security_discovery',
              'meta_analyzer',
            ].includes(analyzer.analyzer_id)) ||
          ['failed', 'partial', 'skipped', 'unaccounted'].some((key) => (analyzer[key] ?? 0) !== 0)
        )
          fail();
      }
    }
    if (
      !count(r.finding_count) ||
      !count(r.raw_finding_count) ||
      !count(r.finding_group_count) ||
      !Array.isArray(r.findings) ||
      r.findings.length > 100_000 ||
      r.findings.length > r.finding_group_count ||
      typeof r.findings_truncated !== 'boolean' ||
      !Array.isArray(r.errors)
    )
      fail();
    const findingIds = new Set<string>();
    for (const value of r.findings) {
      const f = obj(value);
      if (
        !text(f.id, 200) ||
        findingIds.has(f.id) ||
        !text(f.rule, 120) ||
        !['INFO', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL'].includes(f.severity) ||
        !text(f.message, 8 << 20) ||
        !text(f.path, 1000) ||
        !(f.confidence === null || Number.isFinite(f.confidence)) ||
        f.confidence < 0 ||
        f.confidence > 1 ||
        !['unreviewed', 'confirmed', 'false_positive', 'accepted'].includes(f.review_status) ||
        !Array.isArray(f.finding_ids) ||
        !Array.isArray(f.occurrences) ||
        !count(f.raw_count) ||
        !f.raw_count
      )
        fail();
      findingIds.add(f.id);
      obj(f.evidence);
      if (['confirmed', 'accepted'].includes(f.review_status)) fail();
      if (f.review_status === 'false_positive') {
        const annotation = obj(f.exception);
        const match = exceptions.exceptions.find((item) => item.id === annotation.id);
        if (
          r.scanner.version !== '2.13.1' ||
          !match ||
          !exceptions.profiles.some(
            (profile) => profile.id === r.profile.id && profile.digest === r.profile.digest,
          ) ||
          f.rule !== 'E4' ||
          f.severity !== 'HIGH' ||
          f.path !== match.path ||
          f.line !== match.line ||
          f.match_fingerprint !== match.match_fingerprint ||
          f.code_snippet !== match.code_snippet ||
          Object.entries(exceptions.source).some(([key, value]) => source[key] !== value) ||
          r.scanner.revision !== exceptions.scanner_revision ||
          annotation.reason !== exceptions.reason ||
          annotation.source_content_digest !== source.content_digest ||
          annotation.scanner_revision !== r.scanner.revision ||
          annotation.profile_digest !== r.profile.digest ||
          annotation.match_fingerprint !== match.match_fingerprint ||
          annotation.snippet_digest !== match.snippet_digest ||
          f.occurrences.some(
            (item: Record<string, unknown>) => item.path !== match.path || item.line !== match.line,
          )
        )
          fail();
      } else if (f.exception !== undefined) fail();
      if (!digest.test(f.id) || !f.finding_ids.every((id: unknown) => text(id, 1000))) fail();
      for (const o of [f, ...f.occurrences]) {
        if (!safePath(typeof obj(o).path === 'string' ? o.path.replace(/\/$/, '') : o.path)) fail();
        for (const key of ['line', 'end_line', 'start_column', 'end_column'])
          if (
            o[key] !== undefined &&
            (!count(o[key]) || ((key === 'line' || key === 'end_line') && o[key] === 0))
          )
            fail();
        if (o.line !== undefined && o.end_line !== undefined && o.end_line < o.line) fail();
      }
    }
    if (
      r.raw_finding_count < r.finding_count ||
      r.finding_count < r.findings.length ||
      (!r.legacy_format &&
        c.grouping_complete !== false &&
        (r.findings_truncated !== r.finding_group_count > r.findings.length ||
          (!r.findings_truncated &&
            r.finding_count !==
              r.findings.reduce(
                (sum: number, finding: Record<string, number>) => sum + finding.raw_count!,
                0,
              ))))
    )
      fail();
    const retry = obj(r.retry);
    if (
      !count(retry.attempt_count) ||
      !['none', 'scheduled', 'blocked', 'exhausted'].includes(retry.disposition) ||
      !(
        retry.next_attempt_at === null ||
        (typeof retry.next_attempt_at === 'string' &&
          Number.isFinite(Date.parse(retry.next_attempt_at)))
      ) ||
      !Array.isArray(r.attempt_history)
    )
      fail();
  }
  return s.records as SkillSpectorRecord[];
}
