import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import { clientLandingPages } from '../../data/clients';
import { hydrated, setSignedMirrorTestTime, wording } from './i18n-state.helpers';

const exactCommand =
  'npx -y universal-agent-plugins@latest add context7 --target "codex,cursor,copilot"';

async function capture(page: Page, name: string) {
  const directory = process.env.UAP_LANDING_SCREENSHOT_DIR;
  if (!directory) return;
  mkdirSync(directory, { recursive: true });
  await page.screenshot({ path: resolve(directory, `${name}.png`) });
}

type TileBox = { x: number; y: number; width: number; right: number };

// Every agent tile has the same width, and a short last row sits centered
// under the full rows instead of leaving a hole on one side.
async function expectCenteredEqualTiles(page: Page, tiles: TileBox[], rows: number[]) {
  const widths = tiles.map((tile) => tile.width);
  expect(Math.max(...widths) - Math.min(...widths)).toBeLessThanOrEqual(0.5);
  const strip = (await page.locator('.client-strip').boundingBox())!;
  const center = strip.x + strip.width / 2;
  for (const row of rows) {
    const items = tiles.filter((tile) => Math.round(tile.y) === row);
    const left = Math.min(...items.map((tile) => tile.x));
    const right = Math.max(...items.map((tile) => tile.right));
    expect(Math.abs((left + right) / 2 - center)).toBeLessThanOrEqual(1);
  }
}

async function tileGeometry(page: Page) {
  const tiles = await page.locator('.client-strip > li').evaluateAll((nodes) =>
    nodes.map((node) => {
      const rect = node.getBoundingClientRect();
      return { x: rect.x, y: rect.y, width: rect.width, right: rect.right };
    }),
  );
  return { tiles, rows: [...new Set(tiles.map((tile) => Math.round(tile.y)))] };
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

for (const width of [1440, 1024, 390, 320]) {
  test(`hero controls and all client links remain complete at ${width}px`, async ({ page }) => {
    // Validate every client link and loaded icon at four widths without a CI-contention timeout.
    test.setTimeout(60_000);
    await page.setViewportSize({ width, height: width >= 1024 ? 1000 : 844 });
    await page.goto('./');
    await waitForCommand(page);

    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);

    await expect(page.locator('.hero-quick-start__header h2')).toHaveText('Install the Context7 plugin');
    const standards = page.locator('.hero__standards');
    await expect(standards.locator('.hero__standards-label')).toHaveText('Standards');
    await expect(standards.locator('a')).toHaveText(['Agent Plugins 1.0', 'Agent Skills']);
    await expect(standards.locator('a').nth(0)).toHaveAttribute(
      'href',
      'https://agent-plugins.org/specification',
    );
    await expect(standards.locator('a').nth(1)).toHaveAttribute(
      'href',
      'https://agentskills.io/specification',
    );
    await expect(standards.locator('a').nth(1)).toBeVisible();

    const items = page.locator('.client-strip > li');
    await expect(items).toHaveCount(clientLandingPages.length);
    // Verify exact deployment-base URLs: suffix matches could hide a broken prefix.
    const baseURL = test.info().project.use.baseURL;
    if (!baseURL) throw new Error('Browser baseURL is required');
    const basePath = new URL(baseURL).pathname;
    for (const [index, client] of clientLandingPages.entries()) {
      const item = items.nth(index);
      await expect(item.locator('.client-strip__copy strong')).toHaveText(client.name);
      await expect(item.locator('a')).toHaveAttribute('href', `${basePath}agents/${client.slug}/`);
      const image = item.locator('img');
      await expect(image).toHaveAttribute('src', `${basePath}client-icons/${client.icon}`);
      if (width === 1440) {
        await expect
          .poll(() => image.evaluate((node) => (node as HTMLImageElement).naturalWidth))
          .toBeGreaterThan(0);
      }
    }
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
    const perRow = geometry.filter((item) => Math.round(item.y) === rows[0]).length;
    const install = (await page.locator('.hero-quick-start').boundingBox())!;
    const agents = (await page.locator('.client-section__agents').boundingBox())!;
    if (width >= 1024) {
      // Two columns: the install steps sit beside a wrapping agent grid.
      expect(install.x + install.width).toBeLessThanOrEqual(agents.x + 0.5);
      expect(rows.length).toBeGreaterThanOrEqual(2);
      expect(perRow).toBeGreaterThanOrEqual(4);
    } else {
      // Stacked: install steps first, then several agents per row.
      expect(install.y + install.height).toBeLessThanOrEqual(agents.y + 0.5);
      expect(rows.length).toBeGreaterThanOrEqual(3);
      expect(perRow).toBeGreaterThanOrEqual(3);
    }
    for (const item of geometry) {
      expect(item.x).toBeGreaterThanOrEqual(0);
      expect(item.right).toBeLessThanOrEqual(width + 0.5);
    }
    await expectCenteredEqualTiles(page, geometry, rows);
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
  });
}

test('right-to-left agent rows stay centered with equal tiles', async ({ page }) => {
  for (const width of [1440, 768, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('./ar/');
    await hydrated(page);
    await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
    await expect(page.locator('.client-strip > li')).toHaveCount(clientLandingPages.length);
    const { tiles, rows } = await tileGeometry(page);
    expect(rows.length).toBeGreaterThanOrEqual(2);
    await expectCenteredEqualTiles(page, tiles, rows);
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
    await expect(page.getByRole('checkbox', { name: /^VS Code / })).toHaveCount(0);
    for (const name of [/^Codex /, /^Cursor /, /^Copilot /]) {
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
  test.setTimeout(90_000);
  await page.emulateMedia({ reducedMotion: 'no-preference' });
  await page.setViewportSize({ width: 1440, height: 600 });
  const gotoReadyHome = async (url = './') => {
    await page.goto(url);
    await hydrated(page);
    const reducedMotion = await page.evaluate(
      () => window.matchMedia('(prefers-reduced-motion: reduce)').matches,
    );
    // Every full navigation starts a new client lifecycle; reduced motion skips the observer.
    await expect(page.locator('html')).toHaveClass(
      reducedMotion ? /scroll-reveal-ready/ : /scroll-reveal-active/,
      { timeout: 15_000 },
    );
  };
  await gotoReadyHome();
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
  await gotoReadyHome();
  await page.locator('.hero__actions .button--primary').click();
  await expect(catalog).toHaveClass(/scroll-reveal--visible/);
  await expect(catalog).toHaveCSS('opacity', '1');

  await gotoReadyHome('./#plugins');
  await expect(catalog).toHaveClass(/scroll-reveal--visible/);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await gotoReadyHome();
  await expect(catalog).toHaveClass(/scroll-reveal--visible/);

  await page.goto('./plugins/');
  await hydrated(page);
  await expect(page.locator('.catalog')).toHaveAttribute('data-discovery-state', /current|cached/, {
    timeout: 15_000,
  });
  await page.getByRole('searchbox', { name: 'Search plugins' }).fill('context7');
  const badge = page.locator('.plugin-card__ribbon').first();
  await expect(badge).toHaveText('Reviewed');
  const badgeBox = (await badge.boundingBox())!;
  expect(badgeBox.width).toBeLessThan(120);

  await gotoReadyHome();
  await expect(page.locator('.hero-demo-video')).toBeVisible();
  await expect(page.locator('.hero-demo-video button')).toHaveAccessibleName('Watch demo');
  const preview = (await page.locator('.hero-demo-video').boundingBox())!;
  const copy = (await page.locator('.hero__copy').boundingBox())!;
  const supported = (await page.locator('.client-section').boundingBox())!;
  // The video is the hero's right column, above the install steps and agents.
  expect(preview.width).toBeGreaterThanOrEqual(500);
  expect(preview.x).toBeGreaterThanOrEqual(copy.x + copy.width);
  expect(preview.y + preview.height).toBeLessThanOrEqual(supported.y);
  await expect(page.locator('.hero__visual .hero-demo-video')).toHaveCount(1);
  // After seven seconds the hidden warm-up frame is intentional. It must not
  // become an active, focusable or autoplay-enabled player without a gesture.
  const muxFrames = page.locator('iframe[src*="player.mux.com"]');
  await expect(page.locator('iframe[src*="player.mux.com"]:not(.hero-demo-video__preload)')).toHaveCount(0);
  expect(await muxFrames.evaluateAll((frames) => frames.every((frame) =>
    frame.getAttribute('aria-hidden') === 'true' &&
    frame.hasAttribute('inert') && frame.getAttribute('tabindex') === '-1' &&
    !new URL((frame as HTMLIFrameElement).src).searchParams.has('autoplay') &&
    !(frame.getAttribute('allow') || '').includes('autoplay') &&
    getComputedStyle(frame).visibility === 'hidden',
  ))).toBe(true);
  await expect(page.locator('.hero-quick-start__footer')).toHaveCount(0);
  await capture(page, 'hero-early-reveal');
});
