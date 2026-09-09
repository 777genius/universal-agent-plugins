import { publishedLocales } from '../../data/i18n';
import { localizedPath } from '../../utils/localizedRoutes';
import { expect, test } from '@playwright/test';

for (const locale of publishedLocales)
  test(`${locale}: static pages receive only their route-scoped reviewed registry data`, async ({
    request,
  }) => {
    const [detail, download, pluginApi, emptyApi] = await Promise.all([
      request.get(`.${localizedPath('/plugins/context7/', locale)}`),
      request.get(`.${localizedPath('/download/', locale)}`),
      request.get('./api/registry/plugin/context7'),
      request.get('./api/registry/empty'),
    ]);
    for (const response of [detail, download, pluginApi, emptyApi])
      expect(response.ok()).toBe(true);

    const detailHtml = await detail.text();
    const downloadHtml = await download.text();
    expect(Buffer.byteLength(detailHtml)).toBeLessThan(450_000);
    expect(Buffer.byteLength(downloadHtml)).toBeLessThan(450_000);
    expect(detailHtml).not.toContain('registryIndex');
    expect(downloadHtml).not.toContain('registryIndex');
    expect(detailHtml).not.toContain('Community package for the official Atlassian');

    const plugin = (await pluginApi.json()) as { plugins: Array<{ name: string }> };
    const empty = (await emptyApi.json()) as { plugins: unknown[] };
    expect(plugin.plugins.map((item) => item.name)).toEqual(['context7']);
    expect(empty.plugins).toEqual([]);
  });

for (const locale of publishedLocales) {
  test(`${locale}: extracted static payloads keep registry records scoped`, async ({ request }) => {
    const [detail, download, reviewed] = await Promise.all([
      request.get(`.${localizedPath('/plugins/context7/', locale)}_payload.json`),
      request.get(`.${localizedPath('/download/', locale)}_payload.json`),
      request.get('./api/registry/plugin/context7'),
    ]);
    for (const response of [detail, download, reviewed]) {
      expect(response.status()).toBe(200);
      expect(response.headers()['content-type']).toMatch(/json|octet-stream/);
    }
    const detailPayload = await detail.json();
    const downloadPayload = await download.json();
    expect(Array.isArray(detailPayload)).toBe(true);
    expect(Array.isArray(downloadPayload)).toBe(true);
    const plugin = (await reviewed.json()).plugins[0];
    expect(detailPayload).toContain(plugin.description);
    expect(downloadPayload).not.toContain(plugin.description);
    expect(JSON.stringify(downloadPayload)).not.toContain('registry-page:');
    for (const payload of [detailPayload, downloadPayload]) {
      expect(JSON.stringify(payload)).not.toContain('registryIndex');
      expect(payload).not.toContain('Community package for the official Atlassian');
    }
  });

  test(`${locale}: real router transitions use the deployment base and neutral registry keys`, async ({
    page,
    baseURL,
  }) => {
    const requests: string[] = [];
    page.on('request', (request) => requests.push(request.url()));
    await page.goto(`.${localizedPath('/plugins/', locale)}`);
    await expect(page.locator('h1')).toBeVisible();
    await page.waitForFunction(() => {
      const app = (document.querySelector('#__nuxt') as any)?.__vue_app__;
      const nuxt = app?.$nuxt || app?.config.globalProperties.$nuxt;
      return Boolean(app?.config.globalProperties.$router && nuxt?.isHydrating === false);
    });
    // Drive the installed router: this tests SPA data loading even where the current
    // design has no direct detail-to-download navigation control.
    const navigate = async (target: string) => {
      await page.evaluate(async (path) => {
        type AppRoot = HTMLElement & {
          __vue_app__?: {
            config: { globalProperties: { $router: { push: (path: string) => Promise<unknown> } } };
          };
        };
        const router = (document.querySelector('#__nuxt') as AppRoot | null)?.__vue_app__?.config
          .globalProperties.$router;
        if (!router) throw new Error('Hydrated Nuxt router is unavailable');
        await router.push(path);
      }, target);
    };
    await navigate(localizedPath('/plugins/context7/', locale));
    await expect(page).toHaveURL(new RegExp(`${localizedPath('/plugins/context7/', locale)}$`));
    await expect(page.locator('h1')).toContainText(/Context7/i);
    await navigate(localizedPath('/download/', locale));
    await expect(page).toHaveURL(new RegExp(`${localizedPath('/download/', locale)}$`));
    await expect(page.locator('h1')).toBeVisible();
    const base = new URL(baseURL!).pathname;
    const localRequests = requests
      .map((url) => new URL(url))
      .filter((url) => url.origin === new URL(baseURL!).origin);
    const dataRequests = localRequests.filter((url) =>
      /\/api\/|\/_payload\.json$/.test(url.pathname),
    );
    expect(dataRequests.length).toBeGreaterThan(0);
    for (const url of dataRequests) {
      expect(url.pathname.startsWith(base), url.href).toBe(true);
      expect(url.pathname).not.toMatch(/\/(ru|uk)\/api\//);
    }
    const keys = await page.evaluate(() =>
      Object.keys(sessionStorage).filter((key) => key.includes('release_meta')),
    );
    expect(keys.every((key) => key === 'plugin-kit-ai_release_meta')).toBe(true);
  });
}
