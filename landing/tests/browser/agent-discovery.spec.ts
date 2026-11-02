import { expect, test } from '@playwright/test';

test('static site advertises readable agent indexes and matching catalogs', async ({ page }) => {
  await page.goto('./');

  const catalogs = [];
  for (const relation of ['ai-catalog', 'ard']) {
    const link = page.locator(`head link[rel="${relation}"]`);
    await expect(link).toHaveCount(1);
    const href = await link.getAttribute('href');
    expect(href).toBeTruthy();
    const response = await page.request.get(href!);
    expect(response.status()).toBe(200);
    const catalog = await response.json();
    expect(catalog.specVersion).toBe('1.0');
    expect(catalog.host.displayName).toBe('Universal Agent Plugins');
    expect(catalog.entries.length).toBeGreaterThan(0);
    expect(catalog.entries).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ displayName: 'Official documentation', type: 'text/html' }),
        expect.objectContaining({ displayName: 'Agent documentation index', type: 'text/plain' }),
      ]),
    );
    catalogs.push(catalog);
  }
  expect(catalogs[0]).toEqual(catalogs[1]);

  const indexLink = page.locator('head link[rel="describedby"][type="text/plain"]');
  await expect(indexLink).toHaveCount(1);
  const index = await page.request.get((await indexLink.getAttribute('href'))!);
  expect(index.status()).toBe(200);
  expect(index.headers()['content-type']).toContain('text/plain');
  const text = await index.text();
  expect(text).toMatch(/^# Universal Agent Plugins\n/);
  expect(text).toContain('source-build authoring capabilities are separate');
  for (const entry of catalogs[0].entries) {
    if (entry.type === 'text/html') expect(text).toContain(`](${entry.url})`);
  }
});
