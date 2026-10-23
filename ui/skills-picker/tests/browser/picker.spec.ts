import { test, expect } from '@playwright/test';
import { serveFixtures } from './fixtures';

test('cross-folder selection, safe preview, explicit review and completed result', async ({ page }) => {
  const fixture = await serveFixtures(page);
  await page.goto('/#capability=test-capability');
  await expect(page.getByRole('heading', { name: 'Find your next skill.' })).toBeVisible();
  await expect(page).not.toHaveURL(/capability/);
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('textbox', { name: 'Search skills', exact: true }).fill('engineering');
  await page.getByRole('checkbox', { name: 'Select engineering/review', exact: true }).check();
  await expect(page.getByLabel('Selection basket')).toContainText('docs/review');
  await expect(page.getByLabel('Selection basket')).toContainText('engineering/review');
  await page.locator('.skill-description').click();
  await expect(page.getByRole('heading', { name: 'Read carefully' })).toBeVisible();
  await expect(page.locator('.markdown img')).toHaveCount(0);
  await expect(page.locator('.markdown a')).toHaveCount(1);
  expect(await page.evaluate(() => 'injected' in window)).toBe(false);
  await page.getByRole('checkbox', { name: `Target ${fixture.targets.targets[0].label}` }).check();
  await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
  await expect(page.getByRole('dialog')).toContainText('Review installation');
  await expect(page.getByRole('dialog').locator('.plan-action')).toHaveCount(2);
  await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Installation complete' })).toBeVisible();
  expect(fixture.counts()).toMatchObject({ applyCount: 1, planCount: 1, scanCount: 0 });
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Installation complete' })).toBeVisible();
});

test('lost apply response recovers the fast terminal operation without another apply', async ({ page }) => {
  const fixture = await serveFixtures(page, { loseApply: true });
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('checkbox', { name: `Target ${fixture.targets.targets[0].label}` }).check();
  await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
  await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Installation complete' })).toBeVisible();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(fixture.counts().applyCount).toBe(1);
});

test('stale apply refuses confirmation and preserves basket', async ({ page }) => {
  const fixture = await serveFixtures(page, { stale: true });
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('checkbox', { name: `Target ${fixture.targets.targets[0].label}` }).check();
  await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
  await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Plan is stale');
  await expect(page.getByLabel('Selection basket')).toContainText('docs/review');
  await expect(page.getByRole('heading', { name: 'Installation complete' })).toHaveCount(0);
});

test('1000 candidates stay virtualized and local search does not rescan', async ({ page }) => {
  const fixture = await serveFixtures(page, { count: 1000 });
  await page.goto('/#capability=test-capability');
  await expect(page.locator('.skill-card').first()).toBeVisible();
  expect(await page.locator('.skill-card').count()).toBeLessThan(30);
  // Record input-to-next-frame latency without a hardware-dependent timing gate.
  await page.evaluate(() => {
    document.querySelector('#skill-search')!.addEventListener('input', () => {
      performance.mark('picker-search-start');
      const observer = new MutationObserver(() => {
        if (document.querySelectorAll('.skill-card').length !== 1) return;
        observer.disconnect();
        requestAnimationFrame(() => performance.mark('picker-search-visible'));
      });
      observer.observe(document.querySelector('.results')!, { childList: true, subtree: true });
    }, { once: true });
  });
  await page.getByRole('textbox', { name: 'Search skills', exact: true }).fill('skill-999');
  await expect(page.locator('.skill-card')).toHaveCount(1);
  expect(fixture.counts().scanCount).toBe(0);
  await page.waitForFunction(() => performance.getEntriesByName('picker-search-visible').length > 0);
  const duration = await page.evaluate(() => performance.measure('picker-search', 'picker-search-start', 'picker-search-visible').duration);
  await test.info().attach('local-search-timing.json', { body: JSON.stringify({ candidates: 1000, viewport: page.viewportSize(), inputToFrameMs: duration }), contentType: 'application/json' });
  console.info(`Local search: 1000 candidates, ${duration.toFixed(1)} ms input-to-frame at 1440×1000.`);
  const checkbox = await page.getByRole('checkbox', { name: 'Select category-19/skill-999', exact: true }).boundingBox();
  expect(checkbox!.width).toBeGreaterThanOrEqual(24); expect(checkbox!.height).toBeGreaterThanOrEqual(24);
  await page.screenshot({ path: 'test-results/picker-desktop.png', fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole('tab', { name: 'skills', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/picker-mobile.png', fullPage: true });
});

test('tree keyboard keeps focus, preview and checking separate; folder checks preserve other folders', async ({ page }) => {
  await serveFixtures(page);
  await page.goto('/#capability=test-capability');
  const docs = page.getByRole('treeitem').filter({ hasText: /^docs$/ });
  await docs.click();
  await expect(page.getByLabel('Selection basket')).not.toContainText('docs/review');
  await docs.press('Space');
  await expect(page.getByLabel('Selection basket')).toContainText('docs/review');
  await page.getByRole('button', { name: 'All skills', exact: false }).click();
  await page.getByRole('checkbox', { name: 'Select engineering/review', exact: true }).check();
  await docs.press('Space');
  await expect(page.getByLabel('Selection basket')).not.toContainText('docs/review');
  await expect(page.getByLabel('Selection basket')).toContainText('engineering/review');
  await docs.press('ArrowRight');
  await expect(page.getByRole('button', { name: 'Collapse docs', exact: true })).toBeVisible();
  await page.getByRole('checkbox', { name: 'Select all 1 in docs', exact: true }).check();
  await expect(docs).toBeFocused();
});

test('an explicit rescan clears old snapshot selection and preview', async ({ page }) => {
  const fixture = await serveFixtures(page);
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.locator('.skill-description').first().click();
  await page.getByRole('button', { name: 'Change scan scope', exact: false }).click();
  await expect(page.getByRole('dialog')).toContainText('clears your 1 selected skills');
  await page.getByRole('textbox', { name: 'Repository folder' }).fill('docs');
  await page.getByRole('button', { name: 'Scan folder', exact: true }).click();
  await expect(page.getByLabel('Selection basket')).not.toContainText('docs/review');
  await expect(page.getByRole('heading', { name: 'A closer look' })).toBeVisible();
  expect(fixture.counts().scanCount).toBe(1);
});

test('available targets stay prominent while unavailable reasons remain expandable', async ({ page }) => {
  await serveFixtures(page, { unavailableTargets: 10 });
  await page.goto('/#capability=test-capability');
  const targets = page.getByLabel('Install targets', { exact: true });
  await expect(targets.getByRole('checkbox')).toHaveCount(2);
  const summary = targets.locator('summary');
  await expect(summary).toHaveText('10 unavailable targets');
  await expect(targets.getByText('This client is not detected; install it before selecting it.').first()).not.toBeVisible();
  await summary.focus();
  await summary.press('Enter');
  await expect(targets.getByRole('checkbox')).toHaveCount(12);
  await expect(targets.getByRole('checkbox', { name: 'Target Unavailable client 0', exact: true })).toBeDisabled();
  await expect(targets.getByText('This client is not detected; install it before selecting it.').first()).toBeVisible();
});
