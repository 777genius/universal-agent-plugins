import { expect, test } from '@playwright/test';
import { hydrated, setSignedMirrorTestTime } from './i18n-state.helpers';

const idlePlayer = `<!doctype html><body><script>
  document.body.dataset.autoplay = new URLSearchParams(location.search).get('autoplay') || '';
  parent.postMessage(JSON.stringify({ context: 'player.js', event: 'ready' }), '*');
</script></body>`;

test('video warms only after seven seconds and cannot autoplay or take focus while idle', async ({ page }) => {
  await setSignedMirrorTestTime(page);
  const requests: { url: string; at: number }[] = [];
  await page.route('https://player.mux.com/**', async (route) => {
    requests.push({ url: route.request().url(), at: await page.evaluate(() => performance.now()) });
    await route.fulfill({ contentType: 'text/html', body: idlePlayer });
  });
  await page.goto('./');
  await hydrated(page);
  const demo = page.locator('.hero-demo-video');
  const warm = demo.locator('iframe.hero-demo-video__preload');
  await expect(warm).toBeAttached({ timeout: 12_000 });
  await expect.poll(() => requests.length).toBe(1);
  expect(requests[0]!.at).toBeGreaterThanOrEqual(6990);
  expect(new URL(requests[0]!.url).searchParams.get('autoplay')).toBeNull();
  await expect(warm).toHaveAttribute('src', /\?preload=auto$/);
  await expect(warm).toHaveAttribute('aria-hidden', 'true');
  await expect(warm).toHaveAttribute('inert', '');
  await expect(warm).toHaveAttribute('tabindex', '-1');
  await expect(warm).not.toBeVisible();
  await expect(warm).not.toHaveAttribute('allow', /autoplay/);
  await expect(demo.getByRole('button', { name: 'Watch demo' })).toBeVisible();
  expect(await page.evaluate(() => document.fullscreenElement)).toBeNull();
  expect(await page.evaluate(() => document.body.style.overflow)).toBe('');
  await demo.getByRole('button', { name: 'Watch demo' }).click();
  const active = demo.locator('iframe');
  await expect(active).toBeVisible();
  await expect(active).toHaveAttribute('src', /\?autoplay=true&muted=false&no-muted-pref=true&preload=auto$/);
  await expect(active).not.toHaveAttribute('inert');
  await expect(active).not.toHaveAttribute('aria-hidden', 'true');
  await expect.poll(() => requests.filter((item) => new URL(item.url).searchParams.has('autoplay')).length).toBe(1);
});

test('click and close never resurrect an idle preload timer', async ({ page }) => {
  await setSignedMirrorTestTime(page);
  const requests: string[] = [];
  await page.route('https://player.mux.com/**', async (route) => {
    requests.push(route.request().url());
    await route.fulfill({ contentType: 'text/html', body: idlePlayer });
  });
  await page.addInitScript(() => { Object.defineProperty(HTMLElement.prototype, 'requestFullscreen', { value: undefined, configurable: true }); });
  await page.goto('./');
  await hydrated(page);
  const demo = page.locator('.hero-demo-video');
  await demo.getByRole('button', { name: 'Watch demo' }).click();
  await expect(demo.locator('iframe')).toBeVisible();
  await demo.getByRole('button', { name: 'Close demo' }).click();
  const before = requests.length;
  await page.waitForFunction(() => performance.now() >= 8500);
  await expect(demo.locator('iframe')).toHaveCount(0);
  expect(requests).toHaveLength(before);
  await expect(demo.getByRole('button', { name: 'Watch demo' })).toBeFocused();
});

test('leaving home cancels pending preload without late Mux requests', async ({ page }) => {
  await setSignedMirrorTestTime(page);
  const requests: string[] = [];
  await page.route('https://player.mux.com/**', async (route) => {
    requests.push(route.request().url());
    await route.fulfill({ contentType: 'text/html', body: idlePlayer });
  });
  await page.goto('./');
  await hydrated(page);
  await page.evaluate(async () => {
    const app = (document.querySelector('#__nuxt') as HTMLElement & { __vue_app__: { config: { globalProperties: { $router: { push: (path: string) => Promise<void> } } } } }).__vue_app__;
    await app.config.globalProperties.$router.push(location.pathname.replace(/\/?$/, '/') + 'plugins/');
  });
  await expect(page.locator('.hero-demo-video')).toHaveCount(0);
  const before = requests.length;
  await page.waitForFunction(() => performance.now() >= 8500);
  expect(requests).toHaveLength(before);
});
