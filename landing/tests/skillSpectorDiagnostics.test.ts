import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';
import type {
  SkillSpectorDisplayReport,
  SkillSpectorManifestV3,
  SkillSpectorSubject,
} from '../types/skillSpector.ts';
import { skillSpectorAssessment } from '../utils/skillSpector.ts';
import { serializeSkillSpectorDiagnostics } from '../utils/skillSpectorDiagnostics.ts';

const root = new URL('./fixtures/skillspector-v3/', import.meta.url);
const manifest = JSON.parse(readFileSync(new URL('index.json', root), 'utf8')) as SkillSpectorManifestV3;
const shard = JSON.parse(readFileSync(new URL(manifest.shards[0]!.path, root), 'utf8'));
const sample = shard.records[0];
const subject: SkillSpectorSubject = {
  kind: 'plugin',
  repository: sample.source.repository,
  path: sample.plugin_path,
  revision: sample.source.revision,
};

describe('SkillSpector AI diagnostic export', () => {
  it('keeps all original published evidence for the chosen listing without unrelated records', () => {
    const foreign = {
      ...structuredClone(sample),
      id: sample.id + '-unrelated',
      plugin_path: 'plugins/unrelated',
    };
    const report = {
      ...structuredClone(manifest),
      records: [...structuredClone(shard.records), foreign],
      plugin_states: [
        ...structuredClone(manifest.plugin_states),
        {
          id: 'unrelated',
          repository: subject.repository,
          plugin_path: 'plugins/unrelated',
          revision: sample.source.revision,
          state: 'resolved',
          skills_total: 1,
          selected_ids: [foreign.id],
        },
      ],
    } as SkillSpectorDisplayReport;
    const assessment = skillSpectorAssessment(report, subject);
    const exported = JSON.parse(serializeSkillSpectorDiagnostics({
      name: 'Cloudflare',
      scope: 'plugin',
      subject,
      report,
      reportStatus: 'ready',
      displayStatus: assessment.status,
      assessment,
    }));
    assert.equal(exported.format, 'uap.skillspector.diagnostics.v1');
    assert.equal(exported.listing.source.path, subject.path);
    assert.equal(exported.publication.schema_version, 3);
    assert.deepEqual(exported.records, assessment.records);
    assert.deepEqual(exported.assessment, JSON.parse(JSON.stringify(assessment)));
    assert.ok(exported.records.length > 0);
    assert.ok(exported.records.every((record: { plugin_path: string }) =>
      record.plugin_path === subject.path));
    assert.ok(exported.source_states.every((state: { plugin_path: string }) =>
      state.plugin_path === subject.path));
    assert.ok(!JSON.stringify(exported).includes('plugins/unrelated'));
  });

  it('does not invent a scan, finding, source or timestamp when a report is unavailable', () => {
    const assessment = skillSpectorAssessment(undefined, subject);
    const exported = JSON.parse(serializeSkillSpectorDiagnostics({
      name: 'Unknown scan',
      scope: 'plugin',
      subject,
      assessment,
      reportStatus: 'unavailable',
      displayStatus: 'unavailable',
    }));
    assert.equal(exported.publication, null);
    assert.deepEqual(exported.records, []);
    assert.deepEqual(exported.source_states, []);
    assert.equal(exported.listing.report_load_status, 'unavailable');
    assert.equal(exported.listing.displayed_status, 'unavailable');
    assert.ok(exported.guidance.includes('not a completed current scan'));
  });

  it('identifies unsupported external Skills without querying an unrelated repository', () => {
    const exported = JSON.parse(serializeSkillSpectorDiagnostics({
      name: 'Unverified external skill',
      scope: 'skill',
      assessment: skillSpectorAssessment(undefined, undefined),
      reportStatus: 'ready',
      displayStatus: 'unsupported',
    }));
    assert.equal(exported.listing.source, null);
    assert.equal(exported.listing.displayed_status, 'unsupported');
    assert.deepEqual(exported.records, []);
    assert.equal(exported.publication, null);
  });
});
