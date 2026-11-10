import { expect, test } from '@playwright/test';

test('homepage raster images are valid served WebP, not PNG fallbacks', async ({ page }) => {
  await page.goto('./');
  for (const selector of ['.registry-restore__monitor', '.built-with__logo']) {
    const image = page.locator(selector);
    await image.scrollIntoViewIfNeeded();
    await expect(image).toHaveAttribute('src', /\.webp(?:$|\?)/);
    await expect.poll(() => image.evaluate((node) => (node as HTMLImageElement).naturalWidth))
      .toBeGreaterThan(0);
  }
});

test('reviewed plugin logo keeps transparent WebP asset on cards', async ({ page }) => {
  await page.goto('./plugins/');
  const card = page.locator('.plugin-card[data-install-source="context7"]');
  await expect(card).toBeVisible();
  const images = card.locator('img[src*=".webp"]');
  await expect(images.first()).toBeVisible();
  await expect.poll(() => images.first().evaluate((node) => (node as HTMLImageElement).naturalWidth))
    .toBeGreaterThan(0);
});
