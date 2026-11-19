import { expect, test, type Locator } from '@playwright/test';
import { navigate, offline, projections, registryFixture, wording } from './i18n-state.helpers';

function localSourceFixture() {
  const index = registryFixture();
  const plugin = index.plugins[0]!;
  index.plugins = [plugin];
  plugin.trust_state = 'reviewed';
  const distribution = plugin.distributions[0]!;
  const release = distribution.releases[0]!;
  release.targets = [
    { client: 'codex', scopes: ['user', 'project'], delivery: 'managed', authentication: 'not_required' },
    { client: 'cursor', scopes: ['user', 'project'], delivery: 'managed', authentication: 'not_required' },
  ];
  release.blocking_clients = [];
  release.release_status = 'active';
  release.meets_minimum_capabilities = true;
  distribution.releases = [release];
  distribution.targets = release.targets;
  distribution.compatible_clients = ['codex', 'cursor'];
  distribution.status = 'active';
  distribution.selectable = true;
  return index;
}

const names = ['Codex CLI', 'ChatGPT Desktop', 'Cursor', 'Hermes Agent', 'Google Antigravity'];
async function checkChoices(panel: Locator) {
  for (const name of names)
    await expect(panel.getByRole('checkbox', { name: new RegExp(`^${name}(?:\\s|$)`) })).toBeChecked();
}

for (const width of [1440, 390]) {
  test(`local multiselect keeps every surface and one valid common-source command at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 950 });
    const errors = await offline(page);
    const index = localSourceFixture();
    await projections(page, index);
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
    await card.locator('.plugin-card__install-toggle').click();
    const dialog = page.getByRole('dialog');
    for (const name of names) {
      const choice = page.getByRole('checkbox', { name: new RegExp(`^${name}(?:\\s|$)`) });
      await choice.click({ trial: true });
      await choice.click();
      await expect(choice).toBeChecked();
    }
    await checkChoices(dialog);
    const source = index.plugins[0]!.distributions[0]!.releases[0]!.source;
    const selector = `github:${source.repository}@${source.revision}//${source.path}`;
    const npxAdd = `npx -y universal-agent-plugins@0.1.110 add ${selector} --target codex,cursor,hermes,antigravity`;
    const snippet = dialog.locator('.command-snippet--add');
    await expect(snippet.locator('code')).toHaveText(npxAdd);
    await snippet.locator('button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(npxAdd);
    await dialog.locator('.native-cli-checkbox input').check();
    const nativeAdd = npxAdd.replace('npx -y universal-agent-plugins@0.1.110', 'agentplugins');
    await expect(snippet.locator('code')).toHaveText(nativeAdd);
    await snippet.locator('button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(nativeAdd);
    await page.keyboard.press('Escape');
    await card.locator('.plugin-card__install-toggle').click();
    await checkChoices(dialog);
    await page.keyboard.press('Escape');
    await card.locator('.plugin-card__title-link').click();
    await expect(page).toHaveURL(/\/plugins\/i18n-state-00\/$/);
    // Nuxt finishes the route's smooth scroll after rendering the detail page.
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
    const panel = page.locator('.install-panel');
    await checkChoices(panel);
    await expect(panel.locator('.command-snippet--add code')).toHaveText(nativeAdd);
    await panel.locator('.native-cli-checkbox input').uncheck();
    await expect(panel.locator('.command-snippet--add code')).toHaveText(npxAdd);
    // Desktop is explicitly user-only. Preserve all choices on scope change;
    // selecting manual intent in Project must refuse without a global fallback.
    await panel.getByRole('button', { name: 'This project', exact: true }).click();
    await page.getByRole('checkbox', { name: /^Codex CLI(?:\s|$)/ }).click();
    // Switching automatic to manual starts a fresh explicit Codex request.
    await expect(page.getByRole('checkbox', { name: /^ChatGPT Desktop(?:\s|$)/ })).toBeDisabled();
    await expect(panel.locator('.command-snippet--add code')).toContainText('--target codex --scope project');
    expect(errors).toEqual([]);
  });
}

test('incompatible local and Web routes remain selected with an explicit explanation', async ({ page }) => {
  const errors = await offline(page);
  const index = localSourceFixture();
  const plugin = index.plugins[0]!;
  const distribution = plugin.distributions[0]!;
  distribution.releases[0]!.targets.push({
    client: 'chatgpt', scopes: ['user'], delivery: 'manual_activation', authentication: 'unknown',
    app_binding: { app_key: plugin.name, id: 'asdk_app_fixture', mcp_server: plugin.name },
  });
  distribution.targets = distribution.releases[0]!.targets;
  await projections(page, index);
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
  await card.locator('.plugin-card__install-toggle').click();
  const dialog = page.getByRole('dialog');
  await page.getByRole('checkbox', { name: /^ChatGPT Web(?:\s|$)/ }).click();
  await page.getByRole('checkbox', { name: /^Hermes Agent(?:\s|$)/ }).click();
  await expect(page.getByRole('checkbox', { name: /^ChatGPT Web(?:\s|$)/ })).toBeChecked();
  await expect(page.getByRole('checkbox', { name: /^Hermes Agent(?:\s|$)/ })).toBeChecked();
  await expect(dialog.locator('.command-snippet--add')).toHaveCount(0);
  await expect(dialog).toContainText(wording('en')('registryUi.install.selectionRoutesUnavailable'));
  await page.keyboard.press('Escape');
  await card.locator('.plugin-card__title-link').click();
  await expect(page).toHaveURL(/\/plugins\/i18n-state-00\/$/);
  // Wait for the completed route before opening a viewport-anchored picker.
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
  const panel = page.locator('.install-panel');
  await expect(panel.locator('.command-snippet--add')).toHaveCount(0);
  await expect(panel).toContainText(wording('en')('registryUi.install.selectionRoutesUnavailable'));
  await expect(page.getByRole('checkbox', { name: /^Hermes Agent(?:\s|$)/ })).toBeChecked();
  await page.getByRole('checkbox', { name: /^ChatGPT Web(?:\s|$)/ }).click();
  await expect(panel.locator('.command-snippet--add code')).toContainText('--target hermes');
  expect(errors).toEqual([]);
});
