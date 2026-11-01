import { mkdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from '@playwright/test';
import { hydrated } from './i18n-state.helpers';
import { publishedLocales, localeMetadata, type KnownLocale } from '../../data/i18n';
import { localizedPath } from '../../utils/localizedRoutes';

// This suite runs against the assembled artifact on a plain static server at both bases.
for (const locale of publishedLocales) {
  for (const family of [
    '/',
    '/plugins/',
    '/plugins/context7/',
    '/agents/github-copilot-cli/',
    '/download/',
    '/create-plugin/',
    '/plugins/community/',
  ]) {
    test(`${locale} direct static ${family}`, async ({ page, request }) => {
      const url = `.${localizedPath(family, locale)}`;
      const response = await request.get(url);
      expect(response.status()).toBe(200);
      expect(response.headers()['content-type']).toContain('text/html');
      expect(await response.text()).toContain(`lang="${locale}"`);
      const errors: string[] = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (/hydration.*(mismatch|failed)/i.test(message.text())) errors.push(message.text());
      });
      await page.goto(url);
      await expect(page.locator('html')).toHaveAttribute('lang', locale);
      await expect(page.locator('h1')).toBeVisible();
      if (family === '/') {
        const messages = JSON.parse(
          readFileSync(new URL(`../../i18n/locales/${locale}.json`, import.meta.url), 'utf8'),
        ).shell;
        const copy = messages.restore as Record<string, string>;
        const restore = page.locator('#restore-environment');
        await expect(restore.getByRole('heading', { level: 2 })).toHaveText(copy.title!);
        await expect(restore.locator('h1, h2, h3, h4, h5, h6')).toHaveCount(1);
        await expect(restore.locator('.registry-restore__package strong')).toHaveText([
          'v1.0.0',
          'v1.0.0',
        ]);
        await expect(restore.locator('.registry-restore__locked-version')).toHaveText('v1.0.0');
        const snippets = restore.locator('.command-snippet pre code');
        await expect(snippets).toHaveCount(2);
        await expect(snippets.first()).toContainText('agentplugins add');
        await expect(snippets.first()).toContainText('github:brunoborges/javaevolved');
        await expect(snippets.last()).toHaveText('agentplugins install --locked');
        const docsLocale = ['en', 'ru', 'uk', 'zh', 'es', 'hi', 'ar', 'pt', 'fr'].includes(locale) ? locale : 'en';
        const guideLabel =
          docsLocale === locale
            ? copy.guide!
            : messages.docs.englishLabel.replace('{label}', copy.guide!);
        await expect(restore.getByRole('link', { name: guideLabel })).toHaveAttribute(
          'href',
          new RegExp(`/docs/${docsLocale}/use/manage\\.html#restore-project-dependencies$`),
        );
        if (locale === 'en' || locale === 'ar') {
          await page.setViewportSize({ width: locale === 'ar' ? 390 : 1440, height: 1000 });
          await restore.scrollIntoViewIfNeeded();
          expect(
            await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          ).toBe(true);
          await expect(restore).toHaveCSS('direction', locale === 'ar' ? 'rtl' : 'ltr');
          await expect(restore.locator('pre').first()).toHaveCSS('direction', 'ltr');
          const screenshots = process.env.UAP_LANDING_SCREENSHOT_DIR;
          if (screenshots) {
            mkdirSync(screenshots, { recursive: true });
            await restore.screenshot({ path: resolve(screenshots, `restore-${locale}.png`) });
          }
        }
      }
      await page.reload();
      await expect(page.locator('html')).toHaveAttribute('lang', locale);
      expect(errors).toEqual([]);
    });
  }
}

test('root remains EN regardless of language preference', async ({ page, context }) => {
  await context.addCookies([{ name: 'uap_locale', value: 'uk', domain: '127.0.0.1', path: '/' }]);
  await page.goto('./');
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
});

for (const target of ['ru', 'uk'] as const)
  for (const width of [390, 1440]) {
    test(`${target} navigation preserves query/hash and browser history at ${width}px`, async ({
      page,
      baseURL,
    }) => {
      test.skip(
        !(publishedLocales as readonly KnownLocale[]).includes(target),
        'Production publishes EN only; full transition gate activates with reviewed locales',
      );
      await page.setViewportSize({ width, height: 900 });
      const suffix = '?q=context7&tag=one&tag=two&extra=kept#catalog';
      const initialUrl = new URL(`./plugins/${suffix}`, baseURL!).href;
      await page.goto(initialUrl);
      await hydrated(page);
      await expect(page).toHaveURL(initialUrl);
      const initialHistoryLength = await page.evaluate(() => history.length);
      const targetUrl = new URL(`../${target}/plugins/${suffix}`, initialUrl).href;
      // The shell lane supplies accessible language control names via its own namespace.
      if (width < 768) await page.getByRole('button', { name: 'Open navigation menu' }).click();
      const switcher = page
        .getByRole('button', { name: /language|язык|мова/i })
        .filter({ visible: true });
      await switcher.focus();
      await page.keyboard.press('Enter');
      const choice = page.getByRole('menuitemradio', {
        name: localeMetadata[target].name,
        exact: true,
      });
      await expect(choice).toBeVisible();
      if (width < 768) {
        await expect(
          page.getByRole('dialog').getByRole('menuitemradio', {
            name: localeMetadata[target].name,
            exact: true,
          }),
        ).toBeVisible();
      }
      await choice.focus();
      await page.keyboard.press('Escape');
      await expect(choice).not.toBeVisible();
      await expect(switcher).toBeFocused();
      if (width < 768) await expect(page.getByRole('dialog')).toBeVisible();
      // An outside click must keep its chosen input, including inside the modal.
      await switcher.press('Enter');
      await expect(choice).toBeVisible();
      await page.evaluate((mobile) => {
        const input = document.createElement('input');
        input.setAttribute('aria-label', 'Outside language menu focus probe');
        input.style.cssText = 'position:fixed;top:100px;left:20px;z-index:10001';
        (mobile ? document.querySelector('[role="dialog"]')! : document.body).append(input);
      }, width < 768);
      const outsideInput = page.getByRole('textbox', { name: 'Outside language menu focus probe' });
      await outsideInput.click();
      await expect(choice).not.toBeVisible();
      await expect(outsideInput).toBeFocused();
      await outsideInput.evaluate((input) => input.remove());
      await switcher.press('Enter');
      await choice.focus();
      await choice.press('Enter');
      // i18n updates html.lang during the router guard, before history commits.
      // The button clears busy only after navigateTo resolves and preference persists.
      await expect(page).toHaveURL(targetUrl);
      await expect(switcher).toHaveAttribute('aria-busy', 'false');
      await expect(choice).not.toBeVisible();
      await expect(switcher).toBeFocused();
      await expect(page.locator('html')).toHaveAttribute('lang', target);
      expect(await page.evaluate(() => history.length)).toBe(initialHistoryLength + 1);
      expect(
        (await page.context().cookies()).find((cookie) => cookie.name === 'uap_locale')?.value,
      ).toBe(target);
      const url = new URL(page.url());
      expect(url.searchParams.get('q')).toBe('context7');
      expect(url.searchParams.getAll('tag')).toEqual(['one', 'two']);
      expect(url.searchParams.get('extra')).toBe('kept');
      expect(url.hash).toBe('#catalog');
      await page.goBack();
      await expect(page).toHaveURL(initialUrl);
      await expect(page.locator('html')).toHaveAttribute('lang', 'en');
      await expect(page.locator('h1')).toBeVisible();
      expect(
        (await page.context().cookies()).find((cookie) => cookie.name === 'uap_locale')?.value,
      ).toBe(target);
      await page.goForward();
      await expect(page).toHaveURL(targetUrl);
      await expect(page.locator('html')).toHaveAttribute('lang', target);
    });
  }

for (const locale of publishedLocales) {
  test(`${locale} public authoring is translated and readable without JavaScript`, async ({
    browser,
    baseURL,
  }) => {
    const messages = JSON.parse(
      readFileSync(new URL(`../../i18n/locales/${locale}.json`, import.meta.url), 'utf8'),
    );
    const reference = JSON.parse(
      readFileSync(new URL('../../i18n/locales/en.json', import.meta.url), 'utf8'),
    );
    if (locale !== 'en')
      expect(messages.publicAuthoring.title).not.toBe(reference.publicAuthoring.title);
    const context = await browser.newContext({ javaScriptEnabled: false, baseURL });
    try {
      const page = await context.newPage();
      await page.goto(`.${localizedPath('/create-plugin/', locale)}`);
      await expect(page.getByRole('heading', { level: 1 })).toHaveText(
        messages.publicAuthoring.title,
      );
      await expect(page.locator('#use-plugins code')).toHaveText(
        'npx -y universal-agent-plugins add context7',
      );
      await expect(page.locator('#historical-v1')).toHaveCount(0);
    } finally {
      await context.close();
    }
  });
}

test('unknown routes and localized APIs remain real static 404s', async ({ request }) => {
  for (const route of [
    'agents/',
    'agents/unknown-client/',
    'plugins/unknown-plugin/',
    'ru/api/registry/catalog',
    'uk/api/releases/latest',
    'zz/plugins/',
  ]) {
    const response = await request.get(`./${route}`);
    expect(response.status(), route).toBe(404);
  }
});
