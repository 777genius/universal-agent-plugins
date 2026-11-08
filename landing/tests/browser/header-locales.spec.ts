import { readFileSync } from 'node:fs';
import { expect, test } from '@playwright/test';
import { publishedLocales } from '../../data/i18n';
import { localizedPath } from '../../utils/localizedRoutes';

type HeaderCopy = {
  shell: {
    navigation: Record<'demo' | 'plugins' | 'agents' | 'faq' | 'open' | 'title', string>;
    docs: { navLabel: string };
    clientSupport: {
      title: string;
      columns: Record<'agent' | 'user' | 'project' | 'setup' | 'nextStep', string>;
      capabilities: { pluginsSkillsMcp: string; notSupported: string };
      setup: { automatic: string; manual: string };
      nextSteps: { claude: string; chatgpt: string };
    };
  };
  hero: { docsCta: string };
};

for (const locale of publishedLocales) {
  test(`${locale}: translated header and Docs stay readable at desktop and mobile widths`, async ({ page }) => {
    test.setTimeout(90_000);
    const copy = JSON.parse(
      readFileSync(new URL(`../../i18n/locales/${locale}.json`, import.meta.url), 'utf8'),
    ) as HeaderCopy;
    const { navigation, docs } = copy.shell;

    await page.setViewportSize({ width: 960, height: 850 });
    await page.goto(`.${localizedPath('/', locale)}`);
    await expect(page.locator('html')).toHaveAttribute('lang', locale);
    await expect(page.locator('html')).toHaveAttribute('dir', locale === 'ar' ? 'rtl' : 'ltr');
    const nav = page.locator('.app-header__nav a');
    await expect(nav).toHaveText([
      navigation.demo,
      navigation.plugins,
      navigation.agents,
      navigation.faq,
    ]);
    const support = page.locator('#agent-support');
    const tableCopy = copy.shell.clientSupport;
    await expect(support.locator('h2')).toHaveText(tableCopy.title);
    await expect(support.locator('thead th')).toHaveText([
      tableCopy.columns.agent, tableCopy.columns.user, tableCopy.columns.project,
      tableCopy.columns.setup, tableCopy.columns.nextStep,
    ]);
    await expect(support.locator('tbody tr')).toHaveCount(15);
    await expect(support.locator('tbody tr').first().locator('td').first())
      .toHaveText(tableCopy.capabilities.pluginsSkillsMcp);
    await expect(support.locator('tbody tr').first().locator('td').nth(2))
      .toHaveText(tableCopy.setup.automatic);
    await expect(support.locator('tbody tr').first().locator('td').last())
      .toHaveText(tableCopy.nextSteps.claude);
    await expect(support.locator('tbody tr').last().locator('td').nth(1))
      .toHaveText(tableCopy.capabilities.notSupported);
    await expect(support.locator('tbody tr').last().locator('td').nth(2))
      .toHaveText(tableCopy.setup.manual);
    await expect(support.locator('tbody tr').last().locator('td').last())
      .toHaveText(tableCopy.nextSteps.chatgpt);
    await expect(support).toHaveCSS('direction', locale === 'ar' ? 'rtl' : 'ltr');

    const desktopDocs = page.locator('.app-header__docs-btn');
    await expect(desktopDocs).toHaveText(docs.navLabel);
    await expect(desktopDocs).toHaveAccessibleName(copy.hero.docsCta);
    await expect(desktopDocs).toHaveAttribute('href', new RegExp(`/docs/${locale}/$`));
    await expect(page.locator('.app-header__desktop-actions .language-switcher__flag-icon')).toBeVisible();

    for (const width of [960, 1024, 1199, 1279, 1280, 1440]) {
      await page.setViewportSize({ width, height: 850 });
      await expect(nav.first()).toBeVisible();
      await expect(desktopDocs).toBeVisible();
      await expect(desktopDocs.locator('.v-btn__content')).toBeVisible();
      const layout = await page.evaluate(() => {
        const logo = document.querySelector('.app-header__inner > .app-logo')!;
        const nav = document.querySelector('.app-header__nav')!;
        const actions = document.querySelector('.app-header__desktop-actions')!;
        const docs = document.querySelector('.app-header__docs-btn')!;
        const label = docs.querySelector('.v-btn__content')!;
        const rect = (element: Element) => {
          const { left, right, width } = element.getBoundingClientRect();
          return { left, right, width };
        };
        const links = [...nav.querySelectorAll('a')].map((anchor) => ({
          button: rect(anchor),
          text: rect(anchor.querySelector('.v-btn__content')!),
        })).sort((a, b) => a.button.left - b.button.left);
        return {
          logo: rect(logo),
          links,
          nav: rect(nav),
          actions: rect(actions),
          docs: rect(docs),
          label: rect(label),
          gaps: links.slice(1).map((current, index) => current.button.left - links[index]!.button.right),
          overflow: document.documentElement.scrollWidth > innerWidth + 1,
        };
      });
      expect(layout.label.width, `${locale} @ ${width}px: Docs text has no width`).toBeGreaterThan(0);
      expect(layout.label.left, `${locale} @ ${width}px: Docs text clips on left`)
        .toBeGreaterThanOrEqual(layout.docs.left - 1);
      expect(layout.label.right, `${locale} @ ${width}px: Docs text clips on right`)
        .toBeLessThanOrEqual(layout.docs.right + 1);
      expect(layout.nav.right <= layout.actions.left + 1 ||
        layout.actions.right <= layout.nav.left + 1,
      `${locale} @ ${width}px: nav and actions overlap`).toBe(true);
      for (const gap of layout.gaps)
        expect(gap, `${locale} @ ${width}px: nav links touch`).toBeGreaterThanOrEqual(7);
      const horizontalOverlap = (a: { left: number; right: number }, b: { left: number; right: number }) =>
        Math.min(a.right, b.right) - Math.max(a.left, b.left);
      for (const { button, text } of layout.links) {
        expect(text.width, `${locale} @ ${width}px: nav text collapsed`).toBeGreaterThan(0);
        expect(text.left, `${locale} @ ${width}px: nav text clips on left`)
          .toBeGreaterThanOrEqual(button.left - 1);
        expect(text.right, `${locale} @ ${width}px: nav text clips on right`)
          .toBeLessThanOrEqual(button.right + 1);
        expect(horizontalOverlap(button, layout.logo), `${locale} @ ${width}px: nav overlaps logo`)
          .toBeLessThanOrEqual(1);
        expect(horizontalOverlap(button, layout.actions), `${locale} @ ${width}px: nav overlaps actions`)
          .toBeLessThanOrEqual(1);
      }
      expect(layout.overflow, `${locale} @ ${width}px: horizontal overflow`).toBe(false);
    }

    await page.setViewportSize({ width: 390, height: 844 });
    await expect(page.locator('.app-header__desktop-actions')).toBeHidden();
    await page.getByRole('button', { name: navigation.open }).click();
    const mobileMenu = page.getByRole('dialog', { name: navigation.title });
    await expect(mobileMenu).toBeVisible();
    for (const label of [navigation.demo, navigation.plugins, navigation.agents, navigation.faq])
      await expect(mobileMenu.getByRole('link', { name: label, exact: true })).toBeVisible();
    const mobileDocs = mobileMenu.getByRole('link', { name: copy.hero.docsCta, exact: true });
    await expect(mobileDocs).toBeVisible();
    await expect(mobileDocs).toHaveAttribute('href', new RegExp(`/docs/${locale}/$`));
  });
}
