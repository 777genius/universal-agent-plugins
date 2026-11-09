import { expect, test } from '@playwright/test';
import { publishedLocales } from '../../data/i18n';
import { localizedPath } from '../../utils/localizedRoutes';


test('static site advertises readable agent indexes and matching catalogs', async ({ page }) => {
  await page.goto('./');

  const catalogs = [];
  for (const relation of ['ai-catalog', 'ard']) {
    const link = page.locator(`head link[rel="${relation}"]`);
    await expect(link).toHaveCount(1);
    const href = await link.getAttribute('href');
    expect(href).toBeTruthy();
    const response = await page.request.get(href!);
    expect(response.status()).toBe(200);
    const catalog = await response.json();
    expect(catalog.specVersion).toBe('1.0');
    expect(catalog.host.displayName).toBe('Universal Agent Plugins');
    expect(catalog.entries.length).toBeGreaterThan(0);
    expect(catalog.entries).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ displayName: 'Official documentation', type: 'text/html' }),
        expect.objectContaining({ displayName: 'Agent documentation index', type: 'text/plain' }),
      ]),
    );
    catalogs.push(catalog);
  }
  expect(catalogs[0]).toEqual(catalogs[1]);

  const catalogHtml = await page.request.get('plugins/').then((response) => response.text());
  // These attributes survive the production SSR build (Vue removes template comments).
  expect(catalogHtml).toContain('data-initial-catalog="reviewed-directory-snapshot"');
  expect(catalogHtml).toContain('data-catalog-augmentation="signed-client-side-discovery"');
  expect(catalogHtml).toContain('data-count-scope="initial-reviewed-subset-not-full-directory"');
  expect(catalogHtml).toContain('The static page starts with reviewed plugins.');

  const indexLink = page.locator('head link[rel="describedby"][type="text/plain"]');
  await expect(indexLink).toHaveCount(1);
  const index = await page.request.get((await indexLink.getAttribute('href'))!);
  expect(index.status()).toBe(200);
  expect(index.headers()['content-type']).toContain('text/plain');
  const text = await index.text();
  expect(text).toMatch(/^# Universal Agent Plugins\n/);
  expect(text).toContain('source-build authoring capabilities are separate');
  expect(text).toContain('independent SKILL.md Skills');
  expect(text).toContain('initial HTML shows the reviewed');
  expect(text).toContain('HTML-only crawlers do not see those additional records');
  expect(text).toContain('Go Installer/Hooks and TypeScript Management SDKs');
  expect(text).toContain('docs/PRODUCT_CAPABILITIES.md');
  expect(catalogs[0].entries).toEqual(expect.arrayContaining([
    expect.objectContaining({ displayName: 'Standalone Agent Skills' }),
    expect.objectContaining({ displayName: 'Browse standalone Skills' }),
    expect.objectContaining({ displayName: 'Embedding and SDKs' }),
  ]));
  for (const entry of catalogs[0].entries) {
    if (entry.type === 'text/html') expect(text).toContain(`](${entry.url})`);
  }
});


test('prerendered Skills overview is indexable for each locale without JavaScript', async ({ request, baseURL }) => {
  const site = new URL(baseURL!);
  const sitemapResponse = await request.get(new URL('sitemap.xml', site).href);
  expect(sitemapResponse.ok()).toBe(true);
  const sitemap = await sitemapResponse.text();

  for (const locale of publishedLocales) {
    const path = localizedPath('/agent-skills/', locale);
    const url = new URL(path.slice(1), site).href;
    const response = await request.get(url);
    expect(response.status(), url).toBe(200);
    const html = await response.text();
    expect(html).toContain('SKILL.md');
    expect(html).toContain('plugin.json');
    expect(html).toContain('agentplugins skills add owner/repository');
    expect(html).toContain('name="robots"');
    expect(html).not.toMatch(/<meta[^>]+name="robots"[^>]+content="noindex/);
    const canonicalTag = html.match(/<link\b[^>]*rel="canonical"[^>]*>/)?.[0];
    const canonicalHref = canonicalTag?.match(/\bhref="([^"]+)"/)?.[1];
    expect(canonicalHref, `Missing canonical for ${url}`).toBeTruthy();
    const canonical = new URL(canonicalHref!);
    expect(canonical.protocol).toBe('https:');
    expect(canonical.pathname).toBe(new URL(path.slice(1), site).pathname);
    expect(html).toContain('hreflang="x-default"');
    expect(sitemap).toContain(`<loc>${canonical.href}</loc>`);
  }

  // Third-party Skill detail routes are deliberately not public SEO identities.
  const detailResponse = await request.get(new URL('skills/?id=arbitrary', site).href);
  expect(detailResponse.ok()).toBe(true);
  const detailHtml = await detailResponse.text();
  expect(detailHtml).toMatch(/<meta[^>]+name="robots"[^>]+content="noindex/);
  expect(sitemap).not.toContain('/skills/</loc>');
});

test('Skills overview links to the browser and remains usable on a small screen', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('agent-skills/');
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Agent Skills');
  await expect(page.getByRole('link', { name: 'Browse the Skills catalog' })).toBeVisible();
  await expect(page.locator('.skills-overview__example code'))
    .toContainText('agentplugins skills add owner/repository --ui --scope project');
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    .toBe(true);
  await testInfo.attach('standalone-skills-overview-mobile', {
    body: await page.screenshot({ fullPage: true, animations: 'disabled' }),
    contentType: 'image/png',
  });
  await page.getByRole('link', { name: 'Browse the Skills catalog' }).click();
  await expect(page).toHaveURL(/\/plugins\/\?tab=skills/);
  await expect(page.locator('.skills-catalog')).toBeVisible();
});
