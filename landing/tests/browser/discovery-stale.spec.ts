import { expect, test } from '@playwright/test';
import type { DiscoverySnapshot } from '../../types/discovery';
import { hydrated, navigate } from './i18n-state.helpers';

test('expired authenticated mirror keeps community catalog, count and source visible without install commands', async ({
  page,
}) => {
  // Keep every production signature and trusted key unchanged. Only advance
  // the browser clock past the real signed mirror's expiry.
  const latest = await page.request.get('./discovery/latest.json');
  expect(latest.ok()).toBe(true);
  const pointer = (await latest.json()) as { snapshot_path: string };
  const response = await page.request.get(`./discovery/${pointer.snapshot_path}`);
  expect(response.ok()).toBe(true);
  const snapshot = (await response.json()) as DiscoverySnapshot;
  expect(snapshot.records.length).toBeGreaterThan(29);
  await page.clock.setFixedTime(new Date(Date.parse(snapshot.expires_at) + 1_000));
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('./');
  const catalog = page.locator('.catalog');
  await expect(catalog).toHaveAttribute('data-discovery-state', 'stale', { timeout: 20_000 });
  await expect(catalog.locator('.discovery-status')).toContainText(
    new Intl.NumberFormat('en').format(snapshot.records.length),
  );
  await expect(catalog.locator('.discovery-status')).toContainText(snapshot.expires_at);
  const total = Number((await page.locator('.hero__plugin-count').innerText()).replace(/\D/g, ''));
  expect(total).toBeGreaterThan(29);
  await hydrated(page);
  const community = catalog.locator('.plugin-card[data-trust="community"]').first();
  await expect(community).toBeVisible();
  await expect(community).toContainText('catalog expired');
  await expect(community.locator('.plugin-card__install-toggle')).toHaveCount(0);
  await expect(community.locator('.command-snippet')).toHaveCount(0);
  const source = await community.getAttribute('data-install-source');
  expect(source).toBeTruthy();
  await expect(community.locator('.plugin-card__security')).toHaveCount(0);

  await navigate(page, '/plugins/community/', 'en', `?source=${encodeURIComponent(source!)}`);
  await expect(page.locator('.install-panel')).toContainText('catalog has expired');
  await expect(page.locator('.install-panel .command-snippet')).toHaveCount(0);
  await expect(page.locator('#security-review')).toHaveCount(0);
  await expect(page.getByRole('link', { name: 'View package source', exact: true })).toBeVisible();

  // Reload with every Discovery request blocked: the browser must reverify its
  // cached signed bytes and keep the large community catalog.
  await page.route('**/discovery/**', (route) => route.abort());
  await page.goto('./');
  await expect(catalog).toHaveAttribute('data-discovery-state', 'stale', { timeout: 20_000 });
  await expect(catalog.locator('.plugin-card[data-trust="community"]').first()).toBeVisible();
  await expect(page.locator('.hero__plugin-count')).toHaveText(
    new Intl.NumberFormat('en').format(total),
  );
});
