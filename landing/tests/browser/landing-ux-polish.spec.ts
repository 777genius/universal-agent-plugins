import { expect, test } from '@playwright/test';
import { clientSupportRows } from '../../data/clientSupport.ts';

for (const width of [1440, 390]) {
  test(`agent names and logos align to the start edge at ${width}px`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('./');
    const table = page.locator('#agent-support table');
    const header = table.locator('thead th:first-child');
    const icons = table.locator('tbody th:first-child img');
    await expect(icons).toHaveCount(clientSupportRows.length);
    // An offscreen scroll-reveal section has zero opacity even though its
    // table cells have geometry. Wait for the *visible* table before capturing
    // review screenshots, otherwise the proof is an almost-black image.
    const section = page.locator('#agent-support');
    await section.scrollIntoViewIfNeeded();
    await expect(section).toHaveClass(/scroll-reveal--visible/);
    await expect(section).toHaveCSS('opacity', '1');
    const expectedX = await header.evaluate((node) =>
      node.getBoundingClientRect().left + parseFloat(getComputedStyle(node).paddingLeft),
    );
    const positions = await icons.evaluateAll((nodes) => nodes.map((node) =>
      node.getBoundingClientRect().left));
    for (const left of positions) {
      expect(Math.abs(left - expectedX)).toBeLessThan(2);
    }
    await expect(table.locator('tbody th:first-child').first()).toHaveCSS('text-align', 'start');
    await testInfo.attach(`support-table-${width}`, {
      body: await page.locator('#agent-support .registry-client-support__scroll')
        .screenshot({ animations: 'disabled' }),
      contentType: 'image/png',
    });
  });
}

test('no-Skills and scanned SkillSpector rows use the same vertical rhythm', async ({
  page,
}) => {
  await page.goto('./');
  const noSkills = page.locator('[data-install-source="github"] .skillspector--noSkills');
  const scanned = page.locator('[data-install-source="context7"] .skillspector > summary');
  await expect(noSkills).toBeVisible();
  await expect(scanned).toBeVisible();
  const rowHeight = await noSkills.evaluate((node) => node.getBoundingClientRect().height);
  const summaryHeight = await scanned.evaluate((node) => node.getBoundingClientRect().height);
  expect(rowHeight).toBeGreaterThanOrEqual(32);
  expect(Math.abs(rowHeight - summaryHeight)).toBeLessThan(2);
  await expect(noSkills).toContainText('No Skills to check');
  await expect(scanned).not.toContainText('No Skills to check');
});
