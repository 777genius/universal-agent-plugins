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

test('static SEO logo is a valid WebP asset', async ({ page }) => {
  const response = await page.request.get('./logo-192.webp');
  expect(response.ok()).toBe(true);
  expect(response.headers()['content-type']).toContain('image/webp');
  const image = await response.body();
  expect(image.toString('ascii', 0, 4)).toBe('RIFF');
  expect(image.toString('ascii', 8, 12)).toBe('WEBP');
});
