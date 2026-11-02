import { expect, test } from '@playwright/test';
import {
  navigate,
  offline,
  projections,
  registryFixture,
  targets,
  wording,
} from './i18n-state.helpers';

function scopedFixture() {
  const index = registryFixture();
  const plugin = index.plugins[0]!;
  index.plugins = [plugin];
  const primary = plugin.distributions[0]!;
  primary.kind = 'community';
  primary.status = 'active';
  const release = primary.releases[0]!;
  release.release_status = 'active';
  release.blocking_clients = [];
  release.meets_minimum_capabilities = true;
  release.components = ['mcp'];
  release.targets = [
    { client: 'cursor', scopes: ['user'], delivery: 'managed', authentication: 'not_required' },
    { client: 'codex', scopes: ['user'], delivery: 'managed', authentication: 'not_required' },
  ];
  primary.releases = [release];
  const project = structuredClone(primary);
  project.id = 'scope-project-fallback';
  project.releases[0]!.components = ['skills'];
  project.releases[0]!.targets = [
    {
      client: 'codex',
      scopes: ['user', 'project'],
      delivery: 'prepared',
      authentication: 'required',
    },
  ];
  plugin.distributions = [primary, project];
  return index;
}

for (const width of [1440, 390]) {
  test(`manual user Cursor never leaks into default project selection at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await offline(page);
    await projections(page, scopedFixture());
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
    await card.locator('.plugin-card__install-toggle').click();
    await targets(page, card, ['Cursor']);
    await expect(card.locator('.app-multiselect__trigger')).toContainText('Cursor');
    await card.getByRole('button', { name: 'This project', exact: true }).click();
    await expect(card.locator('.app-multiselect__trigger')).toContainText('All installed agents');
    await expect(card.locator('.command-snippet--add code')).toHaveText(
      'npx -y universal-agent-plugins add i18n-state-00 --scope project',
    );
    expect(errors).toEqual([]);
  });

  test(`scope selection, real picker recovery, copied lifecycle commands and navigation at ${width}px`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    const errors = await offline(page);
    await projections(page, scopedFixture());
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
    await card.locator('.plugin-card__install-toggle').click();
    await expect(card.locator('.command-snippet code')).toHaveText(
      'npx -y universal-agent-plugins add i18n-state-00',
    );
    await card.getByRole('button', { name: 'This project', exact: true }).click();
    const autoAdd = 'npx -y universal-agent-plugins add i18n-state-00 --scope project';
    await expect(card.locator('.app-multiselect__trigger')).toContainText('All installed agents');
    await expect(card.locator('.command-snippet code')).toHaveText(autoAdd);
    await card.locator('.app-multiselect__trigger').click();
    await expect(page.locator('.app-multiselect__auto')).toHaveAttribute('aria-pressed', 'true');
    await expect(page.getByRole('checkbox', { name: /^Cursor(?:\s|$)/ })).toBeDisabled();
    await expect(page.getByRole('checkbox', { name: /^Codex(?:\s|$)/ })).toBeEnabled();
    await page.keyboard.press('Escape');
    // An explicit checkbox switches to manual mode; no implicit Cursor selection.
    await targets(page, card, ['Codex']);
    const expectedAdd =
      'npx -y universal-agent-plugins add i18n-state-00 --target codex --scope project';
    await expect(card.locator('.command-snippet code')).toHaveText(expectedAdd);
    await expect(card.locator('.install-outcome')).toContainText('Skills');
    await expect(card.locator('.install-outcome')).not.toContainText('MCP tools');
    await expect(card.locator('.install-outcome')).toContainText('Sign-in or credentials');
    await expect(card.locator('.plugin-card__source-label')).toContainText(
      'Agents: 0 automatic setup · 1 need follow-up',
    );
    await card.locator('.command-snippet button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(expectedAdd);

    await card.locator('.plugin-card__title-link').click();
    const panel = page.locator('.install-panel');
    await expect(panel.getByRole('button', { name: 'This project', exact: true })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await expect(panel).toContainText('Saved in this project’s uap.toml and uap.lock');
    await expect(panel).toContainText('deps status --scope project');
    await expect(panel).not.toContainText('Saved for your user account');
    for (const action of ['add', 'update', 'repair', 'remove']) {
      const snippet = panel.locator(`.command-snippet--${action}`);
      const target = action === 'add' || action === 'repair' ? ' --target codex' : '';
      const command = `npx -y universal-agent-plugins ${action} i18n-state-00${target} --scope project`;
      await expect(snippet.locator('code')).toHaveText(command);
      await snippet.getByRole('button').click();
      await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(command);
    }
    await panel.locator('.app-multiselect__trigger').click();
    await expect(page.locator('.app-multiselect__auto')).toHaveAttribute('aria-pressed', 'false');
    await page.locator('.app-multiselect__auto').click();
    await expect(page.locator('.app-multiselect__auto')).toHaveAttribute('aria-pressed', 'true');
    await page.keyboard.press('Escape');
    await expect(panel.locator('.app-multiselect__trigger')).toContainText('All installed agents');
    await expect(panel.locator('.command-snippet--add code')).toHaveText(autoAdd);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    await panel.screenshot({
      path: testInfo.outputPath(`project-detail-${width}.png`),
      animations: 'disabled',
    });
    await page.goBack();
    await expect(card.locator('.app-multiselect__trigger')).toContainText('All installed agents');
    await expect(card.locator('.command-snippet code')).toHaveText(autoAdd);
    await card.scrollIntoViewIfNeeded();
    // Back navigation remounts the existing catalog scroll-reveal observer.
    // Wait for its real visible state before accepting visual evidence.
    await expect(page.locator('.catalog-wrap')).toHaveCSS('opacity', '1');
    await card.screenshot({
      path: testInfo.outputPath(`project-card-${width}.png`),
      animations: 'disabled',
    });
    await navigate(page, '/', 'ru', '?q=i18n-state-00');
    await expect(
      card.getByRole('button', {
        name: wording('ru')('registryUi.install.scope.project'),
        exact: true,
      }),
    ).toHaveAttribute('aria-pressed', 'true');
    await expect(card.locator('.command-snippet code')).toHaveText(autoAdd);
    expect(errors).toEqual([]);
  });

  test(`feed invalidation retains project intent without a copyable user command at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 1000 });
    const errors = await offline(page);
    const index = scopedFixture();
    await projections(page, index);
    await navigate(page, '/', 'en', '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
    await card.locator('.plugin-card__install-toggle').click();
    await targets(page, card, ['Codex']);
    await card.getByRole('button', { name: 'This project', exact: true }).click();
    await expect(card.locator('.command-snippet code')).toContainText('--scope project');
    // The first detail projection now represents a newer feed that withdrew
    // project support. Preferences must not fall back to automatic user scope.
    index.plugins[0]!.distributions[1]!.releases[0]!.targets[0]!.scopes = ['user'];
    await card.locator('.plugin-card__title-link').click();
    const panel = page.locator('.install-panel');
    await expect(panel.getByRole('button', { name: 'This project', exact: true })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await expect(panel.locator('.app-multiselect__trigger')).toContainText('Codex');
    await expect(panel.locator('.command-snippet')).toHaveCount(0);
    await expect(panel.getByRole('status').first()).toContainText('combination of agents');
    await panel.getByRole('button', { name: 'This computer', exact: true }).click();
    await expect(panel.locator('.command-snippet--add code')).toHaveText(
      'npx -y universal-agent-plugins add i18n-state-00 --target codex',
    );
    // No project-capable release remains; the unavailable selector is not shown.
    await expect(panel.locator('.install-scope')).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}

test('reviewed user-only v1 catalog generates exact pinned project Git commands', async ({ page }) => {
  const errors = await offline(page);
  const index = registryFixture();
  index.plugins[0]!.trust_state = 'reviewed';
  await projections(page, index);
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
  await card.locator('.plugin-card__install-toggle').click();
  const project = card.getByRole('button', { name: 'This project', exact: true });
  await expect(project).toBeEnabled();
  await project.click();
  const source = index.plugins[0]!.distributions[0]!.releases[0]!.source;
  const exact = `github:${source.repository}@${source.revision}//${source.path}`;
  await expect(card.locator('.app-multiselect__trigger')).toContainText('All installed agents');
  await expect(card.locator('.command-snippet code')).toHaveText(
    `npx -y universal-agent-plugins add ${exact} --scope project`,
  );
  await targets(page, card, ['Codex']);
  const expected = `npx -y universal-agent-plugins add ${exact} --target codex --scope project`;
  await expect(card.locator('.command-snippet code')).toHaveText(expected);
  await card.locator('.command-snippet button').click();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(expected);
  await card.locator('.plugin-card__title-link').click();
  const panel = page.locator('.install-panel');
  await expect(panel.locator('.command-snippet--add code')).toHaveText(expected);
  await expect(panel.locator('.command-snippet--update code')).toHaveText(expected);
  await expect(panel.locator('.command-snippet--remove code'))
    .toHaveText('npx -y universal-agent-plugins remove i18n-state-00 --scope project');
  await panel.getByRole('button', { name: 'This computer', exact: true }).click();
  await expect(panel.locator('.command-snippet--add code'))
    .toHaveText('npx -y universal-agent-plugins add i18n-state-00 --target codex');
  expect(errors).toEqual([]);
});

test('unqualified project scope is hidden while auto agent delivery stays non-numeric', async ({
  page,
}) => {
  const errors = await offline(page);
  await projections(page, registryFixture());
  await navigate(page, '/', 'en', '?q=i18n-state-00');
  const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
  // Terminal auto-detection does not know which local agents are installed.
  await expect(card.locator('.plugin-card__source-label')).toHaveCount(0);
  await card.locator('.plugin-card__install-toggle').click();
  await expect(card.locator('.install-scope')).toHaveCount(0);
  await expect(card.locator('.command-snippet code')).toHaveText(
    'npx -y universal-agent-plugins add i18n-state-00',
  );
  await card.locator('.plugin-card__title-link').click();
  await expect(page.locator('.plugin-detail .install-scope')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('Marketing Kit does not advertise unsupported signed project scope or force Cursor', async ({ page }) => {
  const errors = await offline(page);
  const index = scopedFixture();
  const plugin = index.plugins[0]!;
  plugin.name = 'marketing-kit';
  plugin.install_source = 'marketing-kit';
  plugin.display_name = 'Marketing Kit';
  plugin.trust_state = 'reviewed';
  plugin.client_support.clients = ['cursor', 'codex', 'claude', 'gemini'];
  plugin.distributions = plugin.distributions.slice(0, 1);
  plugin.distributions[0]!.releases[0]!.source.revision = 'c5bcedd4ecf12585a8a50401852e57a44891ebec';
  plugin.distributions[0]!.releases[0]!.targets.push(
    { client: 'claude', scopes: ['user'], delivery: 'managed', authentication: 'required' },
    { client: 'gemini', scopes: ['user'], delivery: 'managed', authentication: 'required' },
  );
  await projections(page, index);
  await navigate(page, '/', 'en', '?q=marketing-kit');
  const card = page.locator('.plugin-card[data-install-source="marketing-kit"]');
  await card.locator('.plugin-card__install-toggle').click();

  const computer = card.getByRole('button', { name: 'This computer', exact: true });
  const project = card.getByRole('button', { name: 'This project', exact: true });
  await expect(computer).toHaveAttribute('aria-pressed', 'true');
  await expect(project).toBeDisabled();
  await expect(card.locator('.marketing-kit-project')).toHaveCount(0);
  await expect(card).toContainText('Marketing Kit requires Notion, HubSpot and Playwright, which are not project-qualified yet.');
  await expect(card.locator('.command-snippet--add code')).toHaveText(
    'npx -y universal-agent-plugins add marketing-kit',
  );
  await expect(card.locator('.command-snippet--add code')).not.toContainText('--target cursor');
  await expect(card).not.toContainText('Cursor: Files are prepared.');
  await card.locator('.plugin-card__title-link').click();
  const panel = page.locator('.install-panel');
  await expect(panel.getByRole('button', { name: 'This project', exact: true })).toBeDisabled();
  await expect(panel).toContainText('not project-qualified yet');
  await expect(panel.locator('.marketing-kit-project')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('Marketing Kit project becomes automatic only with a new signed project release', async ({ page }) => {
  const errors = await offline(page);
  const index = scopedFixture();
  const plugin = index.plugins[0]!;
  plugin.name = 'marketing-kit';
  plugin.install_source = 'marketing-kit';
  plugin.display_name = 'Marketing Kit';
  plugin.trust_state = 'reviewed';
  plugin.distributions = plugin.distributions.slice(0, 1);
  const release = plugin.distributions[0]!.releases[0]!;
  release.source.revision = null;
  // Synthetic future signed policy: tests UI selection only, NOT closure
  // qualification. Production must first verify every dependency in project
  // scope before publishing a project-enabled Marketing Kit release.
  for (const target of release.targets) {
    target.scopes = target.client === 'codex' ? ['user', 'project'] : ['user'];
  }
  await projections(page, index);
  await navigate(page, '/', 'en', '?q=marketing-kit');
  const card = page.locator('.plugin-card[data-install-source="marketing-kit"]');
  await card.locator('.plugin-card__install-toggle').click();
  await card.getByRole('button', { name: 'This project', exact: true }).click();
  await expect(card.locator('.app-multiselect__trigger')).toContainText('All installed agents');
  await expect(card.locator('.command-snippet--add code')).toHaveText(
    'npx -y universal-agent-plugins add marketing-kit --scope project',
  );
  await targets(page, card, ['Codex']);
  await expect(card.locator('.command-snippet--add code')).toHaveText(
    'npx -y universal-agent-plugins add marketing-kit --target codex --scope project',
  );
  expect(errors).toEqual([]);
});

for (const locale of ['en', 'ar'] as const) {
  test(`segmented install scope indicator tracks selected button in ${locale}`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 850 });
    const errors = await offline(page);
    await projections(page, scopedFixture());
    await navigate(page, '/', locale, '?q=i18n-state-00');
    const card = page.locator('.plugin-card[data-install-source="i18n-state-00"]');
    await card.locator('.plugin-card__install-toggle').click();
    const options = card.locator('.install-scope__choices');
    const thumb = options.locator('.install-scope__thumb');
    for (const kind of ['user', 'project', 'user'] as const) {
      const selected = options.getByRole('button', {
        name: wording(locale)(`registryUi.install.scope.${kind}`),
        exact: true,
      });
      await selected.click();
      await expect(selected).toHaveAttribute('aria-pressed', 'true');
      await expect.poll(async () => {
        const button = await selected.boundingBox();
        const indicator = await thumb.boundingBox();
        if (!button || !indicator) return 999;
        return Math.abs(button.x + button.width / 2 - indicator.x - indicator.width / 2);
      }).toBeLessThan(1.5);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(errors).toEqual([]);
  });
}
