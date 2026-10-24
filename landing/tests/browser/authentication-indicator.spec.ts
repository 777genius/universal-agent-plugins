import { expect, test } from '@playwright/test';
import {
  automatic,
  navigate,
  locales,
  offline,
  projections,
  registryFixture,
  targets,
  wording,
} from './i18n-state.helpers';
import type { RegistryIndex } from '../../types/registry';

// Declared default answers codex with no authentication while the alternative
// distribution the command can resolve without --target requires it.
function divergentFixture(index: RegistryIndex) {
  const plugin = index.plugins[1]!;
  const declared = plugin.distributions[0]!;
  const alternative = structuredClone(declared);
  alternative.id = `community/${plugin.name}`;
  alternative.kind = 'community';
  for (const target of [
    ...alternative.targets,
    ...alternative.releases.flatMap((release) => release.targets),
  ])
    target.authentication = 'required';
  declared.targets = declared.targets
    .filter((target) => target.client === 'codex')
    .map((target) => ({ ...target, authentication: 'not_required' }));
  for (const release of declared.releases) release.targets = structuredClone(declared.targets);
  plugin.distributions.push(alternative);
  return plugin;
}


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
      }, testInfo) => {
        const errors = await offline(page);
        await projections(page, registryFixture());
        await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
        const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
        const indicator = card.locator('.authentication-indicator');
        const dialog = page.locator('.authentication-indicator__content');
        await expect(indicator).toHaveAttribute('data-authentication', 'unknown');
        await expect(indicator).toHaveAttribute('data-varies', 'true');
        await expect(indicator).toHaveAttribute('data-icon', 'varies');
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
        const style = await dialog.evaluate((element) => {
          const computed = getComputedStyle(element);
          return {
            border: computed.borderTopStyle,
            padding: parseFloat(computed.paddingTop),
            background: computed.backgroundColor,
          };
        });
        expect(style.border).toBe('solid');
        expect(style.padding).toBeGreaterThan(0);
        expect(style.background).not.toBe('rgba(0, 0, 0, 0)');
        const bounds = await dialog.boundingBox();
        expect(bounds).not.toBeNull();
        expect(bounds!.width).toBeGreaterThan(0);
        expect(bounds!.x).toBeGreaterThanOrEqual(0);
        expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(page.viewportSize()!.width);
        await page.screenshot({ path: testInfo.outputPath('unknown.png') });
        await page.keyboard.press('Escape');
        await expect(dialog).toHaveCount(0);
        await page.waitForTimeout(200);
        await expect(dialog).toHaveCount(0);
        await card.getByRole('button', { name: 'Install I18n State 00', exact: true }).click();
        await targets(page, card, ['Codex']);
        await expect(indicator).toHaveAttribute('data-authentication', 'not_required');
        await expect(indicator).toHaveAttribute('data-icon', 'lock-open');
        await indicator.click();
        await expect(dialog).toContainText('No authentication required');
        await expect(dialog.locator('.authentication-indicator__clients')).toHaveText('codex');
        await page.keyboard.press('Escape');
        await targets(page, card, ['Cursor']);
        await expect(indicator).toHaveAttribute('data-authentication', 'unknown');
        await expect(indicator).toHaveAttribute('data-varies', 'true');
        await expect(indicator).toHaveAttribute('data-icon', 'varies');
        await targets(page, card, ['Codex']);
        await expect(indicator).toHaveAttribute('data-authentication', 'required');
        await expect(indicator).toHaveAttribute('data-icon', 'lock');
        await indicator.click();
        await expect(dialog).toContainText('Authentication required');
        await expect(dialog.locator('.authentication-indicator__clients')).toHaveText('cursor');
        await page.keyboard.press('Escape');
        const locale = locales.find((value) => value !== 'en')!;
        await navigate(page, '/plugins/', locale, '?q=i18n-state-00');
        const t = wording(locale);
        await expect(indicator).toHaveAttribute('data-authentication', 'required');
        await expect(indicator).toHaveAccessibleName(
          t('registryUi.authenticationIndicator.required'),
        );
        await indicator.click();
        await expect(dialog).toContainText(t('registryUi.authenticationIndicator.required'));
        await expect(dialog.locator('.authentication-indicator__clients')).toHaveText('cursor');
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
          true,
        );
        expect(errors).toEqual([]);
      });
      test('automatic badge covers every distribution the command can resolve', async ({
        page,
      }) => {
        const errors = await offline(page);
        const index = registryFixture();
        const plugin = divergentFixture(index);
        await projections(page, index);
        await navigate(page, '/plugins/', 'en', `?q=${plugin.name}`);
        const indicator = page
          .locator(`.plugin-card[data-install-source="${plugin.name}"]`)
          .locator('.authentication-indicator');
        await expect(indicator).toHaveAttribute('data-authentication', 'unknown');
        await expect(indicator).toHaveAttribute('data-varies', 'true');
        await expect(indicator).toHaveAttribute('data-icon', 'varies');
        expect(errors).toEqual([]);
      });
      test('automatic badge follows the no-target command instead of the chip selection', async ({
        page,
      }) => {
        const errors = await offline(page);
        const index = registryFixture();
        const plugin = divergentFixture(index);
        await projections(page, index);
        await navigate(page, '/plugins/', 'en', `?q=${plugin.name}`);
        const card = page.locator(`.plugin-card[data-install-source="${plugin.name}"]`);
        const indicator = card.locator('.authentication-indicator');
        await card.getByRole('button', { name: `Install ${plugin.display_name}`, exact: true }).click();
        await targets(page, card, ['Codex']);
        await expect(indicator).toHaveAttribute('data-authentication', 'not_required');
        await automatic(page, card, 'en');
        await expect(indicator).toHaveAttribute('data-authentication', 'unknown');
        expect(errors).toEqual([]);
      });
    },
  );
}
