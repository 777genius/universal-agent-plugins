import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { chromium, expect } from '@playwright/test';
import type { SkillListing, SkillsCatalogPage } from '../types/skillsCatalog.ts';
import { skillInstallCommand } from '../utils/skillsCatalog.ts';
import { createSkillSpectorClient } from '../utils/skillSpectorCatalog.ts';
import { hasNoPublishedRemarks, publishedScanSubject } from '../utils/skillSpectorCurrent.ts';

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
  no_remarks_filter_shown: 0,
  no_remarks_and_scanned_shown: 0,
  no_remarks_records_verified: 0,
  immutable_publication: null as string | null,
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
  // All well-formed API listings remain visible. Install actions require safe
  // commands; rendering a listing never authorizes an installation.
  const eligible = data.items.filter(item => Boolean(skillInstallCommand(item)));
  const unavailable = data.items.filter(item => !skillInstallCommand(item));
  assert.ok(eligible.length > 0, 'No command-eligible Skills in public catalog');
  const initial = Math.min(48, data.items.length);
  await expect(cards).toHaveCount(initial, { timeout: 30000 });
  for (let shown = initial; shown < data.items.length; shown = Math.min(shown + 48, data.items.length)) {
    await page.getByRole('button', { name: /Show more Skills/ }).last().click();
    await expect(cards).toHaveCount(Math.min(shown + 48, data.items.length));
  }

  const renderedIds = await cards.evaluateAll(nodes =>
    nodes.map(node => node.getAttribute('data-skill-id')));
  expect(renderedIds).toEqual(data.items.map(item => item.id));

  report.checked = data.items.length;
  report.installable = eligible.length;
  report.unsupported = unavailable.length;
  for (const [index, item] of data.items.entries()) {
    const command = skillInstallCommand(item);
    const install = cards.nth(index).getByRole('button', { name: `Install ${item.title}`, exact: true });
    if (!command) {
      await expect(install, `Unsafe Install for ${item.id}`).toHaveCount(0);
      await expect(cards.nth(index).locator('.plugin-card__unavailable')).toBeVisible();
    } else {
      report.legacy_recovered += Number(item.availability === 'unsupported-source');
      await expect(install, `Missing installation action: ${item.id}`).toHaveCount(1);
    }
  }
  const firstInstall = data.items.findIndex(item => Boolean(skillInstallCommand(item)));
  const selected = data.items[firstInstall]!;
  await cards.nth(firstInstall).getByRole('button', {
    name: `Install ${selected.title}`, exact: true,
  }).click();
  await expect(cards.nth(firstInstall).locator('.command-snippet code')).toHaveText(
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
  assert.ok(report.default_all_shown > 0, 'Default catalog must show all Skills');
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
  // Unmocked production acceptance of both independent filter states against
  // *immutable, signed-by-content* v3 shard evidence, never badge text alone.
  const publicationRefUrl =
    'https://api.github.com/repos/777genius/universal-agent-plugins-registry/git/ref/heads/skillspector-reports';
  // Retry only transient GitHub availability failures; never accept a bad ref or stale fixture.
  async function readPublicationRef(): Promise<Response> {
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        const response = await fetch(publicationRefUrl, { signal: AbortSignal.timeout(15000) });
        if (response.status === 200 || (response.status !== 429 && response.status < 500) ||
            attempt === 2) return response;
        await response.body?.cancel();
      } catch (error) {
        if (attempt === 2 ||
            !(error instanceof TypeError || (error instanceof Error && error.name === 'TimeoutError'))) {
          throw error;
        }
      }
      await new Promise(resolve => setTimeout(resolve, (attempt + 1) * 300));
    }
    throw new Error('Unreachable publication ref retry limit');
  }
  const publicationRef = await readPublicationRef();
  assert.equal(publicationRef.status, 200, 'Published report branch must be readable');
  const publicationSha = (await publicationRef.json() as { object?: { sha?: string } }).object?.sha;
  assert.match(publicationSha || '', /^[0-9a-f]{40}$/, 'Immutable publication SHA');
  report.immutable_publication = publicationSha!;
  const reportRoot = `https://raw.githubusercontent.com/777genius/universal-agent-plugins-registry/${publicationSha}/reports/skillspector`;
  const client = createSkillSpectorClient(
    reportRoot + '/v2/index.json',
    reportRoot + '/v1/index.json',
    fetch,
    reportRoot + '/v3/index.json',
  );
  assert.ok(await client.coverageSummary(), 'Current published v3 manifest required');

  // The displayed filtered slice can extend beyond the first public API page.
  // Read only the same bounded pages the UI may have loaded, without substitutes.
  const listings = new Map<string, SkillListing>(data.items.map(item => [item.id, item]));
  let nextPage = 1;
  let hasUpstream = data.hasMore;
  async function actualListing(id: string): Promise<SkillListing | undefined> {
    while (!listings.has(id) && hasUpstream && nextPage < 5) {
      const response = await page.request.get(
        `${report.api}?view=popular&page=${nextPage}`,
        { headers: { Origin: 'https://universal-agent-plugins.com' }, timeout: 20000 },
      );
      assert.equal(response.status(), 200, 'Real Skills API pagination required');
      const body = (await response.json()) as SkillsCatalogPage;
      assert.equal(body.page, nextPage);
      assert.ok(Array.isArray(body.items), 'Invalid Skills API page');
      for (const item of body.items) listings.set(item.id, item);
      hasUpstream = body.hasMore;
      nextPage++;
    }
    return listings.get(id);
  }

  for (const scanned of [false, true]) {
    const url = 'https://universal-agent-plugins.com/plugins/?tab=skills&skillRemarks=none' +
      (scanned ? '&skillScan=scanned' : '');
    const result = await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 35000 });
    assert.equal(result?.status(), 200, 'No-remarks filter page must be HTTP 200');
    await expect(defaultCatalog).toHaveAttribute('aria-busy', 'false', { timeout: 90000 });
    await expect(defaultCatalog.locator('.catalog-meta [role="status"]')).toHaveCount(0);
    await defaultCatalog.getByRole('button', { name: /^Filters/ }).click();
    await expect(page.locator('input[aria-labelledby="skills-filter-remarks-label"]')).toBeChecked();
    if (scanned) {
      await expect(page.locator('input[aria-labelledby="skills-filter-scanned-label"]')).toBeChecked();
    } else {
      await expect(page.locator('input[aria-labelledby="skills-filter-scanned-label"]')).not.toBeChecked();
    }
    await page.keyboard.press('Escape');
    const visible = await defaultCatalog.locator('.skill-card').evaluateAll(nodes =>
      nodes.map(node => node.getAttribute('data-skill-id')));
    assert.equal(new Set(visible).size, visible.length, 'Duplicate filtered Skill cards');
    for (const id of visible) {
      assert.ok(id, 'Filtered Skill card must have an exact upstream ID');
      const skill = await actualListing(id!);
      assert.ok(skill, `Unknown Skill from published catalog: ${id}`);
      const subject = publishedScanSubject(skill);
      assert.ok(subject, `Ambiguous published Skill identity: ${id}`);
      const evidence = await client.reportFor(subject.repository);
      assert.ok(hasNoPublishedRemarks(evidence, subject),
        `A visible no-remarks Skill lacks fully inspected, current, zero-finding evidence: ${id}`);
      report.no_remarks_records_verified++;
    }
    if (scanned) report.no_remarks_and_scanned_shown = visible.length;
    else report.no_remarks_filter_shown = visible.length;
  }
  assert.deepEqual(report.page_errors, [], 'No Chromium JavaScript errors on all catalog modes');
  console.log(JSON.stringify(report, null, 2));
} catch (error) {
  report.failure = error instanceof Error ? error.message : String(error);
  throw error;
} finally {
  try { await page.screenshot({ path: join(output, 'skills-catalog.png'), fullPage: true }); } catch { /* page may never have loaded */ }
  writeFileSync(join(output, 'production-report.json'), JSON.stringify(report, null, 2) + '\n');
  await browser.close();
}
