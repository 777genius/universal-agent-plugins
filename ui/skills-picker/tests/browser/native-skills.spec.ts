import { test, expect } from '@playwright/test';
import { spawn } from 'node:child_process';
import { readFile, writeFile, rm, lstat } from 'node:fs/promises';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import type { Operation, Plan } from '../../src/shared/api/types';

// This single hosted-native scenario uses the public CLI and real clients.
// The regular live-bridge test remains a separate deterministic SDK fixture.
test.use({ trace: 'off' }); // Never archive the private capability URL or headers.
test('public browser installs selected raw Skills into real Codex and Claude', async ({ page }, testInfo) => {
  test.skip(process.env.AGENTPLUGINS_NATIVE_BROWSER_LIVE !== '1', 'Run in the opted-in Linux Codex native lane');
  test.setTimeout(300_000);
  page.setDefaultTimeout(20_000);
  const required = (name: string) => {
    const value = process.env[name];
    if (!value) throw new Error('Missing native browser fixture field: ' + name);
    return value;
  };
  const root = required('AGENTPLUGINS_NATIVE_BROWSER_ROOT');
  const scenario = process.env.AGENTPLUGINS_NATIVE_BROWSER_SCENARIO ?? 'user-benchmark';
  expect(['user-benchmark', 'project-anthropic']).toContain(scenario);
  const projectScenario = scenario === 'project-anthropic';
  const selectedSkills = projectScenario ? [
    { name: 'docx', path: 'skills/docx' }, { name: 'frontend-design', path: 'skills/frontend-design' },
  ] : [{ name: 'benchmark-note', path: 'tests/fixtures/uap_apm/skills/benchmark-note' }];
  const privateRoot = join(root, 'browser-private');
  const ready = join(privateRoot, 'ready.json');
  const done = join(privateRoot, 'done');
  const child = spawn(required('AGENTPLUGINS_NATIVE_BROWSER_PYTHON'), [required('AGENTPLUGINS_NATIVE_BROWSER_DRIVER')], {
    env: process.env, stdio: ['ignore', 'pipe', 'pipe'],
  });
  let output = '', closed = false, verified = false, startupError: Error | undefined;
  child.stdout.on('data', (chunk: Buffer) => { output = (output + chunk.toString()).slice(-65536); });
  child.stderr.on('data', (chunk: Buffer) => { output = (output + chunk.toString()).slice(-65536); });
  const completion = new Promise<number | null>((resolve) => {
    child.once('error', (error) => { startupError = error; closed = true; resolve(null); });
    child.once('close', (code) => { closed = true; resolve(code); });
  });
  let releasePrepare = () => {};
  try {
    let url = '';
    const deadline = Date.now() + 150_000;
    while (!url && Date.now() < deadline) {
      if (closed) throw new Error('Public CLI exited before its picker was ready: ' + (startupError?.message ?? '') + '\n' + output);
      try { url = (JSON.parse(await readFile(ready, 'utf8')) as { url: string }).url; }
      catch (error) { if (!(error instanceof SyntaxError) && (error as NodeJS.ErrnoException).code !== 'ENOENT') throw error; }
      if (!url) await delay(100);
    }
    if (!url) throw new Error('Public CLI did not publish its private picker URL:\n' + output);
    const origin = new URL(url).origin;
    const external: string[] = [], errors: string[] = [];
    let applyRequests = 0;
    let operation: Operation | undefined;
    page.on('request', (request) => {
      if (/^https?:/.test(request.url()) && new URL(request.url()).origin !== origin) external.push(request.url());
      if (request.method() === 'POST' && /\/api\/v1\/plans\/[^/]+\/apply$/.test(request.url())) applyRequests++;
    });
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('response', (response) => {
      if (response.ok() && /\/api\/v1\/operations\/[^/]+$/.test(new URL(response.url()).pathname)) {
        void response.json().then((body: Operation) => { operation = body; }).catch(() => errors.push('Could not decode the real operation response'));
      }
    });
    try { await page.goto(url, { timeout: 30_000 }); }
    catch { throw new Error('Could not open the private public-CLI picker URL'); }
    for (const skill of selectedSkills) {
      await page.getByRole('checkbox', { name: 'Select ' + skill.path, exact: true }).check({ timeout: 60_000 });
    }
    expect(new URL(page.url()).hash).toBe('');
    await page.getByRole('checkbox', { name: 'Target OpenAI Codex', exact: true }).check();
    await page.getByRole('checkbox', { name: /^Target Claude/ }).check();
    for (const skill of selectedSkills) {
      await expect(page.getByRole('region', { name: 'Selection basket' })).toContainText(skill.name);
    }
    await page.screenshot({ path: testInfo.outputPath('native-picker-selection.png'), fullPage: true });

    // Delay only delivery of the actual prepared response. Real Git, scanner,
    // planners and native probes still execute; no API body is fabricated.
    const held = new Promise<void>((resolve) => { releasePrepare = resolve; });
    let prepared = false;
    let preparedPlan: Plan | undefined;
    await page.route('**/api/v1/plans', async (route) => {
      const response = await route.fetch();
      preparedPlan = await response.json() as Plan;
      prepared = true;
      await held;
      await route.fulfill({ response });
    }, { times: 1 });
    const installBar = page.locator('.bottom-bar');
    await installBar.getByRole('button', { name: 'Review installation', exact: true }).click();
    await expect(installBar.getByText('Checking installation…', { exact: true })).toBeVisible();
    await expect.poll(() => prepared, { timeout: 90_000 }).toBe(true);
    await page.screenshot({ path: testInfo.outputPath('native-picker-checking.png'), fullPage: true });
    releasePrepare();
    const review = page.getByRole('dialog', { name: 'Review installation' });
    await expect(review).toBeVisible();
    await expect(review.locator('.plan-action')).toHaveCount(selectedSkills.length * 2);
    await expect(review).toContainText('codex');
    await expect(review).toContainText('claude');
    expect(preparedPlan!.actions.map((action) => [action.alias, action.targetId, action.action]).sort()).toEqual(
      selectedSkills.flatMap((skill) => ['claude', 'codex'].map((target) => [skill.name, target, 'install'])).sort(),
    );
    if (projectScenario) {
      const project = required('AGENTPLUGINS_NATIVE_BROWSER_PROJECT');
      await expect(review).toContainText('Installation may trust the selected project "' + project + '" in Claude Code.');
      await expect(review).toContainText('This preference remains after removal.');
      const baseline = JSON.parse(await readFile(join(root, 'native-project-baseline.json'), 'utf8')) as {
        other_project: string; foreign_trust: unknown; codex_config: string;
      };
      const profile = JSON.parse(await readFile(join(required('CLAUDE_CONFIG_DIR'), '.claude.json'), 'utf8')) as {
        projects: Record<string, { hasTrustDialogAccepted?: boolean }>;
      };
      expect(profile.projects[project]?.hasTrustDialogAccepted).not.toBe(true);
      expect(profile.projects[baseline.other_project]).toEqual(baseline.foreign_trust);
      expect(JSON.parse(await readFile(join(project, '.claude', 'settings.json'), 'utf8'))).toEqual({
        enabledPlugins: { 'foreign-project-fixture@skills-dir': false },
      });
      expect(await readFile(join(required('CODEX_HOME'), 'config.toml'), 'utf8')).toBe(baseline.codex_config);
      await expect(lstat(join(project, '.uap', 'state-v2.json'))).rejects.toMatchObject({ code: 'ENOENT' });
      expect(preparedPlan!.actions).toHaveLength(4);
      for (const action of preparedPlan!.actions) {
        expect(action.destinationPath.startsWith(project + '/')).toBe(true);
        await expect(lstat(action.destinationPath)).rejects.toMatchObject({ code: 'ENOENT' });
      }
      for (const skill of selectedSkills) {
        await expect(lstat(join(project, '.agents', 'skills', skill.name))).rejects.toMatchObject({ code: 'ENOENT' });
      }
      expect(applyRequests).toBe(0);
    }
    await page.screenshot({ path: testInfo.outputPath('native-picker-review.png'), fullPage: true });
    await review.getByRole('button', { name: 'Confirm installation', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Installation applied' })).toBeVisible({ timeout: 120_000 });
    await expect(review).toHaveCount(0);
    const progress = page.getByRole('region', { name: 'Installation progress' });
    await expect(progress.locator('.operation-item')).toHaveCount(selectedSkills.length * 2);
    await expect(progress).toContainText('codex · files ready');
    await expect(progress).toContainText('claude · files ready');
    await expect.poll(() => operation?.state).toBe('succeeded');
    expect(operation!.items.map((item) => [item.alias, item.targetId, item.status]).sort()).toEqual(
      selectedSkills.flatMap((skill) => ['claude', 'codex'].map((target) => [skill.name, target, 'succeeded'])).sort(),
    );
    expect(operation!.diagnostics.some((item) => item.code === 'NATIVE_SETUP_REQUIRED')).toBe(false);
    expect(applyRequests).toBe(1);
    expect(external).toEqual([]);
    expect(errors).toEqual([]);
    const readiness = page.getByRole('status', { name: 'Skills readiness' });
    await expect(readiness).toContainText('Files:');
    await expect(readiness).toContainText('Skills installation verified.');
    await expect(readiness).not.toContainText('not verified');
    await page.screenshot({ path: testInfo.outputPath('native-picker-complete.png'), fullPage: true });
    await writeFile(join(root, 'native-browser-installation.json'), JSON.stringify({
      schema_version: 1,
      source: required('AGENTPLUGINS_NATIVE_BROWSER_SKILL_SOURCE'),
      surface: 'Chromium through the public CLI --ui',
      scope: projectScenario ? 'project' : 'user',
      selected_skills: selectedSkills.map((skill) => skill.name), selected_targets: ['codex', 'claude'],
      apply_requests: applyRequests, operation,
      native_inventory: 'The same required Go dual-client lifecycle verifies fresh native inventories after this browser exits.',
    }, null, 2) + '\n', { mode: 0o600 });
    verified = true;
  } finally {
    releasePrepare();
    await writeFile(done, verified ? 'verify' : 'abort', { mode: 0o600 }).catch((error: NodeJS.ErrnoException) => {
      if (error.code !== 'ENOENT' || verified) throw error;
    });
    let forced = false;
    const terminate = setTimeout(() => { forced = true; child.kill('SIGTERM'); }, 30_000);
    const kill = setTimeout(() => { forced = true; child.kill('SIGKILL'); }, 35_000);
    const code = await completion;
    clearTimeout(terminate); clearTimeout(kill);
    await Promise.all([rm(ready, { force: true }), rm(join(privateRoot, 'url'), { force: true })]);
    if (verified) {
      expect(forced, 'The owned public CLI must exit gracefully after its browser session').toBe(false);
      expect(code, output).toBe(0);
    }
  }
});
