import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from '@playwright/test';
import { hydrated, setSignedMirrorTestTime } from './i18n-state.helpers';

const muxPlayerScript = 'https://cdn.jsdelivr.net/npm/@mux/mux-player@3.14.1';
const fakePlayer = `
  class TestMuxPlayer extends HTMLElement {
    play() { this.setAttribute('data-playing', 'true'); return Promise.resolve(); }
  }
  customElements.define('mux-player', TestMuxPlayer);
`;

for (const width of [1440, 390, 320]) {
  test(`demo at ${width}px is responsive, enters fullscreen on activation and leaves it on media end`, async ({
    page,
  }) => {
    await setSignedMirrorTestTime(page);
    const requests: string[] = [];
    await page.route(muxPlayerScript, (route) => {
      requests.push(route.request().url());
      return route.fulfill({ contentType: 'application/javascript', body: fakePlayer });
    });
    await page.setViewportSize({ width, height: 1000 });
    await page.goto('./');
    await hydrated(page);
    const demo = page.locator('.hero-demo-video');
    const button = demo.getByRole('button', { name: 'Watch demo' });
    await expect(button).toBeVisible();
    await expect(demo.locator('mux-player, iframe')).toHaveCount(0);
    expect(requests).toHaveLength(0);
    const installBefore = (await page.locator('.hero__window').boundingBox())!;
    const before = (await demo.boundingBox())!;
    expect(before.width).toBeLessThanOrEqual(480);
    const overlay = await button.evaluate((element) => {
      const video = element.closest('.hero-demo-video')!.getBoundingClientRect();
      const label = element.querySelector('span')!.getBoundingClientRect();
      const play = element.querySelector('svg')!.getBoundingClientRect();
      return {
        fontSize: parseFloat(getComputedStyle(element).fontSize),
        playSize: play.width,
        contained: play.top >= video.top && play.bottom <= video.bottom &&
          label.left >= video.left && label.right <= video.right &&
          label.top >= video.top && label.bottom <= video.bottom,
        posterBlur: getComputedStyle(element.parentElement!.querySelector('img')!).filter,
      };
    });
    expect(overlay.fontSize).toBeCloseTo((before.width - 2) * 0.09375, 0);
    expect(overlay.playSize).toBeCloseTo((before.width - 2) * 0.225, 0);
    expect(overlay.posterBlur).toContain('blur(2px)');
    expect(overlay.contained).toBe(true);
    await button.focus();
    await page.keyboard.press('Enter');
    const player = demo.locator('mux-player');
    await expect(player).toHaveAttribute('playback-id', /[a-zA-Z0-9]+/);
    await expect(player).toHaveAttribute('data-playing', 'true');
    expect(requests).toHaveLength(1);
    if (await demo.evaluate((element) => document.fullscreenElement === element)) {
      await expect(demo).toHaveCSS('height', /.+/);
    }
    await player.evaluate((element) => element.dispatchEvent(new Event('ended')));
    await expect(button).toBeVisible();
    await expect.poll(() => page.evaluate(() => document.fullscreenElement)).toBe(null);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const installAfter = (await page.locator('.hero__window').boundingBox())!;
    expect(Math.abs(installAfter.y - installBefore.y)).toBeLessThan(1);
  });
}

test('fullscreen denial or optional player load failure does not block playback', async ({ page }) => {
  await setSignedMirrorTestTime(page);
  await page.route(muxPlayerScript, (route) => route.abort());
  await page.route('https://player.mux.com/**', (route) =>
    route.fulfill({ contentType: 'text/html', body: '<html><body>Fallback player</body></html>' }),
  );
  await page.addInitScript(() => {
    HTMLElement.prototype.requestFullscreen = () => Promise.reject(new Error('denied'));
  });
  await page.goto('./');
  await hydrated(page);
  await page.getByRole('button', { name: 'Watch demo' }).click();
  const fallback = page.locator('.hero-demo-video iframe');
  await expect(fallback).toBeVisible();
  await expect(fallback).toHaveAttribute('src', /^https:\/\/player\.mux\.com\/[a-zA-Z0-9]+\?autoplay=true$/);
  await expect(fallback).toHaveAttribute('allowfullscreen', '');
});

test('owner-uploaded Mux film actually plays on desktop and mobile', async ({ page }) => {
  test.skip(process.env.UAP_E2E_LIVE_VIDEO !== '1', 'Explicit live CDN check; offline tests mock Mux player.');
  test.setTimeout(120_000);
  await setSignedMirrorTestTime(page);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto('./');
    const demo = page.locator('.hero-demo-video');
    await demo.scrollIntoViewIfNeeded();
    await expect(page.getByRole('link', { name: /^Explore [1-9]/ })).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => demo.locator('img').evaluate(
      (image: HTMLImageElement) => image.complete && image.naturalWidth > 0,
    ), { timeout: 30_000 }).toBe(true);
    const directory = process.env.UAP_LANDING_SCREENSHOT_DIR;
    if (directory) {
      mkdirSync(directory, { recursive: true });
      await page.screenshot({ path: resolve(directory, `hero-video-poster-${width}.png`) });
    }
    await demo.getByRole('button', { name: 'Watch demo' }).click();
    const player = demo.locator('mux-player');
    await expect(player).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => player.evaluate(
      (element: HTMLElement & { currentTime: number }) => element.currentTime,
    ), { timeout: 30_000 }).toBeGreaterThan(1);
    const media = await player.evaluate((element: HTMLElement & {
      duration: number; error?: { code: number }; paused: boolean
    }) => ({
      duration: element.duration,
      error: element.error?.code ?? null,
      paused: element.paused,
    }));
    expect(Math.abs(media.duration - 2638 / 30)).toBeLessThan(0.25);
    expect(media.error).toBeNull();
    expect(media.paused).toBe(false);
    if (directory) await page.screenshot({ path: resolve(directory, `hero-video-playing-${width}.png`) });
    await player.evaluate((element: HTMLElement & { pause: () => void }) => element.pause());
  }
});
