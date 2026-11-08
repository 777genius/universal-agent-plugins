import { expect, test } from '@playwright/test';

test('reviewed plugins prioritize everyday workflows and offer stable A–Z opt-in', async ({ page }) => {
  await page.goto('plugins/');
  const titles = page.locator('.plugin-grid--plugins .plugin-card__title-link');
  await expect(titles.first()).toBeVisible();
  const featured = (await titles.allTextContents()).slice(0, 9).map(name => name.trim().toLowerCase());
  expect(featured).toContain('github');
  expect(featured).toContain('playwright');
  expect(featured.filter(name => name.startsWith('cloudflare')).length).toBeLessThan(2);
  await expect(page.locator('.catalog-curation-note')).toContainText('not a safety rating');
  await expect(page.locator('.plugin-grid--plugins .plugin-card__description').first())
    .not.toContainText('Community package for the official');

  await page.getByRole('combobox', { name: 'Sort plugins' }).click();
  await page.getByRole('option', { name: 'Name A–Z' }).click();
  await expect.poll(() => new URL(page.url()).searchParams.get('sort')).toBe('name');
  await page.reload();
  await expect(titles.first()).toBeVisible();
  const alphabetical = (await titles.allTextContents()).map(name => name.trim().toLowerCase());
  expect(alphabetical).toEqual([...alphabetical].sort());
  await expect(page.locator('.catalog-curation-note')).toHaveCount(0);

  await page.goto('plugins/?q=github&sort=name');
  await expect(titles.first()).toContainText(/github/i);
  await expect(page.locator('.catalog-curation-note')).toHaveCount(0);
});

test('curated Skills show source-bound explanations without altering provider catalog', async ({ page }, testInfo) => {
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route =>
    route.fulfill({
      json: {
        items: [], page: 0, total: 0, hasMore: false, view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-11-08T18:00:00Z' },
      },
    }),
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('plugins/?tab=skills');
  const recommended = page.locator('.skills-curated__item');
  await expect(recommended).toHaveCount(6);
  await expect(page.locator('.skills-curated__heading')).toContainText('not a safety');
  await expect(recommended.first()).toContainText('agent-browser');
  await expect(recommended.first()).toContainText('vercel-labs/agent-browser');
  await expect(recommended.nth(1)).toHaveAttribute(
    'href',
    /skills\.sh%3Aanthropics(?:%2F|\/)skills%3Afrontend-design/i,
  );
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    .toBe(true);
  await testInfo.attach('skills-curation-mobile', {
    body: await page.screenshot({ fullPage: true, animations: 'disabled' }),
    contentType: 'image/png',
  });

  await page.goto('plugins/?tab=skills&q=browser');
  await expect(page.locator('.skills-curated')).toHaveCount(0);
  await page.goto('ru/plugins/?tab=skills');
  await expect(page.locator('.skills-curated__heading')).toContainText('Рекомендуем');
});
