import { createHash } from 'node:crypto';
import { readFileSync, mkdirSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import type { SkillSpectorManifestV3, SkillSpectorRecord } from '../../types/skillSpector.ts';
import {
  decodeSkillSpectorManifestV3,
  decodeSkillSpectorShardV3,
} from '../../utils/skillSpectorV3.ts';
import { offline, navigate } from './i18n-state.helpers';
const root = new URL('../fixtures/skillspector-v3/', import.meta.url);
const seed = JSON.parse(
  readFileSync(new URL('index.json', root), 'utf8'),
) as SkillSpectorManifestV3;
const evidence = JSON.parse(
  readFileSync(
    new URL('../fixtures/skillspector-v3-complete-context.json', import.meta.url),
    'utf8',
  ),
);
const repository = 'affaan-m/ecc';
const id = (name: string) => `skills-sh:${repository}:${name}`;
function skill(name: string) {
  return {
    id: id(name),
    providerId: 'skills-sh',
    providerUrl: `https://skills.sh/${repository}/${name}`,
    title: name,
    skillNameHint: name,
    source: { kind: 'github', repository, url: `https://github.com/${repository}` },
    metrics: { installs: 100, window: 'all-time' },
    availability: 'unverified',
  };
}
function record(
  name: string,
  mode: 'clean' | 'dirty' | 'partial' | 'empty' | 'error' | 'historical' = 'clean',
): SkillSpectorRecord {
  const value = structuredClone(evidence.records.resolved) as SkillSpectorRecord;
  value.id = `fixture-${name}`;
  value.skill_name = name;
  value.source.path = `skills/${name}`;
  Object.assign(value.coverage!, {
    references: [],
    references_total: 0,
    ledger_exceptions: [],
    ledger_exceptions_total: 0,
    reference_evidence_truncated: false,
  });
  if (mode !== 'dirty') {
    value.findings = [];
    value.finding_count = value.raw_finding_count = value.finding_group_count = 0;
    value.coverage!.original_findings = [];
  }
  if (mode === 'partial' || mode === 'empty') {
    value.status = value.coverage!.status = 'partial';
    value.coverage!.reasons = [{ code: 'runtime_limit', message: 'Bounded inspection.' }];
    value.coverage!.fully_inspected_files = mode === 'partial' ? 1 : 0;
    value.coverage!.entirely_uninspected_files = 1;
    value.retry = {
      attempt_count: 1,
      disposition: 'blocked',
      reason: 'fixture_limit',
      next_attempt_at: null,
      next_profile: null,
    };
  }
  if (mode === 'historical') {
    value.freshness = 'historical';
    value.target_revision = 'f'.repeat(40);
  }
  if (mode === 'error') {
    value.status = 'error';
    value.scanned_at = null;
    value.source.content_digest = null;
    value.execution!.state = 'failed';
    value.execution!.exit_code = 2;
    value.coverage!.status = 'not_started';
    value.coverage!.fully_inspected_files =
      value.coverage!.partially_inspected_files =
      value.coverage!.entirely_uninspected_files =
        0;
    value.errors = [{ code: 'acquisition_failed', message: 'Fixture transport failure.' }];
  }
  return value;
}
async function prepare(
  page: Page,
  records = [
    record('clean'),
    record('dirty', 'dirty'),
    record('partial', 'partial'),
    record('empty', 'empty'),
    record('error', 'error'),
    record('historical', 'historical'),
  ],
) {
  const errors = await offline(page);
  const manifest = structuredClone(seed);
  manifest.generated_at = evidence.generated_at;
  manifest.plugin_coverage = [];
  manifest.plugin_states = [];
  manifest.source_states = records.map((value) => ({
    id: `${repository}/${value.skill_name}`,
    repository,
    state: 'resolved',
    current: true,
    reason: null,
    revision: value.target_revision,
    path: value.source.path,
    selected_ids: [value.id],
    candidates: [],
  }));
  const current = records.filter((value) => value.freshness === 'current');
  manifest.coverage = {
    selected: records.length,
    reported: records.length,
    complete: current.filter((value) => value.status === 'complete').length,
    partial: current.filter((value) => value.status === 'partial').length,
    error: current.filter((value) => value.status === 'error').length,
    pending: records.length - current.length,
    stale: records.length - current.length,
  };
  const shard = {
    schema_version: 3,
    repository,
    scanner: manifest.scanner,
    profile: manifest.profile,
    records,
  };
  const body = JSON.stringify(shard);
  const hash = createHash('sha256').update(body).digest('hex');
  const descriptor = {
    repository,
    path: `shards/${hash}.json`,
    digest: `sha256:${hash}`,
    bytes: Buffer.byteLength(body),
    record_count: records.length,
  };
  manifest.shards = [descriptor];
  decodeSkillSpectorManifestV3(manifest);
  decodeSkillSpectorShardV3(shard, manifest, descriptor);
  const catalogRequests: URL[] = [];
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) => {
    catalogRequests.push(new URL(route.request().url()));
    return route.fulfill({
      json: {
        items: [...records.map((value) => skill(value.skill_name!)), skill('unscanned')],
        page: 0,
        total: records.length + 1,
        hasMore: false,
        view: 'popular',
        cache: { status: 'fresh', fetchedAt: evidence.generated_at },
      },
    });
  });
  let unavailable = false;
  let shardRequests = 0;
  await page.route('**/reports/skillspector/v3/**', (route) => {
    if (unavailable) return route.fulfill({ status: 503, body: 'Unavailable' });
    if (route.request().url().endsWith('/index.json')) return route.fulfill({ json: manifest });
    shardRequests++;
    return route.fulfill({ body, contentType: 'application/json' });
  });
  return {
    errors,
    manifest,
    catalogRequests,
    setUnavailable: (value: boolean) => {
      unavailable = value;
    },
    shards: () => shardRequests,
  };
}
const cards = (page: Page) => page.locator('.skills-catalog .skill-card');
async function expectedIds(page: Page, names: string[]) {
  await expect(cards(page)).toHaveCount(names.length);
  expect(
    await cards(page).evaluateAll((nodes) =>
      nodes.map((node) => node.getAttribute('data-skill-id')),
    ),
  ).toEqual(names.map(id));
}
async function openFilters(page: Page) {
  await page.getByRole('button', { name: /^Filters/ }).click();
  return page.getByRole('dialog', { name: 'Filter Skills', exact: true });
}
async function closeFilters(page: Page) {
  await page.keyboard.press('Escape');
}
const allNames = ['clean', 'dirty', 'partial', 'empty', 'error', 'historical', 'unscanned'];

test('explicit filter retry recovers a newly published v3 after fulfilled legacy fallback', async ({ page }) => {
  const fixture = await prepare(page);
  let fallback = true;
  await page.route('**/reports/skillspector/v3/index.json', route =>
    fallback ? route.fulfill({ status: 404 }) : route.fulfill({ json: fixture.manifest }));
  await page.goto('plugins/?tab=skills&skillRemarks=none');
  await expect(page.locator('.skills-catalog')).toHaveAttribute('aria-busy', 'false');
  await expect(cards(page)).toHaveCount(0);
  await expect(page.getByText('Some scan reports could not be verified. Those Skills were excluded.')).toBeVisible();
  fallback = false;
  await page.getByRole('button', { name: 'Try again', exact: true }).click();
  await expectedIds(page, ['clean']);
  expect(fixture.catalogRequests.length).toBe(2);
  expect(fixture.errors).toEqual([]);
});

test('contradictory duplicate source states fail both filters closed at the verified manifest boundary', async ({ page }) => {
  const fixture = await prepare(page);
  const forged = structuredClone(fixture.manifest);
  forged.source_states.push({ ...structuredClone(forged.source_states[0]!),
    current: false, state: 'unresolved', reason: 'ambiguous_name', selected_ids: [],
    candidates: ['skills/one', 'skills/two'] });
  await page.route('**/reports/skillspector/v3/index.json', route => route.fulfill({ json: forged }));
  for (const query of ['skillScan=scanned', 'skillRemarks=none']) {
    await page.goto(`plugins/?tab=skills&${query}`);
    await expect(page.locator('.skills-catalog')).toHaveAttribute('aria-busy', 'false');
    await expect(cards(page)).toHaveCount(0);
    await expect(page.getByText('Some scan reports could not be verified. Those Skills were excluded.')).toBeVisible();
  }
  await page.goto('plugins/?tab=skills');
  await expectedIds(page, allNames);
  expect(fixture.errors).toEqual([]);
});

test('default all Skills and independent filters use exact current records during browse and search', async ({
  page,
}) => {
  const { errors, shards } = await prepare(page);
  await page.goto('plugins/?tab=skills');
  await expectedIds(page, allNames);
  const dialog = await openFilters(page);
  const scanned = dialog.getByRole('checkbox', { name: 'Scanned by SkillSpector', exact: true });
  const clean = dialog.getByRole('checkbox', { name: 'No remarks', exact: true });
  await expect(scanned).not.toBeChecked();
  await expect(clean).not.toBeChecked();
  await scanned.check();
  await closeFilters(page);
  await expectedIds(page, ['clean', 'dirty', 'partial']);
  await expect(page).toHaveURL(/skillScan=scanned/);
  await openFilters(page);
  await clean.check();
  await closeFilters(page);
  await expectedIds(page, ['clean']);
  await openFilters(page);
  await scanned.uncheck();
  await expect(clean).toBeChecked();
  await closeFilters(page);
  await expectedIds(page, ['clean']);
  expect(new URL(page.url()).searchParams.has('skillScan')).toBe(false);
  await page.getByRole('searchbox', { name: 'Search Skills', exact: true }).fill('any');
  await expect(page).toHaveURL(/q=any/);
  await expectedIds(page, ['clean']);
  await openFilters(page);
  await scanned.check();
  await clean.uncheck();
  await closeFilters(page);
  await expectedIds(page, ['clean', 'dirty', 'partial']);
  await openFilters(page);
  await dialog.getByRole('button', { name: 'Reset filters' }).click();
  await closeFilters(page);
  await expectedIds(page, allNames);
  expect(shards()).toBe(1);
  expect(errors).toEqual([]);
});

test('filtered search retains candidates past 64 inspections, displays past 48 and never fetches search page one', async ({
  page,
}) => {
  const records = Array.from({ length: 170 }, (_, index) =>
    record(`candidate-${index}`, index < 70 ? 'dirty' : 'clean'),
  );
  const { errors, catalogRequests } = await prepare(page, records);
  await page.goto('plugins/?tab=skills&q=candidate&skillRemarks=none');
  await expect(page.locator('.skills-catalog')).toHaveAttribute('aria-busy', 'false');
  await expect(cards(page)).toHaveCount(0);
  await expect(page.getByText('More Skills can be checked', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Show more Skills' }).click();
  await expect(cards(page)).toHaveCount(48);
  for (
    let step = 0;
    step < 8 && (await page.getByRole('button', { name: 'Show more Skills' }).count());
    step++
  ) {
    await page.getByRole('button', { name: 'Show more Skills' }).click();
    await expect(page.getByRole('button', { name: 'Loading Skills…', exact: true })).toHaveCount(0);
  }
  await expectedIds(
    page,
    Array.from({ length: 100 }, (_, i) => `candidate-${i + 70}`),
  );
  expect(catalogRequests.every((url) => url.searchParams.get('page') === '0')).toBe(true);
  expect(errors).toEqual([]);
});

test('outage fails selected filters closed and explicit retry recovers without changing default catalog', async ({
  page,
}) => {
  const fixture = await prepare(page);
  fixture.setUnavailable(true);
  await page.goto('plugins/?tab=skills&skillRemarks=none');
  await expect(page.locator('.skills-catalog')).toHaveAttribute('aria-busy', 'false');
  await expect(cards(page)).toHaveCount(0);
  await expect(
    page.getByText('Some scan reports could not be verified. Those Skills were excluded.'),
  ).toBeVisible();
  fixture.setUnavailable(false);
  await page.getByRole('button', { name: 'Try again', exact: true }).click();
  await expectedIds(page, ['clean']);
  await openFilters(page);
  await page.getByRole('button', { name: 'Reset filters' }).click();
  await closeFilters(page);
  await expectedIds(page, allNames);
  expect(fixture.errors).toEqual([]);
});

test('legacy fallback is explicitly unavailable to selected filters and old all links remain unfiltered', async ({
  page,
}) => {
  const errors = await offline(page);
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) =>
    route.fulfill({
      json: {
        items: [skill('unscanned')],
        page: 0,
        hasMore: false,
        view: 'popular',
        cache: { status: 'fresh', fetchedAt: evidence.generated_at },
      },
    }),
  );
  await page.goto('plugins/?tab=skills&skillScan=scanned');
  await expect(page.locator('.skills-catalog')).toHaveAttribute('aria-busy', 'false');
  await expect(cards(page)).toHaveCount(0);
  await expect(
    page.getByText('Some scan reports could not be verified. Those Skills were excluded.'),
  ).toBeVisible();
  await page.goto('plugins/?tab=skills&skillScan=all');
  await expectedIds(page, ['unscanned']);
  expect(errors).toEqual([]);
});

for (const width of [1280, 320]) {
  test(`Filters dialog traps keyboard focus, restores trigger and fits ${width}px`, async ({
    page,
  }) => {
    const { errors } = await prepare(page);
    await page.setViewportSize({ width, height: 800 });
    await page.goto('plugins/?tab=skills');
    const trigger = page.getByRole('button', { name: /^Filters/ });
    await trigger.focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Filter Skills', exact: true });
    await expect(dialog).toBeVisible();
    for (let i = 0; i < 8; i++) {
      await page.keyboard.press('Tab');
      expect(await dialog.evaluate((node) => node.contains(document.activeElement))).toBe(true);
    }
    const checkbox = dialog.getByRole('checkbox', { name: 'No remarks', exact: true });
    await checkbox.focus();
    await page.keyboard.press('Space');
    await expect(checkbox).toBeChecked();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    const screenshots = process.env.UAP_LANDING_SCREENSHOT_DIR;
    if (screenshots) {
      mkdirSync(screenshots, { recursive: true });
      await dialog.screenshot({ path: `${screenshots}/skills-filters-${width}.png` });
    }
    await closeFilters(page);
    await expect(trigger).toBeFocused();
    expect(errors).toEqual([]);
  });
}

test('history restores independent filters and Arabic dialog fits the viewport', async ({
  page,
}) => {
  const { errors } = await prepare(page);
  await page.goto('plugins/?tab=skills&skillScan=scanned');
  await expectedIds(page, ['clean', 'dirty', 'partial']);
  await navigate(page, '/plugins/', 'en', '?tab=skills&skillRemarks=none');
  await expectedIds(page, ['clean']);
  await page.goBack();
  await expectedIds(page, ['clean', 'dirty', 'partial']);
  await openFilters(page);
  await expect(
    page.getByRole('checkbox', { name: 'Scanned by SkillSpector', exact: true }),
  ).toBeChecked();
  await expect(page.getByRole('checkbox', { name: 'No remarks', exact: true })).not.toBeChecked();
  await closeFilters(page);
  await page.goForward();
  await expectedIds(page, ['clean']);
  await page.setViewportSize({ width: 320, height: 800 });
  await navigate(page, '/plugins/', 'ar', '?tab=skills');
  await page.getByRole('button', { name: 'عوامل التصفية', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  const box = await dialog.boundingBox();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(320);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(errors).toEqual([]);
});

test('late verified scan replies cannot repopulate an obsolete filtered search', async ({ page }) => {
  const { errors } = await prepare(page);
  let release!: () => void;
  let started!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  const requested = new Promise<void>(resolve => { started = resolve; });
  await page.route('**/reports/skillspector/v3/shards/**', async route => {
    started();
    await gate;
    return route.fallback();
  });
  await page.goto('plugins/?tab=skills&skillScan=scanned');
  await requested;
  await openFilters(page);
  await page.getByRole('checkbox', { name: 'Scanned by SkillSpector', exact: true }).uncheck();
  await closeFilters(page);
  await expectedIds(page, allNames);
  const reply = page.waitForResponse(response => response.url().includes('/v3/shards/'));
  release();
  await reply;
  await expect(page.locator('.skills-catalog')).toHaveAttribute('aria-busy', 'false');
  await expectedIds(page, allNames);
  expect(new URL(page.url()).searchParams.has('skillScan')).toBe(false);
  expect(errors).toEqual([]);
});
