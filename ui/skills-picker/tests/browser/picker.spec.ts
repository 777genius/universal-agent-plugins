import { test, expect } from '@playwright/test';
import { fixture, serveFixtures } from './fixtures';
import type { Operation, Plan, Session } from '../../src/shared/api/types';

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
  await expect(page.getByRole('heading', { name: 'Installation applied' })).toBeVisible();
  await expect(page.getByRole('status', { name: 'Skills readiness' })).toContainText('Installed 2 skills');
  expect(fixture.counts()).toMatchObject({ applyCount: 1, planCount: 1, scanCount: 0 });
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Installation applied' })).toBeVisible();
  await page.getByRole('checkbox', { name: 'Select engineering/review', exact: true }).check();
  await expect(page.getByRole('region', { name: 'Installation progress' })).toContainText('Previous installation result');
});

test('partial result limits file readiness to successful and unchanged targets', async ({ page }) => {
  const state = await serveFixtures(page);
  const operation = await fixture<Operation>('operation');
  await page.route('**/api/v1/operations/*', (route) => route.fulfill({ json: {
    ...operation, state: 'partial', total: 3, completed: 3,
    items: [
      { ...operation.items[0], status: 'succeeded' },
      { ...operation.items[1], status: 'unchanged' },
      { ...operation.items[1], targetId: 'failed-target', status: 'failed', message: 'Write refused' },
    ],
    result: { message: '2 of 3 targets applied', repairInstructions: ['Repair the failed target.'] },
  } }));
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
  await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
  await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Installation needs attention' })).toBeVisible();
  const readiness = page.getByRole('status', { name: 'Skills readiness' });
  await expect(readiness).toContainText('Files: 2 of 3 targets installed or already up to date; others need attention.');
  await expect(readiness).toContainText('2 of 3 targets applied');
  await expect(readiness).not.toContainText('Not verified');
  await expect(page.getByRole('region', { name: 'Installation progress' })).toContainText('Write refused');
});

for (const setupRequired of [false, true]) {
  test(`completed files show ${setupRequired ? 'remaining setup' : 'verified native configuration; agent discovery unverified'} on desktop and mobile`, async ({ page }, testInfo) => {
    const state = await serveFixtures(page);
    const operation = await fixture<Operation>('operation');
    const summary = setupRequired
      ? 'Skill files installed. Some agents still need setup; check each result below.'
      : 'Skill files and owned native configuration verified. Agent discovery and activation are not verified by this Skills flow.';
    const message = setupRequired
      ? 'Files installed. Agent setup still needs verification; run agentplugins doctor for the next step.'
      : 'The Skill files and owned native projection were verified. Agent-session discovery was not checked.';
    await page.route('**/api/v1/operations/*', (route) => route.fulfill({ json: {
      ...operation, total: 1, completed: 1, items: [{ ...operation.items[0], message }],
      diagnostics: setupRequired ? [{ code: 'NATIVE_SETUP_REQUIRED', severity: 'warning', message: 'Run agentplugins doctor to verify the installed Skills and see any remaining setup steps.' }] : [],
      result: { message: summary, repairInstructions: [] },
    } }));
    await page.goto('/#capability=test-capability');
    await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
    await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
    await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
    await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
    await expect(page.getByRole('heading', { name: setupRequired ? 'Installation needs attention' : 'Installation applied' })).toBeVisible();
    const progress = page.getByRole('region', { name: 'Installation progress' });
    await expect(page.getByRole('status', { name: 'Skills readiness' })).toContainText(summary);
    await expect(progress.getByText(summary, { exact: true })).toHaveCount(1);
    await expect(progress).toContainText(message);
    if (setupRequired) {
      await expect(progress).toContainText('1 / 1 · setup required');
      await expect(progress).toContainText('Agent setup required');
      await expect(progress).not.toContainText('NATIVE_SETUP_REQUIRED');
      await expect(page.getByRole('status', { name: 'Skills readiness' })).not.toHaveClass(/neutral/);
      await expect(page.getByRole('heading', { name: 'Installation applied' })).toHaveCount(0);
    } else {
      await expect(page.getByRole('status', { name: 'Skills readiness' })).toHaveClass(/neutral/);
      await expect(progress).not.toContainText('setup required');
      await expect(progress).not.toContainText('agentplugins doctor');
      await expect(progress).toContainText('Agent-session discovery was not checked.');
      await expect(progress).not.toContainText('detected by the agent');
    }
    const name = setupRequired ? 'setup' : 'ready';
    await page.screenshot({ path: testInfo.outputPath(`picker-${name}-desktop.png`), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.getByRole('tab', { name: /^Selection/ }).click();
    await expect(progress).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath(`picker-${name}-mobile.png`), fullPage: true });
  });
}

test('mobile confirmation reveals the accepted operation without taking focus on polling', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const state = await serveFixtures(page);
  const operation = await fixture<Operation>('operation');
  let completed = false, unavailable = false, observations = 0;
  await page.route('**/api/v1/operations/*', (route) => {
    observations++;
    if (unavailable) return route.fulfill({ status: 503, json: {
      code: 'UNAVAILABLE', message: 'The operation could not be checked. Reconnect before retrying.', retryable: true,
    } });
    return route.fulfill({ json: {
      ...operation, state: completed ? 'succeeded' : 'running', phase: 'Installing skills',
      total: 1, completed: completed ? 1 : 0, items: completed ? [operation.items[0]] : [],
      result: completed ? operation.result : undefined,
    } });
  });
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('tab', { name: /^Selection/ }).click();
  await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
  await page.getByRole('tab', { name: 'skills', exact: true }).click();
  await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
  await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByRole('tab', { name: /^Selection/ })).toHaveAttribute('aria-selected', 'true');
  const heading = page.getByRole('heading', { name: 'Installing skills', exact: true });
  await expect(heading).toBeFocused();
  await expect.poll(async () => {
    const bounds = await heading.boundingBox();
    const footer = await page.locator('.bottom-bar').boundingBox();
    return !!bounds && !!footer && bounds.y >= 0 && bounds.y + bounds.height <= footer.y;
  }).toBe(true);
  const search = page.getByRole('textbox', { name: 'Search skills', exact: true });
  await search.focus();
  const beforePoll = observations;
  await expect.poll(() => observations).toBeGreaterThan(beforePoll);
  await expect(search).toBeFocused();
  unavailable = true;
  await expect(page.getByRole('alert')).toContainText('The operation could not be checked');
  await expect(search).not.toBeFocused();
  await search.focus();
  const beforeFailures = observations;
  await expect.poll(() => observations).toBeGreaterThan(beforeFailures + 1);
  await expect(search).toBeFocused();
  unavailable = false;
  completed = true;
  await expect(page.getByRole('heading', { name: 'Installation applied' })).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(search).toBeFocused();
  expect(state.counts().applyCount).toBe(1);
});

for (const failure of ['stale confirmation', 'unavailable operation', 'unknown outcome']) {
  test(`mobile ${failure} exposes its alert above the footer`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const state = await serveFixtures(page, { stale: failure === 'stale confirmation', loseApply: failure === 'unknown outcome' });
    if (failure === 'unknown outcome') {
      const session = await fixture<Session>('session');
      await page.route('**/api/v1/session', (route) => route.fulfill({ json: { ...session, operationId: undefined, state: 'ready' } }));
    }
    if (failure === 'unavailable operation') await page.route('**/api/v1/operations/*', (route) => route.fulfill({
      status: 503, json: { code: 'UNAVAILABLE', message: 'The operation could not be checked. Reconnect before retrying.', retryable: true },
    }));
    await page.goto('/#capability=test-capability');
    await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
    await page.getByRole('tab', { name: /^Selection/ }).click();
    await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
    await page.getByRole('tab', { name: 'skills', exact: true }).click();
    await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
    await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(page.getByRole('tab', { name: /^Selection/ })).toHaveAttribute('aria-selected', 'true');
    const alert = page.getByRole('alert');
    await expect(alert).toContainText(failure === 'stale confirmation' ? 'Plan is stale' : failure === 'unknown outcome' ? 'Installation status is unknown' : 'The operation could not be checked');
    await expect.poll(async () => {
      const bounds = await alert.boundingBox();
      const footer = await page.locator('.bottom-bar').boundingBox();
      return !!bounds && !!footer && bounds.y >= 0 && bounds.y + bounds.height <= footer.y;
    }).toBe(true);
    await expect(page.getByRole('region', { name: 'Selection basket' })).toContainText('docs/review');
    await expect(page.getByRole('heading', { name: 'Installation applied' })).toHaveCount(0);
    expect(state.counts().applyCount).toBe(1);
  });
}

test('partial result with no applied target keeps the failure summary and recovery visible', async ({ page }) => {
  const state = await serveFixtures(page);
  const operation = await fixture<Operation>('operation');
  const summary = 'No selected agent was updated. Follow the recovery instructions before trying again.';
  await page.route('**/api/v1/operations/*', (route) => route.fulfill({ json: {
    ...operation, state: 'partial', total: 1, completed: 1,
    items: [{ ...operation.items[0], status: 'failed', message: 'The target configuration could not be verified.' }],
    result: { message: summary, repairInstructions: ['Inspect agentplugins deps status before retrying.'] },
  } }));
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
  await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
  await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Installation needs attention' })).toBeVisible();
  const progress = page.getByRole('region', { name: 'Installation progress' });
  await expect(progress.getByText(summary, { exact: true })).toBeVisible();
  await expect(progress).toContainText('Inspect agentplugins deps status before retrying.');
  await expect(page.getByRole('status', { name: 'Skills readiness' })).toHaveCount(0);
});

for (const [action, label] of [['repair', 'Repair'], ['refresh_projection', 'Refresh setup']] as const) {
  test(`${action} is visible in review and applies only after confirmation`, async ({ page }) => {
    const state = await serveFixtures(page);
    const plan = await fixture<Plan>('plan');
    await page.route('**/api/v1/plans', (route) => route.fulfill({ json: {
      ...plan, expiresAt: new Date(Date.now() + 600_000).toISOString(),
      actions: [{ ...plan.actions[0], action }],
    } }));
    await page.goto('/#capability=test-capability');
    await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
    await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
    await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
    const review = page.getByRole('dialog');
    await expect(review.locator('.action-badge')).toHaveText(label);
    await expect(review).toContainText(plan.actions[0].destinationPath);
    expect(state.counts().applyCount).toBe(0);
    await expect(review.getByRole('button', { name: 'Confirm installation', exact: true })).toBeEnabled();
    await review.getByRole('button', { name: 'Confirm installation', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Installation applied' })).toBeVisible();
    expect(state.counts().applyCount).toBe(1);
  });
}

test('lost apply response recovers the fast terminal operation without another apply', async ({ page }) => {
  const fixture = await serveFixtures(page, { loseApply: true });
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('checkbox', { name: `Target ${fixture.targets.targets[0].label}` }).check();
  await page.locator('.bottom-bar').getByRole('button', { name: 'Review installation' }).click();
  await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Installation applied' })).toBeVisible();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).uncheck();
  await expect(page.getByRole('alert')).toHaveCount(0);
  expect(fixture.counts().applyCount).toBe(1);
});

test('accepted running operation returns modal focus to search when its review trigger is disabled', async ({ page }) => {
  const state = await serveFixtures(page);
  const operation = await fixture<Operation>('operation');
  await page.route('**/api/v1/operations/*', (route) => route.fulfill({ json: { ...operation, state: 'running', completed: 0, phase: 'Installing skills', items: [], result: undefined } }));
  await page.goto('/#capability=test-capability');
  await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
  await page.getByRole('checkbox', { name: `Target ${state.targets.targets[0].label}` }).check();
  const trigger = page.locator('.bottom-bar').getByRole('button', { name: 'Review installation', exact: true });
  await trigger.click();
  await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Installing skills', exact: true })).toBeVisible();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(trigger).toBeDisabled();
  await expect(page.getByRole('textbox', { name: 'Search skills', exact: true })).toBeFocused();
  expect(state.counts().applyCount).toBe(1);
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
  await expect(page.getByRole('heading', { name: 'Installation applied' })).toHaveCount(0);
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
  await expect(page.getByRole('region', { name: 'Skill preview' })).toHaveCount(0);
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
