import { expect, test, type Page } from '@playwright/test';
import { hydrated, setSignedMirrorTestTime, wording } from './i18n-state.helpers';

const docsLink = (locale: string, page: string) =>
  new RegExp(`/docs/${locale}/${page.replace('.', '\\.')}$`);

async function openSdk(page: Page, path = './') {
  await page.goto(path);
  await hydrated(page);
  const section = page.locator('#sdk');
  await section.scrollIntoViewIfNeeded();
  return section;
}

test.beforeEach(async ({ page }) => {
  await setSignedMirrorTestTime(page);
});

test('SDK section follows the FAQ with the film scenes, real links and localized guides', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  const section = await openSdk(page);
  const t = wording('en');

  // Directly under the FAQ, which stays the section before it.
  expect(await page.evaluate(() => document.querySelector('#faq')?.nextElementSibling?.id)).toBe(
    'sdk',
  );
  await expect(section.locator('h2')).toHaveText(t('shell.sdk.title'));
  await expect(section.locator('.registry-sdk__feature h3')).toHaveText([
    t('shell.sdk.features.detect.title'),
    t('shell.sdk.features.install.title'),
    t('shell.sdk.features.cli.title'),
    t('shell.sdk.features.hooks.title'),
  ]);
  await expect(section.locator('#sdk-apps-title')).toHaveText(t('shell.sdk.apps.title'));
  await expect(section.locator('.registry-sdk__badge')).toHaveText(t('shell.sdk.apps.badge'));
  await expect(section.getByRole('img', { name: t('shell.sdk.apps.diagramLabel') })).toBeVisible();
  await expect(section.locator('.sdk-flow__target strong')).toHaveText([
    'Codex',
    'Claude Code',
    'Gemini CLI',
  ]);

  await expect(section.locator('#sdk-showcase-title')).toHaveText('Agent Notifications');
  const links = section.locator('.registry-sdk__showcase-links a');
  await expect(links.nth(0)).toHaveAttribute('href', 'https://agent-notifications.com/');
  await expect(links.nth(1)).toHaveAttribute(
    'href',
    'https://github.com/777genius/agent-notifications',
  );
  // Static, owner-approved figure next to the links; no GitHub API request.
  await expect(section.locator('.registry-sdk__stars')).toHaveText(
    t('shell.sdk.showcase.stars', { stars: '800+' }),
  );
  await expect(section.locator('.registry-sdk__footer a')).toHaveAttribute(
    'href',
    docsLink('en', 'sdk/'),
  );
  await expect(section.locator('.registry-sdk__link')).toHaveAttribute(
    'href',
    docsLink('en', 'sdk/typescript-management.html'),
  );

  // Four cards in one row on desktop, no page overflow.
  const cards = await section
    .locator('.registry-sdk__feature')
    .evaluateAll((nodes) => nodes.map((node) => Math.round(node.getBoundingClientRect().y)));
  expect(new Set(cards).size).toBe(1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('SDK scenes loop only while visible and settle for reduced motion', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto('./');
  await hydrated(page);
  const section = page.locator('#sdk');
  await expect(section).toHaveClass(/registry-sdk--motion/);
  await expect(section).not.toHaveClass(/registry-sdk--playing/);
  const states = () =>
    section.evaluate((node) => [
      ...new Set(
        node
          .getAnimations({ subtree: true })
          .filter((animation) => animation instanceof CSSAnimation)
          .map((animation) => animation.playState),
      ),
    ]);
  // Off screen nothing animates; on screen every loop runs.
  expect(await states()).toEqual([]);
  await section.scrollIntoViewIfNeeded();
  await expect(section).toHaveClass(/registry-sdk--playing/);
  await expect.poll(states).toEqual(['running']);
  await page.evaluate(() => scrollTo({ top: 0, behavior: 'instant' }));
  await expect(section).not.toHaveClass(/registry-sdk--playing/);
  await expect.poll(states).toEqual([]);

  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.reload();
  await hydrated(page);
  await section.scrollIntoViewIfNeeded();
  await expect(section).not.toHaveClass(/registry-sdk--motion/);
  expect(await section.evaluate((node) => node.getAnimations({ subtree: true }).length)).toBe(0);
  // The settled frame shows the finished install everywhere.
  const installed = section.locator('.sdk-flow__status-installed');
  await expect(installed).toHaveCount(4);
  for (const label of await installed.all()) await expect(label).toHaveCSS('opacity', '1');
});

for (const { width, locale } of [
  { width: 1024, locale: 'en' },
  { width: 768, locale: 'ru' },
  { width: 390, locale: 'ar' },
] as const) {
  test(`SDK section stays inside the viewport at ${width}px in ${locale}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const section = await openSdk(page, locale === 'en' ? './' : `./${locale}/`);
    const t = wording(locale);
    await expect(section.locator('h2')).toHaveText(t('shell.sdk.title'));
    await expect(section.locator('.registry-sdk__stars')).toHaveText(
      t('shell.sdk.showcase.stars', { stars: '800+' }),
    );
    await expect(section.locator('.registry-sdk__footer a')).toHaveAttribute(
      'href',
      docsLink(locale, 'sdk/'),
    );
    // Arabic reads right to left; the flow keeps its left-to-right order.
    await expect(section).toHaveAttribute('dir', locale === 'ar' ? 'rtl' : 'ltr');
    await expect(section.locator('.sdk-flow__graph')).toHaveAttribute('dir', 'ltr');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    const overflow = await section.evaluate((node) => {
      const bounds = node.getBoundingClientRect();
      return [
        ...node.querySelectorAll<HTMLElement>(
          '.registry-sdk__feature, .sdk-flow__graph > *, .registry-sdk__showcase',
        ),
      ]
        .map((child) => child.getBoundingClientRect())
        .filter((rect) => rect.left < bounds.left - 0.5 || rect.right > bounds.right + 0.5).length;
    });
    expect(overflow).toBe(0);
    if (width <= 720) {
      const columns = await section
        .locator('.registry-sdk__feature')
        .evaluateAll(
          (nodes) => new Set(nodes.map((node) => Math.round(node.getBoundingClientRect().x))).size,
        );
      expect(columns).toBe(1);
    }
  });
}
