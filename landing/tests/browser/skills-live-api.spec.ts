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

  await page.goto('plugins/?tab=skills&skillScan=all');
  const cards = page.locator('.skills-catalog .skill-card');
  await expect(cards.first()).toBeVisible({ timeout: 30000 });
  assert.ok(livePage && livePage.items.length > 0, 'The live Skills API returned no items');
  // All valid API Skills are visible; only command-eligible ones get Install.
  const sample = livePage.items;
  const eligible = sample.filter(item => Boolean(skillInstallCommand(item)));
  assert.ok(eligible.length > 0, 'No command-eligible Skills in live catalog');
  const initial = Math.min(sample.length, 48);
  await expect(cards).toHaveCount(initial);

  // This browser test verifies real API data and UI filtering; the separate
  // native and published npm jobs prove actual installation on real sources.
  for (let count = initial; count < sample.length; count = Math.min(count + 48, sample.length)) {
    await page.getByRole('button', { name: /Show more Skills/ }).last().click();
    await expect(cards).toHaveCount(Math.min(count + 48, sample.length));
  }

  const renderedIds = await cards.evaluateAll(nodes =>
    nodes.map(node => node.getAttribute('data-skill-id')));
  expect(renderedIds).toEqual(sample.map(item => item.id));
  const unavailable = sample.filter(item => !skillInstallCommand(item));
  let legacyRecovered = 0;
  for (const [index, item] of sample.entries()) {
    const command = skillInstallCommand(item);
    const button = cards.nth(index).getByRole('button', { name: `Install ${item.title}`, exact: true });
    if (!command) {
      await expect(button, `No unsafe Install for ${item.id}`).toHaveCount(0);
      await expect(cards.nth(index).locator('.plugin-card__unavailable')).toBeVisible();
    } else {
      if (item.availability === 'unsupported-source') legacyRecovered++;
      await expect(button).toHaveCount(1);
    }
  }
  const firstInstall = sample.findIndex(item => Boolean(skillInstallCommand(item)));
  await cards.nth(firstInstall).getByRole('button', {
    name: `Install ${sample[firstInstall]!.title}`, exact: true,
  }).click();
  await expect(cards.nth(firstInstall).locator('.command-snippet code')).toHaveText(
    skillInstallCommand(sample[firstInstall]!)!,
  );
  console.log(JSON.stringify({
    live: true, checked: sample.length, commandEligible: eligible.length,
    legacyRecovered, visibleWithoutInstall: unavailable.length,
    physicalInstallationProvenHere: false,
  }));
});
