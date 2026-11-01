import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { expect, test, type Page, type BrowserContext } from '@playwright/test';
import { loadRegistryIndex } from '../../build/load-registry';
import type { DiscoverySnapshot } from '../../types/discovery';
import {
  hydrated,
  navigate,
  offline,
  projections,
  registryFixture,
  setSignedMirrorTestTime,
} from './i18n-state.helpers';

const root = new URL('../fixtures/skillspector-v3-published-home/', import.meta.url);
const indexBytes = readFileSync(new URL('index.json', root));
const manifest = JSON.parse(indexBytes.toString());
const shards = manifest.shards.filter((part: { repository: string }) =>
  ['777genius/universal-agent-plugins-registry', 'upstash/context7'].includes(
    part.repository.toLowerCase(),
  ),
);
const bodies = new Map<string, Buffer>([
  ['index.json', indexBytes],
  ...shards.map(
    (part: { path: string }) =>
      [part.path, readFileSync(new URL(part.path, root))] as [string, Buffer],
  ),
]);
const registryShard = shards.find(
  (part: { repository: string }) =>
    part.repository === '777genius/universal-agent-plugins-registry',
).path;
const digest = (text: string) => createHash('sha256').update(text).digest('hex');
const registryRoot = new URL('../../public/registry/schemas/1/', import.meta.url);
const registryPointer = JSON.parse(readFileSync(new URL('latest.json', registryRoot), 'utf8'));
const registry = loadRegistryIndex(
  new URL(registryPointer.snapshot_path, registryRoot).pathname,
  'published_snapshot',
);
const cloudflareSource = registry.plugins.find(
  (plugin) =>
    plugin.source.path === 'plugins/cloudflare' &&
    plugin.source.repository === '777genius/universal-agent-plugins-registry',
)!.install_source;
const discoveryRoot = new URL('../../public/discovery/', import.meta.url);
const discoveryPointer = JSON.parse(readFileSync(new URL('latest.json', discoveryRoot), 'utf8'));
const discovery = JSON.parse(
  readFileSync(new URL(discoveryPointer.snapshot_path, discoveryRoot), 'utf8'),
) as DiscoverySnapshot;
const hasSkillsBySource = new Map([
  ...registry.plugins.map(
    (plugin) => [plugin.install_source, plugin.components.includes('skills')] as const,
  ),
  ...discovery.records.map((record) => [record.slug, record.components.skills > 0] as const),
]);

async function publishedHome(page: Page, context: BrowserContext, isolated = false) {
  const errors = await offline(page);
  await setSignedMirrorTestTime(page);
  const mirrorTime = await page.evaluate(() => Date.now());
  await page.clock.setFixedTime(new Date(Math.max(mirrorTime, Date.parse(manifest.generated_at))));
  await context.addInitScript(() => localStorage.setItem('theme', 'light'));
  let registryRequests = 0;
  const requests = new Map<string, number>();
  await page.route('**/reports/skillspector/v3/**', (route) => {
    const part = new URL(route.request().url()).pathname.split('/v3/')[1]!;
    requests.set(part, (requests.get(part) ?? 0) + 1);
    if (part === registryShard) registryRequests++;
    const body = bodies.get(part);
    return body
      ? route.fulfill({ body, contentType: 'application/json' })
      : route.fulfill({ status: 404 });
  });
  if (isolated) {
    const index = registryFixture();
    const plugin = index.plugins[0]!;
    plugin.components = ['skills'];
    const record = JSON.parse(bodies.get(registryShard)!.toString()).records.find(
      (entry: { plugin_path: string }) => entry.plugin_path === 'plugins/cloudflare',
    );
    Object.assign(plugin.source, {
      repository: record.source.repository,
      path: record.plugin_path,
      revision: record.source.revision,
    });
    for (const distribution of plugin.distributions) {
      Object.assign(distribution.source, plugin.source);
      for (const release of distribution.releases) Object.assign(release.source, plugin.source);
    }
    index.plugins = [plugin];
    await projections(page, index);
    await navigate(page, '/plugins/', 'en', '?q=i18n-state-00');
  } else await page.goto('./');
  await hydrated(page);
  await expect(page.locator('.v-application')).toHaveClass(/v-theme--light/);
  if (!isolated)
    // The large published catalog can finish loading after hydration on a busy CI runner.
    await expect(page.locator('.catalog')).toHaveAttribute('data-discovery-state', 'current', {
      timeout: 20_000,
    });
  // Anchor subsequent probes to stable source identity, not a megabyte of descendant text.
  const report = page.locator(
    `[data-install-source=${JSON.stringify(isolated ? 'i18n-state-00' : cloudflareSource)}] details.skillspector`,
  );
  await expect(report.locator(':scope > summary')).toContainText(/Skill scan · (13\/13 checked · 10 not fully checked|Coverage has limits)/);
  return { report, errors, requests, registryRequests: () => registryRequests };
}

test('published large reports keep closed cards unmounted after real data and app readiness', async ({
  page,
  context,
}) => {
  test.setTimeout(60_000);
  const { report, errors, requests } = await publishedHome(page, context);
  const cardSources = await page
    .locator('.plugin-card')
    .evaluateAll((elements) =>
      elements.map((element) => element.getAttribute('data-install-source')),
    );
  expect(cardSources.length).toBeGreaterThan(20);
  expect(cardSources.every((source) => source !== null && hasSkillsBySource.has(source))).toBe(
    true,
  );
  const expectedSources = cardSources
    .filter((source) => source !== null && hasSkillsBySource.get(source))
    .sort();
  expect(expectedSources.length).toBeGreaterThan(0);
  const assessedSources = await page
    .locator('.plugin-card:has(details.skillspector)')
    .evaluateAll((elements) =>
      elements.map((element) => element.getAttribute('data-install-source')).sort(),
    );
  expect(assessedSources).toEqual(expectedSources);
  await expect(page.locator('details.skillspector')).toHaveCount(expectedSources.length);
  await expect(page.locator('.skillspector__details')).toHaveCount(0);
  await expect(report.locator(':scope > summary')).toContainText(/Skill scan · (13\/13 checked · 10 not fully checked|Coverage has limits)/);
  expect(requests.get('index.json')).toBe(1);
  for (const shard of shards) expect(requests.get(shard.path)).toBe(1);
  expect(errors).toEqual([]);
});

test('published report native keyboard toggles preserve cached evidence and nested disclosure behavior', async ({
  page,
  context,
}) => {
  const { report, errors, registryRequests } = await publishedHome(page, context, true);
  const summary = report.locator(':scope > summary');
  await summary.focus();
  await expect(summary).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(report.locator('.skillspector__details')).toBeVisible();
  const original = await report.evaluate((element) => ({
    open: (element as HTMLDetailsElement).open,
    records: element.querySelectorAll('.skillspector__record').length,
    coverage: element.querySelector('.skillspector__coverage')!.textContent!,
    evidence: element.querySelector('.skillspector__technical')!.textContent!,
  }));
  expect(original.open).toBe(true);
  expect(original.records).toBe(13);
  expect(original.coverage).toContain('node_modules/wrangler/config-schema.json');
  const evidence = digest(original.evidence);
  await report.locator('.skillspector__record > summary').first().click();
  const nested = await report.evaluate((element) => ({
    open: (element as HTMLDetailsElement).open,
    visible: !!element.querySelector('.skillspector__details')!.getBoundingClientRect().height,
  }));
  expect(nested.open).toBe(true);
  expect(nested.visible).toBe(true);
  await report.locator('.skillspector__dialog-close').click();
  await expect(summary).toBeFocused();
  await expect(report).not.toHaveAttribute('open');
  await expect(report.locator('.skillspector__details')).toHaveCount(0);
  await summary.press('Enter');
  await expect(report.locator('.skillspector__details')).toBeVisible();
  const reopened = await report.evaluate((element) => ({
    open: (element as HTMLDetailsElement).open,
    records: element.querySelectorAll('.skillspector__record').length,
    evidence: element.querySelector('.skillspector__technical')!.textContent!,
  }));
  expect(reopened.open).toBe(true);
  expect(reopened.records).toBe(13);
  expect(digest(reopened.evidence)).toBe(evidence);
  expect(registryRequests()).toBe(1);
  expect(errors).toEqual([]);
});

test('published coverage summaries group exact messages and expose all 490 original runtime contexts', async ({
  page,
  context,
}) => {
  const { report, errors } = await publishedHome(page, context, true);
  await report.locator(':scope > summary').click();
  await expect(report.locator('.skillspector__reason-groups > li')).toHaveCount(20);
  const runtime = report
    .locator('.skillspector__reason-groups > li')
    .filter({ has: page.locator('code').filter({ hasText: /^runtime_limit$/ }) })
    .filter({ hasText: 'Inspection reached its configured runtime limit.' });
  await expect(runtime).toHaveCount(1);
  await expect(runtime.locator('summary')).toHaveText('490 original contexts');
  await expect(runtime.locator('pre')).toHaveCount(0);
  await runtime.locator('summary').focus();
  await page.keyboard.press('Enter');
  await expect(runtime.locator('pre')).toBeVisible();
  const originalReasons = JSON.parse(bodies.get(registryShard)!.toString())
    .records.filter(
      (record: { plugin_path: string }) => record.plugin_path === 'plugins/cloudflare',
    )
    .flatMap(
      (record: { coverage: { reasons: Array<{ code: string; message: string }> } }) =>
        record.coverage.reasons,
    )
    .filter(
      (reason: { code: string; message: string }) =>
        reason.code === 'runtime_limit' &&
        reason.message === 'Inspection reached its configured runtime limit.',
    );
  expect(JSON.parse((await runtime.locator('pre').textContent())!)).toEqual(originalReasons);
  await page.keyboard.press('Enter');
  await expect(runtime.locator('pre')).toHaveCount(0);
  await expect(report).toHaveAttribute('open', '');
  expect(errors).toEqual([]);
});
