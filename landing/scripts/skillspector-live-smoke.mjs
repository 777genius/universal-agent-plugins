import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium, expect } from '@playwright/test';
import { createI18n } from 'vue-i18n';
import { loadI18nMessages } from './load-i18n-messages.ts';
import { createSkillSpectorClient } from '../utils/skillSpectorCatalog.ts';
import {
  decodeSkillSpectorManifestV3,
  decodeSkillSpectorShardV3,
} from '../utils/skillSpectorV3.ts';
import {
  skillSpectorAssessment,
  skillSpectorFindingUrl,
  skillSpectorSourceUrl,
} from '../utils/skillSpector.ts';

// Read-only production acceptance with real requests and no route interception.
// Node 24; install Chromium with `pnpm exec playwright install --only-shell chromium`.
// UAP_E2E_LIVE_SKILLSPECTOR=1 node --experimental-strip-types scripts/skillspector-live-smoke.mjs
// Optional: UAP_SKILLSPECTOR_PUBLICATION_COMMIT pins an already reviewed publication.
// UAP_SKILLSPECTOR_PUBLICATION_ONLY=1 validates every published shard before deployment.
assert.equal(process.env.UAP_E2E_LIVE_SKILLSPECTOR, '1', 'Opt in with UAP_E2E_LIVE_SKILLSPECTOR=1');
const site = new URL(
  `${(process.env.UAP_SKILLSPECTOR_SITE_ORIGIN || 'https://universal-agent-plugins.com').replace(/\/+$/, '')}/`,
);
assert.ok(['https:', 'http:'].includes(site.protocol), 'Expected an HTTP site URL');
assert.ok(
  !site.username && !site.password && !site.search && !site.hash,
  'Expected a site base URL',
);
const manifestUrl =
  process.env.NUXT_PUBLIC_SKILLSPECTOR_V3_MANIFEST_URL ||
  'https://raw.githubusercontent.com/777genius/universal-agent-plugins-registry/skillspector-reports/reports/skillspector/v3/index.json';
const publicationMatch =
  /^https:\/\/raw\.githubusercontent\.com\/777genius\/universal-agent-plugins-registry\/([^/]+)\/reports\/skillspector\/v3\/index\.json$/.exec(
    manifestUrl,
  );
assert.ok(publicationMatch, 'Use the registry v3 publication URL');
const output = resolve(process.env.UAP_LANDING_SCREENSHOT_DIR || '/tmp/uap-skillspector-live');
const chromiumArgs = process.env.UAP_CHROMIUM_ARGS_JSON
  ? JSON.parse(process.env.UAP_CHROMIUM_ARGS_JSON)
  : [];
assert.ok(Array.isArray(chromiumArgs) && chromiumArgs.every((arg) => typeof arg === 'string'));
mkdirSync(output, { recursive: true });
const digest = (bytes) => `sha256:${createHash('sha256').update(bytes).digest('hex')}`;
const parse = (bytes) => JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
const sourceFiles = [
  'scripts/skillspector-live-smoke.mjs',
  'utils/skillSpectorCatalog.ts',
  'utils/skillSpectorV3.ts',
  'utils/skillSpector.ts',
  'types/skillSpector.ts',
  'features/skillspector/exceptions.json',
  'components/registry/SkillSpectorAssessment.vue',
];
const sourceHashes = () =>
  Object.fromEntries(
    sourceFiles.map((file) => [file, digest(readFileSync(new URL(`../${file}`, import.meta.url)))]),
  );
const landing = fileURLToPath(new URL('..', import.meta.url));
const evidence = {
  checked_at: new Date().toISOString(),
  site: site.href,
  requested_manifest: manifestUrl,
  consumer_commit: execFileSync('git', ['rev-parse', 'HEAD'], {
    cwd: landing,
    encoding: 'utf8',
  }).trim(),
  consumer_source_hashes: sourceHashes(),
  declared_deployment_commit: process.env.UAP_LANDING_DEPLOYMENT_SHA || null,
  // A declared SHA is provenance supplied by the deployment owner, not a browser attestation.
  environment: {
    custom_chromium: Boolean(process.env.UAP_CHROMIUM_EXECUTABLE_PATH),
    browser_tls_exception: chromiumArgs.includes('--ignore-certificate-errors'),
  },
  publication: {},
  checks: [],
  responses: [],
  page_errors: [],
  success: false,
};

async function bytes(url, maximum) {
  const response = await fetch(url, { credentials: 'omit', signal: AbortSignal.timeout(60_000) });
  assert.equal(response.status, 200, url);
  assert.ok(
    Number(response.headers.get('content-length')) <= maximum,
    `Oversized response: ${url}`,
  );
  assert.ok(response.body, `Missing response body: ${url}`);
  const reader = response.body.getReader();
  const chunks = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      assert.ok(size <= maximum, `Oversized response: ${url}`);
      chunks.push(value);
    }
  } finally {
    await reader.cancel();
    reader.releaseLock();
  }
  return Buffer.concat(chunks, size);
}
async function json(url) {
  return parse(await bytes(url, 16 << 20));
}
async function eachLimited(items, work) {
  let next = 0;
  await Promise.all(
    Array.from({ length: Math.min(4, items.length) }, async () => {
      while (next < items.length) {
        const index = next++;
        await work(items[index], index);
      }
    }),
  );
}
function wording(locale) {
  const messages = loadI18nMessages(locale);
  const i18n = createI18n({
    legacy: false,
    locale,
    fallbackLocale: false,
    messages: { [locale]: messages },
  });
  return (key, values = {}) => {
    const path = `registryUi.skillSpector.${key}`;
    assert.ok(i18n.global.te(path), `${locale}: missing ${path}`);
    return i18n.global.t(path, values);
  };
}
async function hydrated(page) {
  await page.waitForFunction(
    () =>
      document.querySelector('#__nuxt')?.__vue_app__?.config.globalProperties.$nuxt?.isHydrating ===
      false,
  );
}

let browser;
try {
  let publication = process.env.UAP_SKILLSPECTOR_PUBLICATION_COMMIT || publicationMatch[1];
  if (!/^[0-9a-f]{40}$/.test(publication)) {
    const ref = await json(
      `https://api.github.com/repos/777genius/universal-agent-plugins-registry/git/ref/heads/${encodeURIComponent(publication)}`,
    );
    publication = ref.object?.sha;
  }
  assert.match(publication || '', /^[0-9a-f]{40}$/, 'Expected an immutable publication commit');
  const immutableUrl = manifestUrl.replace(
    `/${publicationMatch[1]}/reports/`,
    `/${publication}/reports/`,
  );
  const manifestBytes = await bytes(immutableUrl, 16 << 20);
  const manifest = decodeSkillSpectorManifestV3(parse(manifestBytes));
  assert.ok(manifest.coverage.reported > 128, 'Production must contain broader actual reports');
  Object.assign(evidence.publication, {
    commit: publication,
    manifest_url: immutableUrl,
    manifest_sha256: digest(manifestBytes),
    source_commit: manifest.source_commit,
    scanner: manifest.scanner,
    profile: manifest.profile,
    generated_at: manifest.generated_at,
    coverage: manifest.coverage,
    shards: manifest.shards.length,
  });
  const bodies = new Map([['index.json', manifestBytes]]);
  const records = [];
  await eachLimited(manifest.shards, async (descriptor) => {
    const body = await bytes(new URL(descriptor.path, immutableUrl).href, descriptor.bytes);
    assert.equal(body.byteLength, descriptor.bytes, descriptor.path);
    assert.equal(digest(body), descriptor.digest, descriptor.path);
    records.push(...decodeSkillSpectorShardV3(parse(body), manifest, descriptor));
    bodies.set(descriptor.path, body);
  });
  assert.equal(records.length, manifest.coverage.reported);
  assert.equal(
    new Set(records.map((record) => record.id)).size,
    records.length,
    'Record IDs must be globally unique',
  );
  const current = records.filter((record) => record.freshness === 'current');
  const expectedCoverage = {
    selected: manifest.coverage.selected,
    reported: records.length,
    pending: manifest.coverage.selected - current.length,
    complete: current.filter((record) => record.status === 'complete').length,
    partial: current.filter((record) => record.status === 'partial').length,
    error: current.filter((record) => record.status === 'error').length,
    stale: records.length - current.length,
  };
  assert.deepEqual(
    manifest.coverage,
    expectedCoverage,
    'Global coverage must match current records',
  );
  const requests = new Map();
  const transport = async (url, options) => {
    assert.equal(options?.credentials, 'omit');
    assert.ok(
      url.startsWith(new URL('./', immutableUrl).href),
      'Unexpected fallback or report URL',
    );
    const path = url.slice(new URL('./', immutableUrl).href.length);
    const body = bodies.get(path);
    assert.ok(body, `Unexpected report file: ${path}`);
    requests.set(path, (requests.get(path) || 0) + 1);
    return new Response(body, {
      headers: { 'content-type': 'application/json', 'content-length': String(body.length) },
    });
  };
  const client = createSkillSpectorClient(
    immutableUrl.replace('/v3/', '/v2/'),
    immutableUrl.replace('/v3/', '/v1/'),
    transport,
    immutableUrl,
  );
  const reports = new Map();
  const repositories = [...new Set(manifest.shards.map((shard) => shard.repository))];
  await eachLimited(repositories, async (repository) => {
    const report = await client.reportFor(repository);
    assert.equal(report.schema_version, 3, 'All repositories must use v3');
    reports.set(repository.toLowerCase(), report);
  });
  assert.equal(requests.get('index.json'), 1, 'The actual loader must share its manifest');
  assert.ok(
    [...requests.values()].every((count) => count === 1),
    'The actual loader must share shards',
  );
  for (const alias of manifest.repository_aliases) {
    const original = await client.reportFor(alias.repository);
    const canonical = await client.reportFor(alias.canonical_repository);
    assert.deepEqual(
      original.records,
      canonical.records,
      'Alias join must preserve canonical evidence',
    );
  }
  Object.assign(evidence.publication, {
    strict_shards_verified: manifest.shards.length,
    records_verified: records.length,
    repositories_verified: reports.size,
    loader_shards_verified: requests.size - 1,
    reference_evidence: current.reduce(
      (count, record) => count + (record.coverage.references?.length || 0),
      0,
    ),
    ledger_evidence: current.reduce(
      (count, record) => count + (record.coverage.ledger_exceptions?.length || 0),
      0,
    ),
    success: true,
  });

  if (process.env.UAP_SKILLSPECTOR_PUBLICATION_ONLY !== '1') {
    const projection = await json(new URL('api/registry/plugin/cloudflare', site).href);
    const plugin = projection.plugins.find((item) => item.name === 'cloudflare');
    assert.ok(plugin, 'The real Directory must expose the representative skill-bearing plugin');
    const report = await client.reportFor(plugin.source.repository);
    const assessment = skillSpectorAssessment(report, {
      kind: 'plugin',
      repository: plugin.source.repository,
      path: plugin.source.path,
      revision: plugin.source.revision,
    });
    assert.ok(
      assessment.records.length && assessment.inventoryKnown,
      'Require exact current plugin inventory',
    );
    assert.ok(
      !assessment.outOfDate && assessment.records.every((record) => record.freshness === 'current'),
      'The representative plugin must match the current source revision',
    );
    const main = assessment.records.find(
      (record) => record.source.path === `${plugin.source.path}/skills/cloudflare`,
    );
    assert.ok(main, 'The main Cloudflare skill must retain its report');
    assert.ok(
      main.coverage.references?.length && main.coverage.ledger_exceptions?.length,
      'Require original reference/diagnostic evidence',
    );
    assert.equal(
      main.execution.state,
      'completed',
      'The extended retry must have actually completed',
    );
    assert.ok(
      main.retry.attempt_count >= 2 && main.attempt_history.length >= 2,
      'Require both original and retry attempt evidence',
    );
    assert.ok(
      main.attempt_history.some((attempt) => attempt.profile.id === 'static-standard-v3'),
      'Retain the original standard-profile attempt',
    );
    const latest = main.attempt_history.at(-1);
    assert.equal(main.profile.id, 'static-extended-v3');
    assert.deepEqual(
      latest.profile,
      main.profile,
      'The latest attempt must be the actual extended profile',
    );
    assert.equal(latest.at, main.execution.finished_at);
    assert.equal(latest.status, main.status);
    const classified = main.findings.filter(
      (finding) => finding.review_status === 'false_positive',
    );
    assert.equal(
      classified.length,
      2,
      'Both narrowly matched registry annotations must remain available',
    );
    evidence.representative = {
      source: plugin.source,
      status: assessment.status,
      fully_checked: assessment.fullyChecked,
      selected: assessment.selected,
      main: {
        id: main.id,
        source: main.source,
        status: main.status,
        profile: main.profile,
        retry: main.retry,
        attempt_history: main.attempt_history,
        references: main.coverage.references.length,
        diagnostics: main.coverage.ledger_exceptions.length,
        classified_groups: classified.length,
      },
    };

    // Use actual catalog identity rather than joining a standalone report by title alone.
    let standalone;
    const candidates = [...manifest.source_states]
      .filter((state) => state.state === 'resolved' && state.current)
      .sort((a, b) => a.id.localeCompare(b.id));
    for (const state of candidates.slice(0, 40)) {
      const candidateReport = await client.reportFor(state.repository);
      const record = candidateReport.records.find(
        (item) =>
          state.selected_ids.includes(item.id) &&
          item.kind === 'standalone_skill' &&
          item.skill_name,
      );
      if (!record) continue;
      const page = await json(
        `https://api.universal-agent-plugins.com/v1/skills?q=${encodeURIComponent(record.skill_name)}&limit=100`,
      );
      const listing = page.items.find(
        (item) =>
          item.source.repository?.toLowerCase() === state.repository.toLowerCase() &&
          item.skillNameHint === record.skill_name,
      );
      if (listing) {
        standalone = { report: candidateReport, listing, record };
        break;
      }
    }
    assert.ok(standalone, 'A v3 report must match an actual standalone catalog listing');
    browser = await chromium.launch({
      ...(process.env.UAP_CHROMIUM_EXECUTABLE_PATH
        ? { executablePath: process.env.UAP_CHROMIUM_EXECUTABLE_PATH }
        : {}),
      args: chromiumArgs,
    });
    for (const { locale, width } of [
      { locale: 'en', width: 1280 },
      { locale: 'en', width: 390 },
      { locale: 'ru', width: 1280 },
      { locale: 'ru', width: 390 },
    ]) {
      const t = wording(locale);
      const context = await browser.newContext({
        viewport: { width, height: 1000 },
        hasTouch: width < 600,
      });
      const page = await context.newPage();
      const responseChecks = [];
      const seen = new Map();
      // Cache headers as they arrive, so a small envelope cannot race the
      // pointer-body callback. No requests are intercepted or synthesized.
      const signedResponses = new Map();
      page.on('response', (response) => {
        if (/\/(discovery|security)\//.test(new URL(response.url()).pathname))
          signedResponses.set(response.url(), response);
      });
      const signedReceived = Promise.all(
        ['discovery', 'security'].map(async (kind) => {
          const pointerResponse = await page.waitForResponse(
            (response) =>
              new URL(response.url()).pathname.endsWith(`/${kind}/latest.json`) &&
              response.status() === 200,
          );
          const pointer = parse(await pointerResponse.body());
          const fields =
            kind === 'discovery' ? ['snapshot', 'envelope', 'search'] : ['snapshot', 'envelope'];
          const artifacts = await Promise.all(
            fields.map(async (field) => {
              const url = new URL(pointer[`${field}_path`], pointerResponse.url()).href;
              const response =
                signedResponses.get(url) ||
                (await page.waitForResponse((response) => response.url() === url));
              assert.equal(response.status(), 200, url);
              const body = await response.body();
              assert.ok(
                body.length <= pointer.fetch_contract[`${field}_max_bytes`],
                `Oversized signed feed: ${url}`,
              );
              return { url, bytes: body.length, sha256: digest(body) };
            }),
          );
          return {
            kind,
            pointer_url: pointerResponse.url(),
            sequence: pointer.sequence,
            artifacts,
          };
        }),
      ).then(
        (feeds) => ({ feeds }),
        (error) => ({ error }),
      );
      await page.addInitScript(() => {
        window.__UAP_LIVE_DISCLOSURE_EVENTS__ = [];
        const describe = (element) =>
          element instanceof Element
            ? {
                tag: element.tagName,
                class: element.className,
                label: element.getAttribute('aria-label'),
              }
            : null;
        for (const type of ['focusin', 'focusout', 'toggle'])
          document.addEventListener(
            type,
            (event) => {
              if (
                !(event.target instanceof Element) ||
                !event.target.closest('.plugin-card[data-install-source="cloudflare"]')
              )
                return;
              const panel = document.querySelector(
                '.plugin-card[data-install-source="cloudflare"] .skillspector',
              );
              window.__UAP_LIVE_DISCLOSURE_EVENTS__.push({
                at_ms: performance.now(),
                type,
                target: describe(event.target),
                active: describe(document.activeElement),
                open: panel?.open,
                bodies: panel?.querySelectorAll('.skillspector__details').length,
                records: panel?.querySelectorAll('.skillspector__record').length,
              });
              if (window.__UAP_LIVE_DISCLOSURE_EVENTS__.length > 100)
                window.__UAP_LIVE_DISCLOSURE_EVENTS__.shift();
            },
            true,
          );
      });
      page.on('pageerror', (error) =>
        evidence.page_errors.push({ locale, width, message: error.message }),
      );
      page.on('console', (message) => {
        if (/hydration.*(mismatch|failed)/i.test(message.text()))
          evidence.page_errors.push({ locale, width, message: message.text() });
      });
      page.on('response', (response) => {
        if (!response.url().includes('/reports/skillspector/')) return;
        const check = (async () => {
          assert.equal(response.status(), 200, response.url());
          const part = response.url().split('/reports/skillspector/v3/')[1];
          assert.ok(part && bodies.has(part), 'The deployed consumer must use this v3 publication');
          const body = await response.body();
          assert.equal(
            digest(body),
            digest(bodies.get(part)),
            `Live browser report differs: ${part}`,
          );
          seen.set(part, (seen.get(part) || 0) + 1);
          evidence.responses.push({
            locale,
            width,
            url: response.url(),
            bytes: body.length,
            sha256: digest(body),
            status: response.status(),
          });
        })();
        // Record rejections immediately; await all checks before completing each context.
        responseChecks.push(
          check.then(
            () => null,
            (error) => error,
          ),
        );
      });
      // Receipt of actual report bodies is distinct from rendering. Keep the
      // consumer's own request deadlines and ordinary 5-second UI assertions.
      const reportPaths = (repository) => {
        const canonical = String(
          manifest.repository_aliases.find(
            (alias) => alias.repository.toLowerCase() === repository.toLowerCase(),
          )?.canonical_repository || repository,
        ).toLowerCase();
        return [
          'index.json',
          ...manifest.shards
            .filter((shard) => shard.repository.toLowerCase() === canonical)
            .map((shard) => shard.path),
        ];
      };
      const receiving = (repository) =>
        Promise.all(
          reportPaths(repository)
            .filter((path) => !seen.has(path))
            .map(async (path) => {
              const response = await page.waitForResponse((response) =>
                response.url().endsWith(`/reports/skillspector/v3/${path}`),
              );
              assert.equal(response.status(), 200, response.url());
              assert.equal(
                digest(await response.body()),
                digest(bodies.get(path)),
                `Live report differs: ${path}`,
              );
            }),
        ).then(
          () => null,
          (error) => error,
        );
      const pluginReceived = receiving(plugin.source.repository);
      const prefix = locale === 'en' ? '' : `${locale}/`;
      await page.goto(new URL(`${prefix}plugins/?q=cloudflare`, site).href, {
        waitUntil: 'domcontentloaded',
      });
      await hydrated(page);
      const pluginReceiptError = await pluginReceived;
      if (pluginReceiptError) throw pluginReceiptError;
      const signedReceipt = await signedReceived;
      if (signedReceipt.error) throw signedReceipt.error;
      evidence.signed_feeds ||= [];
      evidence.signed_feeds.push({ locale, width, feeds: signedReceipt.feeds });
      // Native actions require the verified final Directory, after actual signed-feed receipt.
      await expect(page.locator('.catalog')).toHaveAttribute(
        'data-discovery-state',
        /current|cached/,
      );
      const panel = page.locator('.plugin-card[data-install-source="cloudflare"] .skillspector');
      // Keep assertions at Playwright's ordinary 5-second budget.
      await expect(panel).toHaveClass(new RegExp(`skillspector--${assessment.status}(?:\\s|$)`));
      const summary = panel.locator(':scope > summary');
      // Keep the actual fully-checked count in the evidence, while the card
      // reports the number scanned with explicit limited coverage.
      const outcomes = assessment.outcomes;
      const scanned = (outcomes?.complete ?? 0) + (outcomes?.limited ?? 0);
      const scannedLabel = t('fullyChecked', {
        checked: new Intl.NumberFormat(locale).format(scanned),
        total: new Intl.NumberFormat(locale).format(assessment.selected),
      });
      await expect(summary).toContainText(t('title'));
      const showProgress =
        report.schema_version === 3 &&
        !report.records.some((record) => record.legacy_format) &&
        assessment.status === 'incomplete' &&
        assessment.inventoryKnown &&
        (outcomes?.limited ?? 0) > 0 &&
        (outcomes?.failed ?? 0) === 0 &&
        (assessment.selected ?? 0) > 1;
      const expectedSummary = showProgress
        ? t('coverageProgress', {
            checked: new Intl.NumberFormat(locale).format(scanned),
            total: new Intl.NumberFormat(locale).format(assessment.selected),
            limited: new Intl.NumberFormat(locale).format(outcomes.limited),
          })
        : assessment.status === 'incomplete' &&
            (outcomes?.limited ?? 0) > 0 &&
            (outcomes?.failed ?? 0) === 0 &&
            (outcomes?.unscanned ?? 0) === 0
          ? t('outcomelimited')
          : t(assessment.status === 'noStaticFindings' ? 'scanComplete' : assessment.status);
      await expect(summary).toContainText(expectedSummary);
      await expect(summary).not.toContainText(scannedLabel);
      await expect(panel.locator('.skillspector__details')).toHaveCount(0);
      await summary.focus();
      await page.keyboard.press('Enter');
      await expect(panel.locator('.skillspector__details')).toBeVisible();
      assert.deepEqual(
        await panel
          .locator('.skillspector__details > section, .skillspector__details > details')
          .evaluateAll((nodes) => nodes.map((node) => node.className)),
        [
          'skillspector__advice',
          'skillspector__signals',
          'skillspector__coverage',
          'skillspector__technical',
        ],
      );
      await expect(panel.locator('.skillspector__advice')).toContainText(t('installAdvice'));
      await expect(panel.locator('.skillspector__advice')).toContainText(scannedLabel);
      const outcomeRows = Object.entries(assessment.outcomes).filter(([, count]) => count > 0);
      await expect(panel.locator('.skillspector__outcomes dt')).toHaveText(
        outcomeRows.map(([outcome]) => t(outcome === 'complete' ? 'scanComplete' : `outcome${outcome}`)),
      );
      await expect(panel.locator('.skillspector__outcomes dd')).toHaveText(
        outcomeRows.map(([, count]) => new Intl.NumberFormat(locale).format(count)),
      );
      await expect(panel.locator('.skillspector__signals')).toContainText(t('signalExplanation'));
      const technicalRecords = await panel
        .locator('.skillspector__technical > div > details > pre')
        .allTextContents();
      assert.deepEqual(
        technicalRecords.map((value) => JSON.parse(value)),
        assessment.records,
        'Rendered technical evidence must be lossless',
      );
      const recordIndex = assessment.records.indexOf(main);
      const mainPanel = panel.locator('.skillspector__record').nth(recordIndex);
      await mainPanel.locator(':scope > summary').click();
      await expect(panel).toHaveAttribute('open', '');
      const classifications = mainPanel.locator('.skillspector__classification');
      await expect(classifications).toHaveCount(2);
      for (const [index, finding] of classified.entries()) {
        await expect(classifications.nth(index)).toContainText(finding.exception.id);
        await expect(classifications.nth(index)).toContainText(finding.exception.reason);
        await expect(classifications.nth(index)).toContainText(t('classificationExplanation'));
      }
      const expectedLinks = main.findings.flatMap((finding) =>
        (finding.occurrences?.length ? finding.occurrences : [finding])
          .map((position) => skillSpectorFindingUrl(main, { ...finding, ...position }))
          .filter(Boolean),
      );
      assert.deepEqual(
        await mainPanel
          .locator('.skillspector__findings > li > a')
          .evaluateAll((nodes) => nodes.map((node) => node.href)),
        expectedLinks,
      );
      const referencePanel = panel
        .locator('.skillspector__context-record')
        .filter({ has: page.locator(':scope > summary', { hasText: /^cloudflare ·/ }) });
      await referencePanel.locator(':scope > summary').click();
      await expect(referencePanel.locator('.skillspector__references > li')).toHaveCount(
        main.coverage.references.length,
      );
      await expect(referencePanel.locator('.skillspector__diagnostics > ul > li')).toHaveCount(
        main.coverage.ledger_exceptions.length,
      );
      await expect(panel.locator('.skillspector__coverage')).toContainText(
        'node_modules/wrangler/config-schema.json',
      );
      await panel.locator('.skillspector__dialog-close').click();
      await expect(summary).toBeFocused();
      await expect(panel).not.toHaveAttribute('open');
      await expect(panel.locator('.skillspector__details')).toHaveCount(0);
      await summary.press('Enter');
      await expect(panel.locator('.skillspector__details')).toBeVisible();
      assert.deepEqual(
        await panel.locator('.skillspector__technical > div > details > pre').allTextContents(),
        technicalRecords,
        'Close/reopen must preserve evidence',
      );
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        true,
        'No horizontal page overflow',
      );
      await panel.locator('.skillspector__dialog').screenshot({ path: resolve(output, `plugin-${locale}-${width}.png`) });
      await panel.locator('.skillspector__dialog-close').click();
      const standaloneAssessment = skillSpectorAssessment(standalone.report, {
        kind: 'skill',
        repository: standalone.listing.source.repository,
        name: standalone.record.skill_name,
      });
      const standaloneReceived = receiving(standalone.listing.source.repository);
      await page.evaluate(
        async (target) =>
          document
            .querySelector('#__nuxt')
            .__vue_app__.config.globalProperties.$router.push(target),
        `${site.pathname}${prefix}plugins/?tab=skills&q=${encodeURIComponent(standalone.record.skill_name)}`,
      );
      const standaloneReceiptError = await standaloneReceived;
      if (standaloneReceiptError) throw standaloneReceiptError;
      const skill = page.locator(
        `.skill-card[data-skill-id=${JSON.stringify(standalone.listing.id)}]`,
      );
      const skillPanel = skill.locator('.skillspector');
      await expect(skillPanel).toHaveClass(
        new RegExp(`skillspector--${standaloneAssessment.status}(?:\\s|$)`),
      );
      await skillPanel.locator(':scope > summary').click();
      await expect(skillPanel.locator('.skillspector__advice')).toContainText(t('unverified'));
      assert.deepEqual(
        (
          await skillPanel
            .locator('.skillspector__technical > div > details > pre')
            .allTextContents()
        ).map((value) => JSON.parse(value)),
        standaloneAssessment.records,
      );
      await skillPanel.locator('.skillspector__dialog').screenshot({
        path: resolve(output, `standalone-${locale}-${width}.png`),
      });
      const failures = (await Promise.all(responseChecks)).filter(Boolean);
      assert.deepEqual(
        failures.map((error) => error.message),
        [],
        'Every browser report response must match immutable bytes',
      );
      assert.equal(
        seen.get('index.json'),
        1,
        'The manifest must be shared across catalogs and close/reopen',
      );
      assert.ok(
        [...seen.values()].every((count) => count === 1),
        'Each report shard must be fetched once per session',
      );
      const requestedRepositories = [
        plugin.source.repository,
        standalone.listing.source.repository,
      ].map((repository) =>
        String(
          manifest.repository_aliases.find(
            (alias) => alias.repository.toLowerCase() === repository.toLowerCase(),
          )?.canonical_repository || repository,
        ).toLowerCase(),
      );
      for (const descriptor of manifest.shards.filter((shard) =>
        requestedRepositories.includes(shard.repository.toLowerCase()),
      ))
        assert.equal(
          seen.get(descriptor.path),
          1,
          `Real browser must verify required shard ${descriptor.path}`,
        );
      evidence.checks.push({
        locale,
        width,
        plugin_status: assessment.status,
        selected: assessment.selected,
        fully_checked: assessment.fullyChecked,
        outcomes: assessment.outcomes,
        rendered_records: technicalRecords.length,
        classified_groups: classified.length,
        references: main.coverage.references.length,
        standalone: {
          id: standalone.listing.id,
          status: standaloneAssessment.status,
          source: skillSpectorSourceUrl(standalone.record),
        },
        native_close_reopen: true,
        nested_disclosure: true,
        section_order: true,
        publication_bytes_match: true,
      });
      await context.close();
    }
    assert.deepEqual(evidence.page_errors, []);
  }
  assert.deepEqual(
    sourceHashes(),
    evidence.consumer_source_hashes,
    'Consumer source changed during acceptance',
  );
  evidence.success = true;
} catch (error) {
  evidence.error = error.stack || String(error);
  evidence.browser_state = [];
  for (const page of browser?.contexts().flatMap((context) => context.pages()) || []) {
    try {
      evidence.browser_state.push(
        await page.evaluate(() => ({
          url: location.href,
          discovery: document.querySelector('.catalog')?.getAttribute('data-discovery-state'),
          active_element:
            document.activeElement instanceof Element
              ? {
                  tag: document.activeElement.tagName,
                  class: document.activeElement.className,
                  label: document.activeElement.getAttribute('aria-label'),
                }
              : null,
          disclosure_events: window.__UAP_LIVE_DISCLOSURE_EVENTS__ || [],
          reports: Array.from(
            document.querySelectorAll(
              '.plugin-card[data-install-source="cloudflare"] .skillspector',
            ),
          ).map((panel) => ({
            status: panel.className,
            open: panel.open,
            details: panel.querySelectorAll('.skillspector__details').length,
            records: Array.from(panel.querySelectorAll('.skillspector__record > summary')).map(
              (summary) => summary.textContent,
            ),
          })),
        })),
      );
    } catch {
      /* A closed/crashed page is already described by the original failure. */
    }
  }
  process.exitCode = 1;
} finally {
  await browser?.close();
  evidence.completed_at = new Date().toISOString();
  writeFileSync(resolve(output, 'evidence.json'), JSON.stringify(evidence, null, 2) + '\n');
}
console.log(JSON.stringify(evidence, null, 2));
