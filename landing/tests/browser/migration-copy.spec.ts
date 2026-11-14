import { hydrated, pinnedContext7Example, wording } from './i18n-state.helpers';
import { expect, test } from '@playwright/test';
import { clientLandingPages } from '../../data/clients';

for (const scenario of [
  { name: 'registry', path: './agents/codex/', selector: '.agent-page__install .command-snippet', copy: 'Copy command', copied: 'Command copied' },
  { name: 'download', path: './download/', selector: '.download-section__command .command-snippet', copy: 'Copy', copied: 'Copied' },
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
      await expect(snippet.locator('code')).toHaveText(command, { useInnerText: true });
    }
  });
}

test('download shows only installation methods, command and release', async ({ page }) => {
  await page.route('https://api.github.com/repos/**/releases/latest', (route) =>
    route.fulfill({
      json: { tag_name: 'agentplugins-v1.2.3', published_at: '2026-09-01T00:00:00Z', assets: [] },
    }),
  );
  await page.goto('./download/');
  await expect(page.locator('h1')).toHaveText('Install Universal Agent Plugins');
  await expect(page.locator('.download-section__release-info a')).toHaveText('v1.2.3');
  await expect(page.locator('.download-section__install-tab')).toHaveCount(4);
  await expect(page.locator('.download-section__command .command-snippet')).toHaveCount(1);
  await expect(page.locator('.download-section__support-list, .download-section__steps')).toHaveCount(0);
  await expect(page.getByRole('main')).not.toContainText(/vagentplugins|runtime lane|packaging-only/i);
});

for (const client of clientLandingPages) {
  test(`${client.name} copies an installation example and uses canonical catalog links`, async ({
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
    const pinned = ['pi', 'hermes', 'antigravity'].includes(client.id)
      ? await pinnedContext7Example(page, client.id)
      : undefined;
    const command = pinned?.command ?? `npx -y universal-agent-plugins add context7 --target ${client.id}`;
    const install = page.locator('.agent-page__install');
    const snippet = install.locator('.command-snippet--add');
    await expect(snippet).toHaveCount(1);
    await expect(snippet.locator('code')).toHaveText(command);
    if (pinned) {
      await expect(install.locator('p code')).toHaveText(pinned.selector);
      await expect(install).toContainText(wording('en')('registryUi.install.pinnedCatalogSource'));
      await expect(install).not.toContainText('First create a compatible Agent Plugins package at ./my-plugin');
    }
    await snippet.getByRole('button', { name: 'Copy command', exact: true }).click();
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
