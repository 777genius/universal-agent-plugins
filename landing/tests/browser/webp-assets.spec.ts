import { expect, test } from '@playwright/test';

test('homepage raster images are valid served WebP, not PNG fallbacks', async ({ page }) => {
  await page.goto('./');
  const monitor = page.locator('.registry-restore__monitor');
  await monitor.scrollIntoViewIfNeeded();
  await expect(monitor).toHaveAttribute('src', /\.webp(?:$|\?)/);
  await expect.poll(() => monitor.evaluate((node) => (node as HTMLImageElement).naturalWidth))
    .toBeGreaterThan(0);

  // The showcase is conditional on the published reviewed directory. The
  // WebP asset must exist even when that catalog entry is temporarily absent.
  const showcase = await page.request.get('./showcase/agent-notifications.webp');
  expect(showcase.ok()).toBe(true);
  expect(showcase.headers()['content-type']).toContain('image/webp');
  const logo = page.locator('.built-with__logo');
  if (await logo.count()) {
    await logo.scrollIntoViewIfNeeded();
    await expect(logo).toHaveAttribute('src', /\.webp(?:$|\?)/);
    await expect.poll(() => logo.evaluate((node) => (node as HTMLImageElement).naturalWidth))
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
