import { expect, test } from '@playwright/test';
import { clients } from '../../data/clients';

const uniqueLogos = clients.filter((client, index) => clients.findIndex((item) => item.icon === client.icon) === index);

test('CSS background works without JavaScript and keeps unique logos on the circle', async ({ browser, baseURL }) => {
  const context = await browser.newContext({ baseURL, javaScriptEnabled: false, reducedMotion: 'reduce' });
  const page = await context.newPage();
  try {
    await page.goto('./');
    const field = page.locator('.hero__demo .hero-agent-field');
    await expect(field).toHaveAttribute('aria-hidden', 'true');
    await expect(field.locator('[data-client-id]')).toHaveCount(uniqueLogos.length);
    await expect(field.locator('img[src$="/openai.svg"]')).toHaveCount(1);
    for (const client of uniqueLogos) {
      const logo = field.locator(`[data-client-id="${client.id}"] img`);
      await expect(logo).toHaveAttribute('src', new RegExp(`/client-icons/${client.icon}$`));
    }
    await expect
      .poll(() =>
        field
          .locator('[data-client-id] img')
          .evaluateAll((images: HTMLImageElement[]) =>
            images.every((image) => image.complete && image.naturalWidth > 0),
          ),
      )
      .toBe(true);
    // Flatten the camera only; retain the historical spoke/circumference geometry.
    await field.locator('.hero-agent-field__plane').evaluate((node: HTMLElement) => {
      // CSS animations outrank ordinary inline declarations while they are
      // settling. Important keeps this geometry probe deterministic even when
      // the full browser suite starts several contexts at once.
      node.style.setProperty('transform', 'none', 'important');
    });
    await expect.poll(() => field.locator('.hero-agent-field__plane')
      .evaluate((node) => getComputedStyle(node).transform)).toBe('none');
    const error = await field.evaluate((element) => {
      const track = element.querySelector('.hero-agent-field__track')!.getBoundingClientRect();
      return Math.max(...[...element.querySelectorAll('.hero-agent-field__node')].map((node) => {
        const rect = node.getBoundingClientRect();
        return Math.abs(Math.hypot(rect.x + rect.width / 2 - track.x - track.width / 2,
          rect.y + rect.height / 2 - track.y - track.height / 2) - track.width / 2);
      }));
    });
    expect(error).toBeLessThan(2);
    await expect(field.locator('canvas')).toHaveCount(0);
  } finally {
    await context.close();
  }
});

test('CSS background depth animates on the right and pauses offscreen or hidden', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('./');
  const field = page.locator('.hero__demo .hero-agent-field');
  const orbit = field.locator('.hero-agent-field__orbit');
  await expect(field).toHaveClass(/hero-agent-field--active/);
  const first = await orbit.evaluate((node) => getComputedStyle(node).transform);
  await expect.poll(() => orbit.evaluate((node) => getComputedStyle(node).transform)).not.toBe(first);
  expect(await field.locator('.hero-agent-field__plane').evaluate((node) => getComputedStyle(node).transform)).toMatch(/^matrix3d\(/);
  expect(await field.evaluate((node) => getComputedStyle(node).pointerEvents)).toBe('none');
  const copy = (await page.locator('.hero__copy').boundingBox())!;
  const bounds = (await field.boundingBox())!;
  const installation = (await page.locator('.hero__window').boundingBox())!;
  expect(bounds.x).toBeGreaterThan(copy.x + copy.width);
  expect(bounds.y).toBeCloseTo(installation.y, 1);
  expect(bounds.height).toBeCloseTo(installation.height, 1);
  expect(await field.evaluate((node) => getComputedStyle(node).position)).toBe('absolute');
  const sceneAnimations = await field.evaluate((node) => node.getAnimations({ subtree: true })
    .map((animation) => (animation as CSSAnimation).animationName));
  expect(sceneAnimations).toContain('agent-orbit-breathe');
  const tiltMatrices: number[][] = [];
  for (const progress of [0, 1]) {
    tiltMatrices.push(await field.evaluate((node, fraction) => {
      const plane = node.querySelector('.hero-agent-field__plane')!;
      const tilt = plane.getAnimations().find((animation) =>
        (animation as CSSAnimation).animationName === 'agent-plane-tilt')!;
      tilt.pause();
      tilt.currentTime = Number(tilt.effect!.getTiming().duration) * fraction;
      const matrix = new DOMMatrix(getComputedStyle(plane).transform);
      return [matrix.m13, matrix.m23];
    }, progress));
  }
  // Real out-of-plane rotation changes depth on both axes, not just a flat spin.
  expect(Math.abs(tiltMatrices[0]![0]! - tiltMatrices[1]![0]!)).toBeGreaterThan(0.5);
  expect(Math.abs(tiltMatrices[0]![1]! - tiltMatrices[1]![1]!)).toBeGreaterThan(0.5);
  await page.getByRole('contentinfo').scrollIntoViewIfNeeded();
  await expect(field).not.toHaveClass(/hero-agent-field--active/);
  expect(await orbit.evaluate((node) => getComputedStyle(node).animationPlayState)).toBe('paused');
  await field.scrollIntoViewIfNeeded();
  await expect(field).toHaveClass(/hero-agent-field--active/);
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { value: true, configurable: true });
    document.dispatchEvent(new Event('visibilitychange'));
  });
  await expect(field).not.toHaveClass(/hero-agent-field--active/);
  await page.evaluate(() => {
    Reflect.deleteProperty(document, 'hidden');
    document.dispatchEvent(new Event('visibilitychange'));
  });
  await expect(field).toHaveClass(/hero-agent-field--active/);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  expect(await field.evaluate((node) => node.getAnimations({ subtree: true }).length)).toBe(0);
});

test('logos travel clockwise along a stationary ring and stay upright', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('./');
  const field = page.locator('.hero-agent-field');
  await expect(field).toHaveClass(/hero-agent-field--active/);
  await expect(field.locator('.hero-agent-field__orbit')).toHaveCSS('animation-duration', '36s');
  await expect(field.locator('.hero-agent-field__rotor').first()).toHaveCSS('animation-duration', '36s');
  expect(await field.locator('.hero-agent-field__track').evaluate((node) => node.getAnimations().length)).toBe(0);
  // Isolate travel from the camera tilt: a fixed ring alone cannot make this pass.
  await field.locator('.hero-agent-field__plane').evaluate((node: HTMLElement) => {
    node.style.animation = 'none';
    node.style.transform = 'none';
  });
  const positions: { x: number; y: number; logoWidth: number; logoHeight: number }[][] = [];
  for (const fraction of [0, 0.125]) {
    await field.evaluate((node, progress) => {
      for (const animation of node.getAnimations({ subtree: true })) {
        animation.pause();
        animation.currentTime = Number(animation.effect!.getTiming().duration) * progress;
      }
    }, fraction);
    await page.evaluate(() => new Promise(requestAnimationFrame));
    positions.push(await field.evaluate((node) => {
      const ring = node.querySelector('.hero-agent-field__track')!.getBoundingClientRect();
      return [...node.querySelectorAll('.hero-agent-field__node')].map((badge) => {
        const rect = badge.getBoundingClientRect();
        const logo = badge.querySelector('img')!.getBoundingClientRect();
        return { x: rect.x + rect.width / 2 - ring.x - ring.width / 2,
          y: rect.y + rect.height / 2 - ring.y - ring.height / 2,
          logoWidth: logo.width, logoHeight: logo.height };
      });
    }));
  }
  for (const [index, start] of positions[0]!.entries()) {
    const moved = positions[1]![index]!;
    // +45 degrees is clockwise in screen coordinates (positive y points down).
    expect(moved.x).toBeCloseTo((start.x - start.y) / Math.SQRT2, 0);
    expect(moved.y).toBeCloseTo((start.x + start.y) / Math.SQRT2, 0);
    expect(Math.hypot(moved.x, moved.y)).toBeCloseTo(Math.hypot(start.x, start.y), 0);
    // At 45 degrees a missing counter-rotation enlarges the image bounding box.
    expect(moved.logoWidth).toBeCloseTo(start.logoWidth, 1);
    expect(moved.logoHeight).toBeCloseTo(start.logoHeight, 1);
  }
});

test('larger breathing background never changes layout or blocks the foreground', async ({ page }) => {
  await page.goto('./');
  const field = page.locator('.hero-agent-field');
  await expect(field).toHaveClass(/hero-agent-field--active/);
  await expect(field).toHaveCSS('perspective', '1125px');
  for (const width of [1440, 1280, 1024, 800, 390, 280]) {
    await page.setViewportSize({ width, height: 1000 });
    await field.scrollIntoViewIfNeeded();
    const windowBefore = (await page.locator('.hero__window').boundingBox())!;
    const heroBefore = (await page.locator('.hero').boundingBox())!;
    const planeWidth = await field.locator('.hero-agent-field__plane')
      .evaluate((node) => Number.parseFloat(getComputedStyle(node).width));
    const fieldWidth = (await field.boundingBox())!.width;
    expect(await field.locator('.hero-agent-field__plane')
      .evaluate((node) => Number.parseFloat(getComputedStyle(node).left))).toBeCloseTo(fieldWidth * 0.62 - 100, 1);
    expect(planeWidth).toBeCloseTo(Math.min(width <= 720 ? 492 : 720, fieldWidth * 1.5), 1);
    expect(await field.locator('.hero-agent-field__node').first()
      .evaluate((node) => Number.parseFloat(getComputedStyle(node).width))).toBe(width <= 720 ? 30 : 38);
    expect(await field.locator('.hero-agent-field__hub')
      .evaluate((node) => Number.parseFloat(getComputedStyle(node).width))).toBe(width <= 720 ? 96 : 116);
    expect(await field.locator('.hero-agent-field__hub img')
      .evaluate((node) => Number.parseFloat(getComputedStyle(node).width))).toBe(width <= 720 ? 64 : 76);
    await field.evaluate((node: HTMLElement) => { node.style.display = 'none'; });
    expect(await page.locator('.hero__window').boundingBox()).toEqual(windowBefore);
    expect(await page.locator('.hero').boundingBox()).toEqual(heroBefore);
    await field.evaluate((node: HTMLElement) => { node.style.removeProperty('display'); });
    for (const time of [0, 5000, 10000, 36000, 54000]) {
      await field.evaluate((node, elapsed) => {
        for (const animation of node.getAnimations({ subtree: true })) {
          animation.pause();
          animation.currentTime = elapsed;
        }
      }, time);
      await page.evaluate(() => new Promise(requestAnimationFrame));
      const installation = (await page.locator('.hero__window').boundingBox())!;
      expect(installation).toEqual(windowBefore);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await field.scrollIntoViewIfNeeded();
  const sizes: number[] = [];
  for (const progress of [0, 0.5, 1]) {
    await field.evaluate((node, fraction) => {
      for (const animation of node.getAnimations({ subtree: true })) {
        animation.pause();
        animation.currentTime = (animation as CSSAnimation).animationName === 'agent-orbit-breathe'
          ? Number(animation.effect!.getTiming().duration) * fraction : 0;
      }
    }, progress);
    await page.evaluate(() => new Promise(requestAnimationFrame));
    sizes.push((await field.locator('.hero-agent-field__track').boundingBox())!.width);
  }
  expect(sizes[1]!).toBeGreaterThan(sizes[0]! * 1.05);
  expect(sizes[1]!).toBeLessThan(sizes[0]! * 1.25);
  expect(sizes[2]!).toBeCloseTo(sizes[0]!, 1);
});

test('logo thickness follows the viewing angle without extra animation loops', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('./');
  const field = page.locator('.hero-agent-field');
  await expect(field).toHaveClass(/hero-agent-field--active/);
  await expect(field.locator('.hero-agent-field__rim')).toHaveCount(uniqueLogos.length * 4);
  expect(await field.locator('.hero-agent-field__rotor').first().locator('.hero-agent-field__rim')
    .evaluateAll((rims) => rims.map((rim) => new DOMMatrix(getComputedStyle(rim).transform).m43)))
    .toEqual([-1.5, -3, -4.5, -6]);
  // Only the plane, icon orbit and counter-rotations animate, never individual slices.
  expect(await field.evaluate((node) => node.getAnimations({ subtree: true }).length)).toBe(uniqueLogos.length + 3);
  const offsets: { x: number; y: number }[][] = [];
  for (const fraction of [0, 1]) {
    await field.evaluate((node, progress) => {
      for (const animation of node.getAnimations({ subtree: true })) {
        animation.pause();
        animation.currentTime = (animation as CSSAnimation).animationName === 'agent-plane-tilt'
          ? Number(animation.effect!.getTiming().duration) * progress : 0;
      }
    }, fraction);
    await page.evaluate(() => new Promise(requestAnimationFrame));
    offsets.push(await field.locator('.hero-agent-field__rotor').evaluateAll((rotors) => rotors.map((rotor) => {
      const front = rotor.querySelector('.hero-agent-field__face')!.getBoundingClientRect();
      const back = rotor.querySelector('.hero-agent-field__rim:nth-child(4)')!.getBoundingClientRect();
      return { x: back.x + back.width / 2 - front.x - front.width / 2,
        y: back.y + back.height / 2 - front.y - front.height / 2 };
    })));
  }
  for (const [index, first] of offsets[0]!.entries()) {
    const opposite = offsets[1]![index]!;
    // Projected rear/front displacement, not just CSS strings: a flattened stack fails here.
    expect(Math.hypot(first.x, first.y)).toBeGreaterThan(1);
    expect(Math.hypot(opposite.x, opposite.y)).toBeGreaterThan(1);
    expect(first.x * opposite.x + first.y * opposite.y).toBeLessThan(-1);
  }
  await page.emulateMedia({ reducedMotion: 'reduce' });
  expect(await field.evaluate((node) => node.getAnimations({ subtree: true }).length)).toBe(0);
  await expect(field.locator('.hero-agent-field__face')).toHaveCount(uniqueLogos.length);
});

test('installation works with WebGL forbidden and the orbit has no canvas or optional renderer', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.addInitScript(() => {
    const original = HTMLCanvasElement.prototype.getContext;
    HTMLCanvasElement.prototype.getContext = function (type: string, ...args: unknown[]) {
      if (/webgl/i.test(type)) throw new Error('This CSS-only page must not create WebGL');
      return original.apply(this, [type, ...args] as Parameters<typeof original>);
    } as typeof original;
  });
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('./');
  const field = page.locator('.hero-agent-field');
  await expect(field).toHaveClass(/hero-agent-field--active/);
  await expect(field.locator('canvas')).toHaveCount(0);
  await page.getByRole('button', { name: 'Choose target agents: All installed agents', exact: true }).click();
  await page.getByRole('checkbox', { name: /^Cursor / }).click();
  await page.keyboard.press('Escape');
  await expect(page.locator('.hero__demo .command-snippet')).toContainText('--target cursor');
  await page.getByRole('button', { name: 'Toggle theme', exact: true }).click();
  await expect(field.locator('[data-client-id]')).toHaveCount(uniqueLogos.length);
  // Only decorative logos are deduplicated, never the actual install targets.
  await page.getByRole('button', { name: /Choose target agents:/ }).click();
  await expect(page.getByRole('checkbox', { name: /^Codex / })).toBeVisible();
  await expect(page.getByRole('checkbox', { name: /^ChatGPT / })).toBeVisible();
  await page.keyboard.press('Escape');
  expect(errors).toEqual([]);
});
