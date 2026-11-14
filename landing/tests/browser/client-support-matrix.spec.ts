import { expect, test } from '@playwright/test';
import { clientSupportRows } from '../../data/clientSupport.ts';
import { clientIds } from '../../data/clientIds.generated.ts';
import { hiddenPublicClientIds } from '../../data/clients.ts';
import { hydrated, pinnedContext7Example, wording } from './i18n-state.helpers';

test('home lists the complete scope matrix immediately before FAQ', async ({ page }) => {
  await page.goto('./');

  const section = page.locator('#agent-support');
  const table = section.getByRole('table');
  await expect(table.locator('tbody tr')).toHaveCount(clientSupportRows.length);
  expect(clientSupportRows.map((row) => row.id).sort()).toEqual(
    clientIds.filter((id) => !hiddenPublicClientIds.has(id)).sort(),
  );
  await expect(table.getByRole('row', { name: /^VS Code / })).toHaveCount(0);
  const logos = table.locator('tbody th img');
  await expect(logos).toHaveCount(clientSupportRows.length);
  await expect(logos.first()).toHaveAttribute('src', /client-icons\/claude.svg$/);
  await expect(logos.nth(8)).toHaveAttribute('src', /client-icons\/windsurf.svg$/);
  // Icons load lazily: scroll to them before checking their decoded bytes.
  await logos.first().scrollIntoViewIfNeeded();
  await expect.poll(() => logos.first().evaluate((image) =>
    (image as HTMLImageElement).naturalWidth,
  )).toBeGreaterThan(0);
  await logos.last().scrollIntoViewIfNeeded();
  await expect.poll(() => logos.last().evaluate((image) =>
    (image as HTMLImageElement).naturalWidth,
  )).toBeGreaterThan(0);
  await expect(page.locator('#agent-support + #faq')).toHaveCount(1);
  await expect(table.getByRole('columnheader')).toHaveText([
    'Agent',
    'User scope',
    'Project scope',
    'Setup',
    'What you do after installing',
  ]);
  await expect(table.getByRole('row', { name: /^ChatGPT / })).toContainText('Not supported');
  await expect(table.getByRole('row', { name: /^Grok Build / })).toContainText('Skills, MCP (prepared)');
  await expect(table.getByRole('row', { name: /^Pi / })).toContainText(
    'Extensions, Skills, MCP',
  );
  await expect(section).toContainText('User installs into your agent profile');
  await expect(section).toContainText('Prepared = files are ready but need activation');
  await expect(section).not.toContainText('end-to-end');
  await expect(section).not.toContainText('E2E');
});

test('dark theme filters only monochrome client icons, not the entire app', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('theme', 'dark'));
  await page.goto('./');
  await expect(page.locator('.v-application.v-theme--dark')).toHaveCSS('filter', 'none');
  await expect(page.locator('.registry-client-support__icon--contrast').first())
    .toHaveCSS('filter', 'brightness(0) invert(1)');
});

test('scope matrix scrolls within its own region on narrow screens', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 800 });
  await page.goto('./');

  const scroller = page.getByRole('region', { name: 'Agent installation support table' });
  const sizes = await scroller.evaluate((element) => ({
    content: element.scrollWidth,
    container: element.clientWidth,
    page: document.documentElement.scrollWidth,
    viewport: window.innerWidth,
  }));
  expect(sizes.content).toBeGreaterThan(sizes.container);
  expect(sizes.page).toBeLessThanOrEqual(sizes.viewport);
});

test('dark agent icon contrast never filters the app or breaks the fixed orbit', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('./');
  await page.evaluate(() => localStorage.setItem('theme', 'dark'));
  await page.reload();

  const app = page.locator('.v-application').first();
  const monochromeIcon = page.locator('.registry-client-support__icon--contrast').first();
  await expect(app).toHaveClass(/v-theme--dark/);
  await expect(app).toHaveCSS('filter', 'none');
  await expect(monochromeIcon).toHaveCSS('filter', 'brightness(0) invert(1)');
  await expect(page.locator('.hero-agent-field--fixed')).toHaveCSS('position', 'fixed');

  for (const width of [1440, 1100, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
  }

  await page.evaluate(() => localStorage.setItem('theme', 'light'));
  await page.reload();
  await expect(app).toHaveClass(/v-theme--light/);
  await expect(app).toHaveCSS('filter', 'none');
  await expect(monochromeIcon).toHaveCSS('filter', 'none');
  // Both themes must leave the page horizontally contained on phones.
  for (const width of [390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
  }
});

const nativeClientCases = (['hermes', 'antigravity'] as const).flatMap((client) =>
  (['en', 'ar'] as const).flatMap((locale) =>
    (['light', 'dark'] as const).flatMap((theme) =>
      [1440, 390].map((width) => ({ client, locale, theme, width })),
    ),
  ),
);

for (const { client, locale, theme, width } of nativeClientCases) {
  test(`${client} ${locale} ${theme} route copies an immutable source command at ${width}px`, async ({ page, context }, testInfo) => {
    const t = wording(locale);
    const direction = locale === 'ar' ? 'rtl' : 'ltr';
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await context.addInitScript((value) => {
      if (window === window.top) localStorage.setItem('theme', value);
    }, theme);
    await page.goto(`${locale === 'en' ? './' : `./${locale}/`}agents/${client}/`);
    await hydrated(page);
    await expect(page.locator('html')).toHaveAttribute('lang', locale);
    await expect(page.locator('html')).toHaveAttribute('dir', direction);
    await expect(page.locator('.agent-page')).toHaveCSS('direction', direction);
    const containers = page.locator('.agent-page .v-container');
    await expect(containers).toHaveCount(3);
    for (const container of await containers.all()) {
      await expect(container).toHaveClass(new RegExp(`v-locale--is-${direction}`));
      await expect(container).toHaveCSS('direction', direction);
    }
    for (const selector of ['.agent-page__hero-grid', '.agent-page__identity', '.agent-page__intro', '.agent-page__steps'])
      await expect(page.locator(selector)).toHaveCSS('direction', direction);
    const position = (selector: string) => page.locator(selector).evaluate((element) => {
      const { x, y } = element.getBoundingClientRect();
      return { x, y };
    });
    const copyPosition = await position('.agent-page__copy');
    const installPosition = await position('.agent-page__install');
    const firstStep = await position('.agent-page__steps article:first-child');
    const lastStep = await position('.agent-page__steps article:last-child');
    if (width === 1440) {
      if (direction === 'rtl') {
        expect(copyPosition.x).toBeGreaterThan(installPosition.x);
        expect(firstStep.x).toBeGreaterThan(lastStep.x);
      } else {
        expect(copyPosition.x).toBeLessThan(installPosition.x);
        expect(firstStep.x).toBeLessThan(lastStep.x);
      }
    } else {
      expect(copyPosition.y).toBeLessThan(installPosition.y);
      expect(firstStep.y).toBeLessThan(lastStep.y);
    }
    await expect(page.locator('.v-application')).toHaveClass(new RegExp(`v-theme--${theme}`));
    const { command, selector } = await pinnedContext7Example(page, client);
    const install = page.locator('.agent-page__install');
    const snippet = install.locator('.command-snippet--add');
    await expect(snippet).toHaveCount(1);
    await expect(snippet.locator('code')).toHaveText(command);
    await expect(snippet.locator('pre')).toHaveCSS('direction', 'ltr');
    await expect(install.locator('p code')).toHaveText(selector);
    await expect(install).toContainText(t('registryUi.install.pinnedCatalogSource'));
    await expect(install).not.toContainText(t('registryUi.agentPage.packageExampleSourceHint'));
    await snippet.getByRole('button', { name: t('registryUi.command.copyLabel'), exact: true }).click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(command);
    const icon = page.locator('.agent-page__icon img');
    const iconFilename = client === 'hermes' ? 'hermes.webp' : 'antigravity.svg';
    expect((await icon.getAttribute('src'))?.endsWith(`/client-icons/${iconFilename}`)).toBe(true);
    await expect.poll(() => icon.evaluate((image) => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
    await expect(page.locator('.agent-page__steps article').nth(0).locator('p')).toHaveText(t(`registryUi.clients.${client}.delivery`));
    await expect(page.locator('.agent-page__steps article').nth(1).locator('p')).toHaveText(t(`registryUi.clients.${client}.activation`));
    await expect(page.locator('.agent-page__status')).toHaveText(t(`registryUi.clients.${client}.status`));
    if (locale === 'en') {
      if (client === 'hermes')
        await expect(page.locator('.agent-page__flow')).toContainText('Project installation never modifies User MCP configuration');
      else await expect(page.locator('.agent-page__status')).toHaveText('Prepared by CLI');
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);

    const name = `${client}-${locale}-${theme}-${width}.png`;
    const path = testInfo.outputPath(name);
    await page.screenshot({ path, fullPage: true, animations: 'disabled' });
    await testInfo.attach(name, { path, contentType: 'image/png' });
  });
}
