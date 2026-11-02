import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';
import {
  decodeSkillSpectorManifestV3,
  decodeSkillSpectorShardV3,
} from '../utils/skillSpectorV3.ts';
import { createSkillSpectorClient } from '../utils/skillSpectorCatalog.ts';
import { skillSpectorAssessment } from '../utils/skillSpector.ts';
import type { SkillSpectorManifestV3, SkillSpectorRecord } from '../types/skillSpector.ts';
const root = new URL('./fixtures/skillspector-v3/', import.meta.url);
const manifest = JSON.parse(
  readFileSync(new URL('index.json', root), 'utf8'),
) as SkillSpectorManifestV3;
const descriptor = manifest.shards[0]!;
const body = readFileSync(new URL(descriptor.path, root), 'utf8');
const shard = JSON.parse(body);
const subject = {
  kind: 'plugin' as const,
  repository: descriptor.repository,
  path: 'plugins/cloudflare',
  revision: shard.records[0].source.revision,
};
const display = () => ({
  ...structuredClone(manifest),
  records: structuredClone(shard.records) as SkillSpectorRecord[],
});

const referenceFixture = JSON.parse(
  readFileSync(
    new URL('./fixtures/skillspector-v3-reference-context.json', import.meta.url),
    'utf8',
  ),
);
function referenceRecord(key = 'cloudflare') {
  const record = structuredClone(referenceFixture[key]);
  return {
    ...record,
    freshness: 'current',
    target_revision: record.source.revision,
    retry: {
      attempt_count: 1,
      disposition: 'scheduled',
      latest_status: record.status,
      next_attempt_at: null,
      next_profile: 'static-extended-v3',
      reason: 'extended_profile_required',
    },
    attempt_history: [],
  };
}
function decodeReference(record = referenceRecord()) {
  const part = { ...descriptor, repository: record.source.repository, record_count: 1 };
  return decodeSkillSpectorShardV3(
    { ...shard, repository: part.repository, records: [record] },
    manifest,
    part,
  )[0]!;
}
describe('SkillSpector v3 exact producer contract', () => {
  it('accepts a 17,715-root inventory without changing published evidence or per-plugin coverage', () => {
    const published = JSON.parse(
      readFileSync(
        new URL('./fixtures/skillspector-v3-published-home/index.json', import.meta.url),
        'utf8',
      ),
    ) as SkillSpectorManifestV3;
    const expanded = structuredClone(published);
    const addedRoots = 17_715 - expanded.coverage.selected;
    assert.ok(addedRoots > 0);
    expanded.coverage.selected += addedRoots;
    expanded.coverage.pending += addedRoots;
    const decoded = decodeSkillSpectorManifestV3(expanded);
    assert.equal(decoded.coverage.selected, 17_715);
    assert.equal(decoded.coverage.reported, published.coverage.reported);
    assert.deepEqual(decoded.shards, published.shards);
    assert.deepEqual(decoded.plugin_coverage, published.plugin_coverage);
    assert.deepEqual(decoded.source_states, published.source_states);
  });

  it('bounds v3 global and plugin coverage at 20,000 roots while preserving arithmetic', () => {
    const bounded = structuredClone(manifest);
    const addedRoots = 20_000 - bounded.coverage.selected;
    bounded.coverage.selected += addedRoots;
    bounded.coverage.pending += addedRoots;
    bounded.plugin_coverage[0]!.selected += addedRoots;
    bounded.plugin_coverage[0]!.pending += addedRoots;
    assert.equal(decodeSkillSpectorManifestV3(bounded).coverage.selected, 20_000);
    assert.equal(decodeSkillSpectorManifestV3(bounded).plugin_coverage[0]!.selected, 20_000);
    const over = structuredClone(bounded);
    over.coverage.selected++;
    over.coverage.pending++;
    assert.throws(() => decodeSkillSpectorManifestV3(over), /invalid/);
    const inconsistent = structuredClone(bounded);
    inconsistent.coverage.pending--;
    assert.throws(() => decodeSkillSpectorManifestV3(inconsistent), /invalid/);
  });

  it('allows 10,000 listing observations plus 20,000 inventory states but rejects a 30,001st state', () => {
    const combined = structuredClone(manifest);
    const seed = combined.source_states[0]!;
    combined.source_states = Array.from({ length: 30_000 }, (_, index) => ({
      ...seed,
      id: `inventory-or-listing-${index}`,
    }));
    assert.equal(decodeSkillSpectorManifestV3(combined).source_states.length, 30_000);
    combined.source_states.push({ ...seed, id: 'over-source-state-budget' });
    assert.throws(() => decodeSkillSpectorManifestV3(combined), /invalid/);
  });

  it('accepts 10,000 community package states plus Directory states and bounds v3 packages at 20,000', () => {
    const packages = structuredClone(manifest);
    packages.coverage = {
      selected: 0,
      reported: 0,
      pending: 0,
      complete: 0,
      partial: 0,
      error: 0,
      stale: 0,
    };
    packages.shards = [];
    packages.plugin_coverage = [];
    packages.source_states = [];
    const noSkills = {
      ...packages.plugin_states[0]!,
      state: 'not_applicable',
      reason: 'no_skill_files',
      skills_total: 0,
      selected_ids: [],
    };
    packages.plugin_states = Array.from({ length: 10_000 }, (_, index) => ({
      ...noSkills,
      id: `discovery:community/plugin-${index}`,
      repository: `community/plugin-${index}`,
      plugin_path: '',
    }));
    packages.plugin_states.push(noSkills);
    assert.equal(decodeSkillSpectorManifestV3(packages).plugin_states.length, 10_001);
    packages.plugin_states.push(
      ...Array.from({ length: 9_999 }, (_, index) => ({
        ...noSkills,
        id: `directory/package-${index}`,
        repository: `directory/package-${index}`,
      })),
    );
    assert.equal(decodeSkillSpectorManifestV3(packages).plugin_states.length, 20_000);
    packages.plugin_states.push({ ...noSkills, id: 'over-package-state-budget' });
    assert.throws(() => decodeSkillSpectorManifestV3(packages), /invalid/);
  });

  it('accepts coverage for 10,000 community packages plus one Directory package within the v3 root budget', () => {
    const packages = structuredClone(manifest);
    packages.shards = [];
    packages.plugin_states = [];
    packages.source_states = [];
    const pending = {
      selected: 1,
      reported: 0,
      pending: 1,
      complete: 0,
      partial: 0,
      error: 0,
      stale: 0,
    };
    const rows = Array.from({ length: 20_001 }, (_, index) => ({
      ...pending,
      repository: index < 10_000 ? `community/plugin-${index}` : `directory/plugin-${index}`,
      plugin_path: '',
    }));
    for (const count of [10_001, 20_000]) {
      packages.plugin_coverage = rows.slice(0, count);
      packages.coverage = { ...pending, selected: count, pending: count };
      assert.equal(decodeSkillSpectorManifestV3(packages).plugin_coverage.length, count);
    }
    packages.plugin_coverage.push(rows[20_000]!);
    assert.throws(() => decodeSkillSpectorManifestV3(packages), /invalid/);
  });

  it('accepts actual complete reports with resolved and external reference context', () => {
    const fixture = JSON.parse(
      readFileSync(
        new URL('./fixtures/skillspector-v3-complete-context.json', import.meta.url),
        'utf8',
      ),
    );
    for (const record of Object.values(fixture.records) as SkillSpectorRecord[]) {
      const index = { ...manifest, generated_at: fixture.generated_at };
      const part = { ...descriptor, repository: record.source.repository, record_count: 1 };
      const [decoded] = decodeSkillSpectorShardV3(
        { ...shard, repository: part.repository, records: [record] },
        index,
        part,
      );
      assert.equal(decoded!.status, 'complete');
      assert.equal(decoded!.coverage!.status, 'complete');
      assert.ok(decoded!.coverage!.references!.length > 0);
      assert.equal(decoded!.coverage!.ledger_exceptions!.length, 0);
      assert.deepEqual(decoded, record);
    }
  });

  it('accepts all three real normalized reference/ledger records without dropping original positions or evidence', () => {
    for (const [key, references, diagnostics] of [
      ['cloudflare', 6, 664],
      ['web_perf', 15, 11],
      ['asc_workflow', 8, 9],
    ] as const) {
      const record = decodeReference(referenceRecord(key));
      assert.equal(record.coverage!.references!.length, references);
      assert.equal(record.coverage!.ledger_exceptions!.length, diagnostics);
      assert.deepEqual(record.coverage!.references, referenceFixture[key].coverage.references);
      assert.deepEqual(
        record.coverage!.ledger_exceptions,
        referenceFixture[key].coverage.ledger_exceptions,
      );
    }
    const main = decodeReference();
    const missing = main.coverage!.references!.find((reference) =>
      reference.evidence.includes('node_modules/wrangler/config-schema.json'),
    )!;
    assert.equal(missing.source_path, 'SKILL.md');
    assert.equal(missing.line, 32);
    assert.equal(missing.target_path, null);
    assert.equal(main.status, 'partial');
  });
  it('preserves acquired directory links as partial, and rejects forged analyzed/complete evidence', () => {
    const record = referenceRecord();
    const link = record.coverage.references[0];
    Object.assign(link, {
      status: 'uninspected',
      target_path: 'docs',
      disposition: 'partial',
      evidence: '[guides](./docs/)',
    });
    record.coverage.reasons.push({
      code: 'referenced_uninspected',
      message: 'Directory exists, but its contents were not fully inspected.',
    });
    const decoded = decodeReference(record);
    assert.equal(decoded.coverage!.references![0]!.status, 'uninspected');
    assert.equal(decoded.coverage!.references![0]!.target_path, 'docs');
    assert.equal(decoded.status, 'partial');

    const falselyAnalyzed = structuredClone(record);
    falselyAnalyzed.coverage.references[0].disposition = 'analyzed';
    assert.throws(() => decodeReference(falselyAnalyzed));
    // Start with a *valid complete* producer record; otherwise rejection could
    // come from the Cloudflare fixture's unrelated incomplete ledger evidence.
    const completeFixture = JSON.parse(
      readFileSync(
        new URL('./fixtures/skillspector-v3-complete-context.json', import.meta.url),
        'utf8',
      ),
    );
    const complete = structuredClone(
      Object.values(completeFixture.records)[0] as SkillSpectorRecord,
    );
    const index = { ...manifest, generated_at: completeFixture.generated_at };
    const part = { ...descriptor, repository: complete.source.repository, record_count: 1 };
    const decodeComplete = (entry: SkillSpectorRecord) =>
      decodeSkillSpectorShardV3(
        { ...shard, repository: part.repository, records: [entry] },
        index,
        part,
      );
    assert.equal(decodeComplete(complete)[0]!.status, 'complete');
    Object.assign(complete.coverage!.references![0], {
      status: 'uninspected',
      target_path: 'docs',
      disposition: 'partial',
    });
    assert.throws(() => decodeComplete(complete));
    const noTarget = structuredClone(record);
    noTarget.coverage.references[0].target_path = null;
    assert.throws(() => decodeReference(noTarget));
  });

  it('rejects malformed, falsely complete and silently omitted reference context', () => {
    const changes = [
      (r: any) => {
        delete r.coverage.references_total;
      },
      (r: any) => {
        r.coverage.references[0].source_path = '../escape';
      },
      (r: any) => {
        r.coverage.references[0].line = 0;
      },
      (r: any) => {
        r.coverage.references[0].source_sha256 = ['a'.repeat(64)];
      },
      (r: any) => {
        r.coverage.references[0].extra = true;
      },
      (r: any) => {
        r.coverage.ledger_exceptions[0].start_line = 10;
        r.coverage.ledger_exceptions[0].end_line = 2;
      },
      (r: any) => {
        r.coverage.references_total += 1;
      },
      (r: any) => {
        r.coverage.reference_evidence_truncated = true;
      },
      (r: any) => {
        r.status = 'complete';
      },
      (r: any) => {
        r.coverage.status = 'complete';
      },
    ];
    for (const change of changes) {
      const record = referenceRecord();
      change(record);
      assert.throws(() => decodeReference(record));
    }
  });
  it('retains explicit reference context omissions without changing finding grouping counts', () => {
    const record = referenceRecord();
    record.coverage.references.pop();
    record.coverage.reference_evidence_truncated = true;
    record.coverage.reasons.push({
      code: 'normalization_truncated',
      message: 'Reference evidence publication limit.',
    });
    const decoded = decodeReference(record);
    assert.equal(decoded.coverage!.references_total, 6);
    assert.equal(decoded.coverage!.references!.length, 5);
    assert.equal(decoded.finding_count, referenceFixture.cloudflare.finding_count);
    assert.equal(decoded.finding_group_count, referenceFixture.cloudflare.finding_group_count);
    assert.equal(
      decoded.coverage!.grouping_complete,
      referenceFixture.cloudflare.coverage.grouping_complete,
    );
  });

  it('accepts actual registry producer bytes and keeps coverage separate from severity', async () => {
    assert.equal(decodeSkillSpectorManifestV3(manifest).coverage.complete, 3);
    assert.equal(decodeSkillSpectorShardV3(shard, manifest, descriptor).length, 13);
    const fetcher = (async (url: string | URL | Request) =>
      new Response(
        String(url).endsWith('index.json') ? JSON.stringify(manifest) : body,
      )) as typeof fetch;
    const report = await createSkillSpectorClient(
      'https://fixture/v2/index.json',
      'https://fixture/v1/index.json',
      fetcher,
      'https://fixture/v3/index.json',
    ).reportFor(descriptor.repository);
    const result = skillSpectorAssessment(report, subject);
    assert.equal(result.status, 'incomplete');
    assert.equal(result.fullyChecked, 3);
    assert.equal(result.selected, 13);
    assert.equal(result.inventoryKnown, true);
    assert.deepEqual(result.outcomes, {
      complete: 3,
      limited: 10,
      failed: 0,
      historical: 0,
      unscanned: 0,
    });
    const selectedOnly = { ...report, plugin_states: [] };
    assert.equal(skillSpectorAssessment(selectedOnly, subject).inventoryKnown, false);
    assert.ok(result.findingCount > 0);
    assert.ok(result.reasons!.length > 0);
    assert.ok(result.records.some((record) => record.coverage!.analyzers.length > 0));
  });
  it('preserves producer grouping of duplicates and distinct full evidence without regrouping in the UI', () => {
    const identityRoot = new URL('./fixtures/skillspector-v3-identity/', import.meta.url);
    const index = decodeSkillSpectorManifestV3(
      JSON.parse(readFileSync(new URL('index.json', identityRoot), 'utf8')),
    );
    const part = index.shards[0]!;
    const value = JSON.parse(readFileSync(new URL(part.path, identityRoot), 'utf8'));
    const [record] = decodeSkillSpectorShardV3(value, index, part);
    assert.equal(record!.raw_finding_count, 5);
    assert.equal(record!.finding_count, 5);
    assert.equal(record!.finding_group_count, 3);
    const repeated = record!.findings.find((finding) => finding.raw_count === 3)!;
    assert.deepEqual(repeated.finding_ids, ['finding-first', 'duplicate', 'second-place']);
    assert.equal(repeated.occurrences!.length, 2);
    assert.equal(repeated.occurrences![0]!.start_column, 1);
    assert.equal(repeated.confidence, 0.85);
    assert.deepEqual(
      record!.findings
        .filter((finding) => finding.raw_count === 1)
        .map((finding) => String(finding.evidence!.payload).slice(-3))
        .sort(),
      ['one', 'two'],
    );
  });
  it('does not count a different source revision as fully checked', () => {
    const result = skillSpectorAssessment(display(), { ...subject, revision: 'f'.repeat(40) });
    assert.equal(result.status, 'stale');
    assert.equal(result.fullyChecked, 0);
  });
  it('keeps pending, unsupported and unavailable sources distinct', () => {
    const report = display();
    report.records = [];
    report.plugin_coverage = [];
    for (const [state, status] of [
      ['resolved', 'notChecked'],
      ['not_applicable', 'unsupported'],
      ['unavailable', 'unavailable'],
      ['unresolved', 'unavailable'],
    ]) {
      report.plugin_states = [
        {
          repository: subject.repository,
          plugin_path: subject.path,
          state,
          revision: subject.revision,
          skills_total: state === 'not_applicable' ? 0 : 13,
        },
      ];
      assert.equal(skillSpectorAssessment(report, subject).status, status);
    }
  });
  it('distinguishes selected missing reports from inventory with no selected check', () => {
    const report = display();
    report.records = [];
    report.plugin_coverage = [];
    Object.assign(report.plugin_states[0]!, { skills_total: 1, selected_ids: [] });
    const unselected = skillSpectorAssessment(report, subject);
    assert.equal(unselected.status, 'notChecked');
    assert.deepEqual(unselected.outcomes, {
      complete: 0,
      limited: 0,
      failed: 0,
      historical: 0,
      unscanned: 1,
    });
    report.plugin_states[0]!.selected_ids = ['selected-root'];
    const selected = skillSpectorAssessment(report, subject);
    assert.equal(selected.status, 'pending');
    assert.deepEqual(selected.retries, { scheduled: 0, blocked: 0, exhausted: 0 });
  });
  it('counts each Skill once across current, failed, historical and missing reports', () => {
    const report = display();
    report.records[0]!.freshness = 'historical';
    report.records[1]!.status = 'error';
    report.records[1]!.coverage!.status = 'not_started';
    report.records[2]!.status = 'partial';
    report.records[2]!.retry = {
      attempt_count: 2,
      disposition: 'blocked',
      next_attempt_at: null,
      next_profile: null,
      reason: 'reference_missing',
    };
    Object.assign(report.plugin_states[0]!, { skills_total: 15 });
    const result = skillSpectorAssessment(report, subject);
    assert.equal(result.status, 'stale');
    assert.equal(result.outcomes!.historical, 1);
    assert.equal(result.outcomes!.failed, 1);
    assert.equal(result.outcomes!.unscanned, 2);
    assert.equal(
      Object.values(result.outcomes!).reduce((sum, count) => sum + count, 0),
      15,
    );
    assert.ok(result.retries!.blocked > 0);
    assert.equal(
      result.fullyChecked,
      report.records.filter(
        (record) =>
          record.freshness === 'current' &&
          record.status === 'complete' &&
          record.coverage!.status === 'complete' &&
          record.execution!.state === 'completed',
      ).length,
    );
  });
  it('keeps omitted finding groups unresolved even when retained groups were classified', () => {
    const report = display();
    const record = report.records[0]!;
    const finding = record.findings[0]!;
    record.findings = [{ ...finding, raw_count: 1, review_status: 'false_positive' }];
    record.finding_count = 4;
    record.findings_truncated = true;
    record.coverage!.grouping_complete = false;
    report.records = [record];
    const result = skillSpectorAssessment(report, subject);
    assert.equal(result.unreviewedCount, 3);
    assert.equal(result.groupCount, undefined);
    assert.notEqual(result.status, 'reviewedFindings');
  });
  for (const retained of ['classified', 'empty']) {
    it(`keeps decoder-accepted omitted groups unresolved with ${retained} retained findings and no numeric surplus`, () => {
      const value = structuredClone(shard);
      const complete = value.records.find(
        (record: SkillSpectorRecord) => record.status === 'complete',
      );
      const record =
        retained === 'classified'
          ? value.records.find((record: SkillSpectorRecord) =>
              record.findings.some((finding) => finding.review_status === 'false_positive'),
            )
          : structuredClone(complete);
      record.status = 'complete';
      record.coverage = structuredClone(complete.coverage);
      record.execution.state = 'completed';
      record.execution.exit_code = 0;
      record.errors = [];
      record.findings =
        retained === 'classified'
          ? record.findings.filter(
              (finding: { review_status: string }) => finding.review_status === 'false_positive',
            )
          : [];
      record.finding_count = record.findings.reduce(
        (sum: number, finding: { raw_count: number }) => sum + finding.raw_count,
        0,
      );
      record.raw_finding_count = record.finding_count;
      record.finding_group_count = record.findings.length + 1;
      record.findings_truncated = true;
      const [decoded] = decodeSkillSpectorShardV3({ ...value, records: [record] }, manifest, {
        ...descriptor,
        record_count: 1,
      });
      const report = display();
      report.records = [decoded!];
      report.plugin_coverage[0] = {
        ...report.plugin_coverage[0]!,
        selected: 1,
        reported: 1,
        complete: 1,
        partial: 0,
        error: 0,
        pending: 0,
        stale: 0,
      };
      Object.assign(report.plugin_states[0]!, { skills_total: 1, selected_ids: [decoded!.id] });
      const result = skillSpectorAssessment(report, subject);
      assert.equal(
        result.unreviewedCount,
        0,
        'Unknown groups must not invent a numeric finding count',
      );
      assert.equal(result.findingCount, record.finding_count);
      assert.equal(result.status, 'unreviewedFindings');
    });
  }
  it('keeps standalone source states specific and exposes discovery reasons after aliases', () => {
    const report = display();
    report.records = [];
    report.plugin_coverage = [];
    report.repository_aliases = [
      { repository: 'old/repo', canonical_repository: 'new/repo', revision: 'a'.repeat(40) },
    ];
    report.source_states = [
      {
        id: 'old/repo/alpha',
        repository: 'old/repo',
        state: 'resolved',
        selected_ids: ['alpha'],
        reason: null,
      },
      {
        id: 'old/repo/beta',
        repository: 'old/repo',
        state: 'unresolved',
        selected_ids: [],
        reason: 'ambiguous_skill_path',
        candidates: ['skills/one', 'skills/two'],
      },
    ];
    assert.equal(
      skillSpectorAssessment(report, { kind: 'skill', repository: 'old/repo', name: 'alpha' })
        .status,
      'pending',
    );
    const beta = skillSpectorAssessment(report, {
      kind: 'skill',
      repository: 'old/repo',
      name: 'beta',
    });
    assert.equal(beta.status, 'unavailable');
    assert.ok(beta.reasons!.some((reason) => reason.code === 'ambiguous_skill_path'));
    assert.ok(beta.reasons!.some((reason) => reason.message.includes('skills/two')));
  });
  it('retains a removed Skill report as history with no current fully checked content', () => {
    const report = display();
    const old = report.records[0]!;
    old.kind = 'standalone_skill';
    old.plugin_path = null;
    old.skill_name = 'removed-skill';
    old.freshness = 'historical';
    report.records = [old];
    report.plugin_coverage = [];
    report.source_states = [
      {
        id: `${subject.repository}/removed-skill`,
        repository: subject.repository,
        state: 'historical',
        current: false,
        revision: 'f'.repeat(40),
        path: old.source.path,
        reason: 'skill_removed',
        selected_ids: [old.id],
        candidates: [],
      },
    ];
    const result = skillSpectorAssessment(report, {
      kind: 'skill',
      repository: subject.repository,
      name: 'removed-skill',
    });
    assert.equal(result.status, 'stale');
    assert.equal(result.records[0], old);
    assert.equal(result.fullyChecked, 0);
    assert.equal(result.reasons!.at(-1)!.code, 'skill_removed');
  });
  it('accepts the bounded acquisition transfer-limit reason without treating it as a content finding', () => {
    const value = structuredClone(shard);
    const partial = value.records.find((record: SkillSpectorRecord) => record.status === 'partial');
    partial.coverage.reasons.push({
      code: 'acquisition_transfer_limit',
      message: 'Transfer budget exhausted.',
      retryable: true,
    });
    const decoded = decodeSkillSpectorShardV3(value, manifest, descriptor);
    const result = skillSpectorAssessment({ ...manifest, records: decoded }, subject);
    assert.equal(result.status, 'incomplete');
    assert.equal(result.findingCount, skillSpectorAssessment(display(), subject).findingCount);
    assert.ok(result.reasons!.some((reason) => reason.code === 'acquisition_transfer_limit'));
  });
  it('shows unavailable when every acquisition failed without inspecting any content', () => {
    const report = display();
    for (const record of report.records) {
      record.execution!.state = 'failed';
      record.coverage!.status = 'not_started';
    }
    assert.equal(skillSpectorAssessment(report, subject).status, 'unavailable');
  });
  it('rejects forged review confirmation and legacy bypass of complete analysis', () => {
    const forged = structuredClone(shard);
    forged.records.flatMap((record: any) => record.findings)[0].review_status = 'confirmed';
    assert.throws(() => decodeSkillSpectorShardV3(forged, manifest, descriptor), /invalid/);
    const bypass = structuredClone(shard);
    const complete = bypass.records.find((record: any) => record.status === 'complete');
    complete.legacy_format = 2;
    complete.coverage.analyzers = [];
    complete.execution.exit_code = 2;
    assert.throws(() => decodeSkillSpectorShardV3(bypass, manifest, descriptor), /invalid/);
  });
  it('matches verified repository aliases without changing source links or revisions', () => {
    const report = display();
    report.repository_aliases = [
      {
        repository: 'old-owner/repo',
        canonical_repository: descriptor.repository,
        revision: subject.revision,
      },
    ];
    assert.equal(
      skillSpectorAssessment(report, { ...subject, repository: 'old-owner/repo' }).records.length,
      13,
    );
  });
  it('accepts historical repository aliases only at the verified target revision', () => {
    const report = display();
    const renamed = structuredClone(shard);
    const moved = { ...descriptor, repository: 'new-owner/repo' };
    report.repository_aliases = [
      {
        repository: descriptor.repository,
        canonical_repository: moved.repository,
        revision: subject.revision,
      },
    ];
    report.plugin_coverage[0]!.repository = moved.repository;
    renamed.repository = moved.repository;
    for (const record of renamed.records) record.freshness = 'historical';
    const decoded = decodeSkillSpectorShardV3(renamed, report, moved);
    assert.equal(
      skillSpectorAssessment(
        { ...report, records: decoded },
        { ...subject, repository: moved.repository },
      ).records.length,
      13,
    );
    report.repository_aliases[0]!.revision = 'f'.repeat(40);
    assert.throws(() => decodeSkillSpectorShardV3(renamed, report, moved), /invalid/);
  });
  it('accepts the verbatim published reverse historical alias with one verified step on both names', () => {
    const record = JSON.parse(
      readFileSync(
        new URL('./fixtures/skillspector-v3-historical-reverse-alias.json', import.meta.url),
        'utf8',
      ),
    );
    const metadata = JSON.parse(
      readFileSync(
        new URL('./fixtures/skillspector-v3-historical-reverse-alias.meta.json', import.meta.url),
        'utf8',
      ),
    );
    const index = { ...manifest, ...metadata, repository_aliases: [metadata.alias] };
    const part = metadata.descriptor;
    const value = {
      schema_version: 3,
      repository: part.repository,
      scanner: index.scanner,
      profile: index.profile,
      records: [record],
    };
    assert.deepEqual(decodeSkillSpectorShardV3(value, index, part), [record]);

    for (const aliases of [
      [],
      [{ ...metadata.alias, revision: 'f'.repeat(40) }],
      [{ ...metadata.alias, repository: 'unverified/repo' }],
      [
        { ...metadata.alias, canonical_repository: 'intermediate/repo' },
        { ...metadata.alias, repository: 'intermediate/repo' },
      ],
    ]) {
      assert.throws(
        () => decodeSkillSpectorShardV3(value, { ...index, repository_aliases: aliases }, part),
        /invalid/,
      );
    }
    const current = structuredClone(value);
    current.records[0].freshness = 'current';
    assert.throws(() => decodeSkillSpectorShardV3(current, index, part), /invalid/);
  });
  it('preserves and verifies the two content-bound registry false-positive annotations', () => {
    const decoded = decodeSkillSpectorShardV3(shard, manifest, descriptor);
    const annotations = decoded
      .flatMap((record) => record.findings)
      .filter((finding) => finding.review_status === 'false_positive');
    assert.equal(annotations.length, 2);
    assert.ok(
      annotations.every(
        (finding) =>
          finding.severity === 'HIGH' &&
          finding.exception!.reason.includes('static asset upload session'),
      ),
    );
    const result = skillSpectorAssessment({ ...manifest, records: decoded }, subject);
    assert.equal(
      result.unreviewedCount,
      result.findingCount - annotations.reduce((sum, finding) => sum + finding.raw_count!, 0),
    );
    const forged = structuredClone(shard);
    const finding = forged.records
      .flatMap((record: any) => record.findings)
      .find((finding: any) => finding.review_status === 'false_positive');
    finding.code_snippet += ' changed';
    assert.throws(() => decodeSkillSpectorShardV3(forged, manifest, descriptor), /invalid/);
  });
  it('retains output-limited findings as lower bounds with explicit coverage reasons', () => {
    const limitedShard = structuredClone(shard);
    const partial = limitedShard.records.find(
      (record: SkillSpectorRecord) => record.status === 'partial',
    );
    partial.coverage.grouping_complete = false;
    partial.findings_truncated = true;
    partial.coverage.reasons.push({
      code: 'output_limit',
      message: 'Only emitted signals are available.',
      retryable: true,
    });
    const decoded = decodeSkillSpectorShardV3(limitedShard, manifest, descriptor);
    const limited = decoded.find((record) => record.coverage!.grouping_complete === false)!;
    assert.ok(limited);
    assert.equal(limited.status, 'partial');
    assert.equal(limited.findings_truncated, true);
    assert.ok(
      limited.coverage!.reasons.some((reason) =>
        ['output_limit', 'normalization_truncated'].includes(reason.code),
      ),
    );
    assert.equal(
      skillSpectorAssessment({ ...manifest, records: decoded }, subject).groupCount,
      undefined,
    );
  });
  it('rejects ambiguous repository aliases and incomplete publication disguised as complete', () => {
    const aliases = structuredClone(manifest);
    aliases.repository_aliases = [
      { repository: 'old/repo', canonical_repository: 'one/repo', revision: subject.revision },
      { repository: 'OLD/repo', canonical_repository: 'two/repo', revision: subject.revision },
    ];
    assert.throws(() => decodeSkillSpectorManifestV3(aliases), /invalid/);
    const output = structuredClone(shard);
    const record = output.records.find((record: any) => record.status === 'complete');
    record.coverage.grouping_complete = false;
    record.findings_truncated = true;
    assert.throws(() => decodeSkillSpectorShardV3(output, manifest, descriptor), /invalid/);
  });
  it('never turns legacy partial evidence into a complete result', () => {
    const report = display();
    const record = report.records[0]!;
    record.freshness = 'historical';
    record.coverage!.status = 'partial';
    record.coverage!.grouping_complete = false;
    record.finding_count = 603;
    record.findings_truncated = true;
    const result = skillSpectorAssessment(report, subject);
    assert.equal(result.status, 'stale');
    assert.equal(result.groupCount, undefined);
    assert.ok(result.fullyChecked! < 13);
  });
  it('rejects misleading full coverage, invalid confidence and stale counter arithmetic', () => {
    const malformed = structuredClone(shard);
    malformed.records[0].coverage.status = 'complete';
    malformed.records[0].coverage.entirely_uninspected_files = 1;
    assert.throws(() => decodeSkillSpectorShardV3(malformed, manifest, descriptor), /invalid/);
    const confidence = structuredClone(shard);
    const finding = confidence.records.flatMap((record: any) => record.findings)[0];
    finding.confidence = 1.1;
    assert.throws(() => decodeSkillSpectorShardV3(confidence, manifest, descriptor), /invalid/);
    const counters = structuredClone(manifest);
    counters.coverage.complete++;
    assert.throws(() => decodeSkillSpectorManifestV3(counters), /invalid/);
  });
  it('does not fall back after v3 validation or transport errors', async () => {
    for (const response of [new Response('', { status: 503 }), new Response('{}')]) {
      const requests: string[] = [];
      const fetcher = (async (url: string | URL | Request) => {
        requests.push(String(url));
        return response;
      }) as typeof fetch;
      await assert.rejects(
        createSkillSpectorClient(
          'https://fixture/v2/index.json',
          'https://fixture/v1/index.json',
          fetcher,
          'https://fixture/v3/index.json',
        ).reportFor(subject.repository),
      );
      assert.deepEqual(
        requests,
        Array(response.status === 503 ? 2 : 1).fill('https://fixture/v3/index.json'),
      );
    }
  });
});
