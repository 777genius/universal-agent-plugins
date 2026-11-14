import { expect, test } from '@playwright/test';
import { navigate, offline, projections, registryFixture } from './i18n-state.helpers';

function context7Fixture() {
  const index = registryFixture();
  const plugin = index.plugins[0]!;
  index.plugins = [plugin];
  plugin.name = plugin.install_source = 'context7';
  plugin.display_name = 'Context7';
  plugin.trust_state = 'reviewed';
  const upstream = plugin.distributions[0]!;
  upstream.id = plugin.default_distribution = plugin.declared_default_distribution = 'upstash/context7';
  upstream.kind = 'upstream';
  upstream.compatible_clients = ['codex'];
  for (const release of upstream.releases) {
    release.materialized_clients = ['codex'];
    release.source = { ...release.source, repository: 'upstash/context7', path: 'plugins/agent-plugins/context7' };
  }
  return index;
}

for (const width of [1440, 390]) {
  test(`Context7 guided Web setup remains consistent across hero, card and detail at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 950 });
    const errors = await offline(page);
    await projections(page, context7Fixture());
    await navigate(page, '/');
    await page.locator('.hero__demo .app-multiselect__trigger').click();
    await page.getByRole('checkbox', { name: /^ChatGPT Web/ }).click();
    await page.keyboard.press('Escape');
    await expect(page.locator('.hero__demo .command-snippet code')).toContainText('add context7 --target chatgptweb');
    await testInfo.attach(`guided-web-hero-${width}.png`, { body: await page.locator('.hero__demo').screenshot({ animations: 'disabled' }), contentType: 'image/png' });

    const card = page.locator('[data-install-source="context7"]').first();
    await card.locator('.plugin-card__install-toggle').click();
    await card.locator('.app-multiselect__trigger').click();
    const web = page.getByRole('checkbox', { name: /^ChatGPT Web/ });
    await expect(web).toBeEnabled();
    await expect(web).toContainText('Guided ChatGPT Web setup');
    await web.click();
    await page.keyboard.press('Escape');
    await expect(card.locator('.command-snippet code')).toContainText('add context7 --target chatgptweb');
    await expect(card).toContainText('Guided ChatGPT Web setup');
    await testInfo.attach(`guided-web-card-${width}.png`, { body: await card.screenshot({ animations: 'disabled' }), contentType: 'image/png' });

    await navigate(page, '/plugins/context7/');
    await expect(page.locator('.install-panel .command-snippet--add code')).toContainText('add context7 --target chatgptweb');
    await expect(page.locator('.install-panel')).toContainText('Guided ChatGPT Web setup');
    await expect(page.locator('.install-panel .command-snippet')).toHaveCount(1);
    await expect(page.locator('.install-panel')).not.toContainText('ChatGPT preparation does not save a dependency.');
    await expect(page.locator('.install-panel')).not.toContainText('No single source serves this target set');
    await testInfo.attach(`guided-web-detail-${width}.png`, { body: await page.locator('.install-panel').screenshot({ animations: 'disabled' }), contentType: 'image/png' });
    await page.locator('.install-panel .app-multiselect__trigger').click();
    await page.getByRole('checkbox', { name: /^Codex CLI/ }).click();
    await page.keyboard.press('Escape');
    await expect(page.locator('.install-panel .command-snippet')).toHaveCount(4);
    await expect(page.locator('.install-panel .command-snippet--add code'))
      .toHaveText(/add context7 --target (chatgptweb,codex|codex,chatgptweb) --no-save$/);
    for (const action of ['update', 'repair', 'remove'])
      await expect(page.locator(`.install-panel .command-snippet--${action} code`))
        .toHaveText(`npx -y universal-agent-plugins ${action} context7 --target codex`);
    expect(errors).toEqual([]);
  });
}

test('a different source does not inherit Context7 guided Web setup', async ({ page }) => {
  const errors = await offline(page);
  const index = context7Fixture();
  index.plugins[0]!.distributions[0]!.releases[0]!.source.repository = 'other/context7';
  await projections(page, index);
  await navigate(page, '/plugins/context7/');
  await page.locator('.install-panel .app-multiselect__trigger').click();
  await expect(page.getByRole('checkbox', { name: /^ChatGPT Web/ })).toBeDisabled();
  expect(errors).toEqual([]);
});
