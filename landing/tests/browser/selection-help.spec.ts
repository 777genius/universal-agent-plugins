import { expect, test } from '@playwright/test';
import { navigate, offline, projections, registryFixture, wording } from './i18n-state.helpers';

for (const [locale, width] of [['en', 1440], ['ru', 390]] as const) {
  test(`clearing the final agent returns to the terminal choice with an accurate selection counter in ${locale}`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    const index = registryFixture();
    index.plugins = [index.plugins[0]!];
    await projections(page, index);
    await navigate(page, '/', locale, '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
    await card.locator('.plugin-card__install-toggle').click();
    const codex = page.getByRole('checkbox', { name: /^Codex/ });
    await codex.click();
    await expect(codex).toBeChecked();
    await expect(page.locator('.app-multiselect__heading > span')).toHaveText(
      wording(locale)('registryUi.multi.selected', { count: '1' }),
    );
    await expect(page.getByRole('dialog').locator('.install-dialog__footer .command-snippet code')).toContainText('--target codex');
    await testInfo.attach(`selection-help-${locale}.png`, {
      body: await page.screenshot({ animations: 'disabled' }), contentType: 'image/png',
    });
    await codex.click();
    await expect(codex).not.toBeChecked();
    await expect(page.locator('.app-multiselect__auto')).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('.app-multiselect__heading > span')).toHaveText(
      wording(locale)('registryUi.multi.auto'),
    );
    await expect(page.getByRole('dialog').locator('.install-dialog__footer .command-snippet code')).not.toContainText('--target');
    await page.keyboard.press('Escape');
    await card.locator('.plugin-card__title-link').click();
    await expect(page.locator('.install-panel .install-command-row .command-snippet code')).not.toContainText('--target');
    await expect(page.locator('.app-multiselect__auto')).toHaveAttribute('aria-pressed', 'true');
    expect(errors).toEqual([]);
  });
}
