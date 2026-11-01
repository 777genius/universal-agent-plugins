import { expect, test } from '@playwright/test';

function skill(index = 0) {
  return {
    id: `skills-sh:anthropics/skills:frontend-design-${index}`,
    providerId: 'skills-sh',
    providerUrl: 'https://skills.sh/anthropics/skills/frontend-design',
    title: index === 0 ? 'frontend-design' : `Example skill ${index}`,
    skillNameHint: 'frontend-design',
    source: {
      kind: 'github',
      repository: 'anthropics/skills',
      url: 'https://github.com/anthropics/skills',
    },
    metrics: { installs: 1234 - index, window: 'all-time' },
    availability: 'unverified',
  };
}

test('Skills search reveals install choices, copies the exact command and survives catalog navigation', async ({
  page,
  context,
}) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  const requests: string[] = [];
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', async (route) => {
    const url = new URL(route.request().url());
    requests.push(url.search);
    await route.fulfill({
      json: {
        items: [
          skill(),
          {
            ...skill(1),
            source: { kind: 'external', url: 'https://skills.sh/example/external' },
            availability: 'unsupported-source',
          },
        ],
        page: 0,
        total: 9911,
        hasMore: false,
        view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-10-24T10:00:00Z' },
      },
    });
  });
  await page.goto('plugins/?tab=skills');
  const catalog = page.locator('.skills-catalog');
  await expect(
    page.getByRole('heading', { level: 1, name: 'Find your Skills', exact: true }),
  ).toBeVisible();
  await expect(page.locator('h1')).toHaveCount(1);
  await expect(page.locator('.catalog-modes').locator('xpath=..')).toHaveAttribute(
    'id',
    'unified-catalog-title',
  );
  await expect(catalog.locator('.skills-catalog__disclosure')).toHaveCount(0);
  await expect(catalog.locator('.catalog-meta button')).toHaveCount(0);
  await expect(catalog.locator('.plugin-card__install-panel')).toHaveCount(0);
  await catalog.getByRole('searchbox', { name: 'Search Skills', exact: true }).fill('frontend');
  await expect.poll(() => requests.some((query) => query.includes('q=frontend'))).toBe(true);
  await expect(page).toHaveURL(/tab=skills.*q=frontend/);
  await expect(catalog.locator('.catalog-count')).toHaveText('Showing 2 search results');
  // Provider ranking metadata must never create a competitor outbound link.
  await expect(catalog.locator('a[href*="skills.sh"]')).toHaveCount(0);
  const card = catalog.locator('.skill-card').first();
  await expect(card.getByRole('link', { name: 'GitHub source' })).toHaveAttribute(
    'href',
    'https://github.com/anthropics/skills',
  );
  await card.getByRole('button', { name: 'Install frontend-design', exact: true }).click();
  await card
    .getByRole('button', { name: 'Choose agents: Choose agents in terminal', exact: true })
    .click();
  await expect(page.getByRole('checkbox', { name: /ChatGPT|VS Code/ })).toHaveCount(0);
  await page.getByRole('checkbox', { name: /Codex/ }).click();
  await page.keyboard.press('Escape');
  await expect(card.locator('.plugin-card__install-panel pre')).toContainText('--target codex');
  await card.getByRole('button', { name: 'Copy command', exact: true }).click();
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe(
      'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design --target codex',
    );
  await page.locator('.catalog-modes').getByRole('link', { name: 'Plugins', exact: true }).click();
  await expect(page.locator('.skills-catalog')).toHaveCount(0);
  await expect(
    page.getByRole('heading', { level: 1, name: 'Find your Plugins', exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('searchbox', { name: 'Search plugins', exact: true })).toHaveValue(
    'frontend',
  );
  await page.goBack();
  await expect(catalog.getByRole('searchbox', { name: 'Search Skills', exact: true })).toHaveValue(
    'frontend',
  );
  await expect(
    catalog.locator('.skill-card').first().locator('.plugin-card__install-panel pre'),
  ).toContainText('--target codex');
  await page.goto('./?tab=skills');
  await expect(
    page.getByRole('heading', { level: 2, name: 'Find your Skills', exact: true }),
  ).toBeVisible();
});

test('Skills renders in 48-item increments and requests the next upstream page only after loaded items', async ({
  page,
}) => {
  const pages: number[] = [];
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', async (route) => {
    const currentPage = Number(new URL(route.request().url()).searchParams.get('page'));
    pages.push(currentPage);
    await route.fulfill({
      json: {
        items: Array.from({ length: 200 }, (_, index) => skill(index + currentPage * 200)),
        page: currentPage,
        total: 9911,
        hasMore: currentPage === 0,
        view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-10-24T10:00:00Z' },
      },
    });
  });
  await page.goto('plugins/?tab=skills');
  const catalog = page.locator('.skills-catalog');
  await expect(catalog.locator('.skill-card')).toHaveCount(48);
  await expect(catalog.locator('.catalog-count')).toHaveText(
    'Showing 48 of 9,911 in this collection',
  );
  await expect(catalog.locator('a[href*="skills.sh"]')).toHaveCount(0);
  await expect(catalog.getByRole('button', { name: /Show more Skills/ })).toHaveCount(1);
  await expect(catalog.locator('.catalog-meta button')).toHaveCount(0);
  for (const count of [96, 144, 192, 200]) {
    await catalog
      .getByRole('button', { name: /Show more Skills/ })
      .last()
      .click();
    await expect(catalog.locator('.skill-card')).toHaveCount(count);
    await expect(catalog.locator('.catalog-count')).toHaveText(
      `Showing ${count} of 9,911 in this collection`,
    );
    expect(pages).toEqual([0]);
  }
  await catalog
    .getByRole('button', { name: /Show more Skills/ })
    .last()
    .click();
  await expect(catalog.locator('.skill-card')).toHaveCount(248);
  expect(pages).toEqual([0, 1]);
});

test('unsupported standalone source is explicit and never requests a report', async ({ page }) => {
  const reportRequests: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/reports/skillspector/')) reportRequests.push(request.url());
  });
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) =>
    route.fulfill({
      json: {
        items: [
          {
            ...skill(),
            source: { kind: 'external', url: 'https://skills.sh/example/external' },
            availability: 'unsupported-source',
          },
        ],
        page: 0,
        total: 1,
        hasMore: false,
        view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-10-28T10:00:00Z' },
      },
    }),
  );
  await page.goto('plugins/?tab=skills');
  const assessment = page.locator('.skill-card .skillspector');
  await expect(assessment).toHaveClass(/skillspector--unsupported/);
  await expect(assessment.locator(':scope > summary')).toContainText('Not checked');
  await assessment.locator(':scope > summary').click();
  await expect(assessment).toContainText('This source type is not supported');
  await expect(assessment).not.toContainText('other plugin components');
  expect(reportRequests).toEqual([]);
});

test('unverified GitHub Skill names open the installer picker without guessing a selector', async ({
  page,
}) => {
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) =>
    route.fulfill({
      json: {
        items: [{ ...skill(), title: 'Unverified listing', skillNameHint: 'BAD$(echo injected)' }],
        page: 0,
        total: 1,
        hasMore: false,
        view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-10-30T10:00:00Z' },
      },
    }),
  );
  await page.goto('plugins/?tab=skills');
  const card = page.locator('.skills-catalog .skill-card').first();
  await card.getByRole('button', { name: 'Install Unverified listing', exact: true }).click();
  await expect(card.locator('.command-snippet code')).toHaveText(
    'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills',
  );
  await expect(card).toContainText(
    'Select the exact Skill in the installer; this catalog name is not verified.',
  );
  await expect(card.locator('.command-snippet')).not.toContainText('BAD$(echo injected)');
});


test('Website publisher Skills install with UAP and open their own detail page', async ({
  page,
  context,
}) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  const website = {
    ...skill(),
    id: 'skills-sh:open.feishu.cn:lark-mail',
    providerUrl: 'https://www.skills.sh/site/open.feishu.cn/lark-mail',
    title: 'lark-mail',
    skillNameHint: 'lark-mail',
    source: { kind: 'external', url: 'https://open.feishu.cn' },
    availability: 'unverified',
  };
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) =>
    route.fulfill({
      json: {
        items: [website],
        page: 0,
        total: 1,
        hasMore: false,
        view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-10-31T10:00:00Z' },
      },
    }),
  );
  await page.route('https://api.universal-agent-plugins.com/v1/skills/document?**', (route) =>
    route.fulfill({
      json: {
        status: 'resolved',
        source: 'open.feishu.cn',
        markdown: [
          '---', 'name: lark-mail', 'description: Mail', '---',
          '# Mail', '', '**Important instructions**', '',
          '| Action | Command |', '| --- | --- |', '| Read | open |', '',
          '```bash', 'echo hello', '```', '',
          '<script>alert("xss")</script>',
          '[Bad](javascript:alert(1))',
          '[Competitor](https://www.skills.sh/site/something)',
        ].join('\n'),
      },
    }),
  );
  await page.goto('plugins/?tab=skills');
  const card = page.locator('.skills-catalog .skill-card');
  await expect(card).not.toContainText('No verified install source');
  await expect(card.getByRole('link', { name: 'Publisher source' })).toHaveAttribute(
    'href',
    'https://open.feishu.cn',
  );
  await card.getByRole('button', { name: 'Install lark-mail' }).click();
  await expect(card.locator('.command-snippet code')).toContainText(
    'npx universal-agent-plugins@latest skills add https://open.feishu.cn --skill lark-mail',
  );
  await card.getByRole('link', { name: 'lark-mail', exact: true }).click();
  await expect(page).toHaveURL(/\/skills\/\?id=skills-sh/);
  await expect(page.getByRole('heading', { level: 1, name: 'lark-mail' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Installation' })).toBeVisible();
  await page.getByRole('tab', { name: 'Prompt' }).click();
  await expect(page.locator('.skills-detail__install-body .command-snippet')).toContainText(
    'Review the source',
  );
  await page.getByRole('tab', { name: 'Command' }).click();
  await page.getByRole('tab', { name: 'Command' }).press('ArrowRight');
  await expect(page.getByRole('tab', { name: 'Prompt' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('tabpanel')).toHaveAttribute('aria-labelledby', 'skills-prompt-tab');
  await page.getByRole('tab', { name: 'Prompt' }).press('Home');
  await expect(page.getByRole('tab', { name: 'Command' })).toHaveAttribute('aria-selected', 'true');
  await page.locator('.skills-detail__install-body').getByRole('button', { name: 'Copy command' }).click();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(
    'npx universal-agent-plugins@latest skills add https://open.feishu.cn --skill lark-mail',
  );
  await page.reload();
  await expect(page.getByRole('heading', { level: 1, name: 'lark-mail' })).toBeVisible();
  await expect(page.locator('.skills-detail__sidebar')).toContainText('open.feishu.cn');
  await expect(page.locator('.skills-detail__document .skill-markdown h1')).toHaveText('Mail');
  await expect(page.locator('.skills-detail__document .skill-markdown strong')).toHaveText('Important instructions');
  await expect(page.locator('.skills-detail__document .skill-markdown table')).toBeVisible();
  await expect(page.locator('.skills-detail__document .skill-markdown pre')).toBeVisible();
  await expect(page.locator('.skills-detail__document script')).toHaveCount(0);
  await expect(page.locator('a[href*="skills.sh"]')).toHaveCount(0);
});


test('A deep-linked Skill renders source instructions without relying on bounded search', async ({ page }) => {
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) =>
    route.fulfill({
      json: {
        items: [], page: 0, hasMore: false, view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-11-01T10:00:00Z' },
      },
    }),
  );
  await page.route('https://api.universal-agent-plugins.com/v1/skills/document?**', (route) =>
    route.fulfill({
      json: {
        status: 'resolved', source: 'anthropics/skills',
        commit: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        markdown: '---\nname: frontend-design\ndescription: Design\n---\n## Direct source content\n',
      },
    }),
  );
  await page.goto('skills/?id=skills-sh%3Aanthropics%2Fskills%3Afrontend-design&q=frontend-design');
  await expect(page.getByRole('heading', { level: 1, name: 'frontend-design' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Installation' })).toBeVisible();
  await expect(page.locator('.skills-detail__document h2')).toContainText('Direct source content');
  await expect(page.locator('.skills-detail__facts')).not.toContainText('Installs');
  await expect(page.locator('a[href*="skills.sh"]')).toHaveCount(0);
});
