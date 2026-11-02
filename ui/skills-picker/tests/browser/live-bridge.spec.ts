import { test, expect } from '@playwright/test';
import { spawn } from 'node:child_process';
import { mkdtemp, readFile, writeFile, rm, realpath } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';

test('production Go bridge installs two raw Skills and recovers a lost apply response', async ({ page }, testInfo) => {
  test.skip(process.env.UAP_SKILLS_LIVE !== '1', 'Run after npm run bundle:go with UAP_SKILLS_LIVE=1');
  test.setTimeout(300_000);
  page.setDefaultTimeout(15_000);
  page.setDefaultNavigationTimeout(30_000);
  const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../../..');
  const scratch = await mkdtemp(join(tmpdir(), 'uap-skills-browser-'));
  const ready = join(scratch, 'ready.json');
  const child = spawn('go', ['test', './internal/agentpluginscli', '-run', '^TestSkillsBrowserFixture$', '-count=1', '-timeout=5m'], {
    cwd: join(root, 'cli'), env: { ...process.env, TMPDIR: await realpath(tmpdir()), UAP_SKILLS_BROWSER_READY: ready }, stdio: ['ignore', 'pipe', 'pipe'],
  });
  let output = '', closed = false, verified = false;
  child.stdout.on('data', (chunk: Buffer) => { output = (output + chunk.toString()).slice(-65536); });
  child.stderr.on('data', (chunk: Buffer) => { output = (output + chunk.toString()).slice(-65536); });
  const completion = new Promise<number | null>((done, reject) => {
    child.once('error', reject);
    child.once('close', (code) => { closed = true; done(code); });
  });
  try {
    let url = '';
    const deadline = Date.now() + 180_000;
    while (!url && Date.now() < deadline) {
      if (closed) throw new Error(`Go fixture exited before readiness:\n${output}`);
      try { url = (JSON.parse(await readFile(ready, 'utf8')) as { url: string }).url; }
      catch (error) { if (!(error instanceof SyntaxError) && (error as NodeJS.ErrnoException).code !== 'ENOENT') throw error; }
      if (!url) await delay(100);
    }
    if (!url) throw new Error(`Go fixture readiness timeout:\n${output}`);
    const origin = new URL(url).origin;
    const external: string[] = [];
    const errors: string[] = [];
    let applyRequests = 0;
    page.on('request', (request) => {
      if (/^https?:/.test(request.url()) && !request.url().startsWith(`${origin}/`)) external.push(request.url());
      if (request.method() === 'POST' && /\/api\/v1\/plans\/[^/]+\/apply$/.test(request.url())) applyRequests++;
    });
    page.on('pageerror', (error) => errors.push(error.message));
    await page.goto(url);
    await expect(page.getByRole('checkbox', { name: 'Select engineering/review', exact: true })).toBeVisible({ timeout: 30_000 });
    expect(new URL(page.url()).hash).toBe('');
    await page.getByRole('checkbox', { name: 'Select engineering/review', exact: true }).check();
    await page.getByRole('textbox', { name: 'Search skills' }).fill('docs/review');
    await page.getByRole('checkbox', { name: 'Select docs/review', exact: true }).check();
    await expect(page.getByRole('region', { name: 'Selection basket' })).toContainText('engineering/review');
    await expect(page.getByRole('region', { name: 'Selection basket' })).toContainText('docs/review');
    await page.getByRole('textbox', { name: 'Search skills' }).fill('');
    await page.locator('article.skill-card').filter({ hasText: 'engineering/review' }).locator('button.skill-description').click();
    await expect(page.getByRole('region', { name: 'Skill preview' })).toContainText('Review');
    expect(await page.evaluate(() => Boolean((window as unknown as { __skillExecuted?: boolean }).__skillExecuted))).toBe(false);
    await page.getByRole('checkbox', { name: 'Target OpenAI Codex', exact: true }).check();
    await page.getByRole('checkbox', { name: /^Target Claude/ }).check();
    await page.screenshot({ path: testInfo.outputPath('live-picker-desktop.png'), fullPage: true });
    await page.getByRole('button', { name: 'Review installation', exact: true }).first().click();
    await expect(page.getByRole('dialog', { name: 'Review installation' })).toBeVisible({ timeout: 30_000 });
    await page.route('**/api/v1/plans/*/apply', async (route) => {
      // Let the real handler accept/apply, but discard its response at the client.
      await route.fetch();
      await route.abort('failed');
    }, { times: 1 });
    await page.getByRole('button', { name: 'Confirm installation', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Installation complete' })).toBeVisible({ timeout: 60_000 });
    await expect(page.getByRole('dialog')).toHaveCount(0);
    expect(applyRequests).toBe(1);
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Installation complete' })).toBeVisible({ timeout: 30_000 });
    await expect(page.getByRole('region', { name: 'Installation progress' })).toContainText('codex');
    await expect(page.getByRole('region', { name: 'Installation progress' })).toContainText('claude');
    expect(external).toEqual([]);
    expect(errors).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath('live-picker-complete.png'), fullPage: true });
    verified = true;
  } finally {
    await writeFile(`${ready}.done`, verified ? 'verify' : 'abort', { mode: 0o600 });
    const killer = setTimeout(() => child.kill('SIGTERM'), 30_000);
    const code = await completion;
    clearTimeout(killer);
    if (verified) expect(code, output).toBe(0);
    await rm(scratch, { recursive: true, force: true });
  }
});
