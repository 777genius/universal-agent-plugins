import { expect, test } from '@playwright/test';

test('Hero alternatives open the installation-only page', async ({ page }) => {
  await page.goto('./');
  const link = page.locator('.hero-quick-start__more-install');
  await expect(link).toHaveAttribute('href', /\/download\/$/);
  await link.click();
  await expect(page).toHaveURL(/\/download\/$/);
  await expect(page.locator('h1')).toHaveText('Install Universal Agent Plugins');
  await expect(page.locator('.download-section__support-list, .download-section__steps')).toHaveCount(0);
  await expect(page.locator('.download-section__install-tab')).toHaveCount(4);
});

for (const width of [1440, 390]) {
  test('installation methods remain readable at ' + width + 'px', async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.addInitScript(() => {
      let copied = '';
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: {
          writeText: async (value: string) => { copied = value; },
          readText: async () => copied,
        },
      });
    });
    await page.goto('./download/');
    const tabs = page.locator('.download-section__install-tab');
    const command = page.locator('.download-section__command code');
    await expect(command).toHaveCount(1);
    for (const [label, snippet] of [
      ['npx', 'npx -y universal-agent-plugins add context7'],
      ['Homebrew', 'brew install 777genius/agentplugins/agentplugins'],
      ['Verified install script', 'install.sh | sh'],
      ['PowerShell', 'install.ps1 | iex'],
    ]) {
      const tab = tabs.filter({ hasText: label });
      await tab.click();
      await expect(tab).toHaveAttribute('aria-selected', 'true');
      await expect(command).toContainText(snippet);
      const exactCommand = (await command.textContent())!.trim();
      await page.locator('.download-section__command').getByRole('button').click();
      await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(exactCommand);
      await expect(page.locator('.download-section__method-description')).not.toBeEmpty();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
    await testInfo.attach('download-methods-' + width, {
      body: await page.locator('.download-section').screenshot({ animations: 'disabled' }),
      contentType: 'image/png',
    });
  });
}

test('installation methods expose a keyboard-operable tabpanel', async ({ page }) => {
  await page.goto('./download/');
  const tablist = page.getByRole('tablist');
  const tabs = tablist.getByRole('tab');
  const panel = page.getByRole('tabpanel');

  await expect(tabs).toHaveCount(4);
  await tabs.first().click();
  await expect(tabs.first()).toHaveAttribute('aria-selected', 'true');
  await expect(tabs.first()).toHaveAttribute('tabindex', '0');
  await expect(tabs.nth(1)).toHaveAttribute('tabindex', '-1');
  await expect(tabs.first()).toHaveAttribute('aria-controls', 'download-method-panel');
  await expect(panel).toHaveAttribute('aria-labelledby', 'download-method-tab-npm');

  await tabs.first().press('ArrowRight');
  await expect(tabs.nth(1)).toHaveAttribute('aria-selected', 'true');
  await expect(tabs.nth(1)).toBeFocused();
  await expect(panel).toHaveAttribute('aria-labelledby', 'download-method-tab-brew');

  await tabs.nth(1).press('End');
  await expect(tabs.last()).toBeFocused();
  await expect(tabs.last()).toHaveAttribute('aria-selected', 'true');

  await tabs.last().press('Home');
  await expect(tabs.first()).toBeFocused();
  await expect(tabs.first()).toHaveAttribute('aria-selected', 'true');

  await tabs.first().press('ArrowLeft');
  await expect(tabs.last()).toBeFocused();
  await expect(tabs.last()).toHaveAttribute('aria-selected', 'true');
});
