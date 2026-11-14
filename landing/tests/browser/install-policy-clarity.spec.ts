import { expect, test } from '@playwright/test';
import { navigate, offline, projections, registryFixture, targets, wording } from './i18n-state.helpers';

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
  test('reviewed source uses the same explicit release-pinned command in hero, card and detail at ' + width + 'px', async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 950 });
    const errors = await offline(page);
    const index = pluginWithOnlyCodex();
    const plugin = index.plugins[0]!;
    plugin.trust_state = 'reviewed';
    await projections(page, index);
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const hero = page.locator('.hero-shell');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
    const source = plugin.distributions[0]!.releases[0]!.source;
    const manifestName = plugin.distributions[0]!.releases[0]!.manifest_name!;
    const selector = `github:${source.repository}@${source.revision}//${source.path}`;
    const cli = 'npx -y universal-agent-plugins@0.1.108';
    const notice = wording('en')('registryUi.install.pinnedCatalogSource');
    const repairNotice = wording('en')('registryUi.install.antigravityRepairUnavailable', { version: '0.1.108' });
    for (const [agent, client] of [['Hermes Agent', 'hermes'], ['Google Antigravity', 'antigravity'], ['Pi', 'pi']]) {
      await targets(page, hero, [agent!]);
      await expect(hero.locator('.command-snippet--add code')).toHaveText(`${cli} add ${selector} --target ${client}`);
      await expect(hero).toContainText(notice);
      if (client === 'antigravity') await expect(hero).toContainText(repairNotice);
      else await expect(hero).not.toContainText(repairNotice);
    }
    await card.locator('.plugin-card__install-toggle').click();
    for (const [agent, client] of [['Hermes Agent', 'hermes'], ['Google Antigravity', 'antigravity'], ['Pi', 'pi']]) {
      await targets(page, card, [agent!]);
      const add = `${cli} add ${selector} --target ${client}`;
      const snippet = card.locator('.plugin-card__install .command-snippet--add');
      await expect(snippet.locator('code')).toHaveText(add);
      if (client === 'antigravity') await expect(card).toContainText(repairNotice);
      else await expect(card).not.toContainText(repairNotice);
      await snippet.locator('button').click();
      await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(add);
    }
    await testInfo.attach(`explicit-source-card-${width}.png`, {
      body: await card.screenshot({ animations: 'disabled' }), contentType: 'image/png',
    });
    await card.locator('.plugin-card__title-link').click();
    const detail = page.locator('.install-panel');
    await expect(detail).toContainText(notice);
    await expect(detail.locator('.command-snippet--add code')).toHaveText(`${cli} add ${selector} --target pi`);
    for (const action of ['update', 'repair', 'remove'])
      await expect(detail.locator(`.command-snippet--${action} code`))
        .toHaveText(`${cli} ${action} ${manifestName} --target pi`);
    await expect(detail).not.toContainText(repairNotice);
    await detail.getByRole('button', { name: 'This project', exact: true }).click();
    await detail.locator('.app-multiselect__trigger').click();
    await expect(page.getByRole('checkbox', { name: /^Hermes Agent(?:\s|$)/ })).toBeDisabled();
    await page.getByRole('checkbox', { name: /^Google Antigravity(?:\s|$)/ }).click();
    await page.keyboard.press('Escape');
    await expect(detail.locator('.command-snippet--add code'))
      .toHaveText(`${cli} add ${selector} --target antigravity --scope project`);
    await expect(detail).toContainText(repairNotice);
    await expect(detail.locator('.command-snippet--repair')).toHaveCount(0);
    for (const action of ['update', 'remove'])
      await expect(detail.locator(`.command-snippet--${action} code`))
        .toHaveText(`${cli} ${action} ${manifestName} --target antigravity --scope project`);
    await testInfo.attach(`explicit-source-detail-${width}.png`, {
      body: await detail.screenshot({ animations: 'disabled' }), contentType: 'image/png',
    });
    await detail.getByRole('button', { name: 'This computer', exact: true }).click();
    await targets(page, detail, ['Google Antigravity']);
    const userAdd = `${cli} add ${selector} --target antigravity`;
    await expect(detail.locator('.command-snippet--add code')).toHaveText(userAdd);
    await expect(detail).toContainText(repairNotice);
    await expect(detail.locator('.command-snippet--repair')).toHaveCount(0);
    await detail.locator('.command-snippet--add button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(userAdd);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(errors).toEqual([]);
  });

  test('known signed failures keep source alternatives disabled at ' + width + 'px', async ({ page }) => {
    await page.setViewportSize({ width, height: 950 });
    const errors = await offline(page);
    const index = pluginWithOnlyCodex();
    index.plugins[0]!.trust_state = 'reviewed';
    index.plugins[0]!.distributions[0]!.releases[0]!.blocking_clients = ['hermes', 'antigravity', 'pi'];
    await projections(page, index);
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
    await card.locator('.plugin-card__install-toggle').click();
    for (const picker of [card, page.locator('.hero-shell')]) {
      await picker.locator('.app-multiselect__trigger').click();
      for (const agent of ['Hermes Agent', 'Google Antigravity', 'Pi'])
        await expect(page.getByRole('checkbox', { name: new RegExp(`^${agent}(?:\\s|$)`) })).toBeDisabled();
      await page.keyboard.press('Escape');
      await expect(picker.locator('.command-snippet--add code')).not.toContainText('github:');
    }
    expect(errors).toEqual([]);
  });

  test('unsupported package targets explain agent capability and ChatGPT Web binding at ' + width + 'px', async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    await projections(page, pluginWithOnlyCodex());
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
    await card.locator('.plugin-card__install-toggle').click();
    await card.locator('.app-multiselect__trigger').click();
    const web = page.getByRole('checkbox', { name: /^ChatGPT Web/ });
    for (const agent of ['Pi', 'Hermes Agent', 'Google Antigravity']) {
      const option = page.getByRole('checkbox', { name: new RegExp(`^${agent}(?:\\s|$)`) });
      await expect(option).toBeDisabled();
      await expect(option).toContainText(`This plugin has not been published for ${agent} yet.`);
    }
    await expect(web).toBeDisabled();
    await expect(web).toContainText('No verified ChatGPT Web app binding for this plugin.');
    await expect(web).toContainText('Connect Web apps inside ChatGPT.');
    await page.keyboard.press('Escape');
    await expect(card.locator('.plugin-card__install .command-snippet code')).not.toContainText('chatgptweb');
    await page.locator('.hero-shell .app-multiselect__trigger').click();
    for (const agent of ['Pi', 'Hermes Agent', 'Google Antigravity']) {
      const option = page.getByRole('checkbox', { name: new RegExp(`^${agent}(?:\\s|$)`) });
      await expect(option).toBeDisabled();
      await expect(option).toContainText(`This plugin has not been published for ${agent} yet.`);
    }
    await page.keyboard.press('Escape');
    await card.locator('.plugin-card__title-link').click();
    const detail = page.locator('.plugin-detail[data-install-source="i18n-state-00"]');
    await detail.locator('.app-multiselect__trigger').click();
    for (const agent of ['Pi', 'Hermes Agent', 'Google Antigravity']) {
      const option = page.getByRole('checkbox', { name: new RegExp(`^${agent}(?:\\s|$)`) });
      await expect(option).toBeDisabled();
      await expect(option).toContainText(`This plugin has not been published for ${agent} yet.`);
    }
    expect(errors).toEqual([]);
  });

  test('published agent policies enable the same explicit install on every picker at ' + width + 'px', async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    const index = pluginWithOnlyCodex();
    const plugin = index.plugins[0]!;
    const distribution = plugin.distributions[0]!;
    const release = distribution.releases[0]!;
    // These are UI projection fixtures, not production signed authorizations.
    release.targets.push(
      { client: 'hermes', authentication: 'not_required', delivery: 'managed', scopes: ['user'] },
      { client: 'antigravity', authentication: 'not_required', delivery: 'prepared', scopes: ['user'] },
      { client: 'pi', authentication: 'not_required', delivery: 'managed', scopes: ['user'] },
    );
    distribution.targets = release.targets;
    distribution.compatible_clients = release.targets.map(target => target.client);
    await projections(page, index);
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]').first();
    await card.locator('.plugin-card__install-toggle').click();
    const hero = page.locator('.hero-shell');
    for (const picker of [card, hero]) {
      await picker.locator('.app-multiselect__trigger').click();
      for (const agent of ['Pi', 'Hermes Agent', 'Google Antigravity']) {
        const option = page.getByRole('checkbox', { name: new RegExp(`^${agent}(?:\\s|$)`) });
        await expect(option).toBeEnabled();
        await expect(option).not.toContainText('has not been published');
      }
      await page.keyboard.press('Escape');
    }
    await targets(page, card, ['Hermes Agent']);
    await expect(card.locator('.command-snippet--add code')).toContainText('--target hermes');
    await expect(card.locator('.command-snippet--add code')).not.toContainText('--scope project');
    await card.locator('.plugin-card__title-link').click();
    const detail = page.locator('.plugin-detail[data-install-source="i18n-state-00"]');
    await expect(detail.locator('.command-snippet--add code')).toContainText('--target hermes');
    await detail.locator('.app-multiselect__trigger').click();
    await expect(page.getByRole('checkbox', { name: /^Hermes Agent(?:\s|$)/ })).toBeChecked();
    await expect(page.getByRole('checkbox', { name: /^Google Antigravity(?:\s|$)/ })).toBeEnabled();
    await expect(page.getByRole('checkbox', { name: /^Pi(?:\s|$)/ })).toBeEnabled();
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
  const index = pluginWithOnlyCodex();
  // The alternate Git route requires a reviewed active portable source; the
  // generic projection seed deliberately has no reviewed trust declaration.
  index.plugins[0]!.trust_state = 'reviewed';
  await projections(page, index);
  await navigate(page, '/agents/pi', 'en');
  const main = page.locator('main.agent-page');
  await expect(main).toContainText('Install compatible Pi extensions, Skills, and stdio/Streamable HTTP MCP servers');
  await expect(main).toContainText('user or project scope');
  await expect(main).toContainText('Packages with install or setup options for Pi: 1');
  await expect(main).toContainText('The CLI checks components, dependencies and the selected scope first');
  await expect(main.locator('.command-snippet--add code')).toContainText('universal-agent-plugins@0.1.108 add github:');
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

test('detail distinguishes a signed Project-only route from an exact Git alternative', async ({ page }) => {
  const errors = await offline(page);
  const index = pluginWithOnlyCodex();
  const distribution = index.plugins[0]!.distributions[0]!;
  distribution.releases[0]!.targets[0]!.scopes = ['project'];
  distribution.targets = distribution.releases[0]!.targets;
  await projections(page, index);
  await navigate(page, '/plugins/i18n-state-00', 'en');
  const detail = page.locator('.plugin-detail');
  await expect(detail).toContainText('Signed release for a project');
  await expect(detail.locator('.plugin-detail__summary-block').filter({ hasText: 'Signed release for a project' })).not.toContainText('Exact Git source');
  expect(errors).toEqual([]);
});
