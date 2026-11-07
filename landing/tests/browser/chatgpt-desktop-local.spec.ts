import { expect, test } from '@playwright/test';
import { navigate, offline, projections, registryFixture } from './i18n-state.helpers';

function desktopFixture(withCodex: boolean) {
  const index = registryFixture();
  const plugin = index.plugins[0]!;
  index.plugins = [plugin];
  const distribution = plugin.distributions[0]!;
  distribution.kind = 'community';
  distribution.status = 'active';
  distribution.selectable = true;
  const release = distribution.releases[0]!;
  release.release_status = 'active';
  release.meets_minimum_capabilities = true;
  release.blocking_clients = [];
  release.targets = withCodex
    ? [{ client: 'codex', scopes: ['user'], delivery: 'managed', authentication: 'not_required' }]
    : [{ client: 'cursor', scopes: ['user'], delivery: 'managed', authentication: 'not_required' }];
  distribution.targets = release.targets;
  distribution.releases = [release];
  distribution.compatible_clients = release.targets.map((target) => target.client);
  plugin.installable = true;
  return index;
}

const desktopCommand = 'npx -y universal-agent-plugins add i18n-state-00 --target chatgpt-desktop';

for (const width of [1440, 390]) {
  test('ChatGPT Desktop is a distinct selectable target with one copyable command at ' + width + 'px', async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    await projections(page, desktopFixture(true));
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
    await card.locator('.plugin-card__install-toggle').click();
    await expect(card.locator('.desktop-guide')).toHaveCount(0);
    await card.locator('.app-multiselect__trigger').click();
    await expect(page.getByRole('checkbox', { name: /^ChatGPT Web/ })).toBeDisabled();
    await page.getByRole('checkbox', { name: /^ChatGPT Desktop/ }).click();
    await page.keyboard.press('Escape');
    await expect(card.locator('.plugin-card__install .command-snippet code')).toHaveText(desktopCommand);
    const guide = card.locator('.desktop-guide');
    await expect(guide).toBeVisible();
    await guide.locator('summary').click();
    await expect(guide).toContainText('does not prove Desktop runtime');
    await expect(guide).toContainText('ChatGPT web connection');
    await expect(guide.locator('.command-snippet')).toHaveCount(0);

    await card.locator('.plugin-card__install .command-snippet button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(desktopCommand);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(errors).toEqual([]);
  });
}

test('Desktop and Web are both disabled when neither signed target is compatible', async ({ page }) => {
  const errors = await offline(page);
  await projections(page, desktopFixture(false));
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
  await card.locator('.plugin-card__install-toggle').click();
  await card.locator('.app-multiselect__trigger').click();
  await expect(page.getByRole('checkbox', { name: /^ChatGPT Desktop/ })).toBeDisabled();
  await expect(page.getByRole('checkbox', { name: /^ChatGPT Web/ })).toBeDisabled();
  await page.keyboard.press('Escape');
  await expect(card.locator('.desktop-guide')).toHaveCount(0);

  await card.locator('.plugin-card__title-link').click();
  await expect(page.locator('.install-panel .desktop-guide')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('detail page uses the same Desktop target in both themes', async ({ page }) => {
  const errors = await offline(page);
  await projections(page, desktopFixture(true));
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  await page.locator('.plugin-card[data-install-source="i18n-state-00"] .plugin-card__title-link').first().click();
  const panel = page.locator('.install-panel');
  await expect(panel.locator('.desktop-guide')).toHaveCount(0);
  await panel.locator('.app-multiselect__trigger').click();
  await page.getByRole('checkbox', { name: /^ChatGPT Desktop/ }).click();
  await page.keyboard.press('Escape');
  const guide = panel.locator('.desktop-guide');
  await expect(guide).toBeVisible();
  await expect(panel.locator('.command-snippet code')).toHaveText(desktopCommand);
  await page.getByRole('button', { name: 'Light', exact: true }).click();
  await expect(guide).toHaveCSS('color', 'rgb(22, 25, 43)');
  await page.getByRole('button', { name: 'Dark', exact: true }).click();
  await expect(guide).toHaveCSS('color', 'rgb(224, 230, 255)');
  expect(errors).toEqual([]);
});
