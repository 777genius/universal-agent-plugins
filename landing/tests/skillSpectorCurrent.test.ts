import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import type { SkillListing } from '../types/skillsCatalog.ts';
import type { SkillSpectorDisplayReport, SkillSpectorRecord, SkillSpectorManifestV3 } from '../types/skillSpector.ts';
import { hasCurrentPublishedScan, publishedScanSubject } from '../utils/skillSpectorCurrent.ts';

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
  const variants: Array<(report: ReturnType<typeof currentReport>) => void> = [
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
