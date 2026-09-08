import { expect, test } from '@playwright/test';

async function waitForDiscovery(page: import('@playwright/test').Page) {
  await expect(page.locator('.catalog')).toHaveAttribute('data-discovery-state', /current|cached/, {
    timeout: 15_000,
  });
}

test('catalog groups duplicate sources and tolerates a small typo', async ({ page }) => {
  await page.goto('./plugins/');
  await waitForDiscovery(page);
  const search = page.getByRole('searchbox', { name: 'Search plugins' });

  await search.fill('context7');
  await expect(page.getByRole('heading', { name: 'Context7', exact: true })).toHaveCount(1);
  const group = page
    .locator('.plugin-card')
    .filter({ has: page.getByRole('heading', { name: 'Context7', exact: true }) });
  await expect(group).toHaveCount(1);
  const alternatives = group.locator('.plugin-other-sources');
  await expect(alternatives).toBeVisible();
  await expect(group.locator('.plugin-card__ribbon')).toHaveText('reviewed listing');
  await alternatives.locator('summary').click();
  await expect(alternatives.locator('li')).not.toHaveCount(0);
  await expect(group).not.toContainText('777genius/universal-agent-plugins-registry');

  await search.fill('contex7');
  await expect(page.getByRole('heading', { name: 'Context7', exact: true })).toHaveCount(1);
});

test('catalog URL preserves filters through detail navigation and Reset is always available', async ({
  page,
}) => {
  await page.goto('./plugins/');
  await waitForDiscovery(page);
  const search = page.getByRole('searchbox', { name: 'Search plugins' });
  const reset = page.getByRole('button', { name: 'Reset filters', exact: true });
  await expect(reset).toBeDisabled();

  await search.fill('gitlab');
  await expect(page).toHaveURL(/\?q=gitlab$/);
  await expect(reset).toBeEnabled();
  await expect(page.getByRole('button', { name: 'Remove Search: gitlab' })).toBeVisible();

  await page.getByRole('heading', { name: 'GitLab', exact: true }).getByRole('link').click();
  await expect(page).toHaveURL(/\/plugins\/gitlab\/$/);
  await page.goBack();
  await expect(page).toHaveURL(/\/plugins\/\?q=gitlab$/);
  await expect(search).toHaveValue('gitlab');

  await reset.click();
  await expect(page).toHaveURL(/\/plugins\/$/);
  await expect(search).toHaveValue('');
  await expect(reset).toBeDisabled();
});

test('community source labels link to the exact GitHub source', async ({ page }) => {
  await page.goto('./plugins/');
  await waitForDiscovery(page);
  await page.getByRole('searchbox', { name: 'Search plugins' }).fill('hindsight');

  const card = page
    .locator('.plugin-card')
    .filter({ has: page.getByRole('heading', { name: /^hindsight$/i }) });
  const source = card.locator('.plugin-card__source-label a');
  await expect(source).toHaveText('vectorize-io/hindsight/hindsight-integrations/agent-plugin');
  await expect(source).toHaveAttribute(
    'href',
    /^https:\/\/github\.com\/vectorize-io\/hindsight\/tree\/[0-9a-f]{40}\/hindsight-integrations\/agent-plugin$/,
  );
  const popularity = card.locator('.plugin-card__popularity-trigger');
  await expect(popularity).not.toHaveAttribute('title');
  await popularity.focus();
  await expect(page.locator('.app-tooltip')).toContainText(
    "GitHub repository's star count, not a rating for this individual plugin",
  );
});
