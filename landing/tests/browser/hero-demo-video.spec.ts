import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from '@playwright/test';
import { hydrated, setSignedMirrorTestTime } from './i18n-state.helpers';

for (const width of [1440, 390, 320]) {
  test(`demo at ${width}px loads its player only after keyboard activation and keeps installation in place`, async ({
    page,
  }) => {
    await setSignedMirrorTestTime(page);
    const playerRequests: string[] = [];
    await page.route('https://player.mux.com/**', (route) => {
      playerRequests.push(route.request().url());
      return route.fulfill({
        contentType: 'text/html',
        body: '<html><body>Player boundary</body></html>',
      });
    });
    await page.setViewportSize({ width, height: 1000 });
    await page.goto('./');
    await hydrated(page);
    const demo = page.locator('.hero-demo-video');
    const button = demo.getByRole('button', { name: 'Watch demo' });
    await expect(button).toBeVisible();
    await expect(demo.locator('iframe')).toHaveCount(0);
    expect(playerRequests).toHaveLength(0);
    const installBefore = (await page.locator('.hero__window').boundingBox())!;
    const demoBefore = (await demo.boundingBox())!;
    expect(demoBefore.width).toBeLessThanOrEqual(480);
    expect(installBefore.y - (demoBefore.y + demoBefore.height)).toBeGreaterThanOrEqual(27);
    if (width === 1440) {
      const header = (await page.getByRole('banner').boundingBox())!;
      const topGap = demoBefore.y - (header.y + header.height);
      expect(demoBefore.width).toBe(480);
      expect(topGap).toBeGreaterThanOrEqual(16);
      expect(topGap).toBeLessThanOrEqual(32);
    }
    await button.focus();
    await page.keyboard.press('Enter');
    const frame = demo.locator('iframe');
    await expect(frame).toBeVisible();
    await expect(frame).toHaveAttribute(
      'src',
      /^https:\/\/player\.mux\.com\/[a-zA-Z0-9]+\?autoplay=true$/,
    );
    await expect(frame).toHaveAttribute('title', 'Universal Agent Plugins install demo');
    await expect(frame).toHaveAttribute('allowfullscreen', '');
    await expect.poll(() => playerRequests.length).toBe(1);
    const installAfter = (await page.locator('.hero__window').boundingBox())!;
    expect(Math.abs(installAfter.y - installBefore.y)).toBeLessThan(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
  });
}

test('owner-uploaded Mux film actually plays on desktop and mobile', async ({ page }) => {
  test.skip(
    process.env.UAP_E2E_LIVE_VIDEO !== '1',
    'Explicit live CDN check; ordinary CI tests the player boundary offline.',
  );
  test.setTimeout(120_000);
  await page.addInitScript(() => {
    const play = HTMLMediaElement.prototype.play;
    HTMLMediaElement.prototype.play = function () {
      this.muted = true;
      return play.call(this);
    };
  });
  await setSignedMirrorTestTime(page);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto('./');
    const demo = page.locator('.hero-demo-video');
    await demo.scrollIntoViewIfNeeded();
    await expect(page.getByRole('link', { name: /^Explore [1-9]/ })).toBeVisible({
      timeout: 30_000,
    });
    await expect
      .poll(
        () =>
          demo
            .locator('img')
            .evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0),
        { timeout: 30_000 },
      )
      .toBe(true);
    const directory = process.env.UAP_LANDING_SCREENSHOT_DIR;
    if (directory) {
      mkdirSync(directory, { recursive: true });
      await page.screenshot({ path: resolve(directory, `hero-video-poster-${width}.png`) });
    }
    await demo.getByRole('button', { name: 'Watch demo' }).click();
    const video = page.frameLocator('.hero-demo-video iframe').locator('video');
    await expect(video).toBeAttached({ timeout: 30_000 });
    await expect
      .poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime), {
        timeout: 30_000,
      })
      .toBeGreaterThan(1);
    const media = await video.evaluate((element: HTMLVideoElement) => ({
      duration: element.duration,
      error: element.error?.code ?? null,
      paused: element.paused,
    }));
    expect(Math.abs(media.duration - 2484 / 30)).toBeLessThan(0.25);
    expect(media.error).toBeNull();
    expect(media.paused).toBe(false);
    if (directory)
      await page.screenshot({ path: resolve(directory, `hero-video-playing-${width}.png`) });
    await video.evaluate((element: HTMLVideoElement) => element.pause());
  }
});
