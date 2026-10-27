import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { readFileSync } from 'node:fs';
import { publishedLocales } from '../data/i18n.ts';
import { loadI18nMessages } from '../scripts/load-i18n-messages.ts';
import type { SkillSpectorRecord, SkillSpectorReport } from '../types/skillSpector.ts';
import {
  decodeSkillSpectorReport,
  loadSkillSpectorReport,
  skillSpectorAssessment,
  skillSpectorFindingUrl,
  skillSpectorSourceUrl,
} from '../utils/skillSpector.ts';

const now = new Date('2026-10-27T12:00:00Z');
const digest = `sha256:${'1'.repeat(64)}`;
const revision = 'a'.repeat(40);
const record = (overrides: Partial<SkillSpectorRecord> = {}): SkillSpectorRecord => ({
  id: 'example',
  kind: 'plugin_skill',
  plugin_path: 'plugins/example',
  skill_name: 'example',
  source: {
    repository: 'owner/repository',
    revision,
    path: 'plugins/example/skills/example',
    content_digest: digest,
  },
  scanned_at: '2026-10-27T10:00:00Z',
  status: 'complete',
  finding_count: 0,
  findings_truncated: false,
  findings: [],
  errors: [],
  report_digest: digest,
  ...overrides,
});
const report = (records = [record()]): SkillSpectorReport => ({
  schema_version: 1,
  generated_at: '2026-10-27T11:00:00Z',
  source_commit: revision,
  scope: 'selected_sources',
  scanner: {
    id: 'skillspector',
    version: '2.13.1',
    revision: 'f3b25697d76256998ca9d42c198d256258d4c035',
  },
  profile: {
    id: 'static-only-v1',
    digest: 'sha256:ddd91212d95a717ab6c067d99e9f32a64b8dd55589bdf47a36695a85320187d5',
  },
  selection_digest: digest,
  records,
});
const subject = {
  kind: 'plugin',
  repository: 'owner/repository',
  path: 'plugins/example',
  revision,
} as const;

describe('SkillSpector historical display reports', () => {
  it('keeps assessment translations in component-owned resources outside global page payloads', () => {
    const component = readFileSync(
      new URL('../components/registry/SkillSpectorAssessment.vue', import.meta.url),
      'utf8',
    );
    for (const locale of publishedLocales) {
      const base = JSON.parse(
        readFileSync(new URL(`../locales/${locale}.json`, import.meta.url), 'utf8'),
      );
      assert.equal(base.registryUi.skillSpector, undefined);
      const resource = JSON.parse(
        readFileSync(
          new URL(`../features/skillspector/locales/${locale}.json`, import.meta.url),
          'utf8',
        ),
      );
      assert.deepEqual(
        loadI18nMessages(locale).registryUi.skillSpector,
        resource.registryUi.skillSpector,
      );
      assert.ok(
        component.includes(
          `locale="${locale}" lang="json" src="../../features/skillspector/locales/${locale}.json"`,
        ),
      );
    }
    assert.ok(component.includes("useScope: 'local'"));
  });

  it('accepts the normalized output of real pinned scanner runs and preserves partial findings', () => {
    const value = JSON.parse(
      readFileSync(new URL('./fixtures/skillspector-report.json', import.meta.url), 'utf8'),
    );
    const decoded = decodeSkillSpectorReport(value, now);
    assert.deepEqual(
      decoded.records.map(({ status, finding_count }) => [status, finding_count]),
      [
        ['complete', 0],
        ['partial', 50],
      ],
    );
    assert.equal(decoded.records[1]!.findings.length, 32);
    assert.equal(decoded.records[1]!.findings_truncated, true);
  });

  it('decodes complete and failed records without interpreting failure as clean', () => {
    assert.equal(decodeSkillSpectorReport(report(), now).records[0]?.status, 'complete');
    const failed = record({
      kind: 'standalone_skill',
      plugin_path: null,
      skill_name: null,
      status: 'error',
      scanned_at: null,
      source: { ...record().source, content_digest: null },
      report_digest: undefined,
      errors: [{ code: 'acquisition_failed', message: 'Unavailable source' }],
    });
    assert.equal(decodeSkillSpectorReport(report([failed]), now).records[0]?.status, 'error');
    assert.equal(skillSpectorAssessment(report(), undefined, now).status, 'notChecked');
    assert.equal(skillSpectorAssessment(undefined, subject, now).status, 'notChecked');
  });

  it('rejects invalid identity, future timestamps, missing provenance and dishonest counts', () => {
    for (const mutate of [
      (value: SkillSpectorReport) => {
        value.scanner.version = '9.0.0';
      },
      (value: SkillSpectorReport) => {
        value.profile.id = 'llm' as never;
      },
      (value: SkillSpectorReport) => {
        value.profile.digest = digest;
      },
      (value: SkillSpectorReport) => {
        value.records[0]!.scanned_at = '2026-10-28T00:00:00Z';
      },
      (value: SkillSpectorReport) => {
        value.generated_at = '2026-02-30T00:00:00Z';
      },
      (value: SkillSpectorReport) => {
        value.records[0]!.report_digest = undefined;
      },
      (value: SkillSpectorReport) => {
        value.records[0]!.source.content_digest = null;
      },
      (value: SkillSpectorReport) => {
        value.records[0]!.finding_count = 1;
      },
      (value: SkillSpectorReport) => {
        value.records[0]!.errors = [{ code: 'failed', message: 'Failed check' }];
      },
      (value: SkillSpectorReport) => {
        value.records.push(value.records[0]!);
      },
    ]) {
      const value = report();
      mutate(value);
      assert.throws(() => decodeSkillSpectorReport(value, now), /invalid/);
    }
  });

  it('uses exact plugin source identity and leaves older or different revisions neutral', () => {
    assert.equal(skillSpectorAssessment(report(), subject, now).status, 'noFindings');
    assert.equal(
      skillSpectorAssessment(report(), { ...subject, path: 'plugins/other' }, now).status,
      'notChecked',
    );
    assert.equal(
      skillSpectorAssessment(report(), { ...subject, repository: 'other/repository' }, now).status,
      'notChecked',
    );
    assert.equal(
      skillSpectorAssessment(report(), { ...subject, revision: 'b'.repeat(40) }, now).status,
      'historical',
    );
    assert.equal(
      skillSpectorAssessment(report([record({ scanned_at: '2026-10-01T00:00:00Z' })]), subject, now)
        .status,
      'historical',
    );
  });

  it('aggregates selected children conservatively including a child with no usable result', () => {
    const failed = record({
      id: 'failed-child',
      status: 'error',
      scanned_at: null,
      errors: [{ code: 'scan_failed', message: 'Failed check' }],
    });
    const partial = record({ id: 'partial-child', status: 'partial' });
    assert.equal(
      skillSpectorAssessment(report([record(), failed]), subject, now).status,
      'partial',
    );
    assert.equal(
      skillSpectorAssessment(report([record(), partial]), subject, now).status,
      'partial',
    );
    assert.equal(skillSpectorAssessment(report([failed]), subject, now).status, 'error');
    const finding = record({
      finding_count: 1,
      findings: [
        {
          rule: 'STATIC001',
          severity: 'HIGH',
          message: 'Review pattern',
          path: 'SKILL.md',
          line: 2,
        },
      ],
    });
    assert.equal(skillSpectorAssessment(report([finding]), subject, now).status, 'findingsCount');
  });

  it('links findings only to in-root artifacts at the exact scanned revision', () => {
    const finding = {
      rule: 'AE1',
      severity: 'HIGH',
      message: 'Review',
      path: 'SKILL.md',
      line: 50,
    };
    const base = `https://github.com/owner/repository/blob/${revision}/plugins/example/skills/example/`;
    assert.equal(skillSpectorFindingUrl(record(), finding), `${base}SKILL.md#L50`);
    assert.equal(
      skillSpectorFindingUrl(record(), {
        ...finding,
        path: 'references/example file#1.md',
        line: undefined,
      }),
      `${base}references/example%20file%231.md`,
    );
    for (const path of [
      '',
      '/etc/passwd',
      '//evil.example/file',
      '../outside.md',
      'refs/../../outside',
      'refs/./file',
      'refs//file',
      'C:/file',
      'C:\\file',
      'https://evil.example/file',
      '%2e%2e/file',
      'file\u0000.md',
    ]) {
      assert.equal(skillSpectorFindingUrl(record(), { ...finding, path }), undefined, path);
    }
    assert.equal(skillSpectorFindingUrl(record(), { ...finding, line: 0 }), undefined);
    assert.equal(
      skillSpectorFindingUrl(record({ source: { ...record().source, revision: 'main' } }), finding),
      undefined,
    );
  });

  it('keeps revision and age warnings independent from partial or failed checks', () => {
    for (const status of ['partial', 'error'] as const) {
      for (const stale of [
        record({ status, scanned_at: '2026-10-01T00:00:00Z' }),
        record({ status, source: { ...record().source, revision: 'b'.repeat(40) } }),
      ]) {
        const assessment = skillSpectorAssessment(report([stale]), subject, now);
        assert.equal(assessment.status, status);
        assert.equal(assessment.outOfDate, true);
      }
      assert.equal(
        skillSpectorAssessment(report([record({ status })]), subject, now).outOfDate,
        false,
      );
    }
    assert.equal(skillSpectorAssessment(undefined, subject, now).outOfDate, false);
  });

  it('joins standalone historical evidence by repository and declared name, never the provider title', () => {
    const skill = record({
      kind: 'standalone_skill',
      plugin_path: null,
      skill_name: 'Declared Name',
    });
    const wanted = {
      kind: 'skill',
      repository: 'OWNER/repository',
      name: 'Declared Name',
    } as const;
    assert.equal(skillSpectorAssessment(report([skill]), wanted, now).records.length, 1);
    assert.equal(
      skillSpectorAssessment(report([skill]), { ...wanted, name: 'declared-name' }, now).status,
      'notChecked',
    );
    assert.equal(
      skillSpectorSourceUrl(skill),
      `https://github.com/owner/repository/tree/${revision}/plugins/example/skills/example`,
    );
  });

  it('cancels a streamed response at its size bound even without Content-Length', async () => {
    let cancelled = false;
    const body = new ReadableStream({
      pull(controller) {
        controller.enqueue(new Uint8Array((8 << 20) + 1));
      },
      cancel() {
        cancelled = true;
      },
    });
    const fetcher = (async () => new Response(body)) as typeof fetch;
    await assert.rejects(
      loadSkillSpectorReport('https://example.test/report.json', fetcher),
      /invalid/,
    );
    assert.equal(cancelled, true);
  });
});
