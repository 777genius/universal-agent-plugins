import { expect, test } from '@playwright/test';

test('four navigation destinations remain accessible in the tablet menu', async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 800 });
  await page.goto('./');
  await expect(page.locator('.app-header__nav')).toBeHidden();
  await page.getByRole('button', { name: 'Open navigation menu' }).click();
  const menu = page.getByRole('dialog', { name: 'Navigation menu' });
  await expect(menu.locator('.mobile-menu__list a').first()).toHaveText('Video');
  await expect(menu.getByRole('link', { name: 'Supported agents', exact: true })).toBeVisible();
  await expect(menu.getByRole('link', { name: 'FAQ', exact: true })).toBeVisible();
  await menu.getByRole('link', { name: 'Supported agents', exact: true }).click();
  await expect(menu).not.toBeVisible();
  await expect(page).toHaveURL(/#agent-support$/);
});

test('plugin catalog has five desktop columns and progressively adapts', async ({ page }) => {
  await page.goto('./');
  const grid = page.locator('#plugins .plugin-grid');
  await expect(grid.locator('.plugin-card').first()).toBeAttached();

  for (const [width, columns] of [
    [1440, 5],
    [1100, 3],
    [900, 2],
    [390, 1],
  ] as const) {
    await page.setViewportSize({ width, height: 900 });
    const renderedColumns = await grid.evaluate((element) =>
      getComputedStyle(element).gridTemplateColumns.trim().split(/\s+/).length,
    );
    expect(renderedColumns, `at viewport ${width}px`).toBe(columns);
    const documentWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    expect(documentWidth, `no horizontal page overflow at ${width}px`).toBeLessThanOrEqual(width + 1);
  }
});
