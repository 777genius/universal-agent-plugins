import { expect, test, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { loadRegistryIndex } from '../../build/load-registry';
import { catalogVisiblePlugins } from '../../utils/filter';
import type { SkillSpectorRecord, SkillSpectorReport } from '../../types/skillSpector';
import {
  deferred,
  navigate,
  offline as offlineCatalog,
  projections,
  registryFixture,
  wording,
} from './i18n-state.helpers';

/** These fixtures explicitly exercise the retained v1 feed through 404 fallback. */
async function offline(page: Page) {
  const errors = await offlineCatalog(page);
  await page.route('**/reports/skillspector/v2/index.json', (route) =>
    route.fulfill({ status: 404 }),
  );
  return errors;
}

const digest = `sha256:${'1'.repeat(64)}`;
const profileDigest = 'sha256:ddd91212d95a717ab6c067d99e9f32a64b8dd55589bdf47a36695a85320187d5';
const scannerRevision = 'f3b25697d76256998ca9d42c198d256258d4c035';
const checked = () => new Date(Date.now() - 60_000).toISOString().replace(/\.\d{3}Z$/, 'Z');

function skillRegistryFixture() {
  const index = registryFixture();
  const plugin = index.plugins[0]!;
  plugin.components = ['skills'];
  for (const distribution of plugin.distributions) {
    distribution.components = ['skills'];
    for (const release of distribution.releases) release.components = ['skills'];
  }
  return index;
}

for (const mobile of [false, true]) {
  test.describe(mobile ? 'SkillSpector on mobile' : 'SkillSpector on desktop', () => {
    test.use({
      viewport: mobile ? { width: 390, height: 844 } : { width: 1280, height: 900 },
      hasTouch: mobile,
    });
    test('shares historical reports between catalogs, supports keyboard details and never marks a failed child clean', async ({
      page,
    }) => {
      const errors = await offline(page);
      const index = skillRegistryFixture();
      const source = index.plugins[0]!.source;
      const record: SkillSpectorRecord = {
        id: 'selected-plugin-skill',
        kind: 'plugin_skill',
        plugin_path: source.path,
        skill_name: 'example',
        source: {
          repository: source.repository,
          revision: source.revision!,
          path: `${source.path}/skills/example`,
          content_digest: digest,
        },
        scanned_at: checked(),
        status: 'complete',
        finding_count: 0,
        findings_truncated: false,
        findings: [],
        errors: [],
        report_digest: digest,
      };
      const failed: SkillSpectorRecord = {
        ...record,
        id: 'failed-child',
        status: 'error',
        source: { ...record.source, revision: 'b'.repeat(40) },
        scanned_at: null,
        errors: [{ code: 'scan_failed', message: 'A selected child check failed.' }],
      };
      const standalone: SkillSpectorRecord = {
        ...record,
        id: 'historical-standalone',
        kind: 'standalone_skill',
        plugin_path: null,
        skill_name: 'frontend-design',
        source: {
          repository: 'anthropics/skills',
          revision: 'a'.repeat(40),
          path: 'skills/frontend-design',
          content_digest: digest,
        },
      };
      const report: SkillSpectorReport = {
        schema_version: 1,
        generated_at: checked(),
        source_commit: scannerRevision,
        scope: 'selected_sources',
        scanner: { id: 'skillspector', version: '2.13.1', revision: scannerRevision },
        profile: { id: 'static-only-v1', digest: profileDigest },
        selection_digest: digest,
        records: [record, failed, standalone],
      };
      let requests = 0;
      await page.route('**/reports/skillspector/v1/index.json', (route) => {
        requests += 1;
        return route.fulfill({ json: report });
      });
      await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) =>
        route.fulfill({
          json: {
            items: [
              {
                id: 'skills-sh:anthropics/skills:frontend-design',
                providerId: 'skills-sh',
                providerUrl: 'https://skills.sh/anthropics/skills/frontend-design',
                title: 'frontend-design',
                skillNameHint: 'frontend-design',
                source: {
                  kind: 'github',
                  repository: 'anthropics/skills',
                  url: 'https://github.com/anthropics/skills',
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
        }),
      );
      await projections(page, index);
      await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
      const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
      const details = card.locator('.skillspector');
      await expect(details).toHaveClass(/skillspector--incomplete/);
      await expect(details.locator(':scope > summary')).toContainText('Coverage has limits');
      await details.locator(':scope > summary').focus();
      await page.keyboard.press('Enter');
      await expect(details).toHaveAttribute('open', '');
      await expect(details).toContainText(
        'Scope: Skill files in this plugin. Other plugin components are not checked by SkillSpector.',
      );
      await expect(details).toContainText(source.revision!);
      await expect(details).toContainText('A selected child check failed.');
      await expect(details).toContainText(
        'This report is historical or describes a different source revision or analysis policy.',
      );
      const sizes = await details.evaluate((element) => {
        const summary = element.querySelector('summary')!;
        return {
          summary: parseFloat(getComputedStyle(summary).fontSize),
          height: summary.getBoundingClientRect().height,
          body: parseFloat(
            getComputedStyle(element.querySelector('.skillspector__details')!).fontSize,
          ),
          code: parseFloat(getComputedStyle(element.querySelector('code')!).fontSize),
        };
      });
      expect(sizes.summary).toBeGreaterThanOrEqual(12);
      expect(sizes.body).toBeGreaterThanOrEqual(13);
      expect(sizes.code).toBeGreaterThanOrEqual(12);
      expect(sizes.height).toBeGreaterThanOrEqual(mobile ? 44 : 32);
      await details.locator('.skillspector__dialog-close').click();
      await expect(details).not.toHaveAttribute('open');
      await card.getByRole('button', { name: 'Install I18n State 00', exact: true }).click();
      await expect(card.locator('.plugin-card__install-panel pre')).toContainText(
        'npx -y universal-agent-plugins add i18n-state-00',
      );
      await navigate(page, '/plugins/', 'en', '?tab=skills');
      const skill = page.locator('.skill-card').first();
      await expect(skill.locator('.skillspector')).toHaveClass(/skillspector--stale/);
      await skill.locator('.skillspector > summary').click();
      await expect(skill).toContainText('Historical Skill report');
      await expect(skill).toContainText(
        'The current revision of this listing has not been verified.',
      );
      await expect(skill).toContainText('a'.repeat(40));
      // The native report dialog is modal; close it before testing the install action.
      await skill.locator('.skillspector__dialog-close').click();
      await expect(skill.locator('.skillspector__dialog')).toHaveCount(0);
      await expect(skill.locator('.skillspector > summary')).toBeFocused();
      await skill.getByRole('button', { name: 'Install frontend-design', exact: true }).click();
      await expect(skill.locator('.plugin-card__install-panel pre')).toContainText(
        'skills add https://github.com/anthropics/skills --skill frontend-design',
      );
      await navigate(page, '/plugins/', 'ru', '?tab=skills');
      await expect(page.locator('.skill-card .skillspector > summary')).toContainText(
        wording('ru')('registryUi.skillSpector.stale'),
      );
      expect(requests).toBe(1);
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
      ).toBe(true);
      expect(errors).toEqual([]);
    });
  });
}

for (const result of ['malformed', 'unavailable', 'unselected'] as const) {
  test(`${result} report distinguishes loading from unknown and unselected checks`, async ({
    page,
  }) => {
    const errors = await offline(page);
    const gate = deferred();
    let requests = 0;
    let recovered = false;
    await page.route('**/reports/skillspector/v1/index.json', async (route) => {
      requests += 1;
      await gate.promise;
      if (result === 'unavailable' && !recovered)
        return route.fulfill({ status: 503, body: 'Unavailable' });
      if (result === 'malformed')
        return route.fulfill({ json: { schema_version: 1, records: [] } });
      const report: SkillSpectorReport = JSON.parse(
        readFileSync(new URL('../fixtures/skillspector-report.json', import.meta.url), 'utf8'),
      );
      return route.fulfill({ json: { ...report, generated_at: checked(), records: [] } });
    });
    await projections(page, skillRegistryFixture());
    await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
    const details = card.locator('.skillspector');
    await expect(details.locator(':scope > summary')).toContainText('Loading report');
    await details.locator(':scope > summary').click();
    await expect(details).toContainText('Loading the published assessment…');
    await expect(details).not.toContainText('No published scan report matches this listing.');
    await expect(details.locator('.skillspector__details')).toHaveAttribute('aria-busy', 'true');
    await expect(
      card.getByRole('button', { name: 'Install I18n State 00', exact: true }),
    ).toBeEnabled();
    gate.release();
    if (result === 'unselected') {
      await expect(details.locator(':scope > summary')).toContainText('Not checked');
      await expect(details).toContainText('No published scan report matches this listing.');
    } else {
      await expect(details.locator(':scope > summary')).toContainText('Report unavailable');
      await expect(details).toContainText('This listing’s check status is unknown.');
      await expect(details).not.toContainText('No published scan report matches this listing.');
    }
    await expect(details.locator('.skillspector__details')).toHaveAttribute('aria-busy', 'false');
    await navigate(page, '/plugins/', 'ru', '?q=i18n-state-00');
    await expect(page.locator('.skillspector summary').first()).toContainText(
      wording('ru')(
        `registryUi.skillSpector.${result === 'unselected' ? 'notChecked' : 'unavailable'}`,
      ),
    );
    if (result === 'unavailable') {
      recovered = true;
      await page.locator('.skillspector summary').first().click();
      const retry = page.getByRole('button', {
        name: wording('ru')('registryUi.skillSpector.retryLoad'),
      });
      await expect(retry).toBeVisible();
      await retry.focus();
      await page.keyboard.press('Enter');
      await expect(page.locator('.skillspector summary').first()).toContainText(
        wording('ru')('registryUi.skillSpector.notChecked'),
      );
      await expect(retry).toHaveCount(0);
    } else {
      await expect(page.locator('.skillspector__retry')).toHaveCount(0);
    }
    expect(requests).toBe(result === 'unavailable' ? 3 : 1);
    expect(errors).toEqual([]);
  });
}

for (const width of [1280, 390]) {
  test(`temporary report failure recovers in the same open card on ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    let requests = 0;
    const valid: SkillSpectorReport = JSON.parse(
      readFileSync(new URL('../fixtures/skillspector-report.json', import.meta.url), 'utf8'),
    );
    await page.route('**/reports/skillspector/v1/index.json', (route) => {
      requests++;
      return requests <= 2
        ? route.fulfill({ status: 503, body: 'Unavailable' })
        : route.fulfill({ json: { ...valid, generated_at: checked(), records: [] } });
    });
    await projections(page, skillRegistryFixture());
    await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
    const details = card.locator('.skillspector');
    await expect(details.locator(':scope > summary')).toContainText('Report unavailable');
    await details.locator(':scope > summary').click();
    const retry = details.getByRole('button', { name: 'Try loading again' });
    await expect(retry).toBeVisible();
    await retry.click();
    await expect(details.locator(':scope > summary')).toContainText('Not checked');
    await expect(details).toContainText('No published scan report matches this listing.');
    await expect(retry).toHaveCount(0);
    expect(requests).toBe(3);
    expect(errors).toEqual([]);
  });
}

test('partial reports explain coverage and link the visible finding count to scanned files', async ({
  page,
}) => {
  const errors = await offline(page);
  const index = skillRegistryFixture();
  const source = index.plugins[0]!.source;
  const report: SkillSpectorReport = JSON.parse(
    readFileSync(new URL('../fixtures/skillspector-report.json', import.meta.url), 'utf8'),
  );
  const partial = report.records[1]!;
  partial.plugin_path = source.path;
  partial.source = {
    ...partial.source,
    repository: source.repository,
    revision: source.revision!,
    path: `${source.path}/skills/example`,
  };
  partial.scanned_at = checked();
  await page.route('**/reports/skillspector/v1/index.json', (route) =>
    route.fulfill({ json: { ...report, generated_at: checked(), records: [partial] } }),
  );
  await projections(page, index);
  await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
  const details = page.locator('.plugin-card[data-install-source="i18n-state-00"] .skillspector');
  await expect(details.locator(':scope > summary')).toContainText('Coverage has limits');
  await details.locator(':scope > summary').click();
  await expect(details).toContainText('static_parse_limit');
  await expect(details).toContainText('reference_missing');
  await expect(details).toContainText('analyzer_incomplete');
  await expect(details.locator('.skillspector__record')).toHaveCount(0);
  const firstFinding = details.locator('.skillspector__findings a').first();
  await expect(firstFinding).toHaveText('SKILL.md:50 ↗');
  await expect(firstFinding).toHaveAttribute(
    'href',
    `https://github.com/${source.repository}/blob/${source.revision}/${partial.source.path}/SKILL.md#L50`,
  );
  expect(
    await details.evaluate((element) => {
      const findings = element.querySelector('.skillspector__findings')!;
      return (
        [...element.querySelectorAll('p')]
          .find((p) =>
            p.textContent?.includes('The published report contains only the displayed groups.'),
          )!
          .compareDocumentPosition(findings) & Node.DOCUMENT_POSITION_FOLLOWING
      );
    }),
  ).toBeTruthy();
  expect(errors).toEqual([]);
});

test('real Directory metadata limits SkillSpector to listings with Skill files', async ({
  page,
}) => {
  const errors = await offline(page);
  const root = new URL('../../public/registry/schemas/1/', import.meta.url);
  const pointer = JSON.parse(readFileSync(new URL('latest.json', root), 'utf8')) as {
    snapshot_path: string;
  };
  const index = loadRegistryIndex(
    new URL(pointer.snapshot_path, root).pathname,
    'published_snapshot',
  );
  const report: SkillSpectorReport = JSON.parse(
    readFileSync(new URL('../fixtures/skillspector-report.json', import.meta.url), 'utf8'),
  );
  await page.route('**/reports/skillspector/v1/index.json', (route) =>
    route.fulfill({ json: { ...report, generated_at: checked(), records: [] } }),
  );
  await projections(page, index);
  const skillPlugins = index.plugins.filter((plugin) => plugin.components.includes('skills'));
  expect(skillPlugins.map((plugin) => plugin.name)).toEqual(
    expect.arrayContaining(['cloudflare', 'context7', 'agent-notifications']),
  );
  // Retired and owner-unlisted products intentionally stay out of the catalog.
  for (const plugin of catalogVisiblePlugins(skillPlugins)) {
    await navigate(page, '/plugins/', 'en', `?q=${plugin.name}`);
    await expect(
      page.locator(`.plugin-card[data-install-source="${plugin.install_source}"] .skillspector`),
    ).toBeVisible();
  }
  const mcpOnly = index.plugins.find(
    (plugin) => plugin.components.length === 1 && plugin.components[0] === 'mcp',
  )!;
  expect(mcpOnly).toBeTruthy();
  await navigate(page, '/plugins/', 'en', `?q=${mcpOnly.name}`);
  const mcpCard = page.locator(`.plugin-card[data-install-source="${mcpOnly.install_source}"]`);
  await expect(mcpCard).toBeVisible();
  await expect(mcpCard.locator('.skillspector')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('multiple selected skills stay collapsed and let users open a later skill directly', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const errors = await offline(page);
  const index = skillRegistryFixture();
  const source = index.plugins[0]!.source;
  const fixture: SkillSpectorReport = JSON.parse(
    readFileSync(new URL('../fixtures/skillspector-report.json', import.meta.url), 'utf8'),
  );
  const records: SkillSpectorRecord[] = Array.from({ length: 13 }, (_, i) => ({
    ...fixture.records[0]!,
    id: `selected-${i}`,
    plugin_path: source.path,
    skill_name: i === 11 ? 'cloudflare-wrangler' : `selected-skill-${i}`,
    source: {
      ...fixture.records[0]!.source,
      repository: source.repository,
      revision: source.revision!,
      path: `${source.path}/skills/selected-${i}`,
    },
    scanned_at: checked(),
  }));
  Object.assign(records[0]!, {
    status: 'partial',
    finding_count: 50,
    findings_truncated: true,
    findings: fixture.records[1]!.findings,
    errors: fixture.records[1]!.errors,
  });
  Object.assign(records[11]!, { finding_count: 1, findings: [fixture.records[1]!.findings[0]!] });
  Object.assign(records[12]!, {
    status: 'error',
    scanned_at: null,
    errors: [{ code: 'scan_failed', message: 'This selected skill could not be checked.' }],
  });
  await page.route('**/reports/skillspector/v1/index.json', (route) =>
    route.fulfill({ json: { ...fixture, generated_at: checked(), records } }),
  );
  await projections(page, index);
  await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
  const assessment = page.locator(
    '.plugin-card[data-install-source="i18n-state-00"] .skillspector',
  );
  await assessment.locator(':scope > summary').click();
  const children = assessment.locator('.skillspector__record');
  await expect(children).toHaveCount(13);
  await expect(assessment.locator('.skillspector__record[open]')).toHaveCount(0);
  const later = children.nth(11);
  await expect(later.locator('summary')).toContainText(
    'cloudflare-wrangler · Findings need review',
  );
  await later.locator('summary').focus();
  await page.keyboard.press('Enter');
  await expect(later).toHaveAttribute('open', '');
  await expect(children.first()).not.toHaveAttribute('open');
  await expect(later.locator('.skillspector__findings a')).toHaveAttribute(
    'href',
    `https://github.com/${source.repository}/blob/${source.revision}/${records[11]!.source.path}/SKILL.md#L50`,
  );
  const failed = children.last();
  await expect(failed.locator('summary')).toContainText('Could not check');
  await failed.locator('summary').click();
  await expect(failed).toContainText('This selected skill could not be checked.');
  expect(
    await failed.locator('summary').evaluate((element) => element.getBoundingClientRect().height),
  ).toBeGreaterThanOrEqual(44);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(errors).toEqual([]);
});
