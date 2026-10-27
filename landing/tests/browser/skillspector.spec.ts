import { expect, test } from '@playwright/test';
import type { SkillSpectorRecord, SkillSpectorReport } from '../../types/skillSpector';
import { navigate, offline, projections, registryFixture, wording } from './i18n-state.helpers';

const digest = `sha256:${'1'.repeat(64)}`;
const profileDigest = 'sha256:ddd91212d95a717ab6c067d99e9f32a64b8dd55589bdf47a36695a85320187d5';
const scannerRevision = 'f3b25697d76256998ca9d42c198d256258d4c035';
const checked = () => new Date(Date.now() - 60_000).toISOString().replace(/\.\d{3}Z$/, 'Z');

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
      const index = registryFixture();
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
      await expect(details).toHaveClass(/skillspector--partial/);
      await expect(details.locator('summary')).toContainText('Selected skills: Partial check');
      await details.locator('summary').focus();
      await page.keyboard.press('Enter');
      await expect(details).toHaveAttribute('open', '');
      await expect(details).toContainText('Selected skills in this plugin');
      await expect(details).toContainText(source.revision!);
      await expect(details).toContainText('A selected child check failed.');
      await card.getByRole('button', { name: 'Install I18n State 00', exact: true }).click();
      await expect(card.locator('pre')).toContainText(
        'npx -y universal-agent-plugins add i18n-state-00',
      );
      await navigate(page, '/plugins/', 'en', '?tab=skills');
      const skill = page.locator('.skill-card').first();
      await expect(skill.locator('.skillspector')).toHaveClass(/skillspector--historical/);
      await skill.locator('.skillspector summary').click();
      await expect(skill).toContainText('Historical Skill report');
      await expect(skill).toContainText(
        'The current revision of this listing has not been verified.',
      );
      await expect(skill).toContainText('a'.repeat(40));
      await skill.getByRole('button', { name: 'Install frontend-design', exact: true }).click();
      await expect(skill.locator('pre')).toContainText(
        'skills add https://github.com/anthropics/skills --skill frontend-design',
      );
      await navigate(page, '/plugins/', 'ru', '?tab=skills');
      await expect(page.locator('.skill-card .skillspector summary')).toContainText(
        wording('ru')('registryUi.skillSpector.noFindings'),
      );
      expect(requests).toBe(1);
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
      ).toBe(true);
      expect(errors).toEqual([]);
    });
  });
}

test('missing and malformed static reports leave listings not checked and installation available', async ({
  page,
}) => {
  await offline(page);
  await page.route('**/reports/skillspector/v1/index.json', (route) =>
    route.fulfill({ json: { schema_version: 1, records: [] } }),
  );
  await projections(page, registryFixture());
  await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
  const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
  await expect(card.locator('.skillspector')).toHaveClass(/skillspector--notChecked/);
  await card.locator('.skillspector summary').click();
  await expect(card).toContainText('No published assessment matches this listing.');
  await expect(
    card.getByRole('button', { name: 'Install I18n State 00', exact: true }),
  ).toBeEnabled();
});
