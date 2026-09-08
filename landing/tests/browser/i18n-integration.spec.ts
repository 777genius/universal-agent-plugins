import { readFileSync } from 'node:fs';
import { expect, test } from '@playwright/test';
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
    }) => {
      test.skip(
        !(publishedLocales as readonly KnownLocale[]).includes(target),
        'Production publishes EN only; full transition gate activates with reviewed locales',
      );
      await page.setViewportSize({ width, height: 900 });
      const suffix = '?q=context7&tag=one&tag=two&extra=kept#catalog';
      await page.goto(`./plugins/${suffix}`);
      const initialUrl = page.url();
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
        await expect(page.getByRole('dialog').getByRole('menuitemradio', {
          name: localeMetadata[target].name, exact: true,
        })).toBeVisible();
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
      await outsideInput.evaluate(input => input.remove());
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
      expect((await page.context().cookies()).find(cookie => cookie.name === 'uap_locale')?.value).toBe(target);
      const url = new URL(page.url());
      expect(url.searchParams.get('q')).toBe('context7');
      expect(url.searchParams.getAll('tag')).toEqual(['one', 'two']);
      expect(url.searchParams.get('extra')).toBe('kept');
      expect(url.hash).toBe('#catalog');
      await page.goBack();
      await expect(page).toHaveURL(initialUrl);
      await expect(page.locator('html')).toHaveAttribute('lang', 'en');
      await expect(page.locator('h1')).toBeVisible();
      expect((await page.context().cookies()).find(cookie => cookie.name === 'uap_locale')?.value).toBe(target);
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
      readFileSync(new URL(`../../locales/${locale}.json`, import.meta.url), 'utf8'),
    );
    const reference = JSON.parse(
      readFileSync(new URL('../../locales/en.json', import.meta.url), 'utf8'),
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
        'npx universal-agent-plugins add context7',
      );
      await expect(page.locator('#historical-v1 a')).toHaveAttribute(
        'href',
        /docs\/(en|ru)\/guide\/quickstart\.html#historical-v1$/,
      );
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
    'es/plugins/',
  ]) {
    const response = await request.get(`./${route}`);
    expect(response.status(), route).toBe(404);
  }
});
