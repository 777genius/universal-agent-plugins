import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import { clientLandingPages } from '../../data/clients';
import { hydrated, setSignedMirrorTestTime, wording } from './i18n-state.helpers';

const exactCommand = 'npx -y universal-agent-plugins add context7 --target codex,cursor,vscode --no-save';

async function capture(page: Page, name: string) {
  const directory = process.env.UAP_LANDING_SCREENSHOT_DIR;
  if (!directory) return;
  mkdirSync(directory, { recursive: true });
  await page.screenshot({ path: resolve(directory, `${name}.png`) });
}

async function waitForCommand(page: Page) {
  await hydrated(page);
  await expect(page.locator('.hero__window .command-snippet')).toBeVisible({
    timeout: 15_000,
  });
}

test.beforeEach(async ({ page }) => {
  await setSignedMirrorTestTime(page);
  await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
});

test('hero controls and all client links remain complete across four viewports', async ({
  page,
}) => {
  for (const width of [1440, 1024, 390, 320]) {
    await page.setViewportSize({ width, height: width >= 1024 ? 1000 : 844 });
    await page.goto('./');
    await waitForCommand(page);

    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);

    const items = page.locator('.client-strip > li');
    await expect(items).toHaveCount(clientLandingPages.length);
    const geometry = await items.evaluateAll((nodes) =>
      nodes.map((node) => {
        const rect = node.getBoundingClientRect();
        return {
          x: rect.x,
          y: rect.y,
          width: rect.width,
          right: rect.right,
          center: rect.x + rect.width / 2,
        };
      }),
    );
    const rows = [...new Set(geometry.map((item) => Math.round(item.y)))];
    if (width >= 1440) {
      expect(rows).toHaveLength(1);
    } else if (width < 1024) {
      expect(rows.length).toBeGreaterThanOrEqual(3);
    }
    for (const item of geometry) {
      expect(item.x).toBeGreaterThanOrEqual(0);
      expect(item.right).toBeLessThanOrEqual(width + 0.5);
    }
    expect(
      await page
        .locator('.client-strip__copy strong')
        .evaluateAll((nodes) =>
          nodes.every(
            (node) =>
              getComputedStyle(node).textOverflow !== 'ellipsis' &&
              node.scrollWidth <= node.clientWidth,
          ),
        ),
    ).toBe(true);

    const captions = page.locator('.hero-quick-start__caption');
    const controls = page.locator(
      '.hero-quick-start__step .app-multiselect__trigger, .hero-quick-start__command',
    );
    await expect(captions).toHaveCount(2);
    for (let index = 0; index < 2; index += 1) {
      const caption = (await captions.nth(index).boundingBox())!;
      const control = (await controls.nth(index).boundingBox())!;
      const step = (await page.locator('.hero-quick-start__step').nth(index).boundingBox())!;
      expect(caption.height).toBeLessThanOrEqual(24);
      expect(caption.y + caption.height).toBeLessThanOrEqual(control.y + 0.5);
      expect(control.width).toBeGreaterThanOrEqual(step.width - 1);
    }

    const code = page.locator('.hero-quick-start__command pre');
    const copy = page.locator('.hero-quick-start__command button');
    const codeBox = (await code.boundingBox())!;
    const copyBox = (await copy.boundingBox())!;
    expect(codeBox.x + codeBox.width).toBeLessThanOrEqual(copyBox.x + 0.5);
    expect(await code.evaluate((node) => node.scrollWidth <= node.clientWidth)).toBe(true);

    await capture(page, `hero-layout-${width}`);
  }
});

test('selected targets produce one complete copyable command with keyboard activation', async ({
  page,
}) => {
  test.setTimeout(60_000);
  for (const width of [1440, 320]) {
    await page.setViewportSize({ width, height: width === 1440 ? 1000 : 844 });
    await page.goto('./');
    await waitForCommand(page);

    const trigger = page.locator('.hero-quick-start__step .app-multiselect__trigger');
    await trigger.focus();
    await page.keyboard.press('Enter');
    for (const name of [/^Codex /, /^Cursor /, /^VS Code /]) {
      await page.getByRole('checkbox', { name }).click();
    }
    await page.keyboard.press('Escape');
    await expect(trigger).toBeFocused();

    const command = page.locator('.hero-quick-start__command');
    await expect(command.locator('code')).toHaveText(exactCommand);
    const copy = command.locator('button');
    await expect(copy).toHaveAccessibleName('Copy command');
    await copy.focus();
    await page.keyboard.press('Enter');
    await expect(copy).toHaveAccessibleName(wording('en')('registryUi.command.copiedLabel'));
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(exactCommand);
    await capture(page, `hero-command-${width}`);
  }
});

test('catalog reveal is early on navigation, compact trust badges stay compact, and video preview waits for a real Mux asset', async ({
  page,
}) => {
  test.setTimeout(60_000);
  await page.emulateMedia({ reducedMotion: 'no-preference' });
  await page.setViewportSize({ width: 1440, height: 600 });
  await page.goto('./');
  await expect(page.locator('html')).toHaveClass(/scroll-reveal-active/, { timeout: 15_000 });
  const catalog = page.locator('.catalog-wrap');
  await catalog.evaluate((node) => {
    window.scrollTo({
      top: node.getBoundingClientRect().top + window.scrollY - window.innerHeight + 20,
      behavior: 'instant',
    });
  });
  await expect
    .poll(() => catalog.evaluate((node) => node.getBoundingClientRect().top))
    .toBeLessThanOrEqual(580);
  await expect(catalog).toHaveClass(/scroll-reveal--visible/);
  await page.goto('./');
  await page.locator('.hero__actions .button--primary').click();
  await expect(catalog).toHaveClass(/scroll-reveal--visible/);
  await expect(catalog).toHaveCSS('opacity', '1');

  await page.goto('./#plugins');
  await expect(catalog).toHaveClass(/scroll-reveal--visible/);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('./');
  await expect(catalog).toHaveClass(/scroll-reveal--visible/);

  await page.goto('./plugins/');
  await expect(page.locator('.catalog')).toHaveAttribute('data-discovery-state', /current|cached/, {
    timeout: 15_000,
  });
  await page.getByRole('searchbox', { name: 'Search plugins' }).fill('context7');
  const badge = page.locator('.plugin-card__ribbon').first();
  await expect(badge).toHaveText('Reviewed');
  const badgeBox = (await badge.boundingBox())!;
  expect(badgeBox.width).toBeLessThan(90);

  await page.goto('./');
  await expect(page.locator('.hero-demo-video')).toBeVisible();
  await expect(page.locator('.hero-demo-video button')).toHaveAccessibleName('Watch demo');
  const preview = (await page.locator('.hero-demo-video').boundingBox())!;
  const install = (await page.locator('.hero__window').boundingBox())!;
  expect(preview.width).toBe(480);
  expect(install.y - (preview.y + preview.height)).toBeGreaterThanOrEqual(27);
  await expect(page.locator('iframe[src*="player.mux.com"]')).toHaveCount(0);
  await expect(page.locator('.hero-quick-start__footer')).toHaveCount(0);
  await capture(page, 'hero-early-reveal');
});
