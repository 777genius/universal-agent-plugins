import { expect, test } from '@playwright/test';
import { publishedLocales } from '../../data/i18n';
import { hydrated } from './i18n-state.helpers';

// Publication drives visibility; browser/cookie preferences never redirect root.
test('root ignores manual preference/browser language and does not write a locale cookie', async ({ page, context, baseURL }) => {
  const url = new URL(baseURL!);
  await context.addCookies([{ name: 'uap_locale', value: 'ru', domain: url.hostname, path: url.pathname }]);
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'language', { get: () => 'uk-UA' });
    Object.defineProperty(Storage.prototype, 'getItem', { value: () => { throw new Error('Storage denied'); } });
  });
  await page.goto('./');
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  await expect(page).toHaveURL(baseURL!);
  await hydrated(page);
  await expect(page.getByRole('button', { name: /^Language:/ })).toHaveCount(publishedLocales.length > 1 ? 1 : 0);
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
  // Use the app lifecycle gate: Nuxt 4 removes the bootstrap window.__NUXT__ payload.
  await hydrated(page);
  await page.reload();
  await hydrated(page);
  await expect(page).toHaveURL(requested);
  await expect(search).toBeVisible();
  await expect(search).toHaveValue('gitlab');
  expect((await context.cookies()).some(cookie => cookie.name === 'uap_locale')).toBe(false);
});
