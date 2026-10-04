import { expect, test } from '@playwright/test';
import type { DiscoverySnapshot } from '../../types/discovery';
import { hydrated, navigate } from './i18n-state.helpers';

test('expired authenticated mirror keeps community catalog interactive with exact commit installation and no age warnings', async ({
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
  await expect(catalog.locator('.discovery-status')).toHaveCount(0);
  const total = Number((await page.locator('.hero__plugin-count').innerText()).replace(/\D/g, ''));
  expect(total).toBeGreaterThan(29);
  await hydrated(page);
  const community = catalog.locator('.plugin-card[data-trust="community"]').first();
  await expect(community).toBeVisible();
  await expect(community).not.toContainText(
    /catalog expired|catalog has expired|historical signed listing/i,
  );
  const source = await community.getAttribute('data-install-source');
  expect(source).toBeTruthy();
  const record = snapshot.records.find((record) => record.slug === source);
  expect(record).toBeDefined();
  const expectedAdd = `npx universal-agent-plugins add github:${record!.repository}@${record!.revision}${record!.package_path ? `//${record!.package_path}` : ''}`;
  await community.locator('.plugin-card__install-toggle').click();
  await expect(community.locator('.command-snippet--add code')).toHaveText(expectedAdd);
  await expect(community.locator('.plugin-card__security')).toHaveCount(0);

  await navigate(page, '/plugins/community/', 'en', `?source=${encodeURIComponent(source!)}`);
  await expect(page.locator('.install-panel .command-snippet')).toHaveCount(4);
  await expect(page.locator('.install-panel .command-snippet--add code')).toHaveText(expectedAdd);
  await expect(page.locator('.community-plugin-page')).not.toContainText(
    /catalog expired|catalog has expired|historical signed listing|stale Directory/i,
  );
  await expect(page.locator('#security-review')).toHaveCount(0);
  await expect(page.getByRole('link', { name: 'View on GitHub', exact: true })).toBeVisible();

  // Reload with every Discovery request blocked: the browser must reverify its
  // cached signed bytes and keep the large community catalog.
  await page.route('**/discovery/**', (route) => route.abort());
  await page.goto('./');
  await expect(catalog).toHaveAttribute('data-discovery-state', 'stale', { timeout: 20_000 });
  await expect(catalog.locator('.discovery-status')).toHaveCount(0);
  const cachedCommunity = catalog.locator(`.plugin-card[data-install-source="${source}"]`);
  await expect(cachedCommunity).toBeVisible();
  await cachedCommunity.locator('.plugin-card__install-toggle').click();
  await expect(cachedCommunity.locator('.command-snippet--add code')).toHaveText(expectedAdd);
  await expect(cachedCommunity.locator('.plugin-card__security')).toHaveCount(0);
  await expect(page.locator('.hero__plugin-count')).toHaveText(
    new Intl.NumberFormat('en').format(total),
  );
});
