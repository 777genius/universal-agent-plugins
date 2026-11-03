import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import type { SkillSpectorManifestV3, SkillSpectorRecord } from '../../types/skillSpector.ts';
import { offline } from './i18n-state.helpers';

const root = new URL('../fixtures/skillspector-v3/', import.meta.url);
const seed = JSON.parse(readFileSync(new URL('index.json', root), 'utf8')) as SkillSpectorManifestV3;
const evidence = JSON.parse(
  readFileSync(new URL('../fixtures/skillspector-v3-complete-context.json', import.meta.url), 'utf8'),
) as { generated_at: string; records: { resolved: SkillSpectorRecord } };

function skill(repository: string, name: string) {
  return {
    id: 'skills-sh:' + repository + ':' + name,
    providerId: 'skills-sh',
    providerUrl: 'https://skills.sh/' + repository + '/' + name,
    title: name,
    skillNameHint: name,
    source: { kind: 'github', repository, url: 'https://github.com/' + repository },
    metrics: { installs: 100, window: 'all-time' },
    availability: 'unverified',
  };
}

async function prepare(page: Page, mode: 'current' | 'historical' | 'unavailable') {
  const errors = await offline(page);
  const manifest = structuredClone(seed);
  const record = structuredClone(evidence.records.resolved);
  const current = mode === 'current';
  if (!current) {
    record.freshness = 'historical';
    record.target_revision = 'f'.repeat(40);
  }
  manifest.generated_at = evidence.generated_at;
  manifest.plugin_coverage = [];
  manifest.plugin_states = [];
  manifest.source_states = [{
    id: record.source.repository + '/' + record.skill_name,
    repository: record.source.repository,
    state: 'resolved',
    current: true,
    reason: null,
    revision: record.target_revision,
    path: record.source.path,
    selected_ids: [record.id],
    candidates: [],
  }];
  manifest.coverage = {
    selected: 1, reported: 1, pending: current ? 0 : 1,
    complete: current ? 1 : 0, partial: 0, error: 0, stale: current ? 0 : 1,
  };
  const shard = {
    schema_version: 3, repository: record.source.repository,
    scanner: manifest.scanner, profile: manifest.profile, records: [record],
  };
  const body = JSON.stringify(shard);
  const hash = createHash('sha256').update(body).digest('hex');
  manifest.shards = [{
    repository: record.source.repository, path: 'shards/' + hash + '.json',
    digest: 'sha256:' + hash, bytes: Buffer.byteLength(body), record_count: 1,
  }];
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route =>
    route.fulfill({ json: {
      items: [skill(record.source.repository, record.skill_name!), skill('someone/else', 'unscanned')],
      page: 0, total: 2, hasMore: false, view: 'popular',
      cache: { status: 'fresh', fetchedAt: evidence.generated_at },
    } }),
  );
  let verifiedShardRequests = 0;
  await page.route('**/reports/skillspector/v3/**', route => {
    if (mode === 'unavailable') return route.fulfill({ status: 503, body: 'Unavailable' });
    if (route.request().url().endsWith('/index.json'))
      return route.fulfill({ json: manifest });
    verifiedShardRequests++;
    return route.fulfill({ body, contentType: 'application/json' });
  });
  return { errors, verifiedShards: () => verifiedShardRequests };
}

test('default Skills browse shows only a physically executed current source scan', async ({ page }) => {
  const { errors, verifiedShards } = await prepare(page, 'current');
  await page.goto('plugins/?tab=skills');
  const cards = page.locator('.skills-catalog .skill-card');
  await expect(cards).toHaveCount(1);
  await expect(cards.first()).toHaveAttribute(
    'data-skill-id', 'skills-sh:affaan-m/ecc:recsys-pipeline-architect',
  );
  await expect(cards.first().locator('.skillspector > summary')).not.toContainText('Previous check');
  await expect(page.locator('.skills-catalog__scan-notice')).toBeVisible();
  expect(verifiedShards()).toBe(1);
  await page.goto('plugins/?tab=skills&skillScan=all');
  await expect(cards).toHaveCount(2);
  expect(errors).toEqual([]);
});

test('historical-only report does not count as a current scan and can be browsed explicitly', async ({ page }) => {
  const { errors } = await prepare(page, 'historical');
  await page.goto('plugins/?tab=skills');
  await expect(page.locator('.skills-catalog .skill-card')).toHaveCount(0);
  await expect(page.getByText('No Skills with a current scan were found in this bounded selection.')).toBeVisible();
  await page.getByRole('button', { name: 'Browse all installable Skills' }).click();
  await expect(page.locator('.skills-catalog .skill-card')).toHaveCount(2);
  await expect(page).toHaveURL(/skillScan=all/);
  await expect(page.locator('.skill-card').first().locator('.skillspector > summary')).toContainText('Previous check');
  expect(errors).toEqual([]);
});

test('failed published reports never produce a verified Skill card', async ({ page }) => {
  const { errors } = await prepare(page, 'unavailable');
  await page.goto('plugins/?tab=skills');
  await expect(page.locator('.skills-catalog .skill-card')).toHaveCount(0);
  await expect(page.getByText('Some scan reports could not be verified. Those Skills were excluded.')).toBeVisible();
  expect(errors).toEqual([]);
});
