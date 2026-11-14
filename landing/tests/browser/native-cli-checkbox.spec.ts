import { expect, test } from '@playwright/test';
import { navigate, offline, projections, registryFixture } from './i18n-state.helpers';

const npxSkillCommand =
  'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design --scope project';
const nativeSkillCommand =
  'agentplugins skills add https://github.com/anthropics/skills --skill frontend-design --scope project';
const checkboxName = 'Use installed agentplugins CLI instead of npx';

test('native CLI checkbox changes Skills, plugin install and copy commands across navigation', async ({ page }) => {
  const errors = await offline(page);
  const index = registryFixture();
  index.plugins = [index.plugins[0]!];
  await projections(page, index);
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route =>
    route.fulfill({ json: {
      items: [{
        id: 'skills-sh:anthropics/skills:frontend-design',
        providerId: 'skills-sh',
        providerUrl: 'https://skills.sh/anthropics/skills/frontend-design',
        title: 'frontend-design',
        skillNameHint: 'frontend-design',
        source: { kind: 'github', repository: 'anthropics/skills',
          url: 'https://github.com/anthropics/skills' },
        metrics: { installs: 1234, window: 'all-time' },
        availability: 'unverified',
      }],
      page: 0, total: 1, hasMore: false, view: 'popular',
      cache: { status: 'fresh', fetchedAt: '2026-11-06T10:00:00Z' },
    } }),
  );

  await navigate(page, '/plugins/', 'en', '?tab=skills&skillScan=all');
  const skillCard = page.locator('.skills-catalog .skill-card').first();
  await skillCard.getByRole('button', { name: 'Install frontend-design' }).click();
  await skillCard.getByRole('button', { name: 'This project' }).click();
  const skillCode = skillCard.locator('.command-snippet--add code');
  await expect(skillCode).toHaveText(npxSkillCommand);
  await expect(skillCard).not.toContainText('The CLI checks the source and Skill before installation.');

  const checkbox = skillCard.getByRole('checkbox', { name: checkboxName });
  await expect(checkbox).not.toBeChecked();
  await checkbox.check();
  await expect(skillCode).toHaveText(nativeSkillCommand);
  await skillCard.getByRole('button', { name: 'Copy command' }).click();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe(nativeSkillCommand);

  await page.locator('.catalog-modes').getByRole('link', { name: 'Plugins', exact: true }).click();
  const plugin = page.locator('[data-install-source="i18n-state-00"]').first();
  await plugin.locator('.plugin-card__install-toggle').click();
  await expect(plugin.locator('.command-snippet--add code')).toHaveText(/^agentplugins add\s+i18n-state-00/);
  await plugin.getByRole('button', { name: 'Copy command' }).click();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toMatch(/^agentplugins add i18n-state-00/);

  await page.locator('.catalog-modes').getByRole('link', { name: 'Skills', exact: true }).click();
  const restoredCard = page.locator('.skills-catalog .skill-card').first();
  await expect(restoredCard.locator('.command-snippet--add code')).toHaveText(nativeSkillCommand);
  await restoredCard.getByRole('link', { name: 'frontend-design' }).click();
  const detail = page.locator('#skill-install');
  await expect(detail.locator('.command-snippet code')).toHaveText(nativeSkillCommand);
  await detail.getByRole('tab', { name: 'Prompt' }).click();
  await expect(detail.locator('.command-snippet code')).toContainText(nativeSkillCommand);

  await detail.getByRole('checkbox', { name: checkboxName }).uncheck();
  await expect(detail.locator('.command-snippet code')).toContainText(npxSkillCommand);
  await detail.getByRole('tab', { name: 'Command' }).click();
  await expect(detail.locator('.command-snippet code')).toHaveText(npxSkillCommand);
  await detail.getByRole('button', { name: 'Copy command' }).click();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe(npxSkillCommand);
  expect(errors).toEqual([]);
});
