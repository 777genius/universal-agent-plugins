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
  await expect(cards.first().locator('.skillspector > summary')).not.toContainText('Outdated scan');
  await expect(page.locator('.skills-catalog__scan-notice')).toBeVisible();
  await expect(page.locator('.skills-catalog__scan-coverage')).toContainText(
    '1 complete, 0 partial, 0 failed, 0 awaiting a current check',
  );
  expect(verifiedShards()).toBe(1);
  await page.goto('plugins/?tab=skills&skillScan=all');
  await expect(cards).toHaveCount(2);
  await expect(page.locator('.skills-catalog__scan-coverage')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('many unscanned popular Skills cannot crowd a current scan out of the bounded window', async ({ page }) => {
  const { errors } = await prepare(page, 'current');
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route => {
    const pageNumber = Number(new URL(route.request().url()).searchParams.get('page') ?? '0');
    const items = pageNumber < 2
      ? Array.from({ length: 48 }, (_, i) => skill('no-scan/repo-' + (pageNumber * 48 + i), 'candidate'))
      : [skill('affaan-m/ecc', 'recsys-pipeline-architect')];
    return route.fulfill({ json: {
      items, page: pageNumber, total: 97, hasMore: pageNumber < 2, view: 'popular',
      cache: { status: 'fresh', fetchedAt: evidence.generated_at },
    } });
  });
  await page.goto('plugins/?tab=skills');
  const cards = page.locator('.skills-catalog .skill-card');
  await expect(cards).toHaveCount(1);
  await expect(cards.first()).toHaveAttribute(
    'data-skill-id', 'skills-sh:affaan-m/ecc:recsys-pipeline-architect',
  );
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
  await expect(page.locator('.skill-card').first().locator('.skillspector > summary')).toContainText('Outdated scan');
  expect(errors).toEqual([]);
});

test('failed published reports never produce a verified Skill card', async ({ page }) => {
  const { errors } = await prepare(page, 'unavailable');
  await page.goto('plugins/?tab=skills');
  await expect(page.locator('.skills-catalog .skill-card')).toHaveCount(0);
  await expect(page.locator('.skills-catalog__scan-coverage')).toHaveCount(0);
  await expect(page.getByText('Some scan reports could not be verified. Those Skills were excluded.')).toBeVisible();
  expect(errors).toEqual([]);
});

test('legacy v2 fallback is explicitly unavailable, never an empty verified scan', async ({ page }) => {
  const errors = await offline(page);
  const requests: string[] = [];
  page.on('request', request => {
    if (request.url().includes('/reports/skillspector/')) requests.push(request.url());
  });
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route =>
    route.fulfill({ json: {
      items: [skill('affaan-m/ecc', 'recsys-pipeline-architect')],
      page: 0, total: 1, hasMore: false, view: 'popular',
      cache: { status: 'fresh', fetchedAt: evidence.generated_at },
    } }),
  );

  // offline() supplies a v3 404 and a structurally valid v2 manifest.
  await page.goto('plugins/?tab=skills');
  const catalog = page.locator('.skills-catalog');
  await expect(catalog).toHaveAttribute('aria-busy', 'false');
  await expect(catalog.locator('.skill-card')).toHaveCount(0);
  await expect(catalog.locator('.skills-catalog__scan-coverage')).toHaveCount(0);
  await expect(catalog.getByText(
    'Some scan reports could not be verified. Those Skills were excluded.',
  )).toBeVisible();
  expect(requests.some(url => url.endsWith('/v3/index.json'))).toBe(true);
  expect(requests.some(url => url.endsWith('/v2/index.json'))).toBe(true);

  // The opt-in all-installable mode still exposes the independently
  // installable Skill, without assigning it current scan evidence.
  await page.goto('plugins/?tab=skills&skillScan=all');
  await expect(catalog.locator('.skill-card')).toHaveCount(1);
  expect(errors).toEqual([]);
});
