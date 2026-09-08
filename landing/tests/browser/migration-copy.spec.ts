import { expect, test } from '@playwright/test';
import { clientLandingPages } from '../../data/clients';

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
  test(`${client.name} copies a runnable example and uses canonical catalog links`, async ({ page }) => {
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
    const command = `npx universal-agent-plugins add context7 --target ${client.id}`;
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
