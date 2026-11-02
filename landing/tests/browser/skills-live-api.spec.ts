import assert from 'node:assert/strict';
import { expect, test } from '@playwright/test';
import type { SkillsCatalogPage } from '../../types/skillsCatalog.ts';
import { skillInstallCommand } from '../../utils/skillsCatalog.ts';

// Opt-in integration: staging Pages + actual public API bytes. Staging uses a
// narrow CORS bridge because the public API permits the production origin, not
// localhost. We do not synthesize/modify any API JSON or convert failures to 200.
test('staged Skills interface renders every live catalog install command truthfully', async ({ page }) => {
  test.skip(process.env.UAP_E2E_LIVE_SKILLS !== '1', 'Opt-in real public API E2E');
  let livePage: SkillsCatalogPage | undefined;
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', async (route) => {
    const url = new URL(route.request().url());
    // The staged localhost page is not an allowed production CORS origin.
    // Ask the real API as the production origin, then expose the unchanged
    // status/body to localhost. The separate API acceptance checks true CORS.
    const response = await route.fetch({
      timeout: 20000,
      maxRetries: 1,
      headers: {
        ...route.request().headers(),
        origin: 'https://universal-agent-plugins.com',
      },
    });
    if (
      url.pathname === '/v1/skills' &&
      url.searchParams.get('view') === 'popular' &&
      url.searchParams.get('page') === '0' &&
      response.ok()
    ) {
      livePage = (await response.json()) as SkillsCatalogPage;
    }
    await route.fulfill({
      response,
      headers: { ...response.headers(), 'access-control-allow-origin': '*' },
    });
  });

  await page.goto('plugins/?tab=skills');
  const cards = page.locator('.skills-catalog .skill-card');
  await expect(cards.first()).toBeVisible({ timeout: 30000 });
  assert.ok(livePage && livePage.items.length > 0, 'The live Skills API returned no items');
  const sample = livePage.items;
  const initial = Math.min(sample.length, 48);
  await expect(cards).toHaveCount(initial);

  // Show every record on the first real API page, not only the first 48 cards.
  for (let count = initial; count < sample.length; count = Math.min(count + 48, sample.length)) {
    await page.getByRole('button', { name: /Show more Skills/ }).last().click();
    await expect(cards).toHaveCount(Math.min(count + 48, sample.length));
  }

  let installable = 0;
  let legacyRecovered = 0;
  let firstInstallable = -1;
  for (const [index, item] of sample.entries()) {
    const command = skillInstallCommand(item);
    const button = cards.nth(index).getByRole('button', { name: `Install ${item.title}`, exact: true });
    if (command) {
      installable++;
      if (firstInstallable === -1) firstInstallable = index;
      if (item.availability === 'unsupported-source') legacyRecovered++;
      await expect(button, `Missing install for real listing ${item.id}`).toHaveCount(1);
    } else {
      await expect(button, `Unsafe install exposed for real listing ${item.id}`).toHaveCount(0);
    }
  }
  assert.ok(installable > 0, 'No install commands visible for live Skills catalog');
  assert.ok(firstInstallable >= 0);
  await cards.nth(firstInstallable).getByRole('button', {
    name: `Install ${sample[firstInstallable]!.title}`,
    exact: true,
  }).click();
  await expect(cards.nth(firstInstallable).locator('.command-snippet code')).toHaveText(
    skillInstallCommand(sample[firstInstallable]!)!,
  );
  console.log(JSON.stringify({
    live: true, checked: sample.length, installable, legacyRecovered,
    unavailable: sample.length - installable,
  }));
});
