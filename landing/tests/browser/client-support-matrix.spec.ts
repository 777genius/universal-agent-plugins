import { expect, test } from '@playwright/test';

test('home lists the complete scope matrix immediately before FAQ', async ({ page }) => {
  await page.goto('./');

  const section = page.locator('#agent-support');
  const table = section.getByRole('table');
  await expect(table.locator('tbody tr')).toHaveCount(15);
  const logos = table.locator('tbody th img');
  await expect(logos).toHaveCount(15);
  await expect(logos.first()).toHaveAttribute('src', /client-icons\/claude.svg$/);
  await expect(logos.nth(8)).toHaveAttribute('src', /client-icons\/windsurf.svg$/);
  await expect.poll(() => logos.evaluateAll((images) =>
    images.every((image) => (image as HTMLImageElement).naturalWidth > 0),
  ), { timeout: 12_000 }).toBe(true);
  await expect(page.locator('#agent-support + #faq')).toHaveCount(1);
  await expect(table.getByRole('columnheader')).toHaveText([
    'Agent',
    'User scope',
    'Project scope',
    'Setup',
    'What you do after installing',
  ]);
  await expect(table.getByRole('row', { name: /^ChatGPT / })).toContainText('Not supported');
  await expect(table.getByRole('row', { name: /^Grok Build / })).toContainText('Skills only');
  await expect(table.getByRole('row', { name: /^Pi / })).toContainText(
    'Extensions, Skills, MCP',
  );
  await expect(section).toContainText('User installs into your agent profile');
  await expect(section).toContainText('Prepared = files are ready but need activation');
  await expect(section).not.toContainText('end-to-end');
  await expect(section).not.toContainText('E2E');
});

test('scope matrix scrolls within its own region on narrow screens', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 800 });
  await page.goto('./');

  const scroller = page.getByRole('region', { name: 'Agent installation support table' });
  const sizes = await scroller.evaluate((element) => ({
    content: element.scrollWidth,
    container: element.clientWidth,
    page: document.documentElement.scrollWidth,
    viewport: window.innerWidth,
  }));
  expect(sizes.content).toBeGreaterThan(sizes.container);
  expect(sizes.page).toBeLessThanOrEqual(sizes.viewport);
});
