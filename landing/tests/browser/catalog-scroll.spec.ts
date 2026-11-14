import { expect, test } from '@playwright/test';
import { hydrated, navigate, offline } from './i18n-state.helpers';

for (const width of [1440, 390]) {
  test(`homepage catalog search clears stale anchors and preserves its viewport at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) =>
      route.fulfill({
        json: {
          items: [],
          page: 0,
          hasMore: false,
          view: 'popular',
          cache: { status: 'fresh', fetchedAt: '2026-10-27T00:00:00Z' },
        },
      }),
    );
    await page.goto('./?tab=skills&q=turnstile-sp#plugin-dependencies');
    await hydrated(page);
    await expect(page).toHaveURL(/\?tab=skills&q=turnstile-sp#plugins$/);
    const search = page.getByRole('searchbox', { name: 'Search Skills', exact: true });
    await expect(search).toHaveValue('turnstile-sp');
    await search.evaluate((node) =>
      window.scrollTo({
        top: node.getBoundingClientRect().top + scrollY - 180,
        behavior: 'instant',
      }),
    );
    const searchTop = (await search.boundingBox())!.y;
    for (const query of ['turnstile', 'turnstile-sp', 'turnstile-new']) {
      await search.fill(query);
      await expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe(query);
      await expect.poll(() => new URL(page.url()).hash).toBe('');
      await expect(page.locator('.skills-catalog')).toHaveAttribute('aria-busy', 'false');
      expect(Math.abs((await search.boundingBox())!.y - searchTop)).toBeLessThan(25);
      await expect(search).toBeInViewport();
    }
    const modes = page.locator('.catalog-modes');
    // The search can be visible while the mode control is behind the sticky header.
    await modes.evaluate((node) =>
      window.scrollTo({
        top: node.getBoundingClientRect().top + scrollY - 120,
        behavior: 'instant',
      }),
    );
    const modesTop = (await modes.boundingBox())!.y;
    await modes.getByRole('link', { name: 'Plugins', exact: true }).click();
    const plugins = page.getByRole('searchbox', { name: 'Search plugins', exact: true });
    await expect(plugins).toHaveValue('turnstile-new');
    await expect(plugins).toBeInViewport();
    expect(Math.abs((await modes.boundingBox())!.y - modesTop)).toBeLessThan(25);
    expect(new URL(page.url()).hash).toBe('');
    await plugins.fill('context7');
    await expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe('context7');
    await expect(plugins).toBeInViewport();

    // Explicit later anchor navigation still targets its section. Back restores
    // the catalog position rather than reusing the destination's anchor.
    const beforeAnchor = await page.evaluate(() => scrollY);
    await navigate(page, '/', 'en', '?q=context7#plugin-dependencies');
    await expect
      .poll(() =>
        page
          .locator('#plugin-dependencies')
          .evaluate((node) => Math.abs(node.getBoundingClientRect().top)),
      )
      .toBeLessThan(5);
    await page.goBack();
    await expect.poll(() => page.evaluate(() => scrollY)).toBeCloseTo(beforeAnchor, 0);
    expect(errors).toEqual([]);
  });
}
