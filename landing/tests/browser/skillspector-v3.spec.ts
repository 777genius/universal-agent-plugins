import { createHash } from 'node:crypto';
import { readFileSync, mkdirSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { navigate, offline, projections, registryFixture } from './i18n-state.helpers';
import type { SkillSpectorManifestV3 } from '../../types/skillSpector';
const fixtureRoot = new URL('../fixtures/skillspector-v3/', import.meta.url);
const fixture = JSON.parse(
  readFileSync(new URL('index.json', fixtureRoot), 'utf8'),
) as SkillSpectorManifestV3;
const producerShard = JSON.parse(
  readFileSync(new URL(fixture.shards[0]!.path, fixtureRoot), 'utf8'),
);
async function setup(
  page: Page,
  mode: 'mixed' | 'stale' | 'unavailable' | 'invalid' | 'unsupported' | 'selected' = 'mixed',
) {
  const errors = await offline(page);
  const index = registryFixture();
  const plugin = index.plugins[0]!;
  plugin.components = ['skills'];
  const manifest = structuredClone(fixture);
  const shard = structuredClone(producerShard);
  Object.assign(plugin.source, {
    repository: fixture.shards[0]!.repository,
    path: fixture.plugin_coverage[0]!.plugin_path,
    revision: producerShard.records[0].source.revision,
  });
  if (mode === 'stale') plugin.source.revision = 'f'.repeat(40);
  for (const distribution of plugin.distributions) {
    Object.assign(distribution.source, plugin.source);
    for (const release of distribution.releases) Object.assign(release.source, plugin.source);
  }
  shard.repository = plugin.source.repository;
  const body = JSON.stringify(shard);
  const hash = createHash('sha256').update(body).digest('hex');
  manifest.shards = [
    {
      repository: plugin.source.repository,
      path: `shards/${hash}.json`,
      digest: `sha256:${hash}`,
      bytes: Buffer.byteLength(body),
      record_count: shard.records.length,
    },
  ];
  if (mode === 'selected') manifest.plugin_states = [];
  if (mode === 'unsupported') {
    manifest.shards = [];
    manifest.plugin_coverage = [];
    manifest.coverage = {
      selected: 0,
      reported: 0,
      pending: 0,
      complete: 0,
      partial: 0,
      error: 0,
      stale: 0,
    };
    manifest.plugin_states = [
      {
        id: plugin.name,
        repository: plugin.source.repository,
        plugin_path: plugin.source.path,
        revision: plugin.source.revision,
        state: 'not_applicable',
        reason: 'no_skill_files',
        skills_total: 0,
        selected_ids: [],
      },
    ];
  }
  await page.route('**/reports/skillspector/v3/**', (route) => {
    if (mode === 'unavailable') return route.fulfill({ status: 503 });
    if (mode === 'invalid')
      return route.fulfill({
        json: { ...manifest, coverage: { ...manifest.coverage, complete: 13 } },
      });
    return route.request().url().endsWith('index.json')
      ? route.fulfill({ json: manifest })
      : route.fulfill({ body, contentType: 'application/json' });
  });
  await projections(page, index);
  await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
  return {
    assessment: page.locator('.plugin-card[data-install-source="i18n-state-00"] .skillspector'),
    errors,
    plugin,
  };
}
for (const width of [1280, 390]) {
  test(`v3 reports 3 of 13 fully checked and puts review context before technical details (${width})`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const { assessment, errors } = await setup(page);
    await expect(assessment).toHaveClass(/skillspector--incomplete/);
    await expect(assessment.locator(':scope > summary')).toContainText(
      '3 of 13 skills fully checked',
    );
    await expect(assessment.locator(':scope > summary')).not.toContainText('Findings:');
    await assessment.locator(':scope > summary').click();
    await expect(assessment.locator('.skillspector__advice')).toContainText('before installing');
    const order = await assessment
      .locator('.skillspector__details > section, .skillspector__details > details')
      .evaluateAll((elements) => elements.map((element) => element.className));
    expect(order).toEqual([
      'skillspector__advice',
      'skillspector__signals',
      'skillspector__coverage',
      'skillspector__technical',
    ]);
    await assessment.locator('.skillspector__record > summary').first().click();
    const signal = assessment.locator('.skillspector__findings > li').first();
    await expect(signal).toContainText('Unreviewed automated signal');
    await expect(signal).toContainText('Scanner confidence:');
    await expect(signal.locator('a').first()).toHaveAttribute(
      'href',
      /github.com\/.*\/blob\/[a-f0-9]{40}\//,
    );
    await expect(assessment.locator('.skillspector__coverage')).toContainText(
      /static_parse_limit|reference_missing|analyzer_incomplete/,
    );
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    const mainSkill = assessment.locator('.skillspector__record').nth(1);
    await mainSkill.locator(':scope > summary').click();
    const classified = mainSkill
      .locator('.skillspector__findings > li')
      .filter({ hasText: 'Registry classification: false positive' });
    await expect(classified).toHaveCount(2);
    await expect(classified.first()).toContainText('E4 · HIGH');
    await expect(classified.first()).toContainText('not a human security audit');
    await expect(classified.first()).toContainText('static asset upload session');
    await assessment.locator('.skillspector__details').evaluate((element) => {
      element.scrollTop = 0;
    });
    const screenshots = process.env.UAP_LANDING_SCREENSHOT_DIR;
    if (screenshots) {
      mkdirSync(screenshots, { recursive: true });
      await assessment.screenshot({ path: `${screenshots}/report-${width}.png` });
    }
    const badge = page.locator(
      '.plugin-card[data-install-source="i18n-state-00"] .plugin-card__ribbon',
    );
    await badge.focus();
    await expect(
      page.locator('.app-tooltip').filter({ hasText: 'The listing and metadata were reviewed.' }),
    ).toBeVisible();
    expect(
      await page
        .locator('.app-tooltip')
        .filter({ hasText: 'The listing and metadata were reviewed.' })
        .evaluate((element) => Number(getComputedStyle(element).zIndex)),
    ).toBeGreaterThan(1000);
    if (screenshots)
      await page.screenshot({ path: `${screenshots}/metadata-tooltip-${width}.png` });
    expect(errors).toEqual([]);
  });
}
for (const mode of ['stale', 'unavailable', 'invalid', 'unsupported'] as const) {
  test(`v3 keeps ${mode} explicit`, async ({ page }) => {
    const { assessment, errors } = await setup(page, mode);
    await expect(assessment).toHaveClass(
      new RegExp(`skillspector--${mode === 'invalid' ? 'unavailable' : mode}`),
    );
    await expect(assessment.locator(':scope > summary')).not.toContainText('No static findings');
    if (mode === 'stale')
      await expect(assessment.locator(':scope > summary')).toContainText(
        '0 of 13 selected skills fully checked',
      );
    await assessment.locator(':scope > summary').click();
    if (mode === 'unavailable' || mode === 'invalid')
      await expect(assessment).toContainText('could not be loaded');
    expect(errors).toEqual([]);
  });
}

test('v3 detail page opens install advice before installation and explains the metadata badge', async ({
  page,
}) => {
  const { plugin, errors } = await setup(page);
  await navigate(page, `/plugins/${plugin.name}/`, 'en');
  const assessment = page.locator('.plugin-detail .skillspector');
  await expect(assessment).toHaveAttribute('open', '');
  await expect(assessment.locator(':scope > summary')).toContainText(
    '3 of 13 skills fully checked',
  );
  await expect(assessment.locator('.skillspector__advice')).toContainText('Before installing');
  expect(
    await assessment.evaluate((element) =>
      Boolean(
        element.compareDocumentPosition(document.querySelector('#plugin-install')!) &
        Node.DOCUMENT_POSITION_FOLLOWING,
      ),
    ),
  ).toBe(true);
  await expect(page.locator('#plugin-install')).toContainText('i18n-state-00');
  await page.locator('.plugin-detail__status').focus();
  await expect(
    page.locator('.app-tooltip').filter({ hasText: 'The listing and metadata were reviewed.' }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});

test('v3 distinguishes selected roots from known complete plugin inventory', async ({ page }) => {
  const { assessment, errors } = await setup(page, 'selected');
  await expect(assessment.locator(':scope > summary')).toContainText(
    '3 of 13 selected skills fully checked',
  );
  expect(errors).toEqual([]);
});

test('v3 Russian wording identifies the listing review and exact known skill coverage', async ({
  page,
}) => {
  const { errors } = await setup(page);
  await navigate(page, '/plugins/', 'ru', '?q=i18n-state-00');
  const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
  await expect(card.locator('.skillspector > summary')).toContainText(
    'Полностью проверено 3 из 13 скиллов',
  );
  await expect(card.locator('.plugin-card__ribbon')).toHaveText('Карточка проверена');
  await card.locator('.plugin-card__ribbon').focus();
  await expect(
    page.locator('.app-tooltip').filter({ hasText: 'Проверены карточка и метаданные.' }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});

test('known MCP-only metadata shows not applicable on card and detail without report requests', async ({
  page,
}) => {
  const errors = await offline(page);
  const requests: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/reports/skillspector/')) requests.push(request.url());
  });
  const index = registryFixture();
  const plugin = index.plugins[0]!;
  index.plugins = [plugin];
  plugin.components = ['mcp'];
  for (const distribution of plugin.distributions) {
    distribution.components = ['mcp'];
    for (const release of distribution.releases) release.components = ['mcp'];
  }
  await projections(page, index);
  await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
  const assessment = page.locator(
    '.plugin-card[data-install-source="i18n-state-00"] .skillspector',
  );
  await expect(assessment).toHaveClass(/skillspector--unsupported/);
  await assessment.locator(':scope > summary').click();
  await expect(assessment).toContainText('No supported Skill files');
  await navigate(page, `/plugins/${plugin.name}/`, 'en');
  await expect(page.locator('.plugin-detail .skillspector')).toHaveClass(
    /skillspector--unsupported/,
  );
  await expect(page.locator('.plugin-detail .skillspector')).toContainText(
    'No supported Skill files',
  );
  expect(requests).toEqual([]);
  expect(errors).toEqual([]);
});
