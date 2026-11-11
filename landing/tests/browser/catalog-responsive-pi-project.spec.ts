import { expect, test } from '@playwright/test';
import { navigate, offline, projections, registryFixture } from './i18n-state.helpers';

function sentryFixture() {
  const index = registryFixture();
  const plugin = index.plugins[0]!;
  index.plugins = [plugin];
  plugin.trust_state = 'reviewed';
  plugin.name = 'sentry';
  plugin.display_name = 'Sentry';
  plugin.install_source = 'sentry';
  plugin.source.path = 'plugins/sentry';
  plugin.default_distribution = '777genius/sentry';
  plugin.declared_default_distribution = plugin.default_distribution;
  for (const distribution of plugin.distributions) {
    distribution.id = plugin.default_distribution;
    distribution.source.path = plugin.source.path;
    for (const release of distribution.releases) release.source.path = plugin.source.path;
  }
  return index;
}

for (const [width, columns] of [[1280, 3], [1024, 3], [900, 2], [768, 2], [390, 1]] as const) {
  test(`catalog filters use ${columns} columns at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 920 });
    const errors = await offline(page);
    await projections(page, sentryFixture());
    await navigate(page, '/plugins/', 'en');
    const controls = page.locator('.catalog-controls--plugins');
    await expect(controls).toBeVisible();
    const geometry = await controls.evaluate((element) => {
      const columns = getComputedStyle(element).gridTemplateColumns.trim().split(/\s+/).length;
      const pair = element.querySelector('.catalog-filter-pair');
      const search = element.querySelector('.search-field');
      return {
        columns,
        pairDisplay: pair ? getComputedStyle(pair).display : '',
        searchWidth: search?.getBoundingClientRect().width ?? 0,
        controlsWidth: element.getBoundingClientRect().width,
        overflow: document.documentElement.scrollWidth > window.innerWidth,
      };
    });
    expect(geometry.columns).toBe(columns);
    expect(geometry.overflow).toBe(false);
    if (width === 1280 || width === 1024) {
      expect(geometry.pairDisplay).toBe('contents');
      expect(geometry.searchWidth).toBeGreaterThan(geometry.controlsWidth - 3);
    } else if (width === 900) {
      expect(geometry.pairDisplay).toBe('grid');
    }
    if (width > 720) {
      await expect(controls.locator('.catalog-filter-pair .app-select__trigger')).toBeVisible();
      await expect(controls.locator('.catalog-filter-pair .app-combobox__anchor')).toBeVisible();
    }
    await controls.screenshot({ path: testInfo.outputPath(`catalog-filters-${width}.png`), animations: 'disabled' });
    expect(errors).toEqual([]);
  });
}

test('Arabic desktop catalog preserves three filter columns without clipping', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const errors = await offline(page);
  await projections(page, sentryFixture());
  await navigate(page, '/plugins/', 'ar');
  const controls = page.locator('.catalog-controls--plugins');
  await expect(controls).toBeVisible();
  const columns = await controls.evaluate((element) =>
    getComputedStyle(element).gridTemplateColumns.trim().split(/\s+/).length);
  expect(columns).toBe(3);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(errors).toEqual([]);
});

for (const width of [1440, 390]) {
  test(`Pi Project uses pinned direct source, not a fabricated signed target, at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 950 });
    const errors = await offline(page);
    await projections(page, sentryFixture());
    await navigate(page, '/', 'en', '?q=sentry');
    const card = page.locator('.plugin-card[data-install-source="sentry"]');
    await card.locator('.plugin-card__install-toggle').click();

    await card.locator('.app-multiselect__trigger').click();
    const pi = page.getByRole('checkbox', { name: /^Pi(?:\s|$)/ });
    await expect(pi).toBeDisabled(); // User scope does not gain an ungranted signed Pi target.
    await page.keyboard.press('Escape');

    await card.getByRole('button', { name: 'This project', exact: true }).click();
    await card.locator('.app-multiselect__trigger').click();
    await expect(pi).toBeEnabled();
    await expect(pi).toContainText('exact pinned Git source');
    await pi.click();
    await page.keyboard.press('Escape');
    const direct = 'github:777genius/universal-agent-plugins@2ddbb99dd190c1792b79904f9875e6322bccd243//plugins/sentry';
    const add = `npx -y universal-agent-plugins add ${direct} --target pi --scope project`;
    await expect(card.locator('.plugin-card__install .command-snippet--add code')).toHaveText(add);
    await expect(card).toContainText('not a signed Pi target');

    // Switching routes must not combine an unsigned Pi option with signed Codex.
    await card.locator('.app-multiselect__trigger').click();
    await page.getByRole('checkbox', { name: /^Codex(?:\s|$)/ }).click();
    await page.keyboard.press('Escape');
    await expect(card.locator('.plugin-card__install .command-snippet--add code')).toHaveText(
      'npx -y universal-agent-plugins add sentry --target codex --scope project');
    await card.locator('.app-multiselect__trigger').click();
    await pi.click();
    await page.keyboard.press('Escape');
    await expect(card.locator('.plugin-card__install .command-snippet--add code')).toHaveText(add);
    await card.locator('.plugin-card__install .command-snippet--add button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(add);

    await card.locator('.plugin-card__title-link').click();
    const detail = page.locator('.install-panel');
    await expect(detail.getByRole('button', { name: 'This project', exact: true })).toHaveAttribute('aria-pressed', 'true');
    await expect(detail.locator('.command-snippet--add code')).toHaveText(add);
    await expect(detail).toContainText('not a signed Pi target');
    await expect(detail.locator('.command-snippet--repair code')).toContainText('--target pi --scope project');
    await detail.getByRole('button', { name: 'This computer', exact: true }).click();
    await expect(detail.locator('.command-snippet--add code')).toHaveText(
      'npx -y universal-agent-plugins add sentry');
    await expect(detail.locator('.app-multiselect__trigger')).toContainText('All installed agents');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(errors).toEqual([]);
  });
}

test('Pi direct install uses older release provenance without borrowing newer source security review', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const errors = await offline(page);
  const index = sentryFixture();
  const plugin = index.plugins[0]!;
  const distribution = plugin.distributions[0]!;
  const older = distribution.releases[0]!;
  const newer = structuredClone(older);
  newer.release_sequence += 1;
  newer.source.revision = 'f'.repeat(40);
  newer.source.tree_sha256 = 'sha256:' + 'f'.repeat(64);
  newer.targets = [{
    client: 'chatgpt', authentication: 'required', delivery: 'manual_activation',
    scopes: ['user'], app_binding: { app_key: 'sentry-chatgpt', id: 'remote', mcp_server: 'sentry' },
  }];
  distribution.releases = [newer, older];
  distribution.source = newer.source;
  distribution.targets = newer.targets;
  distribution.release_sequence = newer.release_sequence;
  plugin.source = newer.source;
  plugin.security = {
    generated_at: '2026-11-10T12:00:00Z',
    scanner: { id: 'lintai', version: '1.0.0' },
    policy: { id: 'test', version: 1, digest: 'sha256:' + 'a'.repeat(64) },
    outcome: 'no_blocking_findings',
    counts: { blocking: 0, warnings: 0, total: 0 },
    scanned_files: 1,
    findings: [],
  };
  await projections(page, index);
  await navigate(page, '/', 'en', '?q=sentry');
  const card = page.locator('.plugin-card[data-install-source="sentry"]');
  await expect(card.locator('.plugin-card__security')).toHaveCount(1);
  await card.locator('.plugin-card__install-toggle').click();
  await card.getByRole('button', { name: 'This project', exact: true }).click();
  await card.locator('.app-multiselect__trigger').click();
  await page.getByRole('checkbox', { name: /^Pi(?:\\s|$)/ }).click();
  await page.keyboard.press('Escape');

  const sourceUrl = 'https://github.com/777genius/universal-agent-plugins/tree/2ddbb99dd190c1792b79904f9875e6322bccd243/plugins/sentry';
  await expect(card.locator('.plugin-card__install .command-snippet--add code'))
    .toContainText('@2ddbb99dd190c1792b79904f9875e6322bccd243//plugins/sentry --target pi --scope project');
  await expect(card.locator('.plugin-card__install a', { hasText: 'View source' }))
    .toHaveAttribute('href', sourceUrl);
  await expect(card.locator('.plugin-card__security')).toHaveCount(0);
  await card.locator('.plugin-card__title-link').click();
  const panel = page.locator('.install-panel');
  await expect(panel.getByRole('link', { name: 'View package source' }))
    .toHaveAttribute('href', sourceUrl);
  await expect(panel.locator('.command-snippet--add code'))
    .toContainText('@2ddbb99dd190c1792b79904f9875e6322bccd243//plugins/sentry --target pi --scope project');
  expect(errors).toEqual([]);
});
