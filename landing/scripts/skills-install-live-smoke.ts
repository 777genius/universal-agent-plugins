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
  site: 'https://universal-agent-plugins.com/plugins/?tab=skills',
  api: 'https://api.universal-agent-plugins.com/v1/skills',
  published_sha: process.env.UAP_LANDING_DEPLOYMENT_SHA || null,
  browser_response: 0,
  api_response: 0,
  checked: 0,
  installable: 0,
  legacy_recovered: 0,
  unsupported: 0,
  failure: null as string | null,
};
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
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
  const initial = Math.min(48, data.items.length);
  await expect(cards).toHaveCount(initial, { timeout: 30000 });
  // Verify every visible catalog entry from the real first page (up to 200).
  for (let shown = initial; shown < data.items.length; shown = Math.min(shown + 48, data.items.length)) {
    await page.getByRole('button', { name: /Show more Skills/ }).last().click();
    await expect(cards).toHaveCount(Math.min(shown + 48, data.items.length));
  }

  let firstInstallable = -1;
  for (const [index, item] of data.items.entries()) {
    const command = skillInstallCommand(item);
    const button = cards.nth(index).getByRole('button', {
      name: `Install ${item.title}`, exact: true,
    });
    report.checked++;
    report.unsupported += Number(!command);
    if (command) {
      report.installable++;
      if (firstInstallable === -1) firstInstallable = index;
      report.legacy_recovered += Number(item.availability === 'unsupported-source');
      await expect(button, `Missing installation action: ${item.id}`).toHaveCount(1);
    } else {
      await expect(button, `Unsafe installation action: ${item.id}`).toHaveCount(0);
    }
  }
  assert.ok(firstInstallable >= 0, 'No install actions available in public catalog');
  const selected = data.items[firstInstallable]!;
  await cards.nth(firstInstallable).getByRole('button', {
    name: `Install ${selected.title}`, exact: true,
  }).click();
  await expect(cards.nth(firstInstallable).locator('.command-snippet code')).toHaveText(
    skillInstallCommand(selected)!,
  );
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  report.failure = error instanceof Error ? error.message : String(error);
  throw error;
} finally {
  try { await page.screenshot({ path: join(output, 'skills-catalog.png'), fullPage: true }); } catch { /* page may never have loaded */ }
  writeFileSync(join(output, 'production-report.json'), JSON.stringify(report, null, 2) + '\n');
  await browser.close();
}
