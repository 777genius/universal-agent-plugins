import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { SkillSpectorDisplayReport, SkillSpectorRecord } from '../types/skillSpector.ts';
import { applyVerifiedSkillSpectorCohort, readVerifiedSkillSpectorCohort } from '../utils/skillSpectorCohort.ts';

const latest = 'https://raw.githubusercontent.com/777genius/universal-agent-plugins-registry/skillspector-cohort-releases/reports/skillspector/cohort/v1/latest.json';
const prefix = 'reports/skillspector/cohort/v1/';
const at = new Date('2026-11-05T19:00:00Z');
const scanner = { id: 'skillspector', version: '2.13.3', revision: '604a1cff1249ec4023012be4bdb77eac20621893' };
const profile = { id: 'static-cohort-2.13.3-v1', digest: 'sha256:ad4477fa9f9d61952c5fa929fc6810b0792433a1494102308a1e8e183ba6fe74' };
const sources = [
  ['3b7213be12f01b1365846d8ba375d59f', 'sitemap-manager', 'indranilbanerjee/digital-marketing-pro', 'cc45ca06eaf48abde3bb60fb79d31c41229c58db'],
  ['4d73239dc1cf321957d109bdf29c27dd', 'sitemap-manager', 'teachskillofskills-ai/digitalmarketingpro-techshu', 'e97a5bdd91601024a9cfa870807a8d3c81dc3caf'],
  ...['braids', 'braids-audit', 'braids-claims', 'braids-depth', 'braids-help', 'braids-review', 'braids-risk']
    .map(name => ['b0fa2f8fff3c4570b38ffe3e0c1401a7', name,
      'vyas-devgna/braids', '79c913e9d27854d88c7ee58aa9906bd2b45001b4']),
];
const encode = new TextEncoder();
const hash = async (bytes: Uint8Array) =>
  'sha256:' + [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes as BufferSource))]
    .map(b => b.toString(16).padStart(2, '0')).join('');
const stable = (value: unknown): string => {
  if (Array.isArray(value)) return '[' + value.map(stable).join(',') + ']';
  if (value !== null && typeof value === 'object') {
    const data = value as Record<string, unknown>;
    return '{' + Object.keys(data).sort().map(k => JSON.stringify(k) + ':' + stable(data[k])).join(',') + '}';
  }
  const json = JSON.stringify(value);
  assert.equal(typeof json, 'string');
  return json;
};
const canonical = (value: unknown) => encode.encode(stable(value) + '\n');
const b64 = (v: Uint8Array) => btoa(String.fromCharCode(...v));

async function fixture(options: { rejectedReference?: boolean; sitemapRevision?: string | string[] } = {}) {
  const pair = await crypto.subtle.generateKey({ name: 'Ed25519' }, true, ['sign', 'verify']);
  const trusted = [{ key_id: 'cohort-test', public_key: b64(new Uint8Array(
    await crypto.subtle.exportKey('raw', pair.publicKey))) }];
  const records = sources.map(([pack, name, repository, revision], index) => {
    const findings = index === 2 ? [{ rule: 'E4', severity: 'HIGH', confidence: 0.6, message: 'Risk retained',
      path: 'SKILL.md', occurrences: [{ path: 'SKILL.md', line: 7 }] }] : [];
    return {
      id: 'plugin:community:' + pack + '/' + name, kind: 'plugin_skill',
      plugin_path: '', skill_name: name,
      source: { repository, revision: index === 0 ? (options.sitemapRevision ?? revision) : revision,
        path: 'skills/' + name,
        content_digest: 'sha256:' + 'a'.repeat(64) },
      scanner, profile, status: 'complete', findings,
      finding_count: findings.length, finding_group_count: findings.length,
      raw_finding_count: findings.length, findings_truncated: false, errors: [],
      execution: { state: 'completed', exit_code: findings.length ? 1 : 0,
        acquisition_seconds: 0.266917, analysis_seconds: 2.467785 },
      coverage: {
        status: 'complete', fully_inspected_files: 1,
        partially_inspected_files: 0, entirely_uninspected_files: 0,
        grouping_complete: true, reference_evidence_truncated: false,
        references: index === 2 && options.rejectedReference
          ? [{ status: 'rejected', disposition: 'ignored', source_path: 'SKILL.md',
               target_path: '../outside.sh', line: 1, column: 1, evidence: '../outside.sh',
               reference_kind: 'markdown_link' }]
          : [],
        references_total: index === 2 && options.rejectedReference ? 1 : 0,
        ledger_exceptions: [],
        ledger_exceptions_total: 0, reasons: [],
      },
    };
  });
  const candidate = {
    schema_version: 1, kind: 'skillspector-2.13.3-nine-source-candidate',
    candidate_status: 'measured_complete_unpublished', publication_authorized: false,
    live_selection_verified: false, base_report_commit: 'c'.repeat(40),
    base_manifest_digest: 'sha256:' + 'd'.repeat(64), scanner, profile,
    profile_policy_digest: profile.digest, records,
    comparisons: records.map(record => ({
      id: record.id, source_digest: record.source.content_digest,
      candidate_exit_code: record.execution.exit_code,
      candidate_findings: record.finding_count, old_findings: 0, findings_changed: false,
    })),
  };
  const release = {
    schema_version: 1, kind: 'skillspector-cohort-release-v1',
    source_commit: 'e'.repeat(40), generated_at: '2026-11-05T18:00:00Z',
    expires_at: '2026-11-06T18:00:00Z', candidate,
    signed_sources: {
      schema_version: 1,
      community: { sequence: 10, source_commit: 'a'.repeat(40), snapshot_digest: 'sha256:' + 'e'.repeat(64) },
      directory: { sequence: 15, source_commit: 'b'.repeat(40), snapshot_digest: 'sha256:' + 'f'.repeat(64) },
      kind: 'skillspector-signed-community-cohort-preflight',
      source_pins_verified_against_fresh_signed_feeds: true,
      publication_authorized: false, candidate_scan_verified: false, provider_listing_verified: false,
      base_report_commit: candidate.base_report_commit,
      base_manifest_digest: candidate.base_manifest_digest,
      sources: records.map(record => ({
        id: record.id, ...record.source, plugin_path: '',
        signed_package_id: record.id.split('/')[0]!.slice('plugin:'.length),
        signed_package_tree_digest: 'sha256:' + 'b'.repeat(64),
        signed_package_manifest_digest: 'sha256:' + 'c'.repeat(64),
      })),
    },
  };
  const body = canonical(release);
  const domain = encode.encode('UAP-SKILLSPECTOR-COHORT-RELEASE-ED25519-V1\0');
  const signed = new Uint8Array(domain.length + 8 + body.length);
  signed.set(domain);
  new DataView(signed.buffer).setBigUint64(domain.length, BigInt(body.length), false);
  signed.set(body, domain.length + 8);
  const signature = new Uint8Array(await crypto.subtle.sign('Ed25519', pair.privateKey, signed as BufferSource));
  const sealed = canonical({
    schema_version: 1, kind: 'skillspector-cohort-release-envelope-v1',
    algorithm: 'Ed25519', signature_domain: 'UAP-SKILLSPECTOR-COHORT-RELEASE-ED25519-V1',
    snapshot_digest: await hash(body), key_id: 'cohort-test', signature: b64(signature),
  });
  const rd = (await hash(body)).slice(7), ed = (await hash(sealed)).slice(7);
  const pointer = canonical({
    schema_version: 1, kind: 'skillspector-cohort-latest-v1', sequence: 1,
    source_commit: release.source_commit, report_commit: candidate.base_report_commit,
    generated_at: release.generated_at, expires_at: release.expires_at,
    release_digest: 'sha256:' + rd, envelope_digest: 'sha256:' + ed,
    release_path: prefix + 'releases/' + rd + '.json',
    envelope_path: prefix + 'envelopes/' + ed + '.json',
  });
  const map = new Map([
    [latest, pointer], [latest.replace('latest.json', 'releases/' + rd + '.json'), body],
    [latest.replace('latest.json', 'envelopes/' + ed + '.json'), sealed],
  ]);
  const fetcher = (async (url: string | URL | Request) => {
    const payload = map.get(String(url));
    return payload ? new Response(payload as BodyInit, { status: 200 }) : new Response(null, { status: 404 });
  }) as typeof fetch;
  return { trusted, map, fetcher, records };
}

test('real Ed25519 verification requires nine pinned sources and preserves risk', async () => {
  const f = await fixture();
  const result = await readVerifiedSkillSpectorCohort(latest, f.trusted, f.fetcher, at);
  assert.ok(result);
  assert.equal(result.records.size, 9);
  const risk = result.records.get(f.records[2]!.id);
  assert.equal(risk?.status, 'complete');
  assert.equal(risk?.execution?.exit_code, 1);
  assert.equal(risk?.finding_count, 1);
  assert.equal(risk?.findings[0]?.severity, 'HIGH');
  assert.equal(risk?.findings[0]?.confidence, 0.6);
  assert.equal(risk?.execution?.acquisition_seconds, 0.266917);
});

test('reviewed sitemap revision rotation preserves signed evidence and rejects unknown SHAs', async () => {
  for (const revision of [
    'cc45ca06eaf48abde3bb60fb79d31c41229c58db',
    'be4142b25bba6d424962a7788432dd813e6b768c',
  ]) {
    const f = await fixture({ sitemapRevision: revision });
    const cohort = await readVerifiedSkillSpectorCohort(latest, f.trusted, f.fetcher, at);
    assert.equal(cohort?.records.get(f.records[0]!.id)?.source.revision, revision);
    assert.equal(cohort?.records.get(f.records[2]!.id)?.finding_count, 1);
  }
  for (const unreviewedRevision of ['0'.repeat(40), ['be4142b25bba6d424962a7788432dd813e6b768c']]) {
    const unreviewed = await fixture({ sitemapRevision: unreviewedRevision });
    await assert.rejects(() => readVerifiedSkillSpectorCohort(
      latest, unreviewed.trusted, unreviewed.fetcher, at,
    ));
  }
});

test('unregistered key never fetches; signature tampering and expiration reject', async () => {
  let called = false;
  const never = (async () => { called = true; throw new Error('must not fetch'); }) as typeof fetch;
  assert.equal(await readVerifiedSkillSpectorCohort(latest, [], never, at), undefined);
  assert.equal(called, false);
  const f = await fixture();
  const releasePath = [...f.map.keys()].find(k => k.includes('/releases/'))!;
  f.map.set(releasePath, encode.encode('{}\n'));
  await assert.rejects(() => readVerifiedSkillSpectorCohort(latest, f.trusted, f.fetcher, at));
  const good = await fixture();
  await assert.rejects(() => readVerifiedSkillSpectorCohort(
    latest, [{ key_id: good.trusted[0]!.key_id, public_key: b64(new Uint8Array(32)) }], good.fetcher, at));
  await assert.rejects(() => readVerifiedSkillSpectorCohort(latest, good.trusted, good.fetcher,
    new Date('2026-11-07T00:00:00Z')));
  await assert.rejects(() => readVerifiedSkillSpectorCohort('https://attacker.invalid/latest.json',
    good.trusted, good.fetcher, at));
});

test('even a valid Ed25519 signature cannot admit rejected or uninspected references', async () => {
  const signedRejected = await fixture({ rejectedReference: true });
  await assert.rejects(() => readVerifiedSkillSpectorCohort(
    latest, signedRejected.trusted, signedRejected.fetcher, at,
  ));
});

test('all seven Braids Skills sharing one signed package receive individual exact source states', async () => {
  const f = await fixture();
  const cohort = await readVerifiedSkillSpectorCohort(latest, f.trusted, f.fetcher, at);
  assert.ok(cohort);
  const oldRecords = f.records.map((source): SkillSpectorRecord => {
    const record = structuredClone(source) as unknown as SkillSpectorRecord;
    record.status = 'partial';
    record.coverage!.status = 'partial';
    record.coverage!.reasons = [{ code: 'reference_missing', message: 'Metadata is not a file.' }];
    record.errors = [{ code: 'reference_missing', message: 'Metadata is not a file.' }];
    record.scanner = { id: 'skillspector', version: '2.13.1',
      revision: 'f3b25697d76256998ca9d42c198d256258d4c035' };
    record.freshness = 'current';
    record.target_revision = record.source.revision;
    return record;
  });
  // Real 2.13.1 and signed 2.13.3 use different generated IDs for
  // identical LP3 source coordinates. That is not changed risk evidence.
  const oldLp3 = oldRecords.find(record => record.id.endsWith('/braids'))!
    .findings.find(finding => finding.rule === 'LP3')!;
  assert.equal(oldLp3.match_fingerprint, null);
  oldLp3.occurrences![0]!.finding_id = 'finding-different-scanner-generated-id';
  const report = {
    schema_version: 3,
    records: oldRecords,
    source_states: oldRecords.map(record => ({
      id: record.id, repository: record.source.repository,
      revision: record.source.revision, path: record.source.path,
      state: 'resolved', current: true, reason: null,
      selected_ids: [record.id], candidates: [],
    })),
    plugin_states: [...new Set(oldRecords.map(record => record.source.repository))].map(repository => {
      const members = oldRecords.filter(record => record.source.repository === repository);
      return { id: 'community:' + repository, repository, plugin_path: '',
        revision: members[0]!.source.revision, state: 'resolved', reason: null,
        skills_total: members.length, selected_ids: members.map(record => record.id) };
    }),
  } as unknown as Extract<SkillSpectorDisplayReport, { schema_version: 3 }>;
  const updated = applyVerifiedSkillSpectorCohort(report, cohort.manifestDigest, cohort, at);
  assert.equal(updated.records.length, 9);
  assert.equal(updated.records.filter(x => x.signed_cohort_verified).length, 9);
  assert.equal(updated.records.filter(x => x.status === 'complete').length, 9);
  assert.equal(updated.records.filter(x => x.source.repository === 'vyas-devgna/braids').length, 7);
  assert.equal(updated.records.find(x => x.source.path === 'skills/braids')?.finding_count, 1);
  assert.ok(oldRecords.every(x => x.status === 'partial'));
});

test('overlay survives unrelated manifest updates but fails closed on source or selection drift', async () => {
  const f = await fixture();
  const cohort = await readVerifiedSkillSpectorCohort(latest, f.trusted, f.fetcher, at);
  assert.ok(cohort);
  const old = structuredClone(f.records[2]!) as unknown as SkillSpectorRecord;
  old.status = 'partial';
  old.coverage!.status = 'partial';
  old.coverage!.reasons = [{ code: 'reference_missing', message: 'Metadata is not a file.' }];
  old.errors = [{ code: 'reference_missing', message: 'Metadata is not a file.' }];
  old.scanner = { id: 'skillspector', version: '2.13.1',
    revision: 'f3b25697d76256998ca9d42c198d256258d4c035' };
  old.freshness = 'current';
  old.target_revision = old.source.revision;
  old.retry = { attempt_count: 1, reason: 'reference_missing', disposition: 'blocked',
    next_attempt_at: null, next_profile: null };
  const neighbor = structuredClone(old);
  neighbor.id = 'unrelated';
  neighbor.source.path = 'skills/unrelated';
  const report = {
    schema_version: 3, records: [old, neighbor],
    source_states: [{ id: old.id, repository: old.source.repository,
      revision: old.source.revision, path: old.source.path,
      current: true, state: 'resolved', reason: null,
      selected_ids: [old.id], candidates: [] }],
    plugin_states: [{
      id: 'community:fixture', repository: old.source.repository, plugin_path: '',
      revision: old.source.revision, state: 'resolved', reason: null,
      skills_total: 1, selected_ids: [old.id],
    }],
  } as unknown as Extract<SkillSpectorDisplayReport, { schema_version: 3 }>;
  const improved = applyVerifiedSkillSpectorCohort(report, cohort.manifestDigest, cohort, at);
  assert.equal(improved.records[0]!.status, 'complete');
  assert.equal(improved.records[0]!.signed_cohort_verified, true);
  assert.equal(improved.records[0]!.signed_cohort_expires_at, cohort.expiresAt);
  assert.equal(improved.records[0]!.retry?.disposition, 'none');
  assert.equal(improved.records[0]!.finding_count, 1);
  assert.equal(improved.records[1]!.status, 'partial');
  assert.equal(applyVerifiedSkillSpectorCohort(
    report, cohort.manifestDigest, cohort, new Date('2026-11-07T00:00:00Z')), report);
  // An unrelated publisher scan changes the global v3 manifest digest,
  // not the nine signed source content digests.
  const republished = applyVerifiedSkillSpectorCohort(
    report, 'sha256:' + 'f'.repeat(64), cohort, at);
  assert.equal(republished.records[0]!.signed_cohort_verified, true);
  assert.equal(republished.records[0]!.finding_count, 1);
  assert.equal(republished.records[1], report.records[1]);
  assert.equal(applyVerifiedSkillSpectorCohort(report, undefined, cohort, at), report);
  const stale = structuredClone(report);
  stale.source_states[0]!.current = false;
  assert.equal(applyVerifiedSkillSpectorCohort(stale, cohort.manifestDigest, cohort, at), stale);
  const unresolved = structuredClone(report);
  unresolved.plugin_states[0]!.state = 'unresolved';
  assert.equal(applyVerifiedSkillSpectorCohort(unresolved, cohort.manifestDigest, cohort, at), unresolved);
  const omitted = structuredClone(report);
  omitted.plugin_states[0]!.selected_ids = [];
  assert.equal(applyVerifiedSkillSpectorCohort(omitted, cohort.manifestDigest, cohort, at), omitted);
  // A still-valid older Ed25519 release cannot cover new, source-identical
  // authenticated recovery conflicts. Preserve the canonical Partial and all
  // original risk evidence until a new full scan and signed requalification.
  const quarantines: Array<(record: SkillSpectorRecord) => void> = [
    record => { record.coverage!.reasons = [
      { code: 'recovery_conflict', message: 'Independent source evidence differs.' },
    ]; },
    record => { record.coverage!.reasons = [
      { code: 'recovery_predecessor_unavailable', message: 'Parent evidence is missing.' },
    ]; },
    record => { record.coverage!.reasons = [
      { code: 'normalization_truncated', message: 'Incompletely represented risk evidence.' },
    ]; },
    record => { record.coverage!.reasons.push(
      { code: 'static_parse_limit', message: 'New unrelated analysis limitation.' },
    ); },
    record => { record.coverage!.reasons = []; },
    record => { record.errors.push(
      { code: 'acquisition_failed', message: 'Unrelated source acquisition error.' },
    ); },
    record => { record.errors = []; },
    record => { record.findings_truncated = true; },
    record => { record.coverage!.grouping_complete = false; },
    record => { delete record.coverage!.grouping_complete; },
    record => { record.coverage!.reference_evidence_truncated = true; },
    record => { delete record.coverage!.reference_evidence_truncated; },
  ];
  for (const makeUncertain of quarantines) {
    const uncertain = structuredClone(report);
    makeUncertain(uncertain.records[0]!);
    assert.equal(applyVerifiedSkillSpectorCohort(
      uncertain, cohort.manifestDigest, cohort, at), uncertain);
    assert.equal(uncertain.records[0]!.status, 'partial');
    assert.equal(uncertain.records[0]!.finding_count, 1);
    assert.equal(uncertain.records[0]!.signed_cohort_verified, undefined);
  }
  // An older signed Complete cannot discard new risk evidence from a newer
  // source-identical Partial even when only reference_missing remains.
  const newlyObserved = structuredClone(report);
  newlyObserved.records[0]!.findings.push({
    ...structuredClone(newlyObserved.records[0]!.findings[0]!),
    rule: 'E99', severity: 'CRITICAL',
    message: 'New source-identical risk', match_fingerprint: 'new-current-risk',
  });
  newlyObserved.records[0]!.finding_count = 2;
  assert.equal(applyVerifiedSkillSpectorCohort(
    newlyObserved, cohort.manifestDigest, cohort, at), newlyObserved);
  assert.equal(newlyObserved.records[0]!.findings[1]?.severity, 'CRITICAL');
  const changedRisk = structuredClone(report);
  changedRisk.records[0]!.findings[0]!.message = 'Different risk evidence';
  assert.equal(applyVerifiedSkillSpectorCohort(
    changedRisk, cohort.manifestDigest, cohort, at), changedRisk);
  // A changed physical occurrence is a new identity even if an ephemeral
  // finding_id remains the same. The valid older signature may not erase it.
  const movedRisk = structuredClone(report);
  const fingerprintFree = movedRisk.records[0]!.findings.find(f => !f.match_fingerprint)!;
  assert.ok(fingerprintFree?.occurrences?.length);
  fingerprintFree.occurrences![0]!.line = 999;
  assert.equal(applyVerifiedSkillSpectorCohort(
    movedRisk, cohort.manifestDigest, cohort, at), movedRisk);
  const repeatedRisk = structuredClone(report);
  repeatedRisk.records[0]!.findings.push(
    structuredClone(repeatedRisk.records[0]!.findings[0]!));
  repeatedRisk.records[0]!.finding_count = 2;
  assert.equal(applyVerifiedSkillSpectorCohort(
    repeatedRisk, cohort.manifestDigest, cohort, at), repeatedRisk);
  const changedRevision = structuredClone(report);
  changedRevision.records[0]!.source.revision = 'be4142b25bba6d424962a7788432dd813e6b768c';
  changedRevision.records[0]!.target_revision = changedRevision.records[0]!.source.revision;
  changedRevision.source_states[0]!.revision = changedRevision.records[0]!.source.revision;
  changedRevision.plugin_states[0]!.revision = changedRevision.records[0]!.source.revision;
  assert.equal(applyVerifiedSkillSpectorCohort(changedRevision, cohort.manifestDigest, cohort, at), changedRevision);
  const changed = structuredClone(report);
  changed.records[0]!.source.content_digest = 'sha256:' + '0'.repeat(64);
  assert.equal(applyVerifiedSkillSpectorCohort(changed, cohort.manifestDigest, cohort, at), changed);
});
