import { expect, test, type Page } from '@playwright/test'

function observeFailures(page: Page) {
  const failures: string[] = []
  page.on('console', (message) => {
    if (message.type() === 'error') failures.push(`console: ${message.text()}`)
  })
  page.on('pageerror', error => failures.push(`page: ${error.message}`))
  page.on('requestfailed', request => failures.push(`request: ${request.method()} ${request.url()} (${request.failure()?.errorText ?? 'unknown'})`))
  page.on('response', (response) => {
    if (response.status() >= 400) failures.push(`response: ${response.status()} ${response.url()}`)
  })
  return failures
}

async function expectNoHorizontalOverflow(page: Page) {
  const widths = await page.evaluate(() => ({
    client: document.documentElement.clientWidth,
    scroll: document.documentElement.scrollWidth,
  }))
  expect(widths.scroll, `document is ${widths.scroll - widths.client}px wider than its viewport`).toBeLessThanOrEqual(widths.client + 1)
}

test('hydrates finalized CSP pages without runtime or layout failures', async ({ page }) => {
  const failures = observeFailures(page)
  for (const path of ['./', 'plugins', 'plugins/chrome-devtools']) {
    await page.goto(path)
    await expect(page.locator('main')).toBeVisible()
    await expect(page.locator('h1')).toHaveCount(1)
    await expect(page.locator('meta[http-equiv="Content-Security-Policy"]')).toHaveCount(1)
    await expectNoHorizontalOverflow(page)
  }
  expect(failures).toEqual([])
})

test('operates the target multi-select entirely by keyboard', async ({ page }) => {
  const failures = observeFailures(page)
  await page.goto('./')

  const trigger = page.getByRole('button', { name: /Choose target clients:/ })
  await trigger.focus()
  await trigger.press('Enter')
  const codex = page.getByRole('checkbox', { name: /Codex/ })
  await codex.focus()
  await codex.press('Space')
  await expect(codex).toBeChecked()
  await page.keyboard.press('Escape')
  await expect(trigger).toBeFocused()
  await expectNoHorizontalOverflow(page)
  expect(failures).toEqual([])
})

test('operates combobox and select catalog filters entirely by keyboard', async ({ page }) => {
  const failures = observeFailures(page)
  await page.goto('plugins')

  const category = page.getByRole('combobox', { name: 'Filter by category' })
  await category.focus()
  await category.press('Control+A')
  await category.pressSequentially('docs')
  await category.press('ArrowDown')
  await category.press('Enter')
  await expect(category).toHaveValue('docs')

  const component = page.getByRole('combobox', { name: 'Filter by component' })
  await component.focus()
  await component.press('Space')
  await page.getByRole('option', { name: /^mcp/ }).press('Enter')
  await expect(component).toContainText('mcp')

  const source = page.getByRole('combobox', { name: 'Filter by source' })
  await source.focus()
  await source.press('Space')
  await page.getByRole('option', { name: /^Community bridges/ }).press('Enter')
  await expect(source).toContainText('Community bridges')

  const cards = page.locator('.plugin-card')
  await expect(cards).toHaveCount(1)
  await expectNoHorizontalOverflow(page)
  expect(failures).toEqual([])
})

test('keeps bridge alternatives on one product page', async ({ page }) => {
  const failures = observeFailures(page)
  await page.goto('plugins/chrome-devtools')
  await expect(page.locator('.distribution-list')).toContainText('Community bridge')
  await expect(page.getByRole('heading', { name: 'Product release history' })).toBeVisible()
  await expect(page.locator('.distribution-list > li')).toHaveCount(2)
  expect(failures).toEqual([])
})

test('renders target authentication distinctly and keeps it tied to multiselect targets', async ({ page }) => {
  const failures = observeFailures(page)
  await page.goto('plugins')

  const search = page.getByRole('searchbox', { name: 'Search plugins' })
  await search.fill('Agent Code Navigator')
  const navigator = page.locator('.plugin-card').filter({ hasText: 'Agent Code Navigator' })
  await expect(navigator.locator('.plugin-card__auth')).toHaveText('No account required')
  await navigator.getByRole('button', { name: /Choose clients for Agent Code Navigator:/ }).click()
  await page.getByRole('checkbox', { name: /Codex/ }).click()
  await page.keyboard.press('Escape')
  await expect(navigator.locator('.plugin-card__auth')).toHaveText('No account required')
  await expect(navigator.getByRole('button', { name: /Choose clients for Agent Code Navigator: 2 agents/ })).toBeVisible()

  await search.fill('Atlassian')
  const atlassian = page.locator('.plugin-card').filter({ hasText: 'Atlassian' })
  await expect(atlassian.locator('.plugin-card__auth')).toHaveText('Authentication required')

  await page.goto('plugins/atlassian')
  await expect(page.locator('.distribution-list')).toContainText('codex — Managed install; Authentication required')
  await page.goto('plugins/agent-code-navigator')
  await expect(page.locator('.distribution-list')).toContainText('codex — Managed install; No account required')
  await expectNoHorizontalOverflow(page)
  expect(failures).toEqual([])
})

test('unsigned pull-request preview exposes no copyable install command', async ({ page }) => {
  const failures = observeFailures(page)
  for (const path of ['./', 'plugins/chrome-devtools']) {
    await page.goto(path)
    await expect(page.getByRole('button', { name: /Copy command|Command copied/ })).toHaveCount(0)
    await expect(page.getByText(/Commands? unavailable.*review preview/i)).toBeVisible()
  }
  expect(failures).toEqual([])
})
