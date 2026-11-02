import { expect, test } from '@playwright/test';
import { hydrated } from './i18n-state.helpers';

const addCommand = 'npx -y universal-agent-plugins install marketing-kit --scope user';
const restoreCommand = 'npx -y universal-agent-plugins install --scope user';
const manifest = `schema_version = 1

[dependencies]
"777genius/marketing-kit" = "0.1.0"

[targets]
claude = ["777genius/marketing-kit"]
codex = ["777genius/marketing-kit"]
gemini = ["777genius/marketing-kit"]`;

const cases = [
  ['en', 1440], ['en', 1110], ['en', 1100], ['en', 1024],
  ['en', 768], ['en', 540], ['en', 390], ['en', 320], ['en', 280],
  ['fr', 390], ['fr', 320], ['ru', 390], ['ru', 320],
  ['ar', 390], ['ar', 320],
] as const;

for (const theme of ['dark', 'light'] as const) {
  for (const [locale, width] of cases) {
    test(`marketing-kit restore ${locale} ${theme} at ${width}px`, async ({ page, context }, testInfo) => {
      await page.setViewportSize({ width, height: 1000 });
      await page.emulateMedia({ reducedMotion: 'reduce' });
      await context.addInitScript((value) => localStorage.setItem('theme', value), theme);
      await page.goto(locale === 'en' ? './' : `./${locale}/`);
      await hydrated(page);
      await expect(page.locator('.v-application')).toHaveClass(new RegExp(`v-theme--${theme}`));
      await page.evaluate(() => document.fonts.ready.then(() => undefined));

      const section = page.locator('#restore-environment');
      await section.scrollIntoViewIfNeeded();
      await expect(section.locator('.registry-restore__monitor')).toHaveCount(1);
      await expect(section.locator('.registry-restore__laptop')).toHaveCount(1);
      await expect(section.locator('.registry-restore__wallpaper')).toHaveCount(0);
      await expect(section.locator('.registry-restore__laptop')).toHaveAttribute(
        'src', /\/showcase\/restore-teammate-laptop\.svg$/,
      );
      if (locale === 'en') {
        await expect(section.locator('.registry-restore__subtitle')).toContainText('Marketing Kit 0.1.0');
        await expect(section.locator('.registry-restore__handoff-note')).toHaveText(
          "Commit and share both files. Import previews and replaces your teammate's user manifest and lock after confirmation.",
        );
      }
      await expect(section.locator('.registry-restore__export')).toHaveCount(0);
      await expect(section.locator('.registry-restore__boundary')).toHaveCount(0);
      await expect(section.locator('.registry-restore__package strong')).toHaveText(['v0.1.0', 'v0.1.0']);
      await expect(section.locator('.registry-restore__package span')).toHaveText(['marketing-kit', 'marketing-kit']);
      await expect(section.locator('.registry-restore__locked-version')).toHaveText('v0.1.0');
      await expect(section.locator('.registry-restore__terminal')).toHaveCount(2);
      await expect(section.locator('.registry-restore__terminal').first()).toHaveAttribute('data-command', addCommand);
      await expect(section.locator('.registry-restore__terminal').last()).toHaveAttribute('data-command', restoreCommand);
      const typed = section.locator('.typed-command');
      await expect(typed).toHaveCount(2);
      await expect(typed.first()).toHaveAttribute('data-complete', 'true');
      await expect(typed.last()).toHaveAttribute('data-complete', 'true');
      await expect(typed.locator('.typed-command__visible')).toHaveText([addCommand, restoreCommand]);
      // A visually wrapped command must still be one executable CLI command.
      for (const element of await typed.all()) {
        await expect(element.locator('.typed-command__visible > .typed-command__line')).toHaveCount(0);
      }
      const copyButtons = section.locator('.registry-restore__copy');
      await expect(copyButtons).toHaveCount(2);
      if (locale === 'en') {
        await expect(copyButtons.first()).toHaveAccessibleName(/Copy command: My computer/);
        await expect(copyButtons.last()).toHaveAccessibleName(/Copy command: Teammate.s computer/);
      }
      // Both tiny-monitor controls remain usable by touch, even at 280px.
      expect(await copyButtons.evaluateAll((buttons) => buttons.every((button) => {
        const { width: hitWidth, height: hitHeight } = button.getBoundingClientRect();
        return hitWidth >= 24 && hitHeight >= 24;
      }))).toBe(true);
      if (width <= 340) {
        const overlaps = await section.locator('.registry-restore__screen').evaluateAll((screens) =>
          screens.flatMap((screen, index) => {
            const copy = screen.querySelector('.registry-restore__copy')!.getBoundingClientRect();
            return [...screen.querySelectorAll('.registry-restore__package > *')]
              .filter((node) => {
                const badge = node.getBoundingClientRect();
                return copy.left < badge.right && copy.right > badge.left &&
                  copy.top < badge.bottom && copy.bottom > badge.top;
              })
              .map((node) => `screen ${index + 1}: copy overlaps ${node.textContent}`);
          }),
        );
        expect(overlaps).toEqual([]);
      }

      // Only visibly laid out elements contribute; no text is clipped or shifted
      // outside the picture's reserved screen area at any breakpoint.
      expect(await section.locator('.registry-restore__screen').evaluateAll((screens) =>
        screens.flatMap((screen, index) => {
          const bounds = screen.getBoundingClientRect();
          return [...screen.querySelectorAll(
            '.registry-restore__stepbar, .registry-restore__package, .registry-restore__terminal, .registry-restore__commit',
          )].filter((node) => {
            if (getComputedStyle(node).display === 'none') return false;
            const b = node.getBoundingClientRect();
            return b.left < bounds.left - 2 || b.right > bounds.right + 2 ||
              b.top < bounds.top - 2 || b.bottom > bounds.bottom + 2 ||
              node.scrollWidth > node.clientWidth + 2;
          }).map((node) => `${index}: ${node.className}`);
        }),
      )).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      if (theme === 'dark' && ((locale === 'en' && [1440, 390, 280].includes(width)) ||
        (locale === 'ar' && width === 390))) {
        await section.screenshot({
          path: testInfo.outputPath(`marketing-kit-restore-${locale}-${width}.png`),
          animations: 'disabled',
        });
      }
      const machines = await section.locator('.registry-restore__machine').evaluateAll(
        (nodes) => nodes.map((node) => node.getBoundingClientRect().toJSON()),
      );
      if (width > 1100) expect(machines[0]!.top).toBeCloseTo(machines[1]!.top, 0);
      else {
        expect(machines[1]!.top).toBeGreaterThan(machines[0]!.bottom);
        if (width > 480) machines.forEach((machine) => expect(machine.width).toBeLessThanOrEqual(501));
      }
    });
  }
}

test('manifest morph opens by focus and tap, closes with Escape, and preserves valid TOML', async ({ page }, testInfo) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('./');
  await hydrated(page);
  const handoff = page.locator('#restore-environment .registry-restore__handoff');
  await handoff.scrollIntoViewIfNeeded();
  const file = handoff.getByRole('button', { name: /uap.toml/i });
  await expect(file).toHaveAccessibleName(/preview uap\.toml/i);
  await expect(file).toHaveAttribute('aria-expanded', 'false');
  await file.focus();
  await file.press('Enter');
  await expect(file).toHaveAttribute('aria-expanded', 'true');
  const preview = handoff.getByRole('region');
  await expect(preview).toContainText(manifest);
  await expect(preview.locator('.marketing-manifest__heading strong')).toHaveText(
    'Preview uap.toml example',
  );
  await handoff.screenshot({
    path: testInfo.outputPath('marketing-kit-manifest-expanded.png'),
    animations: 'disabled',
  });
  await file.press('Escape');
  await expect(file).toHaveAccessibleName(/preview uap\.toml/i);
  await expect(file).toHaveAttribute('aria-expanded', 'false');
  await file.click();
  await expect(preview).toBeVisible();
  const dismiss = preview.getByRole('button', { name: /close file preview/i });
  await dismiss.focus();
  await dismiss.press('Escape');
  await expect(file).toBeFocused();
  await expect(file).toHaveAttribute('aria-expanded', 'false');
  await file.click();
  await expect(preview).toBeVisible();
  await handoff.locator('.registry-restore__locked-version').click();
  await expect(file).toHaveAccessibleName(/preview uap\.toml/i);
  await expect(file).toHaveAttribute('aria-expanded', 'false');
});

test('manifest unfolds on first view, folds into icon and reopens on hover', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'no-preference' });
  await page.goto('./');
  await hydrated(page);
  await page.mouse.move(0, 0);
  const handoff = page.locator('#restore-environment .registry-restore__handoff');
  const file = handoff.getByRole('button', { name: /uap.toml/i });
  await handoff.scrollIntoViewIfNeeded();
  await expect(file).toHaveAttribute('aria-expanded', 'true', { timeout: 6_000 });
  await expect(file).toHaveAttribute('aria-expanded', 'false', { timeout: 7_000 });
  await file.hover();
  await expect(file).toHaveAttribute('aria-expanded', 'true');
  await page.mouse.move(0, 0);
  await expect(file).toHaveAccessibleName(/preview uap\.toml/i);
  await expect(file).toHaveAttribute('aria-expanded', 'false');
});

test('typing is sequential and always copies complete commands', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.emulateMedia({ reducedMotion: 'no-preference' });
  await page.goto('./');
  await hydrated(page);
  const section = page.locator('#restore-environment');
  await section.scrollIntoViewIfNeeded();
  const buttons = section.locator('.registry-restore__copy');
  const expected = [addCommand, restoreCommand];
  for (let i = 0; i < expected.length; i++) {
    await buttons.nth(i).focus();
    await buttons.nth(i).press('Enter');
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(expected[i]);
    // The persistent live region announces copy success, with step-specific button names.
    await expect(section.locator('.registry-restore__copy-status')).toContainText('Copied:');
  }
  await expect(section.locator('.typed-command').first()).toHaveAttribute('data-complete', 'true', { timeout: 15_000 });
  await expect(section.locator('.typed-command').last()).toHaveAttribute('data-complete', 'true', { timeout: 15_000 });
  await expect(section.locator('.typed-command__visible')).toHaveText([addCommand, restoreCommand]);
});

test('clipboard denial announces failure and restores copy-button focus', async ({ page }) => {
  await page.goto('./');
  await hydrated(page);
  const section = page.locator('#restore-environment');
  await section.scrollIntoViewIfNeeded();
  await page.evaluate(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: async () => { throw new Error('Permission denied'); } },
    });
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: () => false,
    });
  });
  const copy = section.locator('.registry-restore__copy').first();
  await copy.focus();
  await copy.press('Enter');
  await expect(copy).toBeFocused();
  await expect(copy).toHaveText('Copy failed');
  await expect(section.locator('.registry-restore__copy-status')).toContainText('Copy failed: My computer');
  await expect(copy).toHaveAccessibleName('Copy failed: My computer');
});

test('teammate laptop is served from the same deployment base without wallpaper', async ({ request }) => {
  const response = await request.get('./showcase/restore-teammate-laptop.svg');
  expect(response.ok()).toBe(true);
  expect(response.headers()['content-type']).toMatch(/image\/svg\+xml/);
  const source = await response.text();
  expect(source).toContain('UAP laptop chassis');
  expect(source).not.toContain('Purple twilight mountains');
});
