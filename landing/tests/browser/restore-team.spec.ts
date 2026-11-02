import { expect, test } from '@playwright/test';
import { hydrated } from './i18n-state.helpers';

const addCommand = 'npx -y universal-agent-plugins install --scope project';
const restoreCommand = 'agentplugins install --locked';
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
      await expect(section.locator('.registry-restore__monitor')).toHaveCount(2);
      await expect(section.locator('.registry-restore__wallpaper')).toHaveCount(1);
      await expect(section.locator('.registry-restore__wallpaper')).toHaveAttribute(
        'src', /restore-teammate-wallpaper/,
      );
      await expect(section.locator('.registry-restore__package strong')).toHaveText(['v0.1.0', 'v0.1.0']);
      await expect(section.locator('.registry-restore__package span')).toHaveText(['marketing-kit', 'marketing-kit']);
      await expect(section.locator('.registry-restore__locked-version')).toHaveText('v0.1.0');
      await expect(section.locator('.registry-restore__terminal')).toHaveCount(2);
      await expect(section.locator('.registry-restore__terminal').first()).toHaveAttribute('data-command', addCommand);
      await expect(section.locator('.registry-restore__terminal').last()).toHaveAttribute('data-command', restoreCommand);
      await expect(section.locator('.typed-command')).toHaveCount(2);
      await expect(section.locator('.typed-command').first()).toHaveAttribute('data-complete', 'true');
      await expect(section.locator('.typed-command').last()).toHaveAttribute('data-complete', 'true');
      await expect(section.locator('.registry-restore__copy')).toHaveCount(2);

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
  await expect(file).toHaveAttribute('aria-expanded', 'false');
  await file.focus();
  await file.press('Enter');
  await expect(file).toHaveAttribute('aria-expanded', 'true');
  const preview = handoff.getByRole('region');
  await expect(preview).toContainText(manifest);
  await handoff.screenshot({
    path: testInfo.outputPath('marketing-kit-manifest-expanded.png'),
    animations: 'disabled',
  });
  await file.press('Escape');
  await expect(file).toHaveAttribute('aria-expanded', 'false');
  await file.click();
  await expect(preview).toBeVisible();
  await handoff.locator('.registry-restore__locked-version').click();
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
  }
  await expect(section.locator('.typed-command').first()).toHaveAttribute('data-complete', 'true', { timeout: 15_000 });
  await expect(section.locator('.typed-command').last()).toHaveAttribute('data-complete', 'true', { timeout: 15_000 });
});
