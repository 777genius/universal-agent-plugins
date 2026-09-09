import { expect, test } from '@playwright/test';

// Production PR1 remains EN-only; candidate switching is exercised with EN-only
// Nuxt fixture in tests/fixtures/i18n-candidate, never public translations.
test('root ignores draft preference/browser language and does not write a locale cookie', async ({ page, context, baseURL }) => {
  const url = new URL(baseURL!);
  await context.addCookies([{ name: 'uap_locale', value: 'ru', domain: url.hostname, path: url.pathname }]);
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'language', { get: () => 'uk-UA' });
    Object.defineProperty(Storage.prototype, 'getItem', { value: () => { throw new Error('Storage denied'); } });
  });
  await page.goto('./');
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  await expect(page).toHaveURL(baseURL!);
  await page.waitForFunction("window.__NUXT__?.state?.['$slocale:initialized'] === true");
  await expect(page.getByRole('button', { name: /^Language:/ })).toHaveCount(0);
  const cookies = await context.cookies();
  expect(cookies.find(cookie => cookie.name === 'uap_locale')?.value).toBe('ru');
  expect(cookies.some(cookie => cookie.name === 'i18n_redirected')).toBe(false);
});

test('deep link retains query/hash on refresh without preference creation', async ({ page, context, baseURL }) => {
  const requested = new URL('./plugins/?q=gitlab#plugins', baseURL!).href;
  await page.goto('./plugins/?q=gitlab#plugins');
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  const search = page.getByRole('searchbox', { name: 'Search plugins' });
  await expect(page).toHaveURL(requested);
  await expect(search).toBeVisible();
  await expect(search).toHaveValue('gitlab');
  // init-theme-locale runs on app:mounted; Nuxt prefixes useState payload keys with $s.
  await page.waitForFunction("window.__NUXT__?.state?.['$slocale:initialized'] === true");
  await page.reload();
  await page.waitForFunction("window.__NUXT__?.state?.['$slocale:initialized'] === true");
  await expect(page).toHaveURL(requested);
  await expect(search).toBeVisible();
  await expect(search).toHaveValue('gitlab');
  expect((await context.cookies()).some(cookie => cookie.name === 'uap_locale')).toBe(false);
});
