import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from '@playwright/test';
import { hydrated, offline } from './i18n-state.helpers';

for (const width of [1440, 390]) {
  test(`dependency diagram uses compact real layout and reveals children once at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: 'no-preference' });
    const errors = await offline(page);
    await page.goto('./');
    await hydrated(page);
    const scene = page.locator('.dependency-scene');
    await expect(scene).toHaveClass(/dependency-scene--animate/);
    await expect(scene).not.toHaveClass(/dependency-scene--revealed/);
    await expect(scene.getByText('Demonstration package', { exact: true })).toHaveCount(0);
    await expect(scene.locator('[role="img"]')).toHaveAccessibleName(/Playwright/);
    const graph = await scene.locator('.dependency-scene__graph').boundingBox();
    expect(graph!.height).toBe(width > 900 ? 352 : 544);
    if (width > 900) expect((await scene.boundingBox())!.width).toBe(944);
    await scene.scrollIntoViewIfNeeded();
    await expect(scene).toHaveClass(/dependency-scene--revealed/);
    await scene.evaluate(async (node) => {
      await Promise.all(
        node.getAnimations({ subtree: true }).map((animation) => animation.finished),
      );
    });
    for (const node of await scene.locator('.dependency-scene__dependency').all()) {
      await expect(node).toHaveCSS('opacity', '1');
      expect(await node.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(
        true,
      );
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    const shots = process.env.UAP_LANDING_SCREENSHOT_DIR;
    if (shots) {
      mkdirSync(shots, { recursive: true });
      await scene.screenshot({ path: resolve(shots, `dependency-scene-${width}.png`) });
    }
    await page.evaluate(() => window.scrollTo({ top: 0, behavior: 'instant' }));
    await scene.scrollIntoViewIfNeeded();
    expect(
      await scene.evaluate((node) =>
        node
          .getAnimations({ subtree: true })
          .every((animation) => animation.playState === 'finished'),
      ),
    ).toBe(true);
    expect(errors).toEqual([]);
  });
}

test('dependency diagram remains fully visible without motion or JavaScript', async ({
  page,
  browser,
  baseURL,
}) => {
  await offline(page);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('./');
  await hydrated(page);
  const scene = page.locator('.dependency-scene');
  await scene.scrollIntoViewIfNeeded();
  await expect(scene).not.toHaveClass(/dependency-scene--animate/);
  for (const node of await scene.locator('.dependency-scene__dependency').all()) {
    await expect(node).toHaveCSS('opacity', '1');
    await expect(node).toHaveCSS('animation-name', 'none');
  }
  const context = await browser.newContext({ javaScriptEnabled: false });
  try {
    const staticPage = await context.newPage();
    await staticPage.goto(baseURL!);
    await expect(staticPage.locator('.dependency-scene')).not.toHaveClass(
      /dependency-scene--animate/,
    );
    await expect(staticPage.locator('.dependency-scene__dependency')).toHaveCount(5);
    await expect(staticPage.locator('.dependency-scene__dependency').first()).toHaveCSS(
      'opacity',
      '1',
    );
  } finally {
    await context.close();
  }
});
