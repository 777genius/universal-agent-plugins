import { expect, test } from '@playwright/test';

// Real Chromium decode of committed WebP, including Nuxt's bundled artwork
// and both public copies produced by prepare-webp-assets.mjs.
test('committed WebP artwork loads and decodes from generated site', async ({ page }) => {
  await page.goto('./', { waitUntil: 'domcontentloaded' });
  const monitor = page.locator('#restore-environment .registry-restore__monitor');
  await expect(monitor).toHaveAttribute('src', /\\.webp(?:\\?.*)?$/);
  await monitor.scrollIntoViewIfNeeded();
  await expect.poll(() => monitor.evaluate((node) => {
    const img = node as HTMLImageElement;
    return img.complete && img.naturalWidth === 1577 && img.naturalHeight === 997;
  })).toBe(true);

  const result = await page.evaluate(async () => {
    const monitorUrl = (document.querySelector('.registry-restore__monitor') as HTMLImageElement).src;
    const assets = [
      { url: monitorUrl, width: 1577, height: 997, alphaCorner: true },
      { url: new URL('logo-192.webp', location.href).href, width: 192, height: 192, alphaCorner: false },
      { url: new URL('showcase/agent-notifications.webp', location.href).href, width: 0, height: 0, alphaCorner: false },
    ];
    const checks = [];
    for (const asset of assets) {
      const response = await fetch(asset.url);
      if (!response.ok) {
        checks.push({ url: asset.url, error: 'HTTP ' + response.status });
        continue;
      }
      const blob = await response.blob();
      if (blob.type !== 'image/webp') {
        checks.push({ url: asset.url, error: 'Unexpected MIME: ' + blob.type });
        continue;
      }
      const bitmap = await createImageBitmap(blob);
      const dimensionsMatch = bitmap.width > 0 && bitmap.height > 0
        && (!asset.width || (bitmap.width === asset.width && bitmap.height === asset.height));
      let alphaMatch = true;
      if (asset.alphaCorner) {
        const canvas = document.createElement('canvas');
        canvas.width = 1;
        canvas.height = 1;
        const ctx = canvas.getContext('2d', { willReadFrequently: true })!;
        ctx.drawImage(bitmap, 0, 0);
        alphaMatch = ctx.getImageData(0, 0, 1, 1).data[3] === 0;
      }
      checks.push({ url: asset.url, dimensionsMatch, alphaMatch, size: blob.size });
      bitmap.close();
    }
    return checks;
  });
  expect(result, JSON.stringify(result)).toHaveLength(3);
  for (const item of result) {
    expect(item, JSON.stringify(item)).not.toHaveProperty('error');
    expect(item).toMatchObject({ dimensionsMatch: true, alphaMatch: true });
    expect(item.size).toBeGreaterThan(0);
  }
});
