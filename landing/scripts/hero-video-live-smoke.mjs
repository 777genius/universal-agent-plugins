import assert from 'node:assert/strict';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { chromium, expect } from '@playwright/test';

// Real production media acceptance; assertion authority: tests/browser/hero-demo-video.spec.ts.
const output = resolve(process.env.UAP_HERO_VIDEO_REPORT_DIR || '/tmp/uap-hero-video-live');
mkdirSync(output, { recursive: true });
const site = new URL('https://universal-agent-plugins.com/');
const report = { site: site.href, checked_at: new Date().toISOString(), success: false,
  widths: [1440, 390].map(width => ({ width, status: 'unrun', page_errors: [] })) };
let browser, context, page, timer, work;
const save = () => writeFileSync(resolve(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
async function provenance(receipt) {
  const response = await fetch(new URL('BUILD_PROVENANCE.json', site), {
    redirect: 'error', headers: { 'cache-control': 'no-cache' }, signal: AbortSignal.timeout(30_000),
  });
  assert.equal(response.status, 200);
  const value = await response.json();
  assert.equal(value.schema_version, 1);
  for (const key of ['source_commit', 'source_tree', 'control_commit', 'manifest_sha256'])
    assert.equal(value[key], receipt[key], `Production ${key} differs from receipt`);
  assert.equal(value.workflow_commit, receipt.control_commit, 'Production workflow differs from control');
  for (const [key, name] of [['run_id', 'UAP_PAGES_RUN_ID'], ['run_attempt', 'UAP_PAGES_RUN_ATTEMPT']])
    if (process.env[name]) assert.equal(String(value[key]), process.env[name], `Production ${key} differs`);
  return value;
}
async function run() {
  const receipt = JSON.parse(readFileSync(process.env.UAP_PRODUCTION_SOURCE_RECEIPT, 'utf8'));
  assert.equal(receipt.schema_version, 1);
  for (const key of ['source_commit', 'source_tree', 'control_commit']) assert.match(receipt[key], /^[0-9a-f]{40}$/);
  assert.match(receipt.manifest_sha256, /^[0-9a-f]{64}$/);
  report.receipt = receipt;
  report.before = await provenance(receipt);
  browser = await chromium.launch({ channel: 'chrome' });
  report.browser_version = browser.version();
  try {
    for (const entry of report.widths) {
      entry.status = 'running';
      context = await browser.newContext({ viewport: { width: entry.width, height: 1000 } });
      page = await context.newPage();
      page.setDefaultTimeout(30_000);
      page.on('pageerror', error => entry.page_errors.push(error.message));
      page.on('console', message => {
        if (message.type() === 'error' || /hydration.*(mismatch|failed)/i.test(message.text()))
          entry.page_errors.push(message.text());
      });
      const shot = label => page.screenshot({ path: resolve(output, `${entry.width}-${label}.png`) });
      try {
        const response = await page.goto(site.href);
        assert.equal(response.status(), 200); assert.equal(page.url(), site.href);
        await page.waitForFunction(() => document.querySelector('#__nuxt')
          ?.__vue_app__?.config.globalProperties.$nuxt?.isHydrating === false);
        const demo = page.locator('.hero-demo-video');
        await demo.scrollIntoViewIfNeeded();
        await expect(page.getByRole('link', { name: /^Explore [1-9]/ })).toBeVisible({ timeout: 30_000 });
        await expect.poll(() => demo.locator('img').evaluate(image => image.complete && image.naturalWidth > 0),
          { timeout: 30_000 }).toBe(true);
        const warm = demo.locator('iframe.hero-demo-video__preload');
        await expect(warm).toBeAttached({ timeout: 15_000 });
        await expect(warm).toHaveAttribute('src', /\?preload=auto$/);
        await expect(warm).toHaveAttribute('aria-hidden', 'true');
        await expect(warm).toHaveAttribute('inert', '');
        await expect(warm).toHaveAttribute('tabindex', '-1');
        await expect(warm).not.toBeVisible(); await expect(warm).not.toHaveAttribute('allow', /autoplay/);
        const video = page.frameLocator('.hero-demo-video iframe').locator('video');
        await expect(video).toBeAttached({ timeout: 30_000 });
        expect(await video.evaluate(el => ({ paused: el.paused, time: el.currentTime }))).toEqual({ paused: true, time: 0 });
        await shot('poster');
        await demo.getByRole('button', { name: 'Watch demo' }).click();
        await expect(video).toBeAttached({ timeout: 30_000 });
        await expect.poll(() => video.evaluate(el => el.currentTime), { timeout: 30_000 }).toBeGreaterThan(1);
        entry.media = await video.evaluate(el => ({ duration: el.duration, error: el.error?.code ?? null,
          paused: el.paused, muted: el.muted, currentTime: el.currentTime }));
        expect(Math.abs(entry.media.duration - 2797 / 30)).toBeLessThan(0.25);
        expect(entry.media.error).toBeNull(); expect(entry.media.paused).toBe(false); expect(entry.media.muted).toBe(false);
        await shot('playing');
        await video.evaluate(el => el.pause());
        await expect.poll(() => video.evaluate(el => el.paused)).toBe(true);
        if (await page.evaluate(() => Boolean(document.fullscreenElement))) {
          await expect.poll(() => demo.evaluate(el => document.fullscreenElement === el)).toBe(true);
          // Synthetic Escape does not reliably invoke Chrome's native fullscreen exit.
          // Exercise the real browser exit and the component's fullscreenchange cleanup.
          await page.evaluate(() => document.exitFullscreen());
          await expect.poll(() => page.evaluate(() => document.fullscreenElement === null)).toBe(true);
        } else await demo.getByRole('button', { name: 'Close demo' }).click();
        await expect(demo.locator('iframe')).toHaveCount(0);
        await expect(page.locator('.demo-showcase .hero-demo-video')).toHaveCount(1);
        await expect(demo.getByRole('button', { name: 'Watch demo' })).toBeVisible();
        await shot('restored-poster'); expect(entry.page_errors).toEqual([]); entry.status = 'passed';
      } catch (error) {
        entry.status = 'failed'; entry.error = error.stack || String(error);
        await shot('failure').catch(error => { entry.screenshot_error = String(error); });
        throw error;
      } finally {
        await context?.close().catch(() => {}); context = undefined;
        if (entry.status === 'passed' && entry.page_errors.length) {
          entry.status = 'failed'; entry.error = 'Page errors arrived during browser cleanup';
        }
        save();
      }
    }
  } finally {
    try {
      report.after = await provenance(receipt);
      assert.deepEqual(report.after, report.before, 'Production changed during media acceptance');
    } catch (error) { report.provenance_error = error.stack || String(error); }
  }
}
try {
  work = run();
  await Promise.race([work, new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error('Shared 180000ms production video timeout')), 180_000);
  })]);
  report.success = !report.provenance_error && report.widths.every(entry => entry.status === 'passed');
  if (!report.success) process.exitCode = 1;
} catch (error) {
  report.error = error.stack || String(error); process.exitCode = 1;
} finally {
  clearTimeout(timer);
  // Close active operations on a deadline, then settle their postflight and
  // cleanup before writing the final report. A timeout can never become a pass.
  await context?.close().catch(() => {}); await browser?.close().catch(() => {});
  await work?.catch(() => {});
  for (const entry of report.widths) if (entry.status === 'running') entry.status = 'failed';
  await context?.close().catch(() => {}); await browser?.close().catch(() => {}); save();
}
