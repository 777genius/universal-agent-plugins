import { createHash } from 'node:crypto';
import { readFileSync, mkdirSync } from 'node:fs';
import { expect, test, type Locator, type Page } from '@playwright/test';
import {
  deferred,
  hydrated,
  navigate,
  offline,
  projections,
  registryFixture,
  setSignedMirrorTestTime,
} from './i18n-state.helpers';
import type { SkillSpectorManifestV3 } from '../../types/skillSpector';
import type { DiscoverySnapshot } from '../../types/discovery';
import { discoveryPlugin } from '../../utils/discovery';
import { catalogVisiblePlugins, filterPlugins, groupCatalogPlugins } from '../../utils/filter';
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
async function screenshotBelowHeader(assessment: Locator, path: string) {
  await assessment.evaluate((element) => {
    const headerBottom = document.querySelector('header')?.getBoundingClientRect().bottom ?? 0;
    window.scrollBy({
      top: element.getBoundingClientRect().top - headerBottom - 16,
      behavior: 'instant',
    });
  });
  await assessment.screenshot({ path });
}
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
    | 'completeResolved'
    | 'pending'
    | 'unselected'
    | 'singleLimited'
    | 'pendingLimited'
    | 'n8nPending'
    | 'truncated'
    | 'unsupportedSource' = 'mixed',
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
  const standalone = mode === 'completeResolved' || mode === 'unsupportedSource';
  const complete = mode === 'completeExternal' || standalone;
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
    if (standalone) {
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
  if (mode === 'n8nPending') {
    shard.records = shard.records.filter((record: { status: string }) => record.status === 'partial').slice(0, 6);
    manifest.coverage = {
      selected: 15,
      reported: 6,
      complete: 0,
      partial: 6,
      error: 0,
      pending: 9,
      stale: 0,
    };
    manifest.plugin_coverage[0] = { ...manifest.plugin_coverage[0]!, ...manifest.coverage };
    Object.assign(manifest.plugin_states[0]!, {
      skills_total: 15,
      selected_ids: [
        ...shard.records.map((record: { id: string }) => record.id),
        ...Array.from({ length: 9 }, (_, index) => `plugin:fixture/n8n/waiting-${index + 1}`),
      ],
    });
  }
  if (mode === 'singleLimited' || mode === 'pendingLimited') {
    const selected = mode === 'singleLimited' ? 1 : 5;
    shard.records = [shard.records[0]];
    manifest.coverage = {
      selected,
      reported: 1,
      complete: 0,
      partial: 1,
      error: 0,
      pending: selected - 1,
      stale: 0,
    };
    manifest.plugin_coverage[0] = { ...manifest.plugin_coverage[0]!, ...manifest.coverage };
    Object.assign(manifest.plugin_states[0]!, {
      skills_total: selected,
      selected_ids: (manifest.plugin_states[0]!.selected_ids as string[]).slice(0, selected),
    });
  }
  if (mode === 'truncated') {
    shard.records = shard.records
      .filter((record: { status: string }) => record.status === 'complete')
      .slice(0, 2);
    for (const record of shard.records) {
      record.findings = [];
      record.finding_count = 0;
      record.raw_finding_count = 0;
      record.finding_group_count = 0;
      record.findings_truncated = false;
    }
    shard.records[0].finding_group_count = 1;
    shard.records[0].findings_truncated = true;
    manifest.coverage = {
      selected: 2,
      reported: 2,
      complete: 2,
      partial: 0,
      error: 0,
      pending: 0,
      stale: 0,
    };
    manifest.plugin_coverage[0] = { ...manifest.plugin_coverage[0]!, ...manifest.coverage };
    Object.assign(manifest.plugin_states[0]!, {
      skills_total: 2,
      selected_ids: shard.records.map((record: { id: string }) => record.id),
    });
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
  if (mode === 'pending' || mode === 'unselected') {
    manifest.shards = [];
    manifest.coverage = {
      selected: mode === 'pending' ? 1 : 0,
      reported: 0,
      pending: mode === 'pending' ? 1 : 0,
      complete: 0,
      partial: 0,
      error: 0,
      stale: 0,
    };
    manifest.plugin_coverage =
      mode === 'pending' ? [{ ...manifest.plugin_coverage[0]!, ...manifest.coverage }] : [];
    Object.assign(manifest.plugin_states[0]!, {
      skills_total: 1,
      selected_ids: mode === 'pending' ? ['not-yet-scanned'] : [],
    });
  }
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
  if (mode === 'unsupportedSource') {
    manifest.shards = [];
    manifest.coverage = {
      selected: 0,
      reported: 0,
      pending: 0,
      complete: 0,
      partial: 0,
      error: 0,
      stale: 0,
    };
    Object.assign(manifest.source_states[0]!, {
      state: 'unsupported',
      reason: 'unsupported_source_type',
      selected_ids: [],
    });
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
  await navigate(page, '/plugins/', 'en', standalone ? '?tab=skills' : '?q=i18n-state-00');
  return {
    assessment: page.locator(
      standalone
        ? '.skill-card .skillspector'
        : '.plugin-card[data-install-source="i18n-state-00"] .skillspector',
    ),
    errors,
    plugin,
    manifest,
    body,
  };
}
for (const width of [1280, 390]) {
  test(`v3 reports 13 checks and 10 incomplete and puts review context before technical details (${width})`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const { assessment, errors } = await setup(page);
    await expect(assessment).toHaveClass(/skillspector--incomplete/);
    await expect(assessment.locator(':scope > summary')).toContainText(
      'Skill scan · 13/13 checked · 10 not fully checked',
    );
    await expect(assessment.locator(':scope > summary')).not.toContainText('Findings:');
    await assessment.locator(':scope > summary').click();
    await expect(assessment.locator('.skillspector__advice')).toContainText(
      'Skills scanned: 13 of 13',
    );
    await expect(assessment.locator('.skillspector__outcomes dt')).toHaveText([
      'Scan complete',
      'Not fully checked',
    ]);
    await expect(assessment.locator('.skillspector__outcomes dd')).toHaveText(['3', '10']);
    await expect(assessment.locator('.skillspector__details')).toHaveCSS('font-size', '14px');
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
      await assessment.locator('.skillspector__dialog').screenshot({ path: `${screenshots}/report-${width}.png` });
    }
    const badge = page.locator(
      '.plugin-card[data-install-source="i18n-state-00"] .plugin-card__ribbon',
    );
    await assessment.locator('.skillspector__dialog-close').click();
    await badge.scrollIntoViewIfNeeded();
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
test('omitted groups with zero published signals never render a clean result or exhaustive zero counts', async ({
  page,
}) => {
  const { assessment, errors } = await setup(page, 'truncated');
  await expect(assessment.locator(':scope > summary')).toContainText('Findings need review');
  await assessment.locator(':scope > summary').click();
  await expect(assessment.locator('.skillspector__record-summary').first()).toContainText(
    'Findings need review',
  );
  await expect(assessment.locator('.skillspector__advice')).toContainText(
    'Review the reported signals',
  );
  await assessment.locator('.skillspector__record-summary').first().click();
  await expect(assessment.locator('.skillspector__signals')).not.toContainText(
    'No findings were reported',
  );
  await expect(assessment.locator('.skillspector__signals')).not.toContainText('content signals');
  await expect(
    assessment.getByText('Showing 0 of 1 finding groups.', { exact: true }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});
test('n8n known inventory distinguishes six incomplete checks from nine missing current results', async ({ page }) => {
  const { assessment, errors } = await setup(page, 'n8nPending');
  await expect(assessment.locator(':scope > summary')).toContainText(
    '6/15 checked · 9 awaiting · 6 not fully checked',
  );
  await assessment.locator(':scope > summary').click();
  const dialog = assessment.locator('.skillspector__dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('.skillspector__outcomes dd')).toHaveText(['6', '9']);
  await expect(dialog.locator('.skillspector__waiting li')).toHaveCount(9);
  await expect(dialog.locator('.skillspector__waiting')).toContainText('waiting-9');
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expect(assessment.locator(':scope > summary')).toBeFocused();
  await expect(assessment).not.toHaveAttribute('open');
  expect(errors).toEqual([]);
});

for (const mode of ['singleLimited', 'pendingLimited'] as const) {
  for (const width of [1280, 390]) {
    test(`v3 distinguishes executed partial scans from pending Skills (${mode}, ${width}px)`, async ({
      page,
    }) => {
      await page.setViewportSize({ width, height: 900 });
      const { assessment, errors } = await setup(page, mode);
      await expect(assessment).toHaveClass(/skillspector--incomplete/);
      await expect(assessment.locator(':scope > summary')).toContainText(
        mode === 'singleLimited'
          ? 'Skill scan · Not fully checked'
          : 'Skill scan · 1/5 checked · 4 awaiting · 1 not fully checked',
      );
      await expect(assessment.locator(':scope > summary')).not.toContainText('Incomplete check');
      await assessment.locator(':scope > summary').click();
      await expect(assessment.locator('.skillspector__advice')).toContainText(
        `Skills scanned: 1 of ${mode === 'singleLimited' ? 1 : 5}`,
      );
      await expect(assessment.locator('.skillspector__outcomes')).toContainText(
        'Not fully checked',
      );
      if (mode === 'pendingLimited') {
        await expect(assessment.locator('.skillspector__outcomes')).toContainText(
          'No report yet',
        );
        await expect(assessment.locator('.skillspector__outcomes dd')).toHaveText(['1', '4']);
      } else {
        await expect(assessment.locator('.skillspector__outcomes dd')).toHaveText(['1']);
      }
      expect(errors).toEqual([]);
    });
  }
}
for (const mode of ['pending', 'unselected'] as const) {
  test(`unscanned ${mode} source has a clear card and truthful automatic-check explanation`, async ({
    page,
  }) => {
    const { assessment, errors } = await setup(page, mode);
    const summary = assessment.locator(':scope > summary');
    await expect(summary).toContainText(/Skill scan · (Not checked|No current check)/);
    await expect(summary).not.toContainText(/SkillSpector|0 of 1|queued|running/i);
    await summary.focus();
    await page.keyboard.press('Enter');
    await expect(assessment.locator('.skillspector__advice')).toContainText(
      'no run time is confirmed',
    );
    await expect(assessment.locator('.skillspector__outcomes')).toContainText('No report yet');
    await expect(assessment.locator('.skillspector__outcomes dd')).toHaveText(['1']);
    await expect(assessment.locator('.skillspector__advice')).toContainText(
      'Other plugin components are not checked',
    );
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
    await assessment.locator(':scope > summary').click();
    if (mode === 'stale') {
      await expect(assessment.locator('.skillspector__advice')).toContainText(
        'Selected Skills scanned: 0 of 13',
      );
      await expect(assessment.locator('.skillspector__outcomes')).toContainText(
        'Older report only',
      );
      await expect(assessment.locator('.skillspector__record-summary').first()).toContainText(
        'Previous check',
      );
    }
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
    if (mode === 'unavailable')
      await expect(assessment.getByRole('button', { name: 'Try loading again' })).toBeVisible();
    if (mode === 'invalid')
      await expect(assessment.locator('.skillspector__retry')).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}


test('v3 report recovers from 503 in the same open card without losing coverage', async ({
  page,
}) => {
  const { assessment, errors, manifest, body } = await setup(page, 'unavailable');
  await expect(assessment.locator(':scope > summary')).toContainText('Report unavailable');
  await assessment.locator(':scope > summary').click();
  const retry = assessment.getByRole('button', { name: 'Try loading again' });
  await expect(retry).toBeVisible();
  const requests: string[] = [];
  await page.route('**/reports/skillspector/v3/**', (route) => {
    const isManifest = route.request().url().endsWith('/index.json');
    requests.push(isManifest ? 'manifest' : 'shard');
    return isManifest
      ? route.fulfill({ json: manifest })
      : route.fulfill({ body, contentType: 'application/json' });
  });
  await retry.click();
  await expect(assessment.locator(':scope > summary')).toContainText('13/13 checked · 10 not fully checked');
  await expect(assessment.locator('.skillspector__advice')).toContainText(
    'Skills scanned: 13 of 13',
  );
  await expect(retry).toHaveCount(0);
  expect(requests).toEqual(['manifest', 'shard']);
  expect(errors).toEqual([]);
});

test('v3 detail page opens install advice before installation and explains the metadata badge', async ({
  page,
}) => {
  const { plugin, errors } = await setup(page);
  await navigate(page, `/plugins/${plugin.name}/`, 'en');
  const assessment = page.locator('.plugin-detail .skillspector');
  await expect(assessment).not.toHaveAttribute('open');
  await expect(assessment.locator(':scope > summary')).toContainText(
    'Skill scan · 13/13 checked · 10 not fully checked',
  );
  await assessment.locator(':scope > summary').click();
  await expect(assessment.locator('.skillspector__dialog')).toBeVisible();
  await expect(assessment.locator('.skillspector__advice')).toContainText(
    'Skills scanned: 13 of 13',
  );
  await expect(assessment.locator('.skillspector__advice')).toContainText('Before installing');
  await expect(assessment.locator('.skillspector__advice')).toContainText(
    'Other plugin components are not checked by SkillSpector.',
  );
  expect(
    await assessment.evaluate((element) =>
      Boolean(
        element.compareDocumentPosition(document.querySelector('#plugin-install')!) &
        Node.DOCUMENT_POSITION_FOLLOWING,
      ),
    ),
  ).toBe(true);
  await expect(page.locator('#plugin-install')).toContainText('i18n-state-00');
  await assessment.locator('.skillspector__dialog-close').click();
  await page.locator('.plugin-detail__status').focus();
  await expect(
    page.locator('.app-tooltip').filter({ hasText: 'The listing and metadata were reviewed.' }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});

test('v3 distinguishes selected roots from known complete plugin inventory', async ({ page }) => {
  const { assessment, errors } = await setup(page, 'selected');
  // Every selected Skill has an executed partial or complete scan. The absent
  // package inventory changes the denominator's provenance, not the result.
  await expect(assessment.locator(':scope > summary')).toContainText(
    'Skill scan · Not fully checked',
  );
  await assessment.locator(':scope > summary').click();
  await expect(assessment.locator('.skillspector__advice')).toContainText(
    'Selected Skills scanned: 13 of 13',
  );
  expect(errors).toEqual([]);
});

for (const width of [1280, 390]) {
  test(`v3 Russian wording identifies the listing review, scan scope and known Skill coverage (${width})`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const { errors } = await setup(page);
    await navigate(page, '/plugins/', 'ru', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
    await expect(card.locator('.skillspector > summary')).toContainText(
      'Проверка скиллов · Проверено 13/13 · 10 не полностью',
    );
    await card.locator('.skillspector > summary').focus();
    await page.keyboard.press('Enter');
    await expect(card.locator('.skillspector__advice')).toContainText(
      'Скиллов проверено: 13 из 13',
    );
    await expect(card.locator('.skillspector__advice')).toContainText(
      'SkillSpector не проверяет другие компоненты плагина.',
    );
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    const screenshots = process.env.UAP_LANDING_SCREENSHOT_DIR;
    if (screenshots) {
      mkdirSync(screenshots, { recursive: true });
      await card.locator('.skillspector__dialog').screenshot({
        path: `${screenshots}/report-ru-${width}.png`,
      });
    }
    await card.locator('.skillspector__dialog-close').click();
    await expect(card.locator('.plugin-card__ribbon')).toHaveText('Проверено');
    await card.locator('.plugin-card__ribbon').focus();
    await expect(
      page.locator('.app-tooltip').filter({ hasText: 'Проверены карточка и метаданные.' }),
    ).toBeVisible();
    expect(errors).toEqual([]);
  });
}

for (const locale of ['en', 'ru'] as const) {
  for (const width of [1280, 390]) {
    test(`known MCP-only metadata omits card checks and explains detail scope without report requests (${locale}, ${width})`, async ({
      page,
    }) => {
      await page.setViewportSize({ width, height: 900 });
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
      await navigate(page, '/plugins/', locale, '?q=i18n-state-00');
      const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
      await expect(card).toBeVisible();
      await expect(card.locator('.skillspector')).toHaveCount(0);
      await expect(card).not.toContainText('SkillSpector');
      const title = card.getByRole('link', { name: plugin.display_name, exact: true });
      await title.focus();
      await expect(title).toBeFocused();
      await page.keyboard.press('Enter');
      const explanation = page.locator('.plugin-detail .skillspector--noSkills');
      await expect(explanation.locator('strong')).toHaveText(
        `${locale === 'en' ? 'Skill scan · No Skills to check' : 'Проверка скиллов · Нет скиллов для проверки'}`,
      );
      await expect(explanation).toContainText(
        locale === 'en' ? 'This plugin does not include Skills.' : 'В этом плагине нет скиллов.',
      );
      await expect(explanation).toContainText(
        locale === 'en'
          ? 'other plugin components are outside its scope.'
          : 'другие компоненты плагина не входят в область проверки.',
      );
      await expect(explanation.locator('summary, .skillspector__advice')).toHaveCount(0);
      await expect(page.locator('#plugin-install')).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
        true,
      );
      const screenshots = process.env.UAP_LANDING_SCREENSHOT_DIR;
      if (screenshots) {
        mkdirSync(screenshots, { recursive: true });
        await screenshotBelowHeader(explanation, `${screenshots}/no-skills-${locale}-${width}.png`);
      }
      expect(requests).toEqual([]);
      expect(errors).toEqual([]);
    });
  }
}

test('unsupported standalone sources retain their explanation and do not imply an empty completed check', async ({
  page,
}) => {
  const { assessment, errors } = await setup(page, 'unsupportedSource');
  await expect(assessment).toHaveClass(/skillspector--unsupported/);
  await expect(assessment.locator(':scope > summary')).toContainText('Not checked');
  await expect(assessment.locator('.skillspector__summary-coverage')).toHaveCount(0);
  await assessment.locator(':scope > summary').focus();
  await page.keyboard.press('Enter');
  await expect(assessment).toContainText(
    'This source type is not supported by static SkillSpector checks. Its files have not been checked.',
  );
  await expect(assessment.locator('.skillspector--noSkills')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('external standalone Skills remain unsupported without fetching a report or claiming there are no Skills', async ({
  page,
}) => {
  const errors = await offline(page);
  const requests: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/reports/skillspector/')) requests.push(request.url());
  });
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) =>
    route.fulfill({
      json: {
        items: [
          {
            id: 'external-source',
            providerId: 'skills-sh',
            providerUrl: 'https://skills.sh/example/external',
            title: 'External Skill',
            skillNameHint: 'external',
            source: { kind: 'external', url: 'https://skills.sh/example/external' },
            metrics: { installs: 1, window: 'all-time' },
            availability: 'unsupported-source',
          },
        ],
        page: 0,
        total: 1,
        hasMore: false,
        view: 'popular',
        cache: { status: 'fresh', fetchedAt: fixture.generated_at },
      },
    }),
  );
  await projections(page, registryFixture());
  await navigate(page, '/plugins/', 'en', '?tab=skills');
  const assessment = page.locator('.skill-card .skillspector');
  await expect(assessment).toHaveClass(/skillspector--unsupported/);
  await assessment.locator(':scope > summary').focus();
  await page.keyboard.press('Enter');
  await expect(assessment).toContainText(
    'This source type is not supported by static SkillSpector checks. Its files have not been checked.',
  );
  await expect(page.locator('.skillspector--noSkills')).toHaveCount(0);
  expect(requests).toEqual([]);
  expect(errors).toEqual([]);
});

test('community card navigation preserves the source-bound pending Skill report on detail', async ({
  page,
}) => {
  const errors = await offline(page);
  await setSignedMirrorTestTime(page);
  const pointer = await (await page.request.get('./discovery/latest.json')).json();
  const snapshot = (await (
    await page.request.get(`./discovery/${pointer.snapshot_path}`)
  ).json()) as DiscoverySnapshot;
  const cards = groupCatalogPlugins(
    filterPlugins(
      catalogVisiblePlugins(snapshot.records.map((record) => discoveryPlugin(record, snapshot))),
      { component: 'skills', trust: 'conformant_unreviewed' },
    ),
  );
  const plugin = snapshot.records.find(
    (record) => record.slug === cards[0]?.primary.install_source,
  )!;
  expect(plugin).toBeDefined();
  const coverage = {
    selected: plugin.components.skills,
    reported: 0,
    pending: plugin.components.skills,
    complete: 0,
    partial: 0,
    error: 0,
    stale: 0,
  };
  const manifest = {
    ...structuredClone(fixture),
    coverage,
    shards: [],
    source_states: [],
    repository_aliases: [],
    plugin_coverage: [
      { ...coverage, repository: plugin.repository, plugin_path: plugin.package_path },
    ],
    plugin_states: [
      {
        id: plugin.slug,
        repository: plugin.repository,
        plugin_path: plugin.package_path,
        revision: plugin.revision,
        state: 'resolved',
        reason: null,
        skills_total: plugin.components.skills,
        selected_ids: [],
      },
    ],
  };
  let requests = 0;
  await page.route('**/reports/skillspector/v3/index.json', (route) => {
    requests++;
    return route.fulfill({ json: manifest });
  });
  await page.goto('./plugins/?component=skills&trust=conformant_unreviewed');
  await hydrated(page);
  // The prerendered reviewed catalog precedes authenticated community-feed acquisition.
  await expect(page.locator('.catalog')).toHaveAttribute('data-discovery-state', /current|cached/, {
    timeout: 15_000,
  });
  await expect(page.locator('.plugin-card').first()).toHaveAttribute(
    'data-install-source',
    plugin.slug,
  );
  const card = page.locator(`.plugin-card[data-install-source=${JSON.stringify(plugin.slug)}]`);
  const cardReport = card.locator('.skillspector');
  await expect(cardReport).toHaveClass(/skillspector--pending/);
  const summary = await cardReport.locator(':scope > summary').textContent();
  const titleLink = card.locator('.plugin-card__title-link');
  const destination = new URL((await titleLink.getAttribute('href'))!, page.url());
  expect(destination.pathname).toMatch(/\/plugins\/community\/$/);
  expect(destination.searchParams.get('source')).toBe(plugin.slug);
  await titleLink.click();
  await page.waitForURL(destination.href, { timeout: 15_000 });
  // The detail page authenticates its own discovery projection before rendering this source.
  await expect(page.locator('.community-plugin-page .plugin-profile h1')).toHaveText(plugin.name, {
    timeout: 15_000,
  });
  const detailReport = page.locator('.community-plugin-page .skillspector');
  await expect(detailReport).toHaveClass(/skillspector--pending/);
  await expect(detailReport).not.toHaveAttribute('open');
  await expect(detailReport.locator(':scope > summary')).toHaveText(summary ?? '');
  await detailReport.locator(':scope > summary').click();
  await expect(detailReport.locator('.skillspector__dialog')).toBeVisible();
  await expect(detailReport).toContainText(
    'No current scan report has been published for these Skill files.',
  );
  await expect(detailReport).toContainText(
    'Other plugin components are not checked by SkillSpector.',
  );
  await expect(page.locator('.install-panel')).toBeVisible();
  expect(requests).toBe(1);
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
    await expect(report).not.toHaveAttribute('open');
    await report.locator(':scope > summary').click();
    await expect(report.locator('.skillspector__dialog')).toBeVisible();
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
  await expect(assessment.locator('.skillspector__record > summary').first()).toContainText(
    'Could not check',
  );
  await expect(assessment.locator('.skillspector__coverage')).toContainText(
    'Pinned source retrieval failed.',
  );
  await expect(assessment.locator('.skillspector__coverage')).not.toContainText(
    'Incomplete check',
  );
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
    await expect(assessment.locator(':scope > summary')).toContainText(
      mode === 'completeExternal' ? 'Scan complete' : 'Findings need review',
    );
    await assessment.locator(':scope > summary').click();
    await expect(assessment.locator('.skillspector__advice')).toContainText('1 of 1');
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
