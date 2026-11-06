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
  await page.goto('plugins/?tab=skills&skillScan=all');
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
  await expect(catalog.locator('.skill-card .skillspector')).toHaveCount(2);
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
  await page.goto('./?tab=skills&skillScan=all');
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
  await page.goto('plugins/?tab=skills&skillScan=all');
  const catalog = page.locator('.skills-catalog');
  await expect(catalog.locator('.skill-card')).toHaveCount(48);
  await expect(catalog.locator('.catalog-count')).toHaveText('Showing 48 Skills');
  await expect(catalog.locator('a[href*="skills.sh"]')).toHaveCount(0);
  await expect(catalog.getByRole('button', { name: /Show more Skills/ })).toHaveCount(1);
  await expect(catalog.locator('.catalog-meta button')).toHaveCount(0);
  for (const count of [96, 144, 192, 200]) {
    await catalog
      .getByRole('button', { name: /Show more Skills/ })
      .last()
      .click();
    await expect(catalog.locator('.skill-card')).toHaveCount(count);
    await expect(catalog.locator('.catalog-count')).toHaveText(`Showing ${count} Skills`);
    expect(pages).toEqual([0]);
  }
  await catalog
    .getByRole('button', { name: /Show more Skills/ })
    .last()
    .click();
  await expect(catalog.locator('.skill-card')).toHaveCount(248);
  expect(pages).toEqual([0, 1]);
});

test('unresolvable external Skills remain visible without install authority or scan claims', async ({ page }) => {
  const reports: string[] = [];
  page.on('request', request => {
    if (request.url().includes('/reports/skillspector/')) reports.push(request.url());
  });
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route =>
    route.fulfill({ json: {
      items: [{ ...skill(), source: { kind: 'external', url: 'https://www.skills.sh/' },
        providerUrl: 'https://www.skills.sh/', availability: 'unsupported-source',
        id: 'skills-sh:owner/repo/path:unusable' }],
      page: 0, total: 1, hasMore: false, view: 'popular',
      cache: { status: 'fresh', fetchedAt: '2026-11-02T10:00:00Z' },
    } }),
  );
  await page.goto('plugins/?tab=skills&skillScan=all');
  const card = page.locator('.skills-catalog .skill-card');
  await expect(card).toHaveCount(1);
  await expect(card).toContainText('No verified install source');
  await expect(card.getByRole('button', { name: /Install/ })).toHaveCount(0);
  await expect(card.locator('.skillspector')).toContainText('Not checked');
  expect(reports).toEqual([]);
});

test('legacy API website publisher offers safe Install while malformed sources remain unavailable', async ({ page }) => {
  const legacyUrl = 'https://www.skills.sh/site/uizze.com/ui-radar';
  const legacy = {
    ...skill(),
    id: 'skills-sh:uizze.com:ui-radar',
    title: 'ui-radar',
    skillNameHint: 'ui-radar',
    providerUrl: legacyUrl,
    source: { kind: 'external', url: legacyUrl },
    availability: 'unsupported-source',
  };
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', async (route) => {
    if (new URL(route.request().url()).pathname.endsWith('/document')) {
      await route.fulfill({ status: 503, json: { code: 'document_unavailable' } });
      return;
    }
    await route.fulfill({
      json: {
        items: [
          legacy,
          {
            ...skill(1),
            id: 'skills-sh:owner/repo/not-a-github-form:unusable',
            title: 'Unrecoverable source',
            source: { kind: 'external', url: 'https://www.skills.sh/' },
            providerUrl: 'https://www.skills.sh/',
            availability: 'unsupported-source',
          },
          {
            ...skill(2),
            id: 'skills-sh:xn--a.example:ui-radar',
            title: 'Malformed publisher',
            source: { kind: 'external', url: 'https://www.skills.sh/site/xn--a.example/ui-radar' },
            providerUrl: 'https://www.skills.sh/site/xn--a.example/ui-radar',
            availability: 'unsupported-source',
          },
        ],
        page: 0,
        total: 3,
        hasMore: false,
        view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-11-02T10:00:00Z' },
      },
    });
  });

  await page.goto('plugins/?tab=skills&skillScan=all');
  const cards = page.locator('.skills-catalog .skill-card');
  await expect(cards).toHaveCount(3);
  await expect(cards.nth(1).getByRole('button', { name: /Install/ })).toHaveCount(0);
  await expect(cards.nth(2).getByRole('button', { name: /Install/ })).toHaveCount(0);
  const recovered = cards.first();
  await expect(recovered.getByRole('link', { name: 'Publisher source' })).toHaveAttribute(
    'href', 'https://uizze.com',
  );
  await recovered.getByRole('button', { name: 'Install ui-radar' }).click();
  await expect(recovered.locator('.command-snippet code')).toHaveText(
    'npx universal-agent-plugins@latest skills add https://uizze.com --skill ui-radar',
  );
  await expect(recovered.locator('.command-snippet code')).not.toContainText('skills.sh');
  await recovered.getByRole('link', { name: 'ui-radar', exact: true }).click();
  await expect(page).toHaveURL(/\/skills\/\?id=skills-sh/);
  await expect(page.getByRole('heading', { level: 1, name: 'ui-radar' })).toBeVisible();
  await expect(page.locator('.skills-detail__install-body .command-snippet')).toContainText(
    'https://uizze.com --skill ui-radar',
  );
});

test('known non-installable Feishu sources stay visible with Install disabled', async ({ page }) => {
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route =>
    route.fulfill({ json: {
      items: ['open.feishu.cn', 'open.feishu.cn/lark-cli/skills/regular'].map(source => ({
        ...skill(), id: `skills-sh:${source}:lark-mail`,
        skillNameHint: 'lark-mail', title: 'lark-mail',
        providerUrl: 'https://www.skills.sh/',
        source: { kind: 'external', url: 'https://www.skills.sh/' },
        availability: 'unsupported-source',
      })),
      page: 0, total: 2, hasMore: false, view: 'popular',
      cache: { status: 'fresh', fetchedAt: '2026-11-02T10:00:00Z' },
    } }),
  );
  await page.goto('plugins/?tab=skills&skillScan=all');
  const cards = page.locator('.skills-catalog .skill-card');
  await expect(cards).toHaveCount(2);
  await expect(cards.getByRole('button', { name: /Install/ })).toHaveCount(0);
  await expect(cards.first()).toContainText('No verified install source');
});

test('malformed publisher deep link degrades without crashing document rendering', async ({ page }) => {
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith('/document')) {
      return route.fulfill({
        json: { status: 'selection_required', source: 'xn--a.example' },
      });
    }
    return route.fulfill({
      json: {
        items: [], page: 0, total: 0, hasMore: false, view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-11-02T10:00:00Z' },
      },
    });
  });
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('skills/?id=skills-sh%3Axn--a.example%3Alark-mail&q=lark-mail');
  await expect(page.getByRole('heading', { level: 1, name: 'Skill not found' })).toBeVisible();
  expect(errors).toEqual([]);
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
  await page.goto('plugins/?tab=skills&skillScan=all');
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
    id: 'skills-sh:uizze.com:ui-radar',
    providerUrl: 'https://www.skills.sh/site/uizze.com/ui-radar',
    title: 'ui-radar',
    skillNameHint: 'ui-radar',
    source: { kind: 'external', url: 'https://uizze.com' },
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
        source: 'uizze.com',
        markdown: [
          '---', 'name: ui-radar', 'description: UI radar', '---',
          '# UI Radar', '', '**Important instructions**', '',
          '| Action | Command |', '| --- | --- |', '| Read | open |', '',
          '```bash', 'echo hello', '```', '',
          '<script>alert("xss")</script>',
          '[Bad](javascript:alert(1))',
          '[Competitor](https://www.skills.sh/site/something)',
        ].join('\n'),
      },
    }),
  );
  await page.goto('plugins/?tab=skills&skillScan=all');
  const card = page.locator('.skills-catalog .skill-card');
  await expect(card).not.toContainText('No verified install source');
  await expect(card.getByRole('link', { name: 'Publisher source' })).toHaveAttribute(
    'href',
    'https://uizze.com',
  );
  await card.getByRole('button', { name: 'Install ui-radar' }).click();
  await expect(card.locator('.command-snippet code')).toContainText(
    'npx universal-agent-plugins@latest skills add https://uizze.com --skill ui-radar',
  );
  await card.getByRole('link', { name: 'ui-radar', exact: true }).click();
  await expect(page).toHaveURL(/\/skills\/\?id=skills-sh/);
  await expect(page.getByRole('heading', { level: 1, name: 'ui-radar' })).toBeVisible();
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
    'npx universal-agent-plugins@latest skills add https://uizze.com --skill ui-radar',
  );
  await page.reload();
  await expect(page.getByRole('heading', { level: 1, name: 'ui-radar' })).toBeVisible();
  await expect(page.locator('.skills-detail__sidebar')).toContainText('uizze.com');
  await expect(page.locator('.skills-detail__document .skill-markdown h1')).toHaveText('UI Radar');
  await expect(page.locator('.skills-detail__document .skill-markdown strong')).toHaveText('Important instructions');
  await expect(page.locator('.skills-detail__document .skill-markdown table')).toBeVisible();
  await expect(page.locator('.skills-detail__document .skill-markdown__rendered pre')).toBeVisible();
  const raw = page.locator('.skill-markdown__raw');
  await raw.locator('summary').click();
  await expect(raw.locator('pre')).toContainText('description: UI radar');
  await expect(raw.locator('pre')).toContainText('name: ui-radar');
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
  await expect(page.locator('.skills-detail__document .skill-markdown h2')).toContainText('Direct source content');
  await expect(page.locator('.skills-detail__facts')).not.toContainText('Installs');
  await expect(page.locator('a[href*="skills.sh"]')).toHaveCount(0);
});


test('catalog preserves upstream ordering even for non-installable Skills', async ({ page }) => {
  const requested: number[] = [];
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route => {
    const pageNumber = Number(new URL(route.request().url()).searchParams.get('page'));
    requested.push(pageNumber);
    const items = Array.from({ length: 200 }, (_, index) =>
      pageNumber === 0 && index < 175
        ? { ...skill(index), id: `skills-sh:unknown/path/subfolder:${index}`,
            source: { kind: 'external', url: 'https://www.skills.sh/' },
            providerUrl: 'https://www.skills.sh/', availability: 'unsupported-source' }
        : skill(index + pageNumber * 200));
    return route.fulfill({
      json: { items, page: pageNumber, total: 9911,
        hasMore: pageNumber === 0, view: 'popular',
        cache: { status: 'fresh', fetchedAt: '2026-11-02T10:00:00Z' } },
    });
  });
  await page.goto('plugins/?tab=skills&skillScan=all');
  const cards = page.locator('.skills-catalog .skill-card');
  await expect(cards).toHaveCount(48);
  expect(requested).toEqual([0]);
  await expect(page.locator('.catalog-count')).toHaveText('Showing 48 Skills');
  await expect(cards.filter({ hasText: 'External source' })).toHaveCount(48);
  await expect(cards.first().getByRole('button', { name: /Install/ })).toHaveCount(0);
  for (const count of [96, 144, 192, 200]) {
    await page.getByRole('button', { name: /Show more Skills/ }).last().click();
    await expect(cards).toHaveCount(count);
  }
  await expect(cards.nth(175).getByRole('button', { name: /Install/ })).toHaveCount(1);
  expect(requested).toEqual([0]);
  await page.getByRole('button', { name: /Show more Skills/ }).last().click();
  await expect(cards).toHaveCount(248);
  expect(requested).toEqual([0, 1]);
});


test('document API rejects noncanonical remote source detail links without a failed fetch', async ({ page }) => {
  let documentRequests = 0;
  await page.route('https://api.universal-agent-plugins.com/v1/skills/document?**', route => {
    documentRequests++;
    return route.fulfill({ status: 400, json: { error: 'invalid_request' } });
  });
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route =>
    route.fulfill({ json: {
      items: [{ ...skill(), id: 'skills-sh:anthropics/skills.git:frontend-design',
        source: { kind: 'external', url: 'https://www.skills.sh/' },
        providerUrl: 'https://www.skills.sh/', availability: 'unsupported-source' }],
      page: 0, total: 1, hasMore: false, view: 'popular',
      cache: { status: 'fresh', fetchedAt: '2026-11-02T10:00:00Z' },
    } }),
  );
  await page.goto('plugins/?tab=skills&skillScan=all');
  const card = page.locator('.skills-catalog .skill-card').first();
  await expect(card).toHaveCount(1);
  await expect(card.getByRole('button', { name: 'Install frontend-design' })).toBeVisible();
  await expect(card.getByRole('link', { name: 'frontend-design', exact: true })).toHaveCount(0);
  await page.goto('skills/?id=skills-sh%3Aanthropics%2Fskills.git%3Afrontend-design&q=frontend-design');
  await expect(page.getByRole('heading', { name: 'Skill not found' })).toBeVisible();
  expect(documentRequests).toBe(0);
});

test('explicit legacy GitHub URL sources display a safe install command', async ({ page }) => {
  await page.route('https://api.universal-agent-plugins.com/v1/skills**', route =>
    route.fulfill({ json: {
      items: [{ ...skill(), id: 'skills-sh:https://github.com/anthropics/skills:frontend-design',
        source: { kind: 'external', url: 'https://www.skills.sh/' },
        providerUrl: 'https://www.skills.sh/', availability: 'unsupported-source' }],
      page: 0, total: 1, hasMore: false, view: 'popular',
      cache: { status: 'fresh', fetchedAt: '2026-11-02T10:00:00Z' },
    } }),
  );
  await page.goto('plugins/?tab=skills&skillScan=all');
  const card = page.locator('.skills-catalog .skill-card').first();
  await expect(card).toHaveCount(1);
  await card.getByRole('button', { name: 'Install frontend-design' }).click();
  await expect(card.locator('.command-snippet code')).toHaveText(
    'npx universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design',
  );
  await expect(card.getByRole('link', { name: 'GitHub source' })).toHaveAttribute(
    'href', 'https://github.com/anthropics/skills',
  );
  // A non-canonical listing ID is installable, but the document preview API
  // does not accept this URL-shaped ID. Keep the title non-navigable.
  await expect(card.getByRole('link', { name: 'frontend-design', exact: true })).toHaveCount(0);
});
