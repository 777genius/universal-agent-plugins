import { test, expect } from '@playwright/test';
import { fixture, serveFixtures } from './fixtures';
import type { Plan } from '../../src/shared/api/types';

test('2000 candidate bounds retain virtual keyboard focus and modal trigger return', async ({ page }) => {
  await serveFixtures(page, { count: 2000 });
  await page.goto('/#capability=test-capability');
  await expect(page.locator('.skill-card').first()).toBeVisible();
  expect(await page.locator('.skill-card').count()).toBeLessThan(30);
  await page.getByRole('treeitem').first().focus();
  await page.keyboard.press('End');
  await expect(page.getByRole('treeitem', { name: 'category-9', exact: true })).toBeFocused();
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('End');
  const last = page.getByRole('treeitem', { name: 'skill-989', exact: true });
  await expect(last).toBeFocused();
  expect(await page.locator('.tree-scroll').evaluate((element) => element.scrollTop)).toBeGreaterThan(1000);
  expect(await page.getByRole('treeitem').count()).toBeLessThan(30);
  const trigger = page.getByRole('button', { name: 'Change scan scope', exact: false });
  await trigger.click();
  await page.getByRole('dialog').getByRole('button', { name: 'Back', exact: true }).click();
  await expect(trigger).toBeFocused();
});

test('unknown server action cannot reach apply even when confirmable is true', async ({ page }) => {
  const state = await serveFixtures(page);
  const plan = await fixture<Plan>('plan');
  await page.route('**/api/v1/plans', (route) => route.fulfill({ json: { ...plan, confirmable: true, expiresAt: new Date(Date.now() + 600_000).toISOString(), actions: [{ ...plan.actions[0], action: 'unknown-future-action' }] } }));
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
  await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
  await expect(page.getByRole('button', { name: 'Confirm installation', exact: true })).toBeDisabled();
  await expect(page.getByRole('dialog')).toContainText('This plan cannot be confirmed');
  expect(state.counts().applyCount).toBe(0);
});

test('late preview cannot reappear after catalog rescan', async ({ page }) => {
  await serveFixtures(page);
  let release!: () => void;
  const held = new Promise<void>((resolve) => { release = resolve; });
  let started!: () => void;
  const requested = new Promise<void>((resolve) => { started = resolve; });
  let responded!: () => void;
  const completed = new Promise<void>((resolve) => { responded = resolve; });
  await page.route('**/api/v1/candidates/*/preview', async (route) => {
    started(); await held;
    await route.fulfill({ json: { catalogId: 'catalog-fixture', candidateId: 'skill-review-b', path: 'docs/review', content: '# Obsolete preview', format: 'markdown', truncated: false, resources: [], diagnostics: [] } });
    responded();
  });
  await page.goto('/#capability=test-capability');
  await page.locator('.skill-description').first().click();
  await requested;
  await page.getByRole('button', { name: 'Change scan scope', exact: false }).click();
  await page.getByRole('button', { name: 'Scan folder', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Skill preview' })).toHaveCount(0);
  release();
  await completed;
  await expect(page.getByRole('heading', { name: 'Obsolete preview' })).toHaveCount(0);
  await expect(page).not.toHaveURL(/skill-review/);
});

test('mobile, narrow tablet and 200 percent desktop reflow remain usable with reduced motion', async ({ page }) => {
  await serveFixtures(page);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('/#capability=test-capability');
  for (const width of [320, 390, 768, 1024]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(page.getByRole('textbox', { name: 'Search skills', exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    if (width >= 760) expect(await page.locator('.sidebar').evaluate((element) => element.getBoundingClientRect().right <= innerWidth)).toBe(true);
  }
  // 1440 CSS pixels at 200% browser zoom reflow to 720 CSS pixels; use the same
  // layout viewport with doubled device scale rather than a flaky browser-chrome shortcut.
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Emulation.setDeviceMetricsOverride', { width: 720, height: 500, deviceScaleFactor: 2, mobile: false });
  await expect(page.getByRole('tab', { name: /Selection \(0\)/ })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.evaluate(() => {
    const probe = document.createElement('span'); probe.className = 'spin'; probe.id = 'motion-probe'; document.body.append(probe);
  });
  expect(await page.locator('#motion-probe').evaluate((element) => getComputedStyle(element).animationName)).toBe('none');
});

test('desktop preview, selection and targets share the viewport; delayed review restores its trigger', async ({ page }) => {
  const state = await serveFixtures(page, { planDelayMs: 200, unavailableTargets: 10 });
  // Real repositories expose long native paths; they must not enlarge the sidebar.
  state.targets.targets.forEach((target) => { target.displayPath = '/isolated/test-profile/long-native-user-path/managed/client/skills'; });
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('checkbox', { name: 'Select engineering/review', exact: true }).check();
  await page.locator('.skill-description').first().click();
  await expect(page.getByRole('heading', { name: 'Read carefully' })).toBeVisible();
  for (const [width, height] of [[1100, 900], [1280, 900], [1440, 900], [1920, 900], [1280, 720]]) {
    await page.setViewportSize({ width, height });
    await page.screenshot({ path: test.info().outputPath(`desktop-polish-${width}x${height}.png`) });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const footer = await page.locator('.bottom-bar').boundingBox();
    for (const selector of ['.desktop-preview', '.basket', '.targets']) {
      const box = await page.locator(selector).boundingBox();
      expect(box!.x).toBeGreaterThanOrEqual(0); expect(box!.x + box!.width).toBeLessThanOrEqual(width);
      expect(box!.y + box!.height).toBeLessThan(footer!.y);
    }
  }
  await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
  const trigger = page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' });
  await trigger.click();
  await expect(page.getByRole('dialog', { name: 'Review installation' })).toBeVisible();
  await page.getByRole('button', { name: 'Back', exact: true }).click();
  await expect(trigger).toBeFocused();
  await trigger.press('Enter');
  await expect(page.getByRole('dialog', { name: 'Review installation' })).toBeVisible();
  await page.getByRole('dialog').getByRole('button', { name: 'Close', exact: true }).click();
  await expect(trigger).toBeFocused();
  await page.getByRole('button', { name: 'Close preview', exact: true }).click();
  await expect(page.locator('.desktop-preview')).toHaveCount(0);
});

test('invalid metadata keeps a readable title, full path and useful validation reason', async ({ page }) => {
  const state = await serveFixtures(page);
  const candidate = state.catalog.candidates.find((item) => item.path === 'docs/review')!;
  Object.assign(candidate, { name: '', description: '', status: 'invalid', diagnosticCodes: ['skill_name_mismatch'] });
  await page.goto('/#capability=test-capability');
  const card = page.locator('.skill-card').filter({ hasText: 'docs/review' });
  await expect(card.locator('strong')).toHaveText('review');
  await expect(card).toContainText('The declared skill name does not match its folder name.');
  await expect(card.getByRole('checkbox')).toBeDisabled();
  await expect(card.locator('.unavailable')).toHaveAttribute('title', 'skill_name_mismatch');
});

test('1000 candidate folder expansion records input to rendered frame without a timing gate', async ({ page }) => {
  await serveFixtures(page, { count: 1000 });
  await page.goto('/#capability=test-capability');
  const category = page.getByRole('treeitem', { name: 'category-0', exact: true });
  await category.focus();
  await page.evaluate(() => {
    document.querySelector('.tree-scroll')!.addEventListener('keydown', () => {
      performance.mark('picker-expand-start');
      const renderedFrame = () => {
        if (document.querySelector('[aria-label="Collapse category-0"]')) performance.mark('picker-expand-visible');
        else requestAnimationFrame(renderedFrame);
      };
      requestAnimationFrame(renderedFrame);
    }, { once: true, capture: true });
  });
  await category.press('ArrowRight');
  await expect(page.getByRole('button', { name: 'Collapse category-0', exact: true })).toBeVisible();
  await page.waitForFunction(() => performance.getEntriesByName('picker-expand-visible').length > 0);
  const duration = await page.evaluate(() => performance.measure('picker-expand', 'picker-expand-start', 'picker-expand-visible').duration);
  console.info(`Folder expansion:1000 candidates, ${duration.toFixed(1)} ms input-to-frame.`);
  await test.info().attach('folder-expansion-timing.json', { body: JSON.stringify({ candidates: 1000, inputToFrameMs: duration }), contentType: 'application/json' });
  await expect(category).toBeFocused();
  expect(await page.getByRole('treeitem').count()).toBeLessThan(30);
});

test('an idle open review expires visibly and cannot apply', async ({ page }) => {
  const state = await serveFixtures(page);
  const plan = await fixture<Plan>('plan');
  await page.clock.install();
  await page.route('**/api/v1/plans', (route) => route.fulfill({ json: { ...plan, expiresAt: new Date(Date.now() + 2000).toISOString() } }));
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
  await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
  const confirm = page.getByRole('button', { name: 'Confirm installation', exact: true });
  await expect(confirm).toBeEnabled();
  await page.clock.runFor(2500);
  await expect(confirm).toBeDisabled();
  await expect(page.getByRole('dialog')).toContainText('This plan expired. Close and review again.');
  expect(state.counts().applyCount).toBe(0);
});
