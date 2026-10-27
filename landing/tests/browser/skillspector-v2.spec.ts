import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { expect, test } from '@playwright/test';
import type { SkillSpectorManifest, SkillSpectorRecord } from '../../types/skillSpector';
import { skillSpectorV2ProfileDigest } from '../../utils/skillSpectorCatalog';
import { deferred, navigate, offline, projections, registryFixture } from './i18n-state.helpers';

const fixture = JSON.parse(
  readFileSync(new URL('../fixtures/skillspector-report.json', import.meta.url), 'utf8'),
);
const checked = () => new Date(Date.now() - 60_000).toISOString().replace(/\.\d{3}Z$/, 'Z');
const profile = { id: 'static-only-v2' as const, digest: skillSpectorV2ProfileDigest };
function wire(records: SkillSpectorRecord[]) {
  const bodies = new Map<string, string>();
  const shards = records.map((record) => {
    const repository = record.source.repository;
    const body = JSON.stringify({
      schema_version: 2,
      scanner: fixture.scanner,
      profile,
      repository,
      records: [record],
    });
    const hash = createHash('sha256').update(body).digest('hex');
    const path = `shards/${hash}.json`;
    bodies.set(path, body);
    return {
      repository,
      path,
      digest: `sha256:${hash}`,
      bytes: Buffer.byteLength(body),
      record_count: 1,
    };
  });
  const manifest: SkillSpectorManifest = {
    schema_version: 2,
    generated_at: checked(),
    source_commit: 'a'.repeat(40),
    scanner: fixture.scanner,
    profile,
    scope: 'selected_sources',
    selection_digest: `sha256:${'a'.repeat(64)}`,
    coverage: { selected: 10_000, reported: records.length, pending: 10_000 - records.length },
    shards,
  };
  return { manifest, bodies };
}
function record(repository: string, name: string): SkillSpectorRecord {
  const result: SkillSpectorRecord = structuredClone(fixture.records[0]);
  Object.assign(result, {
    id: name,
    kind: 'standalone_skill',
    plugin_path: null,
    skill_name: name,
    scanned_at: checked(),
  });
  result.source.repository = repository;
  return result;
}

for (const width of [1280, 390]) {
  test(`v2 loads only needed repositories and isolates failure across reused cards (${width})`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    const alpha = record('example/alpha', 'alpha');
    const beta = record('example/beta', 'beta');
    const unused = record('example/unused', 'unused');
    const { manifest, bodies } = wire([alpha, beta, unused]);
    const requests: string[] = [];
    const delayed = deferred();
    await page.route('**/reports/skillspector/v2/**', async (route) => {
      const suffix = new URL(route.request().url()).pathname.split('/v2/')[1]!;
      requests.push(suffix);
      if (suffix === 'index.json') return route.fulfill({ json: manifest });
      if (suffix === manifest.shards[0]!.path) await delayed.promise;
      if (suffix === manifest.shards[1]!.path) return route.fulfill({ status: 503 });
      return route.fulfill({ body: bodies.get(suffix), contentType: 'application/json' });
    });
    await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) => {
      const q = new URL(route.request().url()).searchParams.get('q') || 'alpha';
      const name = q === 'beta' ? 'beta' : q === 'pending' ? 'pending' : 'alpha';
      return route.fulfill({
        json: {
          items: [
            {
              id: 'reused-listing',
              providerId: 'skills-sh',
              providerUrl: `https://skills.sh/example/${name}/${name}`,
              title: name,
              skillNameHint: name,
              source: {
                kind: 'github',
                repository: `example/${name}`,
                url: `https://github.com/example/${name}`,
              },
              metrics: { installs: 1234, window: 'all-time' },
              availability: 'unverified',
            },
          ],
          page: 0,
          hasMore: false,
          view: 'popular',
          cache: { status: 'fresh', fetchedAt: checked() },
        },
      });
    });
    await projections(page, registryFixture());
    await navigate(page, '/plugins/', 'en', '?tab=skills&q=alpha');
    const assessment = page.locator('.skill-card .skillspector');
    await expect(assessment).toHaveClass(/skillspector--loading/);
    await expect.poll(() => requests.includes(manifest.shards[0]!.path)).toBe(true);
    // The listing keeps its key while the repository changes. An old in-flight
    // success must not replace the new repository's failure state.
    await navigate(page, '/plugins/', 'en', '?tab=skills&q=beta');
    await expect(assessment).toHaveClass(/skillspector--unavailable/);
    delayed.release();
    await expect(assessment).toHaveClass(/skillspector--unavailable/);
    await navigate(page, '/plugins/', 'en', '?tab=skills&q=pending');
    await expect(assessment).toHaveClass(/skillspector--notChecked/);
    await navigate(page, '/plugins/', 'en', '?tab=skills&q=alpha');
    await expect(assessment).toHaveClass(/skillspector--historical/);
    await assessment.locator(':scope > summary').click();
    await expect(assessment).toContainText(
      'The current revision of this listing has not been verified.',
    );
    expect(requests.filter((path) => path === 'index.json')).toHaveLength(1);
    expect(requests.filter((path) => path === manifest.shards[0]!.path)).toHaveLength(1);
    expect(requests).not.toContain(manifest.shards[2]!.path);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    expect(errors).toEqual([]);
  });
}
