import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { chromium, expect } from '@playwright/test';
import type { SkillsCatalogPage } from '../types/skillsCatalog.ts';
import { skillInstallCommand } from '../utils/skillsCatalog.ts';

// Production E2E: genuine browser, deployed site, real public Skills API.
// No route handlers, mock data, TLS exceptions or disabled CORS.
assert.equal(process.env.UAP_E2E_LIVE_SKILLS, '1', 'Explicit production Skills E2E opt-in required');
const output = process.env.UAP_SKILLS_INSTALL_REPORT_DIR || '/tmp/uap-skills-install-live';
mkdirSync(output, { recursive: true });
const report = {
  checked_at: new Date().toISOString(),
  site: 'https://universal-agent-plugins.com/plugins/?tab=skills&skillScan=all',
  api: 'https://api.universal-agent-plugins.com/v1/skills',
  published_sha: process.env.UAP_LANDING_DEPLOYMENT_SHA || null,
  browser_response: 0,
  api_response: 0,
  checked: 0,
  installable: 0,
  legacy_recovered: 0,
  unsupported: 0,
  default_all_shown: 0,
  scanned_filter_shown: 0,
  page_errors: [] as string[],
  failure: null as string | null,
};
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
page.on('pageerror', error => report.page_errors.push(error.message));
try {
  const pendingAPI = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.origin === 'https://api.universal-agent-plugins.com' &&
      url.pathname === '/v1/skills' &&
      url.searchParams.get('view') === 'popular' &&
      url.searchParams.get('page') === '0';
  }, { timeout: 35000 });
  const root = await page.goto(report.site, { waitUntil: 'domcontentloaded', timeout: 35000 });
  report.browser_response = root?.status() || 0;
  assert.equal(report.browser_response, 200, 'Published Skills page must be HTTP 200');
  const apiResponse = await pendingAPI;
  report.api_response = apiResponse.status();
  assert.equal(report.api_response, 200, 'Published Skills API must be HTTP 200');
  const data = (await apiResponse.json()) as SkillsCatalogPage;
  assert.ok(Array.isArray(data.items) && data.items.length > 0, 'Published catalog returned no items');
  assert.equal(data.page, 0);
  assert.equal(data.view, 'popular');
  const cards = page.locator('.skills-catalog .skill-card');
  // The catalog deliberately hides unusable sources. Compare the deployed UI
  // with the actual API entries eligible for an install command, not its raw
  // total. Syntax alone does not prove an installation (native CI does that).
  const eligible = data.items.filter(item => Boolean(skillInstallCommand(item)));
  const unavailable = data.items.filter(item => !skillInstallCommand(item));
  assert.ok(eligible.length > 0, 'No command-eligible Skills in public catalog');
  const initial = Math.min(48, eligible.length);
  await expect(cards).toHaveCount(initial, { timeout: 30000 });
  for (let shown = initial; shown < eligible.length; shown = Math.min(shown + 48, eligible.length)) {
    await page.getByRole('button', { name: /Show more Skills/ }).last().click();
    await expect(cards).toHaveCount(Math.min(shown + 48, eligible.length));
  }

  const renderedIds = await cards.evaluateAll(nodes =>
    nodes.map(node => node.getAttribute('data-skill-id')));
  expect(renderedIds).toEqual(eligible.map(item => item.id));
  for (const item of unavailable) {
    expect(renderedIds, `Unusable source must be hidden: ${item.id}`).not.toContain(item.id);
  }

  report.checked = data.items.length;
  report.installable = eligible.length;
  report.unsupported = unavailable.length;
  for (const [index, item] of eligible.entries()) {
    report.legacy_recovered += Number(item.availability === 'unsupported-source');
    await expect(cards.nth(index).getByRole('button', {
      name: `Install ${item.title}`, exact: true,
    }), `Missing installation action: ${item.id}`).toHaveCount(1);
  }
  const selected = eligible[0]!;
  await cards.first().getByRole('button', {
    name: `Install ${selected.title}`, exact: true,
  }).click();
  await expect(cards.first().locator('.command-snippet code')).toHaveText(
    skillInstallCommand(selected)!,
  );
  // Verify default all-Skills discovery, then opt into current inspected evidence.
  await page.screenshot({ path: join(output, 'skills-catalog-all.png'), fullPage: true });
  await page.goto('https://universal-agent-plugins.com/plugins/?tab=skills',
    { waitUntil: 'domcontentloaded', timeout: 35000 });
  const defaultCatalog = page.locator('.skills-catalog');
  await expect(defaultCatalog).toHaveAttribute('aria-busy', 'false', { timeout: 90000 });
  await expect(defaultCatalog.getByRole('button', { name: 'Filters', exact: true })).toBeVisible();
  report.default_all_shown = await defaultCatalog.locator('.skill-card').count();
  assert.ok(report.default_all_shown > 0, 'Default catalog must show installable Skills');
  await page.goto('https://universal-agent-plugins.com/plugins/?tab=skills&skillScan=scanned',
    { waitUntil: 'domcontentloaded', timeout: 35000 });
  await expect(defaultCatalog).toHaveAttribute('aria-busy', 'false', { timeout: 90000 });
  const defaultCards = defaultCatalog.locator('.skill-card');
  report.scanned_filter_shown = await defaultCards.count();
  assert.ok(report.scanned_filter_shown > 0, 'Scanned filter must show current inspected Skills');
  for (const status of ['stale', 'notChecked', 'pending', 'unavailable', 'unsupported']) {
    await expect(defaultCards.locator('.skillspector--' + status))
      .toHaveCount(0, { timeout: 10000 });
  }
  await expect(defaultCards.first().locator('.skillspector > summary'))
    .toBeVisible({ timeout: 15000 });
  assert.deepEqual(report.page_errors, [], 'No Chromium JavaScript errors on either catalog mode');
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  report.failure = error instanceof Error ? error.message : String(error);
  throw error;
} finally {
  try { await page.screenshot({ path: join(output, 'skills-catalog.png'), fullPage: true }); } catch { /* page may never have loaded */ }
  writeFileSync(join(output, 'production-report.json'), JSON.stringify(report, null, 2) + '\n');
  await browser.close();
}
