import { expect, test } from '@playwright/test';

const parseJsonLd = async (page: import('@playwright/test').Page) => {
  const values = await page.locator('script[type="application/ld+json"]').allTextContents();
  return values.flatMap((value) => {
    const parsed = JSON.parse(value) as { '@graph'?: Array<Record<string, unknown>> };
    return parsed['@graph'] ?? [];
  });
};

test('homepage installs with auto-detection and exposes the full directory', async ({ page }) => {
  const errors: string[] = [];
  let releaseDiscovery!: () => void;
  const discoveryGate = new Promise<void>((resolve) => {
    releaseDiscovery = resolve;
  });
  await page.route('**/discovery/**', async (route) => {
    await discoveryGate;
    await route.continue();
  });
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(message.text());
  });
  page.on('response', (response) => {
    if (response.status() >= 400) errors.push(`${response.status()} ${response.url()}`);
  });

  await page.goto('./');
  const explorePlugins = page.locator('.hero__actions .button--primary');
  await expect(explorePlugins).toBeVisible();
  await expect(explorePlugins).toHaveAccessibleName('Explore plugins');
  await expect(explorePlugins.locator('.hero__plugin-count')).toHaveCount(0);
  await page.evaluate(() => document.fonts.ready);
  const initialButtonWidth = (await explorePlugins.boundingBox())!.width;
  releaseDiscovery();
  await expect(page.locator('link[rel="icon"][type="image/svg+xml"]')).toHaveAttribute(
    'href',
    /icon\.svg$/,
  );
  await expect(page.locator('.app-logo__mark')).toHaveAttribute('src', /icon\.svg$/);
  await expect(page.locator('.hero-agent-field__hub img')).toHaveAttribute('src', /icon\.svg$/);
  await expect(page.getByRole('heading', { name: /One plugin/ })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'One plugin All your agents' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Install Context7' })).toBeVisible();
  await expect(page.locator('.command-snippet').first()).toContainText(
    'npx universal-agent-plugins add context7',
  );
  await expect(page.locator('.command-snippet').first()).not.toContainText('--target');
  await expect(page.getByText('Supported clients')).toBeVisible();
  await expect(page.locator('.client-strip li')).toHaveCount(11);
  await expect(page.getByRole('contentinfo')).toHaveCount(1);
  await expect(page.locator('.plugin-card').first()).toBeVisible();
  const securityBadge = page.locator('.plugin-card__security').first();
  await expect(securityBadge).toBeVisible({ timeout: 15_000 });
  await expect(securityBadge).toContainText(
    /Automated review: (?:no blocking findings|\d+ notes?|\d+ blocking findings?)/,
  );
  await expect(securityBadge).not.toHaveAttribute('title');
  await securityBadge.hover();
  const securityTooltip = page.locator('.app-tooltip');
  await expect(securityTooltip).toBeVisible();
  await expect(securityTooltip).toContainText(/exact indexed revision [0-9a-f]{12}/);
  await expect(securityTooltip).toContainText(/does not run the plugin or guarantee safety/i);
  await expect(page.getByText(/guarantee of safety/i)).toHaveCount(0);
  await expect(page.locator('.catalog-count')).toContainText(/[2-9]\d{3} plugins/, {
    timeout: 15_000,
  });
  const catalogSummary = await page.locator('.catalog-count').innerText();
  const totalPlugins = Number(catalogSummary.match(/(\d+) plugins/)![1]);
  await expect(explorePlugins).toHaveAccessibleName(
    `Explore ${totalPlugins.toLocaleString('en')} plugins`,
    { timeout: 5_000 },
  );
  expect((await explorePlugins.boundingBox())!.width).toBe(initialButtonWidth);
  await expect(page.getByRole('link', { name: 'Add a plugin', exact: true })).toContainText(
    'Add plugin',
  );
  await expect(page.getByText(/recently found community packages/i)).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('homepage omits the counter when discovery cannot load', async ({ page }) => {
  await page.route('**/discovery/**', (route) => route.abort());
  await page.goto('./');
  await expect(page.locator('.discovery-status--unavailable')).toBeVisible();
  await expect(page.locator('.hero__actions .button--primary')).toHaveAccessibleName(
    'Explore plugins',
  );
  await expect(page.locator('.hero__plugin-count')).toHaveCount(0);
});

test('below-fold sections reveal quickly on scroll without animating the hero', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 480 });
  await page.goto('./');

  await expect(page.locator('.hero-shell')).not.toHaveClass(/scroll-reveal/);
  await expect(page.locator('#plugins')).toHaveClass(/scroll-reveal/);
  const section = page.locator('#why');
  await expect(section).toHaveClass(/scroll-reveal/);
  await expect(page.locator('html')).toHaveClass(/scroll-reveal-active/);
  await expect(section).not.toHaveClass(/scroll-reveal--visible/);
  await expect(section).toHaveCSS('opacity', '0');

  await section.scrollIntoViewIfNeeded();
  await expect(section).toHaveClass(/scroll-reveal--visible/);
  await expect(section).toHaveCSS('opacity', '1', { timeout: 1_000 });
});

test('scroll reveals respect reduced motion', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('./');

  const section = page.locator('#why');
  await expect(section).toHaveClass(/scroll-reveal--visible/);
  await expect(section).toHaveCSS('opacity', '1');
  expect(
    await section.evaluate((node) => Number.parseFloat(getComputedStyle(node).transitionDuration)),
  ).toBeLessThanOrEqual(0.00001);
});

test('below-fold content stays visible when JavaScript is disabled', async ({
  browser,
  baseURL,
}) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  try {
    const page = await context.newPage();
    await page.goto(baseURL!);
    const section = page.locator('#why');
    await expect(section).toBeVisible();
    await expect(section).not.toHaveClass(/scroll-reveal/);
    await expect(section).toHaveCSS('opacity', '1');
  } finally {
    await context.close();
  }
});

test('plugin counter stays inside the hero on a narrow screen', async ({ page }) => {
  await page.setViewportSize({ width: 280, height: 844 });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  let releaseDiscovery!: () => void;
  const discoveryGate = new Promise<void>((resolve) => {
    releaseDiscovery = resolve;
  });
  await page.route('**/discovery/**', async (route) => {
    await discoveryGate;
    await route.continue();
  });
  await page.goto('./');
  await page.evaluate(() => document.fonts.ready);
  const initialWidth = (await page.locator('.hero__actions .button--primary').boundingBox())!.width;
  releaseDiscovery();
  await expect(page.locator('.hero__plugin-count')).toContainText(/[\d,]+/);
  const container = (await page.locator('.hero.container').boundingBox())!;
  const button = (await page.locator('.hero__actions .button--primary').boundingBox())!;
  expect(button.width).toBe(initialWidth);
  expect(button.x).toBeGreaterThanOrEqual(container.x);
  expect(button.x + button.width).toBeLessThanOrEqual(container.x + container.width);
});

test('homepage publishes canonical social metadata and complete product schema', async ({
  page,
}) => {
  const response = await page.goto('./');
  expect(response?.status()).toBe(200);
  await expect(page).toHaveTitle('Universal Agent Plugins CLI | Install Agent Plugins 1.0');
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
    'href',
    'https://777genius.github.io/universal-agent-plugins/',
  );
  await expect(page.locator('link[rel="alternate"][hreflang]')).toHaveCount(0);
  await expect(page.locator('meta[property="og:image"]')).toHaveAttribute(
    'content',
    'https://777genius.github.io/universal-agent-plugins/og-image.png',
  );
  await expect(page.locator('meta[name="twitter:card"]')).toHaveAttribute(
    'content',
    'summary_large_image',
  );

  const graph = await parseJsonLd(page);
  expect(graph.map((item) => item['@type'])).toEqual(
    expect.arrayContaining([
      'WebPage',
      'WebSite',
      'Organization',
      'SoftwareApplication',
      'FAQPage',
    ]),
  );
  const software = graph.find((item) => item['@type'] === 'SoftwareApplication');
  expect(software?.offers).toEqual({ '@type': 'Offer', price: '0', priceCurrency: 'USD' });
  expect(software?.installUrl).toBe('https://www.npmjs.com/package/universal-agent-plugins');
  expect(software?.publisher).toEqual({
    '@id': 'https://777genius.github.io/universal-agent-plugins/#organization',
  });
  const faq = graph.find((item) => item['@type'] === 'FAQPage') as
    { mainEntity?: unknown[] } | undefined;
  expect(faq?.mainEntity).toHaveLength(6);

  const html = await response!.text();
  expect(html).toContain('rel="canonical"');
  expect(html).toContain('application/ld+json');
});

test('supported client links open crawlable client-specific landing pages', async ({ page }) => {
  await page.goto('./');
  await page.locator('.client-strip a[href$="/agents/codex/"]').click();
  await expect(page).toHaveURL(/\/agents\/codex\/?$/);
  await expect(page).toHaveTitle('Agent Plugins for Codex | Universal Agent Plugins');
  await expect(
    page.getByRole('heading', { name: 'Install Agent Plugins for Codex', exact: true }),
  ).toBeVisible();
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
    'href',
    'https://777genius.github.io/universal-agent-plugins/agents/codex/',
  );
  await expect(page.locator('.command-snippet')).toContainText(
    'npx universal-agent-plugins add context7 --target codex',
  );
  const graph = await parseJsonLd(page);
  expect(graph.map((item) => item['@type'])).toEqual(
    expect.arrayContaining(['CollectionPage', 'ItemList', 'BreadcrumbList']),
  );
});

test.describe('mobile navigation and catalog', () => {
  test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

  test('navigation is labeled, modal, keyboard-safe, and restores focus', async ({ page }) => {
    await page.goto('./');
    const trigger = page.getByRole('button', { name: 'Open navigation menu' });

    await expect(trigger).toHaveAttribute('aria-expanded', 'false');
    await trigger.click();
    const dialog = page.getByRole('dialog', { name: 'Navigation menu' });
    await expect(dialog).toBeVisible();
    await expect(page.getByRole('button', { name: 'Close navigation menu' })).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();
  });

  test('keeps advanced filters compact and makes search easy to clear', async ({ page }) => {
    await page.goto('./');
    await page.locator('.catalog .section-heading').scrollIntoViewIfNeeded();

    const toggle = page.locator('.catalog-filter-toggle');
    await expect(toggle).toBeVisible();
    await expect(toggle).toContainText('More filters');
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    const componentFilter = page.locator('.app-select__trigger[aria-label="Filter by component"]');
    await expect(componentFilter).toBeHidden();

    const search = page.getByRole('searchbox', { name: 'Search plugins' });
    await search.fill('gitlab');
    await expect(page.getByRole('button', { name: 'Clear plugin search' })).toBeVisible();
    await page.getByRole('button', { name: 'Clear plugin search' }).click();
    await expect(search).toHaveValue('');

    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    await expect(toggle).toContainText('Hide filters');
    await expect(componentFilter).toBeVisible();
  });
});

test('community plugin titles open an installable plugin page instead of GitHub', async ({
  page,
}) => {
  await page.goto('./');
  const communityCard = page.locator('.plugin-card[data-trust="community"]').first();
  await expect(communityCard).toBeVisible({ timeout: 15_000 });
  const title = communityCard.locator('.plugin-card__title-link');
  const pluginName = (await title.textContent())?.trim();
  await expect(title).toHaveAttribute('href', /\/plugins\/community\/\?source=/);
  await title.click();
  await expect(page).toHaveURL(/\/plugins\/community\/\?source=/);
  await expect(page.getByRole('heading', { name: pluginName, exact: true })).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.locator('.install-panel')).toBeVisible();
});

test('security badges explain the exact checked revision and open full findings', async ({
  page,
}) => {
  await page.goto('./');
  await expect(page.locator('.catalog')).toHaveAttribute('data-discovery-state', /current|cached/, {
    timeout: 15_000,
  });
  const badge = page.locator('.plugin-card__security--warnings').first();
  await expect(badge).toBeVisible({ timeout: 15_000 });
  await expect(badge).not.toHaveAttribute('title');
  await badge.focus();
  const tooltip = page.locator('.app-tooltip');
  await expect(tooltip).toBeVisible();
  await expect(tooltip).toContainText(/SEC\d+/);
  await expect(tooltip).toContainText(/exact indexed revision [0-9a-f]{12}/);
  await expect(badge).toHaveAttribute('href', /plugins\/community.*#security-review/);

  await badge.click();
  await expect(page).toHaveURL(/plugins\/community.*#security-review/);
  const review = page.locator('#security-review');
  await expect(review).toBeVisible({ timeout: 15_000 });
  await expect(review).toContainText(/checked the exact indexed revision/i);
  await expect(review).toContainText('A newer upstream revision is different code');
  await expect(review).toContainText(/SEC\d+/);
  await expect(review).toContainText('do not prove that a plugin is safe');
});

test('metadata-only community records stay out of the install catalog', async ({ page }) => {
  await page.goto('./plugins');
  await expect(page.locator('.catalog')).toHaveAttribute('data-discovery-state', /current|cached/, {
    timeout: 15_000,
  });
  await page.getByRole('searchbox', { name: 'Search plugins' }).fill('remotion');
  await expect(
    page.locator(
      '.plugin-card[data-install-source="discovery:remotion-dev/remotion//packages/agent-plugin"]',
    ),
  ).toHaveCount(0);
});

test('metadata-only community direct links explain why installation is unavailable', async ({
  page,
}) => {
  await page.goto(
    './plugins/community/?source=discovery%3Aremotion-dev%2Fremotion%2F%2Fpackages%2Fagent-plugin',
  );
  await expect(page.getByRole('heading', { name: 'remotion', exact: true })).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.getByRole('heading', { name: 'Not ready to install' })).toBeVisible();
  await expect(page.getByRole('status')).toContainText('This plugin is not installable yet');
  await expect(page.getByRole('status')).toContainText("doesn't include any tools");
  await expect(page.getByText('Not declared')).toHaveCount(2);
  await expect(page.locator('.command-snippet')).toHaveCount(0);
});

test('unavailable community sources never expose stale install guidance', async ({ page }) => {
  await page.goto(
    './plugins/community/?source=discovery%3A777genius%2Funiversal-agent-plugins%2F%2Fbridges%2Fchrome-devtools%2Foverlay',
  );
  await expect(page.getByRole('heading', { name: 'Not ready to install' })).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.getByRole('status')).toContainText(
    'This package is no longer available from its source.',
  );
  await expect(page.getByText('Last known agents')).toBeVisible();
  await expect(page.getByText('Last known components')).toBeVisible();
  await expect(page.locator('.command-snippet')).toHaveCount(0);
  await expect(page.getByText('Automatic detection.')).toHaveCount(0);
});

test('community install controls use the full panel width for long sources', async ({ page }) => {
  await page.goto(
    './plugins/community?source=discovery%3Avectorize-io%2Fhindsight%2F%2Fhindsight-integrations%2Fagent-plugin',
  );

  const selector = page.locator('.install-command-row .target-select');
  const addCommand = page.locator('.install-command-row > .command-snippet');
  await expect(selector).toBeVisible({ timeout: 15_000 });
  await expect(addCommand).toBeVisible();

  for (const width of [1280, 980, 390]) {
    await page.setViewportSize({ width, height: 900 });
    const selectorBox = (await selector.boundingBox())!;
    const commandBox = (await addCommand.boundingBox())!;
    expect(Math.abs(selectorBox.x - commandBox.x)).toBeLessThanOrEqual(1);
    expect(Math.abs(selectorBox.width - commandBox.width)).toBeLessThanOrEqual(1);
    expect(commandBox.y).toBeGreaterThan(selectorBox.y + selectorBox.height);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      ),
    ).toBe(false);

    if (width <= 980) {
      const gridBox = (await page.locator('.plugin-page__grid').boundingBox())!;
      const panelBox = (await page.locator('.install-panel').boundingBox())!;
      expect(Math.abs(gridBox.x - panelBox.x)).toBeLessThanOrEqual(1);
      expect(Math.abs(gridBox.width - panelBox.width)).toBeLessThanOrEqual(1);
    }
  }
});

test('directory filters and reviewed detail keep automatic detection as the default', async ({
  page,
}) => {
  await page.goto('./plugins');
  await expect(page.locator('.catalog-count')).toContainText(/[2-9]\d{3}/, {
    timeout: 15_000,
  });
  await page.getByPlaceholder(/Search by name/).fill('gitlab');
  const gitlabCard = page
    .locator('.plugin-card')
    .filter({ has: page.getByRole('heading', { name: 'GitLab', exact: true }) });
  await expect(gitlabCard).toHaveCount(1);
  await gitlabCard.getByRole('link', { name: 'GitLab', exact: true }).click();
  await expect(page).toHaveURL(/\/plugins\/gitlab\/?$/, { timeout: 15_000 });
  await expect(page.getByRole('heading', { name: 'GitLab', exact: true })).toBeVisible();
  await expect(page.getByText('All installed agents')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Run with npx' })).toBeVisible();
  await expect(page.locator('.command-snippet').first()).toContainText(
    'npx universal-agent-plugins add',
  );
  await expect(page.locator('.command-snippet').first()).not.toContainText('--target');
});

test('directory and reviewed plugin pages expose unique crawlable entities', async ({ page }) => {
  await page.goto('./plugins');
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
    'href',
    'https://777genius.github.io/universal-agent-plugins/plugins/',
  );
  let graph = await parseJsonLd(page);
  const itemList = graph.find((item) => item['@type'] === 'ItemList') as
    { numberOfItems?: number; itemListElement?: unknown[] } | undefined;
  expect(itemList?.numberOfItems).toBeGreaterThanOrEqual(20);
  expect(itemList?.itemListElement).toHaveLength(itemList?.numberOfItems ?? 0);

  await page.goto('./plugins/gitlab');
  await expect(page).toHaveTitle('GitLab Agent Plugin | Universal Agent Plugins');
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
    'href',
    'https://777genius.github.io/universal-agent-plugins/plugins/gitlab/',
  );
  await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toContainText(
    'Plugins/GitLab',
  );
  const description = await page.locator('meta[name="description"]').getAttribute('content');
  expect(description).toContain('Install the GitLab Agent Plugin');
  expect(description?.length).toBeLessThanOrEqual(160);
  graph = await parseJsonLd(page);
  expect(graph.map((item) => item['@type'])).toEqual(
    expect.arrayContaining(['WebPage', 'SoftwareSourceCode', 'BreadcrumbList']),
  );
});

test('sitemap lists only live canonical pages and unstable routes stay out of the index', async ({
  page,
  request,
  baseURL,
}) => {
  const sitemapResponse = await request.get(new URL('sitemap.xml', baseURL).href);
  expect(sitemapResponse.status()).toBe(200);
  const sitemap = await sitemapResponse.text();
  const locations = [...sitemap.matchAll(/<loc>([^<]+)<\/loc>/g)].map((match) => match[1]!);
  expect(locations.length).toBeGreaterThanOrEqual(20);
  expect(locations.every((location) => location.endsWith('/'))).toBe(true);
  expect(locations.some((location) => /\/(ru|es|fr|zh)(?:\/|$)/.test(location))).toBe(false);
  expect(sitemap).not.toContain('<lastmod>');
  expect(sitemap).not.toContain('/plugins/community/');
  expect(sitemap).not.toContain('/create-plugin/');
  expect(locations.filter((location) => location.includes('/agents/'))).toHaveLength(11);

  const prefix = '/universal-agent-plugins/';
  const statuses = await Promise.all(
    locations.map((location) => {
      const pathname = new URL(location).pathname;
      const relative = pathname.startsWith(prefix) ? pathname.slice(prefix.length) : pathname;
      return request.get(new URL(relative, baseURL).href).then((response) => response.status());
    }),
  );
  expect(new Set(statuses)).toEqual(new Set([200]));

  const robotsResponse = await request.get(new URL('robots.txt', baseURL).href);
  expect(robotsResponse.status()).toBe(200);
  const robots = await robotsResponse.text();
  expect(robots).toContain(
    'Sitemap: https://777genius.github.io/universal-agent-plugins/sitemap.xml',
  );
  expect(robots).toContain(
    'Sitemap: https://777genius.github.io/universal-agent-plugins/docs/sitemap.xml',
  );

  await page.goto('./plugins/community?source=discovery%3Aexample');
  await expect(page.locator('meta[name="robots"]')).toHaveAttribute('content', 'noindex, follow');
  await expect(page.locator('link[rel="canonical"]')).toHaveCount(0);
  await expect(page.locator('script[type="application/ld+json"]')).toHaveCount(0);

  await page.goto('./create-plugin');
  await expect(page.locator('meta[name="robots"]')).toHaveAttribute('content', 'noindex, follow');
});

test('an unsupported localized route is never selected for browser-language visitors', async ({
  browser,
  baseURL,
}) => {
  const context = await browser.newContext({ locale: 'ru-RU' });
  const page = await context.newPage();
  const response = await page.goto(baseURL!);
  expect(response?.status()).toBe(200);
  expect(new URL(page.url()).pathname).toBe(new URL(baseURL!).pathname);
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  await context.close();
});

test('download page recommends the detected OS path and preserves the npx alternative', async ({
  browser,
  baseURL,
}) => {
  const context = await browser.newContext({
    userAgent:
      'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 ' +
      '(KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36',
  });
  const page = await context.newPage();

  await page.goto(new URL('download', baseURL).href);
  const tabs = page.locator('.download-section__install-tab');
  const script = tabs.filter({ hasText: 'Verified install script' });
  await expect(script).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByText('Best option for Linux selected')).toBeVisible();
  await expect(page.getByText(/install\.sh \| sh/)).toBeVisible();
  await expect(
    page.getByText('$HOME/.local/bin/agentplugins add context7 --target codex,cursor'),
  ).toBeVisible();

  await tabs.filter({ hasText: 'npx' }).click();
  await expect(
    page.getByText('npx universal-agent-plugins add context7 --target codex,cursor'),
  ).toBeVisible();
  await expect(page.getByText('npx universal-agent-plugins version')).toHaveCount(0);

  await context.close();
});

test('Windows visitors receive the PowerShell installer and invocation', async ({
  browser,
  baseURL,
}) => {
  const context = await browser.newContext({
    userAgent:
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 ' +
      '(KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36',
  });
  const page = await context.newPage();

  await page.goto(new URL('download', baseURL).href);
  const powershell = page
    .locator('.download-section__install-tab')
    .filter({ hasText: 'PowerShell' });
  await expect(powershell).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByText('Best option for Windows selected')).toBeVisible();
  await expect(page.getByText(/install\.ps1 \| iex/)).toBeVisible();
  await expect(
    page.getByText('& "$HOME\\.local\\bin\\agentplugins.exe" add context7 --target codex,cursor'),
  ).toBeVisible();

  await context.close();
});

test('mobile visitors are not told that a desktop CLI command is recommended', async ({
  browser,
  baseURL,
}) => {
  const context = await browser.newContext({
    userAgent:
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 ' +
      '(KHTML, like Gecko) Version/18.6 Mobile/15E148 Safari/604.1',
  });
  const page = await context.newPage();

  await page.goto(new URL('download', baseURL).href);
  const tabs = page.locator('.download-section__install-tabs');
  await expect(tabs.getByText('Recommended')).toHaveCount(0);
  await expect(tabs.getByText('The CLI runs on desktop.')).toBeVisible();
  await expect(
    tabs.locator('.download-section__install-tab').filter({ hasText: 'npx' }),
  ).toHaveAttribute('aria-pressed', 'true');

  await context.close();
});
