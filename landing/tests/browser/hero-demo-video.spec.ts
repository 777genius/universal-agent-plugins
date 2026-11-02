import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from '@playwright/test';
import { hydrated, setSignedMirrorTestTime } from './i18n-state.helpers';

const fakePlayer = `<!doctype html><html><body><script>
  document.body.dataset.autoplay = new URLSearchParams(location.search).get('autoplay') || '';
  window.addEventListener('message', (event) => {
    const data = typeof event.data === 'string' ? JSON.parse(event.data) : event.data;
    if (data?.context === 'player.js' && data?.method === 'addEventListener' && data?.value === 'ended') {
      document.body.dataset.subscribed = 'ended';
    }
  });
  parent.postMessage(JSON.stringify({
    context: 'player.js', version: '0.0.11', event: 'ready',
  }), '*');
</script></body></html>`;

for (const width of [1440, 390, 320]) {
  test(`demo at ${width}px opens fullscreen and restores the poster on authenticated Mux media end`, async ({ page }) => {
    await setSignedMirrorTestTime(page);
    const requests: string[] = [];
    await page.route('https://player.mux.com/**', (route) => {
      requests.push(route.request().url());
      return route.fulfill({ contentType: 'text/html', body: fakePlayer });
    });
    await page.setViewportSize({ width, height: 1000 });
    await page.goto('./');
    await hydrated(page);
    const demo = page.locator('.hero-demo-video');
    const button = demo.getByRole('button', { name: 'Watch demo' });
    await expect(button).toBeVisible();
    await expect(demo.locator('iframe')).toHaveCount(0);
    expect(requests).toHaveLength(0);
    const installBefore = await page.locator('.hero__window').evaluate(
      (element) => element.getBoundingClientRect().top + window.scrollY,
    );
    const before = (await demo.boundingBox())!;
    const supported = (await page.locator('.client-section').boundingBox())!;
    expect(before.y).toBeGreaterThanOrEqual(supported.y + supported.height - 1);
    expect(before.width).toBeGreaterThanOrEqual(Math.min(width - 32, 1600) - 4);
    await expect(page.locator('.hero__demo .hero-demo-video')).toHaveCount(0);
    await expect(page.locator('.demo-showcase .hero-demo-video')).toHaveCount(1);
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
    expect(overlay.fontSize).toBeCloseTo(Math.max(18, Math.min(56, (before.width - 2) * 0.04)), 0);
    expect(overlay.playSize).toBeCloseTo(Math.max(76, Math.min(156, (before.width - 2) * 0.11)), 0);
    expect(overlay.posterBlur).toContain('blur(2px)');
    expect(overlay.contained).toBe(true);
    await button.focus();
    await page.keyboard.press('Enter');
    const frame = demo.locator('iframe');
    await expect(frame).toBeVisible();
    await expect(frame).toHaveAttribute('allowfullscreen', '');
    await expect(frame).toHaveAttribute(
      'src',
      /^https:\/\/player\.mux\.com\/[a-zA-Z0-9]+\?autoplay=any&preload=auto$/,
    );
    await expect.poll(() => page.frameLocator('.hero-demo-video iframe').locator('body')
      .getAttribute('data-autoplay')).toBe('any');
    await expect(demo.locator('.hero-demo-video__launch')).toHaveCount(0);
    await expect.poll(() => page.frameLocator('.hero-demo-video iframe').locator('body').getAttribute('data-subscribed')).toBe('ended');
    expect(requests).toHaveLength(1);
    // Cross-origin frames cannot be trusted without both origin and source.
    await page.evaluate(() => window.postMessage({
      context: 'player.js', event: 'ended',
    }, '*'));
    await expect(frame).toBeVisible();
    if (await demo.evaluate((element) => document.fullscreenElement === element)) {
      await expect(demo).toHaveCSS('height', /.+/);
    }
    await page.frameLocator('.hero-demo-video iframe').locator('body').evaluate(() =>
      window.parent.postMessage(JSON.stringify({
        context: 'player.js', version: '0.0.11', event: 'ended', listener: 'uap-demo-ended',
      }), '*'),
    );
    await expect(button).toBeVisible();
    await expect.poll(() => page.evaluate(() => document.fullscreenElement)).toBe(null);
    const installAfter = await page.locator('.hero__window').evaluate(
      (element) => element.getBoundingClientRect().top + window.scrollY,
    );
    expect(Math.abs(installAfter - installBefore)).toBeLessThan(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test('denied fullscreen still launches a viewport-filling autoplay player on one click', async ({ page }) => {
  await setSignedMirrorTestTime(page);
  await page.route('https://player.mux.com/**', (route) =>
    route.fulfill({ contentType: 'text/html', body: fakePlayer }),
  );
  await page.addInitScript(() => {
    HTMLElement.prototype.requestFullscreen = () => Promise.reject(new Error('denied'));
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('./');
  await hydrated(page);
  const demo = page.locator('.hero-demo-video');
  await demo.getByRole('button', { name: 'Watch demo' }).click();
  const frame = demo.locator('iframe');
  await expect(frame).toBeVisible();
  await expect(frame).toHaveAttribute('src', /\?autoplay=any&preload=auto$/);
  await expect(demo).toHaveClass(/hero-demo-video--fallback/);
  await expect(demo).toHaveCSS('position', 'fixed');
  await expect.poll(() => demo.evaluate((el) => {
    const rect = el.getBoundingClientRect();
    return rect.width >= innerWidth - 1 && rect.height >= innerHeight - 1;
  })).toBe(true);
  await expect.poll(() => page.frameLocator('.hero-demo-video iframe').locator('body')
    .getAttribute('data-subscribed')).toBe('ended');
  await expect(page.getByRole('button', { name: 'Close demo' })).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.body.style.overflow)).toBe('hidden');
  await page.keyboard.press('Escape');
  await expect(demo.getByRole('button', { name: 'Watch demo' })).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.body.style.overflow)).toBe('');
  await demo.getByRole('button', { name: 'Watch demo' }).click();
  await expect(demo).toHaveClass(/hero-demo-video--fallback/);
  await demo.getByRole('button', { name: 'Close demo' }).click();
  await expect(demo.getByRole('button', { name: 'Watch demo' })).toBeVisible();
});

test('missing Fullscreen API uses an accessible fullscreen overlay', async ({ page }) => {
  await setSignedMirrorTestTime(page);
  await page.route('https://player.mux.com/**', (route) =>
    route.fulfill({ contentType: 'text/html', body: fakePlayer }),
  );
  await page.addInitScript(() => {
    Object.defineProperty(HTMLElement.prototype, 'requestFullscreen', { value: undefined, configurable: true });
  });
  await page.goto('./');
  await hydrated(page);
  const demo = page.locator('.hero-demo-video');
  await demo.getByRole('button', { name: 'Watch demo' }).click();
  await expect(demo).toHaveClass(/hero-demo-video--fallback/);
  await expect(demo.locator('iframe')).toBeVisible();
  await demo.getByRole('button', { name: 'Close demo' }).click();
  await expect(demo.locator('iframe')).toHaveCount(0);
});

test('owner-uploaded Mux film actually plays on desktop and mobile', async ({ page }) => {
  test.skip(process.env.UAP_E2E_LIVE_VIDEO !== '1', 'Explicit live CDN check; offline tests mock Player.js protocol.');
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
    const video = page.frameLocator('.hero-demo-video iframe').locator('video');
    await expect(video).toBeAttached({ timeout: 30_000 });
    await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime),
      { timeout: 30_000 }).toBeGreaterThan(1);
    const media = await video.evaluate((element: HTMLVideoElement) => ({
      duration: element.duration, error: element.error?.code ?? null, paused: element.paused,
    }));
    expect(Math.abs(media.duration - 2638 / 30)).toBeLessThan(0.25);
    expect(media.error).toBeNull();
    expect(media.paused).toBe(false);
    if (directory) await page.screenshot({ path: resolve(directory, `hero-video-playing-${width}.png`) });
    await video.evaluate((element: HTMLVideoElement) => element.pause());
  }
});
