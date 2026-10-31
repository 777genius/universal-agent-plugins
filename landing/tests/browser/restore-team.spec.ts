import { expect, test } from '@playwright/test';
import { hydrated } from './i18n-state.helpers';

const commands = [
  'agentplugins add <git-url> --scope project',
  'agentplugins install',
];

// Exercise actual page geometry: long translations, both themes, and the four
// agreed breakpoints. Text must stay inside the display, including Copy controls.
for (const theme of ['dark', 'light']) {
  for (const [locale, width] of [
    ['en', 1440],
    ['en', 1024],
    ['en', 768],
    ['en', 390],
    ['fr', 390],
    ['ru', 390],
    ['ar', 390],
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
      await expect(section.locator('pre code')).toHaveText(commands);
      expect(
        await section.locator('.registry-restore__screen').evaluateAll((screens) =>
          screens.flatMap((screen) => {
            const bounds = screen.getBoundingClientRect();
            return [
              ...screen.querySelectorAll(
                'pre, button, .registry-restore__stage-label, .registry-restore__commit',
              ),
            ]
              .filter((element) => {
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
      if (width === 1440) expect(machines[0]!.top).toBeCloseTo(machines[1]!.top, 0);
      else expect(machines[1]!.top).toBeGreaterThan(machines[0]!.bottom);
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
