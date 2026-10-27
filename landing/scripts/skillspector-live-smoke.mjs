import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { chromium, expect as baseExpect } from '@playwright/test';
import {
  createSkillSpectorClient,
  decodeSkillSpectorManifest,
} from '../utils/skillSpectorCatalog.ts';
import {
  skillSpectorAssessment,
  skillSpectorFindingUrl,
  skillSpectorSourceUrl,
} from '../utils/skillSpector.ts';

// Explicit read-only production proof. Run with Node 24; no route interception.
// UAP_E2E_LIVE_SKILLSPECTOR=1 node --experimental-strip-types scripts/skillspector-live-smoke.mjs
assert.equal(process.env.UAP_E2E_LIVE_SKILLSPECTOR, '1', 'Opt in with UAP_E2E_LIVE_SKILLSPECTOR=1');
const origin = process.env.UAP_SKILLSPECTOR_SITE_ORIGIN || 'https://universal-agent-plugins.com';
const manifestUrl =
  process.env.NUXT_PUBLIC_SKILLSPECTOR_MANIFEST_URL ||
  'https://raw.githubusercontent.com/777genius/universal-agent-plugins-registry/skillspector-reports/reports/skillspector/v2/index.json';
const legacyUrl = manifestUrl.replace('/v2/', '/v1/');
const output = resolve(process.env.UAP_LANDING_SCREENSHOT_DIR || '/tmp/uap-skillspector-live');
mkdirSync(output, { recursive: true });
async function json(url) {
  const response = await fetch(url);
  assert.equal(response.status, 200, url);
  return response.json();
}
const manifest = decodeSkillSpectorManifest(await json(manifestUrl));
assert.ok(manifest.coverage.selected > 128, 'Automatic selection must exceed the previous sample');
assert.ok(
  manifest.coverage.reported > 128,
  'The new pipeline must have published broader actual reports',
);
const client = createSkillSpectorClient(manifestUrl, legacyUrl);
const projection = await json(`${origin}/api/registry/plugin/cloudflare`);
const plugin = projection.plugins.find((item) => item.name === 'cloudflare');
assert.ok(plugin, 'The real Directory must expose the representative skill-bearing plugin');
const pluginReport = await client.reportFor(plugin.source.repository);
const pluginAssessment = skillSpectorAssessment(pluginReport, {
  kind: 'plugin',
  repository: plugin.source.repository,
  path: plugin.source.path,
  revision: plugin.source.revision,
});
assert.ok(pluginAssessment.records.length, 'Real plugin evidence must be present');
let standalone;
for (const repository of [...new Set(manifest.shards.map((shard) => shard.repository))].slice(
  0,
  40,
)) {
  const report = await client.reportFor(repository);
  const candidates = report.records
    .filter((record) => record.kind === 'standalone_skill' && record.skill_name)
    .sort((a, b) => b.findings.length - a.findings.length);
  for (const record of candidates.slice(0, 3)) {
    const page = await json(
      `https://api.universal-agent-plugins.com/v1/skills?q=${encodeURIComponent(record.skill_name)}&limit=100`,
    );
    const listing = page.items.find(
      (item) =>
        item.source.repository?.toLowerCase() === repository.toLowerCase() &&
        item.skillNameHint === record.skill_name,
    );
    if (listing) {
      standalone = { record, report, listing };
      break;
    }
  }
  if (standalone) break;
}
assert.ok(
  standalone,
  'At least one published standalone report must match a real current catalog listing',
);
const evidence = {
  checked_at: new Date().toISOString(),
  source_commit: manifest.source_commit,
  coverage: manifest.coverage,
  profile: manifest.profile,
  responses: [],
  checks: [],
  page_errors: [],
};
const expect = baseExpect.configure({ timeout: 45_000 });
const browser = await chromium.launch({
  ...(process.env.UAP_CHROMIUM_EXECUTABLE_PATH
    ? { executablePath: process.env.UAP_CHROMIUM_EXECUTABLE_PATH }
    : {}),
  args: process.env.UAP_CHROMIUM_ARGS_JSON ? JSON.parse(process.env.UAP_CHROMIUM_ARGS_JSON) : [],
});
async function inspect(page, locator, assessment, label) {
  const visual =
    label === 'standalone' && assessment.status === 'noFindings' ? 'historical' : assessment.status;
  await expect(locator).toHaveClass(new RegExp(`skillspector--${visual}(?:\\s|$)`));
  await locator.locator(':scope > summary').click();
  for (const child of await locator.locator('.skillspector__record').all())
    await child.locator(':scope > summary').click();
  const links = await locator
    .locator('.skillspector__findings a')
    .evaluateAll((nodes) => nodes.map((node) => node.href));
  assert.deepEqual(
    links,
    assessment.records.flatMap((record) =>
      record.findings.map((finding) => skillSpectorFindingUrl(record, finding)).filter(Boolean),
    ),
  );
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  return {
    kind: label,
    status: assessment.status,
    records: assessment.records.length,
    finding_count: assessment.findingCount,
    first_finding: links[0] || null,
  };
}
try {
  for (const width of [1280, 390]) {
    const page = await browser.newPage({
      viewport: { width, height: 1000 },
      hasTouch: width < 600,
    });
    page.on('pageerror', (error) => evidence.page_errors.push(error.message));
    page.on('response', (response) => {
      if (response.url().includes('/reports/skillspector/v2/'))
        evidence.responses.push({ width, url: response.url(), status: response.status() });
    });
    await page.goto(`${origin}/plugins/?q=cloudflare`, { waitUntil: 'domcontentloaded' });
    const pluginCard = page.locator('.plugin-card[data-install-source="cloudflare"]');
    evidence.checks.push({
      width,
      ...(await inspect(page, pluginCard.locator('.skillspector'), pluginAssessment, 'plugin')),
    });
    await pluginCard.screenshot({ path: resolve(output, `plugin-${width}.png`) });
    // Use the real router to preserve the session cache between the two catalogs.
    const target = `/plugins/?tab=skills&q=${encodeURIComponent(standalone.record.skill_name)}`;
    await page.evaluate(async (target) => {
      const app = document.querySelector('#__nuxt').__vue_app__;
      await app.config.globalProperties.$router.push(target);
    }, target);
    const skill = page.locator(
      `.skill-card[data-skill-id=${JSON.stringify(standalone.listing.id)}]`,
    );
    const assessment = skillSpectorAssessment(standalone.report, {
      kind: 'skill',
      repository: standalone.record.source.repository,
      name: standalone.record.skill_name,
    });
    evidence.checks.push({
      width,
      source: skillSpectorSourceUrl(standalone.record),
      ...(await inspect(page, skill.locator('.skillspector'), assessment, 'standalone')),
    });
    await expect(skill).toContainText(
      'The current revision of this listing has not been verified.',
    );
    await skill.screenshot({ path: resolve(output, `standalone-${width}.png`) });
    assert.equal(
      evidence.responses.filter(
        (response) => response.width === width && response.url === manifestUrl,
      ).length,
      1,
    );
    await page.close();
  }
  assert.deepEqual(evidence.page_errors, []);
  assert.ok(
    evidence.responses.every((response) => response.status === 200),
    'Every real v2 manifest/shard response must succeed',
  );
} finally {
  await browser.close();
  writeFileSync(resolve(output, 'evidence.json'), JSON.stringify(evidence, null, 2) + '\n');
}
console.log(JSON.stringify(evidence, null, 2));
