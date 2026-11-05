import { expect, test } from '@playwright/test';
import { hydrated } from './i18n-state.helpers';

test('flat filters retain keyboard selection, category search, and reset', async ({ page }) => {
  // The hosted full-registry fixture loads thousands of rows; keep the real
  // keyboard/filter interactions rather than skipping a late visible control.
  test.setTimeout(45_000);
  await page.goto('./');
  await hydrated(page);
  await expect(page.locator('.hero-quick-start__label').filter({ hasText: 'Copy and run' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Other installation methods' })).toHaveAttribute(
    'href',
    'https://github.com/777genius/universal-agent-plugins#install-the-cli',
  );
  await expect(page.getByText('No target flag means auto-detect')).toHaveCount(0);
  const controls = page.getByRole('search', { name: 'Filter plugins' });
  await controls.scrollIntoViewIfNeeded();
  await expect(controls).toHaveCSS('border-top-width', '0px');
  await expect(controls).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
  for (const field of await controls
    .locator('.search-field input, .app-select__trigger, .app-combobox__anchor')
    .all()) {
    await expect(field).toHaveCSS('border-radius', '0px');
    await expect(field).toHaveCSS('border-top-width', '0px');
    await expect(field).toHaveCSS('background-image', 'none');
    await expect(field).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
    expect((await field.boundingBox())!.height).toBeGreaterThanOrEqual(43.5);
    await field.hover();
    await expect(field).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
  }
  const trust = page.getByRole('combobox', { name: 'Filter by trust level', exact: true });
  await trust.focus();
  await page.keyboard.press('Enter');
  await page.getByRole('option', { name: 'Reviewed listings', exact: true }).click();
  await expect(trust).toBeFocused();
  await expect(page.locator('.plugin-card[data-trust="community"]')).toHaveCount(0);
  await expect(page.locator('.plugin-card').first()).toBeVisible();
  await trust.click();
  await page.getByRole('option', { name: 'All trust levels', exact: true }).click();
  const category = page.getByRole('combobox', { name: 'Filter by category', exact: true });
  await category.fill('impossible-category-qa');
  await expect(page.locator('.app-combobox__empty')).toBeVisible();
  await category.fill('All categories');
  await page.getByRole('option', { name: 'All categories', exact: true }).click();
  await page.keyboard.press('Escape');
  await page.getByRole('searchbox', { name: 'Search plugins' }).fill('gitlab');
  await expect(page.locator('.plugin-card').first()).toBeVisible();
  await page.getByRole('button', { name: 'Clear plugin search' }).click();
  await expect(page.getByRole('searchbox', { name: 'Search plugins' })).toHaveValue('');
});

test('numbered benefits remain readable in both themes and responsive layouts', async ({
  page,
}) => {
  await page.goto('./');
  await hydrated(page);
  const cards = page.locator('.registry-benefits__card');
  await expect(cards).toHaveCount(4);
  await expect(page.locator('.registry-benefits__signature svg')).toHaveCount(4);
  await expect(page.locator('.registry-benefits__number')).toHaveText(['01', '02', '03', '04']);
  const headings = ['Start with plugin.json', 'Fits each agent', 'Manage changes safely', 'Know the source'];
  for (const reducedMotion of ['no-preference', 'reduce'] as const) {
    await page.emulateMedia({ reducedMotion });
    for (const width of [1440, 800, 390, 280]) {
      await page.setViewportSize({ width, height: 1000 });
      await cards.first().scrollIntoViewIfNeeded();
      const columns = await page
        .locator('.registry-benefits__grid')
        .evaluate((node) => getComputedStyle(node).gridTemplateColumns.split(' ').length);
      expect(columns).toBe(width > 980 ? 4 : width > 720 ? 2 : 1);
      for (const [index, card] of (await cards.all()).entries()) {
        await expect(
          card.getByRole('heading', { name: headings[index], exact: true }),
        ).toBeVisible();
        const box = (await card.boundingBox())!;
        expect(box.x).toBeGreaterThanOrEqual(0);
        expect(box.x + box.width).toBeLessThanOrEqual(width);
        // The card intentionally clips its decorative ::after glow beyond
        // its edge. Assert visible text containment, not pseudo-element scrollWidth.
        const textFits = await card.locator('h3, p').evaluateAll((nodes) =>
          nodes.every((node) => {
            const cardBounds = node.closest('.registry-benefits__card')!.getBoundingClientRect();
            const contentBounds = node.getBoundingClientRect();
            return (
              contentBounds.left >= cardBounds.left - 1 &&
              contentBounds.right <= cardBounds.right + 1 &&
              node.scrollWidth <= node.clientWidth + 1
            );
          }),
        );
        expect(textFits).toBe(true);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
        true,
      );
    }
    // The desktop theme control is intentionally absent from the compact mobile header.
    await page.setViewportSize({ width: 1440, height: 1000 });
    const nextTheme = reducedMotion === 'no-preference' ? 'light' : 'dark';
    await page.getByRole('button', { name: nextTheme === 'light' ? 'Light' : 'Dark', exact: true }).click();
    await expect(page.locator('.v-application')).toHaveClass(new RegExp(`v-theme--${nextTheme}`));
  }
});

test('dependency diagram uses the page surface without an outer card', async ({ page }) => {
  await page.goto('./');
  await hydrated(page);
  const scene = page.locator('.dependency-scene');
  for (const theme of ['light', 'dark'] as const) {
    await page.getByRole('button', { name: theme === 'light' ? 'Light' : 'Dark', exact: true }).click();
    await expect(scene).toHaveCSS('background-image', 'none');
    await expect(scene).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
    await expect(scene).toHaveCSS('box-shadow', 'none');
    await expect(scene).toHaveCSS('border-top-width', '0px');
    for (const width of [1440, 390, 280]) {
      await page.setViewportSize({ width, height: 1000 });
      await scene.scrollIntoViewIfNeeded();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
    await page.setViewportSize({ width: 1440, height: 1000 });
  }
});

test('FAQ panels expand smoothly with a keyboard-accessible control', async ({ page }) => {
  await page.goto('./');
  await hydrated(page);
  const items = page.locator('.registry-faq__item');
  await expect(items).toHaveCount(6);
  const first = items.first();
  const firstTrigger = first.getByRole('button');
  const firstPanel = first.locator('.registry-faq__panel');
  await expect(firstTrigger).toHaveAttribute('aria-expanded', 'false');
  await expect(firstPanel).toBeHidden();
  const duration = await firstPanel.evaluate((node) => getComputedStyle(node).transitionDuration);
  expect(duration).toContain('0.38s');
  await firstTrigger.click();
  await expect(firstTrigger).toHaveAttribute('aria-expanded', 'true');
  await expect(firstPanel).toBeVisible();
  await expect.poll(() => firstPanel.evaluate((node) => node.getBoundingClientRect().height)).toBeGreaterThan(30);

  const secondTrigger = items.nth(1).getByRole('button');
  await secondTrigger.focus();
  await page.keyboard.press('Enter');
  await expect(secondTrigger).toHaveAttribute('aria-expanded', 'true');
  await expect(firstTrigger).toHaveAttribute('aria-expanded', 'true');
  await firstTrigger.focus();
  await page.keyboard.press('Space');
  await expect(firstTrigger).toHaveAttribute('aria-expanded', 'false');
  await expect(firstPanel).toBeHidden();

  await page.emulateMedia({ reducedMotion: 'reduce' });
  // The global reduced-motion contract is 0.01ms (computed as 1e-05s).
  // Assert an effectively zero duration rather than an exact string "0s".
  const durations = await firstPanel.evaluate((node) =>
    getComputedStyle(node).transitionDuration.split(',').map((value) => {
      const time = value.trim();
      const seconds = Number.parseFloat(time);
      return time.endsWith('ms') ? seconds / 1000 : seconds;
    }),
  );
  expect(Math.max(...durations)).toBeLessThan(0.0001);
  await firstTrigger.click();
  await expect(firstPanel).toBeVisible();
  for (const width of [390, 280]) {
    await page.setViewportSize({ width, height: 1000 });
    await firstTrigger.scrollIntoViewIfNeeded();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
});
