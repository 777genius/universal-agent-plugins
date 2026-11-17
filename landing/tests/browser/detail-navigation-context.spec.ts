import { expect, test } from '@playwright/test';
import { hydrated, offline, wording } from './i18n-state.helpers';

for (const locale of ['en', 'ar'] as const) {
  test(`Playwright → Cursor preserves plugin context on click and reload (${locale})`, async ({
    page,
  }) => {
    const errors = await offline(page);
    await page.setViewportSize({ width: 390, height: 900 });
    await page.goto(locale === 'en' ? './plugins/playwright/' : `./${locale}/plugins/playwright/`);
    await hydrated(page);
    const origin = page.url();
    const detailNav = page.locator('.plugin-detail .detail-navigation');
    await expect(detailNav.locator('[aria-current="page"]')).toHaveText('Playwright');
    const cursor = page.locator('.plugin-detail__client').filter({ hasText: /^Cursor$/ });
    await expect(cursor).toHaveAttribute('href', /\/agents\/cursor\/\?plugin=playwright$/);
    await cursor.click();
    await expect(page).toHaveURL(/\/agents\/cursor\/\?plugin=playwright$/);

    const command = page.locator('.agent-page__install .command-snippet code');
    const navigation = page.locator('.agent-page .detail-navigation');
    for (let reload = 0; reload < 2; reload++) {
      if (reload) {
        await page.reload();
        await hydrated(page);
      }
      await expect(command).toContainText('playwright');
      await expect(command).toContainText('--target cursor');
      await expect(command).not.toContainText('context7');
      await expect(navigation.locator('[aria-current="page"]')).toHaveText('Cursor');
      await expect(navigation.getByRole('link', { name: 'Playwright', exact: true })).toHaveCount(
        2,
      );
      await expect(navigation.locator('.detail-navigation__back')).toHaveAttribute(
        'href',
        new URL(origin).pathname,
      );
      await expect(page.locator('head link[rel="canonical"]')).toHaveAttribute(
        'href',
        /\/agents\/cursor\/$/,
      );
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
        true,
      );
      if (locale === 'ar') {
        await expect(navigation).toHaveCSS('direction', 'rtl');
        await expect(navigation.locator('.detail-navigation__back')).toHaveCSS(
          'transform',
          'matrix(-1, 0, 0, 1, 0, 0)',
        );
      }
    }
    await expect(page.locator('.command-snippet__label').first()).toHaveText(
      wording(locale)('registryUi.command.terminal'),
    );
    const copy = page.locator('.agent-page__install .command-snippet button');
    await expect(copy.locator('.v-icon')).toHaveCount(1);
    const raw = await command.textContent();
    expect(raw).not.toContain('\n');
    await copy.click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(raw);
    await navigation.locator('.detail-navigation__back').click();
    await expect(page).toHaveURL(origin);
    expect(errors).toEqual([]);
  });
}

test('invalid or repeated plugin context safely preserves the default Cursor example', async ({
  page,
}) => {
  const errors = await offline(page);
  for (const query of [
    'plugin=missing-plugin',
    'plugin=playwright&plugin=context7',
    'plugin=..%2Fplaywright',
  ]) {
    await page.goto(`./agents/cursor/?${query}`);
    await hydrated(page);
    await expect(page.locator('.agent-page__install .command-snippet code')).toContainText(
      'context7',
    );
    await expect(page.locator('.detail-navigation .breadcrumbs a')).toHaveCount(1);
    await expect(page.locator('head link[rel="canonical"]')).toHaveAttribute(
      'href',
      /\/agents\/cursor\/$/,
    );
  }
  expect(errors).toEqual([]);
});
