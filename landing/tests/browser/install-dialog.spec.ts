import { expect, test, type Page } from '@playwright/test';
import { navigate, offline, projections, registryFixture, targets, delayedMirror, setSignedMirrorTestTime } from './i18n-state.helpers';

async function assertDialogFits(page: Page, width: number) {
  const dialog = page.locator('.install-dialog');
  await expect(dialog).toBeVisible();
  const geometry = await dialog.evaluate(element => {
    const rect = element.getBoundingClientRect();
    const body = element.querySelector('.install-dialog__body')!;
    return {
      left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom,
      width: rect.width, scroll: getComputedStyle(body).overflowY,
      bodyOverflow: body.scrollWidth > body.clientWidth,
      viewportOverflow: document.documentElement.scrollWidth > innerWidth,
    };
  });
  expect(geometry.left).toBeGreaterThanOrEqual(0);
  expect(geometry.right).toBeLessThanOrEqual(width);
  expect(geometry.top).toBeGreaterThanOrEqual(0);
  expect(geometry.bottom).toBeLessThanOrEqual(844);
  expect(geometry.width).toBeGreaterThan(width < 600 ? width - 24 : 850);
  expect(geometry.scroll).toBe('auto');
  expect(geometry.bodyOverflow).toBe(false);
  expect(geometry.viewportOverflow).toBe(false);
  const footer = dialog.locator('.install-dialog__footer');
  const footerBefore = (await footer.boundingBox())!;
  const command = footer.locator('.command-snippet--add');
  expect((await command.boundingBox())!.y).toBeLessThan(footerBefore.y);
  await dialog.locator('.install-dialog__body').evaluate((body) => { body.scrollTop = body.scrollHeight; });
  const footerAfter = (await footer.boundingBox())!;
  expect(Math.abs(footerBefore.y - footerAfter.y)).toBeLessThan(1);
  expect(footerAfter.y + footerAfter.height).toBeLessThanOrEqual(844);
  await expect(command.getByRole('button')).toBeInViewport();
  await dialog.locator('.install-dialog__body').evaluate((body) => { body.scrollTop = 0; });
  await expect(dialog).toHaveAttribute('aria-labelledby', /.+/);
  await expect(dialog).toHaveAttribute('aria-describedby', /.+/);
  expect(await page.evaluate(() => document.elementFromPoint(5, 5)?.classList.contains('install-dialog__overlay'))).toBe(true);
  // Repeated Tab navigation stays inside the dialog.
  for (let i = 0; i < 12; i++) {
    await page.keyboard.press('Tab');
    expect(await dialog.evaluate(element => element.contains(document.activeElement))).toBe(true);
  }
}

for (const width of [1280, 390]) {
  for (const path of ['/', '/plugins/']) {
    test(`plugin Install opens an accessible responsive dialog on ${path} at ${width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 844 });
      const errors = await offline(page);
      const index = registryFixture();
      index.plugins = [index.plugins[0]!];
      for (const distribution of index.plugins[0]!.distributions)
        for (const release of distribution.releases)
          for (const target of release.targets)
            if (target.client === 'codex') target.scopes = ['user', 'project'];
      await projections(page, index);
      await navigate(page, path, 'en', '?q=i18n-state-00');
      const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
      const trigger = card.locator('.plugin-card__install-toggle');
      await expect(page.locator('.install-dialog')).toHaveCount(0);
      await trigger.click();
      await expect(trigger).toHaveAttribute('aria-haspopup', 'dialog');
      await expect(page.getByRole('dialog', { name: 'Install I18n State 00', exact: true })).toBeVisible();
      await assertDialogFits(page, width);
      const projectInfo = page.getByRole('dialog').locator('.install-scope__info');
      await projectInfo.focus();
      await expect(page.locator('.app-tooltip:visible')).toContainText(await projectInfo.getAttribute('aria-label') ?? '');
      await page.getByRole('dialog').locator('.install-dialog__close').focus();
      await targets(page, card, ['Codex']);
      const dialog = page.locator('.install-dialog');
      await expect(dialog.getByRole('checkbox', { name: /^Codex(?:\s|$)/ })).toBeChecked();
      await expect(dialog.locator('.app-multiselect__item--selected img')).toHaveCount(1);
      await expect(page.getByRole('checkbox', { name: /^ChatGPT Desktop/ })).toContainText('Local desktop connection');
      await expect(page.getByRole('checkbox', { name: /^ChatGPT Desktop/ })).not.toContainText('does not prove Desktop runtime');
      await testInfo.attach(`plugin-agent-menu-${width}.png`, { body: await page.screenshot({ path: testInfo.outputPath(`agent-menu-${width}.png`) }), contentType: 'image/png' });
      await dialog.getByRole('button', { name: 'This project', exact: true }).click();
      await targets(page, card, ['Codex']);
      await expect(dialog.locator('.command-snippet--add code')).toContainText('--target codex --scope project');
      const command = (await dialog.locator('.command-snippet--add code').textContent())!.replace(/\s+/g, ' ').trim();
      await dialog.locator('.command-snippet--add button').click();
      await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(command);
      await testInfo.attach(`plugin-dialog-${width}.png`, { body: await dialog.screenshot({ path: testInfo.outputPath(`install-dialog-${width}.png`) }), contentType: 'image/png' });
      await dialog.getByRole('button', { name: 'Close installation', exact: true }).click();
      await expect(dialog).toHaveCount(0);
      await expect(trigger).toBeFocused();
      await trigger.click();
      await expect(dialog.locator('.command-snippet--add code')).toHaveText(command!);
      await page.keyboard.press('Escape');
      await expect(trigger).toBeFocused();
      // A saved choice never causes a modal to reopen on a fresh navigation.
      await navigate(page, '/download/');
      await navigate(page, path, 'en', '?q=i18n-state-00');
      await expect(card).toBeVisible();
      await expect(dialog).toHaveCount(0);
      await trigger.click();
      await expect(dialog.locator('.command-snippet--add code')).toHaveText(command!);
      await page.mouse.click(2, 2);
      await expect(dialog).toHaveCount(0);
      await expect(trigger).toBeFocused();
      expect(errors).toEqual([]);
    });
  }

  test(`Skill Install preserves agents and scope in the same responsive dialog at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 844 });
    const errors = await offline(page);
    await page.route('https://api.universal-agent-plugins.com/v1/skills**', route => route.fulfill({ json: {
      items: [{ id: 'skills-sh:anthropics/skills:frontend-design', providerId: 'skills-sh',
        title: 'frontend-design', skillNameHint: 'frontend-design',
        source: { kind: 'github', repository: 'anthropics/skills', url: 'https://github.com/anthropics/skills' },
        metrics: { installs: 1234, window: 'all-time' }, availability: 'unverified' }],
      page: 0, total: 1, hasMore: false, view: 'popular', cache: { status: 'fresh', fetchedAt: new Date().toISOString() },
    } }));
    await page.goto('./download/');
    await navigate(page, '/plugins/', 'en', '?tab=skills&skillScan=all');
    const card = page.locator('.skill-card').first();
    const trigger = card.getByRole('button', { name: 'Install frontend-design', exact: true });
    await trigger.click();
    await assertDialogFits(page, width);
    const dialog = page.locator('.install-dialog');
    await expect(dialog).toContainText('anthropics/skills');
    await targets(page, card, ['Hermes Agent', 'Kiro', 'Pi', 'Codex']);
    await expect(dialog.locator('.app-multiselect__heading > span')).toHaveText('4 selected');
    for (const name of ['Codex', 'Hermes Agent', 'Kiro', 'Pi'])
      await expect(dialog.getByRole('checkbox', { name: new RegExp(`^${name}(?:\\s|$)`) })).toBeChecked();
    await testInfo.attach(`skill-agent-menu-${width}.png`, { body: await page.screenshot({ path: testInfo.outputPath(`agent-menu-${width}.png`) }), contentType: 'image/png' });
    await dialog.getByRole('button', { name: 'This project', exact: true }).click();
    await expect(dialog.locator('.command-snippet--add code')).toContainText('--scope project');
    for (const agent of ['hermes', 'kiro', 'pi', 'codex'])
      await expect(dialog.locator('.command-snippet--add code')).toContainText(agent);
    const command = await dialog.locator('.command-snippet--add code').textContent();
    await testInfo.attach(`skill-dialog-${width}.png`, { body: await dialog.screenshot({ path: testInfo.outputPath(`install-dialog-${width}.png`) }), contentType: 'image/png' });
    await page.keyboard.press('Escape');
    await expect(trigger).toBeFocused();
    await trigger.click();
    await expect(dialog.locator('.command-snippet--add code')).toHaveText(command!);
    await dialog.getByRole('button', { name: 'Close installation' }).click();
    await expect(trigger).toBeFocused();
    expect(errors).toEqual([]);
  });
}


test('a delayed Directory response waits while focus is inside the install dialog', async ({ page }) => {
  await setSignedMirrorTestTime(page);
  const errors = await offline(page);
  const discovery = await delayedMirror(page, 'discovery');
  const security = await delayedMirror(page, 'security');
  try {
    await page.goto('./download/');
    await navigate(page, '/plugins/', 'en', '?q=context7');
    await discovery.seen;
    const catalog = page.locator('.catalog');
    await expect(catalog).toHaveAttribute('data-discovery-state', 'loading');
    const trigger = page.locator('.plugin-card[data-install-source="context7"] .plugin-card__install-toggle');
    await trigger.click();
    const dialog = page.locator('.install-dialog');
    await dialog.getByRole('button', { name: 'Close installation' }).focus();
    discovery.release();
    await discovery.delivered;
    security.release();
    await security.delivered;
    // Allow the signed snapshot and search projection to finish verification;
    // their application must remain deferred while the modal owns focus.
    await page.waitForTimeout(2000);
    await expect(catalog).toHaveAttribute('data-discovery-state', 'loading');
    await expect(dialog.locator('.command-snippet--add code')).toBeVisible();
    await page.keyboard.press('Escape');
    await page.locator('header a').first().focus();
    await expect(catalog).toHaveAttribute('data-discovery-state', /^(current|cached)$/);
    expect(errors).toEqual([]);
  } finally {
    discovery.release();
    security.release();
  }
});
