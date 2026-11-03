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
