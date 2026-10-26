import { hydrated } from './i18n-state.helpers';
import { expect, test } from '@playwright/test';
import { clientLandingPages } from '../../data/clients';

for (const scenario of [
  { name: 'registry', path: './agents/codex/', selector: '.agent-page__install .command-snippet', copy: 'Copy command', copied: 'Command copied' },
  { name: 'download', path: './download/', selector: '.download-section__steps .command-snippet', copy: 'Copy', copied: 'Copied' },
]) {
  test(`${scenario.name} copy refuses false success and cleans up after browser denial`, async ({ page }) => {
    await page.addInitScript(() => {
      let writes = 0;
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: { writeText: async () => {
          if (++writes > 1) throw new DOMException('Clipboard denied', 'NotAllowedError');
        } },
      });
      Object.defineProperty(document, 'execCommand', {
        configurable: true,
        value: () => {
          document.documentElement.dataset.copyFallback = 'attempted';
          if (document.documentElement.dataset.copyFallbackThrows === 'true') {
            throw new DOMException('Copy denied', 'NotAllowedError');
          }
          return false;
        },
      });
    });
    await page.goto(scenario.path);
    await hydrated(page);
    const snippet = page.locator(scenario.selector).first();
    const button = snippet.getByRole('button');
    const command = await snippet.locator('code').innerText();
    const textareas = await page.locator('textarea').count();
    await button.click();
    await expect(button).toHaveAccessibleName(scenario.copied);
    for (const throws of [false, true]) {
      await page.evaluate((throws) => {
        delete document.documentElement.dataset.copyFallback;
        document.documentElement.dataset.copyFallbackThrows = String(throws);
      }, throws);
      await button.click();
      await expect(page.locator('html')).toHaveAttribute('data-copy-fallback', 'attempted');
      // Snapshot after promise/Vue updates, without waiting for the copied reset timer.
      // The old implementation reports success here until its 1.6/1.8-second timeout.
      const state = await snippet.evaluate(async (element) => {
        await Promise.resolve();
        await Promise.resolve();
        return {
          label: element.querySelector('button')?.getAttribute('aria-label'),
          announcement: element.querySelector('[role="status"]')?.textContent ?? '',
          textareas: document.querySelectorAll('textarea').length,
        };
      });
      expect(state.label).toBe(scenario.copy);
      expect(state.announcement.trim()).toBe('');
      expect(state.textareas).toBe(textareas);
      await expect(snippet.locator('code')).toHaveText(command);
    }
  });
}

test('download has one title, a readable version, and current client support', async ({ page }) => {
  await page.route('https://api.github.com/repos/**/releases/latest', (route) =>
    route.fulfill({
      json: { tag_name: 'agentplugins-v1.2.3', published_at: '2026-09-01T00:00:00Z', assets: [] },
    }),
  );
  await page.goto('./download/');
  await expect(
    page.getByRole('heading', { name: 'Install your first plugin', exact: true }),
  ).toHaveCount(1);
  await expect(page.locator('h1')).toHaveText('Install your first plugin');
  await expect(page.locator('.download-section__release-info a')).toHaveText('v1.2.3');
  await expect(page.getByRole('main')).not.toContainText(
    /vagentplugins|runtime lane|packaging-only/i,
  );
  const support = page.locator('.download-section__support-list');
  await expect(support.locator('.download-section__support-item')).toHaveCount(
    clientLandingPages.length,
  );
  for (const client of clientLandingPages) {
    await expect(support.getByRole('link', { name: client.name, exact: true })).toHaveAttribute(
      'href',
      new RegExp(`/agents/${client.slug}/$`),
    );
  }
  await expect(support).toContainText('Setup in app');
  await expect(support).toContainText('Prepared by CLI');
});

for (const client of clientLandingPages) {
  test(`${client.name} copies a runnable example and uses canonical catalog links`, async ({
    page,
  }) => {
    await page.addInitScript(() => {
      let copied = '';
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: {
          writeText: async (value: string) => {
            copied = value;
          },
          readText: async () => copied,
        },
      });
    });
    await page.goto(`./agents/${client.slug}/`);
    await hydrated(page);
    const source = client.id === 'grok' || client.id === 'kimi' ? './my-plugin' : 'context7';
    const command = `npx -y universal-agent-plugins add ${source} --target ${client.id}`;
    const install = page.locator('.agent-page__install');
    await expect(install.locator('code').first()).toHaveText(command);
    await install.getByRole('button', { name: 'Copy command', exact: true }).click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(command);
    await expect(page.locator('.agent-page__directory-link')).toHaveAttribute(
      'href',
      /\/plugins\/$/,
    );
    for (const link of await page.locator('.agent-page__plugin-grid a').all()) {
      await expect(link).toHaveAttribute('href', /\/plugins\/[^/]+\/$/);
    }
  });
}
