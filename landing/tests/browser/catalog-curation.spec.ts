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


test('recommended Skills lead the original cards without changing provider data or modes', async ({
  page,
}, testInfo) => {
  const candidates = [
    { repository: 'example/skills', slug: 'general-guidance' },
    { repository: 'vercel-labs/agent-skills', slug: 'web-design-guidelines' },
    { repository: 'mattpocock/skills', slug: 'tdd' },
    { repository: 'anthropics/skills', slug: 'frontend-design' },
    { repository: 'example/skills', slug: 'another-tool' },
    { repository: 'vercel-labs/agent-browser', slug: 'agent-browser' },
    { repository: 'vercel-labs/skills', slug: 'find-skills' },
    { repository: 'vercel-labs/agent-skills', slug: 'vercel-optimize' },
  ];
  const original = candidates.map(({ repository, slug }, index) => ({
    id: `skills-sh:${repository}:${slug}`,
    providerId: 'skills-sh',
    providerUrl: `https://skills.sh/${repository}/${slug}`,
    title: slug,
    skillNameHint: slug,
    source: { kind: 'github', repository, url: `https://github.com/${repository}` },
    metrics: { installs: 800 - index, window: 'all-time' },
    availability: 'unverified',
  }));
  const requests: string[] = [];
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route => {
    requests.push(new URL(route.request().url()).search);
    return route.fulfill({
      json: {
        items: original, page: 0, total: original.length, hasMore: false, view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-11-08T18:00:00Z' },
      },
    });
  });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('plugins/?tab=skills');
  const cards = page.locator('.skills-catalog .skill-card');
  await expect(cards).toHaveCount(original.length);
  await expect(page.locator('.skills-curated')).toHaveCount(0);
  const ids = () => cards.evaluateAll((elements) =>
    elements.map((item) => item.getAttribute('data-skill-id')));
  expect(await ids()).toEqual([
    original[5]!.id, original[3]!.id, original[6]!.id, original[2]!.id,
    original[7]!.id, original[1]!.id, original[0]!.id, original[4]!.id,
  ]);
  await expect(cards.first().locator('.skill-card__benefit')).toContainText(/browser CLI/i);
  await expect(cards.first()).toContainText('795 installs');
  await cards.first().getByRole('button', { name: 'Install agent-browser' }).click();
  await cards.first().locator('.app-multiselect__trigger').click();
  const picker = page.locator('.app-multiselect__content');
  await expect(picker).toBeVisible();
  await expect(picker.locator('.app-multiselect__item small')).toHaveCount(0);
  await expect(picker.getByRole('checkbox', { name: /VS Code/i })).toHaveCount(0);
  await expect(cards.first().getByText('The CLI checks the source and Skill before installation.', { exact: true })).toHaveCount(1);
  await testInfo.attach('skills-agent-picker-without-duplicate-copy', {
    body: await page.screenshot({ animations: 'disabled' }),
    contentType: 'image/png',
  });
  await page.keyboard.press('Escape');
  await expect(cards.first().locator('.command-snippet code'))
    .toContainText('https://github.com/vercel-labs/agent-browser');
  await expect.poll(() => page.evaluate(() =>
    document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await testInfo.attach('curated-skills-original-cards-mobile', {
    body: await page.screenshot({ fullPage: true, animations: 'disabled' }),
    contentType: 'image/png',
  });

  await page.goto('plugins/?tab=skills&skillView=trending');
  await expect(cards).toHaveCount(original.length);
  expect(await ids()).toEqual(original.map((item) => item.id));
  await page.goto('plugins/?tab=skills&q=browser');
  await expect(cards).toHaveCount(original.length);
  expect(await ids()).toEqual(original.map((item) => item.id));
  expect(requests.some((query) => query.includes('q=browser'))).toBe(true);

  await page.goto('ru/plugins/?tab=skills');
  await expect(cards).toHaveCount(original.length);
  await expect(cards.first().locator('.skill-card__benefit')).toContainText('CLI');
  expect(await ids()).toEqual([
    original[5]!.id, original[3]!.id, original[6]!.id, original[2]!.id,
    original[7]!.id, original[1]!.id, original[0]!.id, original[4]!.id,
  ]);
});

test('plugin source and owner filters share a row without losing mobile usability', async ({
  page,
}, testInfo) => {
  await page.goto('plugins/');

  const toolbar = page.locator('.catalog-controls--plugins');
  const source = toolbar.getByRole('combobox', { name: 'Filter by source' });
  const owner = toolbar.getByRole('combobox', { name: 'Filter by owner' });
  const sourceControl = toolbar.locator('.catalog-filter-pair .app-select__trigger');
  const ownerControl = toolbar.locator('.catalog-filter-pair .app-combobox__anchor');
  const pair = toolbar.locator('.catalog-filter-pair');
  const authenticationControl = toolbar.getByRole('combobox', { name: 'Filter by authentication' });
  const toggle = toolbar.locator('.catalog-filter-toggle');

  for (const width of [1440, 820, 600, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    if (width <= 720 && (await toggle.getAttribute('aria-expanded')) !== 'true') {
      await toggle.click();
    }

    await expect(source).toBeVisible();
    await expect(owner).toBeVisible();
    const sourceRect = await sourceControl.boundingBox();
    const ownerRect = await ownerControl.boundingBox();
    const pairRect = await pair.boundingBox();
    expect(sourceRect).not.toBeNull();
    expect(ownerRect).not.toBeNull();
    expect(pairRect).not.toBeNull();

    if (width > 380) {
      expect(Math.abs(sourceRect!.y - ownerRect!.y)).toBeLessThan(2);
      expect(Math.abs(sourceRect!.width - ownerRect!.width)).toBeLessThan(2);
    } else {
      expect(ownerRect!.y).toBeGreaterThan(sourceRect!.y + sourceRect!.height - 1);
    }
    if (width > 980) {
      // Sources and Owners together take one normal desktop filter slot,
      // next to Authentication, not an entire extra full-width row.
      const authRect = await authenticationControl.boundingBox();
      expect(authRect).not.toBeNull();
      expect(Math.abs(pairRect!.y - authRect!.y)).toBeLessThan(2);
      expect(Math.abs(pairRect!.width - authRect!.width)).toBeLessThan(4);
    } else if (width > 720) {
      // A two-column tablet grid has an odd final filter slot; use both
      // columns rather than leaving half the toolbar empty.
      const toolbarRect = await toolbar.boundingBox();
      expect(toolbarRect).not.toBeNull();
      expect(Math.abs(pairRect!.width - toolbarRect!.width)).toBeLessThan(4);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      .toBe(true);

    if (width === 1440 || width === 390) {
      await testInfo.attach(`source-owner-filters-${width}`, {
        body: await toolbar.screenshot({ animations: 'disabled' }),
        contentType: 'image/png',
      });
    }
  }

  await page.setViewportSize({ width: 1440, height: 900 });
  await source.click();
  await page.getByRole('option', { name: 'Upstream packages' }).click();
  await expect.poll(() => new URL(page.url()).searchParams.get('source')).toBe('upstream');
  await expect(owner).toBeVisible();
});
