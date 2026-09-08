import { expect, test } from '@playwright/test';

test('flat filters retain keyboard selection, category search, and reset', async ({ page }) => {
  await page.goto('./');
  await expect(page.getByText('In your terminal', { exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Other installation methods' })).toHaveAttribute(
    'href',
    'https://github.com/777genius/universal-agent-plugins#quick-start',
  );
  await expect(page.getByText('No target flag means auto-detect')).toHaveCount(0);
  const controls = page.getByRole('search', { name: 'Filter plugins' });
  await controls.scrollIntoViewIfNeeded();
  await expect(controls).toHaveCSS('border-top-width', '0px');
  await expect(controls).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
  for (const field of await controls
    .locator('.search-field input, .app-select__trigger, .app-combobox__anchor')
    .all()) {
    await expect(field).toHaveCSS('border-radius', '0px');
    await expect(field).toHaveCSS('border-top-width', '0px');
    await expect(field).toHaveCSS('background-image', 'none');
    await expect(field).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
    expect((await field.boundingBox())!.height).toBeGreaterThanOrEqual(43.5);
    await field.hover();
    await expect(field).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
  }
  const trust = page.getByRole('combobox', { name: 'Filter by trust level', exact: true });
  await trust.focus();
  await page.keyboard.press('Enter');
  await page.getByRole('option', { name: 'Reviewed listings', exact: true }).click();
  await expect(trust).toBeFocused();
  await expect(page.locator('.plugin-card[data-trust="community"]')).toHaveCount(0);
  await expect(page.locator('.plugin-card').first()).toBeVisible();
  await trust.click();
  await page.getByRole('option', { name: 'All trust levels', exact: true }).click();
  const category = page.getByRole('combobox', { name: 'Filter by category', exact: true });
  await category.fill('impossible-category-qa');
  await expect(page.locator('.app-combobox__empty')).toBeVisible();
  await category.fill('All categories');
  await page.getByRole('option', { name: 'All categories', exact: true }).click();
  await page.keyboard.press('Escape');
  await page.getByRole('searchbox', { name: 'Search plugins' }).fill('gitlab');
  await expect(page.locator('.plugin-card').first()).toBeVisible();
  await page.getByRole('button', { name: 'Clear plugin search' }).click();
  await expect(page.getByRole('searchbox', { name: 'Search plugins' })).toHaveValue('');
});

test('numbered benefits remain readable in both themes and responsive layouts', async ({
  page,
}) => {
  await page.goto('./');
  const cards = page.locator('.registry-benefits__card');
  await expect(cards).toHaveCount(4);
  await expect(page.locator('.registry-benefits__signature svg')).toHaveCount(4);
  await expect(page.locator('.registry-benefits__number')).toHaveText(['01', '02', '03', '04']);
  const headings = ['Standard first', 'Client-aware', 'Full lifecycle', 'Source visible'];
  for (const reducedMotion of ['no-preference', 'reduce'] as const) {
    await page.emulateMedia({ reducedMotion });
    for (const width of [1440, 800, 390, 280]) {
      await page.setViewportSize({ width, height: 1000 });
      await cards.first().scrollIntoViewIfNeeded();
      const columns = await page
        .locator('.registry-benefits__grid')
        .evaluate((node) => getComputedStyle(node).gridTemplateColumns.split(' ').length);
      expect(columns).toBe(width > 980 ? 4 : width > 720 ? 2 : 1);
      for (const [index, card] of (await cards.all()).entries()) {
        await expect(
          card.getByRole('heading', { name: headings[index], exact: true }),
        ).toBeVisible();
        const box = (await card.boundingBox())!;
        expect(box.x).toBeGreaterThanOrEqual(0);
        expect(box.x + box.width).toBeLessThanOrEqual(width);
        expect(await card.evaluate((node) => node.scrollWidth <= node.clientWidth)).toBe(true);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
        true,
      );
    }
    // The desktop theme control is intentionally absent from the compact mobile header.
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.getByRole('button', { name: 'Toggle theme', exact: true }).click();
  }
});
