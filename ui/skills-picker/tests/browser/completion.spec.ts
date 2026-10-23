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
  await expect(page.getByRole('heading', { name: 'A closer look' })).toBeVisible();
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
