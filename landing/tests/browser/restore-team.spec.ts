import { expect, test } from '@playwright/test';
import { hydrated, navigate, offline, projections, registryFixture } from './i18n-state.helpers';

// Each case has an isolated browser context and no cross-test mutable state.
test.describe.configure({ mode: 'parallel' });

const commands = {
  global: ['npx -y universal-agent-plugins install marketing-kit', 'npx -y universal-agent-plugins deps import --install'],
} as const;
const exportCommand = 'npx -y universal-agent-plugins deps export';
const manifest = `schema_version = 1

[dependencies]
"777genius/marketing-kit" = "0.1.0"

[targets]
claude = ["777genius/marketing-kit"]
codex = ["777genius/marketing-kit"]
gemini = ["777genius/marketing-kit"]`;
const cases = [
  ['en', 1440], ['en', 1110], ['en', 1100], ['en', 1024],
  ['en', 768], ['en', 540], ['en', 390], ['en', 320], ['en', 280],
  ['fr', 390], ['fr', 320], ['ru', 390], ['ru', 320], ['ar', 390], ['ar', 320],
] as const;

for (const theme of ['dark', 'light'] as const) {
  for (const [locale, width] of cases) {
    test(`marketing-kit restore ${locale} ${theme} at ${width}px`, async ({ page, context }, testInfo) => {
      // Two mode transitions, visual evidence, and geometry checks can exceed 30s on shared CI.
      if (locale === 'en' && theme === 'dark' && width === 1440) testInfo.setTimeout(60_000);
      await page.setViewportSize({ width, height: 1000 });
      await page.emulateMedia({ reducedMotion: 'reduce' });
      await context.addInitScript((value) => {
        if (window === window.top) localStorage.setItem('theme', value);
      }, theme);
      await page.goto(locale === 'en' ? './' : `./${locale}/`);
      await hydrated(page);
      await expect(page.locator('.v-application')).toHaveClass(new RegExp(`v-theme--${theme}`));
      await page.evaluate(() => document.fonts.ready.then(() => undefined));
      const section = page.locator('#restore-environment');
      await section.scrollIntoViewIfNeeded();
      await expect(section).toHaveAttribute('data-mode', 'global');
      await expect(section.locator('.registry-restore__monitor')).toHaveCount(1);
      await expect(section.locator('.registry-restore__laptop')).toHaveCount(1);
      await expect(section.locator('.registry-restore__wallpaper')).toHaveCount(0);
      await expect(section.locator('.registry-restore__laptop')).toHaveAttribute('src', /\/showcase\/restore-teammate-laptop\.svg$/);
      for (const mode of ['project', 'global'] as const) {
        await section.locator(`[data-mode-option="${mode}"]`).click();
        await expect(section).toHaveAttribute('data-mode', mode);
        await expect(section.locator(`[data-mode-option="${mode}"]`)).toHaveAttribute('aria-pressed', 'true');
        if (mode === 'project') {
          await expect(section.locator('.registry-restore__subtitle')).toContainText(locale === 'en' ? 'Global-only' : '');
          await expect(section.locator('.registry-restore__qualification')).toBeVisible();
          if (locale === 'en') {
            await expect(section.locator('.registry-restore__qualification')).toContainText('not yet available for Project');
            for (const dependency of ['Notion', 'HubSpot', 'Playwright']) {
              await expect(section.locator('.registry-restore__qualification')).toContainText(dependency);
            }
          }
          await expect(section.locator('.registry-restore__flow')).toHaveCount(0);
          await expect(section.locator('.registry-restore__copy')).toHaveCount(0);
          await expect(section).not.toContainText('install marketing-kit --scope project');
          expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
          continue;
        }
        if (locale === 'en') {
          await expect(section.locator('.registry-restore__subtitle')).toHaveText(
            'Marketing Kit example — move your global plugin setup to another computer.');
          await expect(section.locator('.registry-restore__handoff-note')).toHaveText(
            'Export and share both files. Import replaces the global setup after confirmation.');
        }
        await expect(section.locator('.registry-restore__locked-version')).toHaveCount(0);
        await expect(section.locator('.registry-restore__handoff')).not.toContainText('v0.1.0');
        await expect(section.locator('.registry-restore__package strong')).toHaveText(['v0.1.0', 'v0.1.0']);
        await expect(section.locator('.registry-restore__package span')).toHaveText(['marketing-kit', 'marketing-kit']);
        await expect(section.locator('.registry-restore__terminal')).toHaveCount(2);
        const typed = section.locator('.typed-command');
        await expect(typed).toHaveCount(2);
        for (let index = 0; index < 2; index++) {
          await expect(section.locator('.registry-restore__terminal').nth(index)).toHaveAttribute('data-command', commands[mode][index]!);
          await expect(typed.nth(index)).toHaveAttribute('data-complete', 'true');
          await expect(typed.nth(index).locator('.typed-command__visible')).toHaveText(commands[mode][index]!);
          await expect(typed.nth(index).locator('.typed-command__visible > .typed-command__line')).toHaveCount(0);
          const screen = section.locator('.registry-restore__screen').nth(index);
          const commandBox = (await screen.locator('.registry-restore__terminal').boundingBox())!;
          const outputBox = (await screen.locator('.registry-restore__package').boundingBox())!;
          expect(outputBox.y - commandBox.y - commandBox.height).toBeGreaterThanOrEqual(4);
          await expect(screen.locator('.registry-restore__package')).toHaveCSS('visibility', 'visible');
          const fonts = await screen.evaluate((element) => ({
            command: parseFloat(getComputedStyle(element.querySelector('.registry-restore__terminal')!).fontSize),
            output: parseFloat(getComputedStyle(element.querySelector('.registry-restore__package')!).fontSize),
          }));
          expect(fonts.output).toBeLessThan(fonts.command);
          for (const part of await screen.locator('.registry-restore__package > *').all()) {
            await expect(part).toHaveCSS('color', 'rgb(164, 166, 173)');
            await expect(part).toHaveCSS('font-weight', '400');
          }
        }
        await expect(section.locator('.registry-restore__export')).toHaveCount(1);
        await expect(section.locator('.registry-restore__export code')).toHaveText(exportCommand);
        const copyButtons = section.locator('.registry-restore__screen .registry-restore__copy');
        await expect(copyButtons).toHaveCount(2);
        if (locale === 'en') {
          await expect(copyButtons.first()).toHaveAccessibleName(/Copy command: My computer/);
          await expect(copyButtons.last()).toHaveAccessibleName(/Copy command: Teammate.s computer/);
        }
        await expect(copyButtons.first()).toBeEnabled();
        await expect(copyButtons.last()).toBeEnabled();
        await expect(section.locator('.registry-restore__qualification')).toHaveCount(0);
        expect(await copyButtons.evaluateAll((buttons) => buttons.every((button) => {
          const { width: hitWidth, height: hitHeight } = button.getBoundingClientRect();
          return hitWidth >= 24 && hitHeight >= 24;
        }))).toBe(true);
        if (width <= 340) {
          expect(await section.locator('.registry-restore__screen').evaluateAll((screens) =>
            screens.flatMap((screen, index) => {
              const copy = screen.querySelector('.registry-restore__copy')!.getBoundingClientRect();
              return [...screen.querySelectorAll('.registry-restore__package > *')].filter((node) => {
                const badge = node.getBoundingClientRect();
                return copy.left < badge.right && copy.right > badge.left && copy.top < badge.bottom && copy.bottom > badge.top;
              }).map((node) => `screen ${index + 1}: copy overlaps ${node.textContent}`);
            }),
          )).toEqual([]);
        }
        expect(await section.locator('.registry-restore__screen').evaluateAll((screens) =>
          screens.flatMap((screen, index) => {
            const bounds = screen.getBoundingClientRect();
            return [...screen.querySelectorAll('.registry-restore__stepbar, .registry-restore__package, .registry-restore__terminal')]
              .filter((node) => {
                if (getComputedStyle(node).display === 'none') return false;
                const b = node.getBoundingClientRect();
                return b.left < bounds.left - 2 || b.right > bounds.right + 2 || b.top < bounds.top - 2 || b.bottom > bounds.bottom + 2 || node.scrollWidth > node.clientWidth + 2;
              }).map((node) => `${index}: ${node.className}`);
          }),
        )).toEqual([]);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
        if ((locale === 'en' && [1440, 390, 280].includes(width)) || (locale === 'ar' && width === 390)) {
          await section.screenshot({ path: testInfo.outputPath(`marketing-kit-${mode}-${theme}-${locale}-${width}.png`), animations: 'disabled' });
        }
        const machines = await section.locator('.registry-restore__machine').evaluateAll((nodes) => nodes.map((node) => node.getBoundingClientRect().toJSON()));
        if (width > 1100) expect(machines[0]!.top).toBeCloseTo(machines[1]!.top, 0);
        else {
          expect(machines[1]!.top).toBeGreaterThan(machines[0]!.bottom);
          if (width > 480) machines.forEach((machine) => expect(machine.width).toBeLessThanOrEqual(501));
        }
      }
    });
  }
}

test('manifest morph keeps highlighted TOML lossless, accessible and dismissible', async ({ page }, testInfo) => {
  // Screenshot, focus and CSS checks can exceed 30s on a loaded CI runner.
  test.setTimeout(60_000);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('./');
  await hydrated(page);
  const handoff = page.locator('#restore-environment .registry-restore__handoff');
  await handoff.scrollIntoViewIfNeeded();
  const file = handoff.getByRole('button', { name: /uap.toml/i });
  await expect(file).toHaveAccessibleName(/preview uap\.toml/i);
  await expect(file).toHaveAttribute('aria-expanded', 'false');
  await file.focus();
  await file.press('Enter');
  await expect(file).toHaveAttribute('aria-expanded', 'true');
  const preview = handoff.getByRole('region');
  expect(await preview.locator('pre code').textContent()).toBe(manifest);
  await expect(preview.locator('.marketing-manifest__heading strong')).toHaveText('Preview uap.toml example');
  const colors = await Promise.all(['key', 'section', 'string', 'number'].map((kind) =>
    preview.locator(`.toml-token--${kind}`).first().evaluate((element) => getComputedStyle(element).color),
  ));
  expect(new Set(colors).size).toBe(4);
  await handoff.screenshot({ path: testInfo.outputPath('marketing-kit-manifest-expanded.png'), animations: 'disabled' });
  await file.press('Escape');
  await expect(file).toHaveAttribute('aria-expanded', 'false');
  await file.click();
  await expect(preview).toBeVisible();
  const dismiss = preview.getByRole('button', { name: /close file preview/i });
  await dismiss.focus();
  await expect(dismiss).toBeFocused();
  // Exercise the real keyboard event without re-resolving a possibly detached button.
  await page.keyboard.press('Escape');
  await expect(file).toBeFocused();
  await expect(file).toHaveAttribute('aria-expanded', 'false');
  await file.click();
  await expect(preview).toBeVisible();
  await handoff.locator('.registry-restore__handoff-note').click();
  await expect(file).toHaveAttribute('aria-expanded', 'false');
});

test('manifest unfolds on first view, folds into icon and reopens on hover', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'no-preference' });
  await page.goto('./');
  await hydrated(page);
  await page.mouse.move(0, 0);
  const handoff = page.locator('#restore-environment .registry-restore__handoff');
  const file = handoff.getByRole('button', { name: /uap.toml/i });
  await handoff.scrollIntoViewIfNeeded();
  await expect(file).toHaveAttribute('aria-expanded', 'true', { timeout: 6_000 });
  await expect(file).toHaveAttribute('aria-expanded', 'false', { timeout: 7_000 });
  await file.hover();
  await expect(file).toHaveAttribute('aria-expanded', 'true');
  await page.mouse.move(0, 0);
  await expect(file).toHaveAccessibleName(/preview uap\.toml/i);
  await expect(file).toHaveAttribute('aria-expanded', 'false');
});

test('both modes copy complete commands during typing and reset feedback on mode changes', async ({ page, context }) => {
  // The three mode transitions and clipboard operations can exceed the default 30s under CI load.
  test.setTimeout(60_000);
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.emulateMedia({ reducedMotion: 'no-preference' });
  await page.goto('./');
  await hydrated(page);
  const section = page.locator('#restore-environment');
  await section.scrollIntoViewIfNeeded();
  const buttons = section.locator('.registry-restore__screen .registry-restore__copy');
  for (const mode of ['global'] as const) {
    await section.locator(`[data-mode-option="${mode}"]`).click();
    await expect(section.locator('.registry-restore__copy-status')).toBeEmpty();
    for (let i = 0; i < 2; i++) {
      await buttons.nth(i).focus();
      await buttons.nth(i).press('Enter');
      await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(commands[mode][i]);
      await expect(section.locator('.registry-restore__copy-status')).toContainText('Copied:');
    }
    if (mode === 'global') {
      await section.locator('.registry-restore__export button').click();
      await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(exportCommand);
    }
  }
  for (let i = 0; i < 2; i++) await expect(section.locator('.typed-command').nth(i)).toHaveAttribute('data-complete', 'true', { timeout: 15_000 });
  await expect(section.locator('.typed-command__visible')).toHaveText([...commands.global]);
  await expect(section.locator('.registry-restore__package').first()).toHaveCSS('visibility', 'visible');
});

for (const mode of ['global'] as const) {
  test(`${mode} clipboard denial announces failure and restores copy-button focus`, async ({ page }) => {
    await page.goto('./');
    await hydrated(page);
    const section = page.locator('#restore-environment');
    await section.locator(`[data-mode-option="${mode}"]`).click();
    await page.evaluate(() => {
      Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async () => { throw new Error('Permission denied'); } } });
      Object.defineProperty(document, 'execCommand', { configurable: true, value: () => false });
    });
    const copy = section.locator('.registry-restore__screen .registry-restore__copy').first();
    await copy.focus();
    await copy.press('Enter');
    await expect(copy).toBeFocused();
    await expect(copy).toHaveText('Copy failed');
    await expect(section.locator('.registry-restore__copy-status')).toContainText('Copy failed: My computer');
    await expect(copy).toHaveAccessibleName('Copy failed: My computer');
  });
}

for (const qualified of [false, true]) {
  test(`unqualified Marketing Kit Project commands remain unavailable with catalog scopes ${qualified}`, async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    const errors = await offline(page);
    const index = registryFixture();
    const plugin = index.plugins[0]!;
    index.plugins = [plugin];
    plugin.name = 'marketing-kit';
    plugin.install_source = 'marketing-kit';
    plugin.trust_state = 'reviewed';
    plugin.installable = true;
    for (const distribution of plugin.distributions) {
      for (const release of distribution.releases) {
        for (const target of release.targets) target.scopes = qualified ? ['user', 'project'] : ['user'];
      }
    }
    await projections(page, index);
    await navigate(page, '/', 'en');
    const section = page.locator('#restore-environment');
    await section.locator('[data-mode-option="project"]').click();
    await expect(section.locator('.registry-restore__qualification')).toBeVisible();
    await expect(section.locator('.registry-restore__flow')).toHaveCount(0);
    await expect(section.locator('.registry-restore__copy')).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}

test('teammate laptop is served from the same deployment base without wallpaper', async ({ request }) => {
  const response = await request.get('./showcase/restore-teammate-laptop.svg');
  expect(response.ok()).toBe(true);
  expect(response.headers()['content-type']).toMatch(/image\/svg\+xml/);
  const source = await response.text();
  expect(source).toContain('UAP laptop chassis');
  expect(source).not.toContain('Purple twilight mountains');
});
