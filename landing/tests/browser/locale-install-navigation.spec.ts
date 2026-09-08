import { expect, test } from '@playwright/test';
import { publishedLocales, localeMetadata } from '../../data/i18n';
import { downloadHeading, hydrated, navigate, offline, wording } from './i18n-state.helpers';

// Register only published routes: PR1's EN-only foundation must not request draft
// HTML. When RU/UK ship, these production-artifact regressions become mandatory.
for (const target of publishedLocales.filter((locale) => locale !== 'en')) {
  for (const width of [390, 1440]) {
    for (const entry of ['home', 'download'] as const) {
      test(`automatic install survives EN ${entry} → ${target} at ${width}px`, async ({ page }) => {
        const errors = await offline(page);
        await page.setViewportSize({ width, height: 900 });
        // Viewport and platform are separate: the proven mobile-width failure also
        // used Linux. Force its non-default script recommendation at both widths.
        await page.addInitScript(() =>
          Object.defineProperty(navigator, 'userAgent', {
            configurable: true,
            value:
              'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/130.0.0.0 Safari/537.36',
          }),
        );
        await page.goto(entry === 'home' ? './' : './download/');
        await hydrated(page);
        if (entry === 'home') {
          // The home header has section anchors, not a download link. Use the
          // installed router to enter the real download page without a reload.
          await navigate(page, '/download/');
          await expect(page).toHaveURL(/\/download\/$/);
        }
        await expect(
          page.getByRole('heading', { name: downloadHeading('en'), exact: true }),
        ).toBeVisible();
        await expect(page.locator('.download-section__platform-note')).toContainText(
          wording('en')('download.platforms.linux'),
        );
        const selected = page.locator('.download-section__install-tab[aria-pressed="true"]');
        const selectionIndex = await page
          .locator('.download-section__install-tab')
          .evaluateAll((tabs) =>
            tabs.findIndex((tab) => tab.getAttribute('aria-pressed') === 'true'),
          );
        expect(selectionIndex).toBeGreaterThanOrEqual(0);
        const commands = await page.locator('.download-section__steps code').allTextContents();
        expect(commands.length).toBeGreaterThan(0);
        expect(commands.join('\n')).toContain('curl');
        const initialUrl = page.url();
        if (width < 768) await page.getByRole('button', { name: 'Open navigation menu' }).click();
        const switcher = page
          .getByRole('button', { name: /language|язык|мова/i })
          .filter({ visible: true });
        await switcher.click();
        await page
          .getByRole('menuitemradio', { name: localeMetadata[target].name, exact: true })
          .click();
        await expect(page).toHaveURL(new URL(`../${target}/download/`, initialUrl).href);
        await expect(page.locator('html')).toHaveAttribute('lang', target);
        // URL/html.lang alone passed before the freeze. These require a completed
        // scheduler, localized route content and a responsive real language menu.
        await expect(switcher).toHaveAttribute('aria-busy', 'false');
        if (width < 768) {
          await page.getByRole('button', { name: wording(target)('shell.navigation.close'), exact: true }).click();
          await expect(page.getByRole('dialog')).not.toBeVisible();
        }
        await expect(
          page.getByRole('heading', { name: downloadHeading(target), exact: true }),
        ).toBeVisible();
        await expect(page.locator('.download-section__platform-note')).toContainText(
          wording(target)('download.platforms.linux'),
        );
        expect(
          (await page.context().cookies()).find((cookie) => cookie.name === 'uap_locale')?.value,
        ).toBe(target);
        await expect(selected).toHaveCount(1);
        await expect(
          page.locator('.download-section__install-tab').nth(selectionIndex),
        ).toHaveAttribute('aria-pressed', 'true');
        await expect
          .poll(() => page.locator('.download-section__steps code').allTextContents())
          .toEqual(commands);
        if (width < 768) await page.getByRole('button', { name: wording(target)('shell.navigation.open'), exact: true }).click();
        await switcher.click();
        const current = page.getByRole('menuitemradio', {
          name: localeMetadata[target].name,
          exact: true,
        });
        await expect(current).toBeVisible();
        await expect(current).toHaveAttribute('aria-checked', 'true');
        await page.keyboard.press('Escape');
        await expect(current).not.toBeVisible();
        expect(await page.evaluate(() => document.documentElement.lang)).toBe(target);
        expect(errors).toEqual([]);
      });
    }
  }
}
