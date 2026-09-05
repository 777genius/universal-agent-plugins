import { expect, test } from '@playwright/test';

test('community installer is centered, full-width, and readable at desktop/tablet/mobile', async ({
  page,
}) => {
  await page.goto(
    './plugins/community/?source=discovery%3Avectorize-io%2Fhindsight%2F%2Fhindsight-integrations%2Fagent-plugin',
  );
  const panel = page.locator('.install-panel');
  await expect(panel.locator('.command-snippet')).toHaveCount(4, { timeout: 15_000 });
  await expect(page.locator('.breadcrumbs a')).toHaveAttribute('href', /\/plugins\/$/);
  await expect(page.locator('#security-review')).toBeVisible();

  for (const width of [1280, 980, 390]) {
    await page.setViewportSize({ width, height: 900 });
    const grid = (await page.locator('.plugin-page__grid').boundingBox())!;
    const box = (await panel.boundingBox())!;
    expect(Math.abs(grid.x - box.x)).toBeLessThanOrEqual(1);
    expect(Math.abs(grid.width - box.width)).toBeLessThanOrEqual(1);
    expect(Math.abs(box.x + box.width / 2 - width / 2)).toBeLessThanOrEqual(1);
    const label = panel.locator('.app-multiselect__value > span:last-child');
    await expect(label).toHaveText('All installed agents');
    expect(await label.evaluate((node) => node.scrollWidth <= node.clientWidth)).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    for (const snippet of await panel.locator('.command-snippet').all()) {
      await expect(snippet).not.toContainText('--target');
    }
  }
});

test('targetless metadata-only community fixture explains missing installation support', async ({
  page,
}) => {
  await page.goto(
    './plugins/community/?source=discovery%3Aremotion-dev%2Fremotion%2F%2Fpackages%2Fagent-plugin',
  );
  const panel = page.locator('.install-panel');
  await expect(panel.getByRole('status')).toContainText("doesn't include any tools", {
    timeout: 15_000,
  });
  await expect(panel.locator('.command-snippet')).toHaveCount(0);
  await expect(page.locator('.plugin-facts dd').filter({ hasText: /^Not declared$/ })).toHaveCount(
    2,
  );
});
