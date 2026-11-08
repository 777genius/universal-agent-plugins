import type { SkillSpectorDisplayReport, SkillSpectorRecord } from '../types/skillSpector.ts';

const latestPattern = /^https:\/\/raw\.githubusercontent\.com\/777genius\/universal-agent-plugins-registry\/(?:skillspector-cohort-releases|[0-9a-f]{40})\/reports\/skillspector\/cohort\/v1\/latest\.json$/;
const root = 'reports/skillspector/cohort/v1/';
const digestPattern = /^sha256:[0-9a-f]{64}$/;
const shaPattern = /^[0-9a-f]{40}$/;
const releasePattern = /^releases\/([0-9a-f]{64})\.json$/;
const envelopePattern = /^envelopes\/([0-9a-f]{64})\.json$/;
const signatureDomain = 'UAP-SKILLSPECTOR-COHORT-RELEASE-ED25519-V1';
const expectedScanner = {
  id: 'skillspector',
  version: '2.13.3',
  revision: '604a1cff1249ec4023012be4bdb77eac20621893',
};
const expectedProfile = {
  id: 'static-cohort-2.13.3-v1',
  digest: 'sha256:ad4477fa9f9d61952c5fa929fc6810b0792433a1494102308a1e8e183ba6fe74',
};
const sourcePins = new Map<string, readonly [string, readonly string[], string]>([
  ['plugin:community:3b7213be12f01b1365846d8ba375d59f/sitemap-manager',
    ['indranilbanerjee/digital-marketing-pro',
      ['cc45ca06eaf48abde3bb60fb79d31c41229c58db', 'be4142b25bba6d424962a7788432dd813e6b768c'],
      'skills/sitemap-manager']],
  ['plugin:community:4d73239dc1cf321957d109bdf29c27dd/sitemap-manager',
    ['teachskillofskills-ai/digitalmarketingpro-techshu', ['e97a5bdd91601024a9cfa870807a8d3c81dc3caf'], 'skills/sitemap-manager']],
  ...['braids', 'braids-audit', 'braids-claims', 'braids-depth', 'braids-help', 'braids-review', 'braids-risk']
    .map((name): [string, readonly [string, readonly string[], string]] => [
      'plugin:community:b0fa2f8fff3c4570b38ffe3e0c1401a7/' + name,
      ['vyas-devgna/braids', ['79c913e9d27854d88c7ee58aa9906bd2b45001b4'], 'skills/' + name],
    ]),
]);

type Json = Record<string, unknown>;
export interface TrustedCohortKey { key_id: string; public_key: string }
export interface VerifiedCohort {
  expiresAt: number;
  manifestDigest: string;
  oldReportCommit: string;
  records: ReadonlyMap<string, SkillSpectorRecord>;
}

const invalid = (): never => { throw new Error('SkillSpector signed cohort is invalid'); };
const obj = (value: unknown): Json =>
  value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Json : invalid();
const exact = (value: Json, names: readonly string[]): void => {
  if (Object.keys(value).length !== names.length || !names.every(k => Object.hasOwn(value, k))) invalid();
};
const int = (value: unknown): value is number => Number.isSafeInteger(value) && Number(value) >= 0;
const array = (value: unknown): unknown[] => Array.isArray(value) ? value : invalid();
const same = (a: unknown, b: unknown): boolean => canonical(a) === canonical(b);

function canonical(value: unknown): string {
  if (Array.isArray(value)) return '[' + value.map(canonical).join(',') + ']';
  if (value !== null && typeof value === 'object')
    return '{' + Object.keys(value).sort().map(k => JSON.stringify(k) + ':' + canonical(obj(value)[k])).join(',') + '}';
  if (typeof value === 'number' && !Number.isFinite(value)) invalid();
  const serialized = JSON.stringify(value);
  if (typeof serialized !== 'string') invalid();
  return serialized;
}
function canonicalBytes(body: Uint8Array): Json {
  const doc = obj(JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(body)));
  if (new TextDecoder().decode(body) !== canonical(doc) + '\n') invalid();
  return doc;
}
async function digest(body: Uint8Array): Promise<string> {
  const hash = new Uint8Array(await crypto.subtle.digest('SHA-256', body as BufferSource));
  return 'sha256:' + [...hash].map(b => b.toString(16).padStart(2, '0')).join('');
}
function bytesFromBase64(value: unknown, size: number): Uint8Array {
  if (typeof value !== 'string') return invalid();
  if (!/^[A-Za-z0-9+/]+={0,2}$/.test(value)) invalid();
  const data = atob(value);
  const bytes = Uint8Array.from(data, c => c.charCodeAt(0));
  if (bytes.length !== size || btoa(data) !== value) invalid();
  return bytes;
}
function asTime(value: unknown): number {
  if (typeof value !== 'string') return invalid();
  if (!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$/.test(value)) invalid();
  const time = Date.parse(value);
  if (!Number.isFinite(time) || new Date(time).toISOString() !== value.replace('Z', '.000Z')) invalid();
  return time;
}
async function boundedFetch(url: string, fetcher: typeof fetch, max: number): Promise<Uint8Array> {
  const response = await fetcher(url, {
    credentials: 'omit', redirect: 'error', signal: AbortSignal.timeout(12000),
  });
  if (!response.ok || (Number(response.headers.get('content-length')) || 0) > max) invalid();
  const reader = response.body?.getReader();
  if (!reader) {
    const body = new Uint8Array(await response.arrayBuffer());
    if (body.length > max) invalid();
    return body;
  }
  const pieces: Uint8Array[] = [];
  let size = 0;
  try {
    while (true) {
      const chunk = await reader.read();
      if (chunk.done) break;
      size += chunk.value.length;
      if (size > max) invalid();
      pieces.push(chunk.value);
    }
  } finally {
    reader.releaseLock();
  }
  const result = new Uint8Array(size);
  let offset = 0;
  for (const part of pieces) { result.set(part, offset); offset += part.length; }
  return result;
}
function validateRecords(release: Json, pointer: Json): VerifiedCohort {
  exact(release, ['schema_version', 'kind', 'source_commit', 'generated_at',
    'expires_at', 'candidate', 'signed_sources']);
  const candidate = obj(release.candidate);
  const signed = obj(release.signed_sources);
  exact(signed, ['schema_version', 'kind', 'base_report_commit', 'base_manifest_digest',
    'community', 'directory', 'source_pins_verified_against_fresh_signed_feeds',
    'provider_listing_verified', 'candidate_scan_verified', 'publication_authorized', 'sources']);
  for (const name of ['community', 'directory']) {
    const identity = obj(signed[name]);
    exact(identity, ['sequence', 'source_commit', 'snapshot_digest']);
    if (!int(identity.sequence) || identity.sequence < 1 ||
        !shaPattern.test(String(identity.source_commit)) ||
        !digestPattern.test(String(identity.snapshot_digest))) invalid();
  }
  const records = array(candidate.records).map(obj);
  const pins = array(signed.sources).map(obj);
  const comparisons = array(candidate.comparisons).map(obj);
  if (release.schema_version !== 1 || release.kind !== 'skillspector-cohort-release-v1' ||
      release.source_commit !== pointer.source_commit || release.generated_at !== pointer.generated_at ||
      release.expires_at !== pointer.expires_at || !shaPattern.test(String(release.source_commit)) ||
      candidate.schema_version !== 1 || candidate.kind !== 'skillspector-2.13.3-nine-source-candidate' ||
      candidate.candidate_status !== 'measured_complete_unpublished' ||
      candidate.live_selection_verified !== false || candidate.publication_authorized !== false ||
      !same(candidate.scanner, expectedScanner) || !same(candidate.profile, expectedProfile) ||
      candidate.profile_policy_digest !== expectedProfile.digest ||
      candidate.base_report_commit !== pointer.report_commit ||
      !digestPattern.test(String(candidate.base_manifest_digest)) ||
      signed.schema_version !== 1 ||
      signed.kind !== 'skillspector-signed-community-cohort-preflight' ||
      signed.source_pins_verified_against_fresh_signed_feeds !== true ||
      signed.publication_authorized !== false || signed.provider_listing_verified !== false ||
      signed.candidate_scan_verified !== false ||
      signed.base_report_commit !== pointer.report_commit ||
      signed.base_manifest_digest !== candidate.base_manifest_digest ||
      records.length !== 9 || pins.length !== 9 || comparisons.length !== 9) invalid();

  const mapped = new Map<string, SkillSpectorRecord>();
  const pinned = new Map<string, Json>(pins.map(pin => [String(pin.id), pin]));
  const compared = new Map<string, Json>(comparisons.map(item => [String(item.id), item]));
  if (pinned.size !== 9 || compared.size !== 9) invalid();
  for (const raw of records) {
    const expected = sourcePins.get(String(raw.id));
    const pin = pinned.get(String(raw.id));
    const comp = compared.get(String(raw.id));
    const source = obj(raw.source);
    const coverage = obj(raw.coverage);
    const execution = obj(raw.execution);
    if (!expected || !pin || !comp || mapped.has(String(raw.id)) ||
        raw.kind !== 'plugin_skill' || raw.plugin_path !== '' ||
        raw.status !== 'complete' || coverage.status !== 'complete' ||
        !same(raw.scanner, expectedScanner) || !same(raw.profile, expectedProfile) ||
        source.repository !== expected[0] || typeof source.revision !== 'string' ||
        !expected[1].includes(source.revision) ||
        source.path !== expected[2] || !digestPattern.test(String(source.content_digest)) ||
        pin.id !== raw.id ||
        !/^community:[0-9a-f]{32}$/.test(String(pin.signed_package_id)) ||
        !String(raw.id).startsWith('plugin:' + String(pin.signed_package_id) + '/') ||
        pin.repository !== source.repository ||
        pin.revision !== source.revision || pin.path !== source.path ||
        pin.content_digest !== source.content_digest || pin.plugin_path !== raw.plugin_path ||
        !digestPattern.test(String(pin.signed_package_tree_digest)) ||
        !digestPattern.test(String(pin.signed_package_manifest_digest)) ||
        comp.id !== raw.id || comp.source_digest !== source.content_digest ||
        comp.candidate_exit_code !== execution.exit_code ||
        comp.candidate_findings !== raw.finding_count ||
        !int(comp.old_findings) || typeof comp.findings_changed !== 'boolean' ||
        !int(raw.finding_count) || !int(raw.raw_finding_count) ||
        !int(raw.finding_group_count) || !Array.isArray(raw.findings) ||
        raw.finding_group_count !== raw.findings.length ||
        Number(raw.raw_finding_count) < Number(raw.finding_count) ||
        Number(raw.finding_count) < Number(raw.finding_group_count) ||
        raw.findings_truncated !== false || array(raw.errors).length !== 0 ||
        execution.state !== 'completed' || ![0, 1].includes(Number(execution.exit_code)) ||
        !int(execution.exit_code) ||
        !int(coverage.fully_inspected_files) || Number(coverage.fully_inspected_files) < 1 ||
        coverage.partially_inspected_files !== 0 || coverage.entirely_uninspected_files !== 0 ||
        coverage.grouping_complete !== true || coverage.reference_evidence_truncated !== false ||
        array(coverage.reasons).length !== 0 || array(coverage.ledger_exceptions).length !== 0 ||
        coverage.ledger_exceptions_total !== 0 ||
        coverage.references_total !== array(coverage.references).length ||
        array(coverage.references).some(ref => {
          const r = obj(ref);
          return r.status !== 'resolved' || r.disposition !== 'analyzed';
        })) invalid();
    mapped.set(String(raw.id), raw as unknown as SkillSpectorRecord);
  }
  if (mapped.size !== 9) invalid();
  return {
    expiresAt: asTime(release.expires_at),
    manifestDigest: String(candidate.base_manifest_digest),
    oldReportCommit: String(candidate.base_report_commit),
    records: mapped,
  };
}

/** Cryptographically verify a cohort with a pinned independent key; never fetch untrusted keys. */
export async function readVerifiedSkillSpectorCohort(
  latestUrl: string,
  keys: readonly TrustedCohortKey[],
  fetcher: typeof fetch = fetch,
  now: Date = new Date(),
): Promise<VerifiedCohort | undefined> {
  if (!keys.length) return undefined; // A missing production public key must keep old Partial.
  if (!latestPattern.test(latestUrl)) invalid();
  const pointerBytes = await boundedFetch(latestUrl, fetcher, 4096);
  const pointer = canonicalBytes(pointerBytes);
  exact(pointer, ['schema_version', 'kind', 'sequence', 'source_commit', 'report_commit',
    'release_path', 'envelope_path', 'release_digest', 'envelope_digest',
    'generated_at', 'expires_at']);
  if (pointer.schema_version !== 1 || pointer.kind !== 'skillspector-cohort-latest-v1' ||
      !int(pointer.sequence) || Number(pointer.sequence) < 1 ||
      !shaPattern.test(String(pointer.source_commit)) ||
      !shaPattern.test(String(pointer.report_commit)) ||
      !digestPattern.test(String(pointer.release_digest)) ||
      !digestPattern.test(String(pointer.envelope_digest))) invalid();
  const released = asTime(pointer.generated_at);
  const expires = asTime(pointer.expires_at);
  if (released > now.getTime() + 300000 || expires <= now.getTime() ||
      expires <= released || expires - released > 86400000) invalid();
  const relativeRelease = String(pointer.release_path).slice(root.length);
  const relativeEnvelope = String(pointer.envelope_path).slice(root.length);
  const releaseMatch = releasePattern.exec(relativeRelease);
  const envelopeMatch = envelopePattern.exec(relativeEnvelope);
  if (!String(pointer.release_path).startsWith(root) || !String(pointer.envelope_path).startsWith(root) ||
      !releaseMatch || !envelopeMatch || pointer.release_digest !== 'sha256:' + releaseMatch[1] ||
      pointer.envelope_digest !== 'sha256:' + envelopeMatch[1]) invalid();
  const base = latestUrl.slice(0, -('latest.json'.length));
  const [body, sealed] = await Promise.all([
    boundedFetch(base + relativeRelease, fetcher, 2 << 20),
    boundedFetch(base + relativeEnvelope, fetcher, 4096),
  ]);
  if (await digest(body) !== pointer.release_digest ||
      await digest(sealed) !== pointer.envelope_digest) invalid();
  const envelope = canonicalBytes(sealed);
  exact(envelope, ['schema_version', 'kind', 'algorithm', 'signature_domain',
    'snapshot_digest', 'key_id', 'signature']);
  const key = keys.find(k => k.key_id === envelope.key_id);
  if (!key) return invalid();
  if (envelope.schema_version !== 1 || envelope.kind !== 'skillspector-cohort-release-envelope-v1' ||
      envelope.algorithm !== 'Ed25519' || envelope.signature_domain !== signatureDomain ||
      envelope.snapshot_digest !== pointer.release_digest ||
      !/^[a-z0-9][a-z0-9._-]*$/.test(key.key_id)) invalid();
  const pub = await crypto.subtle.importKey(
    'raw', bytesFromBase64(key.public_key, 32) as BufferSource,
    { name: 'Ed25519' }, false, ['verify']);
  const prefix = new TextEncoder().encode(signatureDomain + '\0');
  const message = new Uint8Array(prefix.length + 8 + body.length);
  message.set(prefix);
  const data = new DataView(message.buffer);
  data.setBigUint64(prefix.length, BigInt(body.length), false);
  message.set(body, prefix.length + 8);
  if (!await crypto.subtle.verify(
    'Ed25519', pub, bytesFromBase64(envelope.signature, 64) as BufferSource,
    message as BufferSource,
  )) invalid();
  const release = obj(JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(body)));
  return validateRecords(release, pointer);
}

/** Promote only byte-identical, currently resolved sources from an Ed25519 release. */
export function applyVerifiedSkillSpectorCohort(
  report: SkillSpectorDisplayReport,
  manifestDigest: string | undefined,
  cohort: VerifiedCohort | undefined,
  now: Date = new Date(),
): SkillSpectorDisplayReport {
  // The signed base-manifest digest remains part of the release provenance.
  // Later v3 publications can change unrelated shards; don't invalidate the
  // signed source pins when the current report still matches each exact
  // repository/revision/path/content digest and resolved source/plugin states.
  if (!cohort || cohort.expiresAt <= now.getTime() ||
      !manifestDigest || !digestPattern.test(manifestDigest) ||
      report.schema_version !== 3) return report;
  const records = report.records.map(old => {
    const updated = cohort.records.get(old.id);
    // A valid older signature must never hide fresh recovery conflicts or
    // truncated evidence for that same immutable source. In those states the
    // canonical v3 Partial remains authoritative until a new qualification.
    if (!updated || old.status !== 'partial' || old.coverage?.status !== 'partial' ||
        old.findings_truncated || old.coverage.grouping_complete === false ||
        old.coverage.reference_evidence_truncated === true ||
        old.coverage.reasons?.some(reason => reason.code === 'normalization_truncated' ||
          reason.code.startsWith('recovery_')) ||
        old.freshness !== 'current' || old.target_revision !== old.source.revision ||
        old.scanner?.version !== '2.13.1' ||
        !same(old.source, updated.source) || old.plugin_path !== updated.plugin_path ||
        old.kind !== updated.kind) return old;
    const states = report.source_states.filter(state =>
      Array.isArray(state.selected_ids) && state.selected_ids.length === 1 &&
      state.selected_ids[0] === old.id);
    const state = states[0];
    const packageStates = report.plugin_states.filter(item =>
      String(item.repository).toLowerCase() === old.source.repository.toLowerCase() &&
      item.plugin_path === old.plugin_path);
    const packageState = packageStates[0];
    if (packageStates.length !== 1 || !packageState ||
        packageState.state !== 'resolved' ||
        packageState.revision !== old.source.revision ||
        !Array.isArray(packageState.selected_ids) ||
        packageState.selected_ids.length !== packageState.skills_total ||
        !packageState.selected_ids.includes(old.id)) return old;
    if (states.length !== 1 || !state || state.current !== true ||
        state.state !== 'resolved' || state.reason !== null ||
        state.revision !== old.source.revision ||
        state.path !== old.source.path ||
        String(state.repository).toLowerCase() !== old.source.repository.toLowerCase())
      return old;
    return {
      ...old, ...updated,
      freshness: old.freshness, target_revision: old.target_revision,
      retry: old.retry ? { ...old.retry, disposition: 'none' as const,
        next_profile: null, next_attempt_at: null, reason: 'complete' } : undefined,
      signed_cohort_verified: true as const,
      signed_cohort_expires_at: cohort.expiresAt,
    };
  });
  return records.every((record, i) => record === report.records[i])
    ? report : { ...report, records };
}
