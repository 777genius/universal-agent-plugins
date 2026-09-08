import { expect, test } from '@playwright/test';

// Production PR1 remains EN-only; candidate switching is exercised with EN-only
// adapter fixtures in i18n-foundation.test.ts, never public copies of translations.
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
  await expect(page.getByRole('button', { name: 'Select language' })).toHaveCount(0);
  const cookies = await context.cookies();
  expect(cookies.find(cookie => cookie.name === 'uap_locale')?.value).toBe('ru');
  expect(cookies.some(cookie => cookie.name === 'i18n_redirected')).toBe(false);
});

test('deep link retains query/hash on refresh without preference creation', async ({ page, context }) => {
  await page.goto('./plugins/?q=gitlab#plugins');
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  const before = page.url();
  await page.reload();
  await expect(page).toHaveURL(before);
  expect((await context.cookies()).some(cookie => cookie.name === 'uap_locale')).toBe(false);
});
