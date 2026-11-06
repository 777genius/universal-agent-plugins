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

for (const width of [1440, 390]) {
  test('ChatGPT Desktop guide copies the Codex local command at ' + width + 'px', async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    await projections(page, desktopFixture(true));
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
    await card.locator('.plugin-card__install-toggle').click();
    const guide = card.locator('.desktop-guide');
    await expect(guide).toBeVisible();
    await expect(guide.locator('summary')).toHaveText('ChatGPT Desktop (local)');
    await expect(guide.locator('.command-snippet')).toBeHidden();
    await guide.locator('summary').click();
    await expect(guide.locator('.command-snippet code')).toHaveText(
      'npx -y universal-agent-plugins add i18n-state-00 --target codex',
    );
    expect(await guide.locator('.command-snippet code').textContent()).not.toContain('\n');
    await expect(guide).toContainText('does not prove Desktop runtime');
    await expect(guide).toContainText('ChatGPT web connection');
    await guide.locator('.command-snippet button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(
      'npx -y universal-agent-plugins add i18n-state-00 --target codex',
    );
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(errors).toEqual([]);
  });
}

test('ChatGPT Desktop help is absent without active Codex delivery, while web ChatGPT stays gated', async ({ page }) => {
  const errors = await offline(page);
  await projections(page, desktopFixture(false));
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
  await card.locator('.plugin-card__install-toggle').click();
  await expect(card.locator('.desktop-guide')).toHaveCount(0);
  await card.locator('.app-multiselect__trigger').click();
  await expect(page.getByRole('checkbox', { name: /^ChatGPT(?:\s|$)/ })).toBeDisabled();
  await page.keyboard.press('Escape');

  await card.locator('.plugin-card__title-link').click();
  await expect(page.locator('.install-panel .desktop-guide')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('standalone detail offers the same local Desktop command in both themes', async ({ page }) => {
  const errors = await offline(page);
  await projections(page, desktopFixture(true));
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  await page.locator('.plugin-card[data-install-source="i18n-state-00"] .plugin-card__title-link').first().click();
  const guide = page.locator('.install-panel .desktop-guide');
  await expect(guide).toBeVisible();
  await guide.locator('summary').click();
  await expect(guide.locator('.command-snippet code')).toHaveText(
    'npx -y universal-agent-plugins add i18n-state-00 --target codex',
  );
  await page.getByRole('button', { name: 'Light', exact: true }).click();
  await expect(guide).toHaveCSS('color', 'rgb(22, 25, 43)');
  await page.getByRole('button', { name: 'Dark', exact: true }).click();
  await expect(guide).toHaveCSS('color', 'rgb(224, 230, 255)');
  expect(errors).toEqual([]);
});
