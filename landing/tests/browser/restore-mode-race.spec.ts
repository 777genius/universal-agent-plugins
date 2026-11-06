import { expect, test } from '@playwright/test';
import { hydrated } from './i18n-state.helpers';

test('a delayed clipboard denial cannot copy stale Global commands after switching to Project', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('./');
  await hydrated(page);
  const section = page.locator('#restore-environment');
  await section.locator('[data-mode-option="global"]').click();
  await page.evaluate(() => {
    const state = window as typeof window & {
      rejectRestoreCopy?: () => void;
      restoreFallbackCalls?: number;
    };
    state.restoreFallbackCalls = 0;
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: () => new Promise<void>((_resolve, reject) => {
        state.rejectRestoreCopy = () => reject(new Error('Delayed denial'));
      }) },
    });
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: () => { state.restoreFallbackCalls! += 1; return true; },
    });
  });
  await section.locator('.registry-restore__screen .registry-restore__copy').first().click();
  await page.waitForFunction(() => typeof (window as typeof window & { rejectRestoreCopy?: unknown }).rejectRestoreCopy === 'function');
  const project = section.locator('[data-mode-option="project"]');
  await project.click();
  await page.evaluate(async () => {
    (window as typeof window & { rejectRestoreCopy: () => void }).rejectRestoreCopy();
    await new Promise<void>((resolve) => setTimeout(resolve, 0));
  });
  expect(await page.evaluate(() => (window as typeof window & { restoreFallbackCalls: number }).restoreFallbackCalls)).toBe(0);
  await expect(project).toBeFocused();
  await expect(section.locator('.registry-restore__copy-status')).toBeEmpty();
  await expect(section).toHaveAttribute('data-mode', 'project');
});

test('the previous feedback timer cannot suppress a slower second copy result', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('./');
  await hydrated(page);
  const section = page.locator('#restore-environment');
  await section.locator('[data-mode-option="global"]').click();
  await page.evaluate(() => {
    let calls = 0;
    const state = window as typeof window & { resolveSecondCopy?: () => void };
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: () => {
        calls += 1;
        return calls === 1 ? Promise.resolve() : new Promise<void>((resolve) => { state.resolveSecondCopy = resolve; });
      } },
    });
  });
  const buttons = section.locator('.registry-restore__screen .registry-restore__copy');
  await buttons.first().click();
  await expect(section.locator('.registry-restore__copy-status')).toHaveText('Copied: My computer');
  await buttons.last().click();
  await page.waitForFunction(() => typeof (window as typeof window & { resolveSecondCopy?: unknown }).resolveSecondCopy === 'function');
  // Deliberately exceed the first copy's 1600 ms feedback lifetime.
  await page.evaluate(async () => {
    await new Promise<void>((resolve) => setTimeout(resolve, 1800));
    (window as typeof window & { resolveSecondCopy: () => void }).resolveSecondCopy();
  });
  await expect(section.locator('.registry-restore__copy-status')).toHaveText("Copied: Teammate's computer");
  await expect(buttons.last()).toHaveText('Copied');
});

test('static Global example keeps released commands readable without JavaScript', async ({ browser, baseURL }) => {
  const context = await browser.newContext({ javaScriptEnabled: false, baseURL });
  try {
    const page = await context.newPage();
    await page.goto('./');
    const section = page.locator('#restore-environment');
    await expect(section).toHaveAttribute('data-mode', 'global');
    await expect(section.locator('.registry-restore__package').first()).toBeVisible();
    await expect(section.locator('.registry-restore__package').last()).toBeVisible();
    await expect(section.locator('.typed-command__visible').first()).toHaveText('npx -y universal-agent-plugins install marketing-kit');
    await expect(section.locator('.typed-command__visible').last()).toHaveText('npx -y universal-agent-plugins deps import --install');
    await expect(page.locator('.hero-demo-video iframe')).toHaveCount(0);
  } finally {
    await context.close();
  }
});
