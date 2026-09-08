import { expect, test } from '@playwright/test';
import {
  automatic, downloadHeading, hydrated, deferred, delayedMirror, exactCommands, locales, navigate, offline,
  projections, registryFixture, targets, wording,
} from './i18n-state.helpers';

// Real parent v-model wiring, installed Nuxt router, public DOM; no store access.
// The publication manifest activates reviewed RU/UK without copying dictionaries.
for (const locale of ['ru', 'uk'] as const) {
  test(`${locale}: detail parent/child retains manual and automatic commands on remount`, async ({ page }) => {
    test.skip(!locales.includes(locale), 'Requires published locale in final artifact');
    const errors = await offline(page);
    await projections(page, registryFixture());
    await navigate(page, '/plugins/i18n-state-00/');
    const panel = page.locator('.install-panel');
    await targets(page, panel, ['Codex', 'Cursor']);
    await exactCommands(page, panel, 'i18n-state-00', 'codex,cursor');
    await navigate(page, '/plugins/i18n-state-00/', locale);
    await expect(panel.getByRole('heading', { name: wording(locale)('registryUi.install.useWithYourAgent'), exact: true })).toBeVisible();
    await exactCommands(page, panel, 'i18n-state-00', 'codex,cursor');
    await panel.locator('.app-multiselect__trigger').click();
    for (const name of ['Codex', 'Cursor']) await expect(page.getByRole('checkbox', { name: new RegExp(`^${name}\\s`) })).toBeChecked();
    await page.keyboard.press('Escape');
    await automatic(page, panel, locale);
    await exactCommands(page, panel, 'i18n-state-00');
    await navigate(page, '/plugins/i18n-state-00/');
    await expect(panel).toContainText(wording('en')('registryUi.install.automaticDetection'));
    await exactCommands(page, panel, 'i18n-state-00');
    expect(errors).toEqual([]);
  });

  test(`${locale}: catalog expansion survives remount and resets on changed initial query`, async ({ page }) => {
    test.skip(!locales.includes(locale), 'Requires published locale in final artifact');
    const errors = await offline(page);
    await projections(page, registryFixture());
    const suffix = '?q=state&category=state-large&campaign=keep#catalog';
    await navigate(page, '/plugins/', 'en', suffix);
    const cards = page.locator('.plugin-grid > .plugin-card');
    await expect(cards).toHaveCount(48);
    await page.locator('.catalog-more').getByRole('button', { name: wording('en')('registryUi.catalog.showMore', { count: 6 }) }).click();
    await expect(cards).toHaveCount(54);
    await navigate(page, '/plugins/', locale, suffix);
    const t = wording(locale);
    const search = page.getByRole('searchbox', { name: t('registryUi.catalog.searchPlugins') });
    await expect(search).toHaveValue('state');
    await expect(page.locator('.catalog-active-filters')).toContainText(`${t('registryUi.catalog.category')}: state-large`);
    await expect(cards).toHaveCount(54);
    expect(new URL(page.url()).hash).toBe('#catalog');
    await navigate(page, '/plugins/', locale, '?q=state&category=state-small&campaign=keep#catalog');
    await expect(cards).toHaveCount(6);
    await expect(page.locator('.catalog-active-filters')).toContainText('state-small');
    await navigate(page, '/plugins/', locale, suffix);
    await expect(cards).toHaveCount(48);
    await expect(page.locator('.catalog-more').getByRole('button')).toContainText(t('registryUi.catalog.showMore', { count: 6 }));
    await search.fill('absent-unique-xyz');
    await expect(page.getByRole('heading', { name: t('registryUi.catalog.noMatchingPlugins'), exact: true })).toBeVisible();
    await page.getByRole('button', { name: t('registryUi.catalog.resetFilters'), exact: true }).click();
    await expect(cards).toHaveCount(48);
    await expect.poll(() => new URL(page.url()).searchParams.toString()).toBe('campaign=keep');
    expect(errors).toEqual([]);
  });
}

for (const locale of locales) {
  test(`${locale}: semantic auth hides noauth/auto and changes with selected targets`, async ({ page }) => {
    const errors = await offline(page);
    await projections(page, registryFixture());
    await navigate(page, '/plugins/', locale, '?q=i18n-state-00');
    const t = wording(locale);
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
    await card.getByRole('button', { name: t('registryUi.card.installName', { name: 'I18n State 00' }), exact: true }).click();
    await expect(card.locator('.plugin-card__auth')).toHaveCount(0);
    await targets(page, card, ['Codex']);
    await expect(card.locator('.command-snippet code')).toContainText('--target codex');
    await expect(card.locator('.plugin-card__auth')).toHaveCount(0);
    await targets(page, card, ['Cursor']);
    await expect(card.locator('.plugin-card__auth')).toHaveText(t('registryUi.authentication.varies'));
    await targets(page, card, ['Codex']);
    await expect(card.locator('.plugin-card__auth')).toHaveText(t('registryUi.authentication.required'));
    await expect(card.locator('.command-snippet code')).toContainText('--target cursor');
    await card.locator('.app-multiselect__trigger').click();
    const auto = page.locator('.app-multiselect__auto');
    await expect(auto).toContainText(t('registryUi.card.allInstalledAgentsRecommended'));
    await auto.click();
    await page.keyboard.press('Escape');
    await expect(card.locator('.plugin-card__auth')).toHaveCount(0);
    await card.locator('.command-snippet').getByRole('button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe('npx universal-agent-plugins add i18n-state-00');
    expect(errors).toEqual([]);
  });
}

for (const [theme, width] of [['light', 280], ['dark', 800]] as const) {
  test(`${theme} ${width}px reduced motion: translated install controls are usable`, async ({ page }) => {
    const locale = locales.at(-1)!;
    const t = wording(locale);
    const errors = await offline(page);
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: 'reduce', colorScheme: theme });
    await page.addInitScript((value) => localStorage.setItem('theme', value), theme);
    await projections(page, registryFixture());
    await navigate(page, '/plugins/i18n-state-00/', locale);
    await expect(page.locator('.v-application')).toHaveClass(new RegExp(`(?:^| )v-theme--${theme}(?: |$)`));
    const panel = page.locator('.install-panel');
    await expect(panel.getByRole('heading', { name: t('registryUi.install.useWithYourAgent'), exact: true })).toBeVisible();
    const trigger = panel.locator('.app-multiselect__trigger');
    await trigger.scrollIntoViewIfNeeded();
    const box = await trigger.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width + 1);
    await trigger.focus();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('group', { name: t('registryUi.install.chooseTargetAgents'), exact: true })).toBeVisible();
    await page.getByRole('checkbox', { name: /^Codex\s/ }).click();
    await page.keyboard.press('Escape');
    await expect(trigger).toBeFocused();
    await exactCommands(page, panel, 'i18n-state-00', 'codex');
    expect(errors).toEqual([]);
  });
}

// Bowser is lazy onMounted. Gate scripts requested after controls render, without
// guessing a hashed filename or substituting module code. Fail if the seam changes.
test('manual download channel beats delayed detection and locale remount', async ({ page }) => {
  const errors = await offline(page);
  await page.addInitScript(() => Object.defineProperty(navigator, 'userAgent', {
    configurable: true, value: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/130.0.0.0 Safari/537.36',
  }));
  await page.goto('./create-plugin/');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(wording('en')('publicAuthoring.title'));
  const gate = deferred();
  let held = 0;
  await page.route('**/_nuxt/*.js', async (route) => {
    if (await page.locator('.download-section__install-tab').count()) {
      held++;
      await gate.promise;
    }
    await route.continue();
  });
  try {
    await navigate(page, '/download/');
    await expect.poll(() => held, { message: 'Expected deferred detection script' }).toBeGreaterThan(0);
    await expect(page.locator('.download-section__platform-note')).toHaveCount(0);
    const npm = page.locator('.download-section__install-tab').filter({ hasText: 'npx' });
    await npm.click();
    await expect(npm).toHaveAttribute('aria-pressed', 'true');
    const commands = await page.locator('.download-section__steps code').allTextContents();
    expect(commands.length).toBeGreaterThan(0);
    expect(commands.join('\n')).toContain('npx universal-agent-plugins add');
    gate.release();
    await expect(page.locator('.download-section__platform-note')).toContainText(wording('en')('download.platforms.linux'));
    await expect(npm).toHaveAttribute('aria-pressed', 'true');
    for (const locale of locales.filter((value) => value !== 'en')) {
      await navigate(page, '/download/', locale);
      await expect(page.getByRole('heading', { name: downloadHeading(locale), exact: true })).toBeVisible();
      await expect(npm).toHaveAttribute('aria-pressed', 'true');
      await expect.poll(() => page.locator('.download-section__steps code').allTextContents()).toEqual(commands);
    }
    expect(errors).toEqual([]);
  } finally { gate.release(); }
});

// Mirror gates retain signed bytes and the production verifier, never live GitHub.
// Releasing latest.json starts signed snapshot downloads and verification; wait for
// the resulting public state, rather than treating pointer delivery as completion.
for (const outcome of ['success', 'error', 'security'] as const) {
  for (const itinerary of ['locale', 'aba', 'download'] as const) {
    test(`delayed ${outcome}: ${itinerary} preserves live page state`, async ({ page }) => {
      test.skip(itinerary === 'locale' && locales.length < 2, 'Requires published locale remount');
      const locale = itinerary === 'locale' ? locales[1]! : 'en';
      const t = wording(locale);
      const errors = await offline(page);
      const discovery = await delayedMirror(page, 'discovery', outcome === 'error');
      const security = await delayedMirror(page, 'security');
      try {
        // A fresh Playwright context and a non-registry bootstrap leave module promises
        // and the discovery cache cold; mount the catalog only after hydration.
        await page.goto('./download/');
        await hydrated(page);
        await page.evaluate(async () => {
          for (const key of await caches.keys()) await caches.delete(key);
        });
        await navigate(page, '/plugins/', 'en', outcome === 'security' ? '?q=hindsight' : '?q=context7');
        await Promise.all([discovery.seen, security.seen]);
        await expect(page.locator('.discovery-status')).toHaveText(wording('en')('registryUi.catalog.findingMoreCommunityPluginsOnGithub'));
        if (outcome === 'security') {
          discovery.release();
          await expect(page.locator('.catalog')).toHaveAttribute('data-discovery-state', 'current', { timeout: 15_000 });
          const pending = page.locator('.plugin-card[data-install-source="discovery:vectorize-io/hindsight//hindsight-integrations/agent-plugin"]');
          await expect(pending).toBeVisible();
          await expect(pending.locator('.plugin-card__security')).toHaveCount(0);
        }
        if (itinerary === 'aba') {
          await navigate(page, '/agents/github-copilot-cli/');
          await expect(page.locator('h1')).toHaveText(wording('en')('registryUi.agentPage.installFor', { name: 'GitHub Copilot CLI' }));
          await navigate(page, '/plugins/', 'en', '?q=gitlab&client=cursor');
        } else if (itinerary === 'locale') {
          await navigate(page, '/plugins/', locale, '?q=gitlab&client=cursor');
        } else {
          await navigate(page, '/download/');
          await expect(page.getByRole('heading', { name: downloadHeading(locale), exact: true })).toBeVisible();
          await page.locator('.download-section__install-tab').filter({ hasText: 'npx' }).click();
        }
        discovery.release();
        security.release();
        await Promise.all([discovery.delivered, security.delivered]);
        if (itinerary === 'download') {
          // No registry status is exposed on download; observe its public boundary
          // and return to a registry consumer to await the shared feed outcome.
          await expect(page.locator('.catalog, .plugin-card, .install-panel')).toHaveCount(0);
          await expect(page.locator('.download-section__install-tab[aria-pressed="true"]')).toContainText('npx');
          await navigate(page, '/plugins/', 'en', '?q=gitlab&client=cursor');
        }
        await expect(page.locator('.catalog')).toHaveAttribute('data-discovery-state', outcome === 'error' ? 'unavailable' : 'current', { timeout: 15_000 });
        await expect(page.getByRole('searchbox', { name: t('registryUi.catalog.searchPlugins') })).toHaveValue('gitlab');
        await expect(page.locator('.catalog-active-filters')).toContainText(`${t('registryUi.catalog.agent')}: Cursor`);
        await expect(page.locator('.plugin-card[data-install-source="gitlab"]').getByRole('heading', { name: 'GitLab', exact: true })).toBeVisible();
        await expect(page.locator('.plugin-card[data-install-source="context7"]')).toHaveCount(0);
        if (outcome === 'error') await expect(page.locator('.discovery-status')).toHaveText(t('registryUi.catalog.communityResultsAreTemporarilyUnavailableReviewedListingsRemainAvailable'));
        if (outcome !== 'error') {
          await page.getByRole('button', { name: t('registryUi.catalog.resetFilters'), exact: true }).click();
          await page.getByRole('searchbox', { name: t('registryUi.catalog.searchPlugins') }).fill('hindsight');
          await page.getByRole('searchbox', { name: t('registryUi.catalog.searchPlugins') }).blur();
          const community = page.locator('.plugin-card[data-install-source="discovery:vectorize-io/hindsight//hindsight-integrations/agent-plugin"]');
          await expect(community).toBeVisible();
          const badge = community.locator('.plugin-card__security');
          await expect(badge).toBeVisible();
          await badge.hover();
          await expect(page.locator('.app-tooltip__disclaimer')).toHaveText(t('registryUi.security.tooltipDisclaimer'));
        }
        expect(errors).toEqual([]);
      } finally { discovery.release(); security.release(); }
    });
  }
}
