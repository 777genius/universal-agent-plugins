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
    await expect(card.locator('.command-snippet code')).toHaveText(
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
    await expect(card.locator('.command-snippet code')).toContainText('--target codex,cursor');

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
    await expect(prepared.locator('.plugin-detail__client')).toHaveText(['Codex']);
    await expect(detail.locator('.plugin-detail__client').filter({ hasText: 'Kiro' })).toHaveCount(0);
    await expect(detail.locator('.install-outcome')).toContainText('Cursor');
    await expect(detail.locator('.install-outcome')).toContainText('Codex');
    await expect(detail.locator('.command-snippet--add code')).toContainText('--target codex,cursor');
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
  await card.locator('.plugin-card__title-link').click();
  const detail = page.locator('.plugin-detail[data-install-source="i18n-state-00"]');
  await expect(detail.locator('.plugin-detail__status')).toHaveText('temporarily paused');
  await expect(detail.locator('.plugin-detail__status')).toHaveClass(/plugin-detail__status--muted/);
  await expect(detail.locator('.plugin-detail__install-cta')).toHaveCount(0);
  await expect(detail.locator('.command-snippet--add')).toHaveCount(0);
  expect(errors).toEqual([]);
});
