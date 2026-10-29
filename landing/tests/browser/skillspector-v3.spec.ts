import { createHash } from 'node:crypto';
import { readFileSync, mkdirSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { deferred, navigate, offline, projections, registryFixture } from './i18n-state.helpers';
import type { SkillSpectorManifestV3 } from '../../types/skillSpector';
const fixtureRoot = new URL('../fixtures/skillspector-v3/', import.meta.url);
const fixture = JSON.parse(
  readFileSync(new URL('index.json', fixtureRoot), 'utf8'),
) as SkillSpectorManifestV3;
const referenceFixture = JSON.parse(
  readFileSync(
    new URL('../fixtures/skillspector-v3-reference-context.json', import.meta.url),
    'utf8',
  ),
);
const completeContextFixture = JSON.parse(
  readFileSync(
    new URL('../fixtures/skillspector-v3-complete-context.json', import.meta.url),
    'utf8',
  ),
);
const producerShard = JSON.parse(
  readFileSync(new URL(fixture.shards[0]!.path, fixtureRoot), 'utf8'),
);
async function setup(
  page: Page,
  mode:
    | 'mixed'
    | 'stale'
    | 'unavailable'
    | 'invalid'
    | 'unsupported'
    | 'selected'
    | 'acquisition'
    | 'references'
    | 'failed'
    | 'completeExternal'
    | 'completeResolved' = 'mixed',
  reportGate?: Promise<void>,
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
  if (mode === 'acquisition')
    shard.records[0].coverage.reasons.push({
      code: 'acquisition_transfer_limit',
      message: 'Transfer budget exhausted.',
      retryable: true,
    });
  if (mode === 'references') {
    const position = shard.records.findIndex(
      (record: { source: { path: string } }) =>
        record.source.path === referenceFixture.cloudflare.source.path,
    );
    const scheduled = shard.records[position];
    shard.records[position] = {
      ...referenceFixture.cloudflare,
      freshness: scheduled.freshness,
      target_revision: scheduled.target_revision,
      retry: {
        attempt_count: 1,
        disposition: 'scheduled',
        latest_status: 'partial',
        next_attempt_at: null,
        next_profile: 'static-extended-v3',
        reason: 'extended_profile_required',
      },
      attempt_history: [],
    };
  }
  if (mode === 'failed') {
    for (const record of shard.records) {
      record.status = 'error';
      record.execution.state = 'failed';
      record.execution.exit_code = 2;
      record.coverage = {
        status: 'not_started',
        fully_inspected_files: 0,
        partially_inspected_files: 0,
        entirely_uninspected_files: 0,
        reasons: [{ code: 'acquisition_failed', message: 'Pinned source retrieval failed.' }],
        analyzers: [],
        original_findings: [],
        grouping_complete: true,
      };
      record.findings = [];
      record.finding_count = 0;
      record.raw_finding_count = 0;
      record.finding_group_count = 0;
      record.findings_truncated = false;
      record.errors = [{ code: 'acquisition_failed', message: 'Pinned source retrieval failed.' }];
    }
    manifest.coverage = {
      selected: 13,
      reported: 13,
      pending: 0,
      complete: 0,
      partial: 0,
      error: 13,
      stale: 0,
    };
    manifest.plugin_coverage[0] = { ...manifest.plugin_coverage[0]!, ...manifest.coverage };
  }
  const complete = mode === 'completeExternal' || mode === 'completeResolved';
  if (complete) {
    const record = structuredClone(
      completeContextFixture.records[mode === 'completeExternal' ? 'external' : 'resolved'],
    );
    shard.records = [record];
    shard.repository = record.source.repository;
    manifest.generated_at = completeContextFixture.generated_at;
    manifest.coverage = {
      selected: 1,
      reported: 1,
      pending: 0,
      complete: 1,
      partial: 0,
      error: 0,
      stale: 0,
    };
    if (mode === 'completeResolved') {
      manifest.plugin_coverage = [];
      manifest.plugin_states = [];
      manifest.source_states = [
        {
          id: `${record.source.repository}/${record.skill_name}`,
          repository: record.source.repository,
          state: 'resolved',
          current: true,
          reason: null,
          revision: record.source.revision,
          path: record.source.path,
          selected_ids: [record.id],
          candidates: [],
        },
      ];
      await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) =>
        route.fulfill({
          json: {
            items: [
              {
                id: 'actual-complete-reference',
                providerId: 'skills-sh',
                providerUrl: `https://skills.sh/${record.source.repository}/${record.skill_name}`,
                title: record.skill_name,
                skillNameHint: record.skill_name,
                source: {
                  kind: 'github',
                  repository: record.source.repository,
                  url: `https://github.com/${record.source.repository}`,
                },
                metrics: { installs: 1, window: 'all-time' },
                availability: 'unverified',
              },
            ],
            page: 0,
            total: 1,
            hasMore: false,
            view: 'popular',
            cache: { status: 'fresh', fetchedAt: manifest.generated_at },
          },
        }),
      );
    } else {
      manifest.plugin_coverage[0] = { ...manifest.plugin_coverage[0]!, ...manifest.coverage };
      Object.assign(manifest.plugin_states[0]!, { skills_total: 1, selected_ids: [record.id] });
    }
  }
  const body = JSON.stringify(shard);
  const hash = createHash('sha256').update(body).digest('hex');
  manifest.shards = [
    {
      repository: shard.repository,
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
  await page.route('**/reports/skillspector/v3/**', async (route) => {
    if (reportGate) await reportGate;
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
  await navigate(
    page,
    '/plugins/',
    'en',
    mode === 'completeResolved' ? '?tab=skills' : '?q=i18n-state-00',
  );
  return {
    assessment: page.locator(
      mode === 'completeResolved'
        ? '.skill-card .skillspector'
        : '.plugin-card[data-install-source="i18n-state-00"] .skillspector',
    ),
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
    await expect(assessment.locator('.skillspector__details')).toHaveCSS('font-size', '13px');
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
    await expect(assessment.locator('.skillspector__coverage > strong')).toHaveText(
      'Coverage and reference context',
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
    if (mode === 'stale' || mode === 'unsupported') {
      await expect(assessment.locator('.skillspector__coverage')).not.toContainText(
        'Incomplete check',
      );
      await expect(assessment.locator('.skillspector__coverage > strong')).toHaveText(
        'Coverage and reference context',
      );
    } else await expect(assessment.locator('.skillspector__coverage')).toHaveCount(0);
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

test('source acquisition transfer limits are explained as incomplete coverage', async ({
  page,
}) => {
  const { assessment, errors } = await setup(page, 'acquisition');
  await expect(assessment).toHaveClass(/skillspector--incomplete/);
  await assessment.locator(':scope > summary').click();
  await expect(assessment.locator('.skillspector__coverage')).toContainText(
    'Source retrieval reached this scan profile’s transfer limit.',
  );
  await expect(assessment.locator('.skillspector__coverage')).toContainText(
    'Files that were not retrieved were not inspected.',
  );
  expect(errors).toEqual([]);
});

for (const outcome of ['mixed', 'unavailable'] as const) {
  test(`detail loading keeps coverage unknown before ${outcome === 'mixed' ? 'ready' : 'unavailable'} report`, async ({
    page,
  }) => {
    const gate = deferred();
    const { plugin, errors } = await setup(page, outcome, gate.promise);
    await navigate(page, `/plugins/${plugin.name}/`, 'en');
    const loading = page.locator('.plugin-detail .skillspector');
    await expect(loading).toHaveAttribute('aria-busy', 'true');
    await expect(loading).toContainText(
      'Coverage is unknown; review the source before installing.',
    );
    gate.release();
    const report = page.locator('.plugin-detail details.skillspector');
    await expect(report).toHaveClass(
      outcome === 'mixed' ? /skillspector--incomplete/ : /skillspector--unavailable/,
    );
    await expect(report).toHaveAttribute('open', '');
    await expect(report.locator('.skillspector__advice')).toContainText('Before installing');
    if (outcome === 'unavailable') await expect(report).toContainText('could not be loaded');
    expect(errors).toEqual([]);
  });
}

test('real producer missing references and ledger positions remain coverage context', async ({
  page,
}) => {
  const { assessment, errors } = await setup(page, 'references');
  await assessment.locator(':scope > summary').click();
  const coverage = assessment.locator('.skillspector__coverage');
  await expect(coverage).toContainText('node_modules/wrangler/config-schema.json');
  const context = coverage.locator('.skillspector__context-record');
  await context.locator(':scope > summary').click();
  const reference = context
    .locator('.skillspector__references > li')
    .filter({ hasText: 'node_modules/wrangler/config-schema.json' });
  await expect(reference).toContainText('SKILL.md:32:');
  await expect(reference.locator('a')).toHaveAttribute(
    'href',
    /blob\/4d7d8e075e1b0d899965496bacf2811be5d94091\/plugins\/cloudflare\/skills\/cloudflare\/SKILL.md#L32/,
  );
  await expect(reference).toContainText('Unresolved; see original text');
  await expect(reference).toContainText('missing');
  await expect(coverage).toContainText('not confirmed security findings');
  if (process.env.UAP_LANDING_SCREENSHOT_DIR)
    await context.screenshot({
      path: `${process.env.UAP_LANDING_SCREENSHOT_DIR}/coverage-references.png`,
    });
  await context.locator('.skillspector__diagnostics > summary').click();
  await expect(context.locator('.skillspector__diagnostics > summary')).toContainText('664');
  await expect(context.locator('.skillspector__diagnostics')).toContainText('reference_missing');
  expect(errors).toEqual([]);
});

test('published acquisition failure says could not check while keeping its report available', async ({
  page,
}) => {
  const { assessment, errors } = await setup(page, 'failed');
  await expect(assessment).toHaveClass(/skillspector--unavailable/);
  await expect(assessment.locator(':scope > summary')).toContainText('Could not check');
  await expect(assessment.locator(':scope > summary')).not.toContainText('Report unavailable');
  await assessment.locator(':scope > summary').click();
  await expect(assessment.locator('.skillspector__coverage')).toContainText(
    'Pinned source retrieval failed.',
  );
  await expect(assessment.locator('.skillspector__coverage')).not.toContainText('Incomplete check');
  await expect(assessment.locator('.skillspector__coverage > strong')).toHaveText(
    'Coverage and reference context',
  );
  await expect(assessment.locator('.skillspector__technical')).toBeVisible();
  expect(errors).toEqual([]);
});

for (const mode of ['completeExternal', 'completeResolved'] as const) {
  test(`complete actual ${mode} context does not imply an incomplete check`, async ({ page }) => {
    const { assessment, errors } = await setup(page, mode);
    await expect(assessment).not.toHaveClass(/skillspector--(?:loading|unavailable|incomplete)/);
    await expect(assessment.locator(':scope > summary')).toContainText('1 of 1');
    await assessment.locator(':scope > summary').click();
    const coverage = assessment.locator('.skillspector__coverage');
    await expect(coverage.locator(':scope > strong')).toHaveText('Coverage and reference context');
    await expect(coverage).not.toContainText('Incomplete check');
    await expect(coverage).not.toContainText('What was not inspected');
    await coverage.locator('.skillspector__context-record > summary').click();
    if (mode === 'completeResolved') {
      await expect(coverage).toContainText('resolved · analyzed');
      await expect(coverage.locator('.skillspector__references > li')).toHaveCount(2);
    } else {
      await expect(coverage).toContainText('rejected · out_of_scope');
      await expect(coverage.locator('.skillspector__references > li')).toHaveCount(17);
    }
    expect(errors).toEqual([]);
  });
}

test('opening a card while its report loads preserves the native disclosure and eventual evidence', async ({
  page,
}) => {
  const gate = deferred();
  const { assessment, errors } = await setup(page, 'mixed', gate.promise);
  await expect(assessment).toHaveClass(/skillspector--loading/);
  await assessment.locator(':scope > summary').focus();
  await page.keyboard.press('Enter');
  await expect(assessment).toHaveAttribute('open', '');
  await expect(assessment.locator('.skillspector__details')).toBeVisible();
  gate.release();
  await expect(assessment).toHaveClass(/skillspector--incomplete/);
  await expect(assessment).toHaveAttribute('open', '');
  await expect(assessment.locator('.skillspector__record')).toHaveCount(13);
  await expect(assessment.locator('.skillspector__coverage')).toContainText(
    /reference_missing|static_parse_limit/,
  );
  expect(errors).toEqual([]);
});
