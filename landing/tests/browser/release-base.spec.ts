import { expect, test } from '@playwright/test';
import { publishedLocales } from '../../data/i18n';
import { localizedPath } from '../../utils/localizedRoutes';

// Run remotely against both assembled bases. No server/build is launched by this file.
for (const locale of publishedLocales) {
  test(`${locale}: cold SPA download uses local based release API without payload or GitHub`, async ({
    page,
    request,
    baseURL,
  }) => {
    const base = new URL(baseURL!);
    const endpoint = new URL('api/releases/latest', base).href;
    // Read the real static artifact independently; do not fabricate release metadata.
    const response = await request.get(endpoint);
    expect(response.ok()).toBe(true);
    const release = await response.json();
    expect(release.ok).toBe(true);
    const version = release.version?.match(
      /^(?:agentplugins-)?v?(\d+\.\d+\.\d+(?:-[\w.-]+)?(?:\+[\w.-]+)?)$/,
    )?.[1];
    expect(version).toBeTruthy();

    const payloads: string[] = [];
    const github: string[] = [];
    const localApi: string[] = [];
    await page.route('https://api.github.com/**', async (route) => {
      github.push(route.request().url());
      await route.abort('blockedbyclient');
    });
    await page.route(/\/download\/_payload\.json(?:\?.*)?$/, async (route) => {
      payloads.push(route.request().url());
      await route.abort('blockedbyclient');
    });
    page.on('request', (req) => {
      const url = new URL(req.url());
      if (url.origin === base.origin && url.pathname.includes('/api/releases/'))
        localApi.push(url.href);
    });
    await page.addInitScript(() => sessionStorage.removeItem('plugin-kit-ai_release_meta'));
    await page.goto(`.${localizedPath('/plugins/', locale)}`);
    await expect(page.locator('h1')).toBeVisible();
    await page.waitForFunction(() =>
      Boolean(
        (document.querySelector('#__nuxt') as any)?.__vue_app__?.config.globalProperties.$router,
      ),
    );
    const documents: string[] = [];
    page.on('request', (req) => {
      if (req.isNavigationRequest() && req.frame() === page.mainFrame()) documents.push(req.url());
    });
    await page.evaluate(
      async (target) => {
        const app = (document.querySelector('#__nuxt') as any).__vue_app__;
        const nuxt = app.$nuxt || app.config.globalProperties.$nuxt;
        if (!nuxt) throw new Error('Hydrated Nuxt app is unavailable');
        if (sessionStorage.getItem('plugin-kit-ai_release_meta') !== null)
          throw new Error('Release cache is not cold');
        const key = 'universal-agent-plugins-releases';
        if (nuxt.payload.data[key] !== undefined || nuxt.static?.data?.[key] !== undefined)
          throw new Error('Release payload was already seeded');
        await app.config.globalProperties.$router.push(target);
      },
      localizedPath('/download/', locale),
    );
    await expect(page).toHaveURL(new URL(localizedPath('/download/', locale).slice(1), base).href);
    await expect(page.locator('.download-section__release-info a')).toHaveText(`v${version}`);
    expect(payloads.length, 'missing extracted payload path must execute').toBeGreaterThan(0);
    expect(github.length, 'GitHub refresh must actually be blocked').toBeGreaterThan(0);
    expect(localApi).toEqual([endpoint]);
    expect(documents, 'must remain an SPA navigation').toEqual([]);
    const cache = await page.evaluate(() => ({
      keys: Object.keys(sessionStorage).filter((key) => key.includes('release_meta')),
      entry: JSON.parse(sessionStorage.getItem('plugin-kit-ai_release_meta') || 'null'),
    }));
    expect(cache.keys).toEqual(['plugin-kit-ai_release_meta']);
    expect(cache.entry.data).toEqual(release);
    expect(cache.entry.ts).toBeGreaterThan(0);
  });
}
