import { expect, test } from '@playwright/test';
import { navigate, offline, projections, registryFixture } from './i18n-state.helpers';

function pluginWithOnlyCodex() {
  const index = registryFixture();
  const plugin = index.plugins[0]!;
  index.plugins = [plugin];
  const distribution = plugin.distributions[0]!;
  distribution.kind = 'community';
  distribution.status = 'active';
  distribution.selectable = true;
  const release = distribution.releases[0]!;
  release.release_status = 'active';
  release.meets_minimum_capabilities = true;
  release.blocking_clients = [];
  release.targets = [
    { client: 'codex', authentication: 'not_required', delivery: 'managed', scopes: ['user'] },
  ];
  distribution.releases = [release];
  distribution.targets = release.targets;
  distribution.compatible_clients = ['codex'];
  plugin.installable = true;
  return index;
}

for (const width of [1440, 390]) {
  test('unsupported package targets explain Pi capability and ChatGPT Web binding at ' + width + 'px', async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    await projections(page, pluginWithOnlyCodex());
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
    await card.locator('.plugin-card__install-toggle').click();
    await card.locator('.app-multiselect__trigger').click();
    const pi = page.getByRole('checkbox', { name: /^Pi(?:\s|$)/ });
    const web = page.getByRole('checkbox', { name: /^ChatGPT Web/ });
    await expect(pi).toBeDisabled();
    await expect(pi).toContainText("Pi is supported, but this plugin's signed release has no compatible Pi target.");
    await expect(web).toBeDisabled();
    await expect(web).toContainText('No verified ChatGPT Web app binding for this plugin.');
    await expect(web).toContainText('Connect Web apps inside ChatGPT.');
    await page.keyboard.press('Escape');
    await expect(card.locator('.plugin-card__install .command-snippet code')).not.toContainText('chatgptweb');
    expect(errors).toEqual([]);
  });
}

test('automatic authentication shows a release-dependent client once, not both opposing claims', async ({ page }) => {
  const errors = await offline(page);
  const index = pluginWithOnlyCodex();
  const distribution = index.plugins[0]!.distributions[0]!;
  const older = distribution.releases[0]!;
  const newer = structuredClone(older);
  newer.release_sequence = older.release_sequence + 1;
  newer.targets[0]!.authentication = 'required';
  distribution.releases = [older, newer];
  await projections(page, index);
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
  const indicator = card.locator('.authentication-indicator');
  await expect(indicator).toHaveAttribute('data-varies', 'true');
  await indicator.click();
  const panel = page.locator('.authentication-indicator__content');
  await expect(panel.locator('.authentication-indicator__groups li')).toHaveCount(1);
  await expect(panel.locator('.authentication-indicator__policy')).toHaveText('Depends on agent or release');
  await expect(panel.locator('.authentication-indicator__clients')).toHaveText('codex');
  await expect(panel).toContainText('The same agent may have different sign-in requirements');
  expect(errors).toEqual([]);
});

test('Pi agent page describes extensions, Skills and MCP in user and project scopes', async ({ page }) => {
  const errors = await offline(page);
  await projections(page, pluginWithOnlyCodex());
  await navigate(page, '/agents/pi', 'en');
  const main = page.locator('main.agent-page');
  await expect(main).toContainText('Install compatible Pi extensions, Skills, and stdio/Streamable HTTP MCP servers');
  await expect(main).toContainText('user or project scope');
  await expect(main).toContainText('Signed catalog packages for Pi: 0');
  await expect(main).toContainText('Compatible Skills, MCP servers and extensions can also be installed');
  await expect(main).not.toContainText('Portable skills and MCP projection are outside');
  expect(errors).toEqual([]);
});

test('automatic authentication uses the selected User or Project scope, not a pooled account policy', async ({ page }) => {
  const errors = await offline(page);
  const index = pluginWithOnlyCodex();
  const distribution = index.plugins[0]!.distributions[0]!;
  const userRelease = distribution.releases[0]!;
  const projectRelease = structuredClone(userRelease);
  projectRelease.release_sequence += 1;
  projectRelease.targets[0]!.authentication = 'required';
  projectRelease.targets[0]!.scopes = ['project'];
  distribution.releases = [userRelease, projectRelease];
  await projections(page, index);
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
  const indicator = card.locator('.authentication-indicator');
  await expect(indicator).toHaveAttribute('data-authentication', 'not_required');
  await card.locator('.plugin-card__install-toggle').click();
  const project = card.locator('.install-scope').getByRole('button', { name: 'This project' });
  await expect(project).toBeEnabled();
  await project.click();
  await expect(indicator).toHaveAttribute('data-authentication', 'required');
  await expect(indicator).toHaveAttribute('data-varies', 'false');
  expect(errors).toEqual([]);
});
