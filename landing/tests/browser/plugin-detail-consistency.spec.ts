import { expect, test } from '@playwright/test';
import { navigate, offline, projections, registryFixture, targets } from './i18n-state.helpers';

function eligibleFixture() {
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
  release.targets = [
    { client: 'cursor', scopes: ['user'], delivery: 'managed', authentication: 'not_required' },
    { client: 'codex', scopes: ['user'], delivery: 'prepared', authentication: 'required' },
  ];
  distribution.releases = [release];
  distribution.targets = release.targets;
  distribution.compatible_clients = ['cursor', 'codex'];
  plugin.installable = true;
  // Deliberately stale aggregate: the detail page must resolve active release
  // policy just like the card's installer, not reuse legacy summary fields.
  plugin.client_support.clients = ['kiro', 'cursor', 'codex'];
  plugin.client_support.delivery.kiro = 'managed';
  plugin.client_support.delivery.cursor = 'prepared';
  plugin.client_support.delivery.codex = 'managed';
  return index;
}

for (const width of [1440, 390]) {
  test(`catalog and standalone plugin agree on eligible agents at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    await projections(page, eligibleFixture());
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('[data-install-source="i18n-state-00"]').first();
    await expect(card.locator('.plugin-card__source-label')).toHaveCount(0);
    await card.locator('.plugin-card__install-toggle').click();
    await expect(page.getByRole('dialog').locator('.install-dialog__footer .command-snippet code')).toHaveText(
      'npx -y universal-agent-plugins add i18n-state-00',
    );
    // Switching from terminal detection selects only one initial agent.
    await targets(page, card, ['Cursor']);
    await expect(card.locator('.plugin-card__source-label')).toContainText(
      'Agents: 1 automatic setup · 0 need follow-up',
    );
    await targets(page, card, ['Codex']);
    await expect(card.locator('.plugin-card__source-label')).toContainText(
      'Agents: 1 automatic setup · 1 need follow-up',
    );
    await expect(page.getByRole('dialog').locator('.install-dialog__footer .command-snippet code')).toContainText('--target codex,cursor');

    await page.keyboard.press('Escape');

    await card.locator('.plugin-card__title-link').click();
    const detail = page.locator('.plugin-detail[data-install-source="i18n-state-00"]');
    await expect(detail).toBeVisible();
    await expect(detail.locator('.plugin-detail__status')).toHaveText('Reviewed');
    const managed = detail.locator('.plugin-detail__summary-block').filter({
      has: page.locator('.plugin-detail__summary-title', { hasText: 'Managed by CLI' }),
    });
    const prepared = detail.locator('.plugin-detail__summary-block').filter({
      has: page.locator('.plugin-detail__summary-title', {
        hasText: 'Requires a final step in the app',
      }),
    });
    await expect(managed.locator('.plugin-detail__client')).toHaveText(['Cursor']);
    await expect(prepared.locator('.plugin-detail__client')).toHaveText(['Codex CLI', 'ChatGPT Desktop']);
    await expect(detail.locator('.plugin-detail__client').filter({ hasText: 'Kiro' })).toHaveCount(0);
    await expect(detail.getByRole('checkbox', { name: /^Cursor(?:\s|$)/ })).toBeChecked();
    await expect(detail.getByRole('checkbox', { name: /^Codex CLI(?:\s|$)/ })).toBeChecked();
    await expect(detail.locator('.install-command-row .command-snippet--add code')).toContainText('--target codex,cursor');
    expect(errors).toEqual([]);
  });
}

test('expired Directory disables both catalog and standalone install promises', async ({ page }) => {
  const errors = await offline(page);
  const index = eligibleFixture();
  index.expires_at = '2026-01-01T00:00:00Z';
  await projections(page, index);
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  const card = page.locator('[data-install-source="i18n-state-00"]').first();
  await expect(card.locator('.plugin-card__ribbon')).toHaveText('temporarily paused');
  await expect(card.locator('.plugin-card__install-toggle')).toHaveCount(0);
  await page.keyboard.press('Escape');
  await card.locator('.plugin-card__title-link').click();
  const detail = page.locator('.plugin-detail[data-install-source="i18n-state-00"]');
  await expect(detail.locator('.plugin-detail__status')).toHaveText('temporarily paused');
  await expect(detail.locator('.plugin-detail__status')).toHaveClass(/plugin-detail__status--muted/);
  await expect(detail.locator('.plugin-detail__install-cta')).toHaveCount(0);
  await expect(detail.locator('.command-snippet--add')).toHaveCount(0);
  expect(errors).toEqual([]);
});

for (const width of [1440, 390]) {
  test('plugin agent grid searches without losing selection at ' + width + 'px', async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    await projections(page, eligibleFixture());
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('[data-install-source="i18n-state-00"]').first();
    await card.locator('.plugin-card__install-toggle').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog.locator('.app-multiselect__trigger')).toHaveCount(0);
    const grid = dialog.locator('.app-multiselect__content--inline');
    await expect(grid).toBeVisible();
    const gridBox = (await grid.boundingBox())!;
    const rowBox = (await dialog.locator('.plugin-card__install').boundingBox())!;
    expect(Math.abs(gridBox.width - rowBox.width)).toBeLessThanOrEqual(3);
    expect(await grid.locator('.app-multiselect__options').evaluate(
      (node) => getComputedStyle(node).gridTemplateColumns.split(' ').length,
    )).toBe(width < 600 ? 2 : 3);
    await expect(dialog.getByRole('checkbox', { name: /^ChatGPT Web(?:\s|$)/ })).toHaveCount(0);
    const search = grid.getByRole('searchbox');
    await search.fill('cOdEx');
    await expect(grid.getByRole('checkbox')).toHaveCount(1);
    const codex = grid.getByRole('checkbox', { name: /^Codex(?:\s|$)/ });
    await codex.click();
    await expect(codex).toBeChecked();
    await expect(grid.locator('.app-multiselect__heading > span')).toHaveText('1 selected');
    await expect(dialog.locator('.install-dialog__footer .command-snippet--add code')).toContainText('--target codex');
    await search.fill('no-such-agent');
    await expect(grid.getByRole('checkbox')).toHaveCount(0);
    await expect(grid.locator('.app-multiselect__heading > span')).toHaveText('1 selected');
    await search.fill('');
    await expect(codex).toBeChecked();
    await codex.click();
    await expect(grid.locator('.app-multiselect__auto')).toHaveAttribute('aria-pressed', 'true');
    await expect(dialog.locator('.command-snippet--add code')).not.toContainText('--target');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(errors).toEqual([]);
  });
}

test('plugin detail uses readable theme tokens and aligned mobile breadcrumbs', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  const errors = await offline(page);
  await projections(page, eligibleFixture());
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  await page.locator('[data-install-source="i18n-state-00"] .plugin-card__title-link').first().click();

  const detail = page.locator('.plugin-detail[data-install-source="i18n-state-00"]');
  await page.getByRole('button', { name: 'Light', exact: true }).click();
  await expect(page.locator('.v-application')).toHaveClass(/v-theme--light/);
  await expect(detail.locator('.plugin-detail__title')).toHaveCSS('color', 'rgb(22, 25, 43)');
  await expect(detail.locator('.plugin-detail__summary-title').first()).toHaveCSS(
    'color',
    'rgb(8, 113, 138)',
  );
  await expect(detail.locator('.plugin-detail__client').first()).toHaveCSS(
    'color',
    'rgb(22, 25, 43)',
  );
  await expect(detail.locator('.plugin-detail__list-item').first()).toHaveCSS(
    'color',
    'rgb(83, 93, 117)',
  );

  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    const breadcrumbs = detail.locator('.plugin-detail__hero-topbar .breadcrumbs');
    const centerDelta = await detail.locator('.plugin-detail__hero-topbar').evaluate((bar) => {
      const back = bar.querySelector('.detail-navigation__back')!.getBoundingClientRect();
      const crumb = bar.querySelector('.breadcrumbs')!.getBoundingClientRect();
      return Math.abs(back.top + back.height / 2 - (crumb.top + crumb.height / 2));
    });
    expect(centerDelta).toBeLessThan(3);
    await expect(breadcrumbs).toHaveCSS('padding-top', '0px');
    await expect(breadcrumbs.locator('[aria-current="page"]')).toHaveText('I18n State 00');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.getByRole('button', { name: 'Dark', exact: true }).click();
  await expect(detail.locator('.plugin-detail__title')).toHaveCSS('color', 'rgb(224, 230, 255)');
  expect(errors).toEqual([]);
});
