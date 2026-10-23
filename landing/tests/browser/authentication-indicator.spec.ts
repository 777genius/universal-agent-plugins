import { expect, test } from '@playwright/test';
import {
  navigate,
  locales,
  offline,
  projections,
  registryFixture,
  targets,
  wording,
} from './i18n-state.helpers';

for (const mobile of [false, true]) {
  test.describe(
    mobile ? 'touch authentication indicator' : 'desktop authentication indicator',
    () => {
      test.use({
        viewport: mobile ? { width: 390, height: 844 } : { width: 1280, height: 900 },
        hasTouch: mobile,
      });
      test('always shows accurate state and supports hover, keyboard, Escape and first tap', async ({
        page,
      }) => {
        const errors = await offline(page);
        await projections(page, registryFixture());
        await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
        const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
        const indicator = card.locator('.authentication-indicator');
        const dialog = page.locator('.authentication-indicator__content');
        await expect(indicator).toHaveAttribute('data-authentication', 'unknown');
        if (mobile) await indicator.tap();
        else {
          await indicator.hover();
          await expect(dialog).toContainText('Select agents in Install');
          await page.mouse.move(0, 0);
          await expect(dialog).toHaveCount(0);
          await indicator.focus();
          await page.keyboard.press('Enter');
          await expect(dialog).toBeFocused();
        }
        await expect(dialog).toContainText('Select agents in Install');
        await page.keyboard.press('Escape');
        await expect(dialog).toHaveCount(0);
        await page.waitForTimeout(200);
        await expect(dialog).toHaveCount(0);
        await card.getByRole('button', { name: 'Install I18n State 00', exact: true }).click();
        await targets(page, card, ['Codex']);
        await expect(indicator).toHaveAttribute('data-authentication', 'not_required');
        await indicator.click();
        await expect(dialog).toContainText('codex: No authentication required');
        await page.keyboard.press('Escape');
        await targets(page, card, ['Cursor']);
        await expect(indicator).toHaveAttribute('data-authentication', 'unknown');
        await targets(page, card, ['Codex']);
        await expect(indicator).toHaveAttribute('data-authentication', 'required');
        await indicator.click();
        await expect(dialog).toContainText('cursor: Authentication required');
        await page.keyboard.press('Escape');
        const locale = locales.find((value) => value !== 'en')!;
        await navigate(page, '/plugins/', locale, '?q=i18n-state-00');
        const t = wording(locale);
        await expect(indicator).toHaveAttribute('data-authentication', 'required');
        await expect(indicator).toHaveAccessibleName(
          t('registryUi.authenticationIndicator.required'),
        );
        await indicator.click();
        await expect(dialog).toContainText(
          `cursor: ${t('registryUi.authenticationIndicator.required')}`,
        );
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
          true,
        );
        expect(errors).toEqual([]);
      });
    },
  );
}
