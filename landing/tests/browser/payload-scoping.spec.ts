import { expect, test } from '@playwright/test';

test('static pages receive only their route-scoped reviewed registry data', async ({ request }) => {
  const [detail, download, pluginApi, emptyApi] = await Promise.all([
    request.get('./plugins/context7/'),
    request.get('./download/'),
    request.get('./api/registry/plugin/context7'),
    request.get('./api/registry/empty'),
  ]);
  for (const response of [detail, download, pluginApi, emptyApi]) expect(response.ok()).toBe(true);

  const detailHtml = await detail.text();
  const downloadHtml = await download.text();
  expect(Buffer.byteLength(detailHtml)).toBeLessThan(450_000);
  expect(Buffer.byteLength(downloadHtml)).toBeLessThan(450_000);
  expect(detailHtml).not.toContain('registryIndex');
  expect(downloadHtml).not.toContain('registryIndex');
  expect(detailHtml).not.toContain('Community package for the official Atlassian');

  const plugin = (await pluginApi.json()) as { plugins: Array<{ name: string }> };
  const empty = (await emptyApi.json()) as { plugins: unknown[] };
  expect(plugin.plugins.map((item) => item.name)).toEqual(['context7']);
  expect(empty.plugins).toEqual([]);
});
