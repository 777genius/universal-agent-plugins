import { expect, test } from '@playwright/test';
import { hydrated } from './i18n-state.helpers';

const commands = [
  'agentplugins add github:brunoborges/javaevolved@4bfa3c268a4abb026b16c136330e389fb9cce091//agent-plugins/modern-java-development --target codex --scope project',
  'agentplugins install --locked',
];

// Exercise actual page geometry: long translations, both themes, narrow
// phones, and the desktop-to-stacked transition. All visible text stays on screen.
for (const theme of ['dark', 'light']) {
  for (const [locale, width] of [
    ['en', 1440],
    ['en', 1110],
    ['en', 1100],
    ['en', 1024],
    ['en', 768],
    ['en', 540],
    ['en', 390],
    ['en', 320],
    ['en', 280],
    ['fr', 390],
    ['fr', 320],
    ['ru', 390],
    ['ru', 320],
    ['ar', 390],
    ['ar', 320],
  ] as const) {
    test(`team restore ${locale} ${theme} at ${width}px`, async ({ page, context }) => {
      await page.setViewportSize({ width, height: 1100 });
      await context.addInitScript((value) => localStorage.setItem('theme', value), theme);
      await page.goto(locale === 'en' ? './' : `./${locale}/`);
      await hydrated(page);
      // Persisted theme initializes in Nuxt's idle-ready callback after hydration.
      await expect(page.locator('.v-application')).toHaveClass(new RegExp(`v-theme--${theme}`), {
        timeout: 15_000,
      });
      await page.evaluate(() => document.fonts.ready.then(() => undefined));
      const section = page.locator('#restore-environment');
      await section.scrollIntoViewIfNeeded();
      const monitors = section.locator('img');
      await expect(monitors).toHaveCount(2);
      await monitors.evaluateAll((images) =>
        Promise.all(images.map((image) => (image as HTMLImageElement).decode())),
      );
      expect(
        await monitors.evaluateAll((images) =>
          images.map((image) => (image as HTMLImageElement).currentSrc),
        ),
      ).toEqual(
        [
          await monitors.first().getAttribute('src'),
          await monitors.first().getAttribute('src'),
        ].map((src) => new URL(src!, page.url()).href),
      );
      await expect(section.locator('.registry-restore__package')).toHaveCount(2);
      await expect(section.locator('.registry-restore__package span')).toHaveText([
        'modern-java-development',
        'modern-java-development',
      ]);
      await expect(section.locator('.registry-restore__package strong')).toHaveText([
        'v1.0.0',
        'v1.0.0',
      ]);
      await expect(section.locator('.registry-restore__locked-version')).toHaveText('v1.0.0');
      await expect(section.locator('pre code')).toHaveCount(2);
      await expect(section.locator('pre code').first()).toContainText('agentplugins add');
      await expect(section.locator('pre code').last()).toHaveText('agentplugins install --locked');
      if (width <= 340) {
        // Copy must not cover either the package name or the version badge.
        const collisions = await section.locator('.registry-restore__screen').evaluateAll((screens) =>
          screens.flatMap((screen, index) => {
            const copy = screen.querySelector('button')!.getBoundingClientRect();
            return [...screen.querySelectorAll('.registry-restore__package > *')]
              .filter((element) => {
                const target = element.getBoundingClientRect();
                return (
                  copy.left < target.right &&
                  copy.right > target.left &&
                  copy.top < target.bottom &&
                  copy.bottom > target.top
                );
              })
              .map((element) => `screen ${index + 1}: ${element.textContent}`);
          }),
        );
        expect(collisions).toEqual([]);
      }
      expect(
        await section.locator('.registry-restore__screen').evaluateAll((screens) =>
          screens.flatMap((screen) => {
            const bounds = screen.getBoundingClientRect();
            return [
              ...screen.querySelectorAll(
                'pre, button, .registry-restore__stage-label, .registry-restore__package, .registry-restore__commit',
              ),
            ]
              .filter((element) => {
                if (getComputedStyle(element).display === 'none') return false;
                const box = element.getBoundingClientRect();
                return (
                  box.left < bounds.left - 1 ||
                  box.right > bounds.right + 1 ||
                  box.top < bounds.top - 1 ||
                  box.bottom > bounds.bottom + 1 ||
                  element.scrollWidth > element.clientWidth + 1
                );
              })
              .map((element) => element.textContent);
          }),
        ),
      ).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(
        false,
      );
      const machines = await section
        .locator('.registry-restore__machine')
        .evaluateAll((elements) =>
          elements.map((element) => element.getBoundingClientRect().toJSON()),
        );
      if (width > 1100) {
        expect(machines[0]!.top).toBeCloseTo(machines[1]!.top, 0);
      } else {
        expect(machines[1]!.top).toBeGreaterThan(machines[0]!.bottom);
        // Stacked monitors should not dominate a wide viewport.
        if (width > 480) {
          for (const machine of machines) expect(machine.width).toBeLessThanOrEqual(501);
        }
      }
    });
  }
}

test('both monitor commands copy complete flags and support keyboard focus', async ({
  page,
  context,
}) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.goto('./');
  await hydrated(page);
  const buttons = page.locator('#restore-environment button');
  for (let index = 0; index < commands.length; index++) {
    await buttons.nth(index).focus();
    await expect(buttons.nth(index)).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(buttons.nth(index)).toHaveText('Copied');
    await expect
      .poll(() => page.evaluate(() => navigator.clipboard.readText()))
      .toBe(commands[index]);
  }
});
