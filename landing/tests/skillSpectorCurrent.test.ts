import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import type { SkillListing } from '../types/skillsCatalog.ts';
import type { SkillSpectorDisplayReport, SkillSpectorRecord, SkillSpectorManifestV3 } from '../types/skillSpector.ts';
import { hasCurrentPublishedScan, hasNoPublishedRemarks, publishedScanSubject } from '../utils/skillSpectorCurrent.ts';

const manifest = JSON.parse(readFileSync(new URL('./fixtures/skillspector-v3/index.json', import.meta.url), 'utf8')) as SkillSpectorManifestV3;
const source = JSON.parse(readFileSync(new URL('./fixtures/skillspector-v3-complete-context.json', import.meta.url), 'utf8'));
const original = source.records.resolved as SkillSpectorRecord;
const listing: SkillListing = {
  id: 'skills-sh:affaan-m/ecc:recsys-pipeline-architect',
  providerId: 'skills-sh',
  providerUrl: 'https://skills.sh/affaan-m/ecc/recsys-pipeline-architect',
  title: 'recsys-pipeline-architect',
  skillNameHint: 'recsys-pipeline-architect',
  source: { kind: 'github', repository: 'affaan-m/ecc', url: 'https://github.com/affaan-m/ecc' },
  metrics: { installs: 100, window: 'all-time' },
  availability: 'unverified',
};
function currentReport(): SkillSpectorManifestV3 & { records: SkillSpectorRecord[] } {
  const record = structuredClone(original);
  return {
    ...structuredClone(manifest),
    source_states: [{
      id: 'affaan-m/ecc/recsys-pipeline-architect',
      repository: record.source.repository,
      state: 'resolved',
      current: true,
      reason: null,
      revision: record.source.revision,
      path: record.source.path,
      selected_ids: [record.id],
      candidates: [],
    }],
    records: [record],
  };
}
test('published evidence requires exact source, revision, path, completed execution and current policy', () => {
  const subject = publishedScanSubject(listing);
  assert.ok(subject);
  assert.equal(hasCurrentPublishedScan(currentReport(), subject), true);
  const partlyInspected = currentReport();
  partlyInspected.records[0]!.status = 'partial';
  partlyInspected.records[0]!.coverage!.status = 'partial';
  partlyInspected.records[0]!.coverage!.fully_inspected_files = 0;
  partlyInspected.records[0]!.coverage!.partially_inspected_files = 1;
  assert.equal(hasCurrentPublishedScan(partlyInspected, subject), true);
  const variants: Array<(report: ReturnType<typeof currentReport>) => void> = [
    report => {
      report.records[0]!.status = 'partial';
      report.records[0]!.coverage!.status = 'partial';
      report.records[0]!.coverage!.fully_inspected_files = 0;
      report.records[0]!.coverage!.partially_inspected_files = 0;
    },
    report => { report.records[0]!.freshness = 'historical'; },
    report => { report.records[0]!.execution!.state = 'failed'; },
    report => { report.records[0]!.source.path = 'skills/different'; },
    report => { report.records[0]!.source.revision = 'f'.repeat(40); },
    report => { report.records[0]!.target_revision = 'f'.repeat(40); },
    report => { report.records[0]!.coverage!.status = 'partial'; },
    report => { report.records[0]!.status = 'error'; },
    report => { report.source_states[0]!.selected_ids = ['another']; },
    report => { report.source_states[0]!.selected_ids = [original.id, 'another']; },
    report => { report.source_states[0]!.current = false; },
    report => { report.source_states[0]!.state = 'historical'; },
    report => { report.source_states[0]!.revision = 'f'.repeat(40); },
  ];
  for (const change of variants) {
    const report = currentReport();
    change(report);
    assert.equal(hasCurrentPublishedScan(report, subject), false);
  }
  const legacy = { ...currentReport(), schema_version: 2 };
  assert.equal(hasCurrentPublishedScan(legacy as unknown as SkillSpectorDisplayReport, subject), false);
});

test('unmatched and ambiguous provider source IDs never inherit unrelated scan evidence', () => {
  const valid = publishedScanSubject(listing);
  assert.deepEqual(valid, {
    repository: 'affaan-m/ecc',
    sourceStateId: 'affaan-m/ecc/recsys-pipeline-architect',
    name: 'recsys-pipeline-architect',
  });
  for (const changed of [
    { ...listing, id: 'skills-sh:other/repo:recsys-pipeline-architect' },
    { ...listing, id: 'skills-sh:affaan-m/ecc:different-skill' },
    { ...listing, id: 'skills-sh:affaan-m/ecc/tree/main/skills/recsys-pipeline-architect:recsys-pipeline-architect' },
    { ...listing, source: { kind: 'external' as const, url: 'https://example.com' } },
  ]) assert.equal(publishedScanSubject(changed), undefined);
});

function cleanReport() {
  const report = currentReport();
  const record = report.records[0]!;
  record.finding_count = 0;
  record.raw_finding_count = 0;
  record.finding_group_count = 0;
  record.findings = [];
  record.coverage!.original_findings = [];
  record.coverage!.grouping_complete = true;
  record.retry = {
    attempt_count: 1,
    disposition: 'none',
    reason: 'complete',
    next_attempt_at: null,
    next_profile: null,
  };
  return report;
}
test('no remarks requires complete current exact-root evidence and zero findings in every representation', () => {
  const subject = publishedScanSubject(listing)!;
  assert.equal(hasNoPublishedRemarks(cleanReport(), subject), true);
  const variants: Array<(report: ReturnType<typeof cleanReport>) => void> = [
    (r) => {
      r.records[0]!.status = 'partial';
      r.records[0]!.coverage!.status = 'partial';
    },
    (r) => {
      r.records[0]!.coverage!.fully_inspected_files = 0;
    },
    (r) => {
      r.records[0]!.coverage!.partially_inspected_files = 1;
    },
    (r) => {
      r.records[0]!.coverage!.entirely_uninspected_files = 1;
    },
    (r) => {
      r.records[0]!.coverage!.grouping_complete = false;
    },
    (r) => {
      delete r.records[0]!.coverage!.grouping_complete;
    },
    (r) => {
      r.records[0]!.coverage!.original_findings = [{ id: 'hidden' }];
    },
    (r) => {
      r.records[0]!.coverage!.reasons = [{ code: 'limit', message: 'Limited' }];
    },
    (r) => {
      r.records[0]!.coverage!.reference_evidence_truncated = true;
    },
    (r) => {
      r.records[0]!.coverage!.ledger_exceptions_total = 1;
    },
    (r) => {
      r.records[0]!.findings = [structuredClone(original.findings[0]!)];
    },
    (r) => {
      r.records[0]!.findings = [
        { ...structuredClone(original.findings[0]!), review_status: 'false_positive' },
      ];
    },
    (r) => {
      r.records[0]!.finding_count = 1;
    },
    (r) => {
      r.records[0]!.raw_finding_count = 1;
    },
    (r) => {
      delete r.records[0]!.raw_finding_count;
    },
    (r) => {
      r.records[0]!.finding_group_count = 1;
    },
    (r) => {
      r.records[0]!.findings_truncated = true;
    },
    (r) => {
      r.records[0]!.errors = [{ code: 'error', message: 'Error' }];
    },
    (r) => {
      r.records[0]!.source.content_digest = null;
    },
    (r) => {
      r.records[0]!.scanner!.revision = 'f'.repeat(40);
    },
    (r) => {
      r.records[0]!.profile!.digest = 'sha256:' + 'f'.repeat(64);
    },
    (r) => {
      r.records[0]!.retry!.disposition = 'scheduled';
    },
    (r) => {
      r.records[0]!.freshness = 'historical';
    },
    (r) => {
      r.records[0]!.execution!.exit_code = 1;
    },
    (r) => {
      r.records[0]!.execution!.state = 'failed';
    },
  ];
  variants.forEach((change, index) => {
    const report = cleanReport();
    change(report);
    assert.equal(hasNoPublishedRemarks(report, subject), false, 'variant ' + index);
  });
  const dirty = currentReport();
  const neighbor = cleanReport().records[0]!;
  neighbor.id = 'clean-neighbor';
  neighbor.skill_name = 'neighbor';
  neighbor.source.path = 'skills/neighbor';
  dirty.records.push(neighbor);
  assert.equal(hasCurrentPublishedScan(dirty, subject), true);
  assert.equal(hasNoPublishedRemarks(dirty, subject), false);
});
